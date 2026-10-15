//go:build linux

package autoarranque

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActivarYDesactivar(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	if activo, err := Activo(); err != nil || activo {
		t.Fatalf("antes de activar: %v %v", activo, err)
	}
	if err := Activar("/opt/con espacios/tramtai-escritorio"); err != nil {
		t.Fatal(err)
	}
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("después de activar: %v %v", activo, err)
	}
	datos, err := os.ReadFile(filepath.Join(config, "autostart", nombreArchivo))
	if err != nil || !strings.Contains(string(datos), `Exec="/opt/con espacios/tramtai-escritorio" --oculto`) {
		t.Fatalf("entrada de arranque: %s %v", datos, err)
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
