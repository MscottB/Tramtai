package comp

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestTablaAlineaConTildes(t *testing.T) {
	filas := [][]string{{"Configuración", "activa"}, {"mi-app", "detenida"}}
	lineas := strings.Split(ansi.Strip(Tabla(estilos(), 40, []string{"Nombre", "Estado"}, filas)), "\n")
	if len(lineas) != 3 {
		t.Fatalf("cabecera y dos filas: %q", lineas)
	}
	columna := func(l, celda string) int { return lipgloss.Width(l[:strings.Index(l, celda)]) }
	if a, b, c := columna(lineas[0], "Estado"), columna(lineas[1], "activa"), columna(lineas[2], "detenida"); a != b || b != c {
		t.Fatalf("la segunda columna empieza en %d, %d y %d", a, b, c)
	}
	estrecha := strings.Split(ansi.Strip(Tabla(estilos(), 14, []string{"Nombre", "Estado"}, filas)), "\n")
	for _, l := range estrecha {
		if lipgloss.Width(l) > 14 {
			t.Fatalf("en 14 columnas una línea mide %d: %q", lipgloss.Width(l), l)
		}
	}
	if !strings.HasPrefix(estrecha[1], "Conf") || !strings.Contains(estrecha[1], "…") {
		t.Fatalf("lo que no cabe se recorta con …: %q", estrecha[1])
	}
}
