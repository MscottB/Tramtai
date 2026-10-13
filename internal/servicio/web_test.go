package servicio

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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
	r, cuerpo = n.pedir("POST", "/api/v1/puntos", `{"nombre":"desde_ui","grupo":"pruebas","contratos":["openai"],"agente":"simulado"}`, "X-CSRF-Token", n.csrf, "Content-Type", "application/json")
	if r.StatusCode != 201 || !strings.Contains(cuerpo, `"token":"tt_`) || !strings.Contains(cuerpo, "/p/desde_ui") || !strings.Contains(cuerpo, `"grupo":"pruebas"`) {
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

	// Modelo predeterminado: se guarda, se aplica en marcha y el catálogo lo marca.
	if r, _ := n.pedir("POST", "/api/v1/instancia/predeterminado", `{"modelo":"simulado/m2"}`); r.StatusCode != 403 {
		t.Fatalf("fijar el predeterminado exige el CSRF: %d", r.StatusCode)
	}
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/predeterminado", `{"modelo":"perfil/inexistente"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("un perfil desconocido se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/predeterminado", `{"modelo":"simulado/m2"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("fijar el predeterminado: %d %s", r.StatusCode, cuerpo)
	}
	if _, cuerpo := n.pedir("GET", "/api/v1/modelos", ""); !strings.Contains(cuerpo, `"id":"simulado/m2","agente":"simulado","modelo":"m2","origen":"configurado","predeterminado":true`) {
		t.Fatalf("el catálogo marca el predeterminado: %s", cuerpo)
	}
	if datos, _ := os.ReadFile(rutaGlobal); !strings.Contains(string(datos), "modelo: m2") {
		t.Fatalf("el predeterminado debía guardarse en la configuración global:\n%s", datos)
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

func TestEntradaLocalSinToken(t *testing.T) {
	if !mismoUsuario(netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:2")) && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("sin identificación del dueño de la conexión en este sistema")
	}
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	s.opc.Web = fstest.MapFS{"index.html": {Data: []byte("ui")}}
	n := nuevoNavegador(t, ts.URL)
	if r, _ := n.pedir("GET", "/ui/sesion", ""); r.StatusCode != 401 {
		t.Fatalf("con entrada_local desactivada se pide token: %d", r.StatusCode)
	}
	s.opc.EntradaLocal = true
	r, cuerpo := n.pedir("GET", "/ui/sesion", "")
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"local":true`) || !strings.Contains(cuerpo, `"instancias":[]`) {
		t.Fatalf("el mismo usuario entra sin token aunque no haya instancias: %d %s", r.StatusCode, cuerpo)
	}
	n.csrf = cuerpo[strings.Index(cuerpo, `"csrf":"`)+8:]
	n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]
	if r, cuerpo := n.pedir("POST", "/ui/instancias", `{"ambito":"global"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 || !strings.Contains(cuerpo, `"instancia":"global"`) {
		t.Fatalf("activar la global desde la Web UI: %d %s", r.StatusCode, cuerpo)
	}
	pry, _ := s.ActivarProyecto(proyecto)
	r, cuerpo = n.pedir("GET", "/ui/sesion", "")
	n.csrf = cuerpo[strings.Index(cuerpo, `"csrf":"`)+8:]
	n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]
	if !strings.Contains(cuerpo, pry.ID) || !strings.Contains(cuerpo, `"nombre":"Mi Proyecto"`) {
		t.Fatalf("la sesión local lista las instancias: %s", cuerpo)
	}
	if r, _ := n.pedir("POST", "/ui/sesion", `{"instancia":"`+pry.ID+`"}`); r.StatusCode != 401 {
		t.Fatalf("cambiar de instancia exige el CSRF: %d", r.StatusCode)
	}
	if r, cuerpo := n.pedir("POST", "/ui/sesion", `{"instancia":"`+pry.ID+`"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 || !strings.Contains(cuerpo, `"instancia":"`+pry.ID+`"`) {
		t.Fatalf("cambiar de instancia: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("GET", "/api/v1/instancia/token", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, pry.token) {
		t.Fatalf("la sesión ve la clave para conectar aplicaciones: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/instancia/token", pry.token, ""); r.StatusCode != 401 {
		t.Fatalf("el token de aplicaciones no lee claves: %d", r.StatusCode)
	}
	_, cuerpo = n.pedir("GET", "/ui/sesion", "") // cambiar de instancia creó otra sesión con otro CSRF
	n.csrf = cuerpo[strings.Index(cuerpo, `"csrf":"`)+8:]
	n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]
	if r, _ := n.pedir("POST", "/ui/autenticacion", `{"agente":"claude","modo":"suscripcion"}`); r.StatusCode != 401 {
		t.Fatalf("cambiar la autenticación exige el CSRF: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/ui/autenticacion", `{"agente":"gemini","modo":"suscripcion"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("gemini no admite suscripción: %d", r.StatusCode)
	}

	// Con el código de un solo uso de tramtai abrir, desde este mismo equipo, la sesión también es local.
	codigo, _ := s.CodigoSesion("global")
	otro := nuevoNavegador(t, ts.URL)
	otro.pedir("GET", "/ui/entrar?codigo="+url.QueryEscape(codigo), "")
	if _, cuerpo := otro.pedir("GET", "/ui/sesion", ""); !strings.Contains(cuerpo, `"local":true`) {
		t.Fatalf("la entrada con código desde este equipo debe ser local: %s", cuerpo)
	}

	// Detener el servicio desde la Web UI: solo con sesión local y CSRF.
	if r, _ := n.pedir("POST", "/ui/servicio/detener", ""); r.StatusCode != 401 {
		t.Fatalf("detener exige el CSRF: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/ui/servicio/detener", "", "X-CSRF-Token", n.csrf); r.StatusCode != 202 {
		t.Fatalf("detener: %d", r.StatusCode)
	}
	select {
	case <-s.Detenido():
	case <-time.After(2 * time.Second):
		t.Fatal("el servicio no recibió la orden de detenerse")
	}
}

func TestDireccionesDeProc(t *testing.T) {
	tabla := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:4377 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1 1 0000000000000000 100 0 0 10 0
   1: 0100007F:C3A2 0100007F:4377 01 00000000:00000000 00:00000000 00000000  1001        0 2 1 0000000000000000 20 4 30 10 -1`
	uid, ok := uidDeConexion(tabla, netip.MustParseAddrPort("127.0.0.1:50082"), netip.MustParseAddrPort("127.0.0.1:17271"))
	if !ok || uid != 1001 {
		t.Fatalf("uid %d %v", uid, ok)
	}
	if _, ok := uidDeConexion(tabla, netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:17271")); ok {
		t.Fatal("una conexión ajena no coincide")
	}
	if d, ok := direccionProc("00000000000000000000000001000000:1F90"); !ok || d != netip.MustParseAddrPort("[::1]:8080") {
		t.Fatalf("IPv6: %v", d)
	}
}
