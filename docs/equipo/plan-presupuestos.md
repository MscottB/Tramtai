# Presupuestos: plan de implementación

> **Para agentes:** SUB-SKILL REQUERIDA: superpowers:subagent-driven-development (recomendada) o
> superpowers:executing-plans, tarea por tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** límites diarios y mensuales de peticiones, tokens y USD por API y por instancia, con aviso al 80 %,
corte al 100 %, reserva por adelantado y reinicio manual.

**Arquitectura:** la lógica pura (límites, fracción usada, periodos, error) vive en `nucleo`. El almacén
registra el consumo en una tabla diaria y comprueba el presupuesto dentro de la transacción de `Aceptar`
(una sola conexión SQLite: comprobación y suma son atómicas). El planificador guarda los presupuestos vigentes
(los fija el servicio en cada `Recargar`, como `LimitarPuntos`) y se los da al almacén con un callback. La API
añade el 429, `Retry-After` y la cabecera; servicio, CLI y Web UI configuran, muestran y reinician.

**Stack:** Go 1.27 (SQLite modernc, cobra), Vue 3 + Quasar.

**Spec:** docs/equipo/presupuestos.md

## Restricciones globales

- Unidades: `peticiones` (al aceptar), `tokens` (entrada + salida al terminar), `usd` (el que informa el agente).
- Periodos: día y mes de calendario en hora local (`Almacen.Zona`, por defecto `time.Local`). 0 o ausente = sin límite.
- Código de error `presupuesto_agotado`, HTTP 429, no saltable por el respaldo.
- Cabecera `X-Tramtai-Presupuesto: <entero>`; `Retry-After` en segundos hasta el reinicio del periodo agotado.
- Avisos en niveles 80 y 100, una vez por periodo (`avisos_presupuesto`).
- `presupuesto` es prefijo administrado de la configuración.
- Un fallo al leer o escribir `consumo` nunca rechaza un trabajo: se registra con `log.Printf` y se sigue.
- Consumo y avisos se purgan a los 13 meses.
- Textos nuevos: español en el código, inglés en internal/idioma/en_*.go y web/src/idiomas/en.ts (`TestCobertura`).
- Go: `"C:\Program Files\Go\bin"` puede no estar en PATH. Pruebas: `go test ./...`; Web: `cd web; npm run build`.

## Foco de revisión

1. Un trabajo aceptado un día y terminado al siguiente: sus tokens deben sumar en el día de aceptación y no dejar
   `en_curso` negativo ni inflado. → prueba en la Tarea 2.
2. Reiniciar con trabajos en curso: al terminar, recrean la fila con `terminados > peticiones`; `en_curso` se
   acota a 0. → prueba en la Tarea 1 (`Fraccion` con `Terminados > Peticiones`) y en la Tarea 2.
3. Una petición repetida con la misma clave de idempotencia no consume ni se rechaza por presupuesto: devuelve
   el trabajo original. → prueba en la Tarea 2.
4. El presupuesto de la instancia también frena los trabajos sin API (Probar, API nativa con la clave de la app)
   y el reintento de un trabajo. → prueba en la Tarea 3.
5. Varias peticiones simultáneas con 1 petición restante: solo una pasa. → prueba en la Tarea 2 (goroutines).

---

### Tarea 1: Tipos y evaluación del presupuesto en `nucleo`

**Archivos:**
- Crear: `internal/nucleo/presupuesto.go`, `internal/nucleo/presupuesto_test.go`
- Modificar: `internal/nucleo/errores.go` (código nuevo), `internal/nucleo/configuracion.go` (claves, `tipoDecimal`,
  `Decimal`, prefijo administrado, negativos), `internal/nucleo/configuracion_test.go`, `internal/idioma/en_*.go`

**Interfaces producidas:**

