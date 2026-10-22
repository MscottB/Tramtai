package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
	"tramtai/internal/documentos"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
)

// trozos emite la respuesta en tres fragmentos, como un agente con streaming por tokens (el planificador puede agruparlos).
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
	return servidorCon(t, registro, ajustes, nil)
}

// servidorCon es servidor con la política de un punto de acceso (nil: la API de la instancia).
func servidorCon(t *testing.T, registro map[string]adaptadores.Adaptador, ajustes map[string]any, punto *Punto) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(nuevoServidor(t, registro, ajustes, punto).Handler())
	t.Cleanup(ts.Close)
	return ts
}

// nuevoServidor arma el servidor con su planificador y su almacén; se cierran al terminar la prueba.
func nuevoServidor(t *testing.T, registro map[string]adaptadores.Adaptador, ajustes map[string]any, punto *Punto) *Servidor {
	t.Helper()
	dir := t.TempDir()
	alm, err := persistencia.Abrir(filepath.Join(dir, "instancia.db"))
	if err != nil {
		t.Fatal(err)
	}
	valores := map[string]any{"predeterminados.agente": "simulado", "modelos.perfiles.rapido.simulado": "m1"}
	maps.Copy(valores, ajustes)
	c := configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: valores})
	cargas, err := documentos.AbrirCargas(filepath.Join(dir, "cargas"), time.Hour, 1<<20, int64(c.Limite("conversion.cuota_cargas_mb"))<<20)
	if err != nil {
		t.Fatal(err)
	}
	opc := planificador.OpcionesDesde(c, "global")
	conversor := documentos.Desde(c)
	conversor.Raiz = dir // el conversor que haya preparado el usuario de este equipo no cuenta en las pruebas
	opc.Preparador = documentos.Preparador{Cargas: cargas, Conversor: conversor}
	limites := map[string]int{}
	for nombre := range registro {
		limites[nombre] = 2
	}
	p := planificador.Nuevo(alm, registro, planificador.NuevosSemaforos(limites), opc)
	p.FijarPresupuestos(map[string]nucleo.Presupuesto{persistencia.TodaLaInstancia: configuracion.PresupuestoDe(c)}) // como el servicio
	if err := p.Iniciar(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := &Servidor{Planificador: p, Cargas: cargas, Registro: registro, Config: c, Token: "secreto",
		Hosts: []string{"127.0.0.1", "localhost", "::1"}, Sondeo: 10 * time.Millisecond, Punto: punto}
	t.Cleanup(func() {
		p.Detener()
		alm.Cerrar()
	})
	return s
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

func TestFichasAgentes(t *testing.T) {
	ts := servidor(t, map[string]adaptadores.Adaptador{"simulado": trozos{}, "antigravity": trozos{}, "gemini": trozos{}}, nil)
	sinClave, err := http.Get(ts.URL + "/api/v1/agentes")
	if err != nil {
		t.Fatal(err)
	}
	sinClave.Body.Close()
	if sinClave.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sin clave: %d", sinClave.StatusCode)
	}
	r, cuerpo := pedir(t, ts, "GET", "/api/v1/agentes", "")
	var datos struct {
		Agentes []struct {
			Agente, Nombre, Codigo string
			PermitidoApps          bool `json:"permitido_apps"`
		}
	}
	if r.StatusCode != http.StatusOK || json.Unmarshal([]byte(cuerpo), &datos) != nil || len(datos.Agentes) != 3 {
		t.Fatalf("%d %s", r.StatusCode, cuerpo)
	}
	// En el orden de las fichas, y los que no tienen ficha al final con su id.
	a := datos.Agentes
	if a[0].Agente != "gemini" || a[0].Codigo != "GM" || !a[0].PermitidoApps || a[1].Agente != "antigravity" || a[1].PermitidoApps ||
		a[2].Agente != "simulado" || a[2].Nombre != "simulado" || a[2].Codigo != "" || !a[2].PermitidoApps {
		t.Fatalf("fichas: %+v", a)
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

func TestRegistroSinTexto(t *testing.T) {
	ts := servidor(t, simulado(trozos{}), nil)
	_, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola)
	id := decodificar(t, cuerpo)["id"].(string)
	esperarEstado(t, ts, id, nucleo.Completado)
	r, cuerpo := pedir(t, ts, "GET", "/api/v1/trabajos/"+id+"/registro", "")
	if r.StatusCode != 200 {
		t.Fatalf("registro: %d %s", r.StatusCode, cuerpo)
	}
	var registro struct {
		Eventos []struct {
			Numero   int       `json:"numero"`
			Tipo     string    `json:"tipo"`
			Datos    string    `json:"datos"`
			CreadoEn time.Time `json:"creado_en"`
		} `json:"eventos"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &registro); err != nil {
		t.Fatal(err)
	}
	tipos := map[string]int{}
	for i, e := range registro.Eventos {
		tipos[e.Tipo]++
		if i > 0 && e.CreadoEn.Before(registro.Eventos[i-1].CreadoEn) {
			t.Fatalf("las horas no pueden retroceder: %+v", registro.Eventos)
		}
	}
	if tipos["texto"] != 0 || tipos["estado"] == 0 || tipos["intento"] != 1 {
		t.Fatalf("registro sin texto, con estados y un intento: %v", tipos)
	}
	// Mismo permiso que GET /api/v1/trabajos/{id}: lo que no se ve da 404 y sin clave, 401.
	if r, _ := pedir(t, ts, "GET", "/api/v1/trabajos/trabajo_inexistente/registro", ""); r.StatusCode != 404 {
		t.Fatalf("un trabajo inexistente da 404: %d", r.StatusCode)
	}
	if r, err := http.Get(ts.URL + "/api/v1/trabajos/" + id + "/registro"); err != nil || r.StatusCode != 401 {
		t.Fatalf("sin clave, 401: %v %v", r, err)
	}
}

func TestRegistroDeOtroPunto(t *testing.T) {
	s := nuevoServidor(t, simulado(adaptadores.Simulado{}), nil, nil)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	_, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola)
	id := decodificar(t, cuerpo)["id"].(string)
	esperarEstado(t, ts, id, nucleo.Completado)
	// Una API solo ve sus trabajos: el de la instancia no existe para ella.
	s.Punto = &Punto{Nombre: "otra"}
	if r, _ := pedir(t, ts, "GET", "/api/v1/trabajos/"+id+"/registro", ""); r.StatusCode != 404 {
		t.Fatalf("el trabajo de otro punto da 404: %d", r.StatusCode)
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
	if salida := decodificar(t, strings.TrimSpace(c))["salida"].(string); !strings.Contains(salida, "nombre=\"notas.txt\"") || !strings.Contains(salida, "<<<archivo-") || !strings.HasSuffix(salida, "resume") {
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
	if r.Header.Get("Content-Type") != "text/event-stream" || !strings.Contains(flujo, `"delta":{"content":"Ho`) ||
		!strings.Contains(flujo, `"finish_reason":"stop"`) || !strings.Contains(flujo, `"total_tokens":10`) || !strings.HasSuffix(flujo, "data: [DONE]\n\n") {
		t.Fatalf("chat en streaming:\n%s", flujo)
	}
}

func TestPoliticaDeParametrosYModelo(t *testing.T) {
	ts := servidor(t, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}, "otro": adaptadores.Simulado{}},
		map[string]any{"modelos.perfiles.rapido.otro": "m1", "modelos.perfiles.profundo.otro": "m2"})
	casos := map[string]string{
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"tools":[{"type":"web_search"}]}`: "parametro_no_soportado",
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"n":2}`:                           "parametro_no_soportado",
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"inventado":1}`:                   "parametro_no_soportado",
		`{"model":"m1","messages":[{"role":"user","content":"x"}]}`:                                       "modelo_ambiguo",
		`{"model":"desconocido","messages":[{"role":"user","content":"x"}]}`:                              "modelo_no_disponible",
		`{"model":"simulado","messages":[{"role":"user","content":[{"type":"image_url"}]}]}`:              "parametro_no_soportado",
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

func TestInstruccionesDelPunto(t *testing.T) {
	c := configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{"predeterminados.agente": "simulado"}})
	s := &Servidor{Config: c, Punto: &Punto{Nombre: "es", Instrucciones: "Responde siempre en español."}}
	sol, err := s.ajustar(nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "Hi"}}})
	if err != nil || sol.Instrucciones != "Responde siempre en español.\n\nSé breve." {
		t.Fatalf("%v %q", err, sol.Instrucciones)
	}
	if sol, _ := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "Hi"}}}); sol.Instrucciones != "Responde siempre en español." {
		t.Fatalf("sin instrucciones propias: %q", sol.Instrucciones)
	}
}

func TestPuntoAplicaEsquemaYRespaldo(t *testing.T) {
	c := configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{"predeterminados.agente": "simulado"}})
	s := &Servidor{Config: c, Punto: &Punto{Nombre: "r", Esquema: json.RawMessage(`{"type":"object"}`), Respaldo: []string{"otro"}, ReintentosSalida: 2}}
	mensajes := []nucleo.Mensaje{{Rol: "usuario", Contenido: "Hi"}}
	sol, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: mensajes})
	if err != nil || sol.Salida.Formato != "json" || string(sol.Salida.Esquema) != `{"type":"object"}` || !slices.Equal(sol.Respaldo, []string{"otro"}) || sol.Salida.Reintentos != 2 {
		t.Fatalf("el punto debe aportar esquema, respaldo y reintentos: %v %+v", err, sol)
	}
	propia := nucleo.SolicitudEjecucion{Mensajes: mensajes, Respaldo: []string{"tercero"},
		Salida: nucleo.ConfigSalida{Formato: "json", Esquema: json.RawMessage(`{"type":"array"}`), Reintentos: 1}}
	if sol, err := s.ajustar(propia); err != nil || string(sol.Salida.Esquema) != `{"type":"array"}` || !slices.Equal(sol.Respaldo, []string{"tercero"}) || sol.Salida.Reintentos != 1 {
		t.Fatalf("la solicitud prevalece: %v %+v", err, sol)
	}
	// Sin salida JSON los reintentos del punto no aplican: no habría nada que corregir.
	s.Punto.Esquema = nil
	if sol, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: mensajes}); err != nil || sol.Salida.Reintentos != 0 {
		t.Fatalf("sin JSON no hay reintentos: %v %+v", err, sol.Salida)
	}
}

func TestFachadasInformanDelRespaldo(t *testing.T) {
	registro := map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{Error: nucleo.NuevoError(nucleo.CodigoLimiteUsoProveedor, "límite")},
		"otro": adaptadores.Simulado{Respuesta: "ok"}}
	ts := servidorCon(t, registro, nil, &Punto{Nombre: "r", Contratos: map[string]bool{"openai": true, "anthropic": true}, Respaldo: []string{"otro"}})
	_, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":"x"}]}`)
	if v := decodificar(t, cuerpo); v["model"] != "otro" {
		t.Fatalf("tras un salto, model debe ser el agente que respondió: %s", cuerpo)
	}
	_, cuerpo = pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"simulado","max_tokens":10,"messages":[{"role":"user","content":"x"}]}`)
	if v := decodificar(t, cuerpo); v["model"] != "otro" {
		t.Fatalf("tras un salto, model debe ser el agente que respondió: %s", cuerpo)
	}
}

func TestPuntoForzadoImponeSuRespaldo(t *testing.T) {
	c := configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{"predeterminados.agente": "simulado"}})
	s := &Servidor{Config: c, Punto: &Punto{Nombre: "r", Agente: "simulado", ForzarModelo: true, Respaldo: []string{"otro"}}}
	sol, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "Hi"}}, Respaldo: []string{"caro/opus"}})
	if err != nil || !slices.Equal(sol.Respaldo, []string{"otro"}) {
		t.Fatalf("con forzar_modelo el cliente no elige el respaldo: %v %+v", err, sol.Respaldo)
	}
}

func TestPuntoRespaldoNoPermitido(t *testing.T) {
	c := configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{"predeterminados.agente": "simulado"}})
	s := &Servidor{Config: c, Punto: &Punto{Nombre: "r", ModelosPermitidos: []string{"simulado"}, Respaldo: []string{"otro"}}}
	if _, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "Hi"}}}); nucleo.CodigoDe(err) != nucleo.CodigoModeloNoPermitido {
		t.Fatalf("un respaldo fuera de modelos_permitidos se rechaza: %v", err)
	}
	// El esfuerzo propio no cuenta para modelos_permitidos.
	s.Punto = &Punto{Nombre: "r", ModelosPermitidos: []string{"simulado", "otro"}, Respaldo: []string{"otro+alto"}}
	if _, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "Hi"}}}); err != nil {
		t.Fatalf("otro+alto cabe en modelos_permitidos [otro]: %v", err)
	}
}

func TestFachadaOpenAIConEsquemaDelPunto(t *testing.T) {
	ts := servidorCon(t, simulado(adaptadores.Simulado{Respuesta: "no es json"}), nil,
		&Punto{Nombre: "r", Contratos: map[string]bool{"openai": true}, Esquema: json.RawMessage(`{"type":"object"}`)})
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":"x"}]}`)
	if r.StatusCode != 422 || !strings.Contains(cuerpo, "salida_no_cumple_esquema") {
		t.Fatalf("el esquema del punto se valida sin response_format: %d %s", r.StatusCode, cuerpo)
	}
}

