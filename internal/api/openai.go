package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tramtai/internal/herramientas"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

var politicaChat = politica{
	soportados: conjunto("model", "messages", "stream", "response_format", "reasoning_effort", "tools", "tool_choice",
		"parallel_tool_calls"),
	ignorables: conjunto("temperature", "top_p", "max_tokens", "max_completion_tokens", "user", "metadata", "stream_options",
		"seed", "presence_penalty", "frequency_penalty", "store", "service_tier"),
	noSoportados: conjunto("functions", "function_call", "n", "logprobs", "top_logprobs", "stop",
		"logit_bias", "audio", "modalities", "prediction", "web_search_options"),
}

var politicaRespuestas = politica{
	soportados: conjunto("model", "input", "instructions", "stream", "text", "reasoning", "tools", "tool_choice",
		"parallel_tool_calls"),
	ignorables: conjunto("temperature", "top_p", "max_output_tokens", "metadata", "user", "store",
		"truncation", "service_tier", "include", "stream_options", "prompt_cache_key", "client_metadata"),
	noSoportados: conjunto("previous_response_id", "background", "prompt", "conversation"),
}

type mensajeEntrada struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	ToolCalls []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"` // chat: llamadas del asistente
	ToolCallID string          `json:"tool_call_id"` // chat: resultado de una llamada (rol tool)
	CallID     string          `json:"call_id"`      // responses: function_call y function_call_output
	Name       string          `json:"name"`
	Arguments  string          `json:"arguments"`
	Output     json.RawMessage `json:"output"`
}

// mensajes traduce los mensajes de OpenAI o Anthropic al contrato interno. Las llamadas a herramientas anteriores
// van en el mensaje del asistente y sus resultados como mensajes «herramienta» (internal/herramientas los escribe para
// el agente); los archivos se devuelven aparte para guardarlos como cargas (guardarEnLinea).
func mensajes(entrada []mensajeEntrada) ([]nucleo.Mensaje, []archivoEnLinea, error) {
	var salida []nucleo.Mensaje
	var archivos []archivoEnLinea
	numero := 0                     // archivos de la petición, para documento-<n>
	var pendientes []nucleo.Llamada // function_call seguidos de responses: un único turno del asistente
	cerrar := func() {
		if len(pendientes) > 0 {
			salida = append(salida, nucleo.Mensaje{Rol: "asistente", Llamadas: pendientes})
			pendientes = nil
		}
	}
	resultado := func(id string, contenido json.RawMessage) error {
		texto, err := textoDe(contenido)
		if err == nil {
			salida = append(salida, nucleo.Mensaje{Rol: "herramienta", IDLlamada: id, Contenido: texto})
		}
		return err
	}
	for indice, m := range entrada {
		switch m.Type {
		case "function_call":
			l, err := llamadaDe(m.CallID, m.Name, json.RawMessage(m.Arguments))
			if err != nil {
				return nil, nil, err
			}
			pendientes = append(pendientes, l)
			continue
		case "function_call_output":
			cerrar()
			if err := resultado(m.CallID, m.Output); err != nil {
				return nil, nil, err
			}
			continue
		case "", "message":
		default:
			return nil, nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("elemento de entrada no soportado: %s", m.Type))
		}
		cerrar()
		if m.Role == "tool" {
			if err := resultado(m.ToolCallID, m.Content); err != nil {
				return nil, nil, err
			}
			continue
		}
		rol, err := rolInterno(m.Role)
		if err != nil {
			return nil, nil, err
		}
		texto, llamadas, resultados, adjuntos, err := contenidoDe(m.Content, indice+1, &numero)
		if err != nil {
			return nil, nil, err
		}
		archivos = append(archivos, adjuntos...)
		// El contenido va en el bloque de documentos; en el turno queda una marca de qué archivo venía en él (y un
		// mensaje con solo un archivo no queda vacío).
		for _, a := range adjuntos {
			texto = strings.TrimSpace(texto + "\n[attached file: " + a.Nombre + "]")
		}
		for _, c := range m.ToolCalls {
			l, err := llamadaDe(c.ID, c.Function.Name, json.RawMessage(c.Function.Arguments))
			if err != nil {
				return nil, nil, err
			}
			llamadas = append(llamadas, l)
		}
		// Los resultados de Anthropic llegan en el mensaje del usuario, antes de su texto.
		salida = append(salida, resultados...)
		if texto != "" || len(llamadas) > 0 || len(resultados) == 0 {
			salida = append(salida, nucleo.Mensaje{Rol: rol, Contenido: texto, Llamadas: llamadas})
		}
	}
	cerrar()
	return salida, archivos, nil
}

