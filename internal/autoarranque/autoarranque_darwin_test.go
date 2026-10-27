//go:build darwin

package autoarranque

import (
	"os"
	"strings"
	"testing"
)

// TestElegirServicioDarwin: «Solo el servicio» arranca «tramtai servicio iniciar -d» en los ProgramArguments, en lugar de
// la app con --oculto.
func TestElegirServicioDarwin(t *testing.T) {
	ruta := conCopia(t, "/opt/con espacios & más/tramtai-escritorio")
	if err := Elegir(ModoServicio); err != nil {
		t.Fatal(err)
	}
	datos, err := os.ReadFile(ruta)
	quiero := "<array>\n\t\t<string>/opt/con espacios &amp; más/tramtai</string>\n\t\t<string>servicio</string>\n\t\t<string>iniciar</string>\n\t\t<string>-d</string>\n\t</array>"
	if err != nil || !strings.Contains(string(datos), quiero) || strings.Contains(string(datos), "--oculto") {
		t.Fatalf("registro de arranque: %s %v", datos, err)
	}
}
