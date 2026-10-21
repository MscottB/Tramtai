package api

import (
	"context"
	"net/http/httptest"
	"strings"
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

func TestCapacidadesInformaDesconectado(t *testing.T) {
	s := nuevoServidor(t, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}}, nil, nil)
	s.Conexiones = adaptadores.NuevasConexiones(nil, nil)
	s.Conexiones.Fijar("simulado", true)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	for _, ruta := range []string{"/api/v1/agentes", "/api/v1/capacidades"} {
		if _, c := pedir(t, ts, "GET", ruta, ""); !strings.Contains(c, `"desconectado":true`) {
			t.Errorf("%s no informa desconectado: %s", ruta, c)
		}
	}
}

func TestCapacidadesSesionNoResponde(t *testing.T) {
	s := nuevoServidor(t, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}}, nil, nil)
	s.Salud = adaptadores.NuevaSalud()
	s.Salud.Fallo("simulado")
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	if _, c := pedir(t, ts, "GET", "/api/v1/capacidades", ""); !strings.Contains(c, `"problema_codigo":"sesion_no_responde"`) {
		t.Fatalf("debía avisar de que la sesión no responde: %s", c)
	}
	s.Salud.Exito("simulado")
	if _, c := pedir(t, ts, "GET", "/api/v1/capacidades", ""); strings.Contains(c, "sesion_no_responde") {
		t.Fatalf("tras una petición que funciona ya no: %s", c)
	}
}
