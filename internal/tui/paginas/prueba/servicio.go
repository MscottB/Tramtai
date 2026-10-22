// Package prueba tiene lo que comparten las pruebas de la terminal y de sus páginas.
package prueba

import (
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
)

// Servicio levanta el servicio real en memoria (httptest), con una configuración global temporal y el agente simulado,
// y devuelve un cliente conectado y el id del espacio global. Se cierra al acabar la prueba.
func Servicio(t *testing.T) (*cliente.Cliente, string) {
	t.Helper()
	cli, id, _ := Arrancar(t)
	return cli, id
}

// Arrancar es Servicio con la función que lo cierra antes de tiempo, para probar qué pasa cuando deja de responder.
func Arrancar(t *testing.T) (*cliente.Cliente, string, func()) {
	t.Helper()
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	datos := filepath.Join(dir, "datos")
	c, err := nucleo.CargarConfiguracion("")
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
	var una sync.Once
	cerrar := func() {
		una.Do(func() {
			s.Cerrar()
			ts.Close()
		})
	}
	t.Cleanup(cerrar)
	s.ActivarGlobal()
	s.EscribirDescriptor()
	cli, err := cliente.Conectar(datos)
	if err != nil {
		t.Fatal(err)
	}
	cli.Origen = nucleo.DesdeTerminal
	return cli, "global", cerrar
}
