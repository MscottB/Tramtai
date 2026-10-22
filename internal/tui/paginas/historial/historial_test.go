package historial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

func tecla(texto string) tea.KeyPressMsg {
	switch texto {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	return tea.KeyPressMsg{Code: []rune(texto)[0], Text: texto}
}

// correr ejecuta un Cmd y alimenta a la página con lo que sale, como haría la app. El sondeo se repite sin fin y el
// seguimiento en vivo espera su canal para siempre: a esos no se les vuelve a esperar (las pruebas los mandan a mano o
// leen p.canal). Devuelve los mensajes que son de la app.
func correr(p *Pagina, cmd tea.Cmd) []tea.Msg {
	var fuera []tea.Msg
	cola := []tea.Cmd{cmd}
	for len(cola) > 0 {
		c := cola[0]
		cola = cola[1:]
		if c == nil {
			continue
		}
		switch m := c().(type) {
		case nil:
		case tea.BatchMsg:
			cola = append(cola, m...)
		case comp.Pregunta, comp.NuevoAviso, pagina.IrA, pagina.Volver, pagina.ErrorServicio:
			fuera = append(fuera, m)
		case msgSondeo:
		case msgFlujo:
			p.Update(m)
		default:
			_, siguiente := p.Update(m)
			cola = append(cola, siguiente)
		}
	}
	return fuera
}

// hasta lee el seguimiento en vivo de la página hasta que se cumpla la condición.
func hasta(t *testing.T, p *Pagina, cond func() bool) {
	t.Helper()
	limite := time.After(5 * time.Second)
	for !cond() {
		select {
		case m, ok := <-p.canal:
			if !ok {
				t.Fatalf("el seguimiento se cerró antes de lo esperado:\n%s", p.Vista())
			}
			correr(p, p.recibirVivo(m))
		case <-limite:
			t.Fatalf("no llegó lo esperado:\n%s", p.Vista())
		}
	}
}

// pulsar manda teclas a la página y corre lo que piden; devuelve lo que sale hacia la app.
func pulsar(p *Pagina, teclas ...string) []tea.Msg {
	var fuera []tea.Msg
	for _, k := range teclas {
		_, cmd := p.Update(tecla(k))
		fuera = append(fuera, correr(p, cmd)...)
	}
	return fuera
}

func contexto(cli *cliente.Cliente, instancia string) pagina.Contexto {
	return pagina.Contexto{Ctx: context.Background(), Cli: cli, Instancia: instancia, Idioma: "es",
		Estilos: &pagina.Estilos{Iconos: pagina.IconosUnicode}, Ancho: 60, Alto: 17}
}

// nueva crea la página con las esperas instantáneas.
func nueva(lengua string) *Pagina {
	p := Nueva(lengua).(*Pagina)
	p.esperaPara = func(time.Duration) <-chan time.Time {
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	return p
}

// abierta inicia la página contra el servicio y espera la lista.
func abierta(t *testing.T, cli *cliente.Cliente, id string) *Pagina {
	t.Helper()
	p := nueva("es")
	cmd := p.Iniciar(contexto(cli, id))
	if v := p.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	correr(p, cmd)
	t.Cleanup(p.Ocultar)
	return p
}

func (p *Pagina) origenActual() origen {
	return origen{ctx: p.c.Ctx, instancia: p.c.Instancia, gen: p.gen}
}

// conTrabajos inicia la página y le da una lista hecha a mano, sin pedirla al servicio.
func conTrabajos(t *testing.T, lista ...nucleo.Trabajo) *Pagina {
	t.Helper()
	cli, id := prueba.Servicio(t)
	p := nueva("es")
	p.Iniciar(contexto(cli, id)) // su carga no se corre: la lista es la de la prueba
	t.Cleanup(p.Ocultar)
	p.Update(msgLista{origen: p.origenActual(), n: p.n, lista: lista})
	return p
}

func trabajo(id, texto string, estado nucleo.Estado) nucleo.Trabajo {
	return nucleo.Trabajo{ID: id, Estado: estado, CreadoEn: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		Solicitud: nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}}
}

func ids(p *Pagina) []string {
	var r []string
	for _, t := range p.lista.Visibles() {
		r = append(r, t.ID)
	}
	return r
}

var ctx = context.Background()

// terminar espera a que el trabajo acabe.
func terminar(t *testing.T, cli *cliente.Cliente, inst, id string) {
	t.Helper()
	if err := cli.Eventos(ctx, inst, id, 0, func(int, string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

// crear manda una petición y espera a que acabe.
func crear(t *testing.T, cli *cliente.Cliente, inst, texto string) nucleo.Trabajo {
	t.Helper()
	tr, err := cli.Crear(ctx, inst, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}})
	if err != nil {
		t.Fatal(err)
	}
	terminar(t, cli, inst, tr.ID)
	return tr
}

// crearInvalida manda una petición cuya respuesta no cumple el esquema (el agente simulado devuelve el mensaje tal cual)
// y la conserva, para poder corregirla.
func crearInvalida(t *testing.T, cli *cliente.Cliente, inst string) nucleo.Trabajo {
	t.Helper()
	tr, err := cli.Crear(ctx, inst, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: `{"total":"diez"}`}},
		Salida: nucleo.ConfigSalida{Formato: "json", ConservarSalidaInvalida: true,
			Esquema: json.RawMessage(`{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	terminar(t, cli, inst, tr.ID)
	return tr
}

// bloqueado es un agente que se queda esperando en las peticiones con el mensaje «esperar»: escribe un tramo cuando la
// prueba suelta paso y otro cuando suelta fin. Las demás las contesta al momento con su mensaje. Así hay una petición en
// marcha sin depender del reloj.
type bloqueado struct {
	entro, paso, fin chan struct{}
	una, dos, tres   sync.Once
}

func nuevoBloqueado() *bloqueado {
	return &bloqueado{entro: make(chan struct{}), paso: make(chan struct{}), fin: make(chan struct{})}
}

func (b *bloqueado) soltarPaso() { b.dos.Do(func() { close(b.paso) }) }
func (b *bloqueado) soltarFin()  { b.tres.Do(func() { close(b.fin) }) }

func (b *bloqueado) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	mensaje := s.Mensajes[len(s.Mensajes)-1].Contenido
	if mensaje != "esperar" {
		_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: mensaje})
		return adaptadores.Resultado{Salida: mensaje}, nil
	}
	b.una.Do(func() { close(b.entro) })
	select {
	case <-b.paso:
	case <-ctx.Done():
		return adaptadores.Resultado{}, ctx.Err()
	}
	_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: "primera parte"})
	select {
	case <-b.fin:
	case <-ctx.Done():
		return adaptadores.Resultado{}, ctx.Err()
	}
	_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: " segunda parte"})
	return adaptadores.Resultado{Salida: "primera parte segunda parte"}, nil
}

// conBloqueado levanta el servicio con el agente bloqueado y manda una petición «esperar»: vuelve cuando el agente ya la tiene.
func conBloqueado(t *testing.T) (*cliente.Cliente, string, *bloqueado, nucleo.Trabajo) {
	t.Helper()
	b := nuevoBloqueado()
	cli, id, _ := prueba.ArrancarCon(t, b)
	t.Cleanup(func() { b.soltarPaso(); b.soltarFin() })
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "esperar"}}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.entro:
	case <-time.After(5 * time.Second):
		t.Fatal("el agente no recibió la petición")
	}
	return cli, id, b, tr
}

func aviso(fuera []tea.Msg) (comp.NuevoAviso, bool) {
	for _, m := range fuera {
		if a, ok := m.(comp.NuevoAviso); ok {
			return a, true
		}
	}
	return comp.NuevoAviso{}, false
}

func pregunta(fuera []tea.Msg) (comp.Pregunta, bool) {
	for _, m := range fuera {
		if q, ok := m.(comp.Pregunta); ok {
			return q, true
		}
	}
	return comp.Pregunta{}, false
}

func TestHistorialCarga(t *testing.T) {
	cli, id := prueba.Servicio(t)
	for _, texto := range []string{"primera", "segunda", "tercera"} {
		crear(t, cli, id, texto)
	}
	p := abierta(t, cli, id)
	v := p.Vista()
	for _, quiere := range []string{"Historial", "Estado: Todas", "Origen: Todos los orígenes", "Completado", "tercera", "primera", "Mostrando las 3 más recientes", "hace "} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if !strings.Contains(v, "▸ ✓ Completado") {
		t.Fatalf("el cursor está en la primera fila:\n%s", v)
	}
	// Los más nuevos primero.
	if i, j := strings.Index(v, "tercera"), strings.Index(v, "primera"); i > j {
		t.Fatalf("la más nueva va primero:\n%s", v)
	}
	if p.Titulo() != "Historial" || p.Ayuda() != "historial" || p.EnCampo() {
		t.Fatalf("título %q, ayuda %q, en campo %v", p.Titulo(), p.Ayuda(), p.EnCampo())
	}
}

func TestHistorialSinPeticiones(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	v := p.Vista()
	for _, quiere := range []string{"Aún no hay peticiones", "p Ir a Probar"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	ir, ok := pulsar(p, "p")[0].(pagina.IrA)
	if !ok || ir.Pagina != pagina.PaginaEjecutar {
		t.Fatalf("p lleva a Probar: %+v", ir)
	}
	// Con un filtro el vacío es otro y se ofrece quitarlo.
	pulsar(p, "e")
	v = p.Vista()
	if !strings.Contains(v, "No hay peticiones con este filtro") || !strings.Contains(v, "l Quitar filtros") {
		t.Fatalf("vacío con filtro:\n%s", v)
	}
	if fuera := pulsar(p, "p"); len(fuera) != 0 {
		t.Fatalf("p solo vale sin filtros: %v", fuera)
	}
	pulsar(p, "l")
	if p.hayFiltro() || !strings.Contains(p.Vista(), "Aún no hay peticiones") {
		t.Fatalf("l quita los filtros:\n%s", p.Vista())
	}
}

// TestHistorialVerMasSinRepetir: «Ver más» pide la página siguiente con el cursor del último trabajo (antes y antes_id): cada
// petición sale una sola vez y en orden, hasta que no queda más.
func TestHistorialVerMasSinRepetir(t *testing.T) {
	cli, id := prueba.Servicio(t)
	for i := range 7 {
		crear(t, cli, id, fmt.Sprintf("petición %d", i))
	}
	todas, err := cli.TrabajosFiltrados(ctx, id, cliente.FiltroTrabajos{Limite: 200})
	if err != nil || len(todas) != 7 {
		t.Fatalf("el servicio tiene 7: %d %v", len(todas), err)
	}
	var esperado []string
	for _, tr := range todas {
		esperado = append(esperado, tr.ID)
	}

	p := nueva("es")
	p.tam = 3
	t.Cleanup(p.Ocultar)
	correr(p, p.Iniciar(contexto(cli, id)))
	if len(ids(p)) != 3 || !p.hayMas {
		t.Fatalf("la primera página trae 3 y hay más: %v %v", ids(p), p.hayMas)
	}
	if v := p.Vista(); !strings.Contains(v, "Mostrando las 3 más recientes") || !strings.Contains(p.tx("historial.verMas"), "Ver más") {
		t.Fatalf("dice cuántas muestra:\n%s", v)
	}
	var pie []string
	for _, k := range p.Teclas() {
		pie = append(pie, k.Tecla+" "+k.Accion)
	}
	if !slices.Contains(pie, "m Ver más") {
		t.Fatalf("el pie ofrece «Ver más»: %v", pie)
	}
	pulsar(p, "m")
	if got := ids(p); len(got) != 6 || !slices.Equal(got, esperado[:6]) {
		t.Fatalf("la segunda página sigue a la primera sin repetir: %v\nquería %v", got, esperado[:6])
	}
	if !p.hayMas {
		t.Fatal("una página llena puede tener más")
	}
	pulsar(p, "m")
	if got := ids(p); !slices.Equal(got, esperado) {
		t.Fatalf("las 7, una vez cada una: %v\nquería %v", got, esperado)
	}
	if p.hayMas {
		t.Fatal("una página corta es la última")
	}
	if v := p.Vista(); !strings.Contains(v, "Mostrando las 7 más recientes") {
		t.Fatalf("cuenta las 7:\n%s", v)
	}
	pulsar(p, "m")
	if len(ids(p)) != 7 {
		t.Fatalf("m sin más páginas no hace nada: %v", ids(p))
	}
	// Una página que llega tarde, de un filtro viejo, no se suma a la lista de ahora.
	viejo := p.n
	p.hayMas, p.enVuelo = true, 0
	p.verMas()
	p.Update(pagina.FiltroPunto{Nombre: "p:no-existe"})
	p.Update(msgLista{origen: p.origenActual(), n: viejo, mas: true, lista: []nucleo.Trabajo{trabajo("zz", "vieja", nucleo.Completado)}})
	if slices.Contains(ids(p), "zz") || p.cargandoMas {
		t.Fatalf("la página del filtro viejo no se suma: %v", ids(p))
	}
}

// TestHistorialFiltroPunto: llega con IrA{historial, FiltroPunto}: una API por su nombre, o un filtro ya armado como «o:terminal»
// (lo filtra el servicio); el selector de origen ofrece los orígenes, los grupos y las APIs.
func TestHistorialFiltroPunto(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "desde la terminal")
	if _, _, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "mi-app", Grupo: "web", Agente: "simulado"}); err != nil {
		t.Fatal(err)
	}
	_, token, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "otra", Grupo: "web", Agente: "simulado"})
	if err != nil {
		t.Fatal(err)
	}
	_, tokenApp, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "tienda", Agente: "simulado"})
	if err != nil {
		t.Fatal(err)
	}
	porApi := func(punto, token, texto string) {
		t.Helper()
		req, _ := http.NewRequest("POST", cli.Base+"/p/"+punto+"/api/v1/trabajos", strings.NewReader(`{"mensajes":[{"rol":"usuario","contenido":"`+texto+`"}]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode >= 300 {
			t.Fatalf("petición de la app: %v %v", resp, err)
		}
		var tr nucleo.Trabajo
		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		terminar(t, cli, id, tr.ID)
	}
	porApi("otra", token, "de la otra")
	porApi("tienda", tokenApp, "de la tienda")

	p := abierta(t, cli, id)
	if len(ids(p)) != 3 {
		t.Fatalf("sin filtro salen las 3: %v", ids(p))
	}
	// IrA{historial, FiltroPunto{«tienda»}}: llega después de Iniciar y filtra por la API.
	correr(p, p.Iniciar(contexto(cli, id)))
	_, cmd := p.Update(pagina.FiltroPunto{Nombre: "tienda"})
	correr(p, cmd)
	v := p.Vista()
	if got := ids(p); len(got) != 1 || !strings.Contains(v, "de la tienda") || strings.Contains(v, "de la otra") {
		t.Fatalf("solo la API tienda: %v\n%s", got, v)
	}
	if !strings.Contains(v, "Origen: API tienda") {
		t.Fatalf("la barra dice el origen:\n%s", v)
	}
	// Un filtro ya armado.
	_, cmd = p.Update(pagina.FiltroPunto{Nombre: "o:terminal"})
	correr(p, cmd)
	if v := p.Vista(); len(ids(p)) != 1 || !strings.Contains(v, "desde la terminal") || !strings.Contains(v, "Origen: Terminal") {
		t.Fatalf("solo el origen terminal:\n%s", v)
	}
	// El selector: los cuatro orígenes, el grupo y las APIs.
	pulsar(p, "f")
	if !p.EnCampo() {
		t.Fatal("con el selector abierto las letras son suyas")
	}
	v = p.Vista()
	for _, quiere := range []string{"Todos los orígenes", "Probar", "Terminal", "CLI", "App (clave del espacio)", "Grupo web", "API mi-app", "API otra", "API tienda"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("el selector ofrece %q:\n%s", quiere, v)
		}
	}
	for _, c := range "grupo w" {
		pulsar(p, string(c))
	}
	pulsar(p, "enter") // elige «Grupo web»: las APIs otra y mi-app
	if p.EnCampo() {
		t.Fatal("Intro cierra el selector")
	}
	if v := p.Vista(); len(ids(p)) != 1 || !strings.Contains(v, "de la otra") || !strings.Contains(v, "Origen: Grupo web") {
		t.Fatalf("el grupo web:\n%s", v)
	}
	// Esc cierra el selector sin cambiar nada; l quita los filtros.
	pulsar(p, "f", "esc")
	if p.EnCampo() || p.origenF != "g:web" {
		t.Fatalf("Esc cierra el selector y conserva el filtro: %q", p.origenF)
	}
	pulsar(p, "l")
	if len(ids(p)) != 3 || p.hayFiltro() {
		t.Fatalf("l quita los filtros: %v", ids(p))
	}
}

func TestHistorialFiltroEstado(t *testing.T) {
	p := conTrabajos(t,
		trabajo("a", "uno", nucleo.Completado), trabajo("b", "dos", nucleo.Fallido), trabajo("c", "tres", nucleo.Ejecutando),
		trabajo("d", "cuatro", nucleo.Cancelado), trabajo("e", "cinco", nucleo.Interrumpido), trabajo("f", "seis", nucleo.EnCola),
		trabajo("g", "siete", nucleo.ValidandoSalida), trabajo("h", "ocho", nucleo.CancelacionSolicitada))
	casos := []struct {
		estado string
		quiere []string
	}{
		{"ok", []string{"a"}},
		{"curso", []string{"c", "f", "g", "h"}},
		{"error", []string{"b", "e"}},
		{"todos", []string{"a", "b", "c", "d", "e", "f", "g", "h"}},
		{"nada", []string{"a", "b", "c", "d", "e", "f", "g", "h"}},
		{"", []string{"a", "b", "c", "d", "e", "f", "g", "h"}},
	}
	for _, c := range casos {
		p.Update(pagina.FiltroEstado{Estado: c.estado})
		got := ids(p)
		slices.Sort(got)
		if !slices.Equal(got, c.quiere) {
			t.Fatalf("FiltroEstado %q: %v, quería %v", c.estado, got, c.quiere)
		}
	}
	// La tecla e recorre los estados como el selector de la Web UI: todas, en proceso, completadas y con error.
	p.Update(pagina.FiltroEstado{Estado: "todos"})
	for _, quiere := range []string{"En proceso", "Completadas", "Con error", "Todas"} {
		pulsar(p, "e")
		if v := p.Vista(); !strings.Contains(v, "Estado: "+quiere) {
			t.Fatalf("e pasa a %q:\n%s", quiere, v)
		}
	}
	// Los cancelados solo salen en «Todas».
	for _, estado := range []string{"curso", "ok", "error"} {
		p.Update(pagina.FiltroEstado{Estado: estado})
		if slices.Contains(ids(p), "d") {
			t.Fatalf("un cancelado no entra en %q", estado)
		}
	}
	// Con el estado puesto, un filtro que no deja nada dice que es del filtro, y l lo quita.
	p.Update(pagina.FiltroEstado{Estado: "ok"})
	p.Update(msgLista{origen: p.origenActual(), n: p.n, lista: []nucleo.Trabajo{trabajo("z", "uno", nucleo.Fallido)}})
	if v := p.Vista(); !strings.Contains(v, "No hay peticiones con este filtro") {
		t.Fatalf("vacío por el filtro:\n%s", v)
	}
	pulsar(p, "l")
	if got := ids(p); !slices.Equal(got, []string{"z"}) {
		t.Fatalf("l deja todas: %v", got)
	}
}

func TestHistorialBusqueda(t *testing.T) {
	p := conTrabajos(t, trabajo("trabajo_aaa", "Factura de Soporte", nucleo.Completado), trabajo("trabajo_bbb", "Notas de reunión", nucleo.Completado),
		trabajo("trabajo_ccc", "Otra cosa", nucleo.Fallido))
	pulsar(p, "/")
	if !p.EnCampo() {
		t.Fatal("con la búsqueda abierta las letras son suyas")
	}
	for _, c := range "reunion" { // sin tilde también encuentra «reunión»
		pulsar(p, string(c))
	}
	if got := ids(p); !slices.Equal(got, []string{"trabajo_bbb"}) {
		t.Fatalf("filtra al escribir: %v", got)
	}
	if v := p.Vista(); !strings.Contains(v, "/ reunion_") {
		t.Fatalf("la barra enseña lo que se escribe:\n%s", v)
	}
	pulsar(p, "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace")
	if len(ids(p)) != 3 {
		t.Fatalf("sin texto salen todas: %v", ids(p))
	}
	// También por identificador.
	for _, c := range "ccc" {
		pulsar(p, string(c))
	}
	if got := ids(p); !slices.Equal(got, []string{"trabajo_ccc"}) {
		t.Fatalf("busca por identificador: %v", got)
	}
	// Intro la deja puesta y la barra la recuerda; Esc de una nueva vuelve a lo de antes.
	pulsar(p, "enter")
	if p.EnCampo() || p.busqueda != "ccc" || !strings.Contains(p.Vista(), "/ ccc") {
		t.Fatalf("Intro deja la búsqueda: %q\n%s", p.busqueda, p.Vista())
	}
	pulsar(p, "/", "x", "esc")
	if p.EnCampo() || p.busqueda != "ccc" || !slices.Equal(ids(p), []string{"trabajo_ccc"}) {
		t.Fatalf("Esc vuelve a la búsqueda de antes: %q %v", p.busqueda, ids(p))
	}
	// Lo pegado entra entero, con los saltos de línea como espacios.
	pulsar(p, "/")
	p.Update(tea.PasteMsg{Content: "Factura\nde Soporte"})
	if got := ids(p); p.entrada != "cccFactura de Soporte" || len(got) != 0 {
		t.Fatalf("pegar: %q %v", p.entrada, got)
	}
	pulsar(p, "esc", "l")
	if len(ids(p)) != 3 {
		t.Fatalf("l quita la búsqueda: %v", ids(p))
	}
}

func TestHistorialDetalle(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "hola mundo")
	p := abierta(t, cli, id)
	pulsar(p, "enter")
	v := p.Vista()
	for _, quiere := range []string{"✓ Completado", "En cola ▸ Preparación ▸ Ejecución ▸ Validación ▸ [Llegada]", "[Mensaje]", "Respuesta", "Registro",
		"hola mundo", "Agente y modelo: ", "Origen: Terminal", "Tokens: 0 → 0", "Duración: ", "Identificador: trabajo_"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q en el detalle:\n%s", quiere, v)
		}
	}
	if p.enVivo() || p.canal != nil {
		t.Fatal("un trabajo terminado no se sigue en vivo")
	}
	// La respuesta.
	pulsar(p, "right")
	if v := p.Vista(); !strings.Contains(v, "[Respuesta]") || !strings.Contains(v, "hola mundo") {
		t.Fatalf("pestaña Respuesta:\n%s", v)
	}
	// El registro: la hora de cada paso, con el resumen de lo que duró cada etapa.
	pulsar(p, "right")
	v = p.Vista()
	for _, quiere := range []string{"[Registro]", "En cola", "Ejecutando", "Completado", " +"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q en el registro:\n%s", quiere, v)
		}
	}
	if !strings.Contains(v, "simulado") {
		t.Fatalf("el intento dice con qué agente:\n%s", v)
	}
	pulsar(p, "right") // no pasa de la última
	if p.pestana != pestanaRegistro {
		t.Fatalf("la última pestaña es la del registro: %d", p.pestana)
	}
	pulsar(p, "left", "left", "left")
	if p.pestana != pestanaMensaje {
		t.Fatalf("← vuelve al mensaje: %d", p.pestana)
	}
	// El pie del detalle.
	var pie []string
	for _, k := range p.Teclas() {
		pie = append(pie, k.Tecla+" "+k.Accion)
	}
	for _, quiere := range []string{"Esc vuelve", "←→ pestañas", "/ busca", "t Reintentar", "x Eliminar"} {
		if !slices.Contains(pie, quiere) {
			t.Fatalf("el pie del detalle ofrece %q: %v", quiere, pie)
		}
	}
	// Esc cierra el detalle y, sin nada abierto, vuelve.
	if fuera := pulsar(p, "esc"); len(fuera) != 0 || p.abierto != "" {
		t.Fatalf("Esc cierra el detalle: %v %q", fuera, p.abierto)
	}
	if _, ok := pulsar(p, "esc")[0].(pagina.Volver); !ok {
		t.Fatal("Esc sin nada abierto vuelve")
	}
	// Buscar en el visor.
	pulsar(p, "enter", "/")
	if !p.EnCampo() {
		t.Fatal("la búsqueda del detalle tiene el foco")
	}
	for _, c := range "mundo" {
		pulsar(p, string(c))
	}
	pulsar(p, "enter")
	if p.EnCampo() {
		t.Fatal("Intro aplica la búsqueda del detalle")
	}
}

