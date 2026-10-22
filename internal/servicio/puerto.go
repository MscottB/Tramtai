package servicio

import (
	"net"
	"strconv"

	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// intentosPuerto son los puertos siguientes al configurado que se prueban si está ocupado.
const intentosPuerto = 20

// PuertoLibre indica si ahora mismo se puede escuchar en host:puerto.
func PuertoLibre(host string, puerto int) bool {
	oyente, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(puerto)))
	if err != nil {
		return false
	}
	oyente.Close()
	return true
}

// FijarPuerto guarda servicio.puerto y servicio.puerto_automatico en el archivo global si el puerto está en rango y
// libre; automatico solo decide qué se escribe, no si se comprueba. Se aplica al reiniciar. Es lo que usan
// el servicio, la CLI y la terminal.
func FijarPuerto(rutaGlobal, host string, puerto int, automatico bool) error {
	return FijarPuertoPropio(rutaGlobal, host, puerto, 0, automatico)
}

// FijarPuertoPropio es FijarPuerto cuando ya hay un Tramtai escuchando en propio: ese puerto cuenta como libre.
func FijarPuertoPropio(rutaGlobal, host string, puerto, propio int, automatico bool) error {
	if puerto < 1024 || puerto > 65535 {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el puerto debe estar entre 1024 y 65535"))
	}
	if puerto != propio && !PuertoLibre(host, puerto) {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("el puerto %d está ocupado por otro programa: elige otro", puerto))
	}
	if err := configuracion.FijarValor(rutaGlobal, nucleo.OrigenGlobal, "servicio.puerto", puerto); err != nil {
		return err
	}
	return configuracion.FijarValor(rutaGlobal, nucleo.OrigenGlobal, "servicio.puerto_automatico", automatico)
}

// Escuchar abre host:puerto. Si está ocupado y automatico lo permite (servicio.puerto_automatico), prueba los
// siguientes y, si tampoco, uno libre que elija el sistema. Devuelve el puerto que quedó abierto.
func Escuchar(host string, puerto int, automatico bool) (net.Listener, int, error) {
	abrir := func(p int) (net.Listener, error) { return net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(p))) }
	oyente, err := abrir(puerto)
	if err == nil {
		return oyente, puerto, nil
	}
	if !automatico {
		return nil, 0, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
			"el puerto %d está ocupado por otro programa: elige otro con «tramtai servicio puerto <número>» (o en Ajustes de la Web UI), o activa servicio.puerto_automatico para usar uno libre", puerto))
	}
	for p := puerto + 1; p <= min(puerto+intentosPuerto, 65535); p++ {
		if oyente, err := abrir(p); err == nil {
			return oyente, p, nil
		}
	}
	if oyente, err = abrir(0); err != nil {
		return nil, 0, err
	}
	return oyente, oyente.Addr().(*net.TCPAddr).Port, nil
}
