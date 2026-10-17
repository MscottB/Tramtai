package nucleo

import "tramtai/internal/idioma"

// Rangos de la retención (retencion.horas_resultado, horas_metadatos y max_trabajos). 0 horas de resultado es no
// guardar las respuestas.
const (
	MaximoHorasResultado = 720  // 30 días
	MaximoHorasMetadatos = 2160 // 90 días
	MinimoTrabajos       = 100
	MaximoTrabajos       = 100000
)

// AjustarRetencion lleva cada valor a su rango y el registro a no durar menos que la respuesta; indica si cambió
// algo. Se usa al cargar la configuración, para que un valor fuera de rango no impida arrancar.
func AjustarRetencion(resultado, metadatos, maximo int) (int, int, int, bool) {
	r := min(max(resultado, 0), MaximoHorasResultado)
	m := max(min(max(metadatos, 0), MaximoHorasMetadatos), r)
	x := min(max(maximo, MinimoTrabajos), MaximoTrabajos)
	return r, m, x, r != resultado || m != metadatos || x != maximo
}

// ValidarRetencion rechaza con el motivo los valores que AjustarRetencion cambiaría; se usa al fijarlos.
func ValidarRetencion(resultado, metadatos, maximo int) error {
	switch {
	case resultado < 0 || resultado > MaximoHorasResultado:
		return NuevoError(CodigoSolicitudInvalida, idioma.Tf("las horas de respuesta deben estar entre 0 y %d", MaximoHorasResultado))
	case metadatos < 0 || metadatos > MaximoHorasMetadatos:
		return NuevoError(CodigoSolicitudInvalida, idioma.Tf("las horas de registro deben estar entre 0 y %d", MaximoHorasMetadatos))
	case metadatos < resultado:
		return NuevoError(CodigoSolicitudInvalida, idioma.T("el registro no puede durar menos que la respuesta"))
	case maximo < MinimoTrabajos || maximo > MaximoTrabajos:
		return NuevoError(CodigoSolicitudInvalida, idioma.Tf("el máximo de peticiones debe estar entre %d y %d", MinimoTrabajos, MaximoTrabajos))
	}
	return nil
}
