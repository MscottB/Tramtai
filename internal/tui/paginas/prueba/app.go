package prueba

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"slices"
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
	t       testing.TB
	P       pagina.Pagina
	modelo  tea.Model // con ConducirApp, la terminal entera en lugar de P
	Fuera   []tea.Msg
	llegan  chan tea.Msg
	retener *[]tea.Cmd
}

// plazo es lo que se espera a que llegue lo esperado o se asiente la página antes de dar la prueba por fallida.
var plazo = 10 * time.Second

// Portapapeles es lo que la página manda copiar con OSC 52 (el mensaje de tea.SetClipboard no se exporta).
type Portapapeles string

// flujos son los flujos de eventos (SSE) abiertos: el transporte de las pruebas los cuenta hasta que se cierran.
var flujos atomic.Int64

// FlujosAbiertos son los flujos de eventos (SSE) que las páginas tienen abiertos ahora: el seguimiento en vivo de un
// trabajo y el del espacio.
func FlujosAbiertos() int { return int(flujos.Load()) }

// El transporte se cambia al cargar el paquete, antes de que corra ninguna prueba que lo pueda estar usando: cuenta lo
// que va y viene por las conexiones con los servidores de prueba (red.go) y los flujos.
func init() {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.DialContext = marcar
	http.DefaultTransport = transporteContado{base}
}

type transporteContado struct{ base http.RoundTripper }

func (t transporteContado) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(r)
	if err != nil || !strings.HasSuffix(r.URL.Path, "/eventos") {
		return res, err
	}
	flujos.Add(1)
	res.Body = &cuerpoContado{ReadCloser: res.Body}
	return res, nil
}

// cuerpoContado descuenta el flujo al cerrarse su cuerpo.
type cuerpoContado struct {
	io.ReadCloser
	una sync.Once
}

