// Package persistencia guarda trabajos, eventos e idempotencia de una instancia en SQLite (sección 4.3).
package persistencia

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // driver Go puro: sin CGO

	"tramtai/internal/nucleo"
)

const versionEsquema = 1

// formatoFecha tiene ancho fijo para que las fechas se comparen y ordenen como texto.
const formatoFecha = "2006-01-02T15:04:05.000000000Z"

// sinVencimiento marca la clave de idempotencia de un trabajo activo: no se purga mientras no termine.
const sinVencimiento = "9999-12-31T00:00:00.000000000Z"

const esquemaSQL = `
CREATE TABLE IF NOT EXISTS trabajos (
	id TEXT PRIMARY KEY,
	instancia TEXT NOT NULL,
	estado TEXT NOT NULL,
	codigo_error TEXT NOT NULL DEFAULT '',
	mensaje_error TEXT NOT NULL DEFAULT '',
	solicitud TEXT NOT NULL,
	huella TEXT NOT NULL,
	salida TEXT NOT NULL DEFAULT '',
	intento_de TEXT NOT NULL DEFAULT '',
	creado_en TEXT NOT NULL,
	actualizado_en TEXT NOT NULL,
	terminado_en TEXT
);
CREATE INDEX IF NOT EXISTS trabajos_instancia_estado ON trabajos (instancia, estado, creado_en);
CREATE TABLE IF NOT EXISTS eventos (
	trabajo_id TEXT NOT NULL REFERENCES trabajos (id) ON DELETE CASCADE,
	numero INTEGER NOT NULL,
	tipo TEXT NOT NULL,
	datos TEXT NOT NULL DEFAULT '',
	creado_en TEXT NOT NULL,
	PRIMARY KEY (trabajo_id, numero)
);
CREATE TABLE IF NOT EXISTS idempotencia (
	instancia TEXT NOT NULL,
	clave TEXT NOT NULL,
	huella TEXT NOT NULL,
	trabajo_id TEXT NOT NULL,
	vence_en TEXT NOT NULL,
	PRIMARY KEY (instancia, clave)
);`

const columnas = `id, instancia, estado, codigo_error, mensaje_error, solicitud, huella, salida, intento_de, creado_en, actualizado_en, terminado_en`

// Almacen es la base SQLite de una instancia.
type Almacen struct {
	db *sql.DB
	// VentanaIdempotencia es el tiempo que una clave sigue vinculada a su trabajo tras terminar.
	VentanaIdempotencia time.Duration
	reloj               func() time.Time
}

// Cambio son los datos opcionales que acompañan una transición.
type Cambio struct {
	Codigo  nucleo.CodigoError
	Mensaje string
	Salida  *string
}

// Evento es un registro numerado de un trabajo (sección 7.3).
type Evento struct {
	Numero   int
	Tipo     string
	Datos    string
	CreadoEn time.Time
}

type consultor interface {
	QueryRowContext(ctx context.Context, consulta string, args ...any) *sql.Row
}

type ejecutor interface {
	ExecContext(ctx context.Context, consulta string, args ...any) (sql.Result, error)
}

