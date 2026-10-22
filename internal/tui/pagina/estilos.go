package pagina

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Estilos son los de la terminal; los rellena internal/tui según el fondo, NO_COLOR y TRAMTAI_ASCII. Con su valor cero
// (sin color) también se pinta bien: así lo usan las pruebas.
type Estilos struct {
	Oscuro, Color, ASCII bool

	Titulo      lipgloss.Style // el título de una página o de una caja
	Texto       lipgloss.Style
	Tenue       lipgloss.Style // lo secundario: detalles, ayudas, vacíos
	Acento      lipgloss.Style // lo que se quiere destacar: el nombre de la app, enlaces, la etiqueta con el foco
	Seleccion   lipgloss.Style // la fila elegida
	Exito       lipgloss.Style
	Advertencia lipgloss.Style
	Error       lipgloss.Style
	Tecla       lipgloss.Style // la tecla en el pie
	Grupo       lipgloss.Style // la cabecera de un grupo en el menú y en las listas

	Borde  lipgloss.Border
	Iconos Iconos
}

// Iconos son los caracteres de dibujo; con TRAMTAI_ASCII=1, o si la consola no es UTF-8, son ASCII.
type Iconos struct {
	Cursor, Abierto, Cerrado, Activo, Inactivo, Ok, Fallo, Separador, Elipsis, Vertical string
}

var (
	IconosUnicode = Iconos{Cursor: "▸", Abierto: "▾", Cerrado: "▸", Activo: "●", Inactivo: "○", Ok: "✓", Fallo: "✗", Separador: "·", Elipsis: "…", Vertical: "│"}
	IconosASCII   = Iconos{Cursor: ">", Abierto: "v", Cerrado: ">", Activo: "*", Inactivo: "o", Ok: "+", Fallo: "x", Separador: "-", Elipsis: "...", Vertical: "|"}
)

// Recortar deja texto (con o sin estilos) en ancho columnas, con la elipsis si no cabe. Mide con el ancho que se ve,
// no con len.
func (e *Estilos) Recortar(texto string, ancho int) string {
	return ansi.Truncate(texto, max(ancho, 0), e.Iconos.Elipsis)
}
