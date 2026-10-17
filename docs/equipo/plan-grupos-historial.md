# Grupos, origen y límites del historial: plan de implementación

> **Para agentes:** SUB-SKILL REQUERIDA: superpowers:executing-plans (o subagent-driven-development), tarea por
> tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** grupos de APIs plegables y filtrables con el mapa por grupo, el origen de cada petición en el
historial con filtros y paginación, y límites del historial (tope, rangos, compactación, Ajustes).

**Arquitectura:** el origen viaja en el `context` de la petición (`nucleo.ConOrigenPeticion`), lo pone el servicio
al autenticar y el planificador lo guarda en `trabajos.origen`; así `Planificador.Enviar` no cambia de firma. La
lista de trabajos gana un filtro (`persistencia.FiltroTrabajos`). La retención gana `max_trabajos`, rangos en
`nucleo` y una ruta `GET/PUT /api/v1/instancia/retencion` que guarda como `fijarPredeterminado`. La Web UI usa todo
eso en Mis APIs, el mapa, el Historial, Ajustes e Inicio.

**Stack:** Go 1.27, SQLite (modernc), Vue 3 + Quasar (vue-tsc).

**Spec:** docs/equipo/grupos-historial.md

## Restricciones globales

- Valores de origen: `probar`, `terminal`, `cli`, `app`, `api`; `''` en los trabajos anteriores.
- Encabezado del cliente: `X-Tramtai-Cliente: terminal|cli`. Es solo una etiqueta: no da permisos.
- Rangos: `horas_resultado` 0–720; `horas_metadatos` 0–2160 y ≥ `horas_resultado`; `max_trabajos` 100–100000
  (por defecto 10000). En `config.yaml` se ajustan con aviso; por la API se rechazan con 400 `solicitud_invalida`.
- Los trabajos activos nunca se borran por el tope ni cuentan para él.
- Textos nuevos en `es.ts` y `en.ts` con las mismas claves; mensajes Go nuevos traducidos en `internal/idioma`.
- Tras Go: `go vet ./...` y `go test ./...`; tras la Web UI: `cd web && npm run build` (se versiona
  `internal/recursos/web`).

## Puntos de revisión

- **Un grupo sin APIs** (`?grupo=` de un nombre que ya no existe): lista vacía, nunca todos los trabajos. Prueba en
  la tarea 3.
- **Una API extra que manda `X-Tramtai-Cliente: cli`**: su origen sigue siendo `api`. Prueba en la tarea 3.
- **`horas_resultado: 0` en la configuración** (no guardar respuestas): se respeta, no se sube a 1. Prueba en la
  tarea 1.
- **Base vieja ya grande al actualizar**: el `VACUUM` único corre una vez al abrir; si falla (disco lleno), la base
  se abre igual y se reintenta en el próximo arranque. Prueba de la conversión en la tarea 2; el fallo se revisa
  leyendo el código.
- **Ver más en el último bloque**: si la página trae menos de 200, el botón desaparece y no repite trabajos. Se
  verifica en el navegador en la tarea 6.

---

### Tarea 1: Origen en el núcleo y rangos de retención

**Archivos:**
- Crear: `internal/nucleo/origen.go`, `internal/nucleo/retencion.go`, `internal/nucleo/retencion_test.go`
- Modificar: `internal/nucleo/trabajo.go` (campo `Origen`), `internal/nucleo/configuracion.go` (clave
  `retencion.max_trabajos`, tipo entero, por defecto 10000)

**Interfaces:**
- Produce:
  - constantes `DesdeProbar = "probar"`, `DesdeTerminal = "terminal"`, `DesdeCLI = "cli"`, `DesdeApp = "app"`,
    `DesdeAPI = "api"`;
  - `func ConOrigenPeticion(ctx context.Context, origen string) context.Context` y
    `func OrigenPeticion(ctx context.Context) string` (`''` si no hay);
  - `Trabajo.Origen string` con `json:"origen,omitempty"`;
  - `func AjustarRetencion(resultado, metadatos, maximo int) (int, int, int, bool)`: lleva cada valor a su rango y
    `metadatos` a ≥ `resultado`; el `bool` dice si cambió algo;
  - `func ValidarRetencion(resultado, metadatos, maximo int) error`: `NuevoError(CodigoSolicitudInvalida, …)` con el
    motivo del primer valor fuera de rango.

- [ ] **Paso 1: Pruebas que fallan** (`retencion_test.go`)

