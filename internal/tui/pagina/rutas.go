package pagina

import (
	"os"
	"path/filepath"
	"strings"

	"tramtai/internal/carpetas"
)

// CompletarRuta completa una ruta con lo que hay en disco, como el Tab de una terminal: el nombre entero si solo hay uno
// que empieza así (una carpeta, con su separador) o lo común a todos. Con soloCarpetas, los archivos no cuentan. Devuelve
// también los que encajan.
func CompletarRuta(ruta string, soloCarpetas bool) (string, []string) {
	dir, base := filepath.Split(ruta)
	leer := dir
	if leer == "" {
		leer = "."
	}
	entradas, err := os.ReadDir(carpetas.ExpandirCasa(leer))
	if err != nil {
		return ruta, nil
	}
	var candidatos []string
	for _, e := range entradas {
		if (e.IsDir() || !soloCarpetas) && strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(base)) {
			nombre := e.Name()
			if e.IsDir() {
				nombre += string(filepath.Separator)
			}
			candidatos = append(candidatos, nombre)
		}
	}
	if len(candidatos) == 0 {
		return ruta, nil
	}
	comun := candidatos[0]
	for _, c := range candidatos[1:] {
		for !strings.HasPrefix(c, comun) { // por runas: una tilde no se parte por la mitad
			r := []rune(comun)
			comun = string(r[:len(r)-1])
		}
	}
	if len(comun) < len(base) {
		return ruta, candidatos
	}
	return dir + comun, candidatos
}
