package tui

import (
	"context"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
)

func TestSolicitudDesde(t *testing.T) {
	casos := map[string]nucleo.SolicitudEjecucion{
		"(predeterminado de la instancia)": {},
		"perfil/rapido":                    {PerfilModelo: "rapido"},
		"claude/haiku":                     {Agente: "claude", Modelo: "haiku"},
		"codex":                            {Agente: "codex"},
	}
	for seleccion, esperado := range casos {
		s := solicitudDesde(seleccion, "hola")
		if s.Agente != esperado.Agente || s.Modelo != esperado.Modelo || s.PerfilModelo != esperado.PerfilModelo || s.Mensajes[0].Contenido != "hola" {
			t.Errorf("%s: %+v", seleccion, s)
		}
	}
}

func tecla(texto string) tea.KeyPressMsg {
	switch texto {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	}
	return tea.KeyPressMsg{Code: []rune(texto)[0], Text: texto}
}

func TestNavegacionSinServicio(t *testing.T) {
	m := nuevo(Opciones{Proyecto: "C:/repos/demo", IDProyecto: "demo_1234"})
	vista := m.View().Content
	for _, texto := range []string{"Instancia del proyecto actual", "Instancia global", "Servicio:", "inactivo"} {
		if !strings.Contains(vista, texto) {
			t.Fatalf("falta %q en el inicio:\n%s", texto, vista)
		}
	}
	m.Update(tecla("2")) // instancia global
	if m.pantalla != pInstancia || m.inst != "global" {
		t.Fatalf("debía abrir la instancia global: %v %s", m.pantalla, m.inst)
	}
	if vista := m.View().Content; !strings.Contains(vista, "Activar instancia (inicia el servicio") {
		t.Fatalf("sin servicio se ofrece activarla:\n%s", vista)
	}
	m.Update(tecla("esc"))
	if m.pantalla != pInicio {
		t.Fatalf("Esc vuelve al inicio: %v", m.pantalla)
	}
}

func TestInstanciaActivaMuestraPanel(t *testing.T) {
	m := nuevo(Opciones{})
	m.estado = &servicio.Estado{Instancias: []servicio.EstadoInstancia{{ID: "global", Ambito: "global", Trabajos: map[string]int{"completado": 2}}},
		Puntos: []servicio.EstadoPunto{{Nombre: "app", Instancia: "global", URL: "http://127.0.0.1:17373/p/app", Contratos: []string{"openai"}}}}
	m.inst = "global"
	m.ir(pInstancia)
	vista := m.View().Content
	if !strings.Contains(vista, "completado 2") || !strings.Contains(vista, "Puntos de acceso") {
		t.Fatalf("panel de instancia:\n%s", vista)
	}
	m.ir(pAccesos)
	if vista := m.View().Content; !strings.Contains(vista, "/p/app") || !strings.Contains(vista, "activo") {
		t.Fatalf("puntos de acceso:\n%s", vista)
	}
	m.ir(pEjecutar)
	m.campo = 2
	m.Update(tecla("h"))
	m.Update(tecla("i"))
	if m.mensaje != "hi" {
		t.Fatalf("el campo de mensaje recibe el texto: %q", m.mensaje)
	}
}

// TestAccesosPorElServicio crea, borra y vuelve a crear una API desde la terminal contra un servicio en memoria: los
// cambios pasan por su API, que los guarda en la configuración global.
func TestAccesosPorElServicio(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	datos := filepath.Join(dir, "datos")
	c, _ := nucleo.CargarConfiguracion("")
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	defer ts.Close()
	defer s.Cerrar()
	s.ActivarGlobal()
	s.EscribirDescriptor()
	cli, err := cliente.Conectar(datos)
	if err != nil {
		t.Fatal(err)
	}

	m := nuevo(Opciones{})
	m.cli, m.inst = cli, "global"
	crear := func() string {
		t.Helper()
		m.nombrePunto, m.puertoPunto = "tuiapi", ""
		a := m.crearPunto()().(msgAviso)
		if a.err != nil || !strings.Contains(a.texto, "tuiapi") || !strings.Contains(a.texto, "tt_") {
			t.Fatalf("crear: %+v", a)
		}
		return a.texto[strings.Index(a.texto, "tt_"):]
	}
	primera := crear()
	if guardado, _ := os.ReadFile(global); !strings.Contains(string(guardado), "tuiapi") {
		t.Fatalf("se guarda en la configuración global:\n%s", guardado)
	}
	e, _ := cli.Estado(context.Background())
	m.estado = &e
	m.ir(pAccesos)
	m.cursor = slices.IndexFunc(m.puntosDeInstancia(), func(p contrato.EstadoPunto) bool { return p.Nombre == "tuiapi" })
	if m.teclaAccesos("x"); m.pregunta == "" {
		t.Fatal("eliminar pide confirmación")
	}
	if a := m.siConfirma().(msgAviso); a.err != nil {
		t.Fatalf("eliminar: %v", a.err)
	}
	if guardado, _ := os.ReadFile(global); strings.Contains(string(guardado), "tuiapi") {
		t.Fatalf("eliminar la quita:\n%s", guardado)
	}
	if crear() == primera { // con el mismo nombre: es una API nueva, con clave nueva
		t.Fatal("la API vuelta a crear debía tener otra clave")
	}
}

func TestAvisoConversorAlAdjuntar(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	datos := filepath.Join(dir, "datos")
	c, _ := nucleo.CargarConfiguracion("")
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"},
		RaizUsuario: dir, UV: filepath.Join(dir, "no-existe-uv")})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	defer ts.Close()
	defer s.Cerrar()
	s.ActivarGlobal()
	s.EscribirDescriptor()
	cli, err := cliente.Conectar(datos)
	if err != nil {
		t.Fatal(err)
	}
	pdf := filepath.Join(dir, "informe.pdf")
	os.WriteFile(pdf, []byte("%PDF"), 0o600)
	m := nuevo(Opciones{})
	m.cli, m.inst = cli, "global"
	m.archivos, m.mensaje = pdf, "resume"
	creado, _ := m.lanzar()().(msgCreado)
	if creado.err == nil || !strings.Contains(creado.err.Error(), "ctrl+p") {
		t.Fatalf("sin conversor, aviso con la tecla para prepararlo: %v", creado.err)
	}
	if trabajos, _ := cli.Trabajos(context.Background(), "global", 10); len(trabajos) != 0 {
		t.Fatalf("no se crea el trabajo: %d", len(trabajos))
	}
	cmd := m.teclaEjecutar(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+p prepara el conversor")
	}
	if a, ok := cmd().(msgAviso); !ok || a.err != nil {
		t.Fatalf("preparar: %+v", a)
	}
	if e, _ := cli.Conversor(context.Background(), "global"); e.Estado == "falta" {
		t.Fatalf("la preparación se lanzó: %+v", e)
	}
}
