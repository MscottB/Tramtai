// Package conectar es la página Mis APIs de la terminal, a la par de la de la Web UI: las APIs por aplicación (con sus
// grupos, la búsqueda, los filtros, el detalle con su clave y su consumo, y el formulario para crearlas y ajustarlas) y la
// clave del espacio, con sus URL y el presupuesto del espacio.
package conectar

import (
	"cmp"
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
)

// origen es el pagina.Origen de los mensajes de Mis APIs: un tipo propio, para no confundirlos con los de otra página.
type origen struct{ pagina.Origen }

type (
	// msgCarga trae todo lo de la página: las APIs con su uso y su consumo y, si llegan, el presupuesto y el resumen del
	// espacio, su nombre, lo que dura el historial y la clave del espacio.
	msgCarga struct {
		origen
		n        int
		lista    contrato.ListaPuntos
		espacio  *nucleo.EstadoPresupuesto
		resumen  *contrato.ResumenInstancia
		nombre   string
		horas    int
		clave    string
		errClave error
		err      error
	}
	// msgAccion es la respuesta de un cambio en el servicio. abrir, si no es nil, es la API cuyo detalle se abre (con su
	// clave recién creada o regenerada); claveEspacio, la clave del espacio regenerada.
	msgAccion struct {
		origen
		exito        string
		abrir        *abrir
		claveEspacio string
		cerrarForm   bool
		borrada      string // la API eliminada: si es la del detalle, se cierra
		err          error
	}
)

type abrir struct {
	punto  contrato.EstadoPunto
	clave  string
	creada bool
}

// pendiente es lo que la página hará si el usuario confirma.
type pendiente struct{ accion, nombre string }

const (
	accEliminar         = "eliminar"
	accRegenerar        = "regenerar"
	accReiniciar        = "reiniciar"
	accReiniciarEspacio = "reiniciarEspacio"
	accRegenerarEspacio = "regenerarEspacio"
	accCerrarDetalle    = "cerrarDetalle"
	accCerrarForm       = "cerrarForm"
)

// Las pestañas, en el orden de la Web UI; la primera es la de siempre.
const (
	pestanaApis = iota
	pestanaClave
)

// Pagina es la página Mis APIs.
type Pagina struct {
	c       pagina.Contexto
	gen     int
	lengua  string
	estilos *pagina.Estilos
	lista   comp.Lista[contrato.EstadoPunto]
	visor   comp.Visor
	ahora   func() time.Time

	datos    contrato.ListaPuntos
	espacio  nucleo.EstadoPresupuesto
	resumen  contrato.ResumenInstancia
	nombre   string // el del espacio
	horas    int
	clave    string // la del espacio
	errClave string
	verClave bool

	n        int // la última carga pedida: solo vale su respuesta
	cargado  bool
	sinServ  bool
	err      string
	pestana  int
	ocupado  bool // hay un cambio en marcha: no se lanza otro
	pend     *pendiente
	filtros  filtros
	buscando bool
	entrada  string // lo que se escribe en la búsqueda abierta

	// El detalle: detalle es la API abierta; respaldo, la que se abrió por si la lista aún no la trae (recién creada);
	// claveVista, su clave recién creada o regenerada, que solo se ve esta vez.
	detalle    string
	respaldo   *contrato.EstadoPunto
	claveVista string
	creada     bool

	form *formulario
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina {
	return &Pagina{lengua: lengua, horas: 7 * 24, ahora: time.Now, filtros: sinFiltros}
}

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.conectar") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaConectar }

// EnCampo es verdadero mientras se escribe la búsqueda o hay un formulario abierto: las letras son del campo.
func (p *Pagina) EnCampo() bool { return p.buscando || p.form != nil }

// Pendiente avisa antes de salir si se perdería algo: una clave que no se vuelve a mostrar o un formulario a medias.
func (p *Pagina) Pendiente() string {
	switch {
	case p.claveVista != "":
		return p.tx("cierre.clave")
	case p.form != nil && p.form.cambiado():
		return p.tx("cierre.texto")
	}
	return ""
}

func (p *Pagina) tx(clave string, pares ...string) string {
	return p.estilos.Tx(p.lengua, clave, pares...)
}

// Iniciar empieza de cero: «Cargando…» hasta que llegue la lista, en la pestaña de las APIs.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	p.c, p.lengua = c, c.Idioma
	p.gen++
	if p.estilos != c.Estilos { // los mismos toda la sesión: la lista y el visor se crean una vez
		p.estilos = c.Estilos
		p.lista = comp.NuevaLista(c.Estilos, buscable)
		p.lista.Clave = func(x contrato.EstadoPunto) string { return x.Nombre }
		p.lista.Fila = p.fila
		p.visor = comp.NuevoVisor(c.Estilos)
	}
	p.datos, p.espacio, p.resumen, p.nombre, p.clave, p.errClave, p.verClave = contrato.ListaPuntos{}, nucleo.EstadoPresupuesto{}, contrato.ResumenInstancia{}, "", "", "", false
	p.cargado, p.sinServ, p.err, p.pestana, p.ocupado, p.pend = false, false, "", pestanaApis, false, nil
	p.filtros, p.buscando, p.entrada = sinFiltros, false, ""
	p.cerrarDetalle()
	p.form = nil
	p.aplicarFiltros()
	return p.cargar()
}

