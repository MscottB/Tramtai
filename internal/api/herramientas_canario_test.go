//go:build canario

// Herramientas con los agentes reales: con cada uno que esté instalado y con sesión o clave, el ciclo completo (el
// agente pide clima, se le devuelve el resultado y responde) por OpenAI, Anthropic y la API propia. Los que falten se
// saltan avisando. Usa las suscripciones del usuario con modelos pequeños.
//
// Ejecutar: go test -tags canario -run TestHerramientasConAgentesReales -v -count=1 ./internal/api/
package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes/claude"
	"tramtai/internal/agentes/codex"
	"tramtai/internal/agentes/copilot"
	"tramtai/internal/agentes/gemini"
)

const (
	preguntaClima  = "¿Qué tiempo hace en Santiago de Chile ahora? Usa la herramienta clima."
	resultadoClima = "18 °C y despejado"
	esquemaClima   = `{"type":"object","properties":{"ciudad":{"type":"string"}},"required":["ciudad"]}`
)

// sinSesion salta el agente si la respuesta dice que no tiene sesión ni clave: no es un fallo de Tramtai.
func sinSesion(t *testing.T, codigo int, cuerpo string) {
	t.Helper()
	if codigo == 502 && strings.Contains(cuerpo, "autenticacion_agente") {
		t.Skipf("el agente no tiene sesión ni clave: %s", cuerpo)
	}
}

