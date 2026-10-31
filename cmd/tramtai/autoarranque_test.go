package main

import (
	"strings"
	"testing"
)

// --modo se valida siempre, también sin argumento o con desactivar: un modo desconocido es un error y no toca el arranque
// del equipo (nunca llega a registrarse ni a quitarse nada).
func TestAutoarranqueModoInvalido(t *testing.T) {
	for _, args := range [][]string{
		{"servicio", "autoarranque", "--modo", "otro"},
		{"servicio", "autoarranque", "desactivar", "--modo", "otro"},
		{"servicio", "autoarranque", "activar", "--modo", "otro"},
	} {
		if _, _, err := ejecutarRaiz(t, t.TempDir(), args...); err == nil {
			t.Errorf("%v debía fallar", args)
		}
	}
}

// TestAutoarranqueEnElServidor: en el servidor no hay arranque al iniciar sesión (el servicio del sistema arranca con el
// equipo): la orden lo dice y no toca el arranque de nadie.
func TestAutoarranqueEnElServidor(t *testing.T) {
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	for _, args := range [][]string{
		{"servicio", "autoarranque"},
		{"servicio", "autoarranque", "activar", "--modo", "servicio"},
		{"servicio", "autoarranque", "desactivar"},
	} {
		if _, _, err := ejecutarRaiz(t, t.TempDir(), args...); err == nil || !strings.Contains(err.Error(), "arranca con el equipo") {
			t.Errorf("%v en el servidor: %v", args, err)
		}
	}
}
