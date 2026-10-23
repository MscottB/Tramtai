// Comando tramtai-escritorio: aplicación de escritorio de Tramtai. Es un cliente del servicio: lo
// detecta o lo inicia, abre la Web UI ya autenticada en una ventana propia y queda en la bandeja del sistema,
// donde muestra si el servicio está en marcha y, con el clic derecho, tus APIs y las acciones rápidas.
// Cerrar la ventana la deja en la bandeja; Salir detiene el servicio (con un aviso que se puede no volver a ver).
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"tramtai/internal/autoarranque"
	"tramtai/internal/carpetas"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// icono es el de la ventana y la bandeja (go run ./scripts/icono lo regenera).
//
//go:embed icono/tramtai.png
var icono []byte

// intervaloVigilancia es cada cuánto se repasa el estado del servicio y de las APIs.
const intervaloVigilancia = 3 * time.Second

// iconosMenu son los iconos de las acciones de la bandeja: máscaras de 48 px del icono «outlined» de Material
// que usa la Web UI, que se tiñen y reducen al tamaño del menú.
//
//go:embed icono/menu/*.png
var iconosMenu embed.FS

// Colores de los círculos del menú: en marcha o activa (verde), detenido o desactivada (rojo).
var (
	colorVerde = color.RGBA{0x22, 0xc5, 0x5e, 0xff}
	colorRojo  = color.RGBA{0xef, 0x44, 0x44, 0xff}
)

var textoDetener = idioma.T("Se detienen todos los espacios y las aplicaciones conectadas dejarán de funcionar. Tramtai sigue en la bandeja para volver a iniciarlo. ¿Continuar?")
var textoSalir = idioma.T("Al salir se detiene el servicio: las aplicaciones conectadas a tus APIs dejarán de funcionar hasta que vuelvas a abrir Tramtai. ¿Continuar?")

