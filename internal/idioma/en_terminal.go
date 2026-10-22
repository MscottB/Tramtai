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
		"↑↓ desplaza":              "↑↓ scrolls",
		"siguiente campo":          "next field",
		"envía":                    "sends",
		"cancela":                  "cancels",
		"Cargando…":                "Loading…",

		// Agentes: los nombres, estados, etiquetas y confirmaciones salen de los textos de la Web UI (internal/tui/datos);
		// aquí solo lo adaptado a teclas.
		"detalles":      "details",
		"comprobar":     "check",
		"conexión":      "connection",
		"autenticación": "authentication",
		"probar":        "try it",
		"Modelos":       "Models",
		"Elige cómo conectarlo: con tu suscripción (tecla a) o con una clave de API.":                                              "Choose how to connect it: with your subscription (key a) or with an API key.",
		"Para la clave de API, ejecuta esto en una terminal (pide la clave sin mostrarla) y pulsa c para volver a comprobar:":      "For the API key, run this in a terminal (it asks for the key without showing it) and press c to check again:",
		"Necesita una clave de API. Ejecuta esto en una terminal (pide la clave sin mostrarla) y pulsa c para volver a comprobar:": "Needs an API key. Run this in a terminal (it asks for the key without showing it) and press c to check again:",
		"Instala su CLI y pulsa c para volver a comprobar. Si está en otra ruta, indícala en comandos_agentes.%s.":                 "Install its CLI and press c to check again. If it is somewhere else, set it in comandos_agentes.%s.",
		"Pulsa s para confirmar:": "Press y to confirm:",
		"Instala su CLI antes: no hay nada que conectar ni desconectar":       "Install its CLI first: there is nothing to connect or disconnect",
		"%s no admite la suscripción: solo usa clave de API":                  "%s does not support the subscription: it only uses an API key",
		"Tiene que estar instalado y conectado para cambiar su autenticación": "It has to be installed and connected to change its authentication",
		"En su estado actual solo se puede usar la clave de API":              "In its current state only the API key can be used",
		"Este agente aún no está listo para probar":                           "This agent is not ready to try yet",

		// Inicio: los textos, contadores y pasos salen de los de la Web UI; aquí solo lo propio de la terminal.
		"desplaza": "scroll",
		"Sin conexión en vivo · reconectando…": "No live connection · reconnecting…",
		"hace %d s":          "%d s ago",
		"hace %d min":        "%d min ago",
		"hace %d h":          "%d h ago",
		"hace %d d":          "%d d ago",
		"Ver la Ayuda de %s": "See the %s Help",

		// Historial: los filtros, el detalle, el registro y las confirmaciones salen de los textos de la Web UI; aquí solo lo
		// propio de la terminal.
		"busca":                        "search",
		"aplica":                       "apply",
		"pestañas":                     "tabs",
		"reconectando…":                "reconnecting…",
		"en vivo":                      "live",
		"Petición enviada de nuevo":    "Request sent again",
		"Corrección enviada":           "Correction sent",
		"No hay peticiones terminadas": "There are no finished requests",
	})
}
