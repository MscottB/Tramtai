package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
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

// Con -proyecto sobre una carpeta que ya traía .tramtai/ (o que estaba en otra carpeta), la app hace la misma pregunta
// que la CLI en lugar de quedarse en la página de error.
func TestConfiarAlAbrirElProyecto(t *testing.T) {
	pregunta := &cliente.ErrorConfianza{Err: nucleo.NuevoError(nucleo.CodigoConfianzaRequerida, "este proyecto estaba en /tmp/antes"), Movido: "/tmp/antes"}
	if texto, ok := confiarAlAbrir("/tmp/demo", fmt.Errorf("activar: %w", pregunta)); !ok || !strings.Contains(texto, "/tmp/antes") || !strings.HasSuffix(texto, "?") {
		t.Fatalf("tenía que preguntar si confía: %q %v", texto, ok)
	}
	if _, ok := confiarAlAbrir("", pregunta); ok {
		t.Fatal("sin -proyecto no hay nada que confiar")
	}
	if _, ok := confiarAlAbrir("/tmp/demo", errors.New("otro fallo")); ok {
		t.Fatal("otro error se queda en la página de error")
	}
}