func TestHistorialDetalleConError(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crearInvalida(t, cli, id)
	p := abierta(t, cli, id)
	if v := p.Vista(); !strings.Contains(v, "Falló") {
		t.Fatalf("la fila dice cómo acabó:\n%s", v)
	}
	pulsar(p, "enter")
	v := p.Vista()
	if !strings.Contains(v, "✗ La respuesta no cumple el JSON Schema: ") || !strings.Contains(v, "[Falló]") {
		t.Fatalf("el título del código y el mensaje del servicio, tal cual:\n%s", v)
	}
	pulsar(p, "right")
	if v := p.Vista(); !strings.Contains(v, "diez") || !strings.Contains(v, "La respuesta no cumple el JSON Schema") {
		t.Fatalf("la respuesta que no cumplía sale con el error:\n%s", v)
	}
	var pie []string
	for _, k := range p.Teclas() {
		pie = append(pie, k.Tecla+" "+k.Accion)
	}
	if !slices.Contains(pie, "o Corregir") {
		t.Fatalf("Corregir solo en una que no cumplía el esquema: %v", pie)
	}
}

// TestHistorialEnVivo: una petición en marcha se sigue con los eventos SSE del trabajo: la respuesta se va escribiendo, el
// estado cambia y, al terminar, el detalle queda con el trabajo final. El seguimiento se corta al cerrar el detalle.
func TestHistorialEnVivo(t *testing.T) {
	cli, id, b, tr := conBloqueado(t)
	p := abierta(t, cli, id)
	if got := ids(p); !slices.Equal(got, []string{tr.ID}) {
		t.Fatalf("la lista tiene la petición en marcha: %v", got)
	}
	if v := p.Vista(); !strings.Contains(v, "esperar") {
		t.Fatalf("la fila:\n%s", v)
	}
	pulsar(p, "enter")
	if p.detalle == nil || p.detalle.Estado.Terminal() || p.canal == nil || !p.enVivo() {
		t.Fatalf("abre el detalle y sigue en vivo: %+v", p.detalle)
	}
	canal := p.canal
	if v := p.Vista(); !strings.Contains(v, "en vivo") || !strings.Contains(v, "●") {
		t.Fatalf("dice que está en vivo:\n%s", v)
	}
	pulsar(p, "right")
	if v := p.Vista(); !strings.Contains(v, "Todavía en curso…") {
		t.Fatalf("aún no hay respuesta:\n%s", v)
	}
	// El primer tramo llega mientras sigue en marcha.
	b.soltarPaso()
	hasta(t, p, func() bool { return strings.Contains(p.vivoSalida, "primera parte") })
	v := p.Vista()
	if !strings.Contains(v, "primera parte") || strings.Contains(v, "segunda") || !strings.Contains(v, "en vivo") {
		t.Fatalf("se va escribiendo:\n%s", v)
	}
	if p.detalle.Estado.Terminal() {
		t.Fatal("sigue en marcha")
	}
	// El resto, y el final: el detalle pasa a ser el del trabajo terminado.
	b.soltarFin()
	hasta(t, p, func() bool { return p.detalle.Estado == nucleo.Completado && p.canal == nil })
	v = p.Vista()
	if !strings.Contains(v, "primera parte segunda parte") || strings.Contains(v, "en vivo") || !strings.Contains(v, "✓ Completado") {
		t.Fatalf("termina con la respuesta entera:\n%s", v)
	}
	if p.detalle.Salida != "primera parte segunda parte" || p.vivoSalida != "" {
		t.Fatalf("el trabajo final manda: %q %q", p.detalle.Salida, p.vivoSalida)
	}
	cerrado(t, canal)
	// El registro ya lo tiene completo.
	pulsar(p, "right")
	if v := p.Vista(); !strings.Contains(v, "Ejecutando") || !strings.Contains(v, "Validando") || !strings.Contains(v, "Completado") {
		t.Fatalf("el registro del trabajo terminado:\n%s", v)
	}
}

