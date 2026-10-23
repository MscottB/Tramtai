package pagina

import (
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
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
	Seleccion   lipgloss.Style
	Exito       lipgloss.Style
	Advertencia lipgloss.Style
	Error       lipgloss.Style
	Tecla       lipgloss.Style
	Grupo       lipgloss.Style // la cabecera de un grupo en el menú y en las listas

	Borde  lipgloss.Border
	Iconos Iconos
	// LineaOtra es el color de día y de noche de la línea de un agente sin color propio (linea-otro de DESIGN.md).
	LineaOtra [2]string
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

// AvisoError es el error de una petición, línea a línea: el título de su código (o el código, si la Web UI no lo
// conoce), el mensaje del servicio y qué hacer, que es la solución de la Ayuda para ese código. Las que faltan no van.
func (e *Estilos) AvisoError(lengua string, codigo nucleo.CodigoError, mensaje string) []string {
	titulo := e.Tx(lengua, "error."+string(codigo))
	if titulo == "error."+string(codigo) {
		titulo = string(codigo)
	}
	l := []string{titulo}
	if mensaje != "" {
		l = append(l, mensaje)
	}
	if paso := e.Tx(lengua, "ayuda.problemas."+string(codigo)); paso != "ayuda.problemas."+string(codigo) {
		l = append(l, paso)
	}
	return l
}

// Preambulo es lo que enseña una página en lugar de lo suyo: el aviso con el servicio caído o «Cargando…» mientras llega.
// Con ok falso, la página ya puede enseñar lo suyo.
func (e *Estilos) Preambulo(lengua string, ancho int, sinServicio, cargando bool) (texto string, ok bool) {
	switch {
	case sinServicio:
		return "\n" + e.Advertencia.Render(e.Recortar(e.A(idioma.T("El servicio no responde · r reintenta")), ancho)), true
	case cargando:
		return "\n" + e.Tenue.Render(e.Tx(lengua, "comun.cargando")), true
	}
	return "", false
}

// Periodo es lo que dura el historial con esas horas de retención, como periodoHistorial de la Web UI: «el último día» o
// «los últimos N días», con N redondeado a un decimal.
func (e *Estilos) Periodo(lengua string, horas int) string {
	dias := math.Round(float64(horas)/24*10) / 10
	if dias == 1 {
		return e.Tx(lengua, "comun.ultimoDia")
	}
	return e.Tx(lengua, "comun.ultimosDias", "dias", strconv.FormatFloat(dias, 'f', -1, 64))
}

// Numero escribe un número entero como toLocaleString de la Web UI, con el separador de miles del idioma («200.000»); en
// español, como Intl, no agrupa los de cuatro cifras.
func Numero(lengua string, n int) string {
	s := strconv.Itoa(max(n, -n))
	sep := "."
	if lengua == "en" {
		sep = ","
	} else if len(s) <= 4 {
		sep = ""
	}
	if sep != "" {
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + sep + s[i:]
		}
	}
	if n < 0 {
		return "-" + s
	}
	return s
}

// Envolver parte el texto por palabras, cada párrafo por su lado, en líneas de como mucho ancho columnas. Con n mayor que
// cero, en como mucho n líneas: lo que no cabe se corta con la elipsis en la última.
func (e *Estilos) Envolver(texto string, ancho, n int) []string {
	var lineas []string
	for _, parrafo := range strings.Split(texto, "\n") {
		for _, l := range strings.Split(ansi.Wrap(parrafo, max(ancho, 1), ""), "\n") {
			lineas = append(lineas, strings.TrimRight(l, " "))
		}
	}
	if n > 0 && len(lineas) > n {
		lineas[n-1] = e.Recortar(strings.Join(lineas[n-1:], " "), ancho)
		lineas = lineas[:n]
	}
	return lineas
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
		claro, oscuro = e.LineaOtra[0], e.LineaOtra[1]
	}
	return estilo.Foreground(lipgloss.LightDark(e.Oscuro)(lipgloss.Color(claro), lipgloss.Color(oscuro)))
}

// Recortar deja texto (con o sin estilos) en ancho columnas, con la elipsis si no cabe. Mide con el ancho que se ve,
// no con len.
func (e *Estilos) Recortar(texto string, ancho int) string {
	return ansi.Truncate(texto, max(ancho, 0), e.Iconos.Elipsis)
}
