package api

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

const herramientaTiempo = `{"name":"tiempo","description":"El tiempo de una ciudad","parameters":{"type":"object","properties":{"ciudad":{"type":"string"}},"required":["ciudad"]}}`

// pideTiempo responde como un agente que llama a la herramienta tiempo.
var pideTiempo = adaptadores.Simulado{Respuesta: `{"content":"","tool_calls":[{"name":"tiempo","arguments":"{\"ciudad\": \"Lima\"}"}]}`}

func decodificarEn(t *testing.T, texto string, destino any) {
	t.Helper()
	if err := json.Unmarshal([]byte(texto), destino); err != nil {
		t.Fatalf("JSON no válido %q: %v", texto, err)
	}
}

func TestHerramientasOpenAI(t *testing.T) {
	ts := servidor(t, simulado(pideTiempo), nil)
	peticion := `{"model":"simulado","messages":[{"role":"user","content":"¿Qué tiempo hace en Lima?"}],"tools":[{"type":"function","function":` + herramientaTiempo + `}]`
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", peticion+`}`)
	if r.StatusCode != 200 {
		t.Fatalf("chat con herramientas: %d %s", r.StatusCode, cuerpo)
	}
	eleccion := decodificar(t, cuerpo)["choices"].([]any)[0].(map[string]any)
	mensaje := eleccion["message"].(map[string]any)
	llamadas, _ := mensaje["tool_calls"].([]any)
	if eleccion["finish_reason"] != "tool_calls" || mensaje["content"] != nil || len(llamadas) != 1 {
		t.Fatalf("chat con herramientas: %s", cuerpo)
	}
	funcion := llamadas[0].(map[string]any)["function"].(map[string]any)
	if funcion["name"] != "tiempo" || funcion["arguments"] != `{"ciudad":"Lima"}` {
		t.Fatalf("llamada: %v", funcion)
	}

	_, flujo := pedir(t, ts, "POST", "/openai/v1/chat/completions", peticion+`,"stream":true}`)
	if !strings.Contains(flujo, `"tool_calls":[{`) || !strings.Contains(flujo, `"finish_reason":"tool_calls"`) || !strings.HasSuffix(flujo, "data: [DONE]\n\n") {
		t.Fatalf("chat con herramientas en streaming:\n%s", flujo)
	}

	respuestas := `{"model":"simulado","input":"tiempo en Lima","tools":[{"type":"function",` + strings.TrimPrefix(herramientaTiempo, "{") + `]`
	r, cuerpo = pedir(t, ts, "POST", "/openai/v1/responses", respuestas+`}`)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"type":"function_call"`) || !strings.Contains(cuerpo, `"call_id":"call_trabajo_`) {
		t.Fatalf("responses con herramientas: %d %s", r.StatusCode, cuerpo)
	}
	_, flujo = pedir(t, ts, "POST", "/openai/v1/responses", respuestas+`,"stream":true}`)
	for _, evento := range []string{"event: response.function_call_arguments.done", "event: response.completed"} {
		if !strings.Contains(flujo, evento) {
			t.Fatalf("falta %s:\n%s", evento, flujo)
		}
	}
	if strings.Contains(flujo, "response.output_text.delta") {
		t.Fatalf("solo pidió herramientas y salió un mensaje de texto:\n%s", flujo)
	}

	// Los argumentos que no cumplen el esquema de la herramienta no llegan a la aplicación.
	malo := servidor(t, simulado(adaptadores.Simulado{Respuesta: `{"content":"","tool_calls":[{"name":"tiempo","arguments":"{\"ciudad\":5}"}]}`}), nil)
	if r, cuerpo := pedir(t, malo, "POST", "/openai/v1/chat/completions", peticion+`}`); r.StatusCode != 422 || !strings.Contains(cuerpo, "salida_no_cumple_esquema") {
		t.Fatalf("argumentos que no cumplen: %d %s", r.StatusCode, cuerpo)
	}
	texto := servidor(t, simulado(adaptadores.Simulado{Respuesta: `{"content":"Hace sol","tool_calls":[]}`}), nil)
	if r, cuerpo := pedir(t, texto, "POST", "/openai/v1/chat/completions", peticion+`,"tool_choice":"required"}`); r.StatusCode != 422 {
		t.Fatalf("required sin llamada: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, texto, "POST", "/openai/v1/chat/completions", peticion+`}`); r.StatusCode != 200 ||
		!strings.Contains(cuerpo, `"content":"Hace sol"`) || !strings.Contains(cuerpo, `"finish_reason":"stop"`) {
		t.Fatalf("respuesta de texto con herramientas: %d %s", r.StatusCode, cuerpo)
	}
}

func TestHerramientasAnthropic(t *testing.T) {
	ts := servidor(t, simulado(pideTiempo), nil)
	peticion := `{"model":"simulado","max_tokens":100,"messages":[{"role":"user","content":"tiempo en Lima"}],"tools":[` +
		strings.Replace(herramientaTiempo, `"parameters"`, `"input_schema"`, 1) + `]`
	r, cuerpo := pedir(t, ts, "POST", "/anthropic/v1/messages", peticion+`}`)
	v := decodificar(t, cuerpo)
	if r.StatusCode != 200 || v["stop_reason"] != "tool_use" {
		t.Fatalf("messages con herramientas: %d %s", r.StatusCode, cuerpo)
	}
	bloque := v["content"].([]any)[0].(map[string]any)
	if bloque["type"] != "tool_use" || bloque["name"] != "tiempo" || bloque["input"].(map[string]any)["ciudad"] != "Lima" {
		t.Fatalf("bloque tool_use: %v", bloque)
	}
	_, flujo := pedir(t, ts, "POST", "/anthropic/v1/messages", peticion+`,"stream":true}`)
	for _, parte := range []string{`"type":"tool_use"`, `"type":"input_json_delta"`, `"stop_reason":"tool_use"`, "event: message_stop"} {
		if !strings.Contains(flujo, parte) {
			t.Fatalf("falta %s:\n%s", parte, flujo)
		}
	}
}

func TestHistorialConHerramientas(t *testing.T) {
	// OpenAI chat: llamada del asistente y resultado con rol tool.
	var chat peticionChat
	decodificarEn(t, `{"messages":[{"role":"user","content":"tiempo en Lima"},
		{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"tiempo","arguments":"{\"ciudad\":\"Lima\"}"}}]},
		{"role":"tool","tool_call_id":"call_1","content":"18 grados"}]}`, &chat)
	m, _, err := mensajes(chat.Messages)
	if err != nil || len(m) != 3 || m[1].Rol != "asistente" || len(m[1].Llamadas) != 1 || m[1].Llamadas[0].ID != "call_1" ||
		string(m[1].Llamadas[0].Argumentos) != `{"ciudad":"Lima"}` || m[2].Rol != "herramienta" || m[2].IDLlamada != "call_1" || m[2].Contenido != "18 grados" {
		t.Fatalf("chat: %v %#v", err, m)
	}
	// Anthropic: tool_use en el asistente y tool_result en el usuario, antes de su texto.
	var anthropic peticionMensajes
	decodificarEn(t, `{"messages":[{"role":"user","content":"tiempo en Lima"},
		{"role":"assistant","content":[{"type":"text","text":"Lo miro."},{"type":"tool_use","id":"toolu_1","name":"tiempo","input":{"ciudad":"Lima"}}]},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"18 grados"}]},{"type":"text","text":"¿Y mañana?"}]}]}`, &anthropic)
	m, _, err = mensajes(anthropic.Messages)
	if err != nil || len(m) != 4 || m[1].Contenido != "Lo miro." || len(m[1].Llamadas) != 1 ||
		m[2].Rol != "herramienta" || m[2].IDLlamada != "toolu_1" || m[2].Contenido != "18 grados" || m[3].Rol != "usuario" || m[3].Contenido != "¿Y mañana?" {
		t.Fatalf("anthropic: %v %#v", err, m)
	}
	// Responses: function_call y function_call_output.
	var entrada []mensajeEntrada
	decodificarEn(t, `[{"role":"user","content":"tiempo en Lima"},{"type":"function_call","call_id":"call_1","name":"tiempo","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":"18 grados"}]`, &entrada)
	if m, _, err = mensajes(entrada); err != nil || len(m) != 3 || m[1].Rol != "asistente" || len(m[1].Llamadas) != 1 || m[2].Rol != "herramienta" {
		t.Fatalf("responses: %v %#v", err, m)
	}
	// Unos argumentos que no son un objeto JSON se rechazan.
	decodificarEn(t, `{"messages":[{"role":"assistant","content":null,"tool_calls":[{"id":"c","type":"function","function":{"name":"tiempo","arguments":"[1]"}}]}]}`, &chat)
	if _, _, err := mensajes(chat.Messages); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("argumentos que no son un objeto: %v", err)
	}
}

