package comp

import (
	"strings"
	"testing"
)

func TestSelectorFiltraYElige(t *testing.T) {
	s := NuevoSelector(estilos(), "Cambiar de espacio", []Item{{Valor: "global", Texto: "Global"}, {Valor: "notas_1", Texto: "Notas", Detalle: "C:/x/notas"}, {Valor: "soporte_2", Texto: "Soporte"}})
	if v := s.Vista(40, 10); !strings.Contains(v, "Cambiar de espacio") || !strings.Contains(v, "Notas") {
		t.Fatalf("vista:\n%s", v)
	}
	escribir(s.Update, "so")
	if v := s.Vista(40, 10); strings.Contains(v, "Notas") || !strings.Contains(v, "Soporte") {
		t.Fatalf("escribir filtra:\n%s", v)
	}
	if e, ok := s.Update(tecla("enter"))().(Elegido); !ok || e.Valor != "soporte_2" {
		t.Fatalf("Intro elige: %+v", e)
	}
	s.Update(tecla("backspace"))
	s.Update(tecla("backspace"))
	if v := s.Vista(40, 10); !strings.Contains(v, "Notas") {
		t.Fatalf("borrar el filtro vuelve a enseñarlo todo:\n%s", v)
	}
	if _, ok := s.Update(tecla("esc"))().(Cancelado); !ok {
		t.Fatal("Esc cancela")
	}
}
