package salidas

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// MaximoProblemas es cuántos problemas se guardan de una respuesta: con más, el resto no ayuda a corregirla.
// maximoTexto (en caracteres) y maximoValores acotan cada texto: van en el evento y en el mensaje de corrección.
const (
	MaximoProblemas = 20
	maximoTexto     = 300
	maximoValores   = 10
)

// Problema explica una razón por la que la respuesta no cumple el esquema. Campo es la ruta legible
// (tareas[2].fecha); vacío, la respuesta entera.
type Problema struct {
	Campo    string `json:"campo"`
	Problema string `json:"problema" ts:"'falta' | 'tipo' | 'valor_no_permitido' | 'sobra' | 'formato' | 'tamaño' | 'otro'"`
	Texto    string `json:"texto"`
}

// ErrorSalida es salida_no_cumple_esquema con la lista de problemas; nucleo.CodigoDe y MensajeDe lo reconocen.
type ErrorSalida struct {
	Base      *nucleo.Error
	Problemas []Problema
}

func (e *ErrorSalida) Error() string { return e.Base.Error() }
func (e *ErrorSalida) Unwrap() error { return e.Base }

// ProblemasDe devuelve los problemas de un error de Validar, o nil si no los tiene.
func ProblemasDe(err error) []Problema {
	var e *ErrorSalida
	if errors.As(err, &e) {
		return e.Problemas
	}
	return nil
}

// NoCumple es salida_no_cumple_esquema con esos problemas (al menos uno), como el error de Validar.
func NoCumple(problemas []Problema) error { return errorSalida(problemas) }

func errorSalida(problemas []Problema) *ErrorSalida {
	mensaje := problemas[0].Texto
	if len(problemas) > 1 {
		mensaje += idioma.Tf(" (y %d más)", len(problemas)-1)
	}
	return &ErrorSalida{Base: nucleo.NuevoError(nucleo.CodigoSalidaNoCumpleEsquema, recortar(mensaje)), Problemas: problemas}
}

// problemasDe recorre el error de la biblioteca hasta las hojas y traduce cada una. instancia sirve para saber qué
// partes de la ruta son índices de una lista.
func problemasDe(v *jsonschema.ValidationError, instancia any) []Problema {
	var lista []Problema
	var recorrer func(*jsonschema.ValidationError)
	recorrer = func(v *jsonschema.ValidationError) {
		if len(lista) >= MaximoProblemas {
			return
		}
		switch v.ErrorKind.(type) {
		case *kind.AnyOf, *kind.OneOf:
			// Las opciones fallan cada una por su lado: sus problemas se contradicen («falta a» y «falta b»), así que
			// se explican como uno solo del campo.
			lista = append(lista, opciones(v, ruta(v.InstanceLocation, instancia)))
			return
		}
		if len(v.Causes) > 0 {
			for _, c := range v.Causes {
				recorrer(c)
			}
			return
		}
		lista = append(lista, traducir(v, ruta(v.InstanceLocation, instancia))...)
	}
	recorrer(v)
	if len(lista) > MaximoProblemas {
		lista = lista[:MaximoProblemas]
	}
	for i := range lista {
		lista[i].Texto = acortar(lista[i].Texto, maximoTexto)
	}
	return lista
}

// opciones explica un anyOf o un oneOf que no se cumple. Si todas las opciones solo piden un tipo (como texto o null),
// dice qué tipos valen; si no, que debe cumplir una de las opciones.
func opciones(v *jsonschema.ValidationError, campo string) Problema {
	s := sujeto(campo)
	var tipos []string
	got := ""
	var hojas func(*jsonschema.ValidationError) bool
	hojas = func(c *jsonschema.ValidationError) bool {
		if len(c.Causes) > 0 {
			for _, h := range c.Causes {
				if !hojas(h) {
					return false
				}
			}
			return true
		}
		t, ok := c.ErrorKind.(*kind.Type)
		if !ok || !slices.Equal(c.InstanceLocation, v.InstanceLocation) {
			return false
		}
		got = t.Got
		for _, w := range t.Want {
			if nombre := nombreTipo(w); !slices.Contains(tipos, nombre) {
				tipos = append(tipos, nombre)
			}
		}
		return true
	}
	if len(v.Causes) > 0 && hojas(v) {
		return Problema{Campo: campo, Problema: "tipo", Texto: idioma.Tf("%s debería ser %s y llegó %s", s, strings.Join(tipos, idioma.T(" o ")), nombreTipo(got))}
	}
	if _, varias := v.ErrorKind.(*kind.OneOf); varias && len(v.Causes) == 0 {
		return Problema{Campo: campo, Problema: "otro", Texto: idioma.Tf("%s cumple más de una de las opciones del esquema y debe cumplir solo una", s)}
	}
	return Problema{Campo: campo, Problema: "otro", Texto: idioma.Tf("%s no cumple ninguna de las opciones del esquema", s)}
}

// acortar deja un texto en n caracteres como mucho, sin partir ninguno.
func acortar(texto string, n int) string {
	if r := []rune(texto); len(r) > n {
		return string(r[:n]) + "…"
	}
	return texto
}

