// Package persistencia guarda trabajos, eventos e idempotencia de una instancia en SQLite.
package persistencia

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // driver Go puro: sin CGO

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// formatoFecha tiene ancho fijo para que las fechas se comparen y ordenen como texto.
const formatoFecha = "2006-01-02T15:04:05.000000000Z"

// sinVencimiento marca la clave de idempotencia de un trabajo activo: no se purga mientras no termine.
const sinVencimiento = "9999-12-31T00:00:00.000000000Z"

// migraciones[i] lleva la base de la versión i a la i+1; user_version guarda la versión aplicada.
var migraciones = []string{`
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
);`,
	// v2: modelo resuelto, modelo efectivo y uso informado por el agente.
	`ALTER TABLE trabajos ADD COLUMN modelo_enviado TEXT NOT NULL DEFAULT '';
ALTER TABLE trabajos ADD COLUMN modelo_efectivo TEXT NOT NULL DEFAULT '';
ALTER TABLE trabajos ADD COLUMN tokens_entrada INTEGER NOT NULL DEFAULT 0;
ALTER TABLE trabajos ADD COLUMN tokens_salida INTEGER NOT NULL DEFAULT 0;
ALTER TABLE trabajos ADD COLUMN costo_usd REAL NOT NULL DEFAULT 0;`,
	// v3: punto de acceso que creó el trabajo.
	`ALTER TABLE trabajos ADD COLUMN punto_acceso TEXT NOT NULL DEFAULT '';`,
	// v4: retención. vencido = pasó la ventana del resultado; entradas_eliminadas = ya no se puede reintentar.
	`ALTER TABLE trabajos ADD COLUMN vencido INTEGER NOT NULL DEFAULT 0;
ALTER TABLE trabajos ADD COLUMN entradas_eliminadas INTEGER NOT NULL DEFAULT 0;`,
	// v5: proceso del agente, para terminarlo si el servicio cae y para diagnosticar cancelacion_no_confirmada.
	`ALTER TABLE trabajos ADD COLUMN pid INTEGER NOT NULL DEFAULT 0;
ALTER TABLE trabajos ADD COLUMN firma_proceso TEXT NOT NULL DEFAULT '';`,
	// v6: índices para el listado de la Web UI, la purga por terminado_en y el vencimiento de idempotencia.
	`CREATE INDEX IF NOT EXISTS trabajos_instancia_creado ON trabajos (instancia, creado_en);
CREATE INDEX IF NOT EXISTS trabajos_instancia_terminado ON trabajos (instancia, terminado_en);
CREATE INDEX IF NOT EXISTS idempotencia_trabajo ON idempotencia (trabajo_id);`,
	// v7: presupuestos. consumo por API y día de aceptación (punto '' = sin API); avisos al 80 % y al 100 % por
	// periodo (D:AAAA-MM-DD o M:AAAA-MM), una vez cada uno. La retención de trabajos no los toca.
	`CREATE TABLE IF NOT EXISTS consumo (
	instancia TEXT NOT NULL,
	punto TEXT NOT NULL,
	dia TEXT NOT NULL,
	peticiones INTEGER NOT NULL DEFAULT 0,
	terminados INTEGER NOT NULL DEFAULT 0,
	tokens INTEGER NOT NULL DEFAULT 0,
	usd REAL NOT NULL DEFAULT 0,
	PRIMARY KEY (instancia, punto, dia)
);
CREATE TABLE IF NOT EXISTS avisos_presupuesto (
	instancia TEXT NOT NULL,
	punto TEXT NOT NULL,
	periodo TEXT NOT NULL,
	nivel INTEGER NOT NULL,
	en TEXT NOT NULL,
	PRIMARY KEY (instancia, punto, periodo, nivel)
);`,
	// v8: de dónde llegó cada petición (Probar, terminal, CLI, app o API); '' en los trabajos anteriores.
	`ALTER TABLE trabajos ADD COLUMN origen TEXT NOT NULL DEFAULT '';`,
}

const columnas = `id, instancia, estado, codigo_error, mensaje_error, solicitud, huella, salida, intento_de, creado_en, actualizado_en, terminado_en,
	modelo_enviado, modelo_efectivo, tokens_entrada, tokens_salida, costo_usd, punto_acceso, vencido, entradas_eliminadas, pid, firma_proceso, origen`

