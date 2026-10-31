package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"tramtai/internal/carpetas"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
)

// servicioFalso es un servicio que se deja encontrar como el de verdad (descriptor y token en una carpeta de datos, y
// la prueba de identidad en /healthz) y cuenta todo lo que recibe. peticiones debe quedar en lo que la prueba espera.
func servicioFalso(t *testing.T) (datos string, base string, peticiones *atomic.Int32) {
	t.Helper()
	datos, peticiones = t.TempDir(), &atomic.Int32{}
	const token = "token-de-prueba-del-servicio-falso"
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peticiones.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"prueba": contrato.PruebaIdentidad(token, ts.URL, r.URL.Query().Get("reto"))})
	}))
	t.Cleanup(ts.Close)
	d, _ := json.Marshal(contrato.Descriptor{Identificador: "servicio_falso", PID: 4242, Base: ts.URL})
	if err := os.WriteFile(contrato.RutaDescriptor(datos), d, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(datos, "token-servicio"), []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	return datos, ts.URL, peticiones
}

// TestLaCLIBuscaElServicioEnDirDatos: la CLI encuentra el servicio por la carpeta de datos de dirDatos (descriptor y token
// ahí), no por un puerto ni por la carpeta real del equipo. Es lo que permite a las pruebas ponerle un servicio propio o
// ninguno: sin descriptor, conectar() no hace ninguna petición.
func TestLaCLIBuscaElServicioEnDirDatos(t *testing.T) {
	datos, base, peticiones := servicioFalso(t)
	anterior := dirDatos
	t.Cleanup(func() { dirDatos = anterior })

	dirDatos = func() (string, error) { return datos, nil }
	c, err := conectar()
	if err != nil || c.Base != base || peticiones.Load() == 0 {
		t.Fatalf("con el servicio falso en dirDatos: %v, %v, peticiones %d", c, err, peticiones.Load())
	}

	antes := peticiones.Load()
	dirDatos = func() (string, error) { return t.TempDir(), nil }
	if _, err := conectar(); !errors.Is(err, cliente.ErrServicioInactivo) || peticiones.Load() != antes {
		t.Errorf("con otra carpeta de datos no hay servicio ni peticiones: %v, peticiones %d (antes %d)", err, peticiones.Load(), antes)
	}
}

// TestAdministradorEnServidorHablaConElServicio: en el modo servidor, el administrador no ejecuta los agentes él (lo rechaza
// ejecucionLocalPermitida), pero si el servicio del sistema está en marcha, «ejecutar» le manda la petición: es la salida
// que el aviso indica («usa el servicio»). Por eso el guard no frena el envío al servicio, y por eso las pruebas tienen que
// ser ellas quienes decidan qué servicio hay (dirDatos), no el equipo.
func TestAdministradorEnServidorHablaConElServicio(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	datos := t.TempDir()
	servicioEnMarcha(t, datos)
	anterior := dirDatos
	dirDatos = func() (string, error) { return datos, nil }
	t.Cleanup(func() { dirDatos = anterior })
	raiz := nuevaRaiz()
	var salida strings.Builder
	raiz.SetOut(&salida)
	raiz.SetErr(&salida)
	raiz.SetArgs([]string{"ejecutar", "-a", "simulado", "hola"})
	if err := raiz.Execute(); err != nil {
		t.Fatalf("como administrador, con el servicio en marcha: %v\n%s", err, salida.String())
	}
	// «espacio global» solo lo imprime la ejecución en el servicio (la local no tiene espacio) y el agente simulado devuelve el texto.
	for _, quiere := range []string{"espacio global · agente simulado", "hola", "completado"} {
		if !strings.Contains(salida.String(), quiere) {
			t.Errorf("la petición no pasó por el servicio: falta %q en %q", quiere, salida.String())
		}
	}
}

// dentroDeTemporal dice si ruta cuelga de la carpeta temporal, con o sin enlaces ni nombres cortos (ADMINI~1) resueltos.
func dentroDeTemporal(ruta string) bool {
	bases, rutas := []string{os.TempDir()}, []string{ruta}
	if real, err := filepath.EvalSymlinks(bases[0]); err == nil {
		bases = append(bases, real)
	}
	if real, err := filepath.EvalSymlinks(ruta); err == nil {
		rutas = append(rutas, real)
	}
	for _, b := range bases {
		for _, r := range rutas {
			if rel, err := filepath.Rel(b, r); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				return true
			}
		}
	}
	return false
}

// TestPruebasNoVenElEquipo: nada de lo que mira la CLI para encontrar un servicio (la raíz del usuario, el HOME, la
// carpeta de datos, la raíz del servidor) sale de una carpeta temporal, que pone TestMain para todo el paquete. Si no,
// una prueba como «ejecutar» sin --directo llegaría al Tramtai instalado del equipo, con su suscripción.
func TestPruebasNoVenElEquipo(t *testing.T) {
	var fuera []string
	anotar := func(nombre, ruta string, err error) {
		if err != nil || !dentroDeTemporal(ruta) {
			fuera = append(fuera, nombre+" = "+ruta)
		}
	}
	raiz, err := carpetas.RaizUsuario()
	anotar("RaizUsuario", raiz, err)
	config, err := os.UserConfigDir()
	anotar("UserConfigDir", config, err)
	casa, err := os.UserHomeDir()
	anotar("UserHomeDir", casa, err)
	if runtime.GOOS == "windows" { // en las demás, la raíz del servidor es /var/lib/tramtai y solo se llega con TRAMTAI_SERVIDOR
		anotar("RaizServidor", carpetas.RaizServidor(), nil)
	}
	if len(fuera) > 0 {
		t.Fatalf("estas rutas son del equipo y no de la prueba (falta el TestMain que las aísla): %v", fuera)
	}
	if os.Getenv("TRAMTAI_SERVIDOR") != "" {
		t.Errorf("TRAMTAI_SERVIDOR heredada del entorno: el modo servidor lo fija cada prueba que lo necesita")
	}
	datos, err := dirDatos()
	if err != nil || !dentroDeTemporal(datos) {
		t.Fatalf("dirDatos = %q, %v: no es una carpeta temporal", datos, err)
	}
	// Sin descriptor en esa carpeta, la CLI ni siquiera intenta conectar: no hay a quién.
	if _, err := conectar(); !errors.Is(err, cliente.ErrServicioInactivo) {
		t.Errorf("conectar() con la carpeta de datos de las pruebas: %v", err)
	}
}
