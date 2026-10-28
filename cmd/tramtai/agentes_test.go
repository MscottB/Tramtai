package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/agentes"
	"tramtai/internal/configuracion"
	"tramtai/internal/terminos"
)

// TestSuscripcionPersonalCLI: «agentes suscripcion <agente> --personal» enseña los riesgos 1 y 2 y las condiciones, y
// sin «acepto» no escribe nada ni reinicia; con «acepto», las tres claves en la global del servidor y reinicia. Subir
// las peticiones a la vez pide confirmarlo otra vez; fuera de 1 a 3 no se acepta. También con el alias en inglés.
func TestSuscripcionPersonalCLI(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	simularTOTP(t)
	global := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	reinicios := 0
	anterior := reiniciarServidor
	t.Cleanup(func() { reiniciarServidor = anterior })
	reiniciarServidor = func() error { reinicios++; return nil }

	salida, err := ejecutarAdmin(t, "no\n", "agentes", "suscripcion", "claude", "--personal")
	if err == nil || reinicios != 0 || !strings.Contains(salida, terminos.Riesgos(true)[0]) || !strings.Contains(salida, terminos.Riesgos(true)[1]) ||
		!strings.Contains(salida, "solo la usas tú") {
		t.Fatalf("sin «acepto»: %v %d\n%s", err, reinicios, salida)
	}
	if _, err := os.Stat(global); err == nil {
		t.Fatal("sin «acepto» no se escribe nada")
	}
	if _, err := ejecutarAdmin(t, "", "agentes", "suscripcion", "claude", "--personal", "--simultaneas", "4"); err == nil || !strings.Contains(err.Error(), "1 a 3") {
		t.Fatalf("4 a la vez: %v", err)
	}
	if _, err := ejecutarAdmin(t, "acepto\nno\n", "agents", "subscription", "claude", "--personal", "--simultaneas", "2"); err == nil || reinicios != 0 {
		t.Fatalf("subir a 2 sin confirmarlo: %v %d", err, reinicios)
	}
	if _, err := ejecutarAdmin(t, "acepto\n", "agents", "subscription", "claude", "--personal"); err != nil || reinicios != 1 {
		t.Fatalf("con «acepto»: %v %d", err, reinicios)
	}
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil || c.Texto("adaptadores.claude.autenticacion") != "suscripcion" || !c.Booleano("adaptadores.claude.suscripcion_personal") ||
		c.Entero("adaptadores.claude.suscripcion_simultaneas") != 1 {
		t.Fatalf("las tres claves: %v", err)
	}
	if _, err := ejecutarAdmin(t, "acepto\nacepto\n", "agentes", "suscripcion", "claude", "--personal", "--simultaneas", "3"); err != nil || reinicios != 2 {
		t.Fatalf("3 a la vez, confirmado: %v %d", err, reinicios)
	}
	if c, _ := configuracion.CargarConfiguracion("", agentes.Internos()); c.Entero("adaptadores.claude.suscripcion_simultaneas") != 3 {
		t.Fatal("suscripcion_simultaneas: 3")
	}
	if _, err := ejecutarAdmin(t, "acepto\n", "agentes", "suscripcion", "gemini", "--personal"); err == nil {
		t.Fatal("gemini no admite la suscripción")
	}
	t.Setenv("TRAMTAI_SERVIDOR", "")
	if _, err := ejecutarAdmin(t, "acepto\n", "agentes", "suscripcion", "claude", "--personal"); err == nil || !strings.Contains(err.Error(), "--servidor") {
		t.Fatalf("--personal fuera del servidor: %v", err)
	}
}

// TestEntrarSoloServidor: «agentes entrar» (agents login) es del modo servidor; fuera de él lo dice, sin lanzar nada.
func TestEntrarSoloServidor(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "")
	for _, orden := range [][]string{{"agentes", "entrar", "claude"}, {"agents", "login", "claude"}} {
		if _, err := ejecutarAdmin(t, "", orden...); err == nil || !strings.Contains(err.Error(), "tramtai --servidor agentes entrar") {
			t.Fatalf("%v fuera del servidor: %v", orden, err)
		}
	}
}
