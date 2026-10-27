package main

import (
	"slices"
	"strings"
	"testing"

	"tramtai/internal/contrato"
	"tramtai/internal/credenciales"
	"tramtai/internal/servidor"
)

// TestClavesEnServidor: con --servidor las claves son las del servicio del sistema. Sin ser administrador, cada orden lo
// dice y no toca nada; como administrador, guardar va a servidor.GuardarClave (el aviso de las claves en claro sale
// antes de «Servidor: … guardada»), borrar a servidor.BorrarClave y la lista se pregunta al servicio.
func TestClavesEnServidor(t *testing.T) {
	simularServidor(t, false)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	var guardadas, borradas []string
	g, b, c := guardarClaveServidor, borrarClaveServidor, clavesServicio
	t.Cleanup(func() { guardarClaveServidor, borrarClaveServidor, clavesServicio = g, b, c })
	guardarClaveServidor = func(nombre, valor string) error {
		guardadas = append(guardadas, nombre+"="+valor)
		return servidor.ErrClaveEnClaro
	}
	borrarClaveServidor = func(nombre string) error { borradas = append(borradas, nombre); return nil }
	clavesServicio = func() ([]contrato.ClaveGuardada, error) {
		return []contrato.ClaveGuardada{{Nombre: "ANTHROPIC_API_KEY", Guardada: true}, {Nombre: "OPENAI_API_KEY"}}, nil
	}
	for _, args := range [][]string{{"claves", "guardar", "ANTHROPIC_API_KEY"}, {"keys", "delete", "ANTHROPIC_API_KEY"}, {"keys", "list"}} {
		if _, err := ejecutarServidor(t, "sk-prueba-123\n", args...); err == nil || !strings.Contains(err.Error(), "administrador") {
			t.Errorf("%v sin ser administrador: %v", args, err)
		}
	}
	if len(guardadas)+len(borradas) > 0 {
		t.Fatalf("sin ser administrador no se toca nada: %v %v", guardadas, borradas)
	}

	simularServidor(t, true)
	salida, err := ejecutarServidor(t, "sk-prueba-123\n", "keys", "set", "ANTHROPIC_API_KEY")
	aviso := strings.Index(salida, "aviso: "+servidor.ErrClaveEnClaro.Error())
	if err != nil || !slices.Equal(guardadas, []string{"ANTHROPIC_API_KEY=sk-prueba-123"}) || aviso < 0 ||
		aviso > strings.Index(salida, "Servidor: ANTHROPIC_API_KEY guardada para el servicio del sistema") {
		t.Errorf("guardar: %v %v\n%s", err, guardadas, salida)
	}
	if salida, err := ejecutarServidor(t, "", "claves", "borrar", "ANTHROPIC_API_KEY"); err != nil ||
		!slices.Equal(borradas, []string{"ANTHROPIC_API_KEY"}) || !strings.Contains(salida, "Servidor: ANTHROPIC_API_KEY eliminada") {
		t.Errorf("borrar: %v %v\n%s", err, borradas, salida)
	}
	if salida, err := ejecutarServidor(t, "", "claves", "listar"); err != nil ||
		!strings.Contains(salida, "ANTHROPIC_API_KEY    la tiene el servicio del sistema") || !strings.Contains(salida, "OPENAI_API_KEY       no configurada") {
		t.Errorf("listar: %v\n%s", err, salida)
	}
}

// TestNoSeGuardaTokenClaude: un token de sesión de Claude no se guarda nunca, tampoco con el alias en inglés, y el
// mensaje explica por qué y qué hacer (spec §5): el inicio de sesión lo hace el propio CLI de Claude.
func TestNoSeGuardaTokenClaude(t *testing.T) {
	if credenciales.Admitido("CLAUDE_CODE_OAUTH_TOKEN") {
		t.Fatal("CLAUDE_CODE_OAUTH_TOKEN no es una clave que Tramtai guarde")
	}
	for _, args := range [][]string{{"claves", "guardar", "CLAUDE_CODE_OAUTH_TOKEN"}, {"keys", "set", "CLAUDE_CODE_OAUTH_TOKEN"}} {
		_, err := ejecutarServidor(t, "sk-ant-oat01-prueba\n", args...)
		if err == nil || !strings.Contains(err.Error(), "Anthropic") || !strings.Contains(err.Error(), "claude auth login") {
			t.Errorf("%v: %v", args, err)
		}
	}
	// Otro nombre que no se admite sigue con su mensaje de siempre.
	if _, err := ejecutarServidor(t, "x\n", "claves", "guardar", "OTRA_CLAVE"); err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("otro nombre: %v", err)
	}
}

// TestClavesListaSinInternos: la lista enseña las claves de los proveedores, nunca las internas de Tramtai (el secreto
// TOTP del administrador), también con el alias en inglés.
func TestClavesListaSinInternos(t *testing.T) {
	anterior := leerClave
	t.Cleanup(func() { leerClave = anterior })
	leidas := []string{}
	leerClave = func(nombre string) (string, error) { // nunca el almacén real de quien prueba
		leidas = append(leidas, nombre)
		if nombre == "OPENAI_API_KEY" {
			return "sk-prueba-123", nil
		}
		return "", nil
	}
	for _, nombre := range credenciales.Nombres {
		t.Setenv(nombre, "")
	}
	for _, args := range [][]string{{"claves", "listar"}, {"keys", "list"}} {
		salida, err := ejecutarServidor(t, "", args...)
		if err != nil || !strings.Contains(salida, "OPENAI_API_KEY     "+credenciales.Almacen()) ||
			!strings.Contains(salida, "ANTHROPIC_API_KEY  no configurada") || strings.Contains(salida, "TRAMTAI_TOTP") {
			t.Errorf("%v: %v\n%s", args, err, salida)
		}
	}
	if slices.Contains(leidas, "TRAMTAI_TOTP") {
		t.Error("la lista ni siquiera pregunta por las internas")
	}
	if !credenciales.AdmitidoInterno("TRAMTAI_TOTP") || credenciales.Admitido("TRAMTAI_TOTP") {
		t.Error("TRAMTAI_TOTP es interna: el almacén la admite, la CLI no")
	}
}
