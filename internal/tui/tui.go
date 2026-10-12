// Package tui es la terminal interactiva (sección 13.2): un selector de ámbito y, por instancia, paneles para
// ejecutar, ver trabajos, gestionar puntos de acceso y revisar configuración y agentes, sin escribir comandos.
// Habla con el servicio a través del cliente; puede iniciarlo en segundo plano.
package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/navegador"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/version"
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
	_, err := tea.NewProgram(nuevo(o)).Run()
	return err
}

const (
	negrita  = "\x1b[1m"
	tenue    = "\x1b[2m"
	cian     = "\x1b[36m"
	verde    = "\x1b[32m"
	rojo     = "\x1b[31m"
	amarillo = "\x1b[33m"
	fin      = "\x1b[0m"
)

func estilo(e, texto string) string { return e + texto + fin }

type pantalla int

const (
	pInicio pantalla = iota
	pInstancia
	pEjecutar
	pTrabajos
	pDetalle
	pAccesos
	pNuevoPunto
	pLineas
	pTodas
	pServicio
)

type opcion struct {
	texto  string
	accion func() tea.Cmd
}

type modelo struct {
	o           Opciones
	cli         *cliente.Cliente
	estado      *servicio.Estado
	pantalla    pantalla
	cursor      int
	inst        string
	aviso       string
	esError     bool
	ocupado     string
	pregunta    string
	siConfirma  tea.Cmd
	ancho, alto int

	modelos           []string
	iModelo           int
	campo             int
	archivos, mensaje string
	salida            strings.Builder
	enCurso           string
	flujo             chan tea.Msg
	resumen           string

	trabajos []nucleo.Trabajo
	detalle  nucleo.Trabajo
	scroll   int

	titulo string
	lineas []string

	nombrePunto, puertoPunto string
}

func nuevo(o Opciones) *modelo { return &modelo{o: o, ancho: 100, alto: 30} }

type (
	msgEstado struct {
		cli    *cliente.Cliente
		estado *servicio.Estado
		err    error
	}
	msgAviso struct {
		texto     string
		err       error
		siguiente tea.Cmd
	}
	msgTrabajos struct {
		lista []nucleo.Trabajo
		err   error
	}
	msgDetalle struct {
		t   nucleo.Trabajo
		err error
	}
	msgModelos struct {
		lista []string
		err   error
	}
	msgLineas struct {
		lineas []string
		err    error
	}
	msgCreado struct {
		t     nucleo.Trabajo
		flujo chan tea.Msg
		err   error
	}
	msgEvento struct{ tipo, datos string }
	msgFin    struct {
		t   nucleo.Trabajo
		err error
	}
)

func conPlazo(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func (m *modelo) Init() tea.Cmd { return m.cargarEstado() }

// cargarEstado reconecta y lee el estado; un servicio inactivo no es un error.
func (m *modelo) cargarEstado() tea.Cmd {
	datos := m.o.Datos
	return func() tea.Msg {
		cli, err := cliente.Conectar(datos)
		if err != nil {
			return msgEstado{}
		}
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		e, err := cli.Estado(ctx)
		return msgEstado{cli: cli, estado: &e, err: err}
	}
}

func (m *modelo) avisar(err error) {
	m.aviso, m.esError = nucleo.MensajeDe(err), true
}

func (m *modelo) ir(p pantalla) {
	m.pantalla, m.cursor, m.scroll, m.aviso = p, 0, 0, ""
}

func (m *modelo) confirmar(pregunta string, accion tea.Cmd) tea.Cmd {
	m.pregunta, m.siConfirma = pregunta, accion
	return nil
}

func (m *modelo) instancia(id string) *servicio.EstadoInstancia {
	if m.estado == nil {
		return nil
	}
	for i := range m.estado.Instancias {
		if m.estado.Instancias[i].ID == id {
			return &m.estado.Instancias[i]
		}
	}
	return nil
}

// Update implementa tea.Model.
func (m *modelo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ancho, m.alto = msg.Width, msg.Height
	case msgEstado:
		m.ocupado = ""
		m.cli, m.estado = msg.cli, msg.estado
		if msg.err != nil {
			m.avisar(msg.err)
		}
	case msgAviso:
		m.ocupado = ""
		if msg.err != nil {
			m.avisar(msg.err)
		} else if msg.texto != "" {
			m.aviso, m.esError = msg.texto, false
		}
		return m, tea.Batch(m.cargarEstado(), msg.siguiente)
	case msgTrabajos:
		m.ocupado = ""
		if msg.err != nil {
			m.avisar(msg.err)
		}
		m.trabajos = msg.lista
		if m.cursor >= len(m.trabajos) {
			m.cursor = max(len(m.trabajos)-1, 0)
		}
	case msgDetalle:
		m.ocupado = ""
		if msg.err != nil {
			m.avisar(msg.err)
			return m, nil
		}
		m.detalle, m.scroll = msg.t, 0
		m.pantalla = pDetalle
	case msgModelos:
		m.modelos = append([]string{"(predeterminado de la instancia)"}, msg.lista...)
		if m.iModelo >= len(m.modelos) {
			m.iModelo = 0
		}
		if msg.err != nil {
			m.avisar(msg.err)
		}
	case msgLineas:
		m.ocupado = ""
		m.lineas = msg.lineas
		if msg.err != nil {
			m.avisar(msg.err)
		}
	case msgCreado:
		m.ocupado = ""
		if msg.err != nil {
			m.avisar(msg.err)
			return m, nil
		}
		m.enCurso, m.flujo = msg.t.ID, msg.flujo
		m.resumen = "trabajo " + msg.t.ID + " en cola · Esc cancela"
		return m, escuchar(m.flujo)
	case msgEvento:
		switch msg.tipo {
		case "texto":
			m.salida.WriteString(msg.datos)
		case "estado":
			m.resumen = "trabajo " + m.enCurso + ": " + msg.datos + " · Esc cancela"
		}
		return m, escuchar(m.flujo)
	case msgFin:
		m.enCurso = ""
		if msg.err != nil {
			m.avisar(msg.err)
			return m, m.cargarEstado()
		}
		t := msg.t
		if t.Estado == nucleo.Completado && t.Salida != m.salida.String() {
			if m.salida.Len() > 0 {
				m.salida.WriteString("\n\n— salida final —\n")
			}
			m.salida.WriteString(t.Salida)
		}
		m.resumen = resumenTrabajo(t)
		return m, m.cargarEstado()
	case tea.PasteMsg:
		m.escribir(msg.Content)
	case tea.KeyPressMsg:
		return m, m.tecla(msg)
	}
	return m, nil
}

