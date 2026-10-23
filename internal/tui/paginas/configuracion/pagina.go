// Package configuracion es la página Ajustes de la terminal, a la par de la de la Web UI: la pestaña «Este espacio» (el
// modelo predeterminado y su esfuerzo, el historial, borrar datos y los valores técnicos, con la herencia de Global en un
// proyecto) y la pestaña «Todo Tramtai» (el puerto, el conversor de archivos, las actualizaciones con sus novedades, el
// arranque con el sistema y Acerca de).
package configuracion

import (
	"errors"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// origen es el pagina.Origen de los mensajes de Ajustes: un tipo propio, para no confundirlos con los de otra página.
type origen struct{ pagina.Origen }

type (
	msgCarga struct {
		origen
		n              int // qué carga es: solo vale la última
		nombre, ambito string
		entradas       []contrato.EntradaConfiguracion
		catalogo       []contrato.Modelo
		retencion      contrato.Retencion
		terminados     int
		puntos         []string
		ajustes        contrato.AjustesServicio
		conversor      *contrato.EstadoConversor // nil: el servicio no tiene conversor
		acerca         *contrato.Acerca          // nil si no llegó: Acerca de lo dice
		err            error
	}
	// msgActualizacion trae el estado de las actualizaciones; con ahora, era «Buscar actualización ahora».
	msgActualizacion struct {
		origen
		e     contrato.EstadoActualizacion
		ahora bool
		err   error
	}
	// msgAccion es la respuesta de un cambio en el servicio: el aviso si salió bien y, si venía de un formulario, cuál;
	// preguntar es lo que se pregunta después (reiniciar tras cambiar el puerto) y reinicio, que el servicio se va: no se
	// recarga, la app se vuelve a conectar con el nuevo.
	msgAccion struct {
		origen
		aviso, form, preguntar string
		reinicio               bool
		err                    error
	}
	msgConversor struct {
		origen
		e   contrato.EstadoConversor
		err error
	}
	msgSondeoConversor struct{ origen }
)

const (
	pestanaEspacio = iota
	pestanaTramtai
)

// Las secciones de cada pestaña, en el orden de la Web UI.
const (
	secModelo          = "modelo"
	secHistorial       = "historial"
	secDatos           = "datos"
	secValores         = "valores"
	secPuerto          = "puerto"
	secConversor       = "conversor"
	secActualizaciones = "actualizaciones"
	secArranque        = "arranque"
	secAcerca          = "acerca"
)

// Lo que se abre encima de las secciones y ocupa la página.
const (
	vistaValores   = "valores"
	vistaAcerca    = "acerca"
	vistaNovedades = "novedades"
	vistaProgreso  = "progreso"
)

// Las claves de cada ajuste que se puede devolver al de Global, como en Configuracion.vue.
var (
	clavesPredeterminado = []string{"predeterminados.agente", "predeterminados.modelo", "predeterminados.perfil_modelo", "predeterminados.esfuerzo"}
	clavesRetencion      = []string{"retencion.horas_resultado", "retencion.horas_metadatos", "retencion.max_trabajos"}
)

// formulario es uno de los de la página: el historial, el puerto o qué borrar.
type formulario struct {
	id string
	f  comp.Formulario
}

// Pagina es la página Ajustes.
type Pagina struct {
	c       pagina.Contexto
	gen     int
	lengua  string
	estilos *pagina.Estilos
	visor   comp.Visor
	ahora   func() time.Time
	espera  func(time.Duration) <-chan time.Time

	n       int
	cargado bool
	sinServ bool
	err     string // el de la primera carga, sin nada que enseñar; los de después son avisos
	ocupado bool   // hay un cambio en marcha: no se lanza otro

	nombre, ambito string
	entradas       []contrato.EntradaConfiguracion
	catalogo       []contrato.Modelo
	retencion      contrato.Retencion
	terminados     int
	puntos         []string
	ajustes        contrato.AjustesServicio
	conversor      *contrato.EstadoConversor
	act            *contrato.EstadoActualizacion // nil hasta que llega (o si no llega: la sección no se ve)
	acerca         *contrato.Acerca

	pestana int
	cursor  [2]int // la sección elegida en cada pestaña

	form     *formulario
	sel      *comp.Selector
	selQue   string // modelo o esfuerzo
	abierta  string // vistaValores, vistaAcerca, vistaNovedades o vistaProgreso; "" las secciones
	buscando bool   // en los valores técnicos, escribiendo el filtro
	filtro   string
	pend     string // lo que se confirma: borrar o cancelar (la actualización)
	borrar   struct {
		historial bool
		puntos    []string
	}
	sondeando bool // se sigue la preparación del conversor
	upd       actualizacionEnCurso
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina {
	return &Pagina{lengua: lengua, ahora: time.Now, espera: time.After}
}

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.configuracion") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaConfiguracion }

// EnCampo es verdadero con un formulario, el selector o el filtro de los valores técnicos: las letras son suyas.
func (p *Pagina) EnCampo() bool { return p.form != nil || p.sel != nil || p.buscando }

func (p *Pagina) tx(clave string, pares ...string) string {
	return p.estilos.Tx(p.lengua, clave, pares...)
}

// Iniciar empieza de cero: «Cargando…» hasta que llegan los ajustes. La pestaña y la sección elegidas se recuerdan.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	p.c, p.lengua = c, c.Idioma
	p.gen++
	if p.estilos != c.Estilos {
		p.estilos, p.visor = c.Estilos, comp.NuevoVisor(c.Estilos)
	}
	p.cargado, p.sinServ, p.err, p.ocupado, p.form, p.sel, p.abierta, p.buscando, p.filtro, p.pend = false, false, "", false, nil, nil, "", false, "", ""
	p.act, p.acerca, p.sondeando, p.upd = nil, nil, false, actualizacionEnCurso{}
	return p.cargar()
}

// Ocultar deja de seguir lo que esté en marcha (el conversor, una actualización) mientras no se ve; Mostrar lo retoma.
func (p *Pagina) Ocultar() { p.gen++ }

func (p *Pagina) Mostrar() tea.Cmd {
	p.gen++
	var cmds []tea.Cmd
	switch {
	case !p.cargado: // la carga que estaba en camino ya no vale
		cmds = append(cmds, p.cargar())
	case p.act == nil:
		cmds = append(cmds, p.cargarActualizacion(false))
	}
	if p.sondeando {
		cmds = append(cmds, p.sondearConversor())
	}
	if p.abierta == vistaProgreso {
		cmds = append(cmds, p.sondear())
	}
	return tea.Batch(cmds...)
}

func (p *Pagina) origen() origen        { return origen{p.c.Origen(p.gen)} }
func (p *Pagina) vigente(o origen) bool { return o.Vigente(p.c, p.gen) }

// cargar pide todo lo de la página; las actualizaciones van aparte, porque la primera consulta espera a GitHub.
func (p *Pagina) cargar() tea.Cmd {
	c := p.c
	if c.Cli == nil {
		p.sinServ = true
		return pagina.SinCliente
	}
	p.n++
	m := msgCarga{origen: p.origen(), n: p.n}
	return tea.Batch(func() tea.Msg {
		cli, ctx, id := c.Cli, c.Ctx, c.Instancia
		espacios, err := cli.Espacios(ctx)
		for _, e := range espacios {
			if e.ID == id {
				m.nombre, m.ambito = e.Nombre, e.Ambito
			}
		}
		var inst contrato.ResumenInstancia
		var lista contrato.ListaPuntos
		var errs [6]error
		m.entradas, errs[0] = cli.ConfiguracionInstancia(ctx, id)
		m.catalogo, errs[1] = cli.Catalogo(ctx, id)
		m.retencion, errs[2] = cli.Retencion(ctx, id)
		inst, errs[3] = cli.Instancia(ctx, id)
		lista, errs[4] = cli.Puntos(ctx, id)
		m.ajustes, errs[5] = cli.AjustesServicio(ctx)
		m.err = errors.Join(append([]error{err}, errs[:]...)...)
		for estado, n := range inst.Trabajos {
			if nucleo.Estado(estado).Terminal() {
				m.terminados += n
			}
		}
		for _, x := range lista.Puntos {
			m.puntos = append(m.puntos, x.Nombre)
		}
		if e, err := cli.Conversor(ctx, id); err == nil {
			m.conversor = &e
		}
		if a, err := cli.Acerca(ctx); err == nil {
			m.acerca = &a
		}
		return m
	}, p.cargarActualizacion(false))
}

// cargarActualizacion pide el estado de las actualizaciones; con ahora, el servicio consulta GitHub en el momento.
func (p *Pagina) cargarActualizacion(ahora bool) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	return func() tea.Msg {
		e, err := c.Cli.Actualizacion(c.Ctx, ahora)
		return msgActualizacion{origen: o, e: e, ahora: ahora, err: err}
	}
}

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
		if p.form != nil {
			p.form.f.Update(tea.WindowSizeMsg{Width: m.Width})
		}
	case msgCarga:
		return p, p.alCargar(m)
	case msgActualizacion:
		return p, p.alActualizacion(m)
	case msgAccion:
		return p, p.alAccion(m)
	case msgConversor:
		return p, p.alConversor(m)
	case msgTrasReinicio:
		return p, p.alTrasReinicio(m)
	case msgReiniciado:
		return p, p.alReiniciado(m)
	case msgSondeoConversor:
		if p.vigente(m.origen) && p.sondeando {
			return p, p.leerConversor()
		}
	case pagina.ConNovedades:
		return p, p.pedirNovedades()
	case comp.Elegido:
		return p, p.elegido(m.Valor)
	case comp.Cancelado:
		p.sel, p.pend = nil, ""
	case comp.Confirmado:
		return p, p.confirmado()
	case comp.Enviado:
		return p, p.enviar()
	case tea.PasteMsg:
		if p.form != nil {
			return p, p.form.f.Update(m)
		}
	case tea.KeyPressMsg:
		return p, p.tecla(m)
	default:
		return p, p.updateActualizacion(msg)
	}
	return p, nil
}

