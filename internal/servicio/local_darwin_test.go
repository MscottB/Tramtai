//go:build darwin

package servicio

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"
)

// TestMismoUsuarioDarwin abre una conexión de verdad por loopback y comprueba que su dueño es este usuario. Si falla,
// muestra los elementos de net.inet.tcp.pcblist_n de esa conexión, para ver su formato.
func TestMismoUsuarioDarwin(t *testing.T) {
	oyente, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer oyente.Close()
	go func() {
		if c, err := oyente.Accept(); err == nil {
			defer c.Close()
			_, _ = c.Read(make([]byte, 1))
		}
	}()
	conexion, err := net.Dial("tcp", oyente.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conexion.Close()
	cliente := netip.MustParseAddrPort(conexion.LocalAddr().String())
	servidor := netip.MustParseAddrPort(oyente.Addr().String())
	if mismoUsuario(cliente, servidor) {
		return
	}
	datos, err := unix.SysctlRaw("net.inet.tcp.pcblist_n")
	if err != nil {
		t.Fatal(err)
	}
	t.Errorf("no se encontró la conexión %v → %v en %d bytes (cabecera %s)", cliente, servidor, len(datos), hex.EncodeToString(datos[:min(32, len(datos))]))
	puerto := binary.BigEndian.AppendUint16(nil, cliente.Port())
	le := binary.LittleEndian
	mostrar := 0
	for pos := alinear8(int(le.Uint32(datos))); pos+8 <= len(datos); {
		largo := int(le.Uint32(datos[pos:]))
		if largo <= 0 || pos+largo > len(datos) {
			t.Logf("fin en %d: largo %d", pos, largo)
			break
		}
		e := datos[pos : pos+largo]
		if bytes.Contains(e[:min(len(e), 24)], puerto) {
			mostrar = 6 // el elemento con el puerto y los que le siguen
		}
		if mostrar > 0 {
			t.Logf("pos %d largo %d tipo %#x %s", pos, largo, le.Uint32(e[4:]), hex.EncodeToString(e[:min(len(e), 128)]))
			mostrar--
		}
		pos += alinear8(largo)
	}
}
