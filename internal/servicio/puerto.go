package servicio

import (
	"net"
	"strconv"

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
