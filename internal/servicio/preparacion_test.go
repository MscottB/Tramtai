package servicio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/totp"
)

// preparacionFalsa hace de preparar.Preparador en el servicio: no instala nada.
type preparacionFalsa struct {
	mu        sync.Mutex
	estados   map[string]contrato.EstadoPreparacion
	pedidos   int
	versiones []bool // el forzar de cada llamada a Versiones
	espera    chan struct{}
	cerrada   bool
}

func nuevaPreparacionFalsa() *preparacionFalsa {
	return &preparacionFalsa{estados: map[string]contrato.EstadoPreparacion{}}
}

func (f *preparacionFalsa) Estado(agente string) (contrato.EstadoPreparacion, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.estados[agente]
	return e, ok
}

func (f *preparacionFalsa) Iniciar(agente string, p contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pedidos++
	f.estados[agente] = contrato.EstadoPreparacion{Agente: agente, Accion: p.Accion, Estado: "instalando"}
	return f.estados[agente], nil
}

func (f *preparacionFalsa) Cancelar(string) error { return nil }

func (f *preparacionFalsa) Codigo(string, string) error { return nil }

// Versiones espera, si hay espera, a que la prueba la cierre: así se ve si otra llamada se cuela mientras tanto.
func (f *preparacionFalsa) Versiones(_ context.Context, forzar bool) []contrato.VersionesAgente {
	f.mu.Lock()
	f.versiones = append(f.versiones, forzar)
	n, espera := len(f.versiones), f.espera
	f.mu.Unlock()
	if espera != nil {
		<-espera
	}
	return []contrato.VersionesAgente{{Agente: "claude", Instalada: strings.Repeat("1.", n) + "0"}}
}

func (f *preparacionFalsa) Cerrar() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cerrada = true
}

func (f *preparacionFalsa) fijar(agente string, e contrato.EstadoPreparacion) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.estados[agente] = e
}

func (f *preparacionFalsa) llamadas() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bool{}, f.versiones...)
}

const rutaPreparacion = "/api/v1/agentes/claude/preparacion"

// entrarLocal abre una sesión local de la Web UI (con el código de un solo uso) y deja su CSRF en el navegador.
func entrarLocal(t *testing.T, s *Servicio, base string) *navegador {
	t.Helper()
	n := nuevoNavegador(t, base)
	cod, err := s.CodigoSesion("global")
	if err != nil {
		t.Fatal(err)
	}
	n.pedir("GET", "/ui/entrar?codigo="+cod, "")
	_, cuerpo := n.pedir("GET", "/ui/sesion", "")
	if i := strings.Index(cuerpo, `"csrf":"`); i >= 0 {
		n.csrf = cuerpo[i+8:]
		n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]
	}
	if n.csrf == "" {
		t.Fatalf("sin sesión: %s", cuerpo)
	}
	return n
}

