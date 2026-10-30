package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
	"tramtai/internal/version"
)

func comandoAgentes() *cobra.Command {
	grupo := &cobra.Command{Use: "agentes", Aliases: []string{"agents"}, Short: idioma.T("Diagnostica cada agente: versión, autenticación y términos (sin consumir cuota)"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ejecucionLocalPermitida(); err != nil { // el diagnóstico ejecuta cada CLI (--version)
				return err
			}
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			registro, _ := agentes.Registro(c)
			nombres := make([]string, 0, len(registro))
			for nombre := range registro {
				nombres = append(nombres, nombre)
			}
			sort.Strings(nombres)
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "tramtai", version.Texto())
			for _, nombre := range nombres {
				detectable, ok := registro[nombre].(adaptadores.Detectable)
				if !ok {
					continue
				}
				ctx, cancelar := context.WithTimeout(cmd.Context(), 15*time.Second)
				e := detectable.Detectar(ctx)
				cancelar()
				fmt.Fprintf(out, idioma.T("\n%s\n  versión:       %s\n  ejecutable:    %s\n  autenticación: %s\n  términos:      %s\n"),
					e.Agente, vacio(e.Version), vacio(e.Ruta), e.Autenticacion, e.Terminos)
				if e.Problema != "" {
					fmt.Fprintf(out, idioma.T("  problema:      %s\n"), e.Problema)
				}
				if c.Booleano("adaptadores." + nombre + ".desconectado") {
					fmt.Fprintln(out, idioma.T("  desconectado:  sí (tramtai agentes conectar)"))
				}
			}
			return nil
		}}
	// conectar y desconectar valen en todos los espacios y al momento (el servicio tiene que estar en marcha).
	conexion := func(conectado bool) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.ConectarAgente(ctx, args[0], conectado); err != nil {
				return err
			}
			if conectado {
				fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s conectado\n"), args[0])
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), idioma.T("%s desconectado: no se usa ni como respaldo hasta que lo conectes\n"), args[0])
			}
			return nil
		}
	}
	grupo.AddCommand(
		&cobra.Command{Use: "desconectar <agente>", Short: idioma.T("Desconecta un agente sin desinstalar su CLI"), Args: cobra.ExactArgs(1), RunE: conexion(false)},
		&cobra.Command{Use: "conectar <agente>", Short: idioma.T("Vuelve a conectar un agente desconectado"), Args: cobra.ExactArgs(1), RunE: conexion(true)},
		comandoEntrar(), comandoSuscripcion(),
	)
	return grupo
}

func comandoEjecutar() *cobra.Command {
	var agente, modelo, perfil, sistema, esquema, formato string
	var archivos []string
	var tiempo int
	var directo bool
	var perfilTrabajo, contexto string
	cmd := &cobra.Command{Use: "ejecutar [texto]", Short: idioma.T("Envía una petición a un agente y muestra la respuesta en vivo"),
		Long: idioma.T("Envía una petición y muestra la respuesta en vivo. Si el servicio está en marcha, la petición va al espacio del\nproyecto actual (o al global) y se ve también en la terminal interactiva y en la Web UI; si no, se ejecuta\ndirectamente. Sin texto, se lee de la entrada estándar. Ctrl+C cancela la petición."),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			texto := strings.Join(args, " ")
			if texto == "" {
				datos, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				texto = strings.TrimSpace(string(datos))
			}
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			s := nucleo.SolicitudEjecucion{Agente: agente, Modelo: modelo, PerfilModelo: perfil, Perfil: perfilTrabajo, ModoContexto: contexto, Instrucciones: sistema,
				Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
			s.Ejecucion.TiempoLimiteSegundos = tiempo
			s.Salida.Formato = formato
			if esquema != "" {
				if esquema, err = c.RutaPermitida(esquema); err != nil {
					return err
				}
				if s.Salida.Esquema, err = os.ReadFile(esquema); err != nil {
					return err
				}
			}
			for i, ruta := range archivos {
				if archivos[i], err = c.RutaPermitida(ruta); err != nil {
					return err
				}
			}
			if err := documentos.AdmitirArchivos(archivos); err != nil {
				return err
			}
			if !directo {
				if cli, err := conectar(); err == nil {
					return ejecutarEnServicio(cmd, cli, s, archivos)
				}
			}
			if s, err = c.AplicarPerfil(s); err != nil {
				return err
			}
			return ejecutar(cmd, c, c.AplicarPredeterminados(s), archivos)
		}}
	f := cmd.Flags()
	f.Bool("global", false, idioma.T("usa el espacio global aunque estés dentro de un proyecto"))
	f.StringVarP(&agente, "agente", "a", "", idioma.Tf("agente: %s (predeterminados.agente)", enumerar(idsAgentes(), idioma.T("o"))))
	f.StringVarP(&modelo, "modelo", "m", "", idioma.T("identificador o alias del modelo, por ejemplo haiku"))
	f.StringVarP(&perfil, "perfil", "p", "", idioma.T("perfil de modelo: rapido, equilibrado, profundo (modelos.perfiles)"))
	f.StringVarP(&sistema, "sistema", "s", "", idioma.T("instrucciones de sistema"))
	f.StringVar(&contexto, "contexto", "", idioma.T("aislado (predeterminado) o consciente_proyecto: solo en proyectos de proyectos_confiables"))
	f.StringVar(&perfilTrabajo, "perfil-trabajo", "", idioma.T("perfil de ejecución: profiles/<nombre>.yaml del proyecto o global (instrucciones, formato y esquema)"))
	f.IntVarP(&tiempo, "tiempo", "t", 0, idioma.T("tiempo límite en segundos (300 por defecto)"))
	f.StringArrayVarP(&archivos, "archivo", "f", nil, idioma.T("archivo de entrada, repetible: PDF, Word, PowerPoint, Excel, HTML o texto (txt, md, json, csv…)"))
	f.StringVarP(&esquema, "esquema", "e", "", idioma.T("archivo JSON Schema que debe cumplir la salida (implica formato json)"))
	f.StringVar(&formato, "formato", "", idioma.T("formato de salida: texto, markdown o json"))
	f.BoolVar(&directo, "directo", false, idioma.T("ejecuta sin el servicio aunque esté en marcha (base cli.db propia)"))
	return cmd
}

