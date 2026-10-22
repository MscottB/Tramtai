package servicio

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
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
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
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
	// Solo se guarda la huella de la clave de un punto: para conocerla hay que regenerarla.
	token, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "app"))
	tokenFijo, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "fijo"))
	s.Recargar()
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

	r, cuerpo := pedir(t, "http://127.0.0.1:"+strconv.Itoa(puerto), "POST", "/api/v1/ejecutar", tokenFijo, `{"agente":"otro","mensajes":[{"rol":"usuario","contenido":"x"}]}`)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"modelo_enviado":"m9"`) {
		t.Fatalf("puerto propio con modelo forzado: %d %s", r.StatusCode, cuerpo)
	}
	// Al cambiar el punto se reabre en el mismo puerto: el anterior lo libera antes.
	rutaGlobal, _ := configuracion.RutaGlobal()
	if err := configuracion.EditarPuntos(rutaGlobal, configuracion.OrigenGlobal, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		lista[1].Modelo = "m8"
		return lista, nil
	}); err != nil {
		t.Fatal(err)
	}
	if estado := s.Recargar(); !estado[1].Activo {
		t.Fatalf("el punto con puerto propio debía reabrirse: %+v", estado[1])
	}
	// Las conexiones guardadas eran del servidor anterior, que las cerró; un POST no se reintenta solo.
	http.DefaultClient.CloseIdleConnections()
	r, cuerpo = pedir(t, "http://127.0.0.1:"+strconv.Itoa(puerto), "POST", "/api/v1/ejecutar", tokenFijo, `{"agente":"otro","mensajes":[{"rol":"usuario","contenido":"x"}]}`)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"modelo_enviado":"m8"`) {
		t.Fatalf("punto reabierto en su puerto: %d %s", r.StatusCode, cuerpo)
	}

	// El punto del proyecto se abre al activar su instancia y se cierra al desactivarla. Un punto en local.yaml
	// exige que la carpeta sea confiable en la configuración global: sin confianza el espacio se abre sin él.
	sinConfianza, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	if p := s.puntos["del_proyecto"]; p != nil {
		t.Fatalf("un punto en local.yaml de un proyecto no confiable no se abre: %+v", p)
	}
	s.Desactivar(sinConfianza.ID)
	rutaGlobal = os.Getenv("TRAMTAI_CONFIG_GLOBAL")
	previa, _ := os.ReadFile(rutaGlobal)
	os.WriteFile(rutaGlobal, append(previa, []byte("proyectos_confiables: ['"+proyecto+"']\n")...), 0o600)
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
	rutaGlobal, _ := configuracion.RutaGlobal()
	if err := configuracion.EditarPuntos(rutaGlobal, configuracion.OrigenGlobal, func(lista []ConfigPunto) ([]ConfigPunto, error) {
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
	token, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "dos"))
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

func TestEscucharConPuertoOcupado(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ocupado.Close()
	puerto := ocupado.Addr().(*net.TCPAddr).Port
	if PuertoLibre("127.0.0.1", puerto) {
		t.Fatal("el puerto en uso debe verse ocupado")
	}
	if _, _, err := Escuchar("127.0.0.1", puerto, false); err == nil || !strings.Contains(err.Error(), "ocupado") {
		t.Fatalf("sin automático, el puerto ocupado es un error que lo explica: %v", err)
	}
	oyente, usado, err := Escuchar("127.0.0.1", puerto, true)
	if err != nil || usado == puerto {
		t.Fatalf("con automático se usa otro puerto: %d %v", usado, err)
	}
	oyente.Close()
}

func TestNormalizarReceta(t *testing.T) {
	bien := ConfigPunto{Nombre: "r", Esquema: map[string]any{"type": "object"}, Respaldo: []string{"codex", "gemini/flash"}}
	if _, err := normalizarPunto(bien, "global", 4000); err != nil {
		t.Fatalf("receta válida: %v", err)
	}
	for _, mal := range []ConfigPunto{
		{Nombre: "r", Esquema: map[string]any{"type": 5}},
		{Nombre: "r", Respaldo: []string{"perfil/x"}},
	} {
		_, err := normalizarPunto(mal, "global", 4000)
		if nucleo.CodigoDe(err) != nucleo.CodigoConfiguracionInvalida {
			t.Fatalf("%+v debía rechazarse: %v", mal, err)
		}
		if strings.Count(err.Error(), "esquema no válido") > 1 {
			t.Fatalf("mensaje con el prefijo repetido: %v", err)
		}
	}
}

