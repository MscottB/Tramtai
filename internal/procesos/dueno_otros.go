//go:build !windows && !linux && !darwin

package procesos

import "net/netip"

// MismoUsuario: sin una forma de identificar al dueño de la conexión (otros sistemas), responde que no. Entonces la
// Web UI pide un enlace de tramtai abrir, las etiquetas cli y terminal no valen y el cliente no detiene un servicio
// de una versión anterior.
func MismoUsuario(netip.AddrPort, netip.AddrPort) bool { return false }
