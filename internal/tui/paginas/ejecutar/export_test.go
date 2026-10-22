package ejecutar

import (
	"time"

	"tramtai/internal/tui/pagina"
)

// Lo que las pruebas de caja negra (ejecutar_test) pueden cambiar de fuera de la página: la hora y las esperas (el
// borrador y la reconexión del seguimiento en vivo).

// NuevaConReloj es Nueva con la hora y las esperas de un reloj de prueba.
func NuevaConReloj(lengua string, ahora func() time.Time, espera func(time.Duration) <-chan time.Time) pagina.Pagina {
	p := Nueva(lengua).(*Pagina)
	p.ahora, p.esperaPara = ahora, espera
	return p
}

// EsperaBorrador es lo que se espera tras la última tecla para guardar el borrador.
const EsperaBorrador = esperaBorrador
