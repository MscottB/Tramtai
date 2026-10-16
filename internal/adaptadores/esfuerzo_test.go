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

func TestEsfuerzoPredeterminado(t *testing.T) {
	casos := map[[2]string]string{
		{"claude", "claude-opus-5-5"}:   "medio",
		{"claude", "claude-sonnet-5-5"}: "medio",
		{"claude", "claude-haiku-5-5"}:  "medio",
		{"claude", "claude-opus-4-7"}:   "muy_alto",
		{"claude", "claude-fable-5-1"}:  "alto",
		{"claude", "opus"}:              "", // un alias puede cambiar de modelo: sin la versión real no se sabe
		{"claude", ""}:                  "",
		{"codex", "gpt-5.6-sol"}:        "", // Codex depende del modelo y no lo documenta por modelo
		{"gemini", "gemini-pro-latest"}: "",
	}
	for c, esperado := range casos {
		if obtenido := EsfuerzoPredeterminado(c[0], c[1]); obtenido != esperado {
			t.Errorf("%s/%s: %q, se esperaba %q", c[0], c[1], obtenido, esperado)
		}
	}
}