```go
func TestAjustarRetencion(t *testing.T) {
	r, m, x, cambio := AjustarRetencion(0, 168, 10000)   // esperado: 0, 168, 10000, false (0 se respeta)
	r, m, x, cambio = AjustarRetencion(1000, 5000, 50)    // esperado: 720, 2160, 100, true
	r, m, x, cambio = AjustarRetencion(48, 24, 10000)     // esperado: 48, 48, 10000, true
	r, m, x, cambio = AjustarRetencion(-1, -1, 200000)    // esperado: 0, 0, 100000, true
}

func TestValidarRetencion(t *testing.T) {
	// esperado: nil para (24, 168, 10000) y (0, 0, 100)
	// esperado: CodigoDe(err) == CodigoSolicitudInvalida para (721, 800, 10000), (24, 12, 10000), (24, 168, 99)
}

func TestOrigenPeticion(t *testing.T) {
	// OrigenPeticion(context.Background()) == ""; OrigenPeticion(ConOrigenPeticion(ctx, DesdeCLI)) == "cli"
}
```

- [ ] **Paso 2: Ejecutar y ver que fallan**

Ejecutar: `go test ./internal/nucleo -run 'Retencion|OrigenPeticion'`
Esperado: FAIL por funciones no definidas.

- [ ] **Paso 3: Implementar** en los archivos de la lista. La clave de contexto es un tipo propio no exportado.
Los mensajes de `ValidarRetencion` en español con su traducción en `internal/idioma`.

- [ ] **Paso 4: Ejecutar las pruebas**

Ejecutar: `go test ./internal/nucleo ./internal/idioma`
Esperado: PASS.

- [ ] **Paso 5: Commit**

```bash
git add internal/nucleo internal/idioma
git commit -m "feat(nucleo): origen de la petición y rangos de retención"
```

### Tarea 2: Persistencia: origen, filtro, tope y compactación

**Archivos:**
- Modificar: `internal/persistencia/almacen.go` (migración v8, `columnas`, `Aceptar`, `escanear`, `Listar`,
  `Purgar`, `Abrir`, función nueva `UsoHistorial`)
- Modificar: `internal/planificador/planificador.go:360` (`Listar` con el filtro)
- Prueba: `internal/persistencia/almacen_test.go`

**Interfaces:**
- Consume: `Trabajo.Origen` (tarea 1).
- Produce:
  - `type FiltroTrabajos struct { Punto string; Puntos []string; Origen string; Antes time.Time }`: `Punto` como hoy;
    `Puntos` no vacío limita a esa lista; `Origen` no vacío filtra por origen; `Antes` no cero, `creado_en < Antes`;
  - `func (a *Almacen) Listar(ctx context.Context, instancia string, f FiltroTrabajos, limite int) ([]nucleo.Trabajo, error)`;
  - `func (p *Planificador) Listar(ctx context.Context, limite int, f persistencia.FiltroTrabajos) ([]nucleo.Trabajo, error)`;
  - `Retencion.Maximo int` (0 = sin tope);
  - `type UsoHistorial struct { Trabajos int; Bytes int64 }` con etiquetas json `trabajos` y `bytes`, y
    `func (a *Almacen) UsoHistorial(ctx context.Context, instancia string) (UsoHistorial, error)` (trabajos de la
    instancia; bytes = `page_count * page_size`).

- [ ] **Paso 1: Pruebas que fallan**

```go
func TestOrigenSeGuarda(t *testing.T) {
	// Aceptar un trabajo con Origen "cli"; Obtener lo devuelve con Origen "cli"
}

func TestListarConFiltro(t *testing.T) {
	// trabajos: (punto "", origen "probar"), (punto "", origen "cli"), (punto "tienda", origen "api"),
	//           (punto "soporte", origen "api"), creados en ese orden
	// Origen "cli" → 1; Puntos ["tienda","soporte"] → 2; Antes = creado_en del 3.º → los dos primeros
}

func TestPurgarConTope(t *testing.T) {
	// 5 terminados (terminado_en escalonado) + 1 activo; Purgar con Retencion{Resultado: 24h, Metadatos: 168h, Maximo: 2}
	// esperado: quedan los 2 terminados más recientes y el activo; sus eventos también se fueron
}

func TestAutoVacuumIncremental(t *testing.T) {
	// Abrir una base nueva: PRAGMA auto_vacuum == 2
	// una base creada con auto_vacuum = 0 (abrir con sql.Open, crear una tabla, cerrar) y abierta con Abrir: == 2
}

func TestUsoHistorial(t *testing.T) {
	// con 3 trabajos en "global" y 1 en "otra": Trabajos == 3 y Bytes > 0
}
```

