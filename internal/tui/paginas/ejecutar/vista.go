package ejecutar

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

// Vista pinta lo abierto encima o la pestaña de ahora, en Ancho×Alto.
func (p *Pagina) Vista() string {
	if p.estilos == nil {
		return ""
	}
	e := p.estilos
	var texto string
	switch {
	case p.guardar != nil:
		pista := map[string]string{"nombre": "puntos.nombreHint", "grupo": "puntos.grupoHint"}[p.guardar.Foco()]
		texto = strings.Join(append([]string{e.Titulo.Render(p.tx("ejecutar.guardarApi")), e.Tenue.Render(p.tx("ejecutar.guardarTexto")), "",
			p.guardar.Vista(), ""}, p.tenue(p.envolver(p.tx(pista), 2))...), "\n")
	case p.codigo != nil:
		texto = p.vistaCodigo()
	case p.creada != nil:
		texto = strings.Join([]string{e.Titulo.Render(p.tx("ejecutar.apiCreada", "nombre", p.creada.nombre)), "", p.caja.Vista()}, "\n")
	case p.muestra != "":
		p.caja.Tamano(max(p.c.Ancho, 1), max(p.c.Alto-2, 1))
		texto = strings.Join([]string{e.Titulo.Render(p.tx("receta.ejemplo")), "", p.caja.Vista()}, "\n")
	case !p.armado:
		texto = e.Tenue.Render(p.tx("comun.cargando"))
	default:
		cab := []string{p.pestanas()}
		if p.api != "" {
			cab = append(cab, e.Acento.Render(p.tx("ejecutar.editando", "nombre", p.api)))
		}
		cuerpo := max(p.c.Alto-len(cab), 1)
		switch p.pestana {
		case pestRespuesta:
			texto = p.vistaRespuesta(cuerpo)
		case pestRecientes:
			texto = p.vistaRecientes(cuerpo)
		default:
			texto = p.vistaReceta(cuerpo)
		}
		texto = strings.Join(append(cab, texto), "\n")
	}
	return p.encajar(texto)
}

// encajar corta lo que no cabe en Ancho×Alto: las líneas de más y lo que pasa del ancho, con la elipsis.
func (p *Pagina) encajar(texto string) string {
	lineas := strings.Split(texto, "\n")
	lineas = lineas[:min(len(lineas), max(p.c.Alto, 1))]
	for i, l := range lineas {
		lineas[i] = p.estilos.Recortar(l, max(p.c.Ancho, 1))
	}
	return strings.Join(lineas, "\n")
}

// pestanas son los nombres de las pestañas, con la de ahora entre corchetes; la respuesta lleva el estado del trabajo.
func (p *Pagina) pestanas() string {
	e := p.estilos
	respuesta := p.tx("ejecutar.respuesta")
	if p.trabajo != nil {
		icono, _ := vivo.Icono(e, p.estado)
		respuesta = icono + " " + respuesta
	}
	nombres := []string{p.tx("ejecutar.receta"), respuesta, p.tx("ejecutar.recientes")}
	partes := make([]string, len(nombres))
	for i, n := range nombres {
		if i == p.pestana {
			partes[i] = e.Titulo.Render("[" + n + "]")
		} else {
			partes[i] = e.Tenue.Render(" " + n + " ")
		}
	}
	return strings.Join(partes, " ")
}

// vistaReceta es el formulario desplazado para que el campo con el foco se vea, debajo dónde está el foco (como el «↑↓ 16/28»
// de Inicio) y lo que explica ese campo.
func (p *Pagina) vistaReceta(alto int) string {
	pista := p.envolver(p.pista(), 3)
	visible := max(alto-len(pista)-1, 3)
	lineas := strings.Split(p.form.Vista(), "\n")
	foco := p.form.LineaFoco()
	campo := 2 // la etiqueta, el valor y, si hay, el error
	if p.form.Foco() == "instrucciones" || p.form.Foco() == "esquema" || p.form.Foco() == "mensaje" {
		campo = 8
	}
	campo = min(campo, visible)
	switch {
	case foco < p.desplaz:
		p.desplaz = foco
	case foco+campo > p.desplaz+visible:
		p.desplaz = foco + campo - visible
	}
	p.desplaz = max(min(p.desplaz, len(lineas)-visible), 0)
	vista := lineas[p.desplaz:min(p.desplaz+visible, len(lineas))]
	for len(vista) < visible {
		vista = append(vista, "")
	}
	flechas, n, total := "↑↓", 0, 0
	if p.estilos.ASCII {
		flechas = "^v"
	}
	n, total = p.form.Posicion()
	posicion := p.estilos.Tenue.Render(flechas + " " + itoa(n) + "/" + itoa(total))
	return strings.Join(append(append(vista, posicion), p.tenue(pista)...), "\n")
}