func TestNombrePuntoValido(t *testing.T) {
	for nombre, valido := range map[string]bool{"app": true, "bot_soporte-2": true, "": false, "x/../../token-global": false, "Mayus": false, `a\b`: false} {
		if NombrePuntoValido(nombre) != valido {
			t.Errorf("%q: se esperaba %v", nombre, valido)
		}
	}
}

func TestClavePuntoSoloHuella(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "tokens", "acceso_x")
	token, err := RotarTokenPunto(ruta)
	if err != nil || !strings.HasPrefix(token, "tt_") {
		t.Fatalf("clave nueva: %q %v", token, err)
	}
	datos, _ := os.ReadFile(ruta)
	if strings.Contains(string(datos), token) || !strings.HasPrefix(string(datos), api.PrefijoHuella) {
		t.Fatalf("el archivo debe guardar solo la huella: %q", datos)
	}
	if huella, err := LeerHuellaPunto(ruta); err != nil || huella != api.HuellaToken(token) {
		t.Fatalf("huella leída: %q %v", huella, err)
	}

	// Una clave antigua guardada en claro se convierte a huella y sigue valiendo.
	antigua := "tt_" + strings.Repeat("a", 64)
	_ = os.WriteFile(ruta, []byte(antigua+"\n"), 0o600)
	if huella, err := LeerHuellaPunto(ruta); err != nil || huella != api.HuellaToken(antigua) {
		t.Fatalf("migración: %q %v", huella, err)
	}
	if datos, _ := os.ReadFile(ruta); strings.Contains(string(datos), antigua) {
		t.Fatal("la clave antigua no debe quedar en claro")
	}

	EliminarTokenPunto(filepath.Dir(filepath.Dir(ruta)), "x")
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatalf("eliminar la API debe borrar su clave: %v", err)
	}
}

// conBase sustituye la apertura de las bases durante la prueba.
func conBase(t *testing.T, f func(string) (*persistencia.Almacen, error)) {
	anterior := abrirBase
	abrirBase = f
	t.Cleanup(func() { abrirBase = anterior })
}