Adaptar las llamadas existentes a `Listar` en `almacen_test.go` (líneas ~358-366) a `FiltroTrabajos{Punto: …}`.

- [ ] **Paso 2: Ejecutar y ver que fallan**

Ejecutar: `go test ./internal/persistencia`
Esperado: FAIL (campos y funciones no definidos).

- [ ] **Paso 3: Implementar**

- Migración v8: `ALTER TABLE trabajos ADD COLUMN origen TEXT NOT NULL DEFAULT '';` (al final de `migraciones`).
  `origen` en `columnas`, en el `INSERT` de `Aceptar` y en el escaneo.
- `Listar`: construir el `WHERE` con los filtros presentes (`punto_acceso IN (…)` con marcadores `?`); `Antes` se
  pasa con `fecha(f.Antes)`, el mismo formato con que se guarda `creado_en`, para que la comparación de texto valga.
- `Purgar`: si `Maximo > 0`, antes de los demás pasos, borrar los eventos y los trabajos de
  `instancia = ? AND terminado_en IS NOT NULL AND id NOT IN (SELECT id … ORDER BY terminado_en DESC LIMIT ?)`.
  Tras confirmar la transacción, `PRAGMA incremental_vacuum` (un error solo se ignora).
- `Abrir`: tras las migraciones, si `PRAGMA auto_vacuum` no es 2, `PRAGMA auto_vacuum = INCREMENTAL` y `VACUUM`
  fuera de transacción; un fallo no impide abrir (se reintenta en el próximo arranque).
- `Planificador.Listar` pasa el filtro tal cual.

- [ ] **Paso 4: Ejecutar las pruebas**

Ejecutar: `go test ./internal/persistencia ./internal/planificador`
Esperado: PASS (el paquete `api` no compila hasta la tarea 3; no se ejecuta aquí).

- [ ] **Paso 5: Commit**

```bash
git add internal/persistencia internal/planificador
git commit -m "feat(persistencia): origen, filtro del historial, tope y compactación"
```

### Tarea 3: Origen de punta a punta y filtros de la lista

**Archivos:**
- Modificar: `internal/planificador/planificador.go` (`aceptar` y `Reintentar`)
- Modificar: `internal/api/api.go` (`Servidor.PuntosDeGrupo`), `internal/api/punto.go` (`listarTrabajos`)
- Modificar: `internal/servicio/servicio.go` (`enrutar`, línea ~182; `Servidor` en la línea ~364)
- Modificar: `internal/cliente/cliente.go` (`Cliente.Origen` y `pedir`), `internal/tui/tui.go` (fija `terminal`),
  `cmd/tramtai/ejecutar_servicio.go` y `cmd/tramtai/ejecutar.go:192` (fijan `cli`)
- Prueba: `internal/planificador/planificador_test.go`, `internal/servicio/web_test.go` o `servicio_test.go`,
  `internal/cliente/cliente_test.go` (crear si no existe)

**Interfaces:**
- Consume: tareas 1 y 2.
- Produce:
  - `Servidor.PuntosDeGrupo func(grupo string) []string` (nil en un punto de acceso);
  - `Cliente.Origen string`: si no está vacío, `pedir` manda `X-Tramtai-Cliente` con ese valor;
  - `GET /api/v1/trabajos?origen=&grupo=&antes=` (antes en RFC 3339; inválido → 400).

- [ ] **Paso 1: Pruebas que fallan**

```go
// planificador
func TestOrigenDelTrabajo(t *testing.T) {
	// Enviar con ConOrigenPeticion(ctx, DesdeProbar) → Origen "probar"
	// EnviarPorPunto con ConOrigenPeticion(ctx, DesdeCLI) → Origen "api"
	// Reintentar ese trabajo "probar" con un ctx sin origen → el nuevo tiene Origen "probar"
}

// servicio (con entorno y una sesión de la Web UI como en TestWebUI)
func TestOrigenSegunAutenticacion(t *testing.T) {
	// POST /api/v1/ejecutar con la sesión → origen "probar"
	// con el token de la instancia y X-Tramtai-Cliente: cli → "cli"; con "terminal" → "terminal"
	// con el token sin encabezado → "app"; con X-Tramtai-Cliente: otra → "app"
	// por una API extra (/p/<nombre>/...) con X-Tramtai-Cliente: cli → "api"
}

func TestListarPorGrupoYOrigen(t *testing.T) {
	// APIs tienda y soporte en el grupo web, otra sin grupo; un trabajo por cada una y uno de Probar
	// ?grupo=web → 2; ?grupo=nadie → 0 (lista vacía, no todos); ?origen=probar → 1
	// ?antes=<creado_en del más nuevo> → todos menos ese; ?antes=ayer-no → 400
	// en la API extra tienda, ?grupo=web y ?origen=probar se ignoran: ve solo los suyos
}

// cliente
func TestEncabezadoDelCliente(t *testing.T) {
	// con un httptest que guarda X-Tramtai-Cliente: Origen "" → sin encabezado; Origen "cli" → "cli"
}
```

