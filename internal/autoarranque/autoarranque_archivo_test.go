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
	if err := Activar(escritorio); err != nil {
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
	os.WriteFile(lista[1], []byte(contenido("/media/usb/tramtai/tramtai-escritorio", filepath.Base(lista[1]))), 0o644)
	if activo, err := Activo(); err != nil || !activo {
		t.Fatalf("el arranque de una portable anterior es de esta portable: %v %v", activo, err)
	}
	if err := Activar("/media/usb/tramtai/tramtai-escritorio"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lista[1]); err == nil {
		t.Fatal("el archivo viejo de la portable seguía con el nombre de la instalada")
	}

	como(false)
	if activo, _ := Activo(); activo {
		t.Fatal("el arranque de la portable se vio como de la instalada")
	}
	if err := Activar("/opt/instalada/tramtai-escritorio"); err != nil {
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
	if err := Activar("/opt/una/tramtai-escritorio-dev"); err != nil {
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
