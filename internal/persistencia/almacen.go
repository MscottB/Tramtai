// Package persistencia guarda trabajos, eventos e idempotencia de una instancia en SQLite.
package persistencia

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // driver Go puro: sin CGO

	"tramtai/internal/idioma"
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
CREATE INDEX IF NOT EXISTS trabajos_instancia_actualizado ON trabajos (instancia, actualizado_en);
CREATE INDEX IF NOT EXISTS trabajos_instancia_terminado ON trabajos (instancia, terminado_en);
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
	// v9: el contexto (tokens) que informó cada modelo concreto, para no llamar al CLI con una entrada que no cabe.
	`CREATE TABLE IF NOT EXISTS modelos_vistos (
	agente TEXT NOT NULL,
	modelo TEXT NOT NULL,
	contexto_tokens INTEGER NOT NULL,
	visto_en TEXT NOT NULL,
	PRIMARY KEY (agente, modelo)
);`,
}

// indices son índices que no cambian el esquema: se crean al abrir sin subir la versión de la base, así que una
// versión anterior de Tramtai la sigue abriendo (al volver a una versión anterior, por ejemplo).
const indices = `CREATE INDEX IF NOT EXISTS trabajos_efectivo ON trabajos (creado_en) WHERE modelo_efectivo <> '';`

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
	// Registros recibe los fallos del contador del presupuesto (el registro del servicio); nil los manda al log.
	Registros io.Writer

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

type consultor interface {
	QueryRowContext(ctx context.Context, consulta string, args ...any) *sql.Row
}

type ejecutor interface {
	ExecContext(ctx context.Context, consulta string, args ...any) (sql.Result, error)
}

// rutaURI escribe ruta para un URI file: de SQLite: con barras y con %, ? y # escapados, que si no cortarían la ruta o
// se leerían como parámetros (una carpeta «Datos #2» o «100%»).
func rutaURI(ruta string) string {
	return strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23").Replace(filepath.ToSlash(ruta))
}

// ContarEn cuenta los trabajos de la base de ruta sin abrirla para escribir ni migrarla: la página «Espacios» lo
// enseña también de los espacios detenidos.
func ContarEn(ruta string) (int, error) {
	db, err := sql.Open("sqlite", "file:"+rutaURI(ruta)+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM trabajos`).Scan(&n)
	return n, err
}

// Abrir abre o crea la base en WAL y aplica las migraciones pendientes.
func Abrir(ruta string) (*Almacen, error) {
	if err := registrarPlegar(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+rutaURI(ruta)+"?_pragma=auto_vacuum(2)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
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
	if _, err := db.Exec(indices); err != nil {
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
	// Solo el VACUUM aplica el modo: si falló (disco lleno), la base sigue igual y se reintenta en la próxima apertura.
	return db.QueryRow("PRAGMA auto_vacuum").Scan(&modo) == nil && modo == 2
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

func fecha(t time.Time) string { return t.UTC().Format(formatoFecha) }
