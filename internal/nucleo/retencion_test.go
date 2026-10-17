package nucleo

import (
	"context"
	"testing"
)

func TestAjustarRetencion(t *testing.T) {
	casos := []struct {
		r, m, x, er, em, ex int
		cambio              bool
	}{
		{0, 168, 10000, 0, 168, 10000, false}, // 0 no guarda respuestas: se respeta
		{1000, 5000, 50, 720, 2160, 100, true},
		{48, 24, 10000, 48, 48, 10000, true}, // el registro nunca dura menos que la respuesta
		{-1, -1, 200000, 0, 0, 100000, true},
	}
	for _, c := range casos {
		r, m, x, cambio := AjustarRetencion(c.r, c.m, c.x)
		if r != c.er || m != c.em || x != c.ex || cambio != c.cambio {
			t.Errorf("AjustarRetencion(%d, %d, %d) = %d, %d, %d, %t", c.r, c.m, c.x, r, m, x, cambio)
		}
	}
}

func TestValidarRetencion(t *testing.T) {
	for _, v := range [][3]int{{24, 168, 10000}, {0, 0, 100}} {
		if err := ValidarRetencion(v[0], v[1], v[2]); err != nil {
			t.Errorf("%v debía valer: %v", v, err)
		}
	}
	for _, v := range [][3]int{{721, 800, 10000}, {24, 12, 10000}, {24, 168, 99}} {
		if err := ValidarRetencion(v[0], v[1], v[2]); CodigoDe(err) != CodigoSolicitudInvalida {
			t.Errorf("%v debía rechazarse con solicitud_invalida: %v", v, err)
		}
	}
}

func TestOrigenPeticion(t *testing.T) {
	ctx := context.Background()
	if o := OrigenPeticion(ctx); o != "" {
		t.Fatalf("sin origen: %q", o)
	}
	if o := OrigenPeticion(ConOrigenPeticion(ctx, DesdeCLI)); o != "cli" {
		t.Fatalf("con origen cli: %q", o)
	}
}
