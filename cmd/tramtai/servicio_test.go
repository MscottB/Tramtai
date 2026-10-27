package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/carpetas"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/terminos"
)

// servicioDeOtraVersion simula el servicio de una versión anterior (la forma de su /healthz, sin reto ni prueba) en una
// carpeta de datos nueva. tocado dice si alguien le mandó algo más que /healthz, o el token.
func servicioDeOtraVersion(t *testing.T) (datos string, tocado *atomic.Bool) {
	t.Helper()
	datos, tocado = t.TempDir(), &atomic.Bool{}
	const id = "servicio_del_sistema"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" || r.Header.Get("Authorization") != "" {
			tocado.Store(true)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"estado":"ok","servicio":%q,"version":"0.55.0+21102026"}`+"\n", id)
	}))
	t.Cleanup(ts.Close)
	d, _ := json.Marshal(contrato.Descriptor{Identificador: id, PID: 4242, Base: ts.URL})
	if err := os.WriteFile(contrato.RutaDescriptor(datos), d, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(datos, "token-servicio"), []byte(strings.Repeat("s", 43)), 0o600); err != nil {
		t.Fatal(err)
	}
	return datos, tocado
}

// TestSinServidorNoTocaElDelSistema: el servicio del sistema vive en la raíz del servidor. Que el escritorio y el servidor
// miran en carpetas distintas lo prueban TestRaizServidor y TestRaizDeServidor (internal/carpetas); aquí dirDatos se
// sustituye por una carpeta del usuario y otra del servidor, así que la primera parte solo comprueba el efecto: la CLI de
// escritorio no ve el servicio del sistema. Con --servidor, si responde uno de otra versión, «servicio iniciar» da
// ErrServidorOtraVersion y lo deja como estaba: no lo detiene, no le manda el token y no arranca otro sobre sus datos.
func TestSinServidorNoTocaElDelSistema(t *testing.T) {
	datosServidor, tocado := servicioDeOtraVersion(t)
	datosUsuario := t.TempDir()
	anterior := dirDatos
	dirDatos = func() (string, error) {
		if carpetas.Servidor() {
			return datosServidor, nil
		}
		return datosUsuario, nil
	}
	t.Cleanup(func() { dirDatos = anterior })

	datos, _ := dirDatos()
	if _, err := cliente.Conectar(datos); !errors.Is(err, cliente.ErrServicioInactivo) {
		t.Fatalf("la CLI de escritorio no ve el servicio del sistema: %v", err)
	}

	t.Run("servidor", func(t *testing.T) {
		t.Setenv("TRAMTAI_SERVIDOR", "1")
		// La configuración global de la prueba, no la del equipo (que saldría de RaizUsuario y la fijaría como la del servidor).
		t.Setenv("TRAMTAI_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "config.yaml"))
		cmd := &cobra.Command{}
		var salida bytes.Buffer
		cmd.SetErr(&salida)
		err := iniciarServicio(cmd, "", false)
		if !errors.Is(err, cliente.ErrServidorOtraVersion) || errors.Is(err, cliente.ErrServicioAnterior) {
			t.Fatalf("--servidor con un servicio de otra versión: %v", err)
		}
		if strings.Contains(salida.String(), "deteniendo") {
			t.Errorf("no debe decir que detiene el servicio: %q", salida.String())
		}
		if tocado.Load() {
			t.Fatal("el servicio del sistema recibió algo más que /healthz")
		}
		d, _ := dirDatos()
		if _, err := cliente.Conectar(d); !errors.Is(err, cliente.ErrServidorOtraVersion) {
			t.Fatalf("sigue respondiendo: %v", err)
		}
	})

	// La prueba no deja la raíz de este proceso fijada como la del servidor para las demás.
	if r, _ := carpetas.RaizUsuario(); r == carpetas.RaizServidor() {
		t.Fatalf("la raíz del proceso quedó fijada como la del servidor: %s", r)
	}
}

// arrancarHastaDatos lanza iniciarServicio con esta configuración global y lo corta justo después de las reglas de
// exposición (al pedir la carpeta de datos). Devuelve lo que escribió en el registro y el error.
func arrancarHastaDatos(t *testing.T, config string) (registro string, err error) {
	t.Helper()
	anterior := dirDatos
	dirDatos = func() (string, error) { return "", errors.New("parada de la prueba") }
	t.Cleanup(func() { dirDatos = anterior })
	global := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(global, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	cmd := &cobra.Command{}
	var salida bytes.Buffer
	cmd.SetErr(&salida)
	err = iniciarServicio(cmd, "", false)
	return salida.String(), err
}

// TestAvisoExpuestoEnRegistro: expuesto (aquí, con un proxy de confianza), el arranque deja en el registro el riesgo 3;
// en loopback sin proxy, no.
func TestAvisoExpuestoEnRegistro(t *testing.T) {
	riesgo := terminos.Riesgos(true)[2]
	for config, quiere := range map[string]bool{
		"servicio:\n  proxy:\n    confiables: [127.0.0.1/32]\n": true,
		"servicio:\n  host: 127.0.0.1\n":                        false,
	} {
		registro, err := arrancarHastaDatos(t, config)
		if err == nil || !strings.Contains(err.Error(), "parada de la prueba") {
			t.Fatalf("%q: las reglas lo dejan arrancar: %v", config, err)
		}
		if strings.Contains(registro, riesgo) != quiere {
			t.Errorf("%q: riesgo 3 en el registro %v: %q", config, !quiere, registro)
		}
	}
}

// TestProxyLoopbackConSuscripcion (Revisión 3): un proxy de confianza en este equipo deja el servicio expuesto aunque
// escuche en loopback, así que con Claude en suscripción (no personal) no arranca.
func TestProxyLoopbackConSuscripcion(t *testing.T) {
	_, err := arrancarHastaDatos(t, "servicio:\n  proxy:\n    confiables: [127.0.0.1]\nadaptadores:\n  claude:\n    autenticacion: suscripcion\n")
	if err == nil || strings.Contains(err.Error(), "parada de la prueba") || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("con un proxy de confianza y Claude en suscripción: %v", err)
	}
	_, err = arrancarHastaDatos(t, "servicio:\n  proxy:\n    confiables: [proxy.ejemplo.org]\n")
	if err == nil || !strings.Contains(err.Error(), "servicio.proxy.confiables") {
		t.Fatalf("una red que no es CIDR: %v", err)
	}
}

// TestDominiosEnHosts: Host y Origin valen para servicio.dominios; un nombre que no está ahí lo rechaza api.Proteger.
func TestDominiosEnHosts(t *testing.T) {
	pedir := func(hosts []string) int {
		r := httptest.NewRequest("GET", "/healthz", nil)
		r.Host = "tramtai.ejemplo.org"
		w := httptest.NewRecorder()
		api.Proteger(hosts, nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, r)
		return w.Code
	}
	if c := pedir(hostsPermitidos("127.0.0.1", []string{"Tramtai.Ejemplo.org"})); c != http.StatusOK {
		t.Errorf("con el dominio: %d", c)
	}
	if c := pedir(hostsPermitidos("127.0.0.1", nil)); c != http.StatusForbidden {
		t.Errorf("sin el dominio: %d", c)
	}
	if h := hostsPermitidos("192.0.2.10", nil); !slices.Contains(h, "192.0.2.10") || !slices.Contains(h, "localhost") {
		t.Errorf("el host remoto se sigue sumando: %v", h)
	}
}

// configDePrueba carga esta configuración global como la del servicio.
func configDePrueba(t *testing.T, config string) *configuracion.Configuracion {
	t.Helper()
	global := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(global, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestDominiosNormalizados: servicio.dominios sin espacios, sin el punto final y en minúsculas; uno vacío o con puerto
// es un error que nombra la clave.
func TestDominiosNormalizados(t *testing.T) {
	e, err := entradaServidor(configDePrueba(t, "servicio:\n  dominios: [\" Tramtai.Ejemplo.ORG. \", otro.ejemplo.org]\n"))
	if err != nil || !slices.Equal(e.Dominios, []string{"tramtai.ejemplo.org", "otro.ejemplo.org"}) {
		t.Fatalf("normalizados: %v %v", e.Dominios, err)
	}
	if h := hostsPermitidos("127.0.0.1", e.Dominios); !slices.Contains(h, "tramtai.ejemplo.org") {
		t.Errorf("los hosts usan la forma normalizada: %v", h)
	}
	for _, malo := range []string{`""`, `"."`, "tramtai.ejemplo.org:443", `"tramtai ejemplo.org"`} {
		if _, err := entradaServidor(configDePrueba(t, "servicio:\n  dominios: ["+malo+"]\n")); err == nil || !strings.Contains(err.Error(), "servicio.dominios") {
			t.Errorf("%s: %v", malo, err)
		}
	}
}

// TestEntradaLocalExpuesto: con el servicio expuesto (proxy declarado o host remoto) no hay entrada local.
func TestEntradaLocalExpuesto(t *testing.T) {
	for config, quiere := range map[string]bool{
		"servicio:\n  host: 127.0.0.1\n":                        true,
		"servicio:\n  proxy:\n    confiables: [127.0.0.1/32]\n": false,
		"servicio:\n  host: 0.0.0.0\n":                          false,
		"servicio:\n  entrada_local: false\n":                   false,
	} {
		c := configDePrueba(t, config)
		e, err := entradaServidor(c)
		if err != nil || entradaLocal(c, e) != quiere {
			t.Errorf("%q: entrada local %v (%v)", config, !quiere, err)
		}
	}
}

// TestOpcionServidor: --servidor (y --server) pone el modo servidor antes de que el aviso de los términos pida la carpeta
// de datos; sin la opción no se toca.
func TestOpcionServidor(t *testing.T) {
	var pasos []string
	anteriorDatos, anteriorUsar := dirDatos, usarServidor
	dirDatos = func() (string, error) { pasos = append(pasos, "datos"); return t.TempDir(), nil }
	usarServidor = func() { pasos = append(pasos, "servidor") }
	t.Cleanup(func() { dirDatos, usarServidor = anteriorDatos, anteriorUsar })

	for _, opcion := range []string{"--servidor", "--server"} {
		pasos = nil
		raiz := nuevaRaiz()
		raiz.SetOut(&bytes.Buffer{})
		raiz.SetErr(&bytes.Buffer{})
		raiz.SetArgs([]string{opcion, "version"})
		if err := raiz.Execute(); err != nil || !slices.Equal(pasos, []string{"servidor", "datos"}) {
			t.Errorf("%s: %v %v", opcion, pasos, err)
		}
	}
	pasos = nil
	raiz := nuevaRaiz()
	raiz.SetOut(&bytes.Buffer{})
	raiz.SetErr(&bytes.Buffer{})
	raiz.SetArgs([]string{"version"})
	if err := raiz.Execute(); err != nil || slices.Contains(pasos, "servidor") {
		t.Errorf("sin la opción: %v %v", pasos, err)
	}
}
