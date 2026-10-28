package servicio

import (
	"cmp"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// intervaloRecarga es cada cuánto, como mucho, se mira si cambiaron el certificado y la clave. Una variable para que
// las pruebas no esperen un segundo entre conexiones.
var intervaloRecarga = time.Second

// intervaloAvisos es cada cuánto, como mucho por dominio, se anota que no se pudo obtener su certificado.
var intervaloAvisos = time.Minute

// clienteACME es el cliente ACME de Let's Encrypt; nil usa su directorio de producción. Las pruebas lo apuntan a Pebble.
var clienteACME *acme.Client

// OpcionesTLS son las claves servicio.tls.* y servicio.dominios. Datos, Correo y Dominios son para Let's Encrypt (caché
// datos/acme, correo de la cuenta y HostPolicy); con archivos no se usan.
type OpcionesTLS struct {
	Modo, Certificado, Clave string
	Dominios                 []string
	Datos, Correo            string
	Registros                io.Writer // donde se anota una recarga que falla; nil la descarta
}

// ConfigTLS prepara el TLS propio del oyente: nil si el modo es ninguno (HTTP plano, con o sin proxy delante). Solo TLS
// 1.3: no hay cifrados que elegir y Go ofrece por defecto el intercambio híbrido poscuántico X25519MLKEM768. Sin
// NextProtos, archivos va en HTTP/1.1; letsencrypt también (ver certificadosLetsEncrypt).
func ConfigTLS(o OpcionesTLS) (*tls.Config, error) {
	switch o.Modo {
	case "archivos":
		par, err := cargarPar(o)
		if err != nil {
			return nil, err
		}
		return &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: par.certificado}, nil
	case "letsencrypt":
		return certificadosLetsEncrypt(o)
	}
	return nil, nil
}

// certificadosLetsEncrypt pide y renueva los certificados con autocert (desafío TLS-ALPN-01, en el mismo puerto 443;
// ECDSA P-256) solo para servicio.dominios; la caché y la cuenta quedan en datos/acme, privada. De TLSConfig() toma
// GetCertificate y NextProtos, que lleva acme-tls/1 (el desafío); h2 se quita: como con archivos, el servicio habla
// HTTP/1.1, que no toca el flujo en vivo (SSE).
func certificadosLetsEncrypt(o OpcionesTLS) (*tls.Config, error) {
	carpeta := filepath.Join(o.Datos, "acme")
	if err := os.MkdirAll(carpeta, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(carpeta, 0o700); err != nil { // una carpeta que ya existiera con otros permisos
		return nil, err
	}
	m := &autocert.Manager{Prompt: autocert.AcceptTOS, HostPolicy: autocert.HostWhitelist(o.Dominios...),
		Cache: autocert.DirCache(carpeta), Email: o.Correo, Client: clienteACME}
	c := m.TLSConfig()
	return &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: certificadoACME(c.GetCertificate, o),
		NextProtos: slices.DeleteFunc(c.NextProtos, func(p string) bool { return p == "h2" })}, nil
}

// certificadoACME envuelve el GetCertificate de autocert. Autocert elige RSA o ECDSA mirando solo las suites de TLS 1.2
// del saludo, y un cliente que solo habla TLS 1.3 daría un RSA y una orden de más a la CA; como en TLS 1.3 todo cliente
// admite ECDSA P-256 (RFC 8446 §9.1), le presenta un saludo que lo dice. SupportedProtos no se toca: decide el desafío
// acme-tls/1. Los fallos de un dominio de servicio.dominios (no los SNI ajenos ni los desafíos) se anotan en el
// registro, como mucho una vez por intervaloAvisos y dominio.
func certificadoACME(pedir func(*tls.ClientHelloInfo) (*tls.Certificate, error), o OpcionesTLS) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	registros := cmp.Or[io.Writer](o.Registros, io.Discard)
	propios := map[string]bool{}
	for _, d := range o.Dominios {
		propios[strings.ToLower(strings.TrimSuffix(d, "."))] = true
	}
	var mu sync.Mutex
	ultimo := map[string]time.Time{}
	return func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
		copia := *h
		copia.CipherSuites = []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256}
		copia.SignatureSchemes, copia.SupportedCurves = nil, nil
		cert, err := pedir(&copia)
		nombre := strings.ToLower(strings.TrimSuffix(h.ServerName, "."))
		if err != nil && propios[nombre] && !slices.Contains(h.SupportedProtos, "acme-tls/1") {
			mu.Lock()
			defer mu.Unlock()
			if ahora := time.Now(); ahora.Sub(ultimo[nombre]) >= intervaloAvisos {
				ultimo[nombre] = ahora
				fmt.Fprintln(registros, idioma.Tf("no se pudo obtener el certificado de %s: %s", nombre, err.Error()))
			}
		}
		return cert, err
	}
}

// parRecargable es el par certificado-clave en uso; vuelve a leerlo cuando cambia la fecha de cualquiera de los dos archivos.
type parRecargable struct {
	o          OpcionesTLS
	mu         sync.Mutex
	actual     *tls.Certificate
	fechas     [2]time.Time // las del par leído (o intentado); cero si no se pudo ver el archivo
	comprobado time.Time
}

func cargarPar(o OpcionesTLS) (*parRecargable, error) {
	if o.Registros == nil {
		o.Registros = io.Discard
	}
	p := &parRecargable{o: o, comprobado: time.Now()}
	p.fechas = p.fechasActuales()
	cert, err := tls.LoadX509KeyPair(o.Certificado, o.Clave)
	if err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("no se pueden leer el certificado y la clave (servicio.tls.certificado y servicio.tls.clave): %s", err.Error()))
	}
	p.actual = &cert
	return p, nil
}

func (p *parRecargable) fechasActuales() (f [2]time.Time) {
	for i, ruta := range []string{p.o.Certificado, p.o.Clave} {
		if info, err := os.Stat(ruta); err == nil {
			f[i] = info.ModTime()
		}
	}
	return f
}

// certificado es el GetCertificate del oyente. Si el par nuevo no se lee (por ejemplo, a medio escribir), sigue el
// anterior y se anota; la siguiente fecha que cambie lo intenta de nuevo.
func (p *parRecargable) certificado(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ahora := time.Now(); ahora.Sub(p.comprobado) >= intervaloRecarga {
		p.comprobado = ahora
		if f := p.fechasActuales(); !f[0].Equal(p.fechas[0]) || !f[1].Equal(p.fechas[1]) {
			p.fechas = f
			if cert, err := tls.LoadX509KeyPair(p.o.Certificado, p.o.Clave); err != nil {
				fmt.Fprintln(p.o.Registros, idioma.Tf("no se pudo recargar el certificado (%s y %s); sigue el anterior: %s", p.o.Certificado, p.o.Clave, err.Error()))
			} else {
				p.actual = &cert
			}
		}
	}
	return p.actual, nil
}
