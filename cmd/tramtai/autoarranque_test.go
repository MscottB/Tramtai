package main

import "testing"

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
