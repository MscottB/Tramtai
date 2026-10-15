// Package autoarranque registra la aplicación de escritorio para que arranque, oculta en la bandeja, al iniciar
// sesión en el sistema; así el servicio y la Web UI están disponibles sin abrir ninguna terminal.
package autoarranque

import (
	"errors"
	"os"
	"path/filepath"

	"tramtai/internal/idioma"
)

// Escritorio devuelve la ruta de la aplicación de escritorio, que se instala junto al ejecutable actual.
func Escritorio() (string, error) {
	propio, err := os.Executable()
	if err != nil {
		return "", err
	}
	ruta := filepath.Join(filepath.Dir(propio), "tramtai-escritorio.exe")
	if _, err := os.Stat(ruta); err != nil {
		return "", errors.New(idioma.Tf("no se encontró tramtai-escritorio.exe junto a %s", filepath.Base(propio)))
	}
	return ruta, nil
}
