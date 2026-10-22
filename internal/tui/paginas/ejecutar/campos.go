package ejecutar

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"
)

// El modo campos de «Respuesta con forma»: cada campo son unas filas del formulario con la ruta en el
// id («c0.nombre», «c0.1.tipo» para el segundo subcampo del primero). La estructura (cuántos campos, de qué tipo) vive en
// p.campos y los valores en el formulario; cambiar un tipo o añadir y quitar campos rehace el formulario.

const (
	modoCampos = "campos"
	modoJSON   = "json"
)

var reEspacios = regexp.MustCompile(`\s+`)

// filasCampos son las filas de una lista de campos y de sus subcampos.
func (p *Pagina) filasCampos(campos []campoEsquema, ruta string, nivel int) []comp.Campo {
	var filas []comp.Campo
	for i, c := range campos {
		id := ruta + strconv.Itoa(i)
		et := func(clave string) string { return numeroCampo(id) + " · " + p.tx(clave) }
		tipos, elementos := tiposCampo, []string{"texto", "numero", "objeto"}
		if nivel >= nivelesCampos { // en el último nivel no caben los subcampos de un objeto
			tipos, elementos = tipos[:len(tipos)-1], elementos[:2]
		}
		filas = append(filas, comp.Texto(id+".nombre", et("campos.nombre"), c.Nombre),
			p.opcion(id+".tipo", et("campos.tipo"), tipos, p.nombres("campos.tipo.", tipos), c.Tipo),
			comp.Interruptor(id+".obligatorio", et("campos.obligatorio"), c.Obligatorio),
			comp.Texto(id+".descripcion", et("campos.descripcion"), c.Descripcion).Ejemplo(p.tx("campos.descripcionEjemplo")))
		switch c.Tipo {
		case "opcion":
			filas = append(filas, comp.Texto(id+".opciones", et("campos.opciones"), strings.Join(c.Opciones, ", ")))
		case "lista":
			filas = append(filas, p.opcion(id+".elemento", et("campos.elemento"), elementos, p.nombres("campos.elemento.", elementos), c.Elemento),
				comp.Numero(id+".minimo", et("campos.minimoElementos"), cifra(c.Minimo)), comp.Numero(id+".maximo", et("campos.maximoElementos"), cifra(c.Maximo)))
		case "numero", "entero":
			filas = append(filas, comp.Texto(id+".minimo", et("campos.minimo"), cifra(c.Minimo)), comp.Texto(id+".maximo", et("campos.maximo"), cifra(c.Maximo)))
		}
		if tieneSubcampos(c) {
			filas = append(filas, p.filasCampos(c.Campos, id+".", nivel+1)...)
		}
	}
	return filas
}

func tieneSubcampos(c campoEsquema) bool {
	return c.Tipo == "objeto" || (c.Tipo == "lista" && c.Elemento == "objeto")
}

func (p *Pagina) nombres(prefijo string, valores []string) []string {
	r := make([]string, len(valores))
	for i, v := range valores {
		r[i] = p.tx(prefijo + v)
	}
	return r
}

// numeroCampo es como se numera un campo en sus etiquetas: «c0.1» es «1.2».
func numeroCampo(id string) string {
	partes := strings.Split(strings.TrimPrefix(id, "c"), ".")
	for i, x := range partes {
		n, _ := strconv.Atoi(x)
		partes[i] = strconv.Itoa(n + 1)
	}
	return strings.Join(partes, ".")
}

func cifra(n *float64) string {
	if n == nil {
		return ""
	}
	return strconv.FormatFloat(*n, 'f', -1, 64)
}

