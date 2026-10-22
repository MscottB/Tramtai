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
	a.Update(Aviso("No se pudo guardar", Advertencia)())
	a.Update(viejo)
	if !strings.Contains(a.Vista(estilos()), "No se pudo guardar") {
		t.Fatal("el plazo del aviso anterior no borra el nuevo")
	}
	a.Update(caducado{n: a.n})
	if a.Vista(estilos()) != "" {
		t.Fatalf("a los 4 s se borra: %q", a.Vista(estilos()))
	}
}

// TestErrorSeQueda: un fallo no caduca; se quita con Descartar (la siguiente tecla) y Ultimo lo conserva entero para
// verlo después. En la línea de estado, un error de varias líneas ocupa una.
func TestErrorSeQueda(t *testing.T) {
	var a Avisos
	if cmd := a.Update(Aviso("no se pudo\nleer el archivo", Fallo)()); cmd != nil {
		t.Fatal("un fallo no programa su borrado")
	}
	if v := a.Vista(estilos()); !a.HayError() || strings.Contains(v, "\n") || !strings.Contains(v, "no se pudo leer el archivo") {
		t.Fatalf("se ve en una línea: %q", v)
	}
	a.Descartar()
	if a.HayError() || a.Vista(estilos()) != "" || a.Ultimo() != "no se pudo\nleer el archivo" {
		t.Fatalf("la tecla lo quita de la línea y queda entero para verlo: %q", a.Ultimo())
	}
	a.Update(Aviso("Guardado", Exito)())
	a.Descartar()
	if !strings.Contains(a.Vista(estilos()), "Guardado") {
		t.Fatal("una tecla no quita un aviso que no es un fallo")
	}
}