func (p *Pagina) alCargar(m msgCarga) tea.Cmd {
	if !p.vigente(m.origen) || m.n != p.n {
		return nil
	}
	if m.err != nil {
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			p.sinServ = true
			return cmd
		}
		if p.cargado { // lo que ya se ve no se tapa: el error va en un aviso
			return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
		}
		p.err = nucleo.MensajeDe(m.err)
		return nil
	}
	p.sinServ, p.cargado, p.err = false, true, ""
	p.nombre, p.ambito, p.entradas, p.catalogo, p.retencion = m.nombre, m.ambito, m.entradas, m.catalogo, m.retencion
	p.terminados, p.puntos, p.ajustes, p.conversor, p.acerca = m.terminados, m.puntos, m.ajustes, m.conversor, m.acerca
	if preparando(p.conversor) && !p.sondeando {
		p.sondeando = true
		return p.sondearConversor()
	}
	return nil
}

func (p *Pagina) alAccion(m msgAccion) tea.Cmd {
	if !m.DelEspacio(p.c) { // el cambio ya está hecho en el servicio: vale aunque la página se haya vuelto a iniciar
		return nil
	}
	p.ocupado = false
	if m.err != nil {
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			return cmd
		}
		return tea.Batch(comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo), p.cargar())
	}
	if p.form != nil && p.form.id == m.form {
		p.form = nil
	}
	cmds := []tea.Cmd{comp.Aviso(m.aviso, comp.Exito)}
	if m.reinicio {
		cmds = append(cmds, p.esperarReinicio(0))
	} else {
		cmds = append(cmds, p.cargar())
	}
	if m.preguntar != "" {
		p.pend = "reiniciar"
		cmds = append(cmds, comp.Confirmar(m.preguntar))
	}
	return tea.Batch(cmds...)
}

