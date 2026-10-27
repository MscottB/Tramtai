package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/terminos"
	"tramtai/internal/tui/datos"
)

// riesgosTerminos son los cinco riesgos de los términos en el idioma de la terminal, con los textos de la Web UI. Sin el
// modo servidor, el 3 y el 4 llevan delante «Si usas el modo servidor, » (como terminos.Riesgos).
func (m *modelo) riesgosTerminos() []string {
	r := make([]string, 5)
	for i := range r {
		r[i] = datos.Texto(m.idioma, fmt.Sprintf("riesgos.%d", i+1))
		if i == 2 || i == 3 {
			r[i] = terminos.SiServidor(datos.Texto(m.idioma, "riesgos.siServidor"), r[i])
		}
	}
	return r
}

// teclaTerminos atiende la pantalla «Antes de seguir»: Enter acepta (queda en datos/terminos.json) y q o Ctrl+C salen sin
// usar nada. Es lo único que responde hasta aceptar.
func (m *modelo) teclaTerminos(tecla string) tea.Cmd {
	switch tecla {
	case "enter":
		if err := terminos.Aceptar(m.o.Datos, terminos.ComoInterfaz); err != nil {
			m.errTerminos = idioma.Tf("No se pudo guardar la aceptación: %s", err.Error())
			return nil
		}
		m.terminos, m.errTerminos = false, ""
	case "q", "ctrl+c":
		return tea.Quit
	}
	return nil
}

// vistaTerminos es la pantalla entera «Antes de seguir»: los cinco riesgos, dónde leer los términos y las teclas.
func (m *modelo) vistaTerminos() string {
	e := m.estilos
	caja := lipgloss.NewStyle().Width(max(min(m.ancho-4, 90), 10))
	lineas := []string{"", e.Titulo.Render(datos.Texto(m.idioma, "terminos.titulo")), "",
		caja.Render(datos.Textof(m.idioma, "terminos.intro", "version", terminos.Version)), ""}
	for i, riesgo := range m.riesgosTerminos() {
		lineas = append(lineas, caja.Render(fmt.Sprintf("%d. %s", i+1, riesgo)), "")
	}
	lineas = append(lineas, e.Tenue.Render(caja.Render(datos.Textof(m.idioma, "terminos.leerTerminal", "url", terminos.URL))))
	if m.errTerminos != "" {
		lineas = append(lineas, "", e.Error.Render(caja.Render(m.errTerminos)))
	}
	cuerpo := "  " + strings.ReplaceAll(strings.Join(lineas, "\n"), "\n", "\n  ")
	sep := e.Tenue.Render(" " + e.Iconos.Separador + " ")
	pie := e.Tecla.Render("Enter") + " " + e.Tenue.Render(datos.Texto(m.idioma, "terminos.acepto")) + sep +
		e.Tecla.Render("q") + " " + e.Tenue.Render(datos.Texto(m.idioma, "terminos.salir"))
	return m.encajar(cuerpo, m.ancho, max(m.alto-1, 0)) + "\n" + m.encajar("  "+pie, m.ancho, 1)
}
