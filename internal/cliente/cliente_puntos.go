package cliente

import (
	"context"
	"net/url"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Puntos lista los puntos de acceso de una instancia con su uso y el consumo de su presupuesto.
func (c *Cliente) Puntos(ctx context.Context, instancia string) (contrato.ListaPuntos, error) {
	var l contrato.ListaPuntos
	return l, c.pedir(ctx, "GET", "/api/v1/puntos", instancia, nil, "", &l)
}

// CambiarPunto cambia solo lo que trae cambios (PATCH): activo, instrucciones, esfuerzo, modelo, forzar_modelo,
// modelos_permitidos, esquema, respaldo, reintentos_salida y presupuesto. Devuelve el punto como queda.
func (c *Cliente) CambiarPunto(ctx context.Context, instancia, nombre string, cambios map[string]any) (contrato.EstadoPunto, error) {
	return c.cambiarPunto(ctx, instancia, nombre, cambios)
}

// AjustesPunto son los ajustes de una API tal como los edita Mis APIs: se mandan todos, así que lo vacío vuelve a lo del
// espacio (el modelo, el esfuerzo del agente, cualquier modelo) y Presupuesto nil lo quita.
type AjustesPunto struct {
	Instrucciones     string              `json:"instrucciones"`
	Modelo            string              `json:"modelo"` // un id del catálogo: agente, agente/modelo o perfil/nombre
	ForzarModelo      bool                `json:"forzar_modelo"`
	Esfuerzo          string              `json:"esfuerzo"`
	ModelosPermitidos []string            `json:"modelos_permitidos"`
	Presupuesto       *nucleo.Presupuesto `json:"presupuesto"`
}

// AjustarPunto guarda los ajustes de una API (PATCH) sin tocar lo demás y devuelve la API como queda.
func (c *Cliente) AjustarPunto(ctx context.Context, instancia, nombre string, a AjustesPunto) (contrato.EstadoPunto, error) {
	if a.ModelosPermitidos == nil {
		a.ModelosPermitidos = []string{} // null no cambiaría la lista
	}
	return c.cambiarPunto(ctx, instancia, nombre, a)
}

func (c *Cliente) cambiarPunto(ctx context.Context, instancia, nombre string, cambios any) (contrato.EstadoPunto, error) {
	var r struct {
		Puntos []contrato.EstadoPunto `json:"puntos_acceso"`
	}
	if err := c.pedir(ctx, "PATCH", "/api/v1/puntos/"+url.PathEscape(nombre), instancia, cuerpoJSON(cambios), "application/json", &r); err != nil {
		return contrato.EstadoPunto{}, err
	}
	for _, p := range r.Puntos {
		if p.Nombre == nombre {
			return p, nil
		}
	}
	return contrato.EstadoPunto{}, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("API no encontrada en este espacio: %s", nombre))
}

// ClaveInstancia devuelve la clave del espacio, la que usan la CLI, los scripts y las apps sin API propia.
func (c *Cliente) ClaveInstancia(ctx context.Context, instancia string) (string, error) {
	var r struct {
		Token string `json:"token"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/instancia/token", instancia, nil, "", &r)
	return r.Token, err
}

// PresupuestoInstancia devuelve el consumo del día y del mes frente al presupuesto de la instancia; vacío si no tiene.
func (c *Cliente) PresupuestoInstancia(ctx context.Context, instancia string) (nucleo.EstadoPresupuesto, error) {
	var r struct {
		Presupuesto nucleo.EstadoPresupuesto `json:"presupuesto"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/instancia/presupuesto", instancia, nil, "", &r)
	return r.Presupuesto, err
}

// ReiniciarPresupuestoInstancia pone a cero el consumo del día y del mes de la instancia.
func (c *Cliente) ReiniciarPresupuestoInstancia(ctx context.Context, instancia string) error {
	return c.ReiniciarPresupuesto(ctx, instancia, "")
}
