package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/planificador"
	"tramtai/internal/preparar"
	"tramtai/internal/servicio"
	"tramtai/internal/terminos"
)

// TestSuscripcionPersonalCLI: «agentes suscripcion <agente> --personal» enseña los riesgos 1 y 2 y las condiciones, y
// sin «acepto» no escribe nada ni reinicia; con «acepto», las tres claves en la global del servidor y reinicia. Subir
// las peticiones a la vez pide confirmarlo otra vez; fuera de 1 a 3 no se acepta. También con el alias en inglés.
func TestSuscripcionPersonalCLI(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	simularTOTP(t)
	global := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	reinicios := 0
	anterior := reiniciarServidor
	t.Cleanup(func() { reiniciarServidor = anterior })
	reiniciarServidor = func() error { reinicios++; return nil }

	salida, err := ejecutarAdmin(t, "no\n", "agentes", "suscripcion", "claude", "--personal")
	if err == nil || reinicios != 0 || !strings.Contains(salida, terminos.Riesgos(true)[0]) || !strings.Contains(salida, terminos.Riesgos(true)[1]) ||
		!strings.Contains(salida, "solo la usas tú") {
		t.Fatalf("sin «acepto»: %v %d\n%s", err, reinicios, salida)
	}
	if _, err := os.Stat(global); err == nil {
		t.Fatal("sin «acepto» no se escribe nada")
	}
	if _, err := ejecutarAdmin(t, "", "agentes", "suscripcion", "claude", "--personal", "--simultaneas", "4"); err == nil || !strings.Contains(err.Error(), "1 a 3") {
		t.Fatalf("4 a la vez: %v", err)
	}
	if _, err := ejecutarAdmin(t, "acepto\nno\n", "agents", "subscription", "claude", "--personal", "--simultaneas", "2"); err == nil || reinicios != 0 {
		t.Fatalf("subir a 2 sin confirmarlo: %v %d", err, reinicios)
	}
	if _, err := os.Stat(global); err == nil {
		t.Fatal("sin confirmar las 2 a la vez no se escribe nada")
	}
	if _, err := ejecutarAdmin(t, "acepto\n", "agents", "subscription", "claude", "--personal"); err != nil || reinicios != 1 {
		t.Fatalf("con «acepto»: %v %d", err, reinicios)
	}
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil || c.Texto("adaptadores.claude.autenticacion") != "suscripcion" || !c.Booleano("adaptadores.claude.suscripcion_personal") ||
		c.Entero("adaptadores.claude.suscripcion_simultaneas") != 1 {
		t.Fatalf("las tres claves: %v", err)
	}
	if _, err := ejecutarAdmin(t, "acepto\nacepto\n", "agentes", "suscripcion", "claude", "--personal", "--simultaneas", "3"); err != nil || reinicios != 2 {
		t.Fatalf("3 a la vez, confirmado: %v %d", err, reinicios)
	}
	if c, _ := configuracion.CargarConfiguracion("", agentes.Internos()); c.Entero("adaptadores.claude.suscripcion_simultaneas") != 3 {
		t.Fatal("suscripcion_simultaneas: 3")
	}
	if _, err := ejecutarAdmin(t, "acepto\n", "agentes", "suscripcion", "gemini", "--personal"); err == nil {
		t.Fatal("gemini no admite la suscripción")
	}
	t.Setenv("TRAMTAI_SERVIDOR", "")
	if _, err := ejecutarAdmin(t, "acepto\n", "agentes", "suscripcion", "claude", "--personal"); err == nil || !strings.Contains(err.Error(), "--servidor") {
		t.Fatalf("--personal fuera del servidor: %v", err)
	}
}

