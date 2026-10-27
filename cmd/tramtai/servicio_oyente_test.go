package main

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
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

// TestActivacionMuestraLaBasePublica: con TLS propio el descriptor apunta al oyente local (efímero), pero lo que se
// imprime y las apps guardan es la dirección pública; sin TLS, la del descriptor como siempre.
func TestActivacionMuestraLaBasePublica(t *testing.T) {
	a := contrato.ActivacionInstancia{Instancia: contrato.EstadoInstancia{ID: "global"}}
	for _, c := range []struct {
		nombre string
		cli    *cliente.Cliente
		quiere string
	}{
		{"con TLS propio", &cliente.Cliente{Base: "http://127.0.0.1:50123", Descriptor: contrato.Descriptor{Base: "http://127.0.0.1:50123", BasePublica: "https://tramtai.ejemplo.org:8443"}}, "https://tramtai.ejemplo.org:8443"},
		{"sin TLS", &cliente.Cliente{Base: "http://127.0.0.1:7777"}, "http://127.0.0.1:7777"},
	} {
		var salida bytes.Buffer
		imprimirActivacion(&salida, "global", a, c.cli)
		texto := salida.String()
		if !strings.Contains(texto, "activo en "+c.quiere+"\n") || !strings.Contains(texto, c.quiere+"/openai/v1") ||
			!strings.Contains(texto, c.quiere+"/anthropic") || (c.cli.Base != c.quiere && strings.Contains(texto, c.cli.Base)) {
			t.Errorf("%s:\n%s", c.nombre, texto)
		}
	}
}
