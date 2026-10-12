package api

import (
	"encoding/json"
	"net/http"

	"tramtai/internal/nucleo"
)

var politicaChat = politica{
	soportados: conjunto("model", "messages", "stream", "response_format"),
	ignorables: conjunto("temperature", "top_p", "max_tokens", "max_completion_tokens", "user", "metadata", "stream_options",
		"seed", "presence_penalty", "frequency_penalty", "reasoning_effort", "store", "service_tier", "parallel_tool_calls"),
	noSoportados: conjunto("tools", "tool_choice", "functions", "function_call", "n", "logprobs", "top_logprobs", "stop",
		"logit_bias", "audio", "modalities", "prediction", "web_search_options"),
}

var politicaRespuestas = politica{
	soportados: conjunto("model", "input", "instructions", "stream", "text"),
	ignorables: conjunto("temperature", "top_p", "max_output_tokens", "metadata", "user", "store", "parallel_tool_calls",
		"truncation", "service_tier", "reasoning", "include", "stream_options"),
	noSoportados: conjunto("tools", "tool_choice", "previous_response_id", "background", "prompt", "conversation"),
}

type mensajeEntrada struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type peticionChat struct {
	Model         string           `json:"model"`
	Messages      []mensajeEntrada `json:"messages"`
	Stream        bool             `json:"stream"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
	ResponseFormat *struct {
		Type       string `json:"type"`
		JSONSchema *struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

// mensajes traduce los mensajes de OpenAI o Anthropic al contrato interno.
func mensajes(entrada []mensajeEntrada) ([]nucleo.Mensaje, error) {
	var salida []nucleo.Mensaje
	for _, m := range entrada {
		if m.Type != "" && m.Type != "message" {
			return nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, "elemento de entrada no soportado: "+m.Type)
		}
		rol, err := rolInterno(m.Role)
		if err != nil {
			return nil, err
		}
		texto, err := textoDe(m.Content)
		if err != nil {
			return nil, err
		}
		salida = append(salida, nucleo.Mensaje{Rol: rol, Contenido: texto})
	}
	return salida, nil
}

// formatoSalida aplica response_format (chat) o text.format (responses).
func formatoSalida(sol *nucleo.SolicitudEjecucion, tipo string, esquema json.RawMessage) error {
	switch tipo {
	case "", "text":
	case "json_object":
		sol.Salida.Formato = "json"
	case "json_schema":
		if len(esquema) == 0 {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "json_schema sin schema")
		}
		sol.Salida.Formato, sol.Salida.Esquema = "json", esquema
	default:
		return nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, "formato de respuesta no soportado: "+tipo)
	}
	return nil
}

func usoOpenAI(t nucleo.Trabajo) map[string]any {
	return map[string]any{"prompt_tokens": t.Uso.TokensEntrada, "completion_tokens": t.Uso.TokensSalida,
		"total_tokens": t.Uso.TokensEntrada + t.Uso.TokensSalida}
}

// chatCompletions implementa /openai/v1/chat/completions con streaming básico (sección 8).
func (s *Servidor) chatCompletions(w http.ResponseWriter, r *http.Request) {
	var p peticionChat
	avisos, err := s.leerFachada(w, r, politicaChat, &p)
	if err != nil {
		escribirError(w, r, err)
		return
	}
	sol, err := s.solicitudFachada(p.Model)
	if err == nil {
		sol.Mensajes, err = mensajes(p.Messages)
	}
	if err == nil && p.ResponseFormat != nil {
		var esquema json.RawMessage
		if p.ResponseFormat.JSONSchema != nil {
			esquema = p.ResponseFormat.JSONSchema.Schema
		}
		err = formatoSalida(&sol, p.ResponseFormat.Type, esquema)
	}
	if err != nil {
		escribirError(w, r, err)
		return
	}
	sol, t, err := s.aceptarFachada(r, sol)
	if err != nil {
		escribirError(w, r, err)
		return
	}
	avisar(w, avisos)
	id, creado := "chatcmpl-"+t.ID, t.CreadoEn.Unix()

	if !p.Stream {
		final, err := s.ejecutarFachada(r, t, nil, nil)
		if err != nil {
			return
		}
		if final.Estado != nucleo.Completado {
			escribirError(w, r, errorDeTrabajo(final))
			return
		}
		avisarUso(w, final)
		escribirJSON(w, http.StatusOK, map[string]any{"id": id, "object": "chat.completion", "created": creado, "model": p.Model,
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": final.Salida},
				"finish_reason": "stop", "logprobs": nil}},
			"usage": usoOpenAI(final)})
		return
	}

	f := abrirFlujo(w)
	trozo := func(delta map[string]any, fin any) error {
		return f.json("", map[string]any{"id": id, "object": "chat.completion.chunk", "created": creado, "model": p.Model,
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": fin, "logprobs": nil}}})
	}
	if trozo(map[string]any{"role": "assistant", "content": ""}, nil) != nil {
		return
	}
	directo := enVivo(sol)
	final, err := s.ejecutarFachada(r, t, func(texto string) error {
		if !directo {
			return nil
		}
		return trozo(map[string]any{"content": texto}, nil)
	}, f.latido)
	if err != nil {
		return
	}
	if final.Estado != nucleo.Completado {
		e := errorDeTrabajo(final)
		_ = f.json("", map[string]any{"error": errorOpenAI(estadosHTTP[nucleo.CodigoDe(e)], nucleo.CodigoDe(e), nucleo.MensajeDe(e))})
		_ = f.enviar("", "", "[DONE]")
		return
	}
	if !directo {
		_ = trozo(map[string]any{"content": final.Salida}, nil)
	}
	_ = trozo(map[string]any{}, "stop")
	if p.StreamOptions != nil && p.StreamOptions.IncludeUsage {
		_ = f.json("", map[string]any{"id": id, "object": "chat.completion.chunk", "created": creado, "model": p.Model,
			"choices": []any{}, "usage": usoOpenAI(final)})
	}
	_ = f.enviar("", "", "[DONE]")
}

type peticionRespuesta struct {
	Model        string          `json:"model"`
	Input        json.RawMessage `json:"input"`
	Instructions string          `json:"instructions"`
	Stream       bool            `json:"stream"`
	Text         *struct {
		Format *struct {
			Type   string          `json:"type"`
			Schema json.RawMessage `json:"schema"`
		} `json:"format"`
	} `json:"text"`
}

func objetoRespuesta(t nucleo.Trabajo, model, estado, texto string) map[string]any {
	salida := []any{}
	var uso any
	if estado == "completed" {
		salida = []any{mensajeRespuesta(t, "completed", texto)}
		uso = map[string]any{"input_tokens": t.Uso.TokensEntrada, "output_tokens": t.Uso.TokensSalida,
			"total_tokens": t.Uso.TokensEntrada + t.Uso.TokensSalida}
	}
	return map[string]any{"id": "resp_" + t.ID, "object": "response", "created_at": t.CreadoEn.Unix(), "status": estado,
		"model": model, "output": salida, "usage": uso, "error": nil}
}

func mensajeRespuesta(t nucleo.Trabajo, estado, texto string) map[string]any {
	contenido := []any{}
	if estado == "completed" {
		contenido = []any{map[string]any{"type": "output_text", "text": texto, "annotations": []any{}}}
	}
	return map[string]any{"type": "message", "id": "msg_" + t.ID, "status": estado, "role": "assistant", "content": contenido}
}

// respuestas implementa /openai/v1/responses: input de texto o mensajes básicos, texto de salida y streaming básico.
func (s *Servidor) respuestas(w http.ResponseWriter, r *http.Request) {
	var p peticionRespuesta
	avisos, err := s.leerFachada(w, r, politicaRespuestas, &p)
	if err != nil {
		escribirError(w, r, err)
		return
	}
	sol, err := s.solicitudFachada(p.Model)
	if err == nil {
		sol.Instrucciones = p.Instructions
		var texto string
		if json.Unmarshal(p.Input, &texto) == nil {
			sol.Mensajes = []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}
		} else {
			var entrada []mensajeEntrada
			if err = json.Unmarshal(p.Input, &entrada); err != nil {
				err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "input debe ser texto o una lista de mensajes")
			} else {
				sol.Mensajes, err = mensajes(entrada)
			}
		}
	}
	if err == nil && p.Text != nil && p.Text.Format != nil {
		err = formatoSalida(&sol, p.Text.Format.Type, p.Text.Format.Schema)
	}
	if err != nil {
		escribirError(w, r, err)
		return
	}
	sol, t, err := s.aceptarFachada(r, sol)
	if err != nil {
		escribirError(w, r, err)
		return
	}
	avisar(w, avisos)

	if !p.Stream {
		final, err := s.ejecutarFachada(r, t, nil, nil)
		if err != nil {
			return
		}
		if final.Estado != nucleo.Completado {
			escribirError(w, r, errorDeTrabajo(final))
			return
		}
		avisarUso(w, final)
		respuesta := objetoRespuesta(final, p.Model, "completed", final.Salida)
		respuesta["output_text"] = final.Salida
		escribirJSON(w, http.StatusOK, respuesta)
		return
	}

	f := abrirFlujo(w)
	secuencia := 0
	evento := func(tipo string, datos map[string]any) error {
		datos["type"], datos["sequence_number"] = tipo, secuencia
		secuencia++
		return f.json(tipo, datos)
	}
	idMensaje := "msg_" + t.ID
	parte := func(texto string) map[string]any {
		return map[string]any{"type": "output_text", "text": texto, "annotations": []any{}}
	}
	_ = evento("response.created", map[string]any{"response": objetoRespuesta(t, p.Model, "in_progress", "")})
	_ = evento("response.output_item.added", map[string]any{"output_index": 0, "item": mensajeRespuesta(t, "in_progress", "")})
	_ = evento("response.content_part.added", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "part": parte("")})
	directo := enVivo(sol)
	delta := func(texto string) error {
		return evento("response.output_text.delta", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "delta": texto})
	}
	final, err := s.ejecutarFachada(r, t, func(texto string) error {
		if !directo {
			return nil
		}
		return delta(texto)
	}, f.latido)
	if err != nil {
		return
	}
	if final.Estado != nucleo.Completado {
		e := errorDeTrabajo(final)
		fallo := objetoRespuesta(final, p.Model, "failed", "")
		fallo["error"] = map[string]any{"code": nucleo.CodigoDe(e), "message": nucleo.MensajeDe(e)}
		_ = evento("response.failed", map[string]any{"response": fallo})
		return
	}
	if !directo {
		_ = delta(final.Salida)
	}
	_ = evento("response.output_text.done", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "text": final.Salida})
	_ = evento("response.content_part.done", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "part": parte(final.Salida)})
	_ = evento("response.output_item.done", map[string]any{"output_index": 0, "item": mensajeRespuesta(final, "completed", final.Salida)})
	_ = evento("response.completed", map[string]any{"response": objetoRespuesta(final, p.Model, "completed", final.Salida)})
}

func (s *Servidor) modelosOpenAI(w http.ResponseWriter, _ *http.Request) {
	datos := []any{}
	for _, m := range s.catalogo() {
		datos = append(datos, map[string]any{"id": m.ID, "object": "model", "created": 0, "owned_by": "tramtai"})
	}
	escribirJSON(w, http.StatusOK, map[string]any{"object": "list", "data": datos})
}
