package adaptadores

import (
	"slices"
	"testing"
)

func TestEsfuerzo(t *testing.T) {
	casos := []struct{ agente, nivel, valor string }{
		{"claude", "bajo", "low"},
		{"claude", "muy_alto", "xhigh"},
		{"claude", "maximo", "max"},
		{"codex", "medio", "medium"},
		{"codex", "maximo", "xhigh"}, // por encima de su máximo, el máximo
		{"gemini", "alto", ""},       // no admite esfuerzo
		{"claude", "", ""},
	}
	for _, c := range casos {
		if v := valorEsfuerzo(c.agente, c.nivel); v != c.valor {
			t.Errorf("%s %s: %q, se esperaba %q", c.agente, c.nivel, v, c.valor)
		}
	}
	if n := NivelesEsfuerzo("codex"); !slices.Equal(n, []string{"bajo", "medio", "alto", "muy_alto"}) {
		t.Errorf("niveles de codex: %v", n)
	}
	if n := NivelesEsfuerzo("gemini"); len(n) != 0 {
		t.Errorf("gemini no admite esfuerzo: %v", n)
	}
}