func escuchar(flujo chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-flujo
		if !ok {
			return nil
		}
		return msg
	}
}

func resumenTrabajo(t nucleo.Trabajo) string {
	modelo := t.ModeloEfectivo
	if modelo == "" {
		modelo = "no verificable"
	}
	texto := fmt.Sprintf("%s · modelo %s · tokens %d/%d", t.Estado, modelo, t.Uso.TokensEntrada, t.Uso.TokensSalida)
	if t.Uso.CostoUSD > 0 {
		texto += fmt.Sprintf(" · US$ %.4f", t.Uso.CostoUSD)
	}
	if t.CodigoError != "" {
		texto += " · " + string(t.CodigoError) + ": " + t.MensajeError
	}
	return texto
}

// escribir añade texto al campo editable de la pantalla actual.
func (m *modelo) escribir(texto string) {
	texto = strings.ReplaceAll(texto, "\r", "")
	switch {
	case m.pantalla == pEjecutar && m.campo == 1:
		m.archivos += strings.ReplaceAll(texto, "\n", " ")
	case m.pantalla == pEjecutar && m.campo == 2:
		m.mensaje += texto
	case m.pantalla == pNuevoPunto && m.campo == 0:
		m.nombrePunto += strings.ToLower(strings.TrimSpace(texto))
	case m.pantalla == pNuevoPunto && m.campo == 1:
		m.puertoPunto += strings.TrimSpace(texto)
	}
}

func borrarUltimo(texto string) string {
	r := []rune(texto)
	if len(r) == 0 {
		return texto
	}
	return string(r[:len(r)-1])
}

func (m *modelo) tecla(k tea.KeyPressMsg) tea.Cmd {
	tecla := k.String()
	if tecla == "ctrl+c" {
		return tea.Quit
	}
	if m.pregunta != "" {
		switch tecla {
		case "s", "y", "enter":
			accion := m.siConfirma
			m.pregunta, m.siConfirma = "", nil
			return accion
		case "n", "esc":
			m.pregunta, m.siConfirma = "", nil
		}
		return nil
	}
	switch m.pantalla {
	case pEjecutar:
		return m.teclaEjecutar(k)
	case pNuevoPunto:
		return m.teclaNuevoPunto(k)
	}
	if tecla == "esc" {
		return m.volver()
	}
	if tecla == "r" && m.pantalla != pDetalle {
		return m.refrescar()
	}
	switch m.pantalla {
	case pInicio:
		if tecla == "q" {
			return tea.Quit
		}
		return m.elegir(tecla, m.opcionesInicio())
	case pInstancia:
		return m.elegir(tecla, m.opcionesInstancia())
	case pTrabajos:
		return m.teclaTrabajos(tecla)
	case pDetalle:
		return m.teclaDetalle(tecla)
	case pAccesos:
		return m.teclaAccesos(tecla)
	case pTodas:
		return m.teclaTodas(tecla)
	case pServicio:
		return m.teclaServicio(tecla)
	case pLineas:
		m.desplazar(tecla, len(m.lineas))
	}
	return nil
}

func (m *modelo) volver() tea.Cmd {
	switch m.pantalla {
	case pDetalle:
		m.ir(pTrabajos)
		return m.cargarTrabajos()
	case pInstancia, pTodas, pServicio:
		m.ir(pInicio)
	case pNuevoPunto:
		m.ir(pAccesos)
	default:
		m.ir(pInstancia)
	}
	return m.cargarEstado()
}

func (m *modelo) refrescar() tea.Cmd {
	switch m.pantalla {
	case pTrabajos:
		return m.cargarTrabajos()
	}
	return m.cargarEstado()
}

func (m *modelo) mover(tecla string, n int) bool {
	switch tecla {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return true
	case "down", "j":
		if m.cursor < n-1 {
			m.cursor++
		}
		return true
	}
	return false
}

func (m *modelo) desplazar(tecla string, n int) {
	paso := map[string]int{"up": -1, "k": -1, "down": 1, "j": 1, "pgup": -10, "pgdown": 10}[tecla]
	m.scroll = min(max(m.scroll+paso, 0), max(n-1, 0))
}

func (m *modelo) elegir(tecla string, ops []opcion) tea.Cmd {
	if m.mover(tecla, len(ops)) {
		return nil
	}
	if n, err := strconv.Atoi(tecla); err == nil && n >= 1 && n <= len(ops) {
		m.cursor = n - 1
		return ops[n-1].accion()
	}
	if (tecla == "enter" || tecla == "space") && m.cursor < len(ops) {
		return ops[m.cursor].accion()
	}
	return nil
}

