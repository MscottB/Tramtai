package ejecutar

import (
	"encoding/json"
	"io"
	"slices"
	"strings"
)

// Este archivo es web/src/esquemaCampos.ts en Go: convierte la lista de campos de «Respuesta con forma» en un JSON Schema
// y de vuelta. Solo representa lo que genera (objetos con additionalProperties: false, hasta 3 niveles); con cualquier otra
// cosa camposDe dice que no y el esquema se sigue editando en JSON, sin perder nada.

// campoEsquema es un campo del editor (Campo de esquemaCampos.ts). Tipo es texto, numero, entero, si_no, fecha, opcion,
// lista u objeto; Elemento (en una lista) es texto, numero u objeto; Campos son los de un objeto o de una lista de objetos.
type campoEsquema struct {
	Nombre, Tipo, Descripcion string
	Obligatorio               bool
	Opciones                  []string
	Elemento                  string
	Campos                    []campoEsquema
	Minimo, Maximo            *float64 // numero y entero (minimum, maximum); lista (minItems, maxItems)
}

const nivelesCampos = 3

var tiposCampo = []string{"texto", "numero", "entero", "si_no", "fecha", "opcion", "lista", "objeto"}

func campoVacio() campoEsquema { return campoEsquema{Tipo: "texto", Obligatorio: true} }

// aEsquema genera el esquema de un objeto con esos campos, compacto y con las claves en el orden de la Web UI.
func aEsquema(campos []campoEsquema) string { return "{" + cuerpoObjeto(campos) + "}" }

func cuerpoObjeto(campos []campoEsquema) string {
	// Como Object.fromEntries: un nombre repetido se queda en su primer sitio con el último campo.
	var nombres []string
	ultimo := map[string]campoEsquema{}
	var requeridos []string
	for _, c := range campos {
		if _, visto := ultimo[c.Nombre]; !visto {
			nombres = append(nombres, c.Nombre)
		}
		ultimo[c.Nombre] = c
		if c.Obligatorio {
			requeridos = append(requeridos, c.Nombre)
		}
	}
	props := make([]string, len(nombres))
	for i, n := range nombres {
		props[i] = jsonDe(n, "") + ":" + propiedad(ultimo[n])
	}
	cuerpo := `"type":"object","properties":{` + strings.Join(props, ",") + "}"
	if len(requeridos) > 0 {
		cuerpo += `,"required":` + jsonDe(requeridos, "")
	}
	return cuerpo + `,"additionalProperties":false`
}

func propiedad(c campoEsquema) string {
	var partes []string
	switch c.Tipo {
	case "texto":
		partes = append(partes, `"type":"string"`)
	case "fecha":
		partes = append(partes, `"type":"string"`, `"format":"date"`)
	case "numero":
		partes = append(partes, `"type":"number"`)
	case "entero":
		partes = append(partes, `"type":"integer"`)
	case "si_no":
		partes = append(partes, `"type":"boolean"`)
	case "opcion":
		opciones := []string{}
		for _, o := range c.Opciones {
			if strings.TrimSpace(o) != "" {
				opciones = append(opciones, o)
			}
		}
		partes = append(partes, `"type":"string"`, `"enum":`+jsonDe(opciones, ""))
	case "objeto":
		partes = append(partes, cuerpoObjeto(c.Campos))
	case "lista":
		items := `{"type":"string"}`
		switch c.Elemento {
		case "objeto":
			items = aEsquema(c.Campos)
		case "numero":
			items = `{"type":"number"}`
		}
		partes = append(partes, `"type":"array"`, `"items":`+items)
	}
	if c.Tipo == "numero" || c.Tipo == "entero" || c.Tipo == "lista" {
		minimo, maximo := "minimum", "maximum"
		if c.Tipo == "lista" {
			minimo, maximo = "minItems", "maxItems"
		}
		if c.Minimo != nil {
			partes = append(partes, jsonDe(minimo, "")+":"+jsonDe(*c.Minimo, ""))
		}
		if c.Maximo != nil {
			partes = append(partes, jsonDe(maximo, "")+":"+jsonDe(*c.Maximo, ""))
		}
	}
	if c.Descripcion != "" {
		partes = append(partes, `"description":`+jsonDe(c.Descripcion, ""))
	}
	return "{" + strings.Join(partes, ",") + "}"
}

