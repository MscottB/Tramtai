package api

import (
	"encoding/json"
	"net/http"

	"tramtai/internal/nucleo"
)

// max_tokens se acepta como tope pero ningún adaptador CLI lo aplica: se ignora con aviso (sección 8).
var politicaMensajes = politica{
	soportados:   conjunto("model", "messages", "system", "stream"),
	ignorables:   conjunto("max_tokens", "temperature", "top_p", "top_k", "metadata", "service_tier"),
	noSoportados: conjunto("tools", "tool_choice", "stop_sequences", "thinking", "mcp_servers", "container"),
}

type peticionMensajes struct {
	Model     string           `json:"model"`
	MaxTokens *int             `json:"max_tokens"`
	Messages  []mensajeEntrada `json:"messages"`
	System    json.RawMessage  `json:"system"`
	Stream    bool             `json:"stream"`
}

// mensajesAnthropic implementa /anthropic/v1/messages: system, mensajes de texto y streaming básico.
func (s *Servidor) mensajesAnthropic(w http.ResponseWriter, r *http.Request) {
	var p peticionMensajes
	avisos, err := s.leerFachada(w, r, politicaMensajes, &p)
	if err == nil && p.MaxTokens == nil {
		err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "max_tokens es obligatorio")
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
		avisarUso(w, final)
		EscribirJSON(w, http.StatusOK, mensaje(final, []any{map[string]any{"type": "text", "text": final.Salida}}, "end_turn"))
		return
	}

	f := abrirFlujo(w)
	evento := func(tipo string, datos map[string]any) error {
		datos["type"] = tipo
		return f.json(tipo, datos)
	}
	_ = evento("message_start", map[string]any{"message": mensaje(t, []any{}, nil)})
	_ = evento("content_block_start", map[string]any{"index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
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
	if final.Estado != nucleo.Completado {
		e := errorDeTrabajo(final)
		_ = evento("error", map[string]any{"error": map[string]any{"type": tipoAnthropic(estadosHTTP[nucleo.CodigoDe(e)]), "message": nucleo.MensajeDe(e)}})
		return
	}
	if !directo {
		_ = delta(final.Salida)
	}
	_ = evento("content_block_stop", map[string]any{"index": 0})
	_ = evento("message_delta", map[string]any{"delta": map[string]any{"stop_reason": "end_turn", "stop_sequence": nil},
		"usage": map[string]any{"input_tokens": final.Uso.TokensEntrada, "output_tokens": final.Uso.TokensSalida}})
	_ = evento("message_stop", map[string]any{})
}
