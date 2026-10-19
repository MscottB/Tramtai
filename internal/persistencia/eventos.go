package persistencia

import (
	"context"
	"time"
)

// Evento es un registro numerado de un trabajo.
type Evento struct {
	Numero   int
	Tipo     string
	Datos    string
	CreadoEn time.Time
}

// AgregarEvento añade un evento con el siguiente número del trabajo.
func (a *Almacen) AgregarEvento(ctx context.Context, id, tipo, datos string) error {
	if err := agregarEvento(ctx, a.db, id, tipo, datos, a.reloj().UTC()); err != nil {
		return err
	}
	a.avisar()
	return nil
}

// Eventos devuelve los eventos posteriores al número "desde" (Last-Event-ID).
func (a *Almacen) Eventos(ctx context.Context, id string, desde int) ([]Evento, error) {
	return a.eventos(ctx, `SELECT numero, tipo, datos, creado_en FROM eventos WHERE trabajo_id = ? AND numero > ? ORDER BY numero`, id, desde)
}

// EventosDeRegistro devuelve los eventos sin contenido de la respuesta (estado, entrada, intento, aviso y validacion),
// sin leer el texto.
func (a *Almacen) EventosDeRegistro(ctx context.Context, id string) ([]Evento, error) {
	return a.eventos(ctx, `SELECT numero, tipo, datos, creado_en FROM eventos WHERE trabajo_id = ? AND tipo IN ('estado', 'entrada', 'intento', 'aviso', 'validacion') ORDER BY numero`, id)
}

func (a *Almacen) eventos(ctx context.Context, consulta string, args ...any) ([]Evento, error) {
	filas, err := a.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var eventos []Evento
	for filas.Next() {
		var e Evento
		var creado string
		if err := filas.Scan(&e.Numero, &e.Tipo, &e.Datos, &creado); err != nil {
			return nil, err
		}
		e.CreadoEn, _ = time.Parse(formatoFecha, creado)
		eventos = append(eventos, e)
	}
	return eventos, filas.Err()
}

func agregarEvento(ctx context.Context, e ejecutor, id, tipo, datos string, cuando time.Time) error {
	_, err := e.ExecContext(ctx, `INSERT INTO eventos (trabajo_id, numero, tipo, datos, creado_en)
		SELECT ?, COALESCE(MAX(numero), 0) + 1, ?, ?, ? FROM eventos WHERE trabajo_id = ?`, id, tipo, datos, fecha(cuando), id)
	return err
}
