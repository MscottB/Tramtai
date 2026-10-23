package tui

import (
	"os"

	"charm.land/lipgloss/v2"

	"tramtai/internal/tui/pagina"
)

// tokens son los colores de docs/diseno/DESIGN.md que usa la terminal, con sus nombres (TestColoresDeDesign los compara).
var tokens = map[string]string{
	"acento": "#0b6e64", "acento-noche": "#2dd4bf",
	"texto": "#0f1b1e", "texto-noche": "#e5edf0",
	"texto-suave": "#55666a", "texto-suave-noche": "#93a5ac",
	"ok": "#166534", "ok-noche": "#4ade80",
	"error": "#b42525", "error-noche": "#f87171",
	"aviso": "#854d0e", "aviso-noche": "#fbbf24",
	"linea-otro": "#64748b", "linea-otro-noche": "#8a9aa8",
}

// nuevosEstilos son los de arranque: tema oscuro hasta que la terminal diga de qué color es su fondo.
func nuevosEstilos() *pagina.Estilos {
	e := &pagina.Estilos{}
	rellenar(e, true)
	return e
}

// rellenar pone en e el tema claro u oscuro de DESIGN.md. Con NO_COLOR no hay color: se distingue con negrita, tenue e
// inverso, y los campos con su marca. Con TRAMTAI_ASCII=1, o si la consola no es UTF-8, los iconos y los bordes son ASCII.
// Cambia e por dentro: las páginas ya lo tienen.
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
	// token es el color de día o su par de noche.
	token := func(nombre string) lipgloss.Style {
		if oscuro {
			nombre += "-noche"
		}
		return n().Foreground(lipgloss.Color(tokens[nombre]))
	}
	acento, tenue := token("acento"), token("texto-suave")
	e.Titulo = acento.Bold(true)
	e.Tenue = tenue
	e.Acento = acento.Bold(true)
	// La fila elegida lleva el velo del acento. DESIGN.md lo pone al 9 % (11 % de noche); en muchas paletas de terminal
	// eso no se distingue del fondo, así que aquí va al 18 % (20 % de noche), con la tinta del texto encima.
	velo := lipgloss.LightDark(oscuro)(lipgloss.Color("#d3e5e3"), lipgloss.Color("#113838"))
	e.Seleccion = token("texto").Background(velo)
	e.Exito = token("ok")
	e.Advertencia = token("aviso")
	e.Error = token("error").Bold(true)
	e.Tecla = acento.Bold(true)
	e.Grupo = tenue.Bold(true)
	e.LineaOtra = [2]string{tokens["linea-otro"], tokens["linea-otro-noche"]}
}
