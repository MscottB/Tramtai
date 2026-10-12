//go:build m0b

// Package m0b valida los modos con clave de API de cada adaptador (hito M0b, sección 2.4 del documento maestro).
// Cada prueba se omite si falta su clave. Las claves solo se leen del entorno, cada CLI recibe únicamente la suya
// y nunca se imprimen. Instrucciones en docs/M0B.md.
//
//	ANTHROPIC_API_KEY               Claude Code en modo --bare (solo admite clave de API y omite CLAUDE.md)
//	CODEX_API_KEY u OPENAI_API_KEY  Codex con un CODEX_HOME privado y vacío
//	GEMINI_API_KEY                  Gemini API directa y Antigravity con modelProvider gemini
//
// Ejecutar: go test -tags m0b -v -count=1 ./pruebas/m0b
package m0b

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const preguntaReglas = "Si en tu contexto hay contenido de algún archivo CLAUDE.md, AGENTS.md o GEMINI.md, copia literalmente su primera línea. Si no hay ninguno, responde exactamente NINGUNO."

const baseGemini = "https://generativelanguage.googleapis.com/v1beta"

// variablesClave se retiran del entorno heredado para que cada CLI reciba solo su propia credencial.
var variablesClave = []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "CODEX_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}

func clave(t *testing.T, nombres ...string) string {
	t.Helper()
	for _, nombre := range nombres {
		if valor := os.Getenv(nombre); valor != "" {
			return valor
		}
	}
	t.Skipf("%s no definida: prueba omitida", strings.Join(nombres, " u "))
	return ""
}

func binario(variable, predeterminado string) string {
	if valor := os.Getenv(variable); valor != "" {
		return valor
	}
	return predeterminado
}

// entorno hereda el entorno sin credenciales y añade solo las variables indicadas.
func entorno(extra ...string) []string {
	quitar := map[string]bool{}
	for _, nombre := range variablesClave {
		quitar[nombre] = true
	}
	for _, par := range extra {
		nombre, _, _ := strings.Cut(par, "=")
		quitar[strings.ToUpper(nombre)] = true
	}
	var resultado []string
	for _, par := range os.Environ() {
		if nombre, _, _ := strings.Cut(par, "="); !quitar[strings.ToUpper(nombre)] {
			resultado = append(resultado, par)
		}
	}
	return append(resultado, extra...)
}

func redactar(texto string, secretos ...string) string {
	for _, secreto := range secretos {
		if secreto != "" {
			texto = strings.ReplaceAll(texto, secreto, "[REDACTADO]")
		}
	}
	if len(texto) > 2000 {
		texto = texto[:2000] + "…"
	}
	return texto
}

