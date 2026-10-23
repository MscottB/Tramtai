package comp

import (
	"regexp"
	"strconv"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"tramtai/internal/tui/pagina"
)

// Visor enseña un texto largo con desplazamiento y búsqueda. Las teclas: ↑ ↓ RePág AvPág, g y G (principio y fin),
// n y N (coincidencia siguiente y anterior).
type Visor struct {
	vp      viewport.Model
	estilos *pagina.Estilos
	alto    int // el que le dan; si el texto no cabe, la última fila es la posición
}

// NuevoVisor crea un visor vacío; las líneas largas se parten.
func NuevoVisor(e *pagina.Estilos) Visor {
	vp := viewport.New()
	vp.SoftWrap = true
	return Visor{vp: vp, estilos: e}
}

// Tamano fija el ancho y el alto del visor.
func (v *Visor) Tamano(ancho, alto int) {
	v.vp.SetWidth(ancho)
	v.alto = alto
	v.ajustar()
}

// Poner cambia el texto y quita la búsqueda.
func (v *Visor) Poner(texto string) {
	v.vp.ClearHighlights()
	v.vp.SetContent(texto)
	v.ajustar()
}

// desborda dice si el texto no cabe en el alto que le dan.
func (v *Visor) desborda() bool { return v.vp.TotalLineCount() > v.alto }

// ajustar deja al texto el alto entero o, si no cabe, todo menos la fila de la posición, como en Inicio y Probar.
func (v *Visor) ajustar() {
	if v.desborda() {
		v.vp.SetHeight(max(v.alto-1, 1))
	} else {
		v.vp.SetHeight(v.alto)
	}
}

// Buscar resalta las coincidencias, sin distinguir mayúsculas, salta a la primera y dice cuántas hay.
func (v *Visor) Buscar(texto string) int {
	v.vp.ClearHighlights()
	if texto == "" {
		return 0
	}
	coincidencias := regexp.MustCompile("(?i)"+regexp.QuoteMeta(texto)).FindAllStringIndex(v.vp.GetContent(), -1)
	v.vp.SetYOffset(0)
	v.vp.SetHighlights(coincidencias)
	return len(coincidencias)
}

// Update desplaza y salta entre coincidencias; dice si la tecla era suya.
func (v *Visor) Update(msg tea.Msg) bool {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false
	}
	switch k.String() {
	case "n":
		v.vp.HighlightNext()
	case "N":
		v.vp.HighlightPrevious()
	case "g", "home":
		v.vp.GotoTop()
	case "G", "end":
		v.vp.GotoBottom()
	case "up", "k", "down", "j", "pgup", "pgdown":
		v.vp, _ = v.vp.Update(msg)
	default:
		return false
	}
	return true
}

// Vista pinta lo que se ve del texto, con los estilos de ahora: si el fondo llega después, cambia de tema.
func (v Visor) Vista() string {
	v.vp.HighlightStyle = v.estilos.Tenue.Reverse(true)
	v.vp.SelectedHighlightStyle = v.estilos.Seleccion.Reverse(!v.estilos.Color)
	if !v.desborda() || v.alto < 2 {
		return v.vp.View()
	}
	flechas := "↑↓"
	if v.estilos.ASCII {
		flechas = "^v"
	}
	total := v.vp.TotalLineCount()
	visto := min(v.vp.YOffset()+v.vp.Height(), total)
	return v.vp.View() + "\n" + v.estilos.Tenue.Render(flechas+" "+strconv.Itoa(visto)+"/"+strconv.Itoa(total))
}
