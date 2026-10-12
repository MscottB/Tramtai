package servicio

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
)

// entorno prepara una configuración global aislada, un proyecto y el servicio con el agente simulado.
func entorno(t *testing.T, global, local string, agente adaptadores.Adaptador) (*Servicio, *httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	rutaGlobal := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(rutaGlobal, []byte("predeterminados:\n  agente: simulado\nmodelos:\n  perfiles:\n    rapido:\n      simulado: m1\n"+global), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", rutaGlobal)
	proyecto := filepath.Join(dir, "Mi Proyecto")
	os.MkdirAll(filepath.Join(proyecto, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(proyecto, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Demo App\n"), 0o600)
	if local != "" {
		os.WriteFile(filepath.Join(proyecto, ".tramtai", "local.yaml"), []byte(local), 0o600)
	}
	c, err := nucleo.CargarConfiguracion("")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	puerto := ts.Listener.Addr().(*net.TCPAddr).Port
	s, err := Nuevo(Opciones{Datos: filepath.Join(dir, "datos"), Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": agente},
		Semaforos: planificador.NuevosSemaforos(map[string]int{"simulado": 2}), Host: "127.0.0.1", Puerto: puerto, Hosts: []string{"127.0.0.1", "localhost", "::1"}})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	t.Cleanup(func() {
		ts.Close()
		s.Cerrar()
	})
	return s, ts, proyecto
}

func pedir(t *testing.T, base, metodo, ruta, token, cuerpo string, cabeceras ...string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(metodo, base+ruta, strings.NewReader(cuerpo))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(cabeceras); i += 2 {
		req.Header.Set(cabeceras[i], cabeceras[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	datos, _ := io.ReadAll(resp.Body)
	return resp, string(datos)
}

const hola = `{"mensajes":[{"rol":"usuario","contenido":"hola"}]}`

func TestInstanciasSeparadasPorToken(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	pry, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(pry.ID, "demo_app_") || len(pry.ID) != len("demo_app_")+8 {
		t.Fatalf("identificador de proyecto %q", pry.ID)
	}
	r, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", global.token, hola)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"instancia":"global"`) {
		t.Fatalf("global: %d %s", r.StatusCode, cuerpo)
	}
	id := r.Header.Get("Location")
	if r, _ := pedir(t, ts.URL, "GET", id, pry.token, ""); r.StatusCode != 404 {
		t.Fatalf("el token del proyecto no ve trabajos de la global: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "GET", id, s.tokenAdmin, "", "Tramtai-Instancia", "global"); r.StatusCode != 200 {
		t.Fatalf("la administración accede indicando la instancia: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/modelos", "otro", ""); r.StatusCode != 401 {
		t.Fatalf("token desconocido: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/servicio", global.token, ""); r.StatusCode != 401 {
		t.Fatalf("el token de instancia no administra: %d", r.StatusCode)
	}
	r, cuerpo = pedir(t, ts.URL, "GET", "/api/v1/servicio", s.tokenAdmin, "")
	if r.StatusCode != 200 || !strings.Contains(cuerpo, pry.ID) || !strings.Contains(cuerpo, `"completado":1`) {
		t.Fatalf("estado: %d %s", r.StatusCode, cuerpo)
	}
}

func TestPuntosDeAcceso(t *testing.T) {
	libre, _ := net.Listen("tcp", "127.0.0.1:0")
	puerto := libre.Addr().(*net.TCPAddr).Port
	libre.Close()
	global := `puntos_acceso:
  - nombre: app
    contratos: [openai]
    modelos_permitidos: [simulado/m1, perfil/rapido]
    alias_modelos:
      gpt-4o-mini: simulado/m1
  - nombre: fijo
    puerto: ` + strconv.Itoa(puerto) + `
    agente: simulado
    modelo: m9
    forzar_modelo: true
`
	s, ts, proyecto := entorno(t, global, "puntos_acceso:\n  - nombre: del_proyecto\n", adaptadores.Simulado{})
	global1, _ := s.ActivarGlobal()
	estado := s.Recargar()
	if len(estado) != 2 || !estado[0].Activo || !estado[1].Activo {
		t.Fatalf("puntos: %+v %v", estado, s.erroresPuntos)
	}
	token, _ := LeerOCrearToken(RutaTokenPunto(s.opc.Datos, "app"))
	chat := func(modelo string) string {
		return `{"model":"` + modelo + `","messages":[{"role":"user","content":"hola"}]}`
	}
	if r, cuerpo := pedir(t, ts.URL, "POST", "/p/app/openai/v1/chat/completions", token, chat("gpt-4o-mini")); r.StatusCode != 200 {
		t.Fatalf("alias: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts.URL, "POST", "/p/app/openai/v1/chat/completions", token, chat("simulado/m2")); r.StatusCode != 400 || !strings.Contains(cuerpo, "modelo_no_permitido") {
		t.Fatalf("modelo no permitido: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts.URL, "POST", "/p/app/api/v1/trabajos", token, hola); r.StatusCode != 404 {
		t.Fatalf("la API nativa no está habilitada en el punto: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "POST", "/p/app/openai/v1/chat/completions", global1.token, chat("gpt-4o-mini")); r.StatusCode != 401 {
		t.Fatalf("el token de la instancia no vale en el punto: %d", r.StatusCode)
	}
	_, lista := pedir(t, ts.URL, "GET", "/api/v1/trabajos", global1.token, "")
	if !strings.Contains(lista, `"punto_acceso":"app"`) {
		t.Fatalf("el trabajo registra su punto de acceso: %s", lista)
	}

	tokenFijo, _ := LeerOCrearToken(RutaTokenPunto(s.opc.Datos, "fijo"))
	r, cuerpo := pedir(t, "http://127.0.0.1:"+strconv.Itoa(puerto), "POST", "/api/v1/ejecutar", tokenFijo, `{"agente":"otro","mensajes":[{"rol":"usuario","contenido":"x"}]}`)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"modelo_enviado":"m9"`) {
		t.Fatalf("puerto propio con modelo forzado: %d %s", r.StatusCode, cuerpo)
	}

	// El punto del proyecto se abre al activar su instancia y se cierra al desactivarla.
	pry, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	if p := s.puntos["del_proyecto"]; p == nil || p.err != "" || p.cfg.Instancia != pry.ID {
		t.Fatalf("punto del proyecto: %+v", p)
	}
	s.Desactivar(pry.ID)
	if p := s.puntos["del_proyecto"]; p != nil {
		t.Fatalf("el punto local del proyecto debía retirarse al desactivarlo: %+v", p)
	}
}

func TestRecargarSinTocarLosDemas(t *testing.T) {
	s, ts, _ := entorno(t, "puntos_acceso:\n  - nombre: uno\n  - nombre: dos\n", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	antes := s.puntos["uno"]
	rutaGlobal, _ := nucleo.RutaGlobal()
	if err := EditarPuntos(rutaGlobal, nucleo.OrigenGlobal, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		falso := false
		lista[1].Activo = &falso
		return lista, nil
	}); err != nil {
		t.Fatal(err)
	}
	s.Recargar()
	if s.puntos["uno"] != antes {
		t.Fatal("un punto sin cambios no debe reabrirse")
	}
	token, _ := LeerOCrearToken(RutaTokenPunto(s.opc.Datos, "dos"))
	if r, _ := pedir(t, ts.URL, "GET", "/p/dos/api/v1/modelos", token, ""); r.StatusCode != 404 {
		t.Fatalf("un punto desactivado no responde: %d", r.StatusCode)
	}
}

func TestDesactivarVaciaLaCola(t *testing.T) {
	s, ts, _ := entorno(t, "concurrencia:\n  maximo_trabajos_instancia: 1\n", "", adaptadores.Simulado{Demora: time.Minute})
	g, _ := s.ActivarGlobal()
	pedir(t, ts.URL, "POST", "/api/v1/trabajos", g.token, hola)
	_, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/trabajos", g.token, `{"mensajes":[{"rol":"usuario","contenido":"segundo"}]}`)
	inicio := strings.Index(cuerpo, "trabajo_")
	id := cuerpo[inicio : inicio+len("trabajo_")+24]
	if err := s.Desactivar("global"); err != nil {
		t.Fatal(err)
	}
	alm, err := persistencia.Abrir(filepath.Join(s.opc.Datos, "instancias", "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer alm.Cerrar()
	if tr, _ := alm.Obtener(t.Context(), id); tr.Estado != nucleo.Cancelado {
		t.Fatalf("el trabajo en cola debía cancelarse: %s", tr.Estado)
	}
}

func TestEditarPuntosConservaComentarios(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(ruta, []byte("# mi configuración\nservicio:\n  puerto: 17373 # fijo\n"), 0o600)
	err := EditarPuntos(ruta, nucleo.OrigenGlobal, func(l []ConfigPunto) ([]ConfigPunto, error) {
		return append(l, ConfigPunto{Nombre: "nuevo", Puerto: 17401, Agente: "claude"}), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	datos, _ := os.ReadFile(ruta)
	if !strings.Contains(string(datos), "# mi configuración") || !strings.Contains(string(datos), "# fijo") || !strings.Contains(string(datos), "puerto: 17401") {
		t.Fatalf("archivo:\n%s", datos)
	}
	if err := EditarPuntos(ruta, nucleo.OrigenProyecto, func(l []ConfigPunto) ([]ConfigPunto, error) { return l, nil }); err == nil {
		t.Fatal("puntos_acceso en la configuración versionada debe rechazarse")
	}
	if datos2, _ := os.ReadFile(ruta); string(datos2) != string(datos) {
		t.Fatal("un cambio inválido debe dejar el archivo como estaba")
	}
}

func TestResultadoExpiradoResponde410(t *testing.T) {
	s, ts, _ := entorno(t, "retencion:\n  horas_resultado: 0\n", "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	r, _ := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", global.token, hola)
	id := r.Header.Get("Location")
	if err := global.Purgar(t.Context()); err != nil {
		t.Fatal(err)
	}
	if r, cuerpo := pedir(t, ts.URL, "GET", id+"/salida", global.token, ""); r.StatusCode != 410 || !strings.Contains(cuerpo, "resultado_expirado") {
		t.Fatalf("salida expirada: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts.URL, "GET", id, global.token, ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"disponibilidad":"expirada"`) || strings.Contains(cuerpo, "hola") {
		t.Fatalf("los metadatos siguen, sin contenido: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts.URL, "POST", id+"/reintentar", global.token, ""); r.StatusCode != 410 || !strings.Contains(cuerpo, "entrada_no_disponible") {
		t.Fatalf("reintento sin entradas: %d %s", r.StatusCode, cuerpo)
	}
	if _, cuerpo := pedir(t, ts.URL, "GET", id+"/eventos", global.token, ""); !strings.Contains(cuerpo, "event: aviso") || !strings.Contains(cuerpo, "event: fin") {
		t.Fatalf("SSE avisa de la expiración: %s", cuerpo)
	}
}

func TestDrenarAlCerrar(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{Demora: 300 * time.Millisecond})
	s.opc.PlazoDrenaje = 5 * time.Second
	global, _ := s.ActivarGlobal()
	r, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/trabajos", global.token, hola)
	if r.StatusCode != 202 {
		t.Fatalf("%d %s", r.StatusCode, cuerpo)
	}
	id := r.Header.Get("Location")
	for i := 0; i < 100 && !strings.Contains(cuerpo, `"ejecutando"`); i++ {
		time.Sleep(10 * time.Millisecond)
		_, cuerpo = pedir(t, ts.URL, "GET", id, global.token, "")
	}
	s.Drenar()
	if r, _ := pedir(t, ts.URL, "POST", "/api/v1/trabajos", global.token, hola); r.StatusCode != 503 {
		t.Fatalf("tras drenar se rechazan trabajos nuevos: %d", r.StatusCode)
	}
	if _, cuerpo := pedir(t, ts.URL, "GET", id, global.token, ""); !strings.Contains(cuerpo, `"completado"`) {
		t.Fatalf("el trabajo en curso terminó y su resultado se puede recoger: %s", cuerpo)
	}
}
