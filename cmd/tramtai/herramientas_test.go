package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/servicio"
)

// herramientasFalsas es un servicio que atiende /api/v1/herramientas… con una lista fija y apunta cada petición como
// «MÉTODO ruta cuerpo». Con él, servicioHerramientas devuelve un cliente de verdad (no el Tramtai del equipo).
func herramientasFalsas(t *testing.T) (peticiones func() []string) {
	t.Helper()
	t.Chdir(t.TempDir()) // fuera de cualquier proyecto: la instancia es la global
	var mu sync.Mutex
	var hechas []string
	lista := []contrato.EstadoHerramienta{
		{ID: "claude-code", Nombre: "Claude Code", Estado: "activa", Modelo: "claude/sonnet", Ruta: `C:\Users\ana\.claude\settings.json`, HayCopia: true},
		{ID: "codex", Nombre: "Codex", Estado: "desactualizada", Ruta: `C:\Users\ana\.codex\config.toml`, Motivo: "Tramtai se movió: vuelve a activarla"},
		{ID: "continue", Nombre: "Continue", Estado: "no_detectada"},
	}
	const token = "token-de-prueba-del-servicio-falso"
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasPrefix(r.URL.Path, "/api/v1/herramientas") { // la prueba de identidad de conectar()
			json.NewEncoder(w).Encode(map[string]string{"prueba": contrato.PruebaIdentidad(token, ts.URL, r.URL.Query().Get("reto"))})
			return
		}
		cuerpo, _ := io.ReadAll(r.Body)
		mu.Lock()
		hechas = append(hechas, strings.TrimSpace(r.Method+" "+r.URL.Path+" "+string(cuerpo)))
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/olvidar"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/restaurar"):
			json.NewEncoder(w).Encode(map[string]string{"copia": "claude-code-20261105-140500.json"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"herramientas": lista})
		}
	}))
	t.Cleanup(ts.Close)
	datos := t.TempDir()
	d, _ := json.Marshal(contrato.Descriptor{Identificador: "servicio_falso", PID: 4242, Base: ts.URL})
	if err := os.WriteFile(contrato.RutaDescriptor(datos), d, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(datos, "token-servicio"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	anterior := servicioHerramientas
	servicioHerramientas = func(*cobra.Command) (*cliente.Cliente, error) { return cliente.Conectar(datos) }
	t.Cleanup(func() { servicioHerramientas = anterior })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), hechas...)
	}
}

// ejecutarHerramientas ejecuta la orden raíz con esos argumentos y devuelve stdout, stderr y el error.
func ejecutarHerramientas(t *testing.T, entrada string, args ...string) (salida, errores string, err error) {
	t.Helper()
	var o, e strings.Builder
	raiz := nuevaRaiz()
	raiz.SetOut(&o)
	raiz.SetErr(&e)
	raiz.SetIn(strings.NewReader(entrada))
	raiz.SetArgs(args)
	err = raiz.Execute()
	return o.String(), e.String(), err
}

// sinServicio hace fallar la prueba si la orden intenta hablar con el servicio.
func sinServicio(t *testing.T) {
	t.Helper()
	anterior := servicioHerramientas
	servicioHerramientas = func(*cobra.Command) (*cliente.Cliente, error) {
		t.Error("esta orden no debe contactar con el servicio")
		return nil, errors.New("sin servicio")
	}
	t.Cleanup(func() { servicioHerramientas = anterior })
}

func TestHerramientasAlias(t *testing.T) {
	raiz := nuevaRaiz()
	pares := [][2][]string{
		{{"tools", "list"}, {"herramientas", "listar"}},
		{{"tools", "enable", "--all", "--model", "claude/haiku"}, {"herramientas", "activar", "--todas", "--modelo", "claude/haiku"}},
		{{"tools", "disable", "codex"}, {"herramientas", "desactivar", "codex"}},
		{{"tools", "forget", "codex"}, {"herramientas", "olvidar", "codex"}},
		{{"tools", "restore", "codex"}, {"herramientas", "restaurar", "codex"}},
		{{"tools", "key", "codex"}, {"herramientas", "clave", "codex"}},
	}
	for _, p := range pares {
		en, _, errEn := raiz.Find(p[0])
		es, _, errEs := raiz.Find(p[1])
		if errEn != nil || errEs != nil || en != es || en.Name() == "herramientas" {
			t.Errorf("%v no resuelve a lo mismo que %v: %s / %s (%v, %v)", p[0], p[1], en.CommandPath(), es.CommandPath(), errEn, errEs)
			continue
		}
		if err := en.ParseFlags(p[0][2:]); err != nil {
			t.Errorf("%v: las opciones en inglés no se reconocen: %v", p[0], err)
		}
	}
}

