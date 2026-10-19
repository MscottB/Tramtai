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
	"tramtai/internal/cliente"
	"tramtai/internal/documentos"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
)

// servicioConConversor es servicioDePrueba con el conversor: la raíz del usuario en una carpeta nueva, sin markitdown
// en el PATH y con un uv que no existe (preparar termina en error enseguida).
func servicioConConversor(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	datos := filepath.Join(dir, "datos")
	c, err := nucleo.CargarConfiguracion("")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"},
		RaizUsuario: dir, UV: filepath.Join(dir, "no-existe-uv")})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	t.Cleanup(ts.Close)
	t.Cleanup(s.Cerrar)
	s.ActivarGlobal()
	if err := s.EscribirDescriptor(); err != nil {
		t.Fatal(err)
	}
	anterior := servicioConversor
	servicioConversor = func(*cobra.Command) (*cliente.Cliente, error) { return cliente.Conectar(datos) }
	t.Cleanup(func() { servicioConversor = anterior })
}

func conversor(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var salida strings.Builder
	cmd := comandoConversor()
	cmd.SetOut(&salida)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return salida.String(), err
}

func TestConversorEstadoCLI(t *testing.T) {
	servicioConConversor(t)
	salida, err := conversor(t, "estado")
	if err != nil || !strings.Contains(salida, "falta") || !strings.Contains(salida, "tramtai conversor preparar") {
		t.Fatalf("estado: %v\n%s", err, salida)
	}
}

func TestConversorPrepararCLIFalla(t *testing.T) {
	servicioConConversor(t)
	salida, err := conversor(t, "preparar")
	if err == nil || !strings.Contains(salida, "error") {
		t.Fatalf("una preparación fallida termina con error: %v\n%s", err, salida)
	}
}

func TestArchivosSinConversor(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	pdf, txt := filepath.Join(dir, "informe.pdf"), filepath.Join(dir, "notas.txt")
	os.WriteFile(pdf, []byte("%PDF"), 0o600)
	os.WriteFile(txt, []byte("hola"), 0o600)
	conv := &documentos.Conversor{Comando: "markitdown", Raiz: t.TempDir()}
	if err := archivosSinConversor(conv, []string{txt}); err != nil {
		t.Fatalf("el texto no necesita conversor: %v", err)
	}
	if err := archivosSinConversor(conv, []string{txt, pdf}); nucleo.CodigoDe(err) != nucleo.CodigoConversorNoDisponible {
		t.Fatalf("un PDF sin conversor falla antes de subirlo: %v", err)
	}
}
