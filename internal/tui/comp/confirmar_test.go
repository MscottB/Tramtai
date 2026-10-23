package comp

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestConfirmarEsc(t *testing.T) {
	p, ok := Confirmar("¿Borrar mi-app?")().(Pregunta)
	if !ok {
		t.Fatal("Confirmar pide a la app que pregunte")
	}
	c := Confirmacion{Texto: p.Texto}
	if !strings.Contains(c.Vista(estilos(), 60, 17), "¿Borrar mi-app?") {
		t.Fatalf("enseña la pregunta:\n%s", c.Vista(estilos(), 60, 17))
	}
	if listo, cmd := c.Update(tecla("z")); listo || cmd != nil {
		t.Fatal("otra tecla no responde")
	}
	listo, cmd := c.Update(tecla("esc"))
	if _, ok := cmd().(Cancelado); !listo || !ok {
		t.Fatal("Esc cancela")
	}
	if listo, cmd := c.Update(tecla("enter")); listo || cmd != nil {
		t.Fatal("Intro no confirma: borrar pide s o y")
	}
	for _, k := range []string{"s", "y", "S", "Y"} {
		listo, cmd := c.Update(tecla(k))
		if _, ok := cmd().(Confirmado); !listo || !ok {
			t.Fatalf("%s confirma", k)
		}
	}
	if listo, _ := c.Update(tecla("n")); !listo {
		t.Fatal("n cancela")
	}
}

// textoLargo es una pregunta de unas 30 líneas a 54 columnas.
func textoLargo() string {
	return strings.TrimSpace(strings.Repeat("Una frase de la pregunta que ocupa su sitio en la caja y la hace crecer. ", 24))
}

func TestConfirmarCabe(t *testing.T) {
	c := Confirmacion{Texto: textoLargo()}
	v := c.Vista(estilos(), 58, 17)
	if h := lipgloss.Height(v); h > 17 {
		t.Fatalf("la caja mide %d filas y el área es de 17:\n%s", h, v)
	}
	if !strings.Contains(v, "s confirma · Esc cancela") {
		t.Fatalf("el pie siempre se ve:\n%s", v)
	}
	// El texto se desplaza con ↑↓ hasta el final, y el pie sigue ahí.
	antes := v
	for range 40 {
		c.Update(tecla("down"))
		v = c.Vista(estilos(), 58, 17)
	}
	if v == antes || !strings.Contains(v, "s confirma · Esc cancela") || lipgloss.Height(v) > 17 {
		t.Fatalf("desplazada:\n%s", v)
	}
	if listo, _ := c.Update(tecla("down")); listo {
		t.Fatal("desplazar no responde la pregunta")
	}
	c.Update(tecla("home"))
	if c.Vista(estilos(), 58, 17) != antes {
		t.Fatal("Inicio vuelve al principio")
	}
}
