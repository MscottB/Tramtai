// Package agentes es la página Agentes de la terminal: los asistentes que Tramtai usa, con el estado que les da la Web UI
// (listo, sin conectar, sin instalar, desconectado o por revisar), y las mismas acciones: comprobar, conectar o
// desconectar, cambiar entre suscripción y clave de API, ver sus detalles y probarlo.
package agentes

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// origen es el pagina.Origen de los mensajes de Agentes: un tipo propio, para no confundirlos con los de otra página.
type origen struct{ pagina.Origen }

type (
	msgAgentes struct {
		origen
		n     int // qué comprobación es: solo vale la última
		lista []contrato.Adaptador
		err   error
	}
	msgModelos struct {
		origen
		agente string
		ids    []string
		err    error
	}
	msgAccion struct {
		origen
		exito string // lo que se avisa si salió bien
		err   error
	}
)

// pendiente es lo que la página va a hacer si el usuario confirma.
type pendiente struct{ agente, accion string }

const (
	accionDesconectar = "desconectar"
	accionSuscripcion = "suscripcion"
)

// Pagina es la página Agentes.
type Pagina struct {
	c        pagina.Contexto
	gen      int
	lengua   string
	estilos  *pagina.Estilos
	lista    comp.Lista[contrato.Adaptador]
	visor    comp.Visor
	agentes  []contrato.Adaptador
	n        int  // la última comprobación pedida
	cargando bool // hay una comprobación en marcha
	cargado  bool // ya hay una lista que enseñar
	sinServ  bool
	err      string
	ocupado  bool // hay un cambio en marcha: no se lanza otro
	pend     *pendiente
	detalle  string // el agente del detalle abierto, "" si no hay
	filtro   int    // el de filtros: 0 todos
	modelos  map[string][]string
	errModel map[string]string
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina {
	return &Pagina{lengua: lengua, modelos: map[string][]string{}, errModel: map[string]string{}}
}

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.agentes") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaAgentes }
func (p *Pagina) EnCampo() bool  { return false }

func (p *Pagina) Teclas() []pagina.Tecla {
	if p.detalle != "" {
		return []pagina.Tecla{{Tecla: "Esc", Accion: idioma.T("vuelve")}, {Tecla: "c", Accion: idioma.T("comprobar")},
			{Tecla: "d", Accion: idioma.T("conexión")}, {Tecla: "a", Accion: idioma.T("autenticación")}, {Tecla: "p", Accion: p.tx("menu.ejecutar")}}
	}
	return []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("detalles")}, {Tecla: "← →", Accion: idioma.T("filtro")}, {Tecla: "c", Accion: idioma.T("comprobar")},
		{Tecla: "d", Accion: idioma.T("conexión")}, {Tecla: "a", Accion: idioma.T("autenticación")}, {Tecla: "p", Accion: p.tx("menu.ejecutar")}}
}

// Iniciar empieza de cero: «Cargando…» hasta que llegue la comprobación.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	p.c, p.lengua = c, c.Idioma
	p.gen++
	if p.estilos != c.Estilos { // los mismos toda la sesión: la lista y el visor se crean una vez
		p.estilos = c.Estilos
		p.lista = comp.NuevaLista(c.Estilos, func(a contrato.Adaptador) string { return nombre(a.Agente) + " " + a.Agente })
		p.lista.Clave = func(a contrato.Adaptador) string { return a.Agente }
		p.lista.Fila = p.fila
		p.visor = comp.NuevoVisor(c.Estilos)
	}
	p.visor.Tamano(c.Ancho, max(c.Alto-2, 1))
	p.cargado, p.sinServ, p.err, p.detalle, p.pend, p.ocupado = false, false, "", "", nil, false
	p.modelos, p.errModel = map[string][]string{}, map[string]string{} // el catálogo es del espacio: el de antes no se ve
	return p.comprobar()
}

// comprobar pide el estado de todos los agentes (la API ejecuta cada CLI, así que tarda).
func (p *Pagina) comprobar() tea.Cmd {
	c := p.c
	if c.Cli == nil {
		p.cargando = false
		return pagina.SinCliente
	}
	p.n++
	n := p.n
	p.cargando, p.err = true, ""
	o := p.origen()
	return func() tea.Msg {
		r, err := c.Cli.Capacidades(c.Ctx, c.Instancia)
		return msgAgentes{origen: o, n: n, lista: r.Adaptadores, err: err}
	}
}

