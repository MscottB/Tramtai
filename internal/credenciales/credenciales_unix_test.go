//go:build darwin || linux

package credenciales

import "testing"

// Se ejecuta en el CI de macOS y Linux; se salta donde no hay llavero utilizable (sin secret-tool o sin sesión).
func TestGuardarLeerBorrarUnix(t *testing.T) {
	if !Disponible() {
		t.Skipf("%s no está disponible aquí", Almacen())
	}
	const nombre = "PRUEBA_TRAMTAI_NO_USAR"
	t.Cleanup(func() { _ = Borrar(nombre) })
	if err := Guardar(nombre, "valor-de-prueba_123"); err != nil {
		t.Skipf("%s no se puede usar aquí: %v", Almacen(), err)
	}
	if v, err := Leer(nombre); err != nil || v != "valor-de-prueba_123" {
		t.Fatalf("leída: %q %v", v, err)
	}
	if err := Guardar(nombre, "valor-nuevo"); err != nil {
		t.Fatalf("sustituir: %v", err)
	}
	if v, _ := Leer(nombre); v != "valor-nuevo" {
		t.Fatalf("debía sustituirse: %q", v)
	}
	if err := Borrar(nombre); err != nil {
		t.Fatal(err)
	}
	if v, _ := Leer(nombre); v != "" {
		t.Fatal("debía estar borrada")
	}
	if err := Borrar(nombre); err != nil {
		t.Fatalf("borrar una clave que no existe no es un error: %v", err)
	}
}
