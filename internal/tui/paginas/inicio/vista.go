package inicio

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

// envolver parte un texto (con saltos de línea propios) en líneas de como mucho ancho columnas, con sangria espacios por
// delante, y las pinta con el estilo.
func (p *Pagina) envolver(texto string, ancho, sangria int, estilo lipgloss.Style) []string {
	lineas := p.estilos.Envolver(texto, ancho-sangria, 0)
	for i, l := range lineas {
		lineas[i] = strings.Repeat(" ", sangria) + estilo.Render(l)
	}
	return lineas
}

// paso es uno de Primeros pasos: su título y detalle, si está hecho y la página de la terminal a la que lleva.
type paso struct {
	titulo, detalle, pagina string
	hecho                   bool
}

// pasos son los Primeros pasos que manda el servicio, ya calculados, con sus textos.
func (p *Pagina) pasos() []paso {
	lista := make([]paso, 0, len(p.losPasos))
	for _, x := range p.losPasos {
		switch x.Pagina {
		case pagina.PaginaAgentes:
			lista = append(lista, paso{p.tx("inicio.paso1"), p.tx("inicio.paso1Detalle", "listos", strconv.Itoa(x.Listos), "total", strconv.Itoa(x.Total)), x.Pagina, x.Hecho})
		case pagina.PaginaEjecutar:
			lista = append(lista, paso{p.tx("inicio.paso2"), p.tx("inicio.paso2Detalle"), x.Pagina, x.Hecho})
		case pagina.PaginaConectar:
			lista = append(lista, paso{p.tx("inicio.paso3"), p.tx("inicio.paso3Detalle"), x.Pagina, x.Hecho})
		case pagina.PaginaConfiguracion:
			lista = append(lista, paso{p.tx("inicio.paso4", "sistema", sistema()), p.a(idioma.T("Tus apps funcionan al encender el equipo")), x.Pagina, x.Hecho})
		}
	}
	return lista
}

// listos son los agentes que el servicio da por listos.
func (p *Pagina) listos() int {
	n := 0
	for _, a := range p.agentes {
		if a.Estado == contrato.AgenteListo {
			n++
		}
	}
	return n
}

// origenes son los orígenes de las peticiones: la clave del espacio, cada grupo de APIs y cada API sin grupo.
func (p *Pagina) origenes() int {
	grupos, sueltas := map[string]bool{}, 0
	for _, x := range p.puntos {
		if x.Grupo != "" {
			grupos[x.Grupo] = true
		} else {
			sueltas++
		}
	}
	return 1 + len(grupos) + sueltas
}

// periodo es «los últimos N días» o «el último día», lo que dura el historial.
func (p *Pagina) periodo() string { return p.estilos.Periodo(p.lengua, p.horas) }

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
	lineas := p.envolver(texto, p.c.Ancho, 2, e.Advertencia)
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
	return estilo.Render(icono) + " " + pagina.Rellenar(e.Recortar(api, 18), 18) + " " + pagina.Rellenar(e.Recortar(agente, 14), 14) + " " +
		pagina.Rellenar(e.Recortar(p.tx(claveEtapa(tr)), 13), 13) + " " + e.Tenue.Render(p.tx("vivo.hace", "s", strconv.Itoa(segundos)))
}

// contador es una fila del tablero: el icono, la tecla que abre el Historial con ese estado, el nombre, lo que cuenta y la
// cifra a la derecha.
func (p *Pagina) contador(icono string, estilo lipgloss.Style, tecla, clave string, n int) string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	cifra := strconv.Itoa(n)
	nombre := e.Recortar(p.tx("inicio."+clave), 12)
	libre := ancho - 2 - 15 - 1 - lipgloss.Width(cifra) - 1
	detalle := e.Recortar(p.tx("inicio."+clave+"Detalle"), max(libre, 0))
	hueco := max(ancho-2-15-lipgloss.Width(detalle)-lipgloss.Width(cifra), 1)
	return estilo.Render(icono) + " " + e.Tecla.Render(tecla) + " " + pagina.Rellenar(nombre, 12) + " " + e.Tenue.Render(detalle) + strings.Repeat(" ", hueco) + cifra
}

