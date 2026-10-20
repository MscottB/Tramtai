package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

const herramientaClima = `{"nombre":"clima","descripcion":"El tiempo actual de una ciudad",` +
	`"parametros":{"type":"object","properties":{"ciudad":{"type":"string"}},"required":["ciudad"]}}`

// agenteClima pide la herramienta clima hasta que la conversación trae tantos resultados como rondas; entonces
// responde con texto. Guarda la última conversación que recibió.
type agenteClima struct {
	rondas   int
	mu       *sync.Mutex
	recibida *string
}

func nuevoAgenteClima(rondas int) agenteClima {
	return agenteClima{rondas: rondas, mu: &sync.Mutex{}, recibida: new(string)}
}

func (a agenteClima) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	var b strings.Builder
	for _, m := range s.Mensajes {
		b.WriteString(m.Contenido + "\n")
	}
	conversacion := b.String()
	a.mu.Lock()
	*a.recibida = conversacion
	a.mu.Unlock()
	if strings.Count(conversacion, "Tool result") >= a.rondas {
		return adaptadores.Resultado{Salida: `{"content":"En Santiago hay 18 °C y está despejado.","tool_calls":[]}`}, nil
	}
	return adaptadores.Resultado{Salida: `{"content":"","tool_calls":[{"name":"clima","arguments":"{\"ciudad\":\"Santiago\"}"}]}`}, nil
}

func (a agenteClima) ultima() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return *a.recibida
}

// crearYEsperar crea un trabajo por la API propia y espera a que termine en el estado indicado.
func crearYEsperar(t *testing.T, ts *httptest.Server, cuerpo string, objetivo nucleo.Estado) map[string]any {
	t.Helper()
	r, respuesta := pedir(t, ts, "POST", "/api/v1/trabajos", cuerpo)
	if r.StatusCode != 202 {
		t.Fatalf("crear: %d %s", r.StatusCode, respuesta)
	}
	return esperarEstado(t, ts, decodificar(t, respuesta)["id"].(string), objetivo)
}

func llamadasDe(v map[string]any) []map[string]any {
	lista, _ := v["llamadas"].([]any)
	var salida []map[string]any
	for _, l := range lista {
		salida = append(salida, l.(map[string]any))
	}
	return salida
}

func TestNativaHerramientasDosTurnos(t *testing.T) {
	agente := nuevoAgenteClima(1)
	ts := servidor(t, simulado(agente), nil)
	primero := `{"agente":"simulado","mensajes":[{"rol":"usuario","contenido":"¿Qué tiempo hace en Santiago?"}],"herramientas":[` + herramientaClima + `],"eleccion_herramienta":"auto"}`
	v := crearYEsperar(t, ts, primero, nucleo.Completado)
	llamadas := llamadasDe(v)
	if v["salida"] != nil && v["salida"] != "" || len(llamadas) != 1 || llamadas[0]["id"] != "llamada_"+v["id"].(string)+"_0" ||
		llamadas[0]["nombre"] != "clima" || fmt.Sprint(llamadas[0]["argumentos"]) != "map[ciudad:Santiago]" {
		t.Fatalf("primer turno: %v", v)
	}
	// /salida da el texto (vacío), nunca el sobre interno.
	if _, cuerpo := pedir(t, ts, "GET", "/api/v1/trabajos/"+v["id"].(string)+"/salida", ""); strings.Contains(cuerpo, "tool_calls") {
		t.Fatalf("/salida enseña el sobre: %s", cuerpo)
	}
	llamada, _ := json.Marshal(llamadas[0])
	segundo := `{"agente":"simulado","herramientas":[` + herramientaClima + `],"mensajes":[{"rol":"usuario","contenido":"¿Qué tiempo hace en Santiago?"},` +
		`{"rol":"asistente","contenido":"","llamadas":[` + string(llamada) + `]},` +
		`{"rol":"herramienta","id_llamada":"` + llamadas[0]["id"].(string) + `","contenido":"18 °C, despejado"}]}`
	v = crearYEsperar(t, ts, segundo, nucleo.Completado)
	if v["salida"] != "En Santiago hay 18 °C y está despejado." || v["llamadas"] != nil {
		t.Fatalf("segundo turno: %v", v)
	}
	if !strings.Contains(agente.ultima(), "Tool result for clima (call "+llamadas[0]["id"].(string)+"):\n18 °C, despejado") {
		t.Fatalf("el agente recibe el resultado con su herramienta: %q", agente.ultima())
	}
}

func TestNativaDosRondasIDsDistintos(t *testing.T) {
	agente := nuevoAgenteClima(2)
	ts := servidor(t, simulado(agente), nil)
	mensajes := `{"rol":"usuario","contenido":"¿Y en Santiago?"}`
	var ids []string
	for ronda := 0; ronda < 2; ronda++ {
		v := crearYEsperar(t, ts, `{"agente":"simulado","herramientas":[`+herramientaClima+`],"mensajes":[`+mensajes+`]}`, nucleo.Completado)
		llamadas := llamadasDe(v)
		if len(llamadas) != 1 {
			t.Fatalf("ronda %d: %v", ronda, v)
		}
		id := llamadas[0]["id"].(string)
		ids = append(ids, id)
		llamada, _ := json.Marshal(llamadas[0])
		mensajes += `,{"rol":"asistente","contenido":"","llamadas":[` + string(llamada) + `]},{"rol":"herramienta","id_llamada":"` + id + `","contenido":"18 °C (` + id + `)"}`
	}
	if ids[0] == ids[1] {
		t.Fatalf("los ids de dos rondas no chocan: %v", ids)
	}
	crearYEsperar(t, ts, `{"agente":"simulado","herramientas":[`+herramientaClima+`],"mensajes":[`+mensajes+`]}`, nucleo.Completado)
	for _, id := range ids {
		if !strings.Contains(agente.ultima(), "Tool result for clima (call "+id+"):\n18 °C ("+id+")") {
			t.Fatalf("cada resultado va con su llamada (%s): %q", id, agente.ultima())
		}
	}
}

