package antigravity

import (
	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// Ficha es la de Antigravity (agy) para el catálogo de agentes. Solo con la clave de la Gemini API. No se usa desde apps ni
// APIs hasta que su canario pase con esa clave (el de la M0, con el CLI real y el hook que niega las herramientas, pasa).
var Ficha = adaptadores.Ficha{
	Nombre: "antigravity", NombreVisible: "Antigravity", Programa: "Antigravity CLI", Codigo: "AG",
	ColorClaro: "#db2777", ColorOscuro: "#f472b6", Orden: 40,
	Streaming: "tokens", ValoresEsfuerzo: valoresEsfuerzo,
	Autenticaciones: []string{adaptadores.AutenticacionClaveAPI},
	VariablesClave:  []string{"GEMINI_API_KEY"},
	Comando:         "agy", Concurrencia: 1,
	URLInstalacion: "https://antigravity.google",
	Crear: func(c *configuracion.Configuracion) adaptadores.Adaptador {
		return Antigravity{Programa: c.Texto("comandos_agentes.antigravity"), Autenticacion: c.Texto("adaptadores.antigravity.autenticacion")}
	},
}