// TestSuscripcionServidorExpuesto: con el servidor abierto a la red, antes de preguntar nada, --personal sin TOTP del
// administrador falla con el porqué de la regla 4 (reiniciar dejaría el servicio caído) y sin --personal pide usarla;
// en los dos casos no se escribe ni se reinicia.
func TestSuscripcionServidorExpuesto(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	simularTOTP(t)
	anteriorClave, anteriorReinicio := leerClave, reiniciarServidor
	t.Cleanup(func() { leerClave, reiniciarServidor = anteriorClave, anteriorReinicio })
	leerClave = func(string) (string, error) { return "", nil } // sin TOTP
	reinicios := 0
	reiniciarServidor = func() error { reinicios++; return nil }
	global := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	config := "servicio:\n  proxy:\n    confiables: [10.0.0.0/8]\n"
	if err := os.WriteFile(global, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for orden, quiere := range map[string]string{"--personal": "TOTP", "": "--personal"} {
		args := []string{"agentes", "suscripcion", "claude"}
		if orden != "" {
			args = append(args, orden)
		}
		salida, err := ejecutarAdmin(t, "acepto\n", args...)
		if err == nil || !strings.Contains(err.Error(), quiere) || strings.Contains(salida, terminos.Riesgos(true)[0]) || reinicios != 0 {
			t.Fatalf("%q expuesto: %v %d\n%s", orden, err, reinicios, salida)
		}
		if leido, _ := os.ReadFile(global); string(leido) != config {
			t.Fatalf("%q: la configuración cambió:\n%s", orden, leido)
		}
	}
	leerClave = func(string) (string, error) { return "SECRETO", nil } // con TOTP, sí
	if _, err := ejecutarAdmin(t, "acepto\n", "agentes", "suscripcion", "claude", "--personal"); err != nil || reinicios != 1 {
		t.Fatalf("con TOTP: %v %d", err, reinicios)
	}
}

// TestEntrarPorElServicio: «agentes entrar» (agents login) ya no es solo del modo servidor: va por el servicio y, sin él,
// lo dice. Un agente sin inicio de sesión propio (gemini, antigravity) no lanza nada.
func TestEntrarPorElServicio(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "")
	for _, orden := range [][]string{{"agentes", "entrar", "claude"}, {"agents", "login", "claude"}} {
		if _, err := ejecutarAdmin(t, "", orden...); !errors.Is(err, cliente.ErrServicioInactivo) {
			t.Fatalf("%v sin servicio: %v", orden, err)
		}
	}
	for _, id := range []string{"gemini", "antigravity"} {
		if _, err := ejecutarAdmin(t, "", "agentes", "entrar", id); err == nil || !strings.Contains(err.Error(), "clave de API") {
			t.Fatalf("%s: %v", id, err)
		}
	}
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	simularTOTP(t)
	if _, err := ejecutarAdmin(t, "", "agentes", "entrar", "gemini"); err == nil || !strings.Contains(err.Error(), "clave de API") {
		t.Fatalf("gemini en el servidor: %v", err)
	}
}

// preparacionCLI hace de preparar.Preparador en el servicio de prueba: no instala nada; cada acción sigue el guion que
// da la prueba (los estados que irá viendo la orden, el último se repite).
type preparacionCLI struct {
	mu          sync.Mutex
	guion       func(agente string, p contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error)
	alPegar     []contrato.EstadoPreparacion // lo que ve la orden cuando se le pasa el código
	cola        map[string][]contrato.EstadoPreparacion
	pedidos     []string // «agente:accion» (con «+confirmar» y «+borrar» si van)
	pegados     []string
	cancelados  []string
	versiones   []contrato.VersionesAgente
	falloCodigo int    // los primeros envíos del código que fallan
	forzados    []bool // el forzar de cada consulta de versiones
	alConsultar func() // se llama en cada consulta del estado
}

func (f *preparacionCLI) siguiente(agente string) (contrato.EstadoPreparacion, bool) {
	c := f.cola[agente]
	if len(c) == 0 {
		return contrato.EstadoPreparacion{}, false
	}
	if len(c) > 1 {
		f.cola[agente] = c[1:]
	}
	return c[0], true
}

func (f *preparacionCLI) Estado(agente string) (contrato.EstadoPreparacion, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.alConsultar != nil {
		f.alConsultar()
	}
	return f.siguiente(agente)
}

func (f *preparacionCLI) Iniciar(agente string, p contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pedido := agente + ":" + p.Accion
	if p.Confirmar {
		pedido += "+confirmar"
	}
	if p.BorrarDatos {
		pedido += "+borrar"
	}
	f.pedidos = append(f.pedidos, pedido)
	cola, err := f.guion(agente, p)
	if err != nil {
		return contrato.EstadoPreparacion{}, err
	}
	f.cola[agente] = cola
	e, _ := f.siguiente(agente)
	return e, nil
}

func (f *preparacionCLI) Cancelar(agente string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelados = append(f.cancelados, agente)
	return nil
}

func (f *preparacionCLI) Codigo(agente, codigo string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.falloCodigo > 0 {
		f.falloCodigo--
		return errors.New("código no válido")
	}
	f.pegados = append(f.pegados, codigo)
	if f.alPegar != nil {
		f.cola[agente] = f.alPegar
	}
	return nil
}

func (f *preparacionCLI) Versiones(_ context.Context, forzar bool) []contrato.VersionesAgente {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forzados = append(f.forzados, forzar)
	return f.versiones
}
func (f *preparacionCLI) Cerrar() {}

func (f *preparacionCLI) vistos() (pedidos, pegados, cancelados []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.pedidos), slices.Clone(f.pegados), slices.Clone(f.cancelados)
}

