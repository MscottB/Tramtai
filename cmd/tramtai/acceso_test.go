package main

import (
	"testing"

	"tramtai/internal/nucleo"
)

func TestAplicarPresupuesto(t *testing.T) {
	var p nucleo.Presupuesto
	if err := aplicarPresupuesto(&p, []string{"diario.peticiones=500", "mensual.usd=12.5", "mensual.tokens=9000"}); err != nil {
		t.Fatal(err)
	}
	if p != (nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 500}, Mensual: nucleo.Limites{USD: 12.5, Tokens: 9000}}) {
		t.Fatalf("%+v", p)
	}
	if err := aplicarPresupuesto(&p, []string{"mensual.tokens=0"}); err != nil || p.Mensual.Tokens != 0 || p.Mensual.USD != 12.5 {
		t.Fatalf("0 quita solo ese límite: %v %+v", err, p)
	}
	for _, malo := range []string{"semanal.tokens=1", "diario.horas=1", "diario.tokens=-1", "diario.tokens", "diario.peticiones=1.5", "mensual.usd=mucho"} {
		if err := aplicarPresupuesto(&p, []string{malo}); err == nil {
			t.Errorf("%q debía rechazarse", malo)
		}
	}
}
