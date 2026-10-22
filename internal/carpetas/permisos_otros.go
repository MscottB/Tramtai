//go:build !windows

package carpetas

import "os"

// protegerCarpeta deja la carpeta solo para el usuario.
func protegerCarpeta(ruta string) error { return os.Chmod(ruta, 0o700) }
