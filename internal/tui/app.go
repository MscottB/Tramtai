// Package tui es la terminal interactiva: las mismas páginas que la Web UI, con su menú por grupos, una cabecera con el
// espacio activo y el estado del servicio, y un pie con las teclas. Cada página vive en internal/tui/paginas/<página> y
// habla con la app por los mensajes de internal/tui/pagina; las piezas comunes están en internal/tui/comp. Todo pasa
// por internal/cliente, con el token de servicio.
package tui

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/agentes"
	"tramtai/internal/tui/paginas/ejecutar"
	"tramtai/internal/tui/paginas/historial"
	"tramtai/internal/tui/paginas/inicio"
)

// Opciones configura la terminal.
type Opciones struct {
	Datos      string
	Proyecto   string // raíz del proyecto actual; vacía fuera de un proyecto
	IDProyecto string
	Iniciar    func() (*cliente.Cliente, error) // inicia el servicio en segundo plano
}

// Ejecutar abre la terminal hasta que el usuario sale.
func Ejecutar(o Opciones) error {
	m := nuevo(o)
	defer m.cerrar()
	_, err := tea.NewProgram(m).Run()
	return err
}

const anchoMinimo, altoMinimo = 60, 20

// fabricas crea cada página por su clave del menú; las que aún no están enseñan «Próximamente».
var fabricas = map[string]func(idioma string) pagina.Pagina{
	pagina.PaginaInicio:    inicio.Nueva,
	pagina.PaginaAgentes:   agentes.Nueva,
	pagina.PaginaEjecutar:  ejecutar.Nueva,
	pagina.PaginaHistorial: historial.Nueva,
}

type modelo struct {
	o           Opciones
	idioma      string
	estilos     *pagina.Estilos
	ancho, alto int

	cli         *cliente.Cliente
	instancia   string
	espacios    []contrato.Espacio
	caido       error // el servicio no responde: se avisa y r reintenta
	conectando  bool  // la conexión inicial o un reintento con r: la cabecera lo dice
	comprobando bool  // hay una comprobación en marcha (latido o r): no se lanza otra

	paginas    map[string]pagina.Pagina
	cancelar   map[string]context.CancelFunc // el Ctx de la última vez que se inició cada página
	actual     string
	pila       []string // de dónde se vino con IrA o ?: Volver la recorre
	enMenu     bool     // el menú tiene el foco (y, plegado, se ve)
	cursorMenu int

	selector *comp.Selector     // el cambio rápido de espacio
	pregunta *comp.Confirmacion // la confirmación que pidió la página, la de salir o el último error entero
	saliendo bool               // la pregunta abierta es la de salir: s cierra la terminal
	avisos   comp.Avisos
}

func idiomaDe() string {
	if idioma.Ingles() {
		return "en"
	}
	return "es"
}

func nuevo(o Opciones) *modelo { return nuevoEn(o, idiomaDe()) }

func nuevoEn(o Opciones, lengua string) *modelo {
	return &modelo{o: o, idioma: lengua, estilos: nuevosEstilos(), ancho: 100, alto: 30, paginas: map[string]pagina.Pagina{},
		cancelar: map[string]context.CancelFunc{}, actual: pagina.PaginaInicio}
}

// pagina es la de esa clave, creada la primera vez que se pide: así conserva su estado al ir y volver.
func (m *modelo) pagina(clave string) pagina.Pagina {
	p, ok := m.paginas[clave]
	if !ok {
		if f := fabricas[clave]; f != nil {
			p = f(m.idioma)
		} else {
			p = &proximamente{titulo: datos.Texto(m.idioma, "menu."+clave), estilos: m.estilos}
		}
		m.paginas[clave] = p
	}
	return p
}

// area es el tamaño que le queda a la página: sin la cabecera, la línea de estado, el pie ni, si se ve fijo, el menú.
func (m *modelo) area() (ancho, alto int) {
	ancho = m.ancho
	if m.ancho >= anchoMenuFijo {
		ancho -= anchoMenu
	}
	return max(ancho, 0), max(m.alto-3, 0)
}