// Almacen es la base SQLite de una instancia.
type Almacen struct {
	db *sql.DB
	// VentanaIdempotencia es el tiempo que una clave sigue vinculada a su trabajo tras terminar.
	VentanaIdempotencia time.Duration
	reloj               func() time.Time
	// Compactada indica que Abrir pasó la base a auto_vacuum incremental (un VACUUM único, al actualizar).
	Compactada bool
	// Topes da los presupuestos que se aplican a un trabajo de la API punto ("" = sin API); nil = ninguno.
	Topes func(instancia, punto string) []Tope
	// Zona fija el día y el mes de los presupuestos (hora local del equipo).
	Zona *time.Location

	muAviso sync.Mutex
	aviso   chan struct{}
}

// Avisos devuelve un canal que se cierra en la próxima escritura confirmada (trabajo, estado o evento).
// Quien sigue un trabajo lo pide antes de leer y espera en él en lugar de sondear la base.
func (a *Almacen) Avisos() <-chan struct{} {
	a.muAviso.Lock()
	defer a.muAviso.Unlock()
	if a.aviso == nil {
		a.aviso = make(chan struct{})
	}
	return a.aviso
}

// avisar despierta a quienes esperan en Avisos.
func (a *Almacen) avisar() {
	a.muAviso.Lock()
	defer a.muAviso.Unlock()
	if a.aviso != nil {
		close(a.aviso)
		a.aviso = nil
	}
}

// confirmar confirma la transacción y avisa a quienes siguen los trabajos.
func (a *Almacen) confirmar(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisar()
	return nil
}

// Cambio son los datos opcionales que acompañan una transición.
type Cambio struct {
	Codigo  nucleo.CodigoError
	Mensaje string
	Salida  *string
	// Uso y ModeloEfectivo se guardan cuando Uso no es nil (al terminar una ejecución).
	Uso            *nucleo.Uso
	ModeloEfectivo string
}

// Evento es un registro numerado de un trabajo.
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
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(ruta)+"?_pragma=auto_vacuum(2)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	// auto_vacuum va antes del WAL: en una base nueva se fija sin VACUUM (en una vieja no hace nada; ver compactable).
	// Una sola conexión serializa las escrituras de la instancia y conserva los PRAGMA.
	db.SetMaxOpenConns(1)
	if err := migrar(db); err != nil {
		db.Close()
		return nil, err
	}
	compactada := compactable(db)
	return &Almacen{db: db, VentanaIdempotencia: 24 * time.Hour, reloj: time.Now, Zona: time.Local, Compactada: compactada}, nil
}

// compactable pasa la base a auto_vacuum incremental (con un VACUUM único, fuera de transacción) para que la purga
// devuelva espacio al disco. Si falla (disco lleno), la base se abre igual y se reintenta en el próximo arranque.
func compactable(db *sql.DB) bool {
	var modo int
	if db.QueryRow("PRAGMA auto_vacuum").Scan(&modo) != nil || modo == 2 {
		return false
	}
	if _, err := db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		return false
	}
	db.Exec("VACUUM")
	db.Exec("PRAGMA wal_checkpoint(TRUNCATE)") // el VACUUM deja un WAL del tamaño de la base hasta el cierre
	return true
}

