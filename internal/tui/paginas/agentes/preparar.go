package agentes

// Preparar los CLI de los agentes desde la terminal: instalar, iniciar sesión, actualizar y desinstalar, con las mismas
// reglas y textos que la Web UI (web/src/preparacion.ts y PanelPreparacion.vue). El servicio hace el trabajo; la página
// lo pide, sigue su estado y enseña lo que el usuario tiene que hacer (abrir el enlace, pegar un código).

import (
	"cmp"
	"slices"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	catalogo "tramtai/internal/agentes"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

// cadaSondeo es lo que espera la página entre dos consultas del estado de una preparación (como el panel de la Web UI).
const cadaSondeo = 1500 * time.Millisecond

const (
	accionInstalar      = "instalar"
	accionActualizar    = "actualizar"
	accionIniciar       = "iniciar_sesion"
	accionDesinstalar   = "desinstalar"
	versionNoComprobada = "version_no_comprobada"
)

type (
	// msgVersiones trae las versiones de los CLI; descubrir, además, hace buscar las preparaciones que ya estaban en marcha.
	msgVersiones struct {
		origen
		lista              []contrato.VersionesAgente
		err                error
		forzado, descubrir bool
	}
	// msgLanzada es la respuesta a pedir una acción: el estado con que empieza (o el de la que ya estaba en marcha).
	msgLanzada struct {
		origen
		agente string
		e      contrato.EstadoPreparacion
		err    error
	}
	msgSondeo struct{ origen }
	// msgEstados trae el estado de las preparaciones; descubierto, las que ya estaban en marcha al abrir la página.
	msgEstados struct {
		origen
		estados     map[string]contrato.EstadoPreparacion
		errs        map[string]error
		descubierto bool
	}
	msgRegistro struct {
		origen
		agente string
		e      contrato.EstadoPreparacion
		err    error
	}
	// msgCodigo y msgCancelada no llevan nada más que el resultado: lo pegado no vuelve a la página.
	msgCodigo struct {
		origen
		err error
	}
	msgCancelada struct {
		origen
		err error
	}
	msgCuenta struct {
		origen
		cuenta string
	}
)

// enMarcha: la preparación aún no terminó (listo, error y cancelado son finales; sin estado, no hay ninguna).
func enMarcha(e contrato.EstadoPreparacion) bool {
	return e.Estado != "" && !slices.Contains([]string{"listo", "error", "cancelado"}, e.Estado)
}

// pideSeguir: la versión no estaba comprobada y el usuario puede seguir igualmente (confirmar), salvo al iniciar sesión,
// que no se salta nunca.
func pideSeguir(e contrato.EstadoPreparacion) bool {
	return e.Estado == "error" && e.MotivoCodigo == versionNoComprobada && e.Accion != accionIniciar
}

// confirmarDesinstalar: el diálogo de desinstalar ya pide confirmación; con la versión sin comprobar la manda.
func confirmarDesinstalar(v contrato.VersionesAgente) bool {
	c, ok := v.Comprobadas[accionDesinstalar]
	return ok && !c
}

// masNueva compara dos versiones por sus números («2.10.0» es más nueva que «2.9.9»).
func masNueva(a, b string) bool {
	numeros := func(v string) []int {
		var n []int
		for _, parte := range strings.Split(v, ".") {
			x := 0
			for _, r := range parte {
				if !unicode.IsDigit(r) {
					break
				}
				x = x*10 + int(r-'0')
			}
			n = append(n, x)
		}
		return n
	}
	x, y := numeros(a), numeros(b)
	for i := range max(len(x), len(y)) {
		var u, w int
		if i < len(x) {
			u = x[i]
		}
		if i < len(y) {
			w = y[i]
		}
		if u != w {
			return u > w
		}
	}
	return false
}

// versionNueva es la última publicada si es más nueva que la instalada ("" si no la hay o no se sabe).
func versionNueva(v contrato.VersionesAgente) string {
	if v.Instalada != "" && v.Ultima != "" && masNueva(v.Ultima, v.Instalada) {
		return v.Ultima
	}
	return ""
}

// enlaceSeguro devuelve el enlace solo si es https y no lleva caracteres de control: lo que llega del servicio nunca se
// enseña con otro esquema.
func enlaceSeguro(url string) string {
	if !strings.HasPrefix(url, "https://") || strings.IndexFunc(url, unicode.IsControl) >= 0 || strings.ContainsAny(url, " \t") {
		return ""
	}
	return url
}

// limpio quita de una línea de la salida del CLI lo que no es texto (secuencias de escape, controles).
func limpio(linea string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, ansi.Strip(linea))
}

