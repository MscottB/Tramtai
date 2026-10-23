package inicio

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
)

// Las pruebas de vivo.go son las de web/src/vivo.test.ts traducidas: el reloj es el `ahora` que se les pasa.

var t0 = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func en(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }

func ev(cambios ...func(*contrato.EventoTrabajo)) contrato.EventoTrabajo {
	e := contrato.EventoTrabajo{ID: "a", Estado: "en_cola", Etapa: "cola", Agente: "claude", Modelo: "claude/sonnet", Origen: "probar", Intento: 1}
	for _, c := range cambios {
		c(&e)
	}
	return e
}

func con(id, estado, etapa string) func(*contrato.EventoTrabajo) {
	return func(e *contrato.EventoTrabajo) { e.ID, e.Estado, e.Etapa = id, estado, etapa }
}

func claves(t tranvias) []string {
	var l []string
	for id := range t {
		l = append(l, id)
	}
	slices.Sort(l)
	return l
}

func TestVivoAltaYCambioDeEtapa(t *testing.T) {
	m, borrados := tranvias{}, map[string]bool{}
	aplicar(m, ev(), en(1000), borrados)
	for _, paso := range []struct{ estado, etapa string }{{"preparando_entrada", "preparacion"}, {"ejecutando", "ejecucion"}, {"validando_salida", "validacion"}} {
		aplicar(m, ev(con("a", paso.estado, paso.etapa)), en(1100), borrados)
		if m["a"].Etapa != paso.etapa || m["a"].Estado != paso.estado {
			t.Fatalf("etapa %q: %+v", paso.etapa, m["a"])
		}
	}
	aplicar(m, ev(con("a", "completado", "llegada")), en(1400), borrados)
	if !m["a"].Desde.Equal(en(1000)) || !m["a"].Terminado.Equal(en(1400)) || m["a"].Etapa != "llegada" {
		t.Fatalf("desde es la primera vez y terminado cuándo llegó: %+v", m["a"])
	}
	aplicar(m, ev(con("a", "completado", "llegada")), en(9000), borrados)
	if !m["a"].Terminado.Equal(en(1400)) {
		t.Fatalf("un evento repetido no retrasa el borrado: %+v", m["a"])
	}
}

func TestVivoElRespaldoCambiaDeAgente(t *testing.T) {
	m := tranvias{}
	aplicar(m, ev(con("a", "ejecutando", "ejecucion")), en(0), nil)
	aplicar(m, ev(con("a", "esperando_agente", "preparacion"), func(e *contrato.EventoTrabajo) { e.Agente, e.Intento = "codex", 2 }), en(10), nil)
	if m["a"].Agente != "codex" || m["a"].Intento != 2 || m["a"].Etapa != "preparacion" {
		t.Fatalf("%+v", m["a"])
	}
}

func TestVivoUnFalloConservaSuEstado(t *testing.T) {
	m := tranvias{}
	aplicar(m, ev(con("a", "ejecutando", "ejecucion")), en(0), nil)
	aplicar(m, ev(con("a", "fallido", "llegada"), func(e *contrato.EventoTrabajo) { e.CodigoError = "error_agente" }), en(10), nil)
	if m["a"].Estado != "fallido" || m["a"].CodigoError != "error_agente" || claveEtapa(m["a"]) != "vivo.etapaFallida" {
		t.Fatalf("%+v", m["a"])
	}
}

func TestVivoLimpiarBorraLlegadasA2sYFallosA4s(t *testing.T) {
	m, borrados := tranvias{}, map[string]bool{}
	aplicar(m, ev(con("ok", "completado", "llegada")), en(0), borrados)
	aplicar(m, ev(con("mal", "fallido", "llegada")), en(0), borrados)
	aplicar(m, ev(con("vivo", "ejecutando", "ejecucion")), en(0), borrados)
	limpiar(m, en(1999), borrados)
	if len(m) != 3 {
		t.Fatalf("a los 1999 ms siguen los tres: %v", claves(m))
	}
	limpiar(m, en(2000), borrados)
	if got := claves(m); !slices.Equal(got, []string{"mal", "vivo"}) {
		t.Fatalf("a los 2 s se va el que llegó: %v", got)
	}
	limpiar(m, en(3999), borrados)
	if _, hay := m["mal"]; !hay {
		t.Fatal("el fallo sigue a los 3999 ms")
	}
	limpiar(m, en(4000), borrados)
	if got := claves(m); !slices.Equal(got, []string{"vivo"}) || !borrados["ok"] || !borrados["mal"] || borrados["vivo"] {
		t.Fatalf("a los 4 s se va el fallo: %v %v", got, borrados)
	}
}

func TestVivoLimpiarCuentaCancelacionesComoLlegadas(t *testing.T) {
	m, borrados := tranvias{}, map[string]bool{}
	aplicar(m, ev(con("c", "cancelado", "llegada")), en(0), borrados)
	aplicar(m, ev(con("i", "interrumpido", "llegada")), en(0), borrados)
	limpiar(m, en(2000), borrados)
	if got := claves(m); !slices.Equal(got, []string{"i"}) {
		t.Fatalf("cancelado 2 s, interrumpido 4 s: %v", got)
	}
}

