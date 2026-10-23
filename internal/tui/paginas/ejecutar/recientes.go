package ejecutar

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

// cargarRecientes pide las últimas pruebas lanzadas desde la terminal (las de la Web UI y las APIs están en el Historial).
func (p *Pagina) cargarRecientes() tea.Cmd {
	if p.c.Cli == nil {
		return nil
	}
	c, o := p.c, p.origen()
	return func() tea.Msg {
		l, err := c.Cli.TrabajosFiltrados(c.Ctx, c.Instancia, cliente.FiltroTrabajos{Limite: limiteRecientes, Origen: nucleo.DesdeTerminal})
		return msgRecientes{origen: o, lista: l, err: err}
	}
}

// alRecientes pone la lista; si falla se queda la de antes, como la Web UI.
func (p *Pagina) alRecientes(m msgRecientes) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	if m.err != nil {
		if pagina.NoResponde(m.err) {
			return p.fallo(m.err)
		}
		return nil
	}
	p.cargadoRec = true
	p.recientes.Poner(m.lista)
	return nil
}

// usarDeNuevo (u) pone el mensaje y la receta de una prueba reciente; los adjuntos no se recuperan.
func (p *Pagina) usarDeNuevo(t nucleo.Trabajo) tea.Cmd {
	texto := mensajeDe(t)
	if texto == "" {
		return comp.Aviso(p.tx("trabajo.entradaEliminada"), comp.Advertencia)
	}
	p.cargarReceta(recetaDeSolicitud(t.Solicitud), texto)
	p.api, p.punto = "", nil
	p.pestana = pestReceta
	cmd := p.programarBorrador()
	if len(t.Solicitud.Cargas) > 0 {
		return tea.Batch(cmd, comp.Aviso(p.tx("ejecutar.sinAdjuntos"), comp.Info))
	}
	return cmd
}

// filaReciente es una prueba reciente: el estado, lo que se pidió y hace cuánto.
func (p *Pagina) filaReciente(t nucleo.Trabajo) string {
	e := p.estilos
	ancho := max(p.c.Ancho-2, 10)
	icono, estilo := vivo.Icono(e, t.Estado)
	estado := e.Recortar(icono+" "+p.nombreEstado(t.Estado), 14)
	estado += strings.Repeat(" ", max(14-lipgloss.Width(estado), 0))
	cuando := p.a(vivo.Hace(p.ahora(), t.CreadoEn))
	texto := strings.Join(strings.Fields(mensajeDe(t)), " ")
	if texto == "" {
		texto = p.tx("trabajo.entradaEliminada")
	}
	resto := max(ancho-14-lipgloss.Width(cuando)-2, 1)
	texto = e.Recortar(p.a(texto), resto)
	return estilo.Render(estado) + " " + texto + strings.Repeat(" ", max(resto-lipgloss.Width(texto), 0)) + " " + e.Tenue.Render(cuando)
}

// cargarPunto lee la API que se abre en Probar para editarla (Abrir en Probar de Mis APIs).
func (p *Pagina) cargarPunto(nombre string) tea.Cmd {
	if p.c.Cli == nil {
		return sinServicio
	}
	c, o := p.c, p.origen()
	return func() tea.Msg {
		l, err := c.Cli.Puntos(c.Ctx, c.Instancia)
		m := msgPunto{origen: o, err: err}
		for _, e := range l.Puntos {
			if e.Nombre == nombre {
				m.p = &e
			}
		}
		return m
	}
}

func (p *Pagina) alPunto(m msgPunto) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	if m.err != nil {
		return p.fallo(m.err)
	}
	if m.p != nil {
		p.cargarReceta(recetaDePunto(*m.p), p.form.Valor("mensaje"))
		p.api, p.punto, p.pestana = m.p.Nombre, m.p, pestReceta
	}
	return nil
}