func comandoConvertir() *cobra.Command {
	return &cobra.Command{Use: "convertir <archivo>...", Short: idioma.T("Muestra el texto que recibiría el agente para esos archivos (sin ejecutar ningún agente)"),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := conversionLocalPermitida(args); err != nil {
				return err
			}
			c, err := cargarConfiguracion()
			if err != nil {
				return err
			}
			entradas := make([]documentos.Entrada, len(args))
			for i, ruta := range args {
				real, err := c.RutaPermitida(ruta)
				if err != nil {
					return err
				}
				entradas[i] = documentos.Entrada{Ruta: real, Nombre: filepath.Base(ruta)}
			}
			docs, err := documentos.Desde(c).Convertir(cmd.Context(), entradas)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), docs.Texto)
			return nil
		}}
}

// archivosSinConversor falla, antes de subir nada, si algún archivo necesita el conversor y no hay ninguno listo.
func archivosSinConversor(conversor *documentos.Conversor, archivos []string) error {
	for _, ruta := range archivos {
		if documentos.NecesitaConversor(ruta) {
			if _, ok := documentos.Localizar(conversor.Comando, conversor.Raiz); !ok {
				return documentos.ErrSinConversor()
			}
			return nil
		}
	}
	return nil
}

// conversionLocalPermitida falla si este proceso, administrador en el modo servidor, tendría que convertir algún
// archivo él mismo: el conversor de la raíz del servidor lo puede cambiar la cuenta del servicio, y ejecutarlo así
// le daría los permisos del administrador. Esa conversión la hace el servicio.
func conversionLocalPermitida(archivos []string) error {
	if !carpetas.Servidor() || !esAdministrador() || !slices.ContainsFunc(archivos, documentos.NecesitaConversor) {
		return nil
	}
	return nucleo.NuevoError(nucleo.CodigoConversorNoDisponible, idioma.T("en el servidor, la conversión la hace el servicio: inícialo y repite"))
}

// ejecucionLocalPermitida falla si este proceso, administrador en el modo servidor, iba a ejecutar él mismo los CLI de
// los agentes o el servicio: comandos_agentes sale de una configuración que puede escribir la cuenta del servicio, y
// ejecutarlos así les daría los permisos del administrador (R36). Eso lo hace el servicio, con su cuenta.
func ejecucionLocalPermitida() error {
	if !carpetas.Servidor() || !esAdministrador() {
		return nil
	}
	return errors.New(idioma.T("en el servidor, los agentes y el servicio solo se ejecutan con la cuenta del servicio, no como administrador: usa el servicio (si no está en marcha, inícialo con tramtai-servidor reiniciar o, en Windows, con el administrador de servicios)"))
}

