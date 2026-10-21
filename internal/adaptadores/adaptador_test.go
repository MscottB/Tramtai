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

func TestConexiones(t *testing.T) {
	c := nucleo.Resolver(nucleo.Capa{Origen: nucleo.OrigenGlobal, Valores: map[string]any{"adaptadores.codex.desconectado": true}})
	con := NuevasConexiones(c, []string{"claude", "codex"})
	if !con.Desconectado("codex") || con.Desconectado("claude") {
		t.Fatal("codex viene desconectado de la configuración")
	}
	con.Fijar("codex", false)
	con.Fijar("claude", true)
	if con.Desconectado("codex") || !con.Desconectado("claude") {
		t.Fatal("Fijar cambia la conexión en marcha")
	}
	var nada *Conexiones
	if nada.Desconectado("claude") {
		t.Fatal("sin conexiones, todos conectados")
	}
}

func TestSaludPasiva(t *testing.T) {
	s := NuevaSalud()
	s.Fallo("claude")
	if !s.SinRespuesta("claude") || s.SinRespuesta("codex") {
		t.Fatal("un fallo de autenticación deja al agente sin respuesta")
	}
	s.Exito("claude")
	if s.SinRespuesta("claude") {
		t.Fatal("una petición que funciona lo quita")
	}
	var nada *Salud
	nada.Fallo("claude")
	if nada.SinRespuesta("claude") {
		t.Fatal("sin salud no se marca nada")
	}
}
