//go:build linux

package autoarranque

import (
	"os"
	"strings"
	"testing"
)

// TestElegirServicioLinux: «Solo el servicio» arranca «tramtai servicio iniciar -d», con la ruta entrecomillada, en lugar
// de la app con --oculto.
func TestElegirServicioLinux(t *testing.T) {
	ruta := conCopia(t, "/opt/con espacios/tramtai-escritorio")
	if err := Elegir(ModoServicio); err != nil {
		t.Fatal(err)
	}
	datos, err := os.ReadFile(ruta)
	if err != nil || !strings.Contains(string(datos), "\nExec=\"/opt/con espacios/tramtai\" servicio iniciar -d\n") || strings.Contains(string(datos), "--oculto") {
		t.Fatalf("registro de arranque: %s %v", datos, err)
	}
}
