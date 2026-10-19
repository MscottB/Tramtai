package antigravity

import "golang.org/x/sys/windows"

// rutaCorta devuelve el nombre corto (8.3) de ruta, que no tiene espacios, o ruta si el volumen no los tiene.
func rutaCorta(ruta string) string {
	p, err := windows.UTF16PtrFromString(ruta)
	if err != nil {
		return ruta
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return ruta
	}
	return windows.UTF16ToString(buf[:n])
}
