package cliente

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/idioma"
)

// ErrSinActividad es el flujo que no mandó ni una línea (ni un latido) en el plazo que se le dio: se da por muerto aunque
// su conexión siga abierta.
var ErrSinActividad = errors.New(idioma.T("el flujo de eventos no dio señales"))

// vigilado avisa al reloj cada vez que el flujo trae algo, también un latido, y así solo salta si de verdad calla.
type vigilado struct {
	r     io.Reader
	reloj *time.Timer
	plazo time.Duration
}

func (v *vigilado) Read(p []byte) (int, error) {
	n, err := v.r.Read(p)
	if n > 0 {
		v.reloj.Reset(v.plazo)
	}
	return n, err
}

// leerSSE lee un flujo de eventos (líneas id:, event: y data:) y llama a alEvento con cada uno que traiga datos; los
// comentarios (el latido) no cuentan. Termina sin error con el evento fin y con el error del lector si se corta.
func leerSSE(r io.Reader, alEvento func(numero int, tipo, datos string) error) error {
	lector := bufio.NewScanner(r)
	lector.Buffer(make([]byte, 64<<10), 16<<20)
	numero, tipo, datos := 0, "", []string{}
	for lector.Scan() {
		linea := lector.Text()
		switch {
		case linea == "":
			if tipo == "fin" {
				return nil
			}
			if len(datos) > 0 {
				if err := alEvento(numero, tipo, strings.Join(datos, "\n")); err != nil {
					return err
				}
			}
			numero, tipo, datos = 0, "", nil
		case strings.HasPrefix(linea, "id: "):
			numero, _ = strconv.Atoi(linea[4:])
		case strings.HasPrefix(linea, "event: "):
			tipo = linea[7:]
		case strings.HasPrefix(linea, "data: "):
			datos = append(datos, linea[6:])
		}
	}
	return lector.Err()
}

// seguirSSE abre el flujo de ruta (con desde como Last-Event-ID si es mayor que 0) y lo lee hasta que acaba o se cancela
// ctx. Con inactividad mayor que 0, si pasa ese tiempo sin recibir nada (ni un latido) lo cierra con ErrSinActividad.
func (c *Cliente) seguirSSE(ctx context.Context, ruta, instancia string, desde int, inactividad time.Duration, alEvento func(numero int, tipo, datos string) error) error {
	padre := ctx
	var reloj *time.Timer
	if inactividad > 0 {
		var cancelar context.CancelFunc
		ctx, cancelar = context.WithCancel(ctx)
		defer cancelar()
		reloj = time.AfterFunc(inactividad, cancelar)
		defer reloj.Stop()
	}
	err := c.leerFlujo(ctx, ruta, instancia, desde, reloj, inactividad, alEvento)
	if err != nil && padre.Err() == nil && ctx.Err() != nil {
		return ErrSinActividad
	}
	return err
}

func (c *Cliente) leerFlujo(ctx context.Context, ruta, instancia string, desde int, reloj *time.Timer, inactividad time.Duration, alEvento func(numero int, tipo, datos string) error) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.Base+ruta, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Tramtai-Instancia", instancia)
	if desde > 0 {
		req.Header.Set("Last-Event-ID", strconv.Itoa(desde))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(idioma.T("eventos: el servicio respondió %s"), resp.Status)
	}
	var lector io.Reader = resp.Body
	if reloj != nil {
		lector = &vigilado{r: resp.Body, reloj: reloj, plazo: inactividad}
	}
	return leerSSE(lector, alEvento)
}

// Eventos sigue los eventos SSE de un trabajo desde el número desde hasta el evento fin.
func (c *Cliente) Eventos(ctx context.Context, instancia, id string, desde int, alEvento func(numero int, tipo, datos string) error) error {
	return c.seguirSSE(ctx, "/api/v1/trabajos/"+url.PathEscape(id)+"/eventos", instancia, desde, 0, alEvento)
}

// EventosInstancia sigue GET /api/v1/eventos, el flujo en vivo de la instancia: primero un evento trabajo por cada
// trabajo vivo o reciente y el evento foto que cierra esa lista (lo que no vino ya no existe), y después un evento
// trabajo por cada cambio. datos es el JSON del evento. Sigue hasta que se corte la conexión, alEvento devuelva un
// error o se cancele ctx (entonces devuelve ctx.Err()); quien lo llama decide si reconectar.
func (c *Cliente) EventosInstancia(ctx context.Context, instancia string, alEvento func(tipo string, datos []byte) error) error {
	return c.EventosInstanciaVigilado(ctx, instancia, 0, alEvento)
}

// EventosInstanciaVigilado es EventosInstancia que da el flujo por muerto si pasa inactividad sin recibir nada, ni siquiera
// el latido que el servicio manda cada 15 s: devuelve ErrSinActividad y quien lo llama reconecta. Con 0 no vigila.
func (c *Cliente) EventosInstanciaVigilado(ctx context.Context, instancia string, inactividad time.Duration, alEvento func(tipo string, datos []byte) error) error {
	err := c.seguirSSE(ctx, "/api/v1/eventos", instancia, 0, inactividad, func(_ int, tipo, datos string) error {
		return alEvento(tipo, []byte(datos))
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
