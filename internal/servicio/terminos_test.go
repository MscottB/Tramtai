package servicio

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/terminos"
)

func TestTerminosUI(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	n := nuevoNavegador(t, ts.URL)
	if r, _ := n.pedir("GET", "/ui/terminos", ""); r.StatusCode != 401 {
		t.Fatalf("sin sesión: %d", r.StatusCode)
	}
	if r, _ := n.pedir("POST", "/ui/terminos", ""); r.StatusCode != 401 {
		t.Fatalf("aceptar sin sesión: %d", r.StatusCode)
	}
	codigo, _ := s.CodigoSesion("global")
	n.pedir("GET", "/ui/entrar?codigo="+url.QueryEscape(codigo), "")
	_, cuerpo := n.pedir("GET", "/ui/sesion", "")
	n.csrf = cuerpo[strings.Index(cuerpo, `"csrf":"`)+8:]
	n.csrf = n.csrf[:strings.Index(n.csrf, `"`)]

	estado := func() contrato.EstadoTerminos {
		t.Helper()
		r, cuerpo := n.pedir("GET", "/ui/terminos", "")
		var e contrato.EstadoTerminos
		if r.StatusCode != 200 || json.Unmarshal([]byte(cuerpo), &e) != nil {
			t.Fatalf("GET /ui/terminos: %d %s", r.StatusCode, cuerpo)
		}
		return e
	}
	e := estado()
	if !e.Pendiente || e.Version != terminos.Version || len(e.Riesgos) != 5 {
		t.Fatalf("sin aceptar: %+v", e)
	}
	if r, cuerpo := n.pedir("POST", "/ui/terminos", ""); r.StatusCode != 403 || !strings.Contains(cuerpo, "csrf_invalido") {
		t.Fatalf("aceptar sin CSRF: %d %s", r.StatusCode, cuerpo)
	}
	if !estado().Pendiente {
		t.Fatal("sin CSRF no se acepta")
	}
	if r, cuerpo := n.pedir("POST", "/ui/terminos", "", "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("aceptar con CSRF: %d %s", r.StatusCode, cuerpo)
	}
	if estado().Pendiente {
		t.Error("tras aceptar ya no está pendiente")
	}
	if a, _ := terminos.Leer(s.opc.Datos); a.Como != terminos.ComoInterfaz || a.Version != terminos.Version {
		t.Errorf("queda escrito como «interfaz»: %+v", a)
	}
}