// ejecutar lanza un CLI con stdin vacío; si falla, muestra stderr redactado.
func ejecutar(t *testing.T, dir string, env []string, secreto string, programa string, args ...string) (string, error) {
	t.Helper()
	ctx, cancelar := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancelar()
	cmd := exec.CommandContext(ctx, programa, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = dir, env, strings.NewReader("")
	var salida, errores bytes.Buffer
	cmd.Stdout, cmd.Stderr = &salida, &errores
	err := cmd.Run()
	if err != nil {
		t.Logf("stderr de %s: %s", filepath.Base(programa), redactar(errores.String(), secreto))
	}
	return salida.String(), err
}

func comprobarAislado(t *testing.T, respuesta string) {
	t.Helper()
	if !strings.Contains(strings.ToUpper(respuesta), "NINGUNO") {
		t.Errorf("el agente cargó archivos de reglas o no respondió lo esperado: %q", redactar(respuesta))
	}
}

// TestClaudeClaveAPI: --bare solo admite ANTHROPIC_API_KEY y omite CLAUDE.md, así que un éxito prueba
// a la vez el modo clave y el aislamiento.
func TestClaudeClaveAPI(t *testing.T) {
	k := clave(t, "ANTHROPIC_API_KEY")
	args := []string{"-p", preguntaReglas, "--bare", "--output-format", "json", "--tools", "", "--model", binario("M0B_CLAUDE_MODELO", "haiku")}
	salida, err := ejecutar(t, t.TempDir(), entorno("ANTHROPIC_API_KEY="+k), k, binario("M0B_CLAUDE", "claude"), args...)
	if err != nil {
		t.Fatalf("claude falló: %v", err)
	}
	var r struct {
		Result     string         `json:"result"`
		IsError    bool           `json:"is_error"`
		Usage      map[string]any `json:"usage"`
		ModelUsage map[string]any `json:"modelUsage"`
	}
	if err := json.Unmarshal([]byte(salida), &r); err != nil {
		t.Fatalf("la salida no es JSON: %v", err)
	}
	if r.IsError {
		t.Fatalf("claude devolvió error: %s", redactar(r.Result, k))
	}
	comprobarAislado(t, r.Result)
	if len(r.Usage) == 0 || len(r.ModelUsage) == 0 {
		t.Error("faltan uso de tokens o modelo efectivo")
	}
	var modelos []string
	for modelo := range r.ModelUsage {
		modelos = append(modelos, modelo)
	}
	t.Logf("modelo efectivo: %v; uso: %v", modelos, r.Usage)
}

// TestCodexHomePrivado: con un CODEX_HOME vacío no existe AGENTS.md global; la clave llega por CODEX_API_KEY.
func TestCodexHomePrivado(t *testing.T) {
	k := clave(t, "CODEX_API_KEY", "OPENAI_API_KEY")
	home, espacio := t.TempDir(), t.TempDir()
	args := []string{"exec", "--json", "--ephemeral", "--skip-git-repo-check", "-C", espacio,
		"-c", `approval_policy="never"`, "-c", "features.shell_tool=false", "-c", `web_search="disabled"`,
		"-c", `history.persistence="none"`, "--ignore-rules"}
	if modelo := os.Getenv("M0B_CODEX_MODELO"); modelo != "" {
		args = append(args, "-m", modelo)
	}
	args = append(args, preguntaReglas)
	salida, err := ejecutar(t, espacio, entorno("CODEX_HOME="+home, "CODEX_API_KEY="+k), k, binario("M0B_CODEX", "codex"), args...)
	if err != nil {
		t.Fatalf("codex falló: %v", err)
	}
	var texto string
	var uso map[string]any
	for _, linea := range strings.Split(salida, "\n") {
		var e struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
			Usage map[string]any `json:"usage"`
		}
		if json.Unmarshal([]byte(linea), &e) != nil {
			continue
		}
		if e.Type == "item.completed" && e.Item.Type == "agent_message" {
			texto = e.Item.Text
		}
		if e.Type == "turn.completed" {
			uso = e.Usage
		}
	}
	comprobarAislado(t, texto)
	if len(uso) == 0 {
		t.Error("codex no informó el uso de tokens")
	}
	t.Logf("uso: %v", uso)
	if _, err := os.Stat(filepath.Join(home, "auth.json")); err == nil {
		t.Log("aviso: codex escribió auth.json en el CODEX_HOME privado; Tramtai debe tratar ese directorio como secreto")
	}
}

func peticionGemini(k, metodo, url string, cuerpo any) (*http.Response, error) {
	var lector io.Reader
	if cuerpo != nil {
		datos, err := json.Marshal(cuerpo)
		if err != nil {
			return nil, err
		}
		lector = bytes.NewReader(datos)
	}
	peticion, err := http.NewRequest(metodo, url, lector)
	if err != nil {
		return nil, err
	}
	peticion.Header.Set("x-goog-api-key", k) // la clave va en cabecera, nunca en la URL
	peticion.Header.Set("Content-Type", "application/json")
	return (&http.Client{Timeout: 2 * time.Minute}).Do(peticion)
}

func exigirOK(t *testing.T, respuesta *http.Response, err error, k, que string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", que, err)
	}
	if respuesta.StatusCode != http.StatusOK {
		cuerpo, _ := io.ReadAll(respuesta.Body)
		t.Fatalf("%s: HTTP %d: %s", que, respuesta.StatusCode, redactar(string(cuerpo), k))
	}
}

