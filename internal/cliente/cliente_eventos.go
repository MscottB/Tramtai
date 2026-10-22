package cliente

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"tramtai/internal/idioma"
)

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
// ctx.
func (c *Cliente) seguirSSE(ctx context.Context, ruta, instancia string, desde int, alEvento func(numero int, tipo, datos string) error) error {
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
	return leerSSE(resp.Body, alEvento)
}

// Eventos sigue los eventos SSE de un trabajo desde el número desde hasta el evento fin.
func (c *Cliente) Eventos(ctx context.Context, instancia, id string, desde int, alEvento func(numero int, tipo, datos string) error) error {
	return c.seguirSSE(ctx, "/api/v1/trabajos/"+url.PathEscape(id)+"/eventos", instancia, desde, alEvento)
}

// EventosInstancia sigue GET /api/v1/eventos, el flujo en vivo de la instancia: primero un evento trabajo por cada
// trabajo vivo o reciente y el evento foto que cierra esa lista (lo que no vino ya no existe), y después un evento
// trabajo por cada cambio. datos es el JSON del evento. Sigue hasta que se corte la conexión, alEvento devuelva un
// error o se cancele ctx (entonces devuelve ctx.Err()); quien lo llama decide si reconectar.
func (c *Cliente) EventosInstancia(ctx context.Context, instancia string, alEvento func(tipo string, datos []byte) error) error {
	err := c.seguirSSE(ctx, "/api/v1/eventos", instancia, 0, func(_ int, tipo, datos string) error {
		return alEvento(tipo, []byte(datos))
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
