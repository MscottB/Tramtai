package servicio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/configuracion"
	"tramtai/internal/planificador"
)

// servicioConversor arranca un servicio con la raíz del usuario en una carpeta nueva, sin markitdown en el PATH y con
// un uv que no existe: si intenta preparar, termina en error enseguida; si no, el conversor sigue en falta.
func servicioConversor(t *testing.T, preparar bool) (*Servicio, string) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	s, err := Nuevo(Opciones{Datos: filepath.Join(dir, "datos"), Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Hosts: []string{"127.0.0.1"},
		RaizConversor: dir, PrepararConversor: preparar, UV: filepath.Join(dir, "no-existe-uv")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Cerrar)
	return s, global
}

func TestArranqueNoPreparaSiDesactivado(t *testing.T) {
	s, _ := servicioConversor(t, false)
	if e := s.conversor.Estado(); e.Estado != "falta" || e.PrepararAlIniciar {
		t.Fatalf("con preparar_al_iniciar: false no se prepara: %+v", e)
	}
}

func TestArranquePreparaSiFalta(t *testing.T) {
	s, _ := servicioConversor(t, true)
	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelar()
	if e := s.conversor.Esperar(ctx); e.Estado != "error" || !e.PrepararAlIniciar {
		t.Fatalf("sin conversor, al iniciar se prepara (aquí falla: no hay uv): %+v", e)
	}
}

func TestFijarPrepararConversor(t *testing.T) {
	s, global := servicioConversor(t, false)
	if err := s.fijarPrepararConversor(true); err != nil {
		t.Fatal(err)
	}
	datos, _ := os.ReadFile(global)
	if !strings.Contains(string(datos), "preparar_al_iniciar: true") || !s.conversor.Estado().PrepararAlIniciar {
		t.Fatalf("se guarda en la configuración global y se ve en el estado:\n%s", datos)
	}
}
