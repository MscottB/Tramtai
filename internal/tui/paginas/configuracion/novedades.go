package configuracion

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

// Novedades y actualizar, como el diálogo de App.vue con useActualizacion.ts: las notas de la versión nueva (o las de
// cada una desde la tuya), cómo se instala y, al instalar, su avance cada 400 ms hasta que el instalador cierra el
// servicio. Todo pasa por la administración del servicio, que es quien descarga, comprueba la firma e instala.

const cadaAvance = 400 * time.Millisecond

// sinFaseMaximo son las consultas sin fase tras las que se deja de esperar sin haber visto ninguna (30 s, como
// abandonarSinFase de la Web UI): la consulta a la que esta se unió falló.
const sinFaseMaximo = 75

type (
	msgNovedades struct {
		origen
		v   []contrato.VersionNueva
		err error
	}
	msgActualizar struct {
		origen
		version string
		err     error
	}
	msgAvance struct {
		origen
		p   contrato.ProgresoActualizacion
		err error
	}
	msgSondeo   struct{ origen }
	msgDecision struct {
		origen
		instalar bool
		err      error
	}
)

// actualizacionEnCurso es lo de las novedades y la actualización que sigue la página.
type actualizacionEnCurso struct {
	quiere    bool                    // ConNovedades llegó antes que el estado: se abren al llegar
	novedades []contrato.VersionNueva // lo que trae cada versión desde la tuya
	progreso  *contrato.ProgresoActualizacion
	vistaFase bool // ya llegó alguna fase del servicio
	sinFase   int  // consultas seguidas sin fase
}

// pedirNovedades abre las novedades si ya se sabe que hay una versión nueva; si no, al saberlo.
func (p *Pagina) pedirNovedades() tea.Cmd {
	if p.act != nil && p.act.Disponible {
		return p.abrirNovedades()
	}
	p.upd.quiere = true
	return nil
}

// abrirNovedades enseña la versión nueva y pide lo que trae cada una desde la tuya.
func (p *Pagina) abrirNovedades() tea.Cmd {
	p.abierta, p.upd.novedades = vistaNovedades, nil
	p.visor.Poner("")
	c, o, act := p.c, p.origen(), *p.act
	return func() tea.Msg {
		v, err := c.Cli.Novedades(c.Ctx, act.Actual, act.Version)
		return msgNovedades{origen: o, v: v, err: err}
	}
}

func (p *Pagina) alActualizacion(m msgActualizacion) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	if m.err != nil {
		if !m.ahora { // sin el estado, la sección no sale
			return pagina.SinServicio(m.err)
		}
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			return cmd
		}
		return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
	}
	p.act = &m.e
	switch {
	case (m.ahora || p.upd.quiere) && m.e.Disponible:
		p.upd.quiere = false
		return p.abrirNovedades()
	case !m.ahora:
		return nil
	case m.e.Fallo:
		return comp.Aviso(p.tx("actualizacion.falloConsulta"), comp.Advertencia)
	}
	return comp.Aviso(p.tx("actualizacion.alDia", "actual", m.e.Actual), comp.Info)
}

// instalar empieza la actualización en el servicio (o se une a la que está en curso) y enseña su avance al momento.
func (p *Pagina) instalar() tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	p.abierta = vistaProgreso
	p.upd.progreso = &contrato.ProgresoActualizacion{Fase: "descargando", Version: p.act.Version}
	p.upd.vistaFase, p.upd.sinFase = false, 0
	return func() tea.Msg {
		v, err := c.Cli.Actualizar(c.Ctx)
		return msgActualizar{origen: o, version: v, err: err}
	}
}

func (p *Pagina) sondear() tea.Cmd {
	return pagina.Tras(p.origen(), cadaAvance, p.espera, func(o origen) tea.Msg { return msgSondeo{o} })
}

// decidir instala ya (true) o cancela (false) en la cuenta atrás.
func (p *Pagina) decidir(instalar bool) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	return func() tea.Msg {
		return msgDecision{origen: o, instalar: instalar, err: c.Cli.DecidirInstalacion(c.Ctx, instalar)}
	}
}

// cerrarAvance deja de seguir la actualización.
func (p *Pagina) cerrarAvance() {
	p.upd.progreso = nil
	if p.abierta == vistaProgreso {
		p.abierta = ""
	}
}

