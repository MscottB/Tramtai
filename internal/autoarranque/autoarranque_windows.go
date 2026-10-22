//go:build windows

package autoarranque

import (
	"errors"
	"os"
	"strings"
	"tramtai/internal/carpetas"

	"golang.org/x/sys/windows/registry"
)

// Disponible indica si este sistema admite el arranque automático.
const Disponible = true

const claveRun = `Software\Microsoft\Windows\CurrentVersion\Run`

// Valores de la clave Run: uno para la instalada y otro para la portable, así que activar una no toca la otra. Las
// portables anteriores a la 0.48 usaban el de la instalada: en la portable también se reconoce, y al activarla se
// mueve al suyo.
// Las pruebas usan otros nombres para no tocar los reales.
var (
	nombreValor    = "Tramtai"
	nombrePortable = "Tramtai portable"
	esPortable     = carpetas.Portable
)

// nombres son los valores que pueden ser de esta copia, el propio primero.
func nombres() []string {
	if esPortable() {
		return []string{nombrePortable, nombreValor}
	}
	return []string{nombreValor}
}

// orden es lo que Activar escribe para la aplicación.
func orden(escritorio string) string { return `"` + escritorio + `" --oculto` }

// deEstaCopia dice si un valor de la clave Run arranca la aplicación de esta copia (la misma ruta, sin distinguir
// mayúsculas, o el mismo archivo).
func deEstaCopia(valor string) bool {
	ruta, err := propia()
	if err != nil {
		return false
	}
	if strings.EqualFold(valor, orden(ruta)) {
		return true
	}
	guardada, _, ok := strings.Cut(strings.TrimPrefix(valor, `"`), `"`)
	a, errA := os.Stat(guardada)
	b, errB := os.Stat(ruta)
	return ok && errA == nil && errB == nil && os.SameFile(a, b)
}

// Activo indica si el arranque automático de esta copia está registrado para el usuario actual.
func Activo() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil // en un perfil nuevo la clave Run puede no existir todavía
	}
	if err != nil {
		return false, err
	}
	defer k.Close()
	for _, nombre := range nombres() {
		valor, _, err := k.GetStringValue(nombre)
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return false, err
		}
		if err == nil && deEstaCopia(valor) {
			return true, nil
		}
	}
	return false, nil
}

// Activar registra la aplicación de escritorio para que arranque oculta al iniciar sesión (solo este usuario). En la
// portable, el valor que dejó una portable anterior a la 0.48 para esta copia pasa al suyo.
func Activar(escritorio string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, claveRun, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	propios := nombres()
	if err := k.SetStringValue(propios[0], orden(escritorio)); err != nil {
		return err
	}
	for _, viejo := range propios[1:] {
		if valor, _, err := k.GetStringValue(viejo); err == nil && deEstaCopia(valor) {
			_ = k.DeleteValue(viejo)
		}
	}
	return nil
}

// Desactivar quita el registro de esta copia (nunca el de otra); no es un error si no existía.
func Desactivar() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.SET_VALUE|registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	for _, nombre := range nombres() {
		valor, _, err := k.GetStringValue(nombre)
		if err != nil || !deEstaCopia(valor) {
			continue
		}
		if err := k.DeleteValue(nombre); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	return nil
}