// esfuerzoOpenAI traduce reasoning_effort de OpenAI (none, minimal, low, medium, high, xhigh) al nivel interno.
func esfuerzoOpenAI(valor string) (string, error) {
	switch valor {
	case "", "none":
		return "", nil
	case "minimal", "low":
		return "bajo", nil
	case "medium":
		return "medio", nil
	case "high":
		return "alto", nil
	case "xhigh":
		return "muy_alto", nil
	}
	return "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("reasoning_effort desconocido: %s", valor))
}

// formatoSalida aplica response_format (chat) o text.format (responses).
func formatoSalida(sol *nucleo.SolicitudEjecucion, tipo string, esquema json.RawMessage) error {
	switch tipo {
	case "", "text":
	case "json_object":
		sol.Salida.Formato = "json"
	case "json_schema":
		if len(esquema) == 0 {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("json_schema sin schema"))
		}
		sol.Salida.Formato, sol.Salida.Esquema = "json", esquema
	default:
		return nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("formato de respuesta no soportado: %s", tipo))
	}
	return nil
}

func usoOpenAI(t nucleo.Trabajo) map[string]any {
	return map[string]any{"prompt_tokens": t.Uso.TokensEntrada, "completion_tokens": t.Uso.TokensSalida,
		"total_tokens": t.Uso.TokensEntrada + t.Uso.TokensSalida}
}

// funcionOpenAI es una herramienta en chat ({"type","function":{…}}) o en responses ({"type","name",…}).
type funcionOpenAI struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Function    *struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

// herramientasOpenAI traduce tools, tool_choice y parallel_tool_calls de chat o responses.
func herramientasOpenAI(sol *nucleo.SolicitudEjecucion, funciones []funcionOpenAI, eleccionCruda json.RawMessage, paralelas *bool) error {
	for _, f := range funciones {
		if err := soloFunciones(f.Type, "function"); err != nil {
			return err
		}
		h := nucleo.Herramienta{Nombre: f.Name, Descripcion: f.Description, Parametros: f.Parameters}
		if f.Function != nil {
			h = nucleo.Herramienta{Nombre: f.Function.Name, Descripcion: f.Function.Description, Parametros: f.Function.Parameters}
		}
		sol.Herramientas = append(sol.Herramientas, h)
	}
	e, err := eleccionOpenAI(eleccionCruda)
	if err != nil {
		return err
	}
	sol.EleccionHerramienta, sol.LlamadasParalelas = e, paralelas
	return nil
}

// quitarNoFunciones separa las herramientas que no son funciones y devuelve sus tipos, sin repetir.
func quitarNoFunciones(herramientas []funcionOpenAI) (funciones []funcionOpenAI, tipos []string) {
	for _, h := range herramientas {
		switch {
		case h.Type == "function":
			funciones = append(funciones, h)
		case !contiene(tipos, h.Type):
			tipos = append(tipos, h.Type)
		}
	}
	return funciones, tipos
}

// llamadasChat da las llamadas como tool_calls; en streaming cada una lleva su índice.
func llamadasChat(llamadas []nucleo.Llamada, conIndice bool) []any {
	lista := make([]any, len(llamadas))
	for i, l := range llamadas {
		c := map[string]any{"id": l.ID, "type": "function", "function": map[string]any{"name": l.Nombre, "arguments": string(l.Argumentos)}}
		if conIndice {
			c["index"] = i
		}
		lista[i] = c
	}
	return lista
}

