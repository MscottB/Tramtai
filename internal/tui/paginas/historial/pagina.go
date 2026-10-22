// Package historial es la página Historial de la terminal: las peticiones del espacio con sus filtros (estado, origen, API,
// grupo y texto), «Ver más», el detalle de cada una en un visor con tres pestañas (mensaje, respuesta y registro), su
// seguimiento en vivo si sigue en marcha y las acciones de Trabajos.vue: reintentar, corregir, cancelar, eliminar y vaciar.
package historial

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

const (
	// tamPagina son las peticiones que se piden cada vez (PAGINA de la Web UI); el máximo que admite el servicio.
	tamPagina = 200
	// cadaSondeo es cada cuánto se vuelve a leer la lista mientras se ve (sin detalle abierto ni páginas extra), como la Web UI.
	cadaSondeo = 5 * time.Second
	// esperaRegistro junta los avisos seguidos de un mismo trabajo en una sola lectura del registro.
	esperaRegistro = 200 * time.Millisecond
)

// Los estados del filtro, en el orden del selector de la Web UI: todas, en proceso, completadas y con error.
const (
	estTodos = "todos"
	estCurso = "curso"
	estOk    = "ok"
	estError = "error"
)

var estadosFiltro = []string{estTodos, estCurso, estOk, estError}

// pasaEstado dice qué estados entran en cada filtro (la cuenta de `filas` de Trabajos.vue): en proceso es todo lo que no
// terminó, completadas solo `completado` y con error `fallido` e `interrumpido`; los cancelados solo salen en todas.
func pasaEstado(filtro string, e nucleo.Estado) bool {
	switch filtro {
	case estCurso:
		return !e.Terminal()
	case estOk:
		return e == nucleo.Completado
	case estError:
		return e == nucleo.Fallido || e == nucleo.Interrumpido
	}
	return true
}

// origen es lo que cada mensaje propio lleva para saber si aún vale: el contexto y el espacio con que se pidió y la versión de
// la página (cada Iniciar, Ocultar o Mostrar la cambia).
type origen struct {
	ctx       context.Context
	instancia string
	gen       int
}

type (
	// msgLista es una página de la lista: la primera (n es el número de la consulta; solo vale la última) o, con mas, la
	// siguiente. segundoPlano es el sondeo: no avisa de errores.
	msgLista struct {
		origen
		n                 int
		mas, segundoPlano bool
		lista             []nucleo.Trabajo
		err               error
	}
	msgPuntos struct {
		origen
		lista []contrato.EstadoPunto
		err   error
	}
	msgDetalle struct {
		origen
		n   int
		t   nucleo.Trabajo
		err error
	}
	msgRegistro struct {
		origen
		n     int
		id    string
		lista []nucleo.EventoRegistro
		err   error
	}
	msgRegistroYa struct{ origen } // pasó esperaRegistro desde el último aviso del trabajo: se vuelve a leer el registro
	msgAccion     struct {
		origen
		accion, id string
		t          nucleo.Trabajo
		n          int // en vaciar, cuántas se eliminaron
		err        error
	}
	msgSondeo struct{ origen }
	// msgFlujo es el seguimiento en vivo recién abierto; msgVivo, lo que llega por él (con el origen del Historial).
	msgFlujo = vivo.Flujo[origen]
	msgVivo  = vivo.Msg[origen]
)

// pendiente es lo que la página va a hacer si el usuario confirma: eliminar un trabajo o vaciar el historial.
type pendiente struct{ accion, id string }

const (
	accionReintentar = "reintentar"
	accionCorregir   = "corregir"
	accionCancelar   = "cancelar"
	accionEliminar   = "eliminar"
	accionVaciar     = "vaciar"
)