// camposDe rehace los campos de un esquema escrito; no si no es JSON o usa algo que el editor no representa.
func camposDe(texto string) ([]campoEsquema, bool) {
	v, err := leerJSON(texto)
	if err != nil {
		return nil, false
	}
	return desdeEsquema(v, 1, false)
}

// objeto es un objeto JSON con sus claves en orden: el orden de los campos es el de las propiedades.
type objeto []par

type par struct {
	clave string
	valor any
}

func (o objeto) valor(clave string) (any, bool) {
	for _, p := range o {
		if p.clave == clave {
			return p.valor, true
		}
	}
	return nil, false
}

func (o objeto) solo(permitidas ...string) bool {
	for _, p := range o {
		if !slices.Contains(permitidas, p.clave) {
			return false
		}
	}
	return true
}

// leerJSON lee un JSON con los objetos en orden (objeto), las listas como []any y los números como json.Number.
func leerJSON(texto string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(texto))
	dec.UseNumber()
	v, err := leerValor(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errEsquema
	}
	return v, nil
}

func leerValor(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		o := objeto{}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := leerValor(dec)
			if err != nil {
				return nil, err
			}
			if i := slices.IndexFunc(o, func(p par) bool { return p.clave == k }); i >= 0 { // como JSON.parse: gana el último
				o[i].valor = v
			} else {
				o = append(o, par{k.(string), v})
			}
		}
		_, err := dec.Token()
		return o, err
	case json.Delim('['):
		l := []any{}
		for dec.More() {
			v, err := leerValor(dec)
			if err != nil {
				return nil, err
			}
			l = append(l, v)
		}
		_, err := dec.Token()
		return l, err
	}
	return tok, nil
}

// desdeEsquema rehace los campos de un esquema. conDescripcion: el objeto es el valor de un campo, que guarda su
// descripción; en la raíz o en los elementos de una lista se perdería.
func desdeEsquema(v any, nivel int, conDescripcion bool) ([]campoEsquema, bool) {
	o, ok := v.(objeto)
	if !ok || nivel > nivelesCampos {
		return nil, false
	}
	permitidas := []string{"type", "properties", "required", "additionalProperties"}
	if conDescripcion {
		permitidas = append(permitidas, "description")
	}
	tipo, _ := o.valor("type")
	adicionales, _ := o.valor("additionalProperties")
	props, _ := o.valor("properties")
	propiedades, ok := props.(objeto)
	if !o.solo(permitidas...) || tipo != "object" || adicionales != false || !ok {
		return nil, false
	}
	var requeridos []string
	if r, hay := o.valor("required"); hay {
		lista, ok := r.([]any)
		if !ok {
			return nil, false
		}
		for _, x := range lista {
			s, ok := x.(string)
			if _, existe := propiedades.valor(s); !ok || !existe {
				return nil, false
			}
			requeridos = append(requeridos, s)
		}
	}
	campos := make([]campoEsquema, 0, len(propiedades))
	for _, p := range propiedades {
		c, ok := campoDe(p.clave, p.valor, slices.Contains(requeridos, p.clave), nivel)
		if !ok {
			return nil, false
		}
		campos = append(campos, c)
	}
	return campos, true
}

