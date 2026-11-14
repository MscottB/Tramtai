package agentes

import (
	"time"

	"tramtai/internal/tui/pagina"
)

// NuevaConReloj es Nueva con las esperas de un reloj de prueba: el seguimiento de una preparación.
func NuevaConReloj(lengua string, espera func(time.Duration) <-chan time.Time) pagina.Pagina {
	p := Nueva(lengua).(*Pagina)
	p.espera = espera
	return p
}

// CadaSondeo es lo que espera la página entre dos consultas del estado de una preparación.
const CadaSondeo = cadaSondeo
