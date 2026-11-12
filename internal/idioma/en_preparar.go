package idioma

// Traducciones al inglés de la preparación de los agentes (internal/preparar): instalar, actualizar y desinstalar.
func init() {
	agregar(map[string]string{
		"agente desconocido o sin receta: %s":                  "unknown agent or agent without a recipe: %s",
		"acción no disponible para %s en este sistema: %s":     "action not available for %s on this system: %s",
		"ya hay una preparación de %s en marcha":               "%s is already being set up",
		"%s tiene peticiones en curso: espera a que terminen":  "%s has requests in progress: wait for them to finish",
		"el servicio se reinició a mitad de la preparación":    "the service restarted in the middle of the setup",
		"la preparación superó su plazo de %s y se cerró":      "the setup took longer than its %s limit and was closed",
		"%s %s aún no está comprobado con Tramtai":             "%s %s has not been checked with Tramtai yet",
		"no se pudo consultar la última versión de %s":         "could not check the latest version of %s",
		"el instalador terminó, pero no se encuentra %s":       "the installer finished, but %s cannot be found",
		"no se pudo guardar la ruta de %s: %s":                 "could not save the path of %s: %s",
		"%s está fuera de la carpeta del usuario: no se borra": "%s is outside the user folder: it is not deleted",
		"no se pudo borrar %s: %s":                             "could not delete %s: %s",
		"%s terminó con el código %d":                          "%s finished with exit code %d",
		"no se encuentra %s":                                   "%s cannot be found",
		"solo se descarga por HTTPS: %s":                       "downloads only go over HTTPS: %s",
		"demasiadas redirecciones":                             "too many redirects",
		"no hay descarga para la arquitectura %s":              "there is no download for the %s architecture",
		"%s no está en la lista de sumas de la versión":        "%s is not in the version's checksum list",
		"la suma SHA-256 de %s no coincide con la publicada":   "the SHA-256 checksum of %s does not match the published one",
		"no se pudo descargar %s: %s":                          "could not download %s: %s",
		"%s respondió %d":                                      "%s answered %d",
		"%s es más grande de lo esperado":                      "%s is larger than expected",
		"la descarga no trae %s":                               "the download does not contain %s",
	})
}