func (p *Pagina) origen() origen        { return origen{p.c.Origen(p.gen)} }
func (p *Pagina) vigente(o origen) bool { return o.Vigente(p.c, p.gen) }

// cargar pide todo a la vez. Sin la lista no hay página; lo demás, si falla, se queda como estaba (la clave dice su error).
func (p *Pagina) cargar() tea.Cmd {
	c := p.c
	if c.Cli == nil {
		p.sinServ = true
		return pagina.SinCliente
	}
	p.n++
	m := msgCarga{origen: p.origen(), n: p.n}
	return func() tea.Msg {
		m.lista, m.err = c.Cli.Puntos(c.Ctx, c.Instancia)
		if m.err != nil {
			return m
		}
		if e, err := c.Cli.PresupuestoInstancia(c.Ctx, c.Instancia); err == nil {
			m.espacio = &e
		}
		if r, err := c.Cli.Instancia(c.Ctx, c.Instancia); err == nil {
			m.resumen = &r
		}
		if r, err := c.Cli.Retencion(c.Ctx, c.Instancia); err == nil {
			m.horas = r.HorasMetadatos
		}
		if es, err := c.Cli.Espacios(c.Ctx); err == nil {
			if i := slices.IndexFunc(es, func(e contrato.Espacio) bool { return e.ID == c.Instancia }); i >= 0 {
				m.nombre = es[i].Nombre
			}
		}
		m.clave, m.errClave = c.Cli.ClaveInstancia(c.Ctx, c.Instancia)
		return m
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
	case msgAccion:
		return p, p.alAccion(m)
	case comp.Confirmado:
		return p, p.confirmado()
	case comp.Cancelado:
		p.pend = nil
	case comp.Enviado:
		if p.form != nil {
			return p, p.enviar()
		}
	case tea.PasteMsg:
		if p.form != nil {
			return p, p.form.f.Update(m)
		}
		if p.buscando {
			p.entrada += strings.Join(strings.Fields(m.Content), " ")
			p.lista.Buscar(p.entrada)
		}
	case tea.KeyPressMsg:
		return p, p.tecla(m)
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
		if p.cargado && len(p.datos.Puntos) > 0 { // la lista que ya se ve no se tapa: el error va en un aviso
			return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
		}
		p.err = nucleo.MensajeDe(m.err)
		return nil
	}
	p.sinServ, p.cargado, p.err, p.datos = false, true, "", m.lista
	if m.espacio != nil {
		p.espacio = *m.espacio
	}
	if m.resumen != nil {
		p.resumen = *m.resumen
	}
	if m.horas > 0 {
		p.horas = m.horas
	}
	if m.nombre != "" {
		p.nombre = m.nombre
	}
	p.clave, p.errClave = m.clave, ""
	if m.errClave != nil {
		p.errClave = nucleo.MensajeDe(m.errClave)
	}
	p.aplicarFiltros()
	if p.detalle != "" { // la API abierta ya no está (borrada desde otro sitio): el detalle se cierra
		if _, ok := p.punto(p.detalle); ok {
			p.respaldo = nil
		} else if p.respaldo == nil || p.respaldo.Nombre != p.detalle {
			p.cerrarDetalle()
		}
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
		return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
	}
	if m.cerrarForm {
		p.form = nil
	}
	if m.borrada != "" && m.borrada == p.detalle {
		p.cerrarDetalle()
	}
	if m.abrir != nil {
		x := m.abrir.punto
		p.detalle, p.respaldo, p.claveVista, p.creada = x.Nombre, &x, m.abrir.clave, m.abrir.creada
		p.arriba()
	}
	if m.claveEspacio != "" {
		p.clave, p.errClave, p.verClave = m.claveEspacio, "", true
	}
	cmds := []tea.Cmd{p.cargar()}
	if m.exito != "" {
		cmds = append(cmds, comp.Aviso(m.exito, comp.Exito))
	}
	return tea.Batch(cmds...)
}

