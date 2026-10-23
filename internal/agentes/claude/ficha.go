package claude

import (
	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// valoresEsfuerzo son los de --effort de Claude Code, que llega a max.
var valoresEsfuerzo = []string{"low", "medium", "high", "xhigh", "max"}

// Ficha es la de Claude Code para el catálogo de agentes.
var Ficha = adaptadores.Ficha{
	Nombre: "claude", NombreVisible: "Claude", Programa: "Claude Code", Codigo: "CL",
	ColorClaro: "#c4561d", ColorOscuro: "#f28a4f", Orden: 10,
	Streaming: "tokens", SalidaEstructurada: true, ModeloEfectivoInformado: true,
	ValoresEsfuerzo: valoresEsfuerzo, EsfuerzoPredeterminado: func(modelo string) string { return esfuerzoClaude[modelo] },
	Autenticaciones: []string{adaptadores.AutenticacionSuscripcion, adaptadores.AutenticacionClaveAPI},
	VariablesClave:  []string{"ANTHROPIC_API_KEY"},
	Comando:         "claude", Concurrencia: 1, PermitidoEnApps: true,
	URLInstalacion: "https://docs.anthropic.com/claude-code",
	Crear: func(c *configuracion.Configuracion) adaptadores.Adaptador {
		return Claude{Programa: c.Texto("comandos_agentes.claude"), Autenticacion: adaptadores.NuevoModo(c.Texto("adaptadores.claude.autenticacion"))}
	},
}

// esfuerzoClaude es el esfuerzo que usa Claude Code si no se le indica uno, en los modelos comprobados en su
// documentación (code.claude.com/docs/en/model-config, octubre de 2026): alto en los modelos con esfuerzo, salvo Opus,
// Sonnet y Haiku 5.5 (medio) y Opus 4.7 (muy alto). Un modelo que no está aquí no recibe un valor supuesto; los
// ajustes del usuario en el CLI (effortLevel por modelo) mandan sobre este valor.
var esfuerzoClaude = map[string]string{
	"claude-opus-5-5": "medio", "claude-sonnet-5-5": "medio", "claude-haiku-5-5": "medio",
	"claude-opus-4-7":  "muy_alto",
	"claude-fable-5-1": "alto", "claude-fable-5": "alto", "claude-opus-5": "alto", "claude-sonnet-5": "alto",
	"claude-opus-4-8": "alto", "claude-opus-4-6": "alto", "claude-sonnet-4-6": "alto",
}

// ModoAutenticacion implementa adaptadores.ConAutenticacion: la suscripción se cambia en marcha.
func (c Claude) ModoAutenticacion() *adaptadores.Modo { return c.Autenticacion }
