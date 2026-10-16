// Package credenciales guarda las claves de API de los proveedores en el almacén cifrado del sistema
// (Administrador de credenciales de Windows, Llavero de macOS, Secret Service en Linux), para no dejarlas en
// variables de entorno permanentes ni en archivos: Tramtai nunca las escribe en sus propios archivos. Tampoco lee
// ni guarda tokens OAuth de los CLIs: solo las claves que el usuario registra aquí.
package credenciales

import (
	"errors"
	"slices"
	"strings"

	"tramtai/internal/idioma"
)

// Nombres son las variables que admite el almacén; cada una se guarda como tramtai/<NOMBRE>.
var Nombres = []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "CODEX_API_KEY", "GEMINI_API_KEY"}

// Admitido indica si el nombre es una de las claves que Tramtai sabe usar.
func Admitido(nombre string) bool { return slices.Contains(Nombres, nombre) }

func objetivo(nombre string) string { return "tramtai/" + nombre }

// valorValido rechaza lo que no puede ser una clave de API: vacía, de más de 2048 bytes o con algo que no sea ASCII
// imprimible, comillas o barras invertidas. Además de no ser claves, romperían el paso a la herramienta del Llavero
// (que devuelve en hexadecimal lo que no es ASCII) o pasarían de su línea de órdenes.
func valorValido(valor string) error {
	if valor == "" || len(valor) > 2048 || strings.ContainsAny(valor, "\"'\\") ||
		strings.IndexFunc(valor, func(r rune) bool { return r <= 0x20 || r >= 0x7f }) >= 0 {
		return errors.New(idioma.T("eso no parece una clave de API: sin espacios, comillas ni saltos de línea"))
	}
	return nil
}
