package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/documentos"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
)

// trozos emite la respuesta en tres fragmentos, como un agente con streaming por tokens.
type trozos struct{}

func (trozos) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	for _, t := range []string{"Ho", "la", " mundo"} {
		if err := emitir(adaptadores.Evento{Tipo: "texto", Datos: t}); err != nil {
			return adaptadores.Resultado{}, err
		}
	}
	return adaptadores.Resultado{Salida: "Hola mundo", Uso: nucleo.Uso{TokensEntrada: 7, TokensSalida: 3}}, nil
}

func servidor(t *testing.T, registro map[string]adaptadores.Adaptador, ajustes map[string]any) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	alm, err := persistencia.Abrir(filepath.Join(dir, "instancia.db"))
	if err != nil {
		t.Fatal(err)
	}
	valores := map[string]any{"predeterminados.agente": "simulado", "modelos.perfiles.rapido.simulado": "m1"}
	maps.Copy(valores, ajustes)
	c := nucleo.Resolver(nucleo.Capa{Origen: nucleo.OrigenGlobal, Valores: valores})
	cargas, err := documentos.AbrirCargas(filepath.Join(dir, "cargas"), time.Hour, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	opc := planificador.OpcionesDesde(c, "global")
	opc.Preparador = documentos.Preparador{Cargas: cargas, Conversor: documentos.Desde(c)}
	limites := map[string]int{}
	for nombre := range registro {
		limites[nombre] = 2
	}
	p := planificador.Nuevo(alm, registro, planificador.NuevosSemaforos(limites), opc)
	if err := p.Iniciar(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := &Servidor{Planificador: p, Cargas: cargas, Registro: registro, Config: c, Token: "secreto",
		Hosts: []string{"127.0.0.1", "localhost", "::1"}, Sondeo: 10 * time.Millisecond}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		ts.Close()
		p.Detener()
		alm.Cerrar()
	})
	return ts
}

func simulado(a adaptadores.Adaptador) map[string]adaptadores.Adaptador {
	return map[string]adaptadores.Adaptador{"simulado": a}
}

// pedir hace una petición autenticada; cabeceras es una lista nombre, valor.
func pedir(t *testing.T, ts *httptest.Server, metodo, ruta, cuerpo string, cabeceras ...string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(metodo, ts.URL+ruta, strings.NewReader(cuerpo))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secreto")
	for i := 0; i+1 < len(cabeceras); i += 2 {
		if cabeceras[i] == "Host" {
			req.Host = cabeceras[i+1]
			continue
		}
		req.Header.Set(cabeceras[i], cabeceras[i+1])
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	datos, _ := io.ReadAll(resp.Body)
	return resp, string(datos)
}

func decodificar(t *testing.T, texto string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(texto), &v); err != nil {
		t.Fatalf("JSON no válido %q: %v", texto, err)
	}
	return v
}

