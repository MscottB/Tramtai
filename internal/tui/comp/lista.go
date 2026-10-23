// Package comp tiene las piezas que comparten las páginas de la terminal: lista, formulario, tabla, visor, selector,
// confirmación y avisos. Ninguna habla con el servicio.
package comp

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/pagina"
)

// Lista es una lista con búsqueda, filtro, orden y grupos plegables. Las teclas: ↑ ↓ (o k j), RePág, AvPág, Inicio y
// Fin mueven; en la cabecera de un grupo, Intro o espacio lo pliegan y despliegan, → lo despliega y ← lo pliega
// también desde uno de sus elementos. Intro sobre un elemento es de la página.
type Lista[T any] struct {
	// Fila pinta un elemento; si es nil, se pinta su texto.
	Fila func(T) string
	// Clave identifica un elemento, para que Poner deje la selección en el mismo aunque cambie de fila; si es nil,
	// se usa el texto.
	Clave func(T) string

	estilos  *pagina.Estilos
	texto    func(T) string
	todos    []T
	buscar   string
	filtro   func(T) bool
	orden    func(a, b T) int
	grupo    func(T) string
	plegados map[string]bool
	filas    []fila[T]
	cursor   int
	inicio   int
}

type fila[T any] struct {
	grupo    string
	cabecera bool
	n        int // en una cabecera, cuántos elementos tiene su grupo
	elem     T
}

// NuevaLista crea una lista; texto es lo que se busca en cada elemento y lo que se pinta si no hay Fila.
func NuevaLista[T any](e *pagina.Estilos, texto func(T) string) Lista[T] {
	return Lista[T]{estilos: e, texto: texto, plegados: map[string]bool{}}
}

// Poner cambia los elementos. La selección sigue en el mismo elemento (por su Clave) o en la misma cabecera de grupo;
// si ya no está, el cursor se queda en la misma fila, o en la última.
func (l *Lista[T]) Poner(elems []T) {
	clave := l.Clave
	if clave == nil {
		clave = l.texto
	}
	var antes func(fila[T]) bool
	if l.cursor < len(l.filas) {
		if f := l.filas[l.cursor]; f.cabecera {
			antes = func(g fila[T]) bool { return g.cabecera && g.grupo == f.grupo }
		} else {
			k := clave(f.elem)
			antes = func(g fila[T]) bool { return !g.cabecera && clave(g.elem) == k }
		}
	}
	l.todos = elems
	l.rehacer()
	if antes != nil {
		if i := slices.IndexFunc(l.filas, antes); i >= 0 {
			l.cursor = i
		}
	}
}

// Buscar deja los elementos cuyo texto contiene el buscado, sin distinguir mayúsculas ni tildes.
func (l *Lista[T]) Buscar(texto string) { l.buscar = nucleo.Plegar(texto); l.rehacer() }

// Filtrar deja los elementos para los que f es cierta; nil los deja todos.
func (l *Lista[T]) Filtrar(f func(T) bool) { l.filtro = f; l.rehacer() }

// Ordenar ordena con f (como slices.SortStableFunc); nil deja el orden de Poner.
func (l *Lista[T]) Ordenar(f func(a, b T) int) { l.orden = f; l.rehacer() }

// Agrupar reparte los elementos por el grupo que devuelve f, en el orden en que aparece cada uno; nil quita los grupos.
func (l *Lista[T]) Agrupar(f func(T) string) { l.grupo = f; l.rehacer() }

// Seleccion es el elemento del cursor; no hay si la lista está vacía o el cursor está en la cabecera de un grupo.
func (l Lista[T]) Seleccion() (T, bool) {
	if l.cursor < len(l.filas) && !l.filas[l.cursor].cabecera {
		return l.filas[l.cursor].elem, true
	}
	var cero T
	return cero, false
}

// Grupo es el grupo de la fila del cursor, sea su cabecera o uno de sus elementos; sin grupos o sin filas, no hay.
func (l Lista[T]) Grupo() (string, bool) {
	if l.grupo == nil || l.cursor >= len(l.filas) {
		return "", false
	}
	return l.filas[l.cursor].grupo, true
}

// Visibles son los elementos que se ven, en orden: los que pasan la búsqueda y el filtro, sin los de grupos plegados.
func (l Lista[T]) Visibles() []T {
	var r []T
	for _, f := range l.filas {
		if !f.cabecera {
			r = append(r, f.elem)
		}
	}
	return r
}

