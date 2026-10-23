package servicio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
)

// Si falta un tramo intermedio de la clave, QuitarValor no toca nada: no puede borrar una clave de otro nivel que se
// llame igual que el resto del camino.
func TestQuitarValorSinTramoIntermedio(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "local.yaml")
	original := "retencion:\n  horas_resultado: 5\n"
	os.WriteFile(ruta, []byte(original), 0o600)
	if err := QuitarValor(ruta, nucleo.OrigenLocal, "servicio.retencion.horas_resultado"); err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); !strings.Contains(string(datos), "horas_resultado: 5") {
		t.Fatalf("se borró retencion.horas_resultado al quitar servicio.retencion.horas_resultado:\n%s", datos)
	}
	if err := QuitarValor(ruta, nucleo.OrigenLocal, "retencion.horas_resultado"); err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); strings.Contains(string(datos), "horas_resultado") {
		t.Fatal("la clave que sí está se tenía que quitar")
	}
}
