//go:build linux || darwin

package autoarranque

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"tramtai/internal/carpetas"
)

// Disponible indica si este sistema admite el arranque automático.
const Disponible = true

// En Linux y macOS el arranque automático es un archivo en la carpeta del usuario: carpeta() dice dónde y
// contenido() qué escribe cada sistema. Como en Windows, la instalada y la portable tienen cada una su archivo
// (nombreArchivo y nombrePortable), así que activar una no toca la otra. Las portables anteriores usaban el de la
// instalada: en la portable también se reconoce, y al activarla se mueve al suyo.

// esPortable es variable para que las pruebas hagan de una copia u otra.
var esPortable = carpetas.Portable

// rutas son los archivos que pueden ser de esta copia, el propio primero.
func rutas() ([]string, error) {
	dir, err := carpeta()
	if err != nil {
		return nil, err
	}
	if esPortable() {
		return []string{filepath.Join(dir, nombrePortable), filepath.Join(dir, nombreArchivo)}, nil
	}
	return []string{filepath.Join(dir, nombreArchivo)}, nil
}

// deEstaCopia dice si el archivo existe y arranca la aplicación de esta copia (no la de otra).
func deEstaCopia(ruta string) (bool, error) {
	datos, err := os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	escritorio, err := propia()
	return err == nil && string(datos) == contenido(escritorio, filepath.Base(ruta)), nil
}

// Activo indica si el arranque automático de esta copia está registrado para el usuario actual.
func Activo() (bool, error) {
	lista, err := rutas()
	if err != nil {
		return false, err
	}
	for _, ruta := range lista {
		if propio, err := deEstaCopia(ruta); err != nil || propio {
			return propio, err
		}
	}
	return false, nil
}

// Activar registra la aplicación de escritorio para que arranque oculta al iniciar sesión (solo este usuario). En la
// portable, el archivo que dejó una portable anterior con el nombre de la instalada pasa al suyo.
func Activar(escritorio string) error {
	lista, err := rutas()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lista[0]), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(lista[0], []byte(contenido(escritorio, filepath.Base(lista[0]))), 0o644); err != nil {
		return err
	}
	for _, viejo := range lista[1:] {
		if propio, _ := deEstaCopia(viejo); propio {
			_ = os.Remove(viejo)
		}
	}
	return nil
}

// Desactivar quita el registro de esta copia (nunca el de otra); no es un error si no existía.
func Desactivar() error {
	lista, err := rutas()
	if err != nil {
		return err
	}
	for _, ruta := range lista {
		propio, err := deEstaCopia(ruta)
		if err != nil {
			return err
		}
		if !propio {
			continue
		}
		if err := os.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
