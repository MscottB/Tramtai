package carpetas

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ExpandirCasa cambia un ~ inicial por la carpeta personal, como hace la terminal en macOS y Linux.
func ExpandirCasa(ruta string) string {
	if ruta != "~" && !strings.HasPrefix(ruta, "~/") && !strings.HasPrefix(ruta, `~\`) {
		return ruta
	}
	casa, err := os.UserHomeDir()
	if err != nil {
		return ruta
	}
	return filepath.Join(casa, ruta[1:])
}

// Dentro indica si ruta es raiz o está debajo de ella; fuera de Linux las mayúsculas no cuentan.
func Dentro(raiz, ruta string) bool {
	if runtime.GOOS != "linux" {
		raiz, ruta = strings.ToLower(raiz), strings.ToLower(ruta)
	}
	rel, err := filepath.Rel(raiz, ruta)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// MismaRuta compara dos rutas como lo hace IDProyecto: sin distinguir mayúsculas en Windows y macOS.
func MismaRuta(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS != "linux" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
