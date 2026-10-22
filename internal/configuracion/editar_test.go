package configuracion

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
)

// Si falta un tramo intermedio de la clave, QuitarValor no toca nada: no puede borrar una clave de otro nivel que se
// llame igual que el resto del camino.
func TestQuitarValorSinTramoIntermedio(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "local.yaml")
	original := "retencion:\n  horas_resultado: 5\n"
	os.WriteFile(ruta, []byte(original), 0o600)
	if err := QuitarValor(ruta, nucleo.OrigenLocal, "servicio.retencion.horas_resultado"); err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); !strings.Contains(string(datos), "horas_resultado: 5") {
		t.Fatalf("se borró retencion.horas_resultado al quitar servicio.retencion.horas_resultado:\n%s", datos)
	}
	if err := QuitarValor(ruta, nucleo.OrigenLocal, "retencion.horas_resultado"); err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); strings.Contains(string(datos), "horas_resultado") {
		t.Fatal("la clave que sí está se tenía que quitar")
	}
}

func TestEditarPuntosConservaComentarios(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(ruta, []byte("# mi configuración\nservicio:\n  puerto: 17373 # fijo\n"), 0o600)
	err := EditarPuntos(ruta, nucleo.OrigenGlobal, func(l []contrato.ConfigPunto) ([]contrato.ConfigPunto, error) {
		return append(l, contrato.ConfigPunto{Nombre: "nuevo", Puerto: 17401, Agente: "claude"}), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	datos, _ := os.ReadFile(ruta)
	if !strings.Contains(string(datos), "# mi configuración") || !strings.Contains(string(datos), "# fijo") || !strings.Contains(string(datos), "puerto: 17401") {
		t.Fatalf("archivo:\n%s", datos)
	}
	if err := EditarPuntos(ruta, nucleo.OrigenProyecto, func(l []contrato.ConfigPunto) ([]contrato.ConfigPunto, error) { return l, nil }); err == nil {
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
		if err := FijarValor(ruta, nucleo.OrigenGlobal, "adaptadores."+agente+".autenticacion", "suscripcion"); err != nil {
			t.Fatal(err)
		}
	}
	capa, err := nucleo.CargarCapa(ruta, nucleo.OrigenGlobal)
	datos, _ := os.ReadFile(ruta)
	if err != nil || capa.Valores["adaptadores.claude.autenticacion"] != "suscripcion" || capa.Valores["adaptadores.codex.autenticacion"] != "suscripcion" ||
		!strings.Contains(string(datos), "# mi configuración") {
		t.Fatalf("%v %v\n%s", err, capa.Valores, datos)
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto", "abc"); err == nil {
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
			err := EditarPuntos(ruta, nucleo.OrigenGlobal, func(l []contrato.ConfigPunto) ([]contrato.ConfigPunto, error) {
				return append(l, contrato.ConfigPunto{Nombre: "p" + strconv.Itoa(i), Puerto: 17500 + i, Agente: "claude"}), nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	grupo.Wait()
	capa, err := nucleo.CargarCapa(ruta, nucleo.OrigenGlobal)
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
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto", "no es un número"); err == nil {
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