// Pagina es la página Historial.
type Pagina struct {
	c       pagina.Contexto
	lengua  string
	estilos *pagina.Estilos
	gen     int
	visible bool

	lista       comp.Lista[nucleo.Trabajo]
	visor       comp.Visor
	trabajos    []nucleo.Trabajo // los cargados, del más nuevo al más viejo
	puntos      []contrato.EstadoPunto
	cargado     bool
	sinServ     bool // el servicio no responde
	err         string
	cargando    bool // hay una lista pedida que se espera (no el sondeo)
	cargandoMas bool
	enVuelo     int // consultas de la lista en marcha: el sondeo y Ver más no se apilan
	n           int // la última consulta de la lista: solo vale su respuesta
	hayMas      bool
	tam         int

	// Los filtros. origenF es '' (todos), «o:probar», «g:web» (grupo) o «p:mi-app» (API): lo filtra el servicio; el estado
	// y el texto se filtran aquí, como en la Web UI.
	estado   string
	origenF  string
	busqueda string
	buscando bool
	entrada  string // lo que se escribe en la búsqueda abierta
	antes    string // la búsqueda de antes de abrirla, para Esc
	enVisor  bool   // la búsqueda abierta es la del detalle, no la de la lista

	selector *comp.Selector
	pend     *pendiente
	ocupado  bool // hay una acción en marcha

	// El detalle: abierto es el id del trabajo abierto (con detalle nil mientras llega).
	abierto     string
	detalle     *nucleo.Trabajo
	ndet        int
	pestana     int // 0 mensaje, 1 respuesta, 2 registro
	registro    []nucleo.EventoRegistro
	cargaReg    bool // el registro está pedido y aún no llegó
	errRegistro bool
	nreg        int
	regPend     bool // ya hay una lectura del registro programada
	vivoSalida  string
	sigue       bool // la respuesta en vivo se desplaza sola hasta que se usa el visor
	canal       <-chan msgVivo
	nflujo      int // el seguimiento en vivo de ahora
	cancelar    context.CancelFunc
	corte       bool

	// Lo que cambian las pruebas: el reloj, las esperas y el flujo de un trabajo.
	ahora      func() time.Time
	esperaPara func(time.Duration) <-chan time.Time
	abrir      vivo.Eventos
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina {
	return &Pagina{lengua: lengua, tam: tamPagina, estado: estTodos, ahora: time.Now, esperaPara: time.After}
}

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.historial") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaHistorial }

// EnCampo es verdadero mientras se escribe una búsqueda o se elige un origen: las letras son del campo.
func (p *Pagina) EnCampo() bool { return p.buscando || p.selector != nil }

// tx es un texto de la Web UI (datos.Texto) en el idioma de la sesión, con sus {marcadores} rellenos.
func (p *Pagina) tx(clave string, pares ...string) string {
	return p.a(datos.Textof(p.lengua, clave, pares...))
}

var textoAscii = strings.NewReplacer("·", "-", "…", "...", "’", "'", "“", "\"", "”", "\"", "—", "-", "→", "->", "‹", "<", "›", ">")

// a deja un texto sin los caracteres de dibujo que usan los textos de la Web UI cuando la consola es ASCII.
func (p *Pagina) a(texto string) string {
	if p.estilos != nil && p.estilos.ASCII {
		return textoAscii.Replace(texto)
	}
	return texto
}

func (p *Pagina) ascii() bool { return p.estilos != nil && p.estilos.ASCII }

func (p *Pagina) hayFiltro() bool { return p.estado != estTodos || p.busqueda != "" || p.origenF != "" }

// Iniciar empieza de cero: «Cargando…» hasta que llegue la primera página, con los filtros por defecto.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	p.c, p.lengua = c, c.Idioma
	if p.estilos != c.Estilos { // los mismos toda la sesión: la lista y el visor se crean una vez
		p.estilos = c.Estilos
		p.lista = comp.NuevaLista(c.Estilos, func(t nucleo.Trabajo) string { return textoDe(t, p.lengua) + " " + t.ID })
		p.lista.Clave = func(t nucleo.Trabajo) string { return t.ID }
		p.lista.Fila = p.fila
		p.lista.Filtrar(func(t nucleo.Trabajo) bool { return pasaEstado(p.estado, t.Estado) })
		p.visor = comp.NuevoVisor(c.Estilos)
	}
	p.gen++
	p.visible = true
	p.cerrarVivo()
	p.trabajos, p.puntos, p.hayMas = nil, nil, false
	p.cargado, p.sinServ, p.err, p.cargando, p.cargandoMas, p.enVuelo = false, false, "", false, false, 0
	p.estado, p.origenF, p.busqueda, p.buscando, p.entrada, p.enVisor = estTodos, "", "", false, "", false
	p.selector, p.pend, p.ocupado = nil, nil, false
	p.cerrarDetalle()
	p.lista.Buscar("")
	p.lista.Poner(nil)
	p.ajustar()
	if c.Cli == nil {
		p.sinServ = true
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	return tea.Batch(p.cargar(false), p.cargarPuntos(), p.armarSondeo())
}

// Ocultar suelta el seguimiento en vivo y el sondeo: la página no gasta nada mientras no se ve.
func (p *Pagina) Ocultar() {
	p.visible = false
	p.gen++
	p.enVuelo, p.cargando, p.cargandoMas, p.ocupado, p.regPend = 0, false, false, false, false
	p.cerrarVivo()
}

// Mostrar la recupera al volver (de la Ayuda, por ejemplo) sin iniciarla de nuevo: los filtros y el detalle abierto se
// conservan y se ponen al día.
func (p *Pagina) Mostrar() tea.Cmd {
	p.gen++
	p.visible = true
	if p.c.Cli == nil {
		return nil
	}
	cmds := []tea.Cmd{p.cargar(true), p.cargarPuntos(), p.armarSondeo()}
	if p.abierto != "" {
		cmds = append(cmds, p.abrirDetalle(p.abierto, false))
	}
	return tea.Batch(cmds...)
}

// ajustar da al visor el espacio del detalle: la página menos la cabecera (estado, ruta y pestañas).
func (p *Pagina) ajustar() {
	if p.estilos != nil {
		p.visor.Tamano(max(p.c.Ancho, 1), max(p.c.Alto-3, 1))
	}
}

func (p *Pagina) origen() origen { return origen{ctx: p.c.Ctx, instancia: p.c.Instancia, gen: p.gen} }

// vigente dice si una respuesta sigue valiendo: su contexto no se canceló, es del mismo espacio y de esta versión de la página.
func (p *Pagina) vigente(o origen) bool {
	return o.ctx != nil && o.ctx.Err() == nil && o.instancia == p.c.Instancia && o.gen == p.gen && p.visible
}

// esperarA es un temporizador: pasado d manda lo que haga mensaje, salvo que el contexto de la página se cancele antes.
func (p *Pagina) esperarA(d time.Duration, mensaje func(origen) tea.Msg) tea.Cmd {
	o, espera := p.origen(), p.esperaPara
	return func() tea.Msg {
		select {
		case <-o.ctx.Done():
			return nil
		case <-espera(d):
			return mensaje(o)
		}
	}
}

func (p *Pagina) armarSondeo() tea.Cmd {
	return p.esperarA(cadaSondeo, func(o origen) tea.Msg { return msgSondeo{o} })
}

// filtroServidor son los filtros que atiende el servicio: el origen, el grupo o la API.
func (p *Pagina) filtroServidor() cliente.FiltroTrabajos {
	f := cliente.FiltroTrabajos{Limite: p.tam}
	if len(p.origenF) > 2 {
		switch nombre := p.origenF[2:]; p.origenF[:2] {
		case "o:":
			f.Origen = nombre
		case "g:":
			f.Grupo = nombre
		case "p:":
			f.Punto = nombre
		}
	}
	return f
}

// cargar pide la primera página con los filtros de ahora. En segundo plano (el sondeo) no enseña «Cargando…» ni avisa de
// errores. Solo vale la respuesta de la última consulta.
func (p *Pagina) cargar(segundoPlano bool) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	p.n++
	n := p.n
	p.cargandoMas = false
	p.cargando = !segundoPlano
	p.enVuelo++
	f := p.filtroServidor()
	return func() tea.Msg {
		l, err := c.Cli.TrabajosFiltrados(c.Ctx, c.Instancia, f)
		return msgLista{origen: o, n: n, segundoPlano: segundoPlano, lista: l, err: err}
	}
}