// reiniciable dice si el puerto guardado no es el que se usa y el servicio se puede reiniciar desde aquí para aplicarlo.
func (p *Pagina) reiniciable() bool {
	return p.ajustes.Reiniciable && p.ajustes.PuertoConfigurado != p.ajustes.Puerto
}

// preguntaReiniciar es la de la Web UI tras guardar el puerto.
func (p *Pagina) preguntaReiniciar(puerto int) string {
	return p.tx("puerto.reiniciarTitulo") + "\n\n" + p.tx("puerto.reiniciarTexto", "puerto", strconv.Itoa(puerto))
}

// reiniciar pide al servicio que se reinicie con el puerto nuevo, como reiniciarYSeguir de la Web UI: la terminal se
// vuelve a conectar con el servicio nuevo por su descriptor.
func (p *Pagina) reiniciar() tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	aviso := idioma.T("El servicio se reinicia; la terminal se vuelve a conectar sola")
	return func() tea.Msg {
		_, err := c.Cli.Reiniciar(c.Ctx)
		return msgAccion{origen: o, aviso: aviso, reinicio: true, err: err}
	}
}

// Tras pedir el reinicio, cada cadaReinicio se pregunta al servicio quién es (Acerca de) hasta que responde otro proceso,
// como mucho intentosReinicio veces; entonces se vuelven a leer los ajustes. Si el puerto cambió, la app se reconecta
// con el descriptor nuevo y vuelve a iniciar la página, y este seguimiento deja de valer.
const (
	cadaReinicio     = time.Second
	intentosReinicio = 30
)

type (
	msgTrasReinicio struct {
		origen
		intento int
	}
	msgReiniciado struct {
		origen
		intento int
		nuevo   bool
	}
)

func (p *Pagina) esperarReinicio(intento int) tea.Cmd {
	return pagina.Tras(p.origen(), cadaReinicio, p.espera, func(o origen) tea.Msg { return msgTrasReinicio{origen: o, intento: intento} })
}

// alTrasReinicio pregunta quién responde; un error es que el servicio aún no está (sin aviso: se espera).
func (p *Pagina) alTrasReinicio(m msgTrasReinicio) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	c, antes := p.c, p.acerca
	return func() tea.Msg {
		a, err := c.Cli.Acerca(c.Ctx)
		nuevo := err == nil && (antes == nil || a.Servicio.PID != antes.Servicio.PID || !a.Servicio.IniciadoEn.Equal(antes.Servicio.IniciadoEn))
		return msgReiniciado{origen: m.origen, intento: m.intento, nuevo: nuevo}
	}
}

