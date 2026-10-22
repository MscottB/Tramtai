package historial

import (
	"cmp"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
)

// origenesConocidos y claveOrigen son los orígenes del filtro de la Web UI (nombresOrigen de Trabajos.vue) y sus textos.
var origenesConocidos = []string{"probar", "terminal", "cli", "app"}

var claveOrigen = map[string]string{"probar": "historial.origenProbar", "terminal": "historial.origenTerminal",
	"cli": "historial.origenCli", "app": "historial.origenApp"}

func itoa(n int) string { return strconv.Itoa(n) }

// rellenar completa con espacios hasta ancho columnas visibles.
func rellenar(texto string, ancho int) string {
	return texto + strings.Repeat(" ", max(ancho-lipgloss.Width(texto), 0))
}

// aDerecha alinea el texto a la derecha en ancho columnas visibles.
func aDerecha(texto string, ancho int) string {
	return strings.Repeat(" ", max(ancho-lipgloss.Width(texto), 0)) + texto
}

// textoDe es lo que se pidió: el último mensaje, o el aviso de que la retención borró la entrada (textoDe de formato.ts).
func textoDe(t nucleo.Trabajo, lengua string) string {
	if n := len(t.Solicitud.Mensajes); n > 0 {
		return t.Solicitud.Mensajes[n-1].Contenido
	}
	if t.EntradasEliminadas {
		return datos.Texto(lengua, "trabajo.entradaEliminada")
	}
	return ""
}

// nombreAgente y modeloDe son los de la Web UI (agentes.ts): el agente por su nombre visible y el modelo con que respondió.
func nombreAgente(agente string) string { return catalogo.NombreVisible(agente) }

func modeloDe(t nucleo.Trabajo) string {
	var partes []string
	if t.Solicitud.Agente != "" {
		partes = append(partes, nombreAgente(t.Solicitud.Agente))
	}
	if m := cmp.Or(t.ModeloEfectivo, t.ModeloEnviado); m != "" {
		partes = append(partes, m)
	}
	if len(partes) == 0 {
		return "—"
	}
	return strings.Join(partes, " · ")
}

// nombreEstado es el nombre del estado en la Web UI (estado.<estado>); si no lo conoce, el estado tal cual.
func (p *Pagina) nombreEstado(e nucleo.Estado) string {
	if t := p.tx("estado." + string(e)); t != "estado."+string(e) {
		return t
	}
	return string(e)
}

// etiquetaEstado es el del filtro de estado: los nombres del selector de Trabajos.vue.
func (p *Pagina) etiquetaEstado() string {
	switch p.estado {
	case estCurso:
		return p.tx("inicio.enCurso")
	case estOk:
		return p.tx("inicio.completados")
	case estError:
		return p.tx("inicio.conError")
	}
	return p.tx("historial.todos")
}

// etiquetaOrigen es la del filtro de origen: todos, un origen, un grupo o una API.
func (p *Pagina) etiquetaOrigen() string {
	if len(p.origenF) > 2 {
		nombre := p.origenF[2:]
		switch p.origenF[:2] {
		case "o:":
			if clave, ok := claveOrigen[nombre]; ok {
				return p.tx(clave)
			}
		case "g:":
			return p.tx("historial.grupo", "nombre", nombre)
		case "p:":
			return p.tx("historial.api", "nombre", nombre)
		}
	}
	return p.tx("historial.todosOrigenes")
}

// textoOrigen dice de dónde llegó una petición: la API (con su grupo actual), el origen guardado o, en las antiguas, la
// principal (textoOrigen de Trabajos.vue).
func (p *Pagina) textoOrigen(t nucleo.Trabajo) string {
	if t.PuntoAcceso != "" {
		texto := p.tx("historial.api", "nombre", t.PuntoAcceso)
		for _, pt := range p.puntos {
			if pt.Nombre == t.PuntoAcceso && pt.Grupo != "" {
				return texto + " · " + pt.Grupo
			}
		}
		return texto
	}
	if clave, ok := claveOrigen[t.Origen]; ok {
		return p.tx(clave)
	}
	return p.tx("historial.origenPrincipal")
}

// icono es el dibujo del estado de una petición y su estilo.
func (p *Pagina) icono(e nucleo.Estado) (string, lipgloss.Style) {
	est := p.estilos
	switch {
	case !e.Terminal():
		return est.Iconos.Activo, est.Acento
	case e == nucleo.Completado:
		return est.Iconos.Ok, est.Exito
	case e == nucleo.Cancelado:
		return est.Iconos.Inactivo, est.Tenue
	case e == nucleo.Interrumpido:
		return est.Iconos.Fallo, est.Advertencia
	}
	return est.Iconos.Fallo, est.Error
}

