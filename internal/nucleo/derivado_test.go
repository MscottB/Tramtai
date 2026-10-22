package nucleo

import (
	"encoding/json"
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
