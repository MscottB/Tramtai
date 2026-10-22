package comp

import (
	"strings"
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

// Aviso enseña un texto corto en la línea de estado. Se borra solo a los 4 s, salvo un Fallo: ese se queda hasta la
// siguiente tecla, y la app lo enseña entero si se pide.
func Aviso(texto string, tipo TipoAviso) tea.Cmd {
	return func() tea.Msg { return NuevoAviso{Texto: texto, Tipo: tipo} }
}

type caducado struct{ n int }

// Avisos guarda el aviso que se ve y el último error. Lo usa la app; las páginas solo llaman a Aviso.
type Avisos struct {
	actual NuevoAviso
	ultimo string // el último fallo, entero, aunque ya no se vea
	n      int    // cuenta los avisos: el plazo de uno viejo no borra uno nuevo
}

// Update atiende NuevoAviso y su plazo.
func (a *Avisos) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case NuevoAviso:
		a.actual = msg
		a.n++
		if msg.Tipo == Fallo {
			a.ultimo = msg.Texto
			return nil
		}
		n := a.n
		return tea.Tick(duracionAviso, func(time.Time) tea.Msg { return caducado{n: n} })
	case caducado:
		if msg.n == a.n {
			a.actual = NuevoAviso{}
		}
	}
	return nil
}

// HayError dice si la línea de estado enseña un fallo.
func (a *Avisos) HayError() bool { return a.actual.Tipo == Fallo && a.actual.Texto != "" }

// Descartar quita de la línea de estado el fallo que se ve (la app lo llama con cada tecla); los demás avisos siguen
// hasta su plazo.
func (a *Avisos) Descartar() {
	if a.HayError() {
		a.actual = NuevoAviso{}
	}
}

// Ultimo es el último fallo, entero; "" si no hubo.
func (a *Avisos) Ultimo() string { return a.ultimo }

// Vista pinta el aviso en una línea, o nada si no hay.
func (a *Avisos) Vista(e *pagina.Estilos) string {
	texto := strings.Join(strings.Fields(a.actual.Texto), " ")
	if texto == "" {
		return ""
	}
	switch a.actual.Tipo {
	case Exito:
		return e.Exito.Render(e.Iconos.Ok + " " + texto)
	case Advertencia:
		return e.Advertencia.Render(texto)
	case Fallo:
		return e.Error.Render(e.Iconos.Fallo + " " + texto)
	}
	return e.Texto.Render(texto)
}
