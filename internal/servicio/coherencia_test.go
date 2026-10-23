package servicio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

func TestSesionNuevaVuelveAlUltimoEspacio(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	pry, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	n := abrirSesion(t, s, ts.URL)
	if r, c := n.pedir("POST", "/ui/sesion", `{"instancia":"`+pry.ID+`"}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 {
		t.Fatalf("cambiar: %d %s", r.StatusCode, c)
	}
	// Una sesión local nueva (tras reiniciar el servicio, por ejemplo) empieza en el espacio predeterminado.
	if got := s.instanciaPredeterminada(); got != pry.ID {
		t.Fatalf("una sesión nueva debía volver al último espacio: %q", got)
	}
}

func TestHeredarQuitaDeLocal(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "predeterminados:\n  agente: otro\n", adaptadores.Simulado{})
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	pry, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	heredar := func(instancia, clave string) int {
		r, _ := pedir(t, ts.URL, "POST", "/api/v1/instancia/heredar", s.tokenAdmin, `{"clave":"`+clave+`"}`, "Tramtai-Instancia", instancia)
		return r.StatusCode
	}
	if c := heredar(pry.ID, "predeterminados.agente"); c != 204 {
		t.Fatalf("heredar: %d", c)
	}
	datos, _ := os.ReadFile(filepath.Join(proyecto, ".tramtai", "local.yaml"))
	if strings.Contains(string(datos), "otro") || pry.Config.Origen("predeterminados.agente") == configuracion.OrigenLocal {
		t.Fatalf("la clave debía salir de local.yaml:\n%s", datos)
	}
	if c := heredar(pry.ID, "comandos_agentes.claude"); c != 400 {
		t.Fatalf("una clave que no se hereda desde la app: %d", c)
	}
	if c := heredar(global.ID, "predeterminados.agente"); c != 400 {
		t.Fatalf("la global no hereda de nadie: %d", c)
	}
}

func TestConexionUIPersiste(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	n := abrirSesion(t, s, ts.URL)
	if r, c := n.pedir("POST", "/ui/agentes/conexion", `{"agente":"simulado","conectado":false}`, "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("desconectar: %d %s", r.StatusCode, c)
	}
	datos, _ := os.ReadFile(os.Getenv("TRAMTAI_CONFIG_GLOBAL"))
	if !strings.Contains(string(datos), "desconectado: true") {
		t.Fatalf("debía guardarse en la configuración global:\n%s", datos)
	}
	if r, c := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", global.token, hola); r.StatusCode != 409 || !strings.Contains(c, "agente_desconectado") {
		t.Fatalf("una petición al agente desconectado: %d %s", r.StatusCode, c)
	}
	if r, _ := n.pedir("POST", "/ui/agentes/conexion", `{"agente":"simulado","conectado":true}`, "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("conectar: %d", r.StatusCode)
	}
	if r, c := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", global.token, hola); r.StatusCode != 200 {
		t.Fatalf("conectado de nuevo: %d %s", r.StatusCode, c)
	}
	if r, _ := n.pedir("POST", "/ui/agentes/conexion", `{"agente":"nadie","conectado":false}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 {
		t.Fatalf("un agente que no existe: %d", r.StatusCode)
	}
}

func TestAppConClave24h(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	global, err := s.ActivarGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if r, c := pedir(t, ts.URL, "POST", "/api/v1/ejecutar", global.token, hola); r.StatusCode != 200 {
		t.Fatalf("petición: %d %s", r.StatusCode, c)
	}
	if _, c := pedir(t, ts.URL, "GET", "/api/v1/instancia", global.token, ""); !strings.Contains(c, `"app_con_clave_24h":1`) {
		t.Fatalf("GET /api/v1/instancia: %s", c)
	}
}
