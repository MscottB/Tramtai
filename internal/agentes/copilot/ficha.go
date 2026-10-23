package copilot

import (
	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// valoresEsfuerzo son los de --reasoning-effort de Copilot CLI, que llega a max; con el modelo auto no se envían.
var valoresEsfuerzo = []string{"low", "medium", "high", "xhigh", "max"}

// Ficha es la de GitHub Copilot CLI para el catálogo de agentes. No tiene salida estructurada propia: el esquema va en
// las instrucciones y Tramtai valida la respuesta. Su esfuerzo predeterminado depende del modelo y no se documenta.
var Ficha = adaptadores.Ficha{
	Nombre: "copilot", NombreVisible: "GitHub Copilot", Programa: "GitHub Copilot CLI", Codigo: "GH",
	ColorClaro: "#1f883d", ColorOscuro: "#3fb950", Orden: 25,
	Streaming: "tokens", ModeloEfectivoInformado: true,
	ValoresEsfuerzo: valoresEsfuerzo, ModelosSinEsfuerzo: []string{"", "auto"},
	Autenticaciones: []string{adaptadores.AutenticacionSuscripcion, adaptadores.AutenticacionClaveAPI},
	VariablesClave:  []string{variableClave},
	Comando:         "copilot", Concurrencia: 1, PermitidoEnApps: true,
	URLInstalacion: "https://docs.github.com/copilot/how-tos/set-up/install-copilot-cli",
	Crear: func(c *configuracion.Configuracion) adaptadores.Adaptador {
		return Copilot{Programa: c.Texto("comandos_agentes.copilot"), Autenticacion: adaptadores.NuevoModo(c.Texto("adaptadores.copilot.autenticacion"))}
	},
}

// ModoAutenticacion: la suscripción se cambia en marcha.
func (c Copilot) ModoAutenticacion() *adaptadores.Modo { return c.Autenticacion }