// iniciar vuelve a iniciar la página actual, con un Ctx nuevo (el anterior se cancela) y, si hay datos, se los pasa.
func (m *modelo) iniciar(extra any) tea.Cmd {
	if cancelar := m.cancelar[m.actual]; cancelar != nil {
		cancelar()
	}
	ctx, cancelar := context.WithCancel(context.Background())
	m.cancelar[m.actual] = cancelar
	ancho, alto := m.area()
	cmd := m.pagina(m.actual).Iniciar(pagina.Contexto{Ctx: ctx, Cli: m.cli, Instancia: m.instancia, Idioma: m.idioma, Estilos: m.estilos,
		Ancho: ancho, Alto: alto, Proyecto: m.o.Proyecto, Datos: m.o.Datos})
	if extra == nil {
		return cmd
	}
	return tea.Batch(cmd, m.aPagina(extra))
}

// cerrar cancela el Ctx de todas las páginas: al cambiar de espacio y al salir.
func (m *modelo) cerrar() {
	for clave, cancelar := range m.cancelar {
		cancelar()
		delete(m.cancelar, clave)
	}
}

// ir lleva a una página del menú. Con historia (IrA, ?), Volver regresa a la de ahora; por el menú y los atajos se
// empieza una historia nueva. Una clave que no existe no hace nada.
func (m *modelo) ir(clave string, extra any, historia bool) tea.Cmd {
	if !slices.Contains(ordenMenu(), clave) {
		return nil
	}
	switch {
	case !historia:
		m.pila = nil
	case clave != m.actual:
		m.pila = append(m.pila, m.actual)
	}
	if clave != m.actual {
		m.ocultar()
	}
	m.actual, m.enMenu = clave, false
	return m.iniciar(extra)
}

// ocultar avisa a la página actual, si lo quiere saber, de que deja de verse.
func (m *modelo) ocultar() {
	if o, ok := m.paginas[m.actual].(pagina.Ocultable); ok {
		o.Ocultar()
	}
}

// volver regresa a la página anterior tal como estaba; solo se vuelve a iniciar si su Ctx se canceló (cambió el
// espacio).
func (m *modelo) volver() tea.Cmd {
	if len(m.pila) == 0 {
		return nil
	}
	m.ocultar()
	m.actual, m.pila, m.enMenu = m.pila[len(m.pila)-1], m.pila[:len(m.pila)-1], false
	if _, viva := m.cancelar[m.actual]; viva {
		if o, ok := m.pagina(m.actual).(pagina.Ocultable); ok {
			return o.Mostrar()
		}
		return nil
	}
	return m.iniciar(nil)
}

// aPagina da un mensaje solo a la página actual: las teclas, lo pegado y las respuestas de los componentes.
func (m *modelo) aPagina(msg tea.Msg) tea.Cmd {
	p, cmd := m.pagina(m.actual).Update(msg)
	m.paginas[m.actual] = p
	return cmd
}

