package vivo

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// Icono es el dibujo del estado de una petición y su estilo.
func Icono(e *pagina.Estilos, estado nucleo.Estado) (string, lipgloss.Style) {
	switch {
	case !estado.Terminal():
		return e.Iconos.Activo, e.Acento
	case estado == nucleo.Completado:
		return e.Iconos.Ok, e.Exito
	case estado == nucleo.Cancelado:
		return e.Iconos.Inactivo, e.Tenue
	case estado == nucleo.Interrumpido:
		return e.Iconos.Fallo, e.Advertencia
	}
	return e.Iconos.Fallo, e.Error
}

// NombreEstado es el de la Web UI (estado.<estado>); si no lo conoce, el estado tal cual.
func NombreEstado(lengua string, estado nucleo.Estado) string {
	if t := datos.Texto(lengua, "estado."+string(estado)); t != "estado."+string(estado) {
		return t
	}
	return string(estado)
}

// posicionRuta es la parada de la ruta en que está cada estado.
var posicionRuta = map[nucleo.Estado]int{nucleo.EnCola: 0, nucleo.PreparandoEntrada: 1, nucleo.EsperandoAgente: 1, nucleo.Ejecutando: 2,
	nucleo.CancelacionSolicitada: 2, nucleo.ValidandoSalida: 3, nucleo.Completado: 4}

// Ruta dibuja la petición como un trayecto: cola, preparación, ejecución, validación y llegada. La parada de
// ahora va entre corchetes; al terminar, la llegada toma el resultado.
func Ruta(e *pagina.Estilos, lengua string, estado nucleo.Estado) string {
	terminado := estado.Terminal()
	llegada := datos.Texto(lengua, "ruta.llegada")
	if terminado && estado != nucleo.Completado {
		llegada = NombreEstado(lengua, estado)
	}
	paradas := []string{datos.Texto(lengua, "ruta.cola"), datos.Texto(lengua, "ruta.preparacion"), datos.Texto(lengua, "ruta.ejecucion"),
		datos.Texto(lengua, "ruta.validacion"), llegada}
	actual := posicionRuta[estado]
	if terminado {
		actual = 4
	}
	_, final := Icono(e, estado)
	partes := make([]string, len(paradas))
	for i, parada := range paradas {
		switch {
		case i == actual && terminado:
			partes[i] = final.Render("[" + parada + "]")
		case i == actual:
			partes[i] = e.Acento.Render("[" + parada + "]")
		case i < actual:
			partes[i] = e.Texto.Render(parada)
		default:
			partes[i] = e.Tenue.Render(parada)
		}
	}
	return strings.Join(partes, e.Tenue.Render(" "+e.Iconos.Cerrado+" "))
}

// Hace es «hace 3 min» de t visto desde ahora; pasada una semana, la fecha.
func Hace(ahora, t time.Time) string {
	d := ahora.Sub(t)
	switch {
	case t.IsZero():
		return idioma.T("Sin dato")
	case d < time.Minute:
		return idioma.Tf("hace %d s", max(int(d/time.Second), 0))
	case d < time.Hour:
		return idioma.Tf("hace %d min", int(d/time.Minute))
	case d < 24*time.Hour:
		return idioma.Tf("hace %d h", int(d/time.Hour))
	case d < 7*24*time.Hour:
		return idioma.Tf("hace %d d", int(d/(24*time.Hour)))
	}
	return t.Local().Format("2006-01-02")
}
