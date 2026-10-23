package comp

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/idioma"
	"tramtai/internal/tui/pagina"
)

type tipoCampo int

const (
	campoTexto tipoCampo = iota
	campoArea
	campoOpcion
	campoInterruptor
	campoNumero
	campoPliegue
)

// Campo es un campo de un Formulario; se crea con Texto, Area, Opcion, Interruptor, Numero o Pliegue.
type Campo struct {
	ID, Etiqueta string
	tipo         tipoCampo
	opciones     []string
	opcion       int
	marcado      bool // en un pliegue, abierto
	en           string
	linea        textinput.Model
	area         textarea.Model
}

// Texto es un campo de una línea.
func Texto(id, etiqueta, valor string) Campo {
	c := Campo{ID: id, Etiqueta: etiqueta, tipo: campoTexto, linea: textinput.New()}
	c.linea.Prompt = ""
	c.linea.SetStyles(textinput.Styles{})
	c.linea.SetValue(valor)
	return c
}

// Numero es un campo de una línea que solo admite cifras; valor vacío es sin número.
func Numero(id, etiqueta, valor string) Campo {
	c := Texto(id, etiqueta, valor)
	c.tipo = campoNumero
	return c
}

// Area es un campo de varias líneas: Intro salta de línea, Ctrl+Enter envía y lo pegado entra entero.
func Area(id, etiqueta, valor string) Campo {
	c := Campo{ID: id, Etiqueta: etiqueta, tipo: campoArea, area: textarea.New()}
	c.area.Prompt = ""
	c.area.ShowLineNumbers = false
	c.area.SetStyles(textarea.Styles{})
	c.area.MaxHeight = 6           // lo que se ve
	c.area.MaxContentHeight = 9999 // lo que cabe, en líneas ya partidas
	c.area.SetHeight(6)
	c.area.SetValue(valor)
	c.area.MoveToBegin() // un texto que llega hecho (una receta, una plantilla) se ve desde el principio
	return c
}

// Opcion elige uno de opciones con ← →.
func Opcion(id, etiqueta string, opciones []string, valor string) Campo {
	return Campo{ID: id, Etiqueta: etiqueta, tipo: campoOpcion, opciones: opciones, opcion: max(slices.Index(opciones, valor), 0)}
}

// Interruptor se marca y desmarca con espacio o ← →.
func Interruptor(id, etiqueta string, valor bool) Campo {
	return Campo{ID: id, Etiqueta: etiqueta, tipo: campoInterruptor, marcado: valor}
}

// Ejemplo pone el texto tenue que enseña un campo de texto vacío (el placeholder de la Web UI).
func (c Campo) Ejemplo(texto string) Campo {
	c.linea.Placeholder, c.area.Placeholder = texto, texto
	return c
}

// marcar da a los campos de texto su marca de entrada (la barra vertical, en el acento con el foco) y el estilo del
// ejemplo: así un campo vacío no se confunde con una etiqueta suelta, también sin color.
func (c *Campo) marcar(e *pagina.Estilos) {
	marca := e.Iconos.Vertical + " "
	switch c.tipo {
	case campoArea:
		c.area.Prompt = marca
		c.area.SetStyles(textarea.Styles{Focused: textarea.StyleState{Prompt: e.Acento, Placeholder: e.Tenue},
			Blurred: textarea.StyleState{Prompt: e.Tenue, Placeholder: e.Tenue}})
	case campoTexto, campoNumero:
		c.linea.Prompt = marca
		c.linea.SetStyles(textinput.Styles{Focused: textinput.StyleState{Prompt: e.Acento, Placeholder: e.Tenue},
			Blurred: textinput.StyleState{Prompt: e.Tenue, Placeholder: e.Tenue}})
	}
}

// Pliegue es la cabecera de unas opciones plegadas, como un q-expansion-item de la Web UI: Intro, espacio o ← → lo abren y
// lo cierran. Sus campos se crean con Plegados y, con el pliegue cerrado, ni se ven ni reciben el foco.
func Pliegue(id, etiqueta string, abierto bool) Campo {
	return Campo{ID: id, Etiqueta: etiqueta, tipo: campoPliegue, marcado: abierto}
}