// modeloGemini usa M0B_GEMINI_MODELO o el primer modelo "flash" que admita generateContent.
func modeloGemini(t *testing.T, k string) string {
	t.Helper()
	if modelo := os.Getenv("M0B_GEMINI_MODELO"); modelo != "" {
		return "models/" + strings.TrimPrefix(modelo, "models/")
	}
	respuesta, err := peticionGemini(k, http.MethodGet, baseGemini+"/models?pageSize=200", nil)
	exigirOK(t, respuesta, err, k, "listar modelos")
	defer respuesta.Body.Close()
	var lista struct {
		Models []struct {
			Name    string   `json:"name"`
			Metodos []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(respuesta.Body).Decode(&lista); err != nil {
		t.Fatal(err)
	}
	var candidato string
	for _, m := range lista.Models {
		if !slices.Contains(m.Metodos, "generateContent") {
			continue
		}
		if strings.Contains(m.Name, "flash") {
			return m.Name
		}
		if candidato == "" {
			candidato = m.Name
		}
	}
	if candidato == "" {
		t.Fatal("ningún modelo admite generateContent")
	}
	return candidato
}

func contenido(texto string) []any {
	return []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": texto}}}}
}

// TestGeminiAPIDirecta valida el adaptador de API directa: salida estructurada, uso, modelo efectivo y streaming SSE.
func TestGeminiAPIDirecta(t *testing.T) {
	k := clave(t, "GEMINI_API_KEY")
	modelo := modeloGemini(t, k)
	cuerpo := map[string]any{
		"contents": contenido("Dame la capital de Chile y su población aproximada en millones."),
		"generationConfig": map[string]any{"responseMimeType": "application/json", "responseSchema": map[string]any{
			"type": "OBJECT", "required": []string{"ciudad", "habitantes_millones"},
			"properties": map[string]any{"ciudad": map[string]any{"type": "STRING"}, "habitantes_millones": map[string]any{"type": "NUMBER"}},
		}},
	}
	respuesta, err := peticionGemini(k, http.MethodPost, baseGemini+"/"+modelo+":generateContent", cuerpo)
	exigirOK(t, respuesta, err, k, "generateContent")
	defer respuesta.Body.Close()
	var r struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Uso          map[string]any `json:"usageMetadata"`
		ModelVersion string         `json:"modelVersion"`
	}
	if err := json.NewDecoder(respuesta.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		t.Fatal("respuesta sin contenido")
	}
	texto := r.Candidates[0].Content.Parts[0].Text
	var datos struct {
		Ciudad     string  `json:"ciudad"`
		Habitantes float64 `json:"habitantes_millones"`
	}
	if err := json.Unmarshal([]byte(texto), &datos); err != nil || datos.Ciudad == "" {
		t.Errorf("salida estructurada no válida: %q", texto)
	}
	if len(r.Uso) == 0 {
		t.Error("sin usageMetadata")
	}
	t.Logf("modelo: %s; efectivo: %s; uso: %v; salida: %s", modelo, r.ModelVersion, r.Uso, texto)

	flujo, err := peticionGemini(k, http.MethodPost, baseGemini+"/"+modelo+":streamGenerateContent?alt=sse",
		map[string]any{"contents": contenido("Escribe los números del 1 al 60 separados por comas.")})
	exigirOK(t, flujo, err, k, "streamGenerateContent")
	defer flujo.Body.Close()
	lector := bufio.NewScanner(flujo.Body)
	lector.Buffer(make([]byte, 1<<20), 1<<20)
	fragmentos := 0
	for lector.Scan() {
		if strings.HasPrefix(lector.Text(), "data:") {
			fragmentos++
		}
	}
	if fragmentos == 0 {
		t.Error("el streaming SSE no devolvió fragmentos")
	}
	t.Logf("fragmentos SSE: %d", fragmentos)
}

// TestAntigravityProveedorGemini comprueba que agy usa la clave de la Gemini API y no la sesión OAuth:
// con una clave inválida debe fallar y con la válida debe responder. HOME y USERPROFILE apuntan a un
// directorio privado, como hará Tramtai.
func TestAntigravityProveedorGemini(t *testing.T) {
	k := clave(t, "GEMINI_API_KEY")
	home, espacio := t.TempDir(), t.TempDir()
	ajustes := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(ajustes, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ajustes, "settings.json"), []byte(`{"modelProvider": "gemini"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-p", preguntaReglas, "--output-format", "json"}
	if modelo := os.Getenv("M0B_AGY_MODELO"); modelo != "" {
		args = append(args, "--model", modelo)
	}
	lanzar := func(claveUsada string) (estado, respuesta string, uso map[string]any, err error) {
		salida, err := ejecutar(t, espacio, entorno("USERPROFILE="+home, "HOME="+home, "GEMINI_API_KEY="+claveUsada), k, binario("M0B_AGY", "agy"), args...)
		var r struct {
			Status   string         `json:"status"`
			Response string         `json:"response"`
			Usage    map[string]any `json:"usage"`
		}
		_ = json.Unmarshal([]byte(salida), &r)
		return r.Status, r.Response, r.Usage, err
	}

	if estado, _, _, err := lanzar("clave-invalida-m0b"); err == nil && estado == "SUCCESS" {
		t.Error("agy respondió con una clave inválida: sigue usando la sesión OAuth y no modelProvider gemini")
	}
	estado, respuesta, uso, err := lanzar(k)
	if err != nil || estado != "SUCCESS" {
		t.Fatalf("agy con la clave válida no respondió: estado %q, %v", estado, err)
	}
	comprobarAislado(t, respuesta)
	if len(uso) == 0 {
		t.Error("agy no informó el uso de tokens")
	}
	t.Logf("uso: %v", uso)
}
