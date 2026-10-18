package adaptadores

import (
	"strings"
	"testing"

	"tramtai/internal/nucleo"
)

func TestSinBloqueDeCodigo(t *testing.T) {
	for entrada, salida := range map[string]string{
		"```json\n{\"total\": 1}\n```": `{"total": 1}`,
		"```\n[1, 2]\n```":             "[1, 2]",
		`{"total": 1}`:                 `{"total": 1}`,
		"```":                          "```",
	} {
		if r := SinBloqueDeCodigo(entrada); r != salida {
			t.Errorf("SinBloqueDeCodigo(%q) = %q", entrada, r)
		}
	}
}

func TestFormatoEnInstrucciones(t *testing.T) {
	if f := FormatoEnInstrucciones(nucleo.ConfigSalida{Esquema: []byte(`{"type":"object"}`)}); !strings.Contains(f, `{"type":"object"}`) {
		t.Fatalf("el esquema va en las instrucciones: %q", f)
	}
	if f := FormatoEnInstrucciones(nucleo.ConfigSalida{Formato: "json"}); f == "" {
		t.Fatal("el formato json se pide")
	}
	if f := FormatoEnInstrucciones(nucleo.ConfigSalida{}); f != "" {
		t.Fatalf("sin formato no se pide nada: %q", f)
	}
}
