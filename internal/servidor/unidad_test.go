package servidor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// iguales compara la unidad generada con la de testdata, byte a byte.
func iguales(t *testing.T, archivo, generada string) {
	t.Helper()
	esperada, err := os.ReadFile(filepath.Join("testdata", archivo))
	if err != nil {
		t.Fatal(err)
	}
	if generada != string(esperada) {
		t.Errorf("la unidad no es la de testdata/%s:\n--- generada\n%s\n--- esperada\n%s", archivo, generada, esperada)
	}
}

// TestUnidadSystemd: la unidad de spec §2, con el plazo de parada del drenaje predeterminado (30 s) más 30.
func TestUnidadSystemd(t *testing.T) {
	iguales(t, "tramtai.service", Unidad(UnidadOpciones{}))
	if u := Unidad(UnidadOpciones{PlazoDrenaje: 90}); !strings.Contains(u, "\nTimeoutStopSec=120\n") {
		t.Errorf("con un plazo de drenaje de 90 s, la parada espera 120 s:\n%s", u)
	}
}

// TestUnidadLetsEncrypt: solo Let's Encrypt o un puerto menor de 1024 dan CAP_NET_BIND_SERVICE.
func TestUnidadLetsEncrypt(t *testing.T) {
	iguales(t, "tramtai-letsencrypt.service", Unidad(UnidadOpciones{LetsEncrypt: true}))
	iguales(t, "tramtai-letsencrypt.service", Unidad(UnidadOpciones{PuertoBajo: true}))
	if strings.Contains(Unidad(UnidadOpciones{}), "AmbientCapabilities") {
		t.Error("sin Let's Encrypt ni puerto bajo, la unidad no da capacidades")
	}
}