// cerrado espera a que el canal del seguimiento se cierre (su goroutine terminó).
func cerrado(t *testing.T, canal <-chan msgVivo) {
	t.Helper()
	limite := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-canal:
			if !ok {
				return
			}
		case <-limite:
			t.Fatal("el seguimiento en vivo sigue abierto")
		}
	}
}

// TestHistorialEnVivoSeCorta: el seguimiento se corta al cerrar el detalle (Esc), al ocultar la página (Ocultar), al cancelarse
// el contexto de la página y al abrir otro detalle; Mostrar lo reabre.
func TestHistorialEnVivoSeCorta(t *testing.T) {
	cli, id, _, _ := conBloqueado(t)
	p := abierta(t, cli, id)
	pulsar(p, "enter")
	if p.canal == nil {
		t.Fatal("en vivo")
	}
	primero := p.canal
	pulsar(p, "esc")
	cerrado(t, primero)
	if p.canal != nil || p.enVivo() {
		t.Fatal("Esc corta el seguimiento")
	}

	pulsar(p, "enter")
	segundo := p.canal
	p.Ocultar()
	cerrado(t, segundo)
	if p.visible || p.canal != nil {
		t.Fatal("Ocultar corta el seguimiento")
	}
	// Una respuesta que llega con la página oculta no se aplica.
	if _, cmd := p.Update(msgVivo{origen: p.origenActual(), id: p.abierto, seq: p.nflujo, clase: "evento", tipo: "texto", datos: "tarde"}); cmd != nil || p.vivoSalida != "" {
		t.Fatalf("oculta, no hace caso: %q", p.vivoSalida)
	}
	// Mostrar reabre el seguimiento del mismo detalle, con su pestaña.
	pulsar(p, "right")
	correr(p, p.Mostrar())
	if p.detalle == nil || p.canal == nil || p.canal == segundo || p.abierto == "" {
		t.Fatalf("Mostrar reabre el seguimiento: %v %v", p.detalle, p.canal)
	}
	tercero := p.canal
	if p.pestana != pestanaRespuesta {
		t.Fatalf("conserva la pestaña: %d", p.pestana)
	}
	// El contexto de la página (cambio de espacio, reinicio) también lo corta.
	cerrarCtx, cancelar := context.WithCancel(context.Background())
	cli2, id2, _, _ := conBloqueado(t)
	c := contexto(cli2, id2)
	c.Ctx = cerrarCtx
	p2 := nueva("es")
	correr(p2, p2.Iniciar(c))
	pulsar(p2, "enter")
	cuarto := p2.canal
	if cuarto == nil {
		t.Fatal("en vivo")
	}
	cancelar()
	cerrado(t, cuarto)
	// Otro Iniciar cancela el seguimiento de antes.
	correr(p, p.Iniciar(contexto(cli, id)))
	cerrado(t, tercero)
	if p.abierto != "" || p.detalle != nil {
		t.Fatal("Iniciar empieza de cero, sin detalle")
	}
}

