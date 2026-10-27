package configuracion

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"tramtai/internal/contrato"
)

// Si falta un tramo intermedio de la clave, QuitarValor no toca nada: no puede borrar una clave de otro nivel que se
// llame igual que el resto del camino.
func TestQuitarValorSinTramoIntermedio(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "local.yaml")
	original := "retencion:\n  horas_resultado: 5\n"
	os.WriteFile(ruta, []byte(original), 0o600)
	if err := QuitarValor(ruta, OrigenLocal, "servicio.retencion.horas_resultado"); err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); !strings.Contains(string(datos), "horas_resultado: 5") {
		t.Fatalf("se borró retencion.horas_resultado al quitar servicio.retencion.horas_resultado:\n%s", datos)
	}
	if err := QuitarValor(ruta, OrigenLocal, "retencion.horas_resultado"); err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); strings.Contains(string(datos), "horas_resultado") {
		t.Fatal("la clave que sí está se tenía que quitar")
	}
}

func TestEditarPuntosConservaComentarios(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(ruta, []byte("# mi configuración\nservicio:\n  puerto: 17373 # fijo\n"), 0o600)
	err := EditarPuntos(ruta, OrigenGlobal, func(l []contrato.ConfigPunto) ([]contrato.ConfigPunto, error) {
		return append(l, contrato.ConfigPunto{Nombre: "nuevo", Puerto: 17401, Agente: "claude"}), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	datos, _ := os.ReadFile(ruta)
	if !strings.Contains(string(datos), "# mi configuración") || !strings.Contains(string(datos), "# fijo") || !strings.Contains(string(datos), "puerto: 17401") {
		t.Fatalf("archivo:\n%s", datos)
	}
	if err := EditarPuntos(ruta, OrigenProyecto, func(l []contrato.ConfigPunto) ([]contrato.ConfigPunto, error) { return l, nil }); err == nil {
		t.Fatal("puntos_acceso en la configuración versionada debe rechazarse")
	}
	if datos2, _ := os.ReadFile(ruta); string(datos2) != string(datos) {
		t.Fatal("un cambio inválido debe dejar el archivo como estaba")
	}
}

func TestFijarValor(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(ruta, []byte("# mi configuración\nadaptadores:\n  codex:\n    autenticacion: clave_api # comentario\n"), 0o600)
	for _, agente := range []string{"claude", "codex"} {
		if err := FijarValor(ruta, OrigenGlobal, "adaptadores."+agente+".autenticacion", "suscripcion"); err != nil {
			t.Fatal(err)
		}
	}
	capa, err := CargarCapa(ruta, OrigenGlobal)
	datos, _ := os.ReadFile(ruta)
	if err != nil || capa.Valores["adaptadores.claude.autenticacion"] != "suscripcion" || capa.Valores["adaptadores.codex.autenticacion"] != "suscripcion" ||
		!strings.Contains(string(datos), "# mi configuración") {
		t.Fatalf("%v %v\n%s", err, capa.Valores, datos)
	}
	if err := FijarValor(ruta, OrigenGlobal, "servicio.puerto", "abc"); err == nil {
		t.Fatal("un valor no válido debe rechazarse")
	}
	if datos2, _ := os.ReadFile(ruta); string(datos2) != string(datos) {
		t.Fatal("un cambio inválido debe dejar el archivo como estaba")
	}
}

// Veinte ediciones a la vez del mismo config.yaml (pestañas, la TUI, la CLI) no se pisan: al final están todas.
func TestEditarPuntosConcurrente(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	var grupo sync.WaitGroup
	for i := range 20 {
		grupo.Go(func() {
			err := EditarPuntos(ruta, OrigenGlobal, func(l []contrato.ConfigPunto) ([]contrato.ConfigPunto, error) {
				return append(l, contrato.ConfigPunto{Nombre: "p" + strconv.Itoa(i), Puerto: 17500 + i, Agente: "claude"}), nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	grupo.Wait()
	capa, err := CargarCapa(ruta, OrigenGlobal)
	if err != nil {
		t.Fatal(err)
	}
	puntos, err := LeerPuntos(capa.Valores["puntos_acceso"].([]any))
	if err != nil || len(puntos) != 20 {
		t.Fatalf("quedaron %d de 20 puntos: %v", len(puntos), err)
	}
}

// Un valor que no es válido no llega al disco: el archivo queda igual, byte a byte, y sin temporales.
func TestFijarValorInvalidoNoEscribe(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "config.yaml")
	original := []byte("# mía\nservicio:\n  puerto: 17373\n")
	os.WriteFile(ruta, original, 0o600)
	if err := FijarValor(ruta, OrigenGlobal, "servicio.puerto", "no es un número"); err == nil {
		t.Fatal("se aceptó un puerto que no es un número")
	}
	if datos, _ := os.ReadFile(ruta); string(datos) != string(original) {
		t.Fatalf("el archivo cambió:\n%s", datos)
	}
	if entradas, _ := os.ReadDir(dir); len(entradas) != 1 {
		t.Fatalf("quedaron temporales: %v", entradas)
	}
}

// Escrituras atómicas a la vez sobre el mismo archivo: todas terminan bien y queda una de ellas entera.
func TestEscribirAtomicoConcurrente(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "servicio.json")
	var grupo sync.WaitGroup
	for i := range 20 {
		grupo.Go(func() {
			if err := EscribirAtomico(ruta, []byte(strings.Repeat(strconv.Itoa(i%10), 1000)), 0o600); err != nil {
				t.Error(err)
			}
		})
	}
	grupo.Wait()
	datos, _ := os.ReadFile(ruta)
	if len(datos) != 1000 || strings.Count(string(datos), string(datos[0])) != 1000 {
		t.Fatalf("contenido mezclado o a medias: %d bytes", len(datos))
	}
}

// enlazarCarpeta deja en enlace una carpeta que apunta a destino: un punto de unión en Windows (mklink /J, sin elevación)
// y un enlace simbólico en los demás.
func enlazarCarpeta(t *testing.T, destino, enlace string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if salida, err := exec.Command("cmd", "/c", "mklink", "/J", enlace, destino).CombinedOutput(); err != nil {
			t.Fatalf("mklink /J: %v %s", err, salida)
		}
		return
	}
	if err := os.Symlink(destino, enlace); err != nil {
		t.Fatal(err)
	}
}

// En modo servidor, lo que se escribe dentro de la raíz del servidor va por un os.Root, sin seguir enlaces: la raíz es
// de la cuenta del servicio, que podría dejarlos para que el administrador escribiera fuera. Un config.yaml o un
// datos/terminos.json enlazados fuera se sustituyen (el enlace, no lo de fuera); una carpeta intermedia enlazada es un
// error; sin la raíz también lo es, y no se crea (la crea la instalación). Fuera del modo servidor, como siempre.
func TestEscribirAtomicoServidorSinEnlaces(t *testing.T) {
	base := t.TempDir()
	raiz, fuera := filepath.Join(base, "raiz"), filepath.Join(base, "fuera")
	for _, d := range []string{filepath.Join(raiz, "datos"), fuera} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	anterior := raizServidor
	t.Cleanup(func() { raizServidor = anterior })
	raizServidor = func() string { return raiz }
	t.Setenv("TRAMTAI_SERVIDOR", "1")

	for _, nombre := range []string{"config.yaml", filepath.Join("datos", "terminos.json")} {
		objetivo := filepath.Join(fuera, filepath.Base(nombre))
		if err := os.WriteFile(objetivo, []byte("de fuera\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(objetivo, filepath.Join(raiz, nombre)); err != nil {
			t.Logf("sin enlaces simbólicos de archivo aquí (%v): solo se prueban las carpetas", err)
			break
		}
		if err := EscribirAtomico(filepath.Join(raiz, nombre), []byte("nuevo\n"), 0o600); err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if datos, _ := os.ReadFile(objetivo); string(datos) != "de fuera\n" {
			t.Errorf("%s: se escribió a través del enlace: %q", nombre, datos)
		}
		if info, err := os.Lstat(filepath.Join(raiz, nombre)); err != nil || !info.Mode().IsRegular() {
			t.Errorf("%s: debía quedar un archivo normal en lugar del enlace: %v %v", nombre, info, err)
		}
	}

	enlazarCarpeta(t, fuera, filepath.Join(raiz, "otra"))
	if err := EscribirAtomico(filepath.Join(raiz, "otra", "x.json"), []byte("{}"), 0o600); err == nil {
		t.Error("una carpeta intermedia enlazada debía dar error")
	}
	if _, err := os.Lstat(filepath.Join(fuera, "x.json")); err == nil {
		t.Error("se escribió fuera de la raíz a través de la carpeta enlazada")
	}
	// Las carpetas que faltan dentro de la raíz sí se crean (el servicio guarda los tokens de cada espacio así).
	if err := EscribirAtomico(filepath.Join(raiz, "datos", "instancias", "a", "token"), []byte("t\n"), 0o600); err != nil {
		t.Fatalf("carpetas nuevas dentro de la raíz: %v", err)
	}

	sinRaiz := filepath.Join(base, "sin-raiz")
	raizServidor = func() string { return sinRaiz }
	if err := EscribirAtomico(filepath.Join(sinRaiz, "config.yaml"), []byte("x\n"), 0o600); err == nil {
		t.Error("sin la raíz del servidor debía dar error")
	}
	if _, err := os.Stat(sinRaiz); err == nil {
		t.Error("la raíz del servidor no se crea al escribir: la crea la instalación")
	}

	t.Setenv("TRAMTAI_SERVIDOR", "")
	if err := EscribirAtomico(filepath.Join(raiz, "otra", "y.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("fuera del modo servidor se escribe como siempre: %v", err)
	}
}

// En modo servidor, varias escrituras a la vez que crean la misma carpeta intermedia (archivos distintos, así que no las
// serializa el cerrojo de cada ruta) terminan todas bien: la que llega tarde al Mkdir acepta la carpeta de la otra.
func TestEscribirAtomicoServidorCarpetaALaVez(t *testing.T) {
	raiz := t.TempDir()
	anterior := raizServidor
	t.Cleanup(func() { raizServidor = anterior })
	raizServidor = func() string { return raiz }
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	for vuelta := range 10 {
		var grupo sync.WaitGroup
		for i := range 20 {
			grupo.Go(func() {
				ruta := filepath.Join(raiz, "datos", strconv.Itoa(vuelta), "a", strconv.Itoa(i))
				if err := EscribirAtomico(ruta, []byte("x"), 0o600); err != nil {
					t.Error(err)
				}
			})
		}
		grupo.Wait()
	}
}

// Guardar mientras otro proceso lee el archivo (un Recargar, la CLI, el antivirus) termina bien en cuanto lo suelta: en
// Windows el renombrado se reintenta.
func TestEscribirAtomicoConUnLector(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(ruta, []byte("antes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lector, err := os.Open(ruta)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		lector.Close()
	}()
	if err := EscribirAtomico(ruta, []byte("después\n"), 0o600); err != nil {
		t.Fatalf("guardar con un lector abierto: %v", err)
	}
	if datos, _ := os.ReadFile(ruta); string(datos) != "después\n" {
		t.Fatalf("contenido: %q", datos)
	}
}