// servicioConPreparacion arranca un servicio de prueba con esa preparación falsa y apunta dirDatos a él; las consultas de
// la orden, cada pocos milisegundos.
func servicioConPreparacion(t *testing.T, f *preparacionCLI) *httptest.Server {
	t.Helper()
	if f.cola == nil {
		f.cola = map[string][]contrato.EstadoPreparacion{}
	}
	datos := t.TempDir()
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Secretos: func() []string { return nil }, Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"},
		Preparacion: f})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	t.Cleanup(ts.Close)
	t.Cleanup(s.Cerrar)
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	if err := s.EscribirDescriptor(); err != nil {
		t.Fatal(err)
	}
	anteriorDatos, anteriorConsulta := dirDatos, cadaConsulta
	dirDatos, cadaConsulta = func() (string, error) { return datos, nil }, 2*time.Millisecond
	t.Cleanup(func() { dirDatos, cadaConsulta = anteriorDatos, anteriorConsulta })
	return ts
}

// siempre es un guion que no depende de la acción.
func siempre(estados ...contrato.EstadoPreparacion) func(string, contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
	return func(string, contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) { return estados, nil }
}

func est(agente, accion, estado string) contrato.EstadoPreparacion {
	return contrato.EstadoPreparacion{Agente: agente, Accion: accion, Estado: estado}
}

func salida(err error) int {
	if err == nil {
		return 0
	}
	return codigoSalida(err)
}

// TestAgentesInstalar: imprime el progreso por fases y sale con 0 en «listo»; con 1 en «error», con el motivo y las
// últimas líneas; un id desconocido falla sin tocar nada, aunque haya otros buenos; varios ids se instalan en orden.
func TestAgentesInstalar(t *testing.T) {
	f := &preparacionCLI{}
	f.guion = func(agente string, p contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		if agente == "codex" {
			e := est(agente, p.Accion, "error")
			e.MotivoCodigo, e.Mensaje, e.Lineas = "instalador_fallo", "el instalador dijo: boom", []string{"descargando", "\x1b[31mfallo feo\x1b[0m"}
			return []contrato.EstadoPreparacion{est(agente, p.Accion, "instalando"), e}, nil
		}
		ocupado := est(agente, p.Accion, "instalando")
		ocupado.Etapa = "orden"
		listo := est(agente, p.Accion, "listo")
		listo.VersionDespues = "2.1.288"
		return []contrato.EstadoPreparacion{ocupado, listo}, nil
	}
	servicioConPreparacion(t, f)

	salidaTexto, err := ejecutarAdmin(t, "", "agentes", "instalar", "claude")
	if err != nil || !strings.Contains(salidaTexto, "Instalando Claude") || !strings.Contains(salidaTexto, "con su instalador oficial") ||
		!strings.Contains(salidaTexto, "Claude está instalado (2.1.288)") {
		t.Fatalf("listo: %v\n%s", err, salidaTexto)
	}
	salidaTexto, err = ejecutarAdmin(t, "", "agentes", "install", "codex")
	if salida(err) != 1 || !strings.Contains(salidaTexto, "boom") || !strings.Contains(salidaTexto, "fallo feo") || strings.Contains(salidaTexto, "\x1b") ||
		!strings.Contains(salidaTexto, "El instalador terminó con un error") || !strings.Contains(err.Error(), "No se pudo instalar Codex") {
		t.Fatalf("error: %v\n%s", err, salidaTexto)
	}
	antes, _, _ := f.vistos()
	if _, err = ejecutarAdmin(t, "", "agentes", "instalar", "claude", "gemini"); err == nil || !strings.Contains(err.Error(), "gemini") {
		t.Fatalf("id desconocido: %v", err)
	}
	if despues, _, _ := f.vistos(); len(despues) != len(antes) {
		t.Fatalf("con un id desconocido no se toca nada: %v", despues)
	}
	f.guion = siempre(est("x", "instalar", "listo"))
	if _, err = ejecutarAdmin(t, "", "agentes", "instalar", "claude", "copilot"); err != nil {
		t.Fatal(err)
	}
	if pedidos, _, _ := f.vistos(); !slices.Equal(pedidos[len(antes):], []string{"claude:instalar", "copilot:instalar"}) {
		t.Fatalf("varios ids, en orden: %v", pedidos)
	}
}

