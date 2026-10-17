//go:build agentes

// Conversión con el markitdown real: go test -tags agentes -v ./internal/documentos
package documentos

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMarkitdownReal(t *testing.T) {
	c := &Conversor{TiempoLimite: 2 * time.Minute}
	html := escribir(t, "página.html", []byte("<html><body><h1>Título</h1><table><tr><th>a</th><th>b</th></tr><tr><td>1</td><td>2</td></tr></table></body></html>"))
	inicio := time.Now()
	docs, err := c.Convertir(context.Background(), []Entrada{html})
	salida := docs.Texto
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(salida, "# Título") || !strings.Contains(salida, "| a | b |") {
		t.Fatalf("conversión inesperada: %q", salida)
	}
	t.Logf("convertido en %s:\n%s", time.Since(inicio).Round(time.Millisecond), salida)
}
