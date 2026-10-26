package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/planificador"
	"tramtai/internal/servicio"
)

// tokenDePrueba ejecuta «tramtai global token» (con --rotar si rotar) sobre una carpeta de datos, con la salida a un búfer.
func tokenDePrueba(t *testing.T, datos string, rotar bool) (string, error) {
	t.Helper()
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	var salida strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&salida)
	err = tokenInstancia(cmd, "global", datos, "global", c, rotar)
	return salida.String(), err
}

// En una carpeta de datos sin servicio: token solo dice si hay clave y nunca la da; --rotar la crea, la imprime una vez y
// deja en el archivo solo su huella; y, al iniciar el servicio después, esa clave entra (Revisión 2).
func TestTokenDelEspacioSinServicio(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", "")
	datos := filepath.Join(dir, "datos")
	ruta := servicio.RutaTokenInstancia(datos, "global")
	reClave := regexp.MustCompile(`tt_[0-9a-f]{64}`)

	salida, err := tokenDePrueba(t, datos, false)
	if err != nil || !strings.Contains(salida, "aún no tiene clave") || !strings.Contains(salida, "tramtai global token --rotar") {
		t.Fatalf("sin clave: %v\n%s", err, salida)
	}
	if _, err := os.Stat(ruta); err == nil {
		t.Fatal("preguntar no crea la clave")
	}

	salida, err = tokenDePrueba(t, datos, true)
	clave := strings.TrimSpace(salida)
	if err != nil || !reClave.MatchString(clave) || len(clave) != len("tt_")+64 {
		t.Fatalf("--rotar imprime la clave: %v\n%s", err, salida)
	}
	if guardado, _ := os.ReadFile(ruta); strings.TrimSpace(string(guardado)) != api.HuellaToken(clave) {
		t.Fatalf("en el archivo solo va la huella: %q", guardado)
	}

	salida, err = tokenDePrueba(t, datos, false)
	if err != nil || !strings.Contains(salida, "El espacio tiene clave") || !strings.Contains(salida, ruta) || reClave.MatchString(salida) {
		t.Fatalf("con clave, solo lo dice: %v\n%s", err, salida)
	}

	// Un servicio que arranca después sobre esa carpeta acepta la clave impresa.
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(nil)
	s, err := servicio.Nuevo(servicio.Opciones{Datos: datos, Config: c, Registro: map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}},
		Secretos: func() []string { return nil }, Semaforos: planificador.NuevosSemaforos(nil), Host: "127.0.0.1", Puerto: ts.Listener.Addr().(*net.TCPAddr).Port, Hosts: []string{"127.0.0.1"}})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s
	ts.Start()
	t.Cleanup(ts.Close)
	t.Cleanup(s.Cerrar)
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	pedir := func(clave string) int {
		req, _ := http.NewRequest("GET", ts.URL+"/api/v1/modelos", nil)
		req.Header.Set("Authorization", "Bearer "+clave)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if pedir(clave) != 200 || pedir("tt_"+strings.Repeat("0", 64)) != 401 {
		t.Fatal("la clave impresa entra al iniciar el servicio y otra no")
	}
}

// Una clave que viene de una variable de entorno no se imprime ni se regenera desde la CLI.
func TestTokenDelEspacioPorVariable(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	clave := strings.Repeat("ab", 32)
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", clave)
	datos := filepath.Join(dir, "datos")

	salida, err := tokenDePrueba(t, datos, false)
	if err != nil || !strings.Contains(salida, "viene de la variable TRAMTAI_TOKEN_GLOBAL") || strings.Contains(salida, clave) {
		t.Fatalf("clave por variable: %v\n%s", err, salida)
	}
	if _, err := tokenDePrueba(t, datos, true); err == nil || !strings.Contains(err.Error(), "variable TRAMTAI_TOKEN_GLOBAL") {
		t.Fatalf("--rotar con la clave de una variable: %v", err)
	}
	if _, err := os.Stat(servicio.RutaTokenInstancia(datos, "global")); err == nil {
		t.Fatal("con la variable no se crea archivo")
	}
}
