// Package ejecutar es la página Probar de la terminal, copiada de Ejecutar.vue: la receta (instrucciones, modelo, esfuerzo,
// forzar, respaldo y salida estructurada con sus plantillas), el mensaje y los archivos, la respuesta en vivo con cancelar,
// corregir y copiar, «Ver como código», «Guardar como API», las pruebas recientes con «Usar de nuevo» y el borrador.
package ejecutar

import (
	"context"
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
)

// Las pestañas: la receta con el mensaje, la respuesta y las pruebas recientes.
const (
	pestReceta = iota
	pestRespuesta
	pestRecientes
)

const (
	esperaBorrador  = 500 * time.Millisecond // tras la última tecla, se guarda el borrador
	esperaCorte     = 2 * time.Second        // antes de reabrir el flujo en vivo si se corta
	limiteRecientes = 10
)

// origen es lo que cada mensaje propio lleva para saber si aún vale: el contexto y el espacio con que se pidió y la versión
// de la página (cada Iniciar, Ocultar o Mostrar la cambia).
type origen struct {
	ctx       context.Context
	instancia string
	gen       int
}

type (
	msgCatalogo struct {
		origen
		lista []cliente.ModeloCatalogo
		err   error
	}
	msgLimites struct {
		origen
		l   contrato.Limites
		err error
	}
	msgRecientes struct {
		origen
		lista []nucleo.Trabajo
		err   error
	}
	msgPunto struct {
		origen
		p   *contrato.EstadoPunto
		err error
	}
	// msgEsquema es lo que dice el servicio del esquema que había al salir del campo.
	msgEsquema struct {
		origen
		texto, error string
	}
	msgConversor struct {
		origen
		aviso string
	}
	// msgCreado es el trabajo que se acaba de crear (enviar o corregir) o el que se cargó para seguirlo (receta: además
	// trae su receta y su mensaje al formulario).
	msgCreado struct {
		origen
		t               nucleo.Trabajo
		err             error
		enviado, receta bool
	}
	msgCancelado struct {
		origen
		t   nucleo.Trabajo
		err error
	}
	msgCodigo struct {
		origen
		ejemplos []Ejemplo
	}
	msgGuardado struct {
		origen
		p                   contrato.EstadoPunto
		clave, modeloCreada string
		actualizada         bool
		err                 error
	}
	msgBorrador struct{ n int }
)

