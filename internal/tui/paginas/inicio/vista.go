package inicio

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// terminales son los estados de un trabajo que ya acabó (web/src/formato.ts).
var terminales = []string{"completado", "fallido", "cancelado", "interrumpido"}

func rellenar(texto string, ancho int) string {
	return texto + strings.Repeat(" ", max(ancho-lipgloss.Width(texto), 0))
}

// envolver parte un texto (con saltos de línea propios) en líneas de como mucho ancho columnas, con sangria espacios por
// delante, y las pinta con el estilo.
func envolver(texto string, ancho, sangria int, estilo lipgloss.Style) []string {
	var lineas []string
	for _, parrafo := range strings.Split(texto, "\n") {
		for _, l := range strings.Split(ansi.Wrap(parrafo, max(ancho-sangria, 1), ""), "\n") {
			lineas = append(lineas, strings.Repeat(" ", sangria)+estilo.Render(strings.TrimRight(l, " ")))
		}
	}
	return lineas
}

// total suma los trabajos de esos estados.
func (p *Pagina) total(estados ...string) int {
	n := 0
	for _, e := range estados {
		n += p.inst.Trabajos[e]
	}
	return n
}

// enProceso son los trabajos que no han terminado, sea cual sea su estado.
func (p *Pagina) enProceso() int {
	n := 0
	for estado, cuantos := range p.inst.Trabajos {
		if !slices.Contains(terminales, estado) {
			n += cuantos
		}
	}
	return n
}

// paso es uno de Primeros pasos: su título y detalle, si está hecho y la página de la terminal a la que lleva.
type paso struct {
	titulo, detalle, pagina string
	hecho                   bool
}

// pasos son los de Panel.vue; el del arranque automático solo sale en los sistemas que lo admiten.
func (p *Pagina) pasos() []paso {
	listos := 0
	for _, a := range p.agentes {
		if a.listo() {
			listos++
		}
	}
	detalle := p.tx("inicio.comprobando")
	if p.hayAgentes {
		detalle = p.tx("inicio.paso1Detalle", "listos", strconv.Itoa(listos), "total", strconv.Itoa(len(p.agentes)))
	}
	lista := []paso{
		{p.tx("inicio.paso1"), detalle, pagina.PaginaAgentes, listos > 0},
		{p.tx("inicio.paso2"), p.tx("inicio.paso2Detalle"), pagina.PaginaEjecutar, p.total("completado") > 0},
		{p.tx("inicio.paso3"), p.tx("inicio.paso3Detalle"), pagina.PaginaConectar, len(p.puntos) > 0},
	}
	if p.autoDisp {
		lista = append(lista, paso{p.tx("inicio.paso4", "sistema", sistema()), p.tx("inicio.paso4Detalle"), pagina.PaginaConfiguracion, p.autoActivo})
	}
	return lista
}

// origenes cuenta las estaciones de la red como MapaRed.vue: la clave del espacio, una por grupo de APIs y las APIs sin
// grupo, como mucho 3 de estas y, si hay más, una más que las junta.
func (p *Pagina) origenes() int {
	grupos, sueltas := map[string]bool{}, 0
	for _, x := range p.puntos {
		if x.Grupo != "" {
			grupos[x.Grupo] = true
		} else {
			sueltas++
		}
	}
	estaciones := len(grupos) + sueltas
	return 1 + min(estaciones, 3) + min(estaciones/4, 1)
}

// periodo es «los últimos N días» o «el último día», lo que dura el historial.
func (p *Pagina) periodo() string {
	if p.dias == 1 {
		return p.tx("comun.ultimoDia")
	}
	return p.tx("comun.ultimosDias", "dias", strconv.FormatFloat(p.dias, 'f', -1, 64))
}

// tituloAyuda es el del tema de la Ayuda que acompaña a Inicio («Empezar»).
func (p *Pagina) tituloAyuda() string {
	for _, t := range datos.Ayuda(p.lengua) {
		if t.Pagina == pagina.PaginaInicio {
			return t.Titulo
		}
	}
	return p.tx("menu.grupo.empezar")
}

// accion es una línea de acción con su tecla: «c  Crear una API».
func (p *Pagina) accion(sangria int, pares ...string) string {
	e := p.estilos
	var partes []string
	for i := 0; i+1 < len(pares); i += 2 {
		partes = append(partes, e.Tecla.Render(pares[i])+" "+e.Acento.Render(pares[i+1]))
	}
	return strings.Repeat(" ", sangria) + strings.Join(partes, "  ")
}