// tenue pinta unas líneas con el estilo tenue.
func (p *Pagina) tenue(lineas []string) []string {
	for i, l := range lineas {
		lineas[i] = p.estilos.Tenue.Render(l)
	}
	return lineas
}

// pista explica el campo con el foco, con los textos de la Web UI; en los archivos, los límites y el aviso del conversor.
func (p *Pagina) pista() string {
	if p.enCampos() {
		return p.pistaCampos(p.form.Foco())
	}
	switch p.form.Foco() {
	case "opciones":
		return idioma.T("Enter abre o pliega las opciones de la receta")
	case "avanzadas":
		return p.tx("ejecutar.avanzadasTexto")
	case "instrucciones":
		return p.tx("receta.instruccionesHint")
	case "modelo":
		return "← → " + p.tx("receta.modelo")
	case "esfuerzo":
		return p.tx("esfuerzo.hint")
	case "respaldo":
		var ids []string
		for _, m := range p.catalogo {
			if m.Perfil == "" {
				ids = append(ids, m.ID)
			}
		}
		return p.tx("receta.respaldoHint") + " " + idioma.Tf("Escribe agente o agente/modelo (+esfuerzo), separados por comas: %s", strings.Join(ids, ", "))
	case "estructurada":
		return p.tx("receta.salidaEstructuradaTexto")
	case "plantilla":
		return idioma.T("← → elige la plantilla y Enter la aplica")
	case "esquema":
		if _, ok := camposDe(p.form.Valor("esquema")); !ok && esquemaValido(p.form.Valor("esquema")) {
			return p.tx("receta.esquemaAvanzado") + " " + p.tx("receta.esquemaHint")
		}
		return p.tx("receta.esquemaHint")
	case "modo":
		if _, ok := camposDe(p.form.Valor("esquema")); !ok && esquemaValido(p.form.Valor("esquema")) {
			return p.tx("receta.esquemaAvanzado")
		}
		return idioma.T("← → elige cómo se edita el esquema")
	case "reintentos":
		return p.tx("receta.reintentosHint")
	case "mensaje":
		return idioma.T("Enter salta de línea; Ctrl+Enter o Ctrl+S envía")
	case "archivos":
		partes := []string{idioma.T("Separa las rutas con comas; Tab completa la ruta.")}
		if l := p.limites; l != nil {
			partes = append(partes, p.tx("ejecutar.adjuntarLimites", "n", itoa(l.CargasPorPeticion), "mb", itoa(l.CargaMB), "texto", itoa(l.TextoConvertidoMB)))
			if f := formatos(l.ExtensionesConversor, l.ExtensionesTexto); f != "" {
				partes = append(partes, p.tx("ejecutar.formatos", "lista", f))
			}
		}
		if p.conversor != "" {
			partes = append([]string{p.conversor}, partes...)
		}
		return strings.Join(partes, " ")
	case "perfil":
		return p.tx("ejecutar.perfilHint")
	case "tiempo":
		return p.tx("ejecutar.tiempoHint")
	}
	return ""
}

// formatos es la lista para leer: «DOCX, PDF, MD, TXT».
func formatos(listas ...[]string) string {
	var r []string
	for _, l := range listas {
		for _, x := range l {
			r = append(r, strings.ToUpper(strings.TrimPrefix(x, ".")))
		}
	}
	return strings.Join(r, ", ")
}

