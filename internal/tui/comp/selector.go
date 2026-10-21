package comp

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/tui/pagina"
)

// Item es una opción de un Selector.
type Item struct{ Valor, Texto, Detalle string }

// Elegido lo responde un Selector con Intro.
type Elegido struct{ Valor string }

// Selector elige una opción de una lista: lo que se escribe la filtra, ↑ ↓ mueven, Intro responde Elegido y Esc
// Cancelado. Sirve para el espacio, el modelo, el agente o el esfuerzo.
type Selector struct {
	Titulo  string
	estilos *pagina.Estilos
	lista   Lista[Item]
	filtro  string
}

// NuevoSelector crea un selector con el cursor en la primera opción.
func NuevoSelector(e *pagina.Estilos, titulo string, items []Item) Selector {
	s := Selector{Titulo: titulo, estilos: e, lista: NuevaLista(e, func(i Item) string { return i.Texto + " " + i.Detalle })}
	s.lista.Fila = func(i Item) string {
		if i.Detalle == "" {
			return i.Texto
		}
		return i.Texto + "  " + e.Tenue.Render(i.Detalle)
	}
	s.lista.Poner(items)
	return s
}

// Update atiende las teclas del selector.
func (s *Selector) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch k.String() {
	case "esc":
		return func() tea.Msg { return Cancelado{} }
	case "enter":
		if i, ok := s.lista.Seleccion(); ok {
			return func() tea.Msg { return Elegido{Valor: i.Valor} }
		}
		return nil
	case "backspace":
		r := []rune(s.filtro)
		s.filtro = string(r[:max(len(r)-1, 0)])
	default:
		if k.Text == "" { // las flechas mueven; las letras (también k y j) filtran
			s.lista.Update(k)
			return nil
		}
		s.filtro += k.Text
	}
	s.lista.Buscar(s.filtro)
	return nil
}

// Vista pinta el título, el filtro y las opciones en ancho×alto.
func (s *Selector) Vista(ancho, alto int) string {
	e := s.estilos
	filtro := e.Tenue.Render(idioma.T("Escribe para filtrar"))
	if s.filtro != "" {
		filtro = "/ " + s.filtro
	}
	cabecera := []string{e.Titulo.Render(e.Recortar(s.Titulo, ancho)), e.Recortar(filtro, ancho), ""}
	return strings.Join(append(cabecera, s.lista.Vista(ancho, alto-len(cabecera))), "\n")
}
