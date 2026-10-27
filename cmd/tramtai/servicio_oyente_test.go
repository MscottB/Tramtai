package main

import (
	"net"
	"strings"
	"testing"
)

// TestAbrirOyenteLetsEncrypt: con letsencrypt el puerto automático no se aplica (el desafío TLS-ALPN-01 solo llega al
// 443): un puerto ocupado es un error claro. En los demás modos, el puerto automático sigue valiendo.
func TestAbrirOyenteLetsEncrypt(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ocupado.Close()
	puerto := ocupado.Addr().(*net.TCPAddr).Port

	if o, _, err := abrirOyente("letsencrypt", "127.0.0.1", puerto, true); err == nil || !strings.Contains(err.Error(), "letsencrypt") {
		if o != nil {
			o.Close()
		}
		t.Fatalf("letsencrypt con el puerto ocupado y automático: se esperaba un error claro, salió %v", err)
	}
	for _, modo := range []string{"ninguno", "archivos"} {
		o, usado, err := abrirOyente(modo, "127.0.0.1", puerto, true)
		if err != nil || usado == puerto {
			t.Fatalf("%s con automático: puerto %d, error %v", modo, usado, err)
		}
		o.Close()
	}
}
