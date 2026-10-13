// Comando tramtai-escritorio: aplicación de escritorio de Tramtai (sección 13.3). Es un cliente del servicio: lo
// detecta o lo inicia, abre la Web UI ya autenticada en una ventana propia y queda en la bandeja del sistema,
// donde muestra si el servicio está en marcha y, con el clic derecho, tus APIs y las acciones rápidas.
// Cerrar la ventana la deja en la bandeja; Salir detiene el servicio (con un aviso que se puede no volver a ver).
package main

import (
	"bytes"
	"context"
	_ "embed"
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
	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/version"
)

// icono es el de la ventana y la bandeja (go run ./scripts/icono lo regenera).
//
//go:embed icono/tramtai.png
var icono []byte

// intervaloVigilancia es cada cuánto se repasa el estado del servicio y de las APIs.
const intervaloVigilancia = 3 * time.Second

// Círculos del menú de la bandeja: en marcha o activa (verde), detenido o desactivada (rojo).
var (
	verde = circulo(color.RGBA{0x21, 0xba, 0x45, 0xff})
	rojo  = circulo(color.RGBA{0xd3, 0x2f, 0x2f, 0xff})
)

const textoDetener = "Se detienen todas las instancias y las aplicaciones conectadas dejarán de funcionar. Tramtai sigue en la bandeja para volver a iniciarlo. ¿Continuar?"
const textoSalir = "Al salir se detiene el servicio: las aplicaciones conectadas a tus APIs dejarán de funcionar hasta que vuelvas a abrir Tramtai. ¿Continuar?"

