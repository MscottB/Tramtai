package tui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

const (
	latido = 10 * time.Second // cada cuánto se comprueba el servicio y se releen los espacios
	plazo  = 10 * time.Second
)

type (
	// msgConexion es el resultado de comprobar el servicio; con reiniciar, la página actual se vuelve a iniciar; con
	// latido, era la comprobación periódica y programa la siguiente.
	msgConexion struct {
		cli       *cliente.Cliente
		espacios  []contrato.Espacio
		err       error
		reiniciar bool
		latido    bool
		cambios   int // los cambios de espacio que había al lanzarla
	}
	msgLatido struct{}
)

func latir() tea.Cmd { return tea.Tick(latido, func(time.Time) tea.Msg { return msgLatido{} }) }

// conectar comprueba el servicio leyendo sus espacios: con el cliente que ya hay y, si no responde, con el descriptor (se
// pudo reiniciar en otro puerto o con otro token). Con arrancar y Opciones.Iniciar, si sigue sin responder, lo inicia.
// Quien la lanza pone comprobando; el resultado lo quita.
func (m *modelo) conectar(arrancar, reiniciar, latido bool) tea.Cmd {
	cli, datos, iniciar, cambios := m.cli, m.o.Datos, m.o.Iniciar, m.cambios
	if !arrancar {
		iniciar = nil
	}
	return func() tea.Msg {
		leer := func(c *cliente.Cliente) msgConexion {
			ctx, cancelar := context.WithTimeout(context.Background(), plazo)
			defer cancelar()
			espacios, err := c.Espacios(ctx)
			return msgConexion{cli: c, espacios: espacios, err: err, reiniciar: reiniciar, latido: latido, cambios: cambios}
		}
		r := msgConexion{cli: cli, err: cliente.ErrServicioInactivo, reiniciar: reiniciar, latido: latido, cambios: cambios}
		if cli != nil {
			if r = leer(cli); r.err == nil {
				return r
			}
		}
		if c, err := cliente.Conectar(datos); err == nil {
			c.Origen = nucleo.DesdeTerminal
			if r2 := leer(c); r2.err == nil {
				return r2
			}
		} else if errors.Is(err, cliente.ErrServicioAnterior) {
			r.err = err // no es que no responda: la línea de estado dice que r lo detiene
		}
		if iniciar != nil {
			c, err := iniciar()
			if err == nil {
				return leer(c)
			}
			r.err = err
		}
		return r
	}
}

// conectado guarda el resultado de conectar. La primera vez elige el espacio: el del proyecto, si está activo, o el
// global. Si el espacio activo dejó de estarlo (se detuvo desde la Web UI, por ejemplo), vuelve al global y avisa.
func (m *modelo) conectado(r msgConexion) tea.Cmd {
	m.conectando, m.comprobando = false, false
	var cmds []tea.Cmd
	if r.latido {
		cmds = append(cmds, latir())
	}
	if r.err != nil {
		m.caido = r.err
	} else {
		r.reiniciar = r.reiniciar || m.caido != nil || m.cli != r.cli // si vuelve, la página se recarga
		if m.cli != r.cli {
			m.cerrar() // las de la pila tienen el cliente de antes (o ninguno): se inician con este al volver a ellas
		}
		m.cli, m.espacios, m.caido = r.cli, r.espacios, nil
		if r.latido {
			m.latidos++
		}
		if !m.preguntada || r.reiniciar || (r.latido && m.latidos%latidosNovedades == 0) { // r.reiniciar: el servicio volvió o es otro
			m.preguntada = true
			cmds = append(cmds, novedad(r.cli))
		}
		if r.cambios != m.cambios && !slices.ContainsFunc(r.espacios, func(e contrato.Espacio) bool { return e.ID == m.instancia }) {
			m.comprobando = true // leyó la lista antes de cambiar a un espacio que no trae: se vuelve a leer
			cmds = append(cmds, m.conectar(false, false, false))
		}
		switch nombre, activo := m.espacioActivo(m.instancia); {
		case m.instancia == "":
			m.instancia = "global"
			if _, ok := m.espacioActivo(m.o.IDProyecto); ok && m.o.IDProyecto != "" {
				m.instancia = m.o.IDProyecto
			}
		case !activo && m.instancia != "global" && r.cambios == m.cambios: // una lista leída antes del cambio no sabe del espacio nuevo
			cambio := func() tea.Msg { return pagina.CambioEspacio{ID: "global"} }
			return tea.Batch(append(cmds, cambio, comp.Aviso(idioma.Tf("el espacio %s se detuvo: vuelves al global", nombre), comp.Advertencia))...)
		}
	}
	if r.reiniciar {
		cmds = append(cmds, m.iniciar(nil))
	}
	return tea.Batch(cmds...)
}

