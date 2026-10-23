// Package ayuda es la página Ayuda de la terminal: los mismos temas que la Ayuda de la Web UI y del sitio (los genera
// web/scripts/terminal-datos.ts en internal/tui/datos), con la búsqueda por palabras de la Web UI y, en cada tema, cómo se
// hace en la terminal. Lo que la terminal no pinta (capturas, diagramas, el recorrido por la pantalla) queda en «Ver en la
// web», que abre el tema en el sitio o, sin navegador, enseña su dirección. No usa el servicio: funciona sin conexión.
package ayuda

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/navegador"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
)

// abrir abre una URL en el navegador del sistema; las pruebas lo cambian para no abrir uno de verdad.
var abrir = navegador.Abrir

// msgWeb es lo que pasó al abrir url en el navegador.
type msgWeb struct {
	url string
	err error
}

// Pagina es la página Ayuda: la lista de temas, con su búsqueda, o un tema abierto en un visor.
type Pagina struct {
	c        pagina.Contexto
	lengua   string
	estilos  *pagina.Estilos
	temas    []datos.Tema
	lista    comp.Lista[datos.Tema]
	visor    comp.Visor
	abierto  *datos.Tema    // el tema que se ve; nil en la lista
	deLista  bool           // se abrió desde la lista: Esc vuelve a ella; si lo abrió el ? de una página, Esc vuelve a esa página
	teclas   []pagina.Tecla // las de la página cuyo ? abrió la Ayuda, para enseñarlas en su tema
	teclasDe string         // el tema de esas teclas
	buscando bool
	entrada  string // lo que se escribe en la búsqueda
	antes    string // la búsqueda de antes de abrirla: Esc la devuelve
	busqueda string // la búsqueda que filtra la lista
	url      string // tras w, la dirección que se quiso abrir, para copiarla: en Linux sin escritorio puede no abrirse sin error
	fallo    bool   // no se pudo abrir el navegador
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina { return &Pagina{lengua: lengua} }

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.ayuda") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaAyuda } // su ? lleva al inicio de la Ayuda
func (p *Pagina) EnCampo() bool  { return p.buscando }

func (p *Pagina) Teclas() []pagina.Tecla {
	switch {
	case p.buscando:
		return []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("aplica")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
	case p.abierto != nil:
		return []pagina.Tecla{{Tecla: "Esc", Accion: idioma.T("vuelve")}, {Tecla: "↑↓", Accion: idioma.T("desplaza")},
			{Tecla: "/", Accion: idioma.T("busca")}, {Tecla: "w", Accion: idioma.T("Ver en la web")}, {Tecla: "y", Accion: idioma.T("copia la dirección")}}
	}
	teclas := []pagina.Tecla{{Tecla: "Enter", Accion: idioma.T("ver")}, {Tecla: "/", Accion: idioma.T("busca")}}
	if p.destino() != "" {
		teclas = append(teclas, pagina.Tecla{Tecla: "w", Accion: idioma.T("Ver en la web")}, pagina.Tecla{Tecla: "y", Accion: idioma.T("copia la dirección")})
	}
	if strings.TrimSpace(p.busqueda) != "" {
		teclas = append(teclas, pagina.Tecla{Tecla: "Esc", Accion: idioma.T("quita la búsqueda")})
	}
	return teclas
}

// destino es la dirección que abren w e y: la del tema abierto o, en una búsqueda sin resultados, la de la Ayuda del
// sitio, que tiene su buscador. En la lista con resultados no hay.
func (p *Pagina) destino() string {
	switch {
	case p.abierto != nil:
		return p.abierto.URL
	case len(p.temas) > 0 && len(p.lista.Visibles()) == 0:
		return strings.TrimSuffix(p.temas[0].URL, p.temas[0].ID+".html")
	}
	return ""
}

// Iniciar empieza en la lista de temas, sin búsqueda. Los datos van en el binario: no hay nada que cargar.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	p.c, p.lengua, p.estilos = c, c.Idioma, c.Estilos
	p.temas = datos.Ayuda(c.Idioma)
	p.poblar(p.temas)
	p.abierto, p.teclas, p.teclasDe, p.buscando, p.entrada, p.antes, p.busqueda, p.url = nil, nil, "", false, "", "", "", ""
	return nil
}

// poblar pone esos temas en una lista nueva, con el cursor en el primero: tras buscar, en el mejor resultado.
func (p *Pagina) poblar(temas []datos.Tema) {
	p.lista = comp.NuevaLista(p.estilos, func(t datos.Tema) string { return t.Titulo })
	p.lista.Fila = p.fila
	p.lista.Poner(temas)
}

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
		p.poner()
	case pagina.ConAyuda:
		if i := slices.IndexFunc(p.temas, func(t datos.Tema) bool { return t.Pagina == m.Pagina }); i >= 0 {
			p.teclas, p.teclasDe = m.Teclas, p.temas[i].ID
			p.abrirTema(p.temas[i], false)
		}
	case msgWeb:
		if p.destino() != m.url { // ya se ve otra cosa
			return p, nil
		}
		p.url, p.fallo = m.url, m.err != nil
		p.poner()
		if m.err != nil {
			return p, nil
		}
		return p, comp.Aviso(idioma.T("Abierto en el navegador"), comp.Info)
	case tea.KeyPressMsg:
		return p, p.tecla(m)
	}
	return p, nil
}

