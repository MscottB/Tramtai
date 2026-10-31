// Package autoarranque registra Tramtai para que arranque al iniciar sesión en el sistema, de una de dos maneras: la app
// de escritorio oculta en la bandeja (ModoApp) o solo el servicio, sin ventana ni bandeja (ModoServicio); así el servicio
// y la Web UI están disponibles sin abrir ninguna terminal.
package autoarranque

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"tramtai/internal/idioma"
)

// Modo es cómo arranca Tramtai al iniciar sesión; "" es que no arranca.
type Modo string

const (
	// ModoApp arranca la app de escritorio oculta, en la bandeja.
	ModoApp Modo = "app"
	// ModoServicio arranca solo el servicio, sin ventana ni bandeja.
	ModoServicio Modo = "servicio"
)

// ModoDe lee un modo escrito por el usuario: app, servicio o, en inglés, service. Vacío es ModoApp, el de siempre.
func ModoDe(texto string) (Modo, error) {
	switch texto {
	case "", "app":
		return ModoApp, nil
	case "servicio", "service":
		return ModoServicio, nil
	}
	return "", errors.New(idioma.T("el modo del arranque es app o servicio"))
}

// ErrEnServidor es lo que dice pedir el arranque al iniciar sesión en el modo servidor.
var ErrEnServidor = errors.New(idioma.T("en el servidor no hay arranque al iniciar sesión: el servicio del sistema arranca con el equipo"))

// propia es la aplicación de escritorio de esta copia: el arranque solo cuenta como activo si es el de ella (la
// instalada y la portable no se pisan). Las pruebas la sustituyen.
var propia = Escritorio

// Elegir registra el arranque al iniciar sesión (solo este usuario) en el modo dado, en lugar del que hubiera. Para
// quitarlo, Desactivar.
func Elegir(modo Modo) error {
	if modo != ModoApp && modo != ModoServicio {
		return errors.New(idioma.T("el modo del arranque es app o servicio"))
	}
	escritorio, err := propia()
	if err != nil && modo == ModoServicio && runtime.GOOS != "windows" {
		// En Linux y macOS, ModoServicio arranca tramtai (programa()), no la app: basta con saber dónde estaría.
		escritorio, err = rutaEscritorio()
	}
	if err != nil {
		return err
	}
	return activar(escritorio, modo)
}

// Activo indica si el arranque automático de esta copia está registrado, en cualquier modo.
func Activo() (bool, error) {
	modo, err := Configurado()
	return modo != "", err
}

// Escritorio devuelve la ruta de la aplicación de escritorio, que se instala junto al ejecutable actual.
func Escritorio() (string, error) {
	ruta, err := rutaEscritorio()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(ruta); err != nil {
		propio, _ := os.Executable()
		return "", errors.New(idioma.Tf("no se encontró %s junto a %s", filepath.Base(ruta), filepath.Base(propio)))
	}
	return ruta, nil
}

// rutaEscritorio es donde estaría la aplicación de escritorio: junto al ejecutable actual, esté o no.
func rutaEscritorio() (string, error) {
	propio, err := os.Executable()
	if err != nil {
		return "", err
	}
	nombre := "tramtai-escritorio"
	if runtime.GOOS == "windows" {
		nombre += ".exe"
	}
	return filepath.Join(filepath.Dir(propio), nombre), nil
}