// conEsfuerzo guarda el esfuerzo con el que se ejecutó la última solicitud.
type conEsfuerzo struct{ visto *string }

func (c conEsfuerzo) Ejecutar(_ context.Context, s nucleo.SolicitudEjecucion, _ func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	*c.visto = s.Esfuerzo
	return adaptadores.Resultado{Salida: "ok"}, nil
}

func TestEsfuerzo(t *testing.T) {
	var visto string
	ts := servidor(t, simulado(conEsfuerzo{&visto}), nil)
	for peticion, esperado := range map[string]string{
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"reasoning_effort":"high"}`:    "alto",
		`{"model":"simulado","messages":[{"role":"user","content":"x"}],"reasoning_effort":"minimal"}`: "bajo",
		`{"model":"simulado","messages":[{"role":"user","content":"x"}]}`:                              "",
	} {
		if r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", peticion); r.StatusCode != 200 || visto != esperado {
			t.Fatalf("%s: %d %s, esfuerzo %q", peticion, r.StatusCode, cuerpo, visto)
		}
	}
	if r, cuerpo := pedir(t, ts, "POST", "/openai/v1/responses", `{"model":"simulado","input":"x","reasoning":{"effort":"xhigh"}}`); r.StatusCode != 200 || visto != "muy_alto" {
		t.Fatalf("responses: %d %s, esfuerzo %q", r.StatusCode, cuerpo, visto)
	}
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/ejecutar", `{"agente":"simulado","esfuerzo":"maximo","mensajes":[{"rol":"usuario","contenido":"x"}]}`); r.StatusCode != 200 || visto != "maximo" {
		t.Fatalf("nativa: %d %s, esfuerzo %q", r.StatusCode, cuerpo, visto)
	}
	if r, _ := pedir(t, ts, "POST", "/api/v1/ejecutar", `{"agente":"simulado","esfuerzo":"enorme","mensajes":[{"rol":"usuario","contenido":"x"}]}`); r.StatusCode != 400 {
		t.Fatalf("un esfuerzo desconocido se rechaza: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":"x"}],"reasoning_effort":"enorme"}`); r.StatusCode != 400 {
		t.Fatalf("un reasoning_effort desconocido se rechaza: %d", r.StatusCode)
	}
	// El punto de acceso pone su esfuerzo si la solicitud no trae uno.
	c := configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{"predeterminados.agente": "simulado"}})
	s := &Servidor{Config: c, Punto: &Punto{Nombre: "p", Esfuerzo: "alto"}}
	mensajes := []nucleo.Mensaje{{Rol: "usuario", Contenido: "x"}}
	if sol, _ := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: mensajes}); sol.Esfuerzo != "alto" {
		t.Fatalf("esfuerzo del punto: %q", sol.Esfuerzo)
	}
	if sol, _ := s.ajustar(nucleo.SolicitudEjecucion{Esfuerzo: "bajo", Mensajes: mensajes}); sol.Esfuerzo != "bajo" {
		t.Fatalf("el de la solicitud manda: %q", sol.Esfuerzo)
	}
}