// aviso es un aviso con sus acciones: el texto en advertencia y debajo lo que se puede hacer.
func (p *Pagina) aviso(texto string, acciones ...string) []string {
	e := p.estilos
	lineas := envolver(texto, p.c.Ancho, 2, e.Advertencia)
	lineas[0] = e.Advertencia.Render("!") + " " + strings.TrimLeft(lineas[0], " ")
	return append(lineas, p.accion(2, acciones...))
}

// fila es una fila de la red: la API que mandó la petición, el agente, la etapa y cuánto lleva (o cuánto tardó).
func (p *Pagina) fila(tr tranvia) string {
	e, ahora := p.estilos, p.ahora()
	icono, estilo := e.Iconos.Activo, e.Acento
	switch {
	case tr.Etapa != "llegada":
	case tr.Estado == "completado":
		icono, estilo = e.Iconos.Ok, e.Exito
	case tr.Estado == "cancelado":
		icono, estilo = e.Iconos.Inactivo, e.Tenue
	default:
		icono, estilo = e.Iconos.Fallo, e.Error
	}
	fin := ahora
	if !tr.Terminado.IsZero() {
		fin = tr.Terminado
	}
	segundos := max(int(fin.Sub(tr.Desde)/time.Second), 0)
	api := tr.API
	if api == "" {
		api = p.tx("mapa.principal")
	}
	agente := catalogo.NombreVisible(tr.Agente)
	return estilo.Render(icono) + " " + rellenar(e.Recortar(api, 18), 18) + " " + rellenar(e.Recortar(agente, 14), 14) + " " +
		rellenar(e.Recortar(p.tx(claveEtapa(tr)), 13), 13) + " " + e.Tenue.Render(p.tx("vivo.hace", "s", strconv.Itoa(segundos)))
}

// contador es una fila del tablero: el icono, el nombre, lo que cuenta y la cifra a la derecha.
func (p *Pagina) contador(icono string, estilo lipgloss.Style, clave string, n int) string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	cifra := strconv.Itoa(n)
	nombre := e.Recortar(p.tx("inicio."+clave), 14)
	libre := ancho - 2 - 15 - 1 - lipgloss.Width(cifra) - 1
	detalle := e.Recortar(p.tx("inicio."+clave+"Detalle"), max(libre, 0))
	hueco := max(ancho-2-15-lipgloss.Width(detalle)-lipgloss.Width(cifra), 1)
	return estilo.Render(icono) + " " + rellenar(nombre, 14) + " " + e.Tenue.Render(detalle) + strings.Repeat(" ", hueco) + cifra
}

