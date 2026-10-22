package main

import (
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
)

func TestAplicarPresupuesto(t *testing.T) {
	var p nucleo.Presupuesto
	if err := aplicarPresupuesto(&p, []string{"diario.peticiones=500", "mensual.usd=12.5", "mensual.tokens=9000"}); err != nil {
		t.Fatal(err)
	}
	if p != (nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 500}, Mensual: nucleo.Limites{USD: 12.5, Tokens: 9000}}) {
		t.Fatalf("%+v", p)
	}
	if err := aplicarPresupuesto(&p, []string{"mensual.tokens=0"}); err != nil || p.Mensual.Tokens != 0 || p.Mensual.USD != 12.5 {
		t.Fatalf("0 quita solo ese límite: %v %+v", err, p)
	}
	for _, malo := range []string{"semanal.tokens=1", "diario.horas=1", "diario.tokens=-1", "diario.tokens", "diario.peticiones=1.5", "mensual.usd=mucho", "mensual.usd=NaN", "diario.usd=Inf"} {
		if err := aplicarPresupuesto(&p, []string{malo}); err == nil {
			t.Errorf("%q debía rechazarse", malo)
		}
	}
}

// servicioDePrueba deja tramtai acceso hablando con un servicio en memoria con su configuración global en un
// directorio temporal; devuelve la ruta de esa configuración.
func servicioDePrueba(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	datos := filepath.Join(dir, "datos")
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	t.Cleanup(ts.Close)
	t.Cleanup(s.Cerrar)
	if err := s.EscribirDescriptor(); err != nil {
		t.Fatal(err)
	}
	anterior := servicioAcceso
	servicioAcceso = func(*cobra.Command) (*cliente.Cliente, error) { return cliente.Conectar(datos) }
	t.Cleanup(func() { servicioAcceso = anterior })
	return global
}

// acceder ejecuta tramtai acceso con esos argumentos y devuelve su salida.
func acceder(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var salida strings.Builder
	cmd := comandoAcceso()
	cmd.SetOut(&salida)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return salida.String(), err
}

// La CLI aplica las mismas reglas que la Web UI: no guarda una API con Antigravity (aún no habilitado para apps), que
// rechazaría todas sus peticiones.
func TestAccesoCrearConAgenteNoPermitido(t *testing.T) {
	ruta := servicioDePrueba(t)
	if _, err := acceder(t, "crear", "con-agy", "--agente", "antigravity"); err == nil || !strings.Contains(err.Error(), "Antigravity") {
		t.Fatalf("se creó una API con Antigravity: %v", err)
	}
	if datos, err := os.ReadFile(ruta); err == nil && strings.Contains(string(datos), "con-agy") {
		t.Fatalf("quedó guardada:\n%s", datos)
	}
}

// TestAccesoPorElServicio recorre los subcomandos contra el servicio: crear muestra la clave una vez, editar cambia
// solo lo indicado, --grupo activa y desactiva juntos, y una API borrada y vuelta a crear es nueva.
func TestAccesoPorElServicio(t *testing.T) {
	ruta := servicioDePrueba(t)
	salida, err := acceder(t, "crear", "facturas", "--agente", "simulado", "--grupo", "ventas")
	if err != nil || strings.Count(salida, "facturas") < 2 { // en la lista y con su clave nueva
		t.Fatalf("crear: %v\n%s", err, salida)
	}
	if _, err := acceder(t, "crear", "facturas"); err == nil || !strings.Contains(err.Error(), "ya existe") {
		t.Fatalf("crear otra vez: %v", err)
	}
	if _, err := acceder(t, "editar", "facturas", "--alias", "gpt-4o-mini=simulado", "--tiempo", "90"); err != nil {
		t.Fatal(err)
	}
	guardado, _ := os.ReadFile(ruta)
	for _, esperado := range []string{"grupo: ventas", "gpt-4o-mini: simulado", "tiempo_limite_segundos: 90", "agente: simulado"} {
		if !strings.Contains(string(guardado), esperado) {
			t.Fatalf("editar conserva lo anterior y añade lo nuevo (%q):\n%s", esperado, guardado)
		}
	}
	if _, err := acceder(t, "editar", "facturas", "--esfuerzo-que-no-existe"); err == nil {
		t.Fatal("una bandera desconocida se rechaza")
	}
	if _, err := acceder(t, "desactivar", "--grupo", "ventas"); err != nil {
		t.Fatal(err)
	}
	if guardado, _ := os.ReadFile(ruta); !strings.Contains(string(guardado), "activo: false") {
		t.Fatalf("desactivar por grupo:\n%s", guardado)
	}
	if _, err := acceder(t, "activar", "--grupo", "nadie"); err == nil {
		t.Fatal("un grupo sin puntos es un error")
	}
	if _, err := acceder(t, "token", "facturas"); err == nil || !strings.Contains(err.Error(), "--rotar") {
		t.Fatalf("sin --rotar no hay clave que mostrar: %v", err)
	}
	if salida, err := acceder(t, "token", "facturas", "--rotar"); err != nil || !strings.Contains(salida, "facturas") {
		t.Fatalf("rotar: %v\n%s", err, salida)
	}
	if _, err := acceder(t, "eliminar", "facturas"); err != nil {
		t.Fatal(err)
	}
	if _, err := acceder(t, "eliminar", "facturas"); err == nil || !strings.Contains(err.Error(), "no existe") {
		t.Fatalf("eliminar otra vez: %v", err)
	}
	if guardado, _ := os.ReadFile(ruta); strings.Contains(string(guardado), "facturas") {
		t.Fatalf("eliminar la quita del archivo:\n%s", guardado)
	}
	if salida, err := acceder(t, "listar"); err != nil || strings.Contains(salida, "facturas") {
		t.Fatalf("listar: %v\n%s", err, salida)
	}
}

