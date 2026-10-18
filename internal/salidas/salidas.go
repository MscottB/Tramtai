// Package salidas valida las salidas estructuradas contra el JSON Schema del consumidor.
package salidas

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

const maximoMensaje = 1000

// sinCargaExterna impide que un $ref lea archivos locales o URLs: el esquema llega del consumidor.
type sinCargaExterna struct{}

func (sinCargaExterna) Load(url string) (any, error) {
	return nil, errors.New(idioma.Tf("referencias externas deshabilitadas: %s", url))
}

// Compilar comprueba que el esquema es válido; un esquema inválido es una solicitud inválida.
func Compilar(esquema []byte) (*jsonschema.Schema, error) {
	documento, err := jsonschema.UnmarshalJSON(bytes.NewReader(esquema))
	if err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("el esquema no es JSON válido: %s", err.Error()))
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(sinCargaExterna{})
	c.AssertFormat() // las fechas del editor de campos (format: date) se comprueban de verdad
	if err := c.AddResource("tramtai:esquema", documento); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("esquema no válido: %s", err.Error()))
	}
	compilado, err := c.Compile("tramtai:esquema")
	if err != nil {
		motivo := err.Error()
		var sv *jsonschema.SchemaValidationError
		var v *jsonschema.ValidationError
		if errors.As(err, &sv) && errors.As(sv.Err, &v) {
			if p := problemasDe(v, documento); len(p) > 0 {
				motivo = p[0].Texto // el primer problema del esquema frente al metaesquema, legible
			}
		}
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, recortar(idioma.Tf("esquema no válido: %s", motivo)))
	}
	return compilado, nil
}

// CompilarDetallado es Compilar con la línea y la columna (desde 1) cuando el esquema no es JSON válido; 0 si no
// aplican.
func CompilarDetallado(esquema []byte) (linea, columna int, err error) {
	var documento any
	var sintaxis *json.SyntaxError
	if errors.As(json.Unmarshal(esquema, &documento), &sintaxis) {
		antes := esquema[:min(int(sintaxis.Offset), len(esquema))]
		linea = bytes.Count(antes, []byte("\n")) + 1
		columna = len([]rune(string(antes[bytes.LastIndexByte(antes, '\n')+1:])))
		return linea, columna, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("el esquema no es JSON válido (línea %d, columna %d): %s", linea, columna, sintaxis.Error()))
	}
	_, err = Compilar(esquema)
	return 0, 0, err
}

// Validar comprueba la salida contra el esquema; si no cumple devuelve un *ErrorSalida (salida_no_cumple_esquema) con
// los problemas.
func Validar(esquema []byte, salida string) error {
	compilado, err := compiladoDe(esquema)
	if err != nil {
		return err
	}
	instancia, err := jsonschema.UnmarshalJSON(strings.NewReader(salida))
	if err != nil {
		return errorSalida([]Problema{{Problema: "tipo", Texto: idioma.T("La respuesta no es JSON válido")}})
	}
	if err := compilado.Validate(instancia); err != nil {
		var v *jsonschema.ValidationError
		if errors.As(err, &v) {
			if p := problemasDe(v, instancia); len(p) > 0 {
				return errorSalida(p)
			}
		}
		return errorSalida([]Problema{{Problema: "otro", Texto: recortar(idioma.Tf("la salida no cumple el esquema: %s", err.Error()))}})
	}
	return nil
}

func recortar(texto string) string {
	if len(texto) > maximoMensaje {
		return texto[:maximoMensaje] + "…"
	}
	return texto
}

// compilados guarda los esquemas ya compilados por su contenido: se compila al aceptar y no otra vez al validar.
var compilados = struct {
	sync.Mutex
	m map[[32]byte]*jsonschema.Schema
}{m: map[[32]byte]*jsonschema.Schema{}}

func compiladoDe(esquema []byte) (*jsonschema.Schema, error) {
	clave := sha256.Sum256(esquema)
	compilados.Lock()
	compilado, existe := compilados.m[clave]
	compilados.Unlock()
	if existe {
		return compilado, nil
	}
	compilado, err := Compilar(esquema)
	if err != nil {
		return nil, err
	}
	compilados.Lock()
	if len(compilados.m) >= 256 { // pocos esquemas distintos en uso: se vacía antes que crecer sin límite
		clear(compilados.m)
	}
	compilados.m[clave] = compilado
	compilados.Unlock()
	return compilado, nil
}

// Comprobar compila el esquema (y lo guarda para validar la salida después) o explica por qué no vale.
func Comprobar(esquema []byte) error {
	_, err := compiladoDe(esquema)
	return err
}
