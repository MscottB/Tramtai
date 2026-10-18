package salidas

import (
	"fmt"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
)

const esquema = `{"type":"object","properties":{"ciudad":{"type":"string"},"millones":{"type":"number"}},"required":["ciudad","millones"],"additionalProperties":false}`

func TestValidar(t *testing.T) {
	if err := Validar([]byte(esquema), `{"ciudad":"Santiago","millones":5.3}`); err != nil {
		t.Fatal(err)
	}
	for _, salida := range []string{`{"ciudad":"Santiago"}`, `{"ciudad":"x","millones":1,"extra":true}`, `no es json`} {
		if err := Validar([]byte(esquema), salida); nucleo.CodigoDe(err) != nucleo.CodigoSalidaNoCumpleEsquema {
			t.Errorf("%s: se esperaba salida_no_cumple_esquema: %v", salida, err)
		}
	}
}

func TestEsquemaInvalido(t *testing.T) {
	for _, e := range []string{`{"type":"objeto"}`, `{no json`} {
		if _, err := Compilar([]byte(e)); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
			t.Errorf("%s: se esperaba solicitud_invalida: %v", e, err)
		}
	}
}

func TestProblemas(t *testing.T) {
	const esq = `{"type":"object","additionalProperties":false,"required":["total","categoria","tareas"],"properties":{
		"total":{"type":"number","minimum":0},"categoria":{"enum":["consulta","queja","otro"]},
		"puntos":{"type":"array","items":{"type":"string"},"minItems":3},"fecha":{"type":"string","format":"date"},
		"tareas":{"type":"array","items":{"type":"object","required":["tarea","fecha"],"properties":{"tarea":{"type":"string"},"fecha":{"type":"string"}}}}}}`
	casos := []struct{ salida, campo, problema, texto string }{
		{`{"categoria":"otro","tareas":[]}`, "total", "falta", "Falta el campo `total`"},
		{`{"total":"diez","categoria":"otro","tareas":[]}`, "total", "tipo", "`total` debería ser un número y llegó un texto"},
		{`{"total":1,"categoria":"x","tareas":[]}`, "categoria", "valor_no_permitido", "`categoria` debe ser una de: consulta, queja, otro"},
		{`{"total":1,"categoria":"otro","tareas":[],"notas":1}`, "notas", "sobra", "Sobra el campo `notas`"},
		{`{"total":1,"categoria":"otro","tareas":[],"fecha":"mañana"}`, "fecha", "formato", "`fecha` no tiene el formato date"},
		{`{"total":1,"categoria":"otro","tareas":[],"puntos":["a"]}`, "puntos", "tamaño", "`puntos` debe tener al menos 3 elementos y tiene 1"},
		{`{"total":-0.5,"categoria":"otro","tareas":[]}`, "total", "tamaño", "`total` debe ser como mínimo 0"},
		{`{"total":1,"categoria":"otro","tareas":[{"tarea":"a","fecha":"x"},{"tarea":"b","fecha":"x"},{"tarea":"c"}]}`, "tareas[2].fecha", "falta", "Falta el campo `tareas[2].fecha`"},
		{`[1]`, "", "tipo", "La respuesta debería ser un objeto y llegó una lista"},
		{`no es json`, "", "tipo", "La respuesta no es JSON válido"},
	}
	for _, c := range casos {
		err := Validar([]byte(esq), c.salida)
		p := ProblemasDe(err)
		if nucleo.CodigoDe(err) != nucleo.CodigoSalidaNoCumpleEsquema || len(p) != 1 {
			t.Errorf("%s: se esperaba un problema: %v %+v", c.salida, err, p)
			continue
		}
		if p[0] != (Problema{Campo: c.campo, Problema: c.problema, Texto: c.texto}) {
			t.Errorf("%s: %+v", c.salida, p[0])
		}
		if nucleo.MensajeDe(err) != c.texto {
			t.Errorf("%s: el mensaje debe ser el problema: %q", c.salida, nucleo.MensajeDe(err))
		}
	}
}

func TestProblemasTopeYMensaje(t *testing.T) {
	esq, salida := `{"type":"object","properties":{`, `{`
	for i := range 25 {
		if i > 0 {
			esq, salida = esq+",", salida+","
		}
		esq += fmt.Sprintf(`"c%d":{"type":"number"}`, i)
		salida += fmt.Sprintf(`"c%d":"x"`, i)
	}
	err := Validar([]byte(esq+"}}"), salida+"}")
	if p := ProblemasDe(err); len(p) != MaximoProblemas {
		t.Fatalf("se esperaban %d problemas: %d", MaximoProblemas, len(p))
	}
	if m := nucleo.MensajeDe(err); !strings.HasSuffix(m, "(y 19 más)") {
		t.Errorf("mensaje: %q", m)
	}
}

func TestCompilarDetallado(t *testing.T) {
	if l, c, err := CompilarDetallado([]byte("{\n  \"type\": \"object\",\n  \"properties\": {\n    \"x\" {}\n}}")); err == nil || l != 4 || c != 9 {
		t.Errorf("JSON roto: línea %d columna %d: %v", l, c, err)
	}
	if _, _, err := CompilarDetallado([]byte(`{"type":"objeto"}`)); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida || !strings.Contains(err.Error(), "`type`") {
		t.Errorf("tipo inválido: %v", err)
	}
	if _, _, err := CompilarDetallado([]byte(esquema)); err != nil {
		t.Error(err)
	}
}

func TestReferenciaExternaRechazada(t *testing.T) {
	for _, ref := range []string{"file:///C:/Windows/win.ini", "https://example.com/esquema.json"} {
		if _, err := Compilar([]byte(`{"$ref":"` + ref + `"}`)); err == nil {
			t.Errorf("%s: un $ref externo no debe cargarse", ref)
		}
	}
}
