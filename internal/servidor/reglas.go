package servidor

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/netip"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Suscripcion es un agente que se usa con la suscripción de alguien; Personal, que es la suya y con sus condiciones.
type Suscripcion struct {
	Agente   string
	Personal bool
}

// Entrada es lo que deciden las reglas de exposición, sacado de la configuración (servicio.* y adaptadores.*).
type Entrada struct {
	Host           string
	PermitirRemoto bool
	Confiables     []netip.Prefix // servicio.proxy.confiables
	TLS            string         // servicio.tls.modo: ninguno, archivos o letsencrypt
	Dominios       []string
	Correo         string // servicio.tls.letsencrypt.correo
	Puerto         int
	Certificado    string // servicio.tls.certificado y servicio.tls.clave, en PEM
	Clave          string
	Suscripciones  []Suscripcion
	TOTP           bool // el administrador tiene TOTP
}

// Loopback dice si host es una dirección de este equipo que no sale a la red.
func Loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Expuesto dice si el servicio queda abierto a otros equipos: escucha fuera de loopback o atiende a un proxy.
func Expuesto(e Entrada) bool {
	return !Loopback(e.Host) || len(e.Confiables) > 0
}

// ComprobarArranque aplica las reglas que impiden arrancar (spec §6), en orden; el error dice qué hacer.
func ComprobarArranque(e Entrada) error {
	invalida := func(mensaje string) error { return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, mensaje) }
	remoto := !Loopback(e.Host)
	if remoto && !e.PermitirRemoto {
		return invalida(idioma.Tf("servicio.host %s no es una dirección de este equipo: para abrir el servicio a otros equipos activa servicio.permitir_remoto (mejor, usa un túnel SSH)", e.Host))
	}
	if remoto && e.TLS != "archivos" && e.TLS != "letsencrypt" {
		return invalida(idioma.Tf("servicio.host %s abre el servicio a la red sin HTTPS y las claves viajarían en claro: ponlo detrás de un proxy con HTTPS (servicio.host 127.0.0.1 y servicio.proxy.confiables), activa su HTTPS (servicio.tls.modo: archivos o letsencrypt) o usa un túnel SSH", e.Host))
	}
	if Expuesto(e) {
		for _, s := range e.Suscripciones {
			if !s.Personal {
				return invalida(idioma.Tf("el servicio está abierto a la red (servicio.host o servicio.proxy.confiables) y %s usa una suscripción: así solo se admite una clave de API (adaptadores.%s.autenticacion: clave_api) o tu suscripción personal, con sus condiciones (tramtai-servidor agentes suscripcion %s --personal)", s.Agente, s.Agente, s.Agente))
			}
		}
		for _, s := range e.Suscripciones {
			if !e.TOTP {
				return invalida(idioma.Tf("el servicio está abierto a la red y %s usa tu suscripción personal: hace falta TOTP para el administrador (tramtai --servidor administrador totp activar)", s.Agente))
			}
		}
	}
	switch e.TLS {
	case "letsencrypt":
		if len(e.Dominios) == 0 || e.Correo == "" || e.Puerto != 443 {
			return invalida(idioma.T("servicio.tls.modo letsencrypt necesita servicio.dominios, servicio.tls.letsencrypt.correo y servicio.puerto 443"))
		}
	case "archivos":
		par, err := tls.LoadX509KeyPair(e.Certificado, e.Clave)
		if err != nil {
			return invalida(idioma.Tf("no se pueden leer el certificado y la clave (servicio.tls.certificado y servicio.tls.clave): %s", err.Error()))
		}
		hoja := par.Leaf
		if hoja == nil { // Go 1.27 siempre la rellena; por si acaso, sin pánico
			if hoja, err = x509.ParseCertificate(par.Certificate[0]); err != nil {
				return invalida(idioma.Tf("no se pueden leer el certificado y la clave (servicio.tls.certificado y servicio.tls.clave): %s", err.Error()))
			}
		}
		for _, dominio := range e.Dominios {
			if err := hoja.VerifyHostname(dominio); err != nil {
				return invalida(idioma.Tf("el certificado de servicio.tls.certificado no cubre %s, de servicio.dominios", dominio))
			}
		}
	}
	return nil
}

// Confiables lee servicio.proxy.confiables: direcciones (que valen como /32 o /128) o redes CIDR. Las IPv4 dentro de IPv6
// (::ffff:…) pasan a IPv4. Fiarse de todos, o de casi todos (más anchas que /8 en IPv4 o /32 en IPv6), no se admite:
// cualquiera podría falsificar X-Forwarded-For.
func Confiables(lista []string) ([]netip.Prefix, error) {
	redes := make([]netip.Prefix, 0, len(lista))
	for _, texto := range lista {
		texto = strings.TrimSpace(texto)
		red, err := netip.ParsePrefix(texto)
		if !strings.Contains(texto, "/") {
			var direccion netip.Addr
			if direccion, err = netip.ParseAddr(texto); err == nil {
				red = netip.PrefixFrom(direccion, direccion.BitLen())
			}
		}
		if err == nil && red.Addr().Is4In6() {
			red = netip.PrefixFrom(red.Addr().Unmap(), red.Bits()-96) // menos de /96 sale del rango: Bits() < 0, no válida
		}
		minimo := 32 // los bits de la red más ancha admitida: /8 en IPv4 y /32 en IPv6
		if red.Addr().Is4() {
			minimo = 8
		}
		if err != nil || !red.IsValid() || red.Bits() < minimo {
			return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("servicio.proxy.confiables: %q no es una dirección IP ni una red CIDR admitida (como 127.0.0.1 o 10.0.0.0/8; no se admiten redes más anchas que /8 en IPv4 ni que /32 en IPv6)", texto))
		}
		redes = append(redes, red.Masked())
	}
	return redes, nil
}