// verMas pide la página siguiente: los creados antes del último que ya se ve.
func (p *Pagina) verMas() tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil || len(p.trabajos) == 0 || p.enVuelo > 0 {
		return nil
	}
	ultimo := p.trabajos[len(p.trabajos)-1]
	f := p.filtroServidor()
	f.Antes, f.AntesID = ultimo.CreadoEn, ultimo.ID
	n := p.n // si el filtro cambia mientras carga, esta página ya no es de la lista que se ve
	p.cargandoMas = true
	p.enVuelo++
	return func() tea.Msg {
		l, err := c.Cli.TrabajosFiltrados(c.Ctx, c.Instancia, f)
		return msgLista{origen: o, n: n, mas: true, lista: l, err: err}
	}
}

// cargarPuntos pide las APIs del espacio: son las opciones del filtro de origen. Sin ellas el filtro sigue sirviendo.
func (p *Pagina) cargarPuntos() tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return nil
	}
	return func() tea.Msg {
		r, err := c.Cli.Puntos(c.Ctx, c.Instancia)
		return msgPuntos{origen: o, lista: r.Puntos, err: err}
	}
}

// fallo reparte un error: de conexión es del servicio (lo avisa la app); el de la API, el mensaje tal cual, en la página
// (la carga de la lista) o como aviso (una acción).
func (p *Pagina) fallo(err error, aviso bool) tea.Cmd {
	if pagina.NoResponde(err) {
		p.sinServ = true
		return func() tea.Msg { return pagina.ErrorServicio{Err: err} }
	}
	if aviso {
		return comp.Aviso(nucleo.MensajeDe(err), comp.Fallo)
	}
	p.err = nucleo.MensajeDe(err)
	return nil
}

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
		p.ajustar()
		p.rehacerVisor() // el texto se parte por palabras al ancho de ahora
	case pagina.FiltroPunto:
		// Una API por su nombre (Mis APIs) o un filtro ya armado («o:probar» desde Probar), como origenHistorial.
		p.origenF = "p:" + m.Nombre
		if strings.Contains(m.Nombre, ":") {
			p.origenF = m.Nombre
		}
		return p, p.cargar(false)
	case pagina.FiltroEstado:
		p.estado = estTodos
		if slices.Contains(estadosFiltro, m.Estado) {
			p.estado = m.Estado
		}
		p.lista.Filtrar(func(t nucleo.Trabajo) bool { return pasaEstado(p.estado, t.Estado) })
	case msgSondeo:
		if !p.vigente(m.origen) {
			return p, nil
		}
		siguiente := p.armarSondeo()
		// Con el detalle abierto, con páginas extra cargadas o con otra consulta en marcha, el sondeo no recarga.
		if p.abierto != "" || p.sinServ || !p.cargado || p.enVuelo > 0 || len(p.trabajos) > p.tam {
			return p, siguiente
		}
		return p, tea.Batch(p.cargar(true), siguiente)
	case msgLista:
		return p, p.alLista(m)
	case msgPuntos:
		if p.vigente(m.origen) && m.err == nil { // sin las APIs el filtro de origen sigue sirviendo
			p.puntos = m.lista
		}
	case msgDetalle:
		return p, p.alDetalle(m)
	case msgRegistro:
		p.alRegistro(m)
	case msgRegistroYa:
		if p.vigente(m.origen) {
			p.regPend = false
			return p, p.cargarRegistro()
		}
	case msgFlujo:
		if p.vigente(m.Origen) && m.ID == p.abierto && m.Seq == p.nflujo && p.cancelar != nil {
			p.canal = m.Canal
			return p, vivo.Esperar(m.Canal)
		}
	case msgVivo:
		if !p.vivoVigente(m) { // uno viejo no vuelve a esperar: quedarían dos lectores sobre el canal nuevo
			return p, nil
		}
		cmd := p.alVivo(m)
		if p.canal == nil {
			return p, cmd
		}
		return p, tea.Batch(vivo.Esperar(p.canal), cmd)
	case msgAccion:
		return p, p.alAccion(m)
	case comp.Confirmado:
		return p, p.confirmado()
	case comp.Cancelado:
		if p.selector != nil {
			p.selector = nil
		} else {
			p.pend = nil
		}
	case comp.Elegido:
		if p.selector != nil {
			p.selector = nil
			p.origenF = m.Valor
			return p, p.cargar(false)
		}
	case tea.PasteMsg:
		if p.buscando {
			p.entrada += strings.Join(strings.Fields(m.Content), " ")
			p.aplicarEntrada()
		}
	case tea.KeyPressMsg:
		return p, p.tecla(m)
	}
	return p, nil
}

