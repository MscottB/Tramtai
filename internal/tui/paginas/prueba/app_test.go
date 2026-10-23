package prueba

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

func TestTeclaConModificadores(t *testing.T) {
	for _, nombre := range []string{"enter", "ctrl+s", "shift+tab", "alt+up", "ctrl+end", "space", "a", "?"} {
		if got := Tecla(nombre).String(); got != nombre {
			t.Errorf("Tecla(%q) es %q", nombre, got)
		}
	}
}

// contador es una página mínima: «a» pide sumar uno (un Cmd que tarda; «l», uno que tarda mucho), «b» manda un aviso y «s» espera a que la prueba
// suelte su canal. Cada respuesta lleva la versión con que se pidió y una vieja no cuenta.
type contador struct {
	n, gen int
	soltar chan struct{}
	parar  atomic.Bool // corta el Cmd de «w», que trabaja sin parar
}

type sumado struct{ gen int }

func (c *contador) Titulo() string                  { return "contador" }
func (c *contador) Iniciar(pagina.Contexto) tea.Cmd { return nil }
func (c *contador) Vista() string                   { return string(rune('0' + c.n)) }
func (c *contador) Teclas() []pagina.Tecla          { return nil }
func (c *contador) EnCampo() bool                   { return false }
func (c *contador) Ayuda() string                   { return "" }
func (c *contador) Update(m tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := m.(type) {
	case sumado:
		if m.gen == c.gen {
			c.n++
		}
	case tea.KeyPressMsg:
		gen := c.gen
		switch m.String() {
		case "a":
			return c, func() tea.Msg { time.Sleep(5 * time.Millisecond); return sumado{gen} }
		case "l":
			// lo que tarda un Cmd con la máquina cargada: más que cualquier espera fija del arnés
			return c, func() tea.Msg { time.Sleep(100 * time.Millisecond); return sumado{gen} }
		case "b":
			return c, tea.Batch(comp.Aviso("hola", comp.Info), nil)
		case "s":
			return c, func() tea.Msg { <-c.soltar; return sumado{gen} }
		case "w":
			return c, func() tea.Msg {
				for !c.parar.Load() {
				}
				return nil
			}
		case "g":
			c.gen++
		}
	}
	return c, nil
}

func TestAppComoLaTerminal(t *testing.T) {
	c := &contador{soltar: make(chan struct{})}
	a := Abrir(t, c, pagina.Contexto{Ctx: context.Background()})
	a.Pulsar("a", "a")
	if a.Vista() != "2" {
		t.Fatalf("Pulsar espera a lo que pide cada tecla: %s", a.Vista())
	}
	if fuera := a.Pulsar("b"); len(fuera) != 1 {
		t.Fatalf("un aviso es de la app: %v", fuera)
	}
	// Lo que espera a algo de fuera no frena: la página se asienta y llega cuando se suelta.
	a.Pulsar("s")
	if a.Vista() != "2" {
		t.Fatalf("aún no: %s", a.Vista())
	}
	close(c.soltar)
	a.HastaVer("3")
	// Una respuesta retenida que llega con otra versión no cuenta.
	viejos := a.Retener(func() { a.Pulsar("a") })
	if len(viejos) != 1 || a.Vista() != "3" {
		t.Fatalf("Retener guarda el Cmd sin correrlo: %d %s", len(viejos), a.Vista())
	}
	a.Pulsar("g")
	a.Soltar(viejos)
	if a.Vista() != "3" {
		t.Fatalf("la respuesta vieja se descarta: %s", a.Vista())
	}
}

// apunta es una página que apunta, en orden, los pasos que le llegan.
type (
	paso   int
	apunta struct {
		contador
		pasos []paso
	}
)

func (p *apunta) Update(m tea.Msg) (pagina.Pagina, tea.Cmd) {
	if x, ok := m.(paso); ok {
		p.pasos = append(p.pasos, x)
	}
	return p, nil
}

// TestSecuenciaEnOrden: lo de un tea.Sequence llega en su orden, aunque el primero tarde más; también al recogerlo.
func TestSecuenciaEnOrden(t *testing.T) {
	p := &apunta{}
	a := Conducir(t, p)
	secuencia := func() tea.Cmd {
		return tea.Sequence(func() tea.Msg { time.Sleep(20 * time.Millisecond); return paso(1) }, func() tea.Msg { return paso(2) })
	}
	a.Correr(secuencia())
	if len(p.pasos) != 2 || p.pasos[0] != 1 || p.pasos[1] != 2 {
		t.Fatalf("en orden: %v", p.pasos)
	}
	if r := a.Recoger([]tea.Cmd{secuencia()}); len(r) != 2 || r[0] != paso(1) || r[1] != paso(2) {
		t.Fatalf("Recoger también, en orden: %v", r)
	}
}

// Asentar espera a un Cmd que trabaja, tarde lo que tarde: solo deja de esperar a uno parado en algo de fuera.
func TestAsentarEsperaAlQueTarda(t *testing.T) {
	a := Abrir(t, &contador{}, pagina.Contexto{Ctx: context.Background()})
	if a.Pulsar("l"); a.Vista() != "1" {
		t.Fatalf("Pulsar volvió antes de la respuesta: %s", a.Vista())
	}
}