func main() {
	proyecto := flag.String("proyecto", "", "raíz de un proyecto con .tramtai/config.yaml (por defecto, la instancia global)")
	oculto := flag.Bool("oculto", false, "arranca solo en la bandeja, sin mostrar la ventana (arranque automático)")
	flag.Parse()

	var ventana *application.WebviewWindow
	var recibir func(mensaje, origen string)
	app := application.New(application.Options{
		Name:        "Tramtai",
		Description: "Pasarela local de agentes",
		Icon:        icono,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "cl.tramtai.escritorio",
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

	opciones := application.WebviewWindowOptions{Title: "Tramtai " + version.Version, Width: 1280, Height: 820, MinWidth: 720, MinHeight: 480, Hidden: *oculto}
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
		pregunta.AddButton("Cancelar").SetAsDefault()
		pregunta.Show()
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
			preguntar("Salir de Tramtai", textoSalir, "Salir y detener", func() { detenerYSalir(cli) })
		case "tramtai:detener:preguntar":
			preguntar("Detener el servicio", textoDetener, "Detener", func() { detener(cli) })
		}
	}

	// Clic en el icono abre la ventana; clic derecho, el menú. El menú se rehace cuando cambia lo que muestra.
	bandeja := app.SystemTray.New()
	bandeja.SetIcon(icono)
	bandeja.OnClick(mostrar)
	var repasar func()

	construir := func(cli *cliente.Cliente, puntos []servicio.EstadoPunto, proyectos map[string]string, conWindows, sinPreguntar bool) *application.Menu {
		menu := app.NewMenu()
		if cli == nil {
			menu.Add("Servicio detenido").SetBitmap(rojo).OnClick(func(*application.Context) { mostrar() })
		} else {
			menu.Add("Servicio en marcha · " + strings.TrimPrefix(cli.Base, "http://")).SetBitmap(verde).OnClick(func(*application.Context) { mostrar() })
		}
		menu.AddSeparator()
		menu.Add("Abrir Tramtai").OnClick(func(*application.Context) { mostrar() })
		if cli != nil {
			activas := 0
			for _, p := range puntos {
				if p.Activo {
					activas++
				}
			}
			apis := menu.AddSubmenu(fmt.Sprintf("Mis APIs (%d de %d activas)", activas, len(puntos)))
			for _, p := range puntos {
				habilitada := p.Error != "desactivado"
				etiqueta := p.Nombre
				for _, extra := range []string{p.Grupo, proyectos[p.Instancia]} {
					if extra != "" {
						etiqueta += " · " + extra
					}
				}
				if habilitada && !p.Activo {
					etiqueta += " (con error)"
				}
				circ := rojo
				if p.Activo {
					circ = verde
				}
				// Clic: desactivarla si está habilitada, activarla si no. Se guarda y se aplica sin reiniciar.
				apis.Add(etiqueta).SetBitmap(circ).OnClick(func(*application.Context) {
					ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancelar()
					_ = cli.ActivarPunto(ctx, p.Instancia, p.Nombre, !habilitada)
					repasar()
				})
			}
			if len(puntos) > 0 {
				apis.AddSeparator()
				apis.Add("Gestionar en Mis APIs").OnClick(func(*application.Context) { irA("conectar") })
			} else {
				apis.Add("Crear una API…").OnClick(func(*application.Context) { irA("conectar") })
			}
			menu.Add("Copiar la dirección para tus apps").OnClick(func(*application.Context) { app.Clipboard.SetText(cli.Base) })
		}
		menu.AddSeparator()
		if cli == nil {
			menu.Add("Iniciar el servicio").OnClick(func(*application.Context) {
				// Si ya está en marcha solo vuelve a entrar; si no, lo inicia en segundo plano.
				if nueva, err := prepararSesion(*proyecto); err != nil {
					ventana.SetHTML(paginaError(err))
					mostrar()
				} else {
					cargar(nueva)
				}
				repasar()
			})
		} else {
			menu.Add("Reiniciar el servicio").OnClick(func(*application.Context) {
				ejecutable, err := buscarTramtai()
				datos, errDatos := dirDatos()
				if err != nil || errDatos != nil {
					return
				}
				// Igual que desde la Web UI: el nuevo proceso espera a que este termine sus trabajos y usa el puerto
				// configurado; el repaso vuelve a entrar cuando responde.
				if cliente.Lanzar(ejecutable, datos, "servicio", "iniciar", "--esperar-cierre", "--registro", filepath.Join(datos, "servicio.log")) == nil {
					detener(cli)
				}
			})
			menu.Add("Detener el servicio…").OnClick(func(*application.Context) { avisar("detener") })
		}
		// El arranque con Windows también se cambia en Ajustes; el repaso mantiene la marca al día.
		if runtime.GOOS == "windows" {
			menu.AddCheckbox("Iniciar con Windows", conWindows).OnClick(func(c *application.Context) {
				if propio, err := os.Executable(); err == nil {
					if c.IsChecked() {
						_ = autoarranque.Activar(propio)
					} else {
						_ = autoarranque.Desactivar()
					}
				}
				repasar()
			})
		}
		menu.AddSeparator()
		switch {
		case cli == nil:
			menu.Add("Salir").OnClick(func(*application.Context) { app.Quit() })
		case sinPreguntar:
			menu.Add("Salir y detener el servicio").OnClick(func(*application.Context) { detenerYSalir(cli) })
		default:
			menu.Add("Salir y detener el servicio…").OnClick(func(*application.Context) { avisar("salir") })
		}
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
		datos, err := dirDatos()
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
		var estadoServicio servicio.Estado
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

		nueva := fmt.Sprint(conWindows, sinPreguntar, puntos)
		if cli != nil {
			nueva += cli.Base
		}
		if nueva == firma {
			return
		}
		firma = nueva
		estado := "Servicio detenido"
		if cli != nil {
			estado = "Servicio en marcha en " + strings.TrimPrefix(cli.Base, "http://")
		}
		bandeja.SetTooltip("Tramtai · " + estado)
		anterior := menuActual
		menuActual = construir(cli, puntos, proyectos, conWindows, sinPreguntar)
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

type sesionEscritorio struct {
	cli *cliente.Cliente
	url string
}

// preferencias de la app de escritorio, en escritorio.json junto al directorio de datos.
type preferencias struct {
	SalirSinPreguntar bool `json:"salir_sin_preguntar"` // «No volver a mostrar» en el aviso de Salir
}

func rutaPreferencias() (string, error) {
	datos, err := dirDatos()
	return filepath.Join(filepath.Dir(datos), "escritorio.json"), err
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

// circulo dibuja un punto de color de 16 px, con el borde suavizado, para el menú de la bandeja.
func circulo(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			a := math.Min(1, math.Max(0, 5.5-math.Hypot(float64(x)-7.5, float64(y)-7.5)))
			// image.RGBA guarda el color premultiplicado por el alfa.
			img.SetRGBA(x, y, color.RGBA{uint8(float64(c.R) * a), uint8(float64(c.G) * a), uint8(float64(c.B) * a), uint8(255 * a)})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// dirDatos es el directorio de datos de Tramtai, donde está el descriptor del servicio.
func dirDatos() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tramtai", "datos"), nil
}

// prepararSesion conecta con el servicio (o lo inicia), activa la instancia y obtiene una URL de entrada de un
// solo uso: el token de la instancia nunca pasa por la ventana.
func prepararSesion(proyecto string) (*sesionEscritorio, error) {
	datos, err := dirDatos()
	if err != nil {
		return nil, err
	}
	cli, err := cliente.Conectar(datos)
	if errors.Is(err, cliente.ErrServicioInactivo) {
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
		if raiz, ok := nucleo.BuscarProyecto(proyecto); ok {
			proyecto = raiz
		}
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelar()
	activacion, err := cli.Activar(ctx, ambito, proyecto)
	if err != nil {
		return nil, err
	}
	url, err := cli.URLWeb(ctx, activacion.Instancia.ID)
	if err != nil {
		return nil, err
	}
	return &sesionEscritorio{cli: cli, url: url}, nil
}

// buscarTramtai localiza el ejecutable del servicio: junto a esta aplicación o en el PATH.
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
	ruta, err := exec.LookPath(nombre)
	if err != nil {
		return "", errors.New("no se encontró el ejecutable tramtai junto a la aplicación ni en el PATH")
	}
	return ruta, nil
}

func paginaError(err error) string {
	return `<!doctype html><html lang="es"><body style="font-family:system-ui,sans-serif;padding:2rem;background:#1d1d1d;color:#eee">` +
		`<h2>No se pudo conectar con el servicio de Tramtai</h2><p>` + html.EscapeString(nucleo.MensajeDe(err)) + `</p>` +
		`<p>Prueba <code>tramtai servicio iniciar -d</code> en una terminal y vuelve a abrir la aplicación.</p></body></html>`
}
