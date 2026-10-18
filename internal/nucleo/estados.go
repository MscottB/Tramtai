package nucleo

import "tramtai/internal/idioma"

// Estado es el estado del ciclo de vida de un trabajo.
type Estado string

const (
	EnCola                Estado = "en_cola"
	PreparandoEntrada     Estado = "preparando_entrada"
	EsperandoAgente       Estado = "esperando_agente"
	Ejecutando            Estado = "ejecutando"
	ValidandoSalida       Estado = "validando_salida"
	CancelacionSolicitada Estado = "cancelacion_solicitada"
	Completado            Estado = "completado"
	Fallido               Estado = "fallido"
	Cancelado             Estado = "cancelado"
	Interrumpido          Estado = "interrumpido"
)

// transiciones es la tabla de transiciones permitidas; los estados terminales no tienen salidas.
// Las vueltas a en_cola son el reencolado de trabajos que no tienen un agente en marcha: los que nunca lo lanzaron
// y los que esperaban al siguiente del respaldo (el intento fallido ya terminó y el respaldo guardado solo
// conserva los que quedan).
var transiciones = map[Estado][]Estado{
	EnCola:            {PreparandoEntrada, EsperandoAgente, Fallido, CancelacionSolicitada},
	PreparandoEntrada: {EsperandoAgente, Fallido, CancelacionSolicitada, EnCola},
	EsperandoAgente:   {Ejecutando, Fallido, CancelacionSolicitada, EnCola},
	// Ejecutando → EsperandoAgente: el agente falló antes de producir texto y sigue uno del respaldo.
	Ejecutando: {ValidandoSalida, Fallido, CancelacionSolicitada, Interrumpido, EsperandoAgente},
	// ValidandoSalida → EsperandoAgente: la salida no cumple y se pide una corrección al mismo agente.
	ValidandoSalida:       {Completado, Fallido, CancelacionSolicitada, Interrumpido, EsperandoAgente},
	CancelacionSolicitada: {Cancelado, Fallido, Interrumpido},
}

// Terminal indica si el estado es final: ningún estado terminal vuelve a uno activo.
func (e Estado) Terminal() bool {
	return e == Completado || e == Fallido || e == Cancelado || e == Interrumpido
}

// Valido indica si el estado pertenece al ciclo de vida.
func (e Estado) Valido() bool {
	_, activo := transiciones[e]
	return activo || e.Terminal()
}

// ValidarTransicion devuelve transicion_invalida si la tabla no permite pasar de "de" a "a".
func ValidarTransicion(de, a Estado) error {
	for _, permitido := range transiciones[de] {
		if permitido == a {
			return nil
		}
	}
	return NuevoError(CodigoTransicionInvalida, idioma.Tf("transición no permitida: %s → %s", de, a))
}
