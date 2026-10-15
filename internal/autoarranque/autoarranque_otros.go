//go:build !windows && !linux && !darwin

package autoarranque

import (
	"errors"

	"tramtai/internal/idioma"
)

// Disponible indica si este sistema admite el arranque automático.
const Disponible = false

// Activo indica si el arranque automático está registrado; en este sistema, nunca.
func Activo() (bool, error) { return false, nil }

// Activar no está disponible todavía en este sistema.
func Activar(string) error {
	return errors.New(idioma.T("el arranque automático todavía no está disponible en este sistema"))
}

// Desactivar no hace nada en este sistema.
func Desactivar() error { return nil }
