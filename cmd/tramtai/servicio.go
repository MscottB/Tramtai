package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/navegador"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/recursos"
	"tramtai/internal/servicio"
	"tramtai/internal/version"
)

// dirDatos es el directorio de datos locales del usuario (sección 12.1).
func dirDatos() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	datos := filepath.Join(dir, "tramtai", "datos")
	return datos, os.MkdirAll(datos, 0o700)
}

func contexto() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

func conectar() (*cliente.Cliente, error) {
	datos, err := dirDatos()
	if err != nil {
		return nil, err
	}
	return cliente.Conectar(datos)
}

// asegurarServicio conecta con el servicio o lo inicia en segundo plano (sección 5.1).
func asegurarServicio(cmd *cobra.Command) (*cliente.Cliente, error) {
	datos, err := dirDatos()
	if err != nil {
		return nil, err
	}
	if c, err := cliente.Conectar(datos); err == nil {
		return c, nil
	}
	ejecutable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "iniciando el servicio en segundo plano…")
	return cliente.IniciarEnSegundoPlano(ejecutable, datos)
}

// confirmar pregunta en la terminal; cualquier respuesta que no empiece por s o y es un no.
func confirmar(cmd *cobra.Command, pregunta string) bool {
	fmt.Fprint(cmd.ErrOrStderr(), pregunta+" [s/N] ")
	linea, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	linea = strings.ToLower(strings.TrimSpace(linea))
	return strings.HasPrefix(linea, "s") || strings.HasPrefix(linea, "y")
}

func comandoServicio() *cobra.Command {
	grupo := &cobra.Command{Use: "servicio", Short: "Servicio único: todas las instancias y puntos de acceso en un puerto"}
	var segundoPlano, si bool
	iniciar := &cobra.Command{Use: "iniciar", Short: "Inicia el servicio en primer plano (Ctrl+C lo detiene) o con -d en segundo plano",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !segundoPlano {
				return iniciarServicio(cmd)
			}
			c, err := asegurarServicio(cmd)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "servicio en marcha en %s\nWeb UI: %s (o tramtai abrir)\n", c.Base, c.Base)
			return nil
		}}
	iniciar.Flags().BoolVarP(&segundoPlano, "segundo-plano", "d", false, "inicia el servicio desacoplado de esta terminal")
	detener := &cobra.Command{Use: "detener", Short: "Detiene el servicio y todas sus instancias",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			if !si && !confirmar(cmd, "¿Detener el servicio y todas sus instancias?") {
				return nil
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.Detener(ctx); err != nil {
				return err
			}
			// El servicio espera a los trabajos en curso hasta servicio.plazo_drenaje_segundos (sección 17.3).
			plazo := 30 * time.Second
			if cfg, err := nucleo.CargarConfiguracion(""); err == nil {
				plazo = time.Duration(cfg.Entero("servicio.plazo_drenaje_segundos")) * time.Second
			}
			inicio, avisado := time.Now(), false
			for time.Since(inicio) < plazo+15*time.Second {
				if _, err := conectar(); err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), "servicio detenido")
					return nil
				}
				if !avisado && time.Since(inicio) > 2*time.Second {
					fmt.Fprintf(cmd.OutOrStdout(), "esperando a que terminen los trabajos en curso (hasta %s)…\n", plazo)
					avisado = true
				}
				time.Sleep(200 * time.Millisecond)
			}
			return errors.New("el servicio no confirmó la detención")
		}}
	detener.Flags().BoolVarP(&si, "si", "s", false, "no pedir confirmación")
	estado := &cobra.Command{Use: "estado", Short: "Muestra el servicio, sus instancias y sus puntos de acceso",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := conectar()
			if errors.Is(err, cliente.ErrServicioInactivo) {
				fmt.Fprintln(cmd.OutOrStdout(), "servicio: inactivo (tramtai servicio iniciar -d)")
				return nil
			}
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			e, err := c.Estado(ctx)
			if err != nil {
				return err
			}
			imprimirEstado(cmd.OutOrStdout(), e)
			return nil
		}}
	recargar := &cobra.Command{Use: "recargar", Short: "Aplica los cambios de puntos de acceso sin reiniciar",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			puntos, err := c.Recargar(ctx)
			if err != nil {
				return err
			}
			imprimirPuntos(cmd.OutOrStdout(), puntos)
			return nil
		}}
	token := &cobra.Command{Use: "token", Short: "Muestra el token de la instancia global (igual que tramtai global token)",
		RunE: func(cmd *cobra.Command, _ []string) error { return mostrarTokenInstancia(cmd, "global") }}
	grupo.AddCommand(iniciar, detener, estado, recargar, token)
	return grupo
}

