package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/idioma"
	"tramtai/internal/terminos"
	"tramtai/internal/tui/datos"
)

// quitaSaltos junta las líneas de la vista en una: el texto se parte al ancho de la ventana.
func quitaSaltos(vista string) string {
	return strings.Join(strings.Fields(ansi.Strip(vista)), " ")
}

func TestTerminalPideTerminos(t *testing.T) {
	anterior := idioma.Activo
	defer func() { idioma.Activo = anterior }()
	for _, lengua := range []string{"es", "en"} {
		idioma.Activo = lengua // terminos.Riesgos habla el idioma activo
		datosDir := t.TempDir()
		m := nuevoEn(Opciones{Datos: datosDir, Terminos: true}, lengua)
		vista := quitaSaltos(m.View().Content)
		if !strings.Contains(vista, datos.Texto(lengua, "terminos.titulo")) {
			t.Fatalf("%s: la primera vista son los términos:\n%s", lengua, vista)
		}
		for i, riesgo := range terminos.Riesgos(false) {
			if !strings.Contains(vista, riesgo) {
				t.Errorf("%s: falta el riesgo %d (%q):\n%s", lengua, i+1, riesgo, vista)
			}
		} // El menú y las páginas no se ven ni responden mientras tanto.
		if _, cmd := m.Update(tecla("2")); sale(cmd) || m.actual != "inicio" {
			t.Errorf("%s: los atajos de las páginas no valen hasta aceptar", lengua)
		}
		// Enter acepta: queda escrito y la terminal sigue con su menú.
		if _, cmd := m.Update(tecla("enter")); sale(cmd) {
			t.Fatalf("%s: aceptar no cierra la terminal", lengua)
		}
		if a, err := terminos.Leer(datosDir); err != nil || a.Version != terminos.Version || a.Como != terminos.ComoInterfaz {
			t.Errorf("%s: aceptación guardada: %+v %v", lengua, a, err)
		}
		if vista := quitaSaltos(m.View().Content); strings.Contains(vista, datos.Texto(lengua, "terminos.titulo")) {
			t.Errorf("%s: aceptados, la pantalla de los términos desaparece:\n%s", lengua, vista)
		}
	}
}

func TestTerminalSaleSinAceptar(t *testing.T) {
	for _, tecla_ := range []tea.KeyPressMsg{tecla("q"), {Code: 'c', Mod: tea.ModCtrl}} {
		datosDir := t.TempDir()
		m := nuevoEn(Opciones{Datos: datosDir, Terminos: true}, "es")
		if _, cmd := m.Update(tecla_); !sale(cmd) {
			t.Errorf("%v sale sin aceptar", tecla_)
		}
		if _, err := os.Stat(filepath.Join(datosDir, "terminos.json")); err == nil {
			t.Error("salir no acepta nada")
		}
	}
}

// TestTerminosEnLaTerminalConVentanaPequena: aceptar y salir valen también con la ventana por debajo del mínimo.
func TestTerminosEnLaTerminalConVentanaPequena(t *testing.T) {
	m := nuevoEn(Opciones{Datos: t.TempDir(), Terminos: true}, "es")
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if vista := m.View().Content; strings.Contains(vista, idioma.T("Agranda la ventana (mínimo 60×20)")) {
		t.Fatalf("los términos se ven aunque la ventana sea pequeña:\n%s", vista)
	}
	m.Update(tecla("enter"))
	if m.terminos {
		t.Error("enter acepta con la ventana pequeña")
	}
}

// TestRiesgosComoEnGo: los cinco riesgos que tiene la terminal (de la Web UI, textos.json) son los de terminos.Riesgos, con
// y sin el modo servidor, en los dos idiomas.
func TestRiesgosComoEnGo(t *testing.T) {
	anterior := idioma.Activo
	defer func() { idioma.Activo = anterior }()
	for _, lengua := range []string{"es", "en"} {
		idioma.Activo = lengua
		for i, riesgo := range terminos.Riesgos(true) {
			if web := datos.Texto(lengua, "riesgos."+string(rune('1'+i))); web != riesgo {
				t.Errorf("%s riesgo %d: la Web UI dice %q y terminos.Riesgos %q", lengua, i+1, web, riesgo)
			}
		}
		m := nuevoEn(Opciones{}, lengua)
		for i, riesgo := range terminos.Riesgos(false) {
			if got := m.riesgosTerminos()[i]; got != riesgo {
				t.Errorf("%s riesgo %d sin el modo servidor: la terminal dice %q y terminos.Riesgos %q", lengua, i+1, got, riesgo)
			}
		}
	}
}