// avisosSesion es lo que la cuenta del proveedor exige antes de iniciar sesión y no es un ajuste del CLI: Codex entra por
// código de dispositivo, y ChatGPT pide activarlo una vez en sus ajustes de seguridad.
var avisosSesion = map[string]struct{ texto, enlace string }{
	"codex": {"preparar.avisoCodex", "https://chatgpt.com/#settings/Security"},
}

// opciones es lo que se puede hacer ahora con un agente, según su estado y lo que dicen sus versiones.
type opciones struct {
	instalar, iniciar, actualizar, desinstalar bool
	aviso                                      string // la versión que usaría no está comprobada
}

// conSesion: el agente inicia sesión con su CLI (su ficha admite la suscripción); si no, va con clave (Antigravity).
func conSesion(agente string) bool { return catalogo.AdmiteSuscripcion(agente) }

// avisoNoComprobada es el aviso de una acción con la versión sin comprobar (para iniciar sesión, el motivo del servicio,
// que trae la orden para la consola).
func (p *Pagina) avisoNoComprobada(accion, agente string, v contrato.VersionesAgente) string {
	if accion == accionIniciar && v.MotivoCodigo == versionNoComprobada && v.Motivo != "" {
		return v.Motivo
	}
	if accion == accionActualizar {
		return p.tx("preparar.noComprobada.actualizar")
	}
	titulo := p.tx("preparar.noComprobadaTitulo", "agente", nombre(agente)) + "."
	if accion == accionInstalar && conSesion(agente) {
		return titulo + " " + p.tx("preparar.noComprobada.instalar")
	}
	return titulo
}

// opcionesDe son las acciones de preparar que ofrece el agente: no instalado → instalar; sin sesión, con la sesión sin
// responder o con una clave en lugar de la suscripción → iniciar sesión; instalado → actualizar y desinstalar. Si la receta
// no tiene la acción, nada; si la versión no está comprobada, el aviso, y sin iniciar sesión cuando el servicio no la habilita.
func (p *Pagina) opcionesDe(a contrato.Adaptador) opciones {
	v, ok := p.versiones[a.Agente]
	if !ok {
		return opciones{}
	}
	var o opciones
	principal := ""
	switch {
	case a.Estado == contrato.AgenteNoInstalado:
		principal = accionInstalar
	case slices.Contains([]contrato.EstadoAgente{contrato.AgenteSinSesion, contrato.AgenteSesionNoResponde, contrato.AgenteUsaClave}, a.Estado):
		principal = accionIniciar
	case a.Estado == contrato.AgenteListo && versionNueva(v) != "":
		principal = accionActualizar
	}
	if comprobada, hay := v.Comprobadas[principal]; principal != "" && hay && !comprobada {
		o.aviso = p.avisoNoComprobada(principal, a.Agente, v)
	}
	habilitada := func(accion string) bool { return slices.Contains(v.Acciones, accion) }
	instalado := a.Estado != contrato.AgenteNoInstalado
	o.instalar = principal == accionInstalar && habilitada(accionInstalar)
	o.iniciar = principal == accionIniciar && habilitada(accionIniciar)
	o.actualizar = instalado && habilitada(accionActualizar)
	o.desinstalar = instalado && habilitada(accionDesinstalar)
	return o
}

// etiquetaAccion es el texto de la acción de iniciar sesión: «Iniciar sesión» si no hay sesión y «Volver a iniciar sesión»
// si la hay pero no responde o se pasó a una clave.
func (p *Pagina) etiquetaSesion(a contrato.Adaptador) string {
	if a.Estado == contrato.AgenteSinSesion {
		return p.tx("preparar.iniciarSesion")
	}
	return p.tx("preparar.volverAIniciar")
}