// Pagina es la página Probar.
type Pagina struct {
	c       pagina.Contexto
	lengua  string
	estilos *pagina.Estilos
	gen     int
	visible bool
	sinServ bool

	form       comp.Formulario
	opciones   map[string]opcionesCampo
	armado     bool
	editando   bool // el foco está dentro de un campo: las teclas son del formulario
	desplaz    int  // la primera línea del formulario que se ve
	pestana    int
	catalogo   []cliente.ModeloCatalogo
	limites    *contrato.Limites
	errEsquema string // lo que dijo el servicio del esquema de ahora
	conversor  string // el aviso del conversor para los archivos de ahora
	api        string // la API que se edita (Abrir en Probar o recién guardada): «g» la actualiza
	punto      *contrato.EstadoPunto

	// La respuesta: el trabajo que se sigue, en vivo mientras no termine.
	trabajo  *nucleo.Trabajo
	final    *nucleo.Trabajo
	estado   nucleo.Estado
	salida   string
	avisos   []string
	enviando bool
	ocupado  bool // cancelar o corregir en marcha
	corte    bool
	canal    <-chan msgVivo
	nflujo   int
	cancelar context.CancelFunc
	visor    comp.Visor
	sigue    bool // la respuesta se desplaza sola hasta que se usa el visor

	recientes  comp.Lista[nucleo.Trabajo]
	cargadoRec bool

	// Lo que se abre encima: «Ver como código», «Guardar como API», la API recién creada y la plantilla por confirmar.
	codigo  *vistaCodigo
	guardar *comp.Formulario
	creada  *apiCreada
	caja    comp.Visor
	pend    string

	nborrador int

	ahora      func() time.Time
	esperaPara func(time.Duration) <-chan time.Time
	abrir      func(ctx context.Context, instancia, id string, desde int, alEvento func(numero int, tipo, datos string) error) error
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina {
	return &Pagina{lengua: lengua, ahora: time.Now, esperaPara: time.After}
}

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.ejecutar") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaEjecutar }

// EnCampo es verdadero mientras se escribe en el formulario o en «Guardar como API».
func (p *Pagina) EnCampo() bool { return p.editando || p.guardar != nil }

var textoAscii = strings.NewReplacer("·", "-", "…", "...", "’", "'", "“", "\"", "”", "\"", "—", "-", "→", "->", "«", "\"", "»", "\"")

// tx es un texto de la Web UI (datos.Texto) con sus {marcadores} rellenos; en una consola ASCII, sin caracteres de dibujo.
func (p *Pagina) tx(clave string, pares ...string) string {
	return p.a(datos.Textof(p.lengua, clave, pares...))
}

func (p *Pagina) a(texto string) string {
	if p.estilos != nil && p.estilos.ASCII {
		return textoAscii.Replace(texto)
	}
	return texto
}

func (p *Pagina) origen() origen { return origen{ctx: p.c.Ctx, instancia: p.c.Instancia, gen: p.gen} }

// vigente dice si una respuesta sigue valiendo: su contexto no se canceló, es del mismo espacio y de esta versión.
func (p *Pagina) vigente(o origen) bool {
	return o.ctx != nil && o.ctx.Err() == nil && o.instancia == p.c.Instancia && o.gen == p.gen
}

// activo dice si el trabajo que se ve sigue en marcha.
func (p *Pagina) activo() bool { return p.trabajo != nil && p.final == nil && !p.estado.Terminal() }

// Iniciar carga el catálogo, los límites y las pruebas recientes. La receta y el mensaje se conservan entre visitas (al
// crearse, el mensaje es el del borrador); un cambio de espacio olvida el trabajo que se seguía y la API que se editaba.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	otro := p.c.Instancia != "" && p.c.Instancia != c.Instancia
	p.c, p.lengua = c, c.Idioma
	if p.estilos != c.Estilos {
		p.estilos = c.Estilos
		p.visor, p.caja = comp.NuevoVisor(c.Estilos), comp.NuevoVisor(c.Estilos)
		p.recientes = comp.NuevaLista(c.Estilos, func(t nucleo.Trabajo) string { return mensajeDe(t) })
		p.recientes.Clave = func(t nucleo.Trabajo) string { return t.ID }
		p.recientes.Fila = p.filaReciente
	}
	p.gen++
	p.visible, p.sinServ, p.editando = true, false, false
	p.cerrarVivo()
	if !p.armado {
		p.armar(datosForm{mensaje: leerBorrador(c.Datos)}, "mensaje")
	}
	if otro {
		p.trabajo, p.final, p.salida, p.avisos, p.api, p.punto = nil, nil, "", nil, "", nil
		p.catalogo, p.cargadoRec = nil, false
		p.recientes.Poner(nil)
	}
	p.codigo, p.guardar, p.creada, p.pend, p.enviando, p.ocupado = nil, nil, nil, "", false, false
	p.ajustar()
	if c.Cli == nil {
		p.sinServ = true
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	cmds := []tea.Cmd{p.cargarCatalogo(), p.cargarLimites(), p.cargarRecientes()}
	if p.activo() {
		cmds = append(cmds, p.abrirVivo(p.trabajo.ID))
	}
	return tea.Batch(cmds...)
}

// Ocultar suelta el seguimiento en vivo: la página no gasta nada mientras no se ve.
func (p *Pagina) Ocultar() {
	p.visible = false
	p.gen++
	p.enviando, p.ocupado = false, false
	p.cerrarVivo()
}

// Mostrar la recupera al volver sin iniciarla: el formulario se conserva y el trabajo en marcha se vuelve a seguir.
func (p *Pagina) Mostrar() tea.Cmd {
	p.gen++
	p.visible = true
	if p.c.Cli == nil {
		return nil
	}
	cmds := []tea.Cmd{p.cargarRecientes()}
	if p.activo() {
		cmds = append(cmds, p.abrirVivo(p.trabajo.ID))
	}
	return tea.Batch(cmds...)
}

// ajustar da el ancho al formulario y el tamaño a los visores.
func (p *Pagina) ajustar() {
	if p.estilos == nil {
		return
	}
	p.form.Update(tea.WindowSizeMsg{Width: max(p.c.Ancho, 10)})
	if p.guardar != nil {
		p.guardar.Update(tea.WindowSizeMsg{Width: max(p.c.Ancho, 10)})
	}
	p.visor.Tamano(max(p.c.Ancho, 1), max(p.c.Alto-3, 1))
	p.caja.Tamano(max(p.c.Ancho, 1), max(p.c.Alto-4, 1))
}

func (p *Pagina) cargarCatalogo() tea.Cmd {
	c, o := p.c, p.origen()
	return func() tea.Msg {
		l, err := c.Cli.Catalogo(c.Ctx, c.Instancia)
		return msgCatalogo{origen: o, lista: l, err: err}
	}
}

func (p *Pagina) cargarLimites() tea.Cmd {
	c, o := p.c, p.origen()
	return func() tea.Msg {
		l, err := c.Cli.Limites(c.Ctx, c.Instancia)
		return msgLimites{origen: o, l: l, err: err}
	}
}

// fallo reparte un error: de conexión es del servicio (lo avisa la app); el de la API, su mensaje tal cual, como aviso.
func (p *Pagina) fallo(err error) tea.Cmd {
	if pagina.NoResponde(err) {
		p.sinServ = true
		return func() tea.Msg { return pagina.ErrorServicio{Err: err} }
	}
	return comp.Aviso(nucleo.MensajeDe(err), comp.Fallo)
}

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
		p.ajustar()
		p.rehacerRespuesta()
	case pagina.ConAgente:
		d := p.leer()
		d.modelo, p.api, p.punto = m.Agente, "", nil
		p.armar(d, "mensaje")
		p.pestana = pestReceta
	case pagina.ConPunto:
		return p, p.cargarPunto(m.Nombre)
	case pagina.SeguirTrabajo:
		return p, p.cargarTrabajo(m.ID, true)
	case msgCatalogo:
		if p.vigente(m.origen) {
			if m.err != nil {
				return p, p.fallo(m.err)
			}
			p.catalogo = m.lista
			p.rearmar()
		}
	case msgLimites:
		if p.vigente(m.origen) && m.err == nil { // sin límites, el campo de archivos sigue sirviendo
			p.limites = &m.l
		}
	case msgRecientes:
		return p, p.alRecientes(m)
	case msgPunto:
		return p, p.alPunto(m)
	case msgEsquema:
		if p.vigente(m.origen) && m.texto == p.form.Valor("esquema") {
			p.errEsquema = m.error
			p.form.Revalidar()
		}
	case msgConversor:
		if p.vigente(m.origen) {
			p.conversor = m.aviso
			if m.aviso != "" {
				return p, comp.Aviso(m.aviso, comp.Advertencia)
			}
		}
	case msgCreado:
		return p, p.alCreado(m)
	case msgCancelado:
		if p.vigente(m.origen) {
			p.ocupado = false
			if m.err != nil {
				return p, p.fallo(m.err)
			}
			if p.trabajo != nil && p.trabajo.ID == m.t.ID && p.final == nil {
				p.estado = m.t.Estado
				p.rehacerRespuesta()
			}
		}
	case msgCodigo:
		if p.vigente(m.origen) {
			p.codigo = &vistaCodigo{ejemplos: m.ejemplos, i: 1} // Python primero, como la Web UI
			p.ponerCodigo()
		}
	case msgGuardado:
		return p, p.alGuardado(m)
	case msgBorrador:
		if m.n == p.nborrador {
			return p, escribirBorrador(p.c.Datos, p.form.Valor("mensaje"))
		}
	case msgFlujo:
		if p.vigente(m.origen) && p.trabajo != nil && m.id == p.trabajo.ID && m.seq == p.nflujo && p.cancelar != nil {
			p.canal = m.canal
			return p, esperar(m.canal)
		}
	case msgVivo:
		if !p.vivoVigente(m) { // uno viejo no vuelve a esperar: quedarían dos lectores sobre el canal nuevo
			return p, nil
		}
		cmd := p.alVivo(m)
		if p.canal == nil {
			return p, cmd
		}
		return p, tea.Batch(esperar(p.canal), cmd)
	case comp.Enviado:
		if p.guardar != nil {
			return p, p.crearApi()
		}
		return p, p.enviar()
	case comp.Cancelado:
		switch {
		case p.pend != "":
			p.pend = ""
		case p.guardar != nil:
			p.guardar = nil
		default:
			p.editando = false
		}
	case comp.Confirmado:
		if p.pend != "" {
			id := p.pend
			p.pend = ""
			p.ponerPlantilla(id)
		}
	case tea.PasteMsg:
		return p, p.pegar(m)
	case tea.KeyPressMsg:
		return p, p.tecla(m)
	}
	return p, nil
}