func migrar(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(migraciones) {
		return fmt.Errorf(idioma.T("versión de base de datos %d no soportada por este ejecutable"), version)
	}
	for ; version < len(migraciones); version++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migraciones[version]); err != nil {
			tx.Rollback()
			return fmt.Errorf(idioma.T("migración %d: %w"), version+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
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
			return t, false, nucleo.NuevoError(nucleo.CodigoConflictoIdempotencia, idioma.T("la clave de idempotencia ya se usó con otra solicitud"))
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
		return t, false, nucleo.NuevoError(nucleo.CodigoColaLlena, idioma.T("la cola de la instancia está llena"))
	}
	if err := a.comprobarPresupuesto(ctx, tx, t, ahora); err != nil {
		return t, false, err
	}
	solicitud, err := json.Marshal(t.Solicitud)
	if err != nil {
		return t, false, err
	}
	t.Estado, t.CreadoEn, t.ActualizadoEn = nucleo.EnCola, ahora, ahora
	if _, err := tx.ExecContext(ctx, `INSERT INTO trabajos (id, instancia, estado, solicitud, huella, intento_de, modelo_enviado, punto_acceso, origen, creado_en, actualizado_en) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Instancia, string(t.Estado), string(solicitud), t.Huella, t.IntentoDe, t.ModeloEnviado, t.PuntoAcceso, t.Origen, fecha(ahora), fecha(ahora)); err != nil {
		return t, false, err
	}
	if err := agregarEvento(ctx, tx, t.ID, "estado", string(nucleo.EnCola), ahora); err != nil {
		return t, false, err
	}
	a.sumarConsumo(ctx, tx, t.Instancia, t.PuntoAcceso, ahora, ahora, nucleo.Consumo{Peticiones: 1})
	if clave != "" {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO idempotencia (instancia, clave, huella, trabajo_id, vence_en) VALUES (?, ?, ?, ?, ?)`,
			t.Instancia, clave, t.Huella, t.ID, sinVencimiento); err != nil {
			return t, false, err
		}
	}
	return t, false, a.confirmar(tx)
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
	uso, conUso := nucleo.Uso{}, cambio.Uso != nil
	if conUso {
		uso = *cambio.Uso
	}
	resultado, err := tx.ExecContext(ctx, `UPDATE trabajos SET estado = ?, codigo_error = ?, mensaje_error = ?,
		salida = CASE WHEN ? THEN ? ELSE salida END, actualizado_en = ?, terminado_en = COALESCE(?, terminado_en),
		modelo_efectivo = CASE WHEN ? THEN ? ELSE modelo_efectivo END,
		tokens_entrada = CASE WHEN ? THEN ? ELSE tokens_entrada END,
		tokens_salida = CASE WHEN ? THEN ? ELSE tokens_salida END,
		costo_usd = CASE WHEN ? THEN ? ELSE costo_usd END
		WHERE id = ? AND estado = ?`,
		string(hacia), string(cambio.Codigo), cambio.Mensaje, conSalida, salida, fecha(ahora), terminado,
		conUso, cambio.ModeloEfectivo, conUso, uso.TokensEntrada, conUso, uso.TokensSalida, conUso, uso.CostoUSD, id, string(de))
	if err != nil {
		return err
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		if _, err := obtener(ctx, tx, id); err != nil {
			return err
		}
		return nucleo.NuevoError(nucleo.CodigoTransicionInvalida, idioma.Tf("el trabajo %s ya no está en %s", id, de))
	}
	if err := agregarEvento(ctx, tx, id, "estado", string(hacia), ahora); err != nil {
		return err
	}
	if hacia.Terminal() {
		if _, err := tx.ExecContext(ctx, `UPDATE idempotencia SET vence_en = ? WHERE trabajo_id = ?`,
			fecha(ahora.Add(a.VentanaIdempotencia)), id); err != nil {
			return err
		}
		// El consumo va al día de aceptación: así peticiones − terminados de una fila es lo que sigue en curso.
		// Sin uso en el cambio (una cancelación), cuenta el que ya guardó el trabajo (intentos fallidos del respaldo).
		var instancia, punto, creado string
		var guardado nucleo.Uso
		if err := tx.QueryRowContext(ctx, `SELECT instancia, punto_acceso, creado_en, tokens_entrada, tokens_salida, costo_usd FROM trabajos WHERE id = ?`, id).
			Scan(&instancia, &punto, &creado, &guardado.TokensEntrada, &guardado.TokensSalida, &guardado.CostoUSD); err != nil {
			return err
		}
		if !conUso {
			uso = guardado
		}
		aceptado, _ := time.Parse(formatoFecha, creado)
		a.sumarConsumo(ctx, tx, instancia, punto, aceptado, ahora, nucleo.Consumo{Terminados: 1, Tokens: uso.TokensEntrada + uso.TokensSalida, USD: uso.CostoUSD})
	}
	return a.confirmar(tx)
}

// Eliminar borra un trabajo terminado con sus eventos y su clave de idempotencia.
func (a *Almacen) Eliminar(ctx context.Context, id string) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	resultado, err := tx.ExecContext(ctx, `DELETE FROM trabajos WHERE id = ? AND estado IN (?, ?, ?, ?)`, id,
		string(nucleo.Completado), string(nucleo.Fallido), string(nucleo.Cancelado), string(nucleo.Interrumpido))
	if err != nil {
		return err
	}
	if filas, _ := resultado.RowsAffected(); filas == 0 {
		if _, err := obtener(ctx, tx, id); err != nil {
			return err
		}
		return nucleo.NuevoError(nucleo.CodigoTrabajoNoTerminado, idioma.T("solo se elimina un trabajo terminado; cancélalo antes"))
	}
	for _, consulta := range []string{`DELETE FROM eventos WHERE trabajo_id = ?`, `DELETE FROM idempotencia WHERE trabajo_id = ?`} {
		if _, err := tx.ExecContext(ctx, consulta, id); err != nil {
			return err
		}
	}
	return a.confirmar(tx)
}

