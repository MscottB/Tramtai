package adaptadores

import (
	"strings"

	"tramtai/internal/nucleo"
)

// FormatoEnInstrucciones pide el formato en las instrucciones, para los agentes que no tienen salida estructurada
// propia (o no la pueden usar): Tramtai valida la respuesta después como con cualquiera. Va al modelo, no al usuario:
// no se traduce.
func FormatoEnInstrucciones(c nucleo.ConfigSalida) string {
	switch {
	case len(c.Esquema) > 0:
		return "Responde solo con un JSON válido que cumpla este JSON Schema, sin texto adicional ni bloques de código:\n" + string(c.Esquema)
	case c.Formato == "json":
		return "Responde solo con un JSON válido, sin texto adicional ni bloques de código."
	}
	return ""
}

// SinBloqueDeCodigo quita el bloque ```json … ``` con el que algunos modelos envuelven el JSON aunque se les pida que no.
func SinBloqueDeCodigo(texto string) string {
	t := strings.TrimSpace(texto)
	if !strings.HasPrefix(t, "```") || !strings.HasSuffix(t, "```") || len(t) < 6 {
		return texto
	}
	_, cuerpo, ok := strings.Cut(strings.TrimSuffix(t, "```"), "\n")
	if !ok {
		return texto
	}
	return strings.TrimSpace(cuerpo)
}
