package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/documentos"
)

// servidorConversor es un servidor con una preparación cuyo uv no existe: preparar termina enseguida en error.
func servidorConversor(t *testing.T, punto *Punto) (*httptest.Server, *Servidor) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	s := nuevoServidor(t, simulado(adaptadores.Simulado{}), nil, punto)
	s.Conversor = documentos.NuevaPreparacion(t.TempDir(), "markitdown", filepath.Join(t.TempDir(), "no-existe-uv"))
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func TestConversorEstado(t *testing.T) {
	ts, _ := servidorConversor(t, nil)
	r, cuerpo := pedir(t, ts, "GET", "/api/v1/conversor", "")
	if r.StatusCode != 200 || decodificar(t, cuerpo)["estado"] != "falta" {
		t.Fatalf("estado: %d %s", r.StatusCode, cuerpo)
	}
}

func TestConversorPrepararSoloInstancia(t *testing.T) {
	ts, _ := servidorConversor(t, nil)
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/conversor/preparar", ""); r.StatusCode != 202 || !strings.Contains(cuerpo, `"estado"`) {
		t.Fatalf("la instancia prepara: %d %s", r.StatusCode, cuerpo)
	}
	punto, _ := servidorConversor(t, &Punto{Nombre: "app", Contratos: map[string]bool{"nativa": true}})
	if r, _ := pedir(t, punto, "POST", "/api/v1/conversor/preparar", ""); r.StatusCode != 404 {
		t.Fatalf("una API por aplicación no lanza la descarga: %d", r.StatusCode)
	}
	if r, _ := pedir(t, punto, "GET", "/api/v1/conversor", ""); r.StatusCode != 200 {
		t.Fatalf("pero sí ve el estado: %d", r.StatusCode)
	}
}

func TestConversorAjustes(t *testing.T) {
	ts, s := servidorConversor(t, nil)
	var fijado *bool
	s.FijarPrepararAlIniciar = func(b bool) error { fijado = &b; return nil }
	if r, cuerpo := pedir(t, ts, "PUT", "/api/v1/conversor/ajustes", `{"preparar_al_iniciar":false}`); r.StatusCode != 204 || fijado == nil || *fijado {
		t.Fatalf("ajustes: %d %s %v", r.StatusCode, cuerpo, fijado)
	}
	if r, _ := pedir(t, ts, "PUT", "/api/v1/conversor/ajustes", `{}`); r.StatusCode != 400 {
		t.Fatalf("sin el valor es una solicitud inválida: %d", r.StatusCode)
	}
}

func TestCapacidadesConversor(t *testing.T) {
	ts, _ := servidorConversor(t, nil)
	_, cuerpo := pedir(t, ts, "GET", "/api/v1/capacidades", "")
	if c, ok := decodificar(t, cuerpo)["conversor"].(map[string]any); !ok || c["estado"] != "falta" {
		t.Fatalf("capacidades sin el bloque conversor: %s", cuerpo)
	}
}
