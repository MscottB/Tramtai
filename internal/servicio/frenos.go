package servicio

import (
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
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
}

// maximoCuentas acota la memoria: con tantas direcciones a la vez ya no se abren cuentas nuevas (el freno global de la
// entrada sigue contando).
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

// espera es lo que le queda a la dirección para volver a intentarlo (0 si puede).
func (c *contador) espera(ip netip.Addr, ahora time.Time) time.Duration {
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
			if len(c.cuentas) >= maximoCuentas {
				return false
			}
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

// frenos protegen la entrada con contraseña: 5 fallos por dirección dan 15 minutos de espera; 100 fallos en 10 minutos,
// de las que sean, dan 5 minutos de espera a todas; y como mucho 2 cálculos de argon2id a la vez, para que probar
// contraseñas no agote el servicio.
type frenos struct {
	porIP, global *contador
	argon         chan struct{}
}

func nuevosFrenos() *frenos {
	return &frenos{porIP: nuevoContador(5, 15*time.Minute, 15*time.Minute), global: nuevoContador(100, 10*time.Minute, 5*time.Minute),
		argon: make(chan struct{}, 2)}
}

// Permitido es lo que tiene que esperar la dirección (la suya o la de todos), 0 si puede intentarlo.
func (f *frenos) Permitido(ip netip.Addr, ahora time.Time) (espera time.Duration) {
	return max(f.porIP.espera(ip, ahora), f.global.espera(netip.Addr{}, ahora))
}

// Fallo anota un intento fallido y dice si con él la dirección, o todas, empiezan a esperar.
func (f *frenos) Fallo(ip netip.Addr, ahora time.Time) bool {
	porIP := f.porIP.fallo(ip, ahora)
	return f.global.fallo(netip.Addr{}, ahora) || porIP
}

// Exito borra la cuenta de la dirección.
func (f *frenos) Exito(ip netip.Addr) { f.porIP.olvidar(ip) }

// conArgon calcula con argon2id cuando hay sitio (como mucho 2 a la vez); si la petición se cancela antes, no calcula y
// devuelve false.
func (f *frenos) conArgon(r *http.Request, calcular func()) bool {
	select {
	case f.argon <- struct{}{}:
		defer func() { <-f.argon }()
		calcular()
		return true
	case <-r.Context().Done():
		return false
	}
}

// demasiados responde 429 con Retry-After en segundos.
func demasiados(w http.ResponseWriter, r *http.Request, espera time.Duration, mensaje string) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
	api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoDemasiadosIntentos, mensaje))
}

// frenarClaves rechaza con 429 lo que llega con clave desde una dirección frenada por claves falsas (30 en un minuto dan
// un minuto de espera), antes de mirar si la clave vale: así probar claves no da pistas mientras dure la espera.
func (s *Servicio) frenarClaves(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if api.TokenDe(r) != "" {
			if espera := s.claves.espera(ClienteReal(r, s.opc.Confiables), time.Now()); espera > 0 {
				demasiados(w, r, espera, idioma.T("demasiadas claves no válidas desde tu dirección: espera un minuto y revisa la clave"))
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// claveFalsa anota una clave que no vale, si la petición traía una; el bloqueo queda en el registro.
func (s *Servicio) claveFalsa(r *http.Request) {
	if api.TokenDe(r) == "" {
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