func (m *modelo) opcionesInicio() []opcion {
	var ops []opcion
	if m.o.Proyecto != "" {
		ops = append(ops, opcion{"Instancia del proyecto actual", func() tea.Cmd { m.inst = m.o.IDProyecto; m.ir(pInstancia); return nil }})
	}
	return append(ops,
		opcion{"Instancia global", func() tea.Cmd { m.inst = "global"; m.ir(pInstancia); return nil }},
		opcion{"Ver todas las instancias", func() tea.Cmd { m.ir(pTodas); return m.cargarEstado() }},
		opcion{"Estado del servicio", func() tea.Cmd { m.ir(pServicio); return m.cargarEstado() }},
		opcion{"Salir", func() tea.Cmd { return tea.Quit }},
	)
}

func (m *modelo) opcionesInstancia() []opcion {
	volver := opcion{"Volver", func() tea.Cmd { m.ir(pInicio); return nil }}
	if m.instancia(m.inst) == nil {
		texto := "Activar instancia"
		if m.cli == nil {
			texto += " (inicia el servicio en segundo plano)"
		}
		return []opcion{{texto, m.activar}, volver}
	}
	return []opcion{
		{"Ejecutar", func() tea.Cmd {
			m.ir(pEjecutar)
			m.campo, m.salida, m.resumen = 2, strings.Builder{}, ""
			return m.cargarModelos()
		}},
		{"Trabajos", func() tea.Cmd { m.ir(pTrabajos); return m.cargarTrabajos() }},
		{"Puntos de acceso", func() tea.Cmd { m.ir(pAccesos); return m.cargarEstado() }},
		{"Configuración efectiva", func() tea.Cmd { m.ir(pLineas); m.titulo = "Configuración efectiva"; return m.cargarConfiguracion() }},
		{"Agentes y capacidades", func() tea.Cmd { m.ir(pLineas); m.titulo = "Agentes y capacidades"; return m.cargarAgentes() }},
		{"Abrir Web UI en el navegador", m.abrirWeb},
		{"Desactivar instancia", func() tea.Cmd {
			return m.confirmar("¿Desactivar "+m.inst+"? Se cancela lo que esté en cola.", m.desactivar(m.inst))
		}},
		volver,
	}
}

// activar activa la instancia elegida e inicia el servicio si no está en marcha.
func (m *modelo) activar() tea.Cmd {
	ambito, raiz := "global", ""
	if m.inst != "global" {
		ambito, raiz = "proyecto", m.o.Proyecto
	}
	cli, iniciar := m.cli, m.o.Iniciar
	m.ocupado = "activando " + m.inst + "…"
	return func() tea.Msg {
		var err error
		if cli == nil {
			if cli, err = iniciar(); err != nil {
				return msgAviso{err: err}
			}
		}
		ctx, cancelar := conPlazo(60 * time.Second)
		defer cancelar()
		_, err = cli.Activar(ctx, ambito, raiz)
		return msgAviso{texto: "instancia activa", err: err}
	}
}

// abrirWeb abre la Web UI de la instancia con un código de un solo uso, sin mostrar el token.
func (m *modelo) abrirWeb() tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		url, err := cli.URLWeb(ctx, inst)
		if err == nil {
			err = navegador.Abrir(url)
		}
		return msgAviso{texto: "Web UI abierta en el navegador", err: err}
	}
}

func (m *modelo) desactivar(id string) tea.Cmd {
	cli := m.cli
	return func() tea.Msg {
		ctx, cancelar := conPlazo(60 * time.Second)
		defer cancelar()
		return msgAviso{texto: "instancia " + id + " desactivada", err: cli.Desactivar(ctx, id)}
	}
}

func (m *modelo) cargarModelos() tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		lista, err := cli.Modelos(ctx, inst)
		return msgModelos{lista: lista, err: err}
	}
}

func (m *modelo) cargarTrabajos() tea.Cmd {
	cli, inst := m.cli, m.inst
	m.ocupado = "cargando trabajos…"
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		lista, err := cli.Trabajos(ctx, inst, 50)
		return msgTrabajos{lista: lista, err: err}
	}
}

func (m *modelo) cargarDetalle(id string) tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		t, err := cli.Trabajo(ctx, inst, id)
		return msgDetalle{t: t, err: err}
	}
}

func (m *modelo) cargarConfiguracion() tea.Cmd {
	cli, inst := m.cli, m.inst
	m.ocupado, m.lineas = "cargando…", nil
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		entradas, err := cli.Configuracion(ctx, inst)
		var lineas []string
		for _, e := range entradas {
			lineas = append(lineas, fmt.Sprintf("%-46s %-18v %s", e.Clave, e.Valor, estilo(tenue, e.Origen)))
		}
		return msgLineas{lineas: lineas, err: err}
	}
}

