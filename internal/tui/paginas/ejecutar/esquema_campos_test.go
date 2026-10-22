package ejecutar

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"tramtai/internal/tui/datos"
)

// Las pruebas de esquema_campos.go, con los casos de web/src/esquemaCampos.test.ts.

func num(n float64) *float64 { return &n }

var camposPrueba = []campoEsquema{
	{Nombre: "nombre", Tipo: "texto", Obligatorio: true, Descripcion: "Quién"},
	{Nombre: "total", Tipo: "numero", Minimo: num(0)},
	{Nombre: "unidades", Tipo: "entero", Obligatorio: true, Minimo: num(1), Maximo: num(10)},
	{Nombre: "pagado", Tipo: "si_no", Obligatorio: true},
	{Nombre: "fecha", Tipo: "fecha", Obligatorio: true},
	{Nombre: "estado", Tipo: "opcion", Obligatorio: true, Opciones: []string{"abierto", "cerrado"}},
	{Nombre: "etiquetas", Tipo: "lista", Elemento: "texto", Minimo: num(1), Maximo: num(5)},
	{Nombre: "notas", Tipo: "lista", Elemento: "numero", Obligatorio: true},
	{Nombre: "cliente", Tipo: "objeto", Obligatorio: true, Descripcion: "Datos", Campos: []campoEsquema{
		{Nombre: "direccion", Tipo: "objeto", Campos: []campoEsquema{{Nombre: "calle", Tipo: "texto", Obligatorio: true}}},
	}},
	{Nombre: "lineas", Tipo: "lista", Elemento: "objeto", Obligatorio: true, Campos: []campoEsquema{{Nombre: "precio", Tipo: "numero", Obligatorio: true}}},
}

