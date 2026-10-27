package carpetas

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

// reiniciarRaiz deja la raíz sin calcular y sin modo servidor: la raíz se calcula una vez por proceso, y estas pruebas
// la calculan varias veces.
func reiniciarRaiz(t *testing.T) {
	t.Helper()
	t.Setenv("TRAMTAI_SERVIDOR", "")
	restaurar := func() {
		raiz = sync.OnceValues(calcularRaiz)
		modoServidor.Store(false)
		calculadaComo.Store(sinCalcular)
	}
	restaurar()
	t.Cleanup(restaurar)
}

// conBaseServidor sustituye la carpeta del servidor por una temporal: las pruebas no tocan /var/lib ni %ProgramData%.
func conBaseServidor(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "servidor")
	anterior := raizServidor
	raizServidor = func() string { return base }
	t.Cleanup(func() { raizServidor = anterior })
	return base
}

// En modo servidor la raíz es la del servidor, tal cual: no se crea ni se protege (lo hace la instalación) y no es
// portable, aunque el ejecutable tenga portable.txt a su lado.
func TestRaizDeServidor(t *testing.T) {
	base := conBaseServidor(t)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	r, portable, err := raizDe(ejecutable(t, true))
	if err != nil || portable || r != base {
		t.Fatalf("en modo servidor: %s %v %v", r, portable, err)
	}
	if _, err := os.Stat(base); err == nil {
		t.Fatal("raizDe no crea la carpeta del servidor")
	}
}

// DirDatos en modo servidor no crea la raíz (con permisos de herencia, antes de que la instalación la proteja): solo
// datos/ dentro de una raíz que ya existe.
func TestDirDatosServidorNoCreaLaRaiz(t *testing.T) {
	reiniciarRaiz(t)
	base := conBaseServidor(t)
	UsarServidor()
	if datos, err := DirDatos(); err == nil {
		t.Fatalf("sin la raíz del servidor no hay datos: %s", datos)
	}
	if _, err := os.Stat(base); err == nil {
		t.Fatal("DirDatos creó la raíz del servidor")
	}
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	datos, err := DirDatos()
	if err != nil || datos != filepath.Join(base, "datos") {
		t.Fatalf("con la raíz instalada: %s %v", datos, err)
	}
	if _, err := DirDatos(); err != nil {
		t.Fatalf("datos/ ya existe: %v", err)
	}
}

func TestRaizServidorPorSistema(t *testing.T) {
	esperada := "/var/lib/tramtai"
	if runtime.GOOS == "windows" {
		esperada = filepath.Join(os.Getenv("ProgramData"), "Tramtai")
	}
	if got := RaizServidor(); got != esperada {
		t.Fatalf("RaizServidor() = %q, se esperaba %q", got, esperada)
	}
}

// TestRaizServidor: con TRAMTAI_SERVIDOR=1 la raíz de un proceso es RaizServidor(). Se comprueba en un proceso hijo,
// porque la raíz se calcula una vez por proceso y el de las pruebas ya pudo calcularla.
func TestRaizServidor(t *testing.T) {
	if os.Getenv("TRAMTAI_PRUEBA_HIJO") == "1" {
		r, err := RaizUsuario()
		if err != nil || r != RaizServidor() || !Servidor() || Portable() || Entorno() != "servidor" {
			fmt.Printf("MAL %q %v %v %v %q\n", r, err, Servidor(), Portable(), Entorno())
			return
		}
		fmt.Println("BIEN")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRaizServidor$")
	cmd.Env = append(os.Environ(), "TRAMTAI_PRUEBA_HIJO=1", "TRAMTAI_SERVIDOR=1")
	salida, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(salida), "BIEN") {
		t.Fatalf("en el hijo con TRAMTAI_SERVIDOR=1: %v\n%s", err, salida)
	}
}

// UsarServidor antes de calcular la raíz la cambia a la del servidor.
func TestUsarServidor(t *testing.T) {
	reiniciarRaiz(t)
	conBaseServidor(t)
	if Servidor() { // (Entorno o Portable ya calcularían la raíz del usuario)
		t.Fatal("sin pedirlo no es modo servidor")
	}
	UsarServidor()
	if !Servidor() || Entorno() != "servidor" {
		t.Fatal("tras UsarServidor el modo es servidor")
	}
	if r, err := RaizUsuario(); err != nil || r != RaizServidor() {
		t.Fatalf("raíz tras UsarServidor: %q %v", r, err)
	}
	UsarServidor() // ya calculada como la del servidor: repetirlo no es un error
}

// Después de calcular la raíz como la del usuario, UsarServidor es un error de programación y entra en pánico, en
// lugar de dejar la mitad del proceso en cada carpeta.
func TestUsarServidorTarde(t *testing.T) {
	reiniciarRaiz(t)
	if _, err := RaizUsuario(); err != nil {
		t.Skip("sin raíz de usuario en este equipo:", err)
	}
	defer func() {
		mensaje, _ := recover().(string)
		if !strings.Contains(mensaje, "UsarServidor") || Servidor() {
			t.Fatalf("esperaba un pánico que nombre UsarServidor y sin pasar a modo servidor: %q", mensaje)
		}
	}()
	UsarServidor()
}

func TestEntorno(t *testing.T) {
	reiniciarRaiz(t)
	raiz = func() (resultado, error) { return resultado{ruta: "x", portable: true}, nil }
	if Entorno() != "portable" {
		t.Fatalf("Entorno() = %q", Entorno())
	}
	raiz = func() (resultado, error) { return resultado{ruta: "x"}, nil }
	if Entorno() != "escritorio" {
		t.Fatalf("Entorno() = %q", Entorno())
	}
}
