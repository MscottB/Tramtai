package documentos

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// uvFalso hace de uv: «python install» deja en --install-dir un Python falso (una copia de este binario, que con
// MARKITDOWN_FALSO=eco hace de markitdown) y «pip install» no hace nada. Cada llamada se anota en UV_FALSO_REGISTRO
// con sus argumentos y las variables UV_*. Modos: ok, falla (sin red) y colgar.
func uvFalso(modo string) {
	if registro := os.Getenv("UV_FALSO_REGISTRO"); registro != "" {
		var uv []string
		for _, par := range os.Environ() {
			if strings.HasPrefix(par, "UV_") && !strings.HasPrefix(par, "UV_FALSO") {
				uv = append(uv, par)
			}
		}
		datos, _ := json.Marshal(map[string][]string{"args": os.Args[1:], "entorno": uv})
		f, _ := os.OpenFile(registro, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		f.Write(append(datos, '\n'))
		f.Close()
	}
	switch modo {
	case "falla":
		fmt.Fprintln(os.Stderr, "error: Failed to download https://example.invalid: network unreachable")
		os.Exit(1)
	case "colgar":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if os.Args[1] == "python" {
		dir := os.Args[slices.Index(os.Args, "--install-dir")+1]
		python := filepath.Join(dir, "cpython-3.12.99-test", "python.exe")
		if runtime.GOOS != "windows" {
			python = filepath.Join(dir, "cpython-3.12.99-test", "bin", "python3")
		}
		os.MkdirAll(filepath.Dir(python), 0o700)
		os.MkdirAll(filepath.Join(dir, "cpython-3.12-test"), 0o700) // como uv: también deja un acceso por versión menor
		origen, _ := os.Open(os.Args[0])
		destino, _ := os.OpenFile(python, os.O_CREATE|os.O_WRONLY, 0o700)
		io.Copy(destino, origen)
		origen.Close()
		destino.Close()
	}
	os.Exit(0)
}

// preparacionFalsa usa el uv falso en el modo dado, con una raíz nueva (con espacios y tildes) y sin markitdown en el
// PATH; devuelve la preparación y el archivo con las llamadas a uv.
func preparacionFalsa(t *testing.T, modo string) (*Preparacion, string, string) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("UV_FALSO", modo)
	t.Setenv("MARKITDOWN_FALSO", "eco")
	registro := filepath.Join(t.TempDir(), "uv.jsonl")
	t.Setenv("UV_FALSO_REGISTRO", registro)
	raiz := filepath.Join(t.TempDir(), "José Pérez")
	return NuevaPreparacion(raiz, "markitdown", os.Args[0]), raiz, registro
}

type llamadaUV struct {
	Args    []string `json:"args"`
	Entorno []string `json:"entorno"`
}

func llamadas(t *testing.T, registro string) []llamadaUV {
	t.Helper()
	f, err := os.Open(registro)
	if err != nil {
		return nil
	}
	defer f.Close()
	var todas []llamadaUV
	s := bufio.NewScanner(f)
	for s.Scan() {
		var l llamadaUV
		if err := json.Unmarshal(s.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		todas = append(todas, l)
	}
	return todas
}

func esperar(t *testing.T, p *Preparacion) string {
	t.Helper()
	ctx, cancelar := context.WithTimeout(context.Background(), time.Minute)
	defer cancelar()
	return p.Esperar(ctx).Estado
}

func TestPrepararPasaAListo(t *testing.T) {
	p, raiz, _ := preparacionFalsa(t, "ok")
	if e := p.Estado(); e.Estado != "falta" {
		t.Fatalf("sin conversor el estado es falta: %+v", e)
	}
	p.Preparar()
	if estado := esperar(t, p); estado != "listo" {
		t.Fatalf("tras preparar: %+v", p.Estado())
	}
	e := p.Estado()
	if e.Origen != string(OrigenPreparado) || e.Version != "0.1.8" {
		t.Fatalf("origen y versión: %+v", e)
	}
	var l Listo
	datos, err := os.ReadFile(filepath.Join(raiz, "conversor", "listo.json"))
	if err != nil || json.Unmarshal(datos, &l) != nil || l.LockSHA256 != HuellaLock() || l.Python != "3.12.99" {
		t.Fatalf("listo.json: %s %v", datos, err)
	}
	if _, err := os.Stat(filepath.Join(raiz, "conversor.nuevo")); !os.IsNotExist(err) {
		t.Fatalf("no queda conversor.nuevo: %v", err)
	}
}

func TestPrepararArgumentosUV(t *testing.T) {
	p, raiz, registro := preparacionFalsa(t, "ok")
	p.Preparar()
	esperar(t, p)
	ls := llamadas(t, registro)
	if len(ls) != 2 {
		t.Fatalf("uv se llama para python y para pip: %+v", ls)
	}
	python, pip := ls[0], ls[1]
	descarga := filepath.Join(raiz, "conversor.nuevo", "descarga")
	if !slices.Contains(python.Args, "--no-bin") || !slices.Contains(python.Args, descarga) {
		t.Errorf("python install sin PATH y con la carpeta (con espacios) como un solo argumento: %q", python.Args)
	}
	if runtime.GOOS == "windows" && !slices.Contains(python.Args, "--no-registry") {
		t.Errorf("en Windows, sin registro: %q", python.Args)
	}
	for _, f := range []string{"--break-system-packages", "--require-hashes", "--no-deps"} {
		if !slices.Contains(pip.Args, f) {
			t.Errorf("pip install sin %s: %q", f, pip.Args)
		}
	}
	for _, v := range []string{"UV_NO_CONFIG=1", "UV_NATIVE_TLS=1"} {
		if !slices.Contains(python.Entorno, v) {
			t.Errorf("falta %s en el entorno de uv: %q", v, python.Entorno)
		}
	}
}

func TestPrepararDosALaVez(t *testing.T) {
	p, _, registro := preparacionFalsa(t, "ok")
	p.Preparar()
	if e := p.Preparar(); e.Estado != "preparando" {
		t.Fatalf("la segunda devuelve el estado en curso: %+v", e)
	}
	esperar(t, p)
	instalaciones := 0
	for _, l := range llamadas(t, registro) {
		if l.Args[0] == "python" {
			instalaciones++
		}
	}
	if instalaciones != 1 {
		t.Fatalf("una sola preparación: %d", instalaciones)
	}
}

func TestPrepararFallaDejaError(t *testing.T) {
	p, _, _ := preparacionFalsa(t, "falla")
	p.Preparar()
	if estado := esperar(t, p); estado != "error" || !strings.Contains(p.Estado().Mensaje, "network unreachable") {
		t.Fatalf("sin red: %+v", p.Estado())
	}
}

func TestPrepararFallaConservaElAnterior(t *testing.T) {
	p, raiz, _ := preparacionFalsa(t, "falla")
	conversorFalso(t, filepath.Join(raiz, "conversor"), "lock-anterior", true)
	p.Preparar()
	if estado := esperar(t, p); estado != "listo" || p.Estado().Mensaje == "" {
		t.Fatalf("el anterior sigue listo y se ve el error: %+v", p.Estado())
	}
}

func TestPrepararPlazo(t *testing.T) {
	p, _, _ := preparacionFalsa(t, "colgar")
	p.plazo = 300 * time.Millisecond
	p.Preparar()
	if estado := esperar(t, p); estado != "error" {
		t.Fatalf("una descarga colgada termina en error: %+v", p.Estado())
	}
}

func TestRestosDeCorte(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	raiz := t.TempDir()
	conversorFalso(t, filepath.Join(raiz, "conversor.nuevo"), HuellaLock(), true)
	os.Remove(filepath.Join(raiz, "conversor.nuevo", "listo.json"))
	p := NuevaPreparacion(raiz, "markitdown", "")
	if _, err := os.Stat(filepath.Join(raiz, "conversor.nuevo")); !os.IsNotExist(err) {
		t.Fatalf("lo a medias se borra al empezar: %v", err)
	}
	if e := p.Estado(); e.Estado != "falta" {
		t.Fatalf("lo a medias no cuenta: %+v", e)
	}
}

func TestDesactualizado(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	raiz := t.TempDir()
	conversorFalso(t, filepath.Join(raiz, "conversor"), "otro-lock", true)
	p := NuevaPreparacion(raiz, "markitdown", "")
	if !p.Desactualizado() || p.Estado().Estado != "listo" {
		t.Fatalf("otro lock: desactualizado pero sirve: %v %+v", p.Desactualizado(), p.Estado())
	}
}

func TestPrepararSinUV(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := NuevaPreparacion(t.TempDir(), "markitdown", "")
	p.Preparar()
	if estado := esperar(t, p); estado != "error" || p.Estado().Mensaje == "" {
		t.Fatalf("sin uv, error legible: %+v", p.Estado())
	}
}

func TestEstadoLlevaPrepararAlIniciar(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := NuevaPreparacion(t.TempDir(), "markitdown", "")
	p.FijarAlIniciar(true)
	if !p.Estado().PrepararAlIniciar {
		t.Fatal("el estado dice si se prepara al iniciar")
	}
	p.FijarAlIniciar(false)
	if p.Estado().PrepararAlIniciar {
		t.Fatal("y lo cambia")
	}
}
