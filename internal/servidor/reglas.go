package servidor

import (
	"crypto/tls"
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
				return invalida(idioma.Tf("el servicio está abierto a la red (servicio.host o servicio.proxy.confiables) y %s usa una suscripción: así solo se admite una clave de API (adaptadores.%s.autenticacion: clave_api)", s.Agente, s.Agente))
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
		for _, dominio := range e.Dominios {
			if err := par.Leaf.VerifyHostname(dominio); err != nil {
				return invalida(idioma.Tf("el certificado de servicio.tls.certificado no cubre %s, de servicio.dominios", dominio))
			}
		}
	}
	return nil
}

// Confiables lee servicio.proxy.confiables: direcciones (que valen como /32 o /128) o redes CIDR.
func Confiables(lista []string) ([]netip.Prefix, error) {
	redes := make([]netip.Prefix, 0, len(lista))
	for _, texto := range lista {
		texto = strings.TrimSpace(texto)
		red, err := netip.ParsePrefix(texto)
		if !strings.Contains(texto, "/") {
			var direccion netip.Addr
			if direccion, err = netip.ParseAddr(texto); err == nil {
				direccion = direccion.Unmap()
				red = netip.PrefixFrom(direccion, direccion.BitLen())
			}
		}
		if err != nil {
			return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("servicio.proxy.confiables: %q no es una dirección IP ni una red CIDR (como 127.0.0.1 o 10.0.0.0/8)", texto))
		}
		redes = append(redes, red.Masked())
	}
	return redes, nil
}