func (p *Pagina) origen() origen        { return origen{p.c.Origen(p.gen)} }
func (p *Pagina) vigente(o origen) bool { return o.Vigente(p.c, p.gen) }

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
		p.visor.Tamano(m.Width, max(m.Height-2, 1))
	case msgAgentes:
		if !p.vigente(m.origen) || m.n != p.n {
			return p, nil
		}
		p.cargando = false
		if m.err != nil {
			if cmd := pagina.SinServicio(m.err); cmd != nil {
				p.sinServ = true
				return p, cmd
			}
			p.err = nucleo.MensajeDe(m.err)
			return p, nil
		}
		p.sinServ, p.cargado, p.agentes = false, true, ordenar(m.lista)
		p.lista.Poner(p.agentes)
		p.refrescar()
	case msgModelos:
		if !p.vigente(m.origen) {
			return p, nil
		}
		if m.err != nil {
			if cmd := pagina.SinServicio(m.err); cmd != nil {
				return p, cmd
			}
			p.errModel[m.agente] = nucleo.MensajeDe(m.err)
		} else {
			p.modelos[m.agente] = m.ids
			delete(p.errModel, m.agente)
		}
		p.refrescar()
	case msgAccion:
		if !m.DelEspacio(p.c) { // el cambio ya está hecho en el servicio: vale aunque la página se haya ocultado
			return p, nil
		}
		p.ocupado = false
		if m.err != nil {
			if cmd := pagina.SinServicio(m.err); cmd != nil {
				return p, cmd
			}
			return p, comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
		}
		return p, tea.Batch(comp.Aviso(m.exito, comp.Exito), p.comprobar())
	case comp.Confirmado:
		pend := p.pend
		p.pend = nil
		if pend == nil {
			return p, nil
		}
		switch pend.accion {
		case accionDesconectar:
			return p, p.conectar(pend.agente, false)
		case accionSuscripcion:
			return p, p.autenticar(pend.agente, "suscripcion")
		}
	case comp.Cancelado:
		p.pend = nil
	case tea.KeyPressMsg:
		return p, p.tecla(m)
	}
	return p, nil
}

// tecla atiende una tecla: primero las de la página y, si no, la lista o el visor.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	t := k.String()
	a, hay := p.elegido()
	switch t {
	case "esc":
		if p.detalle != "" {
			p.detalle = ""
			return nil
		}
		return pagina.Atras
	case "c", "r":
		if p.cargando {
			return nil
		}
		return p.comprobar()
	case "enter":
		if p.detalle == "" && hay {
			return p.abrir(a)
		}
	case "d":
		if hay {
			return p.alternarConexion(a)
		}
	case "a":
		if hay {
			return p.alternarAutenticacion(a)
		}
	case "p":
		if hay {
			return p.probar(a)
		}
	case "left", "right":
		if p.detalle == "" && p.cargado {
			p.filtrar(map[string]int{"left": -1, "right": 1}[t])
			return nil
		}
	}
	if p.detalle != "" {
		p.visor.Update(k)
	} else if p.cargado {
		p.lista.Update(k)
	}
	return nil
}

// elegido es el agente del detalle abierto o, en la lista, el del cursor; con «Cargando…» no hay ninguno.
func (p *Pagina) elegido() (contrato.Adaptador, bool) {
	if p.detalle != "" {
		i := slices.IndexFunc(p.agentes, func(a contrato.Adaptador) bool { return a.Agente == p.detalle })
		if i < 0 {
			return contrato.Adaptador{}, false
		}
		return p.agentes[i], true
	}
	if !p.cargado {
		return contrato.Adaptador{}, false
	}
	return p.lista.Seleccion()
}

// abrir enseña los detalles del agente y pide su catálogo de modelos si aún no lo tiene.
func (p *Pagina) abrir(a contrato.Adaptador) tea.Cmd {
	p.detalle = a.Agente
	p.visor.Tamano(p.c.Ancho, max(p.c.Alto-2, 1))
	p.refrescar()
	c, o, agente := p.c, p.origen(), a.Agente
	if _, ya := p.modelos[agente]; ya || c.Cli == nil {
		return nil
	}
	return func() tea.Msg {
		ids, err := c.Cli.Modelos(c.Ctx, c.Instancia)
		var suyos []string
		for _, id := range ids {
			if id == agente || strings.HasPrefix(id, agente+"/") {
				suyos = append(suyos, id)
			}
		}
		return msgModelos{origen: o, agente: agente, ids: suyos, err: err}
	}
}