func campoDe(nombre string, v any, obligatorio bool, nivel int) (campoEsquema, bool) {
	p, ok := v.(objeto)
	if !ok {
		return campoEsquema{}, false
	}
	c := campoEsquema{Nombre: nombre, Tipo: "texto", Obligatorio: obligatorio}
	if d, hay := p.valor("description"); hay {
		if c.Descripcion, ok = d.(string); !ok {
			return c, false
		}
	}
	// rango copia el mínimo y el máximo; no si alguno no es un número.
	rango := func(minimo, maximo string) bool {
		for clave, destino := range map[string]**float64{minimo: &c.Minimo, maximo: &c.Maximo} {
			x, hay := p.valor(clave)
			if !hay {
				continue
			}
			n, ok := x.(json.Number)
			f, err := n.Float64()
			if !ok || err != nil {
				return false
			}
			*destino = &f
		}
		return true
	}
	tipo, _ := p.valor("type")
	switch tipo {
	case "string":
		if f, _ := p.valor("format"); f == "date" && p.solo("type", "format", "description") {
			c.Tipo = "fecha"
			return c, true
		}
		if e, _ := p.valor("enum"); p.solo("type", "enum", "description") {
			if l, ok := e.([]any); ok {
				c.Opciones = make([]string, 0, len(l))
				for _, x := range l {
					s, ok := x.(string)
					if !ok {
						return c, false
					}
					c.Opciones = append(c.Opciones, s)
				}
				c.Tipo = "opcion"
				return c, true
			}
		}
		return c, p.solo("type", "description")
	case "number", "integer":
		c.Tipo = map[any]string{"number": "numero", "integer": "entero"}[tipo]
		return c, p.solo("type", "minimum", "maximum", "description") && rango("minimum", "maximum")
	case "boolean":
		c.Tipo = "si_no"
		return c, p.solo("type", "description")
	case "object":
		c.Tipo = "objeto"
		c.Campos, ok = desdeEsquema(p, nivel+1, true)
		return c, ok
	case "array":
		items, _ := p.valor("items")
		elementos, ok := items.(objeto)
		if !p.solo("type", "items", "minItems", "maxItems", "description") || !rango("minItems", "maxItems") || !ok {
			return c, false
		}
		c.Tipo = "lista"
		switch t, _ := elementos.valor("type"); {
		case t == "object":
			c.Elemento = "objeto"
			c.Campos, ok = desdeEsquema(elementos, nivel+1, false)
			return c, ok
		case (t == "string" || t == "number") && elementos.solo("type"):
			c.Elemento = map[any]string{"string": "texto", "number": "numero"}[t]
			return c, true
		}
	}
	return c, false
}

// problemaCampo es el primer campo que impide generar un esquema útil (ProblemaCampo); vacío si no hay ninguno.
type problemaCampo struct{ Campo, Problema string }

func problemaDeCampos(campos []campoEsquema, ruta string) problemaCampo {
	vistos := map[string]bool{}
	for _, c := range campos {
		aqui := c.Nombre
		if ruta != "" {
			aqui = ruta + "." + c.Nombre
		}
		sub := c.Tipo == "objeto" || (c.Tipo == "lista" && c.Elemento == "objeto")
		switch {
		case strings.TrimSpace(c.Nombre) == "":
			return problemaCampo{ruta, "sin_nombre"}
		case vistos[c.Nombre]:
			return problemaCampo{aqui, "repetido"}
		case c.Tipo == "opcion" && !slices.ContainsFunc(c.Opciones, func(o string) bool { return strings.TrimSpace(o) != "" }):
			return problemaCampo{aqui, "sin_opciones"}
		// Con el mínimo por encima del máximo ninguna respuesta cumple: cada corrección se gastaría en balde.
		case c.Minimo != nil && c.Maximo != nil && *c.Minimo > *c.Maximo:
			return problemaCampo{aqui, "rango"}
		case sub && len(c.Campos) == 0:
			return problemaCampo{aqui, "sin_campos"}
		}
		vistos[c.Nombre] = true
		if sub {
			if p := problemaDeCampos(c.Campos, aqui); p != (problemaCampo{}) {
				return p
			}
		}
	}
	return problemaCampo{}
}
