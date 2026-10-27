package terminos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func escribir(t *testing.T, datos, contenido string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(datos, "terminos.json"), []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMayor(t *testing.T) {
	for v, esperado := range map[string]int{"2.0": 2, "2.1": 2, "10.3": 10, "1": 1, "": 0, "x": 0, "-1.0": 0, ".5": 0} {
		if got := Mayor(v); got != esperado {
			t.Errorf("Mayor(%q) = %d, esperaba %d", v, got, esperado)
		}
	}
}

func TestPendiente(t *testing.T) {
	datos := t.TempDir()
	if p, err := Pendiente(datos); err != nil || !p {
		t.Fatalf("sin archivo: pendiente=%v err=%v, esperaba true", p, err)
	}
	if err := Aceptar(datos, ComoCLI); err != nil {
		t.Fatal(err)
	}
	if p, err := Pendiente(datos); err != nil || p {
		t.Fatalf("tras aceptar: pendiente=%v err=%v, esperaba false", p, err)
	}
	escribir(t, datos, `{"version":"1.0"}`)
	if p, _ := Pendiente(datos); !p {
		t.Error("la versión 1.0 está pendiente de la 2.0")
	}
	escribir(t, datos, `{"version":"2.1"}`)
	if p, _ := Pendiente(datos); p {
		t.Error("un cambio menor (2.1) no vuelve a pedir la aceptación")
	}
	escribir(t, datos, `no es json`)
	if p, err := Pendiente(datos); !p || err == nil {
		t.Errorf("un archivo ilegible cuenta como pendiente y se informa: pendiente=%v err=%v", p, err)
	}
}

func TestAceptarEscribe(t *testing.T) {
	datos := t.TempDir()
	antes := time.Now().UTC().Add(-time.Second)
	if err := Aceptar(datos, ComoInstalador); err != nil {
		t.Fatal(err)
	}
	bruto, err := os.ReadFile(filepath.Join(datos, "terminos.json"))
	if err != nil {
		t.Fatal(err)
	}
	var campos map[string]string
	if err := json.Unmarshal(bruto, &campos); err != nil || len(campos) != 3 {
		t.Fatalf("esperaba tres campos: %s (%v)", bruto, err)
	}
	if campos["version"] != Version || campos["como"] != ComoInstalador {
		t.Errorf("campos: %v", campos)
	}
	if !strings.HasSuffix(campos["fecha"], "Z") {
		t.Errorf("la fecha va en UTC: %q", campos["fecha"])
	}
	a, err := Leer(datos)
	if err != nil || a.Fecha.Before(antes) || a.Fecha.After(time.Now().Add(time.Second)) || a.Fecha.Location() != time.UTC {
		t.Errorf("Leer: %+v %v", a, err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(filepath.Join(datos, "terminos.json")); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("permisos: %v %v", info.Mode(), err)
		}
	}
	if err := Aceptar(datos, "inventado"); err == nil {
		t.Error("un valor de «como» desconocido se rechaza")
	}
}

func TestLeerSinArchivo(t *testing.T) {
	if a, err := Leer(t.TempDir()); err != nil || a != (Aceptacion{}) {
		t.Errorf("sin archivo: %+v %v", a, err)
	}
}

func TestRiesgosCinco(t *testing.T) {
	servidor, escritorio := Riesgos(true), Riesgos(false)
	if len(servidor) != 5 || len(escritorio) != 5 {
		t.Fatalf("deben ser cinco: %d y %d", len(servidor), len(escritorio))
	}
	for i := range servidor {
		prefijo := strings.HasPrefix(escritorio[i], "Si usas el modo servidor, ")
		if want := i == 2 || i == 3; prefijo != want {
			t.Errorf("riesgo %d: prefijo=%v, esperaba %v (%q)", i+1, prefijo, want, escritorio[i])
		}
		if !(i == 2 || i == 3) && escritorio[i] != servidor[i] {
			t.Errorf("riesgo %d: solo el 3 y el 4 cambian sin el modo servidor", i+1)
		}
	}
}

// TestCopiaTerminosAlDia: go:embed no sube de carpeta, así que internal/terminos/TERMINOS.md es una copia del de la raíz.
func TestCopiaTerminosAlDia(t *testing.T) {
	raiz, err := os.ReadFile(filepath.Join("..", "..", "TERMINOS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if Texto != string(raiz) {
		t.Error("internal/terminos/TERMINOS.md difiere de TERMINOS.md: cópialo de la raíz")
	}
	if !strings.Contains(Texto, "**Versión "+Version[:1]+" ·") {
		t.Errorf("TERMINOS.md no dice «Versión %s»", Version[:1])
	}
}