// conModelo informa un modelo efectivo, como Claude con su versión real detrás del alias.
type conModelo struct{}

func (conModelo) Ejecutar(context.Context, nucleo.SolicitudEjecucion, func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	return adaptadores.Resultado{Salida: "ok", ModeloEfectivo: "m1-2026"}, nil
}

func TestCatalogoConVersionesYPredeterminado(t *testing.T) {
	ts := servidor(t, map[string]adaptadores.Adaptador{"simulado": conModelo{}}, map[string]any{"predeterminados.perfil_modelo": "rapido"})
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/ejecutar", `{"agente":"simulado","modelo":"m1","mensajes":[{"rol":"usuario","contenido":"hola"}]}`); r.StatusCode != 200 {
		t.Fatalf("ejecutar: %d %s", r.StatusCode, cuerpo)
	}
	_, cuerpo := pedir(t, ts, "GET", "/api/v1/modelos", "")
	for _, esperado := range []string{
		`"id":"simulado/m1","agente":"simulado","modelo":"m1","origen":"configurado","version":"m1-2026"`,
		`"id":"perfil/rapido","perfil":"rapido","origen":"configurado","resuelto":"simulado/m1","version":"m1-2026","predeterminado":true`,
		`"id":"simulado/m1-2026","agente":"simulado","modelo":"m1-2026","origen":"visto"`,
	} {
		if !strings.Contains(cuerpo, esperado) {
			t.Fatalf("falta %s en %s", esperado, cuerpo)
		}
	}
	// La versión vista también se lista, una sola vez, en /openai/v1/models.
	if _, modelos := pedir(t, ts, "GET", "/openai/v1/models", ""); strings.Count(modelos, `"simulado/m1-2026"`) != 1 {
		t.Fatalf("la versión vista debe aparecer una vez: %s", modelos)
	}
}

