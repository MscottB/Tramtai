// Package autoarranque registra la aplicación de escritorio para que arranque, oculta en la bandeja, al iniciar
// sesión en el sistema; así el servicio y la Web UI están disponibles sin abrir ninguna terminal.
package autoarranque

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"tramtai/internal/idioma"
)

// propia es la aplicación de escritorio de esta copia: el arranque solo cuenta como activo si es el de ella (la
// instalada y la portable no se pisan). Las pruebas la sustituyen.
var propia = Escritorio

// Escritorio devuelve la ruta de la aplicación de escritorio, que se instala junto al ejecutable actual.
func Escritorio() (string, error) {
	propio, err := os.Executable()
	if err != nil {
		return "", err
	}
	nombre := "tramtai-escritorio"
	if runtime.GOOS == "windows" {
		nombre += ".exe"
	}
	ruta := filepath.Join(filepath.Dir(propio), nombre)
	if _, err := os.Stat(ruta); err != nil {
		return "", errors.New(idioma.Tf("no se encontró %s junto a %s", nombre, filepath.Base(propio)))
	}
	return ruta, nil
}