func (p *Pagina) alReiniciado(m msgReiniciado) tea.Cmd {
	switch {
	case !p.vigente(m.origen):
		return nil
	case m.nuevo:
		return p.cargar()
	case m.intento+1 < intentosReinicio:
		return p.esperarReinicio(m.intento + 1)
	}
	return nil
}

// accion lanza un cambio en el servicio; al volver avisa y vuelve a cargar. form es el formulario del que viene.
func (p *Pagina) accion(aviso, form string, hacer func(c pagina.Contexto) error) tea.Cmd {
	return p.accionYPreguntar(aviso, form, "", hacer)
}

// accionYPreguntar es accion y, si sale bien, la pregunta (reiniciar tras cambiar el puerto).
func (p *Pagina) accionYPreguntar(aviso, form, preguntar string, hacer func(c pagina.Contexto) error) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	if p.ocupado {
		return nil
	}
	p.ocupado = true
	return func() tea.Msg {
		return msgAccion{origen: o, aviso: aviso, form: form, preguntar: preguntar, err: hacer(c)}
	}
}

// procedencia de una clave en un proyecto, como herencia.ts: propio si la define el proyecto o su local.yaml; de Global si
// viene de la configuración global o del valor de fábrica. En el global no se marca.
func (p *Pagina) procedencia(clave string) string {
	if p.ambito != "proyecto" {
		return ""
	}
	for _, e := range p.entradas {
		if e.Clave == clave {
			switch e.Origen {
			case "proyecto", "local":
				return "propio"
			case "global", "interno":
				return "global"
			}
		}
	}
	return ""
}

// procedenciaDe un grupo de claves: propio si alguna es propia, de Global si alguna viene de ahí.
func (p *Pagina) procedenciaDe(claves []string) string {
	var lista []string
	for _, c := range claves {
		lista = append(lista, p.procedencia(c))
	}
	switch {
	case slices.Contains(lista, "propio"):
		return "propio"
	case slices.Contains(lista, "global"):
		return "global"
	}
	return ""
}

// heredar devuelve al de Global las claves propias del grupo, como volverAGlobal de la Web UI.
func (p *Pagina) heredar(claves []string) tea.Cmd {
	if p.procedenciaDe(claves) != "propio" {
		return nil
	}
	var propias []string
	for _, c := range claves {
		if p.procedencia(c) == "propio" {
			propias = append(propias, c)
		}
	}
	return p.accion(p.tx("herencia.vuelto"), "", func(c pagina.Contexto) error {
		for _, clave := range propias {
			if err := c.Cli.Heredar(c.Ctx, c.Instancia, clave); err != nil {
				return err
			}
		}
		return nil
	})
}

// modeloActual es la entrada del catálogo que es el modelo predeterminado del espacio.
func (p *Pagina) modeloActual() (contrato.Modelo, bool) {
	i := slices.IndexFunc(p.catalogo, func(m contrato.Modelo) bool { return m.Predeterminado })
	if i < 0 {
		return contrato.Modelo{}, false
	}
	return p.catalogo[i], true
}

// nivel es el nombre de un esfuerzo; "" es el del agente.
func (p *Pagina) nivel(n string) string {
	if n == "" {
		return p.tx("esfuerzo.agente")
	}
	return p.tx("esfuerzo." + n)
}

// abrirSelector abre el del modelo predeterminado (con el catálogo) o el de su esfuerzo (con los niveles que admite).
func (p *Pagina) abrirSelector(que string) {
	var items []comp.Item
	titulo := p.tx("predeterminado.titulo")
	if que == "modelo" {
		for _, m := range p.catalogo {
			items = append(items, comp.Item{Valor: m.ID, Texto: m.ID, Detalle: m.Version})
		}
	} else {
		m, _ := p.modeloActual()
		titulo = p.tx("esfuerzo.titulo")
		items = append(items, comp.Item{Valor: "", Texto: p.nivel(""), Detalle: p.tx("esfuerzo." + m.EsfuerzoPredeterminado)})
		if m.EsfuerzoPredeterminado == "" {
			items[0].Detalle = ""
		}
		for _, n := range m.Esfuerzos {
			items = append(items, comp.Item{Valor: n, Texto: p.nivel(n)})
		}
	}
	s := comp.NuevoSelector(p.estilos, titulo, items)
	p.sel, p.selQue = &s, que
}