func imprimirEstado(out io.Writer, e servicio.Estado) {
	fmt.Fprintf(out, "servicio activo en %s · %s · pid %d · desde %s\n", e.Base, e.Version, e.PID, e.IniciadoEn.Format("02-01-2006 15:04"))
	fmt.Fprintln(out, "\ninstancias:")
	if len(e.Instancias) == 0 {
		fmt.Fprintln(out, "  ninguna (tramtai global iniciar, o tramtai proyecto iniciar en un proyecto)")
	}
	for _, i := range e.Instancias {
		fmt.Fprintf(out, "  %-28s %-9s %s\n", i.ID, i.Ambito, resumenTrabajos(i.Trabajos))
		if i.Raiz != "" {
			fmt.Fprintf(out, "  %-28s raíz: %s\n", "", i.Raiz)
		}
		fmt.Fprintf(out, "  %-28s token: %s\n", "", i.ArchivoToken)
	}
	fmt.Fprintln(out, "\npuntos de acceso:")
	imprimirPuntos(out, e.Puntos)
	for _, err := range e.Errores {
		fmt.Fprintln(out, "configuración: "+err)
	}
}

func imprimirPuntos(out io.Writer, puntos []servicio.EstadoPunto) {
	if len(puntos) == 0 {
		fmt.Fprintln(out, "  ninguno (tramtai acceso crear <nombre>)")
	}
	for _, p := range puntos {
		estado := "activo"
		if p.Error != "" {
			estado = p.Error
		}
		fmt.Fprintf(out, "  %-18s %-40s %-24s %s\n", p.Nombre, valorO(p.URL, "—"), "["+strings.Join(p.Contratos, ",")+"]", estado)
	}
}

func resumenTrabajos(conteo map[string]int) string {
	if len(conteo) == 0 {
		return "sin trabajos"
	}
	estados := make([]string, 0, len(conteo))
	for estado, n := range conteo {
		estados = append(estados, estado+" "+strconv.Itoa(n))
	}
	sort.Strings(estados)
	return strings.Join(estados, " · ")
}

func valorO(texto, alternativa string) string {
	if texto == "" {
		return alternativa
	}
	return texto
}

// iniciarServicio arranca el servicio en primer plano con las instancias que estaban activas.
func iniciarServicio(cmd *cobra.Command) error {
	c, err := nucleo.CargarConfiguracion("")
	if err != nil {
		return err
	}
	host, puerto := c.Texto("servicio.host"), c.Entero("servicio.puerto")
	remoto := !esLoopback(host)
	if remoto {
		if !c.Booleano("servicio.permitir_remoto") {
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "servicio.host "+host+" no es loopback: activa servicio.permitir_remoto (mejor, usa un túnel SSH)")
		}
		for _, agente := range []string{"claude", "codex"} {
			if c.Texto("adaptadores."+agente+".autenticacion") == adaptadores.AutenticacionSuscripcion {
				return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "el acceso remoto no se permite con "+agente+" en modo suscripción (sección 2.4)")
			}
		}
	}
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	if activo, err := cliente.Conectar(datos); err == nil {
		return fmt.Errorf("el servicio ya está en marcha en %s", activo.Base)
	}
	hosts := []string{"127.0.0.1", "localhost", "::1"}
	if remoto {
		hosts = append(hosts, strings.ToLower(host))
	}
	salida := cmd.ErrOrStderr()
	registro, limites := adaptadores.Registro(c)
	sv, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: registro, Semaforos: planificador.NuevosSemaforos(limites),
		Host: host, Puerto: puerto, Hosts: hosts, Registros: salida, Web: recursos.Web(),
		PlazoDrenaje: time.Duration(c.Entero("servicio.plazo_drenaje_segundos")) * time.Second,
		EntradaLocal: c.Booleano("servicio.entrada_local")})
	if err != nil {
		return err
	}
	direccion := net.JoinHostPort(host, strconv.Itoa(puerto))
	oyente, err := net.Listen("tcp", direccion)
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en %s (¿el puerto está en uso?): %w", direccion, err)
	}
	for _, err := range sv.Restaurar() {
		fmt.Fprintln(salida, "no se pudo restaurar una instancia:", nucleo.MensajeDe(err))
	}
	srv := &http.Server{Handler: sv, ReadHeaderTimeout: 10 * time.Second}
	if err := sv.EscribirDescriptor(); err != nil {
		oyente.Close()
		sv.Cerrar()
		return err
	}
	fmt.Fprintf(salida, "tramtai %s escuchando en %s\nWeb UI: %s (o tramtai abrir)\n", version.Texto(), sv.Base(), sv.Base())
	imprimirEstado(salida, sv.Estado())
	fmt.Fprintln(salida, "\nCtrl+C para detener.")

	ctx, parar := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer parar()
	fallo := make(chan error, 1)
	go func() { fallo <- srv.Serve(oyente) }()
	select {
	case err = <-fallo:
	case <-ctx.Done():
	case <-sv.Detenido():
	}
	fmt.Fprintln(salida, "deteniendo…")
	parar()     // un segundo Ctrl+C durante el drenaje termina el proceso: Job Objects y la recuperación cubren el resto
	sv.Drenar() // la API sigue respondiendo para que los clientes recojan resultados; los trabajos nuevos se rechazan
	cierre, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if srv.Shutdown(cierre) != nil {
		srv.Close() // los flujos SSE abiertos no terminan solos
	}
	sv.Cerrar()
	sv.BorrarDescriptor()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func esLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// identificar devuelve la raíz, el identificador y la configuración de la instancia del ámbito.