// Abrir abre o crea la base en WAL y aplica las migraciones pendientes.
func Abrir(ruta string) (*Almacen, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(ruta)+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	// Una sola conexión serializa las escrituras de la instancia y conserva los PRAGMA.
	db.SetMaxOpenConns(1)
	if err := migrar(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Almacen{db: db, VentanaIdempotencia: 24 * time.Hour, reloj: time.Now}, nil
}

func migrar(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > versionEsquema {
		return fmt.Errorf("versión de base de datos %d no soportada por este ejecutable", version)
	}
	if version == versionEsquema {
		return nil
	}
	if _, err := db.Exec(esquemaSQL); err != nil {
		return err
	}
	_, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", versionEsquema))
	return err
}

// Cerrar cierra la base.
func (a *Almacen) Cerrar() error { return a.db.Close() }

// Aceptar registra el trabajo en_cola, su primer evento y su clave de idempotencia en una transacción.
// Con la misma clave y huella devuelve el trabajo original (reutilizado = true); con otra huella, conflicto_idempotencia.
// Si la instancia ya tiene maximoActivos trabajos sin empezar, rechaza con cola_llena sin reservar la clave.
func (a *Almacen) Aceptar(ctx context.Context, t nucleo.Trabajo, clave string, maximoActivos int) (nucleo.Trabajo, bool, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return t, false, err
	}
	defer tx.Rollback()
	ahora := a.reloj().UTC()
	if clave != "" {
		var huella, id string
		err := tx.QueryRowContext(ctx, `SELECT huella, trabajo_id FROM idempotencia WHERE instancia = ? AND clave = ? AND vence_en > ?`,
			t.Instancia, clave, fecha(ahora)).Scan(&huella, &id)
		switch {
		case err == nil && huella == t.Huella:
			existente, err := obtener(ctx, tx, id)
			return existente, true, err
		case err == nil:
			return t, false, nucleo.NuevoError(nucleo.CodigoConflictoIdempotencia, "la clave de idempotencia ya se usó con otra solicitud")
		case !errors.Is(err, sql.ErrNoRows):
			return t, false, err
		}
	}
	var activos int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM trabajos WHERE instancia = ? AND estado IN (?, ?, ?)`,
		t.Instancia, string(nucleo.EnCola), string(nucleo.PreparandoEntrada), string(nucleo.EsperandoAgente)).Scan(&activos); err != nil {
		return t, false, err
	}
	if maximoActivos > 0 && activos >= maximoActivos {
		return t, false, nucleo.NuevoError(nucleo.CodigoColaLlena, "la cola de la instancia está llena")
	}
	solicitud, err := json.Marshal(t.Solicitud)
	if err != nil {
		return t, false, err
	}
	t.Estado, t.CreadoEn, t.ActualizadoEn = nucleo.EnCola, ahora, ahora
	if _, err := tx.ExecContext(ctx, `INSERT INTO trabajos (id, instancia, estado, solicitud, huella, intento_de, creado_en, actualizado_en) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Instancia, string(t.Estado), string(solicitud), t.Huella, t.IntentoDe, fecha(ahora), fecha(ahora)); err != nil {
		return t, false, err
	}
	if err := agregarEvento(ctx, tx, t.ID, "estado", string(nucleo.EnCola), ahora); err != nil {
		return t, false, err
	}
	if clave != "" {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO idempotencia (instancia, clave, huella, trabajo_id, vence_en) VALUES (?, ?, ?, ?, ?)`,
			t.Instancia, clave, t.Huella, t.ID, sinVencimiento); err != nil {
			return t, false, err
		}
	}
	return t, false, tx.Commit()
}

// CambiarEstado aplica una transición validada y registra su evento. Si el trabajo ya no está en "de"
// (otro actor lo cambió antes), devuelve transicion_invalida sin modificar nada.
func (a *Almacen) CambiarEstado(ctx context.Context, id string, de, hacia nucleo.Estado, cambio Cambio) error {
	if err := nucleo.ValidarTransicion(de, hacia); err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ahora := a.reloj().UTC()
	var terminado any
	if hacia.Terminal() {
		terminado = fecha(ahora)
	}
	salida, conSalida := "", cambio.Salida != nil
	if conSalida {
		salida = *cambio.Salida
	}
	resultado, err := tx.ExecContext(ctx, `UPDATE trabajos SET estado = ?, codigo_error = ?, mensaje_error = ?,
		salida = CASE WHEN ? THEN ? ELSE salida END, actualizado_en = ?, terminado_en = COALESCE(?, terminado_en)
		WHERE id = ? AND estado = ?`,
		string(hacia), string(cambio.Codigo), cambio.Mensaje, conSalida, salida, fecha(ahora), terminado, id, string(de))
	if err != nil {
		return err
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		if _, err := obtener(ctx, tx, id); err != nil {
			return err
		}
		return nucleo.NuevoError(nucleo.CodigoTransicionInvalida, fmt.Sprintf("el trabajo %s ya no está en %s", id, de))
	}
	if err := agregarEvento(ctx, tx, id, "estado", string(hacia), ahora); err != nil {
		return err
	}
	if hacia.Terminal() {
		if _, err := tx.ExecContext(ctx, `UPDATE idempotencia SET vence_en = ? WHERE trabajo_id = ?`,
			fecha(ahora.Add(a.VentanaIdempotencia)), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AgregarEvento añade un evento con el siguiente número del trabajo.
func (a *Almacen) AgregarEvento(ctx context.Context, id, tipo, datos string) error {
	return agregarEvento(ctx, a.db, id, tipo, datos, a.reloj().UTC())
}

// Obtener devuelve un trabajo o no_encontrado.
func (a *Almacen) Obtener(ctx context.Context, id string) (nucleo.Trabajo, error) {
	return obtener(ctx, a.db, id)
}

// Eventos devuelve los eventos posteriores al número "desde" (Last-Event-ID).
func (a *Almacen) Eventos(ctx context.Context, id string, desde int) ([]Evento, error) {
	filas, err := a.db.QueryContext(ctx, `SELECT numero, tipo, datos, creado_en FROM eventos WHERE trabajo_id = ? AND numero > ? ORDER BY numero`, id, desde)
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

// ListarPorEstado devuelve los identificadores de la instancia en esos estados, del más antiguo al más reciente.
func (a *Almacen) ListarPorEstado(ctx context.Context, instancia string, estados ...nucleo.Estado) ([]string, error) {
	args := []any{instancia}
	for _, e := range estados {
		args = append(args, string(e))
	}
	marcas := strings.TrimSuffix(strings.Repeat("?, ", len(estados)), ", ")
	filas, err := a.db.QueryContext(ctx, `SELECT id FROM trabajos WHERE instancia = ? AND estado IN (`+marcas+`) ORDER BY creado_en`, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var ids []string
	for filas.Next() {
		var id string
		if err := filas.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, filas.Err()
}

// Recuperar aplica la sección 4.3 al arrancar: lo que pudo llegar al agente pasa a interrumpido
// (nunca se relanza) y lo que nunca lo lanzó vuelve a en_cola.
// shortcut: en M1 las entradas viajan dentro de la solicitud y siempre están disponibles; con cargas (M3)
// se verificarán sus hashes y, si falta alguna, el trabajo pasará a fallido con entrada_no_disponible.
func (a *Almacen) Recuperar(ctx context.Context, instancia string) (interrumpidos, reencolados int, err error) {
	ids, err := a.ListarPorEstado(ctx, instancia, nucleo.PreparandoEntrada, nucleo.EsperandoAgente,
		nucleo.Ejecutando, nucleo.ValidandoSalida, nucleo.CancelacionSolicitada)
	if err != nil {
		return 0, 0, err
	}
	for _, id := range ids {
		t, err := a.Obtener(ctx, id)
		if err != nil {
			return interrumpidos, reencolados, err
		}
		if t.Estado == nucleo.PreparandoEntrada || t.Estado == nucleo.EsperandoAgente {
			err = a.CambiarEstado(ctx, id, t.Estado, nucleo.EnCola, Cambio{})
			reencolados++
		} else {
			err = a.CambiarEstado(ctx, id, t.Estado, nucleo.Interrumpido, Cambio{Mensaje: "el servicio se reinició sin poder confirmar la ejecución"})
			interrumpidos++
		}
		if err != nil {
			return interrumpidos, reencolados, err
		}
	}
	return interrumpidos, reencolados, nil
}

func obtener(ctx context.Context, c consultor, id string) (nucleo.Trabajo, error) {
	var t nucleo.Trabajo
	var estado, codigo, solicitud, creado, actualizado string
	var terminado sql.NullString
	err := c.QueryRowContext(ctx, `SELECT `+columnas+` FROM trabajos WHERE id = ?`, id).Scan(&t.ID, &t.Instancia, &estado,
		&codigo, &t.MensajeError, &solicitud, &t.Huella, &t.Salida, &t.IntentoDe, &creado, &actualizado, &terminado)
	if errors.Is(err, sql.ErrNoRows) {
		return t, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "trabajo no encontrado: "+id)
	}
	if err != nil {
		return t, err
	}
	t.Estado, t.CodigoError = nucleo.Estado(estado), nucleo.CodigoError(codigo)
	if err := json.Unmarshal([]byte(solicitud), &t.Solicitud); err != nil {
		return t, err
	}
	t.CreadoEn, _ = time.Parse(formatoFecha, creado)
	t.ActualizadoEn, _ = time.Parse(formatoFecha, actualizado)
	if terminado.Valid {
		fin, _ := time.Parse(formatoFecha, terminado.String)
		t.TerminadoEn = &fin
	}
	return t, nil
}

func agregarEvento(ctx context.Context, e ejecutor, id, tipo, datos string, cuando time.Time) error {
	_, err := e.ExecContext(ctx, `INSERT INTO eventos (trabajo_id, numero, tipo, datos, creado_en)
		SELECT ?, COALESCE(MAX(numero), 0) + 1, ?, ?, ? FROM eventos WHERE trabajo_id = ?`, id, tipo, datos, fecha(cuando), id)
	return err
}

func fecha(t time.Time) string { return t.UTC().Format(formatoFecha) }
