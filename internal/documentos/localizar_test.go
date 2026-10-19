package documentos

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// conversorFalso crea en dir un conversor con su listo.json y su intérprete (vacío: basta con que exista).
func conversorFalso(t *testing.T, dir string, lock string, conInterprete bool) {
	t.Helper()
	if conInterprete {
		ruta := interprete(dir)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, nil, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	datos, _ := json.Marshal(Listo{Markitdown: "0.1.8", LockSHA256: lock, Python: "3.12.11"})
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "listo.json"), datos, 0o600); err != nil {
		t.Fatal(err)
	}
}

func sinSistema(string) (string, error) { return "", errors.New("no está") }

func enSistema(nombre string) (string, error) { return "/usr/bin/" + nombre, nil }

func TestLocalizarConfigurado(t *testing.T) {
	u, ok := localizarEn("/x/mi-markitdown", t.TempDir(), t.TempDir(), enSistema)
	if !ok || u.Origen != OrigenConfigurado || u.Programa != "/x/mi-markitdown" || len(u.Argumentos) != 0 {
		t.Fatalf("configurado: %+v %v", u, ok)
	}
}

func TestLocalizarPaquete(t *testing.T) {
	ejecutable := t.TempDir()
	conversorFalso(t, filepath.Join(ejecutable, "conversor"), "otro", true)
	raiz := t.TempDir()
	conversorFalso(t, filepath.Join(raiz, "conversor"), HuellaLock(), true)
	u, ok := localizarEn("markitdown", ejecutable, raiz, enSistema)
	if !ok || u.Origen != OrigenPaquete || u.Version != "0.1.8" || u.Lock != "otro" {
		t.Fatalf("paquete: %+v %v", u, ok)
	}
	if u.Programa != interprete(filepath.Join(ejecutable, "conversor")) || !reflect.DeepEqual(u.Argumentos, []string{"-m", "markitdown"}) {
		t.Fatalf("el paquete se ejecuta con python -m markitdown: %+v", u)
	}
}

func TestLocalizarPreparadoAntesQueSistema(t *testing.T) {
	raiz := t.TempDir()
	conversorFalso(t, filepath.Join(raiz, "conversor"), HuellaLock(), true)
	u, ok := localizarEn("markitdown", t.TempDir(), raiz, enSistema)
	if !ok || u.Origen != OrigenPreparado || u.Lock != HuellaLock() {
		t.Fatalf("preparado: %+v %v", u, ok)
	}
}

func TestLocalizarSistema(t *testing.T) {
	u, ok := localizarEn("markitdown", t.TempDir(), t.TempDir(), enSistema)
	if !ok || u.Origen != OrigenSistema || u.Version != "" || u.Programa != "/usr/bin/markitdown" {
		t.Fatalf("sistema: %+v %v", u, ok)
	}
}

func TestLocalizarNada(t *testing.T) {
	if u, ok := localizarEn("markitdown", t.TempDir(), t.TempDir(), sinSistema); ok {
		t.Fatalf("sin conversor no hay ubicación: %+v", u)
	}
}

func TestLocalizarListoSinInterprete(t *testing.T) {
	raiz := t.TempDir()
	conversorFalso(t, filepath.Join(raiz, "conversor"), HuellaLock(), false)
	if u, ok := localizarEn("markitdown", t.TempDir(), raiz, sinSistema); ok {
		t.Fatalf("un listo.json sin intérprete no cuenta: %+v", u)
	}
}

func TestHuellaLock(t *testing.T) {
	if len(HuellaLock()) != 64 || len(LockConversor) == 0 {
		t.Fatalf("huella del lock: %q (%d bytes)", HuellaLock(), len(LockConversor))
	}
}