// TestHistorialEnVivoReconecta: si el flujo se corta sin que el trabajo termine, se reabre desde el último evento visto.
func TestHistorialEnVivoReconecta(t *testing.T) {
	cli, id, b, tr := conBloqueado(t)
	p := nueva("es")
	var mu sync.Mutex
	var desdes []int
	p.abrir = func(ctx context.Context, inst, trabajoID string, desde int, al func(int, string, string) error) error {
		mu.Lock()
		desdes = append(desdes, desde)
		n := len(desdes)
		mu.Unlock()
		if n == 1 { // el primero manda un evento y se corta
			_ = al(7, "texto", "uno ")
			return errors.New("se cortó")
		}
		return cli.Eventos(ctx, inst, trabajoID, desde, al)
	}
	correr(p, p.Iniciar(contexto(cli, id)))
	t.Cleanup(p.Ocultar)
	pulsar(p, "enter")
	hasta(t, p, func() bool { return strings.HasPrefix(p.vivoSalida, "uno ") })
	hasta(t, p, func() bool { return p.corte })
	if v := p.Vista(); !strings.Contains(v, "reconectando…") {
		t.Fatalf("avisa del corte:\n%s", v)
	}
	hasta(t, p, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(desdes) >= 2 && !p.corte
	})
	mu.Lock()
	if desdes[0] != 0 || desdes[1] != 7 {
		t.Fatalf("reabre desde el último evento visto: %v", desdes)
	}
	mu.Unlock()
	b.soltarPaso()
	b.soltarFin()
	hasta(t, p, func() bool { return p.detalle.Estado == nucleo.Completado })
	if p.detalle.ID != tr.ID {
		t.Fatal("es el mismo trabajo")
	}
}

func TestHistorialReintentar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	original := crear(t, cli, id, "otra vez")
	p := abierta(t, cli, id)
	fuera := pulsar(p, "t")
	if a, ok := aviso(fuera); !ok || a.Texto != "Petición enviada de nuevo" || a.Tipo != comp.Exito {
		t.Fatalf("avisa: %+v", fuera)
	}
	lista, err := cli.Trabajos(ctx, id, 10)
	if err != nil || len(lista) != 2 {
		t.Fatalf("el servicio tiene otro trabajo: %d %v", len(lista), err)
	}
	if p.abierto == "" || p.abierto == original.ID {
		t.Fatalf("abre el trabajo nuevo para verlo: %q", p.abierto)
	}
	if len(ids(p)) != 2 {
		t.Fatalf("y la lista lo trae: %v", ids(p))
	}
	// Una en marcha no se reintenta: se cancela.
	cli2, id2, _, run := conBloqueado(t)
	p2 := abierta(t, cli2, id2)
	if fuera := pulsar(p2, "t"); len(fuera) != 0 {
		t.Fatalf("t no vale en una que sigue en marcha: %v", fuera)
	}
	if lista, _ := cli2.Trabajos(ctx, id2, 10); len(lista) != 1 || lista[0].ID != run.ID {
		t.Fatalf("no creó nada: %v", lista)
	}
}

func TestHistorialCancelar(t *testing.T) {
	cli, id, _, tr := conBloqueado(t)
	p := abierta(t, cli, id)
	var pie []string
	for _, k := range p.Teclas() {
		pie = append(pie, k.Tecla+" "+k.Accion)
	}
	if !slices.Contains(pie, "c Cancelar") || slices.Contains(pie, "t Reintentar") || slices.Contains(pie, "x Eliminar") {
		t.Fatalf("en marcha se ofrece cancelar: %v", pie)
	}
	if fuera := pulsar(p, "x"); len(fuera) != 0 {
		t.Fatalf("x no borra una en marcha: %v", fuera)
	}
	pulsar(p, "enter", "c")
	if p.detalle == nil || (p.detalle.Estado != nucleo.CancelacionSolicitada && p.detalle.Estado != nucleo.Cancelado) {
		t.Fatalf("el detalle toma el estado que devuelve el servicio: %+v", p.detalle)
	}
	hasta(t, p, func() bool { return p.detalle.Estado == nucleo.Cancelado })
	if t2, err := cli.Trabajo(ctx, id, tr.ID); err != nil || t2.Estado != nucleo.Cancelado {
		t.Fatalf("el servicio la cancela: %+v %v", t2, err)
	}
	if v := p.Vista(); !strings.Contains(v, "Cancelado") {
		t.Fatalf("el detalle dice Cancelado:\n%s", v)
	}
}

