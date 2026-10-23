package configuracion

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

// Vista es lo abierto (el selector, un formulario, los valores técnicos, Acerca de, las novedades o el avance) o las
// pestañas con sus secciones.
func (p *Pagina) Vista() string {
	e, ancho, alto := p.estilos, p.c.Ancho, p.c.Alto
	if e == nil {
		return ""
	}
	if v, ok := e.Preambulo(p.lengua, ancho, p.sinServ, !p.cargado && p.err == ""); ok {
		return v
	}
	if !p.cargado { // la carga falló: el mensaje del servicio tal cual y cómo reintentar
		return "\n" + e.Error.Width(ancho).Render(p.tx("comun.errorCarga")+": "+p.err) + "\n" + e.Tenue.Render(e.Recortar("r "+p.tx("comun.reintentar"), ancho))
	}
	switch {
	case p.sel != nil:
		return p.sel.Vista(max(ancho, 1), max(alto, 1))
	case p.form != nil:
		return p.vistaForm()
	case p.abierta != "":
		return p.vistaAbierta()
	}
	return p.vistaSecciones()
}

// cortar junta las líneas y deja como mucho alto.
func cortar(lineas []string, alto int) string {
	l := strings.Split(strings.Join(lineas, "\n"), "\n")
	return strings.Join(l[:min(len(l), max(alto, 1))], "\n")
}

// envolver añade a lineas el texto partido por palabras, con el estilo y en como mucho n líneas (0, todas).
func (p *Pagina) envolver(lineas []string, render func(...string) string, texto string, margen, n int) []string {
	for _, l := range p.estilos.Envolver(texto, p.c.Ancho-margen, n) {
		lineas = append(lineas, strings.Repeat(" ", margen)+render(l))
	}
	return lineas
}

// pestanas son las de la Web UI: este espacio, con su nombre, y todo Tramtai. La elegida va entre corchetes.
func (p *Pagina) pestanas() string {
	e := p.estilos
	nombres := []string{p.tx("config.pestanaEspacio", "nombre", p.nombreEspacio()), p.tx("config.pestanaTramtai")}
	partes := make([]string, len(nombres))
	for i, n := range nombres {
		if i == p.pestana {
			partes[i] = e.Titulo.Render("[" + n + "]")
		} else {
			partes[i] = e.Tenue.Render(n)
		}
	}
	return e.Recortar(strings.Join(partes, " "), p.c.Ancho)
}

// bloque es una sección: su título (con la procedencia o el interruptor), lo que vale ahora y, si está elegida, su
// explicación.
type bloque struct {
	titulo, marca string
	lineas        []string
	texto         string
}

func casilla(si bool) string {
	if si {
		return "[x]"
	}
	return "[ ]"
}

