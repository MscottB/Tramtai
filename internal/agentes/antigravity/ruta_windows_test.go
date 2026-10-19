package antigravity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Con la carpeta temporal en una ruta con espacios (un usuario «Ana María»), la orden del hook no lleva espacios y cmd la
// ejecuta: el hook que niega las herramientas llega a correr.
func TestHookEnCarpetaConEspacios(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Ana María", "con espacios")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	guion := filepath.Join(dir, "denegar.cmd")
	if err := os.WriteFile(guion, []byte("@echo {\"decision\":\"deny\"}\r\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	orden, err := ordenDelHook(guion)
	if err != nil {
		if rutaCorta(guion) == guion {
			t.Skip("este volumen no tiene nombres cortos: ordenDelHook se niega, que es lo seguro")
		}
		t.Fatal(err)
	}
	if strings.Contains(orden, " ") {
		t.Fatalf("la orden lleva espacios: %s", orden)
	}
	salida, err := exec.Command("cmd", "/c", orden).Output()
	if err != nil || !strings.Contains(string(salida), `"deny"`) {
		t.Fatalf("cmd no ejecutó el hook: %v %q", err, salida)
	}
}
