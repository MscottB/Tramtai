package prueba

import (
	"slices"
	"sync"
	"time"
)

// Reloj es la hora y las esperas de una página en una prueba: no pasa el tiempo hasta que la prueba lo hace pasar con
// Pasar. Así un temporizador (el sondeo, la reconexión, la limpieza de la red) salta cuando la prueba quiere.
type Reloj struct {
	mu      sync.Mutex
	ahora   time.Time
	esperas []espera
	pedidas []time.Duration
}

type espera struct {
	hasta time.Time
	c     chan time.Time
}

// NuevoReloj empieza a la hora de ahora: las peticiones que la prueba creó antes en el servicio son de hace poco, pero no
// siempre de «hace 0 s» (en un runner lento pasa un segundo o más entre crearlas y crear el reloj).
func NuevoReloj() *Reloj { return &Reloj{ahora: time.Now()} }

// Ahora es la hora del reloj.
func (r *Reloj) Ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ahora
}

// Espera es time.After con la hora del reloj: el canal recibe cuando Pasar llega a d desde ahora.
func (r *Reloj) Espera(d time.Duration) <-chan time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := make(chan time.Time, 1)
	r.pedidas = append(r.pedidas, d)
	if d <= 0 {
		c <- r.ahora
		return c
	}
	r.esperas = append(r.esperas, espera{hasta: r.ahora.Add(d), c: c})
	return c
}

// Pasar adelanta el reloj y suelta las esperas que vencen.
func (r *Reloj) Pasar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ahora = r.ahora.Add(d)
	quedan := r.esperas[:0]
	for _, e := range r.esperas {
		if e.hasta.After(r.ahora) {
			quedan = append(quedan, e)
			continue
		}
		e.c <- r.ahora
	}
	r.esperas = quedan
}

// Pedidas son las esperas que se le han pedido, en orden.
func (r *Reloj) Pedidas() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.pedidas)
}
