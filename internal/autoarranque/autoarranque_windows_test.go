//go:build windows

package autoarranque

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// conCopia hace de la prueba una copia de Tramtai con su aplicación en ruta y nombres de valor propios.
func conCopia(t *testing.T, ruta string, portable bool) {
	t.Helper()
	sufijo := strconv.Itoa(os.Getpid())
	anteriores := []any{nombreValor, nombrePortable, esPortable, propia}
	nombreValor, nombrePortable = "TramtaiPrueba"+sufijo, "TramtaiPruebaPortable"+sufijo
	esPortable = func() bool { return portable }
	propia = func() (string, error) { return ruta, nil }
	t.Cleanup(func() {
		borrar(nombreValor)
		borrar(nombrePortable)
		nombreValor, nombrePortable = anteriores[0].(string), anteriores[1].(string)
		esPortable, propia = anteriores[2].(func() bool), anteriores[3].(func() (string, error))
	})
}

func leer(nombre string) string {
	k, _ := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.QUERY_VALUE)
	defer k.Close()
	valor, _, _ := k.GetStringValue(nombre)
	return valor
}

func escribir(nombre, valor string) {
	k, _, _ := registry.CreateKey(registry.CURRENT_USER, claveRun, registry.SET_VALUE)
	defer k.Close()
	_ = k.SetStringValue(nombre, valor)
}

func borrar(nombre string) {
	k, _ := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.SET_VALUE)
	defer k.Close()
	_ = k.DeleteValue(nombre)
}

func TestActivarYDesactivar(t *testing.T) {
	ruta := `C:\ruta con espacios\tramtai-escritorio.exe`
	conCopia(t, ruta, false)
	if activo, err := Activo(); err != nil || activo {
		t.Fatalf("antes de activar: %v %v", activo, err)
	}
	if err := Activar(ruta); err != nil {
		t.Fatal(err)
	}
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("después de activar: %v %v", activo, err)
	}
	if valor := leer(nombreValor); valor != `"C:\ruta con espacios\tramtai-escritorio.exe" --oculto` {
		t.Fatalf("valor registrado: %s", valor)
	}
	if err := Desactivar(); err != nil {
		t.Fatal(err)
	}
	if err := Desactivar(); err != nil { // dos veces no es un error
		t.Fatal(err)
	}
	if activo, _ := Activo(); activo {
		t.Fatal("sigue activo")
	}
}

// El valor de otra copia no cuenta como activo ni se borra; el de esta cuenta aunque cambien las mayúsculas.
func TestActivoSoloDeEstaCopia(t *testing.T) {
	conCopia(t, `C:\Programas\Tramtai\tramtai-escritorio.exe`, false)
	escribir(nombreValor, `"D:\Portable\Tramtai\tramtai-escritorio.exe" --oculto`)
	if activo, _ := Activo(); activo {
		t.Fatal("el arranque de otra copia se vio como propio")
	}
	if err := Desactivar(); err != nil || leer(nombreValor) == "" {
		t.Fatalf("Desactivar borró el valor de otra copia: %v", err)
	}
	escribir(nombreValor, `"c:\programas\tramtai\TRAMTAI-ESCRITORIO.EXE" --oculto`)
	if activo, _ := Activo(); !activo {
		t.Fatal("con otras mayúsculas sigue siendo esta copia")
	}
}

// La portable usa su propio valor y se lleva el que dejó una portable anterior a la 0.48 (con el nombre de la instalada) para esta copia.
func TestPortableMigraElValorViejo(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "tramtai-escritorio.exe")
	if err := os.WriteFile(ruta, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	conCopia(t, ruta, true)
	escribir(nombreValor, orden(ruta))
	if activo, _ := Activo(); !activo {
		t.Fatal("el valor de una portable anterior a la 0.48 cuenta como activo")
	}
	if err := Activar(ruta); err != nil {
		t.Fatal(err)
	}
	if leer(nombrePortable) != orden(ruta) || leer(nombreValor) != "" {
		t.Fatalf("migración: portable=%q instalada=%q", leer(nombrePortable), leer(nombreValor))
	}
	if err := Desactivar(); err != nil || leer(nombrePortable) != "" {
		t.Fatalf("Desactivar en la portable: %v", err)
	}
}
