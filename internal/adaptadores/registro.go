package adaptadores

import (
	"maps"
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

// Conexiones dice qué agentes están desconectados (adaptadores.<agente>.desconectado): no se usan aunque su CLI esté
// instalado. Se comparte entre todas las instancias y se cambia en marcha (Web UI → Agentes); nil = todos conectados.
type Conexiones struct {
	desconectados atomic.Pointer[map[string]bool]
}

// NuevasConexiones lee de la configuración qué agentes están desconectados (c nil: ninguno).
func NuevasConexiones(c *nucleo.Configuracion, agentes []string) *Conexiones {
	m := map[string]bool{}
	for _, agente := range agentes {
		if c != nil && c.Booleano("adaptadores."+agente+".desconectado") {
			m[agente] = true
		}
	}
	con := &Conexiones{}
	con.desconectados.Store(&m)
	return con
}

// Desconectado indica si el agente está desconectado.
func (c *Conexiones) Desconectado(agente string) bool {
	if c == nil {
		return false
	}
	m := c.desconectados.Load()
	return m != nil && (*m)[agente]
}

// Fijar conecta o desconecta un agente en marcha.
func (c *Conexiones) Fijar(agente string, desconectado bool) {
	for {
		actual := c.desconectados.Load()
		nuevo := map[string]bool{}
		if actual != nil {
			maps.Copy(nuevo, *actual)
		}
		if desconectado {
			nuevo[agente] = true
		} else {
			delete(nuevo, agente)
		}
		if c.desconectados.CompareAndSwap(actual, &nuevo) {
			return
		}
	}
}

// EsVariableCredencial indica si una variable de entorno es una clave de proveedor que se retira del entorno heredado.
// La usa la prueba del catálogo para que la clave de cada agente nuevo no llegue a los demás CLI.
func EsVariableCredencial(nombre string) bool { return slices.Contains(variablesCredenciales, nombre) }
