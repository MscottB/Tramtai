package adaptadores

import (
	"context"
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

// Un CLI colgado no deja la resolución viva para siempre, y una consulta que llega mientras otra está en curso se une a
// ella en lugar de lanzar otra.
func TestModeloConcretoConPlazoYCompartido(t *testing.T) {
	anterior := plazoResolver
	plazoResolver = 500 * time.Millisecond
	t.Cleanup(func() { plazoResolver = anterior })
	a := colgado{llamadas: &atomic.Int32{}}
	inicio := time.Now()
	resultados := make(chan string, 2)
	consultar := func() { resultados <- ModeloConcreto(context.WithoutCancel(context.Background()), a, "colgado", "x") }
	go consultar()
	for a.llamadas.Load() == 0 { // la segunda llega con la primera ya en curso
		time.Sleep(time.Millisecond)
	}
	go consultar()
	for range 2 {
		if m := <-resultados; m != "" {
			t.Errorf("modelo %q de un CLI que no contesta", m)
		}
	}
	if time.Since(inicio) > 5*time.Second {
		t.Fatal("la resolución no respetó su plazo")
	}
	if n := a.llamadas.Load(); n != 1 {
		t.Fatalf("dos consultas a la vez lanzaron %d resoluciones", n)
	}
}
