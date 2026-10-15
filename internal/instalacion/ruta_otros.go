//go:build !windows

package instalacion

import (
	"errors"

	"tramtai/internal/idioma"
)

// EditarRutaUsuario solo existe en Windows: en macOS y Linux el PATH lo gestiona el paquete o la terminal.
func EditarRutaUsuario(string, bool) error { return errors.New(idioma.T("solo disponible en Windows")) }
