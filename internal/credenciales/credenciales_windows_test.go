//go:build windows

package credenciales

import "testing"

func TestGuardarLeerBorrar(t *testing.T) {
	const nombre = "PRUEBA_TRAMTAI_NO_USAR"
	t.Cleanup(func() { _ = Borrar(nombre) })
	if v, err := Leer(nombre); err != nil || v != "" {
		t.Fatalf("sin guardar: %q %v", v, err)
	}
	if err := Guardar(nombre, "valor-de-prueba"); err != nil {
		t.Fatal(err)
	}
	if v, err := Leer(nombre); err != nil || v != "valor-de-prueba" {
		t.Fatalf("leída: %q %v", v, err)
	}
	if err := Borrar(nombre); err != nil {
		t.Fatal(err)
	}
	if v, _ := Leer(nombre); v != "" {
		t.Fatal("debía estar borrada")
	}
}
