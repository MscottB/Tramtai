package servicio

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"sync"

	"tramtai/internal/procesos"
)

// duenoConexion recuerda, por conexión, si su dueño es el usuario del servicio: no cambia mientras siga abierta, así
// que la tabla TCP del sistema se lee una vez por conexión y no en cada petición.
type duenoConexion struct {
	una   sync.Once
	mismo bool
}

type claveDueno struct{}

// ContextoDeConexion es el ConnContext de los servidores HTTP del servicio: prepara lo que se recuerda por conexión.
func ContextoDeConexion(ctx context.Context, _ net.Conn) context.Context {
	return context.WithValue(ctx, claveDueno{}, &duenoConexion{})
}

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
	comprobar := func() bool {
		return procesos.MismoUsuario(netip.AddrPortFrom(cliente.Addr().Unmap(), cliente.Port()), netip.AddrPortFrom(servidor.Addr().Unmap(), servidor.Port()))
	}
	if d, ok := r.Context().Value(claveDueno{}).(*duenoConexion); ok {
		d.una.Do(func() { d.mismo = comprobar() })
		return d.mismo
	}
	return comprobar()
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