// pegar lleva lo pegado al campo con el foco, entero (saltos de línea incluidos); fuera de un campo empieza a editarlo.
func (p *Pagina) pegar(m tea.PasteMsg) tea.Cmd {
	switch {
	case p.guardar != nil:
		return p.guardar.Update(m)
	case p.codigo != nil || p.creada != nil:
		return nil
	}
	p.pestana, p.editando = pestReceta, true
	return p.alForm(m)
}

// tecla reparte una tecla: lo que está abierto encima, el formulario si se edita y, si no, las teclas de la pestaña.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	t := k.String()
	switch {
	case p.guardar != nil:
		return p.guardar.Update(k)
	case p.codigo != nil:
		return p.teclaCodigo(t)
	case p.creada != nil:
		return p.teclaCreada(t)
	case p.editando:
		return p.teclaCampo(k)
	}
	switch t {
	case "ctrl+s", "ctrl+enter":
		cmd := p.form.Update(k) // valida: con errores enfoca el campo, sin ellos responde Enviado
		if cmd == nil {
			p.pestana = pestReceta // el campo con el error se ve
		}
		return cmd
	case "esc":
		if p.activo() && p.estado != nucleo.CancelacionSolicitada {
			return p.cancelarTrabajo()
		}
		return pagina.Atras
	case "left", "right":
		p.pestana = (p.pestana + map[string]int{"left": 2, "right": 1}[t]) % 3
		p.rehacerRespuesta()
		return nil
	case "k":
		return p.verCodigo()
	case "g":
		return p.guardarApi()
	case "d":
		p.api, p.punto = "", nil
		return nil
	case "y":
		return copiar(p.salida)
	case "o":
		return p.corregir()
	case "n":
		return p.nuevaPrueba()
	case "h":
		return func() tea.Msg {
			return pagina.IrA{Pagina: pagina.PaginaHistorial, Datos: pagina.FiltroPunto{Nombre: "o:" + nucleo.DesdeTerminal}}
		}
	case "r":
		if p.c.Cli == nil {
			return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
		}
		return tea.Batch(p.cargarCatalogo(), p.cargarLimites(), p.cargarRecientes())
	case "ctrl+p":
		return p.prepararConversor()
	}
	switch p.pestana {
	case pestReceta:
		switch t {
		case "enter", "e":
			p.editando = true
		case "up":
			return p.alForm(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		case "down":
			return p.alForm(tea.KeyPressMsg{Code: tea.KeyTab})
		}
	case pestRespuesta:
		if t != "k" && t != "j" && p.visor.Update(k) {
			p.sigue = false
		}
	case pestRecientes:
		switch t {
		case "enter":
			if x, ok := p.recientes.Seleccion(); ok {
				return p.cargarTrabajo(x.ID, false)
			}
		case "u":
			if x, ok := p.recientes.Seleccion(); ok {
				return p.usarDeNuevo(x)
			}
		case "k", "j":
		default:
			p.recientes.Update(k)
		}
	}
	return nil
}