// envolver parte un texto por palabras al ancho de la página, en como mucho n líneas.
func (p *Pagina) envolver(texto string, n int) []string {
	if texto == "" {
		return nil
	}
	ancho := max(p.c.Ancho, 10)
	var lineas []string
	linea := ""
	for _, palabra := range strings.Fields(p.a(texto)) {
		switch {
		case linea == "":
			linea = palabra
		case lipgloss.Width(linea)+1+lipgloss.Width(palabra) <= ancho:
			linea += " " + palabra
		default:
			lineas, linea = append(lineas, linea), palabra
		}
	}
	lineas = append(lineas, linea)
	if len(lineas) > n { // lo que no cabe se corta con la elipsis en la última línea
		lineas[n-1] = p.estilos.Recortar(strings.Join(lineas[n-1:], " "), ancho)
		lineas = lineas[:n]
	}
	return lineas
}

// vistaRespuesta es el estado del trabajo, su ruta y el texto de la respuesta.
func (p *Pagina) vistaRespuesta(alto int) string {
	e := p.estilos
	var cab []string
	if p.trabajo != nil {
		icono, estilo := vivo.Icono(e, p.estado)
		linea := estilo.Render(icono + " " + p.nombreEstado(p.estado))
		switch {
		case p.activo() && p.corte:
			linea += " " + e.Advertencia.Render(p.a(idioma.T("reconectando…")))
		case p.activo() && p.cancelar != nil:
			linea += " " + e.Tenue.Render(e.Iconos.Separador) + " " + e.Acento.Render(e.Iconos.Activo+" "+idioma.T("en vivo"))
		}
		cab = append(cab, linea, p.a(vivo.Ruta(e, p.lengua, p.estado)))
	}
	p.visor.Tamano(max(p.c.Ancho, 1), max(alto-len(cab), 1))
	return strings.Join(append(cab, p.visor.Vista()), "\n")
}

// vistaRecientes es la lista de las pruebas recientes, o lo que dice la Web UI si aún no hay.
func (p *Pagina) vistaRecientes(alto int) string {
	e := p.estilos
	switch {
	case !p.cargadoRec:
		return e.Tenue.Render(p.tx("comun.cargando"))
	case len(p.recientes.Visibles()) == 0:
		return strings.Join(p.envolver(p.tx("ejecutar.sinRecientes"), alto), "\n")
	}
	return p.recientes.Vista(max(p.c.Ancho, 1), alto)
}

// vistaCodigo es «Ver como código»: el título, lo que explica, el aviso si se edita una API, los ejemplos y el código.
func (p *Pagina) vistaCodigo() string {
	e, v := p.estilos, p.codigo
	lineas := []string{e.Titulo.Render(p.tx("codigo.titulo"))}
	lineas = append(lineas, p.envolver(p.tx("codigo.texto"), 2)...)
	if p.api != "" {
		lineas = append(lineas, p.envolver(p.tx("codigo.recetaEnApi", "nombre", p.api), 2)...)
	}
	partes := make([]string, len(v.ejemplos))
	for i, x := range v.ejemplos {
		if i == v.i {
			partes[i] = e.Titulo.Render("[" + p.a(x.Titulo) + "]")
		} else {
			partes[i] = e.Tenue.Render(" " + p.a(x.Titulo) + " ")
		}
	}
	lineas = append(lineas, strings.Join(partes, " "))
	p.caja.Tamano(max(p.c.Ancho, 1), max(p.c.Alto-len(lineas), 1))
	return strings.Join(append(lineas, p.caja.Vista()), "\n")
}

func itoa(n int) string { return strconv.Itoa(n) }

// copiar pone un texto en el portapapeles con OSC 52. La terminal no dice si lo admite: el aviso lo explica.
func copiar(texto string) tea.Cmd {
	if strings.TrimSpace(texto) == "" {
		return comp.Aviso(idioma.T("No hay nada que copiar"), comp.Advertencia)
	}
	return tea.Batch(tea.SetClipboard(texto), comp.Aviso(idioma.T("Copiado con OSC 52; si no llega al portapapeles, tu terminal no lo admite"), comp.Exito))
}

