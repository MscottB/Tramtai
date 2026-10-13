//go:build !windows

package autoarranque

import "errors"

// Disponible indica si este sistema admite el arranque automático.
const Disponible = false

// Activo indica si el arranque automático está registrado; fuera de Windows, nunca.
func Activo() (bool, error) { return false, nil }

// Activar no está disponible fuera de Windows todavía.
func Activar(string) error {
	return errors.New("el arranque automático solo está disponible en Windows por ahora")
}

// Desactivar no hace nada fuera de Windows.
func Desactivar() error { return nil }
