package adaptadores

import "tramtai/internal/nucleo"

// NivelesDe devuelve los niveles de nucleo.NivelesEsfuerzo que tienen valor en un CLI (los de su ficha).
func NivelesDe(valores []string) []string {
	return nucleo.NivelesEsfuerzo[:min(len(valores), len(nucleo.NivelesEsfuerzo))]
}

// ValorEsfuerzo traduce un nivel al valor del CLI; un nivel por encima de su máximo usa el máximo.
func ValorEsfuerzo(valores []string, nivel string) string {
	for i, n := range nucleo.NivelesEsfuerzo {
		if n == nivel && len(valores) > 0 {
			return valores[min(i, len(valores)-1)]
		}
	}
	return ""
}