func TestClaveSoloLaClave(t *testing.T) {
	sinServicio(t)
	anterior := leerClave
	t.Cleanup(func() { leerClave = anterior })
	var pedida string
	leerClave = func(nombre string) (string, error) { pedida = nombre; return "tk-secreta-123", nil }
	salida, errores, err := ejecutarHerramientas(t, "", "herramientas", "clave", "codex")
	if err != nil || salida != "tk-secreta-123\n" || errores != "" {
		t.Fatalf("clave: err=%v stdout=%q stderr=%q", err, salida, errores)
	}
	if pedida != servicio.NombreClaveHerramienta("codex") {
		t.Errorf("se leyó %q del almacén, no el nombre de la herramienta", pedida)
	}
}

func TestClaveSinActivar(t *testing.T) {
	sinServicio(t)
	anterior := leerClave
	t.Cleanup(func() { leerClave = anterior })
	for nombre, leer := range map[string]func(string) (string, error){
		"sin clave":           func(string) (string, error) { return "", nil },
		"el almacén falla":    func(string) (string, error) { return "", errors.New("almacén bloqueado") },
		"id sin clave propia": nil, // continue guarda la clave en su archivo: el almacén ni se mira
	} {
		id := "codex"
		if leer == nil {
			id, leer = "continue", func(string) (string, error) {
				t.Error("se leyó el almacén para una herramienta sin clave propia")
				return "x", nil
			}
		}
		leerClave = leer
		salida, errores, err := ejecutarHerramientas(t, "", "herramientas", "clave", id)
		if err == nil || salida != "" || errores == "" || codigoSalida(err) != 1 {
			t.Errorf("%s: err=%v stdout=%q stderr=%q", nombre, err, salida, errores)
		}
	}
}

func TestHerramientasListar(t *testing.T) {
	peticiones := herramientasFalsas(t)
	salida, _, err := ejecutarHerramientas(t, "", "herramientas", "listar")
	if err != nil {
		t.Fatal(err)
	}
	for _, quiere := range []string{"Claude Code", "claude-code", "● Activa", "claude/sonnet", `C:\Users\ana\.claude\settings.json`,
		"Codex", "○ Desactualizada", "↳ Tramtai se movió: vuelve a activarla", "Continue", "○ No detectada"} {
		if !strings.Contains(salida, quiere) {
			t.Errorf("falta %q en:\n%s", quiere, salida)
		}
	}
	if h := peticiones(); len(h) != 1 || h[0] != "GET /api/v1/herramientas" {
		t.Errorf("peticiones: %q", h)
	}
}

// TestMotivoNoEnsanchaLaTabla: la explicación va en la última columna; una larga no mueve las demás (herramientas y espacios).
func TestMotivoNoEnsanchaLaTabla(t *testing.T) {
	largo := strings.Repeat("motivo larguísimo ", 10)
	posicion := func(salida, columna string) int { return strings.Index(strings.SplitN(salida, "\n", 2)[0], columna) }
	var sin, con strings.Builder
	h := contrato.EstadoHerramienta{ID: "codex", Nombre: "Codex", Estado: "desactualizada", Ruta: "config.toml"}
	escribirHerramientas(&sin, []contrato.EstadoHerramienta{h})
	h.Motivo = largo
	escribirHerramientas(&con, []contrato.EstadoHerramienta{h})
	if !strings.Contains(con.String(), "↳ "+largo) || posicion(sin.String(), "ID") != posicion(con.String(), "ID") || posicion(sin.String(), "ARCHIVO") != posicion(con.String(), "ARCHIVO") {
		t.Errorf("el motivo ensancha las columnas de herramientas:\n%s\n%s", sin.String(), con.String())
	}
	sin.Reset()
	con.Reset()
	e := contrato.Espacio{ID: "global", Nombre: "global", Estado: "activo", Ruta: "C:/x"}
	escribirEspacios(&sin, []contrato.Espacio{e})
	e.UltimoError = largo
	escribirEspacios(&con, []contrato.Espacio{e})
	if !strings.Contains(con.String(), "↳ "+largo) || posicion(sin.String(), "ID") != posicion(con.String(), "ID") || posicion(sin.String(), "CARPETA") != posicion(con.String(), "CARPETA") {
		t.Errorf("el error ensancha las columnas de espacios:\n%s\n%s", sin.String(), con.String())
	}
}

