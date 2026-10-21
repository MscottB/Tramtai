package tui

import (
	"context"
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
	// msgConexion es el resultado de comprobar el servicio; con reiniciar, la página actual se vuelve a iniciar.
	msgConexion struct {
		cli       *cliente.Cliente
		espacios  []contrato.Espacio
		err       error
		reiniciar bool
	}
	msgLatido struct{}
)

func latir() tea.Cmd { return tea.Tick(latido, func(time.Time) tea.Msg { return msgLatido{} }) }

// conectar comprueba el servicio leyendo sus espacios: con el cliente que ya hay y, si no responde, con el descriptor (se
// pudo reiniciar en otro puerto o con otro token). Con arrancar y Opciones.Iniciar, si sigue sin responder, lo inicia.
func (m *modelo) conectar(arrancar, reiniciar bool) tea.Cmd {
	cli, datos, iniciar := m.cli, m.o.Datos, m.o.Iniciar
	if !arrancar {
		iniciar = nil
	}
	return func() tea.Msg {
		leer := func(c *cliente.Cliente) msgConexion {
			ctx, cancelar := context.WithTimeout(context.Background(), plazo)
			defer cancelar()
			espacios, err := c.Espacios(ctx)
			return msgConexion{cli: c, espacios: espacios, err: err, reiniciar: reiniciar}
		}
		r := msgConexion{cli: cli, err: cliente.ErrServicioInactivo, reiniciar: reiniciar}
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
// global.
func (m *modelo) conectado(r msgConexion) tea.Cmd {
	m.conectando = false
	if r.err != nil {
		m.caido = r.err
	} else {
		r.reiniciar = r.reiniciar || m.caido != nil || m.cli != r.cli // si vuelve, la página se recarga
		m.cli, m.espacios, m.caido = r.cli, r.espacios, nil
		if m.instancia == "" {
			m.instancia = "global"
			for _, e := range m.espacios {
				if e.ID == m.o.IDProyecto && e.Activo {
					m.instancia = e.ID
				}
			}
		}
	}
	if r.reiniciar {
		return m.iniciar(nil)
	}
	return nil
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
	s := comp.NuevoSelector(m.estilos, idioma.T("Cambiar de espacio"), items)
	m.selector = &s
	return nil
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
		estado = e.Error.Render(e.Iconos.Inactivo + " " + idioma.T("sin servicio"))
	default:
		estado = e.Exito.Render(e.Iconos.Activo + " " + idioma.T("servicio en marcha"))
	}
	der := estado
	if nombre := m.nombreEspacio(); nombre != "" {
		der = e.Texto.Render(nombre) + " " + e.Tenue.Render(e.Iconos.Separador) + " " + estado
	}
	libre := m.ancho - lipgloss.Width(izq) - 2
	der = e.Recortar(der, libre)
	return izq + strings.Repeat(" ", max(m.ancho-lipgloss.Width(izq)-lipgloss.Width(der), 1)) + der
}

// vistaEstado es la línea encima del pie: el aviso de servicio caído o, si no, el último aviso.
func (m *modelo) vistaEstado() string {
	e := m.estilos
	switch {
	case m.caido != nil && m.conectando:
		return e.Tenue.Render(idioma.T("Conectando con el servicio…"))
	case m.caido != nil:
		return e.Error.Render(idioma.T("El servicio no responde · r reintenta"))
	}
	return m.avisos.Vista(e)
}

// vistaPie son las teclas de lo que tiene el foco y, si no hay un campo, las comunes.
func (m *modelo) vistaPie() string {
	e := m.estilos
	var teclas []pagina.Tecla
	switch {
	case m.pregunta != nil:
	case m.selector != nil, m.enMenu:
		teclas = []pagina.Tecla{{Tecla: idioma.T("Intro"), Accion: idioma.T("elige")}, {Tecla: "Esc", Accion: idioma.T("vuelve")}}
	default:
		p := m.pagina(m.actual)
		teclas = p.Teclas()
		if !p.EnCampo() {
			teclas = append(teclas, pagina.Tecla{Tecla: "Tab", Accion: idioma.T("menú")}, pagina.Tecla{Tecla: "?", Accion: idioma.T("ayuda")},
				pagina.Tecla{Tecla: "Ctrl+E", Accion: idioma.T("espacio")}, pagina.Tecla{Tecla: "q", Accion: idioma.T("salir")})
		}
	}
	var partes []string
	for _, t := range teclas {
		partes = append(partes, e.Tecla.Render(t.Tecla)+" "+e.Tenue.Render(t.Accion))
	}
	return e.Recortar(strings.Join(partes, e.Tenue.Render(" "+e.Iconos.Separador+" ")), m.ancho)
}