func esperarEstado(t *testing.T, ts *httptest.Server, id string, objetivo nucleo.Estado) map[string]any {
	t.Helper()
	limite := time.Now().Add(10 * time.Second)
	for {
		_, cuerpo := pedir(t, ts, "GET", "/api/v1/trabajos/"+id, "")
		v := decodificar(t, cuerpo)
		if v["estado"] == string(objetivo) {
			return v
		}
		if time.Now().After(limite) {
			t.Fatalf("%s en %v, se esperaba %s", id, v["estado"], objetivo)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

const solicitudHola = `{"mensajes":[{"rol":"usuario","contenido":"hola"}]}`

func TestSeguridad(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	if r, cuerpo := pedir(t, ts, "GET", "/api/v1/modelos", "", "Host", "atacante.example:17373"); r.StatusCode != 403 || !strings.Contains(cuerpo, "origen_no_permitido") {
		t.Fatalf("un Host ajeno se rechaza antes de autenticar: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/modelos", "", "Origin", "http://atacante.example"); r.StatusCode != 403 {
		t.Fatalf("un Origin ajeno se rechaza: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/modelos", "", "Origin", ts.URL); r.StatusCode != 200 {
		t.Fatalf("el mismo origen se acepta: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/modelos", "", "Authorization", "Bearer otro"); r.StatusCode != 401 || r.Header.Get("WWW-Authenticate") == "" {
		t.Fatalf("un token incorrecto da 401: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/modelos", "", "Authorization", "", "x-api-key", "secreto"); r.StatusCode != 200 {
		t.Fatalf("x-api-key se acepta: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts, "GET", "/healthz", "", "Authorization", ""); r.StatusCode != 200 {
		t.Fatalf("healthz no exige token: %d", r.StatusCode)
	}
	if r, cuerpo := pedir(t, ts, "GET", "/no/existe", ""); r.StatusCode != 404 || !strings.Contains(cuerpo, "no_encontrado") {
		t.Fatalf("ruta desconocida: %d %s", r.StatusCode, cuerpo)
	}
}

func TestTrabajoNativoYEventos(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola)
	if r.StatusCode != 202 || !strings.HasPrefix(r.Header.Get("Location"), "/api/v1/trabajos/trabajo_") {
		t.Fatalf("crear: %d %s", r.StatusCode, cuerpo)
	}
	id := decodificar(t, cuerpo)["id"].(string)
	esperarEstado(t, ts, id, nucleo.Completado)
	if r, salida := pedir(t, ts, "GET", "/api/v1/trabajos/"+id+"/salida", ""); r.StatusCode != 200 || salida != "hola" ||
		!strings.HasPrefix(r.Header.Get("Content-Type"), "text/markdown") {
		t.Fatalf("salida: %d %q %s", r.StatusCode, salida, r.Header.Get("Content-Type"))
	}
	_, eventos := pedir(t, ts, "GET", "/api/v1/trabajos/"+id+"/eventos", "")
	if !strings.HasPrefix(eventos, "id: 1\nevent: estado\ndata: en_cola\n\n") || !strings.Contains(eventos, "event: texto\ndata: hola\n") ||
		!strings.Contains(eventos, "event: fin\ndata: {\"codigo_error\":\"\",\"estado\":\"completado\"}") {
		t.Fatalf("eventos:\n%s", eventos)
	}
	_, reanudados := pedir(t, ts, "GET", "/api/v1/trabajos/"+id+"/eventos", "", "Last-Event-ID", "3")
	if !strings.HasPrefix(reanudados, "id: 4\n") {
		t.Fatalf("Last-Event-ID debe reanudar después del 3:\n%s", reanudados)
	}
}

func TestIdempotenciaYValidacion(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	r1, c1 := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola, "Idempotency-Key", "k1")
	r2, c2 := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola, "Idempotency-Key", "k1")
	if r1.StatusCode != 202 || r2.StatusCode != 200 || decodificar(t, c1)["id"] != decodificar(t, c2)["id"] {
		t.Fatalf("idempotencia: %d %d", r1.StatusCode, r2.StatusCode)
	}
	if r, _ := pedir(t, ts, "POST", "/api/v1/trabajos", `{"mensajes":[{"rol":"usuario","contenido":"otra"}]}`, "Idempotency-Key", "k1"); r.StatusCode != 409 {
		t.Fatalf("misma clave con otra solicitud: %d", r.StatusCode)
	}
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos", `{"mensajes":[],"campo_inventado":1}`); r.StatusCode != 400 || !strings.Contains(cuerpo, "solicitud_invalida") {
		t.Fatalf("campo desconocido: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/trabajos/trabajo_inexistente", ""); r.StatusCode != 404 {
		t.Fatalf("trabajo inexistente: %d", r.StatusCode)
	}
}

func TestColaLlenaDevuelve429(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{Demora: time.Minute}),
		map[string]any{"concurrencia.maximo_trabajos_instancia": 1, "concurrencia.maximo_tamano_cola": 1})
	_, c := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola)
	esperarEstado(t, ts, decodificar(t, c)["id"].(string), nucleo.Ejecutando)
	if r, _ := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola); r.StatusCode != 202 {
		t.Fatalf("el segundo queda en cola: %d", r.StatusCode)
	}
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola); r.StatusCode != 429 || r.Header.Get("Retry-After") == "" || !strings.Contains(cuerpo, "cola_llena") {
		t.Fatalf("cola llena: %d %s", r.StatusCode, cuerpo)
	}
}

