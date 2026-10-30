package servicio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/totp"
)

const contrasenaPrueba = "una contraseña de prueba"

// conAdministrador prepara el servicio de un servidor con la global activa, la contraseña del administrador y, si
// secreto no está vacío, ese secreto TOTP en el almacén (falso). Devuelve también un servidor HTTPS con el mismo servicio.
func conAdministrador(t *testing.T, global, secreto string) (*Servicio, *httptest.Server, *httptest.Server) {
	t.Helper()
	almacen := almacenFalso(t)
	if secreto != "" {
		almacen["TRAMTAI_TOTP"] = secreto
	}
	s, ts, _ := entorno(t, global, "", adaptadores.Simulado{})
	s.opc.Entorno = "servidor"
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	if err := FijarContrasena(s.opc.Datos, contrasenaPrueba); err != nil {
		t.Fatal(err)
	}
	seguro := httptest.NewTLSServer(s)
	t.Cleanup(seguro.Close)
	return s, ts, seguro
}

// navegadorSeguro es un navegador que habla HTTPS con el servidor de prueba (confía en su certificado).
func navegadorSeguro(t *testing.T, seguro *httptest.Server) *navegador {
	n := nuevoNavegador(t, seguro.URL)
	n.http.Transport = seguro.Client().Transport
	return n
}

// acceder entra con la contraseña y el código; si sale bien, el navegador se queda con el CSRF de la sesión.
func (n *navegador) acceder(contrasena, codigo string, cabeceras ...string) (*http.Response, string) {
	n.t.Helper()
	cuerpo, _ := json.Marshal(map[string]string{"contrasena": contrasena, "codigo": codigo})
	r, respuesta := n.pedir("POST", "/ui/acceder", string(cuerpo), cabeceras...)
	var ses struct {
		CSRF string `json:"csrf"`
	}
	if json.Unmarshal([]byte(respuesta), &ses) == nil && ses.CSRF != "" {
		n.csrf = ses.CSRF
	}
	return r, respuesta
}

func TestContrasenaArgon2(t *testing.T) {
	datos := t.TempDir()
	if ok, err := ComprobarContrasena(datos, contrasenaPrueba); ok || err != nil {
		t.Fatalf("sin archivo: %v %v", ok, err)
	}
	if err := FijarContrasena(datos, "once letras"); err == nil || !strings.Contains(err.Error(), "12") {
		t.Fatalf("con 11 caracteres: %v", err)
	}
	if err := FijarContrasena(datos, contrasenaPrueba); err != nil {
		t.Fatal(err)
	}
	linea, err := os.ReadFile(filepath.Join(datos, "administrador"))
	if err != nil || !strings.HasPrefix(string(linea), "$argon2id$v=19$m=65536,t=3,p=1$") || strings.Count(string(linea), "$") != 5 {
		t.Fatalf("archivo: %q %v", linea, err)
	}
	if info, _ := os.Stat(filepath.Join(datos, "administrador")); info.Mode().Perm()&0o077 != 0 && os.PathSeparator == '/' {
		t.Fatalf("permisos: %v", info.Mode())
	}
	if ok, err := ComprobarContrasena(datos, contrasenaPrueba); !ok || err != nil {
		t.Fatalf("la buena: %v %v", ok, err)
	}
	if ok, err := ComprobarContrasena(datos, contrasenaPrueba+"!"); ok || err != nil {
		t.Fatalf("la mala: %v %v", ok, err)
	}
}

func TestRecuperacionUnSoloUso(t *testing.T) {
	datos := t.TempDir()
	codigos, err := NuevosCodigosRecuperacion(datos)
	if err != nil || len(codigos) != 10 {
		t.Fatalf("%v %v", codigos, err)
	}
	for _, c := range codigos {
		if len(c) != 11 || c[5] != '-' || strings.Trim(c, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567-") != "" {
			t.Fatalf("formato: %q", c)
		}
	}
	guardado, _ := os.ReadFile(filepath.Join(datos, "administrador-recuperacion"))
	if strings.Count(string(guardado), "sha256:") != 10 || strings.Contains(string(guardado), codigos[0]) {
		t.Fatalf("solo se guardan las huellas: %s", guardado)
	}
	// Se acepta en minúsculas y sin guion, como se teclea.
	if ok, err := UsarCodigoRecuperacion(datos, strings.ToLower(strings.ReplaceAll(codigos[3], "-", ""))); !ok || err != nil {
		t.Fatalf("primera vez: %v %v", ok, err)
	}
	if ok, _ := UsarCodigoRecuperacion(datos, codigos[3]); ok {
		t.Fatal("un código de recuperación no vale dos veces")
	}
	if ok, _ := UsarCodigoRecuperacion(datos, "AAAAA-AAAAA"); ok {
		t.Fatal("un código inventado no vale")
	}
	if ok, _ := UsarCodigoRecuperacion(datos, codigos[4]); !ok {
		t.Fatal("los demás siguen valiendo")
	}
}

