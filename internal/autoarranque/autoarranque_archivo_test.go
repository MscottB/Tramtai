//go:build linux || darwin

package autoarranque

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActivarYDesactivar(t *testing.T) {
	carpeta := t.TempDir()
	t.Setenv("HOME", carpeta)            // macOS: ~/Library/LaunchAgents
	t.Setenv("XDG_CONFIG_HOME", carpeta) // Linux: ~/.config/autostart
	nombreAnterior := nombreArchivo
	nombreArchivo = "prueba-" + nombreArchivo
	t.Cleanup(func() { nombreArchivo = nombreAnterior })
	const escritorio = "/opt/con espacios & más/tramtai-escritorio"
	anterior := propia
	propia = func() (string, error) { return escritorio, nil }
	t.Cleanup(func() { propia = anterior })
	if activo, err := Activo(); err != nil || activo {
		t.Fatalf("antes de activar: %v %v", activo, err)
	}
	if err := activar(escritorio, ModoApp); err != nil {
		t.Fatal(err)
	}
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("después de activar: %v %v", activo, err)
	}
	lista, _ := rutas()
	ruta := lista[0]
	if !strings.HasPrefix(ruta, carpeta) {
		t.Fatalf("el registro se escribió fuera de la carpeta del usuario de prueba: %s", ruta)
	}
	datos, err := os.ReadFile(ruta)
	if err != nil || !strings.Contains(string(datos), "tramtai-escritorio") || !strings.Contains(string(datos), "--oculto") {
		t.Fatalf("registro de arranque: %s %v", datos, err)
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

// La instalada y la portable tienen cada una su archivo: activar una no pisa la otra. Una portable anterior que usaba
// el archivo de la instalada lo mueve al suyo al activarse, sin perder el arranque.
func TestInstaladaYPortableNoSePisan(t *testing.T) {
	carpeta := t.TempDir()
	t.Setenv("HOME", carpeta)
	t.Setenv("XDG_CONFIG_HOME", carpeta)
	antes := [2]string{nombreArchivo, nombrePortable}
	nombreArchivo, nombrePortable = "prueba-"+nombreArchivo, "prueba-"+nombrePortable
	anterior, portableAnterior := propia, esPortable
	t.Cleanup(func() {
		nombreArchivo, nombrePortable, propia, esPortable = antes[0], antes[1], anterior, portableAnterior
	})
	como := func(portable bool) {
		esPortable = func() bool { return portable }
		app := "/opt/instalada/tramtai-escritorio"
		if portable {
			app = "/media/usb/tramtai/tramtai-escritorio"
		}
		propia = func() (string, error) { return app, nil }
	}

	// Una portable anterior dejó su arranque con el nombre de la instalada: se reconoce y, al activarla, se mueve.
	como(true)
	lista, _ := rutas()
	os.MkdirAll(filepath.Dir(lista[1]), 0o755)
	os.WriteFile(lista[1], []byte(contenido("/media/usb/tramtai/tramtai-escritorio", ModoApp, filepath.Base(lista[1]))), 0o644)
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("el arranque de una portable anterior es de esta portable: %v %v", activo, err)
	}
	if err := activar("/media/usb/tramtai/tramtai-escritorio", ModoApp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lista[1]); err == nil {
		t.Fatal("el archivo viejo de la portable seguía con el nombre de la instalada")
	}

	como(false)
	if activo, _ := Activo(); activo {
		t.Fatal("el arranque de la portable se vio como de la instalada")
	}
	if err := activar("/opt/instalada/tramtai-escritorio", ModoApp); err != nil {
		t.Fatal(err)
	}
	como(true)
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("activar la instalada pisó la portable: %v %v", activo, err)
	}
	if err := Desactivar(); err != nil {
		t.Fatal(err)
	}
	como(false)
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("desactivar la portable quitó la instalada: %v %v", activo, err)
	}
}

// Cada copia tiene su modo: una portable en «Solo el servicio» no pisa la app de la instalada, ni al revés, y quitar el de
// una deja el de la otra.
func TestPortableEnServicioNoPisaLaInstalada(t *testing.T) {
	carpeta := t.TempDir()
	t.Setenv("HOME", carpeta)
	t.Setenv("XDG_CONFIG_HOME", carpeta)
	antes := [2]string{nombreArchivo, nombrePortable}
	nombreArchivo, nombrePortable = "prueba-"+nombreArchivo, "prueba-"+nombrePortable
	anterior, portableAnterior := propia, esPortable
	t.Cleanup(func() {
		nombreArchivo, nombrePortable, propia, esPortable = antes[0], antes[1], anterior, portableAnterior
	})
	como := func(portable bool, app string) {
		esPortable = func() bool { return portable }
		propia = func() (string, error) { return app, nil }
	}
	esperar := func(quiero Modo, donde string) {
		t.Helper()
		if modo, err := Configurado(); err != nil || modo != quiero {
			t.Fatalf("%s: Configurado = %q %v; se esperaba %q", donde, modo, err, quiero)
		}
	}
	const instalada, portable = "/opt/instalada/tramtai-escritorio", "/media/usb/tramtai/tramtai-escritorio"
	como(false, instalada)
	if err := Elegir(ModoApp); err != nil {
		t.Fatal(err)
	}
	como(true, portable)
	if err := Elegir(ModoServicio); err != nil {
		t.Fatal(err)
	}
	esperar(ModoServicio, "la portable")
	como(false, instalada)
	esperar(ModoApp, "la instalada tras elegir en la portable")
	como(true, portable)
	if err := Desactivar(); err != nil {
		t.Fatal(err)
	}
	esperar("", "la portable desactivada")
	como(false, instalada)
	esperar(ModoApp, "la instalada tras desactivar la portable")
}