// TestAgentesVersionNoComprobada: con una versión sin comprobar, instalar, actualizar y desinstalar preguntan (o
// --confirmar/--yes no preguntan) y repiten con confirmar; sin «s», sale con 2 sin tocar nada.
func TestAgentesVersionNoComprobada(t *testing.T) {
	f := &preparacionCLI{}
	f.guion = func(agente string, p contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		if p.Confirmar {
			return []contrato.EstadoPreparacion{est(agente, p.Accion, "listo")}, nil
		}
		e := est(agente, p.Accion, "error")
		e.MotivoCodigo = "version_no_comprobada"
		return []contrato.EstadoPreparacion{e}, nil
	}
	servicioConPreparacion(t, f)

	salidaTexto, err := ejecutarAdmin(t, "n\n", "agentes", "instalar", "claude")
	if salida(err) != 2 || !strings.Contains(salidaTexto, "aún no está comprobada") {
		t.Fatalf("sin confirmar: %v\n%s", err, salidaTexto)
	}
	if pedidos, _, _ := f.vistos(); !slices.Equal(pedidos, []string{"claude:instalar"}) {
		t.Fatalf("sin confirmar no se repite: %v", pedidos)
	}
	if _, err = ejecutarAdmin(t, "s\n", "agentes", "instalar", "claude"); err != nil {
		t.Fatalf("confirmando: %v", err)
	}
	for _, orden := range [][]string{{"agentes", "actualizar", "codex", "--confirmar"}, {"agents", "update", "codex", "--yes"}, {"agents", "update", "codex", "--confirm"}} {
		if _, err = ejecutarAdmin(t, "", orden...); err != nil {
			t.Fatalf("%v: %v", orden, err)
		}
	}
	pedidos, _, _ := f.vistos()
	if !slices.Equal(pedidos[1:], []string{"claude:instalar", "claude:instalar+confirmar", "codex:actualizar", "codex:actualizar+confirmar", "codex:actualizar", "codex:actualizar+confirmar", "codex:actualizar", "codex:actualizar+confirmar"}) {
		t.Fatalf("se repite con confirmar: %v", pedidos)
	}
}

// TestAgentesDesinstalar: pregunta antes (--confirmar no) y --borrar-datos (--delete-data) lo pide al servicio.
func TestAgentesDesinstalar(t *testing.T) {
	f := &preparacionCLI{guion: siempre(est("claude", "desinstalar", "listo"))}
	servicioConPreparacion(t, f)
	if _, err := ejecutarAdmin(t, "n\n", "agentes", "desinstalar", "claude"); salida(err) != 2 {
		t.Fatalf("sin confirmar: %v", err)
	}
	if pedidos, _, _ := f.vistos(); len(pedidos) != 0 {
		t.Fatalf("sin confirmar no se pide nada: %v", pedidos)
	}
	salidaTexto, err := ejecutarAdmin(t, "s\n", "agents", "uninstall", "claude", "--delete-data")
	if err != nil || !strings.Contains(salidaTexto, "Claude está desinstalado") {
		t.Fatalf("confirmado: %v\n%s", err, salidaTexto)
	}
	if _, err = ejecutarAdmin(t, "", "agentes", "desinstalar", "claude", "--borrar-datos", "--confirmar"); err != nil {
		t.Fatal(err)
	}
	if pedidos, _, _ := f.vistos(); !slices.Equal(pedidos, []string{"claude:desinstalar+borrar", "claude:desinstalar+borrar"}) {
		t.Fatalf("pedidos: %v", pedidos)
	}
}

// TestAgentesEntrarPegaCodigo: con «pide_pegar» lee el código de la entrada y lo envía; nunca lo imprime; imprime el
// enlace (solo https) y el aviso de Codex.
func TestAgentesEntrarPegaCodigo(t *testing.T) {
	f := &preparacionCLI{}
	espera := est("claude", "iniciar_sesion", "esperando_usuario")
	espera.Enlace, espera.PidePegar = "https://claude.example/oauth?x=1", true
	f.guion = siempre(est("claude", "iniciar_sesion", "comprobando"), espera)
	f.alPegar = []contrato.EstadoPreparacion{est("claude", "iniciar_sesion", "listo")}
	servicioConPreparacion(t, f)

	salidaTexto, err := ejecutarAdmin(t, "SECRETO-123\n", "agentes", "entrar", "claude")
	if err != nil || strings.Contains(salidaTexto, "SECRETO-123") || !strings.Contains(salidaTexto, "https://claude.example/oauth?x=1") ||
		!strings.Contains(salidaTexto, "Sesión iniciada en Claude") {
		t.Fatalf("entrar: %v\n%s", err, salidaTexto)
	}
	if _, pegados, _ := f.vistos(); !slices.Equal(pegados, []string{"SECRETO-123"}) {
		t.Fatalf("lo pegado va al servicio: %v", pegados)
	}

	// Codex: código de dispositivo, con su aviso; un enlace que no es https no se enseña.
	codex := est("codex", "iniciar_sesion", "esperando_usuario")
	codex.Enlace, codex.Codigo = "http://malo.example/", "ABCD-1234"
	f.guion = siempre(codex, est("codex", "iniciar_sesion", "listo"))
	salidaTexto, err = ejecutarAdmin(t, "", "agents", "login", "codex")
	if err != nil || !strings.Contains(salidaTexto, "https://chatgpt.com/#settings/Security") || !strings.Contains(salidaTexto, "ABCD-1234") || strings.Contains(salidaTexto, "malo.example") {
		t.Fatalf("codex: %v\n%s", err, salidaTexto)
	}
}

