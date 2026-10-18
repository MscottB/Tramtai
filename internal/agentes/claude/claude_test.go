package claude

import (
	"context"
	"encoding/json"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

func solicitudSimple(texto string) nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
}

func TestContextoClaude(t *testing.T) {
	var r resultadoClaude
	datos := `{"result":"ok","modelUsage":{"claude-haiku-4-5":{"outputTokens":5,"contextWindow":200000},"aux":{"outputTokens":1,"contextWindow":100}}}`
	if err := json.Unmarshal([]byte(datos), &r); err != nil {
		t.Fatal(err)
	}
	if r.contexto() != 200000 {
		t.Fatalf("el contexto del modelo efectivo: %d", r.contexto())
	}
}

func TestModeloEfectivoEligeElDeMasSalida(t *testing.T) {
	var r resultadoClaude
	if err := json.Unmarshal([]byte(`{"modelUsage":{"auxiliar":{"outputTokens":3},"principal":{"outputTokens":40}}}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.modeloEfectivo() != "principal" {
		t.Fatalf("modelo efectivo %q", r.modeloEfectivo())
	}
}

// La Web UI decide la ayuda por el código del problema, no por su texto (que cambia con el idioma).
func TestDetectarNoInstalado(t *testing.T) {
	sin := Claude{Programa: "tramtai-no-existe-xyz", Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionSuscripcion)}
	if e := sin.Detectar(context.Background()); e.ProblemaCodigo != adaptadores.ProblemaNoInstalado {
		t.Fatalf("se esperaba no_instalado: %+v", e)
	}
}
