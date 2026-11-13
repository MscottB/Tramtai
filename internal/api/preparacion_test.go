package api

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/preparar"
)

// preparacionFalsa hace de preparar.Preparador: Iniciar deja la preparación «instalando» y fin la termina.
type preparacionFalsa struct {
	mu         sync.Mutex
	estados    map[string]contrato.EstadoPreparacion
	pedidos    []string
	cancelados []string
	pegados    []string
	err        error // lo que devuelve Iniciar
}

func (f *preparacionFalsa) Estado(agente string) (contrato.EstadoPreparacion, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.estados[agente]
	return e, ok
}

func (f *preparacionFalsa) Iniciar(agente string, p contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.estados[agente], f.err
	}
	f.pedidos = append(f.pedidos, agente+" "+p.Accion)
	f.estados[agente] = contrato.EstadoPreparacion{Agente: agente, Accion: p.Accion, Estado: "instalando"}
	return f.estados[agente], nil
}

func (f *preparacionFalsa) Cancelar(agente string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelados = append(f.cancelados, agente)
	return nil
}

// Codigo guarda lo pegado como el de verdad (que lo pasa al CLI) y, si es corto, falla como él: sin repetirlo.
func (f *preparacionFalsa) Codigo(agente, codigo string) error {
	if len(codigo) < 8 {
		return errors.New("el código pegado no es válido")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pegados = append(f.pegados, agente+" "+codigo)
	e := f.estados[agente]
	e.Estado = "comprobando"
	f.estados[agente] = e
	return nil
}

func (f *preparacionFalsa) fin(agente, estado string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := f.estados[agente]
	e.Estado = estado
	f.estados[agente] = e
}

func servidorPreparacion(t *testing.T, punto *Punto) (*httptest.Server, *preparacionFalsa, *[]bool) {
	t.Helper()
	s := nuevoServidor(t, simulado(adaptadores.Simulado{}), nil, punto)
	f := &preparacionFalsa{estados: map[string]contrato.EstadoPreparacion{}}
	var forzados []bool
	var mu sync.Mutex
	s.Preparacion = f
	s.VersionesAgentes = func(_ context.Context, forzar bool) []contrato.VersionesAgente {
		mu.Lock()
		defer mu.Unlock()
		forzados = append(forzados, forzar)
		return []contrato.VersionesAgente{{Agente: "claude", Instalada: "2.1.300", Acciones: []string{"actualizar"}}}
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, f, &forzados
}

// TestPrepararRutas: el ciclo instalar → GET → listo, cancelar, las versiones y los errores con su código.
func TestPrepararRutas(t *testing.T) {
	ts, f, forzados := servidorPreparacion(t, nil)
	if r, cuerpo := pedir(t, ts, "GET", "/api/v1/agentes/claude/preparacion", ""); r.StatusCode != 200 || strings.TrimSpace(cuerpo) != "null" {
		t.Fatalf("sin preparación: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo := pedir(t, ts, "POST", "/api/v1/agentes/claude/preparacion", `{"accion":"instalar","borrar_datos":false}`)
	if r.StatusCode != 202 || decodificar(t, cuerpo)["estado"] != "instalando" || !slices.Equal(f.pedidos, []string{"claude instalar"}) {
		t.Fatalf("instalar: %d %s %q", r.StatusCode, cuerpo, f.pedidos)
	}
	f.fin("claude", "listo")
	if r, cuerpo := pedir(t, ts, "GET", "/api/v1/agentes/claude/preparacion", ""); r.StatusCode != 200 || decodificar(t, cuerpo)["estado"] != "listo" {
		t.Fatalf("listo: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := pedir(t, ts, "DELETE", "/api/v1/agentes/claude/preparacion", ""); r.StatusCode != 204 || !slices.Equal(f.cancelados, []string{"claude"}) {
		t.Fatalf("cancelar: %d %s %q", r.StatusCode, cuerpo, f.cancelados)
	}

	// Un id que no es del catálogo, o uno sin receta (Gemini), es un 404 que no llega a la preparación.
	for _, id := range []string{"nadie", "gemini", "simulado", "Claude"} {
		for _, metodo := range []string{"GET", "POST", "DELETE"} {
			if r, cuerpo := pedir(t, ts, metodo, "/api/v1/agentes/"+id+"/preparacion", `{"accion":"instalar"}`); r.StatusCode != 404 {
				t.Errorf("%s %s: %d %s", metodo, id, r.StatusCode, cuerpo)
			}
		}
		if r, _ := pedir(t, ts, "POST", "/api/v1/agentes/"+id+"/preparacion/codigo", `{"codigo":"abc-12345"}`); r.StatusCode != 404 {
			t.Errorf("código a %s: %d", id, r.StatusCode)
		}
	}
	if len(f.pedidos) != 1 || len(f.cancelados) != 1 || len(f.pegados) != 0 {
		t.Fatalf("un id desconocido no toca nada: %q %q %q", f.pedidos, f.cancelados, f.pegados)
	}

	// Errores: los de la preparación, 409 con su código (ya_en_curso trae el estado de la que está en marcha); los de
	// entrada, 400.
	f.estados["codex"] = contrato.EstadoPreparacion{Agente: "codex", Accion: "instalar", Estado: "instalando"}
	for codigo, quiere := range map[string]int{preparar.CodigoYaEnCurso: 409, preparar.CodigoPeticionesEnCurso: 409} {
		f.err = &preparar.Error{Codigo: codigo, Mensaje: "no ahora"}
		r, cuerpo := pedir(t, ts, "POST", "/api/v1/agentes/codex/preparacion", `{"accion":"actualizar"}`)
		v := decodificar(t, cuerpo)
		fallo, _ := v["error"].(map[string]any)
		if r.StatusCode != quiere || fallo["codigo"] != codigo || fallo["mensaje"] != "no ahora" {
			t.Errorf("%s: %d %s", codigo, r.StatusCode, cuerpo)
		}
		if en, _ := v["preparacion"].(map[string]any); (codigo == preparar.CodigoYaEnCurso) != (en["estado"] == "instalando") {
			t.Errorf("%s: el estado de la que está en marcha: %s", codigo, cuerpo)
		}
	}
	f.err = errors.New("acción no disponible")
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/agentes/codex/preparacion", `{"accion":"bailar"}`); r.StatusCode != 400 || !strings.Contains(cuerpo, "solicitud_invalida") {
		t.Errorf("error de entrada: %d %s", r.StatusCode, cuerpo)
	}
	f.err = nil
	if r, cuerpo := pedir(t, ts, "POST", "/api/v1/agentes/codex/preparacion", `{"accion":"instalar","otro":1}`); r.StatusCode != 400 {
		t.Errorf("campo desconocido: %d %s", r.StatusCode, cuerpo)
	}

	// Las versiones: la guardada, salvo con ?forzar=1.
	if r, cuerpo := pedir(t, ts, "GET", "/api/v1/agentes/versiones", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"instalada":"2.1.300"`) {
		t.Fatalf("versiones: %d %s", r.StatusCode, cuerpo)
	}
	pedir(t, ts, "GET", "/api/v1/agentes/versiones?forzar=1", "")
	if !slices.Equal(*forzados, []bool{false, true}) {
		t.Fatalf("forzar: %v", *forzados)
	}

	// Una API por aplicación no prepara agentes ni lo ve.
	punto, _, _ := servidorPreparacion(t, &Punto{Nombre: "app", Contratos: map[string]bool{"nativa": true}})
	for _, ruta := range []string{"GET /api/v1/agentes/claude/preparacion", "POST /api/v1/agentes/claude/preparacion", "GET /api/v1/agentes/versiones"} {
		metodo, camino, _ := strings.Cut(ruta, " ")
		if r, _ := pedir(t, punto, metodo, camino, `{"accion":"instalar"}`); r.StatusCode != 404 {
			t.Errorf("punto, %s: %d", ruta, r.StatusCode)
		}
	}
}

// TestPrepararCodigoNoSeDevuelve: lo pegado llega a la preparación, pero no vuelve en ninguna respuesta (tampoco en la de
// un código que no vale ni en un cuerpo roto) ni queda en el registro.
func TestPrepararCodigoNoSeDevuelve(t *testing.T) {
	var registro bytes.Buffer
	log.SetOutput(&registro)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	ts, f, _ := servidorPreparacion(t, nil)
	pedir(t, ts, "POST", "/api/v1/agentes/claude/preparacion", `{"accion":"iniciar_sesion"}`)
	const pegado = "abc-12345#estado-secreto"
	var respuestas []string
	for i, cuerpo := range []string{`{"codigo":"` + pegado + `"}`, `{"codigo":"corto"}`, `{"codigo":"` + pegado + `"`, `{"codigo":"` + pegado + `","x":1}`} {
		r, respuesta := pedir(t, ts, "POST", "/api/v1/agentes/claude/preparacion/codigo", cuerpo)
		respuestas = append(respuestas, respuesta)
		if quiere := map[bool]int{true: 204, false: 400}[i == 0]; r.StatusCode != quiere {
			t.Errorf("cuerpo %d: %d %s", i, r.StatusCode, respuesta)
		}
	}
	if !slices.Equal(f.pegados, []string{"claude " + pegado}) {
		t.Fatalf("lo pegado llega a la preparación: %q", f.pegados)
	}
	for _, ruta := range []string{"/api/v1/agentes/claude/preparacion", "/api/v1/agentes/versiones"} {
		_, cuerpo := pedir(t, ts, "GET", ruta, "")
		respuestas = append(respuestas, cuerpo)
	}
	for _, texto := range append(respuestas, registro.String()) {
		if strings.Contains(texto, "abc-12345") || strings.Contains(texto, "corto") || strings.Contains(texto, "estado-secreto") {
			t.Errorf("lo pegado vuelve: %s", texto)
		}
	}
}
