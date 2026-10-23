package tui

import (
	"context"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// Novedades: al conectar, y después cada 15 minutos como la Web UI, la terminal pregunta en segundo plano al servicio si
// hay una versión nueva (lo comprueba su actualizacion.Comprobador, como mucho una vez por hora). Si la hay, la cabecera
// lo avisa y u abre sus novedades en Ajustes, donde se instala.

// latidosNovedades son los latidos (de 10 s) entre una pregunta y la siguiente: 15 minutos.
const latidosNovedades = 90

// msgNovedad es la respuesta del servicio; un error deja el aviso como estaba.
type msgNovedad struct {
	estado contrato.EstadoActualizacion
	err    error
}

// novedad pregunta al servicio por las actualizaciones.
func novedad(cli *cliente.Cliente) tea.Cmd {
	return func() tea.Msg {
		ctx, cancelar := context.WithTimeout(context.Background(), plazo)
		defer cancelar()
		e, err := cli.Actualizacion(ctx, false)
		return msgNovedad{estado: e, err: err}
	}
}

// avisoNovedad es el aviso de la cabecera, el más largo que quepa en libre columnas; "" sin versión nueva o sin sitio.
func (m *modelo) avisoNovedad(libre int) string {
	if !m.novedad.Disponible {
		return ""
	}
	for _, aviso := range []string{datos.Texto(m.idioma, "actualizacion.boton") + " · u", m.novedad.Version + " · u"} {
		if aviso = m.estilos.A(aviso); lipgloss.Width(aviso) <= libre {
			return m.estilos.Acento.Render(aviso)
		}
	}
	return ""
}

// teclaNovedad: con una versión nueva, u lleva a sus novedades, salvo en una página que use la u para otra cosa.
func (m *modelo) teclaNovedad(p pagina.Pagina) (tea.Cmd, bool) {
	if !m.novedad.Disponible || slices.ContainsFunc(p.Teclas(), func(t pagina.Tecla) bool { return t.Tecla == "u" }) {
		return nil, false
	}
	return m.ir(pagina.PaginaConfiguracion, pagina.ConNovedades{}, true), true
}
