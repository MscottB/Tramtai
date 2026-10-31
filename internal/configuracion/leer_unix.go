//go:build unix

package configuracion

import (
	"cmp"
	"io"
	"io/fs"
	"os"
	"syscall"

	"tramtai/internal/carpetas"
)

// leerArchivo lee un archivo de configuración (config.yaml, local.yaml, un perfil o su esquema) sin quedarse colgado si
// es un FIFO y solo si es un archivo normal. En el servidor la carpeta la escribe la cuenta del servicio y la lee
// también root (la CLI): ahí no sigue enlaces, que la llevarían a leer otro archivo. Fuera del servidor, sí (config.yaml
// en un repositorio de dotfiles).
func leerArchivo(ruta string) ([]byte, error) {
	modo := os.O_RDONLY | syscall.O_NONBLOCK
	if carpetas.Servidor() {
		modo |= syscall.O_NOFOLLOW
	}
	f, err := os.OpenFile(ruta, modo, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: ruta, Err: cmp.Or(err, fs.ErrInvalid)}
	}
	return io.ReadAll(f)
}
