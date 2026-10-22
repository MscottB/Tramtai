//go:build darwin

package procesos

import (
	"net/netip"
	"os"

	"golang.org/x/sys/unix"
)

// MismoUsuario indica si el extremo local de la conexión TCP local→remoto es del mismo usuario del sistema que este
// proceso: busca la conexión en la tabla TCP del sistema (net.inet.tcp.pcblist_n, la que lee netstat) y compara su uid.
func MismoUsuario(local, remoto netip.AddrPort) bool {
	datos, err := unix.SysctlRaw("net.inet.tcp.pcblist_n")
	if err != nil {
		return false
	}
	uid, ok := uidDePcblist(datos, local, remoto)
	return ok && uid == os.Getuid()
}
