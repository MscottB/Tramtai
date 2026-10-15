package servicio

import (
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
	if !s.opc.EntradaLocal {
		return false
	}
	// Un proxy o túnel del mismo usuario reenvía peticiones ajenas desde loopback: con sus cabeceras no hay entrada libre.
	for _, cabecera := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-Ip", "Via"} {
		if r.Header.Get(cabecera) != "" {
			return false
		}
	}
	cliente, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || !cliente.Addr().IsLoopback() {
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
