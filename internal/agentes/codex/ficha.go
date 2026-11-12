package codex

import (
	"regexp"

	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// valoresEsfuerzo son los de model_reasoning_effort de Codex, que llega a xhigh.
var valoresEsfuerzo = []string{"low", "medium", "high", "xhigh"}

// receta es la de Codex (m0.md, «Codex»). Sus instaladores cambian el PATH de usuario (Windows) o los archivos de arranque
// (Unix), que el servicio en marcha no ve: se encuentra por Rutas. Desinstalar no tiene orden: se borra lo de Borrar y
// después DatosCLI (que incluye el paquete instalado, ~/.codex/packages).
var receta = adaptadores.Receta{
	Instalar: map[string]adaptadores.OrdenReceta{
		"windows": {Programa: "powershell", Argumentos: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "irm https://chatgpt.com/codex/install.ps1 | iex"},
			Entorno: []string{"CODEX_NON_INTERACTIVE=1", "CODEX_RELEASE={version}"}},
		"darwin": {Programa: "sh", Argumentos: []string{"-c", "curl -fsSL https://chatgpt.com/codex/install.sh | sh"},
			Entorno: []string{"CODEX_NON_INTERACTIVE=1", "CODEX_RELEASE={version}"}},
		"linux": {Programa: "sh", Argumentos: []string{"-c", "curl -fsSL https://chatgpt.com/codex/install.sh | sh"},
			Entorno: []string{"CODEX_NON_INTERACTIVE=1", "CODEX_RELEASE={version}"}},
	},
	// codex update vuelve a ejecutar el instalador y lee CODEX_RELEASE: si sigue puesta, «actualiza» a la misma.
	Actualizar: map[string]adaptadores.OrdenReceta{
		"windows": {Programa: "codex", Argumentos: []string{"update"}, Entorno: []string{"CODEX_NON_INTERACTIVE=1", "CODEX_RELEASE="}},
		"darwin":  {Programa: "codex", Argumentos: []string{"update"}, Entorno: []string{"CODEX_NON_INTERACTIVE=1", "CODEX_RELEASE="}},
		"linux":   {Programa: "codex", Argumentos: []string{"update"}, Entorno: []string{"CODEX_NON_INTERACTIVE=1", "CODEX_RELEASE="}},
	},
	Borrar: map[string][]string{
		"windows": {"~/AppData/Local/Programs/OpenAI/Codex/bin", "~/.codex/packages/standalone"},
		"darwin":  {"~/.local/bin/codex", "~/.codex/packages/standalone"},
		"linux":   {"~/.local/bin/codex", "~/.codex/packages/standalone"},
	},
	Rutas: map[string][]string{
		"windows": {"~/AppData/Local/Programs/OpenAI/Codex/bin/codex.exe", "~/.codex/packages/standalone/current/bin/codex.exe"},
		"darwin":  {"~/.local/bin/codex"},
		"linux":   {"~/.local/bin/codex"},
	},
	DatosCLI:    []string{"~/.codex"},
	FijaVersion: true,
	// El tag_name de GitHub viene como «rust-v0.167.2».
	Ultima: adaptadores.FuenteVersion{Tipo: "github", Nombre: "openai/codex", Prefijo: "rust-v"},
	// Siempre por código de dispositivo: no abre el navegador (el usuario debe activarlo en ChatGPT, Seguridad).
	Sesion: &adaptadores.LectorSesion{
		Argumentos: []string{"login", "--device-auth"},
		Enlace:     regexp.MustCompile(`(https://auth\.openai\.com/codex/device)`),
		Codigo:     regexp.MustCompile(`one-time code[^\n]*\n\s*([A-Z0-9]{4}-[A-Z0-9]{5})\b`),
	},
	Comprobadas: map[string][]string{
		"instalar":       {"0.167.1", "0.167.2"},
		"actualizar":     {"0.167.1", "0.167.2"},
		"iniciar_sesion": {"0.167.1", "0.167.2"},
		"desinstalar":    {"0.167.1", "0.167.2"},
	},
}

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
	InicioSesion: receta.Sesion.Argumentos, URLTerminos: "https://openai.com/policies/",
	Receta: &receta,
	Crear: func(c *configuracion.Configuracion) adaptadores.Adaptador {
		return Codex{Programa: c.Texto("comandos_agentes.codex"), Autenticacion: adaptadores.NuevoModo(c.Texto("adaptadores.codex.autenticacion"))}
	},
}

// ModoAutenticacion: la suscripción se cambia en marcha.
func (c Codex) ModoAutenticacion() *adaptadores.Modo { return c.Autenticacion }