// alLista aplica una página de la lista.
func (p *Pagina) alLista(m msgLista) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	p.enVuelo = max(p.enVuelo-1, 0)
	if m.n != p.n {
		return nil
	}
	if m.mas {
		p.cargandoMas = false
	} else {
		p.cargando = false
	}
	if m.err != nil {
		if m.segundoPlano {
			return nil
		}
		return p.fallo(m.err, m.mas)
	}
	p.sinServ, p.err = false, ""
	if m.mas {
		p.trabajos = append(slices.Clone(p.trabajos), m.lista...)
	} else {
		p.cargado, p.trabajos = true, m.lista
	}
	p.hayMas = len(m.lista) == p.tam
	p.lista.Poner(p.trabajos)
	return nil
}

// cerrarVivo corta el seguimiento en vivo (la goroutine termina y cierra su canal).
func (p *Pagina) cerrarVivo() {
	if p.cancelar != nil {
		p.cancelar()
		p.cancelar = nil
	}
	p.canal, p.corte = nil, false
}

// abrirDetalle pide el trabajo completo (el de la lista no trae todo) y abre el detalle cuando llega. Con nuevo, el detalle
// empieza en la primera pestaña; si no, es el mismo que ya se ve y conserva la suya.
func (p *Pagina) abrirDetalle(id string, nuevo bool) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	if nuevo || p.abierto != id {
		p.detalle = nil
	}
	p.abierto = id
	p.ndet++
	n := p.ndet
	return func() tea.Msg {
		t, err := c.Cli.Trabajo(c.Ctx, c.Instancia, id)
		return msgDetalle{origen: o, n: n, t: t, err: err}
	}
}

// cerrarDetalle cierra el detalle y corta lo que tenía en marcha.
func (p *Pagina) cerrarDetalle() {
	p.cerrarVivo()
	p.abierto, p.detalle, p.registro, p.errRegistro, p.cargaReg, p.regPend = "", nil, nil, false, false, false
	p.vivoSalida, p.pestana = "", 0
	p.ndet++ // lo que esté en camino de un detalle cerrado no vale
	p.nreg++
}