// hace es «hace 3 min»; pasada una semana, la fecha.
func (p *Pagina) hace(t time.Time) string {
	d := p.ahora().Sub(t)
	switch {
	case t.IsZero():
		return "—"
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

// fila es una línea de la lista: el estado, lo que se pidió, el agente y modelo (si cabe) y hace cuánto.
func (p *Pagina) fila(t nucleo.Trabajo) string {
	e := p.estilos
	ancho := max(p.c.Ancho-2, 1) // la lista pone el cursor delante
	icono, _ := p.icono(t.Estado)
	estado := rellenar(e.Recortar(icono+" "+p.a(p.nombreEstado(t.Estado)), 14), 14)
	cuando := aDerecha(p.hace(t.CreadoEn), 10)
	var modelo string
	if ancho >= 90 {
		modelo = rellenar(e.Recortar(modeloDe(t), 24), 24) + " "
	}
	libre := max(ancho-lipgloss.Width(estado)-1-lipgloss.Width(modelo)-lipgloss.Width(cuando)-1, 4)
	texto := strings.Join(strings.Fields(textoDe(t, p.lengua)), " ")
	return p.a(estado + " " + rellenar(e.Recortar(texto, libre), libre) + " " + modelo + cuando)
}

// barra es la línea de los filtros: el estado y el origen de ahora y, si hay, el texto buscado; con la búsqueda abierta, lo
// que se escribe.
func (p *Pagina) barra() string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	if p.buscando && !p.enVisor {
		return e.Recortar("/ "+p.entrada+"_", ancho)
	}
	partes := []string{p.tx("historial.estado") + ": " + p.etiquetaEstado(), p.tx("historial.origen") + ": " + p.etiquetaOrigen()}
	if p.busqueda != "" {
		partes = append(partes, "/ "+p.busqueda)
	}
	return e.Tenue.Render(e.Recortar(strings.Join(partes, " "+e.Iconos.Separador+" "), ancho))
}

// vacio es lo que se ve sin filas: que no hay peticiones, o ninguna con este filtro, y qué hacer (EstadoVacio de la Web UI).
func (p *Pagina) vacio() []string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	if p.hayFiltro() {
		return []string{e.Tenue.Render(e.Recortar(p.tx("historial.sinFiltro"), ancho)),
			e.Recortar(e.Tecla.Render("l")+" "+e.Tenue.Render(p.tx("historial.quitarFiltros")), ancho)}
	}
	return []string{e.Tenue.Render(e.Recortar(p.tx("historial.vacio"), ancho)),
		e.Recortar(e.Tecla.Render("p")+" "+e.Tenue.Render(p.tx("historial.irProbar")), ancho)}
}

// Vista es la lista con sus filtros, el detalle de una petición o el selector de origen.
func (p *Pagina) Vista() string {
	e := p.estilos
	if e == nil {
		return ""
	}
	switch {
	case p.selector != nil:
		return p.selector.Vista(max(p.c.Ancho, 1), max(p.c.Alto, 1))
	case p.abierto != "":
		return p.vistaDetalle()
	}
	return p.vistaLista()
}

func (p *Pagina) vistaLista() string {
	e, ancho, alto := p.estilos, max(p.c.Ancho, 1), max(p.c.Alto, 1)
	l := []string{e.Titulo.Render(e.Recortar(p.Titulo(), ancho))}
	switch {
	case p.sinServ:
		return strings.Join(append(l, "", e.Advertencia.Render(e.Recortar(p.a(idioma.T("El servicio no responde · r reintenta")), ancho))), "\n")
	case !p.cargado && p.err == "":
		return strings.Join(append(l, "", e.Tenue.Render(p.a(idioma.T("Cargando…")))), "\n")
	}
	if p.err != "" {
		l = append(l, e.Error.Render(e.Recortar(p.err, ancho)))
	}
	l = append(l, p.barra())
	if !p.cargado {
		return strings.Join(l, "\n")
	}
	var pie []string
	if n := len(p.trabajos); n > 0 {
		pie = append(pie, p.tx("historial.mostrando", "n", itoa(n)))
	}
	if p.cargando || p.cargandoMas {
		pie = append(pie, p.a(idioma.T("Cargando…")))
	}
	hueco := max(alto-len(l)-min(len(pie), 1), 1)
	if len(p.lista.Visibles()) == 0 {
		l = append(l, p.vacio()...)
	} else {
		l = append(l, p.lista.Vista(ancho, hueco))
	}
	if len(pie) > 0 {
		l = append(l, e.Tenue.Render(e.Recortar(strings.Join(pie, " "+e.Iconos.Separador+" "), ancho)))
	}
	return strings.Join(l, "\n")
}
