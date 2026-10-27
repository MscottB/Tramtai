package servicio

import (
	"encoding/json"
	"maps"
	"sync/atomic"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/autoarranque"
	"tramtai/internal/contrato"
)

// La sugerencia de dar a las apps su propia API la decide el servicio: con más de 20 peticiones en 24 h.
func TestSugerirApiPropia(t *testing.T) {
	for n, quiere := range map[int]bool{0: false, 20: false, 21: true} {
		if sugerirApiPropia(n) != quiere {
			t.Errorf("%d peticiones: %v", n, !quiere)
		}
	}
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	clave := claveDe(t, s, global)
	_, cuerpo := pedir(t, ts.URL, "GET", "/api/v1/instancia", clave, "")
	var r contrato.ResumenInstancia
	if err := json.Unmarshal([]byte(cuerpo), &r); err != nil || r.ID != "global" || r.SugerirApiPropia || r.Base == "" {
		t.Fatalf("%v %s", err, cuerpo)
	}
}

// Los contadores de Inicio agrupan los trabajos con los filtros del Historial: los calcula el servicio.
func TestContadoresDelResumen(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	clave := claveDe(t, s, global)
	if r, c := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", clave, hola); r.StatusCode != 200 {
		t.Fatalf("petición: %d %s", r.StatusCode, c)
	}
	_, cuerpo := pedir(t, ts.URL, "GET", "/api/v1/instancia", clave, "")
	var r contrato.ResumenInstancia
	if err := json.Unmarshal([]byte(cuerpo), &r); err != nil {
		t.Fatal(err)
	}
	if quiere := map[string]int{"en_curso": 0, "completado": 1, "con_error": 0}; !maps.Equal(r.Contadores, quiere) {
		t.Fatalf("contadores %v, quería %v", r.Contadores, quiere)
	}
}

// pasos pide los Primeros pasos de la instancia global con el token de servicio.
func pasos(t *testing.T, s *Servicio, base, instancia string) map[string]contrato.Paso {
	t.Helper()
	r, cuerpo := pedir(t, base, "GET", "/api/v1/instancia/pasos", s.tokenAdmin, "", "Tramtai-Instancia", instancia)
	var p contrato.PrimerosPasos
	if err := json.Unmarshal([]byte(cuerpo), &p); err != nil || r.StatusCode != 200 {
		t.Fatalf("%d %v %s", r.StatusCode, err, cuerpo)
	}
	if len(p.Adaptadores) != 1 || p.Adaptadores[0].Estado != contrato.AgenteListo {
		t.Fatalf("lleva los agentes comprobados: %s", cuerpo)
	}
	porPagina := map[string]contrato.Paso{}
	for _, x := range p.Pasos {
		porPagina[x.Pagina] = x
	}
	return porPagina
}

// Los Primeros pasos los calcula el servicio: la Web UI y la terminal solo los pintan.
func TestPrimerosPasos(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	var disponible atomic.Bool // el servidor lo lee en otra goroutine
	disponible.Store(true)
	s.opc.Autoarranque = func() (bool, autoarranque.Modo) { return disponible.Load(), "" }
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	clave := claveDe(t, s, global)
	p := pasos(t, s, ts.URL, global.ID)
	if a := p["agentes"]; !a.Hecho || a.Listos != 1 || a.Total != 1 {
		t.Fatalf("agentes: %+v", a)
	}
	if p["ejecutar"].Hecho || p["conectar"].Hecho {
		t.Fatalf("sin peticiones ni APIs: %+v", p)
	}
	if a, ok := p["configuracion"]; !ok || a.Hecho {
		t.Fatalf("el arranque automático, disponible y apagado: %+v", p)
	}
	if r, c := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", clave, hola); r.StatusCode != 200 {
		t.Fatalf("petición: %d %s", r.StatusCode, c)
	}
	disponible.Store(false)
	p = pasos(t, s, ts.URL, global.ID)
	if !p["ejecutar"].Hecho {
		t.Fatalf("con una petición completada: %+v", p)
	}
	if _, ok := p["configuracion"]; ok {
		t.Fatalf("sin arranque automático en el sistema no hay paso: %+v", p)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/instancia/pasos", clave, ""); r.StatusCode != 401 {
		t.Fatalf("con la clave de las apps: %d", r.StatusCode)
	}
}
