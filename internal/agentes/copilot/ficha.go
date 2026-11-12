package copilot

import (
	"regexp"

	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// valoresEsfuerzo son los de --reasoning-effort de Copilot CLI, que llega a max; con el modelo auto no se envían.
var valoresEsfuerzo = []string{"low", "medium", "high", "xhigh", "max"}

// descargaWindows es la descarga directa que GitHub documenta (el zip trae solo copilot.exe), con el SHA256SUMS.txt de la
// misma versión. Hay copilot-win32-arm64.zip para ARM, que esta receta aún no distingue.
var descargaWindows = adaptadores.DescargaVerificada{
	URL:      "https://github.com/github/copilot-cli/releases/download/v{version}/copilot-win32-x64.zip",
	URLSumas: "https://github.com/github/copilot-cli/releases/download/v{version}/SHA256SUMS.txt",
}

// instalarUnix es el instalador oficial; sin terminal solo avisa del PATH, y VERSION se omite si no se fija versión.
var instalarUnix = adaptadores.OrdenReceta{Programa: "sh", Argumentos: []string{"-c", "curl -fsSL https://gh.io/copilot-install | bash"},
	Entorno: []string{"VERSION={version}"}}

var actualizar = adaptadores.OrdenReceta{Programa: "copilot", Argumentos: []string{"update"}}

// receta es la de Copilot CLI (m0.md, «GitHub Copilot»). En Windows no usa winget (no está para la cuenta de un servicio ni
// sin administrador): Tramtai baja el zip verificado a la carpeta donde winget deja el CLI, así copilot update y una
// instalación con winget conviven; no toca el PATH, por eso se encuentra por Rutas. Se actualiza solo al arrancar.
var receta = adaptadores.Receta{
	// Windows no tiene entrada en Instalar: va por Descarga.
	Instalar:   map[string]adaptadores.OrdenReceta{"darwin": instalarUnix, "linux": instalarUnix},
	Actualizar: map[string]adaptadores.OrdenReceta{"windows": actualizar, "darwin": actualizar, "linux": actualizar},
	Descarga:   map[string]adaptadores.DescargaVerificada{"windows": descargaWindows},
	Borrar: map[string][]string{
		"windows": {"~/AppData/Local/GitHubCopilotCLI"},
		"darwin":  {"~/.local/bin/copilot"},
		"linux":   {"~/.local/bin/copilot"},
	},
	Rutas: map[string][]string{
		"windows": {"~/AppData/Local/GitHubCopilotCLI/copilot.exe"},
		"darwin":  {"~/.local/bin/copilot", "/usr/local/bin/copilot"},
		"linux":   {"~/.local/bin/copilot", "/usr/local/bin/copilot"},
	},
	DatosCLI:    []string{"~/.copilot"},
	FijaVersion: true,
	Ultima:      adaptadores.FuenteVersion{Tipo: "github", Nombre: "github/copilot-cli", Prefijo: "v"},
	// Con la sola orden login abre el navegador y espera en localhost; --device-code da enlace y código. Aun así copia
	// el código y abre el navegador por su cuenta: COPILOT_DEBUG_BROWSER lo sustituye por un programa que no existe.
	Sesion: &adaptadores.LectorSesion{
		Argumentos: []string{"login", "--device-code"},
		Entorno:    []string{`COPILOT_DEBUG_BROWSER=["tramtai-no-abrir-navegador"]`},
		Enlace:     regexp.MustCompile(`visit (https://github\.com/login/device)`),
		Codigo:     regexp.MustCompile(`enter code ([A-Z0-9]{4}-[A-Z0-9]{4})\b`),
	},
	Comprobadas: map[string][]string{
		"instalar":       {"1.0.108", "1.0.109"},
		"actualizar":     {"1.0.108", "1.0.109"},
		"iniciar_sesion": {"1.0.108", "1.0.109"},
		"desinstalar":    {"1.0.108", "1.0.109"},
	},
}

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
	InicioSesion:   receta.Sesion.Argumentos, URLTerminos: "https://docs.github.com/en/site-policy/github-terms/github-terms-of-service",
	Receta: &receta,
	Crear: func(c *configuracion.Configuracion) adaptadores.Adaptador {
		return Copilot{Programa: c.Texto("comandos_agentes.copilot"), Autenticacion: adaptadores.NuevoModo(c.Texto("adaptadores.copilot.autenticacion"))}
	},
}

// ModoAutenticacion: la suscripción se cambia en marcha.
func (c Copilot) ModoAutenticacion() *adaptadores.Modo { return c.Autenticacion }
