// Package terminos guarda y comprueba la aceptación expresa de los términos de uso (TERMINOS.md): qué versión aceptó
// este entorno, cuándo y cómo, en datos/terminos.json. También da los cinco riesgos que se muestran siempre con las mismas
// palabras (el instalador, la Web UI, la terminal, la CLI y el servidor). Un cambio de versión mayor vuelve a pedir la
// aceptación; uno menor (erratas, enlaces) no.
package terminos

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// URL es la página de los términos vigentes, en el repositorio.
const URL = "https://github.com/MscottB/Tramtai/blob/main/TERMINOS.md"

// Version es la versión vigente de los términos; debe coincidir con la que dice TERMINOS.md.
const Version = "2.0"

// Cómo se aceptó (el campo «como» de terminos.json).
const (
	ComoInstalador       = "instalador"
	ComoServidorInstalar = "servidor-instalar"
	ComoOpcion           = "opcion"
	ComoInterfaz         = "interfaz"
	ComoCLI              = "cli"
)

// Texto es TERMINOS.md. go:embed no sube de carpeta, así que esta es una copia del de la raíz;
// TestCopiaTerminosAlDia falla si difiere.
//
//go:embed TERMINOS.md
var Texto string

// Aceptacion es lo que queda escrito en datos/terminos.json.
type Aceptacion struct {
	Version string    `json:"version"`
	Fecha   time.Time `json:"fecha"`
	Como    string    `json:"como"`
}

func ruta(datos string) string { return filepath.Join(datos, "terminos.json") }

// Mayor es el número mayor de una versión («2.1» es 2); vacío o inválido es 0.
func Mayor(v string) int {
	n, err := strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// Leer lee la aceptación de la carpeta de datos; sin archivo, devuelve una vacía y nil.
func Leer(datos string) (Aceptacion, error) {
	var a Aceptacion
	bruto, err := os.ReadFile(ruta(datos))
	if errors.Is(err, fs.ErrNotExist) {
		return a, nil
	}
	if err == nil {
		err = json.Unmarshal(bruto, &a)
	}
	if err != nil {
		return Aceptacion{}, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("no se pudo leer %s: %s", ruta(datos), err.Error()))
	}
	return a, nil
}

// Aceptar escribe la aceptación de la versión vigente, con la fecha de ahora en UTC.
func Aceptar(datos, como string) error {
	if !slices.Contains([]string{ComoInstalador, ComoServidorInstalar, ComoOpcion, ComoInterfaz, ComoCLI}, como) {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("forma de aceptar desconocida: %s", como))
	}
	bruto, err := json.Marshal(Aceptacion{Version: Version, Fecha: time.Now().UTC().Truncate(time.Second), Como: como})
	if err != nil {
		return err
	}
	return configuracion.EscribirAtomico(ruta(datos), bruto, 0o600)
}

// Pendiente dice si hay que pedir la aceptación: falta, es de una versión mayor anterior o no se puede leer (en ese caso
// también devuelve el error, para quien quiera decirlo).
func Pendiente(datos string) (bool, error) {
	a, err := Leer(datos)
	if err != nil {
		return true, err
	}
	return Mayor(a.Version) < Mayor(Version), nil
}

// Riesgos son los cinco riesgos de los términos, los mismos en todas partes (web/src/idiomas, riesgos.*). Sin el modo
// servidor, el 3 y el 4 se dicen como «Si usas el modo servidor, …»: en el escritorio solo valen si lo activas.
func Riesgos(servidor bool) []string {
	r := []string{
		idioma.T("Tu cuenta con un proveedor puede limitarse, suspenderse o cerrarse si usas tu suscripción con Tramtai, y el proveedor puede cambiar sus reglas sin avisar. El autor no puede recuperarla."),
		idioma.T("Las condiciones de Tramtai (TOTP, una petición a la vez) bajan el riesgo, pero no lo quitan ni son una opinión legal sobre los términos del proveedor."),
		idioma.T("Lo que abres a la red es responsabilidad tuya: HTTPS, cortafuegos, contraseñas, copias y actualizaciones."),
		idioma.T("Si otras personas usan tu servidor, tú respondes de sus datos ante la ley que te aplique."),
		idioma.T("Tramtai es gratuito y se entrega tal cual, sin garantías."),
	}
	if !servidor {
		for _, i := range []int{2, 3} {
			r[i] = SiServidor(idioma.T("Si usas el modo servidor, "), r[i])
		}
	}
	return r
}

// SiServidor pone delante de un riesgo el prefijo «Si usas el modo servidor, » (ya traducido) y baja su primera letra.
func SiServidor(prefijo, riesgo string) string {
	c, n := utf8.DecodeRuneInString(riesgo)
	return prefijo + string(unicode.ToLower(c)) + riesgo[n:]
}
