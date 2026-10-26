// Package espacios es la página Espacios de la terminal, a la par de la de la Web UI: todos los espacios (también los
// detenidos y los que ya no encuentran su carpeta), con las mismas acciones (abrir, activar, detener, confiar o dejar de
// confiar, buscar la carpeta y olvidar) y el diálogo de abrir un proyecto, con la pregunta de confianza.
package espacios

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// origen es el pagina.Origen de los mensajes de Espacios: un tipo propio, para no confundirlos con los de otra página.
type origen struct{ pagina.Origen }

type (
	msgCarga struct {
		origen
		n     int // qué carga es: solo vale la última
		lista []contrato.Espacio
		err   error
	}
	// msgAccion es la respuesta de un cambio en el servicio: qué se hizo y con qué espacio (al abrir un proyecto, el
	// abierto).
	msgAccion struct {
		origen
		accion string
		e      contrato.Espacio
		err    error
	}
)

// Las acciones de cada espacio, con los nombres de la Web UI (espacios.ts); proyecto es abrir un proyecto nuevo.
const (
	accAbrir    = "abrir"
	accActivar  = "activar"
	accDetener  = "detener"
	accConfiar  = "confiar"
	accDejar    = "dejarDeConfiar"
	accBuscar   = "buscar"
	accOlvidar  = "olvidar"
	accProyecto = "proyecto"
)

// teclaDe es la tecla de cada acción.
var teclaDe = map[string]string{accAbrir: "enter", accActivar: "a", accDetener: "d", accConfiar: "c", accDejar: "c", accBuscar: "u", accOlvidar: "x"}

// accionesDe son las de la tarjeta de la Web UI (accionesDe de espacios.ts); actual es el espacio en que está la terminal.
// Solo se olvida un proyecto que no está activo, y la global nunca.
func accionesDe(e contrato.Espacio, actual string) []string {
	var a []string
	switch e.Estado {
	case "no_encontrado":
		a = append(a, accBuscar)
	case "activo":
		if e.ID != actual {
			a = append(a, accAbrir)
		}
		a = append(a, accDetener)
	default:
		a = append(a, accActivar)
	}
	if e.Ambito == "proyecto" && e.Estado != "no_encontrado" && e.Confiado {
		a = append(a, accDejar)
	} else if e.Ambito == "proyecto" && e.Estado != "no_encontrado" {
		a = append(a, accConfiar)
	}
	if e.Ambito == "proyecto" && e.Estado != "activo" {
		a = append(a, accOlvidar)
	}
	return a
}

// ordenar pone la global primero, después los activos del más al menos usado y al final el resto por nombre (ordenar de
// espacios.ts).
func ordenar(lista []contrato.Espacio) []contrato.Espacio {
	rango := func(e contrato.Espacio) int {
		switch {
		case e.Ambito == "global":
			return 0
		case e.Estado == "activo":
			return 1
		}
		return 2
	}
	lista = slices.Clone(lista)
	slices.SortStableFunc(lista, func(a, b contrato.Espacio) int {
		if r := rango(a) - rango(b); r != 0 {
			return r
		}
		if rango(a) == 1 {
			if c := b.UltimoUso.Compare(a.UltimoUso); c != 0 {
				return c
			}
		}
		return cmp.Or(strings.Compare(nucleo.Plegar(a.Nombre), nucleo.Plegar(b.Nombre)), strings.Compare(a.Nombre, b.Nombre))
	})
	return lista
}

// nombreConfirmado: para olvidar un espacio hay que escribir su nombre (sin importar mayúsculas ni espacios al borde).
func nombreConfirmado(e contrato.Espacio, escrito string) bool {
	return strings.EqualFold(strings.TrimSpace(escrito), strings.TrimSpace(e.Nombre))
}

// pendiente es lo que la página hará si el usuario confirma.
type pendiente struct {
	accion string
	e      contrato.Espacio
}