func TestHistorialEliminar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "se queda")
	borrar := crear(t, cli, id, "se borra")
	p := abierta(t, cli, id)
	if fuera := pulsar(p, "x"); len(fuera) != 1 {
		t.Fatalf("x pregunta: %v", fuera)
	} else if q, ok := fuera[0].(comp.Pregunta); !ok || !strings.Contains(q.Texto, "Eliminar petición") || !strings.Contains(q.Texto, "No se puede deshacer") {
		t.Fatalf("la pregunta: %+v", fuera[0])
	}
	// Cancelar no toca nada.
	p.Update(comp.Cancelado{})
	if _, cmd := p.Update(comp.Confirmado{}); cmd != nil {
		t.Fatal("sin pregunta pendiente no hay nada que borrar")
	}
	if lista, _ := cli.Trabajos(ctx, id, 10); len(lista) != 2 {
		t.Fatalf("seguía todo: %d", len(lista))
	}
	// Confirmar borra la de la fila elegida (la más nueva).
	pulsar(p, "x")
	_, cmd := p.Update(comp.Confirmado{})
	correr(p, cmd)
	lista, _ := cli.Trabajos(ctx, id, 10)
	if len(lista) != 1 || lista[0].ID == borrar.ID {
		t.Fatalf("borró la elegida: %v", lista)
	}
	if got := ids(p); len(got) != 1 || got[0] != lista[0].ID {
		t.Fatalf("la lista se pone al día: %v", got)
	}
	// Desde el detalle: borra la abierta y lo cierra.
	pulsar(p, "enter", "x")
	if p.pend == nil || p.pend.id != lista[0].ID {
		t.Fatalf("recuerda cuál: %+v", p.pend)
	}
	_, cmd = p.Update(comp.Confirmado{})
	correr(p, cmd)
	if p.abierto != "" || len(ids(p)) != 0 {
		t.Fatalf("cierra el detalle y la lista queda vacía: %q %v", p.abierto, ids(p))
	}
}

func TestHistorialCorregir(t *testing.T) {
	cli, id := prueba.Servicio(t)
	mala := crearInvalida(t, cli, id)
	p := abierta(t, cli, id)
	pulsar(p, "enter")
	fuera := pulsar(p, "o")
	if a, ok := aviso(fuera); !ok || a.Texto != "Corrección enviada" || a.Tipo != comp.Exito {
		t.Fatalf("avisa: %+v", fuera)
	}
	lista, _ := cli.Trabajos(ctx, id, 10)
	if len(lista) != 2 || p.abierto == "" || p.abierto == mala.ID {
		t.Fatalf("es otra petición y la abre: %d %q", len(lista), p.abierto)
	}
	// Corregir no es reintentar: el trabajo nuevo lleva la conversación con el mensaje de corrección del planificador.
	nuevo, err := cli.Trabajo(ctx, id, p.abierto)
	if err != nil || len(nuevo.Solicitud.Mensajes) < 2 ||
		!strings.HasPrefix(nuevo.Solicitud.Mensajes[len(nuevo.Solicitud.Mensajes)-1].Contenido, "Tu respuesta no cumple el formato pedido:") {
		t.Fatalf("la corrección termina con el mensaje de corrección: %+v %v", nuevo.Solicitud.Mensajes, err)
	}
	if len(nuevo.Solicitud.Mensajes) <= len(mala.Solicitud.Mensajes) {
		t.Fatalf("y añade mensajes a la solicitud: %d contra %d", len(nuevo.Solicitud.Mensajes), len(mala.Solicitud.Mensajes))
	}
	// Una completada no se corrige: avisa con el texto de la Web UI y no manda nada.
	cli2, id2 := prueba.Servicio(t)
	crear(t, cli2, id2, "bien")
	p2 := abierta(t, cli2, id2)
	fuera = pulsar(p2, "enter", "o")
	if a, ok := aviso(fuera); !ok || a.Texto != "Esta petición no se puede corregir" || a.Tipo != comp.Advertencia {
		t.Fatalf("avisa: %+v", fuera)
	}
	if lista, _ := cli2.Trabajos(ctx, id2, 10); len(lista) != 1 {
		t.Fatalf("no mandó nada: %d", len(lista))
	}
}

func TestHistorialVaciar(t *testing.T) {
	cli, id, b, run := conBloqueado(t)
	crear(t, cli, id, "uno")
	crear(t, cli, id, "dos")
	p := abierta(t, cli, id)
	if len(ids(p)) != 3 {
		t.Fatalf("tres: %v", ids(p))
	}
	fuera := pulsar(p, "V")
	q, ok := pregunta(fuera)
	if !ok || !strings.Contains(q.Texto, "Vaciar el historial") || !strings.Contains(q.Texto, "Las que están en proceso se conservan") {
		t.Fatalf("pregunta antes de borrar: %+v", fuera)
	}
	// Cancelar no borra nada.
	p.Update(comp.Cancelado{})
	pulsar(p, "r")
	if len(ids(p)) != 3 {
		t.Fatalf("cancelar no borra: %v", ids(p))
	}
	// Confirmar borra las terminadas y conserva la que sigue en marcha.
	pulsar(p, "V")
	_, cmd := p.Update(comp.Confirmado{})
	fuera = correr(p, cmd)
	if a, ok := aviso(fuera); !ok || a.Texto != "Peticiones eliminadas: 2" || a.Tipo != comp.Exito {
		t.Fatalf("avisa de cuántas: %+v", fuera)
	}
	if got := ids(p); !slices.Equal(got, []string{run.ID}) {
		t.Fatalf("solo queda la que sigue en marcha: %v", got)
	}
	if lista, _ := cli.Trabajos(ctx, id, 10); len(lista) != 1 {
		t.Fatalf("en el servicio también: %d", len(lista))
	}
	// Sin nada terminado no hay qué vaciar.
	fuera = pulsar(p, "V")
	if a, ok := aviso(fuera); !ok || a.Tipo != comp.Advertencia || a.Texto != "No hay peticiones terminadas" {
		t.Fatalf("avisa: %+v", fuera)
	}
	if _, ok := pregunta(fuera); ok {
		t.Fatal("no pregunta por nada")
	}
	b.soltarPaso()
	b.soltarFin()
}

func TestEscVuelve(t *testing.T) {
	p := conTrabajos(t, trabajo("a", "uno", nucleo.Completado))
	if _, ok := pulsar(p, "esc")[0].(pagina.Volver); !ok {
		t.Fatal("Esc fuera de campo vuelve")
	}
	// Esc cierra la búsqueda y el selector antes de volver.
	pulsar(p, "/")
	if fuera := pulsar(p, "esc"); len(fuera) != 0 || p.EnCampo() {
		t.Fatalf("Esc cierra la búsqueda: %v", fuera)
	}
	pulsar(p, "f")
	_, cmd := p.Update(tecla("esc"))
	if _, ok := cmd().(comp.Cancelado); !ok {
		t.Fatal("Esc en el selector lo cancela")
	}
	p.Update(comp.Cancelado{})
	if p.EnCampo() {
		t.Fatal("el selector se cierra")
	}
	if _, ok := pulsar(p, "esc")[0].(pagina.Volver); !ok {
		t.Fatal("y entonces Esc vuelve")
	}
}

func TestServicioCaido(t *testing.T) {
	// Sin cliente, la página avisa y no se cierra.
	p := nueva("es")
	cmd := p.Iniciar(pagina.Contexto{Ctx: context.Background(), Idioma: "es", Estilos: &pagina.Estilos{Iconos: pagina.IconosUnicode}, Ancho: 60, Alto: 17})
	if e, ok := cmd().(pagina.ErrorServicio); !ok || !errors.Is(e.Err, cliente.ErrServicioInactivo) {
		t.Fatalf("sin cliente: %v", cmd())
	}
	if v := p.Vista(); !strings.Contains(v, "El servicio no responde") {
		t.Fatalf("avisa:\n%s", v)
	}
	// Con el servicio parado a mitad: lo cargado se conserva, el aviso sale y r reintenta.
	cli, id, cerrar := prueba.Arrancar(t)
	crear(t, cli, id, "antes de la caída")
	p = abierta(t, cli, id)
	cerrar()
	fuera := pulsar(p, "r")
	if len(fuera) == 0 {
		t.Fatal("r sin servicio avisa a la app")
	}
	if e, ok := fuera[0].(pagina.ErrorServicio); !ok || !pagina.NoResponde(e.Err) {
		t.Fatalf("ErrorServicio: %+v", fuera[0])
	}
	if v := p.Vista(); !strings.Contains(v, "El servicio no responde") {
		t.Fatalf("la página lo dice y sigue:\n%s", v)
	}
	// Las demás teclas siguen respondiendo, y las acciones avisan sin cerrar nada.
	if fuera := pulsar(p, "e", "l", "esc"); len(fuera) != 1 {
		t.Fatalf("la página sigue viva: %v", fuera)
	}
	crear2 := nueva("es")
	cli3, id3, cerrar3 := prueba.Arrancar(t)
	crear(t, cli3, id3, "hay uno")
	correr(crear2, crear2.Iniciar(contexto(cli3, id3)))
	t.Cleanup(crear2.Ocultar)
	cerrar3()
	fuera = pulsar(crear2, "enter")
	if len(fuera) == 0 {
		t.Fatal("abrir el detalle sin servicio avisa")
	}
	if _, ok := fuera[0].(pagina.ErrorServicio); !ok || crear2.abierto != "" {
		t.Fatalf("avisa y no deja un detalle a medias: %+v %q", fuera, crear2.abierto)
	}
}

