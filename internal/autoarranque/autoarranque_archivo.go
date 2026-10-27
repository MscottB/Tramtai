//go:build linux || darwin

package autoarranque

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// programa es el ejecutable tramtai de la copia de esta aplicación de escritorio: se instalan en la misma carpeta.
func programa(escritorio string) string { return filepath.Join(filepath.Dir(escritorio), "tramtai") }

// deEstaCopia dice en qué modo arranca el archivo Tramtai desde esta copia (no desde otra); "" si no existe o no es de
// ella.
func deEstaCopia(ruta string) (Modo, error) {
	datos, err := os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	escritorio, err := propia()
	if err != nil {
		return "", nil
	}
	for _, modo := range []Modo{ModoApp, ModoServicio} {
		if string(datos) == contenido(escritorio, modo, filepath.Base(ruta)) {
			return modo, nil
		}
	}
	return "", nil
}

// Configurado dice en qué modo está registrado el arranque automático de esta copia para el usuario actual; "" si no
// lo está.
func Configurado() (Modo, error) {
	lista, err := rutas()
	if err != nil {
		return "", err
	}
	for _, ruta := range lista {
		if modo, err := deEstaCopia(ruta); err != nil || modo != "" {
			return modo, err
		}
	}
	return "", nil
}

// activar registra Tramtai para que arranque en el modo dado al iniciar sesión (solo este usuario). En la portable, el
// archivo que dejó una portable anterior con el nombre de la instalada pasa al suyo.
func activar(escritorio string, modo Modo) error {
	lista, err := rutas()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lista[0]), 0o755); err != nil {
		return err
	}
	if !esPortable() {
		apartarAjeno(lista[0], filepath.Join(filepath.Dir(lista[0]), nombrePortable))
	}
	if err := os.WriteFile(lista[0], []byte(contenido(escritorio, modo, filepath.Base(lista[0]))), 0o644); err != nil {
		return err
	}
	for _, viejo := range lista[1:] {
		if propio, _ := deEstaCopia(viejo); propio != "" {
			_ = os.Remove(viejo)
		}
	}
	return nil
}

// apartarAjeno pasa a destino (el nombre de la portable) el archivo de la instalada que no es de esta copia: lo dejó una
// portable anterior a la 0.48 y activar la instalada no debe quitarle el arranque. En macOS cambia también su Label, que
// es el nombre del archivo. No pisa un destino que ya existe.
func apartarAjeno(ruta, destino string) {
	if propio, err := deEstaCopia(ruta); err != nil || propio != "" {
		return
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return // no hay archivo
	}
	if _, err := os.Stat(destino); !errors.Is(err, fs.ErrNotExist) {
		return
	}
	etiqueta := func(n string) string { return "<string>" + strings.TrimSuffix(n, ".plist") + "</string>" }
	_ = os.WriteFile(destino, []byte(strings.Replace(string(datos), etiqueta(filepath.Base(ruta)), etiqueta(filepath.Base(destino)), 1)), 0o644)
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
		if propio == "" {
			continue
		}
		if err := os.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