func TestNativaHerramientasErrores(t *testing.T) {
	ts := servidor(t, simulado(nuevoAgenteClima(1)), nil)
	casos := map[string]struct {
		cuerpo string
		error  string
	}{
		"sin nombre":     {`{"mensajes":[{"rol":"usuario","contenido":"x"}],"herramientas":[{"descripcion":"x"}]}`, "solicitud_invalida"},
		"con esquema":    {`{"mensajes":[{"rol":"usuario","contenido":"x"}],"herramientas":[` + herramientaClima + `],"salida":{"formato":"json","esquema":{"type":"object"}}}`, "parametro_no_soportado"},
		"id desconocido": {`{"mensajes":[{"rol":"usuario","contenido":"x"},{"rol":"herramienta","id_llamada":"nada","contenido":"18"}],"herramientas":[` + herramientaClima + `]}`, "solicitud_invalida"},
		"eleccion rara":  {`{"mensajes":[{"rol":"usuario","contenido":"x"}],"herramientas":[` + herramientaClima + `],"eleccion_herramienta":"siempre"}`, "solicitud_invalida"},
	}
	for nombre, c := range casos {
		if r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos", c.cuerpo); r.StatusCode != 400 || !strings.Contains(cuerpo, c.error) {
			t.Errorf("%s: %d %s", nombre, r.StatusCode, cuerpo)
		}
	}
}

func TestNativaRecetaConEsquemaYHerramientas(t *testing.T) {
	ts := servidorCon(t, simulado(nuevoAgenteClima(1)), nil, &Punto{Nombre: "tienda", Contratos: map[string]bool{"nativa": true},
		Esquema: json.RawMessage(`{"type":"object"}`)})
	cuerpo := `{"mensajes":[{"rol":"usuario","contenido":"x"}],"herramientas":[` + herramientaClima + `]}`
	if r, respuesta := pedir(t, ts, "POST", "/api/v1/trabajos", cuerpo); r.StatusCode != 400 || !strings.Contains(respuesta, "parametro_no_soportado") {
		t.Fatalf("una receta con esquema no se combina con herramientas: %d %s", r.StatusCode, respuesta)
	}
}

func TestNativaHerramientasTrabajoFallido(t *testing.T) {
	siempreMal := adaptadores.Simulado{Respuesta: `{"content":"","tool_calls":[{"name":"clima","arguments":"{\"ciudad\":7}"}]}`}
	ts := servidor(t, simulado(siempreMal), nil)
	v := crearYEsperar(t, ts, `{"agente":"simulado","mensajes":[{"rol":"usuario","contenido":"x"}],"herramientas":[`+herramientaClima+`]}`, nucleo.Fallido)
	if v["llamadas"] != nil || v["codigo_error"] != string(nucleo.CodigoSalidaNoCumpleEsquema) {
		t.Fatalf("un trabajo fallido no trae llamadas: %v", v)
	}
	if r, lista := pedir(t, ts, "GET", "/api/v1/trabajos", ""); r.StatusCode != 200 || !strings.Contains(lista, `"fallido"`) {
		t.Fatalf("el listado tampoco se rompe: %d %s", r.StatusCode, lista)
	}
}

func TestNativaSinHerramientasIgualQueAntes(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{Respuesta: "hola"}), nil)
	id := crearYEsperar(t, ts, solicitudHola, nucleo.Completado)["id"].(string)
	_, trabajo := pedir(t, ts, "GET", "/api/v1/trabajos/"+id, "")
	_, lista := pedir(t, ts, "GET", "/api/v1/trabajos", "")
	for _, texto := range []string{trabajo, lista} {
		if strings.Contains(texto, "llamadas") || strings.Contains(texto, "herramientas") || strings.Contains(texto, "errores_salida") {
			t.Fatalf("un trabajo normal no cambia: %s", texto)
		}
	}
	if _, salida := pedir(t, ts, "GET", "/api/v1/trabajos/"+id+"/salida", ""); salida != "hola" {
		t.Fatalf("/salida: %q", salida)
	}
}

func TestNativaCorregirConHerramientas(t *testing.T) {
	var n atomic.Int32
	agente := secuencia{n: &n, salidas: []string{
		`{"content":"","tool_calls":[{"name":"clima","arguments":"{\"ciudad\":7}"}]}`,
		`{"content":"","tool_calls":[{"name":"clima","arguments":"{\"ciudad\":\"Santiago\"}"}]}`}}
	ts := servidor(t, simulado(agente), nil)
	v := crearYEsperar(t, ts, `{"agente":"simulado","mensajes":[{"rol":"usuario","contenido":"x"}],`+
		`"herramientas":[`+herramientaClima+`],"salida":{"conservar_salida_invalida":true}}`, nucleo.Fallido)
	r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos/"+v["id"].(string)+"/reintentar", `{"corregir":true}`)
	if r.StatusCode != 202 {
		t.Fatalf("corregir: %d %s", r.StatusCode, cuerpo)
	}
	v = esperarEstado(t, ts, decodificar(t, cuerpo)["id"].(string), nucleo.Completado)
	if llamadas := llamadasDe(v); len(llamadas) != 1 || fmt.Sprint(llamadas[0]["argumentos"]) != "map[ciudad:Santiago]" {
		t.Fatalf("la corrección termina con la llamada válida: %v", v)
	}
}
