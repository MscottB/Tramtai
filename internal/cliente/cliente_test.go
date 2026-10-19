package cliente

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/servicio"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
)

func TestClienteContraServicio(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	proyecto := filepath.Join(dir, "proyecto")
	os.MkdirAll(filepath.Join(proyecto, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(proyecto, ".tramtai", "config.yaml"), []byte("version: 1\n"), 0o600)
	datos := filepath.Join(dir, "datos")

	if _, err := Conectar(datos); err != ErrServicioInactivo {
		t.Fatalf("sin descriptor el servicio está inactivo: %v", err)
	}
	c, _ := nucleo.CargarConfiguracion("")
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	// /healthz va sin el token: si otro proceso ocupara el puerto con un descriptor viejo, no lo recibiría.
	var saludConToken atomic.Bool
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" && r.Header.Get("Authorization") != "" {
			saludConToken.Store(true)
		}
		s.ServeHTTP(w, r)
	})
	ts.Start()
	defer ts.Close()
	defer s.Cerrar()
	if err := s.EscribirDescriptor(); err != nil {
		t.Fatal(err)
	}

	cli, err := Conectar(datos)
	if saludConToken.Load() {
		t.Fatal("Conectar envió el token de servicio a /healthz antes de comprobar la identidad")
	}
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	activacion, err := cli.Activar(ctx, "proyecto", proyecto)
	if err != nil {
		t.Fatal(err)
	}
	id := activacion.Instancia.ID
	if d, _ := contrato.LeerDescriptor(datos); len(d.Instancias) != 1 || d.Instancias[0] != id {
		t.Fatalf("el descriptor lista las instancias activas: %+v", d)
	}
	ruta := filepath.Join(dir, "notas.txt")
	os.WriteFile(ruta, []byte("contenido"), 0o600)
	carga, err := cli.SubirCarga(ctx, id, ruta)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Cargas: []string{carga.ID}, Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "resume"}}})
	if err != nil {
		t.Fatal(err)
	}
	var texto strings.Builder
	if err := cli.Eventos(ctx, id, tr.ID, 0, func(_ int, tipo, datos string) error {
		if tipo == "texto" {
			texto.WriteString(datos)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(texto.String(), "contenido") || !strings.HasSuffix(texto.String(), "resume") {
		t.Fatalf("eventos: %q", texto.String())
	}
	if lista, err := cli.Trabajos(ctx, id, 5); err != nil || len(lista) != 1 {
		t.Fatalf("trabajos: %v %v", lista, err)
	}
	if cfg, err := cli.Configuracion(ctx, id); err != nil || len(cfg) == 0 {
		t.Fatalf("configuración: %v", err)
	}
	if err := cli.Desactivar(ctx, id); err != nil {
		t.Fatal(err)
	}
	if e, _ := cli.Estado(ctx); len(e.Instancias) != 0 {
		t.Fatalf("la instancia debía desactivarse: %+v", e.Instancias)
	}
	if err := cli.Detener(ctx); err != nil {
		t.Fatal(err)
	}
	<-s.Detenido()
}

func TestEncabezadoDelCliente(t *testing.T) {
	var recibido []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recibido = append(recibido, r.Header.Get("X-Tramtai-Cliente"))
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := &Cliente{Base: srv.URL, token: "x", http: srv.Client()}
	c.Estado(context.Background())
	c.Origen = "cli"
	c.Estado(context.Background())
	if len(recibido) != 2 || recibido[0] != "" || recibido[1] != "cli" {
		t.Fatalf("sin origen no hay encabezado; con origen, su valor: %q", recibido)
	}
}