func TestHerramientasConAgentesReales(t *testing.T) {
	suscripcion := adaptadores.NuevoModo(adaptadores.AutenticacionSuscripcion)
	casos := []struct {
		agente    string
		adaptador adaptadores.Adaptador
		modelo    string
	}{
		{"claude", claude.Claude{Programa: "claude", Autenticacion: suscripcion}, "haiku"},
		{"codex", codex.Codex{Programa: "codex", Autenticacion: suscripcion}, ""},
		// Copilot Free solo admite auto; cada petición gasta un crédito del plan.
		{"copilot", copilot.Copilot{Programa: "copilot", Autenticacion: suscripcion}, "auto"},
		{"gemini", gemini.Gemini{}, "gemini-flash-latest"},
	}
	for _, c := range casos {
		t.Run(c.agente, func(t *testing.T) {
			if d, ok := c.adaptador.(interface {
				Detectar(context.Context) adaptadores.Estado
			}); ok {
				if e := d.Detectar(context.Background()); e.Problema != "" {
					t.Skipf("%s: %s", c.agente, e.Problema)
				}
			}
			ts := servidor(t, map[string]adaptadores.Adaptador{c.agente: c.adaptador}, nil)
			modelo := c.agente
			if c.modelo != "" {
				modelo += "/" + c.modelo
			}

			t.Run("openai", func(t *testing.T) {
				herramientas := `[{"type":"function","function":{"name":"clima","description":"El tiempo actual de una ciudad","parameters":` + esquemaClima + `}}]`
				pregunta := `{"role":"user","content":"` + preguntaClima + `"}`
				r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"`+modelo+`","tools":`+herramientas+`,"messages":[`+pregunta+`]}`)
				sinSesion(t, r.StatusCode, cuerpo)
				var primera struct {
					Choices []struct {
						Message json.RawMessage `json:"message"`
					} `json:"choices"`
				}
				decodificarEn(t, cuerpo, &primera)
				var mensaje struct {
					ToolCalls []struct {
						ID       string `json:"id"`
						Function struct {
							Name, Arguments string
						} `json:"function"`
					} `json:"tool_calls"`
				}
				if r.StatusCode != 200 || len(primera.Choices) == 0 {
					t.Fatalf("%d %s", r.StatusCode, cuerpo)
				}
				decodificarEn(t, string(primera.Choices[0].Message), &mensaje)
				if len(mensaje.ToolCalls) == 0 || mensaje.ToolCalls[0].Function.Name != "clima" || !strings.Contains(mensaje.ToolCalls[0].Function.Arguments, "ciudad") {
					t.Fatalf("debía pedir clima con ciudad: %s", cuerpo)
				}
				resultado := `{"role":"tool","tool_call_id":"` + mensaje.ToolCalls[0].ID + `","content":"` + resultadoClima + `"}`
				r, cuerpo = pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"`+modelo+`","tools":`+herramientas+
					`,"messages":[`+pregunta+`,`+string(primera.Choices[0].Message)+`,`+resultado+`]}`)
				if r.StatusCode != 200 || !strings.Contains(cuerpo, "18") {
					t.Fatalf("la respuesta final debía usar el resultado: %d %s", r.StatusCode, cuerpo)
				}
			})

			t.Run("anthropic", func(t *testing.T) {
				herramientas := `[{"name":"clima","description":"El tiempo actual de una ciudad","input_schema":` + esquemaClima + `}]`
				pregunta := `{"role":"user","content":"` + preguntaClima + `"}`
				r, cuerpo := pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"`+modelo+`","max_tokens":1024,"tools":`+herramientas+`,"messages":[`+pregunta+`]}`)
				sinSesion(t, r.StatusCode, cuerpo)
				var primera struct {
					Content []struct {
						Type, ID, Name string
						Input          map[string]any
					} `json:"content"`
				}
				decodificarEn(t, cuerpo, &primera)
				id := ""
				for _, b := range primera.Content {
					if b.Type == "tool_use" && b.Name == "clima" && b.Input["ciudad"] != nil {
						id = b.ID
					}
				}
				if r.StatusCode != 200 || id == "" {
					t.Fatalf("debía pedir clima con ciudad: %d %s", r.StatusCode, cuerpo)
				}
				var bloques struct {
					Content json.RawMessage `json:"content"`
				}
				decodificarEn(t, cuerpo, &bloques)
				resultado := `{"role":"user","content":[{"type":"tool_result","tool_use_id":"` + id + `","content":"` + resultadoClima + `"}]}`
				r, cuerpo = pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"`+modelo+`","max_tokens":1024,"tools":`+herramientas+
					`,"messages":[`+pregunta+`,{"role":"assistant","content":`+string(bloques.Content)+`},`+resultado+`]}`)
				if r.StatusCode != 200 || !strings.Contains(cuerpo, "18") {
					t.Fatalf("la respuesta final debía usar el resultado: %d %s", r.StatusCode, cuerpo)
				}
			})

			t.Run("propia", func(t *testing.T) {
				herramientas := `[{"nombre":"clima","descripcion":"El tiempo actual de una ciudad","parametros":` + esquemaClima + `}]`
				solicitud := func(mensajes string) string {
					s := `{"agente":"` + c.agente + `","herramientas":` + herramientas + `,"mensajes":[` + mensajes + `]`
					if c.modelo != "" {
						s += `,"modelo":"` + c.modelo + `"`
					}
					return s + `}`
				}
				pregunta := `{"rol":"usuario","contenido":"` + preguntaClima + `"}`
				r, cuerpo := pedir(t, ts, "POST", "/api/v1/ejecutar", solicitud(pregunta))
				v := decodificar(t, strings.TrimSpace(cuerpo))
				if v["codigo_error"] == "autenticacion_agente" {
					t.Skipf("el agente no tiene sesión ni clave: %v", v["mensaje_error"])
				}
				llamadas := llamadasDe(v)
				if r.StatusCode != 200 || len(llamadas) == 0 || llamadas[0]["nombre"] != "clima" {
					t.Fatalf("debía pedir clima: %d %s", r.StatusCode, cuerpo)
				}
				llamada, _ := json.Marshal(llamadas[0])
				r, cuerpo = pedir(t, ts, "POST", "/api/v1/ejecutar", solicitud(pregunta+`,{"rol":"asistente","contenido":"","llamadas":[`+string(llamada)+`]},`+
					`{"rol":"herramienta","id_llamada":"`+llamadas[0]["id"].(string)+`","contenido":"`+resultadoClima+`"}`))
				if v = decodificar(t, strings.TrimSpace(cuerpo)); r.StatusCode != 200 || !strings.Contains(v["salida"].(string), "18") {
					t.Fatalf("la respuesta final debía usar el resultado: %d %s", r.StatusCode, cuerpo)
				}
			})
		})
	}
}
