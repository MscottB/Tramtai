package servidor

import (
	"archive/zip"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
		if err := instalarConversor(r, paquete); err != nil {
			t.Fatal(err)
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
