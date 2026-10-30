package servidor

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// certificadoDePrueba escribe un certificado autofirmado para nombres y su clave, en PEM.
func certificadoDePrueba(t *testing.T, nombres ...string) (certificado, clave string) {
	t.Helper()
	privada, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: nombres[0]}, DNSNames: nombres,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &privada.PublicKey, privada)
	if err != nil {
		t.Fatal(err)
	}
	derClave, err := x509.MarshalPKCS8PrivateKey(privada)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certificado, clave = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "clave.pem")
	if err := os.WriteFile(certificado, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clave, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derClave}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certificado, clave
}

func TestExpuesto(t *testing.T) {
	proxy := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	for _, c := range []struct {
		e    Entrada
		pide bool
	}{
		{Entrada{Host: "127.0.0.1"}, false},
		{Entrada{Host: "localhost"}, false},
		{Entrada{Host: "::1"}, false},
		{Entrada{Host: "127.0.0.1", Confiables: proxy}, true},
		{Entrada{Host: "0.0.0.0"}, true},
		{Entrada{Host: "192.0.2.10"}, true},
		{Entrada{Host: "miequipo"}, true},
	} {
		if Expuesto(c.e) != c.pide {
			t.Errorf("%+v: expuesto %v", c.e, !c.pide)
		}
	}
}

// TestComprobarArranque: una fila por regla de spec §6, en su orden, y dos combinaciones seguras.
func TestComprobarArranque(t *testing.T) {
	certificado, clave := certificadoDePrueba(t, "tramtai.ejemplo.org")
	proxy := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	claude := []Suscripcion{{Agente: "claude"}}
	personal := []Suscripcion{{Agente: "claude", Personal: true}}
	archivos := Entrada{Host: "0.0.0.0", PermitirRemoto: true, TLS: "archivos", Dominios: []string{"tramtai.ejemplo.org"},
		Certificado: certificado, Clave: clave, Puerto: 8443}
	con := func(cambiar func(*Entrada)) Entrada {
		e := archivos
		cambiar(&e)
		return e
	}
	for _, c := range []struct {
		nombre string
		e      Entrada
		falla  string // un trozo del mensaje; vacío si arranca
	}{
		{"loopback sin nada", Entrada{Host: "127.0.0.1", TLS: "ninguno", Suscripciones: claude}, ""},
		{"0.0.0.0 con archivos y dominio", archivos, ""},
		{"1 remoto sin permitir_remoto", Entrada{Host: "0.0.0.0", TLS: "archivos"}, "servicio.permitir_remoto"},
		{"2 remoto sin TLS", Entrada{Host: "192.0.2.10", PermitirRemoto: true, TLS: "ninguno"}, "túnel SSH"},
		{"2 remoto con un modo desconocido", Entrada{Host: "192.0.2.10", PermitirRemoto: true}, "túnel SSH"},
		{"3 proxy con suscripción", Entrada{Host: "127.0.0.1", Confiables: proxy, TLS: "ninguno", Suscripciones: claude}, "claude"},
		{"3 remoto con suscripción", con(func(e *Entrada) { e.Suscripciones = claude }), "clave_api"},
		{"4 personal sin TOTP", con(func(e *Entrada) { e.Suscripciones = personal }), "TOTP"},
		{"5 letsencrypt sin dominios", con(func(e *Entrada) { e.TLS, e.Dominios, e.Correo, e.Puerto = "letsencrypt", nil, "admin@ejemplo.org", 443 }), "letsencrypt"},
		{"5 letsencrypt sin correo", con(func(e *Entrada) { e.TLS, e.Puerto = "letsencrypt", 443 }), "letsencrypt"},
		{"5 letsencrypt en otro puerto", con(func(e *Entrada) { e.TLS, e.Correo = "letsencrypt", "admin@ejemplo.org" }), "443"},
		{"6 archivos ilegibles", con(func(e *Entrada) { e.Certificado = filepath.Join(t.TempDir(), "falta.pem") }), "servicio.tls.certificado"},
		{"6 archivos sin el dominio", con(func(e *Entrada) { e.Dominios = []string{"tramtai.ejemplo.org", "otro.ejemplo.org"} }), "otro.ejemplo.org"},
	} {
		c.e.Servidor = true // las reglas 3 y 4 remiten a órdenes del servidor
		err := ComprobarArranque(c.e)
		switch {
		case c.falla == "" && err != nil:
			t.Errorf("%s: debería arrancar: %v", c.nombre, err)
		case c.falla != "" && (err == nil || !strings.Contains(err.Error(), c.falla)):
			t.Errorf("%s: se esperaba un error con %q: %v", c.nombre, c.falla, err)
		}
	}
	if err := ComprobarArranque(con(func(e *Entrada) {
		e.TLS, e.Correo, e.Puerto, e.Servidor = "letsencrypt", "admin@ejemplo.org", 443, true
	})); err != nil {
		t.Errorf("letsencrypt completo: %v", err)
	}
}

