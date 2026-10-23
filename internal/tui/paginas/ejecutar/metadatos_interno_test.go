package ejecutar

import (
	"slices"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

// TestMetadatosDelIdioma: bajo la respuesta, el coste y la duración van con la coma decimal en español.
func TestMetadatosDelIdioma(t *testing.T) {
	inicio := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	fin := inicio.Add(4200 * time.Millisecond)
	tr := nucleo.Trabajo{CreadoEn: inicio, TerminadoEn: &fin, Uso: nucleo.Uso{CostoUSD: 0.0008}}
	for lengua, quiere := range map[string][]string{"es": {"US$ 0,0008", "4,2 s"}, "en": {"US$ 0.0008", "4.2 s"}} {
		partes := metadatos(lengua, tr, "tokens")
		for _, q := range quiere {
			if !slices.Contains(partes, q) {
				t.Errorf("%s: falta %q en %q", lengua, q, partes)
			}
		}
	}
}