// copiar pone un texto en el portapapeles con OSC 52. La terminal no dice si lo admite: el aviso lo explica.
func copiar(texto string) tea.Cmd {
	if strings.TrimSpace(texto) == "" {
		return comp.Aviso(idioma.T("No hay nada que copiar"), comp.Advertencia)
	}
	return tea.Batch(tea.SetClipboard(texto), comp.Aviso(idioma.T("Copiado con OSC 52; si no llega al portapapeles, tu terminal no lo admite"), comp.Exito))
}

// Teclas son las del pie: las del formulario mientras se edita y, si no, las de la pestaña.
func (p *Pagina) Teclas() []pagina.Tecla {
	switch {
	case p.guardar != nil:
		return p.guardar.Teclas()
	case p.codigo != nil:
		return []pagina.Tecla{{Tecla: "← →", Accion: idioma.T("pestañas")}, {Tecla: "y", Accion: idioma.T("copia")}, {Tecla: "Esc", Accion: idioma.T("cierra")}}
	case p.creada != nil:
		return []pagina.Tecla{{Tecla: "y", Accion: idioma.T("copia la clave")}, {Tecla: "Esc", Accion: idioma.T("cierra")}}
	case p.editando:
		teclas := p.form.Teclas()
		switch p.form.Foco() {
		case "plantilla":
			teclas = append(teclas, pagina.Tecla{Tecla: idioma.T("Intro"), Accion: idioma.T("aplica")})
		case "archivos":
			teclas = append(teclas, pagina.Tecla{Tecla: "Tab", Accion: idioma.T("completa")})
		}
		return teclas
	}
	teclas := []pagina.Tecla{{Tecla: "← →", Accion: idioma.T("pestañas")}}
	switch p.pestana {
	case pestReceta:
		teclas = append(teclas, pagina.Tecla{Tecla: idioma.T("Intro"), Accion: idioma.T("edita")}, pagina.Tecla{Tecla: "↑↓", Accion: idioma.T("campo")})
	case pestRespuesta:
		if p.salida != "" {
			teclas = append(teclas, pagina.Tecla{Tecla: "y", Accion: idioma.T("copia")})
		}
		if p.final != nil && puedeCorregir(*p.final) {
			teclas = append(teclas, pagina.Tecla{Tecla: "o", Accion: p.tx("ejecutar.corregir")})
		}
		if p.final != nil {
			teclas = append(teclas, pagina.Tecla{Tecla: "n", Accion: p.tx("ejecutar.nueva")})
		}
	case pestRecientes:
		teclas = append(teclas, pagina.Tecla{Tecla: idioma.T("Intro"), Accion: idioma.T("ver")}, pagina.Tecla{Tecla: "u", Accion: p.tx("ejecutar.usarDeNuevo")},
			pagina.Tecla{Tecla: "h", Accion: p.tx("ejecutar.verTodo")})
	}
	teclas = append(teclas, pagina.Tecla{Tecla: "Ctrl+S", Accion: p.tx("ejecutar.enviar")}, pagina.Tecla{Tecla: "k", Accion: p.tx("codigo.boton")})
	if p.api != "" {
		teclas = append(teclas, pagina.Tecla{Tecla: "g", Accion: p.tx("ejecutar.actualizarApi", "nombre", p.api)}, pagina.Tecla{Tecla: "d", Accion: p.tx("ejecutar.dejarEditar")})
	} else {
		teclas = append(teclas, pagina.Tecla{Tecla: "g", Accion: p.tx("ejecutar.guardarApi")})
	}
	if p.activo() {
		return append(teclas, pagina.Tecla{Tecla: "Esc", Accion: p.tx("comun.cancelar")})
	}
	return append(teclas, pagina.Tecla{Tecla: "Esc", Accion: idioma.T("vuelve")})
}
