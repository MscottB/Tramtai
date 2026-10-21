package servicio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
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
	if strings.Contains(string(datos), "otro") || pry.Config.Origen("predeterminados.agente") == nucleo.OrigenLocal {
		t.Fatalf("la clave debía salir de local.yaml:\n%s", datos)
	}
	if c := heredar(pry.ID, "comandos_agentes.claude"); c != 400 {
		t.Fatalf("una clave que no se hereda desde la app: %d", c)
	}
	if c := heredar(global.ID, "predeterminados.agente"); c != 400 {
		t.Fatalf("la global no hereda de nadie: %d", c)
	}
}
