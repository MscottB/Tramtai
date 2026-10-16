//go:build linux

package credenciales

import (
	"errors"
	"os/exec"
	"strings"

	"tramtai/internal/idioma"
)

// Secret Service (el almacén cifrado de GNOME, KDE y otros escritorios) mediante secret-tool, sin dependencias ni
// CGO. Cada clave lleva los atributos servicio=tramtai y nombre=<NOMBRE>. Sin secret-tool, las claves van por
// variable de entorno.
const secretTool = "secret-tool"

var errSinSecretTool = errors.New(idioma.T("falta secret-tool: instala el paquete libsecret-tools (o libsecret) o usa variables de entorno"))

// Disponible indica si el sistema tiene almacén de credenciales soportado.
func Disponible() bool {
	_, err := exec.LookPath(secretTool)
	return err == nil
}

// Almacen es el nombre del almacén, para las instrucciones de la interfaz.
func Almacen() string { return idioma.T("Secret Service (secret-tool)") }

// Guardar crea o sustituye la clave; secret-tool la lee de la entrada estándar.
func Guardar(nombre, valor string) error {
	if err := valorValido(valor); err != nil {
		return err
	}
	if !Disponible() {
		return errSinSecretTool
	}
	_, _, _, err := ejecutar(valor, valor, secretTool, "store", "--label=Tramtai "+nombre, "servicio", "tramtai", "nombre", nombre)
	return err
}

// Leer devuelve la clave, o "" si no está guardada o no hay Secret Service (entonces vale la variable de entorno).
func Leer(nombre string) (string, error) {
	if !Disponible() {
		return "", nil
	}
	salida, errores, codigo, err := ejecutar("", "", secretTool, "lookup", "servicio", "tramtai", "nombre", nombre)
	if claveAusente(codigo, 1, salida, errores) {
		return "", nil // no existe; si el llavero no responde, err lo explica
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(salida), nil
}

// Borrar elimina la clave; no es un error si no existía.
func Borrar(nombre string) error {
	if !Disponible() {
		return errSinSecretTool
	}
	_, _, _, err := ejecutar("", "", secretTool, "clear", "servicio", "tramtai", "nombre", nombre)
	return err
}
