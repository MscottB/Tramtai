package adaptadores

import (
	"sync/atomic"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Registro crea los adaptadores reales según la configuración efectiva y devuelve el límite de ejecuciones
// simultáneas de cada uno (concurrencia_servicio.adaptadores).
func Registro(c *nucleo.Configuracion) (map[string]Adaptador, map[string]int) {
	registro := map[string]Adaptador{
		"claude": Claude{Programa: c.Texto("comandos_agentes.claude"), Autenticacion: NuevoModo(c.Texto("adaptadores.claude.autenticacion"))},
		"codex":  Codex{Programa: c.Texto("comandos_agentes.codex"), Autenticacion: NuevoModo(c.Texto("adaptadores.codex.autenticacion"))},
		"gemini": Gemini{},
		"antigravity": Antigravity{Programa: c.Texto("comandos_agentes.antigravity"),
			Autenticacion: c.Texto("adaptadores.antigravity.autenticacion")},
	}
	limites := map[string]int{}
	for nombre := range registro {
		limites[nombre] = c.Entero("concurrencia_servicio.adaptadores." + nombre)
	}
	return registro, limites
}

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

// CambiarAutenticacion cambia el modo de claude o codex en el registro del servicio en marcha.
func CambiarAutenticacion(registro map[string]Adaptador, agente, modo string) error {
	if modo != AutenticacionClaveAPI && modo != AutenticacionSuscripcion {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("modo de autenticación desconocido: %s", modo))
	}
	var m *Modo
	switch a := registro[agente].(type) {
	case Claude:
		m = a.Autenticacion
	case Codex:
		m = a.Autenticacion
	}
	if m == nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("solo claude y codex admiten la suscripción"))
	}
	m.valor.Store(&modo)
	return nil
}
