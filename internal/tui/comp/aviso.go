package comp

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/tui/pagina"
)

// TipoAviso decide el color y el icono de un aviso.
type TipoAviso int

const (
	Info TipoAviso = iota
	Exito
	Advertencia
	Fallo
)

const duracionAviso = 4 * time.Second

// NuevoAviso lo enseña la app en la línea de estado.
type NuevoAviso struct {
	Texto string
	Tipo  TipoAviso
}

// Aviso enseña un texto corto en la línea de estado; se borra solo a los 4 s.
func Aviso(texto string, tipo TipoAviso) tea.Cmd {
	return func() tea.Msg { return NuevoAviso{Texto: texto, Tipo: tipo} }
}

type caducado struct{ n int }

// Avisos guarda el aviso que se ve. Lo usa la app; las páginas solo llaman a Aviso.
type Avisos struct {
	actual NuevoAviso
	n      int // cuenta los avisos: el plazo de uno viejo no borra uno nuevo
}

// Update atiende NuevoAviso y su plazo.
func (a *Avisos) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case NuevoAviso:
		a.actual = msg
		a.n++
		n := a.n
		return tea.Tick(duracionAviso, func(time.Time) tea.Msg { return caducado{n: n} })
	case caducado:
		if msg.n == a.n {
			a.actual = NuevoAviso{}
		}
	}
	return nil
}

// Vista pinta el aviso, o nada si no hay.
func (a *Avisos) Vista(e *pagina.Estilos) string {
	if a.actual.Texto == "" {
		return ""
	}
	switch a.actual.Tipo {
	case Exito:
		return e.Exito.Render(e.Iconos.Ok + " " + a.actual.Texto)
	case Advertencia:
		return e.Advertencia.Render(a.actual.Texto)
	case Fallo:
		return e.Error.Render(e.Iconos.Fallo + " " + a.actual.Texto)
	}
	return e.Texto.Render(a.actual.Texto)
}