// TestAgentesEntrarSinSesion: un «listo» con motivo no es listo para usar: lo dice y sale con 1; una versión sin comprobar
// nunca se confirma: avisa, da la orden de la consola y sale con 2.
func TestAgentesEntrarSinSesion(t *testing.T) {
	f := &preparacionCLI{}
	sin := est("claude", "iniciar_sesion", "listo")
	sin.MotivoCodigo, sin.Mensaje = "sin_sesion", "la sesión no responde"
	f.guion = siempre(sin)
	servicioConPreparacion(t, f)
	salidaTexto, err := ejecutarAdmin(t, "", "agentes", "entrar", "claude")
	if salida(err) != 1 || !strings.Contains(salidaTexto, "no quedó listo para usar") || !strings.Contains(salidaTexto, "la sesión no responde") {
		t.Fatalf("listo con motivo: %v\n%s", err, salidaTexto)
	}
	f.guion = func(_ string, p contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		e := est("claude", p.Accion, "error")
		e.MotivoCodigo, e.Mensaje = "version_no_comprobada", "claude auth login"
		return []contrato.EstadoPreparacion{e}, nil
	}
	salidaTexto, err = ejecutarAdmin(t, "s\n", "agentes", "entrar", "claude")
	if err == nil || salida(err) != 2 || !strings.Contains(salidaTexto, "claude auth login") || !strings.Contains(salidaTexto, "aún no está comprobada") {
		t.Fatalf("versión sin comprobar: %v\n%s", err, salidaTexto)
	}
	if pedidos, _, _ := f.vistos(); len(pedidos) != 2 {
		t.Fatalf("no se repite con confirmar: %v", pedidos)
	}
}

// TestAgentesCancelar: Ctrl+C (el contexto de la orden) cancela la preparación en el servicio y sale con 130; una que ya
// estaba en marcha (ya_en_curso) se sigue.
func TestAgentesCancelar(t *testing.T) {
	f := &preparacionCLI{guion: siempre(est("claude", "instalar", "instalando"))}
	servicioConPreparacion(t, f)
	ctx, parar := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer parar()
	raiz := nuevaRaiz()
	var o bytes.Buffer
	raiz.SetOut(&o)
	raiz.SetErr(&o)
	raiz.SetIn(strings.NewReader(""))
	raiz.SetArgs([]string{"agentes", "instalar", "claude"})
	err := raiz.ExecuteContext(ctx)
	if salida(err) != 130 {
		t.Fatalf("cancelada: %v\n%s", err, o.String())
	}
	if _, _, cancelados := f.vistos(); !slices.Equal(cancelados, []string{"claude"}) {
		t.Fatalf("se cancela en el servicio: %v", cancelados)
	}

	g := &preparacionCLI{}
	g.guion = func(string, contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		return nil, &preparar.Error{Codigo: preparar.CodigoYaEnCurso, Mensaje: "ya en curso"}
	}
	g.cola = map[string][]contrato.EstadoPreparacion{"claude": {est("claude", "instalar", "instalando"), est("claude", "instalar", "listo")}}
	servicioConPreparacion(t, g)
	if salidaTexto, err := ejecutarAdmin(t, "", "agentes", "instalar", "claude"); err != nil || !strings.Contains(salidaTexto, "ya hay una") {
		t.Fatalf("ya en curso: %v\n%s", err, salidaTexto)
	}
}