// dialogo es el de abrir un proyecto, buscar la carpeta de un espacio u olvidarlo: un campo y, al abrir un proyecto, la
// pregunta del servicio si la hay.
type dialogo struct {
	accion string
	e      contrato.Espacio
	f      comp.Formulario
	// La carpeta ya traía la marca de Tramtai: hay que decidir si se confía, viendo sus claves de seguridad y, si parece
	// un proyecto movido, de dónde.
	confianza bool
	claves    []string
	movido    string
	dentro    string // la carpeta está dentro de este proyecto: se abre ese o se crea uno aquí
}

func (d *dialogo) decidiendo() bool { return d.confianza || d.dentro != "" }

// Pagina es la página Espacios.
type Pagina struct {
	c       pagina.Contexto
	gen     int
	lengua  string
	estilos *pagina.Estilos
	lista   comp.Lista[contrato.Espacio]
	ahora   func() time.Time

	espacios []contrato.Espacio
	n        int // la última carga pedida: solo vale su respuesta
	cargado  bool
	sinServ  bool
	err      string // el de la primera carga, sin lista que enseñar; los de después son avisos
	ocupado  bool   // hay un cambio en marcha: no se lanza otro
	pend     *pendiente
	dlg      *dialogo
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina { return &Pagina{lengua: lengua, ahora: time.Now} }

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.espacios") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaEspacios }

// EnCampo es verdadero con un diálogo con su campo a la vista: las letras son del campo. En la pregunta del servicio no
// hay texto que perder, así que las teclas comunes valen (sus respuestas, c y a, no son comunes).
func (p *Pagina) EnCampo() bool { return p.dlg != nil && !p.dlg.decidiendo() }

func (p *Pagina) tx(clave string, pares ...string) string {
	return p.estilos.Tx(p.lengua, clave, pares...)
}

// Iniciar vuelve a cargar: «Cargando…» hasta que llegue la lista. En el mismo espacio (el servicio volvió, o se vuelve por
// el menú) el diálogo abierto se conserva con lo escrito; con otro espacio, se cierra.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	if p.c.Instancia != c.Instancia {
		p.dlg = nil
	}
	p.c, p.lengua = c, c.Idioma
	p.gen++
	if p.estilos != c.Estilos { // los mismos toda la sesión: la lista se crea una vez
		p.estilos = c.Estilos
		p.lista = comp.NuevaLista(c.Estilos, func(e contrato.Espacio) string { return e.Nombre })
		p.lista.Clave = func(e contrato.Espacio) string { return e.ID }
		p.lista.Fila = p.fila
	}
	p.espacios, p.cargado, p.sinServ, p.err, p.ocupado, p.pend = nil, false, false, "", false, nil
	return p.cargar()
}

func (p *Pagina) origen() origen        { return origen{p.c.Origen(p.gen)} }
func (p *Pagina) vigente(o origen) bool { return o.Vigente(p.c, p.gen) }

// cargar pide la lista de espacios.
func (p *Pagina) cargar() tea.Cmd {
	c := p.c
	if c.Cli == nil {
		p.sinServ = true
		return pagina.SinCliente
	}
	p.n++
	m := msgCarga{origen: p.origen(), n: p.n}
	return func() tea.Msg {
		m.lista, m.err = c.Cli.Espacios(c.Ctx)
		return m
	}
}

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
		if p.dlg != nil {
			p.dlg.f.Update(tea.WindowSizeMsg{Width: m.Width})
		}
	case msgCarga:
		return p, p.alCargar(m)
	case msgAccion:
		return p, p.alAccion(m)
	case comp.Confirmado:
		return p, p.confirmado()
	case comp.Cancelado:
		p.pend = nil
	case comp.Enviado:
		if p.dlg != nil && !p.dlg.decidiendo() {
			return p, p.enviar(nil, false)
		}
	case tea.PasteMsg:
		if p.dlg != nil && !p.dlg.decidiendo() {
			return p, p.dlg.f.Update(m)
		}
	case tea.KeyPressMsg:
		return p, p.tecla(m)
	}
	return p, nil
}

func (p *Pagina) alCargar(m msgCarga) tea.Cmd {
	if !p.vigente(m.origen) || m.n != p.n {
		return nil
	}
	if m.err != nil {
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			p.sinServ = true
			return cmd
		}
		if p.cargado { // la lista que ya se ve no se tapa: el error va en un aviso
			return comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo)
		}
		p.err = nucleo.MensajeDe(m.err)
		return nil
	}
	p.sinServ, p.cargado, p.err, p.espacios = false, true, "", ordenar(m.lista)
	p.lista.Poner(p.espacios)
	return nil
}

