// Comando tramtai-escritorio: aplicación de escritorio de Tramtai (sección 13.3). Es un cliente del servicio: lo
// detecta o lo inicia, abre la Web UI ya autenticada en una ventana propia y queda en la bandeja del sistema.
// Cerrar la ventana o salir no detiene el servicio: las aplicaciones conectadas siguen funcionando.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"html"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"tramtai/internal/cliente"
	"tramtai/internal/navegador"
	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

// icono es el de la ventana y la bandeja (go run ./scripts/icono lo regenera).
//
//go:embed icono/tramtai.png
var icono []byte

func main() {
	proyecto := flag.String("proyecto", "", "raíz de un proyecto con .tramtai/config.yaml (por defecto, la instancia global)")
	oculto := flag.Bool("oculto", false, "arranca solo en la bandeja, sin mostrar la ventana (arranque automático)")
	flag.Parse()

	var ventana *application.WebviewWindow
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
	})

	sesion, err := prepararSesion(*proyecto)
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

	bandeja := app.SystemTray.New()
	bandeja.SetTooltip("Tramtai")
	bandeja.SetIcon(icono)
	bandeja.OnClick(func() {
		ventana.Show()
		ventana.Focus()
	})
	menu := app.NewMenu()
	menu.Add("Abrir Tramtai").OnClick(func(*application.Context) {
		ventana.Show()
		ventana.Focus()
	})
	menu.Add("Abrir en el navegador").OnClick(func(*application.Context) {
		if sesion == nil {
			return
		}
		ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelar()
		if url, err := sesion.cli.URLWeb(ctx, sesion.instancia); err == nil {
			_ = navegador.Abrir(url)
		}
	})
	menu.AddSeparator()
	menu.Add("Iniciar el servicio").OnClick(func(*application.Context) {
		// Si ya está en marcha solo vuelve a entrar; si no, lo inicia en segundo plano.
		nueva, err := prepararSesion(*proyecto)
		if err != nil {
			ventana.SetHTML(paginaError(err))
		} else {
			sesion = nueva
			ventana.SetURL(nueva.url)
		}
		ventana.Show()
		ventana.Focus()
	})
	menu.Add("Detener el servicio…").OnClick(func(*application.Context) {
		if sesion == nil {
			return
		}
		pregunta := app.Dialog.Question().SetTitle("Detener el servicio").
			SetMessage("Se detienen todas las instancias y las aplicaciones conectadas dejarán de funcionar. ¿Continuar?")
		pregunta.AddButton("Detener").OnClick(func() {
			ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancelar()
			_ = sesion.cli.Detener(ctx)
			app.Quit()
		})
		pregunta.AddButton("Cancelar").SetAsDefault()
		pregunta.Show()
	})
	menu.Add("Salir (el servicio sigue en marcha)").OnClick(func(*application.Context) { app.Quit() })
	bandeja.SetMenu(menu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

type sesionEscritorio struct {
	cli       *cliente.Cliente
	instancia string
	url       string
}

// prepararSesion conecta con el servicio (o lo inicia), activa la instancia y obtiene una URL de entrada de un
// solo uso: el token de la instancia nunca pasa por la ventana.
func prepararSesion(proyecto string) (*sesionEscritorio, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	datos := filepath.Join(dir, "tramtai", "datos")
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
	return &sesionEscritorio{cli: cli, instancia: activacion.Instancia.ID, url: url}, nil
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
