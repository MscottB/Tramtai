package nucleo

import (
	"os"
	"path/filepath"
	"testing"
)

// conTemporal sustituye la carpeta temporal del sistema: t.TempDir() está dentro de ella.
func conTemporal(t *testing.T, ruta string) {
	anterior := carpetaTemporal
	carpetaTemporal = func() string { return ruta }
	t.Cleanup(func() { carpetaTemporal = anterior })
}

// ejecutable crea una carpeta con un tramtai.exe falso y, si portable, su portable.txt.
func ejecutable(t *testing.T, portable bool) string {
	t.Helper()
	carpeta := filepath.Join(t.TempDir(), "Tramtai portable")
	os.MkdirAll(carpeta, 0o700)
	exe := filepath.Join(carpeta, "tramtai.exe")
	os.WriteFile(exe, nil, 0o700)
	if portable {
		os.WriteFile(filepath.Join(carpeta, ArchivoPortable), []byte("portable"), 0o600)
	}
	return exe
}

func TestRaizDe(t *testing.T) {
	conTemporal(t, filepath.Join(t.TempDir(), "otra"))
	sistema, _ := os.UserConfigDir()

	r, portable, err := raizDe(ejecutable(t, false))
	if err != nil || portable || r != filepath.Join(sistema, "tramtai") {
		t.Fatalf("sin portable.txt: %s %v %v", r, portable, err)
	}

	exe := ejecutable(t, true)
	r, portable, err = raizDe(exe)
	real, _ := filepath.EvalSymlinks(filepath.Dir(exe))
	if err != nil || !portable || r != filepath.Join(real, CarpetaPortable) {
		t.Fatalf("con portable.txt: %s %v %v", r, portable, err)
	}
	if info, err := os.Stat(r); err != nil || !info.IsDir() {
		t.Fatalf("no se creó %s: %v", r, err)
	}

	// Un portable.txt que es una carpeta no cuenta.
	exe = ejecutable(t, false)
	os.Mkdir(filepath.Join(filepath.Dir(exe), ArchivoPortable), 0o700)
	if _, portable, _ := raizDe(exe); portable {
		t.Fatal("una carpeta portable.txt no hace portable la copia")
	}

	// Si no se puede crear datos-tramtai (aquí es un archivo), error y nunca la carpeta del sistema.
	exe = ejecutable(t, true)
	os.WriteFile(filepath.Join(filepath.Dir(exe), CarpetaPortable), nil, 0o600)
	if r, portable, err := raizDe(exe); err == nil || !portable || r == filepath.Join(sistema, "tramtai") {
		t.Fatalf("sin poder escribir: %s %v %v", r, portable, err)
	}
}

// Abierta desde la carpeta temporal (el Explorador abre ahí un .zip sin descomprimir), la portable no usa esa carpeta.
func TestRaizEnCarpetaTemporal(t *testing.T) {
	exe := ejecutable(t, true)
	conTemporal(t, filepath.Dir(filepath.Dir(exe)))
	if _, portable, err := raizDe(exe); err == nil || !portable {
		t.Fatalf("en la carpeta temporal: %v %v", portable, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), CarpetaPortable)); err == nil {
		t.Fatal("no debía crear datos-tramtai en la carpeta temporal")
	}
}

// Un .exe abierto desde dentro de un .zip (el Explorador lo extrae a %TEMP%\Temp1_….zip\, a veces sin portable.txt)
// no se pone a escribir en ninguna parte.
func TestRaizDesdeDentroDelZip(t *testing.T) {
	temporal := t.TempDir()
	conTemporal(t, temporal)
	carpeta := filepath.Join(temporal, "Temp1_Tramtai-v0.48.0-portable-windows-x64.zip")
	os.MkdirAll(carpeta, 0o700)
	exe := filepath.Join(carpeta, "tramtai-escritorio.exe")
	os.WriteFile(exe, nil, 0o700)
	if _, _, err := raizDe(exe); err == nil {
		t.Fatal("desde dentro de un .zip debe dar error, con o sin portable.txt")
	}
}
