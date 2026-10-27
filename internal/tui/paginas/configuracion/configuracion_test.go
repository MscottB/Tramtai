package configuracion_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/actualizacion"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/configuracion"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de Ajustes van por su interfaz, con el arnés de las páginas (prueba.App): teclas, los mensajes de pagina y
// comp, y lo que enseña Vista(). Lo que trae el servicio tras una tecla se espera con HastaVer o Hasta, y lo que la página
// cambia se lee en el servicio. Nada va a GitHub ni toca el arranque del equipo: la versión y el arranque son de prueba.

var (
	aviso    = prueba.Aviso
	contexto = prueba.Contexto
)

func tx(clave string, pares ...string) string { return datos.Textof("es", clave, pares...) }

func abierta(t *testing.T, cli *cliente.Cliente, id string) *prueba.App {
	t.Helper()
	a := prueba.Conducir(t, configuracion.Nueva("es"))
	cmd := a.P.Iniciar(contexto(t, cli, id))
	if v := a.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	a.Correr(cmd)
	a.EsperarCarga()
	return a
}

// linea es la primera línea de la vista que contiene texto; "" si ninguna.
func linea(a *prueba.App, texto string) string {
	for _, l := range strings.Split(a.Vista(), "\n") {
		if strings.Contains(l, texto) {
			return l
		}
	}
	return ""
}

// ir pone el cursor en la sección de ese título.
func ir(t *testing.T, a *prueba.App, titulo string) {
	t.Helper()
	for range 8 {
		a.Pulsar("up")
	}
	for range 8 {
		if strings.Contains(linea(a, "▸"), titulo) {
			return
		}
		a.Pulsar("down")
	}
	t.Fatalf("no se llega a %q:\n%s", titulo, a.Vista())
}

// todoTramtai pasa a la pestaña «Todo Tramtai».
func todoTramtai(t *testing.T, a *prueba.App) {
	t.Helper()
	a.Pulsar("right")
	if !strings.Contains(a.Vista(), "["+tx("config.pestanaTramtai")+"]") {
		t.Fatalf("→ pasa a Todo Tramtai:\n%s", a.Vista())
	}
}

func pregunta(fuera []tea.Msg) (comp.Pregunta, bool) {
	for _, m := range fuera {
		if q, ok := m.(comp.Pregunta); ok {
			return q, true
		}
	}
	return comp.Pregunta{}, false
}

// hastaAviso hace la acción y espera, entregando a la página lo que llegue, al primer aviso que sale tras ella.
func hastaAviso(a *prueba.App, accion func()) comp.NuevoAviso {
	desde := len(a.Fuera)
	accion()
	var av comp.NuevoAviso
	a.Hasta(func() bool {
		var ok bool
		av, ok = aviso(a.Fuera[desde:])
		return ok
	})
	return av
}

// vaciar borra el campo con el foco.
func vaciar(a *prueba.App) {
	for range 12 {
		a.Teclear("backspace")
	}
}

