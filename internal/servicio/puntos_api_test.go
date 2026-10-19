package servicio

import (
	"encoding/json"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
)

// TestReemplazarPuntoUI cubre lo que usa tramtai acceso editar: leer el punto guardado y reemplazarlo entero, con las
// mismas reglas que al crearlo y solo con la sesión de la Web UI o el token de servicio.
func TestReemplazarPuntoUI(t *testing.T) {
	global := `puntos_acceso:
  - nombre: facturas
    contratos: [nativa]
    agente: simulado
`
	s, ts, _ := entorno(t, global, "", adaptadores.Simulado{})
	instancia, _ := s.ActivarGlobal()
	RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "facturas"))
	s.Recargar()
	n := abrirSesion(t, s, ts.URL)
	csrf := []string{"X-CSRF-Token", n.csrf}

	leer := func() ConfigPunto {
		t.Helper()
		r, cuerpo := n.pedir("GET", "/api/v1/puntos/facturas/config", "")
		var p ConfigPunto
		if r.StatusCode != 200 || json.Unmarshal([]byte(cuerpo), &p) != nil {
			t.Fatalf("config: %d %s", r.StatusCode, cuerpo)
		}
		return p
	}
	p := leer()
	if p.Nombre != "facturas" || p.Agente != "simulado" || len(p.Contratos) != 1 {
		t.Fatalf("devuelve el punto tal como está guardado: %+v", p)
	}

	p.Grupo, p.AliasModelos = "ventas", map[string]string{"gpt-4o-mini": "simulado"}
	cuerpo, _ := json.Marshal(p)
	if r, texto := n.pedir("PUT", "/api/v1/puntos/facturas", string(cuerpo), csrf...); r.StatusCode != 200 || !strings.Contains(texto, `"grupo":"ventas"`) {
		t.Fatalf("reemplazar: %d %s", r.StatusCode, texto)
	}
	if q := leer(); q.Grupo != "ventas" || q.AliasModelos["gpt-4o-mini"] != "simulado" {
		t.Fatalf("el cambio se guarda: %+v", q)
	}

	if r, _ := n.pedir("PUT", "/api/v1/puntos/facturas", strings.Replace(string(cuerpo), `"nombre":"facturas"`, `"nombre":"otra"`, 1), csrf...); r.StatusCode != 400 {
		t.Fatalf("un nombre distinto del de la ruta se rechaza: %d", r.StatusCode)
	}
	malo := p
	malo.Respaldo = []string{"inexistente"}
	cuerpoMalo, _ := json.Marshal(malo)
	if r, _ := n.pedir("PUT", "/api/v1/puntos/facturas", string(cuerpoMalo), csrf...); r.StatusCode < 400 {
		t.Fatalf("un respaldo que no existe se rechaza como al crear: %d", r.StatusCode)
	}
	if q := leer(); len(q.Respaldo) != 0 {
		t.Fatalf("un cambio rechazado no se guarda: %+v", q)
	}
	if r, _ := pedir(t, ts.URL, "PUT", "/api/v1/puntos/facturas", instancia.token, string(cuerpo)); r.StatusCode != 401 {
		t.Fatalf("la clave de aplicaciones no edita puntos: %d", r.StatusCode)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/puntos/facturas/config", instancia.token, ""); r.StatusCode != 401 {
		t.Fatalf("la clave de aplicaciones no lee la configuración: %d", r.StatusCode)
	}
	if r, _ := n.pedir("GET", "/api/v1/puntos/nadie/config", ""); r.StatusCode != 404 {
		t.Fatalf("un punto que no existe: %d", r.StatusCode)
	}
}
