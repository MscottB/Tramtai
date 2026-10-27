package servicio

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
)

// parTemporal escribe un certificado ECDSA P-256 autofirmado (con cn de nombre común) y su clave, en PEM, y devuelve sus rutas.
func parTemporal(t *testing.T, dir, cn string) (certificado, clave string) {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}}
	der, err := x509.CreateCertificate(rand.Reader, plantilla, plantilla, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	claveDER, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	certificado, clave = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "clave.pem")
	if err := os.WriteFile(certificado, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clave, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: claveDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certificado, clave
}

// bufferSeguro es un registro que se puede leer mientras el servidor escribe.
type bufferSeguro struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *bufferSeguro) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *bufferSeguro) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// servidorTLS levanta un servidor HTTP con la configuración TLS dada y devuelve su dirección.
func servidorTLS(t *testing.T, cfg *tls.Config) string {
	t.Helper()
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Listener = tls.NewListener(ts.Listener, cfg)
	ts.Start()
	t.Cleanup(ts.Close)
	return ts.Listener.Addr().String()
}

// saludar conecta con la configuración de cliente dada (sin verificar el certificado: es autofirmado) y devuelve la conexión.
func saludar(t *testing.T, direccion string, cliente *tls.Config) (*tls.Conn, error) {
	t.Helper()
	cliente.InsecureSkipVerify = true
	return tls.Dial("tcp", direccion, cliente)
}

func configArchivos(t *testing.T, registros *bufferSeguro) (*tls.Config, OpcionesTLS) {
	t.Helper()
	cert, clave := parTemporal(t, t.TempDir(), "uno")
	o := OpcionesTLS{Modo: "archivos", Certificado: cert, Clave: clave, Registros: registros}
	cfg, err := ConfigTLS(o)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, o
}

func TestConfigTLSModos(t *testing.T) {
	if cfg, err := ConfigTLS(OpcionesTLS{Modo: "ninguno"}); cfg != nil || err != nil {
		t.Fatalf("ninguno: %v %v", cfg, err)
	}
	if _, err := ConfigTLS(OpcionesTLS{Modo: "archivos", Certificado: filepath.Join(t.TempDir(), "falta.pem"), Clave: "falta.key"}); err == nil {
		t.Fatal("archivos ilegibles: se esperaba un error")
	}
	if _, err := ConfigTLS(OpcionesTLS{Modo: "letsencrypt"}); err == nil || !strings.Contains(err.Error(), "letsencrypt") {
		t.Fatalf("letsencrypt aún no está disponible: %v", err)
	}
}

func TestTLSRechaza12(t *testing.T) {
	cfg, _ := configArchivos(t, nil)
	if c, err := saludar(t, servidorTLS(t, cfg), &tls.Config{MaxVersion: tls.VersionTLS12}); err == nil {
		c.Close()
		t.Fatal("un cliente TLS 1.2 no debería conectar")
	}
}

func TestTLSPoscuantico(t *testing.T) {
	cfg, _ := configArchivos(t, nil)
	c, err := saludar(t, servidorTLS(t, cfg), &tls.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if e := c.ConnectionState(); e.Version != tls.VersionTLS13 || e.CurveID != tls.X25519MLKEM768 {
		t.Fatalf("versión %#x, curva %v: se esperaba TLS 1.3 con X25519MLKEM768", e.Version, e.CurveID)
	}
}

func TestTLSRecarga(t *testing.T) {
	defer func(v time.Duration) { intervaloRecarga = v }(intervaloRecarga)
	intervaloRecarga = 0
	var registro bufferSeguro
	cfg, o := configArchivos(t, &registro)
	direccion := servidorTLS(t, cfg)
	nombre := func() string {
		t.Helper()
		c, err := saludar(t, direccion, &tls.Config{})
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		return c.ConnectionState().PeerCertificates[0].Subject.CommonName
	}
	if n := nombre(); n != "uno" {
		t.Fatalf("primer certificado: %q", n)
	}
	// Un par ilegible no tumba el servicio: sigue el anterior y queda anotado.
	if err := os.WriteFile(o.Certificado, []byte("basura"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := nombre(); n != "uno" || !strings.Contains(registro.String(), o.Certificado) {
		t.Fatalf("par ilegible: sirvió %q, registro %q", n, registro.String())
	}
	// Los archivos cambian por otro certificado: la conexión siguiente lo presenta (también si la fecha del archivo no avanza).
	nuevoCert, nuevaClave := parTemporal(t, t.TempDir(), "dos")
	for origen, destino := range map[string]string{nuevoCert: o.Certificado, nuevaClave: o.Clave} {
		datos, err := os.ReadFile(origen)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destino, datos, 0o600); err != nil {
			t.Fatal(err)
		}
		futuro := time.Now().Add(time.Minute)
		os.Chtimes(destino, futuro, futuro)
	}
	if n := nombre(); n != "dos" {
		t.Fatalf("tras el cambio: %q", n)
	}
}

func TestHSTS(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.opc.Confiables = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	for _, c := range []struct {
		nombre, remota, proto string
		tls, quiere           bool
	}{
		{"HTTP plano", "203.0.113.9:4000", "", false, false},
		{"TLS propio", "203.0.113.9:4000", "", true, true},
		{"proxy de confianza con https", "10.0.0.1:4000", "https", false, true},
		{"proxy de confianza con http", "10.0.0.1:4000", "http", false, false},
		{"https dicho por quien no es de confianza", "203.0.113.9:4000", "https", false, false},
	} {
		r := httptest.NewRequest("GET", "/healthz", nil)
		r.Host, r.RemoteAddr = "127.0.0.1", c.remota
		if c.proto != "" {
			r.Header.Set("X-Forwarded-Proto", c.proto)
		}
		if c.tls {
			r.TLS = &tls.ConnectionState{}
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		quiere := ""
		if c.quiere {
			quiere = "max-age=31536000"
		}
		if g := w.Header().Get("Strict-Transport-Security"); g != quiere {
			t.Errorf("%s: Strict-Transport-Security %q, se esperaba %q", c.nombre, g, quiere)
		}
	}
}