func predeterminado(t *testing.T, cli *cliente.Cliente, id string) string {
	t.Helper()
	cat, err := cli.Catalogo(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range cat {
		if m.Predeterminado {
			return m.ID
		}
	}
	return ""
}

func origen(t *testing.T, cli *cliente.Cliente, id, clave string) string {
	t.Helper()
	cfg, err := cli.ConfiguracionInstancia(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range cfg {
		if e.Clave == clave {
			return e.Origen
		}
	}
	return ""
}

func sistema() string {
	return map[string]string{"windows": "Windows", "darwin": "macOS", "linux": "Linux"}[runtime.GOOS]
}

// TestAjustesPredeterminado: Intro en el modelo predeterminado abre el selector con el catálogo; el elegido se guarda en el
// servicio, se avisa como la Web UI y se ve en la página.
func TestAjustesPredeterminado(t *testing.T) {
	cli, id := prueba.Servicio(t)
	cat, err := cli.Catalogo(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(cat, func(m contrato.Modelo) bool { return !m.Predeterminado })
	if i < 0 {
		t.Fatalf("el catálogo de prueba tiene otro modelo: %+v", cat)
	}
	otro := cat[i].ID
	a := abierta(t, cli, id)
	a.HastaVer(tx("predeterminado.titulo"), predeterminado(t, cli, id))
	ir(t, a, tx("predeterminado.titulo"))
	a.Pulsar("enter")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), otro) {
		t.Fatalf("Intro abre el selector con el catálogo:\n%s", a.Vista())
	}
	a.Escribir(otro)
	if av := hastaAviso(a, func() { a.Pulsar("enter") }); av.Texto != tx("predeterminado.guardado", "modelo", otro) || av.Tipo != comp.Exito {
		t.Fatalf("avisa como la Web UI: %+v", av)
	}
	a.Hasta(func() bool { return predeterminado(t, cli, id) == otro })
	a.Hasta(func() bool { return strings.Contains(linea(a, otro), otro) && !a.P.EnCampo() })
}

// TestAjustesHeredar: en un proyecto, un valor propio dice «Propio» y h lo devuelve al de Global (en el servicio); en el
// espacio global no se marca la procedencia ni se ofrece h.
func TestAjustesHeredar(t *testing.T) {
	cli, global := prueba.Servicio(t)
	dir := t.TempDir() // un config.yaml sin predeterminados: el modelo solo es propio por su local.yaml
	os.MkdirAll(filepath.Join(dir, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(dir, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Notas\n"), 0o600)
	e, err := cli.AbrirProyecto(context.Background(), dir, new(true), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.FijarPredeterminado(context.Background(), e.ID, "simulado", new("alto")); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, e.ID)
	a.HastaVer(tx("config.pestanaEspacio", "nombre", e.Nombre), tx("config.avisoProyecto")[:30])
	ir(t, a, tx("predeterminado.titulo"))
	if l := linea(a, tx("predeterminado.titulo")); !strings.Contains(l, tx("herencia.propio")) {
		t.Fatalf("el modelo propio dice «Propio»: %q", l)
	}
	if av := hastaAviso(a, func() { a.Pulsar("h") }); av.Texto != tx("herencia.vuelto") {
		t.Fatalf("avisa de que vuelve el de Global: %+v", av)
	}
	if o := origen(t, cli, e.ID, "predeterminados.esfuerzo"); o == "local" || o == "proyecto" {
		t.Fatalf("el esfuerzo vuelve al de Global: %q", o)
	}
	a.Hasta(func() bool { return strings.Contains(linea(a, tx("predeterminado.titulo")), tx("herencia.global")) })

	b := abierta(t, cli, global)
	if v := b.Vista(); strings.Contains(v, tx("herencia.propio")) || strings.Contains(v, tx("herencia.global")) {
		t.Fatalf("en el global no se marca la procedencia:\n%s", v)
	}
	if f := b.Pulsar("h"); len(f) != 0 {
		t.Fatalf("en el global, h no hace nada: %v", f)
	}
}

// TestAjustesRetencion: la retención valida en el propio campo, con los textos de la Web UI, y no se guarda con un error;
// bien escrita, se guarda en el servicio (el registro en días se guarda en horas).
func TestAjustesRetencion(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 80, Height: 20}) // a 60 columnas, los errores largos se cortan con «…»
	ir(t, a, tx("historialAjustes.titulo"))
	a.Pulsar("e")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), tx("historialAjustes.respuestas")) {
		t.Fatalf("e abre la retención:\n%s", a.Vista())
	}
	vaciar(a)
	a.Escribir("800")
	a.HastaVer(tx("historialAjustes.rango", "min", "0", "max", "720"))
	a.Pulsar("ctrl+s")
	if r, _ := cli.Retencion(context.Background(), id); r.HorasResultado == 800 || !a.P.EnCampo() {
		t.Fatalf("con un error no se guarda: %+v", r)
	}
	vaciar(a)
	a.Escribir("12")
	a.Pulsar("tab")
	vaciar(a)
	a.Escribir("0.25")
	a.HastaVer(tx("historialAjustes.registroMenor", "horas", "12"))
	vaciar(a)
	a.Escribir("2")
	if av := hastaAviso(a, func() { a.Pulsar("ctrl+s") }); av.Texto != tx("historialAjustes.guardado") {
		t.Fatalf("se guarda: %+v", av)
	}
	if r, _ := cli.Retencion(context.Background(), id); r.HorasResultado != 12 || r.HorasMetadatos != 48 {
		t.Fatalf("retención guardada: %+v", r)
	}
	if a.P.EnCampo() {
		t.Fatalf("el formulario se cierra:\n%s", a.Vista())
	}
}

// TestAjustesBorrarDatos: x elige qué borrar (el historial, marcado, y las APIs) y pregunta siempre antes, diciendo de qué
// espacio; cancelar no borra nada y confirmar borra lo elegido.
func TestAjustesBorrarDatos(t *testing.T) {
	cli, id := prueba.Servicio(t)
	ctx := context.Background()
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}})
	if err != nil {
		t.Fatal(err)
	}
	cli.Eventos(ctx, id, tr.ID, 0, func(int, string, string) error { return nil })
	if _, _, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "facturas", Agente: "simulado"}); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	ir(t, a, tx("datos.titulo"))
	a.Pulsar("x")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), tx("datos.historial")) || !strings.Contains(a.Vista(), tx("datos.puntos")) {
		t.Fatalf("x abre lo que se puede borrar:\n%s", a.Vista())
	}
	a.Pulsar("down", "space")
	q, ok := pregunta(a.Pulsar("ctrl+s"))
	if !ok || !strings.Contains(q.Texto, tx("datos.confirmar", "espacio", "Global")) {
		t.Fatalf("pregunta antes de borrar: %+v", q)
	}
	a.Entregar(comp.Cancelado{})
	if l, _ := cli.Puntos(ctx, id); len(l.Puntos) != 1 {
		t.Fatal("cancelar no borra")
	}
	a.Pulsar("x", "down", "space")
	pregunta(a.Pulsar("ctrl+s"))
	if av := hastaAviso(a, func() { a.Entregar(comp.Confirmado{}) }); av.Texto != tx("datos.hecho") {
		t.Fatalf("avisa de que se borró: %+v", av)
	}
	if l, _ := cli.Puntos(ctx, id); len(l.Puntos) != 0 {
		t.Fatalf("las APIs se borran: %+v", l.Puntos)
	}
	if tt, _ := cli.Trabajos(ctx, id, 10); len(tt) != 0 {
		t.Fatalf("el historial se vacía: %d", len(tt))
	}
}

