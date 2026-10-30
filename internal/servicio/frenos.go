package servicio

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"tramtai/internal/api"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// cuenta son los fallos de una dirección desde «desde» y, si llegó al tope, hasta cuándo espera.
type cuenta struct {
	desde, hasta time.Time
	fallos       int
}

// contador cuenta fallos por dirección en ventanas fijas: al llegar a max dentro de la ventana, la dirección espera la
// pausa y vuelve a empezar de cero.
type contador struct {
	mu      sync.Mutex
	max     int
	ventana time.Duration
	pausa   time.Duration
	cuentas map[netip.Addr]*cuenta
	// saturadoHasta: con maximoCuentas direcciones a la vez, las que no tienen cuenta esperan la pausa (no se dejan de
	// frenar por falta de sitio).
	saturadoHasta time.Time
}

// maximoCuentas acota la memoria de cada contador.
const maximoCuentas = 10000

func nuevoContador(max int, ventana, pausa time.Duration) *contador {
	return &contador{max: max, ventana: ventana, pausa: pausa, cuentas: map[netip.Addr]*cuenta{}}
}

// red es la dirección que cuenta: en IPv6, su red /64, porque quien la tiene cambia de dirección dentro sin coste.
func red(ip netip.Addr) netip.Addr {
	if ip.Is6() {
		return netip.PrefixFrom(ip, 64).Masked().Addr()
	}
	return ip
}

// espera es lo que le queda a la dirección para volver a intentarlo (0 si puede); con el contador saturado, también
// esperan las direcciones sin cuenta.
func (c *contador) espera(ip netip.Addr, ahora time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := c.cuentas[red(ip)]
	switch {
	case k != nil && ahora.Before(k.hasta):
		return k.hasta.Sub(ahora)
	case k == nil && ahora.Before(c.saturadoHasta):
		return c.saturadoHasta.Sub(ahora)
	}
	return 0
}

// esperaPropia es la espera de la cuenta de la dirección, sin contar la saturación.
func (c *contador) esperaPropia(ip netip.Addr, ahora time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if k := c.cuentas[red(ip)]; k != nil && ahora.Before(k.hasta) {
		return k.hasta.Sub(ahora)
	}
	return 0
}

// fallo anota un fallo de la dirección y dice si con él empieza a esperar.
func (c *contador) fallo(ip netip.Addr, ahora time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ip = red(ip)
	k := c.cuentas[ip]
	if k == nil {
		if len(c.cuentas) >= maximoCuentas {
			c.purgar(ahora)
		}
		if len(c.cuentas) >= maximoCuentas { // sigue lleno: las direcciones nuevas esperan la pausa
			c.saturadoHasta = ahora.Add(c.pausa)
			return true
		}
		k = &cuenta{desde: ahora}
		c.cuentas[ip] = k
	}
	if ahora.Sub(k.desde) > c.ventana {
		k.desde, k.fallos = ahora, 0
	}
	k.fallos++
	if k.fallos < c.max {
		return false
	}
	k.hasta = ahora.Add(c.pausa)
	k.desde, k.fallos = k.hasta, 0 // la ventana siguiente empieza al acabar la pausa
	return true
}

// olvidar borra la cuenta de la dirección (tras un acierto).
func (c *contador) olvidar(ip netip.Addr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cuentas, red(ip))
}

// purgar quita las cuentas sin pausa en curso y con la ventana pasada; se llama con mu tomado.
func (c *contador) purgar(ahora time.Time) {
	for ip, k := range c.cuentas {
		if !ahora.Before(k.hasta) && ahora.Sub(k.desde) > c.ventana {
			delete(c.cuentas, ip)
		}
	}
}

const (
	// maximoEnCola: con más peticiones esperando un cálculo de argon2id, la entrada responde 503.
	maximoEnCola = 32
	// maximoConocidas y vigenciaConocida: las direcciones que entraron bien hace menos de 30 días (como mucho 256, las
	// más antiguas se descartan) no esperan al freno global.
	maximoConocidas  = 256
	vigenciaConocida = 30 * 24 * time.Hour
)

// frenos protegen la entrada con contraseña: 5 fallos por dirección dan 15 minutos de espera; 100 fallos en 10 minutos,
// de las que sean, dan 5 minutos de espera a todas (salvo a las que entraron bien hace poco, para que un ataque
// repartido no deje fuera al dueño); y como mucho 2 cálculos de argon2id a la vez, con 32 en cola, para que probar
// contraseñas no agote el servicio.
type frenos struct {
	porIP, global *contador
	argon         chan struct{}
	enCola        atomic.Int32

	muConocidas sync.Mutex
	conocidas   map[netip.Addr]time.Time // red de la dirección → último acceso bueno
}

