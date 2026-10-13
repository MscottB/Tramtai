package adaptadores

import "tramtai/internal/nucleo"

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
