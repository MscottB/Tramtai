package antigravity

import (
	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// Ficha es la de Antigravity (agy) para el catálogo de agentes. Experimental: no se usa desde apps ni APIs hasta que su
// canario pase con el CLI real.
var Ficha = adaptadores.Ficha{
	Nombre: "antigravity", NombreVisible: "Antigravity", Programa: "Antigravity CLI", Codigo: "AG",
	ColorClaro: "#db2777", ColorOscuro: "#f472b6", Orden: 40,
	Streaming: "final", SalidaEstructurada: true,
	Autenticaciones: []string{adaptadores.AutenticacionClaveAPI},
	VariablesClave:  []string{"GEMINI_API_KEY"},
	Comando:         "agy", Concurrencia: 1,
	URLInstalacion: "https://antigravity.google",
	Crear: func(c *nucleo.Configuracion) adaptadores.Adaptador {
		return Antigravity{Programa: c.Texto("comandos_agentes.antigravity"), Autenticacion: c.Texto("adaptadores.antigravity.autenticacion")}
	},
}