// elegido guarda lo elegido en el selector. Un esfuerzo que el modelo nuevo no admite vuelve al del agente, como la Web UI:
// si no, se aplicaría sin verse.
func (p *Pagina) elegido(valor string) tea.Cmd {
	if p.sel == nil {
		return nil
	}
	que := p.selQue
	p.sel = nil
	if que == "modelo" {
		var esfuerzo *string
		if m, ok := p.modeloActual(); ok && m.EsfuerzoEspacio != "" {
			i := slices.IndexFunc(p.catalogo, func(x contrato.Modelo) bool { return x.ID == valor })
			if i < 0 || !slices.Contains(p.catalogo[i].Esfuerzos, m.EsfuerzoEspacio) {
				esfuerzo = new("")
			}
		}
		return p.accion(p.tx("predeterminado.guardado", "modelo", valor), "", func(c pagina.Contexto) error {
			return c.Cli.FijarPredeterminado(c.Ctx, c.Instancia, valor, esfuerzo)
		})
	}
	return p.accion(p.tx("predeterminado.esfuerzoGuardado", "nivel", strings.ToLower(p.nivel(valor))), "", func(c pagina.Contexto) error {
		return c.Cli.FijarPredeterminado(c.Ctx, c.Instancia, "", &valor)
	})
}

// numero escribe un número con decimales como la Web UI: con coma en español y sin ceros de más.
func (p *Pagina) numero(x float64, decimales int) string {
	s := strconv.FormatFloat(x, 'f', decimales, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if p.lengua != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

// decimal lee un número escrito con punto o con coma.
func decimal(s string) (float64, bool) {
	x, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(s), ",", ".", 1), 64)
	return x, err == nil
}

// abrirForm abre el formulario de la sección: el historial, el puerto o qué borrar.
func (p *Pagina) abrirForm(id string) {
	var f comp.Formulario
	switch id {
	case secHistorial:
		r := p.retencion
		f = comp.NuevoFormulario(p.estilos,
			comp.Numero("resultado", p.tx("historialAjustes.respuestas"), strconv.Itoa(r.HorasResultado)),
			comp.Texto("registro", p.tx("historialAjustes.registro"), p.numero(float64(r.HorasMetadatos)/24, 2)),
			comp.Numero("maximo", p.tx("historialAjustes.maximo"), strconv.Itoa(r.MaxTrabajos)))
		f.Validar = p.validarRetencion
	case secPuerto:
		f = comp.NuevoFormulario(p.estilos, comp.Numero("puerto", p.tx("puerto.campo"), strconv.Itoa(p.ajustes.PuertoConfigurado)),
			comp.Interruptor("automatico", p.tx("puerto.automatico"), p.ajustes.PuertoAutomatico))
		f.Validar = func(f *comp.Formulario) map[string]string {
			if n, err := strconv.Atoi(f.Valor("puerto")); err != nil || n < 1024 || n > 65535 {
				return map[string]string{"puerto": p.tx("puerto.regla")}
			}
			return nil
		}
	case secDatos:
		f = comp.NuevoFormulario(p.estilos, comp.Interruptor("historial", p.tx("datos.historial"), p.terminados > 0),
			comp.Interruptor("puntos", p.tx("datos.puntos"), false))
		f.Validar = func(f *comp.Formulario) map[string]string {
			if !(f.Marcado("historial") && p.terminados > 0) && !(f.Marcado("puntos") && len(p.puntos) > 0) {
				return map[string]string{"historial": idioma.T("Elige qué borrar")}
			}
			return nil
		}
	default:
		return
	}
	f.Update(tea.WindowSizeMsg{Width: p.c.Ancho})
	p.form = &formulario{id: id, f: f}
}

// validarRetencion son las reglas de cada campo, las mismas que dicen sus ayudas (enRango y registroCubre de la Web UI).
func (p *Pagina) validarRetencion(f *comp.Formulario) map[string]string {
	errores := map[string]string{}
	rango := func(min, max int) string {
		return p.tx("historialAjustes.rango", "min", pagina.Numero(p.lengua, min), "max", pagina.Numero(p.lengua, max))
	}
	resultado, err := strconv.Atoi(f.Valor("resultado"))
	if err != nil || resultado < 0 || resultado > 720 {
		errores["resultado"] = rango(0, 720)
	}
	if dias, ok := decimal(f.Valor("registro")); !ok || dias < 0 || dias > 90 {
		errores["registro"] = rango(0, 90)
	} else if err == nil && int(dias*24+0.5) < resultado {
		errores["registro"] = p.tx("historialAjustes.registroMenor", "horas", strconv.Itoa(resultado))
	}
	if n, err := strconv.Atoi(f.Valor("maximo")); err != nil || n < 100 || n > 100000 {
		errores["maximo"] = rango(100, 100000)
	}
	return errores
}