// EliminarTerminados borra todos los trabajos terminados de la instancia con sus eventos y claves de
// idempotencia; los que están en cola o en ejecución se conservan. Devuelve cuántos borró.
func (a *Almacen) EliminarTerminados(ctx context.Context, instancia string) (int, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	terminados := `SELECT id FROM trabajos WHERE instancia = ? AND estado IN (?, ?, ?, ?)`
	args := []any{instancia, string(nucleo.Completado), string(nucleo.Fallido), string(nucleo.Cancelado), string(nucleo.Interrumpido)}
	for _, consulta := range []string{`DELETE FROM eventos WHERE trabajo_id IN (` + terminados + `)`,
		`DELETE FROM idempotencia WHERE trabajo_id IN (` + terminados + `)`} {
		if _, err := tx.ExecContext(ctx, consulta, args...); err != nil {
			return 0, err
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM trabajos WHERE id IN (`+terminados+`)`, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), a.confirmar(tx)
}

// RegistrarProceso guarda el PID y la firma de inicio del proceso del agente de un trabajo.
func (a *Almacen) RegistrarProceso(ctx context.Context, id string, pid int, firma string) error {
	_, err := a.db.ExecContext(ctx, `UPDATE trabajos SET pid = ?, firma_proceso = ? WHERE id = ?`, pid, firma, id)
	return err
}

// CambiarAgente apunta el trabajo al agente de respaldo que lo ejecuta ahora, con su modelo (el del principal o su
// perfil no valen para otro agente, también al reintentar) y el respaldo que queda tras él, para que un trabajo
// reencolado o reintentado no vuelva a probar los que ya fallaron; con el esfuerzo propio de ese respaldo, si lo tiene.
// La huella no cambia.
func (a *Almacen) CambiarAgente(ctx context.Context, id, agente, modeloEnviado, esfuerzo string, restante []string) error {
	lista, err := json.Marshal(restante)
	if err != nil {
		return err
	}
	if restante == nil {
		lista = []byte("[]")
	}
	// Un esfuerzo vacío conserva el de la solicitud: ese respaldo heredaba el del principal.
	conEsfuerzo, args := "", []any{agente, modeloEnviado, string(lista)}
	if esfuerzo != "" {
		conEsfuerzo, args = ", '$.esfuerzo', ?", append(args, esfuerzo)
	}
	r, err := a.db.ExecContext(ctx, `UPDATE trabajos SET solicitud = json_set(json_remove(solicitud, '$.perfil_modelo'), '$.agente', ?, '$.modelo', ?,
		'$.respaldo', json(?)`+conEsfuerzo+`), modelo_enviado = ? WHERE id = ?`, append(args, modeloEnviado, id)...)
	if err != nil {
		return err
	}
	if n, err := r.RowsAffected(); err == nil && n == 0 {
		return nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("trabajo no encontrado: %s", id))
	}
	a.avisar()
	return nil
}

// AgregarEvento añade un evento con el siguiente número del trabajo.
func (a *Almacen) AgregarEvento(ctx context.Context, id, tipo, datos string) error {
	if err := agregarEvento(ctx, a.db, id, tipo, datos, a.reloj().UTC()); err != nil {
		return err
	}
	a.avisar()
	return nil
}

// Obtener devuelve un trabajo o no_encontrado.
func (a *Almacen) Obtener(ctx context.Context, id string) (nucleo.Trabajo, error) {
	return obtener(ctx, a.db, id)
}

// Estado devuelve la instancia y el estado de un trabajo, o no_encontrado; es la lectura barata de quien lo sigue.
func (a *Almacen) Estado(ctx context.Context, id string) (string, nucleo.Estado, error) {
	var instancia, estado string
	err := a.db.QueryRowContext(ctx, `SELECT instancia, estado FROM trabajos WHERE id = ?`, id).Scan(&instancia, &estado)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("trabajo no encontrado: %s", id))
	}
	return instancia, nucleo.Estado(estado), err
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

// Listar devuelve los trabajos más recientes de la instancia; con punto, solo los de ese punto de acceso.
// FiltroTrabajos acota Listar; los campos vacíos no filtran. Puntos limita a esas APIs (no vacío).
type FiltroTrabajos struct {
	Punto   string
	Puntos  []string
	Origen  string
	Antes   time.Time // solo los creados antes, para pedir la página siguiente
	AntesID string    // con Antes: el último que ya se vio; los empatados en creado_en siguen por orden de llegada
}

func (a *Almacen) Listar(ctx context.Context, instancia string, f FiltroTrabajos, limite int) ([]nucleo.Trabajo, error) {
	donde, args := "instancia = ?", []any{instancia}
	if f.Punto != "" {
		donde, args = donde+" AND punto_acceso = ?", append(args, f.Punto)
	}
	if len(f.Puntos) > 0 {
		donde += " AND punto_acceso IN (?" + strings.Repeat(", ?", len(f.Puntos)-1) + ")"
		for _, p := range f.Puntos {
			args = append(args, p)
		}
	}
	if f.Origen != "" {
		donde, args = donde+" AND origen = ?", append(args, f.Origen)
	}
	if !f.Antes.IsZero() && f.AntesID != "" {
		donde, args = donde+" AND (creado_en < ? OR (creado_en = ? AND rowid < (SELECT rowid FROM trabajos WHERE id = ?)))", append(args, fecha(f.Antes), fecha(f.Antes), f.AntesID)
	} else if !f.Antes.IsZero() {
		donde, args = donde+" AND creado_en < ?", append(args, fecha(f.Antes))
	}
	filas, err := a.db.QueryContext(ctx, `SELECT `+columnas+` FROM trabajos WHERE `+donde+` ORDER BY creado_en DESC, rowid DESC LIMIT ?`, append(args, limite)...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	trabajos := []nucleo.Trabajo{}
	for filas.Next() {
		t, err := escanear(filas)
		if err != nil {
			return nil, err
		}
		trabajos = append(trabajos, t)
	}
	return trabajos, filas.Err()
}

// Contar devuelve cuántos trabajos de la instancia hay en cada estado.
func (a *Almacen) Contar(ctx context.Context, instancia string) (map[nucleo.Estado]int, error) {
	filas, err := a.db.QueryContext(ctx, `SELECT estado, COUNT(*) FROM trabajos WHERE instancia = ? GROUP BY estado`, instancia)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	conteo := map[nucleo.Estado]int{}
	for filas.Next() {
		var estado string
		var n int
		if err := filas.Scan(&estado, &n); err != nil {
			return nil, err
		}
		conteo[nucleo.Estado(estado)] = n
	}
	return conteo, filas.Err()
}

// Recuperar se ejecuta al arrancar: lo que tenía un agente en marcha pasa a interrumpido (nunca se relanza) y lo
// que no lo tenía vuelve a en_cola, incluido un trabajo que esperaba al siguiente agente del respaldo.
// shortcut: las entradas pueden viajar dentro de la solicitud y siempre están disponibles; con cargas
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
			err = a.CambiarEstado(ctx, id, t.Estado, nucleo.Interrumpido, Cambio{Mensaje: idioma.T("el servicio se reinició sin poder confirmar la ejecución")})
			interrumpidos++
		}
		if err != nil {
			return interrumpidos, reencolados, err
		}
	}
	return interrumpidos, reencolados, nil
}

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
	// Los eventos de estado y de intento se conservan con los metadatos; los demás llevan contenido. Solo se miran los
	// trabajos que vencen ahora: los ya vencidos se limpiaron en su purga. Con <= vence también lo terminado en el
	// mismo instante (retención 0): en Windows el reloj avanza a saltos.
	if _, err := tx.ExecContext(ctx, `DELETE FROM eventos WHERE tipo NOT IN ('estado', 'intento') AND trabajo_id IN
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
	a.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&paginas)
	a.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&tamano)
	u.Bytes = paginas * tamano
	return u, nil
}