func nuevosFrenos() *frenos {
	return &frenos{porIP: nuevoContador(5, 15*time.Minute, 15*time.Minute), global: nuevoContador(100, 10*time.Minute, 5*time.Minute),
		argon: make(chan struct{}, 2), conocidas: map[netip.Addr]time.Time{}}
}

// conocida dice si la dirección entró bien en los últimos 30 días.
func (f *frenos) conocida(ip netip.Addr, ahora time.Time) bool {
	f.muConocidas.Lock()
	defer f.muConocidas.Unlock()
	ultimo, ok := f.conocidas[red(ip)]
	return ok && ahora.Sub(ultimo) < vigenciaConocida
}

// Permitido es lo que tiene que esperar la dirección (la suya o la de todos), 0 si puede intentarlo.
func (f *frenos) Permitido(ip netip.Addr, ahora time.Time) (espera time.Duration) {
	if f.conocida(ip, ahora) {
		return f.porIP.esperaPropia(ip, ahora)
	}
	return max(f.porIP.espera(ip, ahora), f.global.espera(netip.Addr{}, ahora))
}

// Fallo anota un intento fallido y dice si con él la dirección, o todas, empiezan a esperar.
func (f *frenos) Fallo(ip netip.Addr, ahora time.Time) bool {
	porIP := f.porIP.fallo(ip, ahora)
	return f.global.fallo(netip.Addr{}, ahora) || porIP
}

// Exito borra la cuenta de la dirección y la recuerda como conocida.
func (f *frenos) Exito(ip netip.Addr, ahora time.Time) {
	f.porIP.olvidar(ip)
	f.muConocidas.Lock()
	defer f.muConocidas.Unlock()
	f.conocidas[red(ip)] = ahora
	if len(f.conocidas) <= maximoConocidas {
		return
	}
	var antigua netip.Addr
	var cuando time.Time
	for d, t := range f.conocidas {
		if cuando.IsZero() || t.Before(cuando) {
			antigua, cuando = d, t
		}
	}
	delete(f.conocidas, antigua)
}

// tomar espera un hueco para calcular argon2id; ocupado si ya hay maximoEnCola esperando, y ok false si la petición se
// cancela antes. Con ok, hay que llamar a soltar.
func (f *frenos) tomar(ctx context.Context) (ocupado, ok bool) {
	if f.enCola.Add(1) > maximoEnCola {
		f.enCola.Add(-1)
		return true, false
	}
	defer f.enCola.Add(-1)
	select {
	case f.argon <- struct{}{}:
		return false, true
	case <-ctx.Done():
		return false, false
	}
}

func (f *frenos) soltar() { <-f.argon }

// demasiados responde 429 con Retry-After en segundos.
func demasiados(w http.ResponseWriter, r *http.Request, espera time.Duration, mensaje string) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
	api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoDemasiadosIntentos, mensaje))
}

// frenarClaves rechaza con 429 lo que llega con clave desde una dirección frenada por claves falsas (30 en un minuto dan
// un minuto de espera), antes de mirar si la clave vale: así probar claves no da pistas mientras dure la espera. Lo que
// llega por loopback sin proxy no se frena: un proceso del equipo no puede dejar a la CLI y al escritorio sin servicio.
// Con el contador lleno falla abierto (esperaPropia): las claves son de 256 bits y el freno es contra el abuso; cerrar
// dejaría sin servicio a las apps con su clave.
func (s *Servicio) frenarClaves(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if api.TokenDe(r) != "" && !desdeEsteEquipo(r) {
			if espera := s.claves.esperaPropia(ClienteReal(r, s.opc.Confiables), time.Now()); espera > 0 {
				demasiados(w, r, espera, idioma.T("demasiadas claves no válidas desde tu dirección: espera un minuto y revisa la clave"))
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// claveFalsa anota una clave que no vale, si la petición traía una y no llega de este equipo; el bloqueo queda en el
// registro.
func (s *Servicio) claveFalsa(r *http.Request) {
	if api.TokenDe(r) == "" || desdeEsteEquipo(r) {
		return
	}
	ip := ClienteReal(r, s.opc.Confiables)
	if s.claves.fallo(ip, time.Now()) {
		fmt.Fprintf(s.opc.Registros, "claves: %s queda frenada un minuto por claves no válidas\n", ip)
	}
}

// noAutorizado responde 401 y cuenta la clave falsa, si la había.
func (s *Servicio) noAutorizado(w http.ResponseWriter, r *http.Request) {
	s.claveFalsa(r)
	api.NoAutorizado(w, r)
}