// Con --local, tramtai acceso trabaja sobre .tramtai/local.yaml del proyecto (activa su instancia) y no toca la global.
func TestAccesoLocal(t *testing.T) {
	global := servicioDePrueba(t)
	proyecto := t.TempDir()
	os.MkdirAll(filepath.Join(proyecto, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(proyecto, ".tramtai", "config.yaml"), []byte("version: 1\n"), 0o600)
	f, _ := os.OpenFile(global, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString("proyectos_confiables:\n  - '" + proyecto + "'\n") // un local.yaml con puntos exige un proyecto confiable
	f.Close()
	t.Chdir(proyecto)
	if _, err := acceder(t, "--local", "crear", "local1", "--agente", "simulado"); err != nil {
		t.Fatal(err)
	}
	local, _ := os.ReadFile(filepath.Join(proyecto, ".tramtai", "local.yaml"))
	if g, _ := os.ReadFile(global); !strings.Contains(string(local), "local1") || strings.Contains(string(g), "local1") {
		t.Fatalf("va al local.yaml del proyecto:\nlocal:\n%s\nglobal:\n%s", local, g)
	}
	if _, err := acceder(t, "eliminar", "local1"); err == nil {
		t.Fatal("sin --local no se ve un punto del proyecto")
	}
	if _, err := acceder(t, "--local", "editar", "local1", "--grupo", "equipo"); err != nil {
		t.Fatal(err)
	}
	if _, err := acceder(t, "--local", "eliminar", "local1"); err != nil {
		t.Fatal(err)
	}
	if local, _ := os.ReadFile(filepath.Join(proyecto, ".tramtai", "local.yaml")); strings.Contains(string(local), "local1") {
		t.Fatalf("eliminar --local:\n%s", local)
	}
}

// tramtai acceso trabaja sobre el archivo, como antes de pasar por el servicio: llega a una API cuya instancia no está
// activa y a una que no se pudo abrir (se puede desactivar o eliminar, pero no guardar otra vez con su error).
func TestAccesoFueraDeLasInstancias(t *testing.T) {
	ruta := servicioDePrueba(t)
	if _, err := acceder(t, "crear", "de-otro", "--agente", "simulado", "--instancia", "proyecto_inactivo"); err != nil {
		t.Fatal(err)
	}
	if _, err := acceder(t, "editar", "de-otro", "--grupo", "g"); err != nil {
		t.Fatalf("editar una API de una instancia inactiva: %v", err)
	}
	if _, err := acceder(t, "eliminar", "de-otro"); err != nil {
		t.Fatalf("eliminar una API de una instancia inactiva: %v", err)
	}

	os.WriteFile(ruta, []byte("predeterminados:\n  agente: simulado\npuntos_acceso:\n  - nombre: rota\n    agente: simulado\n    respaldo: [agente_retirado]\n"), 0o600)
	if _, err := acceder(t, "editar", "rota", "--grupo", "g"); err == nil || !strings.Contains(err.Error(), "agente_retirado") {
		t.Fatalf("guardarla con su error se rechaza: %v", err)
	}
	if _, err := acceder(t, "desactivar", "rota"); err != nil {
		t.Fatalf("desactivarla no pasa por las reglas: %v", err)
	}
	if _, err := acceder(t, "eliminar", "rota"); err != nil {
		t.Fatalf("eliminarla: %v", err)
	}
	if guardado, _ := os.ReadFile(ruta); strings.Contains(string(guardado), "rota") || strings.Contains(string(guardado), "de-otro") {
		t.Fatalf("quedaron en el archivo:\n%s", guardado)
	}
}