// refrescar vuelve a escribir el detalle abierto con lo último que se sabe.
func (p *Pagina) refrescar() {
	if a, ok := p.elegido(); ok && p.detalle != "" {
		p.visor.Poner(p.textoDetalle(a))
	} else if p.detalle != "" {
		p.detalle = "" // el agente ya no está
	}
}

// alternarConexion conecta un agente desconectado y, si está conectado, pide confirmar antes de desconectarlo.
func (p *Pagina) alternarConexion(a contrato.Adaptador) tea.Cmd {
	if p.ocupado {
		return nil
	}
	if a.Estado == contrato.AgenteNoInstalado { // sin CLI no hay nada que conectar ni desconectar
		return comp.Aviso(idioma.T("Instala su CLI antes: no hay nada que conectar ni desconectar"), comp.Advertencia)
	}
	if a.Desconectado {
		return p.conectar(a.Agente, true)
	}
	p.pend = &pendiente{agente: a.Agente, accion: accionDesconectar}
	return comp.Confirmar(p.tx("agentes.desconectarTitulo", "agente", nombre(a.Agente)) + "\n\n" + p.tx("agentes.desconectarTexto"))
}

func (p *Pagina) conectar(agente string, conectado bool) tea.Cmd {
	exito := p.tx("agentes.desconectado", "agente", nombre(agente))
	if conectado {
		exito = p.tx("agentes.conectado", "agente", nombre(agente))
	}
	return p.accion(exito, func(c pagina.Contexto) error { return c.Cli.ConectarAgente(c.Ctx, agente, conectado) })
}

func (p *Pagina) autenticar(agente, modo string) tea.Cmd {
	exito := p.tx("agentes.claveActivada", "agente", nombre(agente))
	if modo == "suscripcion" {
		exito = p.tx("agentes.suscripcionActivada", "agente", nombre(agente))
	}
	return p.accion(exito, func(c pagina.Contexto) error { return c.Cli.CambiarAutenticacion(c.Ctx, agente, modo) })
}

// accion lanza un cambio en el servicio; al volver, la página avisa y vuelve a comprobar.
func (p *Pagina) accion(exito string, hacer func(pagina.Contexto) error) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	p.ocupado = true
	return func() tea.Msg {
		return msgAccion{origen: o, exito: exito, err: hacer(c)}
	}
}

// alternarAutenticacion pasa de suscripción a clave de API o al revés, solo en los agentes que admiten la suscripción y
// en los estados en que la Web UI ofrece el cambio. Pasar a la suscripción pide aceptar su riesgo.
func (p *Pagina) alternarAutenticacion(a contrato.Adaptador) tea.Cmd {
	if p.ocupado {
		return nil
	}
	nom := nombre(a.Agente)
	if !catalogo.AdmiteSuscripcion(a.Agente) {
		return comp.Aviso(idioma.Tf("%s no admite la suscripción: solo usa clave de API", nom), comp.Advertencia)
	}
	est := a.Estado
	if est == contrato.AgenteDesconectado || est == contrato.AgenteNoInstalado {
		return comp.Aviso(idioma.T("Tiene que estar instalado y conectado para cambiar su autenticación"), comp.Advertencia)
	}
	if a.Autenticacion == "suscripcion" {
		return p.autenticar(a.Agente, "clave_api")
	}
	if est != contrato.AgenteListo && est != contrato.AgenteFaltaClave {
		return comp.Aviso(idioma.T("En su estado actual solo se puede usar la clave de API"), comp.Advertencia)
	}
	p.pend = &pendiente{agente: a.Agente, accion: accionSuscripcion}
	// Cabe en 60×20: el título, el riesgo y la aceptación. Lo demás, que es para uso personal en este equipo, está en el detalle.
	return comp.Confirmar(p.tx("agentes.confirmarTitulo", "agente", nom) + "\n\n" + p.tx("agentes.riesgoSuscripcion") + "\n\n" +
		idioma.T("Pulsa s para confirmar:") + " " + p.tx("agentes.aceptoRiesgo"))
}

// probar lleva a Probar con este agente; solo uno listo se puede probar.
func (p *Pagina) probar(a contrato.Adaptador) tea.Cmd {
	if a.Estado != contrato.AgenteListo {
		return comp.Aviso(idioma.T("Este agente aún no está listo para probar"), comp.Advertencia)
	}
	return func() tea.Msg {
		return pagina.IrA{Pagina: pagina.PaginaEjecutar, Datos: pagina.ConAgente{Agente: a.Agente}}
	}
}