// teclasPreparar son las teclas de preparar que valen ahora, para el pie: las de la acción del agente elegido y las del
// seguimiento de la preparación que se enseña.
func (p *Pagina) teclasPreparar() []pagina.Tecla {
	var t []pagina.Tecla
	baja := func(clave string) string { return strings.ToLower(p.tx(clave)) }
	if a, hay := p.elegido(); hay && p.registro == "" {
		o := p.opcionesDe(a)
		if o.instalar {
			t = append(t, pagina.Tecla{Tecla: "i", Accion: baja("preparar.instalar")})
		}
		if o.iniciar {
			t = append(t, pagina.Tecla{Tecla: "s", Accion: strings.ToLower(p.etiquetaSesion(a))})
		}
		if o.actualizar {
			t = append(t, pagina.Tecla{Tecla: "u", Accion: baja("preparar.actualizar")})
		}
		if o.desinstalar {
			t = append(t, pagina.Tecla{Tecla: "x", Accion: baja("preparar.desinstalar")}, pagina.Tecla{Tecla: "X", Accion: idioma.T("desinstalar y borrar sus datos")})
		}
		if _, ok := p.versiones[a.Agente]; ok {
			t = append(t, pagina.Tecla{Tecla: "l", Accion: baja("preparar.verRegistro")})
		}
	}
	if e, ok := p.estados[p.foco]; ok {
		if pideSeguir(e) {
			t = append(t, pagina.Tecla{Tecla: "y", Accion: baja("preparar.seguir")})
		}
		if e.Estado == "esperando_usuario" && e.PidePegar && !p.pegando {
			t = append(t, pagina.Tecla{Tecla: "e", Accion: idioma.T("pegar código")})
		}
	}
	if p.hayEnMarcha() {
		t = append(t, pagina.Tecla{Tecla: "k", Accion: idioma.T("cancela")})
	}
	return t
}

// hayEnMarcha dice si se sigue alguna preparación que no ha terminado.
func (p *Pagina) hayEnMarcha() bool {
	for _, e := range p.estados {
		if enMarcha(e) {
			return true
		}
	}
	return false
}

// objetivo es el agente de la preparación en marcha a la que se refiere k: la que se enseña o, si esa no lo está, la
// primera que lo esté.
func (p *Pagina) objetivo() (string, bool) {
	if e, ok := p.estados[p.foco]; ok && enMarcha(e) {
		return p.foco, true
	}
	ids := make([]string, 0, len(p.estados))
	for id := range p.estados {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if enMarcha(p.estados[id]) {
			return id, true
		}
	}
	return "", false
}

// noDisponible es el aviso de una tecla de preparar que no vale para este agente ahora.
func noDisponible() tea.Cmd {
	return comp.Aviso(idioma.T("Esta acción no está disponible ahora para este agente"), comp.Advertencia)
}

// pedir lanza una acción de preparar sobre el agente elegido si el servicio la habilita.
func (p *Pagina) pedir(a contrato.Adaptador, accion string) tea.Cmd {
	o := p.opcionesDe(a)
	valida := map[string]bool{accionInstalar: o.instalar, accionIniciar: o.iniciar, accionActualizar: o.actualizar, accionDesinstalar: o.desinstalar}[accion]
	if !valida {
		return noDisponible()
	}
	if e, ok := p.estados[a.Agente]; ok && enMarcha(e) {
		return comp.Aviso(p.tx("error.ya_en_curso"), comp.Advertencia)
	}
	return p.lanzar(a.Agente, accion, false, false)
}

// pedirDesinstalar pregunta antes de desinstalar el agente elegido; borrar, con X, también su sesión y sus ajustes.
func (p *Pagina) pedirDesinstalar(a contrato.Adaptador, borrar bool) tea.Cmd {
	if !p.opcionesDe(a).desinstalar {
		return noDisponible()
	}
	if e, ok := p.estados[a.Agente]; ok && enMarcha(e) {
		return comp.Aviso(p.tx("error.ya_en_curso"), comp.Advertencia)
	}
	return p.preguntarDesinstalar(a, borrar)
}

// falloSimple avisa de un fallo de una petición suelta; si es que el servicio no responde, lo dice la app.
func (p *Pagina) falloSimple(err error) tea.Cmd {
	if cmd := pagina.SinServicio(err); cmd != nil {
		return cmd
	}
	return comp.Aviso(nucleo.MensajeDe(err), comp.Fallo)
}

// alCargarVersiones guarda las versiones; si fallan, las tarjetas siguen como antes (solo se avisa si el usuario las pidió).
func (p *Pagina) alCargarVersiones(m msgVersiones) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	if m.err != nil {
		if m.forzado && !pagina.NoResponde(m.err) { // si el servicio no responde, ya lo dice la comprobación de la lista
			return p.falloSimple(m.err)
		}
		return nil
	}
	p.versiones = map[string]contrato.VersionesAgente{}
	for _, v := range m.lista {
		p.versiones[v.Agente] = v
	}
	p.refrescar()
	if m.descubrir {
		return p.descubrir()
	}
	return nil
}

