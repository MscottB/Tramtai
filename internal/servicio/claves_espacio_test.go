package servicio

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/api"
	"tramtai/internal/nucleo"
)

// estadoClaveUI pide a la Web UI el estado de la clave del espacio de la sesión.
func estadoClaveUI(t *testing.T, n *navegador) (EstadoClave, string) {
	t.Helper()
	r, cuerpo := n.pedir("GET", "/api/v1/instancia/token", "")
	var e EstadoClave
	if err := json.Unmarshal([]byte(cuerpo), &e); r.StatusCode != 200 || err != nil {
		t.Fatalf("estado de la clave: %d %s %v", r.StatusCode, cuerpo, err)
	}
	return e, cuerpo
}

// claveEntra dice si la clave entra por la API nativa, por la compatible con OpenAI y por la de Anthropic; las tres
// tienen que coincidir. En la de Anthropic, con el cuerpo vacío «{}», entrar es cualquier respuesta que no sea 401.
func claveEntra(t *testing.T, base, clave string) bool {
	t.Helper()
	nativa, _ := pedir(t, base, "GET", "/api/v1/modelos", clave, "")
	openai, _ := pedir(t, base, "GET", "/openai/v1/models", clave, "")
	anthropic, _ := pedir(t, base, "POST", "/anthropic/v1/messages", clave, `{}`)
	if nativa.StatusCode != openai.StatusCode || (nativa.StatusCode != 200 && nativa.StatusCode != 401) {
		t.Fatalf("la clave en la API nativa y en la de OpenAI: %d %d", nativa.StatusCode, openai.StatusCode)
	}
	if entra := nativa.StatusCode == 200; entra == (anthropic.StatusCode == 401) {
		t.Fatalf("la clave en la API nativa (%d) y en la de Anthropic (%d) no coinciden", nativa.StatusCode, anthropic.StatusCode)
	}
	return nativa.StatusCode == 200
}

// Un espacio recién activado no tiene clave: nadie entra con una inventada y no se crea archivo hasta que se pide.
func TestClaveEspacioNuevo(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	if r, _ := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", "tt_"+strings.Repeat("0", 64), hola); r.StatusCode != 401 {
		t.Fatalf("sin clave creada no entra ninguna: %d", r.StatusCode)
	}
	ruta := RutaTokenInstancia(s.opc.Datos, "global")
	if e, _ := estadoClaveUI(t, abrirSesion(t, s, ts.URL)); e != (EstadoClave{Origen: "archivo", Archivo: ruta}) {
		t.Fatalf("estado de un espacio sin clave: %+v", e)
	}
	if _, err := os.Stat(ruta); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("activar y consultar no crean la clave: %v", err)
	}
}

// Crear y regenerar: la clave sale una vez y entra, en disco solo queda su huella y el estado nunca la devuelve; la
// anterior deja de valer al momento.
func TestClaveEspacioCrearYRegenerar(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	n := abrirSesion(t, s, ts.URL)
	if r, _ := n.pedir("POST", "/api/v1/instancia/token", ""); r.StatusCode != 403 {
		t.Fatalf("crear la clave exige el CSRF: %d", r.StatusCode)
	}
	crear := func() string {
		t.Helper()
		r, cuerpo := n.pedir("POST", "/api/v1/instancia/token", "", "X-CSRF-Token", n.csrf)
		var creada struct{ Token, Archivo string }
		if err := json.Unmarshal([]byte(cuerpo), &creada); r.StatusCode != 200 || err != nil || !strings.HasPrefix(creada.Token, "tt_") || len(creada.Token) != len("tt_")+64 || creada.Archivo == "" {
			t.Fatalf("crear la clave: %d %s %v", r.StatusCode, cuerpo, err)
		}
		return creada.Token
	}
	primera := crear()
	if !claveEntra(t, ts.URL, primera) {
		t.Fatal("la clave recién creada tiene que entrar")
	}
	if datos, _ := os.ReadFile(RutaTokenInstancia(s.opc.Datos, "global")); strings.TrimSpace(string(datos)) != api.HuellaToken(primera) {
		t.Fatalf("en el archivo solo va la huella: %q", datos)
	}
	if e, cuerpo := estadoClaveUI(t, n); !e.TieneClave || strings.Contains(cuerpo, primera) {
		t.Fatalf("el estado dice que hay clave, no cuál es: %s", cuerpo)
	}
	segunda := crear()
	if primera == segunda || claveEntra(t, ts.URL, primera) || !claveEntra(t, ts.URL, segunda) {
		t.Fatal("regenerar deja fuera la anterior y deja entrar la nueva")
	}
}

