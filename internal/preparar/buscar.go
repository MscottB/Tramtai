package preparar

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"tramtai/internal/adaptadores"
	"tramtai/internal/carpetas"
	"tramtai/internal/idioma"
)

// expandir pasa una ruta de la receta («~/…» con «/») a una del sistema; «~» es la casa de la cuenta que corre el servicio.
func expandir(ruta string) string { return carpetas.ExpandirCasa(filepath.FromSlash(ruta)) }

// buscar encuentra el CLI recién instalado: en las rutas de la receta (donde lo deja su instalador), en el PATH del
// servicio y, en Windows, en el PATH de usuario del registro, que el servicio en marcha no ve. Vacío si no está.
func buscar(f adaptadores.Ficha) string {
	for _, r := range f.Receta.Rutas[runtime.GOOS] {
		if ruta := expandir(r); esArchivo(ruta) {
			return ruta
		}
	}
	if ruta := enPath(f.Comando); ruta != "" {
		return ruta
	}
	for _, dir := range pathUsuario() {
		if ruta := filepath.Join(dir, f.Comando+".exe"); esArchivo(ruta) {
			return ruta
		}
	}
	return ""
}

// enPath es la ruta a la que llega comandos_agentes con su valor predeterminado (el PATH del servicio, sin .cmd ni .bat).
func enPath(comando string) string {
	ruta, err := adaptadores.ResolverPrograma(comando)
	if err != nil {
		return ""
	}
	return ruta
}

func esArchivo(ruta string) bool {
	info, err := os.Stat(ruta)
	return err == nil && info.Mode().IsRegular()
}

// mismoArchivo dice si a y b son el mismo ejecutable.
func mismoArchivo(a, b string) bool {
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

// compartidas son carpetas de la casa que usan otros programas: ninguna receta las borra enteras.
var compartidas = []string{".local", ".local/bin", ".local/share", ".local/state", ".config", ".cache", ".npm", ".ssh",
	".gemini", "bin", "AppData", "AppData/Local", "AppData/LocalLow", "AppData/Roaming", "AppData/Local/Programs",
	"AppData/Local/Temp", "Library", "Library/Application Support", "Library/Caches", "Library/Preferences", "Documents",
	"Desktop", "Downloads"}

// RutaBorrable dice si una ruta de Receta.Borrar o Receta.DatosCLI se puede borrar: «~/…», sin «.» ni «..», y que no sea
// una carpeta compartida.
func RutaBorrable(r string) error {
	resto, ok := strings.CutPrefix(r, "~/")
	if !ok || resto == "" || resto == "." || strings.Contains(resto, `\`) || path.Clean(resto) != resto ||
		slices.Contains(strings.Split(resto, "/"), "..") {
		return errors.New(idioma.Tf("%s no es una ruta de la casa que se pueda borrar", r))
	}
	if slices.ContainsFunc(compartidas, func(c string) bool { return strings.EqualFold(c, resto) }) {
		return errors.New(idioma.Tf("%s es una carpeta compartida: no se borra", r))
	}
	return nil
}

// dentroDeCasa comprueba una ruta de la receta y la expande: tiene que caer dentro de casa y las carpetas entre casa y
// ella tienen que ser carpetas de verdad. Un enlace o una junction por el medio podría llevar fuera de la casa (en Windows
// EvalSymlinks no sigue las junction), así que se rechaza.
func dentroDeCasa(casa, r string) (string, error) {
	if err := RutaBorrable(r); err != nil {
		return "", err
	}
	ruta := expandir(r)
	if !carpetas.Dentro(casa, ruta) || carpetas.MismaRuta(casa, ruta) {
		return "", errors.New(idioma.Tf("%s está fuera de la carpeta del usuario: no se borra", ruta))
	}
	rel, err := filepath.Rel(casa, filepath.Dir(ruta))
	if err != nil {
		return "", err
	}
	carpeta := casa
	for _, parte := range strings.Split(rel, string(filepath.Separator)) {
		if parte == "." {
			break
		}
		carpeta = filepath.Join(carpeta, parte)
		info, err := os.Lstat(carpeta)
		if errors.Is(err, fs.ErrNotExist) {
			return ruta, nil // no existe: no hay nada que borrar
		}
		if err != nil || info.Mode().Type() != fs.ModeDir {
			return "", errors.New(idioma.Tf("%s pasa por un enlace: no se borra", ruta))
		}
	}
	return ruta, nil
}

// borrar quita una ruta de la receta. Un enlace o una junction (Codex/bin en Windows) se quita sin tocar lo que hay
// detrás (enlace es true); una carpeta de verdad, con su contenido.
func borrar(ruta string) (enlace bool, err error) {
	info, err := os.Lstat(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	switch info.Mode().Type() {
	case fs.ModeDir:
		return false, os.RemoveAll(ruta)
	case 0:
		return false, os.Remove(ruta)
	}
	return true, os.Remove(ruta)
}
