package servicio

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
)

// TestPuntosEnArchivo cubre las rutas de tramtai acceso: trabajan sobre el archivo, con las reglas de la Web UI y solo
// con el token de servicio.
func TestPuntosEnArchivo(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	admin := func(metodo, ruta, cuerpo string) (int, string) {
		t.Helper()
		r, texto := pedir(t, ts.URL, metodo, ruta, s.tokenAdmin, cuerpo)
		return r.StatusCode, texto
	}

	if c, texto := admin("POST", "/api/v1/servicio/puntos", `{"nombre":"facturas","agente":"simulado","grupo":"ventas","instancia":"otra"}`); c != 200 || !strings.Contains(texto, `"token":"tt_`) {
		t.Fatalf("crear: %d %s", c, texto)
	}
	if c, _ := admin("POST", "/api/v1/servicio/puntos", `{"nombre":"facturas","agente":"simulado"}`); c != 400 {
		t.Fatalf("crear otra vez: %d", c)
	}
	_, texto := admin("GET", "/api/v1/servicio/puntos", "")
	var lista struct {
		Puntos []ConfigPunto `json:"puntos"`
	}
	if json.Unmarshal([]byte(texto), &lista) != nil || len(lista.Puntos) != 1 || lista.Puntos[0].Instancia != "otra" || lista.Puntos[0].Grupo != "ventas" {
		t.Fatalf("con el token de servicio se respeta la instancia del cuerpo (tramtai acceso crear --instancia): %s", texto)
	}

	p := lista.Puntos[0]
	p.AliasModelos = map[string]string{"gpt-4o-mini": "simulado"}
	cuerpo, _ := json.Marshal(p)
	if c, texto := admin("PUT", "/api/v1/servicio/puntos/facturas", string(cuerpo)); c != 200 {
		t.Fatalf("guardar una API de una instancia inactiva: %d %s", c, texto)
	}
	if c, _ := admin("PUT", "/api/v1/servicio/puntos/otra", string(cuerpo)); c != 400 {
		t.Fatalf("un nombre distinto del de la ruta: %d", c)
	}
	malo := p
	malo.Respaldo = []string{"inexistente"}
	cuerpoMalo, _ := json.Marshal(malo)
	if c, texto := admin("PUT", "/api/v1/servicio/puntos/facturas", string(cuerpoMalo)); c < 400 || !strings.Contains(texto, "inexistente") {
		t.Fatalf("un respaldo que no existe se rechaza como al crear: %d %s", c, texto)
	}
	if c, texto := admin("POST", "/api/v1/servicio/puntos/activo", `{"grupo":"ventas","activo":false}`); c != 200 {
		t.Fatalf("desactivar el grupo: %d %s", c, texto)
	}
	if c, _ := admin("POST", "/api/v1/servicio/puntos/activo", `{"grupo":"nadie","activo":false}`); c != 404 {
		t.Fatalf("un grupo sin puntos: %d", c)
	}
	if c, texto := admin("POST", "/api/v1/servicio/puntos/facturas/token", ""); c != 200 || !strings.Contains(texto, `"token":"tt_`) {
		t.Fatalf("rotar: %d %s", c, texto)
	}
	if c, _ := admin("DELETE", "/api/v1/servicio/puntos/facturas", ""); c != 200 {
		t.Fatalf("eliminar: %d", c)
	}
	if c, _ := admin("DELETE", "/api/v1/servicio/puntos/facturas", ""); c != 404 {
		t.Fatalf("eliminar otra vez: %d", c)
	}

	// El local.yaml de un proyecto (con su raíz absoluta), y nada fuera de un proyecto.
	ruta := "/api/v1/servicio/puntos?proyecto=" + strings.ReplaceAll(filepath.ToSlash(proyecto), " ", "%20")
	if c, texto := admin("GET", ruta, ""); c != 200 || !strings.Contains(texto, "local.yaml") {
		t.Fatalf("archivo del proyecto: %d %s", c, texto)
	}
	if c, _ := admin("GET", "/api/v1/servicio/puntos?proyecto="+filepath.ToSlash(t.TempDir()), ""); c != 400 {
		t.Fatalf("una carpeta que no es un proyecto: %d", c)
	}
	if c, _ := admin("GET", "/api/v1/servicio/puntos?proyecto=relativa", ""); c != 400 {
		t.Fatalf("una ruta relativa: %d", c)
	}

	// Solo con el token de servicio: ni la clave de una app ni una sesión de la Web UI.
	inst, _ := s.ActivarGlobal()
	clave := claveDe(t, s, inst)
	if r, _ := pedir(t, ts.URL, "GET", "/api/v1/servicio/puntos", clave, ""); r.StatusCode != 401 {
		t.Fatalf("la clave de aplicaciones: %d", r.StatusCode)
	}
	n := abrirSesion(t, s, ts.URL)
	if r, _ := n.pedir("POST", "/api/v1/servicio/puntos", `{"nombre":"x"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 401 {
		t.Fatalf("una sesión de la Web UI: %d", r.StatusCode)
	}
}

// Una sesión de la Web UI solo gestiona las APIs de su instancia: al crear, la instancia del cuerpo no cuenta, y no
// hay PUT para cambiarla.
func TestSesionNoMuevePuntosDeInstancia(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	n := abrirSesion(t, s, ts.URL)
	csrf := []string{"X-CSRF-Token", n.csrf}
	if r, cuerpo := n.pedir("POST", "/api/v1/puntos", `{"nombre":"mia","agente":"simulado","instancia":"otro_proyecto"}`, csrf...); r.StatusCode != 201 || !strings.Contains(cuerpo, `"instancia":"global"`) {
		t.Fatalf("la API queda en la instancia de la sesión: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := n.pedir("PUT", "/api/v1/puntos/mia", `{"nombre":"mia","instancia":"otro_proyecto"}`, csrf...); r.StatusCode != 404 {
		t.Fatalf("no hay PUT en las rutas de la instancia: %d", r.StatusCode)
	}
}

// Las reglas se aplican a lo nuevo o cambiado, no a desactivar: una API que ya usaba Antigravity se puede desactivar,
// pero no editar ni crear otra con él (fallaría siempre).
func TestReglasAlGuardarPorLaAPI(t *testing.T) {
	global := `puntos_acceso:
  - nombre: vieja
    agente: antigravity
`
	s, ts, _ := entorno(t, global, "", adaptadores.Simulado{})
	s.ActivarGlobal()
	NuevaClave(RutaTokenPunto(s.opc.Datos, "vieja"))
	s.Recargar()
	n := abrirSesion(t, s, ts.URL)
	csrf := []string{"X-CSRF-Token", n.csrf}
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/vieja", `{"activo":false}`, csrf...); r.StatusCode != 200 {
		t.Fatalf("desactivar una API existente no pasa por las reglas: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts.URL, "PUT", "/api/v1/servicio/puntos/vieja", s.tokenAdmin, `{"nombre":"vieja","agente":"antigravity","activo":true}`); r.StatusCode != 200 {
		t.Fatalf("activarla de nuevo, tampoco (solo cambia activo): %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts.URL, "PUT", "/api/v1/servicio/puntos/vieja", s.tokenAdmin, `{"nombre":"vieja","agente":"antigravity","grupo":"otro"}`); r.StatusCode < 400 || !strings.Contains(cuerpo, "Antigravity") {
		t.Fatalf("editarla sí: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("POST", "/api/v1/puntos", `{"nombre":"nueva","agente":"antigravity"}`, csrf...); r.StatusCode < 400 || !strings.Contains(cuerpo, "Antigravity") {
		t.Fatalf("y crear otra con él: %d %s", r.StatusCode, cuerpo)
	}
}
