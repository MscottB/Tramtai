package prueba

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

// App lleva una página como la terminal: cada Cmd corre en su goroutine, como en Bubble Tea, y lo que devuelve le llega a
// la página de uno en uno. Lo que es de la app (preguntas, avisos, IrA, Volver, ErrorServicio y lo que se copia) se guarda
// en Fuera. Así una prueba solo toca la página por su interfaz: teclas, los mensajes de pagina y comp, y Vista().
type App struct {
	t       *testing.T
	P       pagina.Pagina
	Fuera   []tea.Msg
	llegan  chan tea.Msg
	activos atomic.Int64
	retener *[]tea.Cmd
}

// Portapapeles es lo que la página manda copiar con OSC 52 (el mensaje de tea.SetClipboard no se exporta).
type Portapapeles string

// quieto es lo que se espera sin que llegue nada, con algo aún en marcha y ninguna petición al servicio a medias, para
// dar la página por asentada: lo que queda espera a algo de fuera (el seguimiento en vivo, un temporizador).
const quieto = 30 * time.Millisecond

// enCurso son las peticiones HTTP de las pruebas que aún no se han leído enteras, sin los flujos de eventos, que no
// acaban y se cuentan en flujos. El cliente usa el transporte predeterminado, que este paquete cambia por uno que las
// cuenta.
var enCurso, flujos atomic.Int64

// FlujosAbiertos son los flujos de eventos (SSE) que las páginas tienen abiertos ahora: el seguimiento en vivo de un
// trabajo y el del espacio.
func FlujosAbiertos() int { return int(flujos.Load()) }

// El transporte se cambia al cargar el paquete, antes de que corra ninguna prueba que lo pueda estar usando.
func init() { http.DefaultTransport = transporteContado{http.DefaultTransport} }

type transporteContado struct{ base http.RoundTripper }

func (t transporteContado) RoundTrip(r *http.Request) (*http.Response, error) {
	cuenta := &enCurso
	if strings.HasSuffix(r.URL.Path, "/eventos") {
		cuenta = &flujos
	}
	cuenta.Add(1)
	res, err := t.base.RoundTrip(r)
	if err != nil {
		cuenta.Add(-1)
		return res, err
	}
	res.Body = &cuerpoContado{ReadCloser: res.Body, cuenta: cuenta}
	return res, nil
}

// cuerpoContado descuenta la petición al cerrarse su cuerpo: hasta entonces la respuesta aún se está leyendo.
type cuerpoContado struct {
	io.ReadCloser
	cuenta *atomic.Int64
	una    sync.Once
}

func (c *cuerpoContado) Close() error {
	c.una.Do(func() { c.cuenta.Add(-1) })
	return c.ReadCloser.Close()
}

// Contexto es el de una página en una prueba: 60×17, en español y con iconos Unicode. Su Ctx se cancela al acabar la
// prueba, y con él lo que la página dejó esperando.
func Contexto(t *testing.T, cli *cliente.Cliente, instancia string) pagina.Contexto {
	ctx, cancelar := context.WithCancel(context.Background())
	t.Cleanup(cancelar)
	return pagina.Contexto{Ctx: ctx, Cli: cli, Instancia: instancia, Idioma: "es",
		Estilos: &pagina.Estilos{Iconos: pagina.IconosUnicode}, Ancho: 60, Alto: 17}
}

// Abrir inicia la página con c y espera a que se asiente, como al entrar en ella.
func Abrir(t *testing.T, p pagina.Pagina, c pagina.Contexto) *App {
	t.Helper()
	a := Conducir(t, p)
	a.Correr(p.Iniciar(c))
	return a
}

// Conducir lleva una página que la prueba inicia por su cuenta (para ver cómo queda antes de que lleguen sus datos).
func Conducir(t *testing.T, p pagina.Pagina) *App {
	a := &App{t: t, P: p, llegan: make(chan tea.Msg, 4096)}
	if o, ok := p.(pagina.Ocultable); ok {
		t.Cleanup(o.Ocultar)
	}
	return a
}

// EsperarLlegada espera, sin entregarla, a que haya alguna respuesta lista para la página.
func (a *App) EsperarLlegada() {
	a.t.Helper()
	for limite := time.Now().Add(10 * time.Second); len(a.llegan) == 0; time.Sleep(time.Millisecond) {
		if time.Now().After(limite) {
			a.t.Fatalf("no llega nada:\n%s", a.Vista())
		}
	}
}

// Vista es lo que enseña la página.
func (a *App) Vista() string { return a.P.Vista() }

// Pulsar manda las teclas de una en una, como la terminal, y espera a que la página se asiente tras cada una. Devuelve
// lo que salió hacia la app.
func (a *App) Pulsar(teclas ...string) []tea.Msg {
	desde := len(a.Fuera)
	for _, k := range teclas {
		a.actualizar(Tecla(k))
		a.Asentar()
	}
	return a.Fuera[desde:]
}

// Escribir teclea el texto letra a letra, sin esperar entre una y otra, y espera a que la página se asiente al final.
func (a *App) Escribir(texto string) []tea.Msg {
	desde := len(a.Fuera)
	for _, r := range texto {
		a.actualizar(Tecla(string(r)))
	}
	a.Asentar()
	return a.Fuera[desde:]
}

// Teclear manda las teclas seguidas, sin esperar entre una y otra, y espera a que la página se asiente al final.
func (a *App) Teclear(teclas ...string) []tea.Msg {
	desde := len(a.Fuera)
	for _, k := range teclas {
		a.actualizar(Tecla(k))
	}
	a.Asentar()
	return a.Fuera[desde:]
}

