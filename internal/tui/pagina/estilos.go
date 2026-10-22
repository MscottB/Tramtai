package pagina

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/tui/datos"
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

var textoAscii = strings.NewReplacer("·", "-", "…", "...", "’", "'", "“", "\"", "”", "\"", "—", "-", "→", "->", "«", "\"", "»", "\"",
	"‹", "<", "›", ">")

// A deja un texto sin los caracteres de dibujo que usan los textos de la Web UI (· … ’ « »…) cuando la consola es ASCII.
// Vale con e nil: antes de Iniciar, la página aún no tiene estilos.
func (e *Estilos) A(texto string) string {
	if e != nil && e.ASCII {
		return textoAscii.Replace(texto)
	}
	return texto
}

// Tx es un texto de la Web UI (datos.Textof) en esa lengua, con sus {marcadores} rellenos y pasado por A.
func (e *Estilos) Tx(lengua, clave string, pares ...string) string {
	return e.A(datos.Textof(lengua, clave, pares...))
}

// Rellenar completa texto con espacios hasta ancho columnas visibles: para alinear columnas.
func Rellenar(texto string, ancho int) string {
	return texto + strings.Repeat(" ", max(ancho-lipgloss.Width(texto), 0))
}

// Linea es el estilo de la línea de un agente (DESIGN.md: un color por agente, solo para identificarlo), con su color de
// día y de noche; sin uno propio, el de linea-otro. Sin color es negrita: lo identifican las dos letras de su código.
func (e *Estilos) Linea(claro, oscuro string) lipgloss.Style {
	estilo := lipgloss.NewStyle().Bold(true)
	if !e.Color {
		return estilo
	}
	if claro == "" || oscuro == "" {
		claro, oscuro = "#64748b", "#8a9aa8"
	}
	return estilo.Foreground(lipgloss.LightDark(e.Oscuro)(lipgloss.Color(claro), lipgloss.Color(oscuro)))
}

// Recortar deja texto (con o sin estilos) en ancho columnas, con la elipsis si no cabe. Mide con el ancho que se ve,
// no con len.
func (e *Estilos) Recortar(texto string, ancho int) string {
	return ansi.Truncate(texto, max(ancho, 0), e.Iconos.Elipsis)
}
