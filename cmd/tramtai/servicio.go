package main

import (
	"bufio"
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/autoarranque"
	"tramtai/internal/carpetas"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/recursos"
	"tramtai/internal/registro"
	"tramtai/internal/servicio"
	"tramtai/internal/servidor"
	"tramtai/internal/version"
)

func contexto() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

func conectar() (*cliente.Cliente, error) {
	datos, err := carpetas.DirDatos()
	if err != nil {
		return nil, err
	}
	return cliente.Conectar(datos)
}

// asegurarServicio conecta con el servicio o lo inicia en segundo plano.
func asegurarServicio(cmd *cobra.Command) (*cliente.Cliente, error) {
	datos, err := carpetas.DirDatos()
	if err != nil {
		return nil, err
	}
	c, err := cliente.Conectar(datos)
	if err == nil {
		return c, nil
	}
	if errors.Is(err, cliente.ErrServidorOtraVersion) {
		return nil, err
	}
	ejecutable, errEjecutable := os.Executable()
	if errEjecutable != nil {
		return nil, errEjecutable
	}
	if errors.Is(err, cliente.ErrServicioAnterior) {
		fmt.Fprintln(cmd.ErrOrStderr(), idioma.T("deteniendo el servicio de la versión anterior (deja terminar las peticiones en curso)…"))
	}
	fmt.Fprintln(cmd.ErrOrStderr(), idioma.T("iniciando el servicio en segundo plano…"))
	return cliente.IniciarEnSegundoPlano(ejecutable, datos)
}

// confirmar pregunta en la terminal; cualquier respuesta que no empiece por s o y es un no.
func confirmar(cmd *cobra.Command, pregunta string) bool {
	fmt.Fprint(cmd.ErrOrStderr(), pregunta+idioma.T(" [s/N] "))
	linea, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	linea = strings.ToLower(strings.TrimSpace(linea))
	return strings.HasPrefix(linea, "s") || strings.HasPrefix(linea, "y")
}

// esperaReinicio es lo que el servicio nuevo de un reinicio espera al anterior: su plazo de drenaje más un margen
// para cerrar. Con menos, un drenaje largo dejaría al nuevo sin cerrojo y al usuario sin servicio.
func esperaReinicio() time.Duration { return cliente.PlazoDrenaje() + time.Minute }

