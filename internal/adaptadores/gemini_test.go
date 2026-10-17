package adaptadores

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"tramtai/internal/nucleo"
)

// TestMain hace de agy falso cuando AGY_FALSO está definido: comprueba el aislamiento y responde en JSON.
func TestMain(m *testing.M) {
	if modo := os.Getenv("AGY_FALSO"); modo != "" {
		ajustes, err := os.ReadFile(filepath.Join(os.Getenv("USERPROFILE"), ".gemini", "antigravity-cli", "settings.json"))
		switch {
		case err != nil || !strings.Contains(string(ajustes), `"modelProvider": "gemini"`):
			fmt.Println(`{"status":"ERROR","response":"sin modelProvider gemini"}`)
		case os.Getenv("GEMINI_API_KEY") != "clave-falsa":
			fmt.Println(`{"status":"ERROR","response":"API key not valid"}`)
		case modo == "entorno":
			r, _ := json.Marshal(map[string]any{"status": "SUCCESS", "response": os.Getenv("USERPROFILE") + "|" + os.Getenv("APPDATA") + "|" +
				os.Getenv("LOCALAPPDATA") + "|" + os.Getenv("XDG_CONFIG_HOME")})
			fmt.Println(string(r))
		case modo == "esquema":
			fmt.Println(`{"status":"SUCCESS","response":"{\"a\":1,\"extra\":2}","structured_output":{"a":1},"usage":{"input_tokens":10,"output_tokens":3}}`)
		default:
			prompt := ""
			for i, a := range os.Args {
				if a == "-p" && i+1 < len(os.Args) {
					prompt = os.Args[i+1]
				}
			}
			r, _ := json.Marshal(map[string]any{"status": "SUCCESS", "response": "eco: " + prompt + "\n",
				"usage": map[string]int{"input_tokens": 100, "output_tokens": 7, "cache_read_tokens": 5}})
			fmt.Println(string(r))
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

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
	r, err := g.Ejecutar(context.Background(), s, func(e Evento) error { fragmentos = append(fragmentos, e.Datos); return nil })
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
	if _, err := g.Ejecutar(context.Background(), s, func(Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoModeloNoDisponible {
		t.Fatalf("modelo inexistente: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "otra")
	if _, err := g.Ejecutar(context.Background(), s, func(Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente || strings.Contains(err.Error(), "otra") {
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
	if _, err := g.Ejecutar(context.Background(), s, func(Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoErrorProtocoloAgente {
		t.Fatalf("un flujo cortado sin finishReason no se da por completado: %v", err)
	}
	s.Modelo = "completo"
	var avisos []string
	r, err := g.Ejecutar(context.Background(), s, func(e Evento) error {
		if e.Tipo == "aviso" {
			avisos = append(avisos, e.Datos)
		}
		return nil
	})
	if err != nil || r.Salida != "respuesta a medias" || len(avisos) != 1 || !strings.Contains(avisos[0], "MAX_TOKENS") {
		t.Fatalf("MAX_TOKENS es válido pero se avisa: %+v %v %v", r, avisos, err)
	}
}

func TestAntigravityAisladoYSoloConClave(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "eco")
	a := Antigravity{Programa: os.Args[0]}
	r, err := a.Ejecutar(context.Background(), solicitudSimple("hola"), func(Evento) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Salida, "Sé breve.") || !strings.Contains(r.Salida, "hola") || r.Uso.TokensEntrada != 105 || r.Uso.TokensSalida != 7 {
		t.Fatalf("resultado: %+v", r)
	}
	t.Setenv("AGY_FALSO", "esquema")
	s := solicitudSimple("x")
	s.Salida.Esquema = json.RawMessage(`{"type":"object"}`)
	if r, err := a.Ejecutar(context.Background(), s, func(Evento) error { return nil }); err != nil || r.Salida != `{"a":1}` {
		t.Fatalf("structured_output prevalece: %+v %v", r, err)
	}
	if _, err := (Antigravity{Programa: os.Args[0], Autenticacion: AutenticacionSuscripcion}).Ejecutar(context.Background(), s,
		func(Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoConfiguracionInvalida {
		t.Fatalf("la sesión personal no se admite: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := a.Ejecutar(context.Background(), s, func(Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoAutenticacionAgente {
		t.Fatalf("sin clave: %v", err)
	}
}

func TestContextoClaude(t *testing.T) {
	var r resultadoClaude
	datos := `{"result":"ok","modelUsage":{"claude-haiku-4-5":{"outputTokens":5,"contextWindow":200000},"aux":{"outputTokens":1,"contextWindow":100}}}`
	if err := json.Unmarshal([]byte(datos), &r); err != nil {
		t.Fatal(err)
	}
	if r.contexto() != 200000 {
		t.Fatalf("el contexto del modelo efectivo: %d", r.contexto())
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

func TestAntigravityNoEmpiezaPorGuion(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "eco")
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "--yolo"}}}
	r, err := Antigravity{Programa: os.Args[0]}.Ejecutar(context.Background(), s, func(Evento) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	// El argumento tras -p no empieza por «-»: un CLI no lo puede leer como una opción.
	if !strings.Contains(r.Salida, "--yolo") || strings.HasPrefix(strings.TrimPrefix(r.Salida, "eco: "), "-") {
		t.Fatalf("el prompt llegó como %q", r.Salida)
	}
}

func TestAntigravityEntornoPrivado(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "entorno")
	r, err := Antigravity{Programa: os.Args[0]}.Ejecutar(context.Background(), solicitudSimple("x"), func(Evento) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	partes := strings.Split(r.Salida, "|")
	if len(partes) != 4 || partes[0] == "" {
		t.Fatalf("entorno %q", r.Salida)
	}
	for _, p := range partes[1:] {
		if !strings.HasPrefix(p, partes[0]) {
			t.Fatalf("APPDATA, LOCALAPPDATA y XDG_CONFIG_HOME dentro de la carpeta privada %q: %q", partes[0], r.Salida)
		}
	}
}

func TestAntigravityRechazaConsciente(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	s := solicitudSimple("x")
	s.ModoContexto, s.RaizProyecto = "consciente_proyecto", t.TempDir()
	if _, err := (Antigravity{Programa: os.Args[0]}).Ejecutar(context.Background(), s, func(Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoContextoNoPermitido {
		t.Fatalf("consciente_proyecto no se admite: %v", err)
	}
}

func TestAntigravityLineaOrdenes(t *testing.T) {
	for _, c := range []struct {
		arg      string
		unidades int
	}{{"abc", 3}, {"a b", 5}, {`a"b`, 4}, {`a\"b`, 6}, {`a b\`, 7}, {"😀", 2}} {
		if n := unidadesLineaOrdenes(c.arg); n != c.unidades {
			t.Fatalf("%q: %d unidades, se esperaban %d", c.arg, n, c.unidades)
		}
	}
	t.Setenv("GEMINI_API_KEY", "clave-falsa")
	t.Setenv("AGY_FALSO", "eco")
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: strings.Repeat("x", 28001)}}}
	if _, err := (Antigravity{Programa: os.Args[0]}).Ejecutar(context.Background(), s, func(Evento) error { return nil }); nucleo.CodigoDe(err) != nucleo.CodigoEntradaDemasiadoGrande {
		t.Fatalf("más de 28.000 unidades: %v", err)
	}
}
