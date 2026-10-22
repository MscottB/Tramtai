package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

// paginaBorra pregunta antes de borrar con x y apunta si se confirmó.
type paginaBorra struct {
	paginaFalsa
	borrada bool
}

func (p *paginaBorra) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "x" {
			return p, comp.Confirmar("¿Borrar mi-app?")
		}
	case comp.Confirmado:
		p.borrada = true
	}
	return p, nil
}

func TestBorrarSoloConS(t *testing.T) {
	m := nuevo(Opciones{})
	b := &paginaBorra{}
	m.paginas["inicio"] = b
	_, cmd := m.Update(tecla("x"))
	correr(t, m, cmd)
	if m.pregunta == nil {
		t.Fatal("x pregunta")
	}
	_, cmd = m.Update(tecla("enter"))
	correr(t, m, cmd)
	if b.borrada {
		t.Fatal("x seguido de Intro no borra")
	}
	_, cmd = m.Update(tecla("s"))
	correr(t, m, cmd)
	if !b.borrada || m.pregunta != nil {
		t.Fatal("s confirma")
	}
	b.borrada = false
	m.Update(tea.WindowSizeMsg{Width: 59, Height: 20})
	for _, k := range []string{"x", "3", "tab", "?"} {
		if _, cmd := m.Update(tecla(k)); cmd != nil || m.pregunta != nil || m.actual != "inicio" || m.enMenu {
			t.Fatalf("bajo el mínimo, %s no llega a la página ni a la app", k)
		}
	}
}

func TestDobleRUnSoloArranque(t *testing.T) {
	arranques := 0
	m := nuevo(Opciones{Iniciar: func() (*cliente.Cliente, error) {
		arranques++
		return nil, errors.New("no arranca")
	}})
	m.caido = cliente.ErrServicioInactivo
	_, primero := m.Update(tecla("r"))
	if _, segundo := m.Update(tecla("r")); segundo != nil {
		t.Fatal("mientras se reintenta, r se ignora")
	}
	correr(t, m, primero)
	if arranques != 1 {
		t.Fatalf("un solo arranque: %d", arranques)
	}
	if _, otra := m.Update(tecla("r")); otra == nil {
		t.Fatal("con el resultado del reintento, r vuelve a funcionar")
	}
}

func TestLatidoSinComprobacionesALaVez(t *testing.T) {
	m := nuevo(Opciones{})
	m.caido = cliente.ErrServicioInactivo
	m.Update(msgLatido{})
	if !m.comprobando {
		t.Fatal("el latido comprueba")
	}
	if _, cmd := m.Update(tecla("r")); cmd != nil {
		t.Fatal("con una comprobación en marcha, r se ignora")
	}
	if cmd := m.conectado(msgConexion{err: cliente.ErrServicioInactivo, latido: true}); cmd == nil {
		t.Fatal("el resultado del latido programa el siguiente")
	}
	if m.comprobando {
		t.Fatal("con el resultado, ya no hay comprobación en marcha")
	}
	if cmd := m.conectado(msgConexion{err: cliente.ErrServicioInactivo}); cmd != nil {
		t.Fatal("el resultado de r no programa otro latido: habría dos cadenas")
	}
}

func TestIniciarCancelaElAnterior(t *testing.T) {
	m := nuevo(Opciones{})
	falsa := &paginaFalsa{}
	m.paginas["inicio"] = falsa
	m.iniciar(nil)
	primero := falsa.ctx
	m.iniciar(nil)
	if primero.Err() == nil || falsa.ctx.Err() != nil {
		t.Fatal("volver a iniciar cancela el contexto anterior y da uno vivo")
	}
	m.cerrar()
	if falsa.ctx.Err() == nil {
		t.Fatal("al salir se cancela todo")
	}
}

func TestCambioEspacioLlegaAOtraPagina(t *testing.T) {
	m := nuevo(Opciones{})
	otra := &paginaFalsa{}
	m.paginas["historial"] = otra
	m.Update(tecla("5"))
	viejo := otra.ctx
	m.Update(tecla("1"))
	m.Update(pagina.CambioEspacio{ID: "notas_1"})
	if viejo.Err() == nil || otra.cambios != 1 {
		t.Fatalf("el cambio cancela lo de las otras páginas y les llega: %v %d", viejo.Err(), otra.cambios)
	}
	m.Update(tecla("5"))
	if otra.instancia != "notas_1" || otra.ctx.Err() != nil {
		t.Fatalf("al volver, la página se inicia con el espacio nuevo: %q", otra.instancia)
	}
}

func TestEspacioDetenidoVuelveAlGlobal(t *testing.T) {
	m := nuevo(Opciones{})
	falsa := &paginaFalsa{}
	m.paginas["inicio"] = falsa
	m.instancia = "notas_1"
	cmd := m.conectado(msgConexion{cli: &cliente.Cliente{}, espacios: []contrato.Espacio{{ID: "global", Nombre: "Global", Activo: true}, {ID: "notas_1", Nombre: "Notas", Activo: false}}})
	correr(t, m, cmd)
	if m.instancia != "global" || falsa.instancia != "global" {
		t.Fatalf("vuelve al global: %q %q", m.instancia, falsa.instancia)
	}
	if aviso := idioma.Tf("el espacio %s se detuvo: vuelves al global", "Notas"); !strings.Contains(m.View().Content, aviso) {
		t.Fatalf("avisa:\n%s", m.View().Content)
	}
}

func TestVolver(t *testing.T) {
	m := nuevo(Opciones{})
	m.Update(tecla("3"))
	m.Update(tecla("?"))
	if m.actual != pagina.PaginaAyuda {
		t.Fatalf("? abre la Ayuda: %s", m.actual)
	}
	_, cmd := m.Update(tecla("esc"))
	correr(t, m, cmd)
	if m.actual != pagina.PaginaEjecutar {
		t.Fatalf("Esc vuelve de la Ayuda a la página de antes: %s", m.actual)
	}
	m.Update(pagina.IrA{Pagina: pagina.PaginaHistorial, Datos: pagina.FiltroPunto{Nombre: "mi-app"}})
	m.Update(pagina.Volver{})
	if m.actual != pagina.PaginaEjecutar {
		t.Fatalf("Volver tras un IrA lleva a la de origen: %s", m.actual)
	}
	m.Update(pagina.Volver{})
	m.Update(pagina.IrA{Pagina: "nada"})
	if m.actual != pagina.PaginaEjecutar {
		t.Fatalf("sin historia, Volver no hace nada; una clave que no existe tampoco: %s", m.actual)
	}
	m.Update(pagina.IrA{Pagina: pagina.PaginaHistorial})
	m.Update(tecla("1"))
	m.Update(pagina.Volver{})
	if m.actual != pagina.PaginaInicio {
		t.Fatalf("ir por el menú empieza una historia nueva: %s", m.actual)
	}
}
