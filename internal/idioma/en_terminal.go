package idioma

// Traducciones al inglés de la terminal interactiva (internal/tui). Los nombres del menú y de las páginas no están
// aquí: salen de los de la Web UI (internal/tui/datos).
func init() {
	agregar(map[string]string{
		// La app: tamaño, cabecera, estado y pie
		"Agranda la ventana (mínimo 60×20)":          "Enlarge the window (minimum 60×20)",
		"Próximamente":                               "Coming soon",
		"conectando…":                                "connecting…",
		"sin servicio":                               "no service",
		"servicio en marcha":                         "service running",
		"Conectando con el servicio…":                "Connecting to the service…",
		"El servicio no responde · r reintenta":      "The service is not responding · r retries",
		"Cambiar de espacio":                         "Switch workspace",
		"el espacio %s se detuvo: vuelves al global": "workspace %s was stopped: back to the global one",
		"Intro":   "Enter",
		"elige":   "choose",
		"vuelve":  "back",
		"menú":    "menu",
		"ayuda":   "help",
		"espacio": "workspace",
		"salir":   "quit",

		// Componentes
		"Sin resultados":           "No results",
		"Escribe para filtrar":     "Type to filter",
		"s confirma · Esc cancela": "y confirms · Esc cancels",
		"siguiente campo":          "next field",
		"envía":                    "sends",
		"cancela":                  "cancels",
	})
}
