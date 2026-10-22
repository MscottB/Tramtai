package agentes

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

// TestAgentesConfirmacionDeSuscripcionCabe: la pregunta de pasar a la suscripción, con la caja de la app en 60×20, mide
// lo que cabe y deja ver el riesgo, la aceptación y el pie.
func TestAgentesConfirmacionDeSuscripcionCabe(t *testing.T) {
	p, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "clave_api"})})
	_, cmd := p.Update(tecla("a"))
	q := correr(p, cmd)[0].(comp.Pregunta)
	c := comp.Confirmacion{Texto: q.Texto}
	v := c.Vista(&pagina.Estilos{Iconos: pagina.IconosUnicode}, 58, 17) // el área de 60×20 con la caja de la app (ancho-2)
	if h := lipgloss.Height(v); h > 17 {
		t.Fatalf("la caja mide %d filas y el área es de 17:\n%s", h, v)
	}
	for _, quiere := range []string{"Ese riesgo es tuyo", "Pulsa s para confirmar: Entiendo el riesgo", "s confirma · Esc cancela"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q a la vista:\n%s", quiere, v)
		}
	}
}