func (l *Lista[T]) rehacer() {
	var quedan []T
	for _, e := range l.todos {
		if (l.filtro == nil || l.filtro(e)) && strings.Contains(nucleo.Plegar(l.texto(e)), l.buscar) {
			quedan = append(quedan, e)
		}
	}
	if l.orden != nil {
		slices.SortStableFunc(quedan, l.orden)
	}
	l.filas = nil // nuevas: una copia de la lista (Selector la guarda por valor) no comparte filas
	if l.grupo == nil {
		for _, e := range quedan {
			l.filas = append(l.filas, fila[T]{elem: e})
		}
	} else {
		var grupos []string
		por := map[string][]T{}
		for _, e := range quedan {
			g := l.grupo(e)
			if _, ok := por[g]; !ok {
				grupos = append(grupos, g)
			}
			por[g] = append(por[g], e)
		}
		for _, g := range grupos {
			l.filas = append(l.filas, fila[T]{grupo: g, cabecera: true, n: len(por[g])})
			if !l.plegados[g] {
				for _, e := range por[g] {
					l.filas = append(l.filas, fila[T]{grupo: g, elem: e})
				}
			}
		}
	}
	l.cursor = min(l.cursor, max(len(l.filas)-1, 0))
}

func (l *Lista[T]) plegar(grupo string, plegado bool) {
	l.plegados[grupo] = plegado
	l.rehacer()
	l.cursor = max(slices.IndexFunc(l.filas, func(f fila[T]) bool { return f.cabecera && f.grupo == grupo }), 0)
}

// Update mueve el cursor y pliega grupos; dice si la tecla era suya.
func (l *Lista[T]) Update(msg tea.Msg) bool {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || len(l.filas) == 0 {
		return false
	}
	f := l.filas[l.cursor]
	switch k.String() {
	case "up", "k":
		l.cursor = max(l.cursor-1, 0)
	case "down", "j":
		l.cursor = min(l.cursor+1, len(l.filas)-1)
	case "pgup":
		l.cursor = max(l.cursor-10, 0)
	case "pgdown":
		l.cursor = min(l.cursor+10, len(l.filas)-1)
	case "home":
		l.cursor = 0
	case "end":
		l.cursor = len(l.filas) - 1
	case "enter", "space", " ":
		if !f.cabecera {
			return false
		}
		l.plegar(f.grupo, !l.plegados[f.grupo])
	case "right":
		if !f.cabecera {
			return false
		}
		l.plegar(f.grupo, false)
	case "left":
		if l.grupo == nil {
			return false
		}
		l.plegar(f.grupo, true)
	default:
		return false
	}
	return true
}

// Vista pinta las filas que caben en ancho×alto, con el cursor siempre a la vista.
func (l *Lista[T]) Vista(ancho, alto int) string {
	e := l.estilos
	if len(l.filas) == 0 {
		return e.Tenue.Render(e.Recortar(idioma.T("Sin resultados"), ancho))
	}
	alto = max(alto, 1)
	l.inicio = min(max(l.inicio, l.cursor-alto+1), l.cursor)
	l.inicio = max(min(l.inicio, len(l.filas)-alto), 0)
	var lineas []string
	for i := l.inicio; i < min(l.inicio+alto, len(l.filas)); i++ {
		f := l.filas[i]
		marca := "  "
		if i == l.cursor {
			marca = e.Iconos.Cursor + " "
		}
		var texto string
		switch {
		case f.cabecera:
			icono := e.Iconos.Abierto
			if l.plegados[f.grupo] {
				icono = e.Iconos.Cerrado
			}
			texto = e.Grupo.Render(fmt.Sprintf("%s %s (%d)", icono, f.grupo, f.n))
		case l.Fila != nil:
			texto = l.Fila(f.elem)
		default:
			texto = l.texto(f.elem)
		}
		if l.grupo != nil && !f.cabecera {
			texto = "  " + texto
		}
		linea := e.Recortar(marca+texto, ancho)
		if i == l.cursor {
			linea = e.Seleccion.Render(ansi.Strip(linea)) // sin los colores de dentro: su reinicio cortaría el fondo de la fila
		}
		lineas = append(lineas, linea)
	}
	return strings.Join(lineas, "\n")
}