// bloqueDe arma la sección, con lo que dice su tarjeta en la Web UI.
func (p *Pagina) bloqueDe(sec string) bloque {
	e := p.estilos
	etiqueta := func(clave string) string {
		switch p.procedenciaDe(map[string][]string{secModelo: clavesPredeterminado, secHistorial: clavesRetencion}[clave]) {
		case "propio":
			return e.Acento.Render(p.tx("herencia.propio"))
		case "global":
			return e.Tenue.Render(p.tx("herencia.global"))
		}
		return ""
	}
	switch sec {
	case secModelo:
		b := bloque{titulo: p.tx("predeterminado.titulo"), marca: etiqueta(secModelo), texto: p.tx("predeterminado.texto")}
		if p.ambito == "proyecto" {
			b.texto += " " + p.tx("predeterminado.proyecto")
		}
		m, ok := p.modeloActual()
		if !ok {
			return b
		}
		esfuerzo := p.nivel("")
		switch {
		case len(m.Esfuerzos) == 0:
			b.lineas = []string{m.ID + e.Tenue.Render(" "+e.Iconos.Separador+" "+p.tx("esfuerzo.noAdmite"))}
			return b
		case m.EsfuerzoEspacio != "":
			esfuerzo = p.nivel(m.EsfuerzoEspacio)
		case m.EsfuerzoPredeterminado != "":
			esfuerzo = p.tx("esfuerzo.agenteCon", "nivel", strings.ToLower(p.nivel(m.EsfuerzoPredeterminado)))
		}
		b.lineas = []string{m.ID + e.Tenue.Render(" "+e.Iconos.Separador+" "+p.tx("esfuerzo.titulo")+": ") + esfuerzo}
		return b
	case secHistorial:
		r := p.retencion
		uso := p.tx("historialAjustes.uso", "peticiones", pagina.Numero(p.lengua, r.Uso.Trabajos), "mb", p.numero(float64(r.Uso.Bytes)/1048576, 1))
		if r.Uso.Trabajos == 1 {
			uso = p.tx("historialAjustes.usoUno", "mb", p.numero(float64(r.Uso.Bytes)/1048576, 1))
		}
		return bloque{titulo: p.tx("historialAjustes.titulo"), marca: etiqueta(secHistorial), texto: p.tx("historialAjustes.texto"),
			lineas: []string{idioma.Tf("Respuestas %d h · registro %s días · máximo %s", r.HorasResultado, p.numero(float64(r.HorasMetadatos)/24, 2),
				pagina.Numero(p.lengua, r.MaxTrabajos)), e.Tenue.Render(uso)}}
	case secDatos:
		return bloque{titulo: p.tx("datos.titulo"), texto: p.tx("datos.texto"), lineas: p.detallesDatos()}
	case secValores:
		return bloque{titulo: p.tx("config.valoresTecnicos"), texto: p.tx("config.valoresTexto")}
	case secPuerto:
		b := bloque{titulo: p.tx("puerto.titulo"), texto: p.tx("puerto.texto"), lineas: []string{p.tx("puerto.actual", "url", p.c.Cli.Base)}}
		if a := p.ajustes; a.Puerto != a.PuertoConfigurado {
			b.lineas = append(b.lineas, e.Advertencia.Render(p.tx("puerto.distinto", "configurado", strconv.Itoa(a.PuertoConfigurado), "actual", strconv.Itoa(a.Puerto))))
		}
		return b
	case secConversor:
		c := p.conversor
		b := bloque{titulo: p.tx("conversor.titulo"), texto: p.tx("conversor.texto"), lineas: []string{p.textoConversor(*c)}}
		if c.Estado == "error" {
			b.lineas = append(b.lineas, e.Tenue.Render(p.tx("conversor.redEmpresa")))
		}
		b.lineas = append(b.lineas, casilla(c.PrepararAlIniciar)+" "+p.tx("conversor.alIniciar"))
		return b
	case secActualizaciones:
		b := bloque{titulo: p.tx("actualizacion.ajustesTitulo"), marca: casilla(p.act.Buscar), texto: p.tx("actualizacion.ajustesTexto"),
			lineas: []string{p.textoActualizacion()}}
		if !p.act.Comprobado.IsZero() {
			b.lineas = append(b.lineas, e.Tenue.Render(p.tx("actualizacion.comprobado", "hora", p.act.Comprobado.Local().Format("15:04"))))
		}
		return b
	case secArranque:
		return bloque{titulo: p.tx("arranque.titulo", "sistema", sistema()), marca: casilla(*p.ajustes.Autoarranque), texto: p.tx("arranque.texto", "sistema", sistema())}
	}
	b := bloque{titulo: p.tx("acerca.titulo")}
	if p.acerca != nil {
		b.lineas = []string{p.acerca.Version + e.Tenue.Render(" "+e.Iconos.Separador+" "+p.tx("acerca.licencia")+" "+licencia+" "+e.Iconos.Separador+" "+autor)}
	}
	return b
}

// detallesDatos son cuántas peticiones terminadas y cuántas APIs hay para borrar, como las casillas de la Web UI.
func (p *Pagina) detallesDatos() []string {
	historial := p.tx("datos.historialDetalle", "n", pagina.Numero(p.lengua, p.terminados))
	if p.terminados == 1 {
		historial = p.tx("datos.historialDetalleUno")
	}
	puntos := p.tx("datos.puntosDetalle", "n", strconv.Itoa(len(p.puntos)))
	if len(p.puntos) == 1 {
		puntos = p.tx("datos.puntosDetalleUno")
	}
	return []string{p.tx("datos.historial") + ": " + p.estilos.Tenue.Render(historial), p.tx("datos.puntos") + ": " + p.estilos.Tenue.Render(puntos)}
}

