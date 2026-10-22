package gemini

import (
	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// Ficha es la de la API de Gemini (sin CLI: Tramtai la llama directamente) para el catálogo de agentes.
var Ficha = adaptadores.Ficha{
	Nombre: "gemini", NombreVisible: "Gemini API", Programa: "Gemini API", Codigo: "GM",
	ColorClaro: "#7c3aed", ColorOscuro: "#a78bfa", Orden: 30,
	Streaming: "tokens", SalidaEstructurada: true, ModeloEfectivoInformado: true,
	Autenticaciones: []string{adaptadores.AutenticacionClaveAPI},
	VariablesClave:  []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
	Concurrencia:    2, PermitidoEnApps: true,
	URLInstalacion: "https://aistudio.google.com/apikey",
	Crear:          func(*configuracion.Configuracion) adaptadores.Adaptador { return Gemini{} },
}