// accion lanza un cambio en el servicio; al volver, la página avisa (si exito no está vacío) y vuelve a cargar.
func (p *Pagina) accion(exito string, hacer func(c pagina.Contexto, m *msgAccion) error) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	if p.ocupado {
		return nil
	}
	p.ocupado = true
	return func() tea.Msg {
		m := msgAccion{origen: o, exito: exito}
		m.err = hacer(c, &m)
		return m
	}
}

// confirmado hace lo que se preguntó.
func (p *Pagina) confirmado() tea.Cmd {
	pend := p.pend
	p.pend = nil
	if pend == nil {
		return nil
	}
	nombre := pend.nombre
	switch pend.accion {
	case accCerrarDetalle:
		p.cerrarDetalle()
	case accCerrarForm:
		p.form = nil
	case accEliminar:
		return p.accion("", func(c pagina.Contexto, m *msgAccion) error {
			m.borrada = nombre
			return c.Cli.EliminarPunto(c.Ctx, c.Instancia, nombre)
		})
	case accRegenerar:
		x, _ := p.punto(nombre)
		return p.accion("", func(c pagina.Contexto, m *msgAccion) error {
			clave, err := c.Cli.RotarTokenPunto(c.Ctx, c.Instancia, nombre)
			m.abrir = &abrir{punto: x, clave: clave}
			return err
		})
	case accReiniciar, accReiniciarEspacio:
		return p.accion(p.tx("presupuesto.reiniciado"), func(c pagina.Contexto, m *msgAccion) error {
			return c.Cli.ReiniciarPresupuesto(c.Ctx, c.Instancia, nombre)
		})
	case accRegenerarEspacio:
		return p.accion(p.tx("conectar.claveRegenerada"), func(c pagina.Contexto, m *msgAccion) error {
			var err error
			m.claveEspacio, err = c.Cli.RotarClave(c.Ctx, c.Instancia)
			return err
		})
	}
	return nil
}

// preguntar recuerda qué se va a hacer y pregunta antes.
func (p *Pagina) preguntar(accion, nombre, titulo, texto string) tea.Cmd {
	p.pend = &pendiente{accion: accion, nombre: nombre}
	return comp.Confirmar(titulo + "\n\n" + texto)
}

func (p *Pagina) cerrarDetalle() {
	p.detalle, p.respaldo, p.claveVista, p.creada = "", nil, "", false // la clave no se queda en la memoria de la página
	p.arriba()
}

// arriba lleva el visor al principio: el detalle y la clave del espacio lo comparten.
func (p *Pagina) arriba() {
	if p.estilos != nil {
		p.visor.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	}
}

// punto es la API de ese nombre en la lista o, si aún no está, la del detalle recién abierto.
func (p *Pagina) punto(nombre string) (contrato.EstadoPunto, bool) {
	if i := slices.IndexFunc(p.datos.Puntos, func(x contrato.EstadoPunto) bool { return x.Nombre == nombre }); i >= 0 {
		return p.datos.Puntos[i], true
	}
	if p.respaldo != nil && p.respaldo.Nombre == nombre {
		return *p.respaldo, true
	}
	return contrato.EstadoPunto{}, false
}