// TestAgentesActualizarTodos: --todos (--all) actualiza solo los instalados con versión nueva; sin nada que actualizar, lo
// dice; con ids y --todos a la vez, o sin ninguno, no se admite.
func TestAgentesActualizarTodos(t *testing.T) {
	f := &preparacionCLI{guion: siempre(est("x", "actualizar", "listo"))}
	f.versiones = []contrato.VersionesAgente{
		{Agente: "claude", Instalada: "2.9.0", Ultima: "2.10.0", Acciones: []string{"actualizar", "desinstalar"}, Comprobadas: map[string]bool{"actualizar": true}},
		{Agente: "codex", Instalada: "1.0.0", Ultima: "1.0.0", Acciones: []string{"actualizar", "desinstalar"}, Comprobadas: map[string]bool{"actualizar": true}},
		{Agente: "copilot", Ultima: "3.0.0", Acciones: []string{"instalar"}, Comprobadas: map[string]bool{"instalar": true}},
	}
	servicioConPreparacion(t, f)
	for _, orden := range [][]string{{"agentes", "actualizar", "--todos"}, {"agents", "update", "--all"}} {
		if _, err := ejecutarAdmin(t, "", orden...); err != nil {
			t.Fatalf("%v: %v", orden, err)
		}
	}
	if pedidos, _, _ := f.vistos(); !slices.Equal(pedidos, []string{"claude:actualizar", "claude:actualizar"}) {
		t.Fatalf("solo el que tiene versión nueva: %v", pedidos)
	}
	g := &preparacionCLI{guion: f.guion, versiones: f.versiones[1:2]}
	servicioConPreparacion(t, g)
	if salidaTexto, err := ejecutarAdmin(t, "", "agentes", "actualizar", "--todos"); err != nil || !strings.Contains(salidaTexto, "no hay nada que actualizar") {
		t.Fatalf("nada que actualizar: %v\n%s", err, salidaTexto)
	}
	for _, args := range [][]string{{"agentes", "actualizar"}, {"agentes", "actualizar", "claude", "--todos"}} {
		if _, err := ejecutarAdmin(t, "", args...); err == nil {
			t.Fatalf("%v: no se admite", args)
		}
	}
}

// TestAgentesAlias: los alias en inglés de las órdenes y de sus opciones.
func TestAgentesAlias(t *testing.T) {
	raiz := nuevaRaiz()
	for ingles, espanol := range map[string]string{"install": "instalar", "login": "entrar", "update": "actualizar", "uninstall": "desinstalar", "versions": "versiones"} {
		c, _, err := raiz.Find([]string{"agents", ingles})
		if err != nil || c.Name() != espanol {
			t.Errorf("agents %s: %v %v", ingles, c, err)
		}
	}
	for orden, banderas := range map[string][]string{"actualizar": {"todos", "all", "confirmar", "confirm", "yes"}, "desinstalar": {"borrar-datos", "delete-data", "confirmar", "confirm", "yes"},
		"instalar": {"confirmar", "confirm", "yes"}, "versiones": {"comprobar", "check"}} {
		c, _, _ := raiz.Find([]string{"agentes", orden})
		for _, b := range banderas {
			if c.Flags().Lookup(b) == nil {
				t.Errorf("%s --%s", orden, b)
			}
		}
	}
}

// TestAgentesVersiones: tabla con la instalada, la última y si está comprobada.
func TestAgentesVersiones(t *testing.T) {
	f := &preparacionCLI{versiones: []contrato.VersionesAgente{
		{Agente: "claude", Instalada: "2.1.0", Ultima: "2.2.0", Comprobadas: map[string]bool{"actualizar": false}, Acciones: []string{"actualizar"}},
		{Agente: "codex", Instalada: "1.0.0", Ultima: "1.0.0", Comprobadas: map[string]bool{"actualizar": true}},
		{Agente: "copilot", Ultima: "3.0.0", Comprobadas: map[string]bool{"instalar": true}},
		{Agente: "antigravity", MotivoCodigo: "sin_red", Motivo: "sin conexión con el sitio del CLI"},
	}}
	servicioConPreparacion(t, f)
	for _, orden := range [][]string{{"agentes", "versiones"}, {"agents", "versions"}} {
		salidaTexto, err := ejecutarAdmin(t, "", orden...)
		if err != nil {
			t.Fatal(err)
		}
		lineas := strings.Split(strings.TrimSpace(salidaTexto[strings.Index(salidaTexto, "AGENTE"):]), "\n")
		campos := func(i int) []string { return strings.Fields(lineas[i]) }
		if len(lineas) < 5 || !slices.Equal(campos(0), []string{"AGENTE", "INSTALADA", "ÚLTIMA", "COMPROBADA"}) ||
			!slices.Equal(campos(1), []string{"claude", "2.1.0", "2.2.0", "no"}) || !slices.Equal(campos(2), []string{"codex", "1.0.0", "1.0.0", "sí"}) ||
			!slices.Equal(campos(3), []string{"copilot", "-", "3.0.0", "sí"}) || !strings.Contains(salidaTexto, "sin conexión con el sitio del CLI") {
			t.Fatalf("%v:\n%s", orden, salidaTexto)
		}
	}
}

// ejecutarCtx corre una orden con un contexto y una entrada que el test controla (el contexto hace de Ctrl+C).
func ejecutarCtx(t *testing.T, ctx context.Context, entrada io.Reader, args ...string) (string, error) {
	t.Helper()
	raiz := nuevaRaiz()
	var o bytes.Buffer
	raiz.SetOut(&o)
	raiz.SetErr(&o)
	raiz.SetIn(entrada)
	raiz.SetArgs(args)
	err := raiz.ExecuteContext(ctx)
	return o.String(), err
}