func traducir(v *jsonschema.ValidationError, campo string) []Problema {
	uno := func(problema, texto string) []Problema {
		return []Problema{{Campo: campo, Problema: problema, Texto: texto}}
	}
	s := sujeto(campo)
	switch k := v.ErrorKind.(type) {
	case *kind.Required:
		var lista []Problema
		for _, falta := range k.Missing {
			c := hijo(campo, falta)
			lista = append(lista, Problema{Campo: c, Problema: "falta", Texto: idioma.Tf("Falta el campo `%s`", c)})
		}
		return lista
	case *kind.AdditionalProperties:
		var lista []Problema
		for _, sobra := range k.Properties {
			c := hijo(campo, sobra)
			lista = append(lista, Problema{Campo: c, Problema: "sobra", Texto: idioma.Tf("Sobra el campo `%s`", c)})
		}
		return lista
	case *kind.Type:
		var quiere []string
		for _, t := range k.Want {
			quiere = append(quiere, nombreTipo(t))
		}
		return uno("tipo", idioma.Tf("%s debería ser %s y llegó %s", s, strings.Join(quiere, idioma.T(" o ")), nombreTipo(k.Got)))
	case *kind.Enum:
		var valores []string
		for _, w := range k.Want[:min(len(k.Want), maximoValores)] {
			valores = append(valores, valor(w))
		}
		if len(k.Want) > maximoValores {
			valores = append(valores, "…")
		}
		return uno("valor_no_permitido", idioma.Tf("%s debe ser una de: %s", s, strings.Join(valores, ", ")))
	case *kind.Const:
		return uno("valor_no_permitido", idioma.Tf("%s debe ser %s", s, valor(k.Want)))
	case *kind.Format:
		return uno("formato", idioma.Tf("%s no tiene el formato %s", s, k.Want))
	case *kind.MinItems:
		if k.Want == 1 {
			return uno("tamaño", idioma.Tf("%s debe tener al menos un elemento", s))
		}
		return uno("tamaño", idioma.Tf("%s debe tener al menos %d elementos y tiene %d", s, k.Want, k.Got))
	case *kind.MaxItems:
		if k.Want == 1 {
			return uno("tamaño", idioma.Tf("%s debe tener como mucho un elemento y tiene %d", s, k.Got))
		}
		return uno("tamaño", idioma.Tf("%s debe tener como mucho %d elementos y tiene %d", s, k.Want, k.Got))
	case *kind.MinLength:
		if k.Want == 1 {
			return uno("tamaño", idioma.Tf("%s no puede estar vacío", s))
		}
		return uno("tamaño", idioma.Tf("%s debe tener al menos %d caracteres y tiene %d", s, k.Want, k.Got))
	case *kind.MaxLength:
		if k.Want == 1 {
			return uno("tamaño", idioma.Tf("%s debe tener como mucho un carácter y tiene %d", s, k.Got))
		}
		return uno("tamaño", idioma.Tf("%s debe tener como mucho %d caracteres y tiene %d", s, k.Want, k.Got))
	case *kind.Minimum:
		return uno("tamaño", idioma.Tf("%s debe ser como mínimo %s", s, numero(k.Want.RatString())))
	case *kind.Maximum:
		return uno("tamaño", idioma.Tf("%s debe ser como máximo %s", s, numero(k.Want.RatString())))
	}
	// El resto se explica con la regla del esquema que no se cumple: la biblioteca solo tiene sus textos en inglés.
	return uno("otro", idioma.Tf("%s no cumple la regla «%s» del esquema", s, strings.Join(v.ErrorKind.KeywordPath(), "/")))
}

// ruta convierte la ubicación de la biblioteca (["tareas","2","fecha"]) en tareas[2].fecha; un número solo es índice
// si en la respuesta esa parte es una lista.
func ruta(ubicacion []string, instancia any) string {
	var b strings.Builder
	actual := instancia
	for _, parte := range ubicacion {
		switch v := actual.(type) {
		case []any:
			b.WriteString("[" + parte + "]")
			if i, err := strconv.Atoi(parte); err == nil && i >= 0 && i < len(v) {
				actual = v[i]
			} else {
				actual = nil
			}
		default:
			if b.Len() > 0 {
				b.WriteString(".")
			}
			b.WriteString(parte)
			if m, ok := v.(map[string]any); ok {
				actual = m[parte]
			} else {
				actual = nil
			}
		}
	}
	return b.String()
}

func hijo(campo, nombre string) string {
	if campo == "" {
		return nombre
	}
	return campo + "." + nombre
}

func sujeto(campo string) string {
	if campo == "" {
		return idioma.T("La respuesta")
	}
	return "`" + campo + "`"
}

func nombreTipo(t string) string {
	switch t {
	case "string":
		return idioma.T("un texto")
	case "number":
		return idioma.T("un número")
	case "integer":
		return idioma.T("un número entero")
	case "boolean":
		return idioma.T("sí o no (true/false)")
	case "object":
		return idioma.T("un objeto")
	case "array":
		return idioma.T("una lista")
	case "null":
		return "null"
	}
	return t
}

func valor(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// numero muestra 5/2 como 2.5; los racionales sin decimal exacto se quedan como fracción.
func numero(r string) string {
	if a, b, ok := strings.Cut(r, "/"); ok {
		x, err1 := strconv.ParseFloat(a, 64)
		y, err2 := strconv.ParseFloat(b, 64)
		if err1 == nil && err2 == nil && y != 0 {
			return strconv.FormatFloat(x/y, 'f', -1, 64)
		}
	}
	return r
}
