package adaptadores

import (
	"slices"
	"sync/atomic"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Modo es la autenticación de un adaptador CLI. Se comparte entre todas las instancias del servicio y se puede
// cambiar en marcha (Web UI → Agentes); nil equivale a clave_api.
type Modo struct{ valor atomic.Pointer[string] }

// NuevoModo crea un modo con el valor inicial de la configuración.
func NuevoModo(valor string) *Modo {
	m := &Modo{}
	m.valor.Store(&valor)
	return m
}

// Valor devuelve el modo actual.
func (m *Modo) Valor() string {
	if m == nil {
		return AutenticacionClaveAPI
	}
	return *m.valor.Load()
}

// ConAutenticacion lo cumplen los adaptadores cuya autenticación (suscripción o clave de API) se cambia en marcha.
type ConAutenticacion interface {
	ModoAutenticacion() *Modo
}

// CambiarAutenticacion cambia el modo de un agente del registro del servicio en marcha.
func CambiarAutenticacion(registro map[string]Adaptador, agente, modo string) error {
	if modo != AutenticacionClaveAPI && modo != AutenticacionSuscripcion {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("modo de autenticación desconocido: %s", modo))
	}
	a, ok := registro[agente].(ConAutenticacion)
	if !ok || a.ModoAutenticacion() == nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s no admite la suscripción", agente))
	}
	a.ModoAutenticacion().valor.Store(&modo)
	return nil
}

// EsVariableCredencial indica si una variable de entorno es una clave de proveedor que se retira del entorno heredado.
func EsVariableCredencial(nombre string) bool { return slices.Contains(variablesCredenciales, nombre) }