// esperarCierreServicio espera, como mucho esperaReinicio, a que el servicio en marcha termine (drena sus trabajos).
func esperarCierreServicio() {
	datos, err := carpetas.DirDatos()
	if err != nil {
		return
	}
	for limite := time.Now().Add(esperaReinicio()); time.Now().Before(limite); {
		if _, err := cliente.Conectar(datos); err != nil && !errors.Is(err, cliente.ErrServicioAnterior) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// seguirRegistro muestra las últimas n líneas de servicio.log y, con seguir, lo que se añada hasta Ctrl+C.
func seguirRegistro(cmd *cobra.Command, n int, seguir bool) error {
	datos, err := carpetas.DirDatos()
	if err != nil {
		return err
	}
	ruta := filepath.Join(datos, "servicio.log")
	f, err := os.Open(ruta)
	if os.IsNotExist(err) {
		return fmt.Errorf(idioma.T("todavía no hay registro en %s: se crea cuando el servicio arranca en segundo plano (en primer plano se ve en su terminal)"), ruta)
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
	fmt.Fprintln(cmd.ErrOrStderr(), idioma.Tf("-- siguiendo %s (Ctrl+C para salir) --", ruta))
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
	grupo := &cobra.Command{Use: "servicio", Short: idioma.T("Servicio único: todos los espacios y sus APIs en un puerto")}
	var abrirAlIniciar, navegadorAlIniciar bool
	var archivoRegistro string
	var esperarCierre bool
	var segundoPlano, si bool
	iniciar := &cobra.Command{Use: "iniciar", Short: idioma.T("Inicia el servicio en primer plano (Ctrl+C lo detiene) o con -d en segundo plano"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !segundoPlano {
				if esperarCierre {
					esperarCierreServicio()
				}
				if abrirAlIniciar {
					go abrirCuandoResponda(cmd, navegadorAlIniciar)
				}
				return terminarServicio(cmd, iniciarServicio(cmd, archivoRegistro, esperarCierre))
			}
			c, err := asegurarServicio(cmd)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), idioma.T("servicio en marcha en %s\nWeb UI: %s (o tramtai abrir)\n"), c.BasePublica(), c.BasePublica())
			return nil
		}}
	iniciar.Flags().BoolVarP(&segundoPlano, "segundo-plano", "d", false, idioma.T("inicia el servicio desacoplado de esta terminal"))
	iniciar.Flags().BoolVar(&abrirAlIniciar, "abrir", false, idioma.T("en primer plano, abre la interfaz cuando el servicio responda (modo desarrollo)"))
	iniciar.Flags().BoolVar(&navegadorAlIniciar, "navegador", false, idioma.T("con --abrir, usa el navegador en lugar de la app de escritorio"))
	iniciar.Flags().StringVar(&archivoRegistro, "registro", "", idioma.T("escribe el registro en este archivo, con servicio.registro_mb y servicio.registro_copias"))
	_ = iniciar.Flags().MarkHidden("registro") // lo usa el arranque en segundo plano
	iniciar.Flags().BoolVar(&esperarCierre, "esperar-cierre", false, idioma.T("espera a que termine el servicio en marcha (reinicio)"))
	_ = iniciar.Flags().MarkHidden("esperar-cierre")
	detener := &cobra.Command{Use: "detener", Short: idioma.T("Detiene el servicio y todos sus espacios"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := conectar()
			if errors.Is(err, cliente.ErrServicioAnterior) { // tras actualizar, también el de la versión anterior
				if !si && !confirmar(cmd, idioma.T("¿Detener el servicio y todos sus espacios?")) {
					return nil
				}
				datos, err := carpetas.DirDatos()
				if err == nil {
					err = cliente.DetenerAnterior(datos)
				}
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), idioma.T("servicio detenido"))
				return nil
			}
			if err != nil {
				return err
			}
			if !si && !confirmar(cmd, idioma.T("¿Detener el servicio y todos sus espacios?")) {
				return nil
			}
			ctx, cancelar := contexto()
			defer cancelar()
			if err := c.Detener(ctx); err != nil {
				return err
			}
			// El servicio espera a los trabajos en curso hasta servicio.plazo_drenaje_segundos.
			plazo := cliente.PlazoDrenaje()
			inicio, avisado := time.Now(), false
			for time.Since(inicio) < plazo+15*time.Second {
				if _, err := conectar(); err != nil {
					fmt.Fprintln(cmd.OutOrStdout(), idioma.T("servicio detenido"))
					return nil
				}
				if !avisado && time.Since(inicio) > 2*time.Second {
					fmt.Fprintf(cmd.OutOrStdout(), idioma.T("esperando a que terminen las peticiones en curso (hasta %s)…\n"), plazo)
					avisado = true
				}
				time.Sleep(200 * time.Millisecond)
			}
			return errors.New(idioma.T("el servicio no confirmó la detención"))
		}}
	detener.Flags().BoolVarP(&si, "si", "s", false, idioma.T("no pedir confirmación"))
	estado := &cobra.Command{Use: "estado", Short: idioma.T("Muestra el servicio, sus espacios y sus APIs"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := conectar()
			if errors.Is(err, cliente.ErrServicioInactivo) {
				fmt.Fprintln(cmd.OutOrStdout(), idioma.T("servicio: inactivo (tramtai servicio iniciar -d)"))
				if aviso := avisoCruzado(); aviso != "" {
					fmt.Fprintln(cmd.OutOrStdout(), aviso)
				}
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
	recargar := &cobra.Command{Use: "recargar", Short: idioma.T("Aplica los cambios de las APIs sin reiniciar"),
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
	token := &cobra.Command{Use: "token", Short: idioma.T("Dice si el espacio global tiene clave (igual que tramtai global token)"),
		RunE: func(cmd *cobra.Command, _ []string) error { return mostrarTokenInstancia(cmd, "global", false) }}
	var modoArranque string
	autoarranqueCmd := &cobra.Command{Use: "autoarranque [activar|desactivar]",
		Short: idioma.T("Arranca Tramtai al iniciar sesión: la app en la bandeja o solo el servicio (--modo)"),
		Args:  cobra.MaximumNArgs(1), ValidArgs: []string{"activar", "desactivar"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if carpetas.Servidor() {
				return autoarranque.ErrEnServidor
			}
			elegido, err := autoarranque.ModoDe(modoArranque) // se valida siempre, aunque no se use
			if err != nil {
				return err
			}
			switch {
			case len(args) == 0:
			case args[0] == "activar":
				err = autoarranque.Elegir(elegido)
			case args[0] == "desactivar":
				err = autoarranque.Desactivar()
			default:
				err = errors.New(idioma.T("usa activar o desactivar"))
			}
			if err != nil {
				return err
			}
			modo, err := autoarranque.Configurado()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), map[autoarranque.Modo]string{
				autoarranque.ModoApp:      idioma.T("arranque automático: activado, la app en la bandeja"),
				autoarranque.ModoServicio: idioma.T("arranque automático: activado, solo el servicio"),
				"":                        idioma.T("arranque automático: desactivado")}[modo])
			return nil
		}}
	autoarranqueCmd.Flags().StringVar(&modoArranque, "modo", "app", idioma.T("con activar, qué arranca: app (en la bandeja) o servicio (solo el servicio)"))
	var automaticoPuerto bool
	puertoCmd := &cobra.Command{Use: "puerto [número]", Short: idioma.T("Muestra o cambia el puerto del servicio; comprueba que esté libre y se aplica al reiniciar"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := configuracion.CargarConfiguracion("", agentes.Internos())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			activo, errActivo := conectar()
			if len(args) == 0 && !cmd.Flags().Changed("automatico") {
				fmt.Fprintf(out, idioma.T("puerto configurado: %d · si está ocupado, usar otro libre: %v\n"), c.Entero("servicio.puerto"), c.Booleano("servicio.puerto_automatico"))
				if errActivo == nil {
					fmt.Fprintln(out, idioma.Tf("servicio en marcha en %s", activo.BasePublica()))
				}
				return nil
			}
			ruta, err := configuracion.RutaGlobal()
			if err != nil {
				return err
			}
			puerto, automatico := 0, automaticoPuerto // puerto 0: solo cambia el automático
			if len(args) == 1 {
				if puerto, err = strconv.Atoi(args[0]); err != nil {
					return errors.New(idioma.T("el puerto debe ser un número entre 1024 y 65535"))
				}
				if !cmd.Flags().Changed("automatico") {
					automatico = c.Booleano("servicio.puerto_automatico")
				}
			}
			switch {
			case errActivo == nil: // el servicio en marcha escribe config.yaml: dos procesos editándolo perderían un cambio
				ctx, cancelar := contexto()
				defer cancelar()
				err = activo.FijarPuerto(ctx, puerto, automatico)
			case len(args) == 1:
				err = servicio.FijarPuerto(ruta, c.Texto("servicio.host"), puerto, automatico)
			default:
				err = configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "servicio.puerto_automatico", automatico)
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(out, idioma.Tf("guardado en %s; se aplica al reiniciar el servicio (tramtai servicio detener y tramtai abrir, o Reiniciar en la Web UI)", ruta))
			return nil
		}}
	puertoCmd.Flags().BoolVar(&automaticoPuerto, "automatico", true, idioma.T("si el puerto está ocupado al iniciar, usar otro libre"))
	var lineasRegistro int
	var seguirRegistroVivo bool
	registroCmd := &cobra.Command{Use: "registro", Short: idioma.T("Muestra lo que procesa el servicio en segundo plano (servicio.log); -f lo sigue en vivo"),
		Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			return seguirRegistro(cmd, lineasRegistro, seguirRegistroVivo)
		}}
	registroCmd.Flags().IntVarP(&lineasRegistro, "lineas", "n", 30, idioma.T("líneas finales que se muestran"))
	registroCmd.Flags().BoolVarP(&seguirRegistroVivo, "seguir", "f", false, idioma.T("sigue mostrando las líneas nuevas hasta Ctrl+C"))
	grupo.AddCommand(iniciar, detener, estado, recargar, token, autoarranqueCmd, registroCmd, puertoCmd)
	return grupo
}

