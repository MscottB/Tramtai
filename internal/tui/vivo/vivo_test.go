package vivo

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

type origen struct{ gen int }

func ahora(time.Duration) <-chan time.Time {
	c := make(chan time.Time, 1)
	c <- time.Time{}
	return c
}

// recoger sigue el trabajo hasta que el seguimiento se cierra y devuelve lo que llegó, como «clase:datos».
func recoger(t *testing.T, eventos Eventos, trabajo Trabajo) []string {
	t.Helper()
	canal := make(chan Msg[origen], 64)
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	go seguir(ctx, origen{gen: 3}, "global", "t1", 2, eventos, trabajo, ahora, canal)
	var llegado []string
	for m := range canal {
		if m.Origen.gen != 3 || m.ID != "t1" || m.Seq != 2 {
			t.Fatalf("cada mensaje lleva el origen, el trabajo y el seguimiento: %+v", m)
		}
		llegado = append(llegado, m.Clase+":"+m.Datos+string(m.T.Estado))
	}
	return llegado
}

// TestSeguirDescartaRepetidos: tras un corte se reabre desde el último evento visto; lo que el flujo nuevo repite (ya había
// llegado) no se vuelve a entregar, así el texto en vivo no sale duplicado.
func TestSeguirDescartaRepetidos(t *testing.T) {
	var desdes []int
	eventos := func(_ context.Context, _, _ string, desde int, al func(int, string, string) error) error {
		desdes = append(desdes, desde)
		if len(desdes) == 1 {
			_ = al(1, "texto", "uno ")
			_ = al(2, "texto", "dos ")
			return errors.New("se cortó")
		}
		for n, x := range []string{"uno ", "dos ", "tres"} { // el flujo nuevo repite lo de antes
			_ = al(n+1, "texto", x)
		}
		return nil
	}
	lecturas := 0
	trabajo := func(context.Context, string, string) (nucleo.Trabajo, error) {
		lecturas++
		if lecturas == 1 {
			return nucleo.Trabajo{ID: "t1", Estado: nucleo.Ejecutando}, nil
		}
		return nucleo.Trabajo{ID: "t1", Estado: nucleo.Completado}, nil
	}
	got := recoger(t, eventos, trabajo)
	quiere := []string{"evento:uno ", "evento:dos ", "corte:", "abre:", "evento:tres", "fin:completado"}
	if !slices.Equal(got, quiere) {
		t.Fatalf("llegó %q, quería %q", got, quiere)
	}
	if !slices.Equal(desdes, []int{0, 2}) {
		t.Fatalf("reabre desde el último evento visto: %v", desdes)
	}
}

// TestSeguirTerminaConElTrabajo: si el flujo acaba sin fin porque el trabajo terminó mientras tanto, se manda el trabajo final.
func TestSeguirTerminaConElTrabajo(t *testing.T) {
	eventos := func(_ context.Context, _, _ string, _ int, al func(int, string, string) error) error {
		_ = al(3, "estado", "completado")
		return nil // el flujo se cierra sin decirlo
	}
	trabajo := func(context.Context, string, string) (nucleo.Trabajo, error) {
		return nucleo.Trabajo{ID: "t1", Estado: nucleo.Completado}, nil
	}
	if got := recoger(t, eventos, trabajo); !slices.Equal(got, []string{"evento:completado", "fin:completado"}) {
		t.Fatalf("un evento y el fin: %q", got)
	}
}

func TestHace(t *testing.T) {
	antes := idioma.Activo
	idioma.Activo = "es"
	t.Cleanup(func() { idioma.Activo = antes })
	ahora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	casos := map[time.Duration]string{30 * time.Second: "hace 30 s", 5 * time.Minute: "hace 5 min", 3 * time.Hour: "hace 3 h", 48 * time.Hour: "hace 2 d"}
	for d, quiere := range casos {
		if got := Hace(ahora, ahora.Add(-d)); got != quiere {
			t.Errorf("Hace(%v) = %q, quería %q", d, got, quiere)
		}
	}
	if got := Hace(ahora, ahora.Add(-30*24*time.Hour)); got != ahora.Add(-30*24*time.Hour).Local().Format("2006-01-02") {
		t.Errorf("más de una semana es la fecha: %q", got)
	}
	if got := Hace(ahora, time.Time{}); got != "Sin dato" {
		t.Errorf("sin fecha: %q", got)
	}
}
