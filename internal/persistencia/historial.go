package persistencia

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Retencion son los plazos de retención, contados desde la terminación del trabajo.
type Retencion struct {
	Resultado        time.Duration // salida, entradas y eventos con contenido
	Metadatos        time.Duration // el trabajo entero; hasta entonces se responde 410 en lugar de 404
	Maximo           int           // trabajos terminados que se conservan como mucho; 0 = sin tope
	EliminarEntradas bool          // borra las entradas al terminar: el reintento exige reenviarlas
}

// sinEntradas quita de la solicitud guardada todo lo que trae contenido del usuario.
const sinEntradas = `json_remove(solicitud, '$.mensajes', '$.instrucciones', '$.cargas')`

// Purgar aplica la retención a los trabajos terminados de la instancia; los activos nunca se purgan.
// Devuelve cuántos resultados vencieron y cuántos trabajos se eliminaron.
func (a *Almacen) Purgar(ctx context.Context, instancia string, r Retencion) (vencidos, eliminados int, err error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	ahora := a.reloj().UTC()
	porTope := 0
	if r.Maximo > 0 {
		// Tope de cantidad: el corte es la fecha del terminado número Maximo; se borra lo terminado antes, con sus
		// claves de idempotencia (una vigente daría 404 al reintentar) y sus eventos. Los activos no cuentan. Los
		// empatados en el corte pueden dejar unos pocos de más.
		var corte string
		err := tx.QueryRowContext(ctx, `SELECT terminado_en FROM trabajos WHERE instancia = ? AND terminado_en IS NOT NULL
			ORDER BY terminado_en DESC LIMIT 1 OFFSET ?`, instancia, r.Maximo-1).Scan(&corte)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, 0, err
		}
		if corte != "" {
			const viejos = `SELECT id FROM trabajos WHERE instancia = ? AND terminado_en < ?`
			for _, tabla := range []string{"idempotencia WHERE trabajo_id", "eventos WHERE trabajo_id"} {
				if _, err := tx.ExecContext(ctx, `DELETE FROM `+tabla+` IN (`+viejos+`)`, instancia, corte); err != nil {
					return 0, 0, err
				}
			}
			res, err := tx.ExecContext(ctx, `DELETE FROM trabajos WHERE instancia = ? AND terminado_en < ?`, instancia, corte)
			if err != nil {
				return 0, 0, err
			}
			n, _ := res.RowsAffected()
			porTope = int(n)
		}
	}
	if r.EliminarEntradas {
		if _, err := tx.ExecContext(ctx, `UPDATE trabajos SET solicitud = `+sinEntradas+`, entradas_eliminadas = 1
			WHERE instancia = ? AND terminado_en IS NOT NULL AND entradas_eliminadas = 0`, instancia); err != nil {
			return 0, 0, err
		}
	}
	limiteResultado := fecha(ahora.Add(-r.Resultado))
	// Los eventos de estado, entrada e intento se conservan con los metadatos; los demás llevan contenido. Solo se miran los
	// trabajos que vencen ahora: los ya vencidos se limpiaron en su purga. Con <= vence también lo terminado en el
	// mismo instante (retención 0): en Windows el reloj avanza a saltos.
	if _, err := tx.ExecContext(ctx, `DELETE FROM eventos WHERE tipo NOT IN ('estado', 'intento', 'entrada') AND trabajo_id IN
		(SELECT id FROM trabajos WHERE instancia = ? AND terminado_en <= ? AND vencido = 0)`, instancia, limiteResultado); err != nil {
		return 0, 0, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE trabajos SET salida = '', solicitud = `+sinEntradas+`, vencido = 1, entradas_eliminadas = 1
		WHERE instancia = ? AND terminado_en <= ? AND vencido = 0`, instancia, limiteResultado)
	if err != nil {
		return 0, 0, err
	}
	n, _ := res.RowsAffected()
	vencidos = int(n)
	limiteMetadatos := fecha(ahora.Add(-r.Metadatos))
	local := ahora.In(a.Zona)
	haceUnAnio := time.Date(local.Year()-1, local.Month(), 1, 0, 0, 0, 0, a.Zona)
	pasos := []struct {
		consulta string
		args     []any
	}{
		{`DELETE FROM eventos WHERE trabajo_id IN (SELECT id FROM trabajos WHERE instancia = ? AND terminado_en < ?)`, []any{instancia, limiteMetadatos}},
		{`DELETE FROM idempotencia WHERE instancia = ? AND vence_en < ?`, []any{instancia, fecha(ahora)}},
		// Presupuestos: se conservan el mes en curso y los 12 anteriores.
		{`DELETE FROM consumo WHERE instancia = ? AND dia < ?`, []any{instancia, haceUnAnio.Format("2006-01-02")}},
		{`DELETE FROM avisos_presupuesto WHERE instancia = ? AND en < ?`, []any{instancia, fecha(haceUnAnio)}},
		{`DELETE FROM idempotencia WHERE trabajo_id IN (SELECT id FROM trabajos WHERE instancia = ? AND terminado_en < ?)`, []any{instancia, limiteMetadatos}},
		{`DELETE FROM trabajos WHERE instancia = ? AND terminado_en < ?`, []any{instancia, limiteMetadatos}},
	}
	for _, p := range pasos {
		if res, err = tx.ExecContext(ctx, p.consulta, p.args...); err != nil {
			return 0, 0, err
		}
	}
	n, _ = res.RowsAffected() // el último paso: trabajos eliminados
	if err := a.confirmar(tx); err != nil {
		return 0, 0, err
	}
	a.db.ExecContext(ctx, "PRAGMA incremental_vacuum") // devuelve al disco lo liberado; un fallo solo lo aplaza
	return vencidos, int(n) + porTope, nil
}

// UsoHistorial es lo que ocupa el historial: los trabajos de la instancia y el tamaño de la base.
type UsoHistorial struct {
	Trabajos int   `json:"trabajos"`
	Bytes    int64 `json:"bytes"`
}

func (a *Almacen) UsoHistorial(ctx context.Context, instancia string) (UsoHistorial, error) {
	var u UsoHistorial
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM trabajos WHERE instancia = ?`, instancia).Scan(&u.Trabajos); err != nil {
		return u, err
	}
	var paginas, tamano int64
	if err := a.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&paginas); err != nil {
		return u, err
	}
	if err := a.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&tamano); err != nil {
		return u, err
	}
	u.Bytes = paginas * tamano
	return u, nil
}