// iniciarServicio arranca el servicio en primer plano con las instancias que estaban activas. Con archivo, el
// registro va a ese archivo con rotación (segundo plano); sin él, a la terminal. En un reinicio (esperarCierre) espera
// a que el servicio anterior suelte el cerrojo de la carpeta de datos.
func iniciarServicio(cmd *cobra.Command, archivo string, esperarCierre bool) (err error) {
	if servicioWindows, err := ejecutarComoServicio(cmd, archivo); servicioWindows {
		return err
	}
	if archivo != "" { // sin consola: el error que lo para queda en el registro, donde se mira
		defer func() { anotarError(archivo, err) }()
	}
	// Después del SCM: el servicio de Windows (que vuelve a pasar por aquí desde su manejador) sigue arrancando; bajo
	// systemd el proceso es la cuenta tramtai, no root.
	if !comoServicioWindows() {
		if err := ejecucionLocalPermitida(); err != nil {
			return err
		}
	}
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
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
	red, err := entradaServidor(c)
	if err != nil {
		return err
	}
	red = conTOTP(red)
	if err := servidor.ComprobarArranque(red); err != nil {
		return err
	}
	avisosDeRed(salida, red, true)
	datos, err := dirDatos()
	if err != nil {
		return err
	}
	activo, err := cliente.Conectar(datos)
	if err == nil {
		return fmt.Errorf(idioma.T("el servicio ya está en marcha en %s"), activo.BasePublica())
	}
	if errors.Is(err, cliente.ErrServidorOtraVersion) { // el servicio del sistema no se detiene ni se pisa desde aquí
		return err
	}
	if errors.Is(err, cliente.ErrServicioAnterior) { // no toma el cerrojo: se detiene antes de abrir sus datos
		fmt.Fprintln(salida, idioma.T("deteniendo el servicio de la versión anterior (deja terminar las peticiones en curso)…"))
		if err := cliente.DetenerAnterior(datos); err != nil {
			return err
		}
	}
	espera := time.Duration(0)
	if esperarCierre {
		espera = esperaReinicio()
	}
	soltar, err := servicio.TomarCerrojo(datos, espera)
	if err != nil {
		return err
	}
	defer soltar()
	configTLS, err := servicio.ConfigTLS(servicio.OpcionesTLS{Modo: red.TLS, Certificado: red.Certificado, Clave: red.Clave,
		Dominios: red.Dominios, Datos: datos, Correo: red.Correo, Registros: salida})
	if err != nil {
		return err
	}
	automatico := c.Booleano("servicio.puerto_automatico")
	oyente, puertoUsado, err := abrirOyente(red.TLS, host, puerto, automatico)
	if err != nil {
		return err
	}
	if configTLS != nil {
		oyente = tls.NewListener(oyente, configTLS)
	}
	if puertoUsado != puerto {
		fmt.Fprintf(salida, idioma.T("el puerto %d está ocupado: Tramtai usa el %d (para fijar otro: Ajustes o «tramtai servicio puerto <número>»)\n"), puerto, puertoUsado)
	}
	var reinicioPedido atomic.Bool
	reiniciar := reinicioDelServicio(datos, cmp.Or(archivo, filepath.Join(datos, "servicio.log")), &reinicioPedido)
	registro, limites := agentes.Registro(c)
	raizConversor, _ := documentos.RaizConversor() // ahí se prepara el conversor de archivos
	sv, err := nuevoServicio(servicio.Opciones{Datos: datos, Config: c, Registro: registro, Semaforos: planificador.NuevosSemaforos(limites).ConSuscripcion(agentes.Simultaneas(c)),
		Host: host, Puerto: puertoUsado, PuertoConfigurado: puerto, PuertoAutomatico: automatico, Reiniciar: reiniciar,
		Hosts: hostsPermitidos(host, red.Dominios), Confiables: red.Confiables, Dominios: red.Dominios, TLS: configTLS, Registros: salida, Web: recursos.Web(), Entorno: carpetas.Entorno(),
		PlazoDrenaje: time.Duration(c.Entero("servicio.plazo_drenaje_segundos")) * time.Second,
		EntradaLocal: entradaLocal(c, red), Expuesto: servidor.Expuesto(red), BuscarActualizaciones: c.Booleano("servicio.buscar_actualizaciones"),
		RaizConversor: raizConversor, PrepararConversor: c.Booleano("conversion.preparar_al_iniciar")})
	if err != nil {
		oyente.Close()
		return err
	}
	for _, err := range sv.Restaurar() {
		fmt.Fprintln(salida, idioma.T("no se pudo restaurar un espacio:"), nucleo.MensajeDe(err))
	}
	srv := servicio.NuevoServidorHTTP(sv, servicio.ContextoDeConexion)
	if err := sv.EscribirDescriptor(); err != nil {
		oyente.Close()
		sv.Cerrar()
		return err
	}
	if carpetas.Servidor() { // para el aviso de los dos entornos del Tramtai del usuario; sin él, el servicio sigue
		if err := servidor.PublicarPuerto(puertoUsado); err != nil {
			fmt.Fprintln(salida, idioma.Tf("aviso: no se pudo publicar el puerto para el Tramtai de los usuarios del equipo: %s", err))
		} else {
			defer servidor.RetirarPuerto() // al parar con normalidad, el usuario deja de ver el aviso
		}
	}
	fmt.Fprintf(salida, idioma.T("tramtai %s escuchando en %s\nWeb UI: %s (o tramtai abrir)\n"), version.Texto(), sv.BasePublica(), sv.BasePublica())
	imprimirEstado(salida, sv.Estado(context.Background()))
	if archivo == "" {
		fmt.Fprintln(salida, idioma.T("\nAquí verás una línea por cada petición (al recibirla y al terminar). Ctrl+C para detener."))
	} else {
		fmt.Fprintln(salida, idioma.T("en segundo plano: una línea por petición; se detiene con «tramtai servicio detener»"))
	}

	ctx, parar := contextoDeParada(cmd.Context())
	defer parar()
	fallo := make(chan error, 1)
	go func() { fallo <- srv.Serve(oyente) }()
	select {
	case err = <-fallo:
	case <-ctx.Done():
	case <-sv.Detenido():
	}
	fmt.Fprintln(salida, idioma.T("deteniendo…"))
	parar()     // un segundo Ctrl+C (o SIGTERM) durante el drenaje termina el proceso: Job Objects y la recuperación cubren el resto
	sv.Drenar() // la API sigue respondiendo para que los clientes recojan resultados; los trabajos nuevos se rechazan
	cierre, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if srv.Shutdown(cierre) != nil {
		srv.Close() // los flujos SSE abiertos no terminan solos
	}
	sv.Cerrar()
	sv.BorrarDescriptor()
	if reinicioPedido.Load() {
		fmt.Fprintln(salida, errReinicioPedido.Error()) // una línea normal del registro, no un error
		return errReinicioPedido
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// abrirOyente abre el puerto del servicio. Con letsencrypt el desafío TLS-ALPN-01 solo llega al 443, así que
// servicio.puerto_automatico no se aplica: un puerto ocupado es un error, no un puerto distinto.
func abrirOyente(modoTLS, host string, puerto int, automatico bool) (net.Listener, int, error) {
	if modoTLS != "letsencrypt" {
		return servicio.Escuchar(host, puerto, automatico)
	}
	oyente, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(puerto)))
	if err != nil {
		return nil, 0, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("servicio.tls.modo letsencrypt necesita el puerto %d libre (no usa servicio.puerto_automatico): %s", puerto, err.Error()))
	}
	return oyente, puerto, nil
}

