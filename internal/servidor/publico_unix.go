//go:build !windows

package servidor

import (
	"cmp"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// prepararPublica no hace nada: en Linux la carpeta es el RuntimeDirectory de la unidad (/run/tramtai, 0755, de la
// cuenta del servicio), que crea systemd al arrancar.
func prepararPublica(string) error { return nil }

// leerPublico lee como mucho maximo bytes del puerto: en /run solo escriben root y la cuenta del servicio. Esa cuenta
// podría dejar ahí un FIFO (que colgaría al lector) o un enlace: solo se lee un archivo normal, sin seguir enlaces.
func leerPublico(ruta string, maximo int64) ([]byte, error) {
	f, err := os.OpenFile(ruta, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, cmp.Or(err, fs.ErrInvalid)
	}
	return io.ReadAll(io.LimitReader(f, maximo))
}