// updateActualizacion atiende las respuestas de las novedades y de la actualización.
func (p *Pagina) updateActualizacion(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case msgNovedades:
		if p.vigente(m.origen) && m.err == nil { // sin GitHub, las notas de la nueva bastan
			p.upd.novedades = m.v
		}
	case msgActualizar:
		if !p.vigente(m.origen) || p.abierta != vistaProgreso {
			return nil
		}
		if m.err != nil {
			p.upd.progreso, p.abierta = nil, vistaNovedades
			if cmd := pagina.SinServicio(m.err); cmd != nil {
				return cmd
			}
			return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
		}
		p.upd.progreso = &contrato.ProgresoActualizacion{Fase: "descargando", Version: m.version}
		return p.sondear()
	case msgSondeo:
		if p.vigente(m.origen) && p.abierta == vistaProgreso {
			c, o := p.c, m.origen
			return func() tea.Msg {
				a, err := c.Cli.ProgresoActualizacion(c.Ctx)
				return msgAvance{origen: o, p: a, err: err}
			}
		}
	case msgAvance:
		if !p.vigente(m.origen) || p.abierta != vistaProgreso {
			return nil
		}
		switch {
		case m.err != nil: // el instalador ya detuvo el servicio: queda el último estado hasta que la app se cierre
		case m.p.Fase != "":
			p.upd.progreso, p.upd.vistaFase, p.upd.sinFase = &m.p, true, 0
			if m.p.Fase == "error" {
				return nil
			}
		default:
			if p.upd.sinFase++; p.upd.vistaFase || p.upd.sinFase >= sinFaseMaximo {
				p.cerrarAvance() // otra pestaña la canceló, o la consulta a la que se unió falló: no queda nada que esperar
				return nil
			}
		}
		return p.sondear()
	case msgDecision:
		if !p.vigente(m.origen) || m.instalar { // instalar ya: el avance enseña lo que pase
			return nil
		}
		if m.err != nil {
			return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo) // ya estaba instalando: el avance lo dirá
		}
		p.cerrarAvance()
	}
	return nil
}

// teclaActualizacion: en las novedades, i instala; en el avance, i reinicia ya (o reintenta tras un error) y c cancela
// la cuenta atrás, preguntando antes.
func (p *Pagina) teclaActualizacion(t string) (tea.Cmd, bool) {
	fase := ""
	if p.upd.progreso != nil {
		fase = p.upd.progreso.Fase
	}
	switch {
	case p.abierta == vistaNovedades && t == "i" && p.act.Instalable, p.abierta == vistaProgreso && t == "i" && fase == "error":
		return p.instalar(), true
	case p.abierta == vistaProgreso && t == "i" && fase == "listo":
		return p.decidir(true), true
	case p.abierta == vistaProgreso && t == "c" && fase == "listo":
		p.pend = "cancelar"
		return comp.Confirmar(idioma.T("¿Cancelar la actualización?") + "\n\n" + idioma.T("La descarga se borra y Tramtai sigue como está.")), true
	}
	return nil, false
}

func (p *Pagina) teclasActualizacion() []pagina.Tecla {
	switch {
	case p.abierta == vistaNovedades && p.act != nil && p.act.Instalable:
		return []pagina.Tecla{{Tecla: "i", Accion: p.tx("actualizacion.instalar")}}
	case p.abierta != vistaProgreso || p.upd.progreso == nil:
		return nil
	case p.upd.progreso.Fase == "listo":
		return []pagina.Tecla{{Tecla: "i", Accion: p.tx("actualizacion.reiniciarAhora")}, {Tecla: "c", Accion: p.tx("comun.cancelar")}}
	case p.upd.progreso.Fase == "error":
		return []pagina.Tecla{{Tecla: "i", Accion: p.tx("actualizacion.reintentar")}}
	}
	return nil
}