// lanzar arranca un proceso desacoplado; las pruebas lo sustituyen.
var lanzar = cliente.Lanzar

// nuevoServicio crea el servicio con sus opciones; las pruebas lo sustituyen para ver qué recibe.
var nuevoServicio = servicio.Nuevo

// codigoReinicio es el código de salida del servicio del sistema cuando la Web UI pide reiniciarlo: distinto de 0 para
// que systemd (Restart=on-failure) o la recuperación de Windows (también con salidas distintas de 0) lo vuelvan a
// arrancar, y distinto de 1, el de un fallo.
const codigoReinicio = 3

// errReinicioPedido es lo que devuelve el servicio del sistema tras drenar por un «Reiniciar» de la Web UI.
var errReinicioPedido = errors.New(idioma.T("reinicio pedido desde la Web UI: el sistema vuelve a arrancar el servicio"))

// anotarError deja en el registro el error que para el servicio (el del sistema o el de segundo plano, sin consola):
// también el de antes de abrir el registro rotativo, como una configuración inválida. El reinicio pedido ya lo anotó.
func anotarError(archivo string, err error) {
	if err == nil || errors.Is(err, errReinicioPedido) {
		return
	}
	if f, e := os.OpenFile(archivo, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600); e == nil {
		fmt.Fprintf(f, "%s %v\n", time.Now().Format(time.RFC3339), err)
		f.Close()
	}
}