func identificar(ambito string) (raiz, id string, c *nucleo.Configuracion, err error) {
	if ambito == "global" {
		c, err = nucleo.CargarConfiguracion("")
		return "", "global", c, err
	}
	if raiz = raizProyecto(); raiz == "" {
		return "", "", nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida,
			"no hay .tramtai/config.yaml en este directorio ni en los superiores: ejecuta tramtai iniciar")
	}
	if c, err = nucleo.CargarConfiguracion(raiz); err != nil {
		return "", "", nil, err
	}
	return raiz, servicio.IDProyecto(raiz, c), c, nil
}

func mostrarTokenInstancia(cmd *cobra.Command, ambito string) error {
	_, id, c, err := identificar(ambito)
	if err != nil {
		return err
	}
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	valor, _, err := servicio.TokenInstancia(datos, id, c)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), valor)
	return nil
}

// comandoInstancia crea los comandos de la instancia global o de la del proyecto actual (secciones 5.1 y 5.2).
func comandoInstancia(ambito string) *cobra.Command {
	corto := map[string]string{"global": "Instancia global: texto y archivos sin repositorio",
		"proyecto": "Instancia del proyecto actual (.tramtai/config.yaml)"}[ambito]
	grupo := &cobra.Command{Use: ambito, Short: corto}
	iniciar := &cobra.Command{Use: "iniciar", Short: "Activa la instancia (inicia el servicio si hace falta)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			raiz, _, _, err := identificar(ambito)
			if err != nil {
				return err
			}
			c, err := asegurarServicio(cmd)
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			a, err := c.Activar(ctx, ambito, raiz)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "instancia %s activa en %s\n", a.Instancia.ID, c.Base)
			fmt.Fprintf(out, "  token:              %s (tramtai %s token)\n", a.Instancia.ArchivoToken, ambito)
			fmt.Fprintf(out, "  OpenAI base_url:    %s/openai/v1\n  Anthropic base_url: %s/anthropic\n", c.Base, c.Base)
			return nil
		}}
	detener := &cobra.Command{Use: "detener", Short: "Desactiva la instancia: vacía su cola sin afectar a las demás",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, id, _, err := identificar(ambito)
			if err != nil {
				return err
			}
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.Desactivar(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "instancia %s desactivada\n", id)
			return nil
		}}
	estado := &cobra.Command{Use: "estado", Short: "Muestra si la instancia está activa y sus trabajos",
		RunE: func(cmd *cobra.Command, _ []string) error {
			raiz, id, _, err := identificar(ambito)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			c, err := conectar()
			if err != nil {
				fmt.Fprintf(out, "instancia %s: inactiva (servicio detenido)\n", id)
				return nil
			}
			ctx, cancelar := contexto()
			defer cancelar()
			e, err := c.Estado(ctx)
			if err != nil {
				return err
			}
			for _, i := range e.Instancias {
				if i.ID == id {
					fmt.Fprintf(out, "instancia %s: activa en %s\n  %s\n  token: %s\n", id, e.Base, resumenTrabajos(i.Trabajos), i.ArchivoToken)
					return nil
				}
			}
			fmt.Fprintf(out, "instancia %s: inactiva%s (tramtai %s iniciar)\n", id, map[bool]string{true: " · " + raiz}[raiz != ""], ambito)
			return nil
		}}
	token := &cobra.Command{Use: "token", Short: "Muestra el token de la instancia para configurar clientes",
		RunE: func(cmd *cobra.Command, _ []string) error { return mostrarTokenInstancia(cmd, ambito) }}
	abrirWeb := &cobra.Command{Use: "abrir", Short: "Abre la Web UI de la instancia en el navegador (sin escribir el token)",
		RunE: func(cmd *cobra.Command, _ []string) error { return abrirWebUI(cmd, ambito) }}
	grupo.AddCommand(iniciar, detener, estado, token, abrirWeb)
	return grupo
}