func (p *Pagina) alAccion(m msgAccion) tea.Cmd {
	if !m.DelEspacio(p.c) { // el cambio ya está hecho en el servicio: vale aunque la página se haya vuelto a iniciar
		return nil
	}
	p.ocupado = false
	suyo := p.dlg != nil && p.dlg.accion == m.accion // el diálogo de esta acción sigue abierto
	if m.err != nil {
		if cmd := pagina.SinServicio(m.err); cmd != nil {
			return cmd
		}
		var confianza *cliente.ErrorConfianza
		var dentro *cliente.ErrorDentroDeProyecto
		switch {
		case suyo && errors.As(m.err, &confianza):
			p.dlg.confianza, p.dlg.claves, p.dlg.movido = true, confianza.Claves, confianza.Movido
			return nil
		case suyo && errors.As(m.err, &dentro):
			p.dlg.dentro = dentro.Proyecto
			return nil
		}
		return tea.Batch(comp.Aviso(nucleo.MensajeDe(m.err), comp.Fallo), p.cargar())
	}
	if suyo {
		p.dlg = nil
	}
	// La app relee los espacios (la cabecera y Ctrl+E), como api.sesion() tras cada acción en la Web UI.
	cmds := []tea.Cmd{p.cargar(), func() tea.Msg { return pagina.EspaciosCambiados{} }}
	switch {
	case m.accion == accProyecto: // como la Web UI: el proyecto abierto pasa a ser el espacio de la terminal
		cmds = append(cmds, comp.Aviso(p.tx("proyecto.abierto", "nombre", cmp.Or(m.e.Nombre, m.e.ID)), comp.Exito), cambiar(m.e.ID))
	case m.accion == accOlvidar:
		cmds = append(cmds, comp.Aviso(p.tx("espacios.olvidado", "nombre", m.e.Nombre), comp.Exito))
	case m.accion == accDetener && m.e.ID == p.c.Instancia && m.e.ID != "global": // detenido el actual, se vuelve al global
		cmds = append(cmds, cambiar("global"))
	}
	return tea.Batch(cmds...)
}

// cambiar es el Cmd que cambia el espacio de la terminal.
func cambiar(id string) tea.Cmd { return func() tea.Msg { return pagina.CambioEspacio{ID: id} } }

// accion lanza un cambio en el servicio; al volver, la página avisa si hace falta y vuelve a cargar.
func (p *Pagina) accion(acc string, e contrato.Espacio, hacer func(c pagina.Contexto) (contrato.Espacio, error)) tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return pagina.SinCliente
	}
	if p.ocupado {
		return nil
	}
	p.ocupado = true
	return func() tea.Msg {
		r, err := hacer(c)
		if r.ID == "" {
			r = e
		}
		return msgAccion{origen: o, accion: acc, e: r, err: err}
	}
}

// sinEspacio adapta una llamada que no devuelve el espacio.
func sinEspacio(err error) (contrato.Espacio, error) { return contrato.Espacio{}, err }

// hacer es la acción de una tecla sobre el espacio: lo que tiene efecto pregunta antes, como la Web UI.
func (p *Pagina) hacer(e contrato.Espacio, acc string) tea.Cmd {
	switch acc {
	case accAbrir: // como Abrir de la Web UI: cambia de espacio y después lleva a Inicio, que ya arranca en el nuevo
		return tea.Sequence(cambiar(e.ID), func() tea.Msg { return pagina.IrA{Pagina: pagina.PaginaInicio} })
	case accActivar:
		return p.accion(acc, e, func(c pagina.Contexto) (contrato.Espacio, error) { return c.Cli.ActivarEspacio(c.Ctx, e.ID) })
	case accDetener:
		return p.preguntar(acc, e, p.tx("espacios.detenerTitulo", "nombre", e.Nombre), p.tx("espacios.detenerTexto"))
	case accConfiar:
		texto := p.tx("espacios.confiarTexto")
		if len(e.Claves) > 0 { // las claves dicen lo que controlan (las describe el servicio), nunca su nombre interno
			texto += " " + p.tx("espacios.confiarClaves", "claves", strings.Join(e.Claves, ", "))
		}
		return p.preguntar(acc, e, p.tx("espacios.confiarTitulo", "nombre", e.Nombre), texto)
	case accDejar:
		return p.preguntar(acc, e, p.tx("espacios.dejarDeConfiarTitulo", "nombre", e.Nombre), p.tx("espacios.dejarDeConfiarTexto"))
	}
	p.abrirDialogo(acc, e) // buscar y olvidar piden un texto
	return nil
}

