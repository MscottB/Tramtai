package servicio

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
)

type respuestaRetencion struct {
	HorasResultado int `json:"horas_resultado"`
	HorasMetadatos int `json:"horas_metadatos"`
	MaxTrabajos    int `json:"max_trabajos"`
	Uso            struct {
		Trabajos int   `json:"trabajos"`
		Bytes    int64 `json:"bytes"`
	} `json:"uso"`
}

func TestRetencionConfigurable(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	pedir(t, ts.URL, "POST", "/api/v1/trabajos", global.token, hola)
	n := abrirSesion(t, s, ts.URL)
	leer := func() respuestaRetencion {
		r, cuerpo := n.pedir("GET", "/api/v1/instancia/retencion", "")
		var v respuestaRetencion
		if r.StatusCode != 200 || json.Unmarshal([]byte(cuerpo), &v) != nil {
			t.Fatalf("GET retención: %d %s", r.StatusCode, cuerpo)
		}
		return v
	}
	if v := leer(); v.HorasResultado != 24 || v.HorasMetadatos != 168 || v.MaxTrabajos != 10000 || v.Uso.Trabajos != 1 || v.Uso.Bytes <= 0 {
		t.Fatalf("valores por defecto y uso: %+v", v)
	}
	poner := func(cuerpo string) (int, string) {
		r, resp := n.pedir("PUT", "/api/v1/instancia/retencion", cuerpo, "X-CSRF-Token", n.csrf, "Content-Type", "application/json")
		return r.StatusCode, resp
	}
	if c, resp := poner(`{"horas_resultado":48,"horas_metadatos":240,"max_trabajos":500}`); c != 204 {
		t.Fatalf("PUT válido: %d %s", c, resp)
	}
	if v := leer(); v.HorasResultado != 48 || v.HorasMetadatos != 240 || v.MaxTrabajos != 500 {
		t.Fatalf("se aplica al momento: %+v", v)
	}
	if c, resp := poner(`{"max_trabajos":500}`); c != 400 || !strings.Contains(resp, "solicitud_invalida") {
		t.Errorf("un PUT sin los plazos no los pone a 0: %d %s", c, resp)
	}
	for _, malo := range []string{`{"horas_resultado":24,"horas_metadatos":12,"max_trabajos":500}`, `{"horas_resultado":24,"horas_metadatos":168,"max_trabajos":50}`} {
		if c, resp := poner(malo); c != 400 || !strings.Contains(resp, "solicitud_invalida") {
			t.Errorf("%s debía rechazarse: %d %s", malo, c, resp)
		}
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/instancia/retencion", global.token, ""); r.StatusCode != 401 && r.StatusCode != 403 {
		t.Errorf("con la clave de las aplicaciones no se ve: %d", r.StatusCode)
	}
}

func TestRetencionFueraDeRangoEnConfig(t *testing.T) {
	s, _, _ := entorno(t, "retencion:\n  horas_resultado: 5000\n", "", adaptadores.Simulado{})
	var registro bytes.Buffer
	s.opc.Registros = &registro
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if r, m, _, ajustada := global.retencion(); r != 720 || m != 720 || !ajustada {
		t.Fatalf("se ajusta al máximo: %d %d %t", r, m, ajustada)
	}
	s.Cerrar() // espera a la primera purga, que deja el aviso
	if !strings.Contains(registro.String(), "retencion") {
		t.Fatalf("queda un aviso en el registro: %q", registro.String())
	}
}
