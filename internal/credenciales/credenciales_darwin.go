//go:build darwin

package credenciales

import (
	"errors"
	"os"
	"strings"

	"tramtai/internal/idioma"
)

// Llavero de macOS mediante /usr/bin/security, que viene con el sistema: sin dependencias ni CGO. Cada clave es una
// contraseña genérica del usuario con el servicio tramtai/<NOMBRE> y la cuenta tramtai.
const security = "/usr/bin/security"

// noEncontrada es el código de salida de security cuando el elemento no existe (errSecItemNotFound).
const noEncontrada = 44

// Disponible indica si el sistema tiene almacén de credenciales soportado.
func Disponible() bool {
	_, err := os.Stat(security)
	return err == nil
}

// Almacen es el nombre del almacén, para las instrucciones de la interfaz.
func Almacen() string { return idioma.T("Llavero de macOS") }

// Guardar crea o sustituye la clave. security -i lee la orden por la entrada estándar, así que la clave no aparece
// en la lista de procesos; como esa orden no informa de un fallo por el código de salida, se comprueba leyéndola.
func Guardar(nombre, valor string) error {
	if !Admitido(nombre) { // el nombre también va en la orden de security -i: solo los fijos de Nombres
		return errors.New(idioma.T("clave no admitida"))
	}
	if err := valorValido(valor); err != nil {
		return err
	}
	orden := "add-generic-password -U -a tramtai -s " + objetivo(nombre) + " -w " + valor + "\n"
	if _, _, _, err := ejecutar(orden, valor, security, "-i"); err != nil {
		return err
	}
	if guardada, err := Leer(nombre); err != nil || guardada != valor {
		return errors.New(idioma.T("no se pudo guardar la clave en el Llavero"))
	}
	return nil
}

// Leer devuelve la clave, o "" si no está guardada.
func Leer(nombre string) (string, error) {
	salida, _, codigo, err := ejecutar("", "", security, "find-generic-password", "-a", "tramtai", "-s", objetivo(nombre), "-w")
	if codigo == noEncontrada {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(salida), nil
}

// Borrar elimina la clave; no es un error si no existía.
func Borrar(nombre string) error {
	if _, _, codigo, err := ejecutar("", "", security, "delete-generic-password", "-a", "tramtai", "-s", objetivo(nombre)); err != nil && codigo != noEncontrada {
		return err
	}
	return nil
}