// cargador enseña «Cargando…» hasta que llega su primera respuesta, que espera a un temporizador que el arnés no ve: lo
// que pasa con un temporizador del servicio o una espera de E/S que la foto de las goroutines da por quieta.
type cargador struct{ listo bool }

type cargado struct{}

func (c *cargador) Titulo() string         { return "cargador" }
func (c *cargador) Teclas() []pagina.Tecla { return nil }
func (c *cargador) EnCampo() bool          { return false }
func (c *cargador) Ayuda() string          { return "" }
func (c *cargador) Iniciar(pagina.Contexto) tea.Cmd {
	return func() tea.Msg {
		soltar := make(chan struct{})
		time.AfterFunc(50*time.Millisecond, func() { close(soltar) })
		<-soltar
		return cargado{}
	}
}
func (c *cargador) Vista() string {
	if !c.listo {
		return datos.Texto("es", "comun.cargando")
	}
	return "listo"
}
func (c *cargador) Update(m tea.Msg) (pagina.Pagina, tea.Cmd) {
	if _, ok := m.(cargado); ok {
		c.listo = true
	}
	return c, nil
}

// Abrir no vuelve con la página en «Cargando…» mientras su carga sigue en camino, aunque lo que espera parezca quieto:
// si no, la prueba mira la vista antes de que lleguen los datos (lo que hacía fallar de vez en cuando a Historial y
// Agentes con la máquina cargada).
func TestAbrirEsperaLaCargaInicial(t *testing.T) {
	if a := Abrir(t, &cargador{}, pagina.Contexto{Ctx: context.Background()}); a.Vista() != "listo" {
		t.Fatalf("Abrir volvió con la página cargando: %s", a.Vista())
	}
}

// TestConexionOlvidadaNoResucita: una lectura que se cuenta después de que los dos lados se cerraron no deja la conexión
// como si tuviera datos sin leer; y lo que un lado escribe cuenta hasta que el otro lo lee.
func TestConexionOlvidadaNoResucita(t *testing.T) {
	a, b := net.Pipe()
	cli, srv := contada(a, "127.0.0.1:1", 0, &escucha{}), contada(b, "127.0.0.1:1", 1, &escucha{})
	cli.anotar(func(p *par) { p.escrito[0] += 3 })
	if sinLeer() == "" {
		t.Fatal("lo escrito y no leído cuenta")
	}
	srv.anotar(func(p *par) { p.leido[1] += 3 })
	cli.Close()
	srv.Close()
	cli.anotar(func(p *par) { p.leido[0] += 529 }) // una lectura que se anota tarde
	if s := sinLeer(); s != "" {
		t.Fatalf("una conexión cerrada no vuelve: %s", s)
	}
}

// TestQuieta: una goroutine que espera a leer de la red o a un canal no avanza sola; una que escribe en la red y espera a
// que el sistema confirme el envío, o que duerme, sí.
func TestQuieta(t *testing.T) {
	casos := map[string]bool{
		"goroutine 7 [IO wait]:\nnet.(*netFD).Read(0x1)\nnet.(*conn).Read(0x1)":   true,
		"goroutine 7 [IO wait]:\nnet.(*netFD).accept(0x1)":                        true,
		"goroutine 7 [IO wait]:\nnet.(*netFD).Write(0x1)\nnet.(*conn).Write(0x1)": false,
		"goroutine 7 [chan receive, 2 minutes]:\nmain.f()":                        true,
		"goroutine 7 [sleep]:\ntime.Sleep(0x1)":                                   false,
		"goroutine 7 [GC assist wait]:\nmain.f()":                                 false,
	}
	for pila, quiere := range casos {
		if quieta(pila) != quiere {
			t.Errorf("quieta(%q) debía ser %v", pila, quiere)
		}
	}
}

// apuntaFallo es una prueba que apunta su fallo y para ahí, como t.Fatalf, sin dar por fallida la de verdad.
type apuntaFallo struct {
	testing.TB
	fallo string
}

func (f *apuntaFallo) Helper() {}
func (f *apuntaFallo) Fatalf(formato string, args ...any) {
	f.fallo = fmt.Sprintf(formato, args...)
	runtime.Goexit()
}

// TestAsentarFallaSiNoSeAsienta: si algo sigue en marcha al acabar el plazo, Asentar no vuelve como si nada: la prueba
// falla y dice qué sigue en marcha.
func TestAsentarFallaSiNoSeAsienta(t *testing.T) {
	antes := plazo
	plazo = 50 * time.Millisecond
	t.Cleanup(func() { plazo = antes })
	c := &contador{}
	t.Cleanup(func() { c.parar.Store(true) })
	f := &apuntaFallo{TB: t}
	a := &App{t: f, P: c, llegan: make(chan tea.Msg, 1)}
	hecho := make(chan struct{})
	go func() {
		defer close(hecho)
		a.Pulsar("w")
	}()
	<-hecho
	if !strings.Contains(f.fallo, "no se asienta") || !strings.Contains(f.fallo, "prueba.(*contador).Update") {
		t.Fatalf("falla y dice qué sigue en marcha: %q", f.fallo)
	}
}
