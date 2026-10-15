package api

import (
	"encoding/json"
	"net/http"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// max_tokens se acepta como tope pero ningún adaptador CLI lo aplica: se ignora con aviso.
var politicaMensajes = politica{
	soportados:   conjunto("model", "messages", "system", "stream", "tools", "tool_choice"),
	ignorables:   conjunto("max_tokens", "temperature", "top_p", "top_k", "metadata", "service_tier"),
	noSoportados: conjunto("stop_sequences", "thinking", "mcp_servers", "container"),
}

type peticionMensajes struct {
	Model     string           `json:"model"`
	MaxTokens *int             `json:"max_tokens"`
	Messages  []mensajeEntrada `json:"messages"`
	System    json.RawMessage  `json:"system"`
	Stream    bool             `json:"stream"`
	Tools     []struct {
		Type        string          `json:"type"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	} `json:"tools"`
	ToolChoice *struct {
		Type                   string `json:"type"`
		Name                   string `json:"name"`
		DisableParallelToolUse bool   `json:"disable_parallel_tool_use"`
	} `json:"tool_choice"`
}

// herramientasAnthropic prepara tools y tool_choice (auto, any, tool o none).
func herramientasAnthropic(sol *nucleo.SolicitudEjecucion, p peticionMensajes) (*herramientas, error) {
	lista := make([]herramienta, 0, len(p.Tools))
	for _, t := range p.Tools {
		if err := soloFunciones(t.Type, "", "custom"); err != nil {
			return nil, err
		}
		lista = append(lista, herramienta{Nombre: t.Name, Descripcion: t.Description, Parametros: t.InputSchema})
	}
	var e eleccion
	if c := p.ToolChoice; c != nil {
		e.SoloUna = c.DisableParallelToolUse
		switch c.Type {
		case "auto":
		case "any":
			e.Modo = "obligatoria"
		case "tool":
			e.Modo, e.Nombre = "una", c.Name
		case "none":
			e.Modo = "ninguna"
		default:
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("tool_choice desconocido: %s", c.Type))
		}
	}
	return prepararHerramientas(sol, lista, e)
}

// bloquesAnthropic da el contenido de la respuesta: el texto y un bloque tool_use por llamada.
func bloquesAnthropic(texto string, llamadas []llamada) []any {
	var bloques []any
	if conMensaje(texto, llamadas) {
		bloques = append(bloques, map[string]any{"type": "text", "text": texto})
	}
	for _, l := range llamadas {
		bloques = append(bloques, map[string]any{"type": "tool_use", "id": l.ID, "name": l.Nombre, "input": json.RawMessage(l.Argumentos)})
	}
	return bloques
}

func razonAnthropic(llamadas []llamada) string {
	if len(llamadas) > 0 {
		return "tool_use"
	}
	return "end_turn"
}

// mensajesAnthropic implementa /anthropic/v1/messages: system, mensajes de texto, herramientas y streaming básico.
func (s *Servidor) mensajesAnthropic(w http.ResponseWriter, r *http.Request) {
	var p peticionMensajes
	avisos, err := s.leerFachada(w, r, politicaMensajes, &p)
	if err == nil && p.MaxTokens == nil {
		err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("max_tokens es obligatorio"))
	}
	var sol nucleo.SolicitudEjecucion
	if err == nil {
		sol, err = s.solicitudFachada(p.Model)
	}
	if err == nil && len(p.System) > 0 && string(p.System) != "null" {
		sol.Instrucciones, err = textoDe(p.System)
	}
	if err == nil {
		sol.Mensajes, err = mensajes(p.Messages)
	}
	var h *herramientas
	if err == nil {
		h, err = herramientasAnthropic(&sol, p)
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
	mensaje := func(final nucleo.Trabajo, contenido []any, razon any) map[string]any {
		return map[string]any{"id": "msg_" + t.ID, "type": "message", "role": "assistant", "model": p.Model, "content": contenido,
			"stop_reason": razon, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": final.Uso.TokensEntrada, "output_tokens": final.Uso.TokensSalida}}
	}

	if !p.Stream {
		final, err := s.ejecutarFachada(r, t, nil, nil)
		if err != nil {
			return
		}
		if final.Estado != nucleo.Completado {
			EscribirError(w, r, errorDeTrabajo(final))
			return
		}
		texto, llamadas, err := h.respuesta(final, "toolu_"+t.ID)
		if err != nil {
			EscribirError(w, r, err)
			return
		}
		avisarUso(w, final)
		EscribirJSON(w, http.StatusOK, mensaje(final, bloquesAnthropic(texto, llamadas), razonAnthropic(llamadas)))
		return
	}

	f := abrirFlujo(w)
	evento := func(tipo string, datos map[string]any) error {
		datos["type"] = tipo
		return f.json(tipo, datos)
	}
	abrirTexto := func() {
		_ = evento("content_block_start", map[string]any{"index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
	}
	_ = evento("message_start", map[string]any{"message": mensaje(t, []any{}, nil)})
	if h == nil {
		abrirTexto() // el texto llega en vivo; con herramientas no se sabe qué habrá hasta el final
	}
	_ = evento("ping", map[string]any{})
	directo := enVivo(sol)
	delta := func(texto string) error {
		return evento("content_block_delta", map[string]any{"index": 0, "delta": map[string]any{"type": "text_delta", "text": texto}})
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
	fallar := func(e error) {
		_ = evento("error", map[string]any{"error": map[string]any{"type": tipoAnthropic(estadosHTTP[nucleo.CodigoDe(e)]), "message": nucleo.MensajeDe(e)}})
	}
	if final.Estado != nucleo.Completado {
		fallar(errorDeTrabajo(final))
		return
	}
	texto, llamadas, err := h.respuesta(final, "toolu_"+t.ID)
	if err != nil {
		fallar(err)
		return
	}
	indice := 0
	if conMensaje(texto, llamadas) {
		if h != nil {
			abrirTexto()
		}
		if !directo {
			_ = delta(texto)
		}
		_ = evento("content_block_stop", map[string]any{"index": 0})
		indice++
	}
	for _, l := range llamadas {
		_ = evento("content_block_start", map[string]any{"index": indice,
			"content_block": map[string]any{"type": "tool_use", "id": l.ID, "name": l.Nombre, "input": map[string]any{}}})
		_ = evento("content_block_delta", map[string]any{"index": indice, "delta": map[string]any{"type": "input_json_delta", "partial_json": l.Argumentos}})
		_ = evento("content_block_stop", map[string]any{"index": indice})
		indice++
	}
	_ = evento("message_delta", map[string]any{"delta": map[string]any{"stop_reason": razonAnthropic(llamadas), "stop_sequence": nil},
		"usage": map[string]any{"input_tokens": final.Uso.TokensEntrada, "output_tokens": final.Uso.TokensSalida}})
	_ = evento("message_stop", map[string]any{})
}
