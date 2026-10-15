//go:build linux

package autoarranque

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Disponible indica si este sistema admite el arranque automático.
const Disponible = true

// nombreArchivo es la entrada de ~/.config/autostart; las pruebas usan otra para no tocar la real.
var nombreArchivo = "tramtai.desktop"

// plantilla sigue la especificación de arranque automático de freedesktop (GNOME, KDE, Xfce y otros escritorios).
const plantilla = `[Desktop Entry]
Type=Application
Name=Tramtai
Comment=Tramtai en la bandeja con el servicio en marcha
Exec="%s" --oculto
Icon=tramtai
Terminal=false
X-GNOME-Autostart-enabled=true
`

func archivo() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", nombreArchivo), nil
}

// Activo indica si el arranque automático está registrado para el usuario actual.
func Activo() (bool, error) {
	ruta, err := archivo()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(ruta); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
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
	return os.WriteFile(ruta, []byte(fmt.Sprintf(plantilla, escritorio)), 0o644)
}

// Desactivar quita el registro; no es un error si no existía.
func Desactivar() error {
	ruta, err := archivo()
	if err != nil {
		return err
	}
	if err := os.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
