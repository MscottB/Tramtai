package ejecutar

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
)

// datosForm es lo que tiene el formulario: la receta (Receta de recetas.ts, con estructurada aparte del texto del esquema,
// como en EditorReceta), el mensaje, los archivos y las opciones avanzadas.
type datosForm struct {
	instrucciones, modelo, esfuerzo   string
	forzar, estructurada              bool
	respaldo                          []string
	esquema                           string
	modo                              string         // modoCampos o modoJSON
	campos                            []campoEsquema // en modo campos, los campos (y esquema, lo que generan)
	reintentos                        int
	mensaje, archivos, perfil, tiempo string
}

// opcionesCampo son los valores de un campo de opción y lo que se ve de cada uno, en el mismo orden.
type opcionesCampo struct{ valores, etiquetas []string }

// opcion crea un campo de opción y recuerda sus valores: el campo solo ve las etiquetas.
func (p *Pagina) opcion(id, etiqueta string, valores, etiquetas []string, valor string) comp.Campo {
	p.opciones[id] = opcionesCampo{valores, etiquetas}
	i := max(slices.Index(valores, valor), 0)
	return comp.Opcion(id, etiqueta, etiquetas, etiquetas[i])
}

// valorOpcion es el valor elegido en un campo de opción; "" si no está.
func (p *Pagina) valorOpcion(id string) string {
	o := p.opciones[id]
	if i := slices.Index(o.etiquetas, p.form.Valor(id)); i >= 0 {
		return o.valores[i]
	}
	return ""
}

// armar rehace el formulario con esos datos y el foco en ese campo. Se rehace cuando cambia lo que se ve: el modelo (los
// esfuerzos que admite y forzar), la salida estructurada (el esquema, la plantilla y los reintentos) o el catálogo.
func (p *Pagina) armar(d datosForm, foco string) {
	p.opciones = map[string]opcionesCampo{}
	modelos, etiquetas := []string{""}, []string{p.tx("modelo.esPredeterminado")}
	for _, m := range p.catalogo {
		if m.Predeterminado {
			etiquetas[0] += " (" + m.ID + ")"
		}
		modelos, etiquetas = append(modelos, m.ID), append(etiquetas, m.ID)
	}
	if !slices.Contains(modelos, d.modelo) { // un modelo escrito a mano (o de una receta) que el catálogo no trae
		modelos, etiquetas = append(modelos, d.modelo), append(etiquetas, d.modelo)
	}
	niveles, vacio := p.esfuerzos(d.modelo)
	if len(p.catalogo) > 0 && !slices.Contains(niveles, d.esfuerzo) {
		d.esfuerzo = "" // un nivel que el modelo nuevo no admite vuelve al del agente
	}
	esfuerzos, nombres := []string{""}, []string{vacio}
	if len(p.catalogo) == 0 && d.esfuerzo != "" { // sin catálogo no hay niveles que comparar: se conserva el elegido
		niveles = append(niveles, d.esfuerzo)
	}
	for _, n := range niveles {
		esfuerzos, nombres = append(esfuerzos, n), append(nombres, p.tx("esfuerzo."+n))
	}
	campos := []comp.Campo{
		comp.Area("instrucciones", p.tx("receta.instrucciones"), d.instrucciones),
		p.opcion("modelo", p.tx("receta.modelo"), modelos, etiquetas, d.modelo),
		p.opcion("esfuerzo", p.tx("esfuerzo.titulo"), esfuerzos, nombres, d.esfuerzo),
	}
	if d.modelo != "" { // forzar solo vale con un modelo elegido
		campos = append(campos, comp.Interruptor("forzar", p.tx("receta.forzar"), d.forzar))
	}
	campos = append(campos, comp.Texto("respaldo", p.tx("receta.respaldo"), strings.Join(d.respaldo, ", ")),
		comp.Interruptor("estructurada", p.tx("receta.salidaEstructurada"), d.estructurada))
	if d.estructurada {
		var ids, nombresP []string
		for _, pl := range datos.Plantillas(p.lengua) {
			ids, nombresP = append(ids, pl.ID), append(nombresP, p.a(pl.Nombre))
		}
		reintentos := []string{"0", "1", "2", "3"}
		textos := []string{p.tx("receta.reintentos0"), p.tx("receta.reintentosNUno"), p.tx("receta.reintentosN", "n", "2"), p.tx("receta.reintentosN", "n", "3")}
		p.resolverModo(&d)
		campos = append(campos, p.opcion("modo", p.tx("receta.modo"), []string{modoCampos, modoJSON}, []string{p.tx("receta.modoCampos"), p.tx("receta.modoJson")}, d.modo),
			p.opcion("plantilla", p.tx("receta.plantillaCorta"), ids, nombresP, ""))
		if d.modo == modoCampos {
			campos = append(campos, p.filasCampos(d.campos, "c", 1)...)
		} else {
			campos = append(campos, comp.Area("esquema", p.tx("receta.esquema"), d.esquema))
		}
		campos = append(campos, p.opcion("reintentos", p.tx("receta.reintentos"), reintentos, textos, strconv.Itoa(d.reintentos)))
		p.modo, p.campos = d.modo, d.campos
	}
	campos = append(campos, comp.Area("mensaje", p.tx("ejecutar.mensaje"), d.mensaje), comp.Texto("archivos", p.tx("ejecutar.adjuntar"), d.archivos),
		comp.Texto("perfil", p.tx("ejecutar.perfilTrabajo"), d.perfil), comp.Numero("tiempo", p.tx("ejecutar.tiempo")+" (s)", d.tiempo))
	p.form = comp.NuevoFormulario(p.estilos, campos...)
	p.form.Validar = p.validar
	p.form.Update(tea.WindowSizeMsg{Width: max(p.c.Ancho, 10)})
	p.form.Enfocar(foco)
	p.armado = true
}

