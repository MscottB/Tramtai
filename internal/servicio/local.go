package servicio

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

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

// NuevoServidorHTTP es el http.Server de todos los oyentes del servicio (el público, el local y el de cada punto con
// puerto propio): plazo para las cabeceras contra las conexiones lentas, tope de su tamaño y de la inactividad. Sin
// WriteTimeout: los flujos SSE duran lo que el trabajo y cada escritura lleva su propio plazo.
func NuevoServidorHTTP(h http.Handler, contexto func(context.Context, net.Conn) context.Context) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 64 << 10,
		ConnContext: contexto}
}

type claveOyenteLocal struct{}

// contextoOyenteLocal es el ConnContext del oyente local (BaseLocal), el de la CLI, la terminal y la app de este
// equipo: lo que llega por él nunca lo trae un proxy, aunque la dirección de loopback sea de confianza.
func contextoOyenteLocal(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ContextoDeConexion(ctx, c), claveOyenteLocal{}, true)
}

type claveDeProxy struct{}

// marcarProxy envuelve el handler de un oyente público: lo que llega desde una dirección de servicio.proxy.confiables
// lo reenvía ese proxy y nunca es local (desdeEsteEquipo), lleve o no cabeceras de proxy (R35).
func (s *Servicio) marcarProxy(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deProxy(r, s.opc.Confiables) {
			r = r.WithContext(context.WithValue(r.Context(), claveDeProxy{}, true))
		}
		h.ServeHTTP(w, r)
	})
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
// ajenas desde loopback). Lo que llega por un oyente público desde un proxy de confianza (servicio.proxy.confiables)
// nunca es local, aunque no traiga esas cabeceras (marcarProxy). No dice de qué usuario es: eso lo comprueba esLocal
// o, con un código de un solo uso, el hecho de que solo el dueño del servicio puede pedirlo con el token de servicio.
func desdeEsteEquipo(r *http.Request) bool {
	if r.Context().Value(claveDeProxy{}) != nil {
		return false
	}
	for _, cabecera := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Real-Ip", "Via"} {
		if r.Header.Get(cabecera) != "" {
			return false
		}
	}
	cliente, err := netip.ParseAddrPort(r.RemoteAddr)
	return err == nil && cliente.Addr().IsLoopback()
}