func (p *Pagina) alDetalle(m msgDetalle) tea.Cmd {
	if !p.vigente(m.origen) || m.n != p.ndet {
		return nil
	}
	if m.err != nil {
		p.cerrarDetalle()
		return p.fallo(m.err, true)
	}
	t := m.t
	if p.detalle == nil || p.detalle.ID != t.ID {
		p.pestana, p.vivoSalida, p.registro, p.errRegistro, p.sigue = 0, "", nil, false, true
	}
	p.detalle = &t
	p.ajustar()
	p.rehacerVisor()
	cmds := []tea.Cmd{p.cargarRegistro()}
	if t.Estado.Terminal() {
		p.cerrarVivo()
	} else {
		cmds = append(cmds, p.abrirVivo(t.ID))
	}
	return tea.Batch(cmds...)
}

// cargarRegistro lee los pasos del trabajo abierto.
func (p *Pagina) cargarRegistro() tea.Cmd {
	c, o, id := p.c, p.origen(), p.abierto
	if c.Cli == nil || id == "" {
		return nil
	}
	p.nreg++
	n := p.nreg
	p.cargaReg = true
	return func() tea.Msg {
		l, err := c.Cli.Registro(c.Ctx, c.Instancia, id)
		return msgRegistro{origen: o, n: n, id: id, lista: l, err: err}
	}
}

func (p *Pagina) alRegistro(m msgRegistro) {
	if !p.vigente(m.origen) || m.n != p.nreg || m.id != p.abierto {
		return
	}
	p.cargaReg = false
	p.errRegistro = m.err != nil
	if m.err == nil {
		p.registro = m.lista
	}
	p.rehacerVisor()
}

// abrirVivo sigue los eventos del trabajo en una goroutine que los manda por un canal; la página los lee de uno en uno. Se
// corta con el contexto de la página, al cerrar el detalle y al ocultarse.
func (p *Pagina) abrirVivo(id string) tea.Cmd {
	p.cerrarVivo()
	p.vivoSalida = ""
	p.nflujo++
	ctx, cancelar := context.WithCancel(p.c.Ctx)
	p.cancelar = cancelar
	eventos := p.abrir
	if eventos == nil {
		eventos = p.c.Cli.Eventos
	}
	return vivo.Abrir(ctx, p.origen(), p.c.Instancia, id, p.nflujo, eventos, p.c.Cli.Trabajo, p.esperaPara)
}

// recibirVivo aplica lo que llega del seguimiento en vivo si sigue valiendo: es del detalle y del seguimiento de ahora.
func (p *Pagina) recibirVivo(m msgVivo) tea.Cmd {
	if !p.vivoVigente(m) {
		return nil
	}
	return p.alVivo(m)
}

// vivoVigente dice si un mensaje del seguimiento es del detalle y del seguimiento de ahora.
func (p *Pagina) vivoVigente(m msgVivo) bool {
	return p.vigente(m.Origen) && m.ID == p.abierto && m.Seq == p.nflujo
}

// alVivo aplica un evento del trabajo abierto, como los de Ejecutar.vue: el texto se suma a la respuesta, una corrección la
// empieza de nuevo, el estado cambia y los demás avisos piden releer el registro. Devuelve lo que haya que pedir.
func (p *Pagina) alVivo(m msgVivo) tea.Cmd {
	switch m.Clase {
	case "corte":
		p.corte = true
		return nil
	case "abre":
		p.corte = false
		return nil
	case "fin":
		t := m.T
		p.detalle = &t
		p.vivoSalida = ""
		p.cerrarVivo()
		p.rehacerVisor()
		return tea.Batch(p.cargarRegistro(), p.cargar(true))
	}
	p.corte = false
	var refresco tea.Cmd
	switch m.Tipo {
	case "texto":
		p.vivoSalida += m.Datos
	case "intento":
		var i struct {
			Correccion int `json:"correccion"`
		}
		if json.Unmarshal([]byte(m.Datos), &i) == nil && i.Correccion != 0 {
			p.vivoSalida = "" // una corrección vuelve a escribir la respuesta desde cero
		}
		refresco = p.pedirRegistro()
	case "estado":
		if p.detalle != nil {
			p.detalle.Estado = nucleo.Estado(m.Datos)
		}
		refresco = p.pedirRegistro()
	case "aviso", "validacion":
		refresco = p.pedirRegistro()
	}
	p.rehacerVisor()
	return refresco
}

// pedirRegistro programa una lectura del registro; los avisos que lleguen antes de que pase se juntan en ella.
func (p *Pagina) pedirRegistro() tea.Cmd {
	if p.regPend {
		return nil
	}
	p.regPend = true
	return p.esperarA(esperaRegistro, func(o origen) tea.Msg { return msgRegistroYa{o} })
}