// elegida es la API del detalle abierto o, en la lista, la del cursor.
func (p *Pagina) elegida() (contrato.EstadoPunto, bool) {
	if p.detalle != "" {
		return p.punto(p.detalle)
	}
	if !p.cargado || p.pestana != pestanaApis {
		return contrato.EstadoPunto{}, false
	}
	return p.lista.Seleccion()
}

// tecla atiende una tecla: lo abierto primero (el formulario, la búsqueda, el detalle) y luego la pestaña.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	t := k.String()
	switch {
	case p.form != nil:
		if t == "esc" {
			if !p.form.cambiado() {
				p.form = nil
				return nil
			}
			return p.preguntar(accCerrarForm, "", p.tx("cierre.titulo"), p.tx("cierre.texto"))
		}
		return p.form.f.Update(k)
	case p.buscando:
		return p.teclaBusqueda(k)
	case p.detalle != "":
		return p.teclaDetalle(t, k)
	}
	switch t {
	case "esc":
		return pagina.Atras
	case "r":
		return p.cargar()
	case "left", "right":
		if p.cargado {
			p.pestana = map[string]int{"left": pestanaApis, "right": pestanaClave}[t]
			p.arriba()
		}
		return nil
	}
	if p.pestana == pestanaClave {
		return p.teclaClave(t, k)
	}
	return p.teclaLista(t, k)
}

func (p *Pagina) teclaLista(t string, k tea.KeyPressMsg) tea.Cmd {
	x, hay := p.elegida()
	switch t {
	case "n":
		if p.cargado {
			p.abrirForm(nil)
		}
	case "/":
		if p.cargado {
			p.buscando, p.entrada = true, p.filtros.busqueda
		}
	case "f":
		if p.cargado {
			p.abrirFiltros()
		}
	case "l":
		p.filtros = sinFiltros
		p.aplicarFiltros()
	case "g":
		return p.alternarGrupo()
	case "enter":
		if hay {
			p.detalle = x.Nombre
			p.arriba()
			return nil
		}
		p.lista.Update(k)
	case "e":
		if hay {
			p.abrirForm(&x)
		}
	case "a":
		if hay {
			return p.activar(x)
		}
	case "x":
		if hay {
			return p.eliminar(x)
		}
	default:
		if p.cargado {
			p.lista.Update(k)
		}
	}
	return nil
}

func (p *Pagina) teclaDetalle(t string, k tea.KeyPressMsg) tea.Cmd {
	x, hay := p.elegida()
	if !hay {
		p.cerrarDetalle()
		return nil
	}
	switch t {
	case "esc":
		if p.claveVista != "" { // la clave solo se ve esta vez: se pregunta, como escaparConAviso
			return p.preguntar(accCerrarDetalle, "", p.tx("cierre.claveTitulo"), p.tx("cierre.clave"))
		}
		p.cerrarDetalle()
	case "r":
		return p.cargar()
	case "y":
		if p.claveVista != "" {
			return comp.Copiar(p.claveVista)
		}
		return comp.Copiar(x.URL)
	case "t":
		return p.preguntar(accRegenerar, x.Nombre, p.tx("puntos.regenerarTitulo", "nombre", x.Nombre), p.tx("puntos.regenerarTexto"))
	case "a":
		return p.activar(x)
	case "e":
		p.abrirForm(&x)
	case "R":
		if _, ok := p.datos.Presupuestos[x.Nombre]; ok {
			return p.preguntar(accReiniciar, x.Nombre, p.tx("presupuesto.reiniciarTitulo", "nombre", x.Nombre), p.tx("presupuesto.reiniciarTexto"))
		}
	case "h":
		return func() tea.Msg {
			return pagina.IrA{Pagina: pagina.PaginaHistorial, Datos: pagina.FiltroPunto{Nombre: x.Nombre}}
		}
	case "p":
		return func() tea.Msg {
			return pagina.IrA{Pagina: pagina.PaginaEjecutar, Datos: pagina.ConPunto{Nombre: x.Nombre}}
		}
	case "x":
		return p.eliminar(x)
	default:
		p.visor.Update(k)
	}
	return nil
}

