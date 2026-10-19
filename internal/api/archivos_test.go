package api

import (
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
)

// TestMain hace de markitdown falso (MARKITDOWN_FALSO=eco): imprime el archivo que recibe.
func TestMain(m *testing.M) {
	if os.Getenv("MARKITDOWN_FALSO") == "eco" {
		datos, _ := os.ReadFile(os.Args[len(os.Args)-1])
		fmt.Print(string(datos))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// conConversor es un servidor cuyo conversor es este binario haciendo de markitdown.
func conConversor(t *testing.T) *httptest.Server {
	t.Helper()
	t.Setenv("MARKITDOWN_FALSO", "eco")
	return servidor(t, simulado(adaptadores.Simulado{}), map[string]any{"conversion.comando": os.Args[0]})
}

func b64(texto string) string { return base64.StdEncoding.EncodeToString([]byte(texto)) }

func TestChatConArchivo(t *testing.T) {
	ts := conConversor(t)
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":[
		{"type":"text","text":"resume"},
		{"type":"file","file":{"filename":"informe.pdf","file_data":"data:application/pdf;base64,`+b64("contenido del informe")+`"}}]}]}`)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `nombre=\"informe.pdf\"`) || !strings.Contains(cuerpo, "contenido del informe") {
		t.Fatalf("el agente recibe el PDF convertido: %d %s", r.StatusCode, cuerpo)
	}
}

func TestResponsesConArchivo(t *testing.T) {
	ts := conConversor(t)
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/responses", `{"model":"simulado","input":[{"role":"user","content":[
		{"type":"input_text","text":"resume"},
		{"type":"input_file","filename":"tabla.xlsx","file_data":"data:application/vnd.openxmlformats-officedocument.spreadsheetml.sheet;base64,`+b64("celdas")+`"}]}]}`)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `nombre=\"tabla.xlsx\"`) {
		t.Fatalf("responses con input_file: %d %s", r.StatusCode, cuerpo)
	}
}

func TestAnthropicDocumento(t *testing.T) {
	ts := conConversor(t)
	r, cuerpo := pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"simulado","max_tokens":100,"messages":[{"role":"user","content":[
		{"type":"document","title":"Contrato","source":{"type":"base64","media_type":"application/pdf","data":"`+b64("cláusulas")+`"}},
		{"type":"document","source":{"type":"text","media_type":"text/plain","data":"notas en texto"}},
		{"type":"text","text":"resume"}]}]}`)
	if r.StatusCode != 200 || !strings.Contains(cuerpo, `nombre=\"Contrato.pdf\"`) || !strings.Contains(cuerpo, "cláusulas") ||
		!strings.Contains(cuerpo, `nombre=\"documento-2.txt\"`) || !strings.Contains(cuerpo, "notas en texto") {
		t.Fatalf("documentos de Anthropic: %d %s", r.StatusCode, cuerpo)
	}
}

func TestArchivoSinNombre(t *testing.T) {
	ts := conConversor(t)
	_, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":[
		{"type":"file","file":{"file_data":"data:application/vnd.openxmlformats-officedocument.wordprocessingml.document;base64,`+b64("palabras")+`"}}]}]}`)
	if !strings.Contains(cuerpo, `nombre=\"documento-1.docx\"`) {
		t.Fatalf("sin nombre, documento-1 con la extensión del tipo: %s", cuerpo)
	}
}

func TestArchivoMismoEnDosTurnos(t *testing.T) {
	ts := conConversor(t)
	parte := `{"type":"file","file":{"filename":"informe.pdf","file_data":"data:application/pdf;base64,` + b64("el mismo") + `"}}`
	_, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[
		{"role":"user","content":[`+parte+`,{"type":"text","text":"uno"}]},
		{"role":"assistant","content":"vale"},
		{"role":"user","content":[`+parte+`,{"type":"text","text":"dos"}]}]}`)
	if n := strings.Count(cuerpo, "sha256="); n != 1 {
		t.Fatalf("el mismo archivo en dos turnos llega una vez: %d en %s", n, cuerpo)
	}
}

func TestNombreConRuta(t *testing.T) {
	ts := conConversor(t)
	_, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":[
		{"type":"file","file":{"filename":"../../x.pdf","file_data":"data:application/pdf;base64,`+b64("x")+`"}}]}]}`)
	if !strings.Contains(cuerpo, `nombre=\"x.pdf\"`) || strings.Contains(cuerpo, "../") {
		t.Fatalf("el nombre pierde la ruta: %s", cuerpo)
	}
}