// lanzar hace una acción en el servicio; al volver, alAccion la recoge. Mientras haya una en marcha no se lanza otra.
func (p *Pagina) lanzar(hacer func(pagina.Contexto) msgAccion) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	if p.ocupado {
		return nil
	}
	p.ocupado = true
	return func() tea.Msg {
		m := hacer(c)
		m.origen = o
		return m
	}
}

// accion reintenta, corrige o cancela un trabajo. Reintentar y corregir crean otro, que se sigue en Probar.
func (p *Pagina) accion(nombre string, t nucleo.Trabajo) tea.Cmd {
	return p.lanzar(func(c pagina.Contexto) msgAccion {
		var nuevo nucleo.Trabajo
		var err error
		if nombre == accionCorregir {
			nuevo, err = c.Cli.Corregir(c.Ctx, c.Instancia, t.ID)
		} else {
			nuevo, err = c.Cli.Accion(c.Ctx, c.Instancia, t.ID, nombre)
		}
		return msgAccion{accion: nombre, id: t.ID, t: nuevo, err: err}
	})
}

func (p *Pagina) alAccion(m msgAccion) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	p.ocupado = false
	if m.err != nil {
		return p.fallo(m.err, true)
	}
	switch m.accion {
	case accionReintentar, accionCorregir:
		aviso := idioma.T("Petición enviada de nuevo")
		if m.accion == accionCorregir {
			aviso = idioma.T("Corrección enviada")
		}
		// El trabajo nuevo se sigue en Probar, como trabajoSeguido de la Web UI.
		ir := pagina.IrA{Pagina: pagina.PaginaEjecutar, Datos: pagina.SeguirTrabajo{ID: m.t.ID}}
		return tea.Batch(comp.Aviso(aviso, comp.Exito), func() tea.Msg { return ir }, p.cargar(true))
	case accionCancelar:
		if p.detalle != nil && p.detalle.ID == m.id {
			t := m.t
			p.detalle = &t
			p.rehacerVisor()
		}
		return p.cargar(true)
	case accionEliminar:
		if p.abierto == m.id {
			p.cerrarDetalle()
		}
		return p.cargar(true)
	case accionVaciar:
		return tea.Batch(comp.Aviso(p.tx("historial.vaciado", "n", itoa(m.n)), comp.Exito), p.cargar(true))
	}
	return nil
}

// confirmado hace lo que se había preguntado: eliminar un trabajo o vaciar el historial.
func (p *Pagina) confirmado() tea.Cmd {
	pend := p.pend
	p.pend = nil
	if pend == nil {
		return nil
	}
	switch pend.accion {
	case accionEliminar:
		return p.lanzar(func(c pagina.Contexto) msgAccion {
			return msgAccion{accion: accionEliminar, id: pend.id, err: c.Cli.Eliminar(c.Ctx, c.Instancia, pend.id)}
		})
	case accionVaciar:
		return p.lanzar(func(c pagina.Contexto) msgAccion {
			n, err := c.Cli.VaciarTrabajos(c.Ctx, c.Instancia)
			return msgAccion{accion: accionVaciar, n: n, err: err}
		})
	}
	return nil
}

// objetivo es el trabajo sobre el que actúan t, o, c y x: el del detalle abierto o, en la lista, el del cursor.
func (p *Pagina) objetivo() (nucleo.Trabajo, bool) {
	if p.abierto != "" {
		if p.detalle != nil {
			return *p.detalle, true
		}
		return nucleo.Trabajo{}, false
	}
	if !p.cargado {
		return nucleo.Trabajo{}, false
	}
	return p.lista.Seleccion()
}

// puedeCorregir: «Corregir» solo existe para una petición que no cumplía el esquema y conservó su respuesta.
func puedeCorregir(t nucleo.Trabajo) bool {
	return t.CodigoError == nucleo.CodigoSalidaNoCumpleEsquema && t.Salida != ""
}

// hayTerminados dice si hay algo que vaciar (el botón de la Web UI se desactiva si no).
func (p *Pagina) hayTerminados() bool {
	return slices.ContainsFunc(p.trabajos, func(t nucleo.Trabajo) bool { return t.Estado.Terminal() })
}

// aplicarEntrada aplica lo escrito en la búsqueda abierta: la de la lista se ve al escribir; la del detalle, al pulsar Intro.
func (p *Pagina) aplicarEntrada() {
	if !p.enVisor {
		p.busqueda = p.entrada
		p.lista.Buscar(p.busqueda)
	}
}