// resolvible dice qué modelo concreto usará sin ejecutar nada, como Claude con su evento init.
type resolvible struct{ consultas *atomic.Int32 }

func (resolvible) Ejecutar(context.Context, nucleo.SolicitudEjecucion, func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	return adaptadores.Resultado{Salida: "ok", ModeloEfectivo: "m1-viejo"}, nil
}

func (r resolvible) ResolverModelo(_ context.Context, modelo string) (string, error) {
	r.consultas.Add(1)
	if modelo == "" {
		return "predeterminado-2027", nil
	}
	return modelo + "-2027", nil
}

func TestCatalogoConVersionAntesDeUsar(t *testing.T) {
	var consultas atomic.Int32
	ts := servidor(t, map[string]adaptadores.Adaptador{"resolvible": resolvible{&consultas}},
		map[string]any{"predeterminados.agente": "resolvible", "modelos.perfiles.rapido.resolvible": "m1"})
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/ejecutar", `{"agente":"resolvible","modelo":"m1","mensajes":[{"rol":"usuario","contenido":"hola"}]}`); r.StatusCode != 200 {
		t.Fatalf("ejecutar: %d %s", r.StatusCode, cuerpo)
	}
	_, cuerpo := pedir(t, ts, "GET", "/api/v1/modelos", "")
	for _, esperado := range []string{
		// lo que el agente usará ahora manda sobre lo que informó la última vez
		`"id":"resolvible/m1","agente":"resolvible","modelo":"m1","origen":"configurado","version":"m1-2027"`,
		`"id":"resolvible","agente":"resolvible","origen":"predeterminado_del_cli","version":"predeterminado-2027"`,
	} {
		if !strings.Contains(cuerpo, esperado) {
			t.Fatalf("falta %s en %s", esperado, cuerpo)
		}
	}
	antes := consultas.Load()
	pedir(t, ts, "GET", "/api/v1/modelos", "")
	if consultas.Load() != antes {
		t.Fatalf("la segunda consulta del catálogo volvió a preguntar al agente: %d → %d", antes, consultas.Load())
	}
}

