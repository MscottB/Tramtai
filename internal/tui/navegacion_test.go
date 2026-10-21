package tui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/inicio"
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

// paginaOcultable cuenta los avisos de salir y volver, como la de Inicio con su flujo en vivo.
type paginaOcultable struct {
	paginaFalsa
	ocultada, mostrada int
}

func (p *paginaOcultable) Ocultar()         { p.ocultada++ }
func (p *paginaOcultable) Mostrar() tea.Cmd { p.mostrada++; return nil }

// TestOcultableSeAvisaAlSalirYAlVolver: una página con algo en vivo se entera de que deja de verse (por el menú, un
// atajo, IrA o ?) y de que vuelve sin iniciarse de nuevo (Volver). Si vuelve con Iniciar no hace falta Mostrar, y
// cambiar de espacio estando en ella no la oculta.
func TestOcultableSeAvisaAlSalirYAlVolver(t *testing.T) {
	m := nuevo(Opciones{})
	o := &paginaOcultable{}
	m.paginas["inicio"] = o
	m.iniciar(nil)
	m.Update(tecla("3"))
	if o.ocultada != 1 {
		t.Fatalf("un atajo la oculta: %d", o.ocultada)
	}
	m.Update(tecla("1"))
	if o.iniciada != 2 || o.mostrada != 0 {
		t.Fatalf("por un atajo se inicia de nuevo, sin Mostrar: %d %d", o.iniciada, o.mostrada)
	}
	m.Update(tecla("?"))
	if o.ocultada != 2 || m.actual != pagina.PaginaAyuda {
		t.Fatalf("? la oculta: %d", o.ocultada)
	}
	m.Update(pagina.Volver{})
	if m.actual != pagina.PaginaInicio || o.mostrada != 1 || o.iniciada != 2 {
		t.Fatalf("Volver la muestra sin iniciarla: %s %d %d", m.actual, o.mostrada, o.iniciada)
	}
	m.Update(pagina.CambioEspacio{ID: "notas_1"})
	if o.ocultada != 2 || o.iniciada != 3 {
		t.Fatalf("cambiar de espacio no la oculta, la inicia: %d %d", o.ocultada, o.iniciada)
	}
	m.Update(pagina.IrA{Pagina: pagina.PaginaEjecutar})
	m.Update(pagina.CambioEspacio{ID: "global"})
	m.Update(pagina.Volver{}) // su Ctx se canceló al cambiar de espacio: Volver la inicia
	if o.mostrada != 1 || o.iniciada != 4 {
		t.Fatalf("si su contexto se canceló, Volver la inicia y no la muestra: %d %d", o.mostrada, o.iniciada)
	}
}

// TestAtajosDeInicioComoElMenu: el número que Inicio escribe junto a cada paso es el de la tecla que lleva a su página.
func TestAtajosDeInicioComoElMenu(t *testing.T) {
	for n := 1; n <= paginasConAtajo; n++ {
		clave, _ := atajo(strconv.Itoa(n))
		if got := inicio.Atajo(clave); got != strconv.Itoa(n) {
			t.Errorf("%s: Inicio dice %q y la tecla es %d", clave, got, n)
		}
	}
}

func TestInicioRegistrada(t *testing.T) {
	m := nuevo(Opciones{})
	if _, pendiente := m.pagina(pagina.PaginaInicio).(*proximamente); pendiente {
		t.Fatal("Inicio ya no es «Próximamente»")
	}
}

func TestAgentesRegistrada(t *testing.T) {
	m := nuevo(Opciones{})
	if _, pendiente := m.pagina(pagina.PaginaAgentes).(*proximamente); pendiente {
		t.Fatal("Agentes ya no es «Próximamente»")
	}
}

// TestConfirmacionCabeEn60x20: una pregunta larga no se sale del área ni esconde el pie «s confirma · Esc cancela».
func TestConfirmacionCabeEn60x20(t *testing.T) {
	m := nuevo(Opciones{})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	largo := strings.TrimSpace(strings.Repeat("Una frase de la pregunta que ocupa su sitio en la caja y la hace crecer. ", 24))
	m.pregunta = &comp.Confirmacion{Texto: largo}
	v := m.vista()
	if !strings.Contains(v, "s confirma · Esc cancela") {
		t.Fatalf("el pie de la confirmación se ve a 60×20:\n%s", v)
	}
	if lipgloss.Height(v) != 20 || lipgloss.Width(v) > 60 {
		t.Fatalf("la vista mide %dx%d:\n%s", lipgloss.Width(v), lipgloss.Height(v), v)
	}
}
