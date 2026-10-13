package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/autoarranque"
	"tramtai/internal/cliente"
	"tramtai/internal/navegador"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/recursos"
	"tramtai/internal/registro"
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

// esperarCierreServicio espera, como mucho dos minutos, a que el servicio en marcha termine (drena sus trabajos).
func esperarCierreServicio() {
	datos, err := dirDatos()
	if err != nil {
		return
	}
	for range 600 {
		if _, err := cliente.Conectar(datos); err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// abrirCuandoResponda espera a que el servicio en primer plano acepte conexiones y abre la interfaz global.
func abrirCuandoResponda(cmd *cobra.Command, enNavegador bool) {
	datos, err := dirDatos()
	if err != nil {
		return
	}
	for range 100 {
		if c, err := cliente.Conectar(datos); err == nil {
			if err := abrirInterfaz(cmd, c, "global", "", enNavegador); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "no se pudo abrir la interfaz:", err)
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// seguirRegistro muestra las últimas n líneas de servicio.log y, con seguir, lo que se añada hasta Ctrl+C.
func seguirRegistro(cmd *cobra.Command, n int, seguir bool) error {
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	ruta := filepath.Join(datos, "servicio.log")
	f, err := os.Open(ruta)
	if os.IsNotExist(err) {
		return fmt.Errorf("todavía no hay registro en %s: se crea cuando el servicio arranca en segundo plano (en primer plano se ve en su terminal)", ruta)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// Solo el final del archivo: basta para n líneas y evita leer un registro largo entero.
	if _, err := f.Seek(max(0, info.Size()-256<<10), io.SeekStart); err != nil {
		return err
	}
	final, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	lineas := strings.Split(strings.TrimRight(string(final), "\r\n"), "\n")
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, strings.Join(lineas[max(0, len(lineas)-n):], "\n"))
	if !seguir {
		return nil
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "-- siguiendo "+ruta+" (Ctrl+C para salir) --")
	ctx, parar := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer parar()
	for {
		if _, err := io.Copy(out, f); err != nil {
			return err
		}
		// Tras una rotación el archivo es otro (o se vació): se sigue desde el principio del nuevo.
		pos, _ := f.Seek(0, io.SeekCurrent)
		if actual, err := os.Stat(ruta); err == nil {
			if abierto, err := f.Stat(); err == nil && (!os.SameFile(actual, abierto) || actual.Size() < pos) {
				if nuevo, err := os.Open(ruta); err == nil {
					f.Close()
					f = nuevo
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func comandoServicio() *cobra.Command {
	grupo := &cobra.Command{Use: "servicio", Short: "Servicio único: todas las instancias y puntos de acceso en un puerto"}
	var abrirAlIniciar, navegadorAlIniciar bool
	var archivoRegistro string
	var esperarCierre bool
	var segundoPlano, si bool
	iniciar := &cobra.Command{Use: "iniciar", Short: "Inicia el servicio en primer plano (Ctrl+C lo detiene) o con -d en segundo plano",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !segundoPlano {
				if esperarCierre {
					esperarCierreServicio()
				}
				if abrirAlIniciar {
					go abrirCuandoResponda(cmd, navegadorAlIniciar)
				}
				return iniciarServicio(cmd, archivoRegistro)
			}
			c, err := asegurarServicio(cmd)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "servicio en marcha en %s\nWeb UI: %s (o tramtai abrir)\n", c.Base, c.Base)
			return nil
		}}
	iniciar.Flags().BoolVarP(&segundoPlano, "segundo-plano", "d", false, "inicia el servicio desacoplado de esta terminal")
	iniciar.Flags().BoolVar(&abrirAlIniciar, "abrir", false, "en primer plano, abre la interfaz cuando el servicio responda (modo desarrollo)")
	iniciar.Flags().BoolVar(&navegadorAlIniciar, "navegador", false, "con --abrir, usa el navegador en lugar de la app de escritorio")
	iniciar.Flags().StringVar(&archivoRegistro, "registro", "", "escribe el registro en este archivo, con servicio.registro_mb y servicio.registro_copias")
	_ = iniciar.Flags().MarkHidden("registro") // lo usa el arranque en segundo plano
	iniciar.Flags().BoolVar(&esperarCierre, "esperar-cierre", false, "espera a que termine el servicio en marcha (reinicio)")
	_ = iniciar.Flags().MarkHidden("esperar-cierre")
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
	autoarranqueCmd := &cobra.Command{Use: "autoarranque [activar|desactivar]",
		Short: "Arranca Tramtai (la app de escritorio, oculta en la bandeja) al iniciar sesión en Windows",
		Args:  cobra.MaximumNArgs(1), ValidArgs: []string{"activar", "desactivar"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			switch {
			case len(args) == 0:
			case args[0] == "activar":
				var escritorio string
				if escritorio, err = autoarranque.Escritorio(); err == nil {
					err = autoarranque.Activar(escritorio)
				}
			case args[0] == "desactivar":
				err = autoarranque.Desactivar()
			default:
				err = fmt.Errorf("usa activar o desactivar")
			}
			if err != nil {
				return err
			}
			activo, err := autoarranque.Activo()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "arranque automático: "+map[bool]string{true: "activado", false: "desactivado"}[activo])
			return nil
		}}
	var automaticoPuerto bool
	puertoCmd := &cobra.Command{Use: "puerto [número]", Short: "Muestra o cambia el puerto del servicio; comprueba que esté libre y se aplica al reiniciar",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := nucleo.CargarConfiguracion("")
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			activo, errActivo := conectar()
			if len(args) == 0 && !cmd.Flags().Changed("automatico") {
				fmt.Fprintf(out, "puerto configurado: %d · si está ocupado, usar otro libre: %v\n", c.Entero("servicio.puerto"), c.Booleano("servicio.puerto_automatico"))
				if errActivo == nil {
					fmt.Fprintln(out, "servicio en marcha en "+activo.Base)
				}
				return nil
			}
			ruta, err := nucleo.RutaGlobal()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				puerto, err := strconv.Atoi(args[0])
				if err != nil || puerto < 1024 || puerto > 65535 {
					return fmt.Errorf("el puerto debe ser un número entre 1024 y 65535")
				}
				propio := errActivo == nil && strings.HasSuffix(activo.Base, ":"+args[0]) // el que ya usa Tramtai cuenta como libre
				if !propio && !servicio.PuertoLibre(c.Texto("servicio.host"), puerto) {
					return fmt.Errorf("el puerto %d está ocupado por otro programa: elige otro", puerto)
				}
				if err := servicio.FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto", puerto); err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("automatico") {
				if err := servicio.FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto_automatico", automaticoPuerto); err != nil {
					return err
				}
			}
			fmt.Fprintln(out, "guardado en "+ruta+"; se aplica al reiniciar el servicio (tramtai servicio detener y tramtai abrir, o Reiniciar en la Web UI)")
			return nil
		}}
	puertoCmd.Flags().BoolVar(&automaticoPuerto, "automatico", true, "si el puerto está ocupado al iniciar, usar otro libre")
	var lineasRegistro int
	var seguirRegistroVivo bool
	registroCmd := &cobra.Command{Use: "registro", Short: "Muestra lo que procesa el servicio en segundo plano (servicio.log); -f lo sigue en vivo",
		Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			return seguirRegistro(cmd, lineasRegistro, seguirRegistroVivo)
		}}
	registroCmd.Flags().IntVarP(&lineasRegistro, "lineas", "n", 30, "líneas finales que se muestran")
	registroCmd.Flags().BoolVarP(&seguirRegistroVivo, "seguir", "f", false, "sigue mostrando las líneas nuevas hasta Ctrl+C")
	grupo.AddCommand(iniciar, detener, estado, recargar, token, autoarranqueCmd, registroCmd, puertoCmd)
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
		nombre := p.Nombre
		if p.Grupo != "" {
			nombre += " (" + p.Grupo + ")"
		}
		fmt.Fprintf(out, "  %-18s %-40s %-24s %s\n", nombre, valorO(p.URL, "—"), "["+strings.Join(p.Contratos, ",")+"]", estado)
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

// iniciarServicio arranca el servicio en primer plano con las instancias que estaban activas. Con archivo, el
// registro va a ese archivo con rotación (segundo plano); sin él, a la terminal.
func iniciarServicio(cmd *cobra.Command, archivo string) error {
	c, err := nucleo.CargarConfiguracion("")
	if err != nil {
		return err
	}
	salida := cmd.ErrOrStderr()
	if archivo != "" {
		rotativo, err := registro.Abrir(archivo, int64(c.Entero("servicio.registro_mb"))<<20, c.Entero("servicio.registro_copias"))
		if err != nil {
			return err
		}
		defer rotativo.Close()
		salida = rotativo
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
	automatico := c.Booleano("servicio.puerto_automatico")
	oyente, puertoUsado, err := servicio.Escuchar(host, puerto, automatico)
	if err != nil {
		return err
	}
	if puertoUsado != puerto {
		fmt.Fprintf(salida, "el puerto %d está ocupado: Tramtai usa el %d (para fijar otro: Ajustes o «tramtai servicio puerto <número>»)\n", puerto, puertoUsado)
	}
	// El reinicio lanza otro servicio en segundo plano que espera a que este termine.
	registroReinicio := cmp.Or(archivo, filepath.Join(datos, "servicio.log"))
	reiniciar := func() error {
		ejecutable, err := os.Executable()
		if err != nil {
			return err
		}
		return cliente.Lanzar(ejecutable, datos, "servicio", "iniciar", "--esperar-cierre", "--registro", registroReinicio)
	}
	agentes, limites := adaptadores.Registro(c)
	sv, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: agentes, Semaforos: planificador.NuevosSemaforos(limites),
		Host: host, Puerto: puertoUsado, PuertoConfigurado: puerto, PuertoAutomatico: automatico, Reiniciar: reiniciar,
		Hosts: hosts, Registros: salida, Web: recursos.Web(),
		PlazoDrenaje: time.Duration(c.Entero("servicio.plazo_drenaje_segundos")) * time.Second,
		EntradaLocal: c.Booleano("servicio.entrada_local")})
	if err != nil {
		oyente.Close()
		return err
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
	if archivo == "" {
		fmt.Fprintln(salida, "\nAquí verás una línea por cada petición (al recibirla y al terminar). Ctrl+C para detener.")
	} else {
		fmt.Fprintln(salida, "en segundo plano: una línea por petición; se detiene con «tramtai servicio detener»")
	}

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
	var enNavegador bool
	abrirWeb := &cobra.Command{Use: "abrir", Short: "Abre la instancia en la app de escritorio o, con --navegador, en el navegador",
		RunE: func(cmd *cobra.Command, _ []string) error { return abrirWebUI(cmd, ambito, enNavegador) }}
	abrirWeb.Flags().BoolVar(&enNavegador, "navegador", false, "abre la Web UI en el navegador en lugar de la app de escritorio")
	grupo.AddCommand(iniciar, detener, estado, token, abrirWeb)
	return grupo
}

// abrirWebUI inicia el servicio si hace falta, deja claro que sigue en marcha en segundo plano y abre la
// interfaz: la app de escritorio si está junto a tramtai, o la Web UI en el navegador con un enlace de un solo uso.
// (en Windows y Linux el mismo usuario entra igualmente sin él).
func abrirWebUI(cmd *cobra.Command, ambito string, enNavegador bool) error {
	raiz, _, _, err := identificar(ambito)
	if err != nil {
		return err
	}
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	_, errPrevio := cliente.Conectar(datos)
	c, err := asegurarServicio(cmd)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if errPrevio == nil {
		fmt.Fprintf(out, "Tramtai ya estaba en marcha en %s.\n", c.Base)
	} else {
		fmt.Fprintf(out, "Tramtai se inició en segundo plano en %s.\n", c.Base)
	}
	fmt.Fprintln(out, "Sigue funcionando aunque cierres esta terminal o la ventana.")
	if err := abrirInterfaz(cmd, c, ambito, raiz, enNavegador); err != nil {
		return err
	}
	fmt.Fprint(out, `
  Ver lo que procesa:  tramtai servicio registro -f
  Ver su estado:       tramtai servicio estado
  Detenerlo:           tramtai servicio detener
`)
	return nil
}

// abrirInterfaz abre la app de escritorio (si está junto a tramtai y no se pidió el navegador) o la Web UI en el
// navegador, ya dentro y en la instancia del ámbito.
func abrirInterfaz(cmd *cobra.Command, c *cliente.Cliente, ambito, raiz string, enNavegador bool) error {
	out := cmd.OutOrStdout()
	if escritorio, err := autoarranque.Escritorio(); err == nil && !enNavegador {
		args := []string{}
		if ambito == "proyecto" {
			args = append(args, "-proyecto", raiz)
		}
		app := exec.Command(escritorio, args...)
		if err := app.Start(); err == nil {
			_ = app.Process.Release()
			fmt.Fprintln(out, "Abriendo la aplicación de escritorio… (con --navegador se abre en el navegador)")
			return nil
		}
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
	fmt.Fprintln(out, "Abriendo la Web UI en el navegador…")
	return navegador.Abrir(url)
}

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
			if err := servicio.PrepararProyecto(dir); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "creado %s\nsiguiente: tramtai proyecto iniciar\n", ruta)
			return nil
		}}
}