func TestVivoUnEventoTardioDeUnoBorradoNoVuelve(t *testing.T) {
	m := tranvias{}
	aplicar(m, ev(con("ok", "completado", "llegada")), en(5000), map[string]bool{"ok": true})
	if len(m) != 0 {
		t.Fatalf("no vuelve: %v", m)
	}
}

func TestVivoComoMucho12LosMasRecientesYElResto(t *testing.T) {
	m := tranvias{}
	for i := 0; i < 15; i++ {
		aplicar(m, ev(con("t"+strconv.Itoa(i), "en_cola", "cola")), en(i), nil)
	}
	lista, resto := visibles(m, maximoVisibles)
	if len(lista) != 12 || resto != 3 || lista[0].ID != "t14" {
		t.Fatalf("%d visibles, resto %d, primero %s", len(lista), resto, lista[0].ID)
	}
	for _, tr := range lista {
		if tr.ID == "t0" {
			t.Fatal("el más viejo queda fuera")
		}
	}
}

func TestVivoEsperasDeReconexion(t *testing.T) {
	var got []time.Duration
	for i := 0; i < 8; i++ {
		got = append(got, esperaReconexion(i))
	}
	s := time.Second
	if !slices.Equal(got, []time.Duration{1 * s, 2 * s, 4 * s, 8 * s, 16 * s, 30 * s, 30 * s, 30 * s}) {
		t.Fatalf("%v", got)
	}
	if esperaReconexion(1000) != 30*s {
		t.Fatal("un intento enorme no desborda")
	}
}

func TestVivoDesdeEsCuandoSeAceptoLaPeticion(t *testing.T) {
	m := tranvias{}
	creado := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	aplicar(m, ev(con("a", "ejecutando", "ejecucion"), func(e *contrato.EventoTrabajo) { e.Creado = creado }), creado.Add(2*time.Minute), nil)
	if !m["a"].Desde.Equal(creado) {
		t.Fatalf("desde %v", m["a"].Desde)
	}
}

func TestVivoPodarQuitaLosQueLaFotoNoTrajoYNoHabianLlegado(t *testing.T) {
	m, borrados := tranvias{}, map[string]bool{}
	aplicar(m, ev(con("sigue", "ejecutando", "ejecucion")), en(0), borrados)
	aplicar(m, ev(con("perdido", "ejecutando", "ejecucion")), en(0), borrados)
	aplicar(m, ev(con("llegando", "completado", "llegada")), en(0), borrados)
	podar(m, map[string]bool{"sigue": true})
	if got := claves(m); !slices.Equal(got, []string{"llegando", "sigue"}) {
		t.Fatalf("%v", got)
	}
}

func TestVivoReconciliarElQueTerminaLlegaElQueFaltaSeVa(t *testing.T) {
	m, borrados := tranvias{}, map[string]bool{}
	aplicar(m, ev(con("termina", "ejecutando", "ejecucion")), en(0), borrados)
	aplicar(m, ev(con("falta", "ejecutando", "ejecucion")), en(0), borrados)
	reconciliar(m, []contrato.EventoTrabajo{ev(con("termina", "completado", "llegada")), ev(con("viejo", "completado", "llegada")), ev(con("nuevo", "en_cola", "cola"))}, en(100), borrados)
	if got := claves(m); !slices.Equal(got, []string{"nuevo", "termina"}) || !m["termina"].Terminado.Equal(en(100)) {
		t.Fatalf("%v %+v", got, m["termina"])
	}
	limpiar(m, en(2100), borrados)
	if got := claves(m); !slices.Equal(got, []string{"nuevo"}) {
		t.Fatalf("%v", got)
	}
}

func TestVivoEventoDeConvierteUnTrabajoDeLaConsulta(t *testing.T) {
	creado := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	trabajo := nucleo.Trabajo{ID: "x", Estado: "fallido", ModeloEfectivo: "codex/gpt", PuntoAcceso: "otra", Origen: "api", Intento: 2, CodigoError: "error_agente", CreadoEn: creado}
	trabajo.Solicitud.Agente = "codex"
	quiere := contrato.EventoTrabajo{ID: "x", Estado: "fallido", Etapa: "llegada", Agente: "codex", Modelo: "codex/gpt", API: "otra", Origen: "api", Intento: 2, CodigoError: "error_agente", Creado: creado}
	if got := eventoDe(trabajo); got != quiere {
		t.Fatalf("%+v", got)
	}
}

func TestVivoLaLlegadaSeNombraSegunComoTermino(t *testing.T) {
	m := tranvias{}
	for _, estado := range []string{"completado", "fallido", "interrumpido", "cancelado"} {
		aplicar(m, ev(con(estado, estado, "llegada")), en(0), nil)
	}
	aplicar(m, ev(con("va", "ejecutando", "ejecucion")), en(0), nil)
	var got []string
	for _, id := range []string{"completado", "fallido", "interrumpido", "cancelado", "va"} {
		got = append(got, claveEtapa(m[id]))
	}
	if !slices.Equal(got, []string{"vivo.etapaLlegada", "vivo.etapaFallida", "vivo.etapaFallida", "vivo.etapaCancelada", "vivo.etapaEjecucion"}) {
		t.Fatalf("%v", got)
	}
}