// terminarServicio deja pasar el error de iniciarServicio, pero un reinicio pedido no es un fallo: cobra no lo imprime
// como «Error: …» (el registro ya lo dice en una línea normal) y el proceso sale con codigoReinicio. Si no arranca por
// su configuración en el servidor, sale con servidor.CodigoConfiguracion, que el sistema no reinicia (en el escritorio,
// con 1, como siempre).
func terminarServicio(cmd *cobra.Command, err error) error {
	if errors.Is(err, errReinicioPedido) {
		cmd.SilenceErrors = true
	}
	if carpetas.Servidor() && configuracionMala(err) {
		return errConfiguracionServicio{err}
	}
	return err
}

// configuracionMala dice si el servicio no arrancó por su configuración y volver a arrancarlo fallaría igual: no un
// certificado que aún no está (servidor.Reintentable), que puede llegar con un montaje.
func configuracionMala(err error) bool {
	return nucleo.CodigoDe(err) == nucleo.CodigoConfiguracionInvalida && !servidor.Reintentable(err)
}

// errConfiguracionServicio es el error del servicio que no arranca por su configuración (terminarServicio).
type errConfiguracionServicio struct{ error }

func (e errConfiguracionServicio) Unwrap() error { return e.error }

// codigoSalida es el código con el que termina el proceso tras un error.
func codigoSalida(err error) int {
	if errors.Is(err, errReinicioPedido) {
		return codigoReinicio
	}
	if errors.As(err, new(errConfiguracionServicio)) {
		return servidor.CodigoConfiguracion
	}
	return 1
}