// megas escribe bytes en MB con un decimal, como megas de la Web UI.
func (p *Pagina) megas(bytes int64) string {
	s := strconv.FormatFloat(float64(bytes)/1048576, 'f', 1, 64)
	if p.lengua != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

// textoProgreso dice en qué fase va, como textoProgreso de la Web UI.
func (p *Pagina) textoProgreso(a contrato.ProgresoActualizacion) string {
	switch a.Fase {
	case "error":
		return p.tx("actualizacion.fallo", "error", a.Error)
	case "verificando":
		return p.tx("actualizacion.verificando")
	case "instalando":
		return p.tx("actualizacion.instalando")
	case "listo":
		n := 0
		if a.InstalaEn != nil {
			n = max(0, int(math.Ceil(a.InstalaEn.Sub(p.ahora()).Seconds())))
		}
		return p.tx("actualizacion.reiniciaraEn", "n", strconv.Itoa(n))
	}
	if a.Total == 0 {
		return p.tx("actualizacion.descargandoSinTotal", "recibido", p.megas(a.Recibidos))
	}
	return p.tx("actualizacion.descargando", "recibido", p.megas(a.Recibidos), "total", p.megas(a.Total),
		"porcentaje", strconv.FormatInt(a.Recibidos*100/a.Total, 10))
}

var (
	ingles = regexp.MustCompile(`(?m)^## English[ \t]*\r?$`)
	// lineasFuera son las de las notas que no se enseñan: el título, el «desde» y el enlace a los cambios técnicos.
	lineasFuera = regexp.MustCompile(`^(Tramtai v\S+|Cambios desde .*:|Full list of changes.*|(Cambios técnicos|Technical changes): https?:.*)$`)
)

// notas son las de una versión en el idioma de la terminal, como NotasVersion.vue: desde la v0.44 llevan «## English»;
// en inglés se enseña ese bloque y las anteriores se ven en español. Sin el título ni el enlace a los cambios técnicos.
func (p *Pagina) notas(texto string) string {
	partes := ingles.Split(texto, 2)
	elegido := partes[0]
	if p.lengua == "en" && len(partes) == 2 && strings.TrimSpace(partes[1]) != "" {
		elegido = partes[1]
	}
	var l []string
	for _, linea := range strings.Split(strings.ReplaceAll(elegido, "\r", ""), "\n") {
		if !lineasFuera.MatchString(strings.TrimSpace(linea)) {
			l = append(l, strings.TrimRight(linea, " "))
		}
	}
	return strings.TrimSpace(strings.Join(l, "\n"))
}

// textoNovedades es el de la versión nueva: qué tienes, qué trae y cómo se instala.
func (p *Pagina) textoNovedades() string {
	e, a := p.estilos, p.act
	l := []string{e.Titulo.Render(p.tx("actualizacion.titulo", "version", a.Version)), e.Tenue.Render(p.tx("actualizacion.detalle", "actual", a.Actual)), ""}
	if n := p.upd.novedades; len(n) > 1 {
		clave := "actualizacion.novedadesVarias"
		if len(n) >= 10 {
			clave = "actualizacion.novedadesRecientes"
		}
		l = append(l, e.Tenue.Render(p.tx(clave, "n", strconv.Itoa(len(n)))))
		for _, v := range n {
			l = append(l, "", e.Acento.Render(v.Version), p.notas(v.Notas))
		}
	} else if notas := p.notas(a.Notas); notas != "" {
		l = append(l, e.Tenue.Render(p.tx("actualizacion.novedadesTitulo")), notas)
	}
	l = append(l, "")
	if a.Instalable {
		l = append(l, p.tx("actualizacion.comoInstala"))
	} else {
		l = append(l, p.tx("actualizacion.manual"))
		if a.URL != "" {
			l = append(l, e.Acento.Render(a.URL))
		}
	}
	return strings.Join(l, "\n")
}

// textoAvance es el del avance: la versión, la fase y, al instalar, que la app se cerrará.
func (p *Pagina) textoAvance() string {
	e, a := p.estilos, p.upd.progreso
	l := []string{e.Titulo.Render(p.tx("actualizacion.progresoTitulo", "version", a.Version)), ""}
	estilo := e.Texto
	if a.Fase == "error" {
		estilo = e.Error
	}
	l = append(l, estilo.Render(p.textoProgreso(*a)))
	if a.Fase == "instalando" {
		l = append(l, "", e.Tenue.Render(p.tx("actualizacion.cerrara")))
	}
	return strings.Join(l, "\n")
}