func TestHuellaValida(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer tt_secreto")
	if !HuellaValida(r, HuellaToken("tt_secreto")) || HuellaValida(r, HuellaToken("tt_otro")) || HuellaValida(r, "tt_secreto") {
		t.Fatal("la clave se comprueba contra su huella, nunca en claro")
	}
}

func TestPresupuestoAgotadoFachadas(t *testing.T) {
	ts := servidor(t, simulado(trozos{}), map[string]any{"presupuesto.diario.peticiones": 1})
	if r, _ := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola); r.StatusCode != 202 {
		t.Fatalf("la primera petición entra: %d", r.StatusCode)
	}
	casos := []struct {
		ruta, cuerpo string
		comprobar    func(map[string]any) bool
	}{
		{"/api/v1/trabajos", solicitudHola, func(v map[string]any) bool {
			return v["error"].(map[string]any)["codigo"] == "presupuesto_agotado"
		}},
		{"/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":"hola"}]}`, func(v map[string]any) bool {
			e := v["error"].(map[string]any)
			return e["code"] == "presupuesto_agotado" && e["type"] == "rate_limit_error"
		}},
		{"/anthropic/v1/messages", `{"model":"simulado","max_tokens":100,"messages":[{"role":"user","content":"hola"}]}`, func(v map[string]any) bool {
			return v["error"].(map[string]any)["type"] == "rate_limit_error"
		}},
	}
	for _, c := range casos {
		r, cuerpo := pedir(t, ts, "POST", c.ruta, c.cuerpo)
		if r.StatusCode != 429 || !c.comprobar(decodificar(t, cuerpo)) {
			t.Errorf("%s: %d %s", c.ruta, r.StatusCode, cuerpo)
		}
		if segundos, err := strconv.Atoi(r.Header.Get("Retry-After")); err != nil || segundos < 1 || segundos > 86400 {
			t.Errorf("%s: Retry-After %q", c.ruta, r.Header.Get("Retry-After"))
		}
	}
}

func TestCabeceraPresupuesto(t *testing.T) {
	ts := servidor(t, simulado(trozos{}), map[string]any{"presupuesto.mensual.peticiones": 4})
	if r, _ := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola); r.Header.Get("X-Tramtai-Presupuesto") != "0" {
		t.Fatalf("antes de la primera: %q", r.Header.Get("X-Tramtai-Presupuesto"))
	}
	pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola)
	if r, _ := pedir(t, ts, "POST", "/api/v1/trabajos", solicitudHola); r.Header.Get("X-Tramtai-Presupuesto") != "50" {
		t.Fatalf("con 2 de 4: %q", r.Header.Get("X-Tramtai-Presupuesto"))
	}
	sin := servidor(t, simulado(trozos{}), nil)
	if r, _ := pedir(t, sin, "POST", "/api/v1/trabajos", solicitudHola); r.Header.Get("X-Tramtai-Presupuesto") != "" {
		t.Fatalf("sin presupuesto no hay cabecera: %q", r.Header.Get("X-Tramtai-Presupuesto"))
	}
}

func TestFachadasEntradaDemasiadoGrande(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{Respuesta: "ok"}), map[string]any{"limites.contexto_tokens": []any{"simulado=10"}})
	largo := strings.Repeat("x", 4000)
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":"`+largo+`"}]}`)
	if r.StatusCode != 400 || !strings.Contains(cuerpo, `"code":"context_length_exceeded"`) {
		t.Fatalf("OpenAI: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo = pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"simulado","max_tokens":10,"messages":[{"role":"user","content":"`+largo+`"}]}`)
	if r.StatusCode != 400 || !strings.Contains(cuerpo, `"invalid_request_error"`) {
		t.Fatalf("Anthropic: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo = pedir(t, ts, "POST", "/api/v1/ejecutar", `{"agente":"simulado","mensajes":[{"rol":"usuario","contenido":"`+largo+`"}]}`)
	// La API nativa síncrona responde 200 al aceptar y el trabajo final lleva el código, como cualquier fallo.
	if r.StatusCode != 200 || decodificar(t, cuerpo)["codigo_error"] != "entrada_demasiado_grande" {
		t.Fatalf("nativa: %d %s", r.StatusCode, cuerpo)
	}
}

func TestPuntoImponeModoContexto(t *testing.T) {
	c := configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{"predeterminados.agente": "simulado"}})
	hola := []nucleo.Mensaje{{Rol: "usuario", Contenido: "Hi"}}
	s := &Servidor{Config: c, Punto: &Punto{Nombre: "r"}}
	if sol, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: hola}); err != nil || sol.ModoContexto != "aislado" {
		t.Fatalf("una API sin modo trabaja aislada: %q %v", sol.ModoContexto, err)
	}
	// La petición (o el perfil que aplique, que se resuelve antes) no amplía el modo de su API.
	if _, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: hola, ModoContexto: "consciente_proyecto"}); nucleo.CodigoDe(err) != nucleo.CodigoContextoNoPermitido {
		t.Fatalf("consciente_proyecto en una API aislada: %v", err)
	}
	s.Punto.ModoContexto = "consciente_proyecto"
	if sol, err := s.ajustar(nucleo.SolicitudEjecucion{Mensajes: hola}); err != nil || sol.ModoContexto != "consciente_proyecto" {
		t.Fatalf("la API con consciente_proyecto lo da: %q %v", sol.ModoContexto, err)
	}
}