// TestPuertoFueraDeRango: el puerto se valida en el campo (entre 1024 y 65535) y con un error no se guarda; uno válido se
// guarda con el automático, en el servicio.
func TestPuertoFueraDeRango(t *testing.T) {
	cli, id := prueba.Servicio(t)
	ctx := context.Background()
	antes, err := cli.AjustesServicio(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	todoTramtai(t, a)
	ir(t, a, tx("puerto.titulo"))
	a.HastaVer(tx("puerto.actual", "url", cli.Base))
	a.Pulsar("e")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), tx("puerto.campo")) {
		t.Fatalf("e abre el puerto:\n%s", a.Vista())
	}
	for _, malo := range []string{"80", "70000"} {
		vaciar(a)
		a.Escribir(malo)
		a.HastaVer(tx("puerto.regla"))
		a.Pulsar("ctrl+s")
		if d, _ := cli.AjustesServicio(ctx); d.PuertoConfigurado != antes.PuertoConfigurado || d.PuertoAutomatico != antes.PuertoAutomatico || !a.P.EnCampo() {
			t.Fatalf("con %s no se guarda: %+v", malo, d)
		}
	}
	vaciar(a)
	a.Escribir(strings.TrimPrefix(cli.Base[strings.LastIndex(cli.Base, ":"):], ":"))
	a.Pulsar("tab", "space")
	if av := hastaAviso(a, func() { a.Pulsar("ctrl+s") }); !strings.HasPrefix(av.Texto, tx("puerto.guardado")) {
		t.Fatalf("se guarda: %+v", av)
	}
	if d, _ := cli.AjustesServicio(ctx); d.PuertoAutomatico == antes.PuertoAutomatico {
		t.Fatalf("el automático cambia: %+v", d)
	}
}

// TestAjustesArranque: Intro abre la elección («No arrancar», «La app en la bandeja», «Solo el servicio»); lo elegido se guarda
// en el servicio con el arranque de prueba, se avisa como la Web UI y se ve en el título. Debajo, dónde está el servicio del
// sistema (otro entorno).
func TestAjustesArranque(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	todoTramtai(t, a)
	ir(t, a, tx("arranque.titulo", "sistema", sistema()))
	for _, quiero := range []string{tx("arranque.servidor"), "tramtai-servidor instalar", "https://mscottb.github.io/Tramtai/servidor.html"} {
		if !strings.Contains(strings.Join(strings.Fields(a.Vista()), " "), quiero) { // el texto largo se parte en líneas
			t.Fatalf("falta %q:\n%s", quiero, a.Vista())
		}
	}
	for _, c := range []struct{ opcion, aviso, modo string }{
		{"app", tx("arranque.activado", "sistema", sistema()), "app"},
		{"servicio", tx("arranque.activadoServicio", "sistema", sistema()), "servicio"},
		{"no", tx("arranque.desactivado"), ""},
	} {
		a.Pulsar("enter")
		if !a.P.EnCampo() {
			t.Fatalf("Intro abre la elección:\n%s", a.Vista())
		}
		a.Escribir(tx("arranque." + c.opcion))
		if av := hastaAviso(a, func() { a.Pulsar("enter") }); av.Texto != c.aviso || av.Tipo != comp.Exito {
			t.Fatalf("%s: %+v", c.opcion, av)
		}
		d, _ := cli.AjustesServicio(context.Background())
		if d.Autoarranque == nil || *d.Autoarranque != (c.modo != "") || d.ModoArranque != c.modo {
			t.Fatalf("%s: el arranque queda así en el servicio: %+v", c.opcion, d)
		}
		a.Hasta(func() bool {
			return strings.Contains(linea(a, tx("arranque.titulo", "sistema", sistema())), tx("arranque."+c.opcion)) && !a.P.EnCampo()
		})
	}
}

// TestAjustesConversor: el estado del conversor (el del equipo: puede tener markitdown instalado) y a cambia «Preparar al
// iniciar».
func TestAjustesConversor(t *testing.T) {
	cli, id := prueba.SinConversor(t)
	a := abierta(t, cli, id)
	todoTramtai(t, a)
	ir(t, a, tx("conversor.titulo"))
	a.HastaVer("[ ] " + tx("conversor.alIniciar"))
	antes, _ := cli.Conversor(context.Background(), id)
	a.Pulsar("a")
	a.Hasta(func() bool {
		e, _ := cli.Conversor(context.Background(), id)
		return e.PrepararAlIniciar != antes.PrepararAlIniciar
	})
	a.HastaVer("[x] " + tx("conversor.alIniciar"))
}

// conVersion es el servicio con la búsqueda activada y una versión nueva de prueba.
func conVersion(t *testing.T) (*cliente.Cliente, string) {
	t.Helper()
	return prueba.ConVersion(t, actualizacion.Version{Etiqueta: "v99.0.0", Pagina: "https://github.com/MscottB/Tramtai/releases/tag/v99.0.0",
		Notas: "Tramtai v99.0.0\n\nNovedades:\n- Ajustes en la terminal\n## English\nNew:\n- Settings in the terminal"})
}

// TestAjustesNovedades: con una versión nueva, Actualizaciones lo dice y u abre sus novedades en el idioma de la terminal;
// esta copia no se instaló con el instalador, así que explica cómo descargarla. Esc vuelve a la página sin salir.
func TestAjustesNovedades(t *testing.T) {
	cli, id := conVersion(t)
	a := abierta(t, cli, id)
	todoTramtai(t, a)
	ir(t, a, tx("actualizacion.ajustesTitulo"))
	a.HastaVer("v99.0.0")
	a.Pulsar("u")
	a.HastaVer(tx("actualizacion.titulo", "version", "v99.0.0"), "Ajustes en la terminal", tx("actualizacion.manual")[:30])
	if strings.Contains(a.Vista(), "Settings in the terminal") {
		t.Fatalf("las notas, en español:\n%s", a.Vista())
	}
	if f := a.Pulsar("esc"); len(f) != 0 || strings.Contains(a.Vista(), "Ajustes en la terminal") {
		t.Fatalf("Esc cierra las novedades: %v\n%s", f, a.Vista())
	}

	// IrA con ConNovedades (la u de la cabecera) las abre directamente.
	b := prueba.Conducir(t, configuracion.Nueva("es"))
	b.Correr(tea.Batch(b.P.Iniciar(contexto(t, cli, id)), func() tea.Msg { return pagina.ConNovedades{} }))
	b.HastaVer(tx("actualizacion.titulo", "version", "v99.0.0"))
}

