package idioma

// Traducciones al inglés de la terminal interactiva (internal/tui). Los nombres del menú y de las páginas no están
// aquí: salen de los de la Web UI (internal/tui/datos).
func init() {
	agregar(map[string]string{
		// Un valor que falta, en la terminal y en la CLI (como comun.sinDato en la Web UI)
		"Sin dato": "No data",

		// La app: tamaño, cabecera, estado y pie
		"Agranda la ventana (mínimo 60×20)": "Enlarge the window (minimum 60×20)",
		"¿Cancelar la petición en marcha?":  "Cancel the running request?",
		"Enviando…":                         "Sending…",
		"Probar tiene una petición en marcha: seguirá en el servicio y la verás en el Historial.": "Playground has a running request: it will keep going in the service and you will see it in the History.",
		"La API %s tiene cambios sin guardar en Probar (g los guarda).":                           "The API %s has unsaved changes in the Playground (g saves them).",
		"¿Salir de la terminal?":                "Quit the terminal?",
		"¿Cambiar de espacio?":                  "Switch workspace?",
		"ver el error":                          "see the error",
		"Esc cierra":                            "Esc closes",
		"Teclas":                                "Keys",
		"conectando…":                           "connecting…",
		"Conectando con el servicio…":           "Connecting to the service…",
		"El servicio no responde · r reintenta": "The service is not responding · r retries",
		"Sigue en marcha el servicio de una versión anterior · r lo detiene y arranca el nuevo": "A service from an earlier version is still running · r stops it and starts the new one",
		"No se pudo detener el servicio de la versión anterior · ! dice cómo":                   "Couldn’t stop the earlier version’s service · ! shows how",
		"el espacio %s se detuvo: vuelves al global":                                            "workspace %s was stopped: back to the global one",
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

		// Mis APIs
		"filtros":                             "filters",
		"Filtros y orden":                     "Filters and order",
		"activa o detiene el grupo":           "start or stop the group",
		"Escribe un número, por ejemplo 2.50": "Type a number, for example 2.50",
		"O escribe agente/modelo":             "Or type agent/model",
		"El catálogo de modelos está vacío: escribe el modelo a mano.":                                    "The model catalog is empty: type the model by hand.",
		"No llegó el catálogo de modelos (%s): escribe el modelo a mano; r lo vuelve a pedir.":            "The model catalog did not arrive (%s): type the model by hand; r asks for it again.",
		"El respaldo, la respuesta con forma y las correcciones se editan en Probar: p desde el detalle.": "The fallback, the shaped response and the corrections are edited in the Playground: p from the details.",

		// Espacios
		"Nombre del espacio":    "Workspace name",
		"Escribe «%s» tal cual": "Type “%s” exactly",

		// Ajustes: las pestañas, las secciones y las novedades salen de los textos de la Web UI; aquí solo lo propio de la
		// terminal.
		"activa":                             "turn on",
		"desactiva":                          "turn off",
		"Elige qué borrar":                   "Choose what to delete",
		"se aplica al reiniciar el servicio": "it applies when the service restarts",
		"Respuestas %d h · registro %s días · máximo %s":                                                 "Responses %d h · log %s days · up to %s",
		"¿Cancelar la actualización?":                                                                    "Cancel the update?",
		"La descarga se borra y Tramtai sigue como está.":                                                "The download is deleted and Tramtai stays as it is.",
		"El servicio se reinicia; la terminal se vuelve a conectar sola":                                 "The service is restarting; the terminal reconnects on its own",
		"Tramtai se está actualizando: vuelve a abrir la terminal con «tramtai terminal» cuando termine": "Tramtai is updating: open the terminal again with “tramtai terminal” when it finishes",

		// La Ayuda: los temas salen de los de la Web UI; aquí solo lo propio de la terminal.
		"Ver en la web":           "See on the web",
		"copia la dirección":      "copy the address",
		"quita la búsqueda":       "clear the search",
		"Abierto en el navegador": "Opened in the browser",
		"No se pudo abrir el navegador; la tecla y copia la dirección:": "Could not open the browser; the y key copies the address:",
		"Si no se abrió el navegador, la tecla y copia la dirección:":   "If the browser did not open, the y key copies the address:",
		"No se pudo guardar la aceptación: %s":                          "Could not save your acceptance: %s",
	})
}
