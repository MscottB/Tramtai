package api

import (
	"context"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
)

// lento es un agente simulado que tarda en detectarse, como un CLI que responde a --version.
type lento struct{ adaptadores.Simulado }

func (lento) Detectar(context.Context) adaptadores.Estado {
	time.Sleep(300 * time.Millisecond)
	return adaptadores.Estado{Version: "1.0"}
}

// Los agentes se detectan a la vez: tres de 300 ms responden en menos de 600 ms, en orden alfabético.
func TestCapacidadesEnParalelo(t *testing.T) {
	ts := servidor(t, map[string]adaptadores.Adaptador{"c": lento{}, "a": lento{}, "b": lento{}}, nil)
	inicio := time.Now()
	r, cuerpo := pedir(t, ts, "GET", "/api/v1/capacidades", "")
	if tardo := time.Since(inicio); r.StatusCode != 200 || tardo > 600*time.Millisecond {
		t.Fatalf("%d en %v: %s", r.StatusCode, tardo, cuerpo)
	}
	var nombres []string
	for _, a := range decodificar(t, cuerpo)["adaptadores"].([]any) {
		entrada := a.(map[string]any)
		if entrada["version_detectada"] != "1.0" {
			t.Fatalf("cada agente lleva su detección: %v", entrada)
		}
		nombres = append(nombres, entrada["agente"].(string))
	}
	if len(nombres) != 3 || nombres[0] != "a" || nombres[1] != "b" || nombres[2] != "c" {
		t.Fatalf("en orden alfabético: %v", nombres)
	}
}
