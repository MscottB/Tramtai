//go:build linux || darwin

package autoarranque

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Disponible indica si este sistema admite el arranque automático.
const Disponible = true

// En Linux y macOS el arranque automático es un archivo en la carpeta del usuario: archivo() dice dónde y
// contenido() qué escribe cada sistema.

// Activo indica si el arranque automático de esta copia está registrado para el usuario actual: el archivo existe y
// arranca su aplicación (no la de otra copia).
func Activo() (bool, error) {
	ruta, err := archivo()
	if err != nil {
		return false, err
	}
	datos, err := os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	escritorio, err := propia()
	return err == nil && (string(datos) == contenido(escritorio) || strings.Contains(string(datos), escritorio)), nil
}

// Activar registra la aplicación de escritorio para que arranque oculta al iniciar sesión (solo este usuario).
func Activar(escritorio string) error {
	ruta, err := archivo()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ruta, []byte(contenido(escritorio)), 0o644)
}

// Desactivar quita el registro de esta copia (nunca el de otra); no es un error si no existía.
func Desactivar() error {
	ruta, err := archivo()
	if err != nil {
		return err
	}
	if activo, err := Activo(); err != nil || !activo {
		return err
	}
	if err := os.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