func TestEjecutarCancelarReintentarEliminar(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	r, cuerpo := pedir(t, ts, "POST", "/api/v1/ejecutar", solicitudHola)
	v := decodificar(t, strings.TrimSpace(cuerpo))
	if r.StatusCode != 200 || r.Header.Get("Location") == "" || v["estado"] != "completado" || v["salida"] != "hola" {
		t.Fatalf("ejecutar: %d %s", r.StatusCode, cuerpo)
	}
	id := v["id"].(string)
	r, cuerpo = pedir(t, ts, "POST", "/api/v1/trabajos/"+id+"/reintentar", "")
	nuevo := decodificar(t, cuerpo)
	if r.StatusCode != 202 || nuevo["intento_de"] != id {
		t.Fatalf("reintentar: %d %s", r.StatusCode, cuerpo)
	}
	esperarEstado(t, ts, nuevo["id"].(string), nucleo.Completado)
	if r, _ := pedir(t, ts, "DELETE", "/api/v1/trabajos/"+id, ""); r.StatusCode != 204 {
		t.Fatalf("eliminar: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/trabajos/"+id, ""); r.StatusCode != 404 {
		t.Fatalf("eliminado: %d", r.StatusCode)
	}
	if r, cuerpo := pedir(t, ts, "DELETE", "/api/v1/trabajos", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"eliminados":1`) {
		t.Fatalf("vaciar el historial: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/trabajos/"+nuevo["id"].(string), ""); r.StatusCode != 404 {
		t.Fatalf("el historial vaciado conserva el reintento: %d", r.StatusCode)
	}

	lento := servidor(t, simulado(adaptadores.Simulado{Demora: time.Minute}), nil)
	_, c := pedir(t, lento, "POST", "/api/v1/trabajos", solicitudHola)
	idLento := decodificar(t, c)["id"].(string)
	esperarEstado(t, lento, idLento, nucleo.Ejecutando)
	if r, _ := pedir(t, lento, "DELETE", "/api/v1/trabajos/"+idLento, ""); r.StatusCode != 409 {
		t.Fatalf("no se elimina un trabajo activo: %d", r.StatusCode)
	}
	if r, cuerpo := pedir(t, lento, "DELETE", "/api/v1/trabajos", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"eliminados":0`) {
		t.Fatalf("vaciar el historial no toca los activos: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, lento, "POST", "/api/v1/trabajos/"+idLento+"/cancelar", ""); r.StatusCode != 202 {
		t.Fatalf("cancelar: %d", r.StatusCode)
	}
	esperarEstado(t, lento, idLento, nucleo.Cancelado)
}

func TestCargaMultipartYTrabajo(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	var cuerpo bytes.Buffer
	m := multipart.NewWriter(&cuerpo)
	parte, _ := m.CreateFormFile("archivo", "notas.txt")
	parte.Write([]byte("contenido de prueba"))
	m.Close()
	r, respuesta := pedir(t, ts, "POST", "/api/v1/cargas", cuerpo.String(), "Content-Type", m.FormDataContentType())
	carga := decodificar(t, respuesta)
	if r.StatusCode != 201 || carga["tamano_bytes"] != float64(19) {
		t.Fatalf("carga: %d %s", r.StatusCode, respuesta)
	}
	solicitud := `{"cargas":["` + carga["identificador_carga"].(string) + `"],"mensajes":[{"rol":"usuario","contenido":"resume"}]}`
	_, c := pedir(t, ts, "POST", "/api/v1/ejecutar", solicitud)
	if salida := decodificar(t, strings.TrimSpace(c))["salida"].(string); !strings.Contains(salida, "<<<archivo nombre=\"notas.txt\"") || !strings.HasSuffix(salida, "resume") {
		t.Fatalf("el agente debía recibir el archivo y el mensaje: %q", salida)
	}
}

func TestChatCompletions(t *testing.T) {
	ts := servidor(t, simulado(trozos{}), nil)
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions",
		`{"model":"simulado","messages":[{"role":"system","content":"breve"},{"role":"user","content":[{"type":"text","text":"hola"}]}],"temperature":0.2,"n":1,"tools":[]}`)
	v := decodificar(t, cuerpo)
	if r.StatusCode != 200 {
		t.Fatalf("chat: %d %s", r.StatusCode, cuerpo)
	}
	mensaje := v["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	if v["object"] != "chat.completion" || mensaje["content"] != "Hola mundo" ||
		v["usage"].(map[string]any)["total_tokens"] != float64(10) || !strings.Contains(r.Header.Get("Tramtai-Advertencia"), "temperature") {
		t.Fatalf("chat: %s %v", cuerpo, r.Header)
	}

	r, flujo := pedir(t, ts, "POST", "/openai/v1/chat/completions",
		`{"model":"perfil/rapido","messages":[{"role":"user","content":"hola"}],"stream":true,"stream_options":{"include_usage":true}}`)
	if r.Header.Get("Content-Type") != "text/event-stream" || !strings.Contains(flujo, `"delta":{"content":"Ho"}`) ||
		!strings.Contains(flujo, `"finish_reason":"stop"`) || !strings.Contains(flujo, `"total_tokens":10`) || !strings.HasSuffix(flujo, "data: [DONE]\n\n") {
		t.Fatalf("chat en streaming:\n%s", flujo)
	}
}

func TestPoliticaDeParametrosYModelo(t *testing.T) {
	ts := servidor(t, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}, "otro": adaptadores.Simulado{}},
		map[string]any{"modelos.perfiles.rapido.otro": "m1", "modelos.perfiles.profundo.otro": "m2"})
	casos := map[string]string{
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"tools":[{"type":"function"}]}`: "parametro_no_soportado",
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"n":2}`:                         "parametro_no_soportado",
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"inventado":1}`:                 "parametro_no_soportado",
		`{"model":"m1","messages":[{"role":"user","content":"x"}]}`:                                     "modelo_ambiguo",
		`{"model":"desconocido","messages":[{"role":"user","content":"x"}]}`:                            "modelo_no_disponible",
		`{"model":"simulado","messages":[{"role":"user","content":[{"type":"image_url"}]}]}`:            "parametro_no_soportado",
	}
	for peticion, codigo := range casos {
		r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", peticion)
		if r.StatusCode != 400 || decodificar(t, cuerpo)["error"].(map[string]any)["code"] != codigo {
			t.Errorf("%s: %d %s, se esperaba %s", peticion, r.StatusCode, cuerpo, codigo)
		}
	}
	if r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"m2","messages":[{"role":"user","content":"x"}]}`); r.StatusCode != 200 {
		t.Fatalf("un modelo de un solo agente se acepta sin prefijo: %d %s", r.StatusCode, cuerpo)
	}
	_, modelos := pedir(t, ts, "GET", "/openai/v1/models", "")
	for _, id := range []string{`"otro/m1"`, `"perfil/rapido"`, `"simulado"`, `"simulado/m1"`} {
		if !strings.Contains(modelos, id) {
			t.Errorf("falta %s en %s", id, modelos)
		}
	}
}

