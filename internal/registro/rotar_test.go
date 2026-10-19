package registro

import (
	"os"
	"strings"
	"testing"
)

// Si el archivo nuevo no se puede abrir al rotar (aquí es de solo lectura), el registro sigue en la copia y lo anota:
// no se pierde en silencio.
func TestRotarSinPoderAbrir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root escribe en archivos de solo lectura")
	}
	ruta := t.TempDir() + "/servicio.log"
	r, err := Abrir(ruta, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.Write([]byte("primera línea\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ruta, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ruta, 0o644) })
	if _, err := r.Write([]byte("segunda línea\n")); err != nil {
		t.Fatalf("el registro dejó de escribir: %v", err)
	}
	datos, _ := os.ReadFile(ruta + ".1")
	if !strings.Contains(string(datos), "no se pudo rotar") || !strings.Contains(string(datos), "segunda línea") {
		t.Fatalf("la copia debía tener el aviso y la línea: %q", datos)
	}
}
