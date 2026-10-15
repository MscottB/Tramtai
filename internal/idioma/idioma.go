// Package idioma traduce los textos que se ven fuera de la Web UI: la CLI, la terminal interactiva, la app de
// escritorio y los mensajes de error del servicio. El español es el idioma de origen: cada texto se escribe en español
// dentro de T o Tf y se cambia por su traducción cuando el idioma activo es el inglés; sin traducción se queda en
// español. Las traducciones están en los archivos en_*.go, y TestCobertura exige una para cada texto marcado.
package idioma

import (
	"fmt"
	"os"
	"strings"
)

// Activo es "es" o "en": TRAMTAI_IDIOMA si está definida; si no, el idioma del sistema; y si no se reconoce, español.
var Activo = elegir(os.Getenv("TRAMTAI_IDIOMA"), delSistema())

// elegir devuelve "es" para cualquier variante de español, "en" para cualquier otro idioma reconocido y "es" si no hay
// ninguno (también con la configuración regional mínima C o POSIX de los servidores).
func elegir(candidatos ...string) string {
	for _, v := range candidatos {
		v = strings.ToLower(strings.TrimSpace(v))
		switch {
		case v == "" || v == "c" || v == "posix" || strings.HasPrefix(v, "c."):
			continue
		case strings.HasPrefix(v, "es"):
			return "es"
		default:
			return "en"
		}
	}
	return "es"
}

// Ingles indica si los textos se muestran en inglés.
func Ingles() bool { return Activo == "en" }

// T devuelve texto en el idioma activo.
func T(texto string) string {
	if Activo == "en" {
		if t, ok := en[texto]; ok {
			return t
		}
	}
	return texto
}

// Tf traduce un formato de fmt y lo rellena con args; la traducción conserva los mismos verbos en el mismo orden.
func Tf(formato string, args ...any) string { return fmt.Sprintf(T(formato), args...) }

var (
	en         = map[string]string{}
	conflictos []string
)

// agregar suma traducciones al inglés. Cada archivo en_*.go aporta las de una parte del programa; un mismo texto con
// dos traducciones distintas queda en conflictos y TestCobertura lo señala.
func agregar(m map[string]string) {
	for es, tr := range m {
		if previa, ok := en[es]; ok && previa != tr {
			conflictos = append(conflictos, es)
		}
		en[es] = tr
	}
}
