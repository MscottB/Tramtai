package configuracion

import (
	"os"
	"path/filepath"
	"testing"
	"tramtai/internal/nucleo"
)

func TestRutaPermitida(t *testing.T) {
	base := t.TempDir()
	raiz, otra := filepath.Join(base, "raiz"), filepath.Join(base, "raiz-otra")
	for _, d := range []string{raiz, otra} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dentro, fuera := filepath.Join(raiz, "a.txt"), filepath.Join(otra, "b.txt")
	for _, f := range []string{dentro, fuera} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	libre := Resolver(Capa{Origen: OrigenGlobal, Valores: map[string]any{}})
	if _, err := libre.RutaPermitida(fuera); err != nil {
		t.Fatalf("sin raíces no debe haber límite: %v", err)
	}

	c := Resolver(Capa{Origen: OrigenGlobal, Valores: map[string]any{"instancia_global.raices_permitidas": []any{raiz}}})
	if _, err := c.RutaPermitida(dentro); err != nil {
		t.Fatalf("archivo dentro de la raíz rechazado: %v", err)
	}
	// raiz-otra empieza igual que raiz pero no está debajo de ella.
	if _, err := c.RutaPermitida(fuera); nucleo.CodigoDe(err) != nucleo.CodigoRutaNoPermitida {
		t.Fatalf("archivo fuera de la raíz: se esperaba ruta_no_permitida, llegó %v", err)
	}
	if _, err := c.RutaPermitida(filepath.Join(raiz, "..", "raiz-otra", "b.txt")); nucleo.CodigoDe(err) != nucleo.CodigoRutaNoPermitida {
		t.Fatalf("ruta con .. que sale de la raíz aceptada: %v", err)
	}
	if _, err := c.RutaPermitida(filepath.Join(raiz, "no-existe.txt")); err == nil {
		t.Fatal("un archivo inexistente no debe aceptarse")
	}

	// Un enlace dentro de la raíz que apunta fuera se rechaza.
	enlace := filepath.Join(raiz, "enlace.txt")
	if err := os.Symlink(fuera, enlace); err != nil {
		t.Skip("este sistema no permite crear enlaces simbólicos sin permisos:", err)
	}
	if _, err := c.RutaPermitida(enlace); nucleo.CodigoDe(err) != nucleo.CodigoRutaNoPermitida {
		t.Fatalf("enlace que escapa de la raíz aceptado: %v", err)
	}
}