// TestSuscripcionExpuestaFueraDelServidor: fuera del servidor (sin TOTP ni órdenes de tramtai-servidor), con el servicio
// abierto a la red una suscripción, también la personal, no se admite: el mensaje pide la clave de API, sin remitir a
// órdenes que solo existen con --servidor.
func TestSuscripcionExpuestaFueraDelServidor(t *testing.T) {
	proxy := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	for _, personal := range []bool{false, true} {
		err := ComprobarArranque(Entrada{Host: "127.0.0.1", Confiables: proxy, Suscripciones: []Suscripcion{{Agente: "claude", Personal: personal}}})
		if err == nil || !strings.Contains(err.Error(), "adaptadores.claude.autenticacion: clave_api") || strings.Contains(err.Error(), "servidor") {
			t.Errorf("personal %v: %v", personal, err)
		}
	}
}

// TestCertificadoIlegibleReintentable: un certificado o una clave que no se pueden leer impiden arrancar, pero se puede
// reintentar (un montaje que aún no ha llegado al arrancar el equipo); el resto de errores de configuración, no.
func TestCertificadoIlegibleReintentable(t *testing.T) {
	falta := filepath.Join(t.TempDir(), "falta.pem")
	err := ComprobarArranque(Entrada{Host: "127.0.0.1", TLS: "archivos", Certificado: falta, Clave: falta})
	if err == nil || !Reintentable(err) || !strings.Contains(err.Error(), "servicio.tls.certificado") {
		t.Errorf("certificado ilegible: %v (reintentable %v)", err, Reintentable(err))
	}
	if err := ComprobarArranque(Entrada{Host: "0.0.0.0", TLS: "archivos"}); err == nil || Reintentable(err) {
		t.Errorf("otra regla: %v", err)
	}
}

// TestAvisoSinDominios: escuchando en todas las direcciones sin servicio.dominios arranca, pero desde otros equipos no
// se entra (el Host que mandan no está admitido): un aviso, no una regla.
func TestAvisoSinDominios(t *testing.T) {
	for _, c := range []struct {
		e     Entrada
		aviso bool
	}{
		{Entrada{Host: "0.0.0.0"}, true},
		{Entrada{Host: "::"}, true},
		{Entrada{Host: "0.0.0.0", Dominios: []string{"tramtai.ejemplo.org"}}, false},
		{Entrada{Host: "192.0.2.10"}, false},
		{Entrada{Host: "127.0.0.1"}, false},
	} {
		if a := AvisoSinDominios(c.e); (a != "") != c.aviso || (c.aviso && !strings.Contains(a, "servicio.dominios")) {
			t.Errorf("%+v: %q", c.e, a)
		}
	}
}

// TestConfiables: direcciones o redes CIDR; una dirección sola es /32 o /128, y una que no lo es da un error claro.
func TestConfiables(t *testing.T) {
	redes, err := Confiables([]string{"127.0.0.1", "10.0.0.0/8", "::1", "192.168.1.7/24"})
	if err != nil {
		t.Fatal(err)
	}
	quiere := []string{"127.0.0.1/32", "10.0.0.0/8", "::1/128", "192.168.1.0/24"}
	for i, red := range redes {
		if red.String() != quiere[i] {
			t.Errorf("%d: %s, se esperaba %s", i, red, quiere[i])
		}
	}
	if redes, err := Confiables([]string{"::ffff:10.0.0.1", "::ffff:10.0.0.0/104"}); err != nil || len(redes) != 2 ||
		redes[0].String() != "10.0.0.1/32" || redes[1].String() != "10.0.0.0/8" {
		t.Errorf("IPv4 dentro de IPv6 se pasa a IPv4: %v %v", redes, err)
	}
	// Fiarse de todos, o de casi todos (más anchas que /8 en IPv4 o /32 en IPv6), dejaría a cualquiera falsificar
	// X-Forwarded-For.
	if redes, err := Confiables([]string{"2001:db8::/32", "10.0.0.0/8"}); err != nil || len(redes) != 2 {
		t.Errorf("/32 en IPv6 y /8 en IPv4 son las más anchas admitidas: %v %v", redes, err)
	}
	for _, mala := range []string{"proxy.ejemplo.org", "10.0.0.0/33", "", "0.0.0.0/0", "::/0", "::ffff:0.0.0.0/96", "::ffff:10.0.0.0/90",
		"0.0.0.0/1", "128.0.0.0/7", "::ffff:10.0.0.0/103", "::/1", "2000::/31"} {
		if _, err := Confiables([]string{mala}); err == nil || !strings.Contains(err.Error(), "servicio.proxy.confiables") {
			t.Errorf("%q: %v", mala, err)
		}
	}
}