// estadoDifundido es el último estado de las actualizaciones que la página dio a la app (para la cabecera).
func estadoDifundido(fuera []tea.Msg) (contrato.EstadoActualizacion, bool) {
	for i := len(fuera) - 1; i >= 0; i-- {
		if e, ok := fuera[i].(pagina.EstadoActualizacion); ok {
			return e.Estado, true
		}
	}
	return contrato.EstadoActualizacion{}, false
}

// TestBuscarAhora: b consulta en el momento aunque la búsqueda esté apagada y, si hay una versión nueva, abre sus novedades.
// Lo que sabe la página llega a la app, también al activar o desactivar la búsqueda: la cabecera no se queda atrás.
func TestBuscarAhora(t *testing.T) {
	cli, id := conVersion(t)
	if _, err := cli.BuscarActualizaciones(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	if e, ok := estadoDifundido(a.Fuera); !ok || e.Buscar || e.Disponible {
		t.Fatalf("al cargar, la app sabe que la búsqueda está apagada: %+v %v", e, ok)
	}
	todoTramtai(t, a)
	ir(t, a, tx("actualizacion.ajustesTitulo"))
	a.HastaVer(tx("actualizacion.desactivada", "actual", "")[:20])
	a.Pulsar("b")
	a.HastaVer(tx("actualizacion.titulo", "version", "v99.0.0"))
	// El estado para la app sale en otro mensaje que la vista: puede llegar justo después de verla (regla 20).
	a.Hasta(func() bool { e, _ := estadoDifundido(a.Fuera); return e.Disponible && e.Version == "v99.0.0" })
	a.Pulsar("esc", "space")
	a.Hasta(func() bool { e, _ := estadoDifundido(a.Fuera); return e.Buscar })
	a.Pulsar("space")
	a.Hasta(func() bool { e, _ := estadoDifundido(a.Fuera); return !e.Buscar })
}

// TestReiniciarTrasElPuerto: con un servicio que se puede reiniciar, guardar otro puerto pregunta si se reinicia ahora,
// como la Web UI; cancelar no reinicia, y R lo vuelve a ofrecer mientras el puerto configurado no es el que se usa.
// Confirmado, se pide el reinicio al servicio (el de la prueba solo lo apunta).
func TestReiniciarTrasElPuerto(t *testing.T) {
	var pedidos atomic.Int32
	cli, id := prueba.Ajustado(t, func(o *servicio.Opciones) { o.Reiniciar = func() error { pedidos.Add(1); return nil } })
	libre, _ := net.Listen("tcp", "127.0.0.1:0")
	puerto := strconv.Itoa(libre.Addr().(*net.TCPAddr).Port)
	libre.Close()
	a := abierta(t, cli, id)
	todoTramtai(t, a)
	ir(t, a, tx("puerto.titulo"))
	if slices.ContainsFunc(a.P.Teclas(), func(k pagina.Tecla) bool { return k.Tecla == "R" }) {
		t.Fatal("con el puerto de siempre no hay nada que reiniciar")
	}
	a.Pulsar("e")
	vaciar(a)
	a.Escribir(puerto)
	var q comp.Pregunta
	desde := len(a.Fuera)
	a.Pulsar("ctrl+s")
	a.Hasta(func() bool { var ok bool; q, ok = pregunta(a.Fuera[desde:]); return ok })
	if !strings.Contains(q.Texto, tx("puerto.reiniciarTitulo")) || !strings.Contains(q.Texto, tx("puerto.reiniciarTexto", "puerto", puerto)) {
		t.Fatalf("pregunta si se reinicia: %+v", q)
	}
	a.Entregar(comp.Cancelado{})
	if pedidos.Load() != 0 {
		t.Fatal("cancelar no reinicia")
	}
	a.Hasta(func() bool {
		return slices.ContainsFunc(a.P.Teclas(), func(k pagina.Tecla) bool { return k.Tecla == "R" && k.Accion == tx("puerto.reiniciarAhora") })
	})
	if _, ok := pregunta(a.Pulsar("R")); !ok {
		t.Fatal("R pregunta antes de reiniciar")
	}
	if av := hastaAviso(a, func() { a.Entregar(comp.Confirmado{}) }); av.Tipo != comp.Exito {
		t.Fatalf("avisa de que se reinicia: %+v", av)
	}
	if pedidos.Load() != 1 {
		t.Fatalf("se pide el reinicio una vez: %d", pedidos.Load())
	}
}

// actualizable pone delante del servicio un servidor que hace instalable la versión nueva y sirve el avance que le dé la
// prueba; apunta las decisiones (instalar, cancelar) y cuántas veces se pidió actualizar.
type actualizable struct {
	mu         sync.Mutex
	avance     contrato.ProgresoActualizacion
	pedidas    int
	decisiones []string
	caido      bool // el instalador detuvo el servicio: el avance ya no contesta
}

func (x *actualizable) poner(p contrato.ProgresoActualizacion) {
	x.mu.Lock()
	x.avance = p
	x.mu.Unlock()
}

func conActualizable(t *testing.T) (*cliente.Cliente, string, *actualizable) {
	t.Helper()
	cli, id := conVersion(t)
	destino, _ := url.Parse(cli.Base)
	pasar := httputil.NewSingleHostReverseProxy(destino)
	x := &actualizable{}
	base := prueba.Servidor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		x.mu.Lock()
		defer x.mu.Unlock()
		switch r.URL.Path {
		case "/api/v1/servicio/actualizacion":
			json.NewEncoder(w).Encode(contrato.EstadoActualizacion{Actual: "v0.56.0", Buscar: true, Disponible: true, Version: "v99.0.0", Instalable: true})
		case "/api/v1/servicio/actualizar":
			x.pedidas++
			json.NewEncoder(w).Encode(map[string]string{"version": "v99.0.0"})
		case "/api/v1/servicio/actualizacion/progreso":
			if x.caido {
				panic(http.ErrAbortHandler) // sin respuesta: el cliente ve un error de red
			}
			json.NewEncoder(w).Encode(x.avance)
		case "/api/v1/servicio/actualizar/instalar", "/api/v1/servicio/actualizar/cancelar":
			x.decisiones = append(x.decisiones, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
			w.WriteHeader(http.StatusNoContent)
		default:
			pasar.ServeHTTP(w, r)
		}
	}))
	copia := *cli
	copia.Base = base
	return &copia, id, x
}

