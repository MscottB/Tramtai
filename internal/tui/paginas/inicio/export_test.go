package inicio

import (
	"context"
	"time"

	"tramtai/internal/tui/pagina"
)

// Lo que las pruebas de caja negra (inicio_test) pueden cambiar de fuera de la página: la hora, las esperas (la limpieza
// de la red, el refresco y la reconexión) y de dónde lee el flujo de eventos del espacio.

// NuevaConReloj es Nueva con la hora y las esperas de un reloj de prueba.
func NuevaConReloj(lengua string, ahora func() time.Time, espera func(time.Duration) <-chan time.Time) pagina.Pagina {
	p := Nueva(lengua).(*Pagina)
	p.ahora, p.esperaPara = ahora, espera
	return p
}

// ConFlujo cambia de dónde lee la página el flujo del espacio (cliente.EventosInstanciaVigilado).
func ConFlujo(p pagina.Pagina, abrir func(ctx context.Context, instancia string, plazo time.Duration, alEvento func(tipo string, datos []byte) error) error) {
	p.(*Pagina).abrir = abrir
}

// Intervalos de la página, para que las pruebas hagan pasar el tiempo justo.
const (
	CadaLimpieza = cadaLimpieza
	CadaRefresco = cadaRefresco
)
