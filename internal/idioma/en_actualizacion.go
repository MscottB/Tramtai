package idioma

// Traducciones al inglés del aviso de actualizaciones y de tramtai actualizar.
func init() {
	agregar(map[string]string{
		"la última versión publicada no tiene un número válido: %q":                              "the latest published version has no valid number: %q",
		"esta versión no publicó instalador para Windows":                                        "this version did not publish a Windows installer",
		"dirección de descarga inesperada: %s":                                                   "unexpected download address: %s",
		"SHA256SUMS.txt no incluye %s":                                                           "SHA256SUMS.txt does not list %s",
		"la firma de esta versión no es válida: descárgala desde su página":                      "this version’s signature is not valid: download it from its page",
		"la suma SHA-256 de %s no coincide: la descarga está dañada o fue alterada":              "the SHA-256 checksum of %s does not match: the download is damaged or was tampered with",
		"GitHub respondió %s":                                                                    "GitHub answered %s",
		"la instalación automática solo existe en Windows":                                       "automatic installation is only available on Windows",
		"no se pudo consultar la última versión: %s":                                             "could not check the latest version: %s",
		"ya tienes la última versión (%s)":                                                       "you already have the latest version (%s)",
		"esta copia de Tramtai no se instaló con el instalador de Windows: descarga %s desde %s": "this copy of Tramtai was not installed with the Windows installer: download %s from %s",
		"no se pudo descargar la versión nueva: %s":                                              "could not download the new version: %s",
		"Comprueba si hay una versión nueva de Tramtai y la instala":                             "Checks for a new version of Tramtai and installs it",
		"Consulta en GitHub la última versión publicada. Si es más nueva, descarga el instalador, comprueba su suma " +
			"SHA-256 y lo ejecuta en silencio: la app se cierra, el servicio se detiene y ambos vuelven con la versión nueva, " +
			"con tu configuración y tus datos. La instalación automática solo funciona en una copia instalada con el instalador de Windows.": "Checks GitHub for the latest published version. If it is newer, it downloads the installer, verifies its " +
			"SHA-256 checksum and runs it silently: the app closes, the service stops and both come back on the new version, " +
			"with your settings and data. Automatic installation only works on a copy installed with the Windows installer.",
		"Ya tienes la última versión (%s).":      "You already have the latest version (%s).",
		"Hay una versión nueva: %s (tienes %s).": "A new version is available: %s (you have %s).",
		"Novedades: %s": "What’s new: %s",
		"Esta copia no se instaló con el instalador de Windows: descarga la versión nueva desde esa página.": "This copy was not installed with the Windows installer: download the new version from that page.",
		"¿Instalarla ahora? La app se cerrará y el servicio se detendrá unos segundos.":                      "Install it now? The app will close and the service will stop for a few seconds.",
		"Descargando el instalador…": "Downloading the installer…",
		"Suma SHA-256 correcta. Instalando: Tramtai se cerrará y volverá a abrirse.": "SHA-256 checksum verified. Installing: Tramtai will close and open again.",
		"solo comprueba si hay una versión nueva, sin instalarla":                    "only checks for a new version, without installing it",
		"Acerca de Tramtai": "About Tramtai",
		"Acerca de Tramtai · hay una versión nueva: %s": "About Tramtai · new version available: %s",
		"Instalar la versión %s…":                              "Install version %s…",
	})
}