// Revisión 1: un archivo con la clave en claro (de antes de las huellas) pasa a huella al activar y la misma clave sigue
// entrando, también tras reiniciar el servicio.
func TestClaveEspacioMigradaTrasReinicio(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	clave := "tt_" + strings.Repeat("cd", 32)
	ruta := RutaTokenInstancia(s.opc.Datos, "global")
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(clave+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	if !claveEntra(t, ts.URL, clave) {
		t.Fatal("la clave de antes tiene que entrar")
	}
	if datos, _ := os.ReadFile(ruta); strings.TrimSpace(string(datos)) != api.HuellaToken(clave) {
		t.Fatalf("el archivo tenía que quedar con la huella: %q", datos)
	}
	if err := s.Desactivar("global"); err != nil {
		t.Fatal(err)
	}
	reiniciado, err := Nuevo(s.opc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reiniciado.Cerrar)
	if _, err := reiniciado.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	ts2 := httptest.NewServer(reiniciado)
	t.Cleanup(ts2.Close)
	if !claveEntra(t, ts2.URL, clave) {
		t.Fatal("tras reiniciar, la misma clave tiene que seguir entrando")
	}
}

// La clave de la variable de entorno entra, se cambia allí y no se crea archivo.
func TestClaveEspacioVariable(t *testing.T) {
	clave := strings.Repeat("ab", 32)
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", clave)
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	if !claveEntra(t, ts.URL, clave) {
		t.Fatal("la clave de la variable tiene que entrar")
	}
	n := abrirSesion(t, s, ts.URL)
	if e, _ := estadoClaveUI(t, n); e != (EstadoClave{TieneClave: true, Origen: "variable", Variable: "TRAMTAI_TOKEN_GLOBAL"}) {
		t.Fatalf("estado de la clave por variable: %+v", e)
	}
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/token", "", "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("la clave de la variable no se regenera desde aquí: %d %s", r.StatusCode, cuerpo)
	}
	if _, err := os.Stat(RutaTokenInstancia(s.opc.Datos, "global")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("con la variable no se crea archivo: %v", err)
	}
}

// Regenerar la clave de un espacio que ya se olvidó da error y no vuelve a escribir el archivo que Olvidar borró.
func TestRotarClaveDeEspacioOlvidado(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.AbrirProyecto(t.TempDir(), AbrirOpciones{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Olvidar(inst.ID, espacioDe(t, s, inst.ID).Nombre); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RotarTokenInstancia(inst); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("regenerar la clave de un espacio olvidado: %v", err)
	}
	if _, err := os.Stat(RutaTokenInstancia(s.opc.Datos, inst.ID)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("no se vuelve a crear el archivo de la clave: %v", err)
	}
}

// Revisión 3: olvidar un proyecto y volver a abrirlo lo deja sin clave, y la anterior ya no entra.
func TestClaveEspacioOlvidado(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	raiz := t.TempDir()
	inst, err := s.AbrirProyecto(raiz, AbrirOpciones{})
	if err != nil {
		t.Fatal(err)
	}
	clave := claveDe(t, s, inst)
	if !claveEntra(t, ts.URL, clave) {
		t.Fatal("la clave tiene que entrar")
	}
	if err := s.Olvidar(inst.ID, espacioDe(t, s, inst.ID).Nombre); err != nil {
		t.Fatal(err)
	}
	otra, err := s.AbrirProyecto(raiz, AbrirOpciones{})
	if err != nil {
		t.Fatal(err)
	}
	if otra.huella != "" {
		t.Fatalf("un espacio vuelto a abrir no hereda la clave: %q", otra.huella)
	}
	if claveEntra(t, ts.URL, clave) {
		t.Fatal("la clave anterior ya no tiene que entrar")
	}
}
