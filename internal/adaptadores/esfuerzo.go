package adaptadores

import (
	"strings"

	"tramtai/internal/nucleo"
)

// valoresEsfuerzo son los niveles que entiende cada CLI, en el orden de nucleo.NivelesEsfuerzo: Claude Code
// (--effort) llega a max; Codex (model_reasoning_effort) a xhigh. Los demás agentes no tienen esta opción.
var valoresEsfuerzo = map[string][]string{
	"claude": {"low", "medium", "high", "xhigh", "max"},
	"codex":  {"low", "medium", "high", "xhigh"},
}

// NivelesEsfuerzo devuelve los niveles que admite un agente (nil si no admite ninguno).
func NivelesEsfuerzo(agente string) []string {
	return nucleo.NivelesEsfuerzo[:len(valoresEsfuerzo[agente])]
}

// valorEsfuerzo traduce un nivel al valor del CLI; un nivel por encima de su máximo usa el máximo.
func valorEsfuerzo(agente, nivel string) string {
	valores := valoresEsfuerzo[agente]
	for i, n := range nucleo.NivelesEsfuerzo {
		if n == nivel && len(valores) > 0 {
			return valores[min(i, len(valores)-1)]
		}
	}
	return ""
}

// EsfuerzoPredeterminado es el nivel que usa el CLI si no se le indica uno, para un modelo concreto (la versión real,
// por ejemplo claude-opus-5-5); "" si no se conoce con certeza. Claude Code (code.claude.com/docs/en/model-config,
// octubre de 2026): alto en los modelos con esfuerzo, salvo Opus, Sonnet y Haiku 5.5 (medio) y Opus 4.7 (muy alto).
// Codex no tiene un valor fijo: depende del modelo y no se documenta por modelo. Los ajustes del usuario en el CLI
// (effortLevel por modelo) mandan sobre este valor.
func EsfuerzoPredeterminado(agente, modelo string) string {
	if agente != "claude" || !strings.HasPrefix(modelo, "claude-") {
		return ""
	}
	switch {
	case strings.Contains(modelo, "-5-5"):
		return "medio"
	case strings.Contains(modelo, "opus-4-7"):
		return "muy_alto"
	}
	return "alto"
}
