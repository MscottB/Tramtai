package nucleo

import (
	"slices"

	"tramtai/internal/idioma"
)

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

// Terminales son los estados finales; la Web UI recibe la lista generada (internal/contrato/gents).
var Terminales = []Estado{Completado, Fallido, Cancelado, Interrumpido}

// Terminal indica si el estado es final: ningún estado terminal vuelve a uno activo.
func (e Estado) Terminal() bool { return slices.Contains(Terminales, e) }

// estados son todos, en el orden del ciclo de vida.
var estados = []Estado{EnCola, PreparandoEntrada, EsperandoAgente, Ejecutando, ValidandoSalida, CancelacionSolicitada,
	Completado, Fallido, Cancelado, Interrumpido}

// Los filtros agrupan los estados como el filtro del Historial y los contadores de Inicio: en curso es todo lo que no
// terminó, completadas solo completado y con error fallido e interrumpido; los cancelados solo salen sin filtro.
const (
	FiltroEnCurso    = "en_curso"
	FiltroCompletado = "completado"
	FiltroConError   = "con_error"
)

// Filtros son los filtros de estado, en el orden del selector.
var Filtros = []string{FiltroEnCurso, FiltroCompletado, FiltroConError}

// EstadosDe son los estados de un filtro (nil si no es uno de Filtros).
func EstadosDe(filtro string) []Estado {
	var lista []Estado
	for _, e := range estados {
		if filtro == FiltroEnCurso && !e.Terminal() || filtro == FiltroCompletado && e == Completado ||
			filtro == FiltroConError && (e == Fallido || e == Interrumpido) {
			lista = append(lista, e)
		}
	}
	return lista
}

// Etapa agrupa los estados en las 5 paradas de la ruta de un trabajo: cola, preparacion, ejecucion, validacion y
// llegada. Viaja en el JSON del trabajo y en el evento «trabajo»: la Web UI y la terminal solo la pintan.
func (e Estado) Etapa() string {
	switch e {
	case EnCola:
		return "cola"
	case PreparandoEntrada, EsperandoAgente:
		return "preparacion"
	case Ejecutando, CancelacionSolicitada:
		return "ejecucion"
	case ValidandoSalida:
		return "validacion"
	default:
		return "llegada"
	}
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