// Plegados mete esos campos en el pliegue de ese id.
func Plegados(pliegue string, campos ...Campo) []Campo {
	for i := range campos {
		campos[i].en = pliegue
	}
	return campos
}

func (c *Campo) valor() string {
	switch c.tipo {
	case campoArea:
		return c.area.Value()
	case campoOpcion:
		if c.opcion < len(c.opciones) {
			return c.opciones[c.opcion]
		}
		return ""
	case campoInterruptor:
		if c.marcado {
			return "1"
		}
		return ""
	}
	return c.linea.Value()
}

// filas es el alto de un área: lo que ocupa su texto partido al ancho, entre 2 y 6.
func (c *Campo) filas() int {
	n, ancho := 0, max(c.area.Width(), 1)
	for _, l := range strings.Split(c.area.Value(), "\n") {
		n += max((lipgloss.Width(l)+ancho-1)/ancho, 1)
	}
	return min(max(n, 2), 6)
}

func (c *Campo) escribe() bool {
	return c.tipo == campoTexto || c.tipo == campoNumero || c.tipo == campoArea
}

func (c *Campo) enfocar(si bool) {
	switch {
	case c.tipo == campoArea && si:
		c.area.Focus()
	case c.tipo == campoArea:
		c.area.Blur()
	case c.escribe() && si:
		c.linea.Focus()
	case c.escribe():
		c.linea.Blur()
	}
}

type (
	// Enviado lo responde un Formulario sin errores al enviarse.
	Enviado struct{}
	// Cancelado lo responden Formulario, Selector y Confirmacion con Esc.
	Cancelado struct{}
)

// Formulario son campos con validación al escribir. Tab, ↓ y ↑ (fuera de un área) cambian de campo; Intro pasa al
// siguiente y en el último envía; Ctrl+Enter (o Ctrl+S, para las terminales que no distinguen Ctrl+Enter) envía desde
// cualquiera; Esc cancela.
type Formulario struct {
	// Validar devuelve los errores por id de campo; se llama al escribir y al enviar. Un error solo se enseña en un
	// campo ya tocado o después de intentar enviar.
	Validar func(*Formulario) map[string]string

	estilos   *pagina.Estilos
	campos    []Campo
	foco      int
	errores   map[string]string
	tocados   map[string]bool
	intentado bool
	ancho     int
	lineaFoco int // la línea de la etiqueta con el foco en la última Vista
}

// NuevoFormulario crea un formulario con el foco en el primer campo.
func NuevoFormulario(e *pagina.Estilos, campos ...Campo) Formulario {
	f := Formulario{estilos: e, campos: campos, tocados: map[string]bool{}}
	for i := range f.campos {
		f.campos[i].marcar(e)
	}
	f.medir(40)
	if len(f.campos) > 0 {
		f.campos[0].enfocar(true)
	}
	return f
}

func (f *Formulario) campo(id string) *Campo {
	for i := range f.campos {
		if f.campos[i].ID == id {
			return &f.campos[i]
		}
	}
	return &Campo{}
}

// Valor es el texto de un campo; en una opción, la elegida; en un interruptor, "1" si está marcado.
func (f *Formulario) Valor(id string) string { return f.campo(id).valor() }

// Marcado dice si un interruptor está marcado o un pliegue abierto.
func (f *Formulario) Marcado(id string) bool { return f.campo(id).marcado }

// Entero es el valor de un campo Numero; 0 si está vacío.
func (f *Formulario) Entero(id string) int {
	n, _ := strconv.Atoi(f.Valor(id))
	return n
}

// Errores son los errores que se enseñan, por id de campo.
func (f *Formulario) Errores() map[string]string { return f.errores }

// Foco es el id del campo con el foco; "" sin campos.
func (f *Formulario) Foco() string {
	if len(f.campos) == 0 {
		return ""
	}
	return f.campos[f.foco].ID
}

// Enfocar pone el foco en el campo id (y abre su pliegue si estaba cerrado); si no existe, no cambia.
func (f *Formulario) Enfocar(id string) {
	for i, c := range f.campos {
		if c.ID == id {
			f.ir(i)
			return
		}
	}
}