// TestRespuestasViejas: una respuesta de otro espacio, de un contexto cancelado, de una consulta superada o de otra versión
// de la página no se aplica.
func TestRespuestasViejas(t *testing.T) {
	p := conTrabajos(t, trabajo("a", "uno", nucleo.Completado))
	nuevaLista := []nucleo.Trabajo{trabajo("n", "nueva", nucleo.Completado)}
	o := p.origenActual()

	otro := o
	otro.instancia = "otro"
	p.Update(msgLista{origen: otro, n: p.n, lista: nuevaLista})
	cancelado, cancelar := context.WithCancel(context.Background())
	cancelar()
	viejoCtx := o
	viejoCtx.ctx = cancelado
	p.Update(msgLista{origen: viejoCtx, n: p.n, lista: nuevaLista})
	p.Update(msgLista{origen: o, n: p.n - 1, lista: nuevaLista})
	viejaGen := o
	viejaGen.gen--
	p.Update(msgLista{origen: viejaGen, n: p.n, lista: nuevaLista})
	if got := ids(p); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("ninguna de las viejas se aplica: %v", got)
	}
	p.Update(msgLista{origen: o, n: p.n, lista: nuevaLista})
	if got := ids(p); !slices.Equal(got, []string{"n"}) {
		t.Fatalf("la vigente sí: %v", got)
	}
	// Un detalle de otro espacio o ya cerrado tampoco abre nada.
	p.Update(msgDetalle{origen: otro, n: p.ndet, t: trabajo("a", "uno", nucleo.Completado)})
	p.Update(msgDetalle{origen: o, n: p.ndet - 1, t: trabajo("a", "uno", nucleo.Completado)})
	if p.detalle != nil {
		t.Fatal("no se abre un detalle viejo")
	}
	// Ni un error de una consulta vieja se enseña.
	p.Update(msgLista{origen: o, n: p.n - 1, err: errors.New("viejo")})
	if p.err != "" {
		t.Fatalf("error viejo: %q", p.err)
	}
}

// TestRespuestaDeLaListaConError: el error de la API se ve tal cual en la página; el de conexión es del servicio.
func TestListaConError(t *testing.T) {
	p := conTrabajos(t, trabajo("a", "uno", nucleo.Completado))
	cmd := p.alLista(msgLista{origen: p.origenActual(), n: p.n, err: nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "limite debe estar entre 1 y 200")})
	if cmd != nil || p.err != "limite debe estar entre 1 y 200" {
		t.Fatalf("el mensaje del servicio sin traducir: %q", p.err)
	}
	if v := p.Vista(); !strings.Contains(v, "limite debe estar entre 1 y 200") || !strings.Contains(v, "uno") {
		t.Fatalf("lo cargado se conserva y el error se ve:\n%s", v)
	}
	cmd = p.alLista(msgLista{origen: p.origenActual(), n: p.n, err: cliente.ErrServicioInactivo})
	if e, ok := cmd().(pagina.ErrorServicio); !ok || !errors.Is(e.Err, cliente.ErrServicioInactivo) {
		t.Fatalf("de conexión: %v", cmd())
	}
	// En segundo plano (el sondeo) un error no se enseña.
	p2 := conTrabajos(t, trabajo("a", "uno", nucleo.Completado))
	if cmd := p2.alLista(msgLista{origen: p2.origenActual(), n: p2.n, segundoPlano: true, err: errors.New("x")}); cmd != nil || p2.err != "" {
		t.Fatalf("sondeo: %q", p2.err)
	}
}

// TestHistorialSondeo: cada 5 s la lista se vuelve a leer, salvo con el detalle abierto.
func TestHistorialSondeo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "primera")
	p := abierta(t, cli, id)
	crear(t, cli, id, "segunda")
	sondear := func() {
		t.Helper()
		_, cmd := p.Update(msgSondeo{p.origenActual()})
		lote, ok := cmd().(tea.BatchMsg) // la lectura y el siguiente sondeo
		if !ok {
			t.Fatalf("sondeo sin lectura: %T", cmd())
		}
		correr(p, lote[0])
	}
	sondear()
	if len(ids(p)) != 2 {
		t.Fatalf("el sondeo trae lo nuevo: %v", ids(p))
	}
	if strings.Contains(p.Vista(), "Cargando…") {
		t.Fatal("el sondeo no enseña «Cargando…»")
	}
	// Con el detalle abierto no recarga.
	pulsar(p, "enter")
	crear(t, cli, id, "tercera")
	_, cmd := p.Update(msgSondeo{p.origenActual()})
	if _, lote := cmd().(tea.BatchMsg); lote {
		t.Fatal("con el detalle abierto el sondeo solo se vuelve a armar")
	}
	pulsar(p, "esc")
	sondear()
	if len(ids(p)) != 3 {
		t.Fatalf("al cerrarlo vuelve a leer: %v", ids(p))
	}
	// Con páginas extra cargadas tampoco: la recarga solo trae la primera.
	p.tam = 2
	p.hayMas = true
	_, cmd = p.Update(msgSondeo{p.origenActual()})
	if _, lote := cmd().(tea.BatchMsg); lote {
		t.Fatal("con más de una página el sondeo no recarga")
	}
}

func TestHistorialOcultarYMostrar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "uno")
	p := abierta(t, cli, id)
	pulsar(p, "e", "/", "u", "enter")
	gen := p.gen
	p.Ocultar()
	if p.gen == gen {
		t.Fatal("Ocultar cambia la versión: lo que estaba en camino ya no vale")
	}
	crear(t, cli, id, "dos")
	correr(p, p.Mostrar())
	if len(ids(p)) != 0 || p.estado != estCurso || p.busqueda != "u" { // el filtro de estado y el texto se conservan
		t.Fatalf("Mostrar conserva los filtros: %v %q %q", ids(p), p.estado, p.busqueda)
	}
	pulsar(p, "e", "e", "e") // todas
	if len(ids(p)) != 1 || !strings.Contains(p.Vista(), "uno") {
		t.Fatalf("Mostrar la pone al día (y solo «uno» contiene la u): %v", ids(p))
	}
	// Iniciar sí empieza de cero.
	correr(p, p.Iniciar(contexto(cli, id)))
	if p.hayFiltro() || len(ids(p)) != 2 {
		t.Fatalf("Iniciar quita los filtros: %v", ids(p))
	}
}

// TestCabeEn60x20: ninguna vista se sale del área, con tildes y con textos largos; la confirmación cabe en 60×20.
func TestCabeEn60x20(t *testing.T) {
	cli, id := prueba.Servicio(t)
	largo := "una pregunta bastante larga con tildes áéíóú y ñandú que no cabe en una sola línea de sesenta columnas, " + strings.Repeat("más texto ", 30)
	crear(t, cli, id, largo)
	crear(t, cli, id, "corta")
	crearInvalida(t, cli, id)
	for _, alto := range []int{17, 20} {
		p := nueva("es")
		c := contexto(cli, id)
		c.Alto = alto
		correr(p, p.Iniciar(c))
		t.Cleanup(p.Ocultar)
		comprobar := func(donde string) {
			t.Helper()
			v := p.Vista()
			if h := lipgloss.Height(v); h > alto {
				t.Fatalf("%s a 60×%d mide %d filas:\n%s", donde, alto, h, v)
			}
			for _, l := range strings.Split(v, "\n") {
				if w := lipgloss.Width(l); w > 60 {
					t.Fatalf("%s a 60×%d: la línea mide %d columnas: %q", donde, alto, w, l)
				}
			}
		}
		comprobar("lista")
		pulsar(p, "down", "down", "enter")
		for range 3 {
			comprobar("detalle")
			pulsar(p, "right")
		}
		pulsar(p, "esc", "f")
		comprobar("selector")
		pulsar(p, "esc")
		p.Update(comp.Cancelado{})
		pulsar(p, "/", "x", "y", "z")
		comprobar("búsqueda")
		pulsar(p, "esc", "e")
		comprobar("filtro")
		p.Update(tea.WindowSizeMsg{Width: 78, Height: 30})
		comprobar("más grande")
		p.Update(tea.WindowSizeMsg{Width: 60, Height: alto})
		comprobar("de vuelta")

		// Las confirmaciones, que pinta la app con comp.Confirmacion.
		for _, tecla := range []string{"V", "x"} {
			p.Update(tea.WindowSizeMsg{Width: 60, Height: alto})
			p.Update(comp.Cancelado{})
			p.quitarFiltros()
			p.lista.Poner(p.trabajos)
			fuera := pulsar(p, tecla)
			q, ok := pregunta(fuera)
			if !ok {
				t.Fatalf("%s pregunta", tecla)
			}
			caja := (&comp.Confirmacion{Texto: q.Texto}).Vista(p.estilos, 60, 20)
			if h := lipgloss.Height(caja); h > 20 {
				t.Fatalf("la confirmación de %s mide %d filas", tecla, h)
			}
			for _, l := range strings.Split(caja, "\n") {
				if w := lipgloss.Width(l); w > 60 {
					t.Fatalf("la confirmación de %s: línea de %d columnas", tecla, w)
				}
			}
		}
	}
}

