// Package prueba tiene lo que comparten las pruebas de la terminal y de sus páginas.
package prueba

import (
	"context"
	"maps"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"tramtai/internal/actualizacion"
	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
	"tramtai/internal/version"
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
	return ArrancarCon(t, adaptadores.Simulado{})
}

// ArrancarCon es Arrancar con el agente "simulado" que se le da: por ejemplo uno que espera a que la prueba lo suelte, para
// tener un trabajo en marcha sin depender del reloj.
func ArrancarCon(t *testing.T, agente adaptadores.Adaptador) (*cliente.Cliente, string, func()) {
	t.Helper()
	return arrancar(t, agente, nil)
}

// ConAgentes es Servicio con más agentes junto al simulado, por su nombre: así una página ve los estados que calcula
// el servicio (sin CLI, sin clave, con suscripción…) sin fabricarlos.
func ConAgentes(t *testing.T, agentes map[string]adaptadores.Adaptador) (*cliente.Cliente, string) {
	t.Helper()
	return Ajustado(t, func(o *servicio.Opciones) { maps.Copy(o.Registro, agentes) })
}

// Ajustado es Servicio con sus opciones cambiadas: más agentes, el arranque automático del sistema…
func Ajustado(t *testing.T, ajustar func(*servicio.Opciones)) (*cliente.Cliente, string) {
	t.Helper()
	cli, id, _ := arrancar(t, adaptadores.Simulado{}, ajustar)
	return cli, id
}

// Detectado es un agente simulado que se detecta como se le diga.
type Detectado struct {
	adaptadores.Simulado
	Estado adaptadores.Estado
}

func (d Detectado) Detectar(context.Context) adaptadores.Estado { return d.Estado }

// SinConversor es Servicio con un conversor de archivos que no está listo (uv no existe): para el aviso al adjuntar un PDF.
func SinConversor(t *testing.T) (*cliente.Cliente, string) {
	t.Helper()
	dir := t.TempDir()
	cli, id, _ := arrancar(t, adaptadores.Simulado{}, func(o *servicio.Opciones) {
		o.RaizConversor, o.UV = dir, filepath.Join(dir, "no-existe-uv")
	})
	return cli, id
}

// ConVersion es Servicio con la búsqueda de actualizaciones activada y v como la última versión publicada (y como sus
// novedades), siendo la actual la v0.56.0: nada va a GitHub y esta copia no se instaló con el instalador, así que
// actualizar nunca descarga.
func ConVersion(t *testing.T, v actualizacion.Version) (*cliente.Cliente, string) {
	t.Helper()
	antes := version.Version
	version.Version = "v0.56.0" // una compilación «dev» nunca se da por desactualizada
	t.Cleanup(func() { version.Version = antes })
	return Ajustado(t, func(o *servicio.Opciones) {
		o.BuscarActualizaciones = true
		o.ConsultarVersion = func(context.Context) (actualizacion.Version, error) { return v, nil }
		o.Novedades = func(context.Context, string, string) ([]actualizacion.Version, error) {
			return []actualizacion.Version{v}, nil
		}
	})
}

func arrancar(t *testing.T, agente adaptadores.Adaptador, ajustar func(*servicio.Opciones)) (*cliente.Cliente, string, func()) {
	t.Helper()
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	datos := filepath.Join(dir, "datos")
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	var auto atomic.Bool // como un sistema que lo admite, apagado; cambiarlo no toca el arranque del equipo
	opc := servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": agente},
		Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"},
		Autoarranque:        func() (bool, bool) { return true, auto.Load() },
		CambiarAutoarranque: func(activo bool) error { auto.Store(activo); return nil }}
	if ajustar != nil {
		ajustar(&opc)
	}
	s, err := servicio.Nuevo(opc)
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	contarConexiones(ts)
	ts.Start()
	var una sync.Once
	cerrar := func() {
		una.Do(func() {
			s.Cerrar()
			ts.CloseClientConnections() // como un servicio que se cae: los flujos abiertos se cortan y Close no los espera
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
