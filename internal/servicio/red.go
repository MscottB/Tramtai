package servicio

import (
	"net/http"
	"net/netip"
	"strings"
)

// ClienteReal es la dirección del cliente: la de la conexión o, si la conexión viene de un proxy de confianza
// (servicio.proxy.confiables), la última de X-Forwarded-For que no es de confianza, leyendo de derecha a izquierda: cada
// proxy añade la suya al final y lo de la izquierda lo puede escribir el cliente. Si todas son de confianza, la más lejana.
func ClienteReal(r *http.Request, confiables []netip.Prefix) netip.Addr {
	cliente := direccionRemota(r)
	if !deProxy(r, confiables) {
		return cliente
	}
	saltos := lista(r, "X-Forwarded-For")
	for i := len(saltos) - 1; i >= 0; i-- {
		// La dirección sola (también [v6] sin puerto) o con puerto (ip:puerto o [v6]:puerto), como la añade IIS ARR.
		salto, err := netip.ParseAddr(strings.TrimSuffix(strings.TrimPrefix(saltos[i], "["), "]"))
		if err != nil {
			var conPuerto netip.AddrPort
			conPuerto, err = netip.ParseAddrPort(saltos[i])
			salto = conPuerto.Addr()
		}
		if err != nil {
			break // lo que no se entiende corta la cadena: de ahí a la izquierda no se cree
		}
		cliente = salto.Unmap()
		if !deConfianza(cliente, confiables) {
			break
		}
	}
	return cliente
}

// EsHTTPS dice si la petición llegó cifrada: por el TLS propio o, desde un proxy de confianza, con X-Forwarded-Proto: https.
// Vale la última, la del proxy más cercano: las anteriores las puede poner el cliente si el proxy añade en vez de sustituir.
func EsHTTPS(r *http.Request, confiables []netip.Prefix) bool {
	if r.TLS != nil {
		return true
	}
	protos := lista(r, "X-Forwarded-Proto")
	return deProxy(r, confiables) && strings.EqualFold(protos[len(protos)-1], "https")
}

// lista son los elementos de una cabecera de lista, de todas sus apariciones y sin espacios (al menos uno, quizá vacío).
func lista(r *http.Request, cabecera string) []string {
	elementos := strings.Split(strings.Join(r.Header.Values(cabecera), ","), ",")
	for i, e := range elementos {
		elementos[i] = strings.TrimSpace(e)
	}
	return elementos
}

// direccionRemota es la dirección de la conexión (inválida si RemoteAddr no se entiende).
func direccionRemota(r *http.Request) netip.Addr {
	d, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return d.Addr().Unmap()
}

// deProxy dice si la petición la reenvía un proxy de confianza: llega desde una dirección de confiables y no por el
// oyente local (BaseLocal), cuyas peticiones son de la CLI, la terminal y la app de este equipo, nunca de un proxy.
func deProxy(r *http.Request, confiables []netip.Prefix) bool {
	return r.Context().Value(claveOyenteLocal{}) == nil && deConfianza(direccionRemota(r), confiables)
}

func deConfianza(a netip.Addr, confiables []netip.Prefix) bool {
	for _, red := range confiables {
		if red.Contains(a) {
			return true
		}
	}
	return false
}
