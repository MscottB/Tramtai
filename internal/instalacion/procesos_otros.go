//go:build !windows

package instalacion

import (
	"errors"

	"tramtai/internal/idioma"
)

// ProcesosEn solo existe en Windows.
func ProcesosEn(string, string) ([]uint32, error) {
	return nil, errors.New(idioma.T("solo disponible en Windows"))
}

// Cerrar solo existe en Windows.
func Cerrar(string, string) error { return errors.New(idioma.T("solo disponible en Windows")) }