// resolverModo elige cómo se edita el esquema, como EditorReceta: el modo de antes (al principio, campos) si el editor de
// campos lo representa; si no, JSON.
func (p *Pagina) resolverModo(d *datosForm) {
	if d.modo == "" {
		d.modo = cmp.Or(p.modo, modoCampos)
	}
	if d.modo == modoCampos && d.campos == nil {
		c, ok := camposDe(d.esquema)
		if d.campos = c; !ok {
			d.modo = modoJSON
		}
	}
}

// rearmar rehace el formulario con lo que tiene, sin mover el foco.
func (p *Pagina) rearmar() { p.armar(p.leer(), p.form.Foco()) }

// leerForm saca los valores del formulario tal cual.
func (p *Pagina) leerForm() datosForm {
	f := &p.form
	r, _ := strconv.Atoi(p.valorOpcion("reintentos"))
	return datosForm{instrucciones: f.Valor("instrucciones"), modelo: p.valorOpcion("modelo"), esfuerzo: p.valorOpcion("esfuerzo"),
		forzar: f.Marcado("forzar"), estructurada: f.Marcado("estructurada"), respaldo: partir(f.Valor("respaldo")),
		esquema: f.Valor("esquema"), reintentos: r, mensaje: f.Valor("mensaje"), archivos: f.Valor("archivos"),
		perfil: f.Valor("perfil"), tiempo: f.Valor("tiempo"), modo: p.valorOpcion("modo")}
}

// leer saca los datos del formulario; en modo campos, el esquema es el que generan.
func (p *Pagina) leer() datosForm {
	d := p.leerForm()
	if d.estructurada && d.modo == modoCampos && p.modo == modoCampos {
		d.campos = p.leerCampos(p.campos, "c")
		d.esquema = sangrar([]byte(aEsquema(d.campos)))
	}
	return d
}