```go
const CodigoPresupuestoAgotado CodigoError = "presupuesto_agotado"

type Limites struct {
	Peticiones int     `json:"peticiones,omitempty" yaml:"peticiones,omitempty"`
	Tokens     int     `json:"tokens,omitempty" yaml:"tokens,omitempty"`
	USD        float64 `json:"usd,omitempty" yaml:"usd,omitempty"`
}
type Presupuesto struct {
	Diario  Limites `json:"diario,omitzero" yaml:"diario,omitempty"`
	Mensual Limites `json:"mensual,omitzero" yaml:"mensual,omitempty"`
}
func (p Presupuesto) Vacio() bool
func (p Presupuesto) Validar() error // configuracion_invalida si algún valor < 0

type Consumo struct {
	Peticiones int     `json:"peticiones"`
	Terminados int     `json:"terminados"`
	Tokens     int     `json:"tokens"`
	USD        float64 `json:"usd"`
}
// Fraccion es la mayor fracción usada entre los límites definidos y su unidad ("peticiones", "tokens", "usd");
// 0 y "" sin límites. Con reservar, tokens y usd suman max(0, Peticiones−Terminados) × (consumido/Terminados)
// (nada si Terminados == 0).
func (l Limites) Fraccion(c Consumo, reservar bool) (float64, string)

// Periodos da las claves del día ("2006-01-02") y del mes ("2006-01") de ahora en zona, y cuándo reinician.
func Periodos(ahora time.Time, zona *time.Location) (dia, mes string, finDia, finMes time.Time)

// PresupuestoAgotado envuelve un *Error con código presupuesto_agotado (CodigoDe y MensajeDe lo ven por Unwrap).
type PresupuestoAgotado struct{ Err *Error; Reinicio time.Time }
func (e *PresupuestoAgotado) Error() string
func (e *PresupuestoAgotado) Unwrap() error

// Comprobar devuelve *PresupuestoAgotado si un límite del día o del mes tiene Fraccion(reservar=true) >= 1.
// ambito va en el mensaje: idioma.Tf("de la API %s", nombre) o idioma.T("de la instancia").
func (p Presupuesto) Comprobar(dia, mes Consumo, ahora time.Time, zona *time.Location, ambito string) error
// Porcentaje es floor(100 × la mayor Fraccion(reservar=false) de los dos periodos); false si Vacio.
func (p Presupuesto) Porcentaje(dia, mes Consumo) (int, bool)

type EstadoPeriodo struct {
	Limites  Limites   `json:"limites"`
	Consumo  Consumo   `json:"consumo"`
	Reinicio time.Time `json:"reinicio"`
}
type EstadoPresupuesto struct {
	Diario  EstadoPeriodo `json:"diario"`
	Mensual EstadoPeriodo `json:"mensual"`
}

// PresupuestoDe lee presupuesto.{diario,mensual}.{peticiones,tokens,usd} de la configuración efectiva.
func PresupuestoDe(c *Configuracion) Presupuesto
func (c *Configuracion) Decimal(clave string) float64 // acepta int o float64
```

Mensaje de `Comprobar` (copia exacta, con `idioma.Tf`): `"presupuesto %s de %s %s agotado"` con periodo
(`diario`/`mensual`), unidad y ámbito, p. ej. "presupuesto mensual de tokens de la API facturas agotado".

- [ ] **Paso 1: pruebas que fallan** en `presupuesto_test.go`:
  - `TestFraccion`: `Limites{Peticiones: 10}.Fraccion(Consumo{Peticiones: 8}, false)` → `0.8, "peticiones"`;
    `Limites{Tokens: 1000}` con `Consumo{Peticiones: 3, Terminados: 1, Tokens: 300}`: sin reservar `0.3`, con
    reservar `0.9` (300 + 2×300); con `Terminados: 0` y reservar → `0`; con `Peticiones: 1, Terminados: 2,
    Tokens: 500` y reservar → `0.5` (en curso acotado a 0); límites vacíos → `0, ""`.
  - `TestPeriodos`: con zona `time.FixedZone("x", -3*3600)` y ahora `2026-10-31T02:30Z` → dia `2026-10-30`, mes
    `2026-10`, finDia `2026-10-31T00:00-03:00`, finMes `2026-11-01T00:00-03:00`.
  - `TestComprobar`: agotado por peticiones diarias → `CodigoDe(err) == CodigoPresupuestoAgotado`, `Reinicio ==
    finDia`, el mensaje contiene "diario" y el ámbito; agotado en el mes → `Reinicio == finMes`; sin agotar → nil.
  - `TestPorcentaje`: 85 % de tokens mensuales y 10 % de peticiones diarias → `85, true`; vacío → `0, false`.
  - `TestValidarPresupuesto`: `USD: -1` → error `configuracion_invalida`.