// visible dice si un campo se ve: no está en un pliegue o su pliegue está abierto.
func (f *Formulario) visible(i int) bool {
	return f.campos[i].en == "" || f.campo(f.campos[i].en).marcado
}

// ir pone el foco en el campo i, abriendo su pliegue.
func (f *Formulario) ir(i int) {
	if en := f.campos[i].en; en != "" {
		f.campo(en).marcado = true
	}
	f.campos[f.foco].enfocar(false)
	f.foco = i
	f.campos[i].enfocar(true)
}

// Posicion es el número del campo con el foco entre los que se ven, y cuántos se ven: para el indicador «3/9».
func (f *Formulario) Posicion() (int, int) {
	n, actual := 0, 0
	for i := range f.campos {
		if f.visible(i) {
			n++
			if i == f.foco {
				actual = n
			}
		}
	}
	return actual, n
}

// Poner cambia el valor de un campo desde fuera (una plantilla, una receta cargada): en una opción elige ese valor y en
// un interruptor lo marca con "1". No cuenta como tocado.
func (f *Formulario) Poner(id, valor string) {
	c := f.campo(id)
	if c.ID != id || id == "" {
		return
	}
	switch c.tipo {
	case campoArea:
		c.area.SetValue(valor)
		c.area.MoveToBegin()
	case campoTexto, campoNumero:
		c.linea.SetValue(valor)
	case campoOpcion:
		if i := slices.Index(c.opciones, valor); i >= 0 {
			c.opcion = i
		}
	case campoInterruptor:
		c.marcado = valor != ""
	}
	f.validar()
}

// Revalidar vuelve a llamar a Validar: para un error que llega después de escribir (el que da el servicio).
func (f *Formulario) Revalidar() { f.validar() }

// LineaFoco es la línea en que la última Vista pintó la etiqueta del campo con el foco: la página que desplaza el
// formulario la mantiene a la vista.
func (f *Formulario) LineaFoco() int { return f.lineaFoco }

// EnCampo es cierto en cualquier campo, también en una opción o un interruptor: mientras la página enseña el
// formulario, devuelve esto en su EnCampo y así Tab, q y 1…7 son del formulario y no se escapan.
func (f *Formulario) EnCampo() bool { return len(f.campos) > 0 }

// Teclas son las del formulario, para el pie de la página.
func (f *Formulario) Teclas() []pagina.Tecla {
	return []pagina.Tecla{{Tecla: "Tab", Accion: idioma.T("siguiente campo")}, {Tecla: "Ctrl+Enter", Accion: idioma.T("envía")}, {Tecla: "Esc", Accion: idioma.T("cancela")}}
}

func (f *Formulario) medir(ancho int) {
	f.ancho = max(ancho, 10)
	for i := range f.campos {
		switch c := &f.campos[i]; c.tipo {
		case campoArea: // el ancho del área cuenta su marca
			c.area.SetWidth(f.ancho - 2)
		case campoTexto, campoNumero: // el sangrado (2), la marca (2) y el cursor (1)
			c.linea.SetWidth(f.ancho - 5)
		}
	}
}

func (f *Formulario) validar() {
	todos := map[string]string{}
	if f.Validar != nil {
		maps.Copy(todos, f.Validar(f))
	}
	f.errores = map[string]string{}
	for id, e := range todos {
		if f.intentado || f.tocados[id] {
			f.errores[id] = e
		}
	}
}

// mover pasa al campo visible que está paso campos más allá (paso 1 o -1, dando la vuelta).
func (f *Formulario) mover(paso int) {
	i := f.foco
	for range f.campos {
		if i = (i + paso + len(f.campos)) % len(f.campos); f.visible(i) {
			break
		}
	}
	f.ir(i)
}

func (f *Formulario) enviar() tea.Cmd {
	f.intentado = true
	f.validar()
	if len(f.errores) == 0 {
		return func() tea.Msg { return Enviado{} }
	}
	for i, c := range f.campos {
		if f.errores[c.ID] != "" {
			f.ir(i)
			break
		}
	}
	return nil
}