func (m *modelo) cargarAgentes() tea.Cmd {
	cli, inst := m.cli, m.inst
	m.ocupado, m.lineas = "detectando agentes…", nil
	return func() tea.Msg {
		ctx, cancelar := conPlazo(30 * time.Second)
		defer cancelar()
		c, err := cli.Capacidades(ctx, inst)
		var lineas []string
		lista, _ := c["adaptadores"].([]any)
		for _, a := range lista {
			datos, _ := a.(map[string]any)
			capac, _ := datos["capacidades"].(map[string]any)
			lineas = append(lineas, estilo(negrita, fmt.Sprint(datos["agente"]))+fmt.Sprintf("  versión %v · autenticación %v", datos["version_detectada"], datos["autenticacion"]),
				fmt.Sprintf("  streaming %v · salida estructurada %v · modelo efectivo %v", capac["streaming"], capac["salida_estructurada"], capac["modelo_efectivo_informado"]),
				fmt.Sprintf("  términos: %v", datos["terminos"]))
			if p, ok := datos["problema"]; ok {
				lineas = append(lineas, estilo(amarillo, fmt.Sprintf("  problema: %v", p)))
			}
			lineas = append(lineas, "")
		}
		lineas = append(lineas, fmt.Sprintf("formato de conversación: %v", c["formato_serializacion"]))
		return msgLineas{lineas: lineas, err: err}
	}
}

// solicitudDesde traduce la selección de modelo del catálogo a la solicitud.
func solicitudDesde(seleccion, texto string) nucleo.SolicitudEjecucion {
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
	switch {
	case seleccion == "" || strings.HasPrefix(seleccion, "("):
	case strings.HasPrefix(seleccion, "perfil/"):
		s.PerfilModelo = strings.TrimPrefix(seleccion, "perfil/")
	default:
		s.Agente, s.Modelo, _ = strings.Cut(seleccion, "/")
	}
	return s
}

func (m *modelo) teclaEjecutar(k tea.KeyPressMsg) tea.Cmd {
	switch tecla := k.String(); tecla {
	case "esc":
		if m.enCurso != "" {
			return m.accionTrabajo(m.enCurso, "cancelar", nil)
		}
		return m.volver()
	case "tab", "down":
		m.campo = (m.campo + 1) % 3
	case "shift+tab", "up":
		m.campo = (m.campo + 2) % 3
	case "left", "right":
		if m.campo == 0 && len(m.modelos) > 0 {
			paso := map[string]int{"left": -1, "right": 1}[tecla]
			m.iModelo = (m.iModelo + paso + len(m.modelos)) % len(m.modelos)
		}
	case "enter":
		if m.campo < 2 {
			m.campo++
			return nil
		}
		return m.lanzar()
	case "backspace":
		if m.campo == 1 {
			m.archivos = borrarUltimo(m.archivos)
		} else if m.campo == 2 {
			m.mensaje = borrarUltimo(m.mensaje)
		}
	case "ctrl+u":
		if m.campo == 1 {
			m.archivos = ""
		} else if m.campo == 2 {
			m.mensaje = ""
		}
	default:
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			m.escribir(k.Text)
		}
	}
	return nil
}

// lanzar sube los archivos, crea el trabajo y sigue sus eventos en vivo.
func (m *modelo) lanzar() tea.Cmd {
	if m.enCurso != "" {
		return nil
	}
	if strings.TrimSpace(m.mensaje) == "" {
		m.aviso, m.esError = "escribe un mensaje", true
		return nil
	}
	seleccion := ""
	if len(m.modelos) > 0 {
		seleccion = m.modelos[m.iModelo]
	}
	sol := solicitudDesde(seleccion, m.mensaje)
	archivos := strings.FieldsFunc(m.archivos, func(r rune) bool { return r == ',' || r == ';' })
	cli, inst := m.cli, m.inst
	m.salida.Reset()
	m.resumen, m.aviso, m.ocupado = "", "", "enviando…"
	return func() tea.Msg {
		ctx, cancelar := conPlazo(10 * time.Minute)
		defer cancelar()
		for _, ruta := range archivos {
			carga, err := cli.SubirCarga(ctx, inst, strings.Trim(strings.TrimSpace(ruta), `"`))
			if err != nil {
				return msgCreado{err: err}
			}
			sol.Cargas = append(sol.Cargas, carga.ID)
		}
		t, err := cli.Crear(ctx, inst, sol)
		if err != nil {
			return msgCreado{err: err}
		}
		flujo := make(chan tea.Msg, 256)
		go func() {
			defer close(flujo)
			err := cli.Eventos(context.Background(), inst, t.ID, 0, func(_ int, tipo, datos string) error {
				flujo <- msgEvento{tipo: tipo, datos: datos}
				return nil
			})
			ctx, cancelar := conPlazo(10 * time.Second)
			defer cancelar()
			final, err2 := cli.Trabajo(ctx, inst, t.ID)
			if err == nil {
				err = err2
			}
			flujo <- msgFin{t: final, err: err}
		}()
		return msgCreado{t: t, flujo: flujo}
	}
}

func (m *modelo) accionTrabajo(id, accion string, siguiente tea.Cmd) tea.Cmd {
	cli, inst := m.cli, m.inst
	return func() tea.Msg {
		ctx, cancelar := conPlazo(30 * time.Second)
		defer cancelar()
		var err error
		if accion == "eliminar" {
			err = cli.Eliminar(ctx, inst, id)
		} else {
			_, err = cli.Accion(ctx, inst, id, accion)
		}
		return msgAviso{texto: accion + ": " + id, err: err, siguiente: siguiente}
	}
}

func (m *modelo) teclaTrabajos(tecla string) tea.Cmd {
	if m.mover(tecla, len(m.trabajos)) || len(m.trabajos) == 0 {
		return nil
	}
	t := m.trabajos[m.cursor]
	switch tecla {
	case "enter", "space":
		return m.cargarDetalle(t.ID)
	case "c":
		return m.accionTrabajo(t.ID, "cancelar", m.cargarTrabajos())
	case "t":
		return m.accionTrabajo(t.ID, "reintentar", m.cargarTrabajos())
	case "x":
		return m.confirmar("¿Eliminar "+t.ID+"?", m.accionTrabajo(t.ID, "eliminar", m.cargarTrabajos()))
	}
	return nil
}