// enviar guarda el formulario: el historial y el puerto van al servicio; lo que se borra pregunta antes.
func (p *Pagina) enviar() tea.Cmd {
	if p.form == nil {
		return nil
	}
	f := &p.form.f
	switch p.form.id {
	case secHistorial:
		dias, _ := decimal(f.Valor("registro"))
		r := contrato.Retencion{HorasResultado: f.Entero("resultado"), HorasMetadatos: int(dias*24 + 0.5), MaxTrabajos: f.Entero("maximo")}
		return p.accion(p.tx("historialAjustes.guardado"), secHistorial, func(c pagina.Contexto) error {
			return c.Cli.FijarRetencion(c.Ctx, c.Instancia, r)
		})
	case secPuerto:
		puerto, automatico := f.Entero("puerto"), f.Marcado("automatico")
		aviso, preguntar := p.tx("puerto.guardado"), ""
		switch {
		case puerto != p.ajustes.Puerto && p.ajustes.Reiniciable: // como la Web UI: se ofrece reiniciar ahora
			preguntar = p.preguntaReiniciar(puerto)
		case puerto != p.ajustes.Puerto:
			aviso += " · " + idioma.T("se aplica al reiniciar el servicio")
		}
		return p.accionYPreguntar(aviso, secPuerto, preguntar, func(c pagina.Contexto) error { return c.Cli.FijarPuerto(c.Ctx, puerto, automatico) })
	case secDatos:
		p.borrar.historial = f.Marcado("historial") && p.terminados > 0
		p.borrar.puntos = nil
		if f.Marcado("puntos") {
			p.borrar.puntos = slices.Clone(p.puntos)
		}
		p.form, p.pend = nil, "borrar"
		return comp.Confirmar(p.preguntaBorrar())
	}
	return nil
}

// preguntaBorrar dice de qué espacio y qué se borra, con los textos de la Web UI.
func (p *Pagina) preguntaBorrar() string {
	texto := p.tx("datos.titulo") + "\n\n" + p.tx("datos.confirmar", "espacio", p.nombreEspacio())
	if p.borrar.historial {
		texto += "\n" + p.estilos.Iconos.Separador + " " + p.tx("datos.historial")
	}
	if len(p.borrar.puntos) > 0 {
		texto += "\n" + p.estilos.Iconos.Separador + " " + p.tx("datos.puntos")
	}
	return texto
}

func (p *Pagina) nombreEspacio() string {
	if p.nombre != "" {
		return p.nombre
	}
	return p.c.Instancia
}

// confirmado hace lo que se preguntó: borrar los datos o cancelar la actualización.
func (p *Pagina) confirmado() tea.Cmd {
	pend := p.pend
	p.pend = ""
	switch pend {
	case "borrar":
		historial, puntos := p.borrar.historial, p.borrar.puntos
		return p.accion(p.tx("datos.hecho"), "", func(c pagina.Contexto) error {
			if historial {
				if _, err := c.Cli.VaciarTrabajos(c.Ctx, c.Instancia); err != nil {
					return err
				}
			}
			for _, nombre := range puntos {
				if err := c.Cli.EliminarPunto(c.Ctx, c.Instancia, nombre); err != nil {
					return err
				}
			}
			return nil
		})
	case "cancelar":
		return p.decidir(false)
	case "reiniciar":
		return p.reiniciar()
	}
	return nil
}

// secciones son las de la pestaña elegida que hay que enseñar: las de «Todo Tramtai» que el servicio no tiene no salen.
func (p *Pagina) secciones() []string {
	if p.pestana == pestanaEspacio {
		return []string{secModelo, secHistorial, secDatos, secValores}
	}
	l := []string{secPuerto}
	if p.conversor != nil {
		l = append(l, secConversor)
	}
	if p.act != nil {
		l = append(l, secActualizaciones)
	}
	if p.ajustes.Autoarranque != nil {
		l = append(l, secArranque)
	}
	return append(l, secAcerca)
}

// seccion es la elegida.
func (p *Pagina) seccion() string {
	l := p.secciones()
	return l[min(p.cursor[p.pestana], len(l)-1)]
}

// tecla atiende una tecla: lo abierto primero y, si no, la sección elegida.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	t := k.String()
	switch {
	case p.sel != nil:
		return p.sel.Update(k)
	case p.form != nil:
		if t == "esc" {
			p.form = nil
			return nil
		}
		return p.form.f.Update(k)
	case p.abierta != "":
		return p.teclaAbierta(t, k)
	}
	switch t {
	case "esc":
		return pagina.Atras
	case "r":
		return p.cargar()
	case "u":
		if p.act != nil && p.act.Disponible {
			return p.abrirNovedades()
		}
		return nil
	}
	if !p.cargado {
		return nil
	}
	switch t {
	case "left", "right":
		p.pestana = map[string]int{"left": pestanaEspacio, "right": pestanaTramtai}[t]
		return nil
	case "up", "k":
		p.cursor[p.pestana] = max(p.cursor[p.pestana]-1, 0)
		return nil
	case "down", "j":
		p.cursor[p.pestana] = min(p.cursor[p.pestana]+1, len(p.secciones())-1)
		return nil
	}
	return p.teclaSeccion(p.seccion(), t)
}

