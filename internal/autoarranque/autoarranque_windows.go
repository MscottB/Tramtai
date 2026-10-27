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

// Los dos modos arrancan la misma aplicación de escritorio: con --oculto abre la bandeja y con --solo-servicio solo
// inicia el servicio y termina (es un programa sin consola, así que no se ve nada).
const (
	argumentoApp      = "--oculto"
	argumentoServicio = "--solo-servicio"
)

// orden es lo que activar escribe para la aplicación en el modo dado.
func orden(escritorio string, modo Modo) string {
	argumento := argumentoApp
	if modo == ModoServicio {
		argumento = argumentoServicio
	}
	return `"` + escritorio + `" ` + argumento
}

// deEstaCopia dice en qué modo un valor de la clave Run arranca la aplicación de esta copia (la misma ruta, sin
// distinguir mayúsculas, o el mismo archivo); "" si no es de esta copia.
func deEstaCopia(valor string) Modo {
	ruta, err := propia()
	if err != nil {
		return ""
	}
	for _, modo := range []Modo{ModoApp, ModoServicio} {
		if strings.EqualFold(valor, orden(ruta, modo)) {
			return modo
		}
	}
	guardada, resto, ok := strings.Cut(strings.TrimPrefix(valor, `"`), `"`)
	a, errA := os.Stat(guardada)
	b, errB := os.Stat(ruta)
	if !ok || errA != nil || errB != nil || !os.SameFile(a, b) {
		return ""
	}
	if strings.TrimSpace(resto) == argumentoServicio {
		return ModoServicio
	}
	return ModoApp
}

// Configurado dice en qué modo está registrado el arranque automático de esta copia para el usuario actual; "" si no
// lo está.
func Configurado() (Modo, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveRun, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil // en un perfil nuevo la clave Run puede no existir todavía
	}
	if err != nil {
		return "", err
	}
	defer k.Close()
	for _, nombre := range nombres() {
		valor, _, err := k.GetStringValue(nombre)
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return "", err
		}
		if err == nil {
			if modo := deEstaCopia(valor); modo != "" {
				return modo, nil
			}
		}
	}
	return "", nil
}

// activar registra la aplicación de escritorio para que arranque en el modo dado al iniciar sesión (solo este usuario).
// En la portable, el valor que dejó una portable anterior a la 0.48 para esta copia pasa al suyo.
func activar(escritorio string, modo Modo) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, claveRun, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	propios := nombres()
	if err := k.SetStringValue(propios[0], orden(escritorio, modo)); err != nil {
		return err
	}
	for _, viejo := range propios[1:] {
		if valor, _, err := k.GetStringValue(viejo); err == nil && deEstaCopia(valor) != "" {
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
		if err != nil || deEstaCopia(valor) == "" {
			continue
		}
		if err := k.DeleteValue(nombre); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	return nil
}
