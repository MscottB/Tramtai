package idioma

// Traducciones al inglés de la preparación de los agentes (internal/preparar): instalar, actualizar y desinstalar.
func init() {
	agregar(map[string]string{
		"agente desconocido o sin receta: %s":                       "unknown agent or agent without a recipe: %s",
		"acción no disponible para %s en este sistema: %s":          "action not available for %s on this system: %s",
		"ya hay una preparación de %s en marcha":                    "%s is already being set up",
		"%s tiene peticiones en curso: espera a que terminen":       "%s has requests in progress: wait for them to finish",
		"el servicio se reinició a mitad de la preparación":         "the service restarted in the middle of the setup",
		"la preparación superó su plazo de %d min y se cerró":       "the setup took longer than its %d min limit and was closed",
		"%s es un enlace: se quitó el enlace, no lo que hay detrás": "%s is a link: the link was removed, not what it points to",
		"la carpeta del usuario (%s) no es válida para borrar nada": "the user folder (%s) is not valid for deleting anything",
		"versión no válida: %q":                                     "invalid version: %q",
		"%s no es una ruta de la casa que se pueda borrar":          "%s is not a home path that can be deleted",
		"%s es una carpeta compartida: no se borra":                 "%s is a shared folder: it is not deleted",
		"%s pasa por un enlace: no se borra":                        "%s goes through a link: it is not deleted",
		"%s aún no está comprobado con Tramtai":                     "%s has not been checked with Tramtai yet",
		"no se pudo consultar la última versión de %s":              "could not check the latest version of %s",
		"el instalador terminó, pero no se encuentra %s":            "the installer finished, but %s cannot be found",
		"no se pudo guardar la ruta de %s: %s":                      "could not save the path of %s: %s",
		"%s está fuera de la carpeta del usuario: no se borra":      "%s is outside the user folder: it is not deleted",
		"no se pudo borrar %s: %s":                                  "could not delete %s: %s",
		"%s terminó con el código %d":                               "%s finished with exit code %d",
		"no se encuentra %s":                                        "%s cannot be found",
		"solo se descarga por HTTPS: %s":                            "downloads only go over HTTPS: %s",
		"demasiadas redirecciones":                                  "too many redirects",
		"no hay descarga para la arquitectura %s":                   "there is no download for the %s architecture",
		"%s no está en la lista de sumas de la versión":             "%s is not in the version’s checksum list",
		"la suma SHA-256 de %s no coincide con la publicada":        "the SHA-256 checksum of %s does not match the published one",
		"no se pudo descargar %s: %s":                               "could not download %s: %s",
		"%s respondió %d":                                           "%s answered %d",
		"%s es más grande de lo esperado":                           "%s is larger than expected",
		"la descarga no trae %s":                                    "the download does not contain %s",
	})
	// El inicio de sesión (Tarea 4).
	agregar(map[string]string{
		"%s no inicia sesión desde Tramtai: usa su clave de API": "%s does not sign in from Tramtai: use its API key",
		"%s aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, puedes hacerlo en una consola con %s": "%s has not been checked with Tramtai yet: automatic sign-in comes back once it is. Meanwhile, you can sign in from a console with %s",
		"no se reconoció lo que escribió %s en %d s: inicia sesión en una consola con %s":                                                             "what %s wrote in %d s was not recognized: sign in from a console with %s",
		"el inicio de sesión de %s falló (terminó con el código %d): vuelve a intentarlo o hazlo en una consola con %s":                               "the %s sign-in failed (it exited with code %d): try again or sign in from a console with %s",
		"%s terminó sin dejar la sesión iniciada":                                           "%s finished without leaving you signed in",
		"el código pegado no es válido: una sola línea sin espacios, de %d a %d caracteres": "the pasted code is not valid: a single line without spaces, %d to %d characters",
		"%s no está esperando ningún código":                                                "%s is not waiting for a code",
		"no se pudo pasar el código a %s":                                                   "could not pass the code to %s",
	})
	// Las versiones comprobadas (Tarea 5).
	agregar(map[string]string{
		"%s aún no está comprobado con Tramtai: si sigues, el inicio de sesión automático quedará desactivado hasta que lo esté": "%s has not been checked with Tramtai yet: if you go ahead, automatic sign-in will stay off until it is",
	})
}