// Update lleva las teclas y lo pegado al campo con el foco. Con tea.WindowSizeMsg ajusta el ancho.
func (f *Formulario) Update(msg tea.Msg) tea.Cmd {
	if s, ok := msg.(tea.WindowSizeMsg); ok {
		f.medir(s.Width)
		return nil
	}
	if len(f.campos) == 0 {
		return nil
	}
	c := &f.campos[f.foco]
	antes := c.valor()
	if k, ok := msg.(tea.KeyPressMsg); ok {
		tecla := k.String()
		switch {
		case tecla == "ctrl+enter" || tecla == "ctrl+s":
			return f.enviar()
		case tecla == "esc":
			return func() tea.Msg { return Cancelado{} }
		case tecla == "tab" || (tecla == "down" && c.tipo != campoArea):
			f.mover(1)
			return nil
		case tecla == "shift+tab" || (tecla == "up" && c.tipo != campoArea):
			f.mover(-1)
			return nil
		case c.tipo == campoPliegue && (tecla == "enter" || tecla == "space" || tecla == " " || tecla == "left" || tecla == "right"):
			c.marcado = !c.marcado
			return nil
		case tecla == "enter" && c.tipo != campoArea:
			if f.foco == len(f.campos)-1 {
				return f.enviar()
			}
			f.mover(1)
			return nil
		case c.tipo == campoOpcion && (tecla == "left" || tecla == "right") && len(c.opciones) > 0:
			paso := map[string]int{"left": -1, "right": 1}[tecla]
			c.opcion = (c.opcion + paso + len(c.opciones)) % len(c.opciones)
		case c.tipo == campoInterruptor && (tecla == "space" || tecla == " " || tecla == "left" || tecla == "right"):
			c.marcado = !c.marcado
		case c.tipo == campoNumero && k.Text != "" && strings.Trim(k.Text, "0123456789") != "":
			return nil
		}
	}
	var cmd tea.Cmd
	switch c.tipo {
	case campoArea:
		c.area, cmd = c.area.Update(msg)
	case campoTexto, campoNumero:
		if p, ok := msg.(tea.PasteMsg); ok && c.tipo == campoNumero && strings.Trim(p.Content, "0123456789") != "" {
			return nil
		}
		c.linea, cmd = c.linea.Update(msg)
	}
	if c.valor() != antes {
		f.tocados[c.ID] = true
		f.validar()
	}
	return cmd
}

// Vista pinta los campos uno debajo de otro, cada uno con su etiqueta y su error.
func (f *Formulario) Vista() string {
	e := f.estilos
	var b strings.Builder
	for i := range f.campos {
		c := &f.campos[i]
		if !f.visible(i) {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		texto := c.Etiqueta
		if c.tipo == campoPliegue {
			marca := e.Iconos.Cerrado
			if c.marcado {
				marca = e.Iconos.Abierto
			}
			texto = marca + " " + c.Etiqueta
		}
		etiqueta := "  " + e.Texto.Render(texto)
		if i == f.foco {
			etiqueta = e.Acento.Render(e.Iconos.Cursor + " " + texto)
			f.lineaFoco = strings.Count(b.String(), "\n")
		}
		b.WriteString(e.Recortar(etiqueta, f.ancho))
		if c.tipo != campoPliegue {
			b.WriteString("\n")
		}
		switch c.tipo {
		case campoPliegue:
		case campoArea:
			if h := c.filas(); h != c.area.Height() { // crece con lo escrito, como el autogrow de la Web UI
				c.area.SetHeight(h)
			}
			b.WriteString(sangrar(c.area.View(), "  "))
		case campoOpcion:
			izq, der := "‹ ", " ›"
			if e.ASCII {
				izq, der = "< ", " >"
			}
			b.WriteString("  " + e.Recortar(izq+c.valor()+der, f.ancho-2))
		case campoInterruptor:
			marca := "[ ]"
			if c.marcado {
				marca = "[x]"
			}
			b.WriteString("  " + marca)
		default:
			b.WriteString("  " + c.linea.View())
		}
		if err := f.errores[c.ID]; err != "" {
			b.WriteString("\n  " + e.Error.Render(e.Recortar(err, f.ancho-2)))
		}
	}
	return b.String()
}

func sangrar(texto, margen string) string {
	return margen + strings.ReplaceAll(texto, "\n", "\n"+margen)
}
