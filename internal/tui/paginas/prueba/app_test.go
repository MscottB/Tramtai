package prueba

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

func TestTeclaConModificadores(t *testing.T) {
	for _, nombre := range []string{"enter", "ctrl+s", "shift+tab", "alt+up", "ctrl+end", "space", "a", "?"} {
		if got := Tecla(nombre).String(); got != nombre {
			t.Errorf("Tecla(%q) es %q", nombre, got)
		}
	}
}

// contador es una página mínima: «a» pide sumar uno (un Cmd que tarda), «b» manda un aviso y «s» espera a que la prueba
// suelte su canal. Cada respuesta lleva la versión con que se pidió y una vieja no cuenta.
type contador struct {
	n, gen int
	soltar chan struct{}
}

type sumado struct{ gen int }

func (c *contador) Titulo() string                  { return "contador" }
func (c *contador) Iniciar(pagina.Contexto) tea.Cmd { return nil }
func (c *contador) Vista() string                   { return string(rune('0' + c.n)) }
func (c *contador) Teclas() []pagina.Tecla          { return nil }
func (c *contador) EnCampo() bool                   { return false }
func (c *contador) Ayuda() string                   { return "" }
func (c *contador) Update(m tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := m.(type) {
	case sumado:
		if m.gen == c.gen {
			c.n++
		}
	case tea.KeyPressMsg:
		gen := c.gen
		switch m.String() {
		case "a":
			return c, func() tea.Msg { time.Sleep(5 * time.Millisecond); return sumado{gen} }
		case "b":
			return c, tea.Batch(comp.Aviso("hola", comp.Info), nil)
		case "s":
			return c, func() tea.Msg { <-c.soltar; return sumado{gen} }
		case "g":
			c.gen++
		}
	}
	return c, nil
}

func TestAppComoLaTerminal(t *testing.T) {
	c := &contador{soltar: make(chan struct{})}
	a := Abrir(t, c, pagina.Contexto{Ctx: context.Background()})
	a.Pulsar("a", "a")
	if a.Vista() != "2" {
		t.Fatalf("Pulsar espera a lo que pide cada tecla: %s", a.Vista())
	}
	if fuera := a.Pulsar("b"); len(fuera) != 1 {
		t.Fatalf("un aviso es de la app: %v", fuera)
	}
	// Lo que espera a algo de fuera no frena: la página se asienta y llega cuando se suelta.
	a.Pulsar("s")
	if a.Vista() != "2" {
		t.Fatalf("aún no: %s", a.Vista())
	}
	close(c.soltar)
	a.HastaVer("3")
	// Una respuesta retenida que llega con otra versión no cuenta.
	viejos := a.Retener(func() { a.Pulsar("a") })
	if len(viejos) != 1 || a.Vista() != "3" {
		t.Fatalf("Retener guarda el Cmd sin correrlo: %d %s", len(viejos), a.Vista())
	}
	a.Pulsar("g")
	a.Soltar(viejos)
	if a.Vista() != "3" {
		t.Fatalf("la respuesta vieja se descarta: %s", a.Vista())
	}
}
