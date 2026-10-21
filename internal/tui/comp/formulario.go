package comp

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

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
)

// Campo es un campo de un Formulario; se crea con Texto, Area, Opcion, Interruptor o Numero.
type Campo struct {
	ID, Etiqueta string
	tipo         tipoCampo
	opciones     []string
	opcion       int
	marcado      bool
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
}

// NuevoFormulario crea un formulario con el foco en el primer campo.
func NuevoFormulario(e *pagina.Estilos, campos ...Campo) Formulario {
	f := Formulario{estilos: e, campos: campos, tocados: map[string]bool{}}
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

// Marcado dice si un interruptor está marcado.
func (f *Formulario) Marcado(id string) bool { return f.campo(id).marcado }

// Entero es el valor de un campo Numero; 0 si está vacío.
func (f *Formulario) Entero(id string) int {
	n, _ := strconv.Atoi(f.Valor(id))
	return n
}

// Errores son los errores que se enseñan, por id de campo.
func (f *Formulario) Errores() map[string]string { return f.errores }

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
		case campoArea:
			c.area.SetWidth(f.ancho - 2)
		case campoTexto, campoNumero:
			c.linea.SetWidth(f.ancho - 3)
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

func (f *Formulario) mover(paso int) {
	f.campos[f.foco].enfocar(false)
	f.foco = (f.foco + paso + len(f.campos)) % len(f.campos)
	f.campos[f.foco].enfocar(true)
}

func (f *Formulario) enviar() tea.Cmd {
	f.intentado = true
	f.validar()
	if len(f.errores) == 0 {
		return func() tea.Msg { return Enviado{} }
	}
	for i, c := range f.campos {
		if f.errores[c.ID] != "" {
			f.mover(i - f.foco)
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
		if i > 0 {
			b.WriteString("\n\n")
		}
		etiqueta := "  " + e.Texto.Render(c.Etiqueta)
		if i == f.foco {
			etiqueta = e.Acento.Render(e.Iconos.Cursor + " " + c.Etiqueta)
		}
		b.WriteString(e.Recortar(etiqueta, f.ancho) + "\n")
		switch c.tipo {
		case campoArea:
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
