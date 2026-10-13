//go:build windows

package autoarranque

import (
	"os"
	"strconv"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestActivarYDesactivar(t *testing.T) {
	nombreValor = "TramtaiPrueba" + strconv.Itoa(os.Getpid())
	t.Cleanup(func() { _ = Desactivar() })
	if activo, err := Activo(); err != nil || activo {
		t.Fatalf("antes de activar: %v %v", activo, err)
	}
	if err := Activar(`C:\ruta con espacios\tramtai-escritorio.exe`); err != nil {
		t.Fatal(err)
	}
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("después de activar: %v %v", activo, err)
	}
	k, _ := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.QUERY_VALUE)
	valor, _, _ := k.GetStringValue(nombreValor)
	k.Close()
	if valor != `"C:\ruta con espacios\tramtai-escritorio.exe" --oculto` {
		t.Fatalf("valor registrado: %s", valor)
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
