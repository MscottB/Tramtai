// Package vivo es lo que comparten las páginas que siguen peticiones en vivo: Probar y el detalle del Historial siguen una
// (el flujo de eventos con su reconexión) e Inicio sostiene el flujo del espacio con el mismo bucle (Mantener). Las tres
// lo leen con Esperar y pintan el estado igual (icono, nombre, ruta y «hace cuánto»).
package vivo

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/nucleo"
)

// EsperaCorte es lo que se espera antes de reabrir el flujo si se corta.
const EsperaCorte = 2 * time.Second

// Msg es lo que llega del seguimiento de un trabajo: un evento (Clase "evento", con Tipo y Datos), un corte del flujo
// ("corte"), el intento de reabrirlo ("abre") o el final con el trabajo terminado ("fin", con T). Origen es el de la página
// que lo abrió: cada página instancia Msg con su propio tipo de origen, así no toma como suyo el de otra.
type Msg[O any] struct {
	Origen      O
	ID, Clase   string
	Seq         int // de qué seguimiento es: uno viejo (se reabrió) no vale
	Tipo, Datos string
	T           nucleo.Trabajo
}

// Flujo es el seguimiento recién abierto; la página lee su Canal de uno en uno con Esperar.
type Flujo[O any] struct {
	Origen O
	ID     string
	Seq    int
	Canal  <-chan Msg[O]
}

// Eventos lee el flujo SSE de un trabajo desde el evento desde (cliente.Eventos).
type Eventos func(ctx context.Context, instancia, id string, desde int, alEvento func(numero int, tipo, datos string) error) error

// Trabajo lee un trabajo (cliente.Trabajo).
type Trabajo func(ctx context.Context, instancia, id string) (nucleo.Trabajo, error)

// Abrir sigue el trabajo id en una goroutine y devuelve el Cmd que entrega su Flujo. El seguimiento se corta con ctx y
// cierra su canal al terminar.
func Abrir[O any](ctx context.Context, o O, instancia, id string, seq int, eventos Eventos, trabajo Trabajo,
	espera func(time.Duration) <-chan time.Time) tea.Cmd {
	return func() tea.Msg {
		canal := make(chan Msg[O], 64)
		go seguir(ctx, o, instancia, id, seq, eventos, trabajo, espera, canal)
		return Flujo[O]{Origen: o, ID: id, Seq: seq, Canal: canal}
	}
}

// Mantener sostiene un flujo que se corta y se reabre, y cierra canal al terminar. Cada vuelta abre el flujo y lo lee hasta
// que se corta; devuelve cuánto esperar antes de reabrirlo o, con seguir falso, que se acabó. enviar manda un mensaje
// por canal y es falso si ctx se canceló. Lo usan el seguimiento de un trabajo y el flujo del espacio de Inicio.
func Mantener[M any](ctx context.Context, canal chan<- M, espera func(time.Duration) <-chan time.Time,
	vuelta func(intento int, enviar func(M) bool) (pausa time.Duration, seguir bool)) {
	defer close(canal)
	enviar := func(m M) bool {
		select {
		case canal <- m:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for intento := 0; ; intento++ {
		pausa, seguir := vuelta(intento, enviar)
		if !seguir {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-espera(pausa):
		}
	}
}

// seguir mantiene abierto el flujo del trabajo. Al acabar (el cliente no entrega el evento fin) o cortarse, consulta el
// trabajo: si terminó manda fin con él; si no, espera y reabre desde el último evento visto.
func seguir[O any](ctx context.Context, o O, instancia, id string, seq int, eventos Eventos, trabajo Trabajo,
	espera func(time.Duration) <-chan time.Time, canal chan<- Msg[O]) {
	desde := 0
	Mantener(ctx, canal, espera, func(intento int, enviar func(Msg[O]) bool) (time.Duration, bool) {
		mandar := func(m Msg[O]) bool {
			m.Origen, m.ID, m.Seq = o, id, seq
			return enviar(m)
		}
		if intento > 0 && !mandar(Msg[O]{Clase: "abre"}) { // vuelve a probar: si falla enseguida, llega otro corte
			return 0, false
		}
		_ = eventos(ctx, instancia, id, desde, func(numero int, tipo, datos string) error {
			if numero > 0 && numero <= desde { // tras reconectar, lo que ya había llegado
				return nil
			}
			desde = max(desde, numero)
			if !mandar(Msg[O]{Clase: "evento", Tipo: tipo, Datos: datos}) {
				return ctx.Err()
			}
			return nil
		})
		if ctx.Err() != nil {
			return 0, false
		}
		if t, err := trabajo(ctx, instancia, id); err == nil && t.Estado.Terminal() {
			mandar(Msg[O]{Clase: "fin", T: t})
			return 0, false
		}
		return EsperaCorte, mandar(Msg[O]{Clase: "corte"})
	})
}

// Esperar lee el siguiente mensaje de un canal de seguimiento; con el canal cerrado, nada.
func Esperar[M any](canal <-chan M) tea.Cmd {
	return func() tea.Msg {
		if m, ok := <-canal; ok {
			return m
		}
		return nil
	}
}