type peticionChat struct {
	Model           string           `json:"model"`
	Messages        []mensajeEntrada `json:"messages"`
	Stream          bool             `json:"stream"`
	ReasoningEffort string           `json:"reasoning_effort"`
	StreamOptions   *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
	ResponseFormat *struct {
		Type       string `json:"type"`
		JSONSchema *struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
	Tools             []funcionOpenAI `json:"tools"`
	ToolChoice        json.RawMessage `json:"tool_choice"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls"`
}

// chatCompletions implementa /openai/v1/chat/completions con streaming básico y herramientas.
func (s *Servidor) chatCompletions(w http.ResponseWriter, r *http.Request) {
	var p peticionChat
	avisos, err := s.leerFachada(w, r, politicaChat, &p)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	sol, err := s.solicitudFachada(p.Model)
	var archivos []archivoEnLinea
	if err == nil {
		sol.Mensajes, archivos, err = mensajes(p.Messages)
	}
	if err == nil {
		err = s.guardarEnLinea(archivos, &sol)
	}
	if err == nil {
		sol.Esfuerzo, err = esfuerzoOpenAI(p.ReasoningEffort)
	}
	if err == nil && p.ResponseFormat != nil {
		var esquema json.RawMessage
		if p.ResponseFormat.JSONSchema != nil {
			esquema = p.ResponseFormat.JSONSchema.Schema
		}
		err = formatoSalida(&sol, p.ResponseFormat.Type, esquema)
	}
	if err == nil {
		err = herramientasOpenAI(&sol, p.Tools, p.ToolChoice, p.ParallelToolCalls)
	}
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	sol, t, err := s.aceptarFachada(w, r, sol)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	avisar(w, avisos)
	id, creado := "chatcmpl-"+t.ID, t.CreadoEn.Unix()

	if !p.Stream {
		final, texto, llamadas, err := s.resultadoFachada(r, t, "call_", nil, nil)
		if err != nil {
			if !errors.Is(err, errDesconectado) {
				EscribirError(w, r, err)
			}
			return
		}
		mensaje, fin := map[string]any{"role": "assistant", "content": texto}, "stop"
		if len(llamadas) > 0 {
			mensaje["tool_calls"], fin = llamadasChat(llamadas, false), "tool_calls"
			if texto == "" {
				mensaje["content"] = nil
			}
		}
		avisarUso(w, final)
		EscribirJSON(w, http.StatusOK, map[string]any{"id": id, "object": "chat.completion", "created": creado, "model": modeloRespuesta(p.Model, t, final),
			"choices": []any{map[string]any{"index": 0, "message": mensaje, "finish_reason": fin, "logprobs": nil}},
			"usage":   usoOpenAI(final)})
		return
	}

	f := abrirFlujo(w)
	trozo := func(delta map[string]any, fin any) error {
		return f.json("", map[string]any{"id": id, "object": "chat.completion.chunk", "created": creado, "model": p.Model,
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": fin, "logprobs": nil}}})
	}
	fallar := func(e error) {
		_ = f.json("", map[string]any{"error": errorOpenAI(estadosHTTP[nucleo.CodigoDe(e)], nucleo.CodigoDe(e), nucleo.MensajeDe(e))})
		_ = f.enviar("", "", "[DONE]")
	}
	if trozo(map[string]any{"role": "assistant", "content": ""}, nil) != nil {
		return
	}
	directo := enVivo(sol) && !herramientas.Activas(sol)
	final, texto, llamadas, err := s.resultadoFachada(r, t, "call_", func(texto string) error {
		if !directo {
			return nil
		}
		return trozo(map[string]any{"content": texto}, nil)
	}, f.latido)
	if err != nil {
		if !errors.Is(err, errDesconectado) {
			fallar(err)
		}
		return
	}
	if !directo && texto != "" {
		_ = trozo(map[string]any{"content": texto}, nil)
	}
	fin := "stop"
	if len(llamadas) > 0 {
		_ = trozo(map[string]any{"tool_calls": llamadasChat(llamadas, true)}, nil)
		fin = "tool_calls"
	}
	_ = trozo(map[string]any{}, fin)
	if p.StreamOptions != nil && p.StreamOptions.IncludeUsage {
		_ = f.json("", map[string]any{"id": id, "object": "chat.completion.chunk", "created": creado, "model": modeloRespuesta(p.Model, t, final),
			"choices": []any{}, "usage": usoOpenAI(final)})
	}
	_ = f.enviar("", "", "[DONE]")
}

type peticionRespuesta struct {
	Model        string          `json:"model"`
	Input        json.RawMessage `json:"input"`
	Instructions string          `json:"instructions"`
	Stream       bool            `json:"stream"`
	Reasoning    *struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
	Text *struct {
		Format *struct {
			Type   string          `json:"type"`
			Schema json.RawMessage `json:"schema"`
		} `json:"format"`
	} `json:"text"`
	Tools             []funcionOpenAI `json:"tools"`
	ToolChoice        json.RawMessage `json:"tool_choice"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls"`
}

// objetoRespuesta es el objeto response; salida son sus elementos (vacía mientras está en curso o si falló).
func objetoRespuesta(t nucleo.Trabajo, model, estado string, salida []any) map[string]any {
	var uso any
	if estado == "completed" {
		uso = map[string]any{"input_tokens": t.Uso.TokensEntrada, "output_tokens": t.Uso.TokensSalida,
			"total_tokens": t.Uso.TokensEntrada + t.Uso.TokensSalida}
	}
	if salida == nil {
		salida = []any{}
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

func llamadaRespuesta(l nucleo.Llamada, estado string) map[string]any {
	return map[string]any{"type": "function_call", "id": "fc_" + strings.TrimPrefix(l.ID, "call_"), "call_id": l.ID,
		"name": l.Nombre, "arguments": string(l.Argumentos), "status": estado}
}

// conMensaje indica si la respuesta lleva un mensaje de texto: siempre, salvo cuando solo pide herramientas.
func conMensaje(texto string, llamadas []nucleo.Llamada) bool {
	return texto != "" || len(llamadas) == 0
}

func salidaRespuesta(t nucleo.Trabajo, texto string, llamadas []nucleo.Llamada) []any {
	var salida []any
	if conMensaje(texto, llamadas) {
		salida = append(salida, mensajeRespuesta(t, "completed", texto))
	}
	for _, l := range llamadas {
		salida = append(salida, llamadaRespuesta(l, "completed"))
	}
	return salida
}

// respuestas implementa /openai/v1/responses: input de texto o mensajes, texto o herramientas de salida y streaming.
func (s *Servidor) respuestas(w http.ResponseWriter, r *http.Request) {
	var p peticionRespuesta
	avisos, err := s.leerFachada(w, r, politicaRespuestas, &p)
	if err != nil {
		EscribirError(w, r, err)
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
				err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("input debe ser texto o una lista de mensajes"))
			} else {
				var archivos []archivoEnLinea
				if sol.Mensajes, archivos, err = mensajes(entrada); err == nil {
					err = s.guardarEnLinea(archivos, &sol)
				}
			}
		}
	}
	if err == nil && p.Reasoning != nil {
		sol.Esfuerzo, err = esfuerzoOpenAI(p.Reasoning.Effort)
	}
	if err == nil && p.Text != nil && p.Text.Format != nil {
		err = formatoSalida(&sol, p.Text.Format.Type, p.Text.Format.Schema)
	}
	if err == nil {
		// Codex envía siempre herramientas propias (namespace, web_search…) junto a las funciones: se dejan fuera con aviso.
		var otras []string
		p.Tools, otras = quitarNoFunciones(p.Tools)
		for _, tipo := range otras {
			avisos = append(avisos, "tools["+tipo+"]")
		}
		err = herramientasOpenAI(&sol, p.Tools, p.ToolChoice, p.ParallelToolCalls)
	}
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	sol, t, err := s.aceptarFachada(w, r, sol)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	avisar(w, avisos)

	if !p.Stream {
		final, texto, llamadas, err := s.resultadoFachada(r, t, "call_", nil, nil)
		if err != nil {
			if !errors.Is(err, errDesconectado) {
				EscribirError(w, r, err)
			}
			return
		}
		avisarUso(w, final)
		objeto := objetoRespuesta(final, modeloRespuesta(p.Model, t, final), "completed", salidaRespuesta(final, texto, llamadas))
		objeto["output_text"] = texto
		EscribirJSON(w, http.StatusOK, objeto)
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
	abrirMensaje := func() {
		_ = evento("response.output_item.added", map[string]any{"output_index": 0, "item": mensajeRespuesta(t, "in_progress", "")})
		_ = evento("response.content_part.added", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "part": parte("")})
	}
	delta := func(texto string) error {
		return evento("response.output_text.delta", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "delta": texto})
	}
	_ = evento("response.created", map[string]any{"response": objetoRespuesta(t, p.Model, "in_progress", nil)})
	if !herramientas.Activas(sol) {
		abrirMensaje() // el texto llega en vivo; con herramientas no se sabe qué habrá hasta el final
	}
	directo := enVivo(sol) && !herramientas.Activas(sol)
	final, texto, llamadas, err := s.resultadoFachada(r, t, "call_", func(texto string) error {
		if !directo {
			return nil
		}
		return delta(texto)
	}, f.latido)
	fallar := func(e error) {
		fallo := objetoRespuesta(final, modeloRespuesta(p.Model, t, final), "failed", nil)
		fallo["error"] = map[string]any{"code": nucleo.CodigoDe(e), "message": nucleo.MensajeDe(e)}
		_ = evento("response.failed", map[string]any{"response": fallo})
	}
	if err != nil {
		if !errors.Is(err, errDesconectado) {
			fallar(err)
		}
		return
	}
	indice := 0
	if conMensaje(texto, llamadas) {
		if herramientas.Activas(sol) {
			abrirMensaje()
		}
		if !directo {
			_ = delta(texto)
		}
		_ = evento("response.output_text.done", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "text": texto})
		_ = evento("response.content_part.done", map[string]any{"item_id": idMensaje, "output_index": 0, "content_index": 0, "part": parte(texto)})
		_ = evento("response.output_item.done", map[string]any{"output_index": 0, "item": mensajeRespuesta(final, "completed", texto)})
		indice++
	}
	for _, l := range llamadas {
		item := llamadaRespuesta(l, "in_progress")
		item["arguments"] = ""
		_ = evento("response.output_item.added", map[string]any{"output_index": indice, "item": item})
		_ = evento("response.function_call_arguments.delta", map[string]any{"item_id": item["id"], "output_index": indice, "delta": string(l.Argumentos)})
		_ = evento("response.function_call_arguments.done", map[string]any{"item_id": item["id"], "output_index": indice, "arguments": string(l.Argumentos)})
		_ = evento("response.output_item.done", map[string]any{"output_index": indice, "item": llamadaRespuesta(l, "completed")})
		indice++
	}
	_ = evento("response.completed", map[string]any{"response": objetoRespuesta(final, modeloRespuesta(p.Model, t, final), "completed", salidaRespuesta(final, texto, llamadas))})
}

func (s *Servidor) modelosOpenAI(w http.ResponseWriter, r *http.Request) {
	datos := []any{}
	efectivos, _ := s.Planificador.ModelosEfectivos(r.Context())
	for _, m := range s.catalogo(efectivos) {
		datos = append(datos, map[string]any{"id": m.ID, "object": "model", "created": 0, "owned_by": "tramtai"})
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"object": "list", "data": datos})
}
