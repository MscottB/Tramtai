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
	// Confirmado lo responde una Confirmacion con s, y o Intro.
	Confirmado struct{}
)

// Confirmar pregunta antes de algo que borra o no tiene vuelta atrás. La página recibe Confirmado o Cancelado; si puede
// preguntar por varias cosas, recuerda cuál.
func Confirmar(pregunta string) tea.Cmd { return func() tea.Msg { return Pregunta{Texto: pregunta} } }

// Confirmacion es la caja de la pregunta. La abre la app; mientras se ve, se queda con las teclas.
type Confirmacion struct{ Texto string }

// Update responde a una tecla: listo dice si la pregunta ya tiene respuesta, que llega con cmd.
func (c Confirmacion) Update(k tea.KeyPressMsg) (listo bool, cmd tea.Cmd) {
	switch k.String() {
	case "s", "S", "y", "Y", "enter":
		return true, func() tea.Msg { return Confirmado{} }
	case "n", "N", "esc":
		return true, func() tea.Msg { return Cancelado{} }
	}
	return false, nil
}

// Vista pinta la pregunta en una caja de como mucho ancho columnas.
func (c Confirmacion) Vista(e *pagina.Estilos, ancho int) string {
	interior := max(ancho-4, 10)
	texto := lipgloss.NewStyle().Width(interior).Render(c.Texto)
	pie := e.Tenue.Render(e.Recortar(idioma.T("s confirma · Esc cancela"), interior))
	return lipgloss.NewStyle().Border(e.Borde).Padding(0, 1).Render(strings.Join([]string{texto, "", pie}, "\n"))
}
