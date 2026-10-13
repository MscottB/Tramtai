package registro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotaYConservaCopias(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "servicio.log")
	r, err := Abrir(ruta, 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	linea := strings.Repeat("x", 39) + "\n" // 40 bytes: caben 2 por archivo
	for i := 0; i < 10; i++ {
		if _, err := r.Write([]byte(linea)); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()
	for _, nombre := range []string{ruta, ruta + ".1", ruta + ".2"} {
		info, err := os.Stat(nombre)
		if err != nil || info.Size() > 100 || info.Size() == 0 {
			t.Fatalf("%s: %v %v", nombre, info, err)
		}
	}
	if _, err := os.Stat(ruta + ".3"); !os.IsNotExist(err) {
		t.Fatal("solo se conservan 2 copias")
	}
}

func TestSinCopiasSeVacia(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "servicio.log")
	r, _ := Abrir(ruta, 50, 0)
	for i := 0; i < 5; i++ {
		r.Write([]byte(strings.Repeat("y", 29) + "\n"))
	}
	r.Close()
	if info, _ := os.Stat(ruta); info.Size() > 50 {
		t.Fatalf("el registro supera el máximo: %d", info.Size())
	}
}
