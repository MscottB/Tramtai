package historial

import (
	"cmp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

// origenesConocidos y claveOrigen son los orígenes del filtro de la Web UI y sus textos.
var origenesConocidos = []string{"probar", "terminal", "cli", "app"}

var claveOrigen = map[string]string{"probar": "historial.origenProbar", "terminal": "historial.origenTerminal",
	"cli": "historial.origenCli", "app": "historial.origenApp"}

func itoa(n int) string { return strconv.Itoa(n) }

// aDerecha alinea el texto a la derecha en ancho columnas visibles.
func aDerecha(texto string, ancho int) string {
	return strings.Repeat(" ", max(ancho-lipgloss.Width(texto), 0)) + texto
}

// textoDe es lo que se pidió: el último mensaje, o el aviso de que la retención borró la entrada.
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
		return idioma.T("Sin dato")
	}
	return strings.Join(partes, " · ")
}

// nombreEstado es el de la Web UI, sin caracteres de dibujo en una consola ASCII.
func (p *Pagina) nombreEstado(e nucleo.Estado) string { return p.a(vivo.NombreEstado(p.lengua, e)) }

// etiquetaEstado es el del filtro de estado: los nombres del selector de la Web UI.
func (p *Pagina) etiquetaEstado() string {
	switch p.estado {
	case nucleo.FiltroEnCurso:
		return p.tx("inicio.enCurso")
	case nucleo.FiltroCompletado:
		return p.tx("inicio.completados")
	case nucleo.FiltroConError:
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
// principal.
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

// fila es una línea de la lista: el estado, lo que se pidió, el agente y modelo (si cabe) y hace cuánto.
func (p *Pagina) fila(t nucleo.Trabajo) string {
	e := p.estilos
	ancho := max(p.c.Ancho-2, 1) // la lista pone el cursor delante
	icono, _ := vivo.Icono(e, t.Estado)
	estado := pagina.Rellenar(e.Recortar(icono+" "+p.nombreEstado(t.Estado), 14), 14)
	cuando := aDerecha(p.a(vivo.Hace(p.ahora(), t.CreadoEn)), 10)
	var modelo string
	if ancho >= 90 {
		modelo = pagina.Rellenar(e.Recortar(modeloDe(t), 24), 24) + " "
	}
	libre := max(ancho-lipgloss.Width(estado)-1-lipgloss.Width(modelo)-lipgloss.Width(cuando)-1, 4)
	texto := strings.Join(strings.Fields(textoDe(t, p.lengua)), " ")
	return p.a(estado + " " + pagina.Rellenar(e.Recortar(texto, libre), libre) + " " + modelo + cuando)
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

// vacio es lo que se ve sin filas: que no hay peticiones, o ninguna con este filtro, y qué hacer.
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
	var l []string // el título ya está en la cabecera de la app
	switch {
	case p.sinServ:
		return strings.Join(append(l, "", e.Advertencia.Render(e.Recortar(p.a(idioma.T("El servicio no responde · r reintenta")), ancho))), "\n")
	case !p.cargado && p.err == "":
		return strings.Join(append(l, "", e.Tenue.Render(p.tx("comun.cargando"))), "\n")
	}
	if p.err != "" {
		l = append(l, e.Error.Width(ancho).Render(p.err)) // entero, en las líneas que ocupe
	}
	l = append(l, p.barra())
	if !p.cargado {
		return strings.Join(l, "\n")
	}
	var pie []string
	switch n := len(p.trabajos); {
	case n == 1:
		pie = append(pie, p.tx("historial.mostrandoUno"))
	case n > 1:
		pie = append(pie, p.tx("historial.mostrando", "n", itoa(n)))
	}
	if p.cargando || p.cargandoMas {
		pie = append(pie, p.tx("comun.cargando"))
	}
	textoPie := ""
	if len(pie) > 0 { // entero, en las líneas que ocupe
		textoPie = e.Tenue.Width(ancho).Render(strings.Join(pie, " "+e.Iconos.Separador+" "))
	}
	hueco := max(alto-lipgloss.Height(strings.Join(l, "\n"))-min(len(pie), 1)*lipgloss.Height(textoPie), 1) // el error ocupa sus líneas
	if len(p.lista.Visibles()) == 0 {
		l = append(l, p.vacio()...)
	} else {
		l = append(l, p.lista.Vista(ancho, hueco))
	}
	if textoPie != "" {
		l = append(l, textoPie)
	}
	return strings.Join(l, "\n")
}