// lineas es todo lo que dice la página, de arriba abajo; Vista enseña la parte que cabe.
func (p *Pagina) lineas() []string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	l := []string{e.Titulo.Render(e.Recortar(p.Titulo(), ancho))}
	switch {
	case p.sinServ:
		return append(l, "", e.Advertencia.Render(e.Recortar(p.a(idioma.T("El servicio no responde · r reintenta")), ancho)))
	case !p.cargado && p.err == "":
		return append(l, "", e.Tenue.Render(p.a(idioma.T("Cargando…"))))
	}
	if p.err != "" {
		l = append(l, envolver(p.err, ancho, 0, e.Error)...)
	}
	l = append(l, e.Recortar(e.Tenue.Render(p.tx("inicio.direccion"))+" "+e.Acento.Render(p.inst.Base), ancho))
	if p.hayAgentes {
		listos := 0
		for _, a := range p.agentes {
			if a.listo() {
				listos++
			}
		}
		l = append(l, e.Tenue.Render(e.Recortar(p.tx("mapa.descripcion", "apis", strconv.Itoa(p.origenes()), "listos", strconv.Itoa(listos),
			"total", strconv.Itoa(len(p.agentes))), ancho)))
	} else {
		l = append(l, e.Tenue.Render(p.tx("inicio.comprobando")))
	}

	// Avisos
	if sugerirApiPropia(p.inst.AppConClave24h) {
		l = append(l, "")
		l = append(l, p.aviso(p.tx("claves.sugerencia", "n", strconv.Itoa(p.inst.AppConClave24h)), "c", p.tx("claves.crearApi"))...)
	}
	if n := len(p.problemas); n > 0 {
		l = append(l, "")
		texto := p.tx("inicio.espaciosConProblema", "n", strconv.Itoa(n))
		if n == 1 {
			texto = p.tx("inicio.espaciosConProblemaUno", "nombre", p.problemas[0].Nombre)
		}
		l = append(l, p.aviso(texto, "v", p.tx("inicio.verEspacios"))...)
	}
	if p.avisoConversor() {
		l = append(l, "")
		l = append(l, p.aviso(p.textoConversor(), "p", p.tx("conversor.preparar"), "s", p.tx("conversor.irAjustes"))...)
	}

	// La red en vivo
	l = append(l, "")
	filas, resto := visibles(p.red, maximoVisibles)
	cabecera := e.Grupo.Render(p.tx("inicio.red"))
	if n := len(filas) + resto; n == 1 {
		cabecera += e.Tenue.Render(" " + e.Iconos.Separador + " " + p.tx("vivo.enCursoUno"))
	} else if n > 1 {
		cabecera += e.Tenue.Render(" " + e.Iconos.Separador + " " + p.tx("vivo.enCurso", "n", strconv.Itoa(n)))
	}
	l = append(l, cabecera)
	if len(filas) == 0 {
		l = append(l, envolver(p.tx("inicio.redTexto"), ancho, 0, e.Tenue)...)
	}
	for _, tr := range filas {
		l = append(l, e.Recortar(p.fila(tr), ancho))
	}
	if resto > 0 {
		l = append(l, e.Tenue.Render("+"+strconv.Itoa(resto)))
	}
	if p.fallos > 0 && !p.enVivo {
		l = append(l, e.Advertencia.Render(e.Recortar(p.a(idioma.T("Sin conexión en vivo · reconectando…")), ancho)))
	}
	l = append(l, p.accion(0, "g", p.tx("inicio.detalles")))

	// Contadores
	l = append(l, "", e.Grupo.Render(p.tx("inicio.tablero"))+e.Tenue.Render(" "+e.Iconos.Separador+" "+p.periodo()),
		p.contador(e.Iconos.Ok, e.Exito, "completados", p.total("completado")),
		p.contador(e.Iconos.Activo, e.Acento, "enCurso", p.enProceso()),
		p.contador(e.Iconos.Fallo, e.Error, "conError", p.total("fallido", "interrumpido")))

	// Primeros pasos
	pasos := p.pasos()
	hechos := 0
	for _, x := range pasos {
		if x.hecho {
			hechos++
		}
	}
	l = append(l, "")
	if hechos == len(pasos) {
		l = append(l, e.Grupo.Render(p.tx("inicio.primerosPasos")), e.Exito.Render(e.Iconos.Ok+" "+p.tx("inicio.todoListo")))
		l = append(l, envolver(p.tx("inicio.todoListoTexto"), ancho, 2, e.Tenue)...)
	} else {
		l = append(l, e.Grupo.Render(p.tx("inicio.primerosPasos"))+e.Tenue.Render(" "+e.Iconos.Separador+" "+
			p.tx("inicio.guiaProgreso", "hechos", strconv.Itoa(hechos), "total", strconv.Itoa(len(pasos)))))
		for _, x := range pasos {
			icono, estilo := e.Iconos.Inactivo, e.Texto
			if x.hecho {
				icono, estilo = e.Iconos.Ok, e.Exito
			}
			l = append(l, e.Recortar(estilo.Render(icono+" "+x.titulo)+e.Tenue.Render(" "+e.Iconos.Separador+" "+p.tx("menu."+x.pagina)), ancho),
				"  "+e.Tenue.Render(e.Recortar(x.detalle, ancho-2)))
		}
	}
	return append(l, p.accion(0, "a", p.a(idioma.Tf("Ver la Ayuda de %s", p.tituloAyuda()))))
}

// ventana es cuántas líneas del contenido caben: si hay que desplazar, la última fila se queda para el indicador.
func (p *Pagina) ventana(total int) int {
	if alto := max(p.c.Alto, 1); total > alto {
		return max(alto-1, 1)
	}
	return max(p.c.Alto, 1)
}

// Vista es la parte del contenido que cabe, desde la línea desp; si hay más, una última fila dice por dónde va.
func (p *Pagina) Vista() string {
	e := p.estilos
	if e == nil {
		return ""
	}
	l := p.lineas()
	ventana := p.ventana(len(l))
	desp := min(p.desp, max(len(l)-ventana, 0))
	fin := min(desp+ventana, len(l))
	visible := slices.Clone(l[desp:fin])
	if len(l) > max(p.c.Alto, 1) {
		visible = append(visible, e.Tenue.Render(p.flechas()+" "+strconv.Itoa(fin)+"/"+strconv.Itoa(len(l))))
	}
	return strings.Join(visible, "\n")
}