// TestActualizarSigueElAvance: i instala; la página sigue el avance cada 400 ms como la Web UI (descargando, la cuenta
// atrás de listo) y, en listo, i reinicia ya y c cancela tras preguntar. Esc vuelve sin cancelar nada.
func TestActualizarSigueElAvance(t *testing.T) {
	cli, id, x := conActualizable(t)
	reloj := prueba.NuevoReloj()
	a := prueba.Abrir(t, configuracion.NuevaConReloj("es", reloj.Ahora, reloj.Espera), contexto(t, cli, id))
	todoTramtai(t, a)
	a.Pulsar("u")
	a.HastaVer(tx("actualizacion.comoInstala")[:30])
	x.poner(contrato.ProgresoActualizacion{Fase: "descargando", Version: "v99.0.0", Recibidos: 524288, Total: 1048576})
	visto := len(reloj.Pedidas())
	a.Pulsar("i")
	a.HastaVer(tx("actualizacion.progresoTitulo", "version", "v99.0.0"))
	tic(a, reloj, &visto)
	a.HastaVer(tx("actualizacion.descargando", "recibido", "0,5", "total", "1,0", "porcentaje", "50"))
	cuando := reloj.Ahora().Add(10*time.Second + 400*time.Millisecond) // a 10 s cuando llegue la consulta siguiente
	x.poner(contrato.ProgresoActualizacion{Fase: "listo", Version: "v99.0.0", InstalaEn: &cuando})
	tic(a, reloj, &visto)
	a.HastaVer(tx("actualizacion.reiniciaraEn", "n", "10"))
	q, ok := pregunta(a.Pulsar("c"))
	if !ok || !strings.Contains(q.Texto, tx("comun.cancelar")) {
		t.Fatalf("cancelar pregunta: %+v", q)
	}
	a.Entregar(comp.Cancelado{})
	a.Pulsar("i")
	a.Hasta(func() bool { x.mu.Lock(); defer x.mu.Unlock(); return slices.Equal(x.decisiones, []string{"instalar"}) })
	if f := a.Pulsar("esc"); len(f) != 0 || strings.Contains(a.Vista(), tx("actualizacion.progresoTitulo", "version", "v99.0.0")) {
		t.Fatalf("Esc cierra el avance sin cancelar: %v", f)
	}
	if x.pedidas != 1 || len(x.decisiones) != 1 {
		t.Fatalf("pedidas %d, decisiones %v", x.pedidas, x.decisiones)
	}
}

// TestInstalarCierraLaTerminal: el instalador cierra el servicio y la terminal, así que al empezar la instalación el avance
// dice cómo volver a abrirla y la página pide a la app que salga (pagina.Actualizando). Si tras «listo» el servicio deja
// de contestar sin que se viera «instalando», también. Y aunque el avance se cerrara con Esc o Ajustes se ocultara durante
// la cuenta atrás o la descarga: el sondeo sigue hasta que empieza la instalación.
func TestInstalarCierraLaTerminal(t *testing.T) {
	actualizando := func(fuera []tea.Msg) bool {
		return slices.ContainsFunc(fuera, func(m tea.Msg) bool { _, ok := m.(pagina.Actualizando); return ok })
	}
	for _, caso := range []string{"instalando", "sin respuesta tras listo", "Esc en la cuenta atrás", "Ajustes oculta", "Esc al descargar", "Ajustes oculta al descargar"} {
		cli, id, x := conActualizable(t)
		reloj := prueba.NuevoReloj()
		a := prueba.Abrir(t, configuracion.NuevaConReloj("es", reloj.Ahora, reloj.Espera), contexto(t, cli, id))
		todoTramtai(t, a)
		a.Pulsar("u")
		a.HastaVer(tx("actualizacion.comoInstala")[:30])
		cuando := reloj.Ahora().Add(10 * time.Second)
		descargando := strings.HasSuffix(caso, "al descargar")
		x.poner(contrato.ProgresoActualizacion{Fase: "listo", Version: "v99.0.0", InstalaEn: &cuando})
		enPantalla := tx("actualizacion.reiniciaraEn", "n", "10")
		if descargando {
			x.poner(contrato.ProgresoActualizacion{Fase: "descargando", Version: "v99.0.0", Recibidos: 524288, Total: 1048576})
			enPantalla = tx("actualizacion.descargando", "recibido", "0,5", "total", "1,0", "porcentaje", "50")
		}
		visto := len(reloj.Pedidas())
		a.Pulsar("i")
		tic(a, reloj, &visto)
		a.HastaVer(enPantalla)
		if actualizando(a.Fuera) {
			t.Fatalf("%s: antes de instalar la terminal sigue", caso)
		}
		switch caso {
		case "Esc en la cuenta atrás", "Esc al descargar":
			a.Pulsar("esc")
			if strings.Contains(a.Vista(), tx("actualizacion.progresoTitulo", "version", "v99.0.0")) {
				t.Fatalf("Esc cierra el avance:\n%s", a.Vista())
			}
		case "Ajustes oculta", "Ajustes oculta al descargar":
			a.P.(pagina.Ocultable).Ocultar()
		}
		x.mu.Lock()
		if caso != "sin respuesta tras listo" {
			x.avance = contrato.ProgresoActualizacion{Fase: "instalando", Version: "v99.0.0"}
		} else {
			x.caido = true
		}
		x.mu.Unlock()
		fuera := a.HastaSalir(func() { tic(a, reloj, &visto) }, actualizando)
		if !actualizando(fuera) {
			t.Fatalf("%s: la página pide a la app que salga: %v", caso, fuera)
		}
		if caso == "instalando" {
			v := strings.Join(strings.Fields(a.Vista()), " ")
			if !strings.Contains(v, "tramtai terminal") || strings.Contains(v, strings.Join(strings.Fields(tx("actualizacion.cerrara")), " ")) {
				t.Fatalf("el avance dice cómo volver a abrir la terminal, no que se abrirá sola:\n%s", a.Vista())
			}
		}
	}
}

