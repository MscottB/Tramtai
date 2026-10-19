package servicio

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// esLocal indica que la petición llega por loopback desde un programa del mismo usuario del sistema que el
// servicio (el navegador o la aplicación de escritorio). Entonces la Web UI entra sin token: el token protege
// frente a otros usuarios del equipo, y eso se comprueba aquí con el dueño de la conexión. Las
// páginas web ajenas siguen bloqueadas por Host, Origin, SameSite y CSRF; las aplicaciones siguen usando su token.
func (s *Servicio) esLocal(r *http.Request) bool {
	return s.opc.EntradaLocal && mismoUsuarioLocal(r)
}

// mismoUsuarioLocal indica que la petición llega por loopback, sin proxy, desde un programa del mismo usuario del
// sistema que el servicio. Sin poder comprobarlo (un sistema sin implementación), responde que no.
func mismoUsuarioLocal(r *http.Request) bool {
	if !desdeEsteEquipo(r) {
		return false
	}
	cliente, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	direccion, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return false
	}
	servidor, err := netip.ParseAddrPort(direccion.String())
	if err != nil {
		return false
	}
	return mismoUsuario(netip.AddrPortFrom(cliente.Addr().Unmap(), cliente.Port()), netip.AddrPortFrom(servidor.Addr().Unmap(), servidor.Port()))
}

// desdeEsteEquipo indica que la petición llega por loopback y no la reenvía un proxy o túnel (que traería peticiones
// ajenas desde loopback). No dice de qué usuario es: eso lo comprueba esLocal o, con un código de un solo uso, el
// hecho de que solo el dueño del servicio puede pedirlo con el token de servicio.
func desdeEsteEquipo(r *http.Request) bool {
	for _, cabecera := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-Ip", "Via"} {
		if r.Header.Get(cabecera) != "" {
			return false
		}
	}
	cliente, err := netip.ParseAddrPort(r.RemoteAddr)
	return err == nil && cliente.Addr().IsLoopback()
}

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

// Tipos de los elementos de net.inet.tcp.pcblist_n en macOS (xnu, bsd/netinet/in_pcb.h y sys/socketvar.h).
const (
	xsoSocket     = 0x001
	xsoInpcb      = 0x010
	largoXinpgen  = 24 // la cabecera, que también cierra la lista
	largoXinpcbN  = 84 // hasta inp_dependladdr inclusive
	largoXsocketN = 72 // hasta so_uid inclusive
)

// uidDePcblist busca la conexión en la salida de net.inet.tcp.pcblist_n de macOS, la que lee netstat: una cabecera
// xinpgen y, por cada conexión, elementos {largo, tipo, …} alineados a 8 bytes; primero xinpcb_n con los puertos
// (orden de red) y las direcciones, y después xsocket_n con el uid de su dueño (so_uid). Todo en el orden del
// procesador, little endian en Intel y Apple Silicon.
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
				return int(le.Uint32(elemento[68:])), true
			}
			coincide = false
		}
		pos += alinear8(largo)
	}
	return 0, false
}

// conexionPcb lee de un xinpcb_n la dirección local y la remota.
func conexionPcb(e []byte) [2]netip.AddrPort {
	remoto, local := binary.BigEndian.Uint16(e[16:]), binary.BigEndian.Uint16(e[18:])
	direccion := func(union []byte) netip.Addr {
		if e[48]&0x2 != 0 { // INP_IPV6
			return netip.AddrFrom16([16]byte(union)).Unmap()
		}
		return netip.AddrFrom4([4]byte(union[12:16])) // in_addr_4in6: tres palabras de relleno y la IPv4
	}
	return [2]netip.AddrPort{netip.AddrPortFrom(direccion(e[68:84]), local), netip.AddrPortFrom(direccion(e[52:68]), remoto)}
}

func alinear8(n int) int { return (n + 7) &^ 7 }