func (m *modelo) teclaDetalle(tecla string) tea.Cmd {
	id := m.detalle.ID
	switch tecla {
	case "c":
		return m.accionTrabajo(id, "cancelar", m.cargarDetalle(id))
	case "t":
		return m.accionTrabajo(id, "reintentar", nil)
	case "x":
		return m.confirmar("¿Eliminar "+id+"?", m.accionTrabajo(id, "eliminar", func() tea.Msg { return msgTrabajos{} }))
	case "r":
		return m.cargarDetalle(id)
	}
	m.desplazar(tecla, len(strings.Split(m.detalle.Salida, "\n")))
	return nil
}

func (m *modelo) puntosDeInstancia() []servicio.EstadoPunto {
	var lista []servicio.EstadoPunto
	if m.estado != nil {
		for _, p := range m.estado.Puntos {
			if p.Instancia == m.inst {
				lista = append(lista, p)
			}
		}
	}
	return lista
}

// rutaConfig devuelve el archivo donde se guardan los puntos de esta instancia y su origen.
func (m *modelo) rutaConfig(archivo string) (string, nucleo.Origen) {
	if strings.HasSuffix(archivo, "local.yaml") {
		return archivo, nucleo.OrigenLocal
	}
	if archivo != "" {
		return archivo, nucleo.OrigenGlobal
	}
	if m.inst != "global" && m.o.Proyecto != "" {
		return m.o.Proyecto + "/.tramtai/local.yaml", nucleo.OrigenLocal
	}
	ruta, _ := nucleo.RutaGlobal()
	return ruta, nucleo.OrigenGlobal
}

func (m *modelo) editarPuntos(archivo, texto string, f func([]servicio.ConfigPunto) ([]servicio.ConfigPunto, error)) tea.Cmd {
	cli := m.cli
	ruta, origen := m.rutaConfig(archivo)
	return func() tea.Msg {
		if err := servicio.EditarPuntos(ruta, origen, f); err != nil {
			return msgAviso{err: err}
		}
		ctx, cancelar := conPlazo(30 * time.Second)
		defer cancelar()
		_, err := cli.Recargar(ctx)
		return msgAviso{texto: texto, err: err}
	}
}

func (m *modelo) teclaAccesos(tecla string) tea.Cmd {
	puntos := m.puntosDeInstancia()
	if tecla == "n" {
		m.ir(pNuevoPunto)
		m.campo, m.nombrePunto, m.puertoPunto = 0, "", ""
		return m.cargarModelos()
	}
	if m.mover(tecla, len(puntos)) || len(puntos) == 0 {
		return nil
	}
	p := puntos[m.cursor]
	cambiar := func(f func(*servicio.ConfigPunto)) func([]servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
		return func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
			for i := range lista {
				if lista[i].Nombre == p.Nombre {
					f(&lista[i])
					return lista, nil
				}
			}
			return nil, fmt.Errorf("el punto %s no está en %s", p.Nombre, p.Origen)
		}
	}
	switch tecla {
	case "enter", "space":
		activo := !(p.Activo || p.Error != "desactivado")
		return m.editarPuntos(p.Origen, map[bool]string{true: "punto activado", false: "punto desactivado"}[activo],
			cambiar(func(c *servicio.ConfigPunto) { c.Activo = &activo }))
	case "x":
		return m.confirmar("¿Eliminar el punto de acceso "+p.Nombre+"?", m.editarPuntos(p.Origen, "punto eliminado",
			func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
				var resto []servicio.ConfigPunto
				for _, c := range lista {
					if c.Nombre != p.Nombre {
						resto = append(resto, c)
					}
				}
				return resto, nil
			}))
	case "t":
		m.aviso, m.esError = "token de "+p.Nombre+": "+servicio.RutaTokenPunto(m.o.Datos, p.Nombre), false
	}
	return nil
}

func (m *modelo) teclaNuevoPunto(k tea.KeyPressMsg) tea.Cmd {
	switch tecla := k.String(); tecla {
	case "esc":
		return m.volver()
	case "tab", "down":
		m.campo = (m.campo + 1) % 3
	case "shift+tab", "up":
		m.campo = (m.campo + 2) % 3
	case "left", "right":
		if m.campo == 2 && len(m.modelos) > 0 {
			paso := map[string]int{"left": -1, "right": 1}[tecla]
			m.iModelo = (m.iModelo + paso + len(m.modelos)) % len(m.modelos)
		}
	case "backspace":
		if m.campo == 0 {
			m.nombrePunto = borrarUltimo(m.nombrePunto)
		} else if m.campo == 1 {
			m.puertoPunto = borrarUltimo(m.puertoPunto)
		}
	case "enter":
		if m.campo < 2 {
			m.campo++
			return nil
		}
		return m.crearPunto()
	default:
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			m.escribir(k.Text)
		}
	}
	return nil
}

