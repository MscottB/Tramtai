package idioma

// Traducciones al inglés de la terminal interactiva (internal/tui). Los nombres del menú y de las páginas no están
// aquí: salen de los de la Web UI (internal/tui/datos).
func init() {
	agregar(map[string]string{
		// Un valor que falta, en la terminal y en la CLI (como comun.sinDato en la Web UI)
		"Sin dato": "No data",

		// La app: tamaño, cabecera, estado y pie
		"Agranda la ventana (mínimo 60×20)": "Enlarge the window (minimum 60×20)",
		"Próximamente":                      "Coming soon",
		"¿Cancelar la petición en marcha?":  "Cancel the running request?",
		"Probar tiene una petición en marcha: seguirá en el servicio y la verás en el Historial.": "Playground has a running request: it will keep going in the service and you will see it in the History.",
		"La API %s tiene cambios sin guardar en Probar (g los guarda).":                           "The API %s has unsaved changes in the Playground (g saves them).",
		"¿Salir de la terminal?":                     "Quit the terminal?",
		"ver el error":                               "see the error",
		"Esc cierra":                                 "Esc closes",
		"Teclas":                                     "Keys",
		"conectando…":                                "connecting…",
		"sin servicio":                               "no service",
		"servicio en marcha":                         "service running",
		"Conectando con el servicio…":                "Connecting to the service…",
		"El servicio no responde · r reintenta":      "The service is not responding · r retries",
		"Cambiar de espacio":                         "Switch workspace",
		"el espacio %s se detuvo: vuelves al global": "workspace %s was stopped: back to the global one",
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

		// Agentes: los nombres, estados, etiquetas y confirmaciones salen de los textos de la Web UI (internal/tui/datos);
		// aquí solo lo adaptado a teclas.
		"detalles":      "details",
		"comprobar":     "check",
		"conexión":      "connection",
		"autenticación": "authentication",
		"filtro":        "filter",
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
		"Tus apps funcionan al encender el equipo": "Your apps work as soon as the computer starts",

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

		// Probar: la receta, el mensaje y la respuesta salen de los textos de la Web UI; aquí solo lo propio de la terminal.
		"edita":                  "edit",
		"campo":                  "field",
		"ver":                    "view",
		"copia":                  "copy",
		"copia la clave":         "copy the key",
		"cierra":                 "close",
		"completa":               "complete",
		"Escribe un mensaje":     "Write a message",
		"No hay nada que copiar": "There is nothing to copy",
		"Copiado con OSC 52; si no llega al portapapeles, tu terminal no lo admite":              "Copied with OSC 52; if it does not reach the clipboard, your terminal does not support it",
		"el conversor de archivos no está listo (%s): Ctrl+P lo prepara":                         "the file converter is not ready (%s): Ctrl+P prepares it",
		"Preparando el conversor de archivos (%s)":                                               "Preparing the file converter (%s)",
		"Escribe agente o agente/modelo (+esfuerzo), separados por comas: %s":                    "Write agent or agent/model (+effort), separated by commas: %s",
		"← → elige la plantilla y Enter la aplica":                                               "← → chooses the template and Enter applies it",
		"Enter salta de línea; Ctrl+Enter o Ctrl+S envía":                                        "Enter starts a new line; Ctrl+Enter or Ctrl+S sends",
		"Separa las rutas con comas; Tab completa la ruta.":                                      "Separate the paths with commas; Tab completes the path.",
		"No se pudo guardar el borrador del mensaje: %v":                                         "The message draft could not be saved: %v",
		"← → elige cómo se edita el esquema":                                                     "← → chooses how the schema is edited",
		"Separa las opciones con comas":                                                          "Separate the options with commas",
		"mueve el campo":                                                                         "move the field",
		"Enter abre o pliega las opciones de la receta":                                          "Enter opens or folds the recipe options",
		"n añade un campo detrás; x lo quita; Alt+↑↓ lo mueve; v enseña un ejemplo de respuesta": "n adds a field after this one; x removes it; Alt+↑↓ moves it; v shows a sample response",
		"← → elige cómo se edita el esquema; n añade un campo; v enseña un ejemplo de respuesta": "← → chooses how the schema is edited; n adds a field; v shows a sample response",
		"quita el campo": "remove the field",
	})
}
