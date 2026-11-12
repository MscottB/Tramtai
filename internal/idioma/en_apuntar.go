package idioma

// Traducciones al inglés de los errores de internal/apuntar (apuntar otras herramientas a Tramtai).
func init() {
	agregar(map[string]string{
		"archivo ilegible":                       "unreadable file",
		"el archivo cambió mientras se escribía": "the file changed while it was being written",
		"no hay copia: el archivo no existía antes de activar la herramienta": "there is no copy: the file did not exist before the tool was turned on",
		"la herramienta ya está activada en otro archivo: desactívala antes":  "the tool is already turned on in another file: turn it off first",
	})
}