func (m *modelo) crearPunto() tea.Cmd {
	p := servicio.ConfigPunto{Nombre: m.nombrePunto}
	if m.puertoPunto != "" {
		puerto, err := strconv.Atoi(m.puertoPunto)
		if err != nil {
			m.aviso, m.esError = "el puerto debe ser un número", true
			return nil
		}
		p.Puerto = puerto
	}
	if len(m.modelos) > 0 {
		s := solicitudDesde(m.modelos[m.iModelo], "")
		p.Agente, p.Modelo, p.PerfilModelo = s.Agente, s.Modelo, s.PerfilModelo
	}
	ruta, _ := m.rutaConfig("")
	if m.inst == "global" || !strings.HasSuffix(ruta, "local.yaml") {
		if m.inst != "global" {
			p.Instancia = m.inst
		}
	}
	if _, err := p.Normalizar(m.inst, 0); err != nil {
		m.avisar(err)
		return nil
	}
	m.ir(pAccesos)
	return m.editarPuntos("", "punto "+p.Nombre+" creado · token en "+servicio.RutaTokenPunto(m.o.Datos, p.Nombre),
		func(lista []servicio.ConfigPunto) ([]servicio.ConfigPunto, error) {
			for _, c := range lista {
				if c.Nombre == p.Nombre {
					return nil, fmt.Errorf("el punto de acceso %s ya existe", p.Nombre)
				}
			}
			return append(lista, p), nil
		})
}

func (m *modelo) teclaTodas(tecla string) tea.Cmd {
	var lista []servicio.EstadoInstancia
	if m.estado != nil {
		lista = m.estado.Instancias
	}
	if m.mover(tecla, len(lista)) || len(lista) == 0 {
		return nil
	}
	i := lista[m.cursor]
	switch tecla {
	case "enter", "space":
		m.inst = i.ID
		m.ir(pInstancia)
	case "d":
		return m.confirmar("¿Desactivar "+i.ID+"? Se cancela lo que esté en cola.", m.desactivar(i.ID))
	}
	return nil
}

func (m *modelo) teclaServicio(tecla string) tea.Cmd {
	switch tecla {
	case "i":
		if m.cli == nil {
			iniciar := m.o.Iniciar
			m.ocupado = "iniciando el servicio…"
			return func() tea.Msg {
				_, err := iniciar()
				return msgAviso{texto: "servicio iniciado", err: err}
			}
		}
	case "d":
		if m.cli != nil {
			cli := m.cli
			return m.confirmar("¿Detener el servicio y todas sus instancias?", func() tea.Msg {
				ctx, cancelar := conPlazo(30 * time.Second)
				defer cancelar()
				err := cli.Detener(ctx)
				time.Sleep(time.Second)
				return msgAviso{texto: "servicio detenido", err: err}
			})
		}
	case "p":
		if m.cli != nil {
			cli := m.cli
			return func() tea.Msg {
				ctx, cancelar := conPlazo(30 * time.Second)
				defer cancelar()
				_, err := cli.Recargar(ctx)
				return msgAviso{texto: "puntos de acceso recargados", err: err}
			}
		}
	}
	return nil
}

