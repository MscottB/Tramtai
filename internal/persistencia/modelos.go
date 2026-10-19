package persistencia

import (
	"context"
	"database/sql"
	"errors"
)

const consultaEfectivos = `SELECT COALESCE(json_extract(solicitud, '$.agente'), ''), modelo_enviado, modelo_efectivo
		FROM trabajos WHERE modelo_efectivo <> '' ORDER BY creado_en DESC LIMIT 500`

// ModelosEfectivos devuelve, por "agente/modelo enviado" (modelo vacío: el predeterminado del CLI), el último
// modelo efectivo que informó el agente; así la Web UI muestra la versión real detrás de un alias como "sonnet". Con el
// índice trabajos_efectivo lee solo esas filas, por largo que sea el historial.
func (a *Almacen) ModelosEfectivos(ctx context.Context) (map[string]string, error) {
	filas, err := a.db.QueryContext(ctx, consultaEfectivos)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	modelos := map[string]string{}
	for filas.Next() {
		var agente, enviado, efectivo string
		if err := filas.Scan(&agente, &enviado, &efectivo); err != nil {
			return nil, err
		}
		if _, visto := modelos[agente+"/"+enviado]; !visto {
			modelos[agente+"/"+enviado] = efectivo
		}
	}
	return modelos, filas.Err()
}

// GuardarContexto anota el contexto que informó un modelo concreto; el último gana.
func (a *Almacen) GuardarContexto(ctx context.Context, agente, modelo string, tokens int) error {
	_, err := a.db.ExecContext(ctx, `INSERT INTO modelos_vistos (agente, modelo, contexto_tokens, visto_en) VALUES (?, ?, ?, ?)
		ON CONFLICT (agente, modelo) DO UPDATE SET contexto_tokens = excluded.contexto_tokens, visto_en = excluded.visto_en`,
		agente, modelo, tokens, fecha(a.reloj()))
	return err
}

// Contexto devuelve el contexto informado de un modelo concreto; 0 si no se conoce.
func (a *Almacen) Contexto(ctx context.Context, agente, modelo string) (int, error) {
	var tokens int
	err := a.db.QueryRowContext(ctx, `SELECT contexto_tokens FROM modelos_vistos WHERE agente = ? AND modelo = ?`, agente, modelo).Scan(&tokens)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return tokens, err
}
