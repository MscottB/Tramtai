package servidor

import (
	"archive/zip"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tramtai/internal/carpetas"
)

// paqueteDePrueba escribe un .zip con esas entradas (nombre → contenido), como el paquete del conversor de Windows.
func paqueteDePrueba(t *testing.T, entradas map[string]string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "conversor.zip")
	f, err := os.Create(ruta)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for nombre, contenido := range entradas {
		e, err := w.Create(nombre)
		if err == nil {
			_, err = e.Write([]byte(contenido))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// TestAbrirRaizNoSigueEnlaces: si la raíz del servidor es un enlace a otra carpeta, abrirRaiz falla en lugar de
// trabajar en la otra.
func TestAbrirRaizNoSigueEnlaces(t *testing.T) {
	base := t.TempDir()
	otra, enlace := filepath.Join(base, "otra"), filepath.Join(base, "raiz")
	if err := os.Mkdir(otra, 0o700); err != nil {
		t.Fatal(err)
	}
	if r, err := abrirRaiz(otra); err != nil {
		t.Fatalf("una carpeta normal: %v", err)
	} else {
		r.Close()
	}
	if runtime.GOOS == "windows" { // un punto de unión no necesita privilegios ni el modo de desarrollador
		union := filepath.Join(base, "union")
		if salida, err := exec.Command("cmd", "/c", "mklink", "/J", union, otra).CombinedOutput(); err != nil {
			t.Fatalf("mklink /J: %v %s", err, salida)
		}
		if r, err := abrirRaiz(union); err == nil {
			r.Close()
			t.Error("un punto de unión como raíz debe ser un error")
		}
	}
	if err := os.Symlink(otra, enlace); err != nil {
		t.Skipf("sin permiso para crear enlaces simbólicos: %v", err)
	}
	if r, err := abrirRaiz(enlace); err == nil {
		r.Close()
		t.Fatal("un enlace como raíz debe ser un error")
	}
}

// TestInstalarConConversor: --conversor descomprime la carpeta conversor del paquete en la raíz del servidor, sin red,
// después de comprobarla fuera de ella (como «Preparar archivos»). Una entrada que sale de la carpeta o una comprobación
// que falla no dejan nada.
func TestInstalarConConversor(t *testing.T) {
	var comprobadas []string
	comprobar0 := comprobarConversor
	t.Cleanup(func() { comprobarConversor = comprobar0 })
	comprobarConversor = func(_ context.Context, dir string) error {
		comprobadas = append(comprobadas, dir)
		if _, err := os.Stat(filepath.Join(dir, "listo.json")); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(dir, "malo")); err == nil {
			return errors.New("no convierte")
		}
		return nil
	}
	abrir := func(t *testing.T) (*os.Root, string) {
		raiz := t.TempDir()
		r, err := os.OpenRoot(raiz)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		return r, raiz
	}

	t.Run("bueno", func(t *testing.T) {
		comprobadas = nil
		r, raiz := abrir(t)
		paquete := paqueteDePrueba(t, map[string]string{"LEEME.txt": "léeme", "conversor/listo.json": "{}",
			"conversor/python/python.exe": "python", `conversor\python\Lib\uno.py`: "uno"})
		cortada := filepath.Join(raiz, "conversor-paquete-CORTADA", "python") // lo que dejó una instalación cortada
		if err := os.MkdirAll(cortada, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := instalarConversor(r, paquete); err != nil {
			t.Fatal(err)
		}
		if entradas, _ := os.ReadDir(raiz); len(entradas) != 1 || entradas[0].Name() != "conversor" {
			t.Errorf("en la raíz solo queda conversor: %v", entradas)
		}
		for nombre, quiere := range map[string]string{"listo.json": "{}", "python/python.exe": "python", "python/Lib/uno.py": "uno"} {
			if datos, err := os.ReadFile(filepath.Join(raiz, "conversor", nombre)); err != nil || string(datos) != quiere {
				t.Errorf("conversor/%s: %q %v", nombre, datos, err)
			}
		}
		if _, err := os.Stat(filepath.Join(raiz, "LEEME.txt")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("solo se descomprime la carpeta conversor: %v", err)
		}
		if len(comprobadas) != 1 || carpetas.Dentro(raiz, comprobadas[0]) {
			t.Errorf("se comprueba una vez, fuera de la raíz del servidor: %q", comprobadas)
		}
		// Otra vez: sustituye al anterior.
		if err := os.WriteFile(filepath.Join(raiz, "conversor", "viejo"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := instalarConversor(r, paquete); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(raiz, "conversor", "viejo")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("el conversor anterior se sustituye entero: %v", err)
		}
		if entradas, _ := os.ReadDir(raiz); len(entradas) != 1 {
			t.Errorf("el anterior apartado no se queda: %v", entradas)
		}
	})

	for nombre, entradas := range map[string]map[string]string{
		"sale de la carpeta": {"conversor/listo.json": "{}", "conversor/../../fuera.txt": "fuera"},
		"no convierte":       {"conversor/listo.json": "{}", "conversor/malo": ""},
		"sin conversor":      {"LEEME.txt": "léeme"},
	} {
		t.Run(nombre, func(t *testing.T) {
			r, raiz := abrir(t)
			err := instalarConversor(r, paqueteDePrueba(t, entradas))
			if err == nil {
				t.Fatal("debe fallar")
			}
			if entradas, _ := os.ReadDir(raiz); len(entradas) > 0 {
				t.Errorf("no deja nada en la raíz: %v", entradas)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(raiz), "fuera.txt")); err == nil {
				t.Error("escribió fuera de la raíz")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Error("el error dice algo")
			}
		})
	}
}
