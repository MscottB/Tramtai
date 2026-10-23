package servicio

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	// La clave que usan las aplicaciones no abre la Web UI: se entra con el código de un solo uso de la app de
	// escritorio, la terminal o tramtai abrir (o, en el mismo equipo y usuario, sin nada).
	for _, token := range []string{"incorrecto", global.token} {
		if r, _ := n.pedir("POST", "/ui/sesion", `{"token":"`+token+`"}`); r.StatusCode != 401 {
			t.Fatalf("la clave %q no debe abrir la Web UI: %d", token, r.StatusCode)
		}
	}
	codigo, _ := s.CodigoSesion("global")
	if r, _ := n.pedir("GET", "/ui/entrar?codigo="+codigo, ""); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("entrar con código: %d", r.StatusCode)
	}
	r, cuerpo := n.pedir("GET", "/ui/sesion", "")
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"local":true`) {
		t.Fatalf("la sesión con código desde este equipo es local: %d %s", r.StatusCode, cuerpo)
	}
	// Aunque no se pueda comprobar el usuario de la conexión (aquí, con entrada_local desactivada), la sesión
	// abierta con código desde este equipo puede usar los controles locales.
	// Dos veces: activar la instancia crea otra sesión, que debe conservar que se abrió con código.
	for i := range 2 {
		_, cuerpoSesion := n.pedir("GET", "/ui/sesion", "")
		csrfCodigo := cuerpoSesion[strings.Index(cuerpoSesion, `"csrf":"`)+8:]
		csrfCodigo = csrfCodigo[:strings.Index(csrfCodigo, `"`)]
		if r, cuerpo := n.pedir("POST", "/ui/instancias", `{"ambito":"global"}`, "X-CSRF-Token", csrfCodigo); r.StatusCode != 200 {
			t.Fatalf("vuelta %d: la sesión local por código debe poder usar los controles locales: %d %s", i, r.StatusCode, cuerpo)
		}
	}
	r, cuerpo = n.pedir("GET", "/ui/sesion", "") // activar la instancia creó otra sesión con otro CSRF
	// La Web UI explica dónde guardar las claves de proveedor según el almacén de este sistema.
	if !strings.Contains(cuerpo, `"almacen":{"disponible":`) {
		t.Fatalf("la sesión indica el almacén de claves del sistema: %s", cuerpo)
	}
	n.csrf = cuerpo[strings.Index(cuerpo, `"csrf":"`)+8:]
	n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]
	if strings.Contains(cuerpo, global.token) {
		t.Fatal("la sesión no debe devolver el token de la instancia")
	}
	// Con la búsqueda de actualizaciones desactivada (así arranca el entorno), las novedades no consultan GitHub.
	if r, cuerpo := n.pedir("GET", "/ui/actualizacion/novedades?desde=v0.1.0", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"versiones":[]`) {
		t.Fatalf("sin búsqueda no se consulta GitHub: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("GET", "/ui/actualizacion/novedades?desde=v0.1.0&hasta=v0.2.0", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"versiones":[]`) {
		t.Fatalf("tampoco con hasta: %d %s", r.StatusCode, cuerpo)
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
	// La clave se ve al crearla y al regenerarla; después solo queda su huella y la anterior deja de valer.
	var creado struct{ Token string }
	_ = json.Unmarshal([]byte(cuerpo), &creado)
	if r, _ := pedir(t, ts.URL, "GET", "/p/desde_ui/openai/v1/models", creado.Token, ""); r.StatusCode != 200 {
		t.Fatalf("la clave de creación debe valer: %d", r.StatusCode)
	}
	if r, _ := n.pedir("GET", "/api/v1/puntos/desde_ui/token", ""); r.StatusCode != 404 {
		t.Fatalf("la clave de un punto no se puede volver a ver: %d", r.StatusCode)
	}
	r, cuerpo = n.pedir("POST", "/api/v1/puntos/desde_ui/token", "", "X-CSRF-Token", n.csrf)
	var regenerado struct{ Token string }
	_ = json.Unmarshal([]byte(cuerpo), &regenerado)
	if r.StatusCode != 200 || !strings.HasPrefix(regenerado.Token, "tt_") || regenerado.Token == creado.Token {
		t.Fatalf("regenerar: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/p/desde_ui/openai/v1/models", creado.Token, ""); r.StatusCode != 401 {
		t.Fatalf("la clave anterior deja de valer: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/p/desde_ui/openai/v1/models", regenerado.Token, ""); r.StatusCode != 200 {
		t.Fatalf("la clave regenerada vale: %d", r.StatusCode)
	}
	if datos, _ := os.ReadFile(RutaTokenPunto(s.opc.Datos, "desde_ui")); strings.Contains(string(datos), regenerado.Token) {
		t.Fatal("el archivo guarda la huella, no la clave")
	}
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", `{"activo":false}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 || !strings.Contains(cuerpo, "desactivado") {
		t.Fatalf("desactivar: %d %s", r.StatusCode, cuerpo)
	}
	// El modelo de una API ya creada se cambia con un identificador del catálogo; "" vuelve al del espacio.
	for _, caso := range [][2]string{
		{`{"modelo":"simulado/haiku","forzar_modelo":true,"modelos_permitidos":["simulado"]}`, `"agente":"simulado","modelo":"haiku","modelos_permitidos":["simulado"],"forzar_modelo":true`},
		{`{"modelo":"perfil/rapido"}`, `"contratos":["openai"],"perfil_modelo":"rapido","modelos_permitidos":["simulado"],"forzar_modelo":true`},
		{`{"modelo":"","modelos_permitidos":[]}`, `"contratos":["openai"],"activo"`},
	} {
		cuerpoPatch, esperado := caso[0], caso[1]
		r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", cuerpoPatch, "X-CSRF-Token", n.csrf)
		if r.StatusCode != 200 || !strings.Contains(cuerpo, esperado) {
			t.Fatalf("PATCH %s: %d %s; se esperaba %s", cuerpoPatch, r.StatusCode, cuerpo, esperado)
		}
	}
	// Receta: esquema y respaldo se fijan y se quitan; un esquema no válido no toca la configuración.
	for _, caso := range [][2]string{
		{`{"esquema":{"type":"object"},"respaldo":["simulado/m2"]}`, `"esquema":{"type":"object"},"respaldo":["simulado/m2"]`},
		{`{"esquema":null}`, `"respaldo":["simulado/m2"]`},
		{`{"esquema":{"type":"object"},"reintentos_salida":2}`, `"reintentos_salida":2`},
	} {
		r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", caso[0], "X-CSRF-Token", n.csrf)
		if r.StatusCode != 200 || !strings.Contains(cuerpo, caso[1]) {
			t.Fatalf("PATCH %s: %d %s; se esperaba %s", caso[0], r.StatusCode, cuerpo, caso[1])
		}
		if caso[0] == `{"esquema":null}` && strings.Contains(cuerpo, `"esquema"`) {
			t.Fatalf("esquema null debía quitarlo: %s", cuerpo)
		}
	}
	antes, _ := os.ReadFile(rutaGlobal)
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", `{"esquema":{"type":5}}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "configuracion_invalida") {
		t.Fatalf("un esquema no válido se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", `{"reintentos_salida":4}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "reintentos_salida") {
		t.Fatalf("reintentos_salida fuera de rango se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	// Quitar el esquema dejando correcciones: no habría nada que corregir.
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", `{"esquema":null}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "reintentos_salida") {
		t.Fatalf("reintentos_salida sin esquema se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	if despues, _ := os.ReadFile(rutaGlobal); string(despues) != string(antes) {
		t.Fatal("un PATCH rechazado no debe cambiar la configuración")
	}
	// Un respaldo con el que toda petición fallaría se rechaza al guardar: agente desconocido o no permitido.
	for _, cuerpoPatch := range []string{`{"respaldo":["nadie"]}`, `{"modelos_permitidos":["simulado/m1"],"respaldo":["simulado/m2"]}`} {
		if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", cuerpoPatch, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "configuracion_invalida") {
			t.Fatalf("PATCH %s debía rechazarse: %d %s", cuerpoPatch, r.StatusCode, cuerpo)
		}
	}
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/desde_ui", `{"modelo":"inexistente/x"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "solicitud_invalida") {
		t.Fatalf("un agente desconocido se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := n.pedir("DELETE", "/api/v1/puntos/desde_ui", "", "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("eliminar: %d", r.StatusCode)
	}
	if _, err := os.Stat(RutaTokenPunto(s.opc.Datos, "desde_ui")); !os.IsNotExist(err) {
		t.Fatalf("eliminar el punto borra su clave: %v", err)
	}

	// La clave de la instancia se regenera en marcha: la anterior deja de valer sin reiniciar.
	anterior := global.token
	r, cuerpo = n.pedir("POST", "/api/v1/instancia/token", "", "X-CSRF-Token", n.csrf)
	var rotada struct{ Token string }
	_ = json.Unmarshal([]byte(cuerpo), &rotada)
	if r.StatusCode != 200 || !strings.HasPrefix(rotada.Token, "tt_") || rotada.Token == anterior {
		t.Fatalf("regenerar la clave de la instancia: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/modelos", anterior, ""); r.StatusCode != 401 {
		t.Fatalf("la clave anterior de la instancia deja de valer: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/modelos", rotada.Token, ""); r.StatusCode != 200 {
		t.Fatalf("la clave nueva de la instancia vale: %d", r.StatusCode)
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
	// Con su esfuerzo: un nivel desconocido se rechaza y uno válido se guarda y lo indica el catálogo.
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/predeterminado", `{"modelo":"simulado/m2","esfuerzo":"muchisimo"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("un esfuerzo desconocido se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/predeterminado", `{"modelo":"simulado/m2","esfuerzo":"alto"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("fijar el esfuerzo predeterminado: %d %s", r.StatusCode, cuerpo)
	}
	if _, cuerpo := n.pedir("GET", "/api/v1/modelos", ""); !strings.Contains(cuerpo, `"esfuerzo_espacio":"alto"`) {
		t.Fatalf("el catálogo indica el esfuerzo del espacio: %s", cuerpo)
	}
	if datos, _ := os.ReadFile(rutaGlobal); !strings.Contains(string(datos), "esfuerzo: alto") {
		t.Fatalf("el esfuerzo predeterminado debía guardarse:\n%s", datos)
	}
	// Solo el esfuerzo (modelo vacío): no toca el modelo.
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/predeterminado", `{"modelo":"","esfuerzo":"bajo"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("fijar solo el esfuerzo: %d %s", r.StatusCode, cuerpo)
	}
	if datos, _ := os.ReadFile(rutaGlobal); !strings.Contains(string(datos), "esfuerzo: bajo") || !strings.Contains(string(datos), "modelo: m2") {
		t.Fatalf("solo el esfuerzo cambia, el modelo sigue:\n%s", datos)
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
	if r, _ := nuevoNavegador(t, ts.URL).pedir("GET", "/ui/sesion", "", "X-Forwarded-For", "203.0.113.5"); r.StatusCode != 401 {
		t.Fatalf("una petición reenviada por un proxy no entra sin token: %d", r.StatusCode)
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
	if !strings.Contains(cuerpo, pry.ID) || !strings.Contains(cuerpo, `"nombre":"Demo App"`) {
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

	// Abrir un proyecto desde la Web UI: exige el CSRF y una ruta completa; una carpeta nueva se prepara y se activa.
	nueva := t.TempDir()
	cuerpoProyecto := `{"raiz":` + strconv.Quote(nueva) + `}`
	if r, _ := n.pedir("POST", "/ui/proyectos", cuerpoProyecto); r.StatusCode != 401 {
		t.Fatalf("abrir un proyecto exige el CSRF: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/ui/proyectos", `{"raiz":"relativa"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("una ruta relativa se rechaza: %d", r.StatusCode)
	}
	if r, cuerpo := n.pedir("POST", "/ui/proyectos", cuerpoProyecto, "X-CSRF-Token", n.csrf); r.StatusCode != 200 || !strings.Contains(cuerpo, `"ambito":"proyecto"`) {
		t.Fatalf("abrir un proyecto nuevo: %d %s", r.StatusCode, cuerpo)
	}
	if _, err := os.Stat(filepath.Join(nueva, ".tramtai", "config.yaml")); err != nil {
		t.Fatalf("la carpeta nueva debía quedar preparada: %v", err)
	}
	_, cuerpo = n.pedir("GET", "/ui/sesion", "") // abrir el proyecto creó otra sesión con otro CSRF
	n.csrf = cuerpo[strings.Index(cuerpo, `"csrf":"`)+8:]
	n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]

	// Con el código de un solo uso de tramtai abrir, desde este mismo equipo, la sesión también es local.
	codigo, _ := s.CodigoSesion("global")
	otro := nuevoNavegador(t, ts.URL)
	otro.pedir("GET", "/ui/entrar?codigo="+url.QueryEscape(codigo), "")
	if _, cuerpo := otro.pedir("GET", "/ui/sesion", ""); !strings.Contains(cuerpo, `"local":true`) {
		t.Fatalf("la entrada con código desde este equipo debe ser local: %s", cuerpo)
	}

	// Puerto: uno ocupado se rechaza y uno libre se guarda en la configuración global.
	ocupado, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ocupado.Close()
	enUso := ocupado.Addr().(*net.TCPAddr).Port
	if r, cuerpo := n.pedir("POST", "/ui/servicio/puerto", fmt.Sprintf(`{"puerto":%d,"automatico":false}`, enUso), "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "ocupado") {
		t.Fatalf("un puerto ocupado se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	libre, _ := net.Listen("tcp", "127.0.0.1:0")
	puertoLibre := libre.Addr().(*net.TCPAddr).Port
	libre.Close()
	if r, cuerpo := n.pedir("POST", "/ui/servicio/puerto", fmt.Sprintf(`{"puerto":%d,"automatico":false}`, puertoLibre), "X-CSRF-Token", n.csrf); r.StatusCode != 200 || !strings.Contains(cuerpo, fmt.Sprintf(`"puerto_configurado":%d`, puertoLibre)) {
		t.Fatalf("un puerto libre se guarda: %d %s", r.StatusCode, cuerpo)
	}

	// Reiniciar dos veces (dos pestañas, la bandeja) lanza un solo servicio nuevo.
	var lanzados int
	s.opc.Reiniciar = func() error { lanzados++; return nil }
	for range 2 {
		if r, cuerpo := n.pedir("POST", "/ui/servicio/reiniciar", "", "X-CSRF-Token", n.csrf); r.StatusCode != 202 {
			t.Fatalf("reiniciar: %d %s", r.StatusCode, cuerpo)
		}
	}
	if lanzados != 1 {
		t.Fatalf("Reiniciar lanzó %d procesos", lanzados)
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

// pcblist arma una tabla como la de net.inet.tcp.pcblist_n de macOS: cabecera, por cada conexión un xinpcb_n y un
// xsocket_n (más largos que lo que se lee y sin alinear, como en el sistema) y la cabecera de cierre.
func pcblist(conexiones ...struct {
	local, remota netip.AddrPort
	uid           uint32
}) []byte {
	le := binary.LittleEndian
	cabecera := func() []byte { b := make([]byte, 24); le.PutUint32(b, 24); return b }
	elemento := func(tipo uint32, largo int) []byte {
		b := make([]byte, (largo+7)&^7)
		le.PutUint32(b, uint32(largo))
		le.PutUint32(b[4:], tipo)
		return b
	}
	datos := cabecera()
	for _, c := range conexiones {
		pcb := elemento(xsoInpcb, 105)
		binary.BigEndian.PutUint16(pcb[pcbPuertos:], c.remota.Port())
		binary.BigEndian.PutUint16(pcb[pcbPuertos+2:], c.local.Port())
		escribir := func(union []byte, d netip.Addr) {
			if d.Is4() {
				a := d.As4()
				copy(union[12:], a[:])
			} else {
				a := d.As16()
				copy(union, a[:])
			}
		}
		pcb[pcbVflag] = 0x1
		if c.local.Addr().Is6() {
			pcb[pcbVflag] = 0x2
		}
		escribir(pcb[pcbRemota:pcbRemota+16], c.remota.Addr())
		escribir(pcb[pcbLocal:pcbLocal+16], c.local.Addr())
		so := elemento(xsoSocket, 101)
		le.PutUint32(so[socketUID:], c.uid)
		datos = append(append(append(datos, pcb...), so...), elemento(0x002, 50)...) // y un elemento de otro tipo
	}
	return append(datos, cabecera()...)
}

func TestConexionesDePcblist(t *testing.T) {
	cliente, servidor := netip.MustParseAddrPort("127.0.0.1:50082"), netip.MustParseAddrPort("127.0.0.1:17271")
	cliente6, servidor6 := netip.MustParseAddrPort("[::1]:50083"), netip.MustParseAddrPort("[::1]:17271")
	type c = struct {
		local, remota netip.AddrPort
		uid           uint32
	}
	// El lado del servidor (local = servidor) es del servicio: no debe confundirse con el del cliente.
	tabla := pcblist(c{servidor, cliente, 0}, c{cliente, servidor, 501}, c{cliente6, servidor6, 502})
	for _, caso := range []struct {
		cliente, servidor netip.AddrPort
		uid               int
	}{{cliente, servidor, 501}, {cliente6, servidor6, 502}} {
		if uid, ok := uidDePcblist(tabla, caso.cliente, caso.servidor); !ok || uid != caso.uid {
			t.Errorf("%v: uid %d %v", caso.cliente, uid, ok)
		}
	}
	if _, ok := uidDePcblist(tabla, netip.MustParseAddrPort("127.0.0.1:1"), servidor); ok {
		t.Error("una conexión ajena no coincide")
	}
	if _, ok := uidDePcblist(tabla[:30], cliente, servidor); ok {
		t.Error("una tabla cortada no debe leerse fuera de sus límites")
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

// Un xinpcb_n y un xsocket_n copiados de una conexión real en el CI de macOS (127.0.0.1:49253 aceptó a :49254; este
// es el lado del servidor, uid 501): fijan las posiciones de las estructuras con #pragma pack(4).
func TestPcblistCasosLimite(t *testing.T) {
	cliente, servidor := netip.MustParseAddrPort("127.0.0.1:50082"), netip.MustParseAddrPort("127.0.0.1:17271")
	type c = struct {
		local, remota netip.AddrPort
		uid           uint32
	}
	// Un socket IPv6 que habla con IPv4 guarda las direcciones como ::ffff:127.0.0.1.
	mapeada := pcblist(c{netip.MustParseAddrPort("[::ffff:127.0.0.1]:50082"), netip.MustParseAddrPort("[::ffff:127.0.0.1]:17271"), 503})
	if uid, ok := uidDePcblist(mapeada, cliente, servidor); !ok || uid != 503 {
		t.Errorf("IPv4 mapeada: uid %d %v", uid, ok)
	}
	le := binary.LittleEndian
	normal := pcblist(c{cliente, servidor, 501})
	// Un xinpcb_n más corto de lo que se lee no se usa.
	corto := append([]byte(nil), normal...)
	le.PutUint32(corto[24:], 60)
	if _, ok := uidDePcblist(corto, cliente, servidor); ok {
		t.Error("un xinpcb_n corto no debe coincidir")
	}
	// Un xsocket_n sin su xinpcb_n delante no dice de quién es.
	sinPcb := append(append([]byte(nil), normal[:24]...), normal[24+104:]...)
	if _, ok := uidDePcblist(sinPcb, cliente, servidor); ok {
		t.Error("un xsocket_n suelto no debe coincidir")
	}
	// Una cabecera que dice ser enorme termina sin leer fuera.
	enorme := append([]byte(nil), normal...)
	le.PutUint32(enorme, 0xFFFFFFF0)
	if _, ok := uidDePcblist(enorme, cliente, servidor); ok {
		t.Error("con la cabecera rota no hay conexiones")
	}
}

// FuzzUidDePcblist: con cualquier tabla, el análisis no se sale de los límites.
func FuzzUidDePcblist(f *testing.F) {
	f.Add(pcblist(struct {
		local, remota netip.AddrPort
		uid           uint32
	}{netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:2"), 1}))
	f.Fuzz(func(t *testing.T, datos []byte) {
		uidDePcblist(datos, netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:2"))
	})
}

// Con ContextoDeConexion el dueño se comprueba una vez por conexión: las peticiones de la misma conexión comparten el
// resultado (y en este sistema, la conexión de la prueba es de este usuario).
func TestDuenoUnaVezPorConexion(t *testing.T) {
	var mu sync.Mutex
	var vistos []*duenoConexion
	var resultados []bool
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		resultados = append(resultados, mismoUsuarioLocal(r))
		vistos = append(vistos, r.Context().Value(claveDueno{}).(*duenoConexion))
	}))
	ts.Config.ConnContext = ContextoDeConexion
	ts.Start()
	defer ts.Close()
	cliente := ts.Client()
	for range 2 {
		r, err := cliente.Get(ts.URL)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	if len(vistos) != 2 || vistos[0] != vistos[1] {
		t.Fatalf("las dos peticiones de la misma conexión deben compartir el dueño: %v", vistos)
	}
	if !resultados[0] || !resultados[1] {
		t.Fatalf("la conexión de la prueba es de este usuario: %v", resultados)
	}
}

func TestPcblistDeMacOSReal(t *testing.T) {
	pcb, _ := hex.DecodeString("68000000100000002992cdd2abd4f18ec066c0650106e86fdc4b6db7b0010000000000000000800000000200014000000000000000000000000000007f0000010000000000000000000000007f000001000000000000000000000000000000005137139a12000000")
	so, _ := hex.DecodeString("6800000001000000ee3e416292e6c7aa010000000c000000000002012992cdd2abd4f18e06000000020000000000000000000000000000000000000000000000f50100005c07000000000000220f0000000000008000000000080001020000000000000000000000")
	cabecera := make([]byte, 24)
	binary.LittleEndian.PutUint32(cabecera, 24)
	tabla := append(append(append(cabecera, pcb...), so...), cabecera...)
	servidor, cliente := netip.MustParseAddrPort("127.0.0.1:49253"), netip.MustParseAddrPort("127.0.0.1:49254")
	if uid, ok := uidDePcblist(tabla, servidor, cliente); !ok || uid != 501 {
		t.Fatalf("uid %d %v", uid, ok)
	}
}

// Al reiniciar, la Web UI va al puerto configurado solo si está libre: si ahí escucha otro Tramtai, se queda en el actual.
func TestPuertoTrasReiniciar(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ocupado.Close()
	puertoOcupado := ocupado.Addr().(*net.TCPAddr).Port
	libre, _ := net.Listen("tcp", "127.0.0.1:0")
	puertoLibre := libre.Addr().(*net.TCPAddr).Port
	libre.Close()
	casos := []struct{ actual, configurado, esperado int }{
		{17374, puertoOcupado, 17374}, // otro Tramtai en el configurado
		{17374, puertoLibre, puertoLibre},
		{17374, 0, 17374},
		{17374, 17374, 17374},
	}
	for _, c := range casos {
		if p := puertoTrasReiniciar("127.0.0.1", c.actual, c.configurado); p != c.esperado {
			t.Errorf("actual %d, configurado %d: %d, se esperaba %d", c.actual, c.configurado, p, c.esperado)
		}
	}
}

// La cookie de sesión lleva el puerto: dos Tramtai en el mismo navegador (la instalada y una portable) no se pisan.
func TestCookieConPuerto(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	codigo, _ := s.CodigoSesion("global")
	cliente := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, err := cliente.Get(ts.URL + "/ui/entrar?codigo=" + codigo)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	esperado := "tramtai_sesion_" + strconv.Itoa(s.opc.Puerto)
	for _, c := range r.Cookies() {
		if c.Name == esperado {
			return
		}
	}
	t.Fatalf("la sesión debía ir en la cookie %s: %v", esperado, r.Cookies())
}