// partir separa el respaldo escrito: agente[/modelo][+esfuerzo], por comas o espacios.
func partir(texto string) []string {
	return strings.FieldsFunc(texto, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
}

// entrada es la del catálogo elegida (o la predeterminada); un id escrito a mano cae en su agente (entradaCatalogo).
func (p *Pagina) entrada(sel string) (m cliente.ModeloCatalogo, ok bool) {
	for _, x := range p.catalogo {
		if (sel == "" && x.Predeterminado) || (sel != "" && x.ID == sel) {
			return x, true
		}
	}
	agente, _, _ := strings.Cut(sel, "/")
	for _, x := range p.catalogo {
		if sel != "" && x.ID == agente {
			return x, true
		}
	}
	return m, false
}

// esfuerzos son los niveles que admite el modelo elegido y el texto de la opción sin nivel: el del espacio (sin modelo y
// con esfuerzo del espacio), el del agente con su nivel si se conoce o «no admite esfuerzo», como SelectorEsfuerzo.
func (p *Pagina) esfuerzos(sel string) ([]string, string) {
	m, _ := p.entrada(sel)
	if len(m.Esfuerzos) == 0 {
		return nil, p.tx("esfuerzo.noAdmite")
	}
	nivel := func(n string) string { return strings.ToLower(p.tx("esfuerzo." + n)) }
	espacio := ""
	if sel == "" {
		espacio = p.esfuerzoEspacio()
	}
	predeterminado := ""
	for _, x := range p.catalogo { // solo la entrada exacta: el de la del agente es el de otro modelo
		if (sel == "" && x.Predeterminado) || (sel != "" && x.ID == sel) {
			predeterminado = x.EsfuerzoPredeterminado
		}
	}
	switch {
	case espacio != "":
		return m.Esfuerzos, p.tx("esfuerzo.espacioCon", "nivel", nivel(espacio))
	case predeterminado != "":
		return m.Esfuerzos, p.tx("esfuerzo.agenteCon", "nivel", nivel(predeterminado))
	}
	return m.Esfuerzos, p.tx("esfuerzo.agente")
}

// esfuerzoEspacio es el que usa el espacio con su modelo predeterminado; "" el de su agente.
func (p *Pagina) esfuerzoEspacio() string {
	for _, m := range p.catalogo {
		if m.Predeterminado {
			return m.EsfuerzoEspacio
		}
	}
	return ""
}

// validar son los errores del formulario: sin mensaje no se envía y el esquema tiene que ser un objeto JSON (y lo que
// diga el servicio de él al salir del campo).
func (p *Pagina) validar(f *comp.Formulario) map[string]string {
	errores := map[string]string{}
	if strings.TrimSpace(f.Valor("mensaje")) == "" {
		errores["mensaje"] = idioma.T("Escribe un mensaje")
	}
	switch {
	case f.Marcado("estructurada") && p.modo == modoCampos:
		if problema := p.textoProblema(p.leerCampos(p.campos, "c")); problema != "" {
			errores["modo"] = problema
		}
	case f.Marcado("estructurada"):
		_, err := esquemaDe(f.Valor("esquema"))
		switch {
		case strings.TrimSpace(f.Valor("esquema")) == "":
			errores["esquema"] = p.tx("receta.esquemaVacio")
		case p.errEsquema != "":
			errores["esquema"] = p.errEsquema
		case err != nil:
			errores["esquema"] = p.tx("receta.esquemaInvalido")
		}
	}
	return errores
}

var errEsquema = errors.New("esquema")

// esquemaDe convierte el texto del esquema en JSON (esquemaDeReceta): nil sin esquema y un error si no es un objeto JSON.
func esquemaDe(texto string) (json.RawMessage, error) {
	if strings.TrimSpace(texto) == "" {
		return nil, nil
	}
	var objeto map[string]json.RawMessage
	if json.Unmarshal([]byte(texto), &objeto) != nil || objeto == nil {
		return nil, errEsquema
	}
	return json.RawMessage(texto), nil
}

// sangrar deja un JSON con dos espacios de sangría, como JSON.stringify(v, null, 2).
func sangrar(raw []byte) string {
	var b bytes.Buffer
	if json.Indent(&b, raw, "", "  ") != nil {
		return string(raw)
	}
	return b.String()
}

// ejemploEsquema es el esquema con que empieza la salida estructurada: un resumen de texto (ejemploEsquema de EditorReceta).
func ejemploEsquema(lengua string) string {
	campo := "resumen"
	if lengua == "en" {
		campo = "summary"
	}
	return sangrar([]byte(`{"type":"object","properties":{"` + campo + `":{"type":"string"}},"required":["` + campo + `"],"additionalProperties":false}`))
}

// alForm lleva un mensaje al formulario y reacciona a lo que cambió: el modelo o la salida estructurada lo rehacen, el
// mensaje programa el borrador y, al salir del esquema o de los archivos, se comprueban con el servicio.
func (p *Pagina) alForm(msg tea.Msg) tea.Cmd {
	antes := p.leer()
	foco := p.form.Foco()
	cmd := p.form.Update(msg)
	d := p.leer()
	ahora := p.form.Foco()
	var cmds []tea.Cmd
	switch {
	case d.modelo != antes.modelo:
		if d.modelo == "" {
			d.forzar = false
		}
		p.armar(d, ahora)
	case d.estructurada != antes.estructurada:
		d.esquema, d.campos, d.reintentos = "", nil, 0
		if d.estructurada {
			d.esquema = ejemploEsquema(p.lengua)
		}
		p.errEsquema = ""
		p.armar(d, ahora)
	case d.estructurada && d.modo != antes.modo:
		cmds = append(cmds, p.cambiarModo(d, antes.esquema, ahora))
	case d.estructurada && d.modo == modoCampos && forma(d.campos) != forma(p.campos):
		p.armar(d, ahora)
	case strings.HasSuffix(ahora, ".nombre") && reEspacios.MatchString(p.form.Valor(ahora)): // los nombres no llevan espacios
		p.form.Poner(ahora, reEspacios.ReplaceAllString(p.form.Valor(ahora), "_"))
	}
	if d.esquema != antes.esquema && p.errEsquema != "" {
		p.errEsquema = ""
		p.form.Revalidar()
	}
	if d.mensaje != antes.mensaje {
		cmds = append(cmds, p.programarBorrador())
	}
	if d.archivos != antes.archivos {
		p.conversor = ""
	}
	if foco != ahora {
		cmds = append(cmds, p.alSalir(foco, d))
	}
	return tea.Batch(append(cmds, cmd)...)
}

// alSalir comprueba con el servicio el campo que se deja (con Tab, ↑↓ o Esc): el esquema o los archivos.
func (p *Pagina) alSalir(campo string, d datosForm) tea.Cmd {
	switch campo {
	case "esquema":
		return p.validarEsquema(d)
	case "archivos":
		return p.comprobarConversor(rutasDe(d.archivos))
	}
	return nil
}

// teclaCampo es una tecla mientras se edita: Intro en la plantilla la aplica y Tab en los archivos completa la ruta; lo
// demás es del formulario.
func (p *Pagina) teclaCampo(k tea.KeyPressMsg) tea.Cmd {
	switch t := k.String(); {
	case t == "enter" && p.form.Foco() == "plantilla":
		return p.usarPlantilla(p.valorOpcion("plantilla"))
	case t == "tab" && p.form.Foco() == "archivos":
		valor := p.form.Valor("archivos")
		nuevo, candidatos := completar(valor)
		if nuevo != valor {
			p.form.Poner("archivos", nuevo)
			p.conversor = ""
		}
		if len(candidatos) > 1 {
			return comp.Aviso(p.a(strings.Join(candidatos, "  ")), comp.Info)
		}
		if nuevo != valor {
			return nil
		}
	}
	return p.alForm(k)
}

// usarPlantilla pone el esquema de la plantilla; si ya había uno propio, pregunta antes (usarPlantilla de EditorReceta).
func (p *Pagina) usarPlantilla(id string) tea.Cmd {
	actual := strings.TrimSpace(p.leer().esquema)
	if actual == "" || actual == strings.TrimSpace(ejemploEsquema(p.lengua)) {
		p.ponerPlantilla(id)
		return nil
	}
	for _, pl := range datos.Plantillas(p.lengua) {
		if pl.ID == id {
			p.pend = func() { p.ponerPlantilla(id) }
			return comp.Confirmar(p.tx("receta.plantillaReemplazar") + "\n\n" + p.tx("receta.plantillaReemplazarTexto", "nombre", pl.Nombre))
		}
	}
	return nil
}

// ponerPlantilla rellena el esquema y, si la receta no tiene, las instrucciones de la plantilla.
func (p *Pagina) ponerPlantilla(id string) {
	for _, pl := range datos.Plantillas(p.lengua) {
		if pl.ID != id {
			continue
		}
		d := p.leer()
		p.errEsquema = ""
		d.esquema, d.campos, d.modo = sangrar(pl.Esquema), nil, modoCampos // como la Web UI, la plantilla se abre en campos
		if strings.TrimSpace(d.instrucciones) == "" {
			d.instrucciones = pl.Instrucciones
		}
		p.armar(d, p.form.Foco())
	}
}

// validarEsquema pide al servicio que compile el esquema que había al salir del campo: dice lo mismo que diría la petición,
// con la línea si el JSON está roto. Se manda como texto JSON, así también llega uno roto. Sin servicio queda la
// comprobación local.
func (p *Pagina) validarEsquema(d datosForm) tea.Cmd {
	if !d.estructurada || strings.TrimSpace(d.esquema) == "" || p.c.Cli == nil {
		return nil
	}
	c, o, texto := p.c, p.origen(), d.esquema
	return func() tea.Msg {
		raw, _ := json.Marshal(texto)
		errores, err := c.Cli.ValidarEsquema(c.Ctx, c.Instancia, raw)
		m := msgEsquema{origen: o, texto: texto, err: err}
		if len(errores) > 0 {
			m.error = errores[0]
		}
		return m
	}
}

// solicitudDe arma la petición de la receta y el mensaje tal como la envía Probar (solicitudDe de Ejecutar.vue).
func solicitudDe(d datosForm, cargas []string, esquema json.RawMessage) nucleo.SolicitudEjecucion {
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: d.mensaje}}, Cargas: cargas,
		Instrucciones: strings.TrimSpace(d.instrucciones), Respaldo: d.respaldo, Perfil: strings.TrimSpace(d.perfil), Esfuerzo: d.esfuerzo}
	if esquema != nil {
		s.Salida = nucleo.ConfigSalida{Formato: "json", Esquema: esquema, Reintentos: d.reintentos}
	}
	if n, err := strconv.Atoi(d.tiempo); err == nil && n > 0 {
		s.Ejecucion.TiempoLimiteSegundos = n
	}
	return aplicarModelo(s, d.modelo)
}

