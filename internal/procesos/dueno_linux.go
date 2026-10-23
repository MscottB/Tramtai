//go:build linux

package procesos

import (
	"net/netip"
	"os"
)

// MismoUsuario indica si el extremo local de la conexión TCP local→remoto es del mismo usuario del sistema que este
// proceso: busca la conexión en /proc/net/tcp y tcp6 y compara su uid.
func MismoUsuario(local, remoto netip.AddrPort) bool {
	for _, archivo := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		datos, err := os.ReadFile(archivo)
		if err != nil {
			continue
		}
		if uid, ok := uidDeConexion(string(datos), local, remoto); ok {
			return uid == os.Getuid()
		}
	}
	return false
}