func TestCuotaCargasHTTP(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), map[string]any{"conversion.cuota_cargas_mb": 1})
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/cargas?nombre=a.txt", strings.Repeat("a", 600<<10)); r.StatusCode != 201 {
		t.Fatalf("primera: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/cargas?nombre=b.txt", strings.Repeat("b", 600<<10)); r.StatusCode != 507 || !strings.Contains(cuerpo, "cuota_cargas_agotada") {
		t.Fatalf("la que supera la cuota: %d %s", r.StatusCode, cuerpo)
	}
}

func TestCapacidadesLimites(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), map[string]any{"conversion.maximo_carga_mb": 30})
	for _, ruta := range []string{"/api/v1/limites", "/api/v1/capacidades"} {
		r, cuerpo := pedir(t, ts, "GET", ruta, "")
		datos := decodificar(t, cuerpo)
		if ruta == "/api/v1/capacidades" {
			datos, _ = datos["limites"].(map[string]any)
		}
		if r.StatusCode != 200 || datos["carga_mb"] != float64(30) || datos["cargas_por_peticion"] != float64(20) ||
			datos["mensajes"] != float64(1000) || datos["reserva_salida_tokens"] != float64(8192) || datos["texto_convertido_mb"] != float64(20) {
			t.Fatalf("%s: %d %s", ruta, r.StatusCode, cuerpo)
		}
		// Los formatos admitidos son la misma lista que usa la conversión: la Web UI y las apps no la repiten.
		texto, conversor := fmt.Sprint(datos["extensiones_texto"]), fmt.Sprint(datos["extensiones_conversor"])
		if !strings.Contains(texto, ".txt") || !strings.Contains(conversor, ".pdf") || strings.Contains(texto+conversor, ".zip") {
			t.Fatalf("%s: formatos %s %s", ruta, texto, conversor)
		}
	}
	if r, cuerpo := pedir(t, ts, "GET", "/api/v1/capacidades", ""); r.StatusCode != 200 || strings.Contains(cuerpo, `"zip"`) {
		t.Fatalf("capacidades sin ZIP: %s", cuerpo)
	}
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/cargas?nombre=paquete.zip", "PK"); r.StatusCode != 400 || !strings.Contains(cuerpo, "carga_invalida") {
		t.Fatalf("un ZIP se rechaza al subirlo: %d %s", r.StatusCode, cuerpo)
	}
}

