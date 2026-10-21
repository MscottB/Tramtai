// Package agentes es la página Agentes de la terminal: los asistentes que Tramtai usa, con el estado que les da la Web UI
// (listo, sin conectar, sin instalar, desconectado o por revisar), y las mismas acciones: comprobar, conectar o
// desconectar, cambiar entre suscripción y clave de API, ver sus detalles y probarlo.
package agentes

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// adaptador es un agente tal como lo cuenta /api/v1/capacidades.
type adaptador struct {
	Agente         string `json:"agente"`
	Desconectado   bool   `json:"desconectado"`
	Version        string `json:"version_detectada"`
	Autenticacion  string `json:"autenticacion"`
	Terminos       string `json:"terminos"`
	Cuenta         string `json:"cuenta"`
	Problema       string `json:"problema"`
	ProblemaCodigo string `json:"problema_codigo"`
	Capacidades    struct {
		Streaming               string `json:"streaming"`
		SalidaEstructurada      bool   `json:"salida_estructurada"`
		ModeloEfectivoInformado bool   `json:"modelo_efectivo_informado"`
	} `json:"capacidades"`
}

type (
	// Mensajes propios: cada uno lleva el Ctx y la instancia con que se pidió, y se descarta si ya no valen.
	msgAgentes struct {
		ctx       context.Context
		instancia string
		n         int // qué comprobación es: solo vale la última
		lista     []adaptador
		err       error
	}
	msgModelos struct {
		ctx       context.Context
		instancia string
		agente    string
		ids       []string
		err       error
	}
	msgAccion struct {
		ctx       context.Context
		instancia string
		exito     string // lo que se avisa si salió bien
		err       error
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
	lengua   string
	estilos  *pagina.Estilos
	lista    comp.Lista[adaptador]
	visor    comp.Visor
	agentes  []adaptador
	n        int  // la última comprobación pedida
	cargando bool // hay una comprobación en marcha
	cargado  bool // ya hay una lista que enseñar
	sinServ  bool // el servicio no responde
	err      string
	ocupado  bool // hay un cambio en marcha: no se lanza otro
	pend     *pendiente
	detalle  string // el agente del detalle abierto, "" si no hay
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
			{Tecla: "d", Accion: idioma.T("conexión")}, {Tecla: "a", Accion: idioma.T("autenticación")}, {Tecla: "p", Accion: idioma.T("probar")}}
	}
	return []pagina.Tecla{{Tecla: idioma.T("Intro"), Accion: idioma.T("detalles")}, {Tecla: "c", Accion: idioma.T("comprobar")},
		{Tecla: "d", Accion: idioma.T("conexión")}, {Tecla: "a", Accion: idioma.T("autenticación")}, {Tecla: "p", Accion: idioma.T("probar")}}
}

// Iniciar empieza de cero: «Cargando…» hasta que llegue la comprobación.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	p.c, p.lengua = c, c.Idioma
	if p.estilos != c.Estilos { // los mismos toda la sesión: la lista y el visor se crean una vez
		p.estilos = c.Estilos
		p.lista = comp.NuevaLista(c.Estilos, func(a adaptador) string { return nombre(a.Agente) + " " + a.Agente })
		p.lista.Clave = func(a adaptador) string { return a.Agente }
		p.lista.Fila = p.fila
		p.visor = comp.NuevoVisor(c.Estilos)
	}
	p.visor.Tamano(c.Ancho, max(c.Alto-2, 1))
	p.cargado, p.sinServ, p.err, p.detalle, p.pend, p.ocupado = false, false, "", "", nil, false
	return p.comprobar()
}

