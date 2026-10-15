//go:build !windows

package actualizacion

import (
	"errors"

	"tramtai/internal/idioma"
)

// Instalada: fuera de Windows no hay instalador; la versión nueva se descarga desde la página de la versión.
func Instalada() (string, bool) { return "", false }

// Instalar solo existe en Windows.
func Instalar(string, string) error {
	return errors.New(idioma.T("la instalación automática solo existe en Windows"))
}
