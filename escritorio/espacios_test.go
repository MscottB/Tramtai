package main

import (
	"testing"
	"time"

	"tramtai/internal/contrato"
)

func TestPrepararSesionNoReactivaLaGlobal(t *testing.T) {
	ahora := time.Now()
	activos := []contrato.EstadoInstancia{{ID: "viejo_1", Ambito: "proyecto", UltimoUso: ahora.Add(-time.Hour)},
		{ID: "nuevo_2", Ambito: "proyecto", UltimoUso: ahora}}
	if activar, id := espacioAlAbrir("", activos); activar || id != "nuevo_2" {
		t.Fatalf("con espacios activos abre el más reciente sin activar la global: %v %q", activar, id)
	}
	if activar, _ := espacioAlAbrir("", nil); !activar {
		t.Fatal("sin ninguno activo se activa la global")
	}
	if activar, _ := espacioAlAbrir("/tmp/demo", activos); !activar {
		t.Fatal("con -proyecto se activa ese proyecto")
	}
}