- [ ] **Paso 2:** `go test ./internal/nucleo -run 'Fraccion|Periodos|Comprobar|Porcentaje|ValidarPresupuesto'` → FAIL (no compila).
- [ ] **Paso 3:** implementar `presupuesto.go` y el código en `errores.go`.
- [ ] **Paso 4: configuración.** En `configuracion.go`: `tipoDecimal` (acepta `int` o `float64`; `convertir` usa
  `strconv.ParseFloat`), las seis claves `presupuesto.{diario,mensual}.{peticiones,tokens}` (`tipoEntero`) y
  `presupuesto.{diario,mensual}.usd` (`tipoDecimal`), `"presupuesto"` en `prefijosAdministrados`, y rechazo de
  valores negativos en estas claves donde se validan los tipos. Prueba `TestPresupuestoConfiguracion`:
  `PresupuestoDe` lee `usd: 50` (int) y `usd: 12.5`; la capa del repositorio con `presupuesto.mensual.tokens` da
  error; `-5` da error.
- [ ] **Paso 5:** traducciones nuevas en `internal/idioma/en_servicio.go`; `go test ./internal/nucleo ./internal/idioma` → PASS.
- [ ] **Paso 6:** commit `feat(nucleo): presupuesto por día y mes con reserva por adelantado`.

### Tarea 2: Consumo en el almacén

**Archivos:**
- Modificar: `internal/persistencia/almacen.go` (migración v7, `Aceptar`, `CambiarEstado`, `Purgar`, campos de `Almacen`)
- Crear: `internal/persistencia/consumo.go` (funciones nuevas y auxiliares), `internal/persistencia/consumo_test.go`

**Interfaces:**
- Consume: Tarea 1 (`Presupuesto`, `Consumo`, `Periodos`, `Comprobar`, `Fraccion`).
- Produce:

```go
const TodaLaInstancia = "*" // ámbito de un Tope y de Consumo: todas las filas de la instancia (con y sin API)

type Tope struct {
	Punto       string // nombre de la API o TodaLaInstancia
	Presupuesto nucleo.Presupuesto
}
// En Almacen:
//   Topes func(instancia, punto string) []Tope // presupuestos que aplican a un trabajo de ese punto ("" = sin API); nil = ninguno
//   Zona  *time.Location                       // Abrir la deja en time.Local
func (a *Almacen) Consumo(ctx context.Context, instancia, punto string) (dia, mes nucleo.Consumo, err error)
func (a *Almacen) ReiniciarConsumo(ctx context.Context, instancia, punto string) error
func (a *Almacen) EliminarConsumo(ctx context.Context, instancia, punto string) error
```

Migración v7 (añadir a `migraciones`): las dos tablas de la spec con `NOT NULL DEFAULT 0` en los contadores.
En `Aceptar`, tras la búsqueda de idempotencia y la comprobación de cola y antes del `INSERT`: para cada `Tope`
de `a.Topes(t.Instancia, t.PuntoAcceso)`, leer su consumo dentro de la transacción y llamar a `Comprobar`
(ámbito `idioma.Tf("de la API %s", punto)` o `idioma.T("de la instancia")`); tras el `INSERT`,
`INSERT … ON CONFLICT DO UPDATE SET peticiones = peticiones + 1` en la fila `(instancia, punto_acceso, dia)` y
registrar los cruces. En `CambiarEstado`, si `hacia.Terminal()`: leer `instancia, punto_acceso, creado_en` del
trabajo en la transacción, sumar `terminados + 1`, tokens (`TokensEntrada + TokensSalida` de `cambio.Uso`, 0 si
es nil) y USD en la fila del día local de `creado_en`, y registrar cruces. Los cruces (`registrarCruces`): por
cada tope y periodo con límites, `Fraccion(reservar=false)` ≥ 0.8 → `INSERT OR IGNORE` nivel 80; ≥ 1 → nivel
100; `periodo` = `"D:"+dia` o `"M:"+mes`; `punto` = `Tope.Punto`. Cualquier error de estas consultas:
`log.Printf` y seguir (restricción global).

`ReiniciarConsumo` borra las filas del mes en curso (que incluyen el día) del punto, o de toda la instancia con
`TodaLaInstancia`, y los avisos de esos `D:`/`M:`. `EliminarConsumo` borra todas las filas y avisos del punto.
`Purgar` añade: `DELETE FROM consumo WHERE instancia = ? AND dia < ?` con el primer día del mes de hace 12 meses
(se conservan el mes en curso y los 12 anteriores), y lo mismo en avisos con `en`.

