package antigravity

import (
	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// receta es la de Antigravity (m0.md, «Antigravity»): sin inicio de sesión (va con clave de API) ni versión fijable, el
// manifiesto da siempre la última. agy install añade su carpeta al PATH (registro o archivos de arranque), que el servicio
// no ve: se encuentra por Rutas. Se actualiza solo en segundo plano. Desinstalar no tiene orden: se borra lo de Borrar
// (la entrada del PATH queda).
var receta = adaptadores.Receta{
	Instalar: map[string]adaptadores.OrdenReceta{
		"windows": {Programa: "powershell", Argumentos: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "irm https://antigravity.google/cli/install.ps1 | iex"}},
		"darwin":  {Programa: "sh", Argumentos: []string{"-c", "curl -fsSL https://antigravity.google/cli/install.sh | bash"}},
		"linux":   {Programa: "sh", Argumentos: []string{"-c", "curl -fsSL https://antigravity.google/cli/install.sh | bash"}},
	},
	Actualizar: map[string]adaptadores.OrdenReceta{
		"windows": {Programa: "agy", Argumentos: []string{"update"}},
		"darwin":  {Programa: "agy", Argumentos: []string{"update"}},
		"linux":   {Programa: "agy", Argumentos: []string{"update"}},
	},
	Borrar: map[string][]string{
		"windows": {"~/AppData/Local/agy/bin/agy.exe"},
		"darwin":  {"~/.local/bin/agy"},
		"linux":   {"~/.local/bin/agy"},
	},
	Rutas: map[string][]string{
		"windows": {"~/AppData/Local/agy/bin/agy.exe"},
		"darwin":  {"~/.local/bin/agy"},
		"linux":   {"~/.local/bin/agy"},
	},
	DatosCLI: []string{"~/.gemini/antigravity-cli"},
	// Frágil: es el manifiesto que usa el instalador oficial, un Cloud Run con número de proyecto y sin documentar (el
	// DOWNLOAD_BASE_URL de install.sh e install.ps1). Todas las plataformas dan la misma versión. El job de la Tarea 13
	// baja los dos guiones y avisa con un issue si su DOWNLOAD_BASE_URL deja de ser este.
	Ultima: adaptadores.FuenteVersion{Tipo: "url", Nombre: "https://antigravity-cli-auto-updater-974169037036.us-central1.run.app/manifests/windows_amd64.json"},
	Comprobadas: map[string][]string{
		"instalar":    {"1.5.7"},
		"actualizar":  {"1.5.7"},
		"desinstalar": {"1.5.7"},
	},
}

// Ficha es la de Antigravity (agy) para el catálogo de agentes. Solo con la clave de la Gemini API. No se usa desde apps ni
// APIs hasta que su canario pase con esa clave (el canario con el CLI real y el hook que niega las herramientas ya pasa).
var Ficha = adaptadores.Ficha{
	Nombre: "antigravity", NombreVisible: "Antigravity", Programa: "Antigravity CLI", Codigo: "AG",
	ColorClaro: "#db2777", ColorOscuro: "#f472b6", Orden: 40,
	Streaming: "tokens", ValoresEsfuerzo: valoresEsfuerzo,
	Autenticaciones: []string{adaptadores.AutenticacionClaveAPI},
	VariablesClave:  []string{"GEMINI_API_KEY"},
	Comando:         "agy", Concurrencia: 1,
	URLInstalacion: "https://antigravity.google",
	Receta:         &receta,
	Crear: func(c *configuracion.Configuracion) adaptadores.Adaptador {
		return Antigravity{Programa: c.Texto("comandos_agentes.antigravity"), Autenticacion: c.Texto("adaptadores.antigravity.autenticacion")}
	},
}