// Teclas son las del pie: las del formulario mientras se edita y, si no, las de la pestaña.
func (p *Pagina) Teclas() []pagina.Tecla {
	switch {
	case p.guardar != nil:
		return p.guardar.Teclas()
	case p.codigo != nil:
		return []pagina.Tecla{{Tecla: "← →", Accion: idioma.T("pestañas")}, {Tecla: "y", Accion: idioma.T("copia")}, {Tecla: "Esc", Accion: idioma.T("cierra")}}
	case p.creada != nil:
		return []pagina.Tecla{{Tecla: "y", Accion: idioma.T("copia la clave")}, {Tecla: "Esc", Accion: idioma.T("cierra")}}
	case p.muestra != "":
		return []pagina.Tecla{{Tecla: "y", Accion: idioma.T("copia")}, {Tecla: "Esc", Accion: idioma.T("cierra")}}
	case p.editando:
		teclas := p.form.Teclas()
		switch p.form.Foco() {
		case "plantilla":
			teclas = append(teclas, pagina.Tecla{Tecla: "Enter", Accion: idioma.T("aplica")})
		case "archivos":
			teclas = append(teclas, pagina.Tecla{Tecla: "Tab", Accion: idioma.T("completa")})
		}
		return teclas
	}
	teclas := []pagina.Tecla{{Tecla: "← →", Accion: idioma.T("pestañas")}}
	switch p.pestana {
	case pestReceta:
		teclas = append(teclas, pagina.Tecla{Tecla: "Enter", Accion: idioma.T("edita")}, pagina.Tecla{Tecla: "↑↓", Accion: idioma.T("campo")})
		if p.enCampos() {
			teclas = append(teclas, pagina.Tecla{Tecla: "n", Accion: p.tx("campos.anadir")})
		}
		if _, ok := rutaCampo(p.form.Foco()); ok && p.enCampos() {
			teclas = append(teclas, pagina.Tecla{Tecla: "x", Accion: idioma.T("quita el campo")}, pagina.Tecla{Tecla: "Alt+↑↓", Accion: idioma.T("mueve el campo")})
		}
		if len(p.camposConNombre()) > 0 {
			teclas = append(teclas, pagina.Tecla{Tecla: "v", Accion: p.tx("receta.ejemplo")})
		}
	case pestRespuesta:
		if p.salida != "" {
			teclas = append(teclas, pagina.Tecla{Tecla: "y", Accion: idioma.T("copia")})
		}
		if p.final != nil && p.final.Corregible {
			teclas = append(teclas, pagina.Tecla{Tecla: "o", Accion: p.tx("ejecutar.corregir")})
		}
		if p.final != nil {
			teclas = append(teclas, pagina.Tecla{Tecla: "n", Accion: p.tx("ejecutar.nueva")})
		}
	case pestRecientes:
		teclas = append(teclas, pagina.Tecla{Tecla: "Enter", Accion: idioma.T("ver")}, pagina.Tecla{Tecla: "u", Accion: p.tx("ejecutar.usarDeNuevo")},
			pagina.Tecla{Tecla: "h", Accion: p.tx("ejecutar.verTodo")})
	}
	teclas = append(teclas, pagina.Tecla{Tecla: "Ctrl+S", Accion: p.tx("ejecutar.enviar")}, pagina.Tecla{Tecla: "k", Accion: p.tx("codigo.boton")})
	if p.api != "" {
		teclas = append(teclas, pagina.Tecla{Tecla: "g", Accion: p.tx("ejecutar.actualizarApi", "nombre", p.api)}, pagina.Tecla{Tecla: "d", Accion: p.tx("ejecutar.dejarEditar")})
	} else {
		teclas = append(teclas, pagina.Tecla{Tecla: "g", Accion: p.tx("ejecutar.guardarApi")})
	}
	if p.activo() {
		teclas = append(teclas, pagina.Tecla{Tecla: "c", Accion: p.tx("historial.cancelar")})
	}
	return append(teclas, pagina.Tecla{Tecla: "Esc", Accion: idioma.T("vuelve")})
}
