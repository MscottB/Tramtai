package prueba

import (
	"context"
	"fmt"
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
	servidores sync.Map // los servidores de prueba (*escucha), por su dirección

	muPares sync.Mutex
	pares   = map[string]*par{} // por la dirección del lado del cliente
	cambios atomic.Int64        // sube con cada lectura, escritura o cierre contado
)

// par son los dos lados de una conexión, 0 el cliente y 1 el servidor, y el servidor al que llega.
type par struct {
	escrito, leido [2]int64
	cerrado        [2]bool
	servidor       *escucha
}

type conContada struct {
	net.Conn
	clave    string
	lado     int
	servidor *escucha
	una      sync.Once
}

func (c *conContada) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		c.anotar(func(p *par) { p.leido[c.lado] += int64(n) })
	}
	return n, err
}

func (c *conContada) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		c.anotar(func(p *par) { p.escrito[c.lado] += int64(n) })
	}
	return n, err
}

func (c *conContada) Close() error {
	c.una.Do(func() { c.anotar(func(p *par) { p.cerrado[c.lado] = true }) })
	return c.Conn.Close()
}

func (c *conContada) anotar(f func(*par)) {
	muPares.Lock()
	p := pares[c.clave]
	if p == nil {
		p = &par{servidor: c.servidor}
		pares[c.clave] = p
	}
	f(p)
	if p.cerrado[0] && p.cerrado[1] {
		delete(pares, c.clave)
	}
	muPares.Unlock()
	cambios.Add(1)
}

var lados = [2]string{"el cliente", "el servidor"}

// sinLeer dice qué conexión se está abriendo o qué lado escribió algo que el otro, aún abierto, no ha leído; "" si nada.
// Lo de un servidor ya cerrado no cuenta: es de una prueba que terminó o que lo cerró a propósito, y no va a llegar.
func sinLeer() string {
	if n := marcando.Load(); n > 0 {
		return fmt.Sprintf("se están abriendo %d conexiones", n)
	}
	muPares.Lock()
	defer muPares.Unlock()
	for clave, p := range pares {
		if p.servidor.cerrado.Load() {
			delete(pares, clave)
			continue
		}
		for lado, otro := range []int{1, 0} {
			if !p.cerrado[otro] && p.escrito[lado] != p.leido[otro] {
				return fmt.Sprintf("en la conexión de %s, %s escribió %d bytes y %s leyó %d", clave, lados[lado], p.escrito[lado], lados[otro], p.leido[otro])
			}
		}
	}
	return ""
}

// marcando son las conexiones que se están abriendo: mientras tanto ningún lado ha escrito nada y las dos goroutines
// esperan a la red.
var marcando atomic.Int64

// marcar es la conexión del cliente: con un servidor de prueba, contada; con uno ya cerrado, rechazada al momento (Windows
// tarda unos segundos en rechazar una conexión a un puerto cerrado).
func marcar(ctx context.Context, red, direccion string) (net.Conn, error) {
	v, deUnaPrueba := servidores.Load(direccion)
	if deUnaPrueba && v.(*escucha).cerrado.Load() {
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
	return &conContada{Conn: c, clave: c.LocalAddr().String(), servidor: v.(*escucha)}, nil
}

// escucha es la de un servidor de prueba, que cuenta sus conexiones.
type escucha struct {
	net.Listener
	cerrado atomic.Bool
}

func (e *escucha) Accept() (net.Conn, error) {
	c, err := e.Listener.Accept()
	if err != nil {
		return c, err
	}
	return &conContada{Conn: c, clave: c.RemoteAddr().String(), lado: 1, servidor: e}, nil
}

func (e *escucha) Close() error {
	e.cerrado.Store(true)
	return e.Listener.Close()
}

// contarConexiones hace que el servidor, aún sin arrancar, cuente sus conexiones.
func contarConexiones(ts *httptest.Server) {
	e := &escucha{Listener: ts.Listener}
	servidores.Store(e.Addr().String(), e)
	ts.Listener = e
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
