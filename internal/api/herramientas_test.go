package api

import (
	"encoding/json"
	"strings"
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
	if err != nil || len(m) != 3 || m[1].Rol != "asistente" || !strings.Contains(m[1].Contenido, `"tool_calls":[{"id":"call_1","name":"tiempo"`) ||
		m[2].Rol != "usuario" || m[2].Contenido != "Tool result for tiempo (call call_1):\n18 grados" {
		t.Fatalf("chat: %v %#v", err, m)
	}
	// Anthropic: tool_use en el asistente y tool_result en el usuario.
	var anthropic peticionMensajes
	decodificarEn(t, `{"messages":[{"role":"user","content":"tiempo en Lima"},
		{"role":"assistant","content":[{"type":"text","text":"Lo miro."},{"type":"tool_use","id":"toolu_1","name":"tiempo","input":{"ciudad":"Lima"}}]},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"18 grados"}]},{"type":"text","text":"¿Y mañana?"}]}]}`, &anthropic)
	m, _, err = mensajes(anthropic.Messages)
	if err != nil || len(m) != 3 || !strings.Contains(m[1].Contenido, `"content":"Lo miro."`) ||
		m[2].Contenido != "Tool result for tiempo (call toolu_1):\n18 grados\n\n¿Y mañana?" {
		t.Fatalf("anthropic: %v %#v", err, m)
	}
	// Responses: function_call y function_call_output.
	var entrada []mensajeEntrada
	decodificarEn(t, `[{"role":"user","content":"tiempo en Lima"},{"type":"function_call","call_id":"call_1","name":"tiempo","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_1","output":"18 grados"}]`, &entrada)
	if m, _, err = mensajes(entrada); err != nil || len(m) != 3 || m[1].Rol != "asistente" || !strings.HasPrefix(m[2].Contenido, "Tool result for tiempo") {
		t.Fatalf("responses: %v %#v", err, m)
	}
}

func TestPrepararHerramientas(t *testing.T) {
	lista := []herramienta{{Nombre: "a"}, {Nombre: "b", Parametros: []byte(`{"type":"object"}`)}}
	sol := nucleo.SolicitudEjecucion{Instrucciones: "Sé breve."}
	h, err := prepararHerramientas(&sol, lista, eleccion{Modo: "una", Nombre: "b", SoloUna: true})
	if err != nil || h == nil || sol.Salida.Formato != "json" || !strings.Contains(string(sol.Salida.Esquema), `"enum":["b"]`) ||
		!strings.HasPrefix(sol.Instrucciones, "Sé breve.\n\n") || !strings.Contains(sol.Instrucciones, "You must call the tool b.") {
		t.Fatalf("una: %v %s %q", err, sol.Salida.Esquema, sol.Instrucciones)
	}
	if h, err := prepararHerramientas(&nucleo.SolicitudEjecucion{}, lista, eleccion{Modo: "ninguna"}); h != nil || err != nil {
		t.Fatalf("none no debe pedir herramientas: %v %v", h, err)
	}
	casos := []struct {
		lista []herramienta
		e     eleccion
		sol   nucleo.SolicitudEjecucion
	}{
		{[]herramienta{{Nombre: "a"}, {Nombre: "a"}}, eleccion{}, nucleo.SolicitudEjecucion{}},
		{lista, eleccion{Modo: "una", Nombre: "c"}, nucleo.SolicitudEjecucion{}},
		{nil, eleccion{Modo: "obligatoria"}, nucleo.SolicitudEjecucion{}},
		{[]herramienta{{Nombre: "a", Parametros: []byte(`{"type":7}`)}}, eleccion{}, nucleo.SolicitudEjecucion{}},
		{lista, eleccion{}, nucleo.SolicitudEjecucion{Salida: nucleo.ConfigSalida{Formato: "json"}}},
	}
	for i, c := range casos {
		if _, err := prepararHerramientas(&c.sol, c.lista, c.e); err == nil {
			t.Errorf("caso %d: se esperaba un error", i)
		}
	}
	// Con parallel_tool_calls false solo pasa la primera llamada; los argumentos pueden llegar como objeto.
	una := &herramientas{lista: lista, eleccion: eleccion{Modo: "auto", SoloUna: true}}
	_, llamadas, err := una.interpretar(`{"content":"","tool_calls":[{"name":"a","arguments":"{}"},{"name":"b","arguments":{"x":1}}]}`, "call_x")
	if err != nil || len(llamadas) != 1 || llamadas[0].ID != "call_x_0" {
		t.Fatalf("solo una: %v %v", err, llamadas)
	}
}
