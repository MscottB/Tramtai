package ejecutar

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
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
	icono, estilo := p.icono(t.Estado)
	estado := e.Recortar(icono+" "+p.a(p.nombreEstado(t.Estado)), 14)
	estado += strings.Repeat(" ", max(14-lipgloss.Width(estado), 0))
	cuando := p.hace(t.CreadoEn)
	texto := strings.Join(strings.Fields(mensajeDe(t)), " ")
	if texto == "" {
		texto = p.tx("trabajo.entradaEliminada")
	}
	resto := max(ancho-14-lipgloss.Width(cuando)-2, 1)
	texto = e.Recortar(p.a(texto), resto)
	return estilo(estado) + " " + texto + strings.Repeat(" ", max(resto-lipgloss.Width(texto), 0)) + " " + e.Tenue.Render(cuando)
}

// hace es «hace 3 min»; pasada una semana, la fecha.
func (p *Pagina) hace(t time.Time) string {
	d := p.ahora().Sub(t)
	switch {
	case t.IsZero():
		return "-"
	case d < time.Minute:
		return idioma.Tf("hace %d s", max(int(d/time.Second), 0))
	case d < time.Hour:
		return idioma.Tf("hace %d min", int(d/time.Minute))
	case d < 24*time.Hour:
		return idioma.Tf("hace %d h", int(d/time.Hour))
	case d < 7*24*time.Hour:
		return idioma.Tf("hace %d d", int(d/(24*time.Hour)))
	}
	return t.Local().Format("2006-01-02")
}

// cargarPunto lee la API que se abre en Probar para editarla (Abrir en Probar de Mis APIs).
func (p *Pagina) cargarPunto(nombre string) tea.Cmd {
	if p.c.Cli == nil {
		return nil
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
