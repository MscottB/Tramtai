package idioma

// Traducciones al inglés de los errores de internal/apuntar (apuntar otras herramientas a Tramtai).
func init() {
	agregar(map[string]string{
		"archivo ilegible":                       "unreadable file",
		"el archivo cambió mientras se escribía": "the file changed while it was being written",
		"no hay copia: el archivo no existía antes de activar la herramienta":                     "there is no copy: the file did not exist before the tool was turned on",
		"la herramienta ya está activada en otro archivo: desactívala antes":                      "the tool is already turned on in another file: turn it off first",
		"falta la orden que entrega la clave":                                                     "the command that provides the key is missing",
		"falta la clave del espacio":                                                              "the workspace key is missing",
		"la ruta de Tramtai lleva un «%»: cmd.exe la expandiría; instálalo en otra carpeta":       "the Tramtai path contains a \"%\": cmd.exe would expand it; install it in another folder",
		"falta la carpeta del proyecto":                                                           "the project folder is missing",
		"ya hay un proveedor «tramtai» en config.toml que no es de Tramtai: renómbralo o quítalo": "config.toml already has a \"tramtai\" provider that is not Tramtai's: rename or remove it",
		"«model» o «model_provider» de config.toml ocupa varias líneas: déjalo en una sola":       "\"model\" or \"model_provider\" in config.toml spans several lines: put it on a single line",
	})
	agregar(map[string]string{
		"ya hay un modelo «Tramtai» en config.yaml que no es de Tramtai: renómbralo o quítalo": "config.yaml already has a “Tramtai” model that is not Tramtai’s: rename or remove it",
		"«models» de config.yaml está escrito entre corchetes: pásalo a una lista con guiones": "“models” in config.yaml is written in square brackets: change it to a dash list",
	})
}