func (p *Pagina) teclaClave(t string, k tea.KeyPressMsg) tea.Cmd {
	switch t {
	case "v":
		p.verClave = !p.verClave
	case "y":
		return comp.Copiar(p.clave)
	case "t":
		if p.clave != "" {
			return p.preguntar(accRegenerarEspacio, "", p.tx("conectar.regenerarTitulo"), p.tx("conectar.regenerarTexto"))
		}
	case "R":
		if conLimites(p.espacio) {
			return p.preguntar(accReiniciarEspacio, "", p.tx("presupuesto.reiniciarEspacio"), p.tx("presupuesto.reiniciarEspacioTexto"))
		}
	default:
		p.visor.Update(k)
	}
	return nil
}

// teclaBusqueda: lo escrito filtra al momento; Intro la deja puesta y Esc la quita.
func (p *Pagina) teclaBusqueda(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc":
		p.buscando, p.entrada = false, p.filtros.busqueda
	case "enter":
		p.buscando, p.filtros.busqueda = false, strings.TrimSpace(p.entrada)
	case "backspace":
		r := []rune(p.entrada)
		p.entrada = string(r[:max(len(r)-1, 0)])
	default:
		if k.Text == "" {
			return nil
		}
		p.entrada += k.Text
	}
	p.lista.Buscar(p.entrada)
	return nil
}

func (p *Pagina) activar(x contrato.EstadoPunto) tea.Cmd {
	valor := !activo(x)
	return p.accion("", func(c pagina.Contexto, m *msgAccion) error {
		return c.Cli.ActivarPunto(c.Ctx, c.Instancia, x.Nombre, valor)
	})
}

func (p *Pagina) eliminar(x contrato.EstadoPunto) tea.Cmd {
	return p.preguntar(accEliminar, x.Nombre, p.tx("puntos.eliminarTitulo", "nombre", x.Nombre), p.tx("puntos.eliminarTexto"))
}

