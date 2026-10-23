package persistencia

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"modernc.org/sqlite"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Cambio son los datos opcionales que acompañan una transición.
type Cambio struct {
	Codigo  nucleo.CodigoError
	Mensaje string
	Salida  *string
	// Uso y ModeloEfectivo se guardan cuando Uso no es nil (al terminar una ejecución).
	Uso            *nucleo.Uso
	ModeloEfectivo string
	// DevolverPeticion, al terminar, quita la petición del consumo en lugar de contarla como terminada: no llegó a
	// ningún agente (la entrada no cabía en ninguno).
	DevolverPeticion bool
}

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
			return t, false, nucleo.NuevoError(nucleo.CodigoConflictoIdempotencia, idioma.T("la clave de idempotencia ya se usó con otra petición"))
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
		return t, false, nucleo.NuevoError(nucleo.CodigoColaLlena, idioma.T("la cola del espacio está llena"))
	}
	if err := a.comprobarPresupuesto(ctx, tx, t, ahora); err != nil {
		return t, false, err
	}
	solicitud, err := json.Marshal(t.Solicitud)
	if err != nil {
		return t, false, err
	}
	t.Estado, t.CreadoEn, t.ActualizadoEn, t.Intento = nucleo.EnCola, ahora, ahora, max(t.Intento, 1)
	if _, err := tx.ExecContext(ctx, `INSERT INTO trabajos (id, instancia, estado, solicitud, huella, intento_de, modelo_enviado, punto_acceso, origen, intento, creado_en, actualizado_en) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Instancia, string(t.Estado), string(solicitud), t.Huella, t.IntentoDe, t.ModeloEnviado, t.PuntoAcceso, t.Origen, t.Intento, fecha(ahora), fecha(ahora)); err != nil {
		return t, false, err
	}
	if err := agregarEvento(ctx, tx, t.ID, "estado", string(nucleo.EnCola), ahora); err != nil {
		return t, false, err
	}
	if err := a.sumarConsumo(ctx, tx, t.Instancia, t.PuntoAcceso, ahora, ahora, nucleo.Consumo{Peticiones: 1}); err != nil {
		if len(a.topes(t.Instancia, t.PuntoAcceso)) > 0 { // con topes, sin contador dejarían de frenar
			return t, false, a.errContador(t.PuntoAcceso, err)
		}
		a.anotar("presupuesto de %q: el contador falló; la petición se acepta porque no tiene topes: %v", t.PuntoAcceso, err)
	}
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
		return nucleo.NuevoError(nucleo.CodigoTransicionInvalida, idioma.Tf("la petición %s ya no está en %s", id, de))
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
		consumo := nucleo.Consumo{Terminados: 1, Tokens: uso.TokensEntrada + uso.TokensSalida, USD: uso.CostoUSD}
		if cambio.DevolverPeticion {
			consumo.Peticiones, consumo.Terminados = -1, 0 // peticiones − terminados sigue siendo lo que está en curso
		}
		// El costo ya se gastó: deshacer el cambio no protege el presupuesto, perdería la respuesta y haría fallar la
		// recuperación al arrancar. Solo se anota.
		if err := a.sumarConsumo(ctx, tx, instancia, punto, aceptado, ahora, consumo); err != nil {
			a.anotar("presupuesto de %q: no se pudo contar el consumo de la petición %s al terminar: %v", punto, id, err)
		}
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
		return nucleo.NuevoError(nucleo.CodigoTrabajoNoTerminado, idioma.T("solo se elimina una petición terminada; cancélala antes"))
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
// El intento sube 1: así quien lee el trabajo después sabe por qué agente de la cadena va sin haber visto el salto.
// La huella no cambia.
func (a *Almacen) CambiarAgente(ctx context.Context, id, agente, modeloEnviado, esfuerzo string, restante []string) error {
	lista, err := json.Marshal(restante)
	if err != nil {
		return err
	}
	if restante == nil {
		lista = []byte("[]")
	}
	// El esfuerzo es el que usa el agente nuevo: vacío lo quita (usa el de su CLI) en lugar de guardar un null.
	sinEsfuerzo, conEsfuerzo, args := ", '$.esfuerzo'", "", []any{agente, modeloEnviado, string(lista)}
	if esfuerzo != "" {
		sinEsfuerzo, conEsfuerzo, args = "", ", '$.esfuerzo', ?", append(args, esfuerzo)
	}
	r, err := a.db.ExecContext(ctx, `UPDATE trabajos SET solicitud = json_set(json_remove(solicitud, '$.perfil_modelo'`+sinEsfuerzo+`), '$.agente', ?, '$.modelo', ?,
		'$.respaldo', json(?)`+conEsfuerzo+`), modelo_enviado = ?, intento = intento + 1, actualizado_en = ? WHERE id = ?`, append(args, modeloEnviado, fecha(a.reloj()), id)...)
	if err != nil {
		return err
	}
	if n, err := r.RowsAffected(); err == nil && n == 0 {
		return nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("petición no encontrada: %s", id))
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
		return "", "", nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("petición no encontrada: %s", id))
	}
	return instancia, nucleo.Estado(estado), err
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
	Estados []nucleo.Estado
	// Texto busca en el último mensaje de la petición y en su id, sin distinguir mayúsculas (las de ASCII).
	Texto string
}

// registrarPlegar da a SQLite tramtai_plegar(texto), nucleo.Plegar, para buscar sin mayúsculas ni tildes en la base.
var registrarPlegar = sync.OnceValue(func() error {
	return sqlite.RegisterDeterministicScalarFunction("tramtai_plegar", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		switch v := args[0].(type) {
		case string:
			return nucleo.Plegar(v), nil
		case []byte:
			return nucleo.Plegar(string(v)), nil
		}
		return args[0], nil // NULL (sin mensajes, porque la retención los borró) sigue siendo NULL
	})
})

// comodines escapa % y _ (y la barra que los escapa) para buscarlos tal cual con LIKE … ESCAPE '\'.
var comodines = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

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
	if len(f.Estados) > 0 {
		donde += " AND estado IN (?" + strings.Repeat(", ?", len(f.Estados)-1) + ")"
		for _, e := range f.Estados {
			args = append(args, string(e))
		}
	}
	if f.Texto != "" {
		patron := "%" + comodines.Replace(nucleo.Plegar(f.Texto)) + "%"
		donde += ` AND (id LIKE ? ESCAPE '\' OR tramtai_plegar(json_extract(solicitud, '$.mensajes[#-1].contenido')) LIKE ? ESCAPE '\')`
		args = append(args, patron, patron)
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

// LimiteCambios es cuántos trabajos trae como mucho cada lectura del flujo de eventos; quien lee pide la página
// siguiente desde el último si llega llena. Es variable para que las pruebas lo bajen.
var LimiteCambios = 500

// Cursor dice dónde sigue una lectura del flujo de eventos, en el orden (actualizado_en, id): tras el trabajo ID que
// cambió en En. Con ID, los que comparten esa hora (frecuente en Windows, donde el reloj tiene poca resolución) siguen
// en la página siguiente sin perderse ni repetirse; sin ID, la lectura sigue tras la hora.
type Cursor struct {
	En time.Time
	ID string
}

// Tras es el cursor que sigue después del trabajo t.
func Tras(t nucleo.Trabajo) Cursor { return Cursor{En: t.ActualizadoEn, ID: t.ID} }

// condicionVivos son los trabajos de la instancia sin terminar y los terminados desde una fecha. La instancia va en
// cada rama, y con otro parámetro (?6, el mismo valor) para que SQLite no la saque de factor común: así usa
// trabajos_instancia_terminado en las dos ramas en vez de recorrer todo el historial de la instancia.
const condicionVivos = "((instancia = ?1 AND terminado_en IS NULL) OR (instancia = ?6 AND terminado_en >= ?2))"

// Vivos devuelve los trabajos de la instancia sin terminar y los terminados desde terminadosDesde, por orden de
// cambio y a partir de despues (cero: desde el principio): es la foto con la que empieza el flujo de eventos de la
// instancia. punto vacío: los de todas las APIs.
func (a *Almacen) Vivos(ctx context.Context, instancia, punto string, terminadosDesde time.Time, despues Cursor) ([]nucleo.Trabajo, error) {
	return a.porCambio(ctx, instancia, punto, condicionVivos, terminadosDesde, despues)
}

// CambiadosDesde devuelve los trabajos de la instancia que cambiaron después de desde (estado, agente…), por orden de
// cambio y como mucho LimiteCambios: lo que el flujo de eventos envía tras cada aviso.
func (a *Almacen) CambiadosDesde(ctx context.Context, instancia, punto string, desde Cursor) ([]nucleo.Trabajo, error) {
	return a.porCambio(ctx, instancia, punto, "", time.Time{}, desde)
}

// consultaPorCambio arma la lectura de los trabajos de la instancia cambiados después de despues, por orden de
// cambio. condicion (con ?1 y ?6 la instancia y ?2 valor) reemplaza al filtro de instancia; vacía: toda la instancia.
func consultaPorCambio(instancia, punto, condicion string, valor time.Time, despues Cursor) (string, []any) {
	donde, args, orden := "instancia = ?1", []any{instancia, fecha(valor)}, "actualizado_en"
	if condicion != "" {
		// +actualizado_en: si no, SQLite prefiere el índice que ya viene ordenado y recorre toda la instancia.
		donde, orden = condicion, "+actualizado_en"
	}
	switch {
	case despues.En.IsZero():
		args = append(args, nil)
	case despues.ID == "":
		donde, args = donde+" AND actualizado_en > ?3", append(args, fecha(despues.En))
	default:
		donde, args = donde+" AND (actualizado_en > ?3 OR (actualizado_en = ?3 AND id > ?7))", append(args, fecha(despues.En))
	}
	if punto != "" {
		donde = donde + " AND punto_acceso = ?4"
	}
	return `SELECT ` + columnas + ` FROM trabajos WHERE ` + donde + ` ORDER BY ` + orden + `, id LIMIT ?5`, append(args, punto, LimiteCambios, instancia, despues.ID)
}

func (a *Almacen) porCambio(ctx context.Context, instancia, punto, condicion string, valor time.Time, despues Cursor) ([]nucleo.Trabajo, error) {
	consulta, args := consultaPorCambio(instancia, punto, condicion, valor, despues)
	filas, err := a.db.QueryContext(ctx, consulta, args...)
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

// ContarOrigenDesde cuenta los trabajos de la instancia con ese origen creados desde desde; sinPunto deja solo los que
// llegaron con la clave de la instancia (sin API por aplicación): el aviso «dales su propia API» de Mis APIs.
func (a *Almacen) ContarOrigenDesde(ctx context.Context, instancia, origen string, sinPunto bool, desde time.Time) (int, error) {
	consulta := `SELECT COUNT(*) FROM trabajos WHERE instancia = ? AND origen = ? AND creado_en >= ?`
	if sinPunto {
		consulta += ` AND (punto_acceso IS NULL OR punto_acceso = '')`
	}
	var n int
	err := a.db.QueryRowContext(ctx, consulta, instancia, origen, fecha(desde)).Scan(&n)
	return n, err
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
// shortcut: no comprueba las cargas que cita el trabajo ni sus hashes; si ya no existen, falla al preparar la entrada.
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
			err = a.CambiarEstado(ctx, id, t.Estado, nucleo.Interrumpido, Cambio{Mensaje: idioma.T("el servicio se reinició sin poder confirmar la ejecución: vuelve a enviar la petición")})
			interrumpidos++
		}
		if err != nil {
			return interrumpidos, reencolados, err
		}
	}
	return interrumpidos, reencolados, nil
}

// Instancias devuelve las instancias que tienen trabajos en la base.
func (a *Almacen) Instancias(ctx context.Context) ([]string, error) {
	filas, err := a.db.QueryContext(ctx, `SELECT DISTINCT instancia FROM trabajos ORDER BY instancia`)
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
		return t, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("petición no encontrada: %s", id))
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
		&t.ModeloEnviado, &t.ModeloEfectivo, &t.Uso.TokensEntrada, &t.Uso.TokensSalida, &t.Uso.CostoUSD, &t.PuntoAcceso, &vencido, &t.EntradasEliminadas, &t.PID, &t.FirmaProceso, &t.Origen, &t.Intento)
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
