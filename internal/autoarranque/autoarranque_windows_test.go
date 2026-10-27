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
	if err := activar(ruta, ModoApp); err != nil {
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
	escribir(nombreValor, orden(ruta, ModoApp))
	if activo, _ := Activo(); !activo {
		t.Fatal("el valor de una portable anterior a la 0.48 cuenta como activo")
	}
	if err := activar(ruta, ModoApp); err != nil {
		t.Fatal(err)
	}
	if leer(nombrePortable) != orden(ruta, ModoApp) || leer(nombreValor) != "" {
		t.Fatalf("migración: portable=%q instalada=%q", leer(nombrePortable), leer(nombreValor))
	}
	if err := Desactivar(); err != nil || leer(nombrePortable) != "" {
		t.Fatalf("Desactivar en la portable: %v", err)
	}
}

// Cada copia tiene su valor: una portable en «Solo el servicio» no pisa la app de la instalada, ni al revés, y quitar el
// de una deja el de la otra.
func TestPortableEnServicioNoPisaLaInstalada(t *testing.T) {
	dir := t.TempDir()
	instalada, portable := filepath.Join(dir, "instalada", "tramtai-escritorio.exe"), filepath.Join(dir, "portable", "tramtai-escritorio.exe")
	conCopia(t, instalada, false)
	como := func(ruta string, esPortableAhora bool) {
		esPortable = func() bool { return esPortableAhora }
		propia = func() (string, error) { return ruta, nil }
	}
	esperar := func(quiero Modo, donde string) {
		t.Helper()
		if modo, err := Configurado(); err != nil || modo != quiero {
			t.Fatalf("%s: Configurado = %q %v; se esperaba %q", donde, modo, err, quiero)
		}
	}
	if err := Elegir(ModoApp); err != nil {
		t.Fatal(err)
	}
	como(portable, true)
	if err := Elegir(ModoServicio); err != nil {
		t.Fatal(err)
	}
	esperar(ModoServicio, "la portable")
	como(instalada, false)
	esperar(ModoApp, "la instalada tras elegir en la portable")
	como(portable, true)
	if err := Desactivar(); err != nil {
		t.Fatal(err)
	}
	esperar("", "la portable desactivada")
	como(instalada, false)
	esperar(ModoApp, "la instalada tras desactivar la portable")
}

// TestElegirServicioWindows: «Solo el servicio» registra la app de escritorio con --solo-servicio (no abre ventana ni
// bandeja) en lugar de --oculto, y sustituye al modo anterior en el mismo valor.
func TestElegirServicioWindows(t *testing.T) {
	ruta := `C:\ruta con espacios\tramtai-escritorio.exe`
	conCopia(t, ruta, false)
	if err := Elegir(ModoApp); err != nil || leer(nombreValor) != `"C:\ruta con espacios\tramtai-escritorio.exe" --oculto` {
		t.Fatalf("app: %q %v", leer(nombreValor), err)
	}
	if err := Elegir(ModoServicio); err != nil {
		t.Fatal(err)
	}
	if valor := leer(nombreValor); valor != `"C:\ruta con espacios\tramtai-escritorio.exe" --solo-servicio` {
		t.Fatalf("valor registrado: %s", valor)
	}
}

// TestConfiguradoDistingueModos: Configurado dice el modo de esta copia y "" si no hay nada suyo; un modo que no existe
// no se registra.
func TestConfiguradoDistingueModos(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "tramtai-escritorio.exe")
	if err := os.WriteFile(ruta, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	conCopia(t, ruta, false)
	esperar := func(quiero Modo) {
		t.Helper()
		if modo, err := Configurado(); err != nil || modo != quiero {
			t.Fatalf("Configurado = %q %v; se esperaba %q", modo, err, quiero)
		}
		if activo, _ := Activo(); activo != (quiero != "") {
			t.Fatalf("Activo = %v con el modo %q", activo, quiero)
		}
	}
	esperar("")
	_ = activar(ruta, ModoApp)
	esperar(ModoApp)
	_ = activar(ruta, ModoServicio)
	esperar(ModoServicio)
	escribir(nombreValor, `"`+ruta+`"`) // sin argumentos, como lo dejaba una versión anterior
	esperar(ModoApp)
	escribir(nombreValor, `"D:\Otra\tramtai-escritorio.exe" --solo-servicio`)
	esperar("")
	if err := Elegir("otro"); err == nil {
		t.Fatal("un modo desconocido debe fallar")
	}
}
