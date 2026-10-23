package servicio

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// errOcupado es el cerrojo de la carpeta de datos tomado por otro proceso.
var errOcupado = errors.New("cerrojo ocupado")

// TomarCerrojo toma el cerrojo exclusivo de <datos>/servicio.lock: un solo servicio por carpeta de datos, aunque el
// puerto automático le deje abrir otro puerto. Si está ocupado, reintenta durante espera (el reinicio espera a que el
// servicio anterior salga). Lo suelta soltar o, si el proceso cae, el sistema.
func TomarCerrojo(datos string, espera time.Duration) (soltar func(), err error) {
	if err := os.MkdirAll(datos, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(datos, "servicio.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	limite := time.Now().Add(espera)
	for {
		err = bloquear(f)
		if !errors.Is(err, errOcupado) || !time.Now().Before(limite) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	switch {
	case errors.Is(err, errOcupado):
		f.Close()
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
			"otro servicio de Tramtai ya usa la carpeta de datos %s: detenlo antes (tramtai servicio detener)", datos))
	case err != nil:
		f.Close()
		return nil, err
	}
	return func() { f.Close() }, nil
}