// Pegar pega el texto entero, como el pegado de la terminal.
func (a *App) Pegar(texto string) []tea.Msg { return a.Entregar(tea.PasteMsg{Content: texto}) }

// Entregar le da a la página un mensaje de la app (IrA con sus datos, Confirmado, CambioEspacio, el tamaño…).
func (a *App) Entregar(m tea.Msg) []tea.Msg {
	desde := len(a.Fuera)
	a.actualizar(m)
	a.Asentar()
	return a.Fuera[desde:]
}

// Correr corre un Cmd que devolvió la página fuera de Update (Iniciar, Mostrar) y espera a que se asiente.
func (a *App) Correr(cmd tea.Cmd) []tea.Msg {
	desde := len(a.Fuera)
	a.correr(cmd)
	a.Asentar()
	return a.Fuera[desde:]
}

// Lanzar corre un Cmd sin esperar a que la página se asiente: para ver cómo queda mientras algo sigue en camino.
func (a *App) Lanzar(cmd tea.Cmd) { a.correr(cmd) }

// Retener guarda, en lugar de correrlos, los Cmd que la página pida mientras dura f. Con Soltar se corren más tarde:
// así llega una respuesta a destiempo (de otra versión de la página, de otro espacio o con el contexto ya cancelado)
// por el mismo camino que una de verdad.
func (a *App) Retener(f func()) []tea.Cmd {
	var guardados []tea.Cmd
	a.retener = &guardados
	defer func() { a.retener = nil }()
	f()
	return guardados
}

// Recoger corre los Cmd retenidos y devuelve lo que responden, sin dárselo a la página: para entregarlo más tarde, ya
// viejo, o para retener a su vez lo que la página pida al recibirlo. Solo para Cmd que terminan.
func (a *App) Recoger(cmds []tea.Cmd) []tea.Msg {
	var msgs []tea.Msg
	for len(cmds) > 0 {
		c := cmds[0]
		cmds = cmds[1:]
		if c == nil {
			continue
		}
		switch m := c().(type) {
		case nil:
		case tea.BatchMsg:
			cmds = append(cmds, m...)
		default:
			msgs = append(msgs, m)
		}
	}
	return msgs
}

// Soltar corre los Cmd retenidos, entrega lo que devuelven y espera a que la página se asiente.
func (a *App) Soltar(cmds []tea.Cmd) []tea.Msg {
	desde := len(a.Fuera)
	for _, c := range cmds {
		a.correr(c)
	}
	a.Asentar()
	return a.Fuera[desde:]
}

// Hasta entrega lo que llegue hasta que se cumpla cond; si en 10 s no se cumple, la prueba falla con la vista.
func (a *App) Hasta(cond func() bool) {
	a.t.Helper()
	limite := time.After(10 * time.Second)
	for !cond() {
		select {
		case m := <-a.llegan:
			a.entregar(m)
		case <-time.After(10 * time.Millisecond): // lo que se espera puede no ser un mensaje (un archivo, el servicio)
		case <-limite:
			a.t.Fatalf("no llegó lo esperado:\n%s", a.Vista())
		}
	}
}

// HastaVer entrega lo que llegue hasta que la vista enseñe todos los textos.
func (a *App) HastaVer(textos ...string) {
	a.t.Helper()
	a.Hasta(func() bool {
		v := a.Vista()
		for _, s := range textos {
			if !strings.Contains(v, s) {
				return false
			}
		}
		return true
	})
}

// Asentar entrega lo que llegue hasta que no quede nada en marcha, o hasta que lo que quede solo espere a algo de fuera.
func (a *App) Asentar() {
	limite := time.Now().Add(10 * time.Second)
	for time.Now().Before(limite) {
		select {
		case m := <-a.llegan:
			a.entregar(m)
			continue
		default:
		}
		if a.activos.Load() == 0 {
			// Lo último que acabó ya dejó su mensaje (se envía antes de descontarse): se recoge y, si no hay, terminado.
			select {
			case m := <-a.llegan:
				a.entregar(m)
				continue
			default:
				return
			}
		}
		select {
		case m := <-a.llegan:
			a.entregar(m)
		case <-time.After(quieto):
			if enCurso.Load() == 0 {
				return
			}
		}
	}
}

func (a *App) actualizar(m tea.Msg) {
	p, cmd := a.P.Update(m)
	a.P = p
	a.correr(cmd)
}

func (a *App) correr(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if a.retener != nil {
		*a.retener = append(*a.retener, cmd)
		return
	}
	a.activos.Add(1)
	go func() {
		if m := cmd(); m != nil {
			a.llegan <- m
		}
		a.activos.Add(-1)
	}()
}

// entregar reparte un mensaje como la app: un lote se corre, lo de la app se guarda en Fuera y lo demás va a la página.
func (a *App) entregar(m tea.Msg) {
	switch m := m.(type) {
	case tea.BatchMsg:
		for _, c := range m {
			a.correr(c)
		}
	case comp.Pregunta, comp.NuevoAviso, pagina.IrA, pagina.Volver, pagina.ErrorServicio:
		a.Fuera = append(a.Fuera, m)
	default:
		if fmt.Sprintf("%T", m) == "tea.setClipboardMsg" {
			a.Fuera = append(a.Fuera, Portapapeles(fmt.Sprint(m)))
			return
		}
		a.actualizar(m)
	}
}