// difundir da un mensaje a todas las páginas creadas: así le llega a la suya la respuesta de un Cmd aunque ya no sea la
// actual.
func (m *modelo) difundir(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for clave, p := range m.paginas {
		p, cmd := p.Update(msg)
		m.paginas[clave] = p
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// Init pide el color del fondo y se conecta al servicio; el resultado programa el primer latido.
func (m *modelo) Init() tea.Cmd {
	m.conectando, m.comprobando = true, true
	return tea.Batch(tea.RequestBackgroundColor, m.conectar(false, true, true))
}

func (m *modelo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd := m.avisos.Update(msg); cmd != nil {
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ancho, m.alto = msg.Width, msg.Height
		ancho, alto := m.area()
		return m, m.difundir(tea.WindowSizeMsg{Width: ancho, Height: alto})
	case tea.BackgroundColorMsg:
		rellenar(m.estilos, msg.IsDark())
	case msgConexion:
		return m, m.conectado(msg)
	case msgLatido:
		if m.comprobando { // la comprobación en marcha ya traerá su resultado; la cadena sigue
			return m, latir()
		}
		m.comprobando = true
		return m, m.conectar(false, false, true)
	case pagina.ErrorServicio:
		if !m.conectando {
			m.caido = msg.Err
		}
	case pagina.CambioEspacio:
		m.instancia = msg.ID
		m.cerrar()
		return m, tea.Batch(m.difundir(msg), m.iniciar(nil))
	case pagina.IrA:
		return m, m.ir(msg.Pagina, msg.Datos, true)
	case pagina.Volver:
		return m, m.volver()
	case comp.Pregunta:
		m.pregunta, m.saliendo = &comp.Confirmacion{Texto: msg.Texto}, false
	case comp.Elegido:
		if m.selector == nil {
			return m, m.aPagina(msg)
		}
		m.selector = nil
		return m, func() tea.Msg { return pagina.CambioEspacio{ID: msg.Valor} }
	case comp.Cancelado:
		if m.selector == nil {
			return m, m.aPagina(msg)
		}
		m.selector = nil
	case comp.Confirmado, comp.Enviado, tea.PasteMsg:
		return m, m.aPagina(msg)
	case tea.KeyPressMsg:
		return m, m.tecla(msg)
	default:
		return m, m.difundir(msg)
	}
	return m, nil
}

// tecla reparte una tecla: primero lo que está abierto encima (la pregunta, el selector, el menú) y luego la página.
// Las teclas comunes no se aplican mientras la página tiene un campo con el foco. Por debajo del tamaño mínimo no se
// ve nada, así que solo valen q y Ctrl+C. Cualquier tecla quita el error de la línea de estado; ! lo enseña entero.
func (m *modelo) tecla(k tea.KeyPressMsg) tea.Cmd {
	t := k.String()
	if m.pregunta == nil {
		m.avisos.Descartar()
	}
	switch {
	case t == "ctrl+c":
		return tea.Quit
	case m.ancho < anchoMinimo || m.alto < altoMinimo:
		if t == "q" {
			return m.salir()
		}
		return nil
	case m.pregunta != nil:
		listo, cmd := m.pregunta.Update(k)
		if !listo {
			return nil
		}
		m.pregunta = nil
		if m.saliendo {
			m.saliendo = false
			if _, si := cmd().(comp.Confirmado); si {
				return tea.Quit
			}
			return nil
		}
		return cmd
	case m.selector != nil:
		return m.selector.Update(k)
	case t == "ctrl+e":
		return m.elegirEspacio()
	case m.enMenu:
		return m.teclaMenu(t)
	}
	p := m.pagina(m.actual)
	if p.EnCampo() {
		return m.aPagina(k)
	}
	switch t {
	case "q":
		return m.salir()
	case "!":
		if ultimo := m.avisos.Ultimo(); ultimo != "" {
			m.pregunta = &comp.Confirmacion{Texto: ultimo, Leer: true}
			return nil
		}
	case "tab":
		m.abrirMenu()
		return nil
	case "?":
		return m.ir(pagina.PaginaAyuda, pagina.ConAyuda{Pagina: p.Ayuda(), Teclas: slices.Concat(p.Teclas(), comunes())}, true)
	case "r":
		if m.caido != nil {
			if m.comprobando {
				return nil // un solo reintento a la vez: si no, r r arrancaría el servicio dos veces
			}
			m.conectando, m.comprobando = true, true
			return m.conectar(true, true, false)
		}
	}
	if clave, ok := atajo(t); ok {
		return m.ir(clave, nil, false)
	}
	return m.aPagina(k)
}

// salir cierra la terminal; si una página tiene algo en marcha o sin guardar (pagina.Pendiente), antes pregunta y dice qué.
func (m *modelo) salir() tea.Cmd {
	var motivos []string
	for _, clave := range ordenMenu() {
		if p, ok := m.paginas[clave].(pagina.Pendiente); ok {
			if motivo := p.Pendiente(); motivo != "" {
				motivos = append(motivos, motivo)
			}
		}
	}
	if len(motivos) == 0 {
		return tea.Quit
	}
	m.pregunta, m.saliendo = &comp.Confirmacion{Texto: idioma.T("¿Salir de la terminal?") + "\n\n" + strings.Join(motivos, "\n")}, true
	return nil
}

func (m *modelo) View() tea.View {
	v := tea.NewView(m.vista())
	v.AltScreen = true
	return v
}

// vista es la cabecera, el cuerpo (menú y página), la línea de estado y el pie, en exactamente ancho×alto. Por debajo
// del tamaño mínimo es solo el aviso.
func (m *modelo) vista() string {
	if m.ancho < anchoMinimo || m.alto < altoMinimo {
		return idioma.T("Agranda la ventana (mínimo 60×20)")
	}
	e := m.estilos
	ancho, alto := m.area()
	var cuerpo string
	switch {
	case m.pregunta != nil:
		cuerpo = lipgloss.Place(ancho, alto, lipgloss.Center, lipgloss.Center, m.pregunta.Vista(e, min(ancho-2, 60), alto))
	case m.selector != nil:
		caja := lipgloss.NewStyle().Border(e.Borde).Padding(0, 1).Render(m.selector.Vista(min(ancho-6, 60), min(alto-2, 16)))
		cuerpo = lipgloss.Place(ancho, alto, lipgloss.Center, lipgloss.Center, caja)
	case m.enMenu && m.ancho < anchoMenuFijo:
		cuerpo = m.vistaMenu()
	default:
		cuerpo = m.pagina(m.actual).Vista()
	}
	cuerpo = m.encajar(cuerpo, ancho, alto)
	if m.ancho >= anchoMenuFijo {
		separador := strings.TrimSuffix(strings.Repeat(e.Tenue.Render(e.Iconos.Vertical)+" \n", alto), "\n")
		cuerpo = lipgloss.JoinHorizontal(lipgloss.Top, m.encajar(m.vistaMenu(), anchoMenu-2, alto), separador, cuerpo)
	}
	return m.encajar(strings.Join([]string{m.vistaCabecera(), cuerpo, m.vistaEstado(), m.vistaPie()}, "\n"), m.ancho, m.alto)
}

// encajar deja un texto en exactamente ancho×alto: corta lo que sobra, con elipsis, y rellena con espacios.
func (m *modelo) encajar(texto string, ancho, alto int) string {
	lineas := strings.Split(texto, "\n")
	lineas = lineas[:min(len(lineas), alto)]
	for len(lineas) < alto {
		lineas = append(lineas, "")
	}
	for i, l := range lineas {
		l = m.estilos.Recortar(l, ancho)
		lineas[i] = l + strings.Repeat(" ", max(ancho-lipgloss.Width(l), 0))
	}
	return strings.Join(lineas, "\n")
}

// proximamente ocupa el sitio de una página que aún no está hecha. En la Ayuda enseña las teclas de la página de la que
// se vino (ConAyuda), para que ? siempre las diga todas.
type proximamente struct {
	titulo  string
	estilos *pagina.Estilos
	teclas  []pagina.Tecla
}

func (p *proximamente) Titulo() string                  { return p.titulo }
func (p *proximamente) Iniciar(pagina.Contexto) tea.Cmd { return nil }
func (p *proximamente) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch msg := msg.(type) {
	case pagina.ConAyuda:
		p.teclas = msg.Teclas
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return p, pagina.Atras
		}
	}
	return p, nil
}
func (p *proximamente) Teclas() []pagina.Tecla {
	return []pagina.Tecla{{Tecla: "Esc", Accion: idioma.T("vuelve")}}
}
func (p *proximamente) EnCampo() bool { return false }
func (p *proximamente) Ayuda() string { return "" }
func (p *proximamente) Vista() string {
	e := p.estilos
	lineas := []string{e.Tenue.Render(idioma.T("Próximamente"))}
	if len(p.teclas) > 0 {
		ancho := 0
		for _, t := range p.teclas {
			ancho = max(ancho, lipgloss.Width(t.Tecla))
		}
		lineas = append(lineas, "", e.Titulo.Render(idioma.T("Teclas")))
		for _, t := range p.teclas {
			lineas = append(lineas, e.Tecla.Render(t.Tecla)+strings.Repeat(" ", ancho-lipgloss.Width(t.Tecla)+2)+t.Accion)
		}
	}
	return strings.Join(lineas, "\n")
}