// alLanzar sigue la preparación que el servicio aceptó; si no, enseña por qué (peticiones en curso, versión que pide
// confirmar…). Lo hecho en el servicio vale aunque la página se haya ocultado.
func (p *Pagina) alLanzar(m msgLanzada) tea.Cmd {
	if !m.DelEspacio(p.c) {
		return nil
	}
	if m.err != nil {
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			return cmd
		}
		p.fallos[m.agente] = nucleo.MensajeDe(m.err)
		return nil
	}
	p.estados[m.agente] = m.e
	p.sincronizarCampo()
	if enMarcha(m.e) {
		return p.sondear()
	}
	return p.alTerminar()
}

// preguntarDesinstalar pide confirmar antes de desinstalar: el CLI deja de estar también fuera de Tramtai. Con borrar, se
// borran también su sesión y sus ajustes.
func (p *Pagina) preguntarDesinstalar(a contrato.Adaptador, borrar bool) tea.Cmd {
	v := p.versiones[a.Agente]
	p.pend = &pendiente{agente: a.Agente, accion: accionDesinstalar, borrar: borrar, confirmar: confirmarDesinstalar(v)}
	texto := p.tx("preparar.desinstalarTitulo", "agente", nombre(a.Agente)) + "\n\n" + idioma.T("Se desinstala con su orden oficial y deja de estar también fuera de Tramtai.") +
		"\n\n" + p.tx("preparar.borrarDatos") + ": " + p.si(borrar)
	if !borrar {
		texto += "\n" + idioma.T("Para borrarlos también, cancela y pulsa X.")
	}
	if p.pend.confirmar {
		texto += "\n\n" + p.tx("preparar.noComprobadaTitulo", "agente", nombre(a.Agente)) + ". " + p.tx("preparar.noComprobada.desinstalar")
	}
	return comp.Confirmar(texto)
}

// lanzar pide la acción al servicio y empieza a seguirla. Con otra ya en marcha para el agente (esta terminal y la Web UI a
// la vez), sigue esa.
func (p *Pagina) lanzar(agente, accion string, borrar, confirmar bool) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	p.foco, p.acciones[agente], p.borrado[agente] = agente, accion, borrar
	delete(p.estados, agente)
	delete(p.fallos, agente)
	delete(p.errConsulta, agente)
	p.sincronizarCampo()
	return func() tea.Msg {
		e, err := c.Cli.Preparar(c.Ctx, c.Instancia, agente, contrato.PedidoPreparacion{Accion: accion, BorrarDatos: borrar, Confirmar: confirmar})
		if nucleo.CodigoDe(err) == nucleo.CodigoYaEnCurso {
			e, err = c.Cli.Preparacion(c.Ctx, c.Instancia, agente)
		}
		return msgLanzada{origen: o, agente: agente, e: e, err: err}
	}
}

// seguir repite, con confirmar, la acción que se paró por una versión sin comprobar.
func (p *Pagina) seguir() tea.Cmd {
	e, ok := p.estados[p.foco]
	if !ok || !pideSeguir(e) {
		return nil
	}
	return p.lanzar(p.foco, e.Accion, p.borrado[p.foco], true)
}

// cancelar corta la preparación en marcha.
func (p *Pagina) cancelar() tea.Cmd {
	agente, ok := p.objetivo()
	if !ok {
		return nil
	}
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	return func() tea.Msg {
		return msgCancelada{origen: o, err: c.Cli.CancelarPreparacion(c.Ctx, c.Instancia, agente)}
	}
}

// verRegistro pide la última preparación del agente para enseñar las líneas de su salida.
func (p *Pagina) verRegistro(a contrato.Adaptador) tea.Cmd {
	c, o, agente := p.c, p.origen(), a.Agente
	if c.Cli == nil {
		return pagina.SinCliente
	}
	return func() tea.Msg {
		e, err := c.Cli.Preparacion(c.Ctx, c.Instancia, agente)
		return msgRegistro{origen: o, agente: agente, e: e, err: err}
	}
}

