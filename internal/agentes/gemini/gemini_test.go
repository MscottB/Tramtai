package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

func solicitudSimple(texto string) nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
}

func TestGeminiStreamingEsquemaYUso(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	var recibido map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "clave-falsa" || strings.Contains(r.URL.String(), "clave-falsa") {
			http.Error(w, `{"error":{"message":"API key not valid"}}`, http.StatusUnauthorized)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/models/gemini-x:streamGenerateContent") || r.URL.Query().Get("alt") != "sse" {
			http.Error(w, `{"error":{"message":"model not found"}}`, http.StatusNotFound)
			return
		}
		datos, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(datos, &recibido)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, `data: {"candidates":[{"content":{"parts":[{"text":"pensando","thought":true},{"text":"{\"a\":"}]}}]}`+"\n\n")
		io.WriteString(w, `data: {"candidates":[{"content":{"parts":[{"text":"1}"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":4,"thoughtsTokenCount":2},"modelVersion":"gemini-x-001"}`+"\n\n")
	}))
	defer srv.Close()
	g := Gemini{Base: srv.URL}
	s := solicitudSimple("hola")
	s.Modelo, s.Salida.Esquema = "gemini-x", json.RawMessage(`{"type":"object"}`)
	var fragmentos []string
	r, err := g.Ejecutar(context.Background(), s, func(e adaptadores.Evento) error { fragmentos = append(fragmentos, e.Datos); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if r.Salida != `{"a":1}` || len(fragmentos) != 2 || r.ModeloEfectivo != "gemini-x-001" || r.Uso.TokensEntrada != 12 || r.Uso.TokensSalida != 6 {
		t.Fatalf("resultado: %+v fragmentos %v", r, fragmentos)
	}
	config, _ := recibido["generationConfig"].(map[string]any)
	sistema, _ := json.Marshal(recibido["systemInstruction"])
	if config["responseMimeType"] != "application/json" || config["responseJsonSchema"] == nil || !strings.Contains(string(sistema), "Sé breve.") {
		t.Fatalf("cuerpo enviado: %v", recibido)
	}

	s.Modelo = "otro"
	if _, err := g.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoModeloNoDisponible {
		t.Fatalf("modelo inexistente: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "otra")
	if _, err := g.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente || strings.Contains(err.Error(), "otra") {
		t.Fatalf("clave inválida, sin filtrarla: %v", err)
	}
}

func TestGeminiFlujoSinFinishReason(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, `data: {"candidates":[{"content":{"parts":[{"text":"respuesta a me"}]}}]}`+"\n\n")
		if strings.Contains(r.URL.Path, "completo") { // si no, el flujo se corta sin finishReason
			io.WriteString(w, `data: {"candidates":[{"content":{"parts":[{"text":"dias"}]},"finishReason":"MAX_TOKENS"}]}`+"\n\n")
		}
	}))
	defer srv.Close()
	g := Gemini{Base: srv.URL}
	s := solicitudSimple("hola")
	s.Modelo = "cortado"
	if _, err := g.Ejecutar(context.Background(), s, func(adaptadores.Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoErrorProtocoloAgente {
		t.Fatalf("un flujo cortado sin finishReason no se da por completado: %v", err)
	}
	s.Modelo = "completo"
	var avisos []string
	r, err := g.Ejecutar(context.Background(), s, func(e adaptadores.Evento) error {
		if e.Tipo == "aviso" {
			avisos = append(avisos, e.Datos)
		}
		return nil
	})
	if err != nil || r.Salida != "respuesta a medias" || len(avisos) != 1 || !strings.Contains(avisos[0], "MAX_TOKENS") {
		t.Fatalf("MAX_TOKENS es válido pero se avisa: %+v %v %v", r, avisos, err)
	}
}

func TestContextoGemini(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	var llamadas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamadas.Add(1)
		if r.URL.Path != "/models/gemini-contexto" || r.Header.Get("x-goog-api-key") != "clave-falsa" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"name":"models/gemini-contexto","inputTokenLimit":1048576}`)
	}))
	defer srv.Close()
	g := Gemini{Base: srv.URL, Cliente: srv.Client()}
	for range 2 {
		if n, err := g.Contexto(context.Background(), "gemini-contexto"); err != nil || n != 1048576 {
			t.Fatalf("contexto %d %v", n, err)
		}
	}
	if llamadas.Load() != 1 {
		t.Fatalf("se guarda 24 h: %d llamadas", llamadas.Load())
	}
}

func TestContextoGeminiConPlazo(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	var llamadas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamadas.Add(1)
		time.Sleep(300 * time.Millisecond)
		http.Error(w, "no", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	anterior := plazoContextoGemini
	plazoContextoGemini = 50 * time.Millisecond
	defer func() { plazoContextoGemini = anterior }()
	g := Gemini{Base: srv.URL, Cliente: srv.Client()}
	inicio := time.Now()
	for range 2 {
		if n, _ := g.Contexto(context.Background(), "gemini-lento"); n != 0 {
			t.Fatalf("sin respuesta, contexto desconocido: %d", n)
		}
	}
	if time.Since(inicio) > 250*time.Millisecond || llamadas.Load() != 1 {
		t.Fatalf("con plazo y el fallo guardado: %s, %d llamadas", time.Since(inicio), llamadas.Load())
	}
}

// La Web UI decide la ayuda por el código del problema, no por su texto (que cambia con el idioma).
func TestDetectarFaltaClave(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	if adaptadores.ClaveEntorno("GEMINI_API_KEY", "GOOGLE_API_KEY") != "" {
		t.Skip("hay una clave de Gemini en el almacén de credenciales")
	}
	if e := (Gemini{}).Detectar(context.Background()); e.ProblemaCodigo != adaptadores.ProblemaFaltaClave {
		t.Fatalf("se esperaba falta_clave: %+v", e)
	}
}