// teclaSeccion son las acciones de cada sección, como los botones e interruptores de su tarjeta en la Web UI.
func (p *Pagina) teclaSeccion(sec, t string) tea.Cmd {
	switch {
	case sec == secModelo && t == "enter" && len(p.catalogo) > 0:
		p.abrirSelector("modelo")
	case sec == secModelo && t == "f":
		if m, ok := p.modeloActual(); ok && len(m.Esfuerzos) > 0 {
			p.abrirSelector("esfuerzo")
		}
	case sec == secModelo && t == "h":
		return p.heredar(clavesPredeterminado)
	case sec == secHistorial && t == "h":
		return p.heredar(clavesRetencion)
	case (sec == secHistorial || sec == secPuerto) && t == "e", sec == secDatos && t == "x":
		p.abrirForm(sec)
	case sec == secPuerto && t == "R" && p.reiniciable():
		p.pend = "reiniciar"
		return comp.Confirmar(p.preguntaReiniciar(p.ajustes.PuertoConfigurado))
	case sec == secValores && t == "enter":
		p.abierta, p.filtro = vistaValores, ""
	case sec == secConversor && t == "p" && puedePreparar(p.conversor):
		return p.prepararConversor()
	case sec == secConversor && t == "a":
		preparar := !p.conversor.PrepararAlIniciar
		return p.accion(p.tx("conversor.alIniciar")+": "+siNo(p, preparar), "", func(c pagina.Contexto) error {
			return c.Cli.AjustesConversor(c.Ctx, c.Instancia, contrato.AjustesConversor{PrepararAlIniciar: preparar})
		})
	case sec == secActualizaciones && t == "space":
		buscar := !p.act.Buscar
		return p.accion(p.tx("actualizacion.ajustesTitulo")+": "+siNo(p, buscar), "", func(c pagina.Contexto) error {
			_, err := c.Cli.BuscarActualizaciones(c.Ctx, buscar)
			return err
		})
	case sec == secActualizaciones && t == "b":
		return p.cargarActualizacion(true)
	case sec == secArranque && t == "space":
		activo := !*p.ajustes.Autoarranque
		aviso := p.tx("arranque.desactivado")
		if activo {
			aviso = p.tx("arranque.activado", "sistema", sistema())
		}
		return p.accion(aviso, "", func(c pagina.Contexto) error { return c.Cli.Autoarranque(c.Ctx, activo) })
	case sec == secAcerca && t == "enter":
		p.abierta = vistaAcerca
	}
	return nil
}

func siNo(p *Pagina, si bool) string {
	if si {
		return strings.ToLower(p.tx("comun.si"))
	}
	return strings.ToLower(p.tx("comun.no"))
}

// teclaAbierta: en los valores técnicos, / filtra; en Acerca de, c copia los datos para soporte; en las novedades y el
// avance, las de la actualización. Esc cierra sin cancelar nada; las flechas desplazan.
func (p *Pagina) teclaAbierta(t string, k tea.KeyPressMsg) tea.Cmd {
	if p.buscando {
		switch t {
		case "esc":
			p.buscando, p.filtro = false, ""
		case "enter":
			p.buscando = false
		case "backspace":
			r := []rune(p.filtro)
			p.filtro = string(r[:max(len(r)-1, 0)])
		default:
			if k.Text == "" {
				return nil
			}
			p.filtro += k.Text
		}
		return nil
	}
	switch {
	case t == "esc":
		p.abierta = ""
		return nil
	case t == "/" && p.abierta == vistaValores:
		p.buscando = true
		return nil
	case t == "c" && p.abierta == vistaAcerca:
		return p.copiarSoporte()
	case t == "u" && (p.abierta == vistaAcerca || p.abierta == vistaValores) && p.act != nil && p.act.Disponible:
		return p.abrirNovedades() // como dice la cabecera; en el avance no, que corre la cuenta atrás
	case p.abierta == vistaNovedades || p.abierta == vistaProgreso:
		if cmd, ok := p.teclaActualizacion(t); ok {
			return cmd
		}
	}
	p.visor.Update(k)
	return nil
}

// Conversor de archivos: preparar y seguir la preparación cada 2 s, como seguirPreparacion de la Web UI.

const cadaConversor = 2 * time.Second

// preparando dice si hay una preparación en curso: «preparando», o «listo» con etapa (sirve el anterior mientras tanto).
func preparando(e *contrato.EstadoConversor) bool {
	return e != nil && (e.Estado == "preparando" || e.Etapa != "")
}

// puedePreparar: preparar de nuevo solo cambia algo si el que se usa es el preparado por Tramtai.
func puedePreparar(e *contrato.EstadoConversor) bool {
	return e != nil && !preparando(e) && (e.Estado != "listo" || e.Origen == "preparado")
}