// textoRegistro son las últimas líneas de la última preparación, tal cual (los enlaces se ven como texto).
func (p *Pagina) textoRegistro(e contrato.EstadoPreparacion) string {
	if e.Estado == "" {
		return p.tx("preparar.sinRegistro")
	}
	var l []string
	for _, linea := range e.Lineas {
		l = append(l, limpio(linea))
	}
	if len(l) == 0 {
		l = append(l, p.tx("preparar.sinRegistro"))
	}
	if e.Mensaje != "" {
		l = append(l, "", limpio(e.Mensaje))
	}
	return strings.Join(l, "\n")
}

// cargarVersiones pide las versiones de los CLI. Lanza un proceso por CLI en el servicio: se pide al abrir la página y al
// volver a comprobar (forzar), no en cada refresco.
func (p *Pagina) cargarVersiones(forzar, descubrir bool) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return nil
	}
	return func() tea.Msg {
		l, err := c.Cli.VersionesAgentes(c.Ctx, c.Instancia, forzar)
		return msgVersiones{origen: o, lista: l, err: err, forzado: forzar, descubrir: descubrir}
	}
}

// cargarCuenta pide, en el servidor, la cuenta del sistema con la que corre el servicio: se prepara para ella.
func (p *Pagina) cargarCuenta() tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil || c.Entorno != "servidor" {
		return nil
	}
	return func() tea.Msg {
		a, err := c.Cli.Acerca(c.Ctx)
		if err != nil {
			return nil
		}
		return msgCuenta{origen: o, cuenta: a.Cuenta}
	}
}