// UsoPunto resume las peticiones de un punto de acceso que conserva la retención.
type UsoPunto struct {
	Peticiones int        `json:"peticiones"`
	Errores    int        `json:"errores"`
	Ultima     *time.Time `json:"ultima,omitempty"`
}

// UsoPorPunto cuenta las peticiones, las fallidas o interrumpidas y la última, por punto de acceso.
func (a *Almacen) UsoPorPunto(ctx context.Context, instancia string) (map[string]UsoPunto, error) {
	filas, err := a.db.QueryContext(ctx, `SELECT punto_acceso, COUNT(*), SUM(CASE WHEN estado IN (?, ?) THEN 1 ELSE 0 END), MAX(creado_en)
		FROM trabajos WHERE instancia = ? AND punto_acceso <> '' GROUP BY punto_acceso`,
		string(nucleo.Fallido), string(nucleo.Interrumpido), instancia)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	uso := map[string]UsoPunto{}
	for filas.Next() {
		var punto, ultima string
		var u UsoPunto
		if err := filas.Scan(&punto, &u.Peticiones, &u.Errores, &ultima); err != nil {
			return nil, err
		}
		if t, err := time.Parse(formatoFecha, ultima); err == nil {
			u.Ultima = &t
		}
		uso[punto] = u
	}
	return uso, filas.Err()
}