// comprobar pide el estado de todos los agentes (la API ejecuta cada CLI, así que tarda).
func (p *Pagina) comprobar() tea.Cmd {
	c := p.c
	if c.Cli == nil {
		p.cargando = false
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	p.n++
	n := p.n
	p.cargando, p.err = true, ""
	return func() tea.Msg {
		r, err := c.Cli.Capacidades(c.Ctx, c.Instancia)
		return msgAgentes{ctx: c.Ctx, instancia: c.Instancia, n: n, lista: leer(r), err: err}
	}
}

// leer saca los adaptadores de la respuesta, que el cliente entrega sin tipo.
func leer(r map[string]any) []adaptador {
	var lista []adaptador
	if bruto, err := json.Marshal(r["adaptadores"]); err == nil {
		_ = json.Unmarshal(bruto, &lista)
	}
	return lista
}

func (p *Pagina) vigente(ctx context.Context, instancia string) bool {
	return ctx != nil && ctx.Err() == nil && instancia == p.c.Instancia
}

// fallo reparte un error: de conexión es del servicio (lo avisa la app); el de la API, el mensaje tal cual.
func fallo(err error) (tea.Cmd, bool) {
	if pagina.NoResponde(err) {
		return func() tea.Msg { return pagina.ErrorServicio{Err: err} }, true
	}
	return nil, false
}

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
		p.visor.Tamano(m.Width, max(m.Height-2, 1))
	case msgAgentes:
		if !p.vigente(m.ctx, m.instancia) || m.n != p.n {
			return p, nil
		}
		p.cargando = false
		if m.err != nil {
			if cmd, ok := fallo(m.err); ok {
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
		if !p.vigente(m.ctx, m.instancia) {
			return p, nil
		}
		if m.err != nil {
			if cmd, ok := fallo(m.err); ok {
				return p, cmd
			}
			p.errModel[m.agente] = nucleo.MensajeDe(m.err)
		} else {
			p.modelos[m.agente] = m.ids
			delete(p.errModel, m.agente)
		}
		p.refrescar()
	case msgAccion:
		if !p.vigente(m.ctx, m.instancia) {
			return p, nil
		}
		p.ocupado = false
		if m.err != nil {
			if cmd, ok := fallo(m.err); ok {
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
	}
	if p.detalle != "" {
		p.visor.Update(k)
	} else if p.cargado {
		p.lista.Update(k)
	}
	return nil
}

// elegido es el agente del detalle abierto o, en la lista, el del cursor; con «Cargando…» no hay ninguno.
func (p *Pagina) elegido() (adaptador, bool) {
	if p.detalle != "" {
		i := slices.IndexFunc(p.agentes, func(a adaptador) bool { return a.Agente == p.detalle })
		if i < 0 {
			return adaptador{}, false
		}
		return p.agentes[i], true
	}
	if !p.cargado {
		return adaptador{}, false
	}
	return p.lista.Seleccion()
}

// abrir enseña los detalles del agente y pide su catálogo de modelos si aún no lo tiene.
func (p *Pagina) abrir(a adaptador) tea.Cmd {
	p.detalle = a.Agente
	p.visor.Tamano(p.c.Ancho, max(p.c.Alto-2, 1))
	p.refrescar()
	c, agente := p.c, a.Agente
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
		return msgModelos{ctx: c.Ctx, instancia: c.Instancia, agente: agente, ids: suyos, err: err}
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
func (p *Pagina) alternarConexion(a adaptador) tea.Cmd {
	if p.ocupado {
		return nil
	}
	if a.Desconectado {
		return p.conectar(a.Agente, true)
	}
	p.pend = &pendiente{agente: a.Agente, accion: accionDesconectar}
	return comp.Confirmar(idioma.Tf("¿Desconectar %s?", nombre(a.Agente)) + "\n\n" +
		idioma.T("No se usará en ningún espacio, ni como respaldo: las peticiones que lo pidan reciben agente_desconectado. Lo vuelves a conectar cuando quieras."))
}

func (p *Pagina) conectar(agente string, conectado bool) tea.Cmd {
	exito := idioma.Tf("%s está desconectado", nombre(agente))
	if conectado {
		exito = idioma.Tf("%s está conectado", nombre(agente))
	}
	return p.accion(exito, func(c pagina.Contexto) error { return c.Cli.ConectarAgente(c.Ctx, agente, conectado) })
}

func (p *Pagina) autenticar(agente, modo string) tea.Cmd {
	exito := idioma.Tf("%s usa ahora la clave de API", nombre(agente))
	if modo == "suscripcion" {
		exito = idioma.Tf("%s usa ahora tu suscripción", nombre(agente))
	}
	return p.accion(exito, func(c pagina.Contexto) error { return c.Cli.CambiarAutenticacion(c.Ctx, agente, modo) })
}

// accion lanza un cambio en el servicio; al volver, la página avisa y vuelve a comprobar.
func (p *Pagina) accion(exito string, hacer func(pagina.Contexto) error) tea.Cmd {
	c := p.c
	if c.Cli == nil {
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	p.ocupado = true
	return func() tea.Msg {
		return msgAccion{ctx: c.Ctx, instancia: c.Instancia, exito: exito, err: hacer(c)}
	}
}

// alternarAutenticacion pasa de suscripción a clave de API o al revés, solo en los agentes que admiten la suscripción y
// en los estados en que la Web UI ofrece el cambio. Pasar a la suscripción pide aceptar su riesgo.
func (p *Pagina) alternarAutenticacion(a adaptador) tea.Cmd {
	if p.ocupado {
		return nil
	}
	nom := nombre(a.Agente)
	if !catalogo.AdmiteSuscripcion(a.Agente) {
		return comp.Aviso(idioma.Tf("%s no admite la suscripción: solo usa clave de API", nom), comp.Advertencia)
	}
	est := estadoDe(a)
	if est == estDesconectado || est == estNoInstalado {
		return comp.Aviso(idioma.T("Tiene que estar instalado y conectado para cambiar su autenticación"), comp.Advertencia)
	}
	if a.Autenticacion == "suscripcion" {
		return p.autenticar(a.Agente, "clave_api")
	}
	if est != estListo && est != estFaltaClave {
		return comp.Aviso(idioma.T("En su estado actual solo se puede usar la clave de API"), comp.Advertencia)
	}
	p.pend = &pendiente{agente: a.Agente, accion: accionSuscripcion}
	return comp.Confirmar(idioma.Tf("Usar tu suscripción con %s", nom) + "\n\n" +
		idioma.Tf("Tramtai usará la sesión de %s que ya tienes iniciada en este equipo. Según los términos del proveedor es solo para uso personal y local: no la compartas con otras personas.", programa(a.Agente)) + "\n\n" +
		idioma.T("Los términos de cada proveedor pueden no permitir usar una suscripción personal de forma automatizada o para alimentar otras aplicaciones, y el proveedor podría limitar o suspender tu cuenta. Ese riesgo es tuyo. Para tus aplicaciones, lo recomendado es una clave de API. Más detalle en los términos de uso de Tramtai (github.com/MscottB/Tramtai/blob/main/TERMINOS.md).") + "\n\n" +
		idioma.T("Pulsar s significa que entiendes el riesgo y aceptas los términos de uso."))
}

// probar lleva a Probar con este agente; solo uno listo se puede probar.
func (p *Pagina) probar(a adaptador) tea.Cmd {
	if estadoDe(a) != estListo {
		return comp.Aviso(idioma.T("Este agente aún no está listo para probar"), comp.Advertencia)
	}
	return func() tea.Msg {
		return pagina.IrA{Pagina: pagina.PaginaEjecutar, Datos: pagina.ConAgente{Agente: a.Agente}}
	}
}
