package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/inicio"
	"tramtai/internal/tui/paginas/prueba"
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
	m.cli, m.preguntada = &cliente.Cliente{}, true // el cliente no lleva a ningún servicio: no se pregunta por las novedades
	cmd := m.conectado(msgConexion{cli: m.cli, espacios: []contrato.Espacio{{ID: "global", Nombre: "Global", Activo: true}, {ID: "notas_1", Nombre: "Notas", Activo: false}}})
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

// TestTodasRegistradas: las ocho páginas del menú, la Ayuda incluida, tienen su fábrica.
func TestTodasRegistradas(t *testing.T) {
	for _, clave := range ordenMenu() {
		if fabricas[clave] == nil {
			t.Errorf("%s no está registrada en fabricas", clave)
		}
	}
}

// TestInterrogacionAbreSuTema: en cada una de las ocho páginas, ? abre la Ayuda en el tema de esa página (el de su campo
// pagina: Probar es el tema probar). En la propia Ayuda, ? lleva a su inicio, con todos los temas.
func TestInterrogacionAbreSuTema(t *testing.T) {
	for _, clave := range ordenMenu() {
		m := nuevoEn(Opciones{Datos: t.TempDir()}, "es")
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		m.ir(clave, nil, false)
		m.Update(tecla("?"))
		if m.actual != pagina.PaginaAyuda {
			t.Fatalf("%s: ? abre la Ayuda: %s", clave, m.actual)
		}
		v := ansi.Strip(m.pagina(pagina.PaginaAyuda).Vista())
		primera := strings.TrimSpace(strings.Split(v, "\n")[0])
		if clave == pagina.PaginaAyuda {
			if !strings.Contains(v, "Empezar") || !strings.Contains(v, "Preguntas frecuentes") {
				t.Errorf("en la Ayuda, ? lleva a su inicio:\n%s", v)
			}
			continue
		}
		var tema datos.Tema
		for _, x := range datos.Ayuda("es") {
			if x.Pagina == clave {
				tema = x
			}
		}
		if tema.ID == "" || primera != tema.Titulo || !strings.Contains(v, "En la terminal:") {
			t.Errorf("%s: ? abre el tema %q: %q\n%s", clave, tema.Titulo, primera, v)
		}
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

// TestCambioEspacioRecargaPaginas: abrir otro espacio desde Espacios cambia el de la terminal: la cabecera lo nombra
// enseguida, se va a Inicio y Espacios, al volver, dice que estás ahí.
func TestCambioEspacioRecargaPaginas(t *testing.T) {
	cli, _ := prueba.Servicio(t)
	dir := filepath.Join(t.TempDir(), "notas")
	os.MkdirAll(dir, 0o700)
	e, err := cli.AbrirProyecto(context.Background(), dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	m := nuevo(Opciones{})
	m.cli, m.instancia = cli, "global"
	a := conducir(t, m)
	a.Entregar(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.Pulsar("6")
	a.HastaVer(e.Nombre)
	a.Pulsar("down", "enter")
	a.Hasta(func() bool { return m.instancia == e.ID && m.actual == pagina.PaginaInicio })
	a.Hasta(func() bool {
		return strings.Contains(ansi.Strip(a.Vista()), e.Nombre+" · ● "+datos.Texto("es", "app.enMarcha"))
	})
	a.Pulsar("6")
	a.Hasta(func() bool {
		for _, l := range strings.Split(ansi.Strip(a.Vista()), "\n") {
			if strings.Contains(l, e.Nombre) && strings.Contains(l, datos.Texto("es", "espacios.actual")) {
				return true
			}
		}
		return false
	})
}

// TestComprobacionViejaNoDeshaceElCambio: con un latido en marcha al cambiar a un espacio recién abierto, su resultado (que
// leyó los espacios antes) no lo da por detenido ni vuelve al global, y la app vuelve a comprobar: la cabecera lo nombra.
func TestComprobacionViejaNoDeshaceElCambio(t *testing.T) {
	cli, _ := prueba.Servicio(t)
	dir := filepath.Join(t.TempDir(), "notas")
	os.MkdirAll(dir, 0o700)
	e, err := cli.AbrirProyecto(context.Background(), dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	m := nuevo(Opciones{})
	m.paginas["inicio"] = &paginaFalsa{}
	m.cli, m.instancia, m.comprobando = cli, "global", true // el latido está en camino
	vieja := msgConexion{cli: cli, espacios: []contrato.Espacio{{ID: "global", Nombre: "Global", Activo: true}}, cambios: m.cambios}
	m.Update(pagina.CambioEspacio{ID: e.ID})
	a := conducir(t, m)
	a.Correr(m.conectado(vieja))
	a.Hasta(func() bool { return m.instancia == e.ID && m.nombreEspacio() == e.Nombre }) // la comprobación nueva la contesta el servicio
	if m.instancia != e.ID || m.nombreEspacio() != e.Nombre {
		t.Fatalf("sigue en el espacio nuevo y lo nombra: %q %q", m.instancia, m.nombreEspacio())
	}
}

// TestEspaciosCambiadosReleeLaLista: tras un cambio en la página Espacios, la app relee los espacios al momento (como
// api.sesion() en la Web UI): Ctrl+E ya no ofrece el que se detuvo.
func TestEspaciosCambiadosReleeLaLista(t *testing.T) {
	cli, _ := prueba.Servicio(t)
	dir := filepath.Join(t.TempDir(), "notas")
	os.MkdirAll(dir, 0o700)
	e, err := cli.AbrirProyecto(context.Background(), dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	m := nuevo(Opciones{})
	m.cli, m.instancia = cli, "global"
	m.espacios, _ = cli.Espacios(context.Background())
	if err := cli.DetenerEspacio(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	_, cmd := m.Update(pagina.EspaciosCambiados{})
	if cmd == nil || !m.comprobando {
		t.Fatal("relee los espacios")
	}
	if _, otro := m.Update(pagina.EspaciosCambiados{}); otro != nil {
		t.Fatal("con una comprobación en marcha, no lanza otra")
	}
	a := conducir(t, m)
	a.Correr(cmd)
	a.Hasta(func() bool { _, activo := m.espacioActivo(e.ID); return !activo && !m.comprobando }) // la lista la trae el servicio
	if _, activo := m.espacioActivo(e.ID); activo || m.comprobando {
		t.Fatalf("la lista ya sabe que se detuvo: %+v", m.espacios)
	}
}
