package preparar

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"tramtai/internal/adaptadores"
	"tramtai/internal/carpetas"
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

// borrar quita una ruta de la receta. Un enlace o una junction (Codex/bin en Windows) se quita sin tocar lo que hay
// detrás; una carpeta de verdad, con su contenido.
func borrar(ruta string) error {
	info, err := os.Lstat(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().Type() != fs.ModeDir {
		return os.Remove(ruta)
	}
	return os.RemoveAll(ruta)
}
