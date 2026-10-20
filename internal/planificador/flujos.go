package planificador

import (
	"context"
	"sync"
	"time"

	"tramtai/internal/nucleo"
)

// MaximoFlujos son los flujos de eventos de la instancia abiertos a la vez (GET /api/v1/eventos): cada uno lee la
// base tras cada cambio, así que se limitan.
const MaximoFlujos = 8

// ReservarFlujo toma un cupo de flujo de eventos de la instancia; ok es falso si ya hay MaximoFlujos abiertos.
// liberar devuelve el cupo y se puede llamar más de una vez.
func (p *Planificador) ReservarFlujo() (liberar func(), ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.flujos >= MaximoFlujos {
		return func() {}, false
	}
	p.flujos++
	var una sync.Once
	return func() {
		una.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.flujos--
		})
	}, true
}

// Vivos son los trabajos de la instancia sin terminar y los terminados desde «desde» (ver Almacen.Vivos).
func (p *Planificador) Vivos(ctx context.Context, punto string, desde, despues time.Time) ([]nucleo.Trabajo, error) {
	return p.almacen.Vivos(ctx, p.opc.Instancia, punto, desde, despues)
}

// CambiadosDesde son los trabajos de la instancia que cambiaron después de «desde» (ver Almacen.CambiadosDesde).
func (p *Planificador) CambiadosDesde(ctx context.Context, punto string, desde time.Time) ([]nucleo.Trabajo, error) {
	return p.almacen.CambiadosDesde(ctx, p.opc.Instancia, punto, desde)
}
