package tui

import (
	"image/color"
	"io"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/tui/datos"
)

// pulso no es de nadie: lo manda Captura para que el filtro haga una foto de la vista entre mensaje y mensaje.
type pulso struct{}

// Captura pinta la terminal como la vería alguien, para las capturas de la portada (internal/tui/captura, que lanza
// web/scripts/capturas.mjs): con el cliente dado, en ese idioma y tema, de ancho×alto, tras pulsar las teclas desde
// Inicio. Tras abrir y tras cada tecla espera a que la vista se asiente (sin «Cargando…» y sin cambiar durante medio
// segundo, o como mucho espera) y devuelve lo que se ve, con sus estilos ANSI.
func Captura(o Opciones, cli *cliente.Cliente, lengua string, oscuro bool, ancho, alto int, teclas []string, espera time.Duration) (string, error) {
	m := nuevoEn(o, lengua)
	m.cli = cli
	defer m.cerrar()
	// El filtro corre en el bucle del programa: es el único sitio donde leer la vista sin carreras.
	var vista atomic.Value
	vista.Store("")
	filtro := func(mod tea.Model, msg tea.Msg) tea.Msg {
		vista.Store(mod.View().Content)
		return msg
	}
	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler(),
		tea.WithWindowSize(ancho, alto), tea.WithFilter(filtro))
	cargando := datos.Texto(lengua, "comun.cargando")
	asentar := func() {
		anterior, iguales := "", 0
		for limite := time.Now().Add(espera); time.Now().Before(limite) && iguales < 5; {
			time.Sleep(100 * time.Millisecond)
			p.Send(pulso{})
			actual := vista.Load().(string)
			if actual != "" && actual == anterior && !strings.Contains(actual, cargando) {
				iguales++
			} else {
				anterior, iguales = actual, 0
			}
		}
	}
	go func() {
		fondo := color.Color(color.White)
		if oscuro {
			fondo = color.Black
		}
		p.Send(tea.WindowSizeMsg{Width: ancho, Height: alto})
		p.Send(tea.BackgroundColorMsg{Color: fondo})
		asentar()
		for _, k := range teclas {
			// shortcut: solo teclas de un carácter (las de las páginas, del 1 al 7); con Tab o flechas, usar prueba.Tecla.
			p.Send(tea.KeyPressMsg{Code: []rune(k)[0], Text: k})
			asentar()
		}
		p.Quit()
	}()
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	return final.View().Content, nil
}
