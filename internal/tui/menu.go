package tui

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

type grupoMenu struct {
	ID      string
	Paginas []string
}

// gruposMenu son los de web/src/menu.ts, en su orden (TestMenuComoLaWeb lo comprueba). Las claves son las de la Web UI.
var gruposMenu = []grupoMenu{
	{"empezar", []string{pagina.PaginaInicio, pagina.PaginaAgentes, pagina.PaginaEjecutar}},
	{"conectar", []string{pagina.PaginaConectar, pagina.PaginaHistorial}},
	{"organizar", []string{pagina.PaginaEspacios, pagina.PaginaConfiguracion}},
}

const (
	anchoMenu       = 22  // la columna del menú, con su separador
	anchoMenuFijo   = 100 // desde aquí el menú se ve siempre; por debajo se abre con Tab
	paginasConAtajo = 7   // del 1 al 7
)

// ordenMenu son las páginas de arriba abajo, con la Ayuda al final.
func ordenMenu() []string {
	var r []string
	for _, g := range gruposMenu {
		r = append(r, g.Paginas...)
	}
	return append(r, pagina.PaginaAyuda)
}

// atajo es la página de una tecla del 1 al 7.
func atajo(tecla string) (string, bool) {
	n, err := strconv.Atoi(tecla)
	if err != nil || len(tecla) != 1 || n < 1 || n > paginasConAtajo {
		return "", false
	}
	return ordenMenu()[n-1], true
}

func (m *modelo) vistaMenu() string {
	e := m.estilos
	var lineas []string
	n := 0
	for _, g := range gruposMenu {
		lineas = append(lineas, e.Grupo.Render(e.Recortar(datos.Texto(m.idioma, "menu.grupo."+g.ID), anchoMenu-2)))
		for _, p := range g.Paginas {
			n++
			lineas = append(lineas, m.entradaMenu(p, strconv.Itoa(n)))
		}
		lineas = append(lineas, "")
	}
	return strings.Join(append(lineas, m.entradaMenu(pagina.PaginaAyuda, "?")), "\n")
}

func (m *modelo) entradaMenu(p, tecla string) string {
	e := m.estilos
	marca := "  "
	if p == m.actual {
		marca = e.Iconos.Cursor + " "
	}
	linea := e.Recortar(marca+tecla+" "+datos.Texto(m.idioma, "menu."+p), anchoMenu-2)
	switch {
	case m.enMenu && ordenMenu()[m.cursorMenu] == p:
		return e.Seleccion.Render(linea + strings.Repeat(" ", anchoMenu-2-lipgloss.Width(linea)))
	case p == m.actual:
		return e.Acento.Render(linea)
	}
	return e.Texto.Render(linea)
}

// abrirMenu le da el foco al menú, con el cursor en la página actual.
func (m *modelo) abrirMenu() {
	m.enMenu, m.cursorMenu = true, max(slices.Index(ordenMenu(), m.actual), 0)
}

func (m *modelo) teclaMenu(tecla string) tea.Cmd {
	orden := ordenMenu()
	switch tecla {
	case "up", "k":
		m.cursorMenu = max(m.cursorMenu-1, 0)
	case "down", "j":
		m.cursorMenu = min(m.cursorMenu+1, len(orden)-1)
	case "enter", "space":
		return m.ir(orden[m.cursorMenu], nil, false)
	case "esc", "tab":
		m.enMenu = false
	case "q":
		return tea.Quit
	default:
		if p, ok := atajo(tecla); ok {
			return m.ir(p, nil, false)
		}
	}
	return nil
}