func TestRespuestasYEsquema(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{Respuesta: `{"ok":true}`}), nil)
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/responses",
		`{"model":"simulado","input":"hola","instructions":"breve","text":{"format":{"type":"json_schema","name":"x","schema":{"type":"object","required":["ok"]}}}}`)
	v := decodificar(t, cuerpo)
	if r.StatusCode != 200 || v["object"] != "response" || v["output_text"] != `{"ok":true}` {
		t.Fatalf("responses: %d %s", r.StatusCode, cuerpo)
	}
	_, flujo := pedir(t, ts, "POST", "/openai/v1/responses", `{"model":"simulado","input":[{"role":"user","content":"hola"}],"stream":true}`)
	for _, evento := range []string{"event: response.created", "event: response.output_text.delta", "event: response.completed"} {
		if !strings.Contains(flujo, evento) {
			t.Fatalf("falta %s:\n%s", evento, flujo)
		}
	}
	malo := servidor(t, simulado(adaptadores.Simulado{Respuesta: `{"ok":"no"}`}), nil)
	r, cuerpo = pedir(t, malo, "POST", "/openai/v1/chat/completions",
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"response_format":{"type":"json_schema","json_schema":{"name":"x","schema":{"type":"object","properties":{"ok":{"type":"boolean"}}}}}}`)
	if r.StatusCode != 422 || !strings.Contains(cuerpo, "salida_no_cumple_esquema") {
		t.Fatalf("salida que no cumple: %d %s", r.StatusCode, cuerpo)
	}
}

func TestMensajesAnthropic(t *testing.T) {
	ts := servidor(t, simulado(trozos{}), nil)
	if r, cuerpo := pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"simulado","messages":[{"role":"user","content":"hola"}]}`); r.StatusCode != 400 ||
		decodificar(t, cuerpo)["type"] != "error" {
		t.Fatalf("max_tokens es obligatorio: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo := pedir(t, ts, "POST", "/anthropic/v1/messages",
		`{"model":"simulado","max_tokens":100,"system":[{"type":"text","text":"breve"}],"messages":[{"role":"user","content":"hola"}]}`,
		"Authorization", "", "x-api-key", "secreto", "anthropic-version", "2023-06-01")
	v := decodificar(t, cuerpo)
	if r.StatusCode != 200 || v["type"] != "message" || v["content"].([]any)[0].(map[string]any)["text"] != "Hola mundo" ||
		!strings.Contains(r.Header.Get("Tramtai-Advertencia"), "max_tokens") {
		t.Fatalf("messages: %d %s", r.StatusCode, cuerpo)
	}
	_, flujo := pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"simulado","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"hola"}]}`)
	orden := []string{"event: message_start", "event: content_block_start", "event: content_block_delta", "event: content_block_stop", "event: message_delta", "event: message_stop"}
	ultimo := -1
	for _, evento := range orden {
		i := strings.Index(flujo, evento)
		if i <= ultimo {
			t.Fatalf("orden de eventos incorrecto en %s:\n%s", evento, flujo)
		}
		ultimo = i
	}
}