func TestHerramientasActivarYDesactivar(t *testing.T) {
	peticiones := herramientasFalsas(t)
	pasos := []struct {
		args     []string
		peticion string
	}{
		{[]string{"activar", "codex", "claude-code", "--modelo", "claude/haiku"}, `POST /api/v1/herramientas/activar {"ids":["codex","claude-code"],"modelo":"claude/haiku"}`},
		{[]string{"activar", "--todas"}, `POST /api/v1/herramientas/activar {"todas":true}`},
		{[]string{"desactivar", "codex"}, `POST /api/v1/herramientas/desactivar {"ids":["codex"],"todas":false}`},
		{[]string{"desactivar", "--todas"}, `POST /api/v1/herramientas/desactivar {"ids":[],"todas":true}`},
	}
	for _, p := range pasos {
		antes := len(peticiones())
		salida, _, err := ejecutarHerramientas(t, "", append([]string{"herramientas"}, p.args...)...)
		h := peticiones()
		if err != nil || len(h) != antes+1 || h[antes] != p.peticion || !strings.Contains(salida, "Claude Code") {
			t.Errorf("%v: err=%v petición=%q (quería %q)\n%s", p.args, err, h[antes:], p.peticion, salida)
		}
	}
	// Sin nada que activar, o con las dos cosas, ni se llama al servicio.
	antes := len(peticiones())
	for _, args := range [][]string{{"activar"}, {"desactivar"}, {"activar", "codex", "--todas"}, {"desactivar", "codex", "--todas"}} {
		if _, _, err := ejecutarHerramientas(t, "", append([]string{"herramientas"}, args...)...); err == nil {
			t.Errorf("%v debería pedir ids o --todas, no las dos", args)
		}
	}
	if len(peticiones()) != antes {
		t.Errorf("se llamó al servicio con argumentos inválidos: %q", peticiones()[antes:])
	}
}

func TestHerramientasOlvidar(t *testing.T) {
	peticiones := herramientasFalsas(t)
	salida, _, err := ejecutarHerramientas(t, "", "herramientas", "olvidar", "codex")
	if h := peticiones(); err != nil || len(h) != 1 || h[0] != "POST /api/v1/herramientas/codex/olvidar" || !strings.Contains(salida, "codex") {
		t.Fatalf("olvidar: err=%v peticiones=%q salida=%q", err, h, salida)
	}
}

// TestRestaurarPideConfirmacion: como «servicio detener», sin --si pregunta y solo una respuesta afirmativa sigue.
func TestRestaurarPideConfirmacion(t *testing.T) {
	peticiones := herramientasFalsas(t)
	_, errores, err := ejecutarHerramientas(t, "n\n", "herramientas", "restaurar", "claude-code")
	if err != nil || len(peticiones()) != 0 || !strings.Contains(errores, "[s/N]") {
		t.Fatalf("sin confirmar: err=%v peticiones=%q pregunta=%q", err, peticiones(), errores)
	}
	salida, errores, err := ejecutarHerramientas(t, "s\n", "herramientas", "restaurar", "claude-code")
	if h := peticiones(); err != nil || len(h) != 1 || h[0] != "POST /api/v1/herramientas/claude-code/restaurar" ||
		!strings.Contains(errores, "[s/N]") || !strings.Contains(salida, "claude-code-20261105-140500.json") {
		t.Fatalf("confirmando: err=%v peticiones=%q stdout=%q stderr=%q", err, h, salida, errores)
	}
	_, errores, err = ejecutarHerramientas(t, "", "herramientas", "restaurar", "claude-code", "--si")
	if h := peticiones(); err != nil || len(h) != 2 || strings.Contains(errores, "[s/N]") {
		t.Fatalf("con --si: err=%v peticiones=%q stderr=%q", err, h, errores)
	}
}
