package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// Un error al seguir el trabajo que no es del cliente (aquí, un trabajo que la base no tiene) se devuelve para
// responderlo; solo una conexión cerrada cuenta como desconexión.
func TestEjecutarFachadaDistingueDesconexion(t *testing.T) {
	s := nuevoServidor(t, simulado(adaptadores.Simulado{}), nil, nil)
	r := httptest.NewRequest("POST", "/openai/v1/chat/completions", nil)
	if _, err := s.ejecutarFachada(r, nucleo.Trabajo{ID: "trabajo_inexistente"}, nil, nil); err == nil || errors.Is(err, errDesconectado) {
		t.Fatalf("un error interno se tomó por desconexión: %v", err)
	}
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, err := s.ejecutarFachada(r.WithContext(ctx), nucleo.Trabajo{ID: "trabajo_inexistente"}, nil, nil); !errors.Is(err, errDesconectado) {
		t.Fatalf("una petición cancelada debe contar como desconexión: %v", err)
	}
}

// La Idempotency-Key tiene el mismo límite en las fachadas que en la API propia.
func TestIdempotencyKeyLargaEnFachadas(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	r, cuerpo := pedir(t, ts, "POST", "/openai/v1/chat/completions", `{"model":"simulado","messages":[{"role":"user","content":"x"}]}`,
		"Idempotency-Key", strings.Repeat("k", 256))
	if r.StatusCode != 400 {
		t.Fatalf("Idempotency-Key de 256: %d %s", r.StatusCode, cuerpo)
	}
}

// Un cuerpo que llega a cuentagotas se corta al vencer el plazo; un trabajo que tarda más que el plazo después de leer
// el cuerpo termina igual (el plazo no cancela la petición).
func TestPlazoDeLectura(t *testing.T) {
	anterior := plazoLectura
	plazoLectura = 200 * time.Millisecond
	t.Cleanup(func() { plazoLectura = anterior })
	ts := servidor(t, simulado(adaptadores.Simulado{Demora: 600 * time.Millisecond}), nil)

	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/ejecutar", `{"mensajes":[{"rol":"usuario","contenido":"hola"}]}`); r.StatusCode != 200 || !strings.Contains(cuerpo, `"completado"`) {
		t.Fatalf("un trabajo más largo que el plazo de lectura debe terminar: %d %s", r.StatusCode, cuerpo)
	}

	lento, escribir := io.Pipe()
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/ejecutar", lento)
	req.Header.Set("Authorization", "Bearer secreto")
	req.Header.Set("Content-Type", "application/json")
	go func() {
		for _, b := range []byte(`{"mensajes":[{"rol":"usuario","contenido":"hola"}]}`) {
			if _, err := escribir.Write([]byte{b}); err != nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		escribir.Close()
	}()
	inicio := time.Now()
	r, err := http.DefaultClient.Do(req)
	if err == nil {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
	escribir.CloseWithError(io.ErrClosedPipe)
	if err == nil && r.StatusCode < 400 {
		t.Fatalf("un cuerpo a cuentagotas se aceptó: %d", r.StatusCode)
	}
	if time.Since(inicio) > 2*time.Second {
		t.Fatalf("el cuerpo lento retuvo la conexión %s", time.Since(inicio))
	}
}
