//go:build !windows

package antigravity

// rutaCorta no cambia nada fuera de Windows: las carpetas temporales no suelen tener espacios.
func rutaCorta(ruta string) string { return ruta }
