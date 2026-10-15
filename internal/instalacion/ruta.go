// Package instalacion reúne lo que el instalador de Windows delega en tramtai.exe: añadir o quitar su carpeta del
// PATH del usuario. NSIS corta las cadenas a 1024 caracteres y estropearía un PATH largo; aquí no hay ese límite.
package instalacion

import "strings"

// editarRuta añade dir a ruta (entradas separadas por ;) si no está, o lo quita si está. Compara como Windows, sin
// mayúsculas ni barra final, y no toca nada más: ni el orden, ni las %VARIABLES%, ni los ; vacíos o finales. Así
// instalar y desinstalar deja el PATH exactamente como estaba. Si no hay nada que cambiar devuelve ruta sin tocar.
func editarRuta(ruta, dir string, agregar bool) string {
	limpio := func(s string) string { return strings.ToLower(strings.TrimRight(strings.TrimSpace(s), `\/`)) }
	objetivo := limpio(dir)
	if objetivo == "" {
		return ruta
	}
	entradas := strings.Split(ruta, ";")
	var resto []string
	ultima := -1 // última entrada no vacía de resto: dir va detrás de ella, antes de los ; finales
	for _, e := range entradas {
		if limpio(e) == objetivo {
			continue
		}
		resto = append(resto, e)
		if strings.TrimSpace(e) != "" {
			ultima = len(resto) - 1
		}
	}
	encontrado := len(resto) != len(entradas)
	switch {
	case agregar == encontrado:
		return ruta
	case !agregar:
		return strings.Join(resto, ";")
	case ruta == "":
		return dir
	}
	resto = append(resto[:ultima+1], append([]string{dir}, resto[ultima+1:]...)...)
	return strings.Join(resto, ";")
}
