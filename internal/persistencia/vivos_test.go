package persistencia

import (
	"strings"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

// relojFijo abre un almacén cuyo reloj devuelve *ahora.
func relojFijo(t *testing.T, ahora *time.Time) *Almacen {
	t.Helper()
	a := abrir(t)
	a.reloj = func() time.Time { return *ahora }
	return a
}

func ids(trabajos []nucleo.Trabajo) map[string]bool {
	m := map[string]bool{}
	for _, t := range trabajos {
		m[t.ID] = true
	}
	return m
}

func TestVivosTraeEnCursoYRecientes(t *testing.T) {
	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a := relojFijo(t, &ahora)
	enCola := aceptar(t, a, trabajo("global", "uno"), "")
	ejecutando := aceptar(t, a, trabajo("global", "dos"), "")
	avanzar(t, a, ejecutando.ID, nucleo.PreparandoEntrada, nucleo.EsperandoAgente, nucleo.Ejecutando)
	viejo := aceptar(t, a, trabajo("global", "tres"), "")
	reciente := aceptar(t, a, trabajo("global", "cuatro"), "")
	ahora = ahora.Add(-30 * time.Second)
	avanzar(t, a, viejo.ID, nucleo.Fallido)
	ahora = ahora.Add(27 * time.Second)
	avanzar(t, a, reciente.ID, nucleo.Fallido)
	ahora = ahora.Add(3 * time.Second)

	vivos, err := a.Vivos(ctx, "global", "", ahora.Add(-10*time.Second), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(vivos)
	if len(vivos) != 3 || !got[enCola.ID] || !got[ejecutando.ID] || !got[reciente.ID] || got[viejo.ID] {
		t.Fatalf("vivos = %v", got)
	}
}

func TestVivosFiltraPorPunto(t *testing.T) {
	a := abrir(t)
	conA := trabajo("global", "a")
	conA.PuntoAcceso = "a"
	conB := trabajo("global", "b")
	conB.PuntoAcceso = "b"
	aceptar(t, a, conA, "")
	aceptar(t, a, conB, "")
	vivos, err := a.Vivos(ctx, "global", "a", time.Now().Add(-time.Minute), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(vivos) != 1 || vivos[0].PuntoAcceso != "a" {
		t.Fatalf("vivos de a = %+v", vivos)
	}
	cambiados, err := a.CambiadosDesde(ctx, "global", "b", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cambiados) != 1 || cambiados[0].PuntoAcceso != "b" {
		t.Fatalf("cambiados de b = %+v", cambiados)
	}
}

func TestCambiadosDesdeOrdenYCorte(t *testing.T) {
	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a := relojFijo(t, &ahora)
	uno := aceptar(t, a, trabajo("global", "uno"), "")
	dos := aceptar(t, a, trabajo("global", "dos"), "")
	antes := ahora
	ahora = ahora.Add(time.Second)
	avanzar(t, a, dos.ID, nucleo.PreparandoEntrada)
	cambiados, err := a.CambiadosDesde(ctx, "global", "", antes)
	if err != nil {
		t.Fatal(err)
	}
	if len(cambiados) != 1 || cambiados[0].ID != dos.ID {
		t.Fatalf("cambiados desde antes = %v (uno es %s)", ids(cambiados), uno.ID)
	}
	if cambiados, _ = a.CambiadosDesde(ctx, "global", "", ahora); len(cambiados) != 0 {
		t.Fatalf("nada cambió después: %v", ids(cambiados))
	}
}

func TestCambiarAgenteActualiza(t *testing.T) {
	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a := relojFijo(t, &ahora)
	tr := aceptar(t, a, trabajo("global", "uno"), "")
	antes := ahora
	ahora = ahora.Add(time.Second)
	if err := a.CambiarAgente(ctx, tr.ID, "codex", "", "", nil); err != nil {
		t.Fatal(err)
	}
	cambiados, err := a.CambiadosDesde(ctx, "global", "", antes)
	if err != nil {
		t.Fatal(err)
	}
	if len(cambiados) != 1 || cambiados[0].Solicitud.Agente != "codex" {
		t.Fatalf("el cambio de agente no cuenta como cambio: %+v", cambiados)
	}
}

func TestVivosUsaElIndiceDeTerminados(t *testing.T) {
	a := abrir(t)
	consulta, args := consultaPorCambio("global", "", condicionVivos, time.Now(), time.Time{})
	filas, err := a.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+consulta, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()
	plan := ""
	for filas.Next() {
		var id, padre, libre int
		var detalle string
		if err := filas.Scan(&id, &padre, &libre, &detalle); err != nil {
			t.Fatal(err)
		}
		plan += detalle + "\n"
	}
	if !strings.Contains(plan, "trabajos_instancia_terminado") {
		t.Fatalf("la foto recorre el historial de la instancia:\n%s", plan)
	}
}

func TestVivosDespuesDe(t *testing.T) {
	// Reloj fijo: en Windows dos trabajos seguidos pueden tener la misma hora y «después de uno» no los separaría.
	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a := relojFijo(t, &ahora)
	uno := aceptar(t, a, trabajo("global", "uno"), "")
	ahora = ahora.Add(time.Second)
	dos := aceptar(t, a, trabajo("global", "dos"), "")
	vivos, err := a.Vivos(ctx, "global", "", ahora, uno.ActualizadoEn)
	if err != nil {
		t.Fatal(err)
	}
	if len(vivos) != 1 || vivos[0].ID != dos.ID {
		t.Fatalf("después de uno solo queda dos: %v", ids(vivos))
	}
}

func TestContarOrigenDesde(t *testing.T) {
	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	a := relojFijo(t, &ahora)
	nuevo := func(origen, punto string) {
		tr := trabajo("global", "hola")
		tr.Origen, tr.PuntoAcceso = origen, punto
		aceptar(t, a, tr, "")
	}
	ahora = ahora.Add(-48 * time.Hour)
	for range 3 {
		nuevo("app", "")
	}
	ahora = ahora.Add(47 * time.Hour)
	for range 21 {
		nuevo("app", "")
	}
	nuevo("app", "asistente")
	nuevo("probar", "")
	ahora = ahora.Add(time.Hour)
	n, err := a.ContarOrigenDesde(ctx, "global", "app", true, ahora.Add(-24*time.Hour))
	if err != nil || n != 21 {
		t.Fatalf("peticiones de apps con la clave del espacio en 24 h: %d %v", n, err)
	}
}