// TestPrepararSoloAdministradorEnServidor: en el servidor solo el administrador prepara agentes (su sesión, que exige
// TOTP si lo tiene, o la clave del servicio en el propio servidor); sin nada, 401; con la clave de una app, 403, también
// fuera del servidor (Ruling 23).
func TestPrepararSoloAdministradorEnServidor(t *testing.T) {
	almacen := almacenFalso(t)
	secreto, _ := totp.Generar()
	almacen["TRAMTAI_TOTP"] = secreto
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	f := nuevaPreparacionFalsa()
	s.preparar = f
	s.opc.Entorno = "servidor"
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if err := FijarContrasena(s.opc.Datos, contrasenaPrueba); err != nil {
		t.Fatal(err)
	}
	seguro := httptest.NewTLSServer(s)
	t.Cleanup(seguro.Close)
	clave := claveDe(t, s, global)
	const instalar = `{"accion":"instalar"}`

	if r, cuerpo := pedir(t, ts.URL, "POST", rutaPreparacion, "", instalar); r.StatusCode != 401 {
		t.Errorf("sin sesión: %d %s", r.StatusCode, cuerpo)
	}
	for _, ruta := range []string{"POST " + rutaPreparacion, "GET " + rutaPreparacion, "DELETE " + rutaPreparacion,
		"POST " + rutaPreparacion + "/codigo", "GET /api/v1/agentes/versiones"} {
		metodo, camino, _ := strings.Cut(ruta, " ")
		if r, cuerpo := pedir(t, ts.URL, metodo, camino, clave, `{"accion":"instalar","codigo":"abc-12345"}`); r.StatusCode != 403 {
			t.Errorf("con la clave de una app, %s: %d %s", ruta, r.StatusCode, cuerpo)
		}
	}
	n := navegadorSeguro(t, seguro)
	codigo, _ := totp.Codigo(secreto, time.Now())
	if r, cuerpo := n.acceder(contrasenaPrueba, codigo); r.StatusCode != 200 {
		t.Fatalf("acceder: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("POST", rutaPreparacion, instalar, "X-CSRF-Token", n.csrf); r.StatusCode != 202 {
		t.Errorf("el administrador: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts.URL, "POST", rutaPreparacion, s.tokenAdmin, instalar, "Tramtai-Instancia", "global"); r.StatusCode != 202 {
		t.Errorf("la clave del servicio en el servidor (tramtai agentes): %d %s", r.StatusCode, cuerpo)
	}
	if f.pedidos != 2 {
		t.Errorf("preparaciones lanzadas: %d", f.pedidos)
	}

	// Ruling 23: también fuera del servidor, la clave del espacio (la de las apps) no basta.
	s.opc.Entorno = "escritorio"
	if r, cuerpo := pedir(t, ts.URL, "POST", rutaPreparacion, clave, instalar); r.StatusCode != 403 || !strings.Contains(cuerpo, `"codigo":"no_autorizado"`) {
		t.Errorf("fuera del servidor, con la clave del espacio: %d %s", r.StatusCode, cuerpo)
	}
	if f.pedidos != 2 {
		t.Errorf("preparaciones lanzadas: %d", f.pedidos)
	}
}

// pedirAdmin pide con la clave del servicio en el espacio global, como la CLI y la terminal.
func pedirAdmin(t *testing.T, s *Servicio, base, metodo, ruta, cuerpo string) (*http.Response, string) {
	t.Helper()
	return pedir(t, base, metodo, ruta, s.tokenAdmin, cuerpo, "Tramtai-Instancia", "global")
}

// TestPrepararCSRF: con la sesión de la Web UI, lo que cambia algo exige X-CSRF-Token, como el resto de rutas. Lo pegado
// no queda en el registro del servicio.
func TestPrepararCSRF(t *testing.T) {
	registros := &bufferSeguro{}
	s, ts, _ := entornoCon(t, "", "", adaptadores.Simulado{}, func(o *Opciones) { o.Registros = registros })
	f := nuevaPreparacionFalsa()
	s.preparar = f
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	n := entrarLocal(t, s, ts.URL)
	for _, ruta := range []string{"POST " + rutaPreparacion, "DELETE " + rutaPreparacion, "POST " + rutaPreparacion + "/codigo"} {
		metodo, camino, _ := strings.Cut(ruta, " ")
		if r, cuerpo := n.pedir(metodo, camino, `{"accion":"instalar"}`); r.StatusCode != 403 || !strings.Contains(cuerpo, "csrf") {
			t.Errorf("%s sin X-CSRF-Token: %d %s", ruta, r.StatusCode, cuerpo)
		}
	}
	if f.pedidos != 0 {
		t.Fatalf("sin CSRF no se lanza nada: %d", f.pedidos)
	}
	if r, _ := n.pedir("GET", rutaPreparacion, ""); r.StatusCode != 200 {
		t.Errorf("leer no lo exige: %d", r.StatusCode)
	}
	if r, cuerpo := n.pedir("POST", rutaPreparacion, `{"accion":"instalar"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 202 {
		t.Errorf("con CSRF: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("POST", rutaPreparacion+"/codigo", `{"codigo":"pegado-secreto-123"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 204 || strings.Contains(cuerpo, "pegado-secreto") {
		t.Errorf("código: %d %s", r.StatusCode, cuerpo)
	}
	if strings.Contains(registros.String(), "pegado-secreto") {
		t.Errorf("lo pegado quedó en el registro del servicio: %s", registros.String())
	}
}

// TestPrepararFrenoClaves: el freno de claves falsas también cubre estas rutas: una dirección frenada recibe 429 antes de
// que se mire la clave.
func TestPrepararFrenoClaves(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	f := nuevaPreparacionFalsa()
	s.preparar = f
	global, _ := s.ActivarGlobal()
	buena := claveDe(t, s, global)
	ip := netip.MustParseAddr("198.51.100.7")
	for range 30 {
		s.claves.fallo(ip, time.Now())
	}
	r := httptest.NewRequest("POST", rutaPreparacion, strings.NewReader(`{"accion":"instalar"}`))
	r.Host, r.RemoteAddr = "127.0.0.1", "198.51.100.7:4000"
	r.Header.Set("Authorization", "Bearer "+buena)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 429 || f.pedidos != 0 {
		t.Fatalf("frenada: %d %s (%d preparaciones)", w.Code, w.Body, f.pedidos)
	}
}

// TestPeticionesEnCursoDelAgente: EnCurso cuenta las peticiones sin terminar (en cola o en marcha) de ese agente en todas
// las instancias.
func TestPeticionesEnCursoDelAgente(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{Demora: time.Hour})
	global, _ := s.ActivarGlobal()
	pry, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	claveGlobal := claveDe(t, s, global)
	for _, clave := range []string{claveGlobal, claveGlobal, claveDe(t, s, pry)} {
		if r, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/trabajos", clave, hola); r.StatusCode != 202 {
			t.Fatalf("trabajo: %d %s", r.StatusCode, cuerpo)
		}
	}
	if n := s.peticionesEnCurso("simulado"); n != 3 {
		t.Fatalf("simulado: %d en curso, se esperaban 3", n)
	}
	if n := s.peticionesEnCurso("claude"); n != 0 {
		t.Fatalf("claude: %d", n)
	}
}

// Ruling 18: mientras se instala, actualiza o desinstala el CLI de un agente, sus peticiones nuevas reciben el error de
// agente no disponible de siempre, sin quedarse esperando; al iniciar sesión, no.
func TestAgentePreparandoNoDisponible(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	f := nuevaPreparacionFalsa()
	s.preparar = f
	global, _ := s.ActivarGlobal()
	clave := claveDe(t, s, global)
	for estado, quiere := range map[string]int{"instalando": 503, "actualizando": 503, "desinstalando": 503, "esperando_usuario": 200, "listo": 200, "error": 200} {
		f.fijar("simulado", contrato.EstadoPreparacion{Agente: "simulado", Estado: estado})
		r, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", clave, hola)
		if r.StatusCode != quiere || (quiere == 503 && !strings.Contains(cuerpo, "agente_no_disponible")) {
			t.Errorf("%s: %d %s", estado, r.StatusCode, cuerpo)
		}
	}
}

// TestVersionesAgentesGuardadas: GET /api/v1/agentes/versiones sirve la última comprobación; la repite con ?forzar=1 o
// cuando una preparación terminó después (la versión instalada cambió).
func TestVersionesAgentesGuardadas(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	f := nuevaPreparacionFalsa()
	s.preparar = f
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	versiones := func(ruta string) string {
		t.Helper()
		r, cuerpo := pedirAdmin(t, s, ts.URL, "GET", ruta, "")
		if r.StatusCode != 200 {
			t.Fatalf("%s: %d %s", ruta, r.StatusCode, cuerpo)
		}
		return cuerpo
	}
	primera := versiones("/api/v1/agentes/versiones")
	if segunda := versiones("/api/v1/agentes/versiones"); segunda != primera || len(f.llamadas()) != 1 {
		t.Fatalf("la segunda es la guardada: %s / %s (%v)", primera, segunda, f.llamadas())
	}
	s.muVersiones.Lock()
	s.versionesEn = s.versionesEn.Add(-forzarCada - time.Second) // forzar repite solo pasado forzarCada
	s.muVersiones.Unlock()
	versiones("/api/v1/agentes/versiones?forzar=1")
	s.muVersiones.Lock()
	fin := s.versionesEn.Add(time.Nanosecond) // terminó justo después de la última comprobación
	s.muVersiones.Unlock()
	time.Sleep(20 * time.Millisecond) // el reloj de Windows avanza a saltos
	f.fijar("claude", contrato.EstadoPreparacion{Agente: "claude", Estado: "listo", Fin: &fin})
	versiones("/api/v1/agentes/versiones")
	versiones("/api/v1/agentes/versiones")
	if l := f.llamadas(); len(l) != 3 || l[0] || !l[1] || l[2] {
		t.Fatalf("llamadas a Versiones (forzar): %v", l)
	}
}

// TestVersionesUnaALaVez: muchos ?forzar=1 a la vez lanzan una sola comprobación y todos reciben su resultado; uno más
// antes de un minuto da la guardada y, pasado el minuto, se repite.
func TestVersionesUnaALaVez(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	f := nuevaPreparacionFalsa()
	f.espera = make(chan struct{})
	s.preparar = f
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	const ruta = "/api/v1/agentes/versiones?forzar=1"
	cuerpos := make([]string, 8)
	var grupo sync.WaitGroup
	for i := range cuerpos {
		grupo.Go(func() { _, cuerpos[i] = pedirAdmin(t, s, ts.URL, "GET", ruta, "") })
	}
	for len(f.llamadas()) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond) // las demás llegan mientras la primera sigue en marcha
	close(f.espera)
	grupo.Wait()
	for _, cuerpo := range cuerpos {
		if cuerpo != cuerpos[0] || !strings.Contains(cuerpo, `"instalada":"1.0"`) {
			t.Fatalf("todas reciben la misma: %q", cuerpos)
		}
	}
	if l := f.llamadas(); len(l) != 1 {
		t.Fatalf("una sola comprobación: %v", l)
	}
	pedirAdmin(t, s, ts.URL, "GET", ruta, "")
	if l := f.llamadas(); len(l) != 1 {
		t.Fatalf("antes de un minuto, la guardada: %v", l)
	}
	s.muVersiones.Lock()
	s.versionesEn = s.versionesEn.Add(-forzarCada - time.Second)
	s.muVersiones.Unlock()
	if _, cuerpo := pedirAdmin(t, s, ts.URL, "GET", ruta, ""); !strings.Contains(cuerpo, `"instalada":"1.1.0"`) || len(f.llamadas()) != 2 {
		t.Fatalf("pasado el minuto se repite: %s %v", cuerpo, f.llamadas())
	}
}

// TestFijarBuscarVersiones: servicio.buscar_versiones_agentes se guarda en la configuración global y vale en marcha (es
// lo que lee la preparación antes de consultar la red).
func TestFijarBuscarVersiones(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	global, _ := configuracion.RutaGlobal()
	for _, buscar := range []bool{false, true} {
		if err := s.fijarBusquedas(nil, &buscar); err != nil {
			t.Fatal(err)
		}
		texto, _ := os.ReadFile(global)
		if s.buscarVersiones.Load() != buscar || !strings.Contains(string(texto), "buscar_versiones_agentes: "+map[bool]string{true: "true", false: "false"}[buscar]) {
			t.Fatalf("%v: %v %s", buscar, s.buscarVersiones.Load(), texto)
		}
	}
}

// TestRutaBuscarVersiones: Ajustes cambia servicio.buscar_versiones_agentes con la misma ruta que la búsqueda de
// actualizaciones, sin tocar esa, y el estado lo devuelve.
func TestRutaBuscarVersiones(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	ruta := "/api/v1/servicio/actualizacion"
	antes := s.buscar.Load()
	if r, cuerpo := pedirAdmin(t, s, ts.URL, "PUT", ruta, `{"buscar_versiones_agentes": false}`); r.StatusCode != 200 ||
		!strings.Contains(cuerpo, `"buscar_versiones_agentes":false`) || s.buscarVersiones.Load() || s.buscar.Load() != antes {
		t.Fatalf("%d %s %v %v", r.StatusCode, cuerpo, s.buscarVersiones.Load(), s.buscar.Load())
	}
	if _, cuerpo := pedirAdmin(t, s, ts.URL, "PUT", ruta, `{"buscar_versiones_agentes": true}`); !strings.Contains(cuerpo, `"buscar_versiones_agentes":true`) || !s.buscarVersiones.Load() {
		t.Fatalf("%s", cuerpo)
	}
	if _, cuerpo := pedirAdmin(t, s, ts.URL, "PUT", ruta, `{"buscar": false}`); s.buscar.Load() || !s.buscarVersiones.Load() {
		t.Fatalf("buscar solo cambia la de Tramtai: %s", cuerpo)
	}
	// Las dos a la vez se guardan juntas.
	if _, cuerpo := pedirAdmin(t, s, ts.URL, "PUT", ruta, `{"buscar": true, "buscar_versiones_agentes": false}`); !s.buscar.Load() || s.buscarVersiones.Load() ||
		!strings.Contains(cuerpo, `"buscar":true`) || !strings.Contains(cuerpo, `"buscar_versiones_agentes":false`) {
		t.Fatalf("las dos: %s", cuerpo)
	}
}

// guardarComando fija comandos_agentes.<agente> en la configuración global y vale al momento: el adaptador, creado con
// el nombre del programa, llega al CLI recién instalado fuera del PATH del servicio; con "" se quita.
func TestGuardarComando(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	ruta := filepath.Join(t.TempDir(), "claude-tramtai-prueba")
	if runtime.GOOS == "windows" {
		ruta += ".exe"
	}
	if err := os.WriteFile(ruta, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adaptadores.FijarRutaPrograma("claude", "") })
	if err := s.guardarComando("claude", ruta); err != nil {
		t.Fatal(err)
	}
	global, _ := configuracion.RutaGlobal()
	if texto, _ := os.ReadFile(global); !strings.Contains(string(texto), "claude-tramtai-prueba") {
		t.Fatalf("config.yaml: %s", texto)
	}
	if encontrado, err := adaptadores.ResolverPrograma("claude"); err != nil || encontrado != ruta {
		t.Fatalf("el adaptador lo encuentra: %q %v", encontrado, err)
	}
	if err := s.guardarComando("claude", ""); err != nil {
		t.Fatal(err)
	}
	if texto, _ := os.ReadFile(global); strings.Contains(string(texto), "claude-tramtai-prueba") {
		t.Fatalf("se quita de config.yaml: %s", texto)
	}
	if encontrado, _ := adaptadores.ResolverPrograma("claude"); encontrado == ruta {
		t.Fatal("ya no se usa la ruta guardada")
	}
}