// secuencia responde, en orden, las salidas dadas (repite la última).
type secuencia struct {
	n       *atomic.Int32
	salidas []string
}

func (s secuencia) Ejecutar(_ context.Context, _ nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	i := int(s.n.Add(1)) - 1
	return adaptadores.Resultado{Salida: s.salidas[min(i, len(s.salidas)-1)]}, nil
}

func TestFachadaCorrigeArgumentosInvalidos(t *testing.T) {
	var n atomic.Int32
	agente := secuencia{n: &n, salidas: []string{
		`{"content":"","tool_calls":[{"name":"tiempo","arguments":"{\"ciudad\": 7}"}]}`,
		`{"content":"","tool_calls":[{"name":"tiempo","arguments":"{\"ciudad\": \"Lima\"}"}]}`}}
	ts := servidorCon(t, simulado(agente), nil, &Punto{Nombre: "tienda", Contratos: map[string]bool{"openai": true}, ReintentosSalida: 1})
	peticion := `{"model":"simulado","messages":[{"role":"user","content":"¿Qué tiempo hace en Lima?"}],"tools":[{"type":"function","function":` + herramientaTiempo + `}]}`
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", peticion)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `"tool_calls"`) || !strings.Contains(cuerpo, `Lima`) || n.Load() != 2 {
		t.Fatalf("unos argumentos inválidos se corrigen con la receta antes de responder: %d %s (%d intentos)", r.StatusCode, cuerpo, n.Load())
	}
}
