package cliente

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/procesos"
	"tramtai/internal/servicio"
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
	c, _ := configuracion.CargarConfiguracion("", agentes.Internos())
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Secretos: func() []string { return nil }, Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"}})
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
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	estado, token, err := cli.CrearPunto(ctx, "global", contrato.ConfigPunto{Nombre: "facturas", Agente: "simulado"})
	if err != nil || token == "" || estado.Nombre != "facturas" {
		t.Fatalf("crear punto: %+v %q %v", estado, token, err)
	}
	if nueva, err := cli.RotarTokenPunto(ctx, "global", "facturas"); err != nil || nueva == "" || nueva == token {
		t.Fatalf("rotar la clave: %q %v", nueva, err)
	}
	if err := cli.ActivarPunto(ctx, "global", "facturas", false); err != nil {
		t.Fatal(err)
	}
	if err := cli.EliminarPunto(ctx, "global", "facturas"); err != nil {
		t.Fatal(err)
	}
	if err := cli.EliminarPunto(ctx, "global", "facturas"); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("eliminar otra vez da no encontrado: %v", err)
	}

	// Los de tramtai acceso, sobre el archivo global.
	g, err := cli.CrearPuntoEnArchivo(ctx, "", contrato.ConfigPunto{Nombre: "ventas", Agente: "simulado", Grupo: "tienda"})
	if err != nil || g.Token == "" || g.Archivo != global {
		t.Fatalf("crear en el archivo: %+v %v", g, err)
	}
	archivo, puntos, err := cli.PuntosEnArchivo(ctx, "")
	if err != nil || archivo != global || len(puntos) != 1 || puntos[0].Grupo != "tienda" {
		t.Fatalf("puntos del archivo: %s %+v %v", archivo, puntos, err)
	}
	puntos[0].Instrucciones = "Sé breve."
	if _, err := cli.GuardarPuntoEnArchivo(ctx, "", puntos[0]); err != nil {
		t.Fatal(err)
	}
	if g, err := cli.ActivarPuntosEnArchivo(ctx, "", nil, "tienda", false); err != nil || len(g.Puntos) != 1 || g.Puntos[0].Activo {
		t.Fatalf("desactivar el grupo: %+v %v", g, err)
	}
	if nueva, err := cli.RotarTokenEnArchivo(ctx, "ventas"); err != nil || nueva == "" || nueva == g.Token {
		t.Fatalf("rotar en el archivo: %q %v", nueva, err)
	}
	if _, err := cli.EliminarPuntoEnArchivo(ctx, "", "ventas"); err != nil {
		t.Fatal(err)
	}
	if err := cli.Desactivar(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := cli.Desactivar(ctx, "global"); err != nil {
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

// servicioDePrueba levanta un servicio real con el agente simulado y devuelve un cliente conectado a él.
func servicioDePrueba(t *testing.T, global string) (*Cliente, string) {
	t.Helper()
	dir := t.TempDir()
	rutaGlobal := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(rutaGlobal, []byte("predeterminados:\n  agente: simulado\n"+global), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", rutaGlobal)
	datos := filepath.Join(dir, "datos")
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Secretos: func() []string { return nil }, Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"},
		RaizConversor: dir, UV: filepath.Join(dir, "no-existe-uv")})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	t.Cleanup(func() {
		ts.Close()
		s.Cerrar()
	})
	if err := s.EscribirDescriptor(); err != nil {
		t.Fatal(err)
	}
	cli, err := Conectar(datos)
	if err != nil {
		t.Fatal(err)
	}
	return cli, dir
}

var hola = nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}