// ModelosEfectivos devuelve, por "agente/modelo enviado" (modelo vacío: el predeterminado del CLI), el último
// modelo efectivo que informó el agente; así la Web UI muestra la versión real detrás de un alias como "sonnet".
func (a *Almacen) ModelosEfectivos(ctx context.Context) (map[string]string, error) {
	filas, err := a.db.QueryContext(ctx, `SELECT COALESCE(json_extract(solicitud, '$.agente'), ''), modelo_enviado, modelo_efectivo
		FROM trabajos WHERE modelo_efectivo <> '' ORDER BY creado_en DESC LIMIT 500`)
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

// CargasEnUso devuelve las cargas que algún trabajo puede necesitar todavía: activos o terminados que aún
// admiten reintento. La purga de cargas no las toca aunque hayan vencido.
func (a *Almacen) CargasEnUso(ctx context.Context, instancia string) (map[string]bool, error) {
	filas, err := a.db.QueryContext(ctx, `SELECT DISTINCT j.value FROM trabajos t, json_each(t.solicitud, '$.cargas') j
		WHERE t.instancia = ? AND t.entradas_eliminadas = 0`, instancia)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	enUso := map[string]bool{}
	for filas.Next() {
		var id string
		if err := filas.Scan(&id); err != nil {
			return nil, err
		}
		enUso[id] = true
	}
	return enUso, filas.Err()
}

func obtener(ctx context.Context, c consultor, id string) (nucleo.Trabajo, error) {
	t, err := escanear(c.QueryRowContext(ctx, `SELECT `+columnas+` FROM trabajos WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("trabajo no encontrado: %s", id))
	}
	return t, err
}

// escanear lee una fila con las columnas de "columnas"; sirve para *sql.Row y *sql.Rows.
func escanear(fila interface{ Scan(...any) error }) (nucleo.Trabajo, error) {
	var t nucleo.Trabajo
	var estado, codigo, solicitud, creado, actualizado string
	var terminado sql.NullString
	var vencido bool
	err := fila.Scan(&t.ID, &t.Instancia, &estado,
		&codigo, &t.MensajeError, &solicitud, &t.Huella, &t.Salida, &t.IntentoDe, &creado, &actualizado, &terminado,
		&t.ModeloEnviado, &t.ModeloEfectivo, &t.Uso.TokensEntrada, &t.Uso.TokensSalida, &t.Uso.CostoUSD, &t.PuntoAcceso, &vencido, &t.EntradasEliminadas, &t.PID, &t.FirmaProceso, &t.Origen)
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
	switch {
	case vencido:
		t.Disponibilidad = nucleo.SalidaExpirada
	case t.Estado == nucleo.Completado:
		t.Disponibilidad = nucleo.SalidaDisponible
	}
	return t, nil
}

func agregarEvento(ctx context.Context, e ejecutor, id, tipo, datos string, cuando time.Time) error {
	_, err := e.ExecContext(ctx, `INSERT INTO eventos (trabajo_id, numero, tipo, datos, creado_en)
		SELECT ?, COALESCE(MAX(numero), 0) + 1, ?, ?, ? FROM eventos WHERE trabajo_id = ?`, id, tipo, datos, fecha(cuando), id)
	return err
}

func fecha(t time.Time) string { return t.UTC().Format(formatoFecha) }