// TestCamposIdaYVuelta: de los campos al esquema y de vuelta, idéntico; el esquema tiene la forma de aEsquema.
func TestCamposIdaYVuelta(t *testing.T) {
	esquema := aEsquema(camposPrueba)
	var e struct {
		Required             []string                   `json:"required"`
		AdditionalProperties *bool                      `json:"additionalProperties"`
		Properties           map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal([]byte(esquema), &e); err != nil {
		t.Fatal(err)
	}
	if e.AdditionalProperties == nil || *e.AdditionalProperties ||
		strings.Join(e.Required, ",") != "nombre,unidades,pagado,fecha,estado,notas,cliente,lineas" || string(e.Properties["fecha"]) != `{"type":"string","format":"date"}` {
		t.Fatalf("el esquema de aEsquema: %s", esquema)
	}
	if !strings.HasPrefix(esquema, `{"type":"object","properties":{"nombre":{"type":"string","description":"Quién"},"total":{"type":"number","minimum":0}`) {
		t.Fatalf("las claves en el orden de la Web UI: %s", esquema)
	}
	vuelta, ok := camposDe(esquema)
	if !ok || !reflect.DeepEqual(vuelta, camposPrueba) {
		t.Fatalf("de vuelta, los mismos campos:\n%+v", vuelta)
	}
}

// TestEsquemaAvanzadoSeQuedaEnJSON: lo que el editor de campos no representa no se convierte.
func TestEsquemaAvanzadoSeQuedaEnJSON(t *testing.T) {
	avanzados := []string{
		`{"type":"object","properties":{"a":{"type":"string"}}}`,
		`{"type":"object","additionalProperties":false,"properties":{"a":{"type":"string","pattern":"^x"}}}`,
		`{"type":"object","additionalProperties":false,"properties":{"a":{"oneOf":[{"type":"string"},{"type":"number"}]}}}`,
		`{"type":"object","additionalProperties":false,"properties":{"a":{"$ref":"#/$defs/x"}}}`,
		`{"type":"object","additionalProperties":false,"required":["b"],"properties":{"a":{"type":"string"}}}`,
		`{"type":"array","items":{"type":"string"}}`,
		`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`,
		`{"type":"object","description":"Factura","additionalProperties":false,"properties":{"x":{"type":"string"}}}`,
		`{"type":"object","additionalProperties":false,"properties":{"l":{"type":"array","items":{"type":"object","description":"Una línea","additionalProperties":false,"properties":{"x":{"type":"string"}}}}}}`,
		`{"type":"object","additionalProperties":false,"properties":{"n":{"type":"number","minimum":"1"}}}`,
		`{`,
	}
	for _, e := range avanzados {
		if c, ok := camposDe(e); ok {
			t.Errorf("%s se convirtió: %+v", e, c)
		}
	}
	nivel := func(n int) string {
		s := `{"type":"string"}`
		for range n {
			s = `{"type":"object","additionalProperties":false,"properties":{"x":` + s + `}}`
		}
		return s
	}
	if _, ok := camposDe(nivel(3)); !ok {
		t.Error("tres niveles de objetos sí")
	}
	if _, ok := camposDe(nivel(4)); ok {
		t.Error("cuatro no: el editor llega a tres")
	}
}

// TestPlantillasComoLaWeb: cada plantilla (las de datos y las de la copia que genera esquemaCampos.test.ts) se convierte en
// campos y vuelve a dar el mismo esquema, byte a byte.
func TestPlantillasComoLaWeb(t *testing.T) {
	var esquemas []json.RawMessage
	for _, lengua := range []string{"es", "en"} {
		for _, p := range datos.Plantillas(lengua) {
			esquemas = append(esquemas, p.Esquema)
		}
	}
	crudo, err := os.ReadFile("../../../salidas/testdata/plantillas.json")
	if err != nil {
		t.Fatal(err)
	}
	var copia []struct {
		Esquema json.RawMessage `json:"esquema"`
	}
	if err := json.Unmarshal(crudo, &copia); err != nil {
		t.Fatal(err)
	}
	for _, c := range copia {
		esquemas = append(esquemas, c.Esquema)
	}
	if len(esquemas) != 28 {
		t.Fatalf("7 plantillas en dos idiomas, dos veces: %d", len(esquemas))
	}
	for _, e := range esquemas {
		var compacto bytes.Buffer
		if err := json.Compact(&compacto, e); err != nil {
			t.Fatal(err)
		}
		campos, ok := camposDe(string(e))
		if !ok {
			t.Fatalf("la plantilla se edita en campos: %s", compacto.String())
		}
		if p := problemaDeCampos(campos, ""); p != (problemaCampo{}) {
			t.Fatalf("sin problemas: %+v", p)
		}
		if got := aEsquema(campos); got != compacto.String() {
			t.Fatalf("el mismo esquema que la Web UI:\n%s\n%s", got, compacto.String())
		}
	}
}

func TestProblemasDeCampos(t *testing.T) {
	a := campoEsquema{Nombre: "a", Tipo: "texto", Obligatorio: true}
	opcion, objeto, lista := a, a, a
	opcion.Tipo, opcion.Opciones = "opcion", []string{" "}
	objeto.Tipo = "objeto"
	lista.Tipo, lista.Elemento, lista.Minimo, lista.Maximo = "lista", "texto", num(5), num(2)
	anidado := campoEsquema{Nombre: "b", Tipo: "objeto", Campos: []campoEsquema{{Tipo: "texto"}}}
	casos := []struct {
		campos []campoEsquema
		quiere problemaCampo
	}{
		{[]campoEsquema{{Tipo: "texto"}}, problemaCampo{"", "sin_nombre"}},
		{[]campoEsquema{a, a}, problemaCampo{"a", "repetido"}},
		{[]campoEsquema{opcion}, problemaCampo{"a", "sin_opciones"}},
		{[]campoEsquema{objeto}, problemaCampo{"a", "sin_campos"}},
		{[]campoEsquema{lista}, problemaCampo{"a", "rango"}},
		{[]campoEsquema{anidado}, problemaCampo{"b", "sin_nombre"}},
		{camposPrueba, problemaCampo{}},
	}
	for _, c := range casos {
		if got := problemaDeCampos(c.campos, ""); got != c.quiere {
			t.Errorf("%+v: %+v, quería %+v", c.campos, got, c.quiere)
		}
	}
}

// TestCampoProto: un campo llamado __proto__ no se pierde.
func TestCampoProto(t *testing.T) {
	c := []campoEsquema{{Nombre: "__proto__", Tipo: "texto", Obligatorio: true}}
	if vuelta, ok := camposDe(aEsquema(c)); !ok || !reflect.DeepEqual(vuelta, c) {
		t.Fatalf("__proto__ vuelve igual: %+v", vuelta)
	}
}