// textoConversor describe el estado en una frase, como textoConversor de la Web UI.
func (p *Pagina) textoConversor(c contrato.EstadoConversor) string {
	etapa := func(def string) string { return p.tx("conversor.etapa." + cmpOr(c.Etapa, def)) }
	switch c.Estado {
	case "listo":
		origen := p.tx("conversor.origen." + cmpOr(c.Origen, "sistema"))
		listo := p.tx("conversor.listoSinVersion", "origen", origen)
		if c.Version != "" {
			listo = p.tx("conversor.listo", "version", c.Version, "origen", origen)
		}
		switch {
		case c.Etapa != "":
			return listo + " " + p.tx("conversor.preparandoNuevo", "etapa", etapa(""), "progreso", strconv.Itoa(c.Progreso))
		case c.Mensaje != "":
			return listo + " " + p.tx("conversor.ultimoError", "mensaje", c.Mensaje)
		}
		return listo
	case "preparando":
		return p.tx("conversor.preparando", "etapa", etapa("python"), "progreso", strconv.Itoa(c.Progreso))
	case "error":
		return p.tx("conversor.error", "mensaje", c.Mensaje)
	}
	return p.tx("conversor.falta")
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// textoActualizacion es el estado de la búsqueda, como en la tarjeta de la Web UI.
func (p *Pagina) textoActualizacion() string {
	a := p.act
	switch {
	case a.Disponible:
		return p.tx("actualizacion.hayNueva", "version", a.Version)
	case !a.Buscar:
		return p.tx("actualizacion.desactivada", "actual", a.Actual)
	case a.Fallo:
		return p.tx("actualizacion.falloConsulta")
	}
	return p.tx("actualizacion.alDia", "actual", a.Actual)
}

// vistaSecciones son las pestañas, el aviso de a qué afectan y las secciones; la elegida siempre se ve entera.
func (p *Pagina) vistaSecciones() string {
	e, ancho, alto := p.estilos, p.c.Ancho, p.c.Alto
	arriba := []string{p.pestanas()}
	if alto >= 14 {
		aviso := p.tx("config.avisoTramtai")
		switch {
		case p.pestana == pestanaEspacio && p.ambito == "proyecto":
			aviso = p.tx("config.avisoProyecto")
		case p.pestana == pestanaEspacio:
			aviso = p.tx("config.avisoGlobal")
		}
		arriba = p.envolver(arriba, e.Tenue.Render, aviso, 0, 2)
	}
	arriba = append(arriba, "")
	var cuerpo []string
	desde, hasta := 0, 0 // las líneas de la sección elegida
	elegida := p.seccion()
	for _, sec := range p.secciones() {
		b := p.bloqueDe(sec)
		titulo := "  " + e.Texto.Render(b.titulo)
		if sec == elegida {
			desde = len(cuerpo)
			titulo = e.Acento.Render(e.Iconos.Cursor + " " + b.titulo)
		}
		if b.marca != "" {
			titulo += " " + b.marca
		}
		cuerpo = append(cuerpo, e.Recortar(titulo, ancho))
		for _, l := range b.lineas {
			cuerpo = p.envolver(cuerpo, e.Texto.Render, l, 2, 2)
		}
		if sec == elegida && b.texto != "" {
			cuerpo = p.envolver(cuerpo, e.Tenue.Render, b.texto, 2, 3)
		}
		if sec == elegida {
			hasta = len(cuerpo)
		}
	}
	libre := max(alto-len(arriba), 1)
	inicio := 0
	if hasta > libre { // con poco sitio, se baja hasta que la elegida se vea entera
		inicio = min(desde, hasta-libre)
	}
	return cortar(append(arriba, cuerpo[inicio:]...), alto)
}

// vistaForm es el título de la sección, el formulario y lo que hay que saber para rellenarlo.
func (p *Pagina) vistaForm() string {
	e, b := p.estilos, p.bloqueDe(p.form.id)
	lineas := []string{e.Titulo.Render(e.Recortar(b.titulo, p.c.Ancho)), "", p.form.f.Vista()}
	var ayudas []string // las de cada campo, como sus hint en la Web UI
	switch p.form.id {
	case secDatos:
		ayudas = p.detallesDatos()
	case secHistorial:
		for _, c := range []string{"respuestas", "registro", "maximo"} {
			ayudas = append(ayudas, p.tx("historialAjustes."+c)+": "+p.tx("historialAjustes."+c+"Hint"))
		}
	}
	if len(ayudas) > 0 {
		lineas = append(lineas, "")
		for _, l := range ayudas {
			lineas = p.envolver(lineas, e.Tenue.Render, l, 0, 2)
		}
	}
	lineas = append(lineas, "")
	lineas = p.envolver(lineas, e.Tenue.Render, b.texto, 0, 0)
	return cortar(lineas, p.c.Alto)
}

// vistaAbierta pinta en el visor los valores técnicos (con el filtro arriba), Acerca de, las novedades o el avance.
func (p *Pagina) vistaAbierta() string {
	e, ancho := p.estilos, p.c.Ancho
	var arriba []string
	var texto string
	switch p.abierta {
	case vistaValores:
		arriba = append(arriba, e.Titulo.Render(e.Recortar(p.tx("config.valoresTecnicos"), ancho)))
		if p.buscando || p.filtro != "" {
			cursor := ""
			if p.buscando {
				cursor = e.Iconos.Vertical
			}
			arriba = append(arriba, e.Acento.Render(e.Recortar(p.tx("config.buscar")+": "+p.filtro+cursor, ancho)))
		}
		texto = p.textoValores()
	case vistaAcerca:
		texto = p.textoAcerca()
	case vistaNovedades:
		texto = p.textoNovedades()
	case vistaProgreso:
		if p.upd.progreso != nil {
			texto = p.textoAvance()
		}
	}
	p.visor.Tamano(max(ancho, 1), max(p.c.Alto-len(arriba), 1))
	p.visor.Poner(strings.Join(e.Envolver(texto, ancho, 0), "\n")) // por palabras: el visor partiría a media palabra
	return cortar(append(arriba, p.visor.Vista()), p.c.Alto)
}

// secciones de los valores técnicos, por la primera parte de la clave, como en Configuracion.vue.
var seccionDe = map[string]string{"servicio": "servicio", "predeterminados": "predeterminados", "modelos": "modelos", "adaptadores": "agentes",
	"comandos_agentes": "agentes", "concurrencia": "concurrencia", "concurrencia_servicio": "concurrencia", "retencion": "retencion",
	"conversion": "archivos", "compatibilidad": "compatibilidad"}

// origenes son los de cada valor, con su explicación; el global, con la ruta de su archivo si se conoce.
func (p *Pagina) origenes() [][2]string {
	global := ""
	if p.acerca != nil && p.acerca.Rutas != nil {
		global = p.acerca.Rutas.Configuracion
	}
	return [][2]string{{"interno", p.tx("origen.internoDetalle")}, {"global", global}, {"proyecto", p.tx("origen.proyectoDetalle")},
		{"local", p.tx("origen.localDetalle")}, {"entorno", p.tx("origen.entornoDetalle")}}
}

// valor escribe un valor como la Web UI: sí o no, el JSON de una lista o un objeto, y el resto tal cual.
func (p *Pagina) valor(v any) string {
	switch x := v.(type) {
	case bool:
		return siNo(p, x)
	case map[string]any, []any:
		j, _ := json.Marshal(x)
		return string(j)
	}
	return fmt.Sprint(v)
}

// textoValores es la leyenda de los orígenes y la configuración efectiva por secciones, con el filtro.
func (p *Pagina) textoValores() string {
	e := p.estilos
	var l []string
	for _, o := range p.origenes() {
		l = append(l, e.Acento.Render(p.tx("origen."+o[0]))+" "+e.Tenue.Render(o[1]))
	}
	filtro := strings.ToLower(strings.TrimSpace(p.filtro))
	var orden []string
	grupos := map[string][]string{}
	for _, x := range p.entradas {
		valor := p.valor(x.Valor)
		if filtro != "" && !strings.Contains(strings.ToLower(x.Clave), filtro) && !strings.Contains(strings.ToLower(valor), filtro) {
			continue
		}
		primera, _, _ := strings.Cut(x.Clave, ".")
		titulo := p.tx("seccion." + cmpOr(seccionDe[primera], "otros"))
		if _, ok := grupos[titulo]; !ok {
			orden = append(orden, titulo)
		}
		grupos[titulo] = append(grupos[titulo], x.Clave+" = "+valor+" "+e.Tenue.Render(e.Iconos.Separador+" "+p.tx("origen."+x.Origen)))
	}
	if len(orden) == 0 {
		return strings.Join(append(l, "", e.Tenue.Render(p.tx("config.sinCoincidencias"))), "\n")
	}
	for _, g := range orden {
		l = append(l, "", e.Grupo.Render(g))
		l = append(l, grupos[g]...)
	}
	return strings.Join(l, "\n")
}

// Lo que Acerca de enseña igual que la Web UI.
const (
	repositorio = "https://github.com/MscottB/Tramtai"
	autor       = "Michael Scott B."
	licencia    = "PolyForm Noncommercial 1.0.0"
)

// estadoAcerca es el de las actualizaciones, como en Acerca de de la Web UI.
func (p *Pagina) estadoAcerca() string {
	a := p.act
	switch {
	case a == nil:
		return p.tx("acerca.sinComprobar")
	case a.Disponible:
		return p.tx("acerca.hayNueva", "version", a.Version)
	case a.Fallo:
		return p.tx("actualizacion.falloConsulta")
	case !a.Buscar:
		return p.tx("acerca.sinBuscar")
	}
	return p.tx("acerca.alDia")
}

// detallesAcerca son los datos técnicos para pedir ayuda, sin claves: también lo que se copia.
func (p *Pagina) detallesAcerca() [][2]string {
	d := p.acerca
	version := d.Version
	if d.Portable {
		version += " (" + p.tx("acerca.portable") + ")"
	}
	l := [][2]string{{p.tx("acerca.version"), version}}
	if d.Commit != "" {
		commit := d.Commit[:min(12, len(d.Commit))]
		if d.Modificado {
			commit += " (" + p.tx("acerca.modificado") + ")"
		}
		l = append(l, [2]string{p.tx("acerca.commit"), commit})
	}
	sistemas := map[string]string{"windows": "Windows", "darwin": "macOS", "linux": "Linux"}
	l = append(l, [2]string{p.tx("acerca.sistema"), cmpOr(sistemas[d.Sistema], d.Sistema) + " · " + d.Arquitectura}, [2]string{"Go", d.Go},
		[2]string{p.tx("acerca.servicio"), d.Servicio.Direccion + " · pid " + strconv.Itoa(d.Servicio.PID) + " · " +
			p.tx("acerca.desde", "fecha", d.Servicio.IniciadoEn.Local().Format("2006-01-02 15:04"))},
		[2]string{p.tx("acerca.claves"), d.Almacen})
	if r := d.Rutas; r != nil {
		l = append(l, [2]string{p.tx("acerca.datos"), r.Datos}, [2]string{p.tx("acerca.configuracion"), r.Configuracion}, [2]string{p.tx("acerca.registro"), r.Registro})
	}
	return l
}

// textoAcerca es Acerca de: la versión, quién la desarrolla, el estado de las actualizaciones, los enlaces y los detalles.
func (p *Pagina) textoAcerca() string {
	e := p.estilos
	l := []string{e.Titulo.Render(p.tx("acerca.titulo"))}
	if p.acerca != nil {
		version := p.acerca.Version
		if p.acerca.Portable {
			version += " " + e.Iconos.Separador + " " + p.tx("acerca.portable")
		}
		l = append(l, version)
	}
	l = append(l, p.tx("acerca.desarrollado")+" "+autor+" "+e.Tenue.Render("https://github.com/MscottB"), p.estadoAcerca(), "")
	for _, x := range [][2]string{{p.tx("acerca.sitio"), "https://mscottb.github.io/Tramtai/"}, {p.tx("acerca.repositorio"), repositorio},
		{p.tx("acerca.problema"), repositorio + "/issues/new"}, {p.tx("acerca.licencia") + " · " + licencia, repositorio + "/blob/main/LICENSE.md"},
		{p.tx("acerca.terminos"), repositorio + "/blob/main/TERMINOS.md"}} {
		l = append(l, x[0]+": "+e.Acento.Render(x[1]))
	}
	l = append(l, "", e.Grupo.Render(p.tx("acerca.detalles")))
	if p.acerca == nil {
		return strings.Join(append(l, e.Tenue.Render(p.tx("acerca.sinDatos"))), "\n")
	}
	for _, d := range p.detallesAcerca() {
		l = append(l, d[0]+": "+e.Tenue.Render(d[1]))
	}
	return strings.Join(append(l, "", e.Tenue.Render(p.tx("acerca.copiarTexto"))), "\n")
}

// carpetaUsuario es la del usuario en una ruta (también con el nombre corto de Windows, como ADMINI~1): lo copiado se
// pega en una incidencia pública y va como ~, como en la Web UI.
var carpetaUsuario = regexp.MustCompile(`(?:[A-Za-z]:\\Users\\|/home/|/Users/)[^\\/\n]+`)

// copiarSoporte copia los detalles técnicos como texto, sin claves.
func (p *Pagina) copiarSoporte() tea.Cmd {
	if p.acerca == nil {
		return nil
	}
	l := []string{"Tramtai"}
	for _, d := range p.detallesAcerca() {
		l = append(l, d[0]+": "+d[1])
	}
	return comp.Copiar(carpetaUsuario.ReplaceAllString(strings.Join(l, "\n"), "~"))
}