// TestPreparacionDeVerdad, con la preparación de internal/preparar y sin lanzar nada: al crearla recoge la que el
// servicio anterior dejó a medias (queda interrumpida) y sus errores de entrada son un 400 que no repite lo pegado.
func TestPreparacionDeVerdad(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	carpeta := filepath.Join(s.opc.Datos, "agentes", "preparaciones")
	if err := os.MkdirAll(carpeta, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(carpeta, "codex.json"), []byte(`{"agente":"codex","accion":"instalar","estado":"instalando"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s.preparar = s.nuevaPreparacion()
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	if r, cuerpo := pedirAdmin(t, s, ts.URL, "GET", "/api/v1/agentes/codex/preparacion", ""); r.StatusCode != 200 ||
		!strings.Contains(cuerpo, `"estado":"error"`) || !strings.Contains(cuerpo, `"motivo_codigo":"interrumpida"`) {
		t.Fatalf("la que quedó a medias: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedirAdmin(t, s, ts.URL, "POST", "/api/v1/agentes/claude/preparacion/codigo", `{"codigo":"pegado-secreto-123"}`); r.StatusCode != 400 ||
		strings.Contains(cuerpo, "pegado-secreto") {
		t.Fatalf("código sin inicio de sesión: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedirAdmin(t, s, ts.URL, "POST", "/api/v1/agentes/antigravity/preparacion", `{"accion":"iniciar_sesion"}`); r.StatusCode != 400 {
		t.Fatalf("Antigravity no inicia sesión: %d %s", r.StatusCode, cuerpo)
	}
}

// Ruling 19: Cerrar el servicio cierra la preparación (que corta las que están en marcha).
func TestCerrarCierraLaPreparacion(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	f := nuevaPreparacionFalsa()
	s.preparar = f
	s.Cerrar()
	if !f.cerrada {
		t.Fatal("Cerrar no cerró la preparación")
	}
}
