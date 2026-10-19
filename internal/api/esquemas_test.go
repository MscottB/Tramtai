package api

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// porTurnos responde cada vez con la siguiente respuesta (la última se repite).
type porTurnos struct {
	mu         *sync.Mutex
	n          *int
	respuestas []string
}

func nuevoPorTurnos(respuestas ...string) porTurnos {
	return porTurnos{mu: &sync.Mutex{}, n: new(int), respuestas: respuestas}
}

func (p porTurnos) Ejecutar(context.Context, nucleo.SolicitudEjecucion, func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	*p.n++
	return adaptadores.Resultado{Salida: p.respuestas[min(*p.n, len(p.respuestas))-1]}, nil
}

func TestValidarEsquema(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	casos := []struct{ cuerpo, contiene string }{
		{`{"esquema":{"type":"object"}}`, `{"valido":true}`},
		{`{"esquema":"{\"type\": \"object\"}"}`, `{"valido":true}`},
		{`{"esquema":"{\n  \"type\": \"object\",\n  \"x\" 1\n}"}`, `"linea":3`},
		{`{"esquema":{"type":"objeto"}}`, "`type`"},
		{`{"esquema":"` + strings.Repeat(" ", 257<<10) + `{}"}`, `"valido":false`},
	}
	for _, c := range casos {
		r, cuerpo := pedir(t, ts, "POST", "/api/v1/esquemas/validar", c.cuerpo)
		if r.StatusCode != 200 || !strings.Contains(cuerpo, c.contiene) {
			t.Errorf("%.60s: %d %s; se esperaba %s", c.cuerpo, r.StatusCode, cuerpo, c.contiene)
		}
	}
	if r, _ := pedir(t, ts, "POST", "/api/v1/esquemas/validar", `{}`); r.StatusCode != 400 {
		t.Errorf("sin esquema: %d", r.StatusCode)
	}
}

func TestErroresSalidaYCorregir(t *testing.T) {
	ts := servidor(t, simulado(nuevoPorTurnos(`{"total":"diez"}`, `{"total":10}`)), nil)
	sol := `{"agente":"simulado","mensajes":[{"rol":"usuario","contenido":"x"}],"salida":{"formato":"json","conservar_salida_invalida":true,
		"esquema":{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}}}`
	_, cuerpo := pedir(t, ts, "POST", "/api/v1/ejecutar", sol)
	var final struct {
		ID            string                                    `json:"id"`
		Codigo        string                                    `json:"codigo_error"`
		ErroresSalida []struct{ Campo, Problema, Texto string } `json:"errores_salida"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(cuerpo)), &final); err != nil || final.Codigo != "salida_no_cumple_esquema" ||
		len(final.ErroresSalida) != 1 || final.ErroresSalida[0].Campo != "total" || final.ErroresSalida[0].Problema != "tipo" {
		t.Fatalf("ejecutar devuelve los problemas: %v %s", err, cuerpo)
	}
	if _, cuerpo := pedir(t, ts, "GET", "/api/v1/trabajos/"+final.ID, ""); !strings.Contains(cuerpo, `"errores_salida":[{"campo":"total","problema":"tipo"`) {
		t.Fatalf("GET del trabajo con errores_salida: %s", cuerpo)
	}
	r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos/"+final.ID+"/reintentar", `{"corregir":true}`)
	if r.StatusCode != 202 {
		t.Fatalf("corregir: %d %s", r.StatusCode, cuerpo)
	}
	nuevo := decodificar(t, cuerpo)["id"].(string)
	esperarEstado(t, ts, nuevo, nucleo.Completado)
	_, cuerpo = pedir(t, ts, "GET", "/api/v1/trabajos/"+nuevo, "")
	if !strings.Contains(cuerpo, `"salida":"{\"total\":10}"`) || !strings.Contains(cuerpo, `"rol":"asistente"`) {
		t.Fatalf("la corrección completa con la conversación: %s", cuerpo)
	}
	// Corregir un trabajo completado: 400 correccion_no_disponible. Sin cuerpo sigue siendo un reintento normal.
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos/"+nuevo+"/reintentar", `{"corregir":true}`); r.StatusCode != 400 || !strings.Contains(cuerpo, "correccion_no_disponible") {
		t.Fatalf("corregir sin fallo de esquema: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/trabajos/"+nuevo+"/reintentar", ""); r.StatusCode != 202 {
		t.Fatalf("reintentar sin cuerpo: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts, "POST", "/api/v1/trabajos/"+nuevo+"/reintentar", `{"otro":1}`); r.StatusCode != 400 {
		t.Fatalf("un campo desconocido se rechaza: %d", r.StatusCode)
	}
}

func TestFachadasCorrigenConLosReintentosDelPunto(t *testing.T) {
	esquema := json.RawMessage(`{"type":"object","required":["ok"],"properties":{"ok":{"type":"boolean"}}}`)
	for _, ruta := range [][2]string{
		{"/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":"x"}]}`},
		{"/anthropic/v1/messages", `{"model":"simulado","max_tokens":10,"messages":[{"role":"user","content":"x"}]}`},
	} {
		ts := servidorCon(t, simulado(nuevoPorTurnos(`{}`, `{"ok":true}`)), nil,
			&Punto{Nombre: "r", Contratos: map[string]bool{"openai": true, "anthropic": true}, Esquema: esquema, ReintentosSalida: 1})
		if r, cuerpo := pedir(t, ts, "POST", ruta[0], ruta[1]); r.StatusCode != 200 || !strings.Contains(cuerpo, `{\"ok\":true}`) {
			t.Fatalf("%s: la API corrige con reintentos_salida: %d %s", ruta[0], r.StatusCode, cuerpo)
		}
	}
}
