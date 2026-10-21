package comp

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"tramtai/internal/tui/pagina"
)

// Tabla pinta una cabecera y sus filas en columnas alineadas por el ancho que se ve (las tildes cuentan una). Si no cabe
// en ancho, encoge la columna más ancha y recorta con elipsis lo que no quepa.
func Tabla(e *pagina.Estilos, ancho int, cabecera []string, filas [][]string) string {
	anchos := make([]int, len(cabecera))
	for _, f := range append([][]string{cabecera}, filas...) {
		for i := range min(len(f), len(anchos)) {
			anchos[i] = max(anchos[i], lipgloss.Width(f[i]))
		}
	}
	const hueco = 2
	for total := sumar(anchos) + hueco*(len(anchos)-1); total > ancho; total-- {
		i := slices.Index(anchos, slices.Max(anchos))
		if anchos[i] <= 1 {
			break
		}
		anchos[i]--
	}
	pintar := func(f []string, estilo lipgloss.Style) string {
		var celdas []string
		for i, a := range anchos {
			celda := ""
			if i < len(f) {
				celda = e.Recortar(f[i], a)
			}
			celdas = append(celdas, celda+strings.Repeat(" ", a-lipgloss.Width(celda)))
		}
		return estilo.Render(strings.TrimRight(strings.Join(celdas, strings.Repeat(" ", hueco)), " "))
	}
	lineas := []string{pintar(cabecera, e.Tenue.Bold(true))}
	for _, f := range filas {
		lineas = append(lineas, pintar(f, e.Texto))
	}
	return strings.Join(lineas, "\n")
}

func sumar(ns []int) (s int) {
	for _, n := range ns {
		s += n
	}
	return s
}
