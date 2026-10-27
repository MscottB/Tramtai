package main

import (
	"strings"
	"testing"

	"tramtai/internal/credenciales"
)

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
	for _, args := range [][]string{{"claves", "listar"}, {"keys", "list"}} {
		salida, err := ejecutarServidor(t, "", args...)
		if err != nil || !strings.Contains(salida, "ANTHROPIC_API_KEY") || strings.Contains(salida, "TRAMTAI_TOTP") {
			t.Errorf("%v: %v\n%s", args, err, salida)
		}
	}
	if !credenciales.AdmitidoInterno("TRAMTAI_TOTP") || credenciales.Admitido("TRAMTAI_TOTP") {
		t.Error("TRAMTAI_TOTP es interna: el almacén la admite, la CLI no")
	}
}