// teclaBusqueda atiende las teclas de la búsqueda abierta: Intro la deja puesta, Esc la cancela y vuelve a lo de antes.
func (p *Pagina) teclaBusqueda(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc":
		p.buscando = false
		if !p.enVisor {
			p.busqueda = p.antes
			p.lista.Buscar(p.busqueda)
		}
	case "enter":
		p.buscando = false
		if p.enVisor {
			p.visor.Buscar(p.entrada)
		}
	case "backspace":
		r := []rune(p.entrada)
		p.entrada = string(r[:max(len(r)-1, 0)])
		p.aplicarEntrada()
	default:
		if k.Text != "" {
			p.entrada += k.Text
			p.aplicarEntrada()
		}
	}
	return nil
}

// abrirBusqueda abre el campo de búsqueda: el de la lista o, con el detalle abierto, el del visor.
func (p *Pagina) abrirBusqueda() {
	p.buscando, p.enVisor = true, p.abierto != ""
	p.antes, p.entrada = p.busqueda, p.busqueda
	if p.enVisor {
		p.entrada = ""
	}
}

// tecla atiende una tecla. Esc cierra lo que haya abierto (el selector, la búsqueda, el detalle) y, si no hay nada, vuelve.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	switch {
	case p.selector != nil:
		return p.selector.Update(k)
	case p.buscando:
		return p.teclaBusqueda(k)
	case p.abierto != "":
		return p.teclaDetalle(k)
	}
	return p.teclaLista(k)
}

func (p *Pagina) teclaLista(k tea.KeyPressMsg) tea.Cmd {
	t, hay := p.objetivo()
	switch k.String() {
	case "esc":
		return pagina.Atras
	case "r":
		return tea.Batch(p.cargar(false), p.cargarPuntos())
	case "/":
		p.abrirBusqueda()
	case "e":
		p.estado = estadosFiltro[(slices.Index(estadosFiltro, p.estado)+1)%len(estadosFiltro)]
		p.lista.Filtrar(func(t nucleo.Trabajo) bool { return pasaEstado(p.estado, t.Estado) })
	case "f":
		s := comp.NuevoSelector(p.estilos, p.tx("historial.origen"), p.opcionesOrigen())
		p.selector = &s
	case "l":
		return p.quitarFiltros()
	case "m":
		if p.hayMas {
			return p.verMas()
		}
	case "p":
		if p.cargado && len(p.trabajos) == 0 && !p.hayFiltro() { // el estado vacío: «Ir a Probar»
			return func() tea.Msg { return pagina.IrA{Pagina: pagina.PaginaEjecutar} }
		}
	case "enter":
		if hay {
			return p.abrirDetalle(t.ID, true)
		}
	case "V":
		return p.vaciar()
	case "t", "o", "c", "x":
		if hay {
			return p.actuar(k.String(), t)
		}
	default:
		if p.cargado {
			p.lista.Update(k)
		}
	}
	return nil
}

func (p *Pagina) teclaDetalle(k tea.KeyPressMsg) tea.Cmd {
	switch s := k.String(); s {
	case "esc":
		p.cerrarDetalle()
	case "r":
		return p.abrirDetalle(p.abierto, false)
	case "left", "right":
		if p.detalle != nil {
			p.pestana = min(max(p.pestana+map[string]int{"left": -1, "right": 1}[s], 0), 2)
			p.sigue = true
			p.rehacerVisor()
			p.visorTecla("g")
		}
	case "/":
		if p.detalle != nil {
			p.abrirBusqueda()
		}
	case "t", "o", "c", "x":
		if t, hay := p.objetivo(); hay {
			return p.actuar(s, t)
		}
	default:
		if p.visor.Update(k) {
			p.sigue = s == "G" || s == "end"
		}
	}
	return nil
}

// visorTecla manda una tecla al visor, por ejemplo g (arriba) o G (abajo).
func (p *Pagina) visorTecla(texto string) {
	p.visor.Update(tea.KeyPressMsg{Code: []rune(texto)[0], Text: texto})
}

// actuar hace t, o, c o x sobre un trabajo, solo en los estados en que la Web UI ofrece la acción: reintentar y eliminar en
// uno terminado, cancelar en uno en marcha y corregir en uno que no cumplía el esquema.
func (p *Pagina) actuar(tecla string, t nucleo.Trabajo) tea.Cmd {
	terminado := t.Estado.Terminal()
	switch tecla {
	case "t":
		if terminado {
			return p.accion(accionReintentar, t)
		}
	case "o":
		if puedeCorregir(t) {
			return p.accion(accionCorregir, t)
		}
		return comp.Aviso(p.tx("error."+string(nucleo.CodigoCorreccionNoDisponible)), comp.Advertencia)
	case "c":
		if !terminado {
			return p.accion(accionCancelar, t)
		}
	case "x":
		if terminado {
			p.pend = &pendiente{accion: accionEliminar, id: t.ID}
			return comp.Confirmar(p.tx("historial.eliminarTitulo") + "\n\n" + p.tx("historial.eliminarTexto"))
		}
	}
	return nil
}