// TestTextosComoLaWeb: los nombres salen de los textos de la Web UI, en el idioma de la sesión.
func TestTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "hello")
	p := nueva("en")
	c := contexto(cli, id)
	c.Idioma, c.Alto = "en", 30
	correr(p, p.Iniciar(c))
	t.Cleanup(p.Ocultar)
	v := p.Vista()
	for _, quiere := range []string{datos.Texto("en", "menu.historial"), datos.Texto("en", "historial.estado") + ": " + datos.Texto("en", "historial.todos"),
		datos.Texto("en", "historial.origen") + ": " + datos.Texto("en", "historial.todosOrigenes"), datos.Textof("en", "historial.mostrando", "n", "1"),
		datos.Texto("en", "estado.completado")} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	pulsar(p, "enter")
	v = p.Vista()
	for _, quiere := range []string{"[" + datos.Texto("en", "historial.solicitud") + "]", datos.Texto("en", "ejecutar.respuesta"), datos.Texto("en", "registro.titulo"),
		datos.Texto("en", "ruta.cola"), datos.Texto("en", "historial.agenteModelo") + ": ", datos.Texto("en", "historial.identificador") + ": "} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q en el detalle:\n%s", quiere, v)
		}
	}
	// En español, los de la Web UI en español.
	pe := nueva("es")
	correr(pe, pe.Iniciar(contexto(cli, id)))
	t.Cleanup(pe.Ocultar)
	pulsar(pe, "enter")
	if v := pe.Vista(); !strings.Contains(v, "[Mensaje]") || !strings.Contains(v, "Respuesta") || !strings.Contains(v, "Registro") {
		t.Fatalf("pestañas en español:\n%s", v)
	}
	// Y los pies, de los textos de la Web UI.
	pe.Update(tecla("esc"))
	var pie []string
	for _, k := range pe.Teclas() {
		pie = append(pie, k.Tecla+" "+k.Accion)
	}
	for _, quiere := range []string{"e Estado", "f Origen", "t Reintentar", "x Eliminar", "V Vaciar historial"} {
		if !slices.Contains(pie, quiere) {
			t.Fatalf("el pie ofrece %q: %v", quiere, pie)
		}
	}
}

func TestSoloAscii(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "hola")
	crearInvalida(t, cli, id)
	p := nueva("es")
	c := contexto(cli, id)
	c.Estilos = &pagina.Estilos{ASCII: true, Iconos: pagina.IconosASCII}
	correr(p, p.Iniciar(c))
	t.Cleanup(p.Ocultar)
	sinDibujo := func(donde string) {
		t.Helper()
		v := p.Vista()
		for _, r := range v {
			if r > 127 && !strings.ContainsRune("áéíóúüñÁÉÍÓÚÜÑ¿¡×", r) {
				t.Fatalf("%s: sale %q en ASCII:\n%s", donde, r, v)
			}
		}
	}
	sinDibujo("lista")
	pulsar(p, "enter")
	for range 3 {
		sinDibujo("detalle")
		pulsar(p, "right")
	}
	for _, k := range p.Teclas() {
		if strings.ContainsAny(k.Tecla, "←→") {
			t.Fatalf("la tecla %q no es ASCII", k.Tecla)
		}
	}
}

func TestPasaEstado(t *testing.T) {
	todos := []nucleo.Estado{nucleo.EnCola, nucleo.PreparandoEntrada, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.ValidandoSalida,
		nucleo.CancelacionSolicitada, nucleo.Completado, nucleo.Fallido, nucleo.Cancelado, nucleo.Interrumpido}
	quiere := map[string][]nucleo.Estado{
		"todos": todos,
		"curso": todos[:6],
		"ok":    {nucleo.Completado},
		"error": {nucleo.Fallido, nucleo.Interrumpido},
	}
	for filtro, estados := range quiere {
		for _, e := range todos {
			if pasa := pasaEstado(filtro, e); pasa != slices.Contains(estados, e) {
				t.Errorf("pasaEstado(%q, %s) = %v", filtro, e, pasa)
			}
		}
	}
}

func TestHace(t *testing.T) {
	p := nueva("es")
	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	p.ahora = func() time.Time { return ahora }
	casos := map[time.Duration]string{30 * time.Second: "hace 30 s", 5 * time.Minute: "hace 5 min", 3 * time.Hour: "hace 3 h", 48 * time.Hour: "hace 2 d"}
	for d, quiere := range casos {
		if got := p.hace(ahora.Add(-d)); got != quiere {
			t.Errorf("hace(%v) = %q, quería %q", d, got, quiere)
		}
	}
	if got := p.hace(ahora.Add(-30 * 24 * time.Hour)); got != ahora.Add(-30*24*time.Hour).Local().Format("2006-01-02") {
		t.Errorf("más de una semana es la fecha: %q", got)
	}
}

func TestNumeroYTiempo(t *testing.T) {
	es, en := nueva("es"), nueva("en")
	for n, quiere := range map[int][2]string{999: {"999", "999"}, 1000: {"1000", "1,000"}, 12345: {"12.345", "12,345"}, 1234567: {"1.234.567", "1,234,567"}} {
		if es.numero(n) != quiere[0] || en.numero(n) != quiere[1] {
			t.Errorf("numero(%d) = %q y %q", n, es.numero(n), en.numero(n))
		}
	}
	if es.tiempo(250*time.Millisecond) != "250 ms" || es.tiempo(1500*time.Millisecond) != "1,5 s" || en.tiempo(1500*time.Millisecond) != "1.5 s" || es.tiempo(2*time.Second) != "2 s" {
		t.Errorf("tiempo: %q %q %q", es.tiempo(250*time.Millisecond), es.tiempo(1500*time.Millisecond), en.tiempo(1500*time.Millisecond))
	}
}

// TestRegistroComoLaWeb: las líneas del registro dicen lo mismo que RegistroTrabajo.vue: el intento junto a su estado, las
// correcciones, el esfuerzo del agente, el contexto, la entrada y la validación.
func TestRegistroComoLaWeb(t *testing.T) {
	p := conTrabajos(t, trabajo("a", "uno", nucleo.Completado))
	p.estilos = &pagina.Estilos{Iconos: pagina.IconosUnicode}
	inicio := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	ev := func(n int, tipo, datos string, ms int) nucleo.EventoRegistro {
		return nucleo.EventoRegistro{Numero: n, Tipo: tipo, Datos: datos, CreadoEn: inicio.Add(time.Duration(ms) * time.Millisecond)}
	}
	p.registro = []nucleo.EventoRegistro{
		ev(1, "estado", "en_cola", 0), ev(2, "entrada", `{"tokens_estimados":200000,"documentos":[{"nombre":"notas.pdf","tokens_estimados":150000}]}`, 10),
		ev(3, "estado", "ejecutando", 1500), ev(4, "intento", `{"agente":"simulado","modelo":"rapido","esfuerzo":"alto","del_agente":true,"contexto_tokens":200000,"correccion":1,"correcciones":2}`, 1501),
		ev(5, "aviso", "el agente llegó a su límite", 2000), ev(6, "estado", "validando_salida", 3000),
		ev(7, "validacion", `{"problemas":[{"texto":"falta total"},{"texto":"otro"},{"texto":"y otro"}]}`, 3100), ev(8, "estado", "completado", 4000),
	}
	texto := p.textoRegistro(trabajo("a", "uno", nucleo.Completado))
	for _, quiere := range []string{
		"En cola 1,5 s · Ejecución 1,5 s · Validación 1 s", // las etapas arriba
		"+0 ms  En cola", "+10 ms  Entrada: ~200.000 tokens (notas.pdf ~150.000)",
		"+1,5 s  Ejecutando · corrección 1 de 2 · simulado/rapido · alto (del agente) · contexto 200.000",
		"+2 s  Aviso: el agente llegó a su límite", "+3 s  Validando", "No cumple el esquema: falta total (y 2 más)", "+4 s  Completado",
	} {
		if !strings.Contains(texto, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, texto)
		}
	}
	if got := p.esfuerzoUsado(trabajo("a", "uno", nucleo.Completado)); got != "Alto (del agente)" {
		t.Fatalf("esfuerzo del último intento: %q", got)
	}
	p.registro = nil
	t2 := trabajo("a", "uno", nucleo.Completado)
	t2.Solicitud.Esfuerzo = "medio"
	if got := p.esfuerzoUsado(t2); got != "Medio" {
		t.Fatalf("sin intentos, el de la solicitud: %q", got)
	}
	p.errRegistro = true
	if got := p.textoRegistro(t2); got != "No se pudo cargar el registro." {
		t.Fatalf("si falla: %q", got)
	}
}

