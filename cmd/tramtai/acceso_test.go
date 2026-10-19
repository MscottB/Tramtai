package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
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
	for _, malo := range []string{"semanal.tokens=1", "diario.horas=1", "diario.tokens=-1", "diario.tokens", "diario.peticiones=1.5", "mensual.usd=mucho", "mensual.usd=NaN", "diario.usd=Inf"} {
		if err := aplicarPresupuesto(&p, []string{malo}); err == nil {
			t.Errorf("%q debía rechazarse", malo)
		}
	}
}

// La CLI aplica las mismas reglas que la Web UI: no guarda una API con Antigravity (aún no habilitado para apps), que
// rechazaría todas sus peticiones.
func TestAccesoCrearConAgenteNoPermitido(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", ruta)
	cmd := comandoAcceso()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"crear", "con-agy", "--agente", "antigravity"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "Antigravity") {
		t.Fatalf("se creó una API con Antigravity: %v", err)
	}
	if datos, err := os.ReadFile(ruta); err == nil && strings.Contains(string(datos), "con-agy") {
		t.Fatalf("quedó guardada:\n%s", datos)
	}
}
