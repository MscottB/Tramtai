package idioma

// Traducciones al inglés de la app de escritorio (escritorio).
func init() {
	agregar(map[string]string{
		"Tramtai no puede usar esta carpeta": "Tramtai cannot use this folder",
		"Tramtai portable":                   "Tramtai portable",
		"falta tramtai.exe junto a la aplicación portable: vuelve a descomprimir Tramtai": "tramtai.exe is missing next to the portable app: unzip Tramtai again",
		"Ver las %d restantes en Mis APIs…":                                               "See the other %d in My APIs…",
		"recién actualizada: muestra la ventana por delante de las demás":                 "just updated: shows the window in front of the others",
		"Buscar actualizaciones":                                                          "Check for updates",
		"raíz de un proyecto con .tramtai/config.yaml (por defecto, el espacio global)":   "root of a project with .tramtai/config.yaml (default: the global workspace)",
		"arranca solo en la bandeja, sin mostrar la ventana (arranque automático)":        "start only in the tray, without showing the window (automatic startup)",
		"Pasarela local de agentes":                                                       "Local agent gateway",

		// Avisos del sistema
		"Se detienen todos los espacios y las aplicaciones conectadas dejarán de funcionar. Tramtai sigue en la bandeja para volver a iniciarlo. ¿Continuar?": "All workspaces stop and connected apps will stop working. Tramtai stays in the tray so you can start it again. Continue?",
		"Al salir se detiene el servicio: las aplicaciones conectadas a tus APIs dejarán de funcionar hasta que vuelvas a abrir Tramtai. ¿Continuar?":         "Quitting stops the service: apps connected to your APIs will stop working until you open Tramtai again. Continue?",
		"Cancelar":             "Cancel",
		"Salir de Tramtai":     "Quit Tramtai",
		"Salir y detener":      "Quit and stop",
		"Detener el servicio":  "Stop the service",
		"Detener":              "Stop",
		"Carpeta del proyecto": "Project folder",

		// Bandeja
		"Servicio detenido":                 "Service stopped",
		"Servicio en marcha · %s":           "Service running · %s",
		"Servicio en marcha en %s":          "Service running at %s",
		"Abrir Tramtai":                     "Open Tramtai",
		"Mis APIs (%d de %d activas)":       "My APIs (%d of %d active)",
		" (con error)":                      " (error)",
		"Gestionar en Mis APIs":             "Manage in My APIs",
		"Crear una API…":                    "Create an API…",
		"Copiar la dirección para tus apps": "Copy the address for your apps",
		"Iniciar el servicio":               "Start the service",
		"Reiniciar el servicio":             "Restart the service",
		"Detener el servicio…":              "Stop the service…",
		"Iniciar al entrar en la sesión":    "Start when I sign in",
		"Iniciar con Windows":               "Start with Windows",
		"Salir":                             "Quit",
		"Salir y detener el servicio":       "Quit and stop the service",
		"Salir y detener el servicio…":      "Quit and stop the service…",

		// Página de error
		"no se encontró el ejecutable tramtai junto a la aplicación ni en el PATH":                        "the tramtai executable was not found next to the app or in the PATH",
		"No se pudo conectar con el servicio de Tramtai":                                                  "Could not connect to the Tramtai service",
		"Prueba <code>tramtai servicio iniciar -d</code> en una terminal y vuelve a abrir la aplicación.": "Try <code>tramtai servicio iniciar -d</code> in a terminal and open the app again.",
	})
}
