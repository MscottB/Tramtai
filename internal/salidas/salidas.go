// Package salidas valida las salidas estructuradas contra el JSON Schema del consumidor.
package salidas

import (
	"bytes"
	"errors"
	"strings"

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
	if err := c.AddResource("tramtai:esquema", documento); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("esquema no válido: %s", err.Error()))
	}
	compilado, err := c.Compile("tramtai:esquema")
	if err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, recortar(idioma.Tf("esquema no válido: %s", err.Error())))
	}
	return compilado, nil
}

// Validar comprueba la salida contra el esquema; si no cumple devuelve salida_no_cumple_esquema.
func Validar(esquema []byte, salida string) error {
	compilado, err := Compilar(esquema)
	if err != nil {
		return err
	}
	instancia, err := jsonschema.UnmarshalJSON(strings.NewReader(salida))
	if err != nil {
		return nucleo.NuevoError(nucleo.CodigoSalidaNoCumpleEsquema, idioma.T("la salida no es JSON válido"))
	}
	if err := compilado.Validate(instancia); err != nil {
		return nucleo.NuevoError(nucleo.CodigoSalidaNoCumpleEsquema, recortar(idioma.Tf("la salida no cumple el esquema: %s", err.Error())))
	}
	return nil
}

func recortar(texto string) string {
	if len(texto) > maximoMensaje {
		return texto[:maximoMensaje] + "…"
	}
	return texto
}