// View implementa tea.Model.
func (m *modelo) View() tea.View {
	var b strings.Builder
	b.WriteString(estilo(negrita+cian, "Tramtai") + " " + estilo(tenue, version.Texto()) + "\n\n")
	ayuda := ""
	switch m.pantalla {
	case pInicio:
		ayuda = m.vistaInicio(&b)
	case pInstancia:
		ayuda = m.vistaInstancia(&b)
	case pEjecutar:
		ayuda = m.vistaEjecutar(&b)
	case pTrabajos:
		ayuda = m.vistaTrabajos(&b)
	case pDetalle:
		ayuda = m.vistaDetalle(&b)
	case pAccesos:
		ayuda = m.vistaAccesos(&b)
	case pNuevoPunto:
		ayuda = m.vistaNuevoPunto(&b)
	case pLineas:
		b.WriteString(estilo(negrita, m.titulo+" · "+m.inst) + "\n\n")
		for _, l := range ventana(m.lineas, m.scroll, m.alto-8) {
			b.WriteString(l + "\n")
		}
		ayuda = "↑↓ desplazar · Esc volver"
	case pTodas:
		ayuda = m.vistaTodas(&b)
	case pServicio:
		ayuda = m.vistaServicio(&b)
	}
	b.WriteString("\n")
	switch {
	case m.pregunta != "":
		b.WriteString(estilo(amarillo+negrita, m.pregunta+" (s/n)"))
	case m.ocupado != "":
		b.WriteString(estilo(amarillo, m.ocupado))
	case m.aviso != "" && m.esError:
		b.WriteString(estilo(rojo, m.aviso))
	case m.aviso != "":
		b.WriteString(estilo(verde, m.aviso))
	}
	b.WriteString("\n" + estilo(tenue, ayuda+" · Ctrl+C salir"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func ventana(lineas []string, desde, alto int) []string {
	alto = max(alto, 5)
	desde = min(desde, max(len(lineas)-1, 0))
	hasta := min(desde+alto, len(lineas))
	return lineas[desde:hasta]
}

func (m *modelo) menu(b *strings.Builder, ops []opcion) {
	for i, op := range ops {
		if i == m.cursor {
			fmt.Fprintf(b, "%s %d. %s\n", estilo(cian, "›"), i+1, estilo(negrita, op.texto))
		} else {
			fmt.Fprintf(b, "  %d. %s\n", i+1, op.texto)
		}
	}
}

func marcaActiva(activa bool) string {
	if activa {
		return estilo(verde, "activa")
	}
	return estilo(tenue, "inactiva")
}

func (m *modelo) vistaInicio(b *strings.Builder) string {
	b.WriteString("Seleccione el ámbito de administración:\n\n")
	m.menu(b, m.opcionesInicio())
	b.WriteString("\n")
	if m.o.Proyecto != "" {
		fmt.Fprintf(b, "Proyecto actual: %s\n", m.o.Proyecto)
	} else {
		b.WriteString(estilo(tenue, "Proyecto actual: ninguno (tramtai iniciar en un repositorio)") + "\n")
	}
	if m.cli != nil {
		fmt.Fprintf(b, "Servicio: %s en %s\n", estilo(verde, "activo"), m.cli.Base)
	} else {
		fmt.Fprintf(b, "Servicio: %s\n", estilo(tenue, "inactivo"))
	}
	if m.o.Proyecto != "" {
		fmt.Fprintf(b, "Instancia proyecto: %s\n", marcaActiva(m.instancia(m.o.IDProyecto) != nil))
	}
	fmt.Fprintf(b, "Instancia global: %s\n", marcaActiva(m.instancia("global") != nil))
	return "↑↓ o número · Enter elegir · r actualizar · q salir"
}

func (m *modelo) vistaInstancia(b *strings.Builder) string {
	inst := m.instancia(m.inst)
	fmt.Fprintf(b, "%s  %s\n", estilo(negrita, "Instancia "+m.inst), marcaActiva(inst != nil))
	if inst != nil {
		if inst.Raiz != "" {
			fmt.Fprintf(b, "Raíz: %s\n", inst.Raiz)
		}
		base := "—"
		if m.cli != nil {
			base = m.cli.Base
		}
		fmt.Fprintf(b, "Endpoint: %s  (OpenAI %s/openai/v1 · Anthropic %s/anthropic)\n", base, base, base)
		fmt.Fprintf(b, "Token: %s\n", inst.ArchivoToken)
		fmt.Fprintf(b, "Agente predeterminado: %s · perfil: %s\n", valor(inst.Agente), valor(inst.Perfil))
		fmt.Fprintf(b, "Trabajos: %s\n", conteo(inst.Trabajos))
	}
	b.WriteString("\n")
	m.menu(b, m.opcionesInstancia())
	return "↑↓ o número · Enter elegir · Esc volver"
}

func valor(texto string) string {
	if texto == "" {
		return "—"
	}
	return texto
}

func conteo(c map[string]int) string {
	if len(c) == 0 {
		return "ninguno"
	}
	var partes []string
	for _, e := range []string{"en_cola", "ejecutando", "completado", "fallido", "cancelado", "interrumpido"} {
		if n := c[e]; n > 0 {
			partes = append(partes, fmt.Sprintf("%s %d", e, n))
		}
	}
	return strings.Join(partes, " · ")
}

func (m *modelo) campoTexto(indice int, etiqueta, texto string) string {
	if m.campo == indice {
		return estilo(cian, "› ") + estilo(negrita, etiqueta) + " " + texto + estilo(cian, "▏")
	}
	return "  " + etiqueta + " " + texto
}

func (m *modelo) vistaEjecutar(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, "Ejecutar en "+m.inst))
	seleccion := "…"
	if len(m.modelos) > 0 {
		seleccion = "◀ " + m.modelos[m.iModelo] + " ▶"
	}
	b.WriteString(m.campoTexto(0, "Modelo:  ", seleccion) + "\n")
	b.WriteString(m.campoTexto(1, "Archivos:", m.archivos+estilo(tenue, map[bool]string{true: " (rutas separadas por comas)", false: ""}[m.archivos == ""])) + "\n")
	b.WriteString(m.campoTexto(2, "Mensaje: ", m.mensaje) + "\n")
	b.WriteString(estilo(tenue, strings.Repeat("─", min(m.ancho, 100))) + "\n")
	lineas := strings.Split(m.salida.String(), "\n")
	for _, l := range ventana(lineas, max(len(lineas)-(m.alto-14), 0), m.alto-14) {
		b.WriteString(l + "\n")
	}
	if m.resumen != "" {
		b.WriteString(estilo(tenue, strings.Repeat("─", min(m.ancho, 100))) + "\n" + estilo(cian, m.resumen) + "\n")
	}
	return "Tab cambia de campo · ←→ modelo · Enter ejecuta · Ctrl+U borra el campo · Esc " + map[bool]string{true: "cancela el trabajo", false: "volver"}[m.enCurso != ""]
}

func (m *modelo) vistaTrabajos(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, "Trabajos de "+m.inst))
	if len(m.trabajos) == 0 {
		b.WriteString(estilo(tenue, "sin trabajos") + "\n")
	}
	desde := max(m.cursor-(m.alto-10)+1, 0)
	for i, t := range m.trabajos {
		if i < desde || i >= desde+max(m.alto-10, 5) {
			continue
		}
		modelo := t.ModeloEfectivo
		if modelo == "" {
			modelo = t.ModeloEnviado
		}
		linea := fmt.Sprintf("%-13s %-32s %-8s %-26s %s", colorEstado(t.Estado), t.ID, t.Solicitud.Agente, valor(modelo), t.CreadoEn.Local().Format("02-01 15:04"))
		if i == m.cursor {
			b.WriteString(estilo(cian, "› ") + linea + "\n")
		} else {
			b.WriteString("  " + linea + "\n")
		}
	}
	return "↑↓ · Enter detalle · c cancelar · t reintentar · x eliminar · r actualizar · Esc volver"
}

func colorEstado(e nucleo.Estado) string {
	texto := fmt.Sprintf("%-13s", e)
	switch e {
	case nucleo.Completado:
		return estilo(verde, texto)
	case nucleo.Fallido, nucleo.Interrumpido:
		return estilo(rojo, texto)
	case nucleo.Cancelado:
		return estilo(tenue, texto)
	}
	return estilo(amarillo, texto)
}

