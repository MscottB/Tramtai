//go:build agentes

// Pruebas contra los CLIs reales (consumen cuota): go test -tags agentes -v ./internal/agentes
// Usan el modo de autenticación de TRAMTAI_PRUEBA_AUTENTICACION (clave_api por defecto).
package agentes

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes/claude"
	"tramtai/internal/agentes/codex"
	"tramtai/internal/nucleo"
)

func autenticacion() string {
	if a := os.Getenv("TRAMTAI_PRUEBA_AUTENTICACION"); a != "" {
		return a
	}
	return adaptadores.AutenticacionClaveAPI
}

func probar(t *testing.T, a adaptadores.Adaptador, modelo string) {
	t.Helper()
	s := nucleo.SolicitudEjecucion{Modelo: modelo, Mensajes: []nucleo.Mensaje{
		{Rol: "sistema", Contenido: "Responde solo con la palabra pedida, en mayúsculas."},
		{Rol: "usuario", Contenido: "Escribe la palabra: tramtai"}}}
	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancelar()
	var eventos int
	r, err := a.Ejecutar(ctx, s, func(adaptadores.Evento) error { eventos++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(r.Salida), "TRAMTAI") || eventos == 0 || r.Uso.TokensSalida == 0 {
		t.Fatalf("resultado inesperado: %+v (eventos %d)", r, eventos)
	}
	t.Logf("salida %q, modelo efectivo %q, uso %+v, eventos %d", r.Salida, r.ModeloEfectivo, r.Uso, eventos)

	// Cancelación: una respuesta larga se corta y el adaptador devuelve context.Canceled.
	ctx, cancelar = context.WithCancel(context.Background())
	s.Mensajes = []nucleo.Mensaje{{Rol: "usuario", Contenido: "Escribe un ensayo de 2000 palabras sobre los puentes."}}
	time.AfterFunc(4*time.Second, cancelar)
	inicio := time.Now()
	if _, err := a.Ejecutar(ctx, s, func(adaptadores.Evento) error { return nil }); err != context.Canceled {
		t.Fatalf("se esperaba context.Canceled: %v", err)
	}
	t.Logf("cancelado en %s", time.Since(inicio).Round(time.Millisecond))
}

func TestClaudeReal(t *testing.T) {
	probar(t, claude.Claude{Programa: "claude", Autenticacion: adaptadores.NuevoModo(autenticacion())}, "haiku")
}

func TestCodexReal(t *testing.T) {
	probar(t, codex.Codex{Programa: "codex", Autenticacion: adaptadores.NuevoModo(autenticacion())}, os.Getenv("TRAMTAI_PRUEBA_MODELO_CODEX"))
}
