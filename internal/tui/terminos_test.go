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
	for _, lengua := range []string{"es", "en"} {
		datosDir := t.TempDir()
		m := nuevoEn(Opciones{Datos: datosDir, Terminos: true}, lengua)
		vista := quitaSaltos(m.View().Content)
		if !strings.Contains(vista, datos.Texto(lengua, "terminos.titulo")) {
			t.Fatalf("%s: la primera vista son los términos:\n%s", lengua, vista)
		}
		for i := 1; i <= 5; i++ {
			riesgo := datos.Texto(lengua, "riesgos."+string(rune('0'+i)))
			if i == 3 || i == 4 { // en la terminal, sin el modo servidor, el 3 y el 4 lo dicen
				riesgo = terminos.SiServidor(datos.Texto(lengua, "riesgos.siServidor"), riesgo)
			}
			if !strings.Contains(vista, riesgo) {
				t.Errorf("%s: falta el riesgo %d (%q):\n%s", lengua, i, riesgo, vista)
			}
		}
		// El menú y las páginas no se ven ni responden mientras tanto.
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

// TestRiesgosComoEnGo: los cinco riesgos que tiene la terminal (de la Web UI) son los de terminos.Riesgos, en los dos idiomas.
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
		for i, riesgo := range terminos.Riesgos(false) {
			if i != 2 && i != 3 {
				continue
			}
			if web := terminos.SiServidor(datos.Texto(lengua, "riesgos.siServidor"), datos.Texto(lengua, "riesgos."+string(rune('1'+i)))); web != riesgo {
				t.Errorf("%s riesgo %d sin el modo servidor: %q y %q", lengua, i+1, web, riesgo)
			}
		}
	}
}
