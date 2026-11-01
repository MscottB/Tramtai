package tui

import (
	"image/color"
	"io"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/tui/datos"
)

// pulso no es de nadie: lo manda Captura para que el filtro haga una foto de la vista entre mensaje y mensaje.
type pulso struct{}

// secuenciaANSI es un estilo SGR: se quita antes de buscar texto que el estilo parte (la fila de la red lleva el tiempo
// en otro color).
var secuenciaANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// redEnMovimiento reconoce, en ese idioma, lo que dice la red de Inicio mientras todavía cambia: la cabecera con
// peticiones en curso («2 peticiones en curso.», «1 petición en curso.») y las filas que ya terminaron, fallaron o se
// cancelaron («terminada 9 s») y siguen a la vista unos segundos antes de irse. Una captura hecha entonces enseñaría una
// pantalla que cambia a los pocos segundos. Se busca en el texto sin estilos (secuenciaANSI).
func redEnMovimiento(lengua string) *regexp.Regexp {
	q := func(clave string) string { return regexp.QuoteMeta(datos.Texto(lengua, clave)) }
	cabecera := strings.Replace(q("vivo.enCurso"), `\{n\}`, `\d+`, 1) + "|" + q("vivo.enCursoUno")
	fila := "(?:" + q("vivo.etapaLlegada") + "|" + q("vivo.etapaFallida") + "|" + q("vivo.etapaCancelada") + `)\s+` +
		strings.Replace(q("vivo.hace"), `\{s\}`, `\d+`, 1)
	return regexp.MustCompile(cabecera + "|" + fila)
}

// Captura pinta la terminal como la vería alguien, para las capturas de la portada (internal/tui/captura, que lanza
// web/scripts/capturas.mjs): con el cliente dado, en ese idioma y tema, de ancho×alto, tras pulsar las teclas desde
// Inicio. Tras abrir y tras cada tecla espera a que la vista se asiente (sin «Cargando…», sin «Comprobando…», con la red
// de Inicio quieta, es decir sin peticiones en curso ni filas ya terminadas que se van, y sin cambiar durante medio
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
	cargando, comprobando := datos.Texto(lengua, "comun.cargando"), datos.Texto(lengua, "inicio.comprobando")
	enMovimiento := redEnMovimiento(lengua)
	asentar := func() {
		anterior, iguales := "", 0
		for limite := time.Now().Add(espera); time.Now().Before(limite) && iguales < 5; {
			time.Sleep(100 * time.Millisecond)
			p.Send(pulso{})
			actual := vista.Load().(string)
			if actual != "" && actual == anterior && !strings.Contains(actual, cargando) &&
				!strings.Contains(actual, comprobando) && !enMovimiento.MatchString(secuenciaANSI.ReplaceAllString(actual, "")) {
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
