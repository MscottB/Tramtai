//go:build !windows && !linux && !darwin

package servicio

import "net/netip"

// Sin una forma de identificar al dueño de la conexión (otros sistemas), la Web UI pide un enlace de tramtai abrir y
// las etiquetas cli y terminal no valen.
func mismoUsuario(netip.AddrPort, netip.AddrPort) bool { return false }
