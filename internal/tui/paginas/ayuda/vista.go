package ayuda

import (
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
)

func (p *Pagina) tx(clave string, pares ...string) string {
	return p.estilos.Tx(p.lengua, clave, pares...)
}
func (p *Pagina) a(texto string) string { return p.estilos.A(texto) }

// Vista es la lista de temas, con la búsqueda arriba, o el tema abierto y, si no hubo navegador, su dirección debajo.
func (p *Pagina) Vista() string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	if p.abierto != nil {
		return strings.Join(append([]string{p.visor.Vista()}, p.abajo()...), "\n")
	}
	if len(p.lista.Visibles()) == 0 {
		lineas := []string{p.barra()}
		for _, l := range e.Envolver(p.tx("ayuda.sinResultados", "texto", strings.TrimSpace(p.busqueda)), ancho, max(p.c.Alto-1-len(p.abajo()), 1)) {
			lineas = append(lineas, e.Tenue.Render(l))
		}
		return strings.Join(append(lineas, p.abajo()...), "\n")
	}
	return p.barra() + "\n" + p.lista.Vista(ancho, max(p.c.Alto-1, 1))
}

// barra es la línea de arriba de la lista: lo que se escribe en la búsqueda, la búsqueda puesta con sus resultados o, sin
// búsqueda, de qué trata la Ayuda.
func (p *Pagina) barra() string {
	e, ancho := p.estilos, max(p.c.Ancho, 1)
	switch busqueda := strings.TrimSpace(p.busqueda); {
	case p.buscando && p.entrada == "":
		return e.Recortar(e.Acento.Render("/ ")+e.Tenue.Render(p.tx("ayuda.buscar")), ancho)
	case p.buscando:
		// Más ancha que la ventana, se ve el final: lo último escrito y el cursor.
		texto := "/ " + p.entrada + "_"
		if sobra := lipgloss.Width(texto) - ancho; sobra > 0 {
			texto = ansi.TruncateLeft(texto, sobra+lipgloss.Width(e.Iconos.Elipsis), e.Iconos.Elipsis)
		}
		return e.Acento.Render(texto)
	case busqueda != "":
		n := len(p.lista.Visibles())
		resultados := p.tx("ayuda.ui.resultados", "n", strconv.Itoa(n))
		if n == 1 {
			resultados = p.tx("ayuda.ui.resultadosUno")
		}
		return e.Tenue.Render(e.Recortar("/ "+busqueda+" "+e.Iconos.Separador+" "+resultados, ancho))
	}
	return e.Tenue.Render(e.Recortar(p.tx("ayuda.subtitulo"), ancho))
}

// fila es un tema en la lista: su título y, en tenue, su resumen.
func (p *Pagina) fila(t datos.Tema) string {
	return p.a(t.Titulo) + "  " + p.estilos.Tenue.Render(p.a(t.Resumen))
}

// abajo es, tras w, la dirección que se quiso abrir, para copiarla: también si el navegador dijo que sí, porque en Linux
// sin escritorio xdg-open arranca y falla después sin decir nada.
func (p *Pagina) abajo() []string {
	if p.url == "" {
		return nil
	}
	e := p.estilos
	aviso := idioma.T("Si no se abrió el navegador, la tecla y copia la dirección:")
	if p.fallo {
		aviso = idioma.T("No se pudo abrir el navegador; la tecla y copia la dirección:")
	}
	return append(p.parrafo(e.Advertencia, "", aviso), p.parrafo(e.Acento, "", p.url)...)
}

// poner escribe el tema abierto en el visor, al ancho de ahora, y le deja el alto que no ocupa la dirección.
func (p *Pagina) poner() {
	if p.abierto == nil || p.estilos == nil {
		return
	}
	p.visor.Tamano(max(p.c.Ancho, 1), max(p.c.Alto-len(p.abajo()), 1))
	p.visor.Poner(p.textoTema(*p.abierto))
}

// parrafo parte un texto en líneas que caben en el ancho, con el prefijo delante de la primera y su sangría en las demás.
func (p *Pagina) parrafo(estilo lipgloss.Style, prefijo, texto string) []string {
	sangria := strings.Repeat(" ", lipgloss.Width(prefijo))
	var r []string
	for i, l := range p.estilos.Envolver(p.a(texto), max(p.c.Ancho-len(sangria), 1), 0) {
		if i == 0 {
			r = append(r, prefijo+estilo.Render(l))
		} else {
			r = append(r, sangria+estilo.Render(l))
		}
	}
	return r
}

