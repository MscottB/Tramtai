//go:build darwin || linux

package credenciales

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// En el servicio del sistema de Linux, systemd entrega las claves en $CREDENTIALS_DIRECTORY/tramtai.<NOMBRE>: Leer las
// toma de ahí sin preguntar a Secret Service (sin secret-tool en el PATH) y el almacén se llama como corresponde.
func TestLeerCredentialsDirectory(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("las credenciales de systemd son de Linux")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tramtai.ANTHROPIC_API_KEY"), []byte("sk-prueba-123"), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", dir)
	t.Setenv("PATH", t.TempDir()) // sin secret-tool: si se le preguntara, no habría clave
	if v, err := Leer("ANTHROPIC_API_KEY"); err != nil || v != "sk-prueba-123" {
		t.Fatalf("leída: %q %v", v, err)
	}
	if v, err := Leer("OPENAI_API_KEY"); err != nil || v != "" {
		t.Fatalf("una clave que systemd no entregó: %q %v", v, err)
	}
	if Almacen() != "credenciales de systemd" {
		t.Fatalf("almacén: %q", Almacen())
	}
}

// Se ejecuta en el CI de macOS y Linux; se salta donde no hay llavero utilizable (sin secret-tool o sin sesión).
func TestGuardarLeerBorrarUnix(t *testing.T) {
	if !Disponible() {
		t.Skipf("%s no está disponible aquí", Almacen())
	}
	const nombre = "PRUEBA_TRAMTAI_NO_USAR"
	antes := Nombres // una entrada propia de la prueba, nunca una clave real: se admite solo mientras dura
	Nombres = append(slices.Clone(Nombres), nombre)
	t.Cleanup(func() { Nombres = antes })
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
