//go:build !unix

package configuracion

import "os"

// leerArchivo lee un archivo de configuración. En Windows no hay FIFO en una carpeta y la del servidor solo la escriben
// SYSTEM, los administradores y la cuenta del servicio (leer_unix.go explica el caso de Unix).
func leerArchivo(ruta string) ([]byte, error) { return os.ReadFile(ruta) }
