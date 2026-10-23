package historial

import (
	"time"

	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

// Lo que las pruebas de caja negra (historial_test) pueden cambiar de fuera de la página: la hora y las esperas (el sondeo
// y la reconexión del seguimiento en vivo) y de dónde lee los eventos de un trabajo.

// NuevaConReloj es Nueva con la hora y las esperas de un reloj de prueba.
func NuevaConReloj(lengua string, ahora func() time.Time, espera func(time.Duration) <-chan time.Time) pagina.Pagina {
	p := Nueva(lengua).(*Pagina)
	p.ahora, p.esperaPara = ahora, espera
	return p
}

// ConEventos cambia de dónde lee la página los eventos de un trabajo (cliente.Eventos).
func ConEventos(p pagina.Pagina, eventos vivo.Eventos) { p.(*Pagina).abrir = eventos }
