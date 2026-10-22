package cliente

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"tramtai/internal/nucleo"
)

// FiltroTrabajos es lo que se pide a GET /api/v1/trabajos. Para la página siguiente, Antes y AntesID son el CreadoEn y
// el ID del último trabajo de la anterior (AntesID desempata los creados en el mismo instante).
type FiltroTrabajos struct {
	Limite int // 1 a 200; 0 deja el del servicio
	Punto  string
	Grupo  string
	Origen string // probar, terminal, cli o app
	Estado string // en_curso, completado o con_error (nucleo.Filtros)
	Texto  string // busca en el mensaje y en el id
	Antes  time.Time
	// AntesID solo vale junto a Antes.
	AntesID string
}

// TrabajosFiltrados lista los trabajos de una instancia, los más nuevos primero.
func (c *Cliente) TrabajosFiltrados(ctx context.Context, instancia string, f FiltroTrabajos) ([]nucleo.Trabajo, error) {
	q := url.Values{}
	if f.Limite > 0 {
		q.Set("limite", strconv.Itoa(f.Limite))
	}
	for clave, valor := range map[string]string{"punto": f.Punto, "grupo": f.Grupo, "origen": f.Origen, "estado": f.Estado, "texto": f.Texto} {
		if valor != "" {
			q.Set(clave, valor)
		}
	}
	if !f.Antes.IsZero() {
		q.Set("antes", f.Antes.UTC().Format(time.RFC3339Nano))
		if f.AntesID != "" {
			q.Set("antes_id", f.AntesID)
		}
	}
	var r struct {
		Trabajos []nucleo.Trabajo `json:"trabajos"`
	}
	ruta := "/api/v1/trabajos"
	if len(q) > 0 {
		ruta += "?" + q.Encode()
	}
	err := c.pedir(ctx, "GET", ruta, instancia, nil, "", &r)
	return r.Trabajos, err
}

// Trabajos lista los trabajos recientes de una instancia.
func (c *Cliente) Trabajos(ctx context.Context, instancia string, limite int) ([]nucleo.Trabajo, error) {
	return c.TrabajosFiltrados(ctx, instancia, FiltroTrabajos{Limite: limite})
}

// Registro devuelve los pasos de un trabajo, sin el texto de la respuesta: estados, intentos y avisos.
func (c *Cliente) Registro(ctx context.Context, instancia, id string) ([]nucleo.EventoRegistro, error) {
	var r struct {
		Eventos []nucleo.EventoRegistro `json:"eventos"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/trabajos/"+url.PathEscape(id)+"/registro", instancia, nil, "", &r)
	return r.Eventos, err
}

// VaciarTrabajos elimina los trabajos terminados de la instancia (los que siguen en marcha se quedan) y dice cuántos.
func (c *Cliente) VaciarTrabajos(ctx context.Context, instancia string) (int, error) {
	var r struct {
		Eliminados int `json:"eliminados"`
	}
	err := c.pedir(ctx, "DELETE", "/api/v1/trabajos", instancia, nil, "", &r)
	return r.Eliminados, err
}