// Recargar lee las claves de las APIs fuera de s.mu: mientras espera una lectura lenta, el servicio sigue atendiendo
// (hallazgo 12).
func TestRecargarNoBloqueaElServicio(t *testing.T) {
	s, _, _ := entorno(t, puntosOrigen, "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	dentro, soltar := make(chan struct{}), make(chan struct{})
	var una sync.Once
	anterior := leerHuellaPunto
	leerHuellaPunto = func(ruta string) (string, error) {
		una.Do(func() { close(dentro); <-soltar })
		return anterior(ruta)
	}
	t.Cleanup(func() { leerHuellaPunto = anterior })
	hecho := make(chan struct{})
	go func() { s.Recargar(); close(hecho) }()
	<-dentro
	respondio := make(chan struct{})
	go func() { s.instancia("global"); close(respondio) }()
	select {
	case <-respondio:
	case <-time.After(5 * time.Second):
		close(soltar)
		t.Fatal("con Recargar leyendo una clave, el servicio dejó de atender")
	}
	close(soltar)
	<-hecho
}

// Activar la misma instancia a la vez abre su base una sola vez y todos reciben la misma.
func TestActivarALaVez(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	var aperturas atomic.Int32
	conBase(t, func(ruta string) (*persistencia.Almacen, error) {
		aperturas.Add(1)
		time.Sleep(100 * time.Millisecond)
		return persistencia.Abrir(ruta)
	})
	instancias := make([]*Instancia, 5)
	var grupo sync.WaitGroup
	for i := range instancias {
		grupo.Go(func() {
			inst, err := s.ActivarGlobal()
			if err != nil {
				t.Error(err)
			}
			instancias[i] = inst
		})
	}
	grupo.Wait()
	for _, inst := range instancias {
		if inst == nil || inst != instancias[0] {
			t.Fatal("activaciones a la vez dieron instancias distintas")
		}
	}
	if n := aperturas.Load(); n != 1 {
		t.Fatalf("la base se abrió %d veces", n)
	}
}

// Mientras una instancia abre su base (migraciones, recuperación), las demás siguen respondiendo.
func TestAbrirUnaInstanciaNoBloqueaLasDemas(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	soltar := make(chan struct{})
	conBase(t, func(ruta string) (*persistencia.Almacen, error) {
		<-soltar
		return persistencia.Abrir(ruta)
	})
	listo := make(chan error, 1)
	go func() {
		_, err := s.ActivarProyecto(proyecto)
		listo <- err
	}()
	for i := 0; ; i++ {
		s.mu.RLock()
		abriendo := len(s.abriendo)
		s.mu.RUnlock()
		if abriendo == 1 {
			break
		}
		if i > 200 {
			t.Fatal("el proyecto no empezó a abrirse")
		}
		time.Sleep(10 * time.Millisecond)
	}
	hecho := make(chan struct{})
	go func() {
		s.ActivarGlobal()
		s.Estado(context.Background())
		close(hecho)
	}()
	select {
	case <-hecho:
	case <-time.After(2 * time.Second):
		t.Fatal("la instancia global quedó bloqueada mientras se abría otra")
	}
	close(soltar)
	if err := <-listo; err != nil {
		t.Fatal(err)
	}
}

// Cerrar espera a las tareas de fondo que usan las bases (una purga en curso) antes de cerrarlas.
func TestCerrarEsperaElFondo(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	var terminada atomic.Bool
	s.fondo.Go(func() {
		time.Sleep(100 * time.Millisecond)
		terminada.Store(true)
	})
	s.Cerrar()
	if !terminada.Load() {
		t.Fatal("Cerrar no esperó a la tarea de fondo")
	}
}

// Un instancias.json dañado no deja el servicio sin aviso: se aparta, se informa y se abre la global.
func TestRestaurarConInstanciasDanado(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	ruta := filepath.Join(s.opc.Datos, "instancias.json")
	os.MkdirAll(s.opc.Datos, 0o700)
	os.WriteFile(ruta, []byte("{no es json"), 0o600)
	errores := s.Restaurar()
	if len(errores) != 1 || !strings.Contains(errores[0].Error(), "dañado") {
		t.Fatalf("errores: %v", errores)
	}
	if apartados, _ := filepath.Glob(ruta + ".dañado-*"); len(apartados) != 1 {
		t.Fatalf("no se apartó el archivo dañado con su fecha: %v", apartados)
	}
	if _, activa := s.instancias["global"]; !activa {
		t.Fatal("la instancia global debía abrirse")
	}
}

// /healthz no expone el identificador del servicio y solo prueba la identidad con el reto: el HMAC del token de servicio.
func TestSaludConReto(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, cuerpo := pedir(t, ts.URL, "GET", "/healthz", "", ""); strings.Contains(cuerpo, "prueba") {
		t.Fatalf("sin reto no hay prueba: %s", cuerpo)
	}
	_, cuerpo := pedir(t, ts.URL, "GET", "/healthz?reto=abc", "", "")
	if !strings.Contains(cuerpo, `"prueba":"`+contrato.PruebaIdentidad(s.tokenAdmin, s.Base(), "abc")+`"`) {
		t.Fatalf("la prueba es el HMAC del token sobre el reto: %s", cuerpo)
	}
}

// La CLI de la versión anterior (0.55) reconoce al servicio por el identificador en /healthz, sin reto: se lo da solo
// al mismo usuario del sistema, que ya puede leerlo en servicio.json. A otro usuario o a otro equipo, no (hallazgo 2).
func TestSaludParaLaCLIAnterior(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("sin identificación del dueño de la conexión en este sistema")
	}
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, cuerpo := pedir(t, ts.URL, "GET", "/healthz", "", ""); !strings.Contains(cuerpo, `"servicio":"`+s.ID+`"`) {
		t.Fatalf("el mismo usuario recibe el identificador: %s", cuerpo)
	}
	r := httptest.NewRequest("GET", "/healthz", nil)
	r.Host, r.RemoteAddr = "127.0.0.1", "203.0.113.5:4000"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), s.ID) {
		t.Fatalf("desde otro equipo no hay identificador: %d %s", w.Code, w.Body)
	}
}
