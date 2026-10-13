//go:build !windows && !linux

package servicio

import "net/netip"

// shortcut: en macOS no se identifica al dueño de la conexión; la Web UI pide la clave o un enlace de tramtai abrir.
// Con libproc (proc_pidinfo) se podría hacer igual que en Windows y Linux.
func mismoUsuario(netip.AddrPort, netip.AddrPort) bool { return false }