func main() {
	proyecto := flag.String("proyecto", "", idioma.T("raíz de un proyecto con .tramtai/config.yaml (por defecto, el espacio global)"))
	oculto := flag.Bool("oculto", false, idioma.T("arranca solo en la bandeja, sin mostrar la ventana (arranque automático)"))
	actualizado := flag.Bool("actualizado", false, idioma.T("recién actualizada: muestra la ventana por delante de las demás"))
	flag.Parse()

	var ventana *application.WebviewWindow
	var recibir func(mensaje, origen string)
	// La raíz se resuelve antes de crear la app: Wails cierra la app con su propio aviso si la carpeta del navegador no
	// vale, y así el error de la portable (carpeta de solo lectura o temporal) se ve en la ventana, como los demás.
	raizUsuario, errRaiz := carpetas.RaizUsuario()
	datosNavegador := "" // modo normal: el predeterminado de Wails
	switch {
	case errRaiz != nil:
		// Una carpeta fija: la app solo enseña el error, y así no deja un perfil nuevo en %TEMP% en cada arranque.
		datosNavegador = filepath.Join(os.TempDir(), "tramtai-navegador-error")
	case carpetas.Portable():
		datosNavegador = filepath.Join(raizUsuario, "webview")
	}
	propio, _ := os.Executable()
	nombre := "Tramtai"
	if carpetas.Portable() {
		nombre = idioma.T("Tramtai portable")
	}
	app := application.New(application.Options{
		Name:        "Tramtai",
		Description: idioma.T("Tus agentes de IA como una API local"),
		Icon:        icono,
		Windows:     application.WindowsOptions{WebviewUserDataPath: datosNavegador},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: uniqueID(carpetaCopia(propio, raizUsuario), carpetas.Portable()),
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if ventana != nil {
					ventana.Show()
					ventana.Focus()
				}
			},
		},
		// Respuestas de los avisos que la Web UI muestra por encargo de la bandeja (postMessage de WebView2).
		RawMessageHandler: func(_ application.Window, mensaje string, origen *application.OriginInfo) {
			if recibir != nil && origen != nil {
				recibir(mensaje, origen.Origin)
			}
		},
	})

	sesion, err := prepararSesion(*proyecto)
	// La sesión y el servicio vivo los cambian la bandeja y el vigilante, cada uno en su goroutine.
	var mu sync.Mutex
	var vivo *cliente.Cliente // el servicio según el último repaso; nil si está detenido
	actual := func() *sesionEscritorio {
		mu.Lock()
		defer mu.Unlock()
		return sesion
	}
	enMarcha := func() *cliente.Cliente {
		mu.Lock()
		defer mu.Unlock()
		return vivo
	}
	cargar := func(nueva *sesionEscritorio) {
		mu.Lock()
		sesion = nueva
		mu.Unlock()
		ventana.SetURL(nueva.url)
	}

	opciones := application.WebviewWindowOptions{Title: nombre, Width: 1280, Height: 820, MinWidth: 720, MinHeight: 480, Hidden: *oculto}
	if err != nil {
		opciones.HTML = paginaError(err)
	} else {
		opciones.URL = sesion.url
	}
	ventana = app.Window.NewWithOptions(opciones)
	// Cerrar la ventana la oculta: la aplicación sigue en la bandeja y el servicio sigue atendiendo.
	ventana.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		ventana.Hide()
		e.Cancel()
	})
	mostrar := func() {
		ventana.Show()
		ventana.Focus()
	}
	// La abre el instalador, un proceso en segundo plano al que Windows no deja traer ventanas al frente: se pone
	// encima de las demás un momento para que se vea que volvió.
	if *actualizado && !*oculto {
		ventana.SetAlwaysOnTop(true)
		mostrar()
		time.AfterFunc(3*time.Second, func() { ventana.SetAlwaysOnTop(false) })
	}
	irA := func(pagina string) {
		ventana.ExecJS("location.hash = '" + pagina + "'")
		mostrar()
	}
	// avisar abre en la ventana el aviso de la Web UI, con el estilo de la app. Si la página no lo tiene (una
	// versión anterior del servicio), pide el aviso del sistema con «tramtai:<accion>:preguntar».
	avisar := func(accion string) {
		mostrar()
		ventana.ExecJS("window.tramtaiEscritorio ? window.tramtaiEscritorio." + accion + "() : window.chrome.webview.postMessage('tramtai:" + accion + ":preguntar')")
	}
	preguntar := func(titulo, texto, boton string, accion func()) {
		pregunta := app.Dialog.Question().SetTitle(titulo).SetMessage(texto)
		pregunta.AddButton(boton).OnClick(accion)
		pregunta.AddButton(idioma.T("Cancelar")).SetAsDefault()
		pregunta.Show()
	}
	// -proyecto sobre una carpeta que pide confianza: se pregunta con un aviso del sistema cuando la app ya arrancó.
	if texto, ok := confiarAlAbrir(*proyecto, err); ok {
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			go preguntar(idioma.T("Abrir el proyecto"), texto, idioma.T("Confiar y abrir"), func() {
				if nueva, err := confiarYAbrir(*proyecto); err != nil {
					ventana.SetHTML(paginaError(err))
				} else {
					cargar(nueva)
				}
			})
		})
	}
	detener := func(cli *cliente.Cliente) {
		ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelar()
		_ = cli.Detener(ctx)
	}
	detenerYSalir := func(cli *cliente.Cliente) {
		detener(cli)
		app.Quit()
	}
	recibir = func(mensaje, origen string) {
		cli := enMarcha()
		// Solo la Web UI del servicio en marcha puede pedirlo.
		if cli == nil || !strings.HasPrefix(origen, cli.Base+"/") {
			return
		}
		switch mensaje {
		case "tramtai:salir:0", "tramtai:salir:1":
			if mensaje == "tramtai:salir:1" {
				_ = guardarPreferencias(preferencias{SalirSinPreguntar: true})
			}
			detenerYSalir(cli)
		case "tramtai:salir:preguntar":
			preguntar(idioma.T("Salir de Tramtai"), textoSalir, idioma.T("Salir y detener"), func() { detenerYSalir(cli) })
		case "tramtai:detener:preguntar":
			preguntar(idioma.T("Detener el servicio"), textoDetener, idioma.T("Detener"), func() { detener(cli) })
		case "tramtai:carpeta":
			// «Examinar…» al abrir un proyecto: la carpeta elegida vuelve a la Web UI.
			ruta, err := app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).SetTitle(idioma.T("Carpeta del proyecto")).PromptForSingleSelection()
			if err == nil && ruta != "" {
				texto, _ := json.Marshal(ruta)
				ventana.ExecJS("window.tramtaiCarpeta && window.tramtaiCarpeta(" + string(texto) + ")")
			}
		}
	}

	// Clic en el icono abre la ventana; clic derecho, el menú. El menú se rehace cuando cambia lo que muestra.
	bandeja := app.SystemTray.New()
	bandeja.SetIcon(icono)
	bandeja.OnClick(mostrar)
	var repasar func()

	// construir pinta en Wails las entradas que calcula entradasMenu y conecta cada acción.
	construir := func(cli *cliente.Cliente, e estadoMenu) *application.Menu {
		lado, oscuro := ladoIconoMenu(), menuOscuro()
		verde, rojo := circulo(colorVerde, lado), circulo(colorRojo, lado)
		bitmap := func(nombre string) []byte {
			switch nombre {
			case "verde":
				return verde
			case "rojo":
				return rojo
			}
			return iconoMenu(nombre, lado, oscuro)
		}
		alPulsar := func(n entrada) func(*application.Context) {
			switch n.accion {
			case accionMostrar:
				return func(*application.Context) { mostrar() }
			case accionAcercaDe:
				return func(*application.Context) {
					mostrar()
					ventana.ExecJS("window.tramtaiAcercaDe && window.tramtaiAcercaDe()")
				}
			case accionActualizar:
				return func(*application.Context) {
					mostrar()
					ventana.ExecJS("window.tramtaiActualizar && window.tramtaiActualizar()")
				}
			case accionBuscarActualizaciones:
				return func(*application.Context) {
					mostrar()
					ventana.ExecJS("window.tramtaiBuscarActualizaciones && window.tramtaiBuscarActualizaciones()")
				}
			case accionAlternarApi:
				p, habilitada := n.punto, n.habilitada
				return func(*application.Context) {
					ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancelar()
					if err := cli.ActivarPunto(ctx, p.Instancia, p.Nombre, !habilitada); err != nil {
						// Sin aviso no pasaría nada visible: se abre la ventana con el error, como en la Web UI.
						mostrar()
						texto, _ := json.Marshal(nucleo.MensajeDe(err))
						ventana.ExecJS("window.tramtaiAviso && window.tramtaiAviso(" + string(texto) + ")")
					}
					repasar()
				}
			case accionIrMisApis:
				return func(*application.Context) { irA("conectar") }
			case accionCopiarDireccion:
				return func(*application.Context) { app.Clipboard.SetText(cli.Base) }
			case accionIniciar:
				return func(*application.Context) {
					// Si ya está en marcha solo vuelve a entrar; si no, lo inicia en segundo plano.
					if nueva, err := prepararSesion(*proyecto); err != nil {
						ventana.SetHTML(paginaError(err))
						mostrar()
					} else {
						cargar(nueva)
					}
					repasar()
				}
			case accionReiniciar:
				return func(*application.Context) {
					ejecutable, err := buscarTramtai()
					datos, errDatos := carpetas.DirDatos()
					if err != nil || errDatos != nil {
						return
					}
					// Igual que desde la Web UI: el nuevo proceso espera a que este termine sus trabajos y usa el puerto
					// configurado; el repaso vuelve a entrar cuando responde.
					if cliente.Lanzar(ejecutable, datos, "servicio", "iniciar", "--esperar-cierre", "--registro", filepath.Join(datos, "servicio.log")) == nil {
						detener(cli)
					}
				}
			case accionAvisarDetener:
				return func(*application.Context) { avisar("detener") }
			case accionAutoarranque:
				return func(c *application.Context) {
					if propio, err := os.Executable(); err == nil {
						if c.IsChecked() {
							_ = autoarranque.Activar(propio)
						} else {
							_ = autoarranque.Desactivar()
						}
					}
					repasar()
				}
			case accionSalir:
				return func(*application.Context) { app.Quit() }
			case accionDetenerYSalir:
				return func(*application.Context) { detenerYSalir(cli) }
			case accionAvisarSalir:
				return func(*application.Context) { avisar("salir") }
			}
			return nil
		}
		menu := app.NewMenu()
		pintarMenu(menu, entradasMenu(e), bitmap, alPulsar)
		return menu
	}

	// repasar comprueba el servicio y las APIs. Si el servicio volvió distinto (reiniciado, en otro puerto o en
	// uno libre porque el suyo estaba ocupado) la ventana vuelve a entrar con un enlace nuevo; si está detenido no
	// se inicia: eso solo lo hace «Iniciar el servicio».
	var muMenu sync.Mutex
	var firma string
	var menuActual *application.Menu
	repasar = func() {
		muMenu.Lock()
		defer muMenu.Unlock()
		datos, err := carpetas.DirDatos()
		if err != nil {
			return
		}
		cli, err := cliente.Conectar(datos)
		if err != nil {
			cli = nil
		}
		if s := actual(); cli != nil && (s == nil || s.cli.Descriptor.Identificador != cli.Descriptor.Identificador) {
			if nueva, err := prepararSesion(*proyecto); err == nil {
				cargar(nueva)
			}
		}
		// Las APIs de todos los espacios en marcha (global y proyectos), no solo las de la ventana.
		var estadoServicio contrato.Estado
		if cli != nil {
			ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
			estadoServicio, _ = cli.Estado(ctx)
			cancelar()
		}
		puntos := estadoServicio.Puntos
		proyectos := map[string]string{}
		for _, i := range estadoServicio.Instancias {
			if i.Ambito == "proyecto" {
				proyectos[i.ID] = filepath.Base(i.Raiz)
			}
		}
		conWindows, _ := autoarranque.Activo()
		sinPreguntar := leerPreferencias().SalirSinPreguntar
		mu.Lock()
		vivo = cli
		mu.Unlock()

		// También el tema y el DPI del sistema, porque cambian el color y el tamaño de los iconos.
		masNueva := ""
		if estadoServicio.Actualizacion != nil {
			masNueva = estadoServicio.Actualizacion.Version
		}
		nueva := fmt.Sprint(conWindows, sinPreguntar, puntos, menuOscuro(), ladoIconoMenu(), masNueva)
		if cli != nil {
			nueva += cli.Base
		}
		if nueva == firma {
			return
		}
		firma = nueva
		estado := idioma.T("Servicio detenido")
		if cli != nil {
			estado = idioma.Tf("Servicio en marcha en %s", strings.TrimPrefix(cli.Base, "http://"))
		}
		bandeja.SetTooltip(nombre + " · " + estado)
		anterior := menuActual
		base := ""
		if cli != nil {
			base = cli.Base
		}
		menuActual = construir(cli, estadoMenu{base: base, puntos: puntos, proyectos: proyectos, conWindows: conWindows,
			sinPreguntar: sinPreguntar, nueva: masNueva, autoarranque: autoarranque.Disponible, windows: runtime.GOOS == "windows"})
		bandeja.SetMenu(menuActual)
		if anterior != nil {
			anterior.Clear() // suelta los elementos del menú anterior del registro global de Wails
		}
	}
	repasar()
	go func() {
		for range time.Tick(intervaloVigilancia) {
			repasar()
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// pintarMenu añade a menu las entradas, con su icono y la acción que les da alPulsar.
func pintarMenu(menu *application.Menu, entradas []entrada, bitmap func(string) []byte, alPulsar func(entrada) func(*application.Context)) {
	for _, n := range entradas {
		var elemento *application.MenuItem
		switch {
		case n.separador:
			menu.AddSeparator()
			continue
		case n.submenu != nil:
			pintarMenu(menu.AddSubmenu(n.texto), n.submenu, bitmap, alPulsar)
			continue
		case n.casilla:
			elemento = menu.AddCheckbox(n.texto, n.marcada)
		default:
			elemento = menu.Add(n.texto)
		}
		if n.icono != "" {
			elemento.SetBitmap(bitmap(n.icono))
		}
		if f := alPulsar(n); f != nil {
			elemento.OnClick(f)
		}
	}
}

type sesionEscritorio struct {
	cli *cliente.Cliente
	url string
}

// preferencias de la app de escritorio, en escritorio.json junto al directorio de datos.
type preferencias struct {
	SalirSinPreguntar bool `json:"salir_sin_preguntar"` // «No volver a mostrar» en el aviso de Salir
}

func rutaPreferencias() (string, error) {
	raiz, err := carpetas.RaizUsuario()
	return filepath.Join(raiz, "escritorio.json"), err
}

func leerPreferencias() (p preferencias) {
	if ruta, err := rutaPreferencias(); err == nil {
		if datos, err := os.ReadFile(ruta); err == nil {
			_ = json.Unmarshal(datos, &p)
		}
	}
	return p
}

func guardarPreferencias(p preferencias) error {
	ruta, err := rutaPreferencias()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	datos, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(ruta, datos, 0o600)
}

// circulo dibuja un punto de color de lado px, con el borde suavizado, para el menú de la bandeja.
func circulo(c color.RGBA, lado int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, lado, lado))
	centro, radio := float64(lado)/2-0.5, float64(lado)*0.3
	for y := range lado {
		for x := range lado {
			a := math.Min(1, math.Max(0, radio+0.5-math.Hypot(float64(x)-centro, float64(y)-centro)))
			// image.RGBA guarda el color premultiplicado por el alfa.
			img.SetRGBA(x, y, color.RGBA{uint8(float64(c.R) * a), uint8(float64(c.G) * a), uint8(float64(c.B) * a), uint8(255 * a)})
		}
	}
	return codificar(img)
}

