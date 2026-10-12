// Package recursos incorpora la Web UI compilada en el ejecutable (sección 21): web/ se compila con
// "npm run build" en internal/recursos/web y se versiona, para que go build no necesite Node.
package recursos

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var web embed.FS

// Web devuelve los archivos de la Web UI.
func Web() fs.FS {
	sub, err := fs.Sub(web, "web")
	if err != nil {
		panic(err) // el directorio está incorporado en tiempo de compilación
	}
	return sub
}
