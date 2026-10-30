package codex

import (
	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// valoresEsfuerzo son los de model_reasoning_effort de Codex, que llega a xhigh.
var valoresEsfuerzo = []string{"low", "medium", "high", "xhigh"}

// Ficha es la de Codex CLI para el catálogo de agentes. Codex no tiene un esfuerzo predeterminado fijo: depende del
// modelo y no se documenta por modelo.
var Ficha = adaptadores.Ficha{
	Nombre: "codex", NombreVisible: "Codex", Programa: "Codex CLI", Codigo: "CX",
	ColorClaro: "#2563eb", ColorOscuro: "#5b8def", Orden: 20,
	Streaming: "final", SalidaEstructurada: true,
	ValoresEsfuerzo: valoresEsfuerzo,
	Autenticaciones: []string{adaptadores.AutenticacionSuscripcion, adaptadores.AutenticacionClaveAPI},
	VariablesClave:  []string{"OPENAI_API_KEY", "CODEX_API_KEY"},
	Comando:         "codex", Concurrencia: 1, PermitidoEnApps: true,
	URLInstalacion: "https://developers.openai.com/codex/cli",
	// La página de políticas y no la de un solo país: OpenAI tiene términos distintos para Europa y para el resto del mundo.
	InicioSesion: []string{"login", "--device-auth"}, URLTerminos: "https://openai.com/policies/",
	Crear: func(c *configuracion.Configuracion) adaptadores.Adaptador {
		return Codex{Programa: c.Texto("comandos_agentes.codex"), Autenticacion: adaptadores.NuevoModo(c.Texto("adaptadores.codex.autenticacion"))}
	},
}

// ModoAutenticacion: la suscripción se cambia en marcha.
func (c Codex) ModoAutenticacion() *adaptadores.Modo { return c.Autenticacion }