// TestFlujoDeUnTrabajoTerminado: si el flujo acaba sin fin porque el trabajo terminó mientras tanto, se manda el trabajo final.
func TestSeguirTerminaConElTrabajo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	tr := crear(t, cli, id, "ya terminó")
	canal := make(chan msgVivo, 8)
	c := contexto(cli, id)
	abrir := func(ctx context.Context, inst, trabajoID string, desde int, al func(int, string, string) error) error {
		_ = al(3, "estado", "completado")
		return nil // el flujo se cierra sin decirlo
	}
	seguir(ctx, origen{ctx: ctx, instancia: id}, c, tr.ID, 1, abrir, func(time.Duration) <-chan time.Time { return nil }, canal)
	var clases []string
	for m := range canal {
		clases = append(clases, m.clase)
		if m.clase == "fin" && m.t.ID != tr.ID {
			t.Fatalf("el fin lleva el trabajo: %+v", m.t)
		}
	}
	if !slices.Equal(clases, []string{"evento", "fin"}) {
		t.Fatalf("un evento y el fin: %v", clases)
	}
}

// TestDetalleParteLasPalabras: el texto del detalle se parte por palabras al ancho de la página, y sigue al redimensionar.
func TestDetalleParteLasPalabras(t *testing.T) {
	p := conTrabajos(t, trabajo("a", strings.Repeat("palabra ", 40), nucleo.Completado))
	t.Cleanup(p.Ocultar)
	pulsar(p, "enter")
	p.Update(msgDetalle{origen: p.origenActual(), n: p.ndet, t: trabajo("a", strings.Repeat("palabra ", 40), nucleo.Completado)})
	for _, ancho := range []int{60, 40, 60} {
		p.Update(tea.WindowSizeMsg{Width: ancho, Height: 17})
		lineas := strings.Split(p.Vista(), "\n")
		if len(lineas) > 17 {
			t.Fatalf("a %d mide %d filas", ancho, len(lineas))
		}
		for i, l := range lineas {
			if lipgloss.Width(l) > ancho {
				t.Fatalf("a %d la línea %d mide %d: %q", ancho, i, lipgloss.Width(l), l)
			}
			if i >= 3 {
				for _, campo := range strings.Fields(l) {
					if strings.Contains(l, "palabra") && campo != "palabra" && !strings.HasSuffix(campo, ":") && !strings.HasPrefix(campo, "Agente") {
						t.Fatalf("a %d se partió una palabra: %q", ancho, l)
					}
				}
			}
		}
	}
}

// conDetalle deja abierto el detalle de un trabajo en marcha con el seguimiento número 2, sin pedir nada al servicio.
func conDetalle(t *testing.T) *Pagina {
	t.Helper()
	p := conTrabajos(t, trabajo("a", "uno", nucleo.Ejecutando))
	tr := trabajo("a", "uno", nucleo.Ejecutando)
	p.abierto, p.detalle, p.nflujo, p.nreg, p.ndet = "a", &tr, 2, 3, 4
	return p
}

// TestRespuestasViejasDelDetalle: lo que llega del seguimiento en vivo, del registro o de una acción, con el seguimiento, la
// consulta, el trabajo, el espacio o la versión de la página de antes, no se aplica.
func TestRespuestasViejasDelDetalle(t *testing.T) {
	texto := func(o origen, id string, seq int) msgVivo {
		return msgVivo{origen: o, id: id, seq: seq, clase: "evento", tipo: "texto", datos: "x"}
	}
	p := conDetalle(t)
	o := p.origenActual()
	otroEspacio, otraGen := o, o
	otroEspacio.instancia = "otro"
	otraGen.gen--
	for nombre, m := range map[string]msgVivo{"seguimiento viejo": texto(o, "a", 1), "otro trabajo": texto(o, "b", 2),
		"otro espacio": texto(otroEspacio, "a", 2), "otra versión": texto(otraGen, "a", 2)} {
		p.Update(m)
		if p.vivoSalida != "" {
			t.Fatalf("msgVivo de %s se aplicó: %q", nombre, p.vivoSalida)
		}
	}
	p.Update(texto(o, "a", 2))
	if p.vivoSalida != "x" {
		t.Fatalf("el vigente sí: %q", p.vivoSalida)
	}
	// El fin de un seguimiento viejo no cambia el detalle.
	final := trabajo("a", "uno", nucleo.Completado)
	p.Update(msgVivo{origen: o, id: "a", seq: 1, clase: "fin", t: final})
	if p.detalle.Estado != nucleo.Ejecutando {
		t.Fatalf("un fin viejo cambió el detalle: %s", p.detalle.Estado)
	}

	// El registro.
	reg := []nucleo.EventoRegistro{{Numero: 1, Tipo: "estado", Datos: "en_cola"}}
	for nombre, m := range map[string]msgRegistro{"consulta vieja": {origen: o, n: 2, id: "a", lista: reg}, "otro trabajo": {origen: o, n: 3, id: "b", lista: reg},
		"otro espacio": {origen: otroEspacio, n: 3, id: "a", lista: reg}, "otra versión": {origen: otraGen, n: 3, id: "a", lista: reg}} {
		p.Update(m)
		if p.registro != nil {
			t.Fatalf("msgRegistro de %s se aplicó", nombre)
		}
	}
	p.Update(msgRegistro{origen: o, n: 2, id: "a", err: errors.New("viejo")})
	if p.errRegistro {
		t.Fatal("un error viejo del registro no se enseña")
	}
	p.Update(msgRegistro{origen: o, n: 3, id: "a", lista: reg})
	if len(p.registro) != 1 {
		t.Fatal("el vigente sí")
	}

	// Una acción de otro espacio o de otra versión no suelta el ocupado ni cierra el detalle.
	for nombre, oa := range map[string]origen{"otro espacio": otroEspacio, "otra versión": otraGen} {
		p.ocupado = true
		p.Update(msgAccion{origen: oa, accion: accionEliminar, id: "a"})
		if p.abierto != "a" || !p.ocupado {
			t.Fatalf("msgAccion de %s se aplicó", nombre)
		}
	}
	p.Update(msgAccion{origen: o, accion: accionEliminar, id: "a"})
	if p.abierto != "" || p.ocupado {
		t.Fatal("la vigente sí cierra el detalle")
	}
	// Ni un detalle de otra versión.
	p.Update(msgDetalle{origen: otraGen, n: p.ndet, t: trabajo("a", "uno", nucleo.Completado)})
	if p.detalle != nil {
		t.Fatal("un detalle de otra versión no se abre")
	}
}

// TestVivoViejoNoRearmaLaLectura: un mensaje de un seguimiento ya cortado no vuelve a esperar sobre el canal nuevo: si no,
// quedan dos lectores y el texto en vivo puede llegar desordenado.
func TestVivoViejoNoRearmaLaLectura(t *testing.T) {
	p := conDetalle(t)
	p.canal = make(chan msgVivo) // el seguimiento de ahora
	o := p.origenActual()
	viejo := msgVivo{origen: o, id: "a", seq: 1, clase: "evento", tipo: "texto", datos: "x"}
	if _, cmd := p.Update(viejo); cmd != nil {
		t.Fatal("un mensaje del seguimiento viejo no espera sobre el canal nuevo")
	}
	otroTrabajo := msgVivo{origen: o, id: "b", seq: 2, clase: "evento", tipo: "texto", datos: "x"}
	if _, cmd := p.Update(otroTrabajo); cmd != nil {
		t.Fatal("ni uno de otro trabajo")
	}
	oculta := o
	oculta.gen--
	if _, cmd := p.Update(msgVivo{origen: oculta, id: "a", seq: 2, clase: "evento", tipo: "texto", datos: "x"}); cmd != nil {
		t.Fatal("ni uno de otra versión de la página")
	}
	if _, cmd := p.Update(msgVivo{origen: o, id: "a", seq: 2, clase: "evento", tipo: "texto", datos: "x"}); cmd == nil {
		t.Fatal("el del seguimiento de ahora sí sigue leyendo")
	}
	// Un msgFlujo viejo tampoco toma el canal.
	p.cancelar = func() {}
	p.canal = nil
	if _, cmd := p.Update(msgFlujo{origen: o, id: "a", seq: 1, canal: make(chan msgVivo)}); cmd != nil || p.canal != nil {
		t.Fatal("un flujo viejo no toma el canal")
	}
}