// preguntar recuerda qué se va a hacer y pregunta antes.
func (p *Pagina) preguntar(acc string, e contrato.Espacio, titulo, texto string) tea.Cmd {
	p.pend = &pendiente{accion: acc, e: e}
	return comp.Confirmar(titulo + "\n\n" + texto)
}

// confirmado hace lo que se preguntó.
func (p *Pagina) confirmado() tea.Cmd {
	pend := p.pend
	p.pend = nil
	if pend == nil {
		return nil
	}
	e := pend.e
	switch pend.accion {
	case accDetener:
		return p.accion(accDetener, e, func(c pagina.Contexto) (contrato.Espacio, error) {
			return sinEspacio(c.Cli.DetenerEspacio(c.Ctx, e.ID))
		})
	case accConfiar, accDejar:
		confiar := pend.accion == accConfiar
		return p.accion(pend.accion, e, func(c pagina.Contexto) (contrato.Espacio, error) {
			return sinEspacio(c.Cli.Confiar(c.Ctx, e.ID, confiar))
		})
	}
	return nil
}

// abrirDialogo abre el de abrir un proyecto (con la ruta), buscar la carpeta de e (con la ruta) u olvidar e (con su
// nombre).
func (p *Pagina) abrirDialogo(acc string, e contrato.Espacio) {
	campo := comp.Texto("ruta", p.tx("proyecto.ruta"), "")
	validar := func(f *comp.Formulario) map[string]string {
		if strings.TrimSpace(f.Valor("ruta")) == "" {
			return map[string]string{"ruta": p.tx("proyecto.rutaRegla")}
		}
		return nil
	}
	if acc == accOlvidar {
		campo = comp.Texto("nombre", idioma.T("Nombre del espacio"), "")
		validar = func(f *comp.Formulario) map[string]string {
			if !nombreConfirmado(e, f.Valor("nombre")) {
				return map[string]string{"nombre": p.estilos.A(idioma.Tf("Escribe «%s» tal cual", e.Nombre))}
			}
			return nil
		}
	}
	f := comp.NuevoFormulario(p.estilos, campo)
	f.Validar = validar
	f.Update(tea.WindowSizeMsg{Width: p.c.Ancho})
	p.dlg = &dialogo{accion: acc, e: e, f: f}
}

// enviar manda el diálogo al servicio; al abrir un proyecto, con lo que se decidió (confiar, crear uno aquí).
func (p *Pagina) enviar(confiar *bool, crearAqui bool) tea.Cmd {
	d := p.dlg
	ruta, e := strings.TrimSpace(d.f.Valor("ruta")), d.e
	switch d.accion {
	case accProyecto:
		return p.accion(accProyecto, e, func(c pagina.Contexto) (contrato.Espacio, error) {
			return c.Cli.AbrirProyecto(c.Ctx, ruta, confiar, crearAqui)
		})
	case accBuscar:
		return p.accion(accBuscar, e, func(c pagina.Contexto) (contrato.Espacio, error) { return c.Cli.Ubicar(c.Ctx, e.ID, ruta) })
	case accOlvidar:
		nombre := d.f.Valor("nombre")
		return p.accion(accOlvidar, e, func(c pagina.Contexto) (contrato.Espacio, error) {
			return sinEspacio(c.Cli.Olvidar(c.Ctx, e.ID, nombre))
		})
	}
	return nil
}

// elegido es el espacio del cursor; con «Cargando…» no hay ninguno.
func (p *Pagina) elegido() (contrato.Espacio, bool) {
	if !p.cargado {
		return contrato.Espacio{}, false
	}
	return p.lista.Seleccion()
}

