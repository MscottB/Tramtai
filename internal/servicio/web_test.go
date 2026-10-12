package servicio

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// navegador es un cliente con cookies que no sigue redirecciones, como un navegador visto desde las pruebas.
type navegador struct {
	t    *testing.T
	base string
	http *http.Client
	csrf string
}

func nuevoNavegador(t *testing.T, base string) *navegador {
	jar, _ := cookiejar.New(nil)
	return &navegador{t: t, base: base, http: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}}
}

func (n *navegador) pedir(metodo, ruta, cuerpo string, cabeceras ...string) (*http.Response, string) {
	n.t.Helper()
	req, _ := http.NewRequest(metodo, n.base+ruta, strings.NewReader(cuerpo))
	for i := 0; i+1 < len(cabeceras); i += 2 {
		req.Header.Set(cabeceras[i], cabeceras[i+1])
	}
	resp, err := n.http.Do(req)
	if err != nil {
		n.t.Fatal(err)
	}
	defer resp.Body.Close()
	datos, _ := io.ReadAll(resp.Body)
	return resp, string(datos)
}

func TestWebUI(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.opc.Web = fstest.MapFS{"index.html": {Data: []byte("<html>tramtai</html>")}, "assets/app.js": {Data: []byte("console.log(1)")}}
	global, _ := s.ActivarGlobal()
	n := nuevoNavegador(t, ts.URL)

	if r, cuerpo := n.pedir("GET", "/trabajos", ""); r.StatusCode != 200 || cuerpo != "<html>tramtai</html>" || r.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("la navegación sirve index.html con CSP: %d %q", r.StatusCode, cuerpo)
	}
	if r, _ := n.pedir("GET", "/assets/app.js", ""); r.StatusCode != 200 || !strings.Contains(r.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("archivo compilado: %d", r.StatusCode)
	}
	if r, _ := n.pedir("GET", "/ui/sesion", ""); r.StatusCode != 401 {
		t.Fatalf("sin sesión: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/ui/sesion", `{"token":"incorrecto"}`); r.StatusCode != 401 {
		t.Fatalf("token incorrecto: %d", r.StatusCode)
	}
	r, cuerpo := n.pedir("POST", "/ui/sesion", `{"token":"`+global.token+`"}`)
	if r.StatusCode != 200 || !strings.Contains(r.Header.Get("Set-Cookie"), "HttpOnly") || !strings.Contains(r.Header.Get("Set-Cookie"), "SameSite=Strict") {
		t.Fatalf("entrar: %d %s %s", r.StatusCode, cuerpo, r.Header.Get("Set-Cookie"))
	}
	n.csrf = cuerpo[strings.Index(cuerpo, `"csrf":"`)+8:]
	n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]
	if strings.Contains(cuerpo, global.token) {
		t.Fatal("la sesión no debe devolver el token de la instancia")
	}

	if r, cuerpo := n.pedir("GET", "/api/v1/instancia", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"id":"global"`) {
		t.Fatalf("instancia con la cookie: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := n.pedir("POST", "/api/v1/trabajos", hola); r.StatusCode != 403 {
		t.Fatalf("una modificación sin X-CSRF-Token se rechaza: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/api/v1/trabajos", hola, "X-CSRF-Token", n.csrf); r.StatusCode != 202 {
		t.Fatalf("con X-CSRF-Token: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/api/v1/trabajos", hola, "X-CSRF-Token", n.csrf, "Origin", "http://atacante.example"); r.StatusCode != 403 {
		t.Fatalf("un Origin ajeno se rechaza aunque haya sesión: %d", r.StatusCode)
	}

	// Puntos de acceso desde la Web UI: no con el token de aplicaciones, sí con la sesión.
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/puntos", global.token, ""); r.StatusCode != 401 {
		t.Fatalf("el token de aplicaciones no gestiona puntos: %d", r.StatusCode)
	}
	r, cuerpo = n.pedir("POST", "/api/v1/puntos", `{"nombre":"desde_ui","contratos":["openai"],"agente":"simulado"}`, "X-CSRF-Token", n.csrf, "Content-Type", "application/json")
	if r.StatusCode != 201 || !strings.Contains(cuerpo, `"token":"tt_`) || !strings.Contains(cuerpo, "/p/desde_ui") {
		t.Fatalf("crear punto: %d %s", r.StatusCode, cuerpo)
	}
	rutaGlobal, _ := nucleo.RutaGlobal()
	if datos, _ := os.ReadFile(rutaGlobal); !strings.Contains(string(datos), "desde_ui") {
		t.Fatalf("el punto debía guardarse en la configuración global:\n%s", datos)
	}
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", `{"activo":false}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 || !strings.Contains(cuerpo, "desactivado") {
		t.Fatalf("desactivar: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := n.pedir("DELETE", "/api/v1/puntos/desde_ui", "", "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("eliminar: %d", r.StatusCode)
	}

	if r, _ := n.pedir("DELETE", "/ui/sesion", ""); r.StatusCode != 204 {
		t.Fatalf("salir: %d", r.StatusCode)
	}
	if r, _ := n.pedir("GET", "/api/v1/instancia", ""); r.StatusCode != 401 {
		t.Fatalf("tras salir la cookie no vale: %d", r.StatusCode)
	}
}

func TestCodigoDeUnSoloUso(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.opc.Web = fstest.MapFS{"index.html": {Data: []byte("ui")}}
	if _, err := s.CodigoSesion("global"); err == nil {
		t.Fatal("no hay código para una instancia inactiva")
	}
	s.ActivarGlobal()
	codigo, _ := s.CodigoSesion("global")
	n := nuevoNavegador(t, ts.URL)
	if r, _ := n.pedir("GET", "/ui/entrar?codigo="+url.QueryEscape(codigo), ""); r.StatusCode != 303 || r.Header.Get("Location") != "/" {
		t.Fatalf("canjear el código: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	if r, _ := n.pedir("GET", "/ui/sesion", ""); r.StatusCode != 200 {
		t.Fatalf("la cookie queda establecida: %d", r.StatusCode)
	}
	otro := nuevoNavegador(t, ts.URL)
	if r, _ := otro.pedir("GET", "/ui/entrar?codigo="+url.QueryEscape(codigo), ""); r.Header.Get("Location") != "/?error=codigo" {
		t.Fatalf("el código no se puede reutilizar: %s", r.Header.Get("Location"))
	}
	if r, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/servicio/sesiones", s.tokenAdmin, `{"instancia":"global"}`); r.StatusCode != 200 || !strings.Contains(cuerpo, "/ui/entrar?codigo=") {
		t.Fatalf("la administración pide códigos: %d %s", r.StatusCode, cuerpo)
	}
}