// terminado espera a que el trabajo acabe.
func terminado(t *testing.T, cli *Cliente, inst, id string) {
	t.Helper()
	if err := cli.Eventos(context.Background(), inst, id, 0, func(int, string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestClientePuntosConUso(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	_, token, err := cli.CrearPunto(ctx, "global", contrato.ConfigPunto{Nombre: "facturas", Agente: "simulado"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.CambiarPunto(ctx, "global", "facturas", map[string]any{"presupuesto": map[string]any{"diario": map[string]any{"peticiones": 50}}}); err != nil {
		t.Fatal(err)
	}
	// Una petición por la API de la aplicación: es el uso que cuenta la lista.
	req, _ := http.NewRequest("POST", cli.Base+"/p/facturas/api/v1/trabajos", strings.NewReader(`{"mensajes":[{"rol":"usuario","contenido":"hola"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode >= 300 {
		t.Fatalf("petición de la app: %v %v", resp, err)
	}
	lista, err := cli.Puntos(ctx, "global")
	if err != nil {
		t.Fatal(err)
	}
	if len(lista.Puntos) != 1 || lista.Puntos[0].Nombre != "facturas" {
		t.Fatalf("puntos: %+v", lista.Puntos)
	}
	if uso := lista.Uso["facturas"]; uso.Peticiones != 1 || uso.Ultima == nil {
		t.Fatalf("uso: %+v", lista.Uso)
	}
	if p, ok := lista.Presupuestos["facturas"]; !ok || p.Diario.Limites.Peticiones != 50 {
		t.Fatalf("presupuestos: %+v", lista.Presupuestos)
	}
	if err := cli.ReiniciarPresupuesto(ctx, "global", "facturas"); err != nil {
		t.Fatal(err)
	}
}

func TestClienteCambiarPunto(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cli.CrearPunto(ctx, "global", contrato.ConfigPunto{Nombre: "facturas", Agente: "simulado"}); err != nil {
		t.Fatal(err)
	}
	p, err := cli.CambiarPunto(ctx, "global", "facturas", map[string]any{"instrucciones": "Sé breve.", "esfuerzo": "alto"})
	if err != nil || p.Nombre != "facturas" || p.Instrucciones != "Sé breve." || p.Esfuerzo != "alto" {
		t.Fatalf("cambiar: %+v %v", p, err)
	}
	// Un cambio parcial no toca lo demás.
	p, err = cli.CambiarPunto(ctx, "global", "facturas", map[string]any{"activo": false})
	if err != nil || p.Activo || p.Instrucciones != "Sé breve." || p.Esfuerzo != "alto" {
		t.Fatalf("parcial: %+v %v", p, err)
	}
	if _, err := cli.CambiarPunto(ctx, "global", "facturas", map[string]any{"esfuerzo": "enorme"}); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("un esfuerzo desconocido no vale: %v", err)
	}
	if _, err := cli.CambiarPunto(ctx, "global", "no-existe", map[string]any{"activo": true}); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("un punto que no existe: %v", err)
	}
}

// TestClienteAjustarPunto: los ajustes de una API se mandan enteros, así que lo vacío vuelve a lo del espacio y un
// presupuesto nil lo quita, sin tocar lo que no son ajustes.
func TestClienteAjustarPunto(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cli.CrearPunto(ctx, "global", contrato.ConfigPunto{Nombre: "facturas", Agente: "simulado", Grupo: "web"}); err != nil {
		t.Fatal(err)
	}
	p, err := cli.AjustarPunto(ctx, "global", "facturas", AjustesPunto{Instrucciones: "Sé breve.", Esfuerzo: "alto",
		ModelosPermitidos: []string{"simulado"}, Presupuesto: &nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 50}}})
	if err != nil || p.Instrucciones != "Sé breve." || p.Esfuerzo != "alto" || len(p.ModelosPermitidos) != 1 || p.Presupuesto == nil ||
		p.Presupuesto.Diario.Peticiones != 50 || p.Grupo != "web" {
		t.Fatalf("ajustar: %+v %v", p, err)
	}
	p, err = cli.AjustarPunto(ctx, "global", "facturas", AjustesPunto{})
	if err != nil || p.Instrucciones != "" || p.Esfuerzo != "" || len(p.ModelosPermitidos) != 0 || p.Presupuesto != nil {
		t.Fatalf("vacío quita los ajustes: %+v %v", p, err)
	}
}

// TestClienteClaveInstancia: la clave del espacio se lee con el token de servicio y es la que vale tras regenerarla.
func TestClienteClaveInstancia(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	antes, err := cli.ClaveInstancia(ctx, "global")
	if err != nil || !strings.HasPrefix(antes, "tt_") {
		t.Fatalf("clave: %q %v", antes, err)
	}
	nueva, err := cli.RotarClave(ctx, "global")
	if err != nil {
		t.Fatal(err)
	}
	if ahora, err := cli.ClaveInstancia(ctx, "global"); err != nil || ahora != nueva || ahora == antes {
		t.Fatalf("tras regenerar: %q (nueva %q, antes %q) %v", ahora, nueva, antes, err)
	}
}

func TestClienteTrabajosFiltrados(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err := cli.Crear(ctx, "global", hola); err != nil {
			t.Fatal(err)
		}
	}
	cli.Origen = "terminal"
	if _, err := cli.Crear(ctx, "global", hola); err != nil {
		t.Fatal(err)
	}
	cli.Origen = ""

	vistos := map[string]bool{}
	f := FiltroTrabajos{Limite: 4}
	var paginas []int
	for {
		pagina, err := cli.TrabajosFiltrados(ctx, "global", f)
		if err != nil {
			t.Fatal(err)
		}
		paginas = append(paginas, len(pagina))
		for _, tr := range pagina {
			if vistos[tr.ID] {
				t.Fatalf("%s sale en dos páginas", tr.ID)
			}
			vistos[tr.ID] = true
		}
		if len(pagina) < f.Limite {
			break
		}
		ultimo := pagina[len(pagina)-1]
		f.Antes, f.AntesID = ultimo.CreadoEn, ultimo.ID
	}
	if len(vistos) != 6 || !slices.Equal(paginas, []int{4, 2}) {
		t.Fatalf("dos páginas con cursor, sin repetidos: %v %d", paginas, len(vistos))
	}
	delTerminal, err := cli.TrabajosFiltrados(ctx, "global", FiltroTrabajos{Origen: "terminal"})
	if err != nil || len(delTerminal) != 1 {
		t.Fatalf("filtro por origen: %d %v", len(delTerminal), err)
	}
	if _, err := cli.TrabajosFiltrados(ctx, "global", FiltroTrabajos{Origen: "otro"}); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("un origen desconocido no vale: %v", err)
	}
	if lista, err := cli.Trabajos(ctx, "global", 3); err != nil || len(lista) != 3 {
		t.Fatalf("Trabajos sigue funcionando: %d %v", len(lista), err)
	}
}

func TestClienteRegistro(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	tr, err := cli.Crear(ctx, "global", hola)
	if err != nil {
		t.Fatal(err)
	}
	terminado(t, cli, "global", tr.ID)
	eventos, err := cli.Registro(ctx, "global", tr.ID)
	if err != nil || len(eventos) == 0 || eventos[0].Numero == 0 || eventos[0].Tipo == "" || eventos[0].CreadoEn.IsZero() {
		t.Fatalf("registro: %+v %v", eventos, err)
	}
	if _, err := cli.Registro(ctx, "global", "no-existe"); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("un trabajo que no existe: %v", err)
	}
}

// TestClienteCorregir: corregir es reintentar con {"corregir": true}; solo vale para un trabajo que no cumplía el esquema
// y conservó su respuesta, y el nuevo es otro trabajo.
func TestClienteCorregir(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	// El agente simulado devuelve el mensaje tal cual: {"total":"diez"} no cumple el esquema.
	malo, err := cli.Crear(ctx, "global", nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: `{"total":"diez"}`}},
		Salida: nucleo.ConfigSalida{Formato: "json", ConservarSalidaInvalida: true,
			Esquema: json.RawMessage(`{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	terminado(t, cli, "global", malo.ID)
	if m, err := cli.Trabajo(ctx, "global", malo.ID); err != nil || m.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || m.Salida == "" {
		t.Fatalf("no cumple el esquema y conserva la respuesta: %+v %v", m, err)
	}
	nuevo, err := cli.Corregir(ctx, "global", malo.ID)
	if err != nil || nuevo.ID == "" || nuevo.ID == malo.ID {
		t.Fatalf("corregir da otro trabajo: %+v %v", nuevo, err)
	}
	// Uno completado no se puede corregir: el servicio lo dice con su código.
	bueno, err := cli.Crear(ctx, "global", hola)
	if err != nil {
		t.Fatal(err)
	}
	terminado(t, cli, "global", bueno.ID)
	if _, err := cli.Corregir(ctx, "global", bueno.ID); nucleo.CodigoDe(err) != nucleo.CodigoCorreccionNoDisponible {
		t.Fatalf("un trabajo sin fallo de esquema no se corrige: %v", err)
	}
	if _, err := cli.Corregir(ctx, "global", "no-existe"); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("un trabajo que no existe: %v", err)
	}
}

func TestClienteVaciar(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		tr, err := cli.Crear(ctx, "global", hola)
		if err != nil {
			t.Fatal(err)
		}
		terminado(t, cli, "global", tr.ID)
	}
	if n, err := cli.VaciarTrabajos(ctx, "global"); err != nil || n != 2 {
		t.Fatalf("vaciar: %d %v", n, err)
	}
	if lista, err := cli.Trabajos(ctx, "global", 10); err != nil || len(lista) != 0 {
		t.Fatalf("debía quedar vacío: %d %v", len(lista), err)
	}
}

func TestClienteHeredar(t *testing.T) {
	cli, dir := servicioDePrueba(t, "")
	ctx := context.Background()
	proyecto := filepath.Join(dir, "mi-app")
	os.MkdirAll(filepath.Join(proyecto, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(proyecto, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Notas\n"), 0o600)
	a, err := cli.Activar(ctx, "proyecto", proyecto)
	if err != nil {
		t.Fatal(err)
	}
	id := a.Instancia.ID
	origen := func(clave string) string {
		t.Helper()
		cfg, err := cli.ConfiguracionInstancia(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range cfg {
			if e.Clave == clave {
				return e.Origen
			}
		}
		return "" // sin valor en ninguna capa, no sale
	}
	antes := origen("predeterminados.esfuerzo")
	if err := cli.FijarPredeterminado(ctx, id, "simulado", new("alto")); err != nil {
		t.Fatal(err)
	}
	if o := origen("predeterminados.esfuerzo"); o == antes || !strings.Contains(strings.ToLower(o), "local") {
		t.Fatalf("el esfuerzo propio del proyecto sale como local: %q (antes %q)", o, antes)
	}
	// Sin esfuerzo (nil), cambiar el modelo no toca el que ya tiene, como la Web UI.
	if err := cli.FijarPredeterminado(ctx, id, "simulado", nil); err != nil || !strings.Contains(strings.ToLower(origen("predeterminados.esfuerzo")), "local") {
		t.Fatalf("el esfuerzo se queda: %v", err)
	}
	if err := cli.FijarPredeterminado(ctx, id, "agente-que-no-existe", nil); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("un agente desconocido no vale: %v", err)
	}
	if err := cli.Heredar(ctx, id, "predeterminados.esfuerzo"); err != nil {
		t.Fatal(err)
	}
	if o := origen("predeterminados.esfuerzo"); o != antes {
		t.Fatalf("volver al de Global: %q, antes %q", o, antes)
	}
	if err := cli.Heredar(ctx, id, "proyecto.nombre"); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("solo se heredan los valores que lo admiten: %v", err)
	}

	// La retención se lee y se fija por espacio, y se devuelve a la de Global igual.
	r, err := cli.Retencion(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	r.HorasResultado, r.HorasMetadatos, r.MaxTrabajos = 12, 48, 100
	if err := cli.FijarRetencion(ctx, id, r); err != nil {
		t.Fatal(err)
	}
	if r2, err := cli.Retencion(ctx, id); err != nil || r2.HorasResultado != 12 || r2.HorasMetadatos != 48 || r2.MaxTrabajos != 100 {
		t.Fatalf("retención: %+v %v", r2, err)
	}
	if err := cli.Heredar(ctx, id, "retencion.horas_resultado"); err != nil {
		t.Fatal(err)
	}
}

func TestClientePresupuestoYLimites(t *testing.T) {
	cli, _ := servicioDePrueba(t, "presupuesto:\n  diario:\n    peticiones: 100\n")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	p, err := cli.PresupuestoInstancia(ctx, "global")
	if err != nil || p.Diario.Limites.Peticiones != 100 {
		t.Fatalf("presupuesto de la instancia: %+v %v", p, err)
	}
	if err := cli.ReiniciarPresupuestoInstancia(ctx, "global"); err != nil {
		t.Fatal(err)
	}
	l, err := cli.Limites(ctx, "global")
	if err != nil || l.CargasPorPeticion == 0 || l.Mensajes == 0 || len(l.ExtensionesTexto) == 0 {
		t.Fatalf("límites: %+v %v", l, err)
	}
	if err := cli.AjustesConversor(ctx, "global", contrato.AjustesConversor{PrepararAlIniciar: true}); err != nil {
		t.Fatal(err)
	}
	if e, err := cli.Conversor(ctx, "global"); err != nil || !e.PrepararAlIniciar {
		t.Fatalf("conversor: %+v %v", e, err)
	}
}

func TestClienteValidarEsquema(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	if _, err := cli.Activar(ctx, "global", ""); err != nil {
		t.Fatal(err)
	}
	if problemas, err := cli.ValidarEsquema(ctx, "global", json.RawMessage(`{"type":"object"}`)); err != nil || len(problemas) != 0 {
		t.Fatalf("esquema válido: %v %v", problemas, err)
	}
	if problemas, err := cli.ValidarEsquema(ctx, "global", json.RawMessage(`{"type":"nada"}`)); err != nil || len(problemas) == 0 || problemas[0] == "" {
		t.Fatalf("esquema no válido: %v %v", problemas, err)
	}
	if problemas, err := cli.ValidarEsquema(ctx, "global", json.RawMessage(`"{ esto no es JSON"`)); err != nil || len(problemas) == 0 {
		t.Fatalf("texto que no es JSON: %v %v", problemas, err)
	}
}

func TestClienteEspaciosYProyectos(t *testing.T) {
	cli, dir := servicioDePrueba(t, "")
	ctx := context.Background()
	clonado := filepath.Join(dir, "clonado")
	os.MkdirAll(filepath.Join(clonado, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(clonado, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Soporte\n"), 0o600)
	os.WriteFile(filepath.Join(clonado, ".tramtai", "local.yaml"), []byte("comandos_agentes:\n  claude: otro.exe\n"), 0o600)

	// Un proyecto con .tramtai/ que no creó la app pide confianza, con lo que hay que enseñar.
	_, err := cli.AbrirProyecto(ctx, clonado, nil, false)
	var pide *ErrorConfianza
	if !errors.As(err, &pide) || nucleo.CodigoDe(err) != nucleo.CodigoConfianzaRequerida || !slices.Equal(pide.Claves, []string{"el programa de cada agente"}) {
		t.Fatalf("debía pedir confianza: %v", err)
	}
	si := true
	e, err := cli.AbrirProyecto(ctx, clonado, &si, false)
	if err != nil || e.Estado != "activo" || !e.Confiado || e.Nombre != "Soporte" {
		t.Fatalf("abrir confiando: %+v %v", e, err)
	}
	if err := cli.Confiar(ctx, e.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := cli.Confiar(ctx, "global", true); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("la global no se confía: %v", err)
	}
	dentro := filepath.Join(clonado, "sub")
	os.MkdirAll(dentro, 0o700)
	var enOtro *ErrorDentroDeProyecto
	if _, err := cli.AbrirProyecto(ctx, dentro, nil, false); !errors.As(err, &enOtro) || enOtro.Proyecto == "" {
		t.Fatalf("debía avisar de que está dentro de otro proyecto: %v", err)
	}

	if err := cli.DetenerEspacio(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if a, err := cli.ActivarEspacio(ctx, e.ID); err != nil || a.ID != e.ID || a.Estado != "activo" {
		t.Fatalf("activar: %+v %v", a, err)
	}
	if _, err := cli.Ubicar(ctx, e.ID, t.TempDir()); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("una carpeta sin .tramtai no vale: %v", err)
	}
	if u, err := cli.Ubicar(ctx, e.ID, clonado); err != nil || u.ID != e.ID {
		t.Fatalf("ubicar: %+v %v", u, err)
	}
	if err := cli.CambiarAutenticacion(ctx, "gemini", "suscripcion"); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("gemini no admite la suscripción: %v", err)
	}
}

func TestClienteEventosInstancia(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	if _, err := cli.Activar(context.Background(), "global", ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	tipos := make(chan string, 16)
	fin := make(chan error, 1)
	go func() {
		fin <- cli.EventosInstancia(ctx, "global", func(tipo string, datos []byte) error {
			if tipo == "trabajo" {
				var e struct{ ID string }
				if json.Unmarshal(datos, &e) != nil || e.ID == "" {
					t.Errorf("el evento trabajo trae su id: %s", datos)
				}
			}
			tipos <- tipo
			return nil
		})
	}()
	espera := func(quiero string) {
		t.Helper()
		for {
			select {
			case tipo := <-tipos:
				if tipo == quiero {
					return
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("no llegó el evento %s", quiero)
			}
		}
	}
	espera("foto")
	if _, err := cli.Crear(context.Background(), "global", hola); err != nil {
		t.Fatal(err)
	}
	espera("trabajo")
	cancelar()
	select {
	case err := <-fin:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("al cancelar el contexto termina con su error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el flujo no terminó al cancelar el contexto")
	}
}

// flujoDePrueba es un servidor que abre un flujo SSE, manda lo que diga mandar y se queda abierto hasta que el cliente
// se vaya.
func flujoDePrueba(t *testing.T, mandar func(w http.ResponseWriter, f http.Flusher, r *http.Request)) *Cliente {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		f.Flush()
		mandar(w, f, r)
		<-r.Context().Done()
	}))
	t.Cleanup(ts.Close)
	return &Cliente{Base: ts.URL, token: "t", http: &http.Client{}}
}

// TestEventosInstanciaSinActividad: un flujo que se queda mudo más del plazo se da por muerto (ErrSinActividad) aunque
// el contexto siga vivo; con latidos (comentarios) sigue hasta que se cancele.
func TestEventosInstanciaSinActividad(t *testing.T) {
	mudo := flujoDePrueba(t, func(w http.ResponseWriter, f http.Flusher, _ *http.Request) {
		_, _ = w.Write([]byte("event: foto\ndata: {}\n\n"))
		f.Flush()
	})
	var tipos []string
	err := mudo.EventosInstanciaVigilado(context.Background(), "global", 150*time.Millisecond, func(tipo string, _ []byte) error {
		tipos = append(tipos, tipo)
		return nil
	})
	if !errors.Is(err, ErrSinActividad) || !slices.Equal(tipos, []string{"foto"}) {
		t.Fatalf("mudo: err %v, eventos %v", err, tipos)
	}

	latidos := flujoDePrueba(t, func(w http.ResponseWriter, f http.Flusher, r *http.Request) {
		for r.Context().Err() == nil {
			_, _ = w.Write([]byte(": latido\n\n"))
			f.Flush()
			time.Sleep(40 * time.Millisecond)
		}
	})
	ctx, cancelar := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelar()
	if err := latidos.EventosInstanciaVigilado(ctx, "global", 150*time.Millisecond, func(string, []byte) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("con latidos solo termina el contexto: %v", err)
	}

	// Sin plazo (EventosInstancia) no se vigila nada.
	ctx2, cancelar2 := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelar2()
	if err := mudo.EventosInstancia(ctx2, "global", func(string, []byte) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sin vigilancia: %v", err)
	}
}

func TestClienteInstancia(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	if _, err := cli.Activar(context.Background(), "global", ""); err != nil {
		t.Fatal(err)
	}
	r, err := cli.Instancia(context.Background(), "global")
	if err != nil || r.Base != cli.Base || r.Trabajos == nil || r.AppConClave24h != 0 {
		t.Fatalf("resumen: %+v %v", r, err)
	}
}

// Lanzar otra vez no borra lo que explicaba el fallo del arranque anterior: pasa a servicio-salida.log.1.
func TestLanzarConservaElFalloAnterior(t *testing.T) {
	// El hijo desacoplado sigue con servicio-salida.log abierto un momento y no se puede esperar (Lanzar lo suelta):
	// una carpeta propia con limpieza tolerante, para que un Windows cargado no haga fallar el borrado de t.TempDir.
	datos, err := os.MkdirTemp("", "tramtai-lanzar-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for range 50 {
			if os.RemoveAll(datos) == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	ruta := filepath.Join(datos, "servicio-salida.log")
	os.WriteFile(ruta, []byte("fallo del arranque anterior\n"), 0o600)
	if err := Lanzar(os.Args[0], datos, "-test.run=^$"); err != nil {
		t.Fatal(err)
	}
	if anterior, _ := os.ReadFile(ruta + ".1"); !strings.Contains(string(anterior), "fallo del arranque anterior") {
		t.Fatalf("se perdió el registro del fallo anterior: %q", anterior)
	}
}

// Un impostor que ocupa el puerto y conoce el identificador del descriptor (es público) no recibe el token: no puede
// responder al reto sin conocerlo.
func TestImpostorNoRecibeElToken(t *testing.T) {
	datos := t.TempDir()
	const id = "servicio_conocido"
	var conToken atomic.Bool
	impostor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			conToken.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"estado": "ok", "servicio": id, "prueba": strings.Repeat("0", 64)})
	}))
	defer impostor.Close()
	d, _ := json.Marshal(contrato.Descriptor{Identificador: id, Base: impostor.URL})
	os.WriteFile(contrato.RutaDescriptor(datos), d, 0o600)
	os.WriteFile(filepath.Join(datos, "token-servicio"), []byte(strings.Repeat("s", 43)), 0o600)

	if _, err := Conectar(datos); !errors.Is(err, ErrServicioInactivo) {
		t.Fatalf("el impostor pasó la comprobación: %v", err)
	}
	if conToken.Load() {
		t.Fatal("el token de servicio salió hacia el impostor")
	}
}

// servicioAnterior simula un servicio 0.55 con la forma exacta de su /healthz (sin reto ni prueba) y su ruta de
// detener. Devuelve la carpeta de datos, si llegó el token y si se pidió detener.
func servicioAnterior(t *testing.T) (datos string, conToken, detenido *atomic.Bool) {
	datos = t.TempDir()
	const id = "servicio_anterior"
	conToken, detenido = &atomic.Bool{}, &atomic.Bool{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			conToken.Store(true)
		}
		switch {
		case r.URL.Path == "/healthz" && !detenido.Load():
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"estado":"ok","servicio":%q,"version":"0.55.0+21102026"}`+"\n", id)
		case r.Method == "POST" && r.URL.Path == "/api/v1/servicio/detener" && r.Header.Get("Authorization") == "Bearer "+strings.Repeat("s", 43):
			detenido.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	d, _ := json.Marshal(contrato.Descriptor{Identificador: id, PID: 4242, Base: ts.URL})
	os.WriteFile(contrato.RutaDescriptor(datos), d, 0o600)
	os.WriteFile(filepath.Join(datos, "token-servicio"), []byte(strings.Repeat("s", 43)), 0o600)
	return datos, conToken, detenido
}

// Un servicio de la versión anterior no es un impostor ni un servicio inactivo: Conectar lo distingue y no le manda
// el token, porque no puede probar que lo conoce.
func TestConectarDistingueElServicioAnterior(t *testing.T) {
	datos, conToken, _ := servicioAnterior(t)
	if _, err := Conectar(datos); !errors.Is(err, ErrServicioAnterior) {
		t.Fatalf("esperaba ErrServicioAnterior: %v", err)
	}
	if conToken.Load() {
		t.Fatal("el token salió hacia un servicio que no probó su identidad")
	}
}

// Si no se puede comprobar que el servicio anterior es de este usuario, no se le manda el token, no se lanza otro
// servicio sobre los mismos datos y el error dice cómo detenerlo a mano.
func TestNoLanzaOtroServicioSobreElAnterior(t *testing.T) {
	datos, conToken, detenido := servicioAnterior(t)
	duenoPropio = func(netip.AddrPort, netip.AddrPort) bool { return false }
	t.Cleanup(func() { duenoPropio = procesos.MismoUsuario })
	_, err := IniciarEnSegundoPlano(os.Args[0], datos)
	if !errors.Is(err, ErrServicioAnterior) || !strings.Contains(err.Error(), "4242") {
		t.Fatalf("esperaba ErrServicioAnterior con el PID: %v", err)
	}
	if conToken.Load() || detenido.Load() {
		t.Fatal("sin comprobar al dueño no se manda el token")
	}
	if _, err := os.Stat(filepath.Join(datos, "servicio-salida.log")); err == nil {
		t.Fatal("se lanzó un segundo servicio sobre los mismos datos")
	}
}

// Con el servicio anterior del mismo usuario, DetenerAnterior le pide detenerse con su protocolo y espera a que deje
// de responder (drena sus trabajos).
func TestDetenerElServicioAnterior(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("sin identificación del dueño de la conexión en este sistema")
	}
	datos, _, detenido := servicioAnterior(t)
	if err := DetenerAnterior(datos); err != nil {
		t.Fatal(err)
	}
	if !detenido.Load() {
		t.Fatal("no se pidió detener el servicio anterior")
	}
	if _, err := Conectar(datos); !errors.Is(err, ErrServicioInactivo) {
		t.Fatalf("tras detenerlo no queda servicio: %v", err)
	}
}

// Un proceso que ocupa el puerto del descriptor y reenvía el reto al servicio real (que escucha en otro) no pasa: la
// prueba ata la dirección a la que el cliente preguntó.
func TestReenviarElRetoNoBasta(t *testing.T) {
	real, dir := servicioDePrueba(t, "")
	datos := filepath.Join(dir, "datos")
	reenvio := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(real.Base + r.URL.RequestURI())
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
	}))
	defer reenvio.Close()
	d := real.Descriptor
	d.Base = reenvio.URL
	contenido, _ := json.Marshal(d)
	os.WriteFile(contrato.RutaDescriptor(datos), contenido, 0o600)
	if _, err := Conectar(datos); !errors.Is(err, ErrServicioInactivo) {
		t.Fatalf("el reenvío del reto pasó la comprobación: %v", err)
	}
}

// PlazoDrenaje sale de servicio.plazo_drenaje_segundos de la configuración global.
func TestPlazoDrenajeDeLaConfiguracion(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(ruta, []byte("servicio:\n  plazo_drenaje_segundos: 300\n"), 0o600)
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", ruta)
	if p := PlazoDrenaje(); p != 300*time.Second {
		t.Fatalf("plazo: %s", p)
	}
}