// entradaSinFin es una entrada que nunca responde (el usuario no escribe nada).
func entradaSinFin(t *testing.T) io.Reader {
	r, w := io.Pipe()
	t.Cleanup(func() { w.Close() })
	return r
}

// TestAgentesInstalarSinSesion: instalado, pero falta un paso (sin sesión o sin clave): sale con 3 y lo dice; no es 0.
func TestAgentesInstalarSinSesion(t *testing.T) {
	f := &preparacionCLI{}
	f.guion = func(agente string, p contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		e := est(agente, p.Accion, "listo")
		e.VersionDespues, e.MotivoCodigo, e.Mensaje = "1.0.0", "sin_sesion", "falta iniciar sesión"
		return []contrato.EstadoPreparacion{e}, nil
	}
	servicioConPreparacion(t, f)
	salidaTexto, err := ejecutarAdmin(t, "", "agentes", "instalar", "claude")
	if salida(err) != 3 || !strings.Contains(salidaTexto, "tramtai agentes entrar claude") || !strings.Contains(salidaTexto, "falta iniciar sesión") {
		t.Fatalf("claude: %v\n%s", err, salidaTexto)
	}
	if salidaTexto, err = ejecutarAdmin(t, "", "agentes", "instalar", "antigravity"); salida(err) != 3 || !strings.Contains(salidaTexto, "clave de API") {
		t.Fatalf("antigravity: %v\n%s", err, salidaTexto)
	}
	if _, err = ejecutarAdmin(t, "", "agentes", "actualizar", "claude"); salida(err) != 3 {
		t.Fatalf("actualizar sin sesión: %v", err)
	}
}

// TestAgentesCodigoFallido: si el envío del código falla, la orden lo dice y vuelve a pedirlo.
func TestAgentesCodigoFallido(t *testing.T) {
	f := &preparacionCLI{falloCodigo: 1}
	espera := est("claude", "iniciar_sesion", "esperando_usuario")
	espera.PidePegar = true
	f.guion = siempre(espera)
	f.alPegar = []contrato.EstadoPreparacion{est("claude", "iniciar_sesion", "listo")}
	servicioConPreparacion(t, f)
	salidaTexto, err := ejecutarAdmin(t, "uno\ndos\n", "agentes", "entrar", "claude")
	if err != nil || strings.Count(salidaTexto, "Pega aquí el código") != 2 {
		t.Fatalf("segundo intento: %v\n%s", err, salidaTexto)
	}
	if _, pegados, _ := f.vistos(); !slices.Equal(pegados, []string{"dos"}) {
		t.Fatalf("solo llegó el segundo: %v", pegados)
	}
}

// TestAgentesCtrlCEnPreguntas: Ctrl+C en una pregunta sale con 130 sin pedir ni cancelar nada en el servicio; Ctrl+C
// esperando el código sí cancela la preparación que esta orden lanzó.
func TestAgentesCtrlCEnPreguntas(t *testing.T) {
	f := &preparacionCLI{}
	f.guion = func(agente string, p contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		if p.Accion == "iniciar_sesion" {
			e := est(agente, p.Accion, "esperando_usuario")
			e.PidePegar = true
			return []contrato.EstadoPreparacion{e}, nil
		}
		e := est(agente, p.Accion, "error")
		e.MotivoCodigo = "version_no_comprobada"
		return []contrato.EstadoPreparacion{e}, nil
	}
	servicioConPreparacion(t, f)
	corto := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 150*time.Millisecond)
	}

	ctx, parar := corto()
	_, err := ejecutarCtx(t, ctx, entradaSinFin(t), "agentes", "desinstalar", "claude")
	parar()
	if pedidos, _, cancelados := f.vistos(); salida(err) != 130 || len(pedidos) != 0 || len(cancelados) != 0 {
		t.Fatalf("¿Desinstalar?: %v %v %v", err, pedidos, cancelados)
	}
	ctx, parar = corto()
	_, err = ejecutarCtx(t, ctx, entradaSinFin(t), "agentes", "instalar", "claude")
	parar()
	if pedidos, _, cancelados := f.vistos(); salida(err) != 130 || len(pedidos) != 1 || len(cancelados) != 0 {
		t.Fatalf("¿Seguir igualmente?: %v %v %v", err, pedidos, cancelados)
	}
	ctx, parar = corto()
	_, err = ejecutarCtx(t, ctx, entradaSinFin(t), "agentes", "entrar", "claude")
	parar()
	if _, _, cancelados := f.vistos(); salida(err) != 130 || !slices.Equal(cancelados, []string{"claude"}) {
		t.Fatalf("esperando el código: %v %v", err, cancelados)
	}
}

