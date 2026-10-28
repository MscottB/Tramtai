//go:build !windows

package servidor

import (
	"io"
	"os"
)

// prepararPublica no hace nada: en Linux la carpeta es el RuntimeDirectory de la unidad (/run/tramtai, 0755, de la
// cuenta del servicio), que crea systemd al arrancar.
func prepararPublica(string) error { return nil }

// leerPublico lee como mucho maximo bytes del puerto: en /run solo escriben root y la cuenta del servicio.
func leerPublico(ruta string, maximo int64) ([]byte, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maximo))
}
