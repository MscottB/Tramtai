package procesos

// Dueño de una conexión TCP local: qué usuario del sistema tiene el proceso de un extremo. El servicio lo usa para
// reconocer al mismo usuario (entrada local, etiquetas cli y terminal) y el cliente, para saber si el proceso que
// escucha en el puerto del servicio es suyo.

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"strconv"
	"strings"
)

// uidDeConexion busca en una tabla con el formato de /proc/net/tcp (o tcp6) la conexión que el cliente abrió hacia
// el servidor y devuelve el uid de su dueño.
func uidDeConexion(tabla string, cliente, servidor netip.AddrPort) (int, bool) {
	for _, linea := range strings.Split(tabla, "\n") {
		campos := strings.Fields(linea)
		if len(campos) < 8 || campos[0] == "sl" {
			continue
		}
		local, ok1 := direccionProc(campos[1])
		remota, ok2 := direccionProc(campos[2])
		if !ok1 || !ok2 || local != cliente || remota != servidor {
			continue
		}
		uid, err := strconv.Atoi(campos[7])
		return uid, err == nil
	}
	return 0, false
}

// direccionProc traduce "0100007F:1F90": la IP en palabras de 32 bits en orden del procesador (little endian) y
// el puerto en hexadecimal.
func direccionProc(texto string) (netip.AddrPort, bool) {
	ip, puerto, ok := strings.Cut(texto, ":")
	crudo, err := hex.DecodeString(ip)
	p, errPuerto := strconv.ParseUint(puerto, 16, 16)
	if !ok || err != nil || errPuerto != nil || (len(crudo) != 4 && len(crudo) != 16) {
		return netip.AddrPort{}, false
	}
	for i := 0; i < len(crudo); i += 4 {
		crudo[i], crudo[i+1], crudo[i+2], crudo[i+3] = crudo[i+3], crudo[i+2], crudo[i+1], crudo[i]
	}
	direccion, _ := netip.AddrFromSlice(crudo)
	return netip.AddrPortFrom(direccion.Unmap(), uint16(p)), true
}

// Tipos y posiciones de los elementos de net.inet.tcp.pcblist_n en macOS (xnu, bsd/netinet/in_pcb.h y sys/socketvar.h).
// Se declaran con #pragma pack(4): los enteros de 64 bits van alineados a 4 bytes, no a 8 (TestMismoUsuarioDarwin).
const (
	xsoSocket    = 0x001
	xsoInpcb     = 0x010
	largoXinpgen = 24 // la cabecera, que también cierra la lista
	// xinpcb_n: inp_fport y inp_lport (orden de red), inp_vflag, y las direcciones remota y local (16 bytes cada una).
	pcbPuertos   = 16
	pcbVflag     = 44
	pcbRemota    = 48
	pcbLocal     = 64
	largoXinpcbN = 80
	// xsocket_n: so_uid.
	socketUID     = 64
	largoXsocketN = 68
)

// uidDePcblist busca la conexión en la salida de net.inet.tcp.pcblist_n de macOS (la que lee netstat): una cabecera
// xinpgen y, por cada conexión, elementos {largo, tipo, …} alineados a 8 bytes: xinpcb_n con puertos y direcciones, y
// después xsocket_n con el uid de su dueño. Todo en el orden del procesador (little endian).
func uidDePcblist(datos []byte, cliente, servidor netip.AddrPort) (int, bool) {
	le := binary.LittleEndian
	if len(datos) < largoXinpgen {
		return 0, false
	}
	coincide := false
	for pos := alinear8(int(le.Uint32(datos))); pos+8 <= len(datos); {
		largo := int(le.Uint32(datos[pos:]))
		if largo <= largoXinpgen || pos+largo > len(datos) {
			break
		}
		elemento := datos[pos : pos+largo]
		switch le.Uint32(elemento[4:]) {
		case xsoInpcb:
			coincide = largo >= largoXinpcbN && conexionPcb(elemento) == [2]netip.AddrPort{cliente, servidor}
		case xsoSocket:
			if coincide && largo >= largoXsocketN {
				return int(le.Uint32(elemento[socketUID:])), true
			}
			coincide = false
		}
		pos += alinear8(largo)
	}
	return 0, false
}

// conexionPcb lee de un xinpcb_n la dirección local y la remota.
func conexionPcb(e []byte) [2]netip.AddrPort {
	remoto, local := binary.BigEndian.Uint16(e[pcbPuertos:]), binary.BigEndian.Uint16(e[pcbPuertos+2:])
	direccion := func(union []byte) netip.Addr {
		if e[pcbVflag]&0x2 != 0 { // INP_IPV6
			return netip.AddrFrom16([16]byte(union)).Unmap()
		}
		return netip.AddrFrom4([4]byte(union[12:16])) // in_addr_4in6: tres palabras de relleno y la IPv4
	}
	return [2]netip.AddrPort{netip.AddrPortFrom(direccion(e[pcbLocal:pcbLocal+16]), local), netip.AddrPortFrom(direccion(e[pcbRemota:pcbRemota+16]), remoto)}
}

func alinear8(n int) int { return (n + 7) &^ 7 }