// reinicioDelServicio es el «Reiniciar» de la Web UI. Fuera del modo servidor lanza otro servicio en segundo plano que
// espera a que este termine. En el modo servidor no lanza nada fuera de systemd o del administrador de servicios: lo
// anota en pedido, y el servicio, tras drenar, sale con codigoReinicio para que el sistema lo vuelva a arrancar.
func reinicioDelServicio(datos, registro string, pedido *atomic.Bool) func() error {
	if carpetas.Servidor() {
		return func() error { pedido.Store(true); return nil }
	}
	return func() error {
		ejecutable, err := os.Executable()
		if err != nil {
			return err
		}
		return lanzar(ejecutable, datos, "servicio", "iniciar", "--esperar-cierre", "--registro", registro)
	}
}

// entradaLocal dice si el mismo usuario entra sin token (servicio.entrada_local): nunca con el servicio expuesto.
func entradaLocal(c *configuracion.Configuracion, red servidor.Entrada) bool {
	return c.Booleano("servicio.entrada_local") && !servidor.Expuesto(red)
}

// hostsPermitidos son los nombres que valen en Host y Origin: loopback, el host si no lo es y servicio.dominios.
func hostsPermitidos(host string, dominios []string) []string {
	hosts := []string{"127.0.0.1", "localhost", "::1"}
	if !servidor.Loopback(host) {
		hosts = append(hosts, strings.ToLower(host))
	}
	for _, dominio := range dominios {
		hosts = append(hosts, strings.ToLower(dominio))
	}
	return hosts
}

