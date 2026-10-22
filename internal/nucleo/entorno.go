package nucleo

import (
	"regexp"
	"strings"
)

// VariableSecreta indica si el nombre de una variable de entorno parece guardar un secreto (claves, tokens,
// contraseñas, credenciales): los programas externos que lanza Tramtai no la heredan.
func VariableSecreta(nombre string) bool {
	nombre = strings.ToUpper(nombre)
	for _, marca := range []string{"API_KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "PRIVATE", "CLAVE", "PASSPHRASE"} {
		if strings.Contains(nombre, marca) {
			return true
		}
	}
	for _, final := range []string{"_KEY", "_PASS", "_PWD"} {
		if strings.HasSuffix(nombre, final) {
			return true
		}
	}
	return nombre == "SSH_AUTH_SOCK"
}

// credencialEnURL es una URL con usuario y contraseña (postgres://u:p@h, http://u:p@proxy).
var credencialEnURL = regexp.MustCompile(`://[^/@\s:]+:[^/@\s]+@`)

// EntornoSecreto dice si un par «NOMBRE=valor» del entorno no debe llegar a un programa externo: por su nombre
// (VariableSecreta) o porque su valor es una URL con credenciales.
func EntornoSecreto(par string) bool {
	nombre, valor, _ := strings.Cut(par, "=")
	// Un proxy con usuario y clave sí pasa: los CLI lo necesitan para llegar a su proveedor y no tienen herramientas
	// con que filtrarlo.
	proxy := strings.HasSuffix(strings.ToUpper(nombre), "_PROXY")
	return VariableSecreta(nombre) || (!proxy && credencialEnURL.MatchString(valor))
}
