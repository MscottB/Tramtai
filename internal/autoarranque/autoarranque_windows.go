//go:build windows

package autoarranque

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// Disponible indica si este sistema admite el arranque automático.
const Disponible = true

const claveRun = `Software\Microsoft\Windows\CurrentVersion\Run`

// nombreValor es el valor de la clave Run; las pruebas usan otro para no tocar el real.
var nombreValor = "Tramtai"

// Activo indica si el arranque automático está registrado para el usuario actual.
func Activo() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil // en un perfil nuevo la clave Run puede no existir todavía
	}
	if err != nil {
		return false, err
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(nombreValor); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Activar registra la aplicación de escritorio para que arranque oculta al iniciar sesión (solo este usuario).
func Activar(escritorio string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, claveRun, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(nombreValor, `"`+escritorio+`" --oculto`)
}

// Desactivar quita el registro; no es un error si no existía.
func Desactivar() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(nombreValor); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