// TestAgentesNoPosible: peticiones en curso y no disponible en el servidor salen con 2; una acción ajena en marcha no se
// adopta (2, sin cancelar); un estado «cancelado» sale con 130.
func TestAgentesNoPosible(t *testing.T) {
	f := &preparacionCLI{}
	f.guion = func(string, contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		return nil, &preparar.Error{Codigo: "peticiones_en_curso", Mensaje: "hay peticiones"}
	}
	servicioConPreparacion(t, f)
	if _, err := ejecutarAdmin(t, "", "agentes", "instalar", "claude"); salida(err) != 2 {
		t.Fatalf("peticiones_en_curso: %v", err)
	}
	f.guion = func(string, contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		return nil, &preparar.Error{Codigo: "no_disponible_en_servidor", Mensaje: "no"}
	}
	if _, err := ejecutarAdmin(t, "", "agentes", "instalar", "claude"); salida(err) != 2 {
		t.Fatalf("no_disponible_en_servidor: %v", err)
	}
	f.guion = func(string, contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		return nil, &preparar.Error{Codigo: preparar.CodigoYaEnCurso, Mensaje: "ya"}
	}
	f.cola["claude"] = []contrato.EstadoPreparacion{est("claude", "actualizar", "actualizando")}
	salidaTexto, err := ejecutarAdmin(t, "", "agentes", "instalar", "claude")
	if _, _, cancelados := f.vistos(); salida(err) != 2 || len(cancelados) != 0 || !strings.Contains(salidaTexto, "otra preparación") {
		t.Fatalf("acción ajena: %v %v\n%s", err, cancelados, salidaTexto)
	}
	f.guion = siempre(est("claude", "instalar", "cancelado"))
	if _, err = ejecutarAdmin(t, "", "agentes", "instalar", "claude"); salida(err) != 130 {
		t.Fatalf("cancelado: %v", err)
	}
}

// TestAgentesSondeoFalla: tres consultas seguidas que fallan (el servicio se fue) acaban la orden con error, sin cancelar.
func TestAgentesSondeoFalla(t *testing.T) {
	f := &preparacionCLI{guion: siempre(est("claude", "instalar", "instalando"))}
	ts := servicioConPreparacion(t, f)
	var una sync.Once
	f.alConsultar = func() { una.Do(func() { go ts.Close() }) }
	_, err := ejecutarAdmin(t, "", "agentes", "instalar", "claude")
	if err == nil || salida(err) != 1 {
		t.Fatalf("sondeo: %v", err)
	}
	if _, _, cancelados := f.vistos(); len(cancelados) != 0 {
		t.Fatalf("no se cancela: %v", cancelados)
	}
}

// TestAgentesVersionesComprobar: --comprobar (--check) pide forzar la consulta (?forzar=1); sin ella no.
func TestAgentesVersionesComprobar(t *testing.T) {
	f := &preparacionCLI{versiones: []contrato.VersionesAgente{{Agente: "claude", Instalada: "1.0.0"}}}
	ts := servicioConPreparacion(t, f)
	var mu sync.Mutex
	var pedidas []string
	siguiente := ts.Config.Handler
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/agentes/versiones") {
			mu.Lock()
			pedidas = append(pedidas, r.URL.RawQuery)
			mu.Unlock()
		}
		siguiente.ServeHTTP(w, r)
	})
	if _, err := ejecutarAdmin(t, "", "agentes", "versiones"); err != nil {
		t.Fatal(err)
	}
	if _, err := ejecutarAdmin(t, "", "agents", "versions", "--check"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(pedidas, []string{"", "forzar=1"}) {
		t.Fatalf("consultas: %q", pedidas)
	}
}

// TestAgentesCtrlCAdoptada: Ctrl+C siguiendo una preparación que lanzó otra pantalla sale con 130, no la cancela y dice
// que sigue en marcha.
func TestAgentesCtrlCAdoptada(t *testing.T) {
	f := &preparacionCLI{}
	f.guion = func(string, contrato.PedidoPreparacion) ([]contrato.EstadoPreparacion, error) {
		return nil, &preparar.Error{Codigo: preparar.CodigoYaEnCurso, Mensaje: "ya"}
	}
	f.cola = map[string][]contrato.EstadoPreparacion{"claude": {est("claude", "instalar", "instalando")}}
	servicioConPreparacion(t, f)
	ctx, parar := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer parar()
	salidaTexto, err := ejecutarCtx(t, ctx, entradaSinFin(t), "agentes", "instalar", "claude")
	if _, _, cancelados := f.vistos(); salida(err) != 130 || len(cancelados) != 0 || !strings.Contains(salidaTexto, "sigue en marcha: la lanzó otra pantalla") {
		t.Fatalf("adoptada: %v %v\n%s", err, cancelados, salidaTexto)
	}
}