// descubrir busca las preparaciones que ya estaban en marcha (la lanzó otra pantalla, o antes de abrir esta página).
func (p *Pagina) descubrir() tea.Cmd {
	c, o := p.c, p.origen()
	ids := make([]string, 0, len(p.versiones))
	for id := range p.versiones {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return func() tea.Msg {
		m := msgEstados{origen: o, estados: map[string]contrato.EstadoPreparacion{}, descubierto: true}
		for _, id := range ids {
			if e, err := c.Cli.Preparacion(c.Ctx, c.Instancia, id); err == nil && enMarcha(e) {
				m.estados[id] = e
			}
		}
		return m
	}
}

// consultar pide el estado de cada preparación en marcha.
func (p *Pagina) consultar() tea.Cmd {
	c, o := p.c, p.origen()
	var ids []string
	for id, e := range p.estados {
		if enMarcha(e) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	slices.Sort(ids)
	return func() tea.Msg {
		m := msgEstados{origen: o, estados: map[string]contrato.EstadoPreparacion{}, errs: map[string]error{}}
		for _, id := range ids {
			if e, err := c.Cli.Preparacion(c.Ctx, c.Instancia, id); err != nil {
				m.errs[id] = err
			} else {
				m.estados[id] = e
			}
		}
		return m
	}
}

// sondear programa la siguiente consulta mientras haya una preparación en marcha; solo hay una espera a la vez.
func (p *Pagina) sondear() tea.Cmd {
	if p.sondeando || !p.hayEnMarcha() {
		return nil
	}
	p.sondeando = true
	return pagina.Tras(p.origen(), cadaSondeo, p.espera, func(o origen) tea.Msg { return msgSondeo{o} })
}

// alTerminar refresca la lista y las versiones: la preparación cambió lo instalado.
func (p *Pagina) alTerminar() tea.Cmd {
	return tea.Batch(p.comprobar(), p.cargarVersiones(false, false))
}

// aplicar recoge el estado de las preparaciones que llegó y devuelve lo que sigue: el refresco si alguna terminó y la
// siguiente consulta.
func (p *Pagina) aplicar(m msgEstados) tea.Cmd {
	ids := make([]string, 0, len(m.estados))
	for id := range m.estados {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	terminada := false
	for _, id := range ids {
		e := m.estados[id]
		antes, tenia := p.estados[id]
		if m.descubierto && tenia {
			continue // la lanzó esta página mientras tanto: manda su propio seguimiento
		}
		p.estados[id] = e
		delete(p.errConsulta, id)
		if p.foco == "" || (m.descubierto && !enMarcha(p.estados[p.foco])) {
			p.foco = id
		}
		terminada = terminada || (tenia && enMarcha(antes) && !enMarcha(e))
	}
	for id, err := range m.errs {
		if cmd := pagina.SinServicio(err); cmd != nil {
			return cmd
		}
		if p.errConsulta[id]++; p.errConsulta[id] >= 3 { // como la Web UI: tres intentos y, si sigue fallando, el error
			e := p.estados[id]
			e.Estado = "error"
			p.estados[id], p.fallos[id] = e, nucleo.MensajeDe(err)
		}
	}
	p.sincronizarCampo()
	cmds := []tea.Cmd{p.sondear()}
	if terminada {
		cmds = append(cmds, p.alTerminar())
	}
	return tea.Batch(cmds...)
}

// sincronizarCampo muestra el campo para pegar mientras el CLI espera un código y lo vacía cuando deja de esperarlo.
func (p *Pagina) sincronizarCampo() {
	e, ok := p.estados[p.foco]
	quiere := ok && e.Estado == "esperando_usuario" && e.PidePegar
	if quiere && !p.hayCampo {
		p.pegando = true
		p.pegar.Focus()
	}
	if !quiere {
		p.pegar.Reset()
		p.pegar.Blur()
		p.pegando = false
	}
	p.hayCampo = quiere
}

// nuevoCampo es el campo donde se pega el código del inicio de sesión: oculta lo escrito (es de un solo uso).
func nuevoCampo() textinput.Model {
	c := textinput.New()
	c.EchoMode = textinput.EchoPassword
	c.CharLimit = 2048
	c.SetStyles(textinput.Styles{})
	return c
}

// teclaCampo atiende una tecla con el campo para pegar a la vista: Intro envía, Esc suelta el foco.
func (p *Pagina) teclaCampo(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "esc":
		p.pegando = false
		p.pegar.Blur()
		return nil
	case "enter":
		return p.enviarCodigo()
	}
	var cmd tea.Cmd
	p.pegar, cmd = p.pegar.Update(k)
	return cmd
}

// enviarCodigo pasa al CLI lo pegado y vacía el campo en el acto: no se queda en la página ni en ningún mensaje.
func (p *Pagina) enviarCodigo() tea.Cmd {
	codigo := strings.TrimSpace(p.pegar.Value())
	p.pegar.Reset()
	c, o, agente := p.c, p.origen(), p.foco
	if codigo == "" || c.Cli == nil {
		return nil
	}
	return func() tea.Msg {
		return msgCodigo{origen: o, err: c.Cli.CodigoPreparacion(c.Ctx, c.Instancia, agente, codigo)}
	}
}

// ---- lo que se ve ----

// titular es lo que dice la preparación del agente ahora, en una frase.
func (p *Pagina) titular(agente string) string {
	e, hay := p.estados[agente]
	n := nombre(agente)
	accion := cmp.Or(e.Accion, p.acciones[agente])
	if !hay {
		if p.fallos[agente] != "" && accion != "" {
			return p.tx("preparar.fallo."+accion, "agente", n)
		}
		return p.tx("preparar.empezando", "agente", n)
	}
	switch e.Estado {
	case "esperando_usuario":
		return p.tx(map[bool]string{true: "preparar.esperaPegar", false: "preparar.esperaCodigo"}[e.PidePegar], "agente", n)
	case "listo":
		versiones := []string{"agente", n, "antes", cmp.Or(e.VersionAntes, idioma.T("Sin dato")), "despues", cmp.Or(e.VersionDespues, idioma.T("Sin dato"))}
		if e.MotivoCodigo != "" && e.Accion == accionInstalar {
			return p.tx("preparar.listoSinSesion", versiones...)
		}
		return p.tx("preparar.listo."+e.Accion, versiones...)
	case "cancelado":
		return p.tx("preparar.cancelado", "agente", n)
	case "error":
		if e.MotivoCodigo == versionNoComprobada {
			return p.tx("preparar.noComprobadaTitulo", "agente", n)
		}
		return p.tx("preparar.fallo."+e.Accion, "agente", n)
	}
	titulo := p.tx("preparar.estado."+e.Estado, "agente", n)
	if e.Etapa != "" {
		titulo += " · " + p.tx("preparar.etapa."+e.Etapa)
	}
	return titulo
}

// codigosExplicados son los códigos de error de preparar con explicación en preparar.error.*.
var codigosExplicados = []string{"instalador_fallo", "no_encontrado_tras_instalar", "plazo_agotado", "salida_no_reconocida", "inicio_sesion_fallo", "sin_red",
	"no_disponible_en_servidor", "interrumpida"}

// explicacion dice qué pasó y qué hacer (el detalle del servicio va debajo).
func (p *Pagina) explicacion(e contrato.EstadoPreparacion) string {
	if e.MotivoCodigo == versionNoComprobada {
		return p.tx("preparar.noComprobada."+e.Accion, "agente", nombre(e.Agente))
	}
	if slices.Contains(codigosExplicados, e.MotivoCodigo) {
		return p.tx("preparar.error." + e.MotivoCodigo)
	}
	return ""
}

// panel es lo que se ve de la preparación que se sigue: qué pasa, lo que el usuario tiene que hacer (abrir el enlace,
// pegar el código), por qué falló y las últimas líneas de la salida. El enlace y el código se ven como texto, para
// seleccionarlos y copiarlos: la terminal no abre ningún navegador.
func (p *Pagina) panel(ancho int) []string {
	g := p.foco
	if g == "" {
		return nil
	}
	e, hay := p.estados[g]
	if !hay && p.fallos[g] == "" && p.acciones[g] == "" {
		return nil
	}
	s := p.estilos
	ancho = max(ancho, 10)
	var l []string
	texto := func(estilo func(...string) string, t string) {
		for _, linea := range s.Envolver(t, ancho, 0) {
			l = append(l, estilo(linea))
		}
	}
	plano := func(t ...string) string { return strings.Join(t, "") }
	tenue := func(t ...string) string { return s.Tenue.Render(strings.Join(t, "")) }
	if p.cuenta != "" {
		texto(tenue, p.tx("preparar.cuentaServicio", "cuenta", p.cuenta))
	}
	// El titular, con su icono y su color: éxito, aviso (un listo con motivo, o seguir con una versión sin comprobar) o error.
	icono, tono := s.Iconos.Activo, s.Acento
	switch {
	case !hay:
		icono, tono = s.Iconos.Fallo, s.Error
		if p.fallos[g] == "" {
			icono, tono = s.Iconos.Activo, s.Acento
		}
	case e.Estado == "listo" && e.MotivoCodigo == "", e.Estado == "esperando_usuario":
		icono, tono = s.Iconos.Ok, s.Exito
		if e.Estado == "esperando_usuario" {
			icono, tono = s.Iconos.Activo, s.Acento
		}
	case e.Estado == "listo", pideSeguir(e):
		icono, tono = "!", s.Advertencia
	case e.Estado == "cancelado":
		icono, tono = s.Iconos.Inactivo, s.Tenue
	case e.Estado == "error":
		icono, tono = s.Iconos.Fallo, s.Error
	}
	for i, linea := range s.Envolver(p.titular(g), ancho-2, 0) {
		marca := "  "
		if i == 0 {
			marca = icono + " "
		}
		l = append(l, tono.Render(marca+linea))
	}
	if e.Estado == "esperando_usuario" {
		if aviso, ok := avisosSesion[g]; ok {
			texto(plano, p.tx(aviso.texto))
			texto(plano, p.tx("preparar.abrirAjustesSeguridad")+": "+aviso.enlace)
		}
		if enlace := enlaceSeguro(e.Enlace); enlace != "" {
			// Entero y partido en líneas del ancho de la terminal: así se selecciona y se copia.
			l = append(l, strings.Split(ansi.Hardwrap(enlace, ancho, true), "\n")...)
		}
		if e.Codigo != "" {
			l = append(l, tenue(idioma.T("Código")+": ")+s.Titulo.Render(limpio(e.Codigo)))
		}
		if e.PidePegar {
			p.pegar.Prompt = s.Iconos.Vertical + " "
			p.pegar.Placeholder = p.tx("preparar.pegar")
			p.pegar.SetWidth(max(ancho-3, 8))
			l = append(l, p.pegar.View())
		}
	}
	if e.Estado == "error" || (e.Estado == "listo" && e.Mensaje != "") {
		if x := p.explicacion(e); x != "" && e.Estado == "error" {
			texto(plano, x)
		}
		if e.Mensaje != "" {
			texto(func(t ...string) string {
				if e.Estado == "listo" {
					return s.Advertencia.Render(strings.Join(t, ""))
				}
				return tenue(t...)
			}, limpio(e.Mensaje))
		}
	}
	if f := p.fallos[g]; f != "" {
		texto(plano, f)
	}
	if n := len(e.Lineas); n > 0 && (e.Estado == "error" || enMarcha(e)) {
		for _, linea := range e.Lineas[max(n-3, 0):] {
			l = append(l, tenue(s.Recortar(limpio(linea), ancho)))
		}
	}
	return l
}