- [ ] **Paso 1: pruebas que fallan** en `consumo_test.go` (reloj y `Zona` fijos, `Topes` de prueba):
  - `TestConsumoAceptarYTerminar`: aceptar 2 trabajos y terminar 1 con `Uso{100, 50, 0.2}` → `dia ==
    Consumo{2, 1, 150, 0.2}` y el mismo `mes`.
  - `TestConsumoDiaDeAceptacion`: aceptar a las 23:59 y terminar a las 00:01 del día siguiente → los tokens
    están en la fila del primer día; el segundo día tiene `Consumo{}`.
  - `TestPresupuestoAgotadoEnAceptar`: `Tope{"facturas", Diario{Peticiones: 2}}` → la tercera `Aceptar` da
    `presupuesto_agotado` y no inserta el trabajo.
  - `TestPresupuestoIdempotencia`: con 1 petición restante, repetir la misma clave y huella devuelve el
    original (`reutilizado == true`) sin error y `peticiones` sigue en 1.
  - `TestPresupuestoConcurrente`: límite 1 y 5 goroutines con `Aceptar` → exactamente 1 sin error.
  - `TestPresupuestoReserva`: `Tokens: 1000`, 1 terminado con 400 y 1 en curso → la siguiente pasa
    (400 + 1×400 = 800); con 2 en curso se rechaza (400 + 2×400 ≥ 1000).
  - `TestPresupuestoInstancia`: `Tope{TodaLaInstancia, …}` suma trabajos con `PuntoAcceso` "" y "facturas".
  - `TestAvisosPresupuesto`: cruzar el 80 % y luego el 100 % inserta exactamente 2 filas; volver a sumar no
    añade más.
  - `TestReiniciarConsumo`: tras reiniciar, `Consumo` del día y del mes es cero, los avisos de esos periodos
    desaparecen y terminar un trabajo que estaba en curso deja `Consumo{0, 1, …}` sin error.
  - `TestPurgarConsumo`: filas de hace 13 meses se borran; las de hace 12 se conservan.
- [ ] **Paso 2:** `go test ./internal/persistencia -run 'Consumo|Presupuesto|Avisos|Purgar'` → FAIL.
- [ ] **Paso 3:** implementar la migración, los cambios de `Aceptar`, `CambiarEstado` y `Purgar`, y `consumo.go`.
- [ ] **Paso 4:** `go test ./internal/persistencia` → PASS (también las pruebas existentes de migración).
- [ ] **Paso 5:** commit `feat(persistencia): consumo diario, comprobación de presupuesto y avisos`.

### Tarea 3: Planificador

**Archivos:** Modificar `internal/planificador/planificador.go`, `internal/planificador/planificador_test.go`

**Interfaces producidas** (los presupuestos se guardan en un `atomic.Pointer[map[string]nucleo.Presupuesto]`:
el almacén llama a `Topes` dentro de sus transacciones y no debe esperar a `p.mu`):

```go
// FijarPresupuestos fija los presupuestos vigentes: por nombre de API y, con clave persistencia.TodaLaInstancia,
// el de la instancia. Los vacíos se omiten.
func (p *Planificador) FijarPresupuestos(m map[string]nucleo.Presupuesto)
func (p *Planificador) EstadoPresupuesto(ctx context.Context, punto string) (nucleo.EstadoPresupuesto, bool, error)
// PorcentajePresupuesto es el mayor entre el de la API y el de la instancia; false si ninguno tiene presupuesto o falla la lectura.
func (p *Planificador) PorcentajePresupuesto(ctx context.Context, punto string) (int, bool)
func (p *Planificador) ReiniciarPresupuesto(ctx context.Context, punto string) error
func (p *Planificador) EliminarConsumo(ctx context.Context, punto string) error
```

`Nuevo` asigna `almacen.Topes` a un método que devuelve el tope de la API (si `punto != ""` y tiene) y el de la
instancia (si tiene).

- [ ] **Paso 1: pruebas que fallan:**
  - `TestPresupuestoInstanciaSinAPI`: `FijarPresupuestos({TodaLaInstancia: Diario{Peticiones: 1}})`; `Enviar` 1 OK,
    2.º → `presupuesto_agotado`; `EnviarPorPunto` también se rechaza; `Reintentar` del primero, terminado → rechazado.
  - `TestPresupuestoNoSalta`: con respaldo configurado, un trabajo rechazado por presupuesto no se crea (no hay
    salto: el rechazo es al aceptar).
  - `TestPorcentajePresupuesto`: API al 50 % e instancia al 90 % → `90, true`; sin presupuestos → `false`.
- [ ] **Paso 2:** `go test ./internal/planificador -run Presupuesto` → FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** `go test ./internal/planificador` → PASS.
- [ ] **Paso 5:** commit `feat(planificador): presupuestos vigentes por API e instancia`.