- [ ] **Paso 2: Ejecutar y ver que fallan**

Ejecutar: `go test ./internal/planificador ./internal/servicio ./internal/cliente`
Esperado: FAIL.

- [ ] **Paso 3: Implementar**

- `aceptar`: `t.Origen = nucleo.OrigenPeticion(ctx)`, y `nucleo.DesdeAPI` si `punto != ""`. `Reintentar`: llamar a
  `aceptar` con `nucleo.ConOrigenPeticion(ctx, anterior.Origen)`.
- `enrutar`: antes de `inst.rutas.ServeHTTP`, `r = r.WithContext(nucleo.ConOrigenPeticion(r.Context(), origen))`
  con `probar` si `porSesion`, el valor del encabezado si es `terminal` o `cli`, y `app` en otro caso.
- `listarTrabajos`: armar `FiltroTrabajos` con `punto` (como hoy), `origen` (solo los cuatro valores; otro → 400),
  `antes` y `grupo` vía `PuntosDeGrupo`; si el grupo no tiene APIs, responder `{"trabajos": []}` sin consultar. En un
  punto de acceso, solo `Punto` y `Antes`.
- `servicio.go:364`: `PuntosDeGrupo` con `s.puntosDe(id)` filtrando por `Grupo`.
- `Cliente.Origen` en `pedir`; la TUI lo fija a `terminal` al recibir el cliente; `ejecutarEnServicio` a `cli`; la
  ejecución directa (`ejecutar.go:192`) usa `nucleo.ConOrigenPeticion(cmd.Context(), nucleo.DesdeCLI)`.

- [ ] **Paso 4: Ejecutar las pruebas**

Ejecutar: `go vet ./... && go test ./...`
Esperado: PASS.

- [ ] **Paso 5: Commit**

```bash
git add internal cmd
git commit -m "feat: origen de cada petición y filtros del historial por origen, grupo y fecha"
```

### Tarea 4: Retención configurable

**Archivos:**
- Modificar: `internal/servicio/servicio.go` (`Purgar`, línea ~393: ajuste con aviso y `Maximo`)
- Modificar: `internal/servicio/web.go` (`rutaInstancia`: `GET/PUT /api/v1/instancia/retencion`; función
  `fijarRetencion` junto a `fijarPredeterminado`)
- Prueba: `internal/servicio/web_test.go` o `servicio_test.go`

**Interfaces:**
- Consume: `AjustarRetencion`, `ValidarRetencion` (tarea 1); `Retencion.Maximo`, `UsoHistorial` (tarea 2).
- Produce: `GET /api/v1/instancia/retencion` →
  `{"horas_resultado": n, "horas_metadatos": n, "max_trabajos": n, "uso": {"trabajos": n, "bytes": n}}`;
  `PUT` con los tres números → 204, o 400 `solicitud_invalida`.

- [ ] **Paso 1: Pruebas que fallan**

```go
func TestRetencionConfigurable(t *testing.T) {
	// GET con la sesión → 24, 168, 10000 y uso.trabajos == número de trabajos de la instancia
	// PUT {48, 240, 500} → 204; GET devuelve 48, 240, 500; la configuración efectiva tiene esos valores
	// PUT {24, 12, 500} → 400 solicitud_invalida; PUT {24, 168, 50} → 400
	// con el token de la instancia (sin sesión) → la protección de rutaInstancia (no autorizado)
}

func TestRetencionFueraDeRangoEnConfig(t *testing.T) {
	// entorno con "retencion:\n  horas_resultado: 5000\n": la purga usa 720 h y el registro contiene un aviso
}
```

- [ ] **Paso 2: Ejecutar y ver que fallan**