// tecla atiende una tecla: la búsqueda abierta, el tema abierto o la lista. Esc cierra lo abierto y, sin nada, vuelve.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	t := k.String()
	if url := p.destino(); !p.buscando && url != "" {
		switch t {
		case "w":
			abrirURL := abrir
			return func() tea.Msg { return msgWeb{url: url, err: abrirURL(url)} }
		case "y":
			return comp.Copiar(url)
		}
	}
	switch {
	case p.buscando:
		p.teclaBusqueda(k)
	case p.abierto != nil:
		switch t {
		case "esc":
			if !p.deLista {
				return pagina.Atras
			}
			p.abierto, p.url = nil, ""
		case "/":
			p.abierto, p.url = nil, ""
			p.abrirBusqueda()
		default:
			p.visor.Update(k)
		}
	default:
		switch t {
		case "esc":
			if strings.TrimSpace(p.busqueda) == "" {
				return pagina.Atras
			}
			p.buscar("")
		case "/":
			p.abrirBusqueda()
		case "enter":
			if tm, ok := p.lista.Seleccion(); ok {
				p.abrirTema(tm, true)
			}
		default:
			p.lista.Update(k)
		}
	}
	return nil
}

// abrirBusqueda abre el campo con la búsqueda de ahora, para seguir escribiendo.
func (p *Pagina) abrirBusqueda() {
	p.buscando, p.antes, p.entrada = true, p.busqueda, p.busqueda
}

// teclaBusqueda: la lista se filtra mientras se escribe; Intro deja la búsqueda puesta y Esc devuelve la de antes.
func (p *Pagina) teclaBusqueda(k tea.KeyPressMsg) {
	switch k.String() {
	case "esc":
		p.buscando, p.entrada = false, p.antes
		p.buscar(p.antes)
	case "enter":
		p.buscando = false
	case "backspace":
		r := []rune(p.entrada)
		p.entrada = string(r[:max(len(r)-1, 0)])
		p.buscar(p.entrada)
	default:
		if k.Text != "" {
			p.entrada += k.Text
			p.buscar(p.entrada)
		}
	}
}

// abrirTema enseña un tema desde arriba.
func (p *Pagina) abrirTema(t datos.Tema, deLista bool) {
	p.abierto, p.deLista, p.url, p.buscando = &t, deLista, "", false
	p.visor = comp.NuevoVisor(p.estilos)
	p.poner()
}

// sufijos son las terminaciones que quita raiz, como en web/src/ayuda/buscar.ts.
var sufijos = regexp.MustCompile(`(aciones|acion|adas|ados|ada|ado|idas|idos|ida|ido|es|s)$`)

// normalizar es el de la Web UI: minúsculas, sin tildes y con el guion bajo de los códigos como espacio («cola llena»
// encuentra cola_llena).
func normalizar(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(nucleo.Plegar(s), "_", " ")), " ")
}

// raiz quita a una palabra larga su terminación más común, como la Web UI: «claves» encuentra «clave».
func raiz(palabra string) string {
	if utf8.RuneCountInString(palabra) <= 5 {
		return palabra
	}
	if corta := sufijos.ReplaceAllString(palabra, ""); utf8.RuneCountInString(corta) >= 4 {
		return corta
	}
	return palabra
}

type campo struct {
	texto string
	peso  int
}

// campos son los textos de un tema donde busca la Web UI, con su peso: el título, el resumen, los pasos, las notas, las
// filas de las tablas, las paradas de los diagramas, las preguntas con su respuesta y los errores con qué hacer.
func campos(t datos.Tema) []campo {
	c := []campo{{t.Titulo, 3}, {t.Resumen, 1}}
	for _, b := range t.Bloques {
		switch b.Tipo {
		case "pasos", "texto", "web":
			for _, l := range b.Lineas {
				c = append(c, campo{l, 1})
			}
		case "tabla":
			for _, f := range b.Filas {
				c = append(c, campo{strings.Join(f, " "), 1})
			}
		case "preguntas", "errores":
			for _, par := range b.Pares {
				c = append(c, campo{par[0], 2}, campo{par[1], 1})
			}
		}
	}
	return c
}

// buscar deja en la lista los temas con algún texto que tenga todas las palabras buscadas (por su raíz), en cualquier
// orden, como la Web UI; primero los que coinciden en lo que más pesa y, a igual peso, en el orden de la Ayuda.
func (p *Pagina) buscar(consulta string) {
	p.busqueda, p.url = consulta, ""
	palabras := strings.Fields(normalizar(consulta))
	if len(palabras) == 0 {
		p.poblar(p.temas)
		return
	}
	for i := range palabras {
		palabras[i] = raiz(palabras[i])
	}
	type resultado struct {
		tema   datos.Tema
		puntos int
	}
	var r []resultado
	for _, t := range p.temas {
		mejor := 0
		for _, c := range campos(t) {
			texto := normalizar(c.texto)
			if c.peso > mejor && !slices.ContainsFunc(palabras, func(w string) bool { return !strings.Contains(texto, w) }) {
				mejor = c.peso
			}
		}
		if mejor > 0 {
			r = append(r, resultado{t, mejor})
		}
	}
	slices.SortStableFunc(r, func(a, b resultado) int { return b.puntos - a.puntos })
	temas := make([]datos.Tema, len(r))
	for i, x := range r {
		temas[i] = x.tema
	}
	p.poblar(temas)
}
