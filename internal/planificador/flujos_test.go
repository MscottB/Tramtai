package planificador

import (
	"testing"

	"tramtai/internal/adaptadores"
)

func TestReservarFlujoTope(t *testing.T) {
	p := iniciar(t, abrir(t), adaptadores.Simulado{}, 1, opciones())
	var liberar []func()
	for i := 0; i < MaximoFlujos; i++ {
		l, ok := p.ReservarFlujo()
		if !ok {
			t.Fatalf("la reserva %d debía caber", i+1)
		}
		liberar = append(liberar, l)
	}
	if _, ok := p.ReservarFlujo(); ok {
		t.Fatal("la reserva 9 debía rechazarse")
	}
	liberar[0]()
	liberar[0]() // liberar dos veces no deja dos cupos
	if _, ok := p.ReservarFlujo(); !ok {
		t.Fatal("tras liberar debía caber una más")
	}
	if _, ok := p.ReservarFlujo(); ok {
		t.Fatal("liberar dos veces dejó dos cupos")
	}
}
