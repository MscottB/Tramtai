package persistencia

import (
	"context"
	"database/sql"
	"log"
	"slices"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// TodaLaInstancia es el ámbito de un Tope o de Consumo que abarca todas las filas de la instancia (con y sin API).
const TodaLaInstancia = "*"

// Tope es un presupuesto que se aplica a los trabajos de una API, o de toda la instancia con TodaLaInstancia.
type Tope struct {
	Punto       string
	Presupuesto nucleo.Presupuesto
}

// ambitoConsumo filtra las filas de una API ("" = trabajos sin API) o, con TodaLaInstancia, todas las de la instancia.
const ambitoConsumo = `instancia = ? AND (? = '*' OR punto = ?)`

// Consumo devuelve el consumo del día y del mes en curso de una API o de la instancia.
func (a *Almacen) Consumo(ctx context.Context, instancia, punto string) (dia, mes nucleo.Consumo, err error) {
	return a.consumoEn(ctx, a.db, instancia, punto, a.reloj())
}

func (a *Almacen) consumoEn(ctx context.Context, c consultor, instancia, punto string, ahora time.Time) (dia, mes nucleo.Consumo, err error) {
	clave, claveMes, _, _ := nucleo.Periodos(ahora, a.Zona)
	err = c.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(CASE WHEN dia = ? THEN peticiones END), 0), COALESCE(SUM(CASE WHEN dia = ? THEN terminados END), 0),
		COALESCE(SUM(CASE WHEN dia = ? THEN tokens END), 0), COALESCE(SUM(CASE WHEN dia = ? THEN usd END), 0),
		COALESCE(SUM(peticiones), 0), COALESCE(SUM(terminados), 0), COALESCE(SUM(tokens), 0), COALESCE(SUM(usd), 0)
		FROM consumo WHERE `+ambitoConsumo+` AND dia LIKE ?`,
		clave, clave, clave, clave, instancia, punto, punto, claveMes+"-%").Scan(
		&dia.Peticiones, &dia.Terminados, &dia.Tokens, &dia.USD, &mes.Peticiones, &mes.Terminados, &mes.Tokens, &mes.USD)
	return dia, mes, err
}

func (a *Almacen) topes(instancia, punto string) []Tope {
	if a.Topes == nil {
		return nil
	}
	return a.Topes(instancia, punto)
}

// comprobarPresupuesto rechaza el trabajo si agotaría algún tope que se le aplica. Un fallo al leer el consumo no
// lo rechaza: un contador roto no debe tumbar la API.
func (a *Almacen) comprobarPresupuesto(ctx context.Context, tx *sql.Tx, t nucleo.Trabajo, ahora time.Time) error {
	for _, tope := range a.topes(t.Instancia, t.PuntoAcceso) {
		dia, mes, err := a.consumoEn(ctx, tx, t.Instancia, tope.Punto, ahora)
		if err != nil {
			log.Printf("presupuesto de %s: no se pudo leer el consumo: %v", tope.Punto, err)
			continue
		}
		ambito := idioma.T("de la instancia")
		if tope.Punto != TodaLaInstancia {
			ambito = idioma.Tf("de la API %s", tope.Punto)
		}
		if err := tope.Presupuesto.Comprobar(dia, mes, ahora, a.Zona, ambito); err != nil {
			return err
		}
	}
	return nil
}

// sumarConsumo suma c en la fila del día de aceptación del trabajo y registra los cruces del 80 % y del 100 %.
// Los errores se registran y no deshacen la transacción del trabajo.
func (a *Almacen) sumarConsumo(ctx context.Context, tx *sql.Tx, instancia, punto string, aceptado, ahora time.Time, c nucleo.Consumo) {
	dia, _, _, _ := nucleo.Periodos(aceptado, a.Zona)
	if _, err := tx.ExecContext(ctx, `INSERT INTO consumo (instancia, punto, dia, peticiones, terminados, tokens, usd) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (instancia, punto, dia) DO UPDATE SET peticiones = peticiones + excluded.peticiones,
		terminados = terminados + excluded.terminados, tokens = tokens + excluded.tokens, usd = usd + excluded.usd`,
		instancia, punto, dia, c.Peticiones, c.Terminados, c.Tokens, c.USD); err != nil {
		log.Printf("presupuesto: no se pudo sumar el consumo de %q: %v", punto, err)
		return
	}
	claveDia, claveMes, _, _ := nucleo.Periodos(ahora, a.Zona)
	for _, tope := range a.topes(instancia, punto) {
		dia, mes, err := a.consumoEn(ctx, tx, instancia, tope.Punto, ahora)
		if err != nil {
			log.Printf("presupuesto de %s: no se pudo leer el consumo: %v", tope.Punto, err)
			continue
		}
		for _, per := range []struct {
			periodo string
			limites nucleo.Limites
			consumo nucleo.Consumo
		}{{"D:" + claveDia, tope.Presupuesto.Diario, dia}, {"M:" + claveMes, tope.Presupuesto.Mensual, mes}} {
			f, unidad := per.limites.Fraccion(per.consumo, false)
			for _, nivel := range []int{80, 100} {
				if unidad == "" || f*100 < float64(nivel) {
					continue
				}
				if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO avisos_presupuesto (instancia, punto, periodo, nivel, en) VALUES (?, ?, ?, ?, ?)`,
					instancia, tope.Punto, per.periodo, nivel, fecha(ahora)); err != nil {
					log.Printf("presupuesto de %s: no se pudo registrar el aviso: %v", tope.Punto, err)
				}
			}
		}
	}
}

