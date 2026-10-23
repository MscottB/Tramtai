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

// TestVisorDicePorDondeVa: como Inicio y Probar, si el texto no cabe, la última fila dice por dónde va (↑↓ visto/total),
// sin pasarse del alto; si cabe, no hay fila de más.
func TestVisorDicePorDondeVa(t *testing.T) {
	var lineas []string
	for i := range 20 {
		lineas = append(lineas, fmt.Sprintf("línea %d", i))
	}
	v := NuevoVisor(estilos())
	v.Tamano(40, 5)
	v.Poner(strings.Join(lineas, "\n"))
	lineasDe := func() []string {
		l := strings.Split(vista(v), "\n")
		for i := range l {
			l[i] = strings.TrimRight(l[i], " ")
		}
		return l
	}
	if l := lineasDe(); len(l) != 5 || l[4] != "↑↓ 4/20" || l[3] != "línea 3" {
		t.Fatalf("cuatro líneas y la posición:\n%s", vista(v))
	}
	v.Update(tecla("G"))
	if l := lineasDe(); l[3] != "línea 19" || l[4] != "↑↓ 20/20" {
		t.Fatalf("al final se ve la última línea:\n%s", vista(v))
	}
	v.Poner("corto")
	if strings.Contains(vista(v), "↑↓") {
		t.Fatalf("si cabe no hay posición:\n%q", vista(v))
	}
}

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