// espacioActivo dice el nombre de un espacio (o su id, si no se conoce) y si está activo.
func (m *modelo) espacioActivo(id string) (string, bool) {
	for _, e := range m.espacios {
		if e.ID == id {
			return e.Nombre, e.Activo
		}
	}
	return id, false
}

// elegirEspacio abre el cambio rápido de espacio (Ctrl+E) con los espacios activos.
func (m *modelo) elegirEspacio() tea.Cmd {
	if m.cli == nil || m.caido != nil {
		return nil // el aviso de la app ya lo dice
	}
	var items []comp.Item
	for _, e := range m.espacios {
		if e.Activo {
			items = append(items, comp.Item{Valor: e.ID, Texto: e.Nombre, Detalle: e.Ruta})
		}
	}
	s := comp.NuevoSelector(m.estilos, m.estilos.Tx(m.idioma, "app.cambiarInstancia"), items)
	s.Marcar(m.instancia)
	m.selector = &s
	return nil
}

// cambiarEspacio cambia al espacio elegido en Ctrl+E. El de ahora no cambia nada; si una página tiene algo que se
// perdería (pagina.Pendiente), antes pregunta y dice qué, como al salir.
func (m *modelo) cambiarEspacio(id string) tea.Cmd {
	if id == m.instancia {
		return nil
	}
	if motivos := m.pendientes(); len(motivos) > 0 {
		m.pregunta, m.saliendo, m.cambiarA = &comp.Confirmacion{Texto: idioma.T("¿Cambiar de espacio?") + "\n\n" + strings.Join(motivos, "\n")}, false, id
		return nil
	}
	return func() tea.Msg { return pagina.CambioEspacio{ID: id} }
}

func (m *modelo) nombreEspacio() string {
	for _, e := range m.espacios {
		if e.ID == m.instancia {
			return e.Nombre
		}
	}
	return m.instancia
}

// vistaCabecera: la app y la página a la izquierda; el espacio activo y el estado del servicio a la derecha.
func (m *modelo) vistaCabecera() string {
	e := m.estilos
	izq := e.Acento.Render("Tramtai") + " " + e.Titulo.Render(m.pagina(m.actual).Titulo())
	var estado string
	switch {
	case m.conectando:
		estado = e.Tenue.Render(e.Iconos.Inactivo + " " + idioma.T("conectando…"))
	case m.caido != nil || m.cli == nil:
		estado = e.Error.Render(e.Iconos.Inactivo + " " + e.Tx(m.idioma, "app.sinConexion"))
	default:
		estado = e.Exito.Render(e.Iconos.Activo + " " + e.Tx(m.idioma, "app.enMarcha"))
	}
	der := estado
	if nombre := m.nombreEspacio(); nombre != "" {
		der = e.Texto.Render(nombre) + " " + e.Tenue.Render(e.Iconos.Separador) + " " + estado
	}
	libre := m.ancho - lipgloss.Width(izq) - 2
	if aviso := m.avisoNovedad(libre - lipgloss.Width(der) - 2); aviso != "" { // solo si cabe entero: lo demás no se corta
		der = aviso + "  " + der
	}
	der = e.Recortar(der, libre)
	return izq + strings.Repeat(" ", max(m.ancho-lipgloss.Width(izq)-lipgloss.Width(der), 1)) + der
}