// verWeb es la línea que ocupa el sitio de lo que solo se ve en la web.
func (p *Pagina) verWeb() string {
	e := p.estilos
	return e.Tecla.Render("w") + " " + e.Tenue.Render(idioma.T("Ver en la web"))
}

// textoTema es el tema entero: su título, el resumen, cómo se hace en la terminal, las teclas de la página que pidió la
// Ayuda y sus bloques.
func (p *Pagina) textoTema(t datos.Tema) string {
	e := p.estilos
	l := append(p.parrafo(e.Titulo, "", t.Titulo), p.parrafo(e.Tenue, "", t.Resumen)...)
	if t.Terminal != "" {
		l = append(append(l, ""), p.parrafo(e.Acento, "", t.Terminal)...)
	}
	if t.ID == p.teclasDe && len(p.teclas) > 0 {
		ancho := 0
		for _, k := range p.teclas {
			ancho = max(ancho, lipgloss.Width(k.Tecla))
		}
		l = append(l, "", e.Titulo.Render(idioma.T("Teclas")))
		for _, k := range p.teclas {
			l = append(l, p.parrafo(e.Texto, e.Tecla.Render(k.Tecla)+strings.Repeat(" ", ancho-lipgloss.Width(k.Tecla)+2), k.Accion)...)
		}
	}
	for _, b := range t.Bloques {
		if r := p.bloque(b); len(r) > 0 {
			l = append(append(l, ""), r...)
		}
	}
	return strings.Join(l, "\n")
}

// bloque pinta un bloque del tema. El recorrido por la pantalla y las capturas son de la Web UI: quedan en «Ver en la
// web»; de un diagrama se ven sus paradas. Los bloques web sin texto (descargas, ejemplos, enlaces) no se pintan: w abre
// el tema entero.
func (p *Pagina) bloque(b datos.Bloque) []string {
	e := p.estilos
	var r []string
	switch b.Tipo {
	case "texto":
		for _, x := range b.Lineas {
			r = append(r, p.parrafo(e.Texto, "", x)...)
		}
	case "pasos":
		for i, x := range b.Lineas {
			r = append(r, p.parrafo(e.Texto, strconv.Itoa(i+1)+". ", x)...)
		}
	case "tabla":
		r = p.tabla(b)
	case "preguntas", "errores":
		for i, par := range b.Pares {
			if i > 0 {
				r = append(r, "")
			}
			respuesta := par[1]
			if b.Tipo == "errores" {
				r = append(r, p.parrafo(e.Advertencia, "", par[0])...)
				respuesta = p.tx("ayuda.ui.queHago") + " " + respuesta
			} else {
				r = append(r, p.parrafo(e.Titulo, "", par[0])...)
			}
			r = append(r, p.parrafo(e.Texto, "", respuesta)...)
		}
	case "pantalla":
		r = append(p.parrafo(e.Titulo, "", p.tx("ayuda.ui.recorridoTitulo")), p.verWeb())
	case "web":
		for _, x := range b.Lineas {
			r = append(r, p.parrafo(e.Texto, "", x)...)
		}
		if b.Titulo != "" {
			r = append(r, p.parrafo(e.Tenue, "", b.Titulo)...)
		}
		if len(r) > 0 {
			r = append(r, p.verWeb())
		}
	}
	return r
}

// tabla pinta las columnas si caben enteras; si no, cada fila en su párrafo, con el nombre de cada columna, para no cortar
// nada.
func (p *Pagina) tabla(b datos.Bloque) []string {
	e := p.estilos
	cabecera := make([]string, len(b.Lineas))
	for i, c := range b.Lineas {
		cabecera[i] = p.a(c)
	}
	filas := make([][]string, len(b.Filas))
	for i, f := range b.Filas {
		filas[i] = make([]string, len(f))
		for j, c := range f {
			filas[i][j] = p.a(c)
		}
	}
	if t := comp.Tabla(e, math.MaxInt32, cabecera, filas); lipgloss.Width(t) <= max(p.c.Ancho, 1) {
		return strings.Split(t, "\n")
	}
	var r []string
	for i, f := range b.Filas {
		if i > 0 {
			r = append(r, "")
		}
		for j, celda := range f {
			switch {
			case j == 0:
				r = append(r, p.parrafo(e.Titulo, "", celda)...)
			case j < len(b.Lineas):
				r = append(r, p.parrafo(e.Texto, "  ", b.Lineas[j]+": "+celda)...)
			}
		}
	}
	return r
}