// lineas es todo lo que dice la página, de arriba abajo; Vista enseña la parte que cabe.
func (p *Pagina) lineas() []string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	if v, ok := e.Preambulo(p.lengua, ancho, p.sinServ, !p.cargado && p.err == ""); ok {
		return strings.Split(v, "\n")
	}
	var l []string // el título ya está en la cabecera de la app
	if p.err != "" {
		l = append(l, p.envolver(p.err, ancho, 0, e.Error)...)
	}
	l = append(l, e.Recortar(e.Tenue.Render(p.tx("inicio.direccion"))+" "+e.Acento.Render(p.inst.Base), ancho))
	if p.hayAgentes {
		listos := p.listos()
		clave := "mapa.descripcion"
		if p.origenes() == 1 {
			clave = "mapa.descripcionUno"
		}
		red := p.tx(clave, "apis", strconv.Itoa(p.origenes()), "listos", strconv.Itoa(listos), "total", strconv.Itoa(len(p.agentes)))
		l = append(l, e.Tenue.Render(e.Recortar(red, ancho)))
	} else {
		l = append(l, e.Tenue.Render(p.tx("inicio.comprobando")))
	}

	if p.inst.SugerirApiPropia {
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

	l = append(l, "")
	filas, resto := visibles(p.red, maximoVisibles)
	cabecera := e.Grupo.Render(p.tx("inicio.red"))
	// Las que ya llegaron siguen a la vista unos segundos (vivo.go) pero no están en curso.
	enCurso := 0
	for _, tr := range p.red {
		if tr.Terminado.IsZero() {
			enCurso++
		}
	}
	if enCurso == 1 {
		cabecera += e.Tenue.Render(" " + e.Iconos.Separador + " " + p.tx("vivo.enCursoUno"))
	} else if enCurso > 1 {
		cabecera += e.Tenue.Render(" " + e.Iconos.Separador + " " + p.tx("vivo.enCurso", "n", strconv.Itoa(enCurso)))
	}
	l = append(l, cabecera)
	if len(filas) == 0 {
		l = append(l, p.envolver(p.tx("inicio.redTexto"), ancho, 0, e.Tenue)...)
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

	l = append(l, "", e.Grupo.Render(p.tx("inicio.tablero"))+e.Tenue.Render(" "+e.Iconos.Separador+" "+p.periodo()),
		p.contador(e.Iconos.Ok, e.Exito, teclaCompletados, "completados", p.inst.Contadores[nucleo.FiltroCompletado]),
		p.contador(e.Iconos.Activo, e.Acento, teclaEnCurso, "enCurso", p.inst.Contadores[nucleo.FiltroEnCurso]),
		p.contador(e.Iconos.Fallo, e.Error, teclaConError, "conError", p.inst.Contadores[nucleo.FiltroConError]))

	pasos := p.pasos()
	hechos := 0
	for _, x := range pasos {
		if x.hecho {
			hechos++
		}
	}
	l = append(l, "")
	switch {
	case !p.hayAgentes: // los pasos llegan con la comprobación de los agentes
		l = append(l, e.Grupo.Render(p.tx("inicio.primerosPasos")), e.Tenue.Render(e.Recortar(p.tx("inicio.comprobando"), ancho)))
	case hechos == len(pasos):
		l = append(l, e.Grupo.Render(p.tx("inicio.primerosPasos")), e.Exito.Render(e.Iconos.Ok+" "+p.tx("inicio.todoListo")))
		l = append(l, p.envolver(p.tx("inicio.todoListoTexto"), ancho, 2, e.Tenue)...)
	default:
		l = append(l, e.Grupo.Render(p.tx("inicio.primerosPasos"))+e.Tenue.Render(" "+e.Iconos.Separador+" "+
			p.tx("inicio.guiaProgreso", "hechos", strconv.Itoa(hechos), "total", strconv.Itoa(len(pasos)))))
		for _, x := range pasos {
			icono, estilo := e.Iconos.Inactivo, e.Texto
			if x.hecho {
				icono, estilo = e.Iconos.Ok, e.Exito
			}
			l = append(l, e.Recortar(estilo.Render(icono+" "+x.titulo)+e.Tenue.Render(" "+e.Iconos.Separador+" ")+e.Tecla.Render(Atajo(x.pagina))+e.Tenue.Render(" "+p.tx("menu."+x.pagina)), ancho),
				"  "+e.Tenue.Render(e.Recortar(x.detalle, ancho-2)))
		}
	}
	l = append(l, p.accion(0, "a", p.a(idioma.Tf("Ver la Ayuda de %s", p.tituloAyuda()))))

	l = append(l, "", e.Grupo.Render(p.tx("inicio.recientes")))
	if len(p.recientes) == 0 {
		l = append(l, p.envolver(p.tx("inicio.sinTrabajos"), ancho, 0, e.Tenue)...)
	}
	for _, j := range p.recientes {
		l = append(l, p.reciente(j)...)
	}
	return append(l, p.accion(0, "h", p.tx("inicio.verHistorial")))
}

// Atajo es la tecla (1 a 7) que lleva a una página del menú; "" si no tiene. Debe coincidir con el menú de internal/tui.
func Atajo(clave string) string {
	n := slices.Index([]string{pagina.PaginaInicio, pagina.PaginaAgentes, pagina.PaginaEjecutar, pagina.PaginaConectar, pagina.PaginaHistorial,
		pagina.PaginaEspacios, pagina.PaginaConfiguracion}, clave)
	if n < 0 {
		return ""
	}
	return strconv.Itoa(n + 1)
}

// reciente son las dos líneas de una petición reciente: lo que se escribió, y debajo hace cuánto, el agente con
// su modelo y cómo acabó.
func (p *Pagina) reciente(j nucleo.Trabajo) []string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	texto := p.tx("trabajo.entradaEliminada")
	if n := len(j.Solicitud.Mensajes); n > 0 {
		texto = strings.Join(strings.Fields(j.Solicitud.Mensajes[n-1].Contenido), " ")
	} else if !j.EntradasEliminadas {
		texto = ""
	}
	icono, estilo := vivo.Icono(e, j.Estado)
	estado := vivo.NombreEstado(p.lengua, j.Estado)
	partes := []string{vivo.Hace(p.ahora(), j.CreadoEn), strings.TrimSpace(catalogo.NombreVisible(j.Solicitud.Agente) + " " + cmp.Or(j.ModeloEfectivo, j.ModeloEnviado)), estado}
	return []string{estilo.Render(icono) + " " + e.Recortar(texto, ancho-2),
		"  " + e.Tenue.Render(e.Recortar(p.a(strings.Join(partes, " "+e.Iconos.Separador+" ")), ancho-2))}
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