// tic hace pasar una consulta del avance (400 ms) cuando la página ya pidió su espera siguiente: así ningún paso del
// reloj llega antes de que la página lo espere. visto son las esperas ya pedidas.
func tic(a *prueba.App, reloj *prueba.Reloj, visto *int) {
	a.Hasta(func() bool { return len(reloj.Pedidas()) > *visto })
	*visto = len(reloj.Pedidas())
	reloj.Pasar(400 * time.Millisecond)
	a.Asentar()
}

// TestActualizacionQueFallaDejaDeEsperar: unida a una actualización que falla en el servicio, el avance llega sin fase; a
// las 75 consultas (30 s) deja de esperar, como la Web UI. Tras ver una fase, al momento.
func TestActualizacionQueFallaDejaDeEsperar(t *testing.T) {
	cli, id, x := conActualizable(t)
	reloj := prueba.NuevoReloj()
	a := prueba.Abrir(t, configuracion.NuevaConReloj("es", reloj.Ahora, reloj.Espera), contexto(t, cli, id))
	todoTramtai(t, a)
	visto := len(reloj.Pedidas())
	a.Pulsar("u", "i")
	titulo := tx("actualizacion.progresoTitulo", "version", "v99.0.0")
	a.HastaVer(titulo)
	for range 74 {
		tic(a, reloj, &visto)
	}
	a.Hasta(func() bool { return len(reloj.Pedidas()) > visto }) // la consulta 75 ya espera
	if !strings.Contains(a.Vista(), titulo) {
		t.Fatalf("antes de 30 s sigue esperando:\n%s", a.Vista())
	}
	tic(a, reloj, &visto)
	a.Hasta(func() bool { return !strings.Contains(a.Vista(), titulo) })

	x.poner(contrato.ProgresoActualizacion{Fase: "descargando", Version: "v99.0.0", Recibidos: 104858})
	a.Pulsar("u", "i")
	tic(a, reloj, &visto)
	a.HastaVer(tx("actualizacion.descargandoSinTotal", "recibido", "0,1"))
	x.poner(contrato.ProgresoActualizacion{})
	tic(a, reloj, &visto)
	a.Hasta(func() bool { return !strings.Contains(a.Vista(), titulo) })
}

// TestUEnLasVistasAbiertas: con una versión nueva, u abre sus novedades también desde Acerca de y los valores técnicos,
// como dice la cabecera; durante el avance de una actualización no hace nada (la cuenta atrás sigue a la vista y no se
// vuelve a pedir).
func TestUEnLasVistasAbiertas(t *testing.T) {
	titulo := tx("actualizacion.titulo", "version", "v99.0.0")
	cli, id := conVersion(t)
	a := abierta(t, cli, id)
	ir(t, a, tx("config.valoresTecnicos"))
	a.Pulsar("enter", "u")
	a.HastaVer(titulo)
	a.Pulsar("esc")
	todoTramtai(t, a)
	ir(t, a, tx("acerca.titulo"))
	a.Pulsar("enter", "u")
	a.HastaVer(titulo)

	cli2, id2, x := conActualizable(t)
	reloj := prueba.NuevoReloj()
	b := prueba.Abrir(t, configuracion.NuevaConReloj("es", reloj.Ahora, reloj.Espera), contexto(t, cli2, id2))
	cuando := reloj.Ahora().Add(10 * time.Second)
	x.poner(contrato.ProgresoActualizacion{Fase: "listo", Version: "v99.0.0", InstalaEn: &cuando})
	visto := len(reloj.Pedidas())
	b.Pulsar("u", "i")
	tic(b, reloj, &visto)
	b.HastaVer(tx("actualizacion.reiniciaraEn", "n", "10"))
	if f := b.Pulsar("u"); len(f) != 0 || !strings.Contains(b.Vista(), tx("actualizacion.reiniciaraEn", "n", "10")) || x.pedidas != 1 {
		t.Fatalf("u en el avance no hace nada: %v (pedidas %d)\n%s", f, x.pedidas, b.Vista())
	}
}

