//go:build windows

package credenciales

import (
	"slices"
	"testing"
)

func TestGuardarLeerBorrar(t *testing.T) {
	const nombre = "PRUEBA_TRAMTAI_NO_USAR"
	antes := Nombres // una entrada propia de la prueba, nunca una clave real: se admite solo mientras dura
	Nombres = append(slices.Clone(Nombres), nombre)
	t.Cleanup(func() { Nombres = antes })
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
