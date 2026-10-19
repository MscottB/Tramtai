package adaptadores

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

// colgado es un CLI que nunca contesta a ResolverModelo: solo vuelve cuando vence su plazo.
type colgado struct{ llamadas *atomic.Int32 }

func (colgado) Ejecutar(context.Context, nucleo.SolicitudEjecucion, func(Evento) error) (Resultado, error) {
	return Resultado{}, nil
}

func (c colgado) ResolverModelo(ctx context.Context, _ string) (string, error) {
	c.llamadas.Add(1)
	<-ctx.Done()
	return "", ctx.Err()
}

// Un CLI colgado no deja la resolución viva para siempre, y dos consultas a la vez lanzan una sola.
func TestModeloConcretoConPlazoYCompartido(t *testing.T) {
	anterior := plazoResolver
	plazoResolver = 50 * time.Millisecond
	t.Cleanup(func() { plazoResolver = anterior })
	a := colgado{llamadas: &atomic.Int32{}}
	inicio := time.Now()
	var grupo sync.WaitGroup
	for range 2 {
		grupo.Go(func() {
			if m := ModeloConcreto(context.WithoutCancel(context.Background()), a, "colgado", "x"); m != "" {
				t.Errorf("modelo %q de un CLI que no contesta", m)
			}
		})
	}
	grupo.Wait()
	if time.Since(inicio) > 5*time.Second {
		t.Fatal("la resolución no respetó su plazo")
	}
	if n := a.llamadas.Load(); n != 1 {
		t.Fatalf("dos consultas a la vez lanzaron %d resoluciones", n)
	}
}