func TestCatalogoConContexto(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), map[string]any{"limites.contexto_tokens": []any{"simulado=12345"}})
	_, cuerpo := pedir(t, ts, "GET", "/api/v1/modelos", "")
	for _, m := range decodificar(t, cuerpo)["modelos"].([]any) {
		entrada := m.(map[string]any)
		if entrada["id"] == "simulado" {
			if entrada["contexto_tokens"] != float64(12345) || entrada["contexto_origen"] != "configurado" {
				t.Fatalf("contexto en el catálogo: %v", entrada)
			}
			return
		}
	}
	t.Fatalf("sin la entrada simulado: %s", cuerpo)
}

func TestCrearTrabajoSinConversor(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	_, respuesta := pedir(t, ts, "POST", "/api/v1/cargas?nombre=informe.pdf", "%PDF-1.4")
	solicitud := `{"cargas":["` + decodificar(t, respuesta)["identificador_carga"].(string) + `"],"mensajes":[{"rol":"usuario","contenido":"resume"}]}`
	r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos", solicitud)
	if r.StatusCode != 503 || !strings.Contains(cuerpo, "conversor_no_disponible") {
		t.Fatalf("sin conversor, 503 al aceptar: %d %s", r.StatusCode, cuerpo)
	}
	if _, lista := pedir(t, ts, "GET", "/api/v1/trabajos", ""); strings.Contains(lista, "trabajo_") {
		t.Fatalf("no queda trabajo creado: %s", lista)
	}
}

func TestRetryAfterConversorPreparando(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/trabajos", nil)
	EscribirError(w, r, &nucleo.ConversorPreparando{Err: nucleo.NuevoError(nucleo.CodigoConversorNoDisponible, "preparando")})
	if w.Code != 503 || w.Header().Get("Retry-After") != "60" {
		t.Fatalf("preparando: %d Retry-After=%q", w.Code, w.Header().Get("Retry-After"))
	}
}