// vaciar pide confirmar antes de borrar todas las peticiones terminadas.
func (p *Pagina) vaciar() tea.Cmd {
	if !p.hayTerminados() {
		return comp.Aviso(idioma.T("No hay peticiones terminadas"), comp.Advertencia)
	}
	p.pend = &pendiente{accion: accionVaciar}
	return comp.Confirmar(p.tx("historial.vaciarTitulo") + "\n\n" + p.tx("historial.vaciarTexto"))
}

// quitarFiltros deja el estado, el origen y el texto como al entrar; si cambia el origen, vuelve a pedir la lista.
func (p *Pagina) quitarFiltros() tea.Cmd {
	cambia := p.origenF != ""
	p.estado, p.origenF, p.busqueda = estTodos, "", ""
	p.lista.Buscar("")
	p.lista.Filtrar(func(t nucleo.Trabajo) bool { return pasaEstado(p.estado, t.Estado) })
	if cambia {
		return p.cargar(false)
	}
	return nil
}

func (p *Pagina) Teclas() []pagina.Tecla {
	switch {
	case p.selector != nil:
		return []pagina.Tecla{{Tecla: idioma.T("Intro"), Accion: idioma.T("elige")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
	case p.buscando:
		return []pagina.Tecla{{Tecla: idioma.T("Intro"), Accion: idioma.T("aplica")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
	case p.abierto != "":
		teclas := []pagina.Tecla{{Tecla: "Esc", Accion: idioma.T("vuelve")}, {Tecla: p.flechasLado(), Accion: idioma.T("pestañas")}, {Tecla: "/", Accion: idioma.T("busca")}}
		if t, ok := p.objetivo(); ok {
			teclas = append(teclas, p.teclasDe(t)...)
		}
		return teclas
	}
	teclas := []pagina.Tecla{{Tecla: idioma.T("Intro"), Accion: idioma.T("detalles")}, {Tecla: "/", Accion: idioma.T("busca")},
		{Tecla: "e", Accion: p.tx("historial.estado")}, {Tecla: "f", Accion: p.tx("historial.origen")}}
	if p.hayFiltro() {
		teclas = append(teclas, pagina.Tecla{Tecla: "l", Accion: p.tx("historial.quitarFiltros")})
	}
	if p.hayMas {
		teclas = append(teclas, pagina.Tecla{Tecla: "m", Accion: p.tx("historial.verMas")})
	}
	if t, ok := p.objetivo(); ok {
		teclas = append(teclas, p.teclasDe(t)...)
	}
	if p.hayTerminados() {
		teclas = append(teclas, pagina.Tecla{Tecla: "V", Accion: p.tx("historial.vaciar")})
	}
	return teclas
}

// teclasDe son las acciones que ofrece un trabajo según su estado.
func (p *Pagina) teclasDe(t nucleo.Trabajo) []pagina.Tecla {
	if !t.Estado.Terminal() {
		return []pagina.Tecla{{Tecla: "c", Accion: p.tx("comun.cancelar")}}
	}
	teclas := []pagina.Tecla{{Tecla: "t", Accion: p.tx("historial.reintentar")}}
	if puedeCorregir(t) {
		teclas = append(teclas, pagina.Tecla{Tecla: "o", Accion: p.tx("ejecutar.corregir")})
	}
	return append(teclas, pagina.Tecla{Tecla: "x", Accion: p.tx("comun.eliminar")})
}

// flechasLado es el dibujo de las teclas de cambiar de pestaña; en ASCII, <>.
func (p *Pagina) flechasLado() string {
	if p.ascii() {
		return "<>"
	}
	return "←→"
}

// opcionesOrigen son las del filtro de origen de la Web UI: todos, los cuatro orígenes, los grupos de las APIs y cada API.
func (p *Pagina) opcionesOrigen() []comp.Item {
	items := []comp.Item{{Valor: "", Texto: p.tx("historial.todosOrigenes")}}
	for _, o := range origenesConocidos {
		items = append(items, comp.Item{Valor: "o:" + o, Texto: p.tx(claveOrigen[o])})
	}
	var grupos []string
	for _, pt := range p.puntos {
		if pt.Grupo != "" && !slices.Contains(grupos, pt.Grupo) {
			grupos = append(grupos, pt.Grupo)
		}
	}
	slices.Sort(grupos)
	for _, g := range grupos {
		items = append(items, comp.Item{Valor: "g:" + g, Texto: p.tx("historial.grupo", "nombre", g)})
	}
	for _, pt := range p.puntos {
		items = append(items, comp.Item{Valor: "p:" + pt.Nombre, Texto: p.tx("historial.api", "nombre", pt.Nombre)})
	}
	return items
}
