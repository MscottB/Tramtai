//go:build !windows

package credenciales

import (
	"errors"

	"tramtai/internal/idioma"
)

// shortcut: en macOS y Linux las claves siguen por variable de entorno; Llavero y Secret Service llegan con M8.
var errNoSoportado = errors.New(idioma.T("el almacén de credenciales solo está disponible en Windows; usa variables de entorno"))

// Disponible indica si el sistema tiene almacén de credenciales soportado.
func Disponible() bool { return false }

// Guardar no está disponible fuera de Windows.
func Guardar(string, string) error { return errNoSoportado }

// Leer no encuentra nada fuera de Windows.
func Leer(string) (string, error) { return "", nil }

// Borrar no está disponible fuera de Windows.
func Borrar(string) error { return errNoSoportado }