func (c *cuerpoContado) Close() error {
	c.una.Do(func() { flujos.Add(-1) })
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

// ConducirApp lleva la terminal entera en lugar de una página: todo lo que llega es para ella, y en Fuera solo quedan
// tea.Quit y lo que se copia.
func ConducirApp(t *testing.T, m tea.Model) *App {
	return &App{t: t, modelo: m, llegan: make(chan tea.Msg, 4096)}
}

// EsperarLlegada espera, sin entregarla, a que haya alguna respuesta lista para la página.
func (a *App) EsperarLlegada() {
	a.t.Helper()
	for limite := time.Now().Add(plazo); len(a.llegan) == 0; time.Sleep(time.Millisecond) {
		if time.Now().After(limite) {
			a.t.Fatalf("no llega nada:\n%s", a.Vista())
		}
	}
}

// Vista es lo que enseña la página, o la terminal.
func (a *App) Vista() string {
	if a.modelo != nil {
		return a.modelo.View().Content
	}
	return a.P.Vista()
}

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
	limite := time.After(plazo)
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

// Asentar entrega lo que llegue hasta que la prueba quede quieta: nada por entregar y nada en el proceso que pueda
// avanzar solo (enMarcha). Lo que queda esperando, espera a la prueba (que suelte un canal, haga pasar el Reloj o cierre
// el servicio) o a un temporizador del servicio, como el que junta el texto en vivo: lo que trae ese se espera con
// Hasta. Si en 10 s no se asienta, la prueba falla con la vista y lo que sigue en marcha.
func (a *App) Asentar() {
	a.t.Helper()
	limite := time.Now().Add(plazo)
	for {
		select {
		case m := <-a.llegan:
			a.entregar(m)
			continue
		default:
		}
		ocupadas := enMarcha()
		if len(ocupadas) == 0 && len(a.llegan) == 0 { // lo que llegó durante la foto se entrega en la vuelta siguiente
			return
		}
		if time.Now().After(limite) {
			a.t.Fatalf("no se asienta en %v:\n%s\n\nsigue en marcha:\n%s\n\nen las conexiones:\n%s", plazo, a.Vista(),
				strings.Join(ocupadas, "\n\n"), strings.Join(conConexion(), "\n\n"))
		}
		select {
		case m := <-a.llegan:
			a.entregar(m)
		case <-time.After(time.Millisecond):
		}
	}
}

// esperando son los estados de una goroutine que no avanza sola: espera a otra (un canal, un select, un cerrojo) o a
// un temporizador; la red va aparte, en quieta. En cualquier otro (corre, duerme, está en una llamada al sistema o espera al
// recolector de basura) avanzará sin que la prueba haga nada.
var esperando = []string{"chan receive", "chan send", "select", "sync.", "semacquire", "finalizer wait", "coroutine"}

// quieta dice si la goroutine de la pila g no avanza sola. «IO wait» solo cuenta si espera a que llegue algo de la red
// (una lectura o una conexión nueva); lo que llega por una conexión con el servicio ya lo cuenta sinLeer. Una escritura
// a medias también espera en «IO wait» (en Windows, a que el sistema confirme el envío), pero termina sola.
func quieta(g string) bool {
	estado := g[strings.Index(g, "[")+1 : strings.Index(g, "]")]
	if strings.HasPrefix(estado, "IO wait") {
		return strings.Contains(g, "net.(*netFD).Read(") || strings.Contains(g, "net.(*netFD).accept(")
	}
	return slices.ContainsFunc(esperando, func(e string) bool { return strings.HasPrefix(estado, e) })
}

// enMarcha es lo que puede avanzar solo: las pilas de las goroutines del proceso, sin la de la prueba, que no están
// esperando, y lo escrito en una conexión con un servidor de prueba que el otro lado aún no ha leído. Si no hay nada,
// nada cambia hasta que la prueba haga algo. runtime.Stack para el mundo, así que la foto vale para todas las goroutines
// a la vez; lo que se lee o escribe mientras se toma la invalida.
func enMarcha() []string {
	antes := cambios.Load()
	if conexion := sinLeer(); conexion != "" {
		return []string{conexion}
	}
	var ocupadas []string
	for _, g := range pilas() {
		if !quieta(g) {
			ocupadas = append(ocupadas, g)
		}
	}
	if len(ocupadas) == 0 && cambios.Load() != antes {
		return []string{"una conexión con el servicio se movió durante la foto"}
	}
	return ocupadas
}

// pilas son las de todas las goroutines del proceso menos la de la prueba, con el mundo parado.
func pilas() []string {
	pila := make([]byte, 1<<16)
	for n := runtime.Stack(pila, true); ; n = runtime.Stack(pila, true) {
		if n < len(pila) {
			pila = pila[:n]
			break
		}
		pila = make([]byte, 2*len(pila))
	}
	return strings.Split(string(pila), "\n\n")[1:] // la primera es la de la prueba
}

// conConexion son las pilas de las goroutines que están leyendo o escribiendo en una conexión contada: para explicar un
// fallo de Asentar por datos sin leer.
func conConexion() []string {
	var r []string
	for _, g := range pilas() {
		if strings.Contains(g, "prueba.(*conContada)") {
			r = append(r, g)
		}
	}
	return r
}

func (a *App) actualizar(m tea.Msg) {
	var cmd tea.Cmd
	if a.modelo != nil {
		a.modelo, cmd = a.modelo.Update(m)
	} else {
		a.P, cmd = a.P.Update(m)
	}
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
	go func() {
		if m := cmd(); m != nil {
			a.llegan <- m
		}
	}()
}

// entregar reparte un mensaje como la app: un lote se corre, lo de la app se guarda en Fuera y lo demás va a la página.
func (a *App) entregar(m tea.Msg) {
	switch m := m.(type) {
	case tea.BatchMsg:
		for _, c := range m {
			a.correr(c)
		}
	case tea.QuitMsg:
		a.Fuera = append(a.Fuera, m)
	case comp.Pregunta, comp.NuevoAviso, pagina.IrA, pagina.Volver, pagina.ErrorServicio:
		if a.modelo != nil {
			a.actualizar(m)
			return
		}
		a.Fuera = append(a.Fuera, m)
	default:
		if fmt.Sprintf("%T", m) == "tea.setClipboardMsg" {
			a.Fuera = append(a.Fuera, Portapapeles(fmt.Sprint(m)))
			return
		}
		a.actualizar(m)
	}
}