// Activar la instalada no pisa el arranque que una portable anterior dejó con el nombre de la instalada: pasa al nombre
// de la portable y esa portable lo sigue reconociendo como suyo (hallazgo 19).
func TestInstaladaApartaLaPortableAnterior(t *testing.T) {
	carpeta := t.TempDir()
	t.Setenv("HOME", carpeta)
	t.Setenv("XDG_CONFIG_HOME", carpeta)
	antes := [2]string{nombreArchivo, nombrePortable}
	nombreArchivo, nombrePortable = "prueba-aparta-"+nombreArchivo, "prueba-aparta-"+nombrePortable
	anterior, portableAnterior := propia, esPortable
	t.Cleanup(func() {
		nombreArchivo, nombrePortable, propia, esPortable = antes[0], antes[1], anterior, portableAnterior
	})
	const usb, instalada = "/media/usb/tramtai/tramtai-escritorio", "/opt/instalada/tramtai-escritorio"
	esPortable = func() bool { return false }
	propia = func() (string, error) { return instalada, nil }
	lista, _ := rutas()
	os.MkdirAll(filepath.Dir(lista[0]), 0o755)
	os.WriteFile(lista[0], []byte(contenido(usb, ModoApp, nombreArchivo)), 0o644)
	if err := activar(instalada, ModoApp); err != nil {
		t.Fatal(err)
	}
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("la instalada quedó activa: %v %v", activo, err)
	}
	esPortable = func() bool { return true }
	propia = func() (string, error) { return usb, nil }
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("la portable anterior perdió su arranque: %v %v", activo, err)
	}
}

// El arranque de otra copia no cuenta como activo ni se borra.
func TestActivoSoloDeEstaCopia(t *testing.T) {
	carpeta := t.TempDir()
	t.Setenv("HOME", carpeta)
	t.Setenv("XDG_CONFIG_HOME", carpeta)
	nombreAnterior := nombreArchivo
	nombreArchivo = "prueba-otra-" + nombreArchivo
	t.Cleanup(func() { nombreArchivo = nombreAnterior })
	anterior := propia
	propia = func() (string, error) { return "/opt/una/tramtai-escritorio", nil }
	t.Cleanup(func() { propia = anterior })
	// Una ruta que contiene a la propia («…-escritorio-dev») tampoco es esta copia.
	if err := activar("/opt/una/tramtai-escritorio-dev", ModoApp); err != nil {
		t.Fatal(err)
	}
	if activo, _ := Activo(); activo {
		t.Fatal("el arranque de otra copia se vio como propio")
	}
	if err := Desactivar(); err != nil {
		t.Fatal(err)
	}
	if lista, _ := rutas(); func() bool { _, err := os.Stat(lista[0]); return err != nil }() {
		t.Fatal("Desactivar borró el arranque de otra copia")
	}
}

// conCopia hace de la prueba una copia de Tramtai con su aplicación de escritorio en escritorio, con el arranque en una
// carpeta de usuario temporal, y devuelve la ruta del archivo de arranque.
func conCopia(t *testing.T, escritorio string) string {
	t.Helper()
	carpeta := t.TempDir()
	t.Setenv("HOME", carpeta)
	t.Setenv("XDG_CONFIG_HOME", carpeta)
	nombreAnterior, anterior := nombreArchivo, propia
	nombreArchivo = "prueba-modo-" + nombreArchivo
	propia = func() (string, error) { return escritorio, nil }
	t.Cleanup(func() { nombreArchivo, propia = nombreAnterior, anterior })
	lista, _ := rutas()
	return lista[0]
}

// Elegir deja un solo registro y lo cambia de modo; Configurado distingue el modo y no ve el de otra copia.
func TestConfiguradoDistingueModos(t *testing.T) {
	ruta := conCopia(t, "/opt/con espacios/tramtai-escritorio")
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
	for _, modo := range []Modo{ModoApp, ModoServicio, ModoApp} {
		if err := Elegir(modo); err != nil {
			t.Fatal(err)
		}
		esperar(modo)
	}
	if err := Elegir("otro"); err == nil {
		t.Fatal("un modo desconocido debe fallar")
	}
	otra := contenido("/opt/otra/tramtai-escritorio", ModoServicio, filepath.Base(ruta))
	if err := os.WriteFile(ruta, []byte(otra), 0o644); err != nil {
		t.Fatal(err)
	}
	esperar("")
	if err := Desactivar(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Fatal("Desactivar borró el arranque de otra copia")
	}
}