### Tarea 4: API: 429, `Retry-After` y cabecera

**Archivos:** Modificar `internal/api/api.go`; pruebas en `internal/api/api_test.go`

- `estadosHTTP[nucleo.CodigoPresupuestoAgotado] = http.StatusTooManyRequests`.
- `EscribirError`: con `errors.As` a `*nucleo.PresupuestoAgotado`, `Retry-After` =
  `max(1, ceil(segundos hasta Reinicio))`.
- `Rutas` envuelve el `mux`: en peticiones `POST`, si `s.Planificador.PorcentajePresupuesto(ctx, nombre)`
  (nombre de `s.Punto` o "") da `ok`, fija `X-Tramtai-Presupuesto` antes de llamar al manejador.

- [ ] **Paso 1: pruebas que fallan:** `TestPresupuestoAgotadoFachadas` (tabla con `/api/v1/trabajos`,
  `/openai/v1/chat/completions`, `/anthropic/v1/messages`): con el presupuesto agotado → 429; nativa
  `error.codigo == "presupuesto_agotado"`; OpenAI `error.code == "presupuesto_agotado"` y `type ==
  "rate_limit_error"`; Anthropic `error.type == "rate_limit_error"`; `Retry-After` > 0 en las tres.
  `TestCabeceraPresupuesto`: al 50 % → `X-Tramtai-Presupuesto: 50`; sin presupuesto → cabecera ausente.
- [ ] **Paso 2:** `go test ./internal/api -run Presupuesto` → FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** `go test ./internal/api` → PASS.
- [ ] **Paso 5:** commit `feat(api): 429 presupuesto_agotado con Retry-After y cabecera de consumo`.

### Tarea 5: Servicio: configuración, recarga y rutas de administración

**Archivos:** Modificar `internal/servicio/puntos.go`, `internal/servicio/web.go`; pruebas en
`internal/servicio/servicio_test.go` / `web_test.go`

- `ConfigPunto.Presupuesto *nucleo.Presupuesto` (`json/yaml:"presupuesto,omitempty"`); `Normalizar` llama a
  `Validar` y deja `nil` si `Vacio()`. (`api.Punto` no lo necesita: el planificador lo aplica.)
- `Recargar`: junto a `LimitarPuntos`, `inst.Planificador.FijarPresupuestos(m)` con el presupuesto de cada API
  activa de la instancia y `m[persistencia.TodaLaInstancia] = nucleo.PresupuestoDe(inst.Config)`.
- `GET /api/v1/puntos`: añade `presupuestos: {<nombre>: EstadoPresupuesto}` solo para las API con presupuesto
  (la spec lo ponía bajo `uso`, pero `uso` sale de otra consulta con retención de 7 días).
- `GET /api/v1/instancia/presupuesto` → `{"presupuesto": EstadoPresupuesto}` o `{}` sin presupuesto.
- `POST /api/v1/instancia/presupuesto/reiniciar` → 204 (`ReiniciarPresupuesto(TodaLaInstancia)`).
- `puntoUI`: `accion == "presupuesto/reiniciar" && POST` → 204; `PATCH` acepta `presupuesto` (`null` lo quita);
  `DELETE` llama también a `EliminarConsumo`.

- [ ] **Paso 1: pruebas que fallan:** `TestPresupuestoPuntoUI`: crear una API con `presupuesto.diario.peticiones:
  1`, hacer una petición, `GET /api/v1/puntos` muestra `consumo.peticiones == 1`; `POST
  …/presupuesto/reiniciar` → 204 y vuelve a 0; `PATCH {"presupuesto": null}` lo quita; sin sesión privilegiada →
  401. `TestNormalizarPresupuestoNegativo` → error. `TestPresupuestoInstanciaUI`: con
  `presupuesto.mensual.peticiones: 5` en la configuración global, `GET /api/v1/instancia/presupuesto` devuelve
  los límites.
- [ ] **Paso 2:** `go test ./internal/servicio -run Presupuesto` → FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** `go test ./internal/servicio` → PASS.
- [ ] **Paso 5:** commit `feat(servicio): presupuestos en la configuración de API e instancia y rutas de reinicio`.

### Tarea 6: CLI

**Archivos:** Modificar `cmd/tramtai/acceso.go`, `internal/cliente/cliente.go`, `internal/idioma/en_cli.go`;
prueba en `cmd/tramtai/*_test.go` (donde se prueban hoy las banderas de `acceso`)