// entradaServidor reúne de la configuración lo que deciden las reglas de exposición (servidor.ComprobarArranque). Las
// suscripciones son las de los agentes en modo suscripción; personal, si adaptadores.<agente>.suscripcion_personal.
func entradaServidor(c *configuracion.Configuracion) (servidor.Entrada, error) {
	confiables, err := servidor.Confiables(textos(c.Lista("servicio.proxy.confiables")))
	if err != nil {
		return servidor.Entrada{}, err
	}
	e := servidor.Entrada{Servidor: carpetas.Servidor(), Host: c.Texto("servicio.host"), PermitirRemoto: c.Booleano("servicio.permitir_remoto"), Confiables: confiables,
		TLS: c.Texto("servicio.tls.modo"), Correo: c.Texto("servicio.tls.letsencrypt.correo"),
		Puerto: c.Entero("servicio.puerto"), Certificado: c.Texto("servicio.tls.certificado"), Clave: c.Texto("servicio.tls.clave")}
	for _, dominio := range textos(c.Lista("servicio.dominios")) {
		// Como llegan en Host: sin espacios, sin el punto final y en minúsculas; sin puerto.
		normal := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(dominio), "."))
		if normal == "" || strings.ContainsAny(normal, ":/ ") {
			return servidor.Entrada{}, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("servicio.dominios: %q no es un nombre de dominio (sin puerto, como tramtai.ejemplo.org)", dominio))
		}
		e.Dominios = append(e.Dominios, normal)
	}
	for _, f := range agentes.Todos() {
		if agente := f.Nombre; agentes.AdmiteSuscripcion(agente) && c.Texto("adaptadores."+agente+".autenticacion") == adaptadores.AutenticacionSuscripcion {
			e.Suscripciones = append(e.Suscripciones, servidor.Suscripcion{Agente: agente, Personal: c.Booleano("adaptadores." + agente + ".suscripcion_personal")})
		}
	}
	return e, nil
}

// conTOTP completa la entrada con si el administrador tiene TOTP (la regla 4), con el secreto del almacén del servicio
// (systemd-creds en Linux). Solo se pregunta al almacén cuando la regla puede aplicarse (servicio expuesto y con
// suscripciones), para no tocar el llavero del escritorio en cada arranque.
func conTOTP(red servidor.Entrada) servidor.Entrada {
	if servidor.Expuesto(red) && len(red.Suscripciones) > 0 {
		secreto, _ := leerClave("TRAMTAI_TOTP")
		red.TOTP = secreto != ""
	}
	return red
}

// textos pasa una lista de la configuración a textos.
func textos(lista []any) []string {
	r := make([]string, len(lista))
	for i, v := range lista {
		r[i] = fmt.Sprint(v)
	}
	return r
}

// identificar devuelve la raíz, el identificador y la configuración de la instancia del ámbito.
func identificar(ambito string) (raiz, id string, c *configuracion.Configuracion, err error) {
	if ambito == "global" {
		c, err = configuracion.CargarConfiguracion("", agentes.Internos())
		return "", "global", c, err
	}
	if raiz = raizProyecto(); raiz == "" {
		return "", "", nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida,
			idioma.T("no hay .tramtai/config.yaml en este directorio ni en los superiores: ejecuta tramtai iniciar"))
	}
	if c, err = configuracion.CargarConfiguracion(raiz, agentes.Internos()); err != nil {
		return "", "", nil, err
	}
	return raiz, idDeCarpeta(raiz, c), c, nil
}

// idDeCarpeta es el espacio de la carpeta raiz con la regla del servicio, sobre el registro de la carpeta de datos.
func idDeCarpeta(raiz string, c *configuracion.Configuracion) string {
	datos, _ := carpetas.DirDatos()
	return servicio.IDDeCarpeta(datos, raiz, c)
}
