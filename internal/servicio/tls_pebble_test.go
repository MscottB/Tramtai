//go:build pebble

package servicio

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
)

// TestLetsEncryptPebble obtiene de verdad un certificado de Pebble (la CA de pruebas de Let's Encrypt, con
// PEBBLE_VA_ALWAYS_VALID=0) por TLS-ALPN-01. Variables: PEBBLE_URL (el directorio ACME, https://localhost:14000/dir),
// PEBBLE_CA (el certificado raíz con el que Pebble sirve ese directorio: test/certs/pebble.minica.pem de su
// repositorio) y PEBBLE_ALPN_PORT (el puerto al que Pebble llega para el desafío, el 5001 de su configuración).
// tramtai.prueba tiene que resolver, desde Pebble, a esta máquina.
func TestLetsEncryptPebble(t *testing.T) {
	const dominio = "tramtai.prueba"
	url, ca, puerto := os.Getenv("PEBBLE_URL"), os.Getenv("PEBBLE_CA"), os.Getenv("PEBBLE_ALPN_PORT")
	if url == "" || ca == "" {
		t.Fatal("faltan PEBBLE_URL y PEBBLE_CA (ver el comentario de la prueba)")
	}
	if puerto == "" {
		puerto = "5001"
	}
	pem, err := os.ReadFile(ca)
	if err != nil {
		t.Fatal(err)
	}
	raices := x509.NewCertPool()
	if !raices.AppendCertsFromPEM(pem) {
		t.Fatalf("%s no tiene ningún certificado", ca)
	}
	anterior := clienteACME
	clienteACME = &acme.Client{DirectoryURL: url, HTTPClient: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: raices}}}}
	t.Cleanup(func() { clienteACME = anterior })

	datos := t.TempDir()
	cfg, err := ConfigTLS(OpcionesTLS{Modo: "letsencrypt", Dominios: []string{dominio}, Datos: datos, Correo: "admin@tramtai.prueba"})
	if err != nil {
		t.Fatal(err)
	}
	oyente, err := net.Listen("tcp", ":"+puerto)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.NotFoundHandler()}
	go srv.Serve(tls.NewListener(oyente, cfg))
	t.Cleanup(func() { srv.Close() })

	// El primer saludo espera al certificado: autocert lo pide a Pebble, que vuelve a este mismo puerto con acme-tls/1.
	ctx, cancelar := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelar()
	conn, err := (&tls.Dialer{Config: &tls.Config{ServerName: dominio, InsecureSkipVerify: true}}).DialContext(ctx, "tcp", "127.0.0.1:"+puerto)
	if err != nil {
		t.Fatalf("sin certificado de Pebble: %v", err)
	}
	defer conn.Close()
	hoja := conn.(*tls.Conn).ConnectionState().PeerCertificates[0]
	if k, ok := hoja.PublicKey.(*ecdsa.PublicKey); !ok || k.Curve != elliptic.P256() {
		t.Fatalf("la clave del certificado es %T: se esperaba ECDSA P-256", hoja.PublicKey)
	}
	if len(hoja.DNSNames) != 1 || hoja.DNSNames[0] != dominio || !hoja.NotAfter.After(time.Now()) {
		t.Fatalf("certificado para %v hasta %v: se esperaba %s vigente", hoja.DNSNames, hoja.NotAfter, dominio)
	}
	guardado, err := os.ReadFile(filepath.Join(datos, "acme", dominio))
	if err != nil || !strings.Contains(string(guardado), "EC PRIVATE KEY") || !strings.Contains(string(guardado), "CERTIFICATE") {
		t.Fatalf("el certificado y su clave deberían quedar en la caché datos/acme: %v", err)
	}
}
