package api

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// GET /api/v1/trabajos filtra por estado (los filtros del Historial) y busca en el texto en el servicio, con todo el
// historial y no solo la primera página.
func TestListarTrabajosPorEstadoYTexto(t *testing.T) {
	ts := servidor(t, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}, "roto": adaptadores.Simulado{Error: errors.New("falla")}}, nil)
	bien := crearHola(t, ts, `{"mensajes":[{"rol":"usuario","contenido":"Resume la factura"}]}`)
	mal := crearHola(t, ts, `{"agente":"roto","mensajes":[{"rol":"usuario","contenido":"Otra factura"}]}`)
	esperarEstado(t, ts, bien, nucleo.Completado)
	esperarEstado(t, ts, mal, nucleo.Fallido)
	ids := func(consulta string) []string {
		r, cuerpo := pedir(t, ts, "GET", "/api/v1/trabajos?"+consulta, "")
		if r.StatusCode != 200 {
			t.Fatalf("%s: %d %s", consulta, r.StatusCode, cuerpo)
		}
		var salida []string
		for _, x := range decodificar(t, cuerpo)["trabajos"].([]any) {
			salida = append(salida, x.(map[string]any)["id"].(string))
		}
		return salida
	}
	if got := ids("estado=con_error"); len(got) != 1 || got[0] != mal {
		t.Fatalf("con error: %v", got)
	}
	if got := ids("estado=completado&texto=" + url.QueryEscape("FACTURA")); len(got) != 1 || got[0] != bien {
		t.Fatalf("completadas con «factura»: %v", got)
	}
	if got := ids("texto=resume"); len(got) != 1 || got[0] != bien {
		t.Fatalf("por texto: %v", got)
	}
	if got := ids("estado=en_curso"); len(got) != 0 {
		t.Fatalf("en curso: %v", got)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/trabajos?estado=terminados", ""); r.StatusCode != 400 {
		t.Fatalf("un filtro desconocido: %d", r.StatusCode)
	}
	// El tope de texto son 200 caracteres, no bytes: con tildes también se admiten 200 (hallazgo 42).
	if r, cuerpo := pedir(t, ts, "GET", "/api/v1/trabajos?texto="+url.QueryEscape(strings.Repeat("á", 200)), ""); r.StatusCode != 200 {
		t.Fatalf("200 caracteres con tilde: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts, "GET", "/api/v1/trabajos?texto="+url.QueryEscape(strings.Repeat("a", 201)), ""); r.StatusCode != 400 {
		t.Fatalf("201 caracteres: %d", r.StatusCode)
	}
}
