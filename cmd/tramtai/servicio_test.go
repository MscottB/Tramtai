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

	"tramtai/internal/carpetas"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
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

// TestSinServidorNoTocaElDelSistema: el servicio del sistema vive en la raíz del servidor. La CLI de escritorio no lo ve
// (mira en la suya) y, con --servidor, si responde uno de otra versión, «servicio iniciar» da ErrServidorOtraVersion y
// lo deja como estaba: no lo detiene, no le manda el token y no arranca otro sobre sus datos.
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

	t.Setenv("TRAMTAI_SERVIDOR", "1")
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