// TestAccederSoloHTTPS: sin TLS y sin proxy no se entra con contraseña (403); por HTTPS, sí, con la cookie __Host- y
// Secure; también desde un proxy de confianza que dice X-Forwarded-Proto: https. La sesión es de la global.
func TestAccederSoloHTTPS(t *testing.T) {
	s, ts, seguro := conAdministrador(t, "", "")
	if r, cuerpo := nuevoNavegador(t, ts.URL).acceder(contrasenaPrueba, ""); r.StatusCode != 403 || !strings.Contains(cuerpo, "HTTPS") {
		t.Fatalf("sin TLS: %d %s", r.StatusCode, cuerpo)
	}
	n := navegadorSeguro(t, seguro)
	if r, cuerpo := n.pedir("GET", "/ui/entrada", ""); r.StatusCode != 200 || cuerpo != `{"contrasena":true,"totp":false,"https":true}`+"\n" {
		t.Fatalf("entrada: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo := n.acceder(contrasenaPrueba, "")
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"local":false`) || !strings.Contains(cuerpo, `"instancia":"global"`) {
		t.Fatalf("por HTTPS: %d %s", r.StatusCode, cuerpo)
	}
	cookies := r.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "__Host-tramtai_sesion_"+strconv.Itoa(s.opc.Puerto) || !cookies[0].Secure || !cookies[0].HttpOnly ||
		cookies[0].Path != "/" || cookies[0].Domain != "" || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie: %+v", cookies)
	}
	if r, cuerpo := n.pedir("GET", "/ui/sesion", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"local":false`) {
		t.Fatalf("la sesión sigue: %d %s", r.StatusCode, cuerpo)
	}
	s.opc.Confiables = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	if r, cuerpo := nuevoNavegador(t, ts.URL).acceder(contrasenaPrueba, "", "X-Forwarded-Proto", "https", "X-Forwarded-For", "203.0.113.4"); r.StatusCode != 200 {
		t.Fatalf("desde un proxy de confianza con https: %d %s", r.StatusCode, cuerpo)
	}
}

// TestAccederMismoMensaje: con TOTP, una contraseña mala y un código malo dan la misma respuesta; las dos buenas entran,
// y el mismo código no vale dos veces.
func TestAccederMismoMensaje(t *testing.T) {
	secreto, _ := totp.Generar()
	_, _, seguro := conAdministrador(t, "", secreto)
	n := navegadorSeguro(t, seguro)
	if _, cuerpo := n.pedir("GET", "/ui/entrada", ""); !strings.Contains(cuerpo, `"totp":true`) {
		t.Fatalf("entrada: %s", cuerpo)
	}
	bueno, _ := totp.Codigo(secreto, time.Now())
	malo := "000000"
	if malo == bueno {
		malo = "111111"
	}
	r1, c1 := n.acceder("otra contraseña cualquiera", bueno)
	r2, c2 := n.acceder(contrasenaPrueba, malo)
	if r1.StatusCode != 401 || r2.StatusCode != 401 || c1 != c2 || !strings.Contains(c1, "contraseña o código incorrectos") {
		t.Fatalf("contraseña mala: %d %s\ncódigo malo: %d %s", r1.StatusCode, c1, r2.StatusCode, c2)
	}
	if r, cuerpo := n.acceder(contrasenaPrueba, ""); r.StatusCode != 401 {
		t.Fatalf("sin código: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.acceder(contrasenaPrueba, bueno); r.StatusCode != 200 {
		t.Fatalf("las dos buenas: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := navegadorSeguro(t, seguro).acceder(contrasenaPrueba, bueno); r.StatusCode != 401 {
		t.Fatalf("el mismo código otra vez: %d", r.StatusCode)
	}
}

// TestAccederConRecuperacion: un código de recuperación sustituye al TOTP una sola vez.
func TestAccederConRecuperacion(t *testing.T) {
	secreto, _ := totp.Generar()
	s, _, seguro := conAdministrador(t, "", secreto)
	codigos, _ := NuevosCodigosRecuperacion(s.opc.Datos)
	if r, cuerpo := navegadorSeguro(t, seguro).acceder(contrasenaPrueba, codigos[0]); r.StatusCode != 200 {
		t.Fatalf("con recuperación: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := navegadorSeguro(t, seguro).acceder(contrasenaPrueba, codigos[0]); r.StatusCode != 401 {
		t.Fatalf("la segunda vez: %d", r.StatusCode)
	}
	// Con la contraseña mala no se gasta ningún código.
	navegadorSeguro(t, seguro).acceder("otra contraseña cualquiera", codigos[1])
	if r, _ := navegadorSeguro(t, seguro).acceder(contrasenaPrueba, codigos[1]); r.StatusCode != 200 {
		t.Fatalf("una contraseña mala no gasta el código: %d", r.StatusCode)
	}
}

// TestFrenoPorIP: tras 5 fallos desde una dirección, el sexto intento (aunque sea con la contraseña buena) da 429 con
// Retry-After, y queda en el registro.
func TestFrenoPorIP(t *testing.T) {
	s, _, seguro := conAdministrador(t, "", "")
	registros := &bufferSeguro{}
	s.opc.Registros = registros
	n := navegadorSeguro(t, seguro)
	for i := range 5 {
		if r, cuerpo := n.acceder("otra contraseña cualquiera", ""); r.StatusCode != 401 {
			t.Fatalf("fallo %d: %d %s", i+1, r.StatusCode, cuerpo)
		}
	}
	r, cuerpo := n.acceder(contrasenaPrueba, "")
	if espera, _ := strconv.Atoi(r.Header.Get("Retry-After")); r.StatusCode != 429 || espera < 14*60 || espera > 15*60 {
		t.Fatalf("sexto intento: %d %q %s", r.StatusCode, r.Header.Get("Retry-After"), cuerpo)
	}
	if texto := registros.String(); !strings.Contains(texto, "127.0.0.1") || !strings.Contains(texto, "bloque") {
		t.Fatalf("registro: %s", texto)
	}
}

// TestFrenoNoSeEvadeConXFF (Revisión 5): desde una dirección que no es un proxy de confianza, cambiar X-Forwarded-For en
// cada intento no cambia la dirección que cuenta.
func TestFrenoNoSeEvadeConXFF(t *testing.T) {
	_, _, seguro := conAdministrador(t, "", "")
	n := navegadorSeguro(t, seguro)
	for i := range 6 {
		r, _ := n.acceder("otra contraseña cualquiera", "", "X-Forwarded-For", "198.51.100."+strconv.Itoa(i+1))
		if quiere := map[bool]int{true: 429, false: 401}[i == 5]; r.StatusCode != quiere {
			t.Fatalf("intento %d: %d", i+1, r.StatusCode)
		}
	}
}

// TestFrenoGlobal: 100 fallos en 10 minutos, de direcciones distintas, frenan 5 minutos a todas.
func TestFrenoGlobal(t *testing.T) {
	f := nuevosFrenos()
	ahora := time.Unix(1_700_000_000, 0)
	for i := range 100 {
		ip := netip.AddrFrom4([4]byte{198, 51, byte(i / 250), byte(i % 250)})
		if espera := f.Permitido(ip, ahora); espera != 0 {
			t.Fatalf("fallo %d: ya frenado %s", i, espera)
		}
		f.Fallo(ip, ahora.Add(time.Duration(i)*time.Second))
	}
	nueva := netip.MustParseAddr("203.0.113.50")
	if espera := f.Permitido(nueva, ahora.Add(100*time.Second)); espera <= 4*time.Minute || espera > 5*time.Minute {
		t.Fatalf("tras 100 fallos: %s", espera)
	}
	if espera := f.Permitido(nueva, ahora.Add(100*time.Second+5*time.Minute)); espera != 0 {
		t.Fatalf("pasados los 5 minutos: %s", espera)
	}
	// 99 fallos repartidos en más de 10 minutos no frenan.
	g := nuevosFrenos()
	for i := range 150 {
		g.Fallo(netip.AddrFrom4([4]byte{198, 51, 100, byte(i)}), ahora.Add(time.Duration(i)*7*time.Second))
	}
	if espera := g.Permitido(nueva, ahora.Add(150*7*time.Second)); espera != 0 {
		t.Fatalf("fallos espaciados: %s", espera)
	}
	if cap(f.argon) != 2 {
		t.Fatalf("como mucho 2 cálculos de argon2id a la vez: %d", cap(f.argon))
	}
}

// TestFrenoPorIPUnitario: 5 fallos frenan 15 minutos a esa dirección y no a otra; un acierto borra la cuenta. En IPv6
// cuenta la red /64, que un atacante cambia de dirección sin coste dentro de ella.
func TestFrenoPorIPUnitario(t *testing.T) {
	f := nuevosFrenos()
	ahora := time.Unix(1_700_000_000, 0)
	ip := netip.MustParseAddr("198.51.100.7")
	for range 4 {
		f.Fallo(ip, ahora)
	}
	f.Exito(ip, ahora)
	f.Fallo(ip, ahora)
	if espera := f.Permitido(ip, ahora); espera != 0 {
		t.Fatalf("el acierto borra la cuenta: %s", espera)
	}
	for range 4 {
		f.Fallo(ip, ahora)
	}
	if espera := f.Permitido(ip, ahora); espera != 15*time.Minute {
		t.Fatalf("quinto fallo: %s", espera)
	}
	if espera := f.Permitido(netip.MustParseAddr("198.51.100.8"), ahora); espera != 0 {
		t.Fatalf("otra dirección: %s", espera)
	}
	for i := range 5 {
		f.Fallo(netip.MustParseAddr("2001:db8::"+strconv.Itoa(i+1)), ahora)
	}
	if espera := f.Permitido(netip.MustParseAddr("2001:db8::ff"), ahora); espera == 0 {
		t.Fatal("en IPv6 cuenta la red /64")
	}
}

// TestCambiarContrasenaCierraSesiones: cambiar la contraseña (con la actual) cierra todas las sesiones, también las
// de otros navegadores; la nueva entra y la anterior no. «Cerrar todas las sesiones» hace lo mismo.
func TestCambiarContrasenaCierraSesiones(t *testing.T) {
	_, _, seguro := conAdministrador(t, "", "")
	a, b := navegadorSeguro(t, seguro), navegadorSeguro(t, seguro)
	a.acceder(contrasenaPrueba, "")
	b.acceder(contrasenaPrueba, "")
	nueva := "otra contraseña bastante larga"
	if r, cuerpo := a.pedir("POST", "/ui/administrador/contrasena", `{"actual":"no es la actual","nueva":"`+nueva+`"}`, "X-CSRF-Token", a.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "actual") {
		t.Fatalf("con la actual mal: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := a.pedir("POST", "/ui/administrador/contrasena", `{"actual":"`+contrasenaPrueba+`","nueva":"`+nueva+`"}`); r.StatusCode != 403 {
		t.Fatalf("sin CSRF: %d", r.StatusCode)
	}
	if r, cuerpo := a.pedir("POST", "/ui/administrador/contrasena", `{"actual":"`+contrasenaPrueba+`","nueva":"corta"}`, "X-CSRF-Token", a.csrf); r.StatusCode != 400 {
		t.Fatalf("nueva corta: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := a.pedir("POST", "/ui/administrador/contrasena", `{"actual":"`+contrasenaPrueba+`","nueva":"`+nueva+`"}`, "X-CSRF-Token", a.csrf); r.StatusCode != 204 {
		t.Fatalf("cambiar: %d %s", r.StatusCode, cuerpo)
	}
	for nombre, n := range map[string]*navegador{"la que la cambió": a, "otro navegador": b} {
		if r, _ := n.pedir("GET", "/ui/sesion", ""); r.StatusCode != 401 {
			t.Errorf("%s sigue dentro: %d", nombre, r.StatusCode)
		}
	}
	if r, _ := navegadorSeguro(t, seguro).acceder(contrasenaPrueba, ""); r.StatusCode != 401 {
		t.Fatalf("la anterior: %d", r.StatusCode)
	}
	c, d := navegadorSeguro(t, seguro), navegadorSeguro(t, seguro)
	if r, _ := c.acceder(nueva, ""); r.StatusCode != 200 {
		t.Fatalf("la nueva: %d", r.StatusCode)
	}
	d.acceder(nueva, "")
	if r, _ := c.pedir("POST", "/ui/sesiones/cerrar", "", "X-CSRF-Token", c.csrf); r.StatusCode != 204 {
		t.Fatalf("cerrar todas: %d", r.StatusCode)
	}
	if r, _ := d.pedir("GET", "/ui/sesion", ""); r.StatusCode != 401 {
		t.Fatalf("tras cerrar todas: %d", r.StatusCode)
	}
}

// TestCambiarContrasenaDesdeLaCLI: la contraseña que pone la CLI (escribe el archivo directamente) cierra todas las
// sesiones: las abiertas con la anterior y también las abiertas con un código de un solo uso. Las de después valen.
func TestCambiarContrasenaDesdeLaCLI(t *testing.T) {
	s, _, seguro := conAdministrador(t, "", "")
	n := navegadorSeguro(t, seguro)
	n.acceder(contrasenaPrueba, "")
	conCodigo := navegadorSeguro(t, seguro)
	codigo, _ := s.CodigoSesion("global")
	if r, _ := conCodigo.pedir("GET", "/ui/entrar?codigo="+codigo, ""); r.StatusCode != http.StatusSeeOther {
		t.Fatalf("entrar con código: %d", r.StatusCode)
	}
	if r, _ := conCodigo.pedir("GET", "/ui/sesion", ""); r.StatusCode != 200 {
		t.Fatalf("la sesión con código: %d", r.StatusCode)
	}
	if err := FijarContrasena(s.opc.Datos, "la que pone la terminal"); err != nil {
		t.Fatal(err)
	}
	s.muSesiones.Lock()
	s.lecturaAdministrador = time.Time{} // como si hubiera pasado el segundo entre lecturas del archivo
	s.muSesiones.Unlock()
	for nombre, b := range map[string]*navegador{"con contraseña": n, "con código": conCodigo} {
		if r, _ := b.pedir("GET", "/ui/sesion", ""); r.StatusCode != 401 {
			t.Errorf("%s sigue dentro: %d", nombre, r.StatusCode)
		}
	}
	nueva := navegadorSeguro(t, seguro)
	nueva.acceder("la que pone la terminal", "")
	if r, _ := nueva.pedir("GET", "/ui/sesion", ""); r.StatusCode != 200 {
		t.Fatalf("la de después: %d", r.StatusCode)
	}
}

// TestContrasenaCambiaMientrasSeEntra: si la contraseña cambia entre comprobarla y crear la sesión (aquí, mientras el
// intento espera su hueco de argon2id), no se entra con la anterior: 401 y la cookie que llegue no vale.
func TestContrasenaCambiaMientrasSeEntra(t *testing.T) {
	s, _, seguro := conAdministrador(t, "", "")
	s.frenos.argon <- struct{}{} // los dos huecos, ocupados
	s.frenos.argon <- struct{}{}
	n := navegadorSeguro(t, seguro)
	hecho := make(chan int, 1)
	go func() {
		r, _ := n.acceder(contrasenaPrueba, "")
		if r.Header.Get("Set-Cookie") != "" { // ni la cookie de una sesión que no vale
			t.Errorf("el 401 lleva cookie: %q", r.Header.Get("Set-Cookie"))
		}
		hecho <- r.StatusCode
	}()
	for limite := time.Now().Add(10 * time.Second); s.frenos.enCola.Load() < 1; {
		if time.Now().After(limite) {
			t.Fatal("el intento no llegó a la cola")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := FijarContrasena(s.opc.Datos, "otra contraseña bastante larga"); err != nil {
		t.Fatal(err)
	}
	<-s.frenos.argon
	<-s.frenos.argon
	if estado := <-hecho; estado != 401 {
		t.Fatalf("entrar con la anterior durante el cambio: %d", estado)
	}
	if r, _ := n.pedir("GET", "/ui/sesion", ""); r.StatusCode != 401 {
		t.Fatalf("la sesión de ese intento: %d", r.StatusCode)
	}
}

// TestSesionRemotaNoEsLocal: la sesión abierta con la contraseña no guarda claves ni usa los controles del servicio ni
// ve las carpetas del servidor en Acerca de.
func TestSesionRemotaNoEsLocal(t *testing.T) {
	_, _, seguro := conAdministrador(t, "", "")
	n := navegadorSeguro(t, seguro)
	n.acceder(contrasenaPrueba, "")
	if r, cuerpo := n.pedir("PUT", "/api/v1/servicio/claves/ANTHROPIC_API_KEY", `{"valor":"sk-prueba-123"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 401 {
		t.Fatalf("PUT claves: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("POST", "/ui/agentes/conexion", `{"agente":"claude","conectado":false}`, "X-CSRF-Token", n.csrf); r.StatusCode != 401 {
		t.Fatalf("una ruta solo local: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo := n.pedir("GET", "/ui/acerca", "")
	if r.StatusCode != 200 || strings.Contains(cuerpo, `"rutas"`) {
		t.Fatalf("acerca: %d %s", r.StatusCode, cuerpo)
	}
}

// TestAccederSinRedireccion: entrar responde JSON y nunca redirige a una URL de la petición.
func TestAccederSinRedireccion(t *testing.T) {
	_, _, seguro := conAdministrador(t, "", "")
	n := navegadorSeguro(t, seguro)
	cuerpo, _ := json.Marshal(map[string]string{"contrasena": contrasenaPrueba, "codigo": ""})
	r, respuesta := n.pedir("POST", "/ui/acceder?siguiente=https://otro.ejemplo", string(cuerpo))
	if r.StatusCode != 200 || r.Header.Get("Location") != "" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || strings.Contains(respuesta, "otro.ejemplo") {
		t.Fatalf("%d %q %s", r.StatusCode, r.Header.Get("Location"), respuesta)
	}
}

// TestActivarTOTPEnDosPasos: la Web UI enseña el secreto y solo lo guarda tras confirmarlo con un código válido; entonces
// da los 10 códigos de recuperación. En Linux responde 409 con la orden de la CLI.
func TestActivarTOTPEnDosPasos(t *testing.T) {
	s, _, seguro := conAdministrador(t, "", "")
	anterior := totpSoloCLI
	t.Cleanup(func() { totpSoloCLI = anterior })
	totpSoloCLI = false
	n := navegadorSeguro(t, seguro)
	n.acceder(contrasenaPrueba, "")
	if r, cuerpo := n.pedir("POST", "/ui/administrador/totp", `{"actual":"no es la actual","codigo":""}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "actual") {
		t.Fatalf("sin la contraseña actual: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo := n.pedir("POST", "/ui/administrador/totp", `{"actual":"`+contrasenaPrueba+`","codigo":""}`, "X-CSRF-Token", n.csrf)
	var nuevo struct{ Secreto, URI string }
	if r.StatusCode != 200 || json.Unmarshal([]byte(cuerpo), &nuevo) != nil || !strings.HasPrefix(nuevo.URI, "otpauth://totp/Tramtai:") {
		t.Fatalf("primer paso: %d %s", r.StatusCode, cuerpo)
	}
	if v, _ := leerCredencial("TRAMTAI_TOTP"); v != "" {
		t.Fatal("el secreto no se guarda antes de confirmarlo")
	}
	if r, cuerpo := n.pedir("POST", "/ui/administrador/totp", `{"codigo":"000000"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("con un código que no es: %d %s", r.StatusCode, cuerpo)
	}
	codigo, _ := totp.Codigo(nuevo.Secreto, time.Now())
	r, cuerpo = n.pedir("POST", "/ui/administrador/totp", `{"codigo":"`+codigo+`"}`, "X-CSRF-Token", n.csrf)
	var hecho struct{ Codigos []string }
	if r.StatusCode != 200 || json.Unmarshal([]byte(cuerpo), &hecho) != nil || len(hecho.Codigos) != 10 {
		t.Fatalf("confirmar: %d %s", r.StatusCode, cuerpo)
	}
	if v, _ := leerCredencial("TRAMTAI_TOTP"); v != nuevo.Secreto {
		t.Fatalf("guardado: %q", v)
	}
	if ok, _ := UsarCodigoRecuperacion(s.opc.Datos, hecho.Codigos[9]); !ok {
		t.Fatal("los códigos de recuperación quedan guardados")
	}
	if r, cuerpo := n.pedir("POST", "/ui/administrador/recuperacion", `{"actual":"no es la actual"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("regenerar sin la contraseña actual: %d %s", r.StatusCode, cuerpo)
	}
	// Con TOTP, regenerar y quitar piden además un código (de la app o de recuperación): una sesión robada con la
	// contraseña no basta.
	if r, cuerpo := n.pedir("POST", "/ui/administrador/recuperacion", `{"actual":"`+contrasenaPrueba+`"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "código") {
		t.Fatalf("regenerar sin código: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo = n.pedir("POST", "/ui/administrador/recuperacion", `{"actual":"`+contrasenaPrueba+`","codigo":"`+hecho.Codigos[8]+`"}`, "X-CSRF-Token", n.csrf)
	var regenerados struct{ Codigos []string }
	if r.StatusCode != 200 || json.Unmarshal([]byte(cuerpo), &regenerados) != nil || len(regenerados.Codigos) != 10 {
		t.Fatalf("regenerar: %d %s", r.StatusCode, cuerpo)
	}
	if ok, _ := UsarCodigoRecuperacion(s.opc.Datos, hecho.Codigos[0]); ok {
		t.Fatal("regenerarlos invalida los anteriores")
	}
	// Con TOTP, uno nuevo pide además un código del actual.
	activar := `{"actual":"` + contrasenaPrueba + `","codigo":""}`
	if r, cuerpo := n.pedir("POST", "/ui/administrador/totp", activar, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "código") {
		t.Fatalf("cambiar el TOTP sin el código actual: %d %s", r.StatusCode, cuerpo)
	}
	siguiente, _ := totp.Codigo(nuevo.Secreto, time.Now().Add(30*time.Second)) // el de la confirmación ya está visto
	if r, cuerpo := n.pedir("POST", "/ui/administrador/totp", `{"actual":"`+contrasenaPrueba+`","codigo_actual":"`+siguiente+`","codigo":""}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 {
		t.Fatalf("cambiar el TOTP con el código actual: %d %s", r.StatusCode, cuerpo)
	}
	s.frenos = nuevosFrenos() // los fallos de arriba son a propósito: que no lleguen a la espera
	if r, cuerpo := n.pedir("DELETE", "/ui/administrador/totp", `{"actual":"no es la actual"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("quitar sin la contraseña actual: %d %s", r.StatusCode, cuerpo)
	}
	if v, _ := leerCredencial("TRAMTAI_TOTP"); v != nuevo.Secreto {
		t.Fatal("sin la contraseña actual no se quita")
	}
	if r, cuerpo := n.pedir("DELETE", "/ui/administrador/totp", `{"actual":"`+contrasenaPrueba+`"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 || !strings.Contains(cuerpo, "código") {
		t.Fatalf("quitar sin código: %d %s", r.StatusCode, cuerpo)
	}
	if v, _ := leerCredencial("TRAMTAI_TOTP"); v != nuevo.Secreto {
		t.Fatal("sin código no se quita")
	}
	if r, cuerpo := n.pedir("DELETE", "/ui/administrador/totp", `{"actual":"`+contrasenaPrueba+`","codigo":"`+regenerados.Codigos[0]+`"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("quitar: %d %s", r.StatusCode, cuerpo)
	}
	if v, _ := leerCredencial("TRAMTAI_TOTP"); v != "" {
		t.Fatal("quitar borra el secreto")
	}
	totpSoloCLI = true
	if r, cuerpo := n.pedir("POST", "/ui/administrador/totp", `{"codigo":""}`, "X-CSRF-Token", n.csrf); r.StatusCode != 409 || !strings.Contains(cuerpo, "administrador totp activar") {
		t.Fatalf("en Linux: %d %s", r.StatusCode, cuerpo)
	}
}

// TestCuerpoUIAcotado: las rutas /ui/ leen como mucho 64 KiB de cuerpo, antes de los frenos y de la cola de argon2id:
// entrar con una contraseña de 1 MB da 400 sin calcular nada.
func TestCuerpoUIAcotado(t *testing.T) {
	_, _, seguro := conAdministrador(t, "", "")
	r, cuerpo := navegadorSeguro(t, seguro).acceder(strings.Repeat("a", 1<<20), "")
	if r.StatusCode != 400 || !strings.Contains(cuerpo, "solicitud_invalida") {
		t.Fatalf("1 MB en /ui/acceder: %d %.200s", r.StatusCode, cuerpo)
	}
}

// TestCodigoActualEnFrenos: con TOTP, un código malo en Ajustes → Seguridad cuenta en los frenos como un intento de
// entrada, aunque la contraseña sea buena: 5 dan la espera.
func TestCodigoActualEnFrenos(t *testing.T) {
	secreto, _ := totp.Generar()
	_, _, seguro := conAdministrador(t, "", secreto)
	n := navegadorSeguro(t, seguro)
	codigo, _ := totp.Codigo(secreto, time.Now())
	if r, cuerpo := n.acceder(contrasenaPrueba, codigo); r.StatusCode != 200 {
		t.Fatalf("acceder: %d %s", r.StatusCode, cuerpo)
	}
	for i := range 6 {
		quiere := map[bool]int{true: 400, false: 429}[i < 5]
		if r, cuerpo := n.pedir("POST", "/ui/administrador/recuperacion", `{"actual":"`+contrasenaPrueba+`","codigo":"no-es-un-codigo"}`, "X-CSRF-Token", n.csrf); r.StatusCode != quiere {
			t.Fatalf("código malo %d: %d %s, se esperaba %d", i+1, r.StatusCode, cuerpo, quiere)
		}
	}
}

// TestQuitarTOTPConSuscripcionPersonal: mientras un agente usa la suscripción personal, el TOTP no se quita (409 con el
// porqué), ni desde la Web UI ni por la API de administración que usa la CLI en Windows.
func TestQuitarTOTPConSuscripcionPersonal(t *testing.T) {
	secreto, _ := totp.Generar()
	s, ts, seguro := conAdministrador(t, "adaptadores:\n  claude:\n    autenticacion: suscripcion\n    suscripcion_personal: true\n", secreto)
	n := navegadorSeguro(t, seguro)
	codigo, _ := totp.Codigo(secreto, time.Now())
	if r, cuerpo := n.acceder(contrasenaPrueba, codigo); r.StatusCode != 200 {
		t.Fatalf("acceder: %d %s", r.StatusCode, cuerpo)
	}
	siguiente, _ := totp.Codigo(secreto, time.Now().Add(30*time.Second)) // el de la entrada ya está visto
	if r, cuerpo := n.pedir("DELETE", "/ui/administrador/totp", `{"actual":"`+contrasenaPrueba+`","codigo":"`+siguiente+`"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 409 || !strings.Contains(cuerpo, "suscripción personal") {
		t.Fatalf("Web UI: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := admin(t, s, ts.URL, "DELETE", "/api/v1/servicio/totp", ""); r.StatusCode != 409 || !strings.Contains(cuerpo, "suscripción personal") {
		t.Fatalf("API de administración: %d %s", r.StatusCode, cuerpo)
	}
	if v, _ := leerCredencial("TRAMTAI_TOTP"); v != secreto {
		t.Fatal("el secreto sigue guardado")
	}
}

// TestAdminTOTP: la API de administración (la CLI en Windows) guarda y quita el secreto con el token de servicio y solo
// desde este equipo; un secreto que no es base32 se rechaza.
func TestAdminTOTP(t *testing.T) {
	s, ts, _ := conAdministrador(t, "", "")
	anterior := totpSoloCLI
	t.Cleanup(func() { totpSoloCLI = anterior })
	totpSoloCLI = false
	secreto, _ := totp.Generar()
	if r, _ := pedir(t, ts.URL, "PUT", "/api/v1/servicio/totp", s.tokenAdmin, `{"secreto":"`+secreto+`"}`, "X-Forwarded-For", "203.0.113.7"); r.StatusCode != 403 {
		t.Fatalf("por un proxy: %d", r.StatusCode)
	}
	if r, cuerpo := admin(t, s, ts.URL, "PUT", "/api/v1/servicio/totp", `{"secreto":"no es base32!"}`); r.StatusCode != 400 {
		t.Fatalf("secreto malo: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := admin(t, s, ts.URL, "PUT", "/api/v1/servicio/totp", `{"secreto":"GEZDGNBVGY3TQOJQ"}`); r.StatusCode != 400 || !strings.Contains(cuerpo, "20 bytes") {
		t.Fatalf("secreto de 10 bytes: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := admin(t, s, ts.URL, "PUT", "/api/v1/servicio/totp", `{"secreto":"`+secreto+`"}`); r.StatusCode != 204 {
		t.Fatalf("guardar: %d %s", r.StatusCode, cuerpo)
	}
	if v, _ := leerCredencial("TRAMTAI_TOTP"); v != secreto {
		t.Fatalf("guardado: %q", v)
	}
	if r, cuerpo := admin(t, s, ts.URL, "DELETE", "/api/v1/servicio/totp", ""); r.StatusCode != 204 {
		t.Fatalf("quitar: %d %s", r.StatusCode, cuerpo)
	}
}

// TestFrenoClavesFalsas: 30 claves falsas en un minuto desde una dirección (en la API del espacio o en la de un punto de
// acceso) frenan esa dirección: la 31 da 429, aunque llegue con una clave buena; otra dirección sigue recibiendo 401.
func TestFrenoClavesFalsas(t *testing.T) {
	s, _, _ := entorno(t, "puntos_acceso:\n  - nombre: app\n", "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	buena := claveDe(t, s, global)
	s.Recargar()
	pedirDesde := func(remota, ruta, clave string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", ruta, strings.NewReader(hola))
		r.Host, r.RemoteAddr = "127.0.0.1", remota
		r.Header.Set("Authorization", "Bearer "+clave)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	for i := range 30 {
		ruta := "/api/v1/ejecutar"
		if i%3 == 0 {
			ruta = "/p/app/api/v1/ejecutar"
		}
		if w := pedirDesde("198.51.100.7:4000", ruta, "falsa-"+strconv.Itoa(i)); w.Code != 401 {
			t.Fatalf("clave falsa %d en %s: %d %s", i+1, ruta, w.Code, w.Body)
		}
	}
	w := pedirDesde("198.51.100.7:4001", "/api/v1/ejecutar", "falsa-31")
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("la 31: %d %s", w.Code, w.Body)
	}
	if w := pedirDesde("198.51.100.7:4002", "/api/v1/ejecutar", buena); w.Code != 429 {
		t.Fatalf("frenada, tampoco con la buena: %d", w.Code)
	}
	if w := pedirDesde("203.0.113.9:4000", "/api/v1/ejecutar", "falsa-otra"); w.Code != 401 {
		t.Fatalf("otra dirección: %d %s", w.Code, w.Body)
	}
	if w := pedirDesde("203.0.113.9:4000", "/api/v1/ejecutar", buena); w.Code != 200 {
		t.Fatalf("otra dirección con la buena: %d %s", w.Code, w.Body)
	}
}

// TestClavesFalsasSaturado: con el contador de claves falsas lleno (p. ej. desde una /48 de IPv6), las direcciones sin
// cuenta no esperan: el freno es contra el abuso y las claves son de 256 bits, así que falla abierto y las apps con su
// clave siguen funcionando. El de la contraseña sí cierra (TestContadorSaturado).
func TestClavesFalsasSaturado(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	buena := claveDe(t, s, global)
	ahora := time.Now()
	for i := range maximoCuentas + 1 {
		s.claves.fallo(netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), ahora)
	}
	for clave, quiere := range map[string]int{buena: 200, "falsa": 401} {
		r := httptest.NewRequest("POST", "/api/v1/ejecutar", strings.NewReader(hola))
		r.Host, r.RemoteAddr = "127.0.0.1", "198.51.100.20:4000"
		r.Header.Set("Authorization", "Bearer "+clave)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != quiere {
			t.Errorf("contador lleno, dirección nueva, clave %s: %d %s, se esperaba %d", clave, w.Code, w.Body, quiere)
		}
	}
}

// TestFrenoAccederALaVez: 20 intentos a la vez desde una dirección no se saltan el freno aunque pasen todos la primera
// comprobación antes de que se cuente ningún fallo: como mucho 6 llegan a comparar la contraseña (los 5 del freno y el que
// ya calculaba con el otro hueco) y los demás reciben 429.
func TestFrenoAccederALaVez(t *testing.T) {
	_, _, seguro := conAdministrador(t, "", "")
	estados := make(chan int, 20)
	var grupo sync.WaitGroup
	for range 20 {
		grupo.Go(func() {
			r, _ := navegadorSeguro(t, seguro).acceder("otra contraseña cualquiera", "")
			estados <- r.StatusCode
		})
	}
	grupo.Wait()
	close(estados)
	cuenta := map[int]int{}
	for e := range estados {
		cuenta[e]++
	}
	if cuenta[401] < 5 || cuenta[401] > 6 || cuenta[401]+cuenta[429] != 20 {
		t.Fatalf("a la vez: %v", cuenta)
	}
}

// TestFrenoGlobalConocidas (Ruling R25): con el freno global activo, una dirección (o su red /64) que entró bien en los
// últimos 30 días sigue pudiendo intentarlo; una nueva no, y una conocida hace más de 30 días tampoco. Se recuerdan como
// mucho 256: la más antigua se olvida.
func TestFrenoGlobalConocidas(t *testing.T) {
	f := nuevosFrenos()
	ahora := time.Unix(1_700_000_000, 0)
	duena, vieja := netip.MustParseAddr("2001:db8:1::5"), netip.MustParseAddr("198.51.100.200")
	f.Exito(vieja, ahora.Add(-31*24*time.Hour))
	f.Exito(duena, ahora.Add(-24*time.Hour))
	for i := range 100 {
		f.Fallo(netip.AddrFrom4([4]byte{203, 0, byte(113 + i/250), byte(i % 250)}), ahora)
	}
	if espera := f.Permitido(netip.MustParseAddr("192.0.2.1"), ahora); espera == 0 {
		t.Fatal("el freno global está activo")
	}
	if espera := f.Permitido(netip.MustParseAddr("2001:db8:1::77"), ahora); espera != 0 {
		t.Fatalf("la red de la dueña sigue pudiendo: %s", espera)
	}
	if espera := f.Permitido(vieja, ahora); espera == 0 {
		t.Fatal("una conocida de hace más de 30 días espera como las demás")
	}
	for range 5 { // su propio freno sí cuenta
		f.Fallo(duena, ahora)
	}
	if espera := f.Permitido(duena, ahora); espera == 0 {
		t.Fatal("5 fallos frenan también a una conocida")
	}
	g := nuevosFrenos()
	for i := range maximoConocidas + 1 {
		g.Exito(netip.AddrFrom4([4]byte{10, 0, byte(i / 250), byte(i % 250)}), ahora.Add(time.Duration(i)*time.Second))
	}
	if len(g.conocidas) != maximoConocidas || g.conocida(netip.AddrFrom4([4]byte{10, 0, 0, 0}), ahora) {
		t.Fatalf("como mucho %d, sin la más antigua: %d", maximoConocidas, len(g.conocidas))
	}
}

// TestContadorSaturado: con el contador lleno (10 000 direcciones con cuenta) una dirección nueva no se deja de frenar:
// espera la pausa.
func TestContadorSaturado(t *testing.T) {
	c := nuevoContador(30, time.Minute, time.Minute)
	ahora := time.Unix(1_700_000_000, 0)
	for i := range maximoCuentas {
		c.fallo(netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)}), ahora)
	}
	nueva := netip.MustParseAddr("198.51.100.9")
	if !c.fallo(nueva, ahora) || c.espera(nueva, ahora) == 0 {
		t.Fatal("lleno: la dirección nueva espera")
	}
	if c.espera(netip.AddrFrom4([4]byte{10, 0, 0, 1}), ahora) != 0 {
		t.Fatal("las que tienen cuenta siguen con la suya")
	}
	if c.espera(nueva, ahora.Add(2*time.Minute)) != 0 {
		t.Fatal("pasada la pausa, vuelve a poder")
	}
}

// TestColaArgonLlena: con 32 intentos esperando su cálculo de argon2id, el siguiente recibe 503 con Retry-After.
func TestColaArgonLlena(t *testing.T) {
	s, _, seguro := conAdministrador(t, "", "")
	s.frenos.argon <- struct{}{} // los dos huecos, ocupados
	s.frenos.argon <- struct{}{}
	var grupo sync.WaitGroup
	for range maximoEnCola {
		grupo.Go(func() { navegadorSeguro(t, seguro).acceder("otra contraseña cualquiera", "") })
	}
	for limite := time.Now().Add(10 * time.Second); s.frenos.enCola.Load() < maximoEnCola; {
		if time.Now().After(limite) {
			t.Fatalf("en cola: %d", s.frenos.enCola.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, cuerpo := navegadorSeguro(t, seguro).acceder(contrasenaPrueba, "")
	if r.StatusCode != 503 || r.Header.Get("Retry-After") == "" || !strings.Contains(cuerpo, "entrada_ocupada") {
		t.Fatalf("con la cola llena: %d %s", r.StatusCode, cuerpo)
	}
	<-s.frenos.argon
	<-s.frenos.argon
	grupo.Wait()
}

// TestClavesFalsasDesdeEsteEquipo: lo que llega por loopback sin proxy no se frena por claves falsas (un proceso del
// equipo no deja sin servicio a la CLI ni al escritorio); por loopback con cabeceras de proxy, sí.
func TestClavesFalsasDesdeEsteEquipo(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	pedirLocal := func(cabeceras ...string) int {
		r := httptest.NewRequest("POST", "/api/v1/ejecutar", strings.NewReader(hola))
		r.Host, r.RemoteAddr = "127.0.0.1", "127.0.0.1:4000"
		r.Header.Set("Authorization", "Bearer falsa")
		for i := 0; i+1 < len(cabeceras); i += 2 {
			r.Header.Set(cabeceras[i], cabeceras[i+1])
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w.Code
	}
	for i := range 40 {
		if e := pedirLocal(); e != 401 {
			t.Fatalf("desde este equipo, la %d: %d", i+1, e)
		}
	}
	ultimo := 0
	for range 31 {
		ultimo = pedirLocal("X-Forwarded-For", "203.0.113.4")
	}
	if ultimo != 429 {
		t.Fatalf("por loopback con cabeceras de proxy: %d", ultimo)
	}
}

// TestRecuperacionALaVez: el mismo código de recuperación usado a la vez solo entra una vez.
func TestRecuperacionALaVez(t *testing.T) {
	datos := t.TempDir()
	codigos, _ := NuevosCodigosRecuperacion(datos)
	var aciertos atomic.Int32
	var grupo sync.WaitGroup
	for range 10 {
		grupo.Go(func() {
			if ok, _ := UsarCodigoRecuperacion(datos, codigos[2]); ok {
				aciertos.Add(1)
			}
		})
	}
	grupo.Wait()
	if aciertos.Load() != 1 {
		t.Fatalf("entró %d veces", aciertos.Load())
	}
}

// TestAdministradorDanado: con datos/administrador dañado, quien entra recibe un mensaje sin la ruta de datos/; la ruta
// queda en el registro.
func TestAdministradorDanado(t *testing.T) {
	s, _, seguro := conAdministrador(t, "", "")
	registros := &bufferSeguro{}
	s.opc.Registros = registros
	if err := os.WriteFile(filepath.Join(s.opc.Datos, "administrador"), []byte("no es una huella\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, cuerpo := navegadorSeguro(t, seguro).acceder(contrasenaPrueba, "")
	escapada, _ := json.Marshal(s.opc.Datos)
	if r.StatusCode != 400 || strings.Contains(cuerpo, strings.Trim(string(escapada), `"`)) || !strings.Contains(registros.String(), filepath.Join(s.opc.Datos, "administrador")) {
		t.Fatalf("%d %s\nregistro: %s", r.StatusCode, cuerpo, registros.String())
	}
}