// leerCampos saca los campos del formulario con la estructura que tienen. Un tipo o un elemento cambiado ya sale con lo que
// necesita: un objeto, con un campo vacío; una lista, de textos.
func (p *Pagina) leerCampos(estructura []campoEsquema, ruta string) []campoEsquema {
	f := &p.form
	campos := make([]campoEsquema, 0, len(estructura))
	for i, viejo := range estructura {
		id := ruta + strconv.Itoa(i)
		c := campoEsquema{Nombre: reEspacios.ReplaceAllString(f.Valor(id+".nombre"), "_"), Tipo: p.valorOpcion(id + ".tipo"),
			Obligatorio: f.Marcado(id + ".obligatorio"), Descripcion: f.Valor(id + ".descripcion")}
		switch viejo.Tipo {
		case "opcion":
			for _, o := range strings.Split(f.Valor(id+".opciones"), ",") {
				if o = strings.TrimSpace(o); o != "" {
					c.Opciones = append(c.Opciones, o)
				}
			}
		case "lista":
			c.Elemento = p.valorOpcion(id + ".elemento")
			c.Minimo, c.Maximo = numeroDe(f.Valor(id+".minimo")), numeroDe(f.Valor(id+".maximo"))
		case "numero", "entero":
			c.Minimo, c.Maximo = numeroDe(f.Valor(id+".minimo")), numeroDe(f.Valor(id+".maximo"))
		}
		if tieneSubcampos(viejo) {
			c.Campos = p.leerCampos(viejo.Campos, id+".")
		}
		if c.Tipo != viejo.Tipo {
			c = campoEsquema{Nombre: c.Nombre, Tipo: c.Tipo, Obligatorio: c.Obligatorio, Descripcion: c.Descripcion, Opciones: c.Opciones,
				Campos: c.Campos}
			if c.Tipo == "lista" {
				c.Elemento = "texto"
			}
		}
		switch {
		case !tieneSubcampos(c):
			c.Campos = nil
		case len(c.Campos) == 0:
			c.Campos = []campoEsquema{campoVacio()}
		}
		if c.Tipo != "opcion" {
			c.Opciones = nil
		}
		campos = append(campos, c)
	}
	return campos
}

// numeroDe es un límite escrito; nil si aún no es un número (mientras se escribe «-» o «0.»).
func numeroDe(texto string) *float64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(texto), 64)
	if err != nil {
		return nil
	}
	return &n
}

// forma resume la estructura de los campos: si cambia, el formulario tiene otras filas.
func forma(campos []campoEsquema) string {
	var b strings.Builder
	for _, c := range campos {
		b.WriteString(c.Tipo + ":" + c.Elemento + "(" + forma(c.Campos) + ")")
	}
	return b.String()
}

// rutaCampo es la ruta del campo de una fila («c0.1» de «c0.1.tipo»); no si la fila no es de un campo.
func rutaCampo(id string) (string, bool) {
	if len(id) < 2 || id[0] != 'c' || id[1] < '0' || id[1] > '9' {
		return "", false
	}
	return id[:strings.LastIndex(id, ".")], true
}

// enCampos dice si el foco está en el editor de campos (en una de sus filas o en «Editar con»).
func (p *Pagina) enCampos() bool {
	_, ok := rutaCampo(p.form.Foco())
	return p.form.Marcado("estructurada") && p.modo == modoCampos && (ok || p.form.Foco() == "modo")
}

// lista es la lista de campos en que está el campo de la ruta, y su posición en ella.
func lista(campos *[]campoEsquema, ruta string) (*[]campoEsquema, int) {
	partes := strings.Split(strings.TrimPrefix(ruta, "c"), ".")
	for _, x := range partes[:len(partes)-1] {
		n, _ := strconv.Atoi(x)
		campos = &(*campos)[n].Campos
	}
	i, _ := strconv.Atoi(partes[len(partes)-1])
	return campos, i
}

// padre es el prefijo de los ids de la lista de la ruta: «c» para «c2», «c0.» para «c0.1».
func padre(ruta string) string { return ruta[:strings.LastIndexAny(ruta, "c.")+1] }

// anadirCampo (n) añade un campo detrás del que tiene el foco, o al final si el foco está en «Editar con», y lo edita.
func (p *Pagina) anadirCampo() tea.Cmd {
	d := p.leer()
	nuevo := "c" + strconv.Itoa(len(d.campos))
	if ruta, ok := rutaCampo(p.form.Foco()); ok {
		l, i := lista(&d.campos, ruta)
		*l = slices.Insert(*l, i+1, campoVacio())
		nuevo = padre(ruta) + strconv.Itoa(i+1)
	} else {
		d.campos = append(d.campos, campoVacio())
	}
	p.armar(d, nuevo+".nombre")
	p.editando = true
	return nil
}

// moverCampo (Alt+↑, Alt+↓) cambia el campo con el foco de sitio en su lista; el foco lo sigue. En el primero no sube y en el último no baja.
func (p *Pagina) moverCampo(paso int) tea.Cmd {
	ruta, ok := rutaCampo(p.form.Foco())
	if !ok || !p.enCampos() {
		return nil
	}
	d := p.leer()
	l, i := lista(&d.campos, ruta)
	j := i + paso
	if j < 0 || j >= len(*l) {
		return nil
	}
	(*l)[i], (*l)[j] = (*l)[j], (*l)[i]
	p.armar(d, padre(ruta)+strconv.Itoa(j)+strings.TrimPrefix(p.form.Foco(), ruta))
	return nil
}

