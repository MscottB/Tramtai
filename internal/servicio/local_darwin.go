//go:build darwin

package servicio

import (
	"net/netip"
	"os"

	"golang.org/x/sys/unix"
)

// mismoUsuario busca la conexión en la tabla TCP del sistema (net.inet.tcp.pcblist_n, la que lee netstat) y compara
// su uid con el del servicio.
func mismoUsuario(cliente, servidor netip.AddrPort) bool {
	datos, err := unix.SysctlRaw("net.inet.tcp.pcblist_n")
	if err != nil {
		return false
	}
	uid, ok := uidDePcblist(datos, cliente, servidor)
	return ok && uid == os.Getuid()
}