func (p *Pagina) prepararConversor() tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	return func() tea.Msg {
		e, err := c.Cli.PrepararConversor(c.Ctx, c.Instancia)
		return msgConversor{origen: o, e: e, err: err}
	}
}

func (p *Pagina) leerConversor() tea.Cmd {
	c, o := p.c, p.origen()
	return func() tea.Msg {
		e, err := c.Cli.Conversor(c.Ctx, c.Instancia)
		return msgConversor{origen: o, e: e, err: err}
	}
}

func (p *Pagina) sondearConversor() tea.Cmd {
	return pagina.Tras(p.origen(), cadaConversor, p.espera, func(o origen) tea.Msg { return msgSondeoConversor{o} })
}

func (p *Pagina) alConversor(m msgConversor) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	if m.err != nil {
		p.sondeando = false
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			return cmd
		}
		return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
	}
	p.conversor = &m.e
	if p.sondeando = preparando(p.conversor); p.sondeando {
		return p.sondearConversor()
	}
	return nil
}

// sistema es el nombre del sistema, como en la Web UI.
func sistema() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	}
	return "Linux"
}

// Teclas son las del pie: las de lo abierto o las de la sección elegida, como los botones de su tarjeta.
func (p *Pagina) Teclas() []pagina.Tecla {
	switch {
	case p.sel != nil:
		return []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("elige")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
	case p.form != nil:
		envia := p.tx("comun.guardar")
		if p.form.id == secDatos {
			envia = p.tx("datos.boton")
		}
		return []pagina.Tecla{{Tecla: "Ctrl+S", Accion: envia}, {Tecla: "Tab", Accion: idioma.T("siguiente campo")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
	case p.buscando:
		return []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("aplica")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
	case p.abierta == vistaValores || p.abierta == vistaAcerca:
		teclas := []pagina.Tecla{{Tecla: "/", Accion: idioma.T("busca")}}
		if p.abierta == vistaAcerca {
			teclas = []pagina.Tecla{{Tecla: "c", Accion: p.tx("acerca.copiar")}}
		}
		if p.act != nil && p.act.Disponible {
			teclas = append(teclas, pagina.Tecla{Tecla: "u", Accion: p.tx("actualizacion.boton")})
		}
		return append(teclas, pagina.Tecla{Tecla: "↑↓", Accion: idioma.T("desplaza")}, pagina.Tecla{Tecla: "Esc", Accion: idioma.T("vuelve")})
	case p.abierta != "":
		return append(p.teclasActualizacion(), pagina.Tecla{Tecla: "↑↓", Accion: idioma.T("desplaza")}, pagina.Tecla{Tecla: "Esc", Accion: idioma.T("vuelve")})
	case !p.cargado:
		return []pagina.Tecla{{Tecla: "r", Accion: p.tx("comun.reintentar")}}
	}
	var teclas []pagina.Tecla
	poner := func(tecla, accion string) { teclas = append(teclas, pagina.Tecla{Tecla: tecla, Accion: accion}) }
	switch p.seccion() {
	case secModelo:
		poner("Enter", idioma.T("elige"))
		if m, ok := p.modeloActual(); ok && len(m.Esfuerzos) > 0 {
			poner("f", p.tx("esfuerzo.titulo"))
		}
		if p.procedenciaDe(clavesPredeterminado) == "propio" {
			poner("h", p.tx("herencia.volver"))
		}
	case secHistorial:
		poner("e", idioma.T("edita"))
		if p.procedenciaDe(clavesRetencion) == "propio" {
			poner("h", p.tx("herencia.volver"))
		}
	case secDatos:
		poner("x", p.tx("datos.boton"))
	case secValores, secAcerca:
		poner("Enter", idioma.T("ver"))
	case secPuerto:
		poner("e", idioma.T("edita"))
		if p.reiniciable() {
			poner("R", p.tx("puerto.reiniciarAhora"))
		}
	case secConversor:
		if puedePreparar(p.conversor) {
			poner("p", p.tx(map[bool]string{true: "conversor.prepararDeNuevo", false: "conversor.preparar"}[p.conversor.Estado == "listo"]))
		}
		poner("a", p.tx("conversor.alIniciar"))
	case secActualizaciones:
		poner("Espacio", map[bool]string{true: idioma.T("desactiva"), false: idioma.T("activa")}[p.act.Buscar])
		poner("b", p.tx("actualizacion.buscarAhora"))
	case secArranque:
		poner("Espacio", map[bool]string{true: idioma.T("desactiva"), false: idioma.T("activa")}[*p.ajustes.Autoarranque])
	}
	if p.act != nil && p.act.Disponible {
		poner("u", p.tx("actualizacion.boton"))
	}
	return append(teclas, pagina.Tecla{Tecla: "←→", Accion: idioma.T("pestañas")})
}
