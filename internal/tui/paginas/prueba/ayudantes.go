package prueba

import (
	"context"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
)

// teclasConNombre son las teclas que se escriben por su nombre.
var teclasConNombre = map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "up": tea.KeyUp,
	"down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight, "pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
	"home": tea.KeyHome, "end": tea.KeyEnd, "backspace": tea.KeyBackspace, "delete": tea.KeyDelete, "space": tea.KeySpace}

// Tecla es la pulsación de una tecla por su nombre (enter, esc, tab, flechas, space…) o de una letra, con los
// modificadores delante: ctrl+s, alt+up, shift+tab, ctrl+end.
func Tecla(texto string) tea.KeyPressMsg {
	var mod tea.KeyMod
	for _, m := range []struct {
		prefijo string
		mod     tea.KeyMod
	}{{"ctrl+", tea.ModCtrl}, {"alt+", tea.ModAlt}, {"shift+", tea.ModShift}} {
		if len(texto) > len(m.prefijo) && texto[:len(m.prefijo)] == m.prefijo {
			texto, mod = texto[len(m.prefijo):], mod|m.mod
		}
	}
	if code, ok := teclasConNombre[texto]; ok {
		k := tea.KeyPressMsg{Code: code, Mod: mod}
		if texto == "space" && mod == 0 {
			k.Text = " "
		}
		return k
	}
	k := tea.KeyPressMsg{Code: []rune(texto)[0], Mod: mod}
	if mod == 0 {
		k.Text = texto
	}
	return k
}

// Aviso es el primer aviso entre lo que una página manda a la app.
func Aviso(fuera []tea.Msg) (comp.NuevoAviso, bool) {
	for _, m := range fuera {
		if a, ok := m.(comp.NuevoAviso); ok {
			return a, true
		}
	}
	return comp.NuevoAviso{}, false
}

// HayAviso dice si entre lo que una página manda a la app hay un aviso: la condición de HastaSalir más común.
func HayAviso(fuera []tea.Msg) bool { _, ok := Aviso(fuera); return ok }

// Bloqueado es un agente que, con el mensaje «esperar», avisa en Entro, espera a SoltarPaso, escribe «primera parte»,
// espera a SoltarFin y escribe « segunda parte»; los demás mensajes los contesta al momento con su texto. Así hay una
// petición en marcha sin depender del reloj. Ultima es la última solicitud que recibió.
type Bloqueado struct {
	Entro          chan struct{}
	paso, fin      chan struct{}
	una, dos, tres sync.Once
	mu             sync.Mutex
	ultima         nucleo.SolicitudEjecucion
}

func (b *Bloqueado) SoltarPaso() { b.dos.Do(func() { close(b.paso) }) }
func (b *Bloqueado) SoltarFin()  { b.tres.Do(func() { close(b.fin) }) }

func (b *Bloqueado) Ultima() nucleo.SolicitudEjecucion {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ultima
}

func (b *Bloqueado) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	b.mu.Lock()
	b.ultima = s
	b.mu.Unlock()
	mensaje := s.Mensajes[len(s.Mensajes)-1].Contenido
	if mensaje != "esperar" {
		_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: mensaje})
		return adaptadores.Resultado{Salida: mensaje}, nil
	}
	b.una.Do(func() { close(b.Entro) })
	esperar := func(c chan struct{}) error {
		select {
		case <-c:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := esperar(b.paso); err != nil {
		return adaptadores.Resultado{}, err
	}
	_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: "primera parte"})
	if err := esperar(b.fin); err != nil {
		return adaptadores.Resultado{}, err
	}
	_ = emitir(adaptadores.Evento{Tipo: "texto", Datos: " segunda parte"})
	return adaptadores.Resultado{Salida: "primera parte segunda parte"}, nil
}

// ConBloqueado levanta el servicio con un agente Bloqueado, que se suelta al acabar la prueba.
func ConBloqueado(t *testing.T) (*cliente.Cliente, string, *Bloqueado) {
	t.Helper()
	b := &Bloqueado{Entro: make(chan struct{}), paso: make(chan struct{}), fin: make(chan struct{})}
	cli, id, _ := ArrancarCon(t, b)
	t.Cleanup(func() { b.SoltarPaso(); b.SoltarFin() })
	return cli, id, b
}