Ejecutar: `go test ./internal/servicio -run Retencion`
Esperado: FAIL (ruta inexistente; sin ajuste).

- [ ] **Paso 3: Implementar**

- `Purgar`: leer los tres valores, `AjustarRetencion`; si cambió algo, una línea en `s.opc.Registros` (una vez por
  instancia, no en cada purga); pasar `Maximo`.
- `fijarRetencion(inst, resultado, metadatos, maximo)`: `ValidarRetencion`; escribir las tres claves con
  `FijarValor(archivoPuntos(inst))` y aplicarlas en marcha como `fijarPredeterminado` (global → todas las
  instancias); luego `go inst.Purgar(context.Background())`.
- Ruta en `rutaInstancia` (ya exige sesión o token de servicio). El `GET` lee la configuración efectiva y
  `UsoHistorial`.

- [ ] **Paso 4: Ejecutar las pruebas**

Ejecutar: `go test ./internal/servicio ./internal/idioma`
Esperado: PASS.

- [ ] **Paso 5: Commit**

```bash
git add internal/servicio internal/idioma
git commit -m "feat(servicio): retención configurable con tope y uso del historial"
```

### Tarea 5: Mis APIs con grupos plegables y filtro, y el mapa por grupo

**Archivos:**
- Modificar: `web/src/paginas/Accesos.vue` (cabecera de grupo, filtro), `web/src/componentes/MapaRed.vue`
  (estaciones por grupo, evento al pulsar), `web/src/paginas/Panel.vue` (pasa el evento), `web/src/api.ts`
  (`grupoInicial`), `web/src/idiomas/es.ts` y `en.ts`

**Interfaces:**
- Produce: `export const grupoInicial = ref('')` en `api.ts` (Mis APIs lo lee y lo vacía al montar, como
  `origenHistorial`); `MapaRed` emite `abrirGrupo: [grupo: string]` y `abrirApis: []`.

- [ ] **Paso 1: Mis APIs**

- Plegar: estado `plegados = ref<Set<string>>` leído y guardado en `localStorage` `tramtai.gruposPlegados` (lista
  JSON) con `try/catch`; la cabecera es un botón con `aria-expanded` y su icono; el grupo plegado no pinta sus filas.
- Filtro (con más de una API): `q-select` de grupo («Todos», cada grupo, «Sin grupo») y `q-input` de búsqueda por
  nombre, encima de la lista; los grupos sin filas tras filtrar no se pintan.
- Claves: `puntos.filtroGrupo` «Grupo», `puntos.todosGrupos` «Todos», `puntos.buscarApi` «Buscar API»,
  `puntos.plegar` «Plegar {grupo}», `puntos.desplegar` «Desplegar {grupo}»; `mapa.grupo` «{grupo} · {n} APIs»,
  `mapa.activas` «{n} activas» (y sus versiones en inglés).

- [ ] **Paso 2: Mapa**

En `origenes`: la principal, luego una entrada por grupo (`id: 'g:' + grupo`, nombre `t('mapa.grupo')`, `activo` si
alguna API del grupo lo está, `detalle: t('mapa.activas')`) y luego las APIs sin grupo; recortar a `MAX_APIS` con
«y N más» como hoy. Pulsar una estación de grupo emite `abrirGrupo`; una API o «y N más», `abrirApis`. Panel fija
`grupoInicial` y navega a `conectar`.

- [ ] **Paso 3: Compilar y verificar**

Ejecutar: `cd web && npm run build` → «built in» sin errores. En el navegador con el servicio de prueba y APIs
`tienda` y `soporte` en el grupo `web` más una sin grupo: plegar `web`, recargar y sigue plegado; el filtro deja solo
`web` o solo «Sin grupo»; el mapa muestra «web · 2 APIs» y pulsarlo abre Mis APIs filtrada. Claro, oscuro y 375 px.

- [ ] **Paso 4: Commit**

```bash
git add web/src internal/recursos/web
git commit -m "feat(web): grupos plegables y filtro en Mis APIs, y el mapa por grupo"
```

### Tarea 6: Historial por origen y Ajustes · Historial

**Archivos:**
- Modificar: `web/src/api.ts` (`Trabajo.origen`, `trabajos()` con filtro, `retencion()`, `fijarRetencion()`),
  `web/src/paginas/Trabajos.vue`, `web/src/paginas/Configuracion.vue`, `web/src/paginas/Panel.vue` (días),
  `es.ts`, `en.ts`