- Bandera `--presupuesto` (`StringSliceVar`, repetible) en crear y editar: `periodo.unidad=valor` con periodo
  `diario|mensual` y unidad `peticiones|tokens|usd`. Función `aplicarPresupuesto(p *nucleo.Presupuesto, entradas
  []string) error`; valor 0 quita ese límite.
- `tramtai acceso reiniciar-presupuesto [nombre]` (sin nombre = la instancia; `--instancia` como en `activar`):
  `Cliente.ReiniciarPresupuesto(ctx, instancia, nombre string) error` → POST a la ruta de la Tarea 5. Si el
  servicio no está en marcha, error claro (el consumo está en su base).
- `acceso listar` muestra el presupuesto de cada API en una línea (`diario: 500 peticiones; mensual: 50 USD`).

- [ ] **Paso 1: prueba que falla** `TestAplicarPresupuesto`: `["diario.peticiones=500", "mensual.usd=12.5"]` →
  `Presupuesto{Diario{Peticiones: 500}, Mensual{USD: 12.5}}`; `"mensual.tokens=0"` quita el límite;
  `"semanal.tokens=1"`, `"diario.horas=1"`, `"diario.tokens=-1"` y `"diario.tokens"` → error.
- [ ] **Paso 2:** `go test ./cmd/tramtai -run Presupuesto` → FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** `go test ./cmd/tramtai ./internal/cliente ./internal/idioma` → PASS.
- [ ] **Paso 5:** commit `feat(cli): --presupuesto y reiniciar-presupuesto`.

### Tarea 7: Web UI

**Archivos:**
- Crear: `web/src/componentes/BarrasPresupuesto.vue` (barras de un `EstadoPresupuesto`, botón "Reiniciar
  consumo" con confirmación que emite `reiniciar`), `web/src/componentes/EditorPresupuesto.vue` (seis campos
  numéricos, `v-model` de `Presupuesto`; nota junto a USD: "solo suma el coste que informa el agente (hoy Claude)").
- Modificar: `web/src/api.ts` (tipos `Limites`, `Presupuesto`, `Consumo`, `EstadoPresupuesto`; `puntosConUso` lee
  `presupuestos`; `presupuestoInstancia()`, `reiniciarPresupuesto(nombre?)`; `AjustesPunto` y `ConfigPunto` con
  `presupuesto`), `web/src/paginas/Accesos.vue` (barras por API y bloque de la instancia arriba;
  `EditorPresupuesto` en los diálogos de crear y de ajustes), `web/src/idiomas/es.ts`, `web/src/idiomas/en.ts`.

Barras: color `positive` por debajo del 80 %, `warning` desde el 80 %, `negative` al 100 %; texto
`consumo / límite` y "se reinicia {fecha}". Una barra por límite definido.

- [ ] **Paso 1:** implementar componentes, tipos y llamadas.
- [ ] **Paso 2:** `cd web; npm run build` → sin errores de vue-tsc; deja `internal/recursos/web` actualizado.
- [ ] **Paso 3:** verificar en el navegador (preview): crear una API con límite diario de 1 petición, lanzar una
  con curl, ver la barra roja, reiniciar y verla a 0. Captura.
- [ ] **Paso 4:** `go test ./internal/idioma ./internal/recursos/...` → PASS.
- [ ] **Paso 5:** commit `feat(web): presupuestos en Mis APIs con barras y reinicio`.

### Tarea 8: Documentación y cierre

**Archivos:** `docs/api.md` (cabecera, 429 `presupuesto_agotado`, `Retry-After`, rutas de lectura y reinicio),
`docs/operacion.md` (configuración por API e instancia, límite aproximado por la reserva, `usd` solo con
Claude), `web/src/paginas/Ayuda.vue`, `site/index.html` y `site/ayuda.html` (presupuestos como función de
equipo), `docs/equipo/pendientes.md` (quitar presupuestos si figura).

- [ ] **Paso 1:** escribir la documentación; recompilar la Web UI si cambia Ayuda.
- [ ] **Paso 2:** `gofmt -l` sobre los archivos tocados vacío (sin `-w` sobre carpetas enteras), `go vet ./...`,
  `go test ./...`, `cd web; npm run build` → todo PASS.
- [ ] **Paso 3:** `graphify update .`
- [ ] **Paso 4:** commit `docs: presupuestos por API e instancia`.
- [ ] **Paso 5:** revisión de toda la rama `feat/presupuestos` por un revisor nuevo; aplicar lo que encuentre.
