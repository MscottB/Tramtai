package nucleo

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestEtapa(t *testing.T) {
	casos := map[Estado]string{
		EnCola: "cola", PreparandoEntrada: "preparacion", EsperandoAgente: "preparacion",
		Ejecutando: "ejecucion", CancelacionSolicitada: "ejecucion", ValidandoSalida: "validacion",
		Completado: "llegada", Fallido: "llegada", Cancelado: "llegada", Interrumpido: "llegada",
	}
	for estado, etapa := range casos {
		if got := estado.Etapa(); got != etapa {
			t.Errorf("%s → %s, quería %s", estado, got, etapa)
		}
	}
}

// Los filtros del Historial y los contadores de Inicio agrupan los estados igual: en curso lo que no terminó,
// completadas y con error (fallido e interrumpido); los cancelados no entran en ninguno.
func TestFiltrosDeEstado(t *testing.T) {
	quiere := map[string][]Estado{
		FiltroEnCurso:    {EnCola, PreparandoEntrada, EsperandoAgente, Ejecutando, ValidandoSalida, CancelacionSolicitada},
		FiltroCompletado: {Completado},
		FiltroConError:   {Fallido, Interrumpido},
		"otro":           nil,
	}
	for filtro, estados := range quiere {
		if got := EstadosDe(filtro); !slices.Equal(got, estados) {
			t.Errorf("%s: %v, quería %v", filtro, got, estados)
		}
	}
}

func TestSePuedeCorregir(t *testing.T) {
	noCumple := Trabajo{Estado: Fallido, CodigoError: CodigoSalidaNoCumpleEsquema, Salida: `{"a":1}`}
	casos := []struct {
		nombre string
		t      Trabajo
		quiere bool
	}{
		{"no cumplía y conservó la respuesta", noCumple, true},
		{"sin la respuesta", Trabajo{Estado: Fallido, CodigoError: CodigoSalidaNoCumpleEsquema}, false},
		{"otro error", Trabajo{Estado: Fallido, CodigoError: CodigoAgenteNoDisponible, Salida: "x"}, false},
		{"sin terminar", Trabajo{Estado: ValidandoSalida, CodigoError: CodigoSalidaNoCumpleEsquema, Salida: "x"}, false},
		{"entradas vencidas", Trabajo{Estado: Fallido, CodigoError: CodigoSalidaNoCumpleEsquema, Salida: "x", EntradasEliminadas: true}, false},
	}
	for _, c := range casos {
		if got := c.t.SePuedeCorregir(); got != c.quiere {
			t.Errorf("%s: %v, quería %v", c.nombre, got, c.quiere)
		}
	}
}

// Un evento de estado del registro lleva la etapa de ese estado; los demás eventos no llevan etapa.
func TestEventoRegistroJSONLlevaLaEtapa(t *testing.T) {
	for ev, quiere := range map[EventoRegistro]string{{Tipo: "estado", Datos: "esperando_agente"}: "preparacion", {Tipo: "aviso", Datos: "en_cola"}: ""} {
		datos, _ := json.Marshal(ev)
		var leido EventoRegistro
		if err := json.Unmarshal(datos, &leido); err != nil || leido.Etapa != quiere {
			t.Errorf("%s: %v %q", datos, err, leido.Etapa)
		}
	}
}

// El JSON de un trabajo lleva su etapa y si se puede corregir: los clientes los pintan sin calcularlos.
func TestTrabajoJSONLlevaLoDerivado(t *testing.T) {
	datos, err := json.Marshal(Trabajo{Estado: Fallido, CodigoError: CodigoSalidaNoCumpleEsquema, Salida: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var leido Trabajo
	if err := json.Unmarshal(datos, &leido); err != nil {
		t.Fatal(err)
	}
	if leido.Etapa != "llegada" || !leido.Corregible || leido.Estado != Fallido || leido.Salida != "x" {
		t.Fatalf("%s → %+v", datos, leido)
	}
}
