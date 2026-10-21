// Package tui es la terminal interactiva: un selector de ámbito y, por instancia, paneles para
// ejecutar, ver trabajos, gestionar puntos de acceso y revisar configuración y agentes, sin escribir comandos.
// Habla con el servicio a través del cliente; puede iniciarlo en segundo plano.
package tui

import (
	"context"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
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
	estado      *contrato.Estado
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
	dejarDeSeguir     context.CancelFunc // corta el flujo de eventos del trabajo en curso
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
		estado *contrato.Estado
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
		parar context.CancelFunc
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
		cli.Origen = nucleo.DesdeTerminal
		ctx, cancelar := conPlazo(10 * time.Second)
		defer cancelar()
		e, err := cli.Estado(ctx)
		return msgEstado{cli: cli, estado: &e, err: err}
	}
}

func (m *modelo) avisar(err error) {
	m.aviso, m.esError = nucleo.MensajeDe(err), true
}

// parar deja de seguir el trabajo en curso (si lo hay): su goroutine termina aunque el servicio no conteste.
func (m *modelo) parar() {
	if m.dejarDeSeguir != nil {
		m.dejarDeSeguir()
		m.dejarDeSeguir = nil
	}
}

// salir cierra la terminal sin dejar flujos abiertos.
func (m *modelo) salir() tea.Cmd {
	m.parar()
	return tea.Quit
}

func (m *modelo) ir(p pantalla) {
	m.pantalla, m.cursor, m.scroll, m.aviso = p, 0, 0, ""
}

func (m *modelo) confirmar(pregunta string, accion tea.Cmd) tea.Cmd {
	m.pregunta, m.siConfirma = pregunta, accion
	return nil
}

func (m *modelo) instancia(id string) *contrato.EstadoInstancia {
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
		m.modelos = append([]string{idioma.T("(predeterminado del espacio)")}, msg.lista...)
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
		m.parar()
		m.enCurso, m.flujo, m.dejarDeSeguir = msg.t.ID, msg.flujo, msg.parar
		m.resumen = idioma.Tf("trabajo %s en cola · Esc cancela", msg.t.ID)
		return m, escuchar(m.flujo)
	case msgEvento:
		switch msg.tipo {
		case "texto":
			m.salida.WriteString(msg.datos)
		case "estado":
			m.resumen = idioma.Tf("trabajo %s: %s · Esc cancela", m.enCurso, msg.datos)
		}
		return m, escuchar(m.flujo)
	case msgFin:
		m.parar()
		m.enCurso = ""
		if msg.err != nil {
			m.avisar(msg.err)
			return m, m.cargarEstado()
		}
		t := msg.t
		if t.Estado == nucleo.Completado && t.Salida != m.salida.String() {
			if m.salida.Len() > 0 {
				m.salida.WriteString(idioma.T("\n\n— salida final —\n"))
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
		return m.salir()
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
			return m.salir()
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
