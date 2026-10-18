package salidas

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// MaximoProblemas es cuántos problemas se guardan de una respuesta: con más, el resto no ayuda a corregirla.
const MaximoProblemas = 20

// Problema explica una razón por la que la respuesta no cumple el esquema. Campo es la ruta legible
// (tareas[2].fecha); vacío, la respuesta entera.
type Problema struct {
	Campo    string `json:"campo"`
	Problema string `json:"problema"` // falta, tipo, valor_no_permitido, sobra, formato, tamaño u otro
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
	return lista
}

func traducir(v *jsonschema.ValidationError, campo string) []Problema {
	uno := func(problema, texto string) []Problema { return []Problema{{Campo: campo, Problema: problema, Texto: texto}} }
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
		for _, w := range k.Want {
			valores = append(valores, valor(w))
		}
		return uno("valor_no_permitido", idioma.Tf("%s debe ser una de: %s", s, strings.Join(valores, ", ")))
	case *kind.Const:
		return uno("valor_no_permitido", idioma.Tf("%s debe ser %s", s, valor(k.Want)))
	case *kind.Format:
		return uno("formato", idioma.Tf("%s no tiene el formato %s", s, k.Want))
	case *kind.MinItems:
		return uno("tamaño", idioma.Tf("%s debe tener al menos %d elementos y tiene %d", s, k.Want, k.Got))
	case *kind.MaxItems:
		return uno("tamaño", idioma.Tf("%s debe tener como mucho %d elementos y tiene %d", s, k.Want, k.Got))
	case *kind.MinLength:
		return uno("tamaño", idioma.Tf("%s debe tener al menos %d caracteres y tiene %d", s, k.Want, k.Got))
	case *kind.MaxLength:
		return uno("tamaño", idioma.Tf("%s debe tener como mucho %d caracteres y tiene %d", s, k.Want, k.Got))
	case *kind.Minimum:
		return uno("tamaño", idioma.Tf("%s debe ser como mínimo %s", s, numero(k.Want.RatString())))
	case *kind.Maximum:
		return uno("tamaño", idioma.Tf("%s debe ser como máximo %s", s, numero(k.Want.RatString())))
	}
	return uno("otro", fmt.Sprintf("%s: %s", s, v.ErrorKind.LocalizedString(message.NewPrinter(language.English))))
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