func TestArchivoRechazos(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	pdf := "data:application/pdf;base64," + b64("x")
	muchos := make([]string, 21)
	for i := range muchos {
		muchos[i] = `{"type":"file","file":{"filename":"a` + fmt.Sprint(i) + `.txt","file_data":"data:text/plain;base64,` + b64(fmt.Sprint("texto ", i)) + `"}}`
	}
	casos := []struct {
		nombre, ruta, parte, codigo, texto string
	}{
		{"base64 roto", "/openai/v1/chat/completions", `{"type":"file","file":{"filename":"a.pdf","file_data":"data:application/pdf;base64,%%%"}}`, "solicitud_invalida", "mensaje 1, parte 2"},
		{"sin ;base64,", "/openai/v1/chat/completions", `{"type":"file","file":{"filename":"a.pdf","file_data":"data:application/pdf,hola"}}`, "solicitud_invalida", "mensaje 1, parte 2"},
		{"file_id", "/openai/v1/chat/completions", `{"type":"file","file":{"file_id":"file-123"}}`, "parametro_no_soportado", "file_data"},
		{"imagen chat", "/openai/v1/chat/completions", `{"type":"image_url","image_url":{"url":"` + pdf + `"}}`, "parametro_no_soportado", "imágenes"},
		{"file_url", "/openai/v1/responses", `{"type":"input_file","file_url":"https://example.com/a.pdf"}`, "parametro_no_soportado", "URL"},
		{"imagen responses", "/openai/v1/responses", `{"type":"input_image","image_url":"` + pdf + `"}`, "parametro_no_soportado", "imágenes"},
		{"url anthropic", "/anthropic/v1/messages", `{"type":"document","source":{"type":"url","url":"https://example.com/a.pdf"}}`, "parametro_no_soportado", "URL"},
		{"imagen anthropic", "/anthropic/v1/messages", `{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + b64("x") + `"}}`, "parametro_no_soportado", "imágenes"},
		{"tipo raro", "/openai/v1/chat/completions", `{"type":"file","file":{"file_data":"data:application/x-foo;base64,` + b64("x") + `"}}`, "carga_invalida", "application/x-foo"},
		{"21 archivos", "/openai/v1/chat/completions", strings.Join(muchos, ","), "solicitud_invalida", "20"},
	}
	for _, c := range casos {
		cuerpo := `{"model":"simulado","max_tokens":10,"messages":[{"role":"user","content":[{"type":"text","text":"hola"},` + c.parte + `]}]}`
		if strings.Contains(c.ruta, "responses") {
			cuerpo = `{"model":"simulado","input":[{"role":"user","content":[{"type":"input_text","text":"hola"},` + c.parte + `]}]}`
		}
		r, respuesta := pedir(t, ts, "POST", c.ruta, cuerpo)
		codigoVisible := strings.Contains(respuesta, c.codigo) || strings.Contains(c.ruta, "anthropic")
		if r.StatusCode != 400 || !codigoVisible || !strings.Contains(respuesta, c.texto) {
			t.Errorf("%s: %d %s", c.nombre, r.StatusCode, respuesta)
		}
	}
}

func TestArchivoSinConversorFachada(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	parte := `{"type":"file","file":{"filename":"a.pdf","file_data":"data:application/pdf;base64,` + b64("x") + `"}}`
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":[`+parte+`]}]}`)
	if r.StatusCode != 503 || !strings.Contains(cuerpo, `"code":"conversor_no_disponible"`) {
		t.Fatalf("OpenAI sin conversor: %d %s", r.StatusCode, cuerpo)
	}
	doc := `{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"` + b64("x") + `"}}`
	r, cuerpo = pedir(t, ts, "POST", "/anthropic/v1/messages", `{"model":"simulado","max_tokens":10,"messages":[{"role":"user","content":[`+doc+`]}]}`)
	if r.StatusCode != 503 || !strings.Contains(cuerpo, `"type":"error"`) || !strings.Contains(cuerpo, "conversor") {
		t.Fatalf("Anthropic sin conversor: %d %s", r.StatusCode, cuerpo)
	}
}

func TestCuerpoGrandeFachada(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	cuerpo := `{"model":"simulado","messages":[{"role":"user","content":"` + strings.Repeat("a", 17<<20) + `"}]}`
	r, respuesta := pedir(t, ts, "POST", "/openai/v1/chat/completions", cuerpo)
	if r.StatusCode != 400 || !strings.Contains(respuesta, "12 MB") || !strings.Contains(respuesta, "/api/v1/cargas") {
		t.Fatalf("un cuerpo de más de 16 MB explica el tope y la alternativa: %d %.300s", r.StatusCode, respuesta)
	}
}