// aplicarModelo traduce la selección del catálogo a la solicitud: perfil/nombre, agente o agente/modelo.
func aplicarModelo(s nucleo.SolicitudEjecucion, sel string) nucleo.SolicitudEjecucion {
	switch {
	case sel == "":
	case strings.HasPrefix(sel, "perfil/"):
		s.PerfilModelo = strings.TrimPrefix(sel, "perfil/")
	default:
		s.Agente, s.Modelo, _ = strings.Cut(sel, "/")
	}
	return s
}

// recetaDeSolicitud rehace la receta de una petición ya enviada (Usar de nuevo y el trabajo que llega del Historial).
func recetaDeSolicitud(s nucleo.SolicitudEjecucion) datosForm {
	d := datosForm{instrucciones: s.Instrucciones, esfuerzo: s.Esfuerzo, respaldo: slices.Clone(s.Respaldo), reintentos: s.Salida.Reintentos}
	switch {
	case s.PerfilModelo != "":
		d.modelo = "perfil/" + s.PerfilModelo
	case s.Modelo != "":
		d.modelo = s.Agente + "/" + s.Modelo
	default:
		d.modelo = s.Agente
	}
	if len(s.Salida.Esquema) > 0 {
		d.esquema, d.estructurada = sangrar(s.Salida.Esquema), true
	}
	return d
}

