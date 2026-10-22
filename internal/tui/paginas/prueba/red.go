package prueba

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
)

// Las conexiones con los servidores de las pruebas cuentan lo que cada lado escribe y lo que el otro lee. Lo enviado
// por TCP y aún no leído no está en ninguna goroutine: sin contarlo, Asentar daría por quieta una página a la que aún
// le llega algo (una respuesta, un evento del flujo).

var (
	servidores sync.Map // las direcciones de los servidores de prueba

	muPares sync.Mutex
	pares   = map[string]*par{} // por la dirección del lado del cliente
	cambios atomic.Int64        // sube con cada lectura, escritura o cierre contado
)

// par son los dos lados de una conexión: 0 el cliente y 1 el servidor.
type par struct {
	escrito, leido [2]int64
	cerrado        [2]bool
}

type conContada struct {
	net.Conn
	clave string
	lado  int
	una   sync.Once
}

func (c *conContada) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		anotar(c.clave, func(p *par) { p.leido[c.lado] += int64(n) })
	}
	return n, err
}

func (c *conContada) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		anotar(c.clave, func(p *par) { p.escrito[c.lado] += int64(n) })
	}
	return n, err
}

func (c *conContada) Close() error {
	c.una.Do(func() { anotar(c.clave, func(p *par) { p.cerrado[c.lado] = true }) })
	return c.Conn.Close()
}

func anotar(clave string, f func(*par)) {
	muPares.Lock()
	p := pares[clave]
	if p == nil {
		p = &par{}
		pares[clave] = p
	}
	f(p)
	if p.cerrado[0] && p.cerrado[1] {
		delete(pares, clave)
	}
	muPares.Unlock()
	cambios.Add(1)
}

// sinLeer dice si una conexión se está abriendo o si un lado escribió algo que el otro, aún abierto, no ha leído.
func sinLeer() bool {
	if marcando.Load() > 0 {
		return true
	}
	muPares.Lock()
	defer muPares.Unlock()
	for _, p := range pares {
		for lado, otro := range []int{1, 0} {
			if !p.cerrado[otro] && p.escrito[lado] != p.leido[otro] {
				return true
			}
		}
	}
	return false
}

// marcando son las conexiones que se están abriendo: mientras tanto ningún lado ha escrito nada y las dos goroutines
// esperan a la red.
var marcando atomic.Int64

// marcar es la conexión del cliente: con un servidor de prueba, contada; con uno ya cerrado, rechazada al momento (Windows
// tarda unos segundos en rechazar una conexión a un puerto cerrado).
func marcar(ctx context.Context, red, direccion string) (net.Conn, error) {
	abierto, deUnaPrueba := servidores.Load(direccion)
	if deUnaPrueba && !abierto.(bool) {
		return nil, &net.OpError{Op: "dial", Net: red, Err: syscall.ECONNREFUSED}
	}
	marcando.Add(1)
	cambios.Add(1)
	c, err := (&net.Dialer{}).DialContext(ctx, red, direccion)
	marcando.Add(-1)
	cambios.Add(1)
	if err != nil || !deUnaPrueba {
		return c, err
	}
	return &conContada{Conn: c, clave: c.LocalAddr().String()}, nil
}

type escuchaContada struct{ net.Listener }

func (e escuchaContada) Accept() (net.Conn, error) {
	c, err := e.Listener.Accept()
	if err != nil {
		return c, err
	}
	return &conContada{Conn: c, clave: c.RemoteAddr().String(), lado: 1}, nil
}

func (e escuchaContada) Close() error {
	servidores.Store(e.Addr().String(), false)
	return e.Listener.Close()
}

// contarConexiones hace que el servidor, aún sin arrancar, cuente sus conexiones.
func contarConexiones(ts *httptest.Server) {
	servidores.Store(ts.Listener.Addr().String(), true)
	ts.Listener = escuchaContada{ts.Listener}
}

// Servidor arranca un servidor de prueba con h, con sus conexiones contadas, y devuelve su URL. Se cierra al acabar la
// prueba.
func Servidor(t *testing.T, h http.Handler) string {
	ts := httptest.NewUnstartedServer(h)
	contarConexiones(ts)
	ts.Start()
	t.Cleanup(ts.Close)
	return ts.URL
}
