package servicio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/apuntar"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/planificador"
)

// pruebaHerramientas es un servicio de prueba con «claude» simulado, las carpetas de las herramientas en temporales
// (nunca las del usuario), el PATH vacío (ningún programa detectado) y un almacén en memoria.
type pruebaHerramientas struct {
	s        *Servicio
	ts       *httptest.Server
	proyecto string
	casa     string
	almacen  map[string]string
	exe      string
}

func entornoHerramientas(t *testing.T, ajustar func(*Opciones), detectadas ...string) *pruebaHerramientas {
	t.Helper()
	casa := t.TempDir()
	for _, v := range []string{"HOME", "USERPROFILE", "APPDATA"} {
		t.Setenv(v, casa)
	}
	t.Setenv("PATH", t.TempDir())
	for v, d := range map[string]string{"CLAUDE_CONFIG_DIR": "claude", "CODEX_HOME": "codex", "CONTINUE_GLOBAL_DIR": "continue", "XDG_CONFIG_HOME": "xdg"} {
		t.Setenv(v, filepath.Join(casa, d))
	}
	for _, id := range detectadas {
		ruta, err := herramienta(id).Ruta(apuntar.AlcanceGlobal, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	s, ts, proyecto := entornoCon(t, "", "", adaptadores.Simulado{}, func(o *Opciones) {
		o.Registro["claude"] = adaptadores.Simulado{}
		o.Semaforos = planificador.NuevosSemaforos(map[string]int{"simulado": 2, "claude": 2})
		if ajustar != nil {
			ajustar(o)
		}
	})
	p := &pruebaHerramientas{s: s, ts: ts, proyecto: proyecto, casa: casa, almacen: map[string]string{},
		exe: filepath.Join(casa, "Ana María", "tramtai.exe")}
	os.MkdirAll(filepath.Dir(p.exe), 0o700)
	os.WriteFile(p.exe, []byte("x"), 0o700)
	s.guardarClave = func(nombre, valor string) error { p.almacen[nombre] = valor; return nil }
	s.leerClave = func(nombre string) (string, error) { return p.almacen[nombre], nil }
	s.borrarClave = func(nombre string) error { delete(p.almacen, nombre); return nil }
	s.ejecutable = func() (string, error) { return p.exe, nil }
	return p
}

func herramienta(id string) apuntar.Herramienta {
	i := slices.IndexFunc(apuntar.Lista(), func(h apuntar.Herramienta) bool { return h.ID == id })
	return apuntar.Lista()[i]
}

func (p *pruebaHerramientas) pedir(t *testing.T, inst, metodo, ruta, cuerpo string) (*http.Response, string) {
	t.Helper()
	return pedir(t, p.ts.URL, metodo, ruta, p.s.tokenAdmin, cuerpo, "Tramtai-Instancia", inst, "Content-Type", "application/json")
}

// estados pide la lista (o lee la de una respuesta) y la devuelve por id.
func (p *pruebaHerramientas) estados(t *testing.T, inst string) map[string]contrato.EstadoHerramienta {
	t.Helper()
	r, cuerpo := p.pedir(t, inst, "GET", "/api/v1/herramientas", "")
	if r.StatusCode != 200 {
		t.Fatalf("lista: %d %s", r.StatusCode, cuerpo)
	}
	return porID(t, cuerpo)
}

func porID(t *testing.T, cuerpo string) map[string]contrato.EstadoHerramienta {
	t.Helper()
	var r struct {
		Herramientas []contrato.EstadoHerramienta `json:"herramientas"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &r); err != nil || len(r.Herramientas) != len(apuntar.Lista()) {
		t.Fatalf("no es la lista de herramientas: %s", cuerpo)
	}
	m := map[string]contrato.EstadoHerramienta{}
	for _, e := range r.Herramientas {
		m[e.ID] = e
	}
	return m
}

func (p *pruebaHerramientas) activar(t *testing.T, inst, cuerpo string) map[string]contrato.EstadoHerramienta {
	t.Helper()
	r, respuesta := p.pedir(t, inst, "POST", "/api/v1/herramientas/activar", cuerpo)
	if r.StatusCode != 200 {
		t.Fatalf("activar %s: %d %s", cuerpo, r.StatusCode, respuesta)
	}
	return porID(t, respuesta)
}

// apiGlobal es la API de ese nombre en la configuración global, tal como está en el archivo.
func apiGlobal(t *testing.T, nombre string) *ConfigPunto {
	t.Helper()
	ruta, _ := configuracion.RutaGlobal()
	lista, err := leerArchivoPuntos(ruta, configuracion.OrigenGlobal)
	if err != nil {
		t.Fatal(err)
	}
	if i := slices.IndexFunc(lista, func(c ConfigPunto) bool { return c.Nombre == nombre }); i >= 0 {
		return &lista[i]
	}
	return nil
}

func TestActivarCodexCreaAPIYGuardaClave(t *testing.T) {
	p := entornoHerramientas(t, nil, "codex")
	if _, err := p.s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	e := p.activar(t, "global", `{"ids":["codex"],"modelo":"claude/haiku"}`)["codex"]
	if e.Estado != "activa" || e.Modelo != "claude/haiku" || e.Nombre != "Codex" || e.ClaveEnArchivo || e.HayCopia {
		t.Fatalf("estado tras activar: %+v", e)
	}
	api := apiGlobal(t, "codex")
	if api == nil || api.Grupo != "Herramientas" || !slices.Equal(api.ModelosPermitidos, []string{"claude/haiku"}) || !api.EstaActivo() {
		t.Fatalf("la API de la herramienta: %+v", api)
	}
	clave := p.almacen["TRAMTAI_HERRAMIENTA_CODEX"]
	if !strings.HasPrefix(clave, "tt_") || len(p.almacen) != 1 {
		t.Fatalf("la clave va al almacén con su nombre: %v", p.almacen)
	}
	if r, cuerpo := pedir(t, p.ts.URL, "GET", "/p/codex/openai/v1/models", clave, ""); r.StatusCode != 200 {
		t.Fatalf("la clave guardada vale en /p/codex/openai/v1: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, p.ts.URL, "GET", "/p/codex/openai/v1/models", "tt_otra", ""); r.StatusCode != 401 {
		t.Fatalf("otra clave no vale: %d", r.StatusCode)
	}
	toml, err := os.ReadFile(filepath.Join(p.casa, "codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(toml), clave) || !strings.Contains(string(toml), "/p/codex/openai/v1") || !strings.Contains(string(toml), "Ana María") ||
		!strings.Contains(string(toml), "herramientas") {
		t.Fatalf("config.toml lleva la URL de su API y la orden, nunca la clave:\n%s", toml)
	}
	a, activa, _ := apuntar.Leer(p.s.opc.Datos, "codex")
	args, _ := json.Marshal(a.Escrito["model_providers.tramtai.auth.args"])
	if !activa || a.Orden != p.exe || a.Escrito["model_providers.tramtai.auth.command"] != p.exe || string(args) != `["herramientas","clave","codex"]` ||
		a.Puerto != p.s.opc.Puerto || a.Escrito["model_providers.tramtai.base_url"] != p.ts.URL+"/p/codex/openai/v1" {
		t.Fatalf("la orden es el ejecutable + herramientas clave codex, y la URL la de su API: %+v", a)
	}
}

func TestActivarTodasSoloDetectadas(t *testing.T) {
	p := entornoHerramientas(t, nil, "codex", "continue")
	p.s.ActivarGlobal()
	estados := p.activar(t, "global", `{"todas":true,"modelo":"claude/haiku"}`)
	for id, e := range estados {
		if activa := id == "codex" || id == "continue"; activa != (e.Estado == "activa") {
			t.Errorf("%s: %+v", id, e)
		}
	}
	if !estados["continue"].ClaveEnArchivo || len(p.almacen) != 1 || p.almacen["TRAMTAI_HERRAMIENTA_CODEX"] == "" {
		t.Fatalf("Continue lleva la clave en su archivo; solo Codex va al almacén: %v", p.almacen)
	}
	if apiGlobal(t, "continue") == nil || apiGlobal(t, "opencode") != nil {
		t.Fatal("solo las detectadas tienen API")
	}
}

func TestDesactivarDetieneAPI(t *testing.T) {
	p := entornoHerramientas(t, nil, "codex")
	p.s.ActivarGlobal()
	p.activar(t, "global", `{"ids":["codex"],"modelo":"claude/haiku"}`)
	r, cuerpo := p.pedir(t, "global", "POST", "/api/v1/herramientas/desactivar", `{"ids":["codex"]}`)
	if r.StatusCode != 200 || porID(t, cuerpo)["codex"].Estado != "desactivada" {
		t.Fatalf("desactivar: %d %s", r.StatusCode, cuerpo)
	}
	if api := apiGlobal(t, "codex"); api == nil || api.EstaActivo() {
		t.Fatalf("la API queda detenida: %+v", api)
	}
	if p.almacen["TRAMTAI_HERRAMIENTA_CODEX"] == "" {
		t.Fatal("la clave sigue en el almacén")
	}
	if toml, _ := os.ReadFile(filepath.Join(p.casa, "codex", "config.toml")); strings.Contains(string(toml), "/p/codex") {
		t.Fatalf("config.toml ya no apunta a Tramtai:\n%s", toml)
	}
	if e := p.estados(t, "global")["codex"]; e.Modelo != "claude/haiku" {
		t.Fatalf("desactivada recuerda su modelo: %+v", e)
	}
	// Volver a activarla reanuda la misma API.
	if e := p.activar(t, "global", `{"ids":["codex"]}`)["codex"]; e.Estado != "activa" || e.Modelo != "claude" || !apiGlobal(t, "codex").EstaActivo() {
		t.Fatalf("reactivar con el modelo por defecto: %+v", e)
	}
}

func TestOlvidarBorraClave(t *testing.T) {
	p := entornoHerramientas(t, nil, "codex")
	p.s.ActivarGlobal()
	p.activar(t, "global", `{"ids":["codex"],"modelo":"claude/haiku"}`)
	if r, cuerpo := p.pedir(t, "global", "POST", "/api/v1/herramientas/codex/olvidar", ""); r.StatusCode != 204 {
		t.Fatalf("olvidar: %d %s", r.StatusCode, cuerpo)
	}
	if len(p.almacen) != 0 {
		t.Fatalf("la clave sale del almacén: %v", p.almacen)
	}
	if _, activa, _ := apuntar.Leer(p.s.opc.Datos, "codex"); activa {
		t.Fatal("no queda anotación")
	}
	if e := p.estados(t, "global")["codex"]; e.Estado != "detectada" || apiGlobal(t, "codex") != nil {
		t.Fatalf("olvidada: %+v", e)
	}
}

func TestDesdeProyectoSoloClaudeCode(t *testing.T) {
	p := entornoHerramientas(t, nil, "codex", "claude-code")
	p.s.ActivarGlobal()
	pry, err := p.s.ActivarProyecto(p.proyecto)
	if err != nil {
		t.Fatal(err)
	}
	if e := p.estados(t, pry.ID)["codex"]; e.Estado != "solo_global" {
		t.Fatalf("codex desde un proyecto: %+v", e)
	}
	if r, cuerpo := p.pedir(t, pry.ID, "POST", "/api/v1/herramientas/activar", `{"ids":["codex"]}`); r.StatusCode != 400 {
		t.Fatalf("codex no se activa desde un proyecto: %d %s", r.StatusCode, cuerpo)
	}
	e := p.activar(t, pry.ID, `{"ids":["claude-code"],"modelo":"claude/haiku"}`)["claude-code"]
	local := filepath.Join(p.proyecto, ".claude", "settings.local.json")
	if e.Estado != "activa" || !mismaRuta(e.Ruta, local) {
		t.Fatalf("claude code desde el proyecto: %+v", e)
	}
	if datos, err := os.ReadFile(local); err != nil || !strings.Contains(string(datos), "/p/claude-code/anthropic") {
		t.Fatalf("settings.local.json: %v %s", err, datos)
	}
	for _, otro := range []string{filepath.Join(p.proyecto, ".claude", "settings.json"), filepath.Join(p.casa, "claude", "settings.json")} {
		if _, err := os.Stat(otro); err == nil {
			t.Fatalf("no se toca %s", otro)
		}
	}
	// La API va en la global con la instancia del proyecto (un local.yaml sin confianza no admite APIs).
	if api := apiGlobal(t, "claude-code"); api == nil || api.Instancia != pry.ID {
		t.Fatalf("la API del proyecto: %+v", api)
	}
	if clave := p.almacen["TRAMTAI_HERRAMIENTA_CLAUDE_CODE"]; clave == "" {
		t.Fatal("la clave de Claude Code va al almacén")
	} else {
		for _, prueba := range []struct {
			clave string
			vale  bool
		}{{clave, true}, {"tt_otra", false}} {
			r, cuerpo := pedir(t, p.ts.URL, "POST", "/p/claude-code/anthropic/v1/messages", prueba.clave, "{}")
			if (r.StatusCode != 401) != prueba.vale {
				t.Fatalf("la API del proyecto con %q: %d %s", prueba.clave, r.StatusCode, cuerpo)
			}
		}
	}
	// Activa en el proyecto: desde la global se ve dónde y activarla ahí pide desactivarla antes.
	if e := p.estados(t, "global")["claude-code"]; e.Estado != "activa" || e.Motivo == "" {
		t.Fatalf("vista desde la global: %+v", e)
	}
	if r, cuerpo := p.pedir(t, "global", "POST", "/api/v1/herramientas/activar", `{"ids":["claude-code"]}`); r.StatusCode != 409 {
		t.Fatalf("otra activación: %d %s", r.StatusCode, cuerpo)
	}
}

func TestDesactualizada(t *testing.T) {
	p := entornoHerramientas(t, nil, "codex")
	p.s.ActivarGlobal()
	p.activar(t, "global", `{"ids":["codex"],"modelo":"claude/haiku"}`)
	comprobar := func(caso, motivo string) {
		t.Helper()
		e := p.estados(t, "global")["codex"]
		if e.Estado != "desactualizada" || !strings.Contains(e.Motivo, motivo) {
			t.Fatalf("%s: %+v", caso, e)
		}
	}
	os.Rename(p.exe, p.exe+".movido")
	comprobar("la orden ya no existe", p.exe)
	os.Rename(p.exe+".movido", p.exe)

	p.s.mu.Lock()
	puerto := p.s.opc.Puerto
	p.s.opc.Puerto = puerto + 1
	p.s.mu.Unlock()
	comprobar("otro puerto", "puerto")
	p.s.mu.Lock()
	p.s.opc.Puerto = puerto
	p.s.mu.Unlock()
	if e := p.estados(t, "global")["codex"]; e.Estado != "activa" {
		t.Fatalf("todo en orden: %+v", e)
	}

	delete(p.almacen, "TRAMTAI_HERRAMIENTA_CODEX")
	comprobar("sin la clave en el almacén", "almacén")
	p.activar(t, "global", `{"ids":["codex"],"modelo":"claude/haiku"}`)

	toml := filepath.Join(p.casa, "codex", "config.toml")
	original, _ := os.ReadFile(toml)
	os.WriteFile(toml, []byte("model = \"gpt-5\"\n"), 0o600)
	comprobar("el usuario lo cambió", "config.toml")
	os.WriteFile(toml, original, 0o600)

	if r, cuerpo := pedir(t, p.ts.URL, "DELETE", "/api/v1/puntos/codex", p.s.tokenAdmin, "", "Tramtai-Instancia", "global"); r.StatusCode != 204 {
		t.Fatalf("borrar la API: %d %s", r.StatusCode, cuerpo)
	}
	comprobar("sin API", "API")
	r, cuerpo := p.pedir(t, "global", "POST", "/api/v1/herramientas/desactivar", `{"todas":true}`)
	if r.StatusCode != 200 || porID(t, cuerpo)["codex"].Estado != "detectada" {
		t.Fatalf("desactivar una desactualizada no falla: %d %s", r.StatusCode, cuerpo)
	}
}

func TestRestaurarHerramienta(t *testing.T) {
	p := entornoHerramientas(t, nil, "codex")
	p.s.ActivarGlobal()
	p.activar(t, "global", `{"ids":["codex"],"modelo":"claude/haiku"}`)
	if r, cuerpo := p.pedir(t, "global", "POST", "/api/v1/herramientas/codex/restaurar", ""); r.StatusCode != 400 || !strings.Contains(cuerpo, "no hay copia") {
		t.Fatalf("sin archivo previo no hay copia: %d %s", r.StatusCode, cuerpo)
	}
	p.pedir(t, "global", "POST", "/api/v1/herramientas/codex/olvidar", "")
	toml := filepath.Join(p.casa, "codex", "config.toml")
	os.WriteFile(toml, []byte("model = \"gpt-5\"\n"), 0o600)
	if e := p.activar(t, "global", `{"ids":["codex"],"modelo":"claude/haiku"}`)["codex"]; !e.HayCopia {
		t.Fatalf("con archivo previo hay copia: %+v", e)
	}
	r, cuerpo := p.pedir(t, "global", "POST", "/api/v1/herramientas/codex/restaurar", "")
	var copia struct {
		Copia string `json:"copia"`
	}
	json.Unmarshal([]byte(cuerpo), &copia)
	if datos, _ := os.ReadFile(toml); r.StatusCode != 200 || copia.Copia == "" || string(datos) != "model = \"gpt-5\"\n" {
		t.Fatalf("restaurar: %d %s %q", r.StatusCode, cuerpo, datos)
	}
	if e := p.estados(t, "global")["codex"]; e.Estado != "desactivada" || apiGlobal(t, "codex").EstaActivo() {
		t.Fatalf("restaurada, la API se detiene: %+v", e)
	}
}

func TestHerramientaContinueConConfigJSON(t *testing.T) {
	p := entornoHerramientas(t, nil, "continue")
	p.s.ActivarGlobal()
	os.WriteFile(filepath.Join(p.casa, "continue", "config.json"), []byte("{}"), 0o600)
	if e := p.estados(t, "global")["continue"]; e.Estado != "detectada" || !strings.Contains(e.Motivo, "config.json") {
		t.Fatalf("Continue con config.json: %+v", e)
	}
	r, cuerpo := p.pedir(t, "global", "POST", "/api/v1/herramientas/activar", `{"ids":["continue"]}`)
	if r.StatusCode != 400 || !strings.Contains(cuerpo, "config.json") {
		t.Fatalf("no se activa: %d %s", r.StatusCode, cuerpo)
	}
	if apiGlobal(t, "continue") != nil {
		t.Fatal("una activación rechazada no deja API")
	}
}

func TestServidorNoOfreceHerramientas(t *testing.T) {
	p := entornoHerramientas(t, func(o *Opciones) { o.Entorno = "servidor" }, "codex")
	p.s.ActivarGlobal()
	for _, peticion := range [][2]string{{"GET", "/api/v1/herramientas"}, {"POST", "/api/v1/herramientas/activar"}, {"POST", "/api/v1/herramientas/codex/olvidar"}} {
		r, cuerpo := p.pedir(t, "global", peticion[0], peticion[1], `{"todas":true}`)
		if r.StatusCode != 409 || !strings.Contains(cuerpo, "no_disponible_en_servidor") {
			t.Fatalf("%v: %d %s", peticion, r.StatusCode, cuerpo)
		}
	}
	if _, err := os.Stat(filepath.Join(p.casa, "codex", "config.toml")); err == nil {
		t.Fatal("en el servidor no se escribe nada")
	}
}
