//go:build linux

package servicio

import (
	"net/netip"
	"os"
)

// mismoUsuario busca la conexión en /proc/net/tcp y tcp6 y compara su uid con el del servicio.
func mismoUsuario(cliente, servidor netip.AddrPort) bool {
	for _, archivo := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		datos, err := os.ReadFile(archivo)
		if err != nil {
			continue
		}
		if uid, ok := uidDeConexion(string(datos), cliente, servidor); ok {
			return uid == os.Getuid()
		}
	}
	return false
}
