package comp

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/tui/pagina"
)

type (
	// Pregunta pide a la app que enseñe una Confirmacion; la respuesta, Confirmado o Cancelado, le llega a la página.
	Pregunta struct{ Texto string }
	// Confirmado lo responde una Confirmacion con s o y (Intro no confirma).
	Confirmado struct{}
)

// Confirmar pregunta antes de algo que borra o no tiene vuelta atrás. La página recibe Confirmado o Cancelado; si puede
// preguntar por varias cosas, recuerda cuál.
func Confirmar(pregunta string) tea.Cmd { return func() tea.Msg { return Pregunta{Texto: pregunta} } }

// Confirmacion es la caja de la pregunta. La abre la app; mientras se ve, se queda con las teclas. Si la pregunta no cabe
// en el alto que hay, el texto se desplaza con ↑↓, RePág, AvPág, Inicio y Fin, y el pie se ve siempre.
type Confirmacion struct {
	Texto string

	desp, max int // la primera línea que se ve y hasta dónde se puede bajar (lo fija Vista)
}

// Update responde a una tecla: listo dice si la pregunta ya tiene respuesta, que llega con cmd. Solo s o y confirman:
// Intro no hace nada, para que un Intro de más no borre. Las teclas de desplazamiento mueven el texto.
func (c *Confirmacion) Update(k tea.KeyPressMsg) (listo bool, cmd tea.Cmd) {
	switch k.String() {
	case "s", "S", "y", "Y":
		return true, func() tea.Msg { return Confirmado{} }
	case "n", "N", "esc":
		return true, func() tea.Msg { return Cancelado{} }
	case "up", "k":
		c.desp--
	case "down", "j":
		c.desp++
	case "pgup":
		c.desp -= 5
	case "pgdown":
		c.desp += 5
	case "home":
		c.desp = 0
	case "end":
		c.desp = c.max
	}
	c.desp = min(max(c.desp, 0), c.max)
	return false, nil
}

// Vista pinta la pregunta en una caja de como mucho ancho columnas y alto filas: si el texto no cabe, se corta según el
// desplazamiento, y el pie (con el aviso de que se desplaza) se ve siempre.
func (c *Confirmacion) Vista(e *pagina.Estilos, ancho, alto int) string {
	interior := max(ancho-4, 10)
	lineas := strings.Split(lipgloss.NewStyle().Width(interior).Render(c.Texto), "\n")
	filas := max(alto-4, 1) // el borde (2), el hueco y el pie
	c.max = max(len(lineas)-filas, 0)
	c.desp = min(max(c.desp, 0), c.max)
	pie := idioma.T("s confirma · Esc cancela")
	if c.max > 0 {
		pie = idioma.T("↑↓ desplaza") + " " + e.Iconos.Separador + " " + pie
	}
	lineas = lineas[c.desp:min(c.desp+filas, len(lineas))]
	pie = e.Tenue.Render(e.Recortar(pie, interior))
	return lipgloss.NewStyle().Border(e.Borde).Padding(0, 1).Render(strings.Join(append(lineas, "", pie), "\n"))
}