// vistaEstado es la línea encima del pie: el aviso de servicio caído o, si no, el último aviso.
func (m *modelo) vistaEstado() string {
	e := m.estilos
	switch {
	case m.caido != nil && m.conectando:
		return e.Tenue.Render(idioma.T("Conectando con el servicio…"))
	case m.caido == cliente.ErrServicioAnterior:
		return e.Error.Render(idioma.T("Sigue en marcha el servicio de una versión anterior · r lo detiene y arranca el nuevo"))
	case m.anterior():
		return e.Error.Render(idioma.T("No se pudo detener el servicio de la versión anterior · ! dice cómo"))
	case m.caido != nil:
		return e.Error.Render(idioma.T("El servicio no responde · r reintenta"))
	}
	return m.avisos.Vista(e)
}

// anterior dice si el servicio que responde es el de una versión anterior (sin detener todavía o porque no se pudo):
// ! enseña su mensaje entero, que dice cómo detenerlo a mano.
func (m *modelo) anterior() bool { return errors.Is(m.caido, cliente.ErrServicioAnterior) }

// comunes son las teclas de la app fuera de un campo: las tres primeras se ven siempre en el pie; Ctrl+E y W, si caben.
func comunes() []pagina.Tecla {
	return []pagina.Tecla{{Tecla: "Tab", Accion: idioma.T("menú")}, {Tecla: "?", Accion: idioma.T("ayuda")}, {Tecla: "q", Accion: idioma.T("salir")},
		{Tecla: "Ctrl+E", Accion: idioma.T("espacio")}, {Tecla: "W", Accion: idioma.T("web")}}
}

// vistaPie son las teclas de lo que tiene el foco. Fuera de un campo, «Tab menú · ? ayuda · q salir» van a la derecha y
// se ven a cualquier ancho; las de la página ocupan lo que queda y, si no caben, ? las enseña todas.
func (m *modelo) vistaPie() string {
	e := m.estilos
	var teclas, fijas []pagina.Tecla
	switch {
	case m.pregunta != nil:
	case m.selector != nil, m.enMenu:
		teclas = []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("elige")}, {Tecla: "Esc", Accion: idioma.T("vuelve")}}
		if m.selector == nil { // en el selector, q filtra
			fijas = comunes()[2:3]
		}
	default:
		p := m.pagina(m.actual)
		teclas = p.Teclas()
		if !p.EnCampo() { // en un campo, ! se escribe: no abre el error
			teclas, fijas = append(teclas, comunes()[3:]...), comunes()[:3]
			if m.avisos.HayError() || m.anterior() {
				teclas = append([]pagina.Tecla{{Tecla: "!", Accion: idioma.T("ver el error")}}, teclas...)
			}
		}
	}
	sep := e.Tenue.Render(" " + e.Iconos.Separador + " ")
	der := m.teclasPie(fijas, sep)
	izq := e.Recortar(m.teclasPie(teclas, sep), max(m.ancho-lipgloss.Width(der)-lipgloss.Width(sep), 0))
	if der == "" || izq == "" {
		return e.Recortar(izq+der, m.ancho)
	}
	return izq + strings.Repeat(" ", max(m.ancho-lipgloss.Width(izq)-lipgloss.Width(der), 1)) + der
}

func (m *modelo) teclasPie(teclas []pagina.Tecla, sep string) string {
	e := m.estilos
	partes := make([]string, len(teclas))
	for i, t := range teclas {
		partes[i] = e.Tecla.Render(e.A(t.Tecla)) + " " + e.Tenue.Render(e.A(t.Accion))
	}
	return strings.Join(partes, sep)
}