// tecla atiende una tecla: el diálogo abierto primero y, si no, la página y la lista.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	t := k.String()
	if p.dlg != nil {
		return p.teclaDialogo(t, k)
	}
	switch t {
	case "esc":
		return pagina.Atras
	case "r":
		return p.cargar()
	case "n":
		p.abrirDialogo(accProyecto, contrato.Espacio{})
		return nil
	}
	if e, hay := p.elegido(); hay {
		for _, acc := range accionesDe(e, p.c.Instancia) {
			if teclaDe[acc] == t {
				return p.hacer(e, acc)
			}
		}
	}
	if p.cargado {
		p.lista.Update(k)
	}
	return nil
}

// teclaDialogo: con la pregunta del servicio, sus respuestas (Esc vuelve a la ruta); si no, Esc cierra, Tab completa la
// ruta con las carpetas del disco y lo demás es del campo.
func (p *Pagina) teclaDialogo(t string, k tea.KeyPressMsg) tea.Cmd {
	d := p.dlg
	if d.decidiendo() {
		si, no := true, false
		switch {
		case t == "esc":
			d.confianza, d.claves, d.movido, d.dentro = false, nil, "", ""
		case t == "c" && d.confianza:
			return p.enviar(&si, false)
		case t == "a" && d.confianza:
			return p.enviar(&no, false)
		case t == "c": // dentro de otro proyecto: crear uno aquí
			return p.enviar(nil, true)
		case t == "a": // dentro de otro proyecto: abrir ese
			d.f.Poner("ruta", d.dentro)
			d.dentro = ""
			return p.enviar(nil, false)
		}
		return nil
	}
	switch {
	case t == "esc":
		p.dlg = nil
		return nil
	case t == "tab" && d.accion != accOlvidar:
		valor := d.f.Valor("ruta")
		nuevo, candidatos := pagina.CompletarRuta(valor, true)
		d.f.Poner("ruta", nuevo)
		if len(candidatos) > 1 {
			return comp.Aviso(p.estilos.A(strings.Join(candidatos, "  ")), comp.Info)
		}
		return nil
	}
	return d.f.Update(k)
}

// Teclas son las del pie: las del diálogo abierto o, en la lista, las acciones del espacio elegido, como los botones de
// su tarjeta.
func (p *Pagina) Teclas() []pagina.Tecla {
	if d := p.dlg; d != nil {
		switch {
		case d.confianza:
			return []pagina.Tecla{{Tecla: "c", Accion: p.tx("proyecto.confiarYAbrir")}, {Tecla: "a", Accion: p.tx("proyecto.abrirSinConfiar")},
				{Tecla: "Esc", Accion: idioma.T("vuelve")}}
		case d.dentro != "":
			return []pagina.Tecla{{Tecla: "a", Accion: p.tx("proyecto.abrirEse")}, {Tecla: "c", Accion: p.tx("proyecto.crearAqui")},
				{Tecla: "Esc", Accion: idioma.T("vuelve")}}
		}
		envia := map[string]string{accProyecto: p.tx("espacios.abrir"), accBuscar: p.tx("espacios.buscar"), accOlvidar: p.tx("espacios.olvidar")}[d.accion]
		teclas := []pagina.Tecla{{Tecla: "Enter", Accion: envia}}
		if d.accion != accOlvidar {
			teclas = append(teclas, pagina.Tecla{Tecla: "Tab", Accion: idioma.T("completa")})
		}
		return append(teclas, pagina.Tecla{Tecla: "Esc", Accion: idioma.T("cancela")})
	}
	var teclas []pagina.Tecla
	if e, hay := p.elegido(); hay {
		for _, acc := range accionesDe(e, p.c.Instancia) {
			tecla := teclaDe[acc]
			if tecla == "enter" {
				tecla = "Enter"
			}
			teclas = append(teclas, pagina.Tecla{Tecla: tecla, Accion: p.tx("espacios." + acc)})
		}
	}
	return append(teclas, pagina.Tecla{Tecla: "n", Accion: p.tx("proyecto.abrir")}, pagina.Tecla{Tecla: "r", Accion: p.tx("espacios.actualizar")})
}
