package comp

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestVisorBusca(t *testing.T) {
	var lineas []string
	for i := range 100 {
		lineas = append(lineas, fmt.Sprintf("línea %d", i))
	}
	lineas[70] = "línea 70 con la aguja"
	lineas[90] = "otra Aguja más"
	v := NuevoVisor(estilos())
	v.Tamano(40, 10)
	v.Poner(strings.Join(lineas, "\n"))
	if !strings.Contains(vista(v), "línea 0") {
		t.Fatalf("empieza arriba:\n%s", vista(v))
	}
	if n := v.Buscar("AGUJA"); n != 2 {
		t.Fatalf("dos coincidencias, sin distinguir mayúsculas: %d", n)
	}
	if !strings.Contains(vista(v), "con la aguja") {
		t.Fatalf("salta a la primera:\n%s", vista(v))
	}
	v.Update(tecla("n"))
	if !strings.Contains(vista(v), "otra Aguja") {
		t.Fatalf("n salta a la siguiente:\n%s", vista(v))
	}
	v.Update(tecla("N"))
	if !strings.Contains(vista(v), "con la aguja") {
		t.Fatalf("N vuelve a la anterior:\n%s", vista(v))
	}
	v.Update(tecla("g"))
	if !strings.Contains(vista(v), "línea 0") {
		t.Fatalf("g vuelve arriba:\n%s", vista(v))
	}
	if n := v.Buscar("nada"); n != 0 {
		t.Fatalf("sin coincidencias: %d", n)
	}
}

func vista(v Visor) string { return ansi.Strip(v.Vista()) }

// TestVisorCambiaDeTema: el fondo llega después de crear el visor; al pintar usa los estilos de ese momento.
func TestVisorCambiaDeTema(t *testing.T) {
	e := estilos()
	v := NuevoVisor(e)
	v.Tamano(40, 5)
	v.Poner("una aguja")
	v.Buscar("aguja")
	e.Seleccion = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
	if !strings.Contains(v.Vista(), "38;2;255;0;0") {
		t.Fatalf("el resaltado usa los estilos de ahora: %q", v.Vista())
	}
}
