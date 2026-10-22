package comp

import (
	"strings"
	"testing"
)

func TestConfirmarEsc(t *testing.T) {
	p, ok := Confirmar("¿Borrar mi-app?")().(Pregunta)
	if !ok {
		t.Fatal("Confirmar pide a la app que pregunte")
	}
	c := Confirmacion{Texto: p.Texto}
	if !strings.Contains(c.Vista(estilos(), 60), "¿Borrar mi-app?") {
		t.Fatalf("enseña la pregunta:\n%s", c.Vista(estilos(), 60))
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
