//go:build linux || darwin

package autoarranque

import (
	"os"
	"strings"
	"testing"
)

func TestActivarYDesactivar(t *testing.T) {
	carpeta := t.TempDir()
	t.Setenv("HOME", carpeta)            // macOS: ~/Library/LaunchAgents
	t.Setenv("XDG_CONFIG_HOME", carpeta) // Linux: ~/.config/autostart
	nombreArchivo = "prueba-" + nombreArchivo
	if activo, err := Activo(); err != nil || activo {
		t.Fatalf("antes de activar: %v %v", activo, err)
	}
	if err := Activar("/opt/con espacios & más/tramtai-escritorio"); err != nil {
		t.Fatal(err)
	}
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("después de activar: %v %v", activo, err)
	}
	ruta, _ := archivo()
	if !strings.HasPrefix(ruta, carpeta) {
		t.Fatalf("el registro se escribió fuera de la carpeta del usuario de prueba: %s", ruta)
	}
	datos, err := os.ReadFile(ruta)
	if err != nil || !strings.Contains(string(datos), "tramtai-escritorio") || !strings.Contains(string(datos), "--oculto") {
		t.Fatalf("registro de arranque: %s %v", datos, err)
	}
	if err := Desactivar(); err != nil {
		t.Fatal(err)
	}
	if err := Desactivar(); err != nil { // dos veces no es un error
		t.Fatal(err)
	}
	if activo, _ := Activo(); activo {
		t.Fatal("sigue activo")
	}
}