// TestTrasReiniciarSeRelee: tras pedir el reinicio, la página espera a que responda el servicio nuevo (otro proceso) y
// vuelve a leer sus ajustes: R deja de ofrecerse y no quedan datos viejos. Mientras no responde, sigue esperando.
func TestTrasReiniciarSeRelee(t *testing.T) {
	var fase atomic.Int32 // 0 el de antes, 1 reiniciándose (no responde), 2 el nuevo
	cli, id := prueba.Ajustado(t, func(o *servicio.Opciones) { o.Reiniciar = func() error { fase.Store(1); return nil } })
	destino, _ := url.Parse(cli.Base)
	pasar := httputil.NewSingleHostReverseProxy(destino)
	base := prueba.Servidor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch f := fase.Load(); {
		case f == 1 && r.URL.Path != "/api/v1/servicio/reiniciar":
			w.WriteHeader(http.StatusBadGateway)
		case f == 2 && r.URL.Path == "/api/v1/servicio/acerca":
			json.NewEncoder(w).Encode(contrato.Acerca{Version: "v0.56.0", Servicio: contrato.ServicioAcerca{PID: -1}})
		case f == 2 && r.URL.Path == "/api/v1/servicio/ajustes":
			json.NewEncoder(w).Encode(contrato.AjustesServicio{Puerto: 18000, PuertoConfigurado: 18000, Reiniciable: true})
		default:
			pasar.ServeHTTP(w, r)
		}
	}))
	copia := *cli
	copia.Base = base
	libre, _ := net.Listen("tcp", "127.0.0.1:0")
	puerto := strconv.Itoa(libre.Addr().(*net.TCPAddr).Port)
	libre.Close()
	reloj := prueba.NuevoReloj()
	a := prueba.Abrir(t, configuracion.NuevaConReloj("es", reloj.Ahora, reloj.Espera), contexto(t, &copia, id))
	todoTramtai(t, a)
	ir(t, a, tx("puerto.titulo"))
	a.Pulsar("e")
	vaciar(a)
	a.Escribir(puerto)
	desde := len(a.Fuera)
	a.Pulsar("ctrl+s")
	a.Hasta(func() bool { _, ok := pregunta(a.Fuera[desde:]); return ok })
	visto := len(reloj.Pedidas())
	hastaAviso(a, func() { a.Entregar(comp.Confirmado{}) })
	conR := func() bool {
		return slices.ContainsFunc(a.P.Teclas(), func(k pagina.Tecla) bool { return k.Tecla == "R" })
	}
	esperarYPasar := func() { // cuando la página ya espera la pregunta siguiente
		a.Hasta(func() bool { return len(reloj.Pedidas()) > visto })
		visto = len(reloj.Pedidas())
		reloj.Pasar(time.Second)
		a.Asentar()
	}
	for range 3 { // el servicio no responde: sigue esperando, sin avisar de nada
		desde = len(a.Fuera)
		esperarYPasar()
		if len(a.Fuera[desde:]) != 0 || !conR() {
			t.Fatalf("mientras se reinicia, espera: %v", a.Fuera[desde:])
		}
	}
	fase.Store(2)
	esperarYPasar()
	a.Hasta(func() bool { return !conR() })
	a.HastaVer(tx("puerto.actual", "url", copia.Base))
}

// TestValoresTecnicos: Intro abre la configuración efectiva con el origen de cada valor; / filtra por clave o valor.
func TestValoresTecnicos(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	ir(t, a, tx("config.valoresTecnicos"))
	a.Pulsar("enter")
	a.HastaVer(tx("origen.internoDetalle"), tx("origen.localDetalle"))
	a.Pulsar("/")
	if !a.P.EnCampo() {
		t.Fatal("/ abre la búsqueda")
	}
	a.Escribir("predeterminados.agente")
	a.HastaVer("predeterminados.agente", "simulado", tx("origen.global"), tx("seccion.predeterminados"))
	if v := a.Vista(); strings.Contains(v, "retencion.") {
		t.Fatalf("filtra:\n%s", v)
	}
	a.Escribir("zzzz")
	a.HastaVer(tx("config.sinCoincidencias"))
	a.Pulsar("esc")
	if a.P.EnCampo() || strings.Contains(a.Vista(), tx("config.sinCoincidencias")) {
		t.Fatalf("Esc quita la búsqueda:\n%s", a.Vista())
	}
	if f := a.Pulsar("esc"); len(f) != 0 || strings.Contains(a.Vista(), tx("origen.internoDetalle")) {
		t.Fatalf("Esc cierra los valores: %v", f)
	}
}