// alternarGrupo activa las APIs del grupo del cursor que estén detenidas o, si todas están activas, las detiene: el grupo
// entero, aunque los filtros escondan alguna, como «Activar todas» y «Detener todas» de la Web UI.
func (p *Pagina) alternarGrupo() tea.Cmd {
	g, ok := p.lista.Grupo()
	if !ok {
		return nil
	}
	var del []contrato.EstadoPunto
	for _, x := range p.datos.Puntos {
		if p.grupoDe(x) == g {
			del = append(del, x)
		}
	}
	valor := slices.ContainsFunc(del, func(x contrato.EstadoPunto) bool { return !activo(x) })
	return p.accion("", func(c pagina.Contexto, m *msgAccion) error {
		for _, x := range del {
			if activo(x) != valor {
				if err := c.Cli.ActivarPunto(c.Ctx, c.Instancia, x.Nombre, valor); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// crear y ajustar mandan el formulario al servicio.
func (p *Pagina) crear(cfg contrato.ConfigPunto) tea.Cmd {
	return p.accion("", func(c pagina.Contexto, m *msgAccion) error {
		x, clave, err := c.Cli.CrearPunto(c.Ctx, c.Instancia, cfg)
		if x.Nombre == "" {
			x.Nombre = cfg.Nombre
		}
		m.abrir, m.cerrarForm = &abrir{punto: x, clave: clave, creada: true}, true
		return err
	})
}

func (p *Pagina) ajustar(nombre string, a cliente.AjustesPunto) tea.Cmd {
	return p.accion(p.tx("puntos.ajustesGuardados"), func(c pagina.Contexto, m *msgAccion) error {
		_, err := c.Cli.AjustarPunto(c.Ctx, c.Instancia, nombre, a)
		m.cerrarForm = true
		return err
	})
}

// activo es como en la Web UI: una API está activa salvo que su error diga que está desactivada.
func activo(x contrato.EstadoPunto) bool { return x.Error != "desactivado" }

// idModelo es el modelo de la receta como id del catálogo (perfil/…, agente/modelo o agente); "" si es el del espacio.
func idModelo(x contrato.EstadoPunto) string {
	switch {
	case x.PerfilModelo != "":
		return "perfil/" + x.PerfilModelo
	case x.Agente != "" && x.Modelo != "":
		return x.Agente + "/" + x.Modelo
	}
	return x.Agente
}

// buscable es lo que encuentra la búsqueda: el nombre, el grupo, el modelo y las instrucciones.
func buscable(x contrato.EstadoPunto) string {
	return strings.Join([]string{x.Nombre, x.Grupo, idModelo(x), x.Instrucciones}, " ")
}

// grupoDe es el nombre del grupo en la lista; las que no tienen van en «Sin grupo».
func (p *Pagina) grupoDe(x contrato.EstadoPunto) string {
	return cmp.Or(x.Grupo, p.tx("puntos.sinGrupo"))
}

// Teclas son las del pie: las del formulario o la búsqueda mientras se escriben y, si no, las de lo que se ve.
func (p *Pagina) Teclas() []pagina.Tecla {
	switch {
	case p.form != nil:
		return p.form.f.Teclas()
	case p.buscando:
		return []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("aplica")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
	case p.detalle != "":
		x, _ := p.elegida()
		teclas := []pagina.Tecla{{Tecla: "Esc", Accion: idioma.T("vuelve")}, {Tecla: "y", Accion: idioma.T("copia")},
			{Tecla: "t", Accion: p.tx("puntos.regenerarClave")}, {Tecla: "a", Accion: p.textoActivar(x)}, {Tecla: "e", Accion: p.tx("puntos.ajustes")}}
		if _, ok := p.datos.Presupuestos[x.Nombre]; ok {
			teclas = append(teclas, pagina.Tecla{Tecla: "R", Accion: p.tx("presupuesto.reiniciar")})
		}
		return append(teclas, pagina.Tecla{Tecla: "h", Accion: p.tx("puntos.verPeticiones")}, pagina.Tecla{Tecla: "p", Accion: p.tx("puntos.abrirProbar")},
			pagina.Tecla{Tecla: "x", Accion: p.tx("comun.eliminar")})
	case p.pestana == pestanaClave:
		ver := p.tx("conectar.mostrarClave")
		if p.verClave {
			ver = p.tx("conectar.ocultarClave")
		}
		teclas := []pagina.Tecla{{Tecla: p.flechas(), Accion: idioma.T("pestañas")}, {Tecla: "v", Accion: ver}, {Tecla: "y", Accion: idioma.T("copia")},
			{Tecla: "t", Accion: p.tx("conectar.regenerarClave")}}
		if conLimites(p.espacio) {
			teclas = append(teclas, pagina.Tecla{Tecla: "R", Accion: p.tx("presupuesto.reiniciar")})
		}
		return teclas
	}
	teclas := []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("detalles")}, {Tecla: p.flechas(), Accion: idioma.T("pestañas")},
		{Tecla: "/", Accion: idioma.T("busca")}, {Tecla: "f", Accion: idioma.T("filtros")}, {Tecla: "n", Accion: p.tx("puntos.nuevo")}}
	if p.filtros.hay() {
		teclas = append(teclas, pagina.Tecla{Tecla: "l", Accion: p.tx("puntos.limpiarFiltros")})
	}
	if x, ok := p.elegida(); ok {
		teclas = append(teclas, pagina.Tecla{Tecla: "e", Accion: p.tx("puntos.ajustes")}, pagina.Tecla{Tecla: "a", Accion: p.textoActivar(x)})
	}
	if _, ok := p.lista.Grupo(); ok {
		teclas = append(teclas, pagina.Tecla{Tecla: "g", Accion: idioma.T("activa o detiene el grupo")})
	}
	if _, ok := p.elegida(); ok {
		teclas = append(teclas, pagina.Tecla{Tecla: "x", Accion: p.tx("comun.eliminar")})
	}
	return teclas
}

func (p *Pagina) textoActivar(x contrato.EstadoPunto) string {
	if activo(x) {
		return p.tx("puntos.accionDesactivar")
	}
	return p.tx("puntos.accionActivar")
}

// flechas es el dibujo de las teclas de cambiar de pestaña; en ASCII, <>.
func (p *Pagina) flechas() string {
	if p.estilos != nil && p.estilos.ASCII {
		return "<>"
	}
	return "← →"
}
