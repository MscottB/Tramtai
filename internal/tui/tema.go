package tui

import (
	"os"

	"charm.land/lipgloss/v2"

	"tramtai/internal/tui/pagina"
)

// nuevosEstilos son los de arranque: tema oscuro hasta que la terminal diga de qué color es su fondo.
func nuevosEstilos() *pagina.Estilos {
	e := &pagina.Estilos{}
	rellenar(e, true)
	return e
}

// rellenar pone en e el tema claro u oscuro. Con NO_COLOR no hay color: se distingue con negrita, tenue e inverso. Con
// TRAMTAI_ASCII=1, o si la consola no es UTF-8, los iconos y los bordes son ASCII. Cambia e por dentro: las páginas ya
// lo tienen.
func rellenar(e *pagina.Estilos, oscuro bool) {
	n := lipgloss.NewStyle
	*e = pagina.Estilos{
		Oscuro: oscuro, Color: os.Getenv("NO_COLOR") == "", ASCII: os.Getenv("TRAMTAI_ASCII") == "1" || !consolaUTF8(),
		Titulo: n().Bold(true), Texto: n(), Tenue: n().Faint(true), Acento: n().Bold(true), Seleccion: n().Reverse(true),
		Exito: n(), Advertencia: n().Bold(true), Error: n().Bold(true), Tecla: n().Bold(true), Grupo: n().Bold(true),
		Borde: lipgloss.RoundedBorder(), Iconos: pagina.IconosUnicode,
	}
	if e.ASCII {
		e.Borde, e.Iconos = lipgloss.ASCIIBorder(), pagina.IconosASCII
	}
	if !e.Color {
		return
	}
	// Los dos temas con contraste AA sobre fondo blanco y sobre fondo negro.
	c := lipgloss.LightDark(oscuro)
	acento := c(lipgloss.Color("#1d4ed8"), lipgloss.Color("#7aa2f7"))
	tenue := c(lipgloss.Color("#57606a"), lipgloss.Color("#9aa5b1"))
	e.Titulo = e.Titulo.Foreground(acento)
	e.Tenue = n().Foreground(tenue)
	e.Acento = e.Acento.Foreground(acento)
	e.Seleccion = n().Background(c(lipgloss.Color("#dbe4ff"), lipgloss.Color("#2b3550"))).Foreground(c(lipgloss.Color("#0b1220"), lipgloss.Color("#e6ecff")))
	e.Exito = n().Foreground(c(lipgloss.Color("#1a7f37"), lipgloss.Color("#7ee2a8")))
	e.Advertencia = n().Foreground(c(lipgloss.Color("#9a6700"), lipgloss.Color("#e3b341")))
	e.Error = n().Bold(true).Foreground(c(lipgloss.Color("#cf222e"), lipgloss.Color("#ff7b72")))
	e.Tecla = e.Tecla.Foreground(acento)
	e.Grupo = n().Bold(true).Foreground(tenue)
}