**Interfaces:**
- Consume: rutas de las tareas 3 y 4.
- Produce: `api.trabajos(limite: number, filtro?: { punto?: string; grupo?: string; origen?: string; antes?: string })`
  (las llamadas actuales siguen valiendo); `api.retencion(): Promise<Retencion>` y
  `api.fijarRetencion(r: { horas_resultado: number; horas_metadatos: number; max_trabajos: number })`.

- [ ] **Paso 1: Historial**

- Columna y detalle «Origen» con `textoOrigen(t)`: `probar` «Probar», `terminal` «Terminal», `cli` «CLI», `app`
  «App (clave principal)», con `punto_acceso` «API {nombre}» más « · {grupo}» si tiene, y `''` «API principal».
- Selector Origen: «Todos», los cuatro orígenes, «Grupo {nombre}» por grupo y cada API; el valor elige `origen`,
  `grupo` o `punto` en la petición.
- «Mostrando las {n} más recientes» y **Ver más**: pide 200 con `antes = creado_en` del último y los añade; si
  llegan menos de 200, se oculta. Cambiar de filtro reinicia la lista.

- [ ] **Paso 2: Ajustes · Historial**

Sección nueva en `Configuracion.vue`: tres `q-input type=number` (horas de respuesta 0–720; días de registro 0–90,
convertidos a horas; máximo 100–100000), «Guardar» con `fijarRetencion` y aviso del error del servicio, y la línea
de uso «{trabajos} peticiones · {MB} MB».

- [ ] **Paso 3: Inicio**

El texto del tablero que dice «últimos 7 días» usa `Math.round(horas_metadatos / 24)` de `api.retencion()` (clave con
`{dias}` en es y en).

- [ ] **Paso 4: Compilar y verificar**

Ejecutar: `cd web && npm run build`. En el navegador: una petición desde Probar sale como «Probar»; filtrar por grupo
`web` y por «Probar»; con más de 200 trabajos, Ver más trae el resto y desaparece al final; Ajustes guarda 48/10/500,
rechaza 12 de registro con 24 de respuesta mostrando el motivo, y el tablero de Inicio dice «últimos 10 días».

- [ ] **Paso 5: Commit**

```bash
git add web/src internal/recursos/web
git commit -m "feat(web): historial por origen y grupo, y retención en Ajustes"
```

### Tarea 7: Capturas, documentación y cierre

**Archivos:**
- Modificar: `site/capturas/*.png`, `docs/imagenes/inicio.png`, `docs/imagenes/mis-apis-oscuro.png`
- Modificar: `docs/operacion.md`, `docs/arquitectura.md` (tabla de retención), `docs/api.md`,
  `web/src/paginas/Ayuda.vue` y `es.ts`/`en.ts` (pregunta nueva), `docs/equipo/pendientes.md`

- [ ] **Paso 1: Capturas**

En la configuración de la instancia de prueba (fuera del repositorio), renombrar la API `facturas` a `tienda`, crear
`soporte` y ponerlas en el grupo `web`. Recompilar el binario de prueba con la versión de la rama, rehacer las cuatro
capturas de `site/capturas/` (Inicio y Mis APIs, claro y oscuro) y copiar Inicio claro y Mis APIs oscuro a
`docs/imagenes/`.

- [ ] **Paso 2: Documentación**

- `operacion.md`: grupos (plegar, filtrar, mapa) y retención (rangos, tope, compactación, Ajustes · Historial).
- `arquitectura.md`: fila `retencion.max_trabajos` y los rangos en la tabla de retención.
- `api.md`: `origen` del trabajo con sus valores; `?origen=`, `?grupo=`, `?antes=`; `X-Tramtai-Cliente`;
  `GET/PUT /api/v1/instancia/retencion`.
- Ayuda: pregunta «¿Cuánto se guarda el historial?» con la respuesta (24 h de respuestas, 7 días de registro, 10 000
  como máximo; se cambia en Ajustes · Historial).
- `pendientes.md`: fila del release en la hoja de ruta.

- [ ] **Paso 3: Comprobaciones finales**

Ejecutar: `go vet ./...`, `go test ./...`, `cd escritorio && go vet ./...`, `cd web && npm run build`,
`graphify update .`
Esperado: todo sin errores.

- [ ] **Paso 4: Commit**

```bash
git add site docs web internal/recursos/web
git commit -m "docs: grupos, origen y límites del historial; capturas con nombres neutros"
```