// recetaDePunto es la receta guardada en una API (recetaDePunto de recetas.ts).
func recetaDePunto(e contrato.EstadoPunto) datosForm {
	d := datosForm{instrucciones: e.Instrucciones, forzar: e.Forzar, esfuerzo: e.Esfuerzo, respaldo: slices.Clone(e.Respaldo), reintentos: e.ReintentosSalida}
	switch {
	case e.PerfilModelo != "":
		d.modelo = "perfil/" + e.PerfilModelo
	case e.Modelo != "":
		d.modelo = e.Agente + "/" + e.Modelo
	default:
		d.modelo = e.Agente
	}
	if len(e.Esquema) > 0 {
		raw, _ := json.Marshal(e.Esquema)
		d.esquema, d.estructurada = sangrar(raw), true
	}
	return d
}

// cargarReceta pone una receta y un mensaje en el formulario; los archivos no se recuperan y las opciones avanzadas se
// conservan, como en la Web UI.
func (p *Pagina) cargarReceta(d datosForm, mensaje string) {
	actual := p.leer()
	d.mensaje, d.perfil, d.tiempo = mensaje, actual.perfil, actual.tiempo
	p.errEsquema, p.conversor = "", ""
	p.armar(d, "mensaje")
}

// prefijoCorreccion es el texto fijo con que el servicio pide una corrección.
const prefijoCorreccion = "Tu respuesta no cumple el formato pedido:"

// mensajeDe es el último mensaje del usuario de una petición; "" si sus entradas ya se borraron. Una corrección añade la
// respuesta y el pedido de corrección: se toma el mensaje de antes (mensajeDe de recetas.ts).
func mensajeDe(t nucleo.Trabajo) string {
	m := t.Solicitud.Mensajes
	fin := len(m)
	for fin >= 3 && m[fin-2].Rol == "asistente" && m[fin-1].Rol == "usuario" && strings.HasPrefix(m[fin-1].Contenido, prefijoCorreccion) {
		fin -= 2
	}
	for i := fin - 1; i >= 0; i-- {
		if m[i].Rol == "usuario" {
			return m[i].Contenido
		}
	}
	return ""
}