// abrirWebUI activa la instancia, inicia el servicio si hace falta y abre la Web UI con un enlace de un solo uso
// (en Windows y Linux el mismo usuario entra igualmente sin él).
func abrirWebUI(cmd *cobra.Command, ambito string) error {
	raiz, _, _, err := identificar(ambito)
	if err != nil {
		return err
	}
	c, err := asegurarServicio(cmd)
	if err != nil {
		return err
	}
	ctx, cancelar := contexto()
	defer cancelar()
	a, err := c.Activar(ctx, ambito, raiz)
	if err != nil {
		return err
	}
	url, err := c.URLWeb(ctx, a.Instancia.ID)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Web UI de %s: %s\n", a.Instancia.ID, c.Base)
	return navegador.Abrir(url)
}

const plantillaProyecto = `# Configuración de Tramtai para este proyecto (versionada). Las claves de seguridad, los puertos y los
# puntos de acceso van en .tramtai/local.yaml (no versionado) o en la configuración global.
version: 1
proyecto:
  nombre: %q
predeterminados:
  agente: claude
  perfil_modelo: equilibrado
  modo_contexto: aislado
  formato_salida: markdown
`

func comandoIniciar() *cobra.Command {
	return &cobra.Command{Use: "iniciar", Short: "Prepara el directorio actual como proyecto de Tramtai (.tramtai/ con un perfil de ejemplo)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			ruta := filepath.Join(dir, ".tramtai", "config.yaml")
			if _, err := os.Stat(ruta); err == nil {
				return fmt.Errorf("%s ya existe", ruta)
			}
			if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(ruta, []byte(fmt.Sprintf(plantillaProyecto, filepath.Base(dir))), 0o644); err != nil {
				return err
			}
			ignorar := filepath.Join(dir, ".tramtai", ".gitignore")
			if err := os.WriteFile(ignorar, []byte("# configuración propia de cada máquina\nlocal.yaml\n"), 0o644); err != nil {
				return err
			}
			// Un perfil de ejemplo con su esquema (sección 22.3): tramtai ejecutar --perfil-trabajo resumen "…"
			for nombre, contenido := range map[string]string{
				filepath.Join("profiles", "resumen.yaml"):       perfilEjemplo,
				filepath.Join("schemas", "resumen.schema.json"): esquemaEjemplo,
			} {
				destino := filepath.Join(dir, ".tramtai", nombre)
				if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(destino, []byte(contenido), 0o644); err != nil {
					return err
				}
			}
			if _, err := nucleo.CargarCapa(ruta, nucleo.OrigenProyecto); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "creado %s\nsiguiente: tramtai proyecto iniciar\n", ruta)
			return nil
		}}
}

const perfilEjemplo = `nombre: resumen
modo: perfil
instrucciones: |
  Resume el contenido en tres puntos y lista los pendientes. No inventes información.
salida:
  formato: json
  validacion_estricta: true
  esquema: ../schemas/resumen.schema.json
  conservar_salida_invalida: false
`

const esquemaEjemplo = `{
  "type": "object",
  "required": ["puntos", "pendientes"],
  "properties": {
    "puntos": {"type": "array", "items": {"type": "string"}, "maxItems": 3},
    "pendientes": {"type": "array", "items": {"type": "string"}}
  },
  "additionalProperties": false
}
`