func (m *modelo) vistaDetalle(b *strings.Builder) string {
	t := m.detalle
	fmt.Fprintf(b, "%s  %s\n", estilo(negrita, t.ID), colorEstado(t.Estado))
	fmt.Fprintf(b, "Agente: %s · modelo enviado %s · efectivo %s\n", valor(t.Solicitud.Agente), valor(t.ModeloEnviado), valor(t.ModeloEfectivo))
	if t.PuntoAcceso != "" {
		fmt.Fprintf(b, "Punto de acceso: %s\n", t.PuntoAcceso)
	}
	fmt.Fprintf(b, "%s\n", resumenTrabajo(t))
	fmt.Fprintf(b, "Creado: %s\n", t.CreadoEn.Local().Format("02-01-2006 15:04:05"))
	if t.Disponibilidad == nucleo.SalidaExpirada {
		b.WriteString(estilo(tenue, "Resultado expirado por la retención (retencion.horas_resultado)") + "\n")
	}
	b.WriteString(estilo(tenue, strings.Repeat("─", min(m.ancho, 100))) + "\n")
	for _, l := range ventana(strings.Split(t.Salida, "\n"), m.scroll, m.alto-12) {
		b.WriteString(l + "\n")
	}
	return "↑↓ desplazar · c cancelar · t reintentar · x eliminar · r actualizar · Esc volver"
}

func (m *modelo) vistaAccesos(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, "Puntos de acceso de "+m.inst))
	puntos := m.puntosDeInstancia()
	if len(puntos) == 0 {
		b.WriteString(estilo(tenue, "ninguno; pulsa n para crear uno") + "\n")
	}
	for i, p := range puntos {
		estado := estilo(verde, "● activo")
		if p.Error != "" {
			estado = estilo(rojo, "○ "+p.Error)
		}
		modelo := strings.Trim(p.Agente+"/"+p.Modelo, "/")
		linea := fmt.Sprintf("%-18s %-40s %-22s %-20s %s", p.Nombre, valor(p.URL), "["+strings.Join(p.Contratos, ",")+"]", valor(modelo), estado)
		if i == m.cursor {
			b.WriteString(estilo(cian, "› ") + linea + "\n")
		} else {
			b.WriteString("  " + linea + "\n")
		}
	}
	b.WriteString("\n" + estilo(tenue, "Alias, modelos permitidos y forzado: tramtai acceso editar <nombre> --alias … --permitidos … --forzar") + "\n")
	return "↑↓ · Enter activar/desactivar · n nuevo · x eliminar · t ruta del token · Esc volver"
}

func (m *modelo) vistaNuevoPunto(b *strings.Builder) string {
	fmt.Fprintf(b, "%s\n\n", estilo(negrita, "Nuevo punto de acceso en "+m.inst))
	b.WriteString(m.campoTexto(0, "Nombre:", m.nombrePunto) + "\n")
	b.WriteString(m.campoTexto(1, "Puerto:", m.puertoPunto+estilo(tenue, map[bool]string{true: " (vacío = /p/<nombre> en el puerto del servicio)", false: ""}[m.puertoPunto == ""])) + "\n")
	seleccion := "…"
	if len(m.modelos) > 0 {
		seleccion = "◀ " + m.modelos[m.iModelo] + " ▶"
	}
	b.WriteString(m.campoTexto(2, "Modelo:", seleccion) + "\n")
	return "Tab cambia de campo · ←→ modelo · Enter crea · Esc volver"
}

func (m *modelo) vistaTodas(b *strings.Builder) string {
	b.WriteString(estilo(negrita, "Todas las instancias") + "\n\n")
	if m.estado == nil || len(m.estado.Instancias) == 0 {
		b.WriteString(estilo(tenue, "ninguna activa") + "\n")
		return "Esc volver"
	}
	for i, inst := range m.estado.Instancias {
		linea := fmt.Sprintf("%-28s %-9s %s", inst.ID, inst.Ambito, conteo(inst.Trabajos))
		if inst.Raiz != "" {
			linea += estilo(tenue, "  "+inst.Raiz)
		}
		if i == m.cursor {
			b.WriteString(estilo(cian, "› ") + linea + "\n")
		} else {
			b.WriteString("  " + linea + "\n")
		}
	}
	return "↑↓ · Enter abrir · d desactivar · r actualizar · Esc volver"
}

func (m *modelo) vistaServicio(b *strings.Builder) string {
	b.WriteString(estilo(negrita, "Estado del servicio") + "\n\n")
	if m.cli == nil || m.estado == nil {
		b.WriteString("Servicio: " + estilo(tenue, "inactivo") + "\n")
		return "i iniciar en segundo plano · Esc volver"
	}
	e := m.estado
	fmt.Fprintf(b, "Servicio: %s en %s\nVersión: %s · pid %d · desde %s\n", estilo(verde, "activo"), e.Base, e.Version, e.PID, e.IniciadoEn.Local().Format("02-01-2006 15:04"))
	fmt.Fprintf(b, "Instancias activas: %d\n\nPuntos de acceso:\n", len(e.Instancias))
	if len(e.Puntos) == 0 {
		b.WriteString(estilo(tenue, "  ninguno") + "\n")
	}
	for _, p := range e.Puntos {
		estado := estilo(verde, "activo")
		if p.Error != "" {
			estado = estilo(rojo, p.Error)
		}
		fmt.Fprintf(b, "  %-18s %-40s %-24s %s\n", p.Nombre, valor(p.URL), p.Instancia, estado)
	}
	for _, err := range e.Errores {
		b.WriteString(estilo(amarillo, "configuración: "+err) + "\n")
	}
	return "d detener (pide confirmación) · p recargar puntos · r actualizar · Esc volver"
}
