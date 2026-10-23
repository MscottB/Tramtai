package configuracion

import (
	"time"

	"tramtai/internal/tui/pagina"
)

// NuevaConReloj es Nueva con la hora y las esperas de un reloj de prueba: el seguimiento de una actualización y de la
// preparación del conversor.
func NuevaConReloj(lengua string, ahora func() time.Time, espera func(time.Duration) <-chan time.Time) pagina.Pagina {
	p := Nueva(lengua).(*Pagina)
	p.ahora, p.espera = ahora, espera
	return p
}