func ejecutar(cmd *cobra.Command, c *configuracion.Configuracion, s nucleo.SolicitudEjecucion, archivos []string) error {
	if err := ejecucionLocalPermitida(); err != nil {
		return err
	}
	if err := conversionLocalPermitida(archivos); err != nil {
		return err
	}
	datos, err := carpetas.DirDatos()
	if err != nil {
		return err
	}
	alm, err := persistencia.Abrir(filepath.Join(datos, "cli.db"))
	if err != nil {
		return err
	}
	defer alm.Cerrar()

	cargas, err := documentos.AbrirCargas(filepath.Join(datos, "cargas", "cli"),
		time.Duration(c.Entero("retencion.horas_carga_huerfana"))*time.Hour, int64(c.Limite("conversion.maximo_carga_mb"))<<20,
		int64(c.Limite("conversion.cuota_cargas_mb"))<<20)
	if err != nil {
		return err
	}
	// cli.db guarda una instancia por ejecución: al terminar se aplica la retención a todas (antes de cerrar la base).
	defer func() {
		if ids, err := alm.Instancias(context.Background()); err == nil {
			if err := servicio.PurgarBase(context.Background(), alm, cargas, c, ids...); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), idioma.Tf("no se pudo aplicar la retención: %s", nucleo.MensajeDe(err)))
			}
		}
	}()
	conversor := documentos.Desde(c)
	if err := archivosSinConversor(conversor, archivos); err != nil {
		return err
	}
	for _, ruta := range archivos {
		archivo, err := os.Open(ruta)
		if err != nil {
			return err
		}
		carga, err := cargas.Guardar(filepath.Base(ruta), "", archivo)
		archivo.Close()
		if err != nil {
			return err
		}
		s.Cargas = append(s.Cargas, carga.ID)
	}

	registro, limites := agentes.Registro(c)
	// shortcut: cada ejecución directa usa su propia instancia para no relanzar trabajos de otra ejecución
	// interrumpida; el servicio tiene instancias persistentes con su recuperación.
	opc := planificador.OpcionesDesde(c, "cli-"+nucleo.NuevoID()[len("trabajo_"):])
	opc.Preparador = documentos.Preparador{Cargas: cargas, Conversor: conversor}
	if dir, err := os.Getwd(); err == nil {
		if raiz, ok := configuracion.BuscarProyecto(dir); ok && c.ProyectoConfiable(raiz) {
			opc.RaizConfiable = raiz
		}
	}
	p := planificador.Nuevo(alm, registro, planificador.NuevosSemaforos(limites), opc)
	if err := p.Iniciar(cmd.Context()); err != nil {
		return err
	}
	defer p.Detener()

	t, _, err := p.Enviar(nucleo.ConOrigenPeticion(cmd.Context(), nucleo.DesdeCLI), s, "")
	if err != nil {
		return err
	}
	errOut, out := cmd.ErrOrStderr(), cmd.OutOrStdout()
	fmt.Fprintf(errOut, idioma.T("tramtai %s · %s · agente %s · modelo %s\n\n"), version.Texto(), t.ID, s.Agente, vacio(t.ModeloEnviado))

	// Ctrl+C cancela el trabajo; un segundo Ctrl+C sale sin esperar a que el agente lo confirme.
	senales := make(chan os.Signal, 2)
	signal.Notify(senales, os.Interrupt)
	defer signal.Stop(senales)
	inicio, ultimo, cancelado := time.Now(), 0, false
	var impreso strings.Builder
	for {
		select {
		case <-senales:
			if cancelado {
				fmt.Fprintln(errOut, idioma.T("se sale sin esperar a que termine la cancelación"))
				// Se detiene el planificador (termina el árbol del agente, que en Unix tiene su propio grupo y no recibe
				// el Ctrl+C) con un tope, para salir aunque no lo confirme.
				detenido := make(chan struct{})
				go func() { p.Detener(); close(detenido) }()
				select {
				case <-detenido:
				case <-time.After(5 * time.Second):
				}
				os.Exit(130)
			}
			cancelado = true
			fmt.Fprintln(errOut, idioma.T("\ncancelando… (Ctrl+C otra vez para salir sin esperar)"))
			if _, err := p.Cancelar(context.Background(), t.ID); err != nil {
				return err
			}
		default:
		}
		eventos, err := alm.Eventos(context.Background(), t.ID, ultimo)
		if err != nil {
			return err
		}
		terminado := false
		for _, e := range eventos {
			ultimo = e.Numero
			switch {
			case e.Tipo == "texto":
				fmt.Fprint(out, e.Datos)
				impreso.WriteString(e.Datos)
			case e.Tipo == "estado" && e.Datos == string(nucleo.PreparandoEntrada):
				fmt.Fprintln(errOut, idioma.T("convirtiendo archivos…"))
			case e.Tipo == "estado" && nucleo.Estado(e.Datos).Terminal():
				terminado = true
			}
		}
		if terminado {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	final, err := alm.Obtener(context.Background(), t.ID)
	if err != nil {
		return err
	}
	return imprimirFinal(cmd, final, impreso.String(), inicio)
}

func vacio(texto string) string {
	if texto == "" {
		return idioma.T("Sin dato")
	}
	return texto
}

// idsAgentes son los identificadores del catálogo (claude, codex…), en su orden.
func idsAgentes() []string {
	var ids []string
	for _, f := range agentes.Todos() {
		ids = append(ids, f.Nombre)
	}
	return ids
}