// verEjemplo (v) enseña una respuesta de muestra con la forma del esquema («Ver un ejemplo de respuesta»).
// Solo si los campos lo representan y alguno tiene nombre.
func (p *Pagina) verEjemplo() tea.Cmd {
	if c := p.camposConNombre(); len(c) > 0 {
		p.muestra = sangrar([]byte(ejemploDe(c)))
		p.caja.Poner(p.muestra)
	}
	return nil
}

// camposConNombre son los campos de primer nivel con nombre del esquema de ahora (también escrito en JSON, si los campos
// lo representan); nil sin salida estructurada.
func (p *Pagina) camposConNombre() []campoEsquema {
	d := p.leer()
	if !d.estructurada {
		return nil
	}
	campos, ok := d.campos, d.modo == modoCampos
	if !ok {
		campos, ok = camposDe(d.esquema)
	}
	var r []campoEsquema
	for _, c := range campos {
		if ok && c.Nombre != "" {
			r = append(r, c)
		}
	}
	return r
}

// pedirQuitar (x) pregunta antes de quitar el campo con el foco.
func (p *Pagina) pedirQuitar() tea.Cmd {
	ruta, ok := rutaCampo(p.form.Foco())
	if !ok {
		return nil
	}
	nombre := p.form.Valor(ruta + ".nombre")
	if nombre == "" {
		nombre = numeroCampo(ruta)
	}
	p.pend = func() tea.Cmd { p.quitarCampo(ruta); return nil }
	return comp.Confirmar(p.tx("campos.quitar", "campo", nombre))
}

func (p *Pagina) quitarCampo(ruta string) {
	d := p.leer()
	l, i := lista(&d.campos, ruta)
	*l = slices.Delete(*l, i, i+1)
	foco := "modo"
	switch {
	case len(*l) > 0:
		foco = padre(ruta) + strconv.Itoa(min(i, len(*l)-1)) + ".nombre"
	case padre(ruta) != "c":
		foco = strings.TrimSuffix(padre(ruta), ".") + ".nombre"
	}
	p.armar(d, foco)
}

// textoProblema es lo que dice la Web UI del primer problema de los campos; "" si no hay.
func (p *Pagina) textoProblema(campos []campoEsquema) string {
	if pr := problemaDeCampos(campos, ""); pr != (problemaCampo{}) {
		return p.tx("campos.problema."+pr.Problema, "campo", pr.Campo)
	}
	return ""
}

// pistaCampos explica la fila del editor de campos con el foco, con el problema de los campos delante si lo hay.
func (p *Pagina) pistaCampos(foco string) string {
	pista := idioma.T("n añade un campo detrás; x lo quita; Alt+↑↓ lo mueve; v enseña un ejemplo de respuesta")
	switch {
	case foco == "modo":
		pista = idioma.T("← → elige cómo se edita el esquema; n añade un campo; v enseña un ejemplo de respuesta")
	case strings.HasSuffix(foco, ".descripcion"):
		pista = p.tx("campos.descripcionEjemplo")
	case strings.HasSuffix(foco, ".opciones"):
		pista = idioma.T("Separa las opciones con comas")
	}
	if problema := p.textoProblema(p.leerCampos(p.campos, "c")); problema != "" {
		return problema + ". " + pista
	}
	return pista
}

// cambiarModo pasa el esquema de campos a JSON (siempre se puede) o de JSON a campos, si el editor lo representa; si no,
// se queda en JSON y lo avisa.
func (p *Pagina) cambiarModo(d datosForm, esquema, foco string) tea.Cmd {
	d.esquema, d.campos, p.errEsquema = esquema, nil, ""
	var cmd tea.Cmd
	if d.modo == modoCampos {
		c, ok := camposDe(esquema)
		switch {
		case ok:
			d.campos = c
		case esquemaValido(esquema):
			d.modo, cmd = modoJSON, comp.Aviso(p.tx("receta.esquemaAvanzado"), comp.Advertencia)
		default:
			d.modo, cmd = modoJSON, comp.Aviso(p.tx("receta.esquemaInvalido"), comp.Advertencia)
		}
	}
	p.armar(d, foco)
	return cmd
}

func esquemaValido(texto string) bool {
	_, err := esquemaDe(texto)
	return err == nil && strings.TrimSpace(texto) != ""
}