// TestAcercaDe: la versión, el autor, la licencia y los enlaces de la Web UI; c copia los datos para soporte con la carpeta
// del usuario como ~.
func TestAcercaDe(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	todoTramtai(t, a)
	ir(t, a, tx("acerca.titulo"))
	a.Pulsar("enter")
	a.HastaVer("Michael Scott B.", "PolyForm Noncommercial 1.0.0", tx("acerca.repositorio"), "github.com/MscottB/Tramtai", tx("acerca.detalles"))
	f := a.Pulsar("c")
	i := slices.IndexFunc(f, func(m tea.Msg) bool { _, ok := m.(prueba.Portapapeles); return ok })
	if i < 0 {
		t.Fatalf("c copia: %v", f)
	}
	if c := string(f[i].(prueba.Portapapeles)); !strings.Contains(c, "Tramtai") || strings.Contains(strings.ToLower(c), `\users\`) {
		t.Fatalf("lo copiado no lleva la carpeta del usuario: %s", c)
	}
}

func TestEscVuelve(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	ir(t, a, tx("historialAjustes.titulo"))
	a.Pulsar("e")
	if f := a.Pulsar("esc"); len(f) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc cierra el formulario: %v", f)
	}
	if f := a.Pulsar("esc"); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("sin nada abierto, Esc vuelve: %+v", f)
	}
}

// TestServicioCaido: sin cliente o con el servicio cerrado, la página no se cierra y la app avisa.
func TestServicioCaido(t *testing.T) {
	hastaError := func(a *prueba.App) {
		a.Hasta(func() bool {
			return slices.ContainsFunc(a.Fuera, func(m tea.Msg) bool { e, ok := m.(pagina.ErrorServicio); return ok && pagina.NoResponde(e.Err) })
		})
	}
	sin := prueba.Conducir(t, configuracion.Nueva("es"))
	sin.Correr(sin.P.Iniciar(contexto(t, nil, "global")))
	hastaError(sin)
	cli, id, cerrar := prueba.Arrancar(t)
	a := abierta(t, cli, id)
	cerrar()
	a.Pulsar("r")
	hastaError(a)
	a.HastaVer("El servicio no responde · r reintenta")
}

// TestRespuestasViejas: una carga pedida antes de volver a iniciar la página se descarta aunque llegue después; un cambio
// ya hecho en el servicio, en el mismo espacio, sí se aplica.
func TestRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	ctx := context.Background()
	cat, _ := cli.Catalogo(ctx, id)
	i := slices.IndexFunc(cat, func(m contrato.Modelo) bool { return !m.Predeterminado })
	otro := cat[i].ID
	a := abierta(t, cli, id)
	vieja := a.Recoger(a.Retener(func() { a.Pulsar("r") })) // la carga de antes, con el modelo de antes
	if err := cli.FijarPredeterminado(ctx, id, otro, nil); err != nil {
		t.Fatal(err)
	}
	a.Correr(a.P.Iniciar(contexto(t, cli, id)))
	a.HastaVer(otro)
	if len(vieja) == 0 {
		t.Fatal("la carga vieja trae algo")
	}
	for _, m := range vieja {
		a.Entregar(m)
	}
	if !strings.Contains(a.Vista(), otro) {
		t.Fatalf("la carga vieja se descarta:\n%s", a.Vista())
	}

	// Un cambio retenido del mismo espacio se aplica tras volver a iniciar.
	ir(t, a, tx("historialAjustes.titulo"))
	a.Pulsar("e")
	vaciar(a)
	a.Escribir("6")
	accion := a.Retener(func() { a.Entregar(comp.Enviado{}) })
	a.Correr(a.P.Iniciar(contexto(t, cli, id)))
	a.Soltar(accion)
	a.Hasta(func() bool { r, _ := cli.Retencion(ctx, id); return r.HorasResultado == 6 })
}

// TestVolverConLaCargaEnCamino: si se deja la página antes de que llegue su carga (? abre la Ayuda) y se vuelve con Esc, esa
// carga ya no vale: Mostrar pide otra y la página no se queda en «Cargando…».
func TestVolverConLaCargaEnCamino(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := prueba.Conducir(t, configuracion.Nueva("es"))
	vieja := a.Retener(func() { a.Lanzar(a.P.Iniciar(contexto(t, cli, id))) })
	o := a.P.(pagina.Ocultable)
	o.Ocultar()
	a.Correr(o.Mostrar())
	a.HastaVer(tx("predeterminado.titulo"))
	a.Soltar(vieja)
	if !strings.Contains(a.Vista(), tx("predeterminado.titulo")) {
		t.Fatalf("la carga de antes no cambia nada:\n%s", a.Vista())
	}
}

// TestCabe: las dos pestañas, los formularios, el selector, los valores técnicos, Acerca de y las novedades caben en el área
// de 60×20; las confirmaciones caben en la caja de la app.
func TestCabe(t *testing.T) {
	cli, id := conVersion(t)
	if _, _, err := cli.CrearPunto(context.Background(), id, contrato.ConfigPunto{Nombre: "facturas", Agente: "simulado"}); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 60, Height: 17})
	comprobar := func(donde string) {
		t.Helper()
		v := a.Vista()
		if lipgloss.Height(v) > 17 || lipgloss.Width(v) > 60 {
			t.Errorf("%s mide %dx%d:\n%s", donde, lipgloss.Width(v), lipgloss.Height(v), v)
		}
	}
	comprobar("este espacio")
	for _, paso := range []struct{ seccion, tecla string }{{tx("predeterminado.titulo"), "enter"}, {tx("historialAjustes.titulo"), "e"},
		{tx("datos.titulo"), "x"}, {tx("config.valoresTecnicos"), "enter"}} {
		ir(t, a, paso.seccion)
		comprobar(paso.seccion)
		a.Pulsar(paso.tecla)
		a.Asentar()
		comprobar(paso.seccion + " abierto")
		a.Pulsar("esc")
	}
	todoTramtai(t, a)
	comprobar("todo Tramtai")
	for _, paso := range []struct{ seccion, tecla string }{{tx("puerto.titulo"), "e"}, {tx("acerca.titulo"), "enter"}, {tx("actualizacion.ajustesTitulo"), "u"},
		{tx("arranque.titulo", "sistema", sistema()), "enter"}} {
		ir(t, a, paso.seccion)
		comprobar(paso.seccion)
		a.Pulsar(paso.tecla)
		a.Asentar()
		comprobar(paso.seccion + " abierto")
		a.Pulsar("esc")
	}
	a.Pulsar("left")
	ir(t, a, tx("datos.titulo"))
	a.Pulsar("x", "down", "space")
	q, _ := pregunta(a.Pulsar("ctrl+s"))
	a.Entregar(comp.Cancelado{})
	v := (&comp.Confirmacion{Texto: q.Texto}).Vista(&pagina.Estilos{Iconos: pagina.IconosUnicode}, 58, 17)
	if q.Texto == "" || lipgloss.Height(v) > 17 || !strings.Contains(v, "s confirma · Esc cancela") {
		t.Fatalf("la pregunta de borrar mide %d filas:\n%s", lipgloss.Height(v), v)
	}
}
