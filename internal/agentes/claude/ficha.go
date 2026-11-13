package claude

import (
	"regexp"

	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// valoresEsfuerzo son los de --effort de Claude Code, que llega a max.
var valoresEsfuerzo = []string{"low", "medium", "high", "xhigh", "max"}

// receta es la de Claude Code (m0.md, «Claude Code»). Desinstalar no tiene orden propia: se borra lo de Borrar. El PATH no
// lo toca el instalador, así que el CLI se encuentra por Rutas. Se actualiza solo en segundo plano.
var receta = adaptadores.Receta{
	Instalar: map[string]adaptadores.OrdenReceta{
		// Sin versión, el argumento "{version}" se omite y el instalador toma la última.
		"windows": {Programa: "powershell", Argumentos: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
			"& { & ([scriptblock]::Create((irm https://claude.ai/install.ps1))) @args }", "{version}"}},
		"darwin": {Programa: "sh", Argumentos: []string{"-c", `curl -fsSL https://claude.ai/install.sh | bash -s -- "$@"`, "sh", "{version}"}},
		"linux":  {Programa: "sh", Argumentos: []string{"-c", `curl -fsSL https://claude.ai/install.sh | bash -s -- "$@"`, "sh", "{version}"}},
	},
	Actualizar: map[string]adaptadores.OrdenReceta{
		"windows": {Programa: "claude", Argumentos: []string{"update"}},
		"darwin":  {Programa: "claude", Argumentos: []string{"update"}},
		"linux":   {Programa: "claude", Argumentos: []string{"update"}},
	},
	Borrar: map[string][]string{
		"windows": {"~/.local/bin/claude.exe", "~/.local/share/claude"},
		"darwin":  {"~/.local/bin/claude", "~/.local/share/claude"},
		"linux":   {"~/.local/bin/claude", "~/.local/share/claude"},
	},
	Rutas: map[string][]string{
		"windows": {"~/.local/bin/claude.exe"},
		"darwin":  {"~/.local/bin/claude"},
		"linux":   {"~/.local/bin/claude"},
	},
	DatosCLI:    []string{"~/.claude", "~/.claude.json"},
	FijaVersion: true,
	Ultima:      adaptadores.FuenteVersion{Tipo: "url", Nombre: "https://downloads.claude.ai/claude-code-releases/latest"},
	Sesion: &adaptadores.LectorSesion{
		Argumentos: []string{"auth", "login"},
		Enlace:     regexp.MustCompile(`visit: (https://\S+)`),
		PidePegar:  regexp.MustCompile(`Paste code here if prompted >`),
	},
	Comprobadas: map[string][]string{
		"instalar":       {"2.1.311", "2.1.323"},
		"actualizar":     {"2.1.311", "2.1.323"},
		"iniciar_sesion": {"2.1.311", "2.1.323"},
		"desinstalar":    {"2.1.311", "2.1.323"},
	},
}

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
	InicioSesion:   receta.Sesion.Argumentos, URLTerminos: "https://code.claude.com/docs/en/legal-and-compliance",
	Receta: &receta,
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

// ModoAutenticacion: la suscripción se cambia en marcha.
func (c Claude) ModoAutenticacion() *adaptadores.Modo { return c.Autenticacion }
