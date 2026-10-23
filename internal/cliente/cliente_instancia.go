package cliente

import (
	"context"
	"encoding/json"

	"tramtai/internal/contrato"
)

func (c *Cliente) Instancia(ctx context.Context, instancia string) (contrato.ResumenInstancia, error) {
	var r contrato.ResumenInstancia
	return r, c.pedir(ctx, "GET", "/api/v1/instancia", instancia, nil, "", &r)
}

// ConfiguracionInstancia devuelve la configuración efectiva de la instancia con el origen de cada valor.
func (c *Cliente) ConfiguracionInstancia(ctx context.Context, instancia string) ([]contrato.EntradaConfiguracion, error) {
	var r struct {
		Configuracion []contrato.EntradaConfiguracion `json:"configuracion"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/instancia/configuracion", instancia, nil, "", &r)
	return r.Configuracion, err
}

// Heredar quita del local.yaml de un proyecto un valor (predeterminados.*, retencion.* o presupuesto.*) para que valga
// el de Global.
func (c *Cliente) Heredar(ctx context.Context, instancia, clave string) error {
	return c.pedir(ctx, "POST", "/api/v1/instancia/heredar", instancia, cuerpoJSON(map[string]string{"clave": clave}), "application/json", nil)
}

// FijarPredeterminado guarda el modelo que se usa cuando la solicitud no indica agente ni modelo (agente,
// agente/modelo o perfil/nombre) y su esfuerzo. Con modelo vacío solo cambia el esfuerzo; con esfuerzo nil no lo toca
// y con "" vuelve al del agente.
func (c *Cliente) FijarPredeterminado(ctx context.Context, instancia, modelo string, esfuerzo *string) error {
	cuerpo := cuerpoJSON(map[string]any{"modelo": modelo, "esfuerzo": esfuerzo})
	return c.pedir(ctx, "POST", "/api/v1/instancia/predeterminado", instancia, cuerpo, "application/json", nil)
}

// Retencion devuelve los plazos y el tope del historial de la instancia, y lo que ocupa.
func (c *Cliente) Retencion(ctx context.Context, instancia string) (contrato.Retencion, error) {
	var r contrato.Retencion
	return r, c.pedir(ctx, "GET", "/api/v1/instancia/retencion", instancia, nil, "", &r)
}

// FijarRetencion guarda los plazos y el tope del historial y purga al momento; Uso no se envía.
func (c *Cliente) FijarRetencion(ctx context.Context, instancia string, r contrato.Retencion) error {
	cuerpo := cuerpoJSON(map[string]int{"horas_resultado": r.HorasResultado, "horas_metadatos": r.HorasMetadatos, "max_trabajos": r.MaxTrabajos})
	return c.pedir(ctx, "PUT", "/api/v1/instancia/retencion", instancia, cuerpo, "application/json", nil)
}

// ValidarEsquema comprueba un JSON Schema (un objeto, o un texto JSON que lo contenga) y devuelve lo que está mal,
// legible; sin problemas, nil.
func (c *Cliente) ValidarEsquema(ctx context.Context, instancia string, esquema json.RawMessage) ([]string, error) {
	var r struct {
		Valido bool   `json:"valido"`
		Error  string `json:"error"` // ya dice la línea y la columna si las conoce
	}
	if err := c.pedir(ctx, "POST", "/api/v1/esquemas/validar", instancia, cuerpoJSON(map[string]json.RawMessage{"esquema": esquema}), "application/json", &r); err != nil {
		return nil, err
	}
	if r.Valido {
		return nil, nil
	}
	return []string{r.Error}, nil
}

// Limites devuelve los topes de una petición y de las cargas.
func (c *Cliente) Limites(ctx context.Context, instancia string) (contrato.Limites, error) {
	var l contrato.Limites
	return l, c.pedir(ctx, "GET", "/api/v1/limites", instancia, nil, "", &l)
}

func (c *Cliente) AjustesConversor(ctx context.Context, instancia string, a contrato.AjustesConversor) error {
	return c.pedir(ctx, "PUT", "/api/v1/conversor/ajustes", instancia, cuerpoJSON(a), "application/json", nil)
}
