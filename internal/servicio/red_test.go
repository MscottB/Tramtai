package servicio

import (
	"crypto/tls"
	"net/http/httptest"
	"net/netip"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"tramtai/internal/adaptadores"
	"tramtai/internal/procesos"
)

func TestClienteReal(t *testing.T) {
	confiables := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	for _, c := range []struct {
		remota, xff, quiere string
	}{
		{"10.0.0.1:4000", "203.0.113.9, 10.0.0.2", "203.0.113.9"},       // desde el proxy: la última que no es de confianza
		{"10.0.0.1:4000", "198.51.100.1, 203.0.113.9", "203.0.113.9"},   // lo que puso el cliente a la izquierda no cuenta
		{"10.0.0.1:4000", "10.0.0.5", "10.0.0.5"},                       // todas de confianza: la más lejana
		{"10.0.0.1:4000", "", "10.0.0.1"},                               // sin cabecera: la conexión
		{"10.0.0.1:4000", "basura, 10.0.0.2", "10.0.0.2"},               // una que no se entiende corta la cadena
		{"203.0.113.50:4000", "198.51.100.1", "203.0.113.50"},           // no es de confianza: XFF no se cree
		{"[::ffff:10.0.0.1]:4000", "203.0.113.9", "203.0.113.9"},        // IPv4 dentro de IPv6
		{"10.0.0.1:4000", "203.0.113.9:51234, 10.0.0.2", "203.0.113.9"}, // con puerto, como IIS ARR
		{"10.0.0.1:4000", "[2001:db8::7]:51234", "2001:db8::7"},         // IPv6 con puerto
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remota
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if a := ClienteReal(r, confiables); a.String() != c.quiere {
			t.Errorf("%s con XFF %q: %s, se esperaba %s", c.remota, c.xff, a, c.quiere)
		}
	}
	// Varias cabeceras X-Forwarded-For valen como una lista seguida.
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:4000"
	r.Header.Add("X-Forwarded-For", "203.0.113.9")
	r.Header.Add("X-Forwarded-For", "10.0.0.2")
	if a := ClienteReal(r, confiables); a.String() != "203.0.113.9" {
		t.Errorf("varias cabeceras: %s", a)
	}
}

func TestEsHTTPS(t *testing.T) {
	confiables := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	for _, c := range []struct {
		remota      string
		protos      []string // una X-Forwarded-Proto por elemento
		tls, quiere bool
	}{
		{"203.0.113.9:4000", nil, true, true},                       // TLS propio
		{"127.0.0.1:4000", []string{"https"}, false, true},          // proxy de confianza con HTTPS
		{"127.0.0.1:4000", []string{" HTTPS "}, false, true},        // sin distinguir mayúsculas ni espacios
		{"127.0.0.1:4000", []string{"http"}, false, false},          // el proxy dice que llegó en claro
		{"127.0.0.1:4000", nil, false, false},                       // sin cabecera
		{"203.0.113.9:4000", []string{"https"}, false, false},       // la cabecera de quien no es de confianza no vale
		{"127.0.0.1:4000", []string{"https", "http"}, false, false}, // la primera la puso el cliente; manda la del proxy
		{"127.0.0.1:4000", []string{"https, http"}, false, false},   // lo mismo en una sola cabecera
		{"127.0.0.1:4000", []string{"http, https"}, false, true},    // la última es la del proxy más cercano
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remota
		for _, proto := range c.protos {
			r.Header.Add("X-Forwarded-Proto", proto)
		}
		if c.tls {
			r.TLS = &tls.ConnectionState{}
		}
		if EsHTTPS(r, confiables) != c.quiere {
			t.Errorf("%s, proto %q, tls %v: se esperaba %v", c.remota, c.protos, c.tls, c.quiere)
		}
	}
}

// TestProxyLoopbackNoEsLocal (Revisión 3): un proxy de confianza en este equipo llega por loopback, pero lo que reenvía
// no abre la entrada local: cualquier cabecera de proxy hace que la petición no sea local.
func TestProxyLoopbackNoEsLocal(t *testing.T) {
	if !procesos.MismoUsuario(netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:2")) && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("sin identificación del dueño de la conexión en este sistema")
	}
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.opc.Web = fstest.MapFS{"index.html": {Data: []byte("ui")}}
	s.opc.EntradaLocal = true
	s.opc.Confiables = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	if r, cuerpo := nuevoNavegador(t, ts.URL).pedir("GET", "/ui/sesion", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"local":true`) {
		t.Fatalf("sin cabeceras de proxy, el mismo usuario entra: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := nuevoNavegador(t, ts.URL).pedir("GET", "/ui/sesion", "", "X-Forwarded-For", "203.0.113.5"); r.StatusCode != 401 {
		t.Fatalf("lo que reenvía un proxy de confianza no entra sin token: %d", r.StatusCode)
	}
}
