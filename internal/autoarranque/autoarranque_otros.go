//go:build !windows && !linux && !darwin

package autoarranque

import (
	"errors"

	"tramtai/internal/idioma"
)

// Disponible indica si este sistema admite el arranque automático.
const Disponible = false

// Configurado dice en qué modo está registrado el arranque automático; en este sistema, en ninguno.
func Configurado() (Modo, error) { return "", nil }

// activar no está disponible todavía en este sistema.
func activar(string, Modo) error {
	return errors.New(idioma.T("el arranque automático todavía no está disponible en este sistema"))
}

// Desactivar no hace nada en este sistema.
func Desactivar() error { return nil }
