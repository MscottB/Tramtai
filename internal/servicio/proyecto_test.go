package servicio

import (
	"os"
	"path/filepath"
	"testing"
	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
)

func TestRutasDeProyecto(t *testing.T) {
	casa, err := os.UserHomeDir()
	if err != nil {
		t.Skip("sin carpeta personal")
	}
	for entrada, esperada := range map[string]string{
		"~":               casa,
		"~/proyectos/app": filepath.Join(casa, "proyectos", "app"),
		"/srv/app":        "/srv/app",
		"~otro/app":       "~otro/app", // solo la carpeta personal propia
	} {
		if r := carpetas.ExpandirCasa(entrada); r != esperada {
			t.Errorf("ExpandirCasa(%q) = %q, se esperaba %q", entrada, r, esperada)
		}
	}

	// Un proyecto alcanzado por un enlace simbólico da la misma raíz (y el mismo espacio) que por su ruta real.
	real := t.TempDir()
	if err := PrepararProyecto(real); err != nil {
		t.Fatal(err)
	}
	enlace := filepath.Join(t.TempDir(), "enlace")
	if err := os.Symlink(real, enlace); err != nil {
		t.Skip("este sistema no permite crear enlaces simbólicos sin permisos:", err)
	}
	desdeEnlace, ok1 := configuracion.BuscarProyecto(enlace)
	directa, ok2 := configuracion.BuscarProyecto(real)
	if !ok1 || !ok2 || desdeEnlace != directa {
		t.Fatalf("la misma carpeta dio raíces distintas: %q y %q", desdeEnlace, directa)
	}
}
