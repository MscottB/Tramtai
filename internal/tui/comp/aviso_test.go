package comp

import (
	"strings"
	"testing"
	"time"
)

func TestAvisoSeBorra(t *testing.T) {
	if duracionAviso != 4*time.Second {
		t.Fatalf("un aviso dura 4 s: %v", duracionAviso)
	}
	var a Avisos
	if cmd := a.Update(Aviso("Guardado", Exito)()); cmd == nil {
		t.Fatal("el aviso programa su borrado")
	}
	if !strings.Contains(a.Vista(estilos()), "Guardado") {
		t.Fatal("enseña el aviso")
	}
	viejo := caducado{n: a.n}
	a.Update(Aviso("No se pudo guardar", Fallo)())
	a.Update(viejo)
	if !strings.Contains(a.Vista(estilos()), "No se pudo guardar") {
		t.Fatal("el plazo del aviso anterior no borra el nuevo")
	}
	a.Update(caducado{n: a.n})
	if a.Vista(estilos()) != "" {
		t.Fatalf("a los 4 s se borra: %q", a.Vista(estilos()))
	}
}
