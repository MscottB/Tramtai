package apuntar

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

var ordenPrueba = []string{`C:\Tramtai\tramtai.exe`, "herramientas", "clave", "claude-code"}

func valoresClaude() Valores {
	return Valores{URLAnthropic: "http://127.0.0.1:17373/p/claude-code/anthropic", Modelo: "claude/sonnet",
		Orden: ordenPrueba, Puerto: 17373}
}

func valoresOpenCode() Valores {
	return Valores{URLOpenAI: "http://127.0.0.1:17373/p/opencode/openai/v1", Modelo: "claude/sonnet",
		Clave: "tk_secreta_123", Puerto: 17373}
}

func datosPrueba(t *testing.T, nombre string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "json", nombre))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func comoMapa(t *testing.T, b []byte) map[string]any {
	t.Helper()
	b, err := hujsonEstandar(b)
	if err != nil {
		t.Fatalf("no es JSON(C) válido: %v\n%s", err, b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	return m
}

func hujsonEstandar(b []byte) ([]byte, error) { return hujson.Standardize(bytes.TrimPrefix(b, bom)) }

func en(t *testing.T, m map[string]any, ruta ...string) any {
	t.Helper()
	var v any = m
	for _, p := range ruta {
		o, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = o[p]
	}
	return v
}

// herramientaTemporal apunta una herramienta de Lista a un archivo temporal, con su carpeta de datos.
func herramientaTemporal(t *testing.T, id string, inicial []byte, nombre string) (datos, ruta string, h Herramienta) {
	t.Helper()
	datos, ruta = t.TempDir(), filepath.Join(t.TempDir(), nombre)
	for _, l := range Lista() {
		if l.ID == id {
			h = l
		}
	}
	if h.ID == "" {
		t.Fatalf("%s no está en Lista", id)
	}
	h.Ruta = func(Alcance, string) (string, error) { return ruta, nil }
	if inicial != nil {
		if err := os.WriteFile(ruta, inicial, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return datos, ruta, h
}

func TestClaudeCodeConservaLoDemas(t *testing.T) {
	original := datosPrueba(t, "settings.json")
	nuevo, escrito, previos, err := ClaudeCode().Aplicar(original, valoresClaude())
	if err != nil {
		t.Fatal(err)
	}
	m, o := comoMapa(t, nuevo), comoMapa(t, original)
	if !reflect.DeepEqual(m["permissions"], o["permissions"]) || m["includeCoAuthoredBy"] != false {
		t.Errorf("se perdió algo del archivo: %s", nuevo)
	}
	if en(t, m, "env", "OTRA") != "1" {
		t.Errorf("env.OTRA perdida: %s", nuevo)
	}
	if en(t, m, "env", "ANTHROPIC_BASE_URL") != valoresClaude().URLAnthropic || en(t, m, "env", "ANTHROPIC_MODEL") != "claude/sonnet" {
		t.Errorf("faltan las claves de Tramtai: %s", nuevo)
	}
	if m["apiKeyHelper"] != `C:\Tramtai\tramtai.exe herramientas clave claude-code` {
		t.Errorf("apiKeyHelper = %v", m["apiKeyHelper"])
	}
	if len(previos) != 0 || len(escrito) != 3 {
		t.Errorf("escrito = %v, previos = %v", escrito, previos)
	}
	// lo añadido sigue el estilo del archivo: una clave por línea, con su sangrado de 2 espacios
	if want := `  "env": {
    "OTRA": "1",
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:17373/p/claude-code/anthropic",
    "ANTHROPIC_MODEL": "claude/sonnet"
  },
  "includeCoAuthoredBy": false,
  "apiKeyHelper": "C:\\Tramtai\\tramtai.exe herramientas clave claude-code"
}
`; !strings.HasSuffix(string(nuevo), want) {
		t.Errorf("no respetó el estilo del archivo:\n%s", nuevo)
	}
	// y Quitar devuelve lo de antes
	quitado, vacio, err := ClaudeCode().Quitar(nuevo, Anotacion{Escrito: escrito, Previos: previos})
	if err != nil || vacio {
		t.Fatalf("Quitar: vacio = %v, err = %v", vacio, err)
	}
	if !reflect.DeepEqual(comoMapa(t, quitado), o) {
		t.Errorf("tras Quitar: %s", quitado)
	}
}

func TestClaudeCodeGuardaPrevios(t *testing.T) {
	original := []byte(`{"env":{"ANTHROPIC_MODEL":"opus","OTRA":"1"}}`)
	nuevo, escrito, previos, err := ClaudeCode().Aplicar(original, valoresClaude())
	if err != nil {
		t.Fatal(err)
	}
	if previos["env.ANTHROPIC_MODEL"] != "opus" || len(previos) != 1 {
		t.Fatalf("previos = %v", previos)
	}
	quitado, vacio, err := ClaudeCode().Quitar(nuevo, Anotacion{Escrito: escrito, Previos: previos})
	if err != nil || vacio {
		t.Fatalf("vacio = %v, err = %v", vacio, err)
	}
	if !reflect.DeepEqual(comoMapa(t, quitado), comoMapa(t, original)) {
		t.Errorf("Quitar no devolvió el original: %s", quitado)
	}
}

func TestClaudeCodeRespetaCambioDelUsuario(t *testing.T) {
	nuevo, escrito, previos, err := ClaudeCode().Aplicar(datosPrueba(t, "settings.json"), valoresClaude())
	if err != nil {
		t.Fatal(err)
	}
	a := Anotacion{Escrito: escrito, Previos: previos}
	if !ClaudeCode().Contiene(nuevo, a) {
		t.Fatal("Contiene debería ser true recién aplicado")
	}
	tocado := []byte(strings.Replace(string(nuevo), `"claude/sonnet"`, `"sonnet"`, 1))
	if ClaudeCode().Contiene(tocado, a) {
		t.Error("Contiene debería ser false tras el cambio del usuario")
	}
	quitado, _, err := ClaudeCode().Quitar(tocado, a)
	if err != nil {
		t.Fatal(err)
	}
	m := comoMapa(t, quitado)
	if en(t, m, "env", "ANTHROPIC_MODEL") != "sonnet" {
		t.Errorf("Quitar pisó el cambio del usuario: %s", quitado)
	}
	if en(t, m, "env", "ANTHROPIC_BASE_URL") != nil || m["apiKeyHelper"] != nil {
		t.Errorf("Quitar dejó lo que seguía siendo de Tramtai: %s", quitado)
	}
	if en(t, m, "env", "OTRA") != "1" {
		t.Errorf("Quitar tocó lo ajeno: %s", quitado)
	}
}

func TestClaudeCodeOrdenConEspacios(t *testing.T) {
	v := valoresClaude()
	v.Orden = []string{`C:\Users\Ana María\Tramtai\tramtai.exe`, "herramientas", "clave", "claude-code"}
	_, escrito, _, err := ClaudeCode().Aplicar(nil, v)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"C:\Users\Ana María\Tramtai\tramtai.exe" herramientas clave claude-code`; escrito["apiKeyHelper"] != want {
		t.Errorf("apiKeyHelper = %q, quería %q", escrito["apiKeyHelper"], want)
	}
	// en el archivo va escapado como JSON
	nuevo, _, _, _ := ClaudeCode().Aplicar(nil, v)
	if !strings.Contains(string(nuevo), `"apiKeyHelper": "\"C:\\Users\\Ana María\\Tramtai\\tramtai.exe\" herramientas clave claude-code"`) {
		t.Errorf("escape en el archivo: %s", nuevo)
	}
}

func TestClaudeCodeSinOrden(t *testing.T) {
	v := valoresClaude()
	v.Orden = nil
	if _, _, _, err := ClaudeCode().Aplicar(nil, v); !errors.Is(err, errSinOrden) {
		t.Errorf("err = %v", err)
	}
}

func TestClaudeGestionado(t *testing.T) {
	carpeta := t.TempDir()
	gestionado := filepath.Join(carpeta, "managed-settings.json")
	if err := os.WriteFile(gestionado, datosPrueba(t, "managed-settings.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	sinNada := filepath.Join(carpeta, "otro.json")
	if err := os.WriteFile(sinNada, []byte(`{"permissions":{"deny":["WebFetch"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	conHelper := filepath.Join(carpeta, "helper.json")
	if err := os.WriteFile(conHelper, []byte(`// política
{"apiKeyHelper":"/opt/empresa/clave"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	roto := filepath.Join(carpeta, "roto.json")
	if err := os.WriteFile(roto, datosPrueba(t, "roto.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	no := filepath.Join(carpeta, "no-existe.json")
	for _, c := range []struct {
		nombre string
		rutas  []string
		want   bool
	}{
		{"fija la URL", []string{gestionado}, true},
		{"fija el helper (con comentarios)", []string{conHelper}, true},
		{"entre varios", []string{no, sinNada, gestionado}, true},
		{"sin URL ni helper", []string{sinNada}, false},
		{"no existe", []string{no}, false},
		{"roto", []string{roto}, false},
		{"ninguno", nil, false},
	} {
		if got := claudeGestionado(c.rutas); got != c.want {
			t.Errorf("%s: claudeGestionado = %v, quería %v", c.nombre, got, c.want)
		}
	}
}

func TestOpenCodeConservaComentarios(t *testing.T) {
	original := datosPrueba(t, "opencode.jsonc")
	datos, ruta, h := herramientaTemporal(t, "opencode", original, "opencode.jsonc")
	a, err := Aplicar(datos, h, AlcanceGlobal, "", valoresOpenCode())
	if err != nil {
		t.Fatal(err)
	}
	escrito := leerTexto(t, ruta)
	for _, c := range []string{"// Mi configuración de opencode", "/* tema del editor */", "// el que me gusta", `"theme": "tokyonight"`} {
		if !strings.Contains(escrito, c) {
			t.Errorf("tras Aplicar se perdió %q:\n%s", c, escrito)
		}
	}
	m := comoMapa(t, []byte(escrito))
	if en(t, m, "model") != "tramtai/claude/sonnet" || en(t, m, "provider", "otro", "name") != "Otro" {
		t.Errorf("modelo o proveedor ajeno: %s", escrito)
	}
	p := en(t, m, "provider", "tramtai").(map[string]any)
	if p["npm"] != "@ai-sdk/openai-compatible" || p["name"] != "Tramtai" ||
		en(t, p, "options", "baseURL") != valoresOpenCode().URLOpenAI || en(t, p, "options", "apiKey") != "tk_secreta_123" ||
		en(t, p, "models", "claude/sonnet", "name") != "claude/sonnet (Tramtai)" {
		t.Errorf("proveedor tramtai: %v", p)
	}
	if !h.Formato.Contiene([]byte(escrito), a) {
		t.Error("Contiene debería ser true")
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	despues := leerTexto(t, ruta)
	for _, c := range []string{"// Mi configuración de opencode", "/* tema del editor */", "// el que me gusta"} {
		if !strings.Contains(despues, c) {
			t.Errorf("tras Quitar se perdió %q:\n%s", c, despues)
		}
	}
	if strings.Contains(despues, "tramtai") || strings.Contains(despues, "tk_secreta_123") {
		t.Errorf("Quitar dejó restos de Tramtai:\n%s", despues)
	}
	if !reflect.DeepEqual(comoMapa(t, []byte(despues)), comoMapa(t, original)) {
		t.Errorf("Quitar no devolvió lo mismo:\n%s", despues)
	}
}

func TestOpenCodeLaAnotacionNoLlevaLaClave(t *testing.T) {
	datos, _, h := herramientaTemporal(t, "kilo", nil, "kilo.jsonc")
	a, err := Aplicar(datos, h, AlcanceGlobal, "", valoresOpenCode())
	if err != nil {
		t.Fatal(err)
	}
	crudo := leerTexto(t, filepath.Join(datos, "herramientas", "kilo.json"))
	if strings.Contains(crudo, "tk_secreta_123") {
		t.Errorf("la anotación lleva la clave en claro:\n%s", crudo)
	}
	if got, _ := a.Escrito["provider.tramtai.options.apiKey"].(string); got != marca("tk_secreta_123") {
		t.Errorf("la clave se anota como %q", got)
	}
	if _, hay := a.Previos["provider.tramtai.options.apiKey"]; hay {
		t.Error("una clave no puede ir a previos")
	}
}

func TestOpenCodeCambioDeClaveYDeModelo(t *testing.T) {
	datos, ruta, h := herramientaTemporal(t, "opencode", datosPrueba(t, "opencode.jsonc"), "opencode.jsonc")
	a, err := Aplicar(datos, h, AlcanceGlobal, "", valoresOpenCode())
	if err != nil {
		t.Fatal(err)
	}
	// el usuario cambia la clave a mano: ya no es lo que escribió Tramtai
	tocado := strings.Replace(leerTexto(t, ruta), "tk_secreta_123", "otra_clave", 1)
	if h.Formato.Contiene([]byte(tocado), a) {
		t.Error("Contiene debería ser false con otra clave")
	}
	if err := os.WriteFile(ruta, []byte(tocado), 0o600); err != nil {
		t.Fatal(err)
	}
	// otro modelo: se reaplica y el anterior no se queda como resto
	v := valoresOpenCode()
	v.Modelo = "claude/haiku-4.5"
	a, err = Aplicar(datos, h, AlcanceGlobal, "", v)
	if err != nil {
		t.Fatal(err)
	}
	m := comoMapa(t, []byte(leerTexto(t, ruta)))
	models := en(t, m, "provider", "tramtai", "models").(map[string]any)
	if len(models) != 1 || en(t, models, "claude/haiku-4.5", "name") != "claude/haiku-4.5 (Tramtai)" {
		t.Errorf("models = %v", models)
	}
	if en(t, m, "provider", "tramtai", "options", "apiKey") != "tk_secreta_123" {
		t.Error("la clave no se actualizó")
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if got := comoMapa(t, []byte(leerTexto(t, ruta))); !reflect.DeepEqual(got, comoMapa(t, datosPrueba(t, "opencode.jsonc"))) {
		t.Errorf("tras Quitar: %v", got)
	}
}

func TestOpenCodeCreaYBorraElArchivo(t *testing.T) {
	datos, ruta, h := herramientaTemporal(t, "kilo", nil, "kilo.jsonc")
	a, err := Aplicar(datos, h, AlcanceGlobal, "", valoresOpenCode())
	if err != nil {
		t.Fatal(err)
	}
	if !a.NoExistia || !strings.HasSuffix(leerTexto(t, ruta), "}\n") {
		t.Errorf("NoExistia = %v, archivo:\n%s", a.NoExistia, leerTexto(t, ruta))
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Errorf("el archivo debería haberse borrado: %v", err)
	}
}

func TestClaudeCodeArchivoVacioYConBOM(t *testing.T) {
	nuevo, _, _, err := ClaudeCode().Aplicar([]byte("  \n"), valoresClaude())
	if err != nil || en(t, comoMapa(t, nuevo), "env", "ANTHROPIC_MODEL") != "claude/sonnet" {
		t.Fatalf("vacío: %v %s", err, nuevo)
	}
	conBOM := append([]byte("\xef\xbb\xbf"), datosPrueba(t, "settings.json")...)
	nuevo, escrito, previos, err := ClaudeCode().Aplicar(conBOM, valoresClaude())
	if err != nil || !strings.HasPrefix(string(nuevo), "\xef\xbb\xbf{") {
		t.Fatalf("BOM: %v %q", err, nuevo[:8])
	}
	quitado, _, err := ClaudeCode().Quitar(nuevo, Anotacion{Escrito: escrito, Previos: previos})
	if err != nil || !strings.HasPrefix(string(quitado), "\xef\xbb\xbf{") {
		t.Fatalf("BOM al quitar: %v", err)
	}
}

func TestJSONEstilos(t *testing.T) {
	for _, c := range []struct {
		nombre, antes, quiere string
		vacio                 bool // tras Aplicar y Quitar no queda nada
	}{
		{"sin archivo", "", "{\n  \"env\": {\n    \"ANTHROPIC_BASE_URL\"", true},
		{"objeto vacío", "{}", "{\n  \"env\": {\n    \"ANTHROPIC_BASE_URL\"", true},
		{"en una línea", `{"a":1}`, `{"a":1,"env":{"ANTHROPIC_BASE_URL"`, false},
		{"con tabuladores", "{\n\t\"a\": 1\n}\n", "{\n\t\"a\": 1,\n\t\"env\": {\n\t\t\"ANTHROPIC_BASE_URL\"", false},
	} {
		nuevo, escrito, previos, err := ClaudeCode().Aplicar([]byte(c.antes), valoresClaude())
		if err != nil || !strings.Contains(string(nuevo), c.quiere) {
			t.Errorf("%s: err = %v, quería %q en:\n%s", c.nombre, err, c.quiere, nuevo)
			continue
		}
		quitado, vacio, err := ClaudeCode().Quitar(nuevo, Anotacion{Escrito: escrito, Previos: previos})
		if err != nil || vacio != c.vacio {
			t.Errorf("%s: Quitar vacio = %v, err = %v", c.nombre, vacio, err)
		}
		if _, hay := comoMapa(t, quitado)["env"]; hay {
			t.Errorf("%s: Quitar dejó env vacío: %s", c.nombre, quitado)
		}
	}
}

func TestJSONComentarioDeLineaEnLaUltimaClave(t *testing.T) {
	original := "{\n  \"env\": {\n    \"ANTHROPIC_MODEL\": \"opus\" // el mío\n  },\n  \"x\": 1 // y este\n}\n"
	nuevo, escrito, previos, err := ClaudeCode().Aplicar([]byte(original), valoresClaude())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"// el mío", "// y este", "\n    \"ANTHROPIC_BASE_URL\": ", "\n  \"apiKeyHelper\": "} {
		if !strings.Contains(string(nuevo), c) {
			t.Errorf("falta %q (sangrado o comentario) en:\n%s", c, nuevo)
		}
	}
	if strings.Contains(string(nuevo), "\n\"") {
		t.Errorf("una clave quedó en la columna 0:\n%s", nuevo)
	}
	quitado, _, err := ClaudeCode().Quitar(nuevo, Anotacion{Escrito: escrito, Previos: previos})
	if err != nil {
		t.Fatal(err)
	}
	if string(quitado) != original {
		t.Errorf("Quitar no devolvió el original:\n%s", quitado)
	}
}

func TestJSONConFinesDeLineaCRLF(t *testing.T) {
	crlf := func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n") }
	original := crlf(string(datosPrueba(t, "settings.json")))
	nuevo, escrito, previos, err := ClaudeCode().Aplicar([]byte(original), valoresClaude())
	if err != nil {
		t.Fatal(err)
	}
	solo := func(s string) bool { return strings.Count(s, "\n") == strings.Count(s, "\r\n") }
	if !solo(string(nuevo)) || en(t, comoMapa(t, nuevo), "env", "ANTHROPIC_MODEL") != "claude/sonnet" {
		t.Errorf("fines de línea mezclados tras Aplicar: %q", nuevo)
	}
	quitado, _, err := ClaudeCode().Quitar(nuevo, Anotacion{Escrito: escrito, Previos: previos})
	if err != nil || string(quitado) != original {
		t.Errorf("tras Quitar (err %v): %q", err, quitado)
	}
	// también un valor compuesto (models de opencode)
	nuevo, _, _, err = OpenCode().Aplicar([]byte(crlf(string(datosPrueba(t, "opencode.jsonc")))), valoresOpenCode())
	if err != nil || !solo(string(nuevo)) {
		t.Errorf("opencode con CRLF (err %v): %q", err, nuevo)
	}
}

func TestOrdenEnLineaCmd(t *testing.T) {
	for _, c := range []struct{ parte, quiere string }{
		{`C:\Tramtai\tramtai.exe`, `C:\Tramtai\tramtai.exe`},
		{`C:\Ana María\t.exe`, `"C:\Ana María\t.exe"`},
		{`C:\R&D\t.exe`, `"C:\R&D\t.exe"`},
		{`C:\Program Files (x86)\t.exe`, `"C:\Program Files (x86)\t.exe"`},
		{`C:\a^b\t.exe`, `"C:\a^b\t.exe"`},
		{`C:\a|b\t.exe`, `"C:\a|b\t.exe"`},
		{`C:\a<b>\t.exe`, `"C:\a<b>\t.exe"`},
	} {
		got, err := ordenEnLinea([]string{c.parte, "herramientas"})
		if err != nil || got != c.quiere+" herramientas" {
			t.Errorf("%q -> %q, %v; quería %q", c.parte, got, err, c.quiere)
		}
	}
	v := valoresClaude()
	v.Orden = []string{`C:\100%\tramtai.exe`, "herramientas"}
	if _, _, _, err := ClaudeCode().Aplicar(nil, v); !errors.Is(err, errOrdenConPorcentaje) {
		t.Errorf("con %%: err = %v", err)
	}
}

func TestJSONRoto(t *testing.T) {
	roto := datosPrueba(t, "roto.json")
	for _, c := range []struct{ nombre, contenido string }{
		{"cortado", string(roto)},
		{"la raíz no es un objeto", `[1,2]`},
		{"env no es un objeto", `{"env":"x"}`},
	} {
		datos, ruta, h := herramientaTemporal(t, "claude-code", []byte(c.contenido), "settings.json")
		if _, err := Aplicar(datos, h, AlcanceGlobal, "", valoresClaude()); !errors.Is(err, ErrArchivoIlegible) {
			t.Errorf("%s: err = %v, quería ErrArchivoIlegible", c.nombre, err)
		}
		if got := leerTexto(t, ruta); got != c.contenido {
			t.Errorf("%s: el archivo cambió: %q", c.nombre, got)
		}
		if _, hay, _ := Leer(datos, "claude-code"); hay {
			t.Errorf("%s: quedó una anotación", c.nombre)
		}
	}
}

func TestRutasDeLista(t *testing.T) {
	casa, xdg, claude := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	ruta := func(id string, alcance Alcance, proyecto string) string {
		for _, h := range Lista() {
			if h.ID == id {
				r, err := h.Ruta(alcance, proyecto)
				if err != nil {
					t.Fatal(err)
				}
				return r
			}
		}
		t.Fatalf("%s no está en Lista", id)
		return ""
	}
	if got, want := ruta("claude-code", AlcanceGlobal, ""), filepath.Join(casa, ".claude", "settings.json"); got != want {
		t.Errorf("claude-code = %s, quería %s", got, want)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", claude)
	if got, want := ruta("claude-code", AlcanceGlobal, ""), filepath.Join(claude, "settings.json"); got != want {
		t.Errorf("claude-code con CLAUDE_CONFIG_DIR = %s, quería %s", got, want)
	}
	if got, want := ruta("claude-code", AlcanceProyecto, filepath.Join(casa, "p")), filepath.Join(casa, "p", ".claude", "settings.local.json"); got != want {
		t.Errorf("claude-code en proyecto = %s, quería %s", got, want)
	}
	if _, err := Lista()[0].Ruta(AlcanceProyecto, ""); !errors.Is(err, errSinProyecto) {
		t.Errorf("sin proyecto: err = %v", err)
	}
	if got, want := ruta("opencode", AlcanceGlobal, ""), filepath.Join(casa, ".config", "opencode", "opencode.json"); got != want {
		t.Errorf("opencode = %s, quería %s", got, want)
	}
	if err := os.MkdirAll(filepath.Join(casa, ".config", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	jsonc := filepath.Join(casa, ".config", "opencode", "opencode.jsonc")
	if err := os.WriteFile(jsonc, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ruta("opencode", AlcanceGlobal, ""); got != jsonc {
		t.Errorf("opencode con .jsonc = %s, quería %s", got, jsonc)
	}
	if got, want := ruta("kilo", AlcanceGlobal, ""), filepath.Join(casa, ".config", "kilo", "kilo.jsonc"); got != want {
		t.Errorf("kilo = %s, quería %s", got, want)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := ruta("kilo", AlcanceGlobal, ""), filepath.Join(xdg, "kilo", "kilo.jsonc"); got != want {
		t.Errorf("kilo con XDG_CONFIG_HOME = %s, quería %s", got, want)
	}
	// por herramienta: si la clave va en el archivo (si no, en el almacén y una orden la entrega) y si admite proyecto
	claveEnArchivo := map[string]bool{"claude-code": false, "codex": false, "opencode": true, "kilo": true}
	for _, h := range Lista() {
		want, conocida := claveEnArchivo[h.ID]
		if !conocida || h.ClaveEnArchivo != want || h.EnProyecto != (h.ID == "claude-code") || (h.Gestionado != nil) != (h.ID == "claude-code") {
			t.Errorf("%s: EnProyecto/ClaveEnArchivo/Gestionado mal puestos", h.ID)
		}
	}
}