// iconoMenu reduce la máscara del icono a lado px (promediando 4×4 muestras por píxel) y la tiñe del color del
// texto del menú: claro en modo oscuro, oscuro en modo claro. Devuelve nil si el icono no existe.
func iconoMenu(nombre string, lado int, oscuro bool) []byte {
	datos, err := iconosMenu.ReadFile("icono/menu/" + nombre + ".png")
	if err != nil {
		return nil
	}
	origen, err := png.Decode(bytes.NewReader(datos))
	if err != nil {
		return nil
	}
	tono := uint32(0x4a)
	if oscuro {
		tono = 0xdc
	}
	const muestras = 4
	b := origen.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, lado, lado))
	for y := range lado {
		for x := range lado {
			var suma uint32
			for j := range muestras {
				for i := range muestras {
					_, _, _, a := origen.At(b.Min.X+(x*muestras+i)*b.Dx()/(lado*muestras), b.Min.Y+(y*muestras+j)*b.Dy()/(lado*muestras)).RGBA()
					suma += a >> 8
				}
			}
			a := suma / (muestras * muestras)
			v := uint8(tono * a / 255)
			img.SetRGBA(x, y, color.RGBA{v, v, v, uint8(a)})
		}
	}
	return codificar(img)
}

func codificar(img image.Image) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// prepararSesion conecta con el servicio (o lo inicia), activa la instancia y obtiene una URL de entrada de un
// solo uso: el token de la instancia nunca pasa por la ventana.
func prepararSesion(proyecto string) (*sesionEscritorio, error) {
	datos, err := carpetas.DirDatos()
	if err != nil {
		return nil, err
	}
	cli, err := cliente.Conectar(datos)
	// El de una versión anterior (tras actualizar) lo detiene IniciarEnSegundoPlano antes de lanzar el nuevo.
	if errors.Is(err, cliente.ErrServicioInactivo) || errors.Is(err, cliente.ErrServicioAnterior) {
		ejecutable, errBuscar := buscarTramtai()
		if errBuscar != nil {
			return nil, errBuscar
		}
		cli, err = cliente.IniciarEnSegundoPlano(ejecutable, datos)
	}
	if err != nil {
		return nil, err
	}
	ambito := "global"
	if proyecto != "" {
		ambito = "proyecto"
		if raiz, ok := configuracion.BuscarProyecto(proyecto); ok {
			proyecto = raiz
		}
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelar()
	var activos []contrato.EstadoInstancia
	if e, err := cli.Estado(ctx); err == nil {
		activos = e.Instancias
	}
	activar, id := espacioAlAbrir(proyecto, activos)
	if activar {
		activacion, err := cli.Activar(ctx, ambito, proyecto)
		if err != nil {
			return nil, err
		}
		id = activacion.Instancia.ID
	}
	url, err := cli.URLWeb(ctx, id)
	if err != nil {
		return nil, err
	}
	return &sesionEscritorio{cli: cli, url: url}, nil
}

// confiarAlAbrir dice si el fallo al abrir -proyecto es la pregunta de confianza (una carpeta que ya traía .tramtai/ o
// que estaba en otra ruta) y devuelve el texto con el que se pregunta, el mismo que en la CLI.
func confiarAlAbrir(proyecto string, err error) (string, bool) {
	var pregunta *cliente.ErrorConfianza
	if proyecto == "" || !errors.As(err, &pregunta) {
		return "", false
	}
	return pregunta.Error() + idioma.T(". ¿Confías en esta carpeta?"), true
}

// confiarYAbrir confía en el proyecto de la carpeta proyecto y vuelve a preparar la sesión de la ventana.
func confiarYAbrir(proyecto string) (*sesionEscritorio, error) {
	datos, err := carpetas.DirDatos()
	if err != nil {
		return nil, err
	}
	cli, err := cliente.Conectar(datos)
	if err != nil {
		return nil, err
	}
	if raiz, ok := configuracion.BuscarProyecto(proyecto); ok {
		proyecto = raiz
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelar()
	si := true
	if _, err := cli.AbrirProyecto(ctx, proyecto, &si, false); err != nil {
		return nil, err
	}
	return prepararSesion(proyecto)
}

// espacioAlAbrir decide qué abre la ventana: el proyecto pedido (-proyecto), el espacio activo que se usó por última
// vez (sin reactivar la global si se detuvo a propósito) o, si no hay ninguno activo, la global. activar indica que
// hay que activarlo antes.
func espacioAlAbrir(proyecto string, activos []contrato.EstadoInstancia) (activar bool, id string) {
	if proyecto != "" || len(activos) == 0 {
		return true, ""
	}
	elegido := activos[0]
	for _, a := range activos[1:] {
		if a.UltimoUso.After(elegido.UltimoUso) {
			elegido = a
		}
	}
	return false, elegido.ID
}

// buscarTramtai localiza el ejecutable del servicio: junto a esta aplicación o, salvo en la portable, en el PATH. La
// portable no usa el del PATH: sería la copia instalada, con sus datos en otra carpeta.
func buscarTramtai() (string, error) {
	nombre := "tramtai"
	if propio, err := os.Executable(); err == nil {
		for _, candidato := range []string{"tramtai.exe", "tramtai"} {
			ruta := filepath.Join(filepath.Dir(propio), candidato)
			if info, err := os.Stat(ruta); err == nil && !info.IsDir() {
				return ruta, nil
			}
		}
	}
	if carpetas.Portable() {
		return "", errors.New(idioma.T("falta tramtai.exe junto a la aplicación portable: vuelve a descomprimir Tramtai"))
	}
	ruta, err := exec.LookPath(nombre)
	if err != nil {
		return "", errors.New(idioma.T("no se encontró el ejecutable tramtai junto a la aplicación ni en el PATH"))
	}
	return ruta, nil
}

func paginaError(err error) string {
	titulo, consejo := idioma.T("No se pudo conectar con el servicio de Tramtai"),
		`<p>`+idioma.T("Prueba <code>tramtai servicio iniciar -d</code> en una terminal y vuelve a abrir la aplicación.")+`</p>`
	if _, errRaiz := carpetas.RaizUsuario(); errRaiz != nil {
		// La carpeta de la copia no sirve (temporal, dentro del .zip o de solo lectura): el mensaje ya dice qué hacer.
		titulo, consejo = idioma.T("Tramtai no puede usar esta carpeta"), ""
	}
	return `<!doctype html><html lang="` + idioma.Activo + `"><body style="font-family:system-ui,sans-serif;padding:2rem;background:#1d1d1d;color:#eee">` +
		`<h2>` + titulo + `</h2><p>` + html.EscapeString(nucleo.MensajeDe(err)) + `</p>` + consejo + `</body></html>`
}
