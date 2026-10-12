package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/nucleo"
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