// ReiniciarConsumo borra el consumo y los avisos del día y del mes en curso de una API, o de toda la instancia con
// TodaLaInstancia. El mes incluye el día: comparten filas.
func (a *Almacen) ReiniciarConsumo(ctx context.Context, instancia, punto string) error {
	dia, mes, _, _ := nucleo.Periodos(a.reloj(), a.Zona)
	return a.borrarConsumo(ctx, instancia, punto, mes+"-%", "D:"+dia, "M:"+mes)
}

// EliminarConsumo borra todo el consumo y los avisos de una API (al eliminarla).
func (a *Almacen) EliminarConsumo(ctx context.Context, instancia, punto string) error {
	return a.borrarConsumo(ctx, instancia, punto, "")
}

// borrarConsumo borra las filas del ámbito; con dias (patrón LIKE) solo esas y los avisos de esos periodos.
func (a *Almacen) borrarConsumo(ctx context.Context, instancia, punto, dias string, periodos ...string) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ambito := []any{instancia, punto, punto}
	filas, argsFilas := ambitoConsumo, ambito
	avisos, argsAvisos := `DELETE FROM avisos_presupuesto WHERE `+ambitoConsumo, ambito
	if dias != "" {
		filas, argsFilas = filas+` AND dia LIKE ?`, append(slices.Clone(ambito), dias)
		avisos, argsAvisos = avisos+` AND periodo IN (?, ?)`, append(slices.Clone(ambito), periodos[0], periodos[1])
	}
	// Reiniciar o eliminar una API no devuelve consumo a la instancia: sus filas pasan a "~", que ninguna API usa
	// (NombrePuntoValido no admite "~") y que solo cuenta en la suma de la instancia.
	if punto != TodaLaInstancia {
		if _, err := tx.ExecContext(ctx, `INSERT INTO consumo (instancia, punto, dia, peticiones, terminados, tokens, usd)
			SELECT instancia, '~', dia, peticiones, terminados, tokens, usd FROM consumo WHERE `+filas+`
			ON CONFLICT (instancia, punto, dia) DO UPDATE SET peticiones = peticiones + excluded.peticiones,
			terminados = terminados + excluded.terminados, tokens = tokens + excluded.tokens, usd = usd + excluded.usd`, argsFilas...); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM consumo WHERE `+filas, argsFilas...); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, avisos, argsAvisos...); err != nil {
		return err
	}
	return a.confirmar(tx)
}
