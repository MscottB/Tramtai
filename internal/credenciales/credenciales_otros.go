//go:build !windows && !darwin && !linux

package credenciales

import (
	"errors"

	"tramtai/internal/idioma"
)

// En otros sistemas no hay almacén soportado: las claves van por variable de entorno.
var errNoSoportado = errors.New(idioma.T("este sistema no tiene almacén de credenciales soportado; usa variables de entorno"))

// Disponible indica si el sistema tiene almacén de credenciales soportado.
func Disponible() bool { return false }

// Almacen es el nombre del almacén; vacío si no hay ninguno.
func Almacen() string { return "" }

// Guardar no está disponible en este sistema.
func Guardar(string, string) error { return errNoSoportado }

// Leer no encuentra nada en este sistema.
func Leer(string) (string, error) { return "", nil }

// Borrar no está disponible en este sistema.
func Borrar(string) error { return errNoSoportado }
