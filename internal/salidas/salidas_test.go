package salidas

import (
	"testing"

	"tramtai/internal/nucleo"
)

const esquema = `{"type":"object","properties":{"ciudad":{"type":"string"},"millones":{"type":"number"}},"required":["ciudad","millones"],"additionalProperties":false}`

func TestValidar(t *testing.T) {
	if err := Validar([]byte(esquema), `{"ciudad":"Santiago","millones":5.3}`); err != nil {
		t.Fatal(err)
	}
	for _, salida := range []string{`{"ciudad":"Santiago"}`, `{"ciudad":"x","millones":1,"extra":true}`, `no es json`} {
		if err := Validar([]byte(esquema), salida); nucleo.CodigoDe(err) != nucleo.CodigoSalidaNoCumpleEsquema {
			t.Errorf("%s: se esperaba salida_no_cumple_esquema: %v", salida, err)
		}
	}
}

func TestEsquemaInvalido(t *testing.T) {
	for _, e := range []string{`{"type":"objeto"}`, `{no json`} {
		if _, err := Compilar([]byte(e)); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
			t.Errorf("%s: se esperaba solicitud_invalida: %v", e, err)
		}
	}
}

func TestReferenciaExternaRechazada(t *testing.T) {
	for _, ref := range []string{"file:///C:/Windows/win.ini", "https://example.com/esquema.json"} {
		if _, err := Compilar([]byte(`{"$ref":"` + ref + `"}`)); err == nil {
			t.Errorf("%s: un $ref externo no debe cargarse", ref)
		}
	}
}
