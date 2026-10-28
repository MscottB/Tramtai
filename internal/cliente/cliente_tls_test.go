package cliente

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
)

// TestClienteConTLSPropio: con TLS propio el descriptor apunta al oyente HTTP de 127.0.0.1, así que Conectar funciona
// aunque el puerto principal hable TLS; lo que se anuncia a las personas es https y el oyente local se cierra con el servicio.
func TestClienteConTLSPropio(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	datos := filepath.Join(dir, "datos")
	c, _ := configuracion.CargarConfiguracion("", agentes.Internos())

	cert := httptest.NewUnstartedServer(nil) // solo para tener un certificado de prueba
	cert.StartTLS()
	cert.Close()
	cfg := &tls.Config{Certificates: cert.TLS.Certificates, MinVersion: tls.VersionTLS13}

	ts := httptest.NewUnstartedServer(nil)
	puerto := ts.Listener.Addr().(*net.TCPAddr).Port
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Secretos: func() []string { return nil }, Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: puerto,
		Hosts: []string{"127.0.0.1"}, Dominios: []string{"tramtai.ejemplo.org"}, TLS: cfg})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Listener = tls.NewListener(ts.Listener, cfg)
	ts.Start()
	defer ts.Close()
	defer s.Cerrar()
	if err := s.EscribirDescriptor(); err != nil {
		t.Fatal(err)
	}

	cli, err := Conectar(datos)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cli.Base, "http://127.0.0.1:") || cli.Base == "http://127.0.0.1:"+strconv.Itoa(puerto) {
		t.Fatalf("el descriptor debe apuntar al oyente local, no al puerto TLS: %s", cli.Base)
	}
	if _, err := cli.Activar(context.Background(), "global", ""); err != nil {
		t.Fatalf("la CLI opera por el oyente local: %v", err)
	}
	publica := "https://tramtai.ejemplo.org:" + strconv.Itoa(puerto)
	if d, _ := contrato.LeerDescriptor(datos); d.Base != cli.Base || d.BasePublica != publica || s.BasePublica() != publica || cli.BasePublica() != publica {
		t.Fatalf("descriptor %s, público %s (%s, cliente %s)", d.Base, d.BasePublica, s.BasePublica(), cli.BasePublica())
	}
	// Lo que se anuncia a las personas (el enlace de entrada de la Web UI) también es la base pública.
	if enlace, err := cli.URLWeb(context.Background(), "global"); err != nil || !strings.HasPrefix(enlace, publica+"/ui/entrar?codigo=") {
		t.Fatalf("enlace de entrada: %q %v", enlace, err)
	}
	s.Cerrar()
	if r, err := http.Get(cli.Base + "/healthz"); err == nil {
		r.Body.Close()
		t.Fatal("el oyente local sigue abierto tras Cerrar")
	}
}
