# v0.62, uso y diagnóstico: plan de implementación

> **Para quien lo ejecuta:** se sigue tarea a tarea con superpowers:subagent-driven-development (o
> superpowers:executing-plans). Los pasos usan casillas (`- [ ]`).

**Objetivo:** que Tramtai enseñe la cuota de cada suscripción y el dinero de las claves, actúe al llegar a los límites,
deje a los alias fijar el esfuerzo y diga por qué una API no responde.

**Arquitectura:**
- Los CLI siguen igual; sus lectores aprenden a sacar la cuota (`rate_limit_event` de Claude) y dos agentes ganan una
  consulta aparte (`codex app-server` y, si la M0 lo confirma, Copilot).
- El almacén guarda tokens crudos con la caché aparte, el modo de autenticación, el uso por día (`uso_diario`) y la
  última lectura de cuota (`cuotas`).
- El dinero sale de una tabla de precios embebida (`internal/precios`). El planificador pone en `Uso.CostoUSD` el coste
  de esa tabla solo con clave, y así los presupuestos en USD que ya existen cuentan lo correcto sin otro cambio.
- El servicio arma el informe de uso para la Web UI, la terminal y la CLI.

**Tecnología:**
- Go 1.27 con `go test` (`-race` en planificador y persistencia);
- Vue/Quasar con node:test (`cd web; npm test`), compilado a `internal/recursos/web`;
- Bubble Tea en la terminal;
- SQLite (migración v11).

**Spec:** `docs/equipo/releases/v0.62-uso-y-diagnostico/spec.md`. Se lee junto con este plan, y con
`investigacion.md` para los formatos de cada CLI.

## Restricciones globales

- **Dependencias:** ninguna nueva, ni Go ni npm. El JSON-RPC de `codex app-server` (y de Copilot, si aplica) se habla con
  la biblioteca estándar.
- **Términos de uso:** nunca se leen credenciales ni archivos de sesión de los CLI (`~/.claude/.credentials.json`,
  `~/.codex/auth.json`, `~/.codex/sessions`, `~/.copilot`), ni se llama a endpoints privados (`api/oauth/usage`,
  `copilot_internal`, `chatgpt.com/backend-api`). Solo el CLI oficial con la sesión del usuario.
- **«Desconocido», nunca 0:** un dato de cuota que falta se guarda como nulo y se enseña como «desconocido». Un fallo de
  lectura no es cuota agotada y nunca hace fallar la petición del usuario.
- **Pruebas:** ninguna toca los CLI reales, las sesiones del usuario, su Tramtai instalado ni el puerto 17373. Se usan
  los datos de prueba de la M0 (Tarea 1) y agentes simulados.
- **Compatibilidad:** las columnas de hoy no cambian de significado (`tokens_entrada` sigue incluyendo la caché). Un
  `config.yaml` de la v0.61 sin `semanal`, `aviso` ni `al_limite` se comporta igual que hoy: avisa al 80 % y al 100 % y
  para al 100 %.
- **Textos:**
  - todo texto visible va en es y en: `idioma.T`/`idioma.Tf` en Go y `web/src/idiomas/*.ts` en la Web UI;
  - la terminal recibe los textos de la Ayuda regenerados con `node web/scripts/terminal-datos.mjs`;
  - el apóstrofo tipográfico (’) y el glosario, que vigila `estilo-textos.test.ts`.
- **Dinero:** se calcula en USD y se guarda en USD. La moneda del usuario solo se aplica al mostrar.
- **Capturas:** solo con copias en `bin/capturas-*` y el puerto 17388. Las completas hacen peticiones reales a
  claude/haiku con la suscripción del usuario, así que solo se rehacen las necesarias, con `--solo`.
- **Git:**
  - cada tarea en su rama `v0.62/tNN-<tema>` y su PR con Auto-fix, apilado sobre la anterior; nunca push a `main`;
  - se fusiona y se publica solo con el «sí» del usuario;
  - los commits terminan con `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`;
  - `anti-slop/` nunca se sube.
- **Al terminar cada tarea:** `go vet ./...`, `go test ./...`, y si toca la Web UI `cd web; npm test; npm run build`.
  Después, `graphify update .`.
- **Entorno:** Windows. La herramienta Bash está rota, así que se usa PowerShell. Un hook (GateGuard) pide datos antes de
  editar.

## Lo que más puede fallar (foco de las revisiones)

1. **Una base de la v0.61 con presupuestos y consumo:** tras migrar, un presupuesto que hoy para tiene que seguir
   parando, con las mismas cifras del día y del mes. Lo prueba la Tarea 3 (`TestMigracionV11ConservaConsumo`) y la
   Tarea 5 (`TestPresupuestoV061SigueParando`).
2. **Un CLI que cambia su salida o un `app-server` que no contesta** (versión vieja sin el RPC, campo renombrado,
   proceso colgado): la petición del usuario sigue igual y la cuota queda «desconocido». Lo prueba la Tarea 4
   (`TestCodexAppServerSinRespuesta`, `TestEventoClaudeRaroNoRompe`).
3. **Una lectura vieja o pasada de su reinicio** no puede parar peticiones ni desviarlas al respaldo. Lo prueba la Tarea
   6 (`TestCuotaPasadaDeReinicioNoPara`).
4. **Fechas límite:** una semana que cruza mes o año, y un trabajo aceptado antes de medianoche que termina después: el
   consumo y el aviso van al período de aceptación. Lo prueba la Tarea 5 (`TestSemanaCruzaAnio`,
   `TestAvisoEnPeriodoDeAceptacion`).
5. **Un modelo sin precio con presupuesto en USD:** no puede contar como 0 en silencio. La API avisa «sin precio» y ese
   coste no frena. Lo prueba la Tarea 3 (`TestCosteSinPrecio`) y la Tarea 5 (`TestPresupuestoUSDConModeloSinPrecio`).

---

### Tarea 1: M0 con los CLI reales

**Requisito previo:** el permiso explícito del usuario, pedido justo antes con estas palabras: la M0 usa tus sesiones
reales de Claude Code, Codex y Copilot solo para leer la cuota y hacer unas pocas peticiones mínimas. Sin ese permiso se
salta esta tarea: las Tareas 4 y 6 usan datos de prueba escritos a mano desde `investigacion.md`, y `m0.md` lo dice.

**Archivos:**
- Crear: `docs/equipo/releases/v0.62-uso-y-diagnostico/m0.md`
- Crear: `internal/agentes/claude/testdata/cuota-*.jsonl`, `internal/agentes/codex/testdata/ratelimits-*.json`,
  `internal/agentes/copilot/testdata/{cuota,uso}-*.json`

**Interfaces:**
- **Produce:** los datos de prueba anteriores, sin cuentas, ids ni correos, que usan las Tareas 4 y 6.

- [ ] **Paso 1:** huellas SHA-256 de los archivos de sesión y configuración de cada CLI, sin abrir su contenido (como
  en la M0 de la v0.60, `docs/equipo/releases/v0.60-preparar-agentes/m0.md`).
- [ ] **Paso 2: Claude:** tres veces `"responde OK" | claude -p --output-format stream-json --verbose --model haiku`.
  Guardar las líneas `rate_limit_event`: ¿viene `utilization`?, ¿`resetsAt` en segundos o en milisegundos?, ¿qué
  `rateLimitType`? Comprobar `claude --help` para `--max-budget-usd` y la salida al pasarse con clave (si el usuario no
  tiene clave de Anthropic, se anota como no comprobado).
- [ ] **Paso 3: Codex:** lanzar `codex app-server`, hacer `initialize` e `initialized`, llamar a
  `account/rateLimits/read` y cerrar. Guardar la respuesta. Anotar la versión de Codex y si `codex exec --json` trae
  algo de límites.
- [ ] **Paso 4: Copilot:** buscar en la ayuda del CLI instalado y en la documentación del SDK una vía documentada para
  `account.getQuota` sin instalar nada. Si existe, guardar la respuesta. Guardar también un `--usage-output-file` y el
  `result` con `premiumRequests` de una petición mínima.
- [ ] **Paso 5:** repetir el Paso 1. Las huellas tienen que coincidir; si no, parar y avisar al usuario.
- [ ] **Paso 6:** escribir `m0.md` (versiones, formatos vistos, lo que no se pudo comprobar) y los datos de prueba
  limpios.
- [ ] **Paso 7: commit**, `docs(v0.62): M0 de la cuota con los CLI reales`.

### Tarea 2: la tabla de precios

**Archivos:**
- Crear: `web/scripts/precios.mjs`, `internal/precios/precios.go`, `internal/precios/precios.json`
- Probar: `internal/precios/precios_test.go`, `web/scripts/precios.test.ts`

**Interfaces:**
- **Produce:**
  - `precios.json`: `{ "fecha": "AAAA-MM-DD", "origen": "<commit de LiteLLM>", "modelos": { "<proveedor>/<modelo>":
    { "entrada", "salida", "cache_lectura", "cache_escritura" } } }`, en USD por millón de tokens; `<proveedor>` es
    `anthropic`, `openai` o `gemini`.
  - `type Precio struct { Entrada, Salida, CacheLectura, CacheEscritura float64 }`.
  - `func Tabla() (fecha, origen string, modelos map[string]Precio)`, cargada una vez con `sync.OnceValue`, como
    `internal/tui/datos`.
  - `func Proveedor(agente string) string`: claude → anthropic, codex → openai, antigravity y gemini → gemini, el resto
    → "".
  - `func Buscar(agente, modelo string, propios map[string]Precio) (Precio, bool)`: primero los propios, que se nombran
    por el modelo tal cual, y después la tabla con `<proveedor>/<modelo>`. Si no está, prueba sin el sufijo de fecha
    (`-20251001`).
  - `func Costo(p Precio, u nucleo.Uso) float64`:
    `((u.TokensEntrada − u.CacheLectura − u.CacheEscritura)·Entrada + u.CacheLectura·CacheLectura +
    u.CacheEscritura·CacheEscritura + u.TokensSalida·Salida) / 1e6`. Usa los campos de caché que añade la Tarea 3.
  - `func Propios(lista []string) (map[string]Precio, error)`: lee entradas `modelo=entrada/salida/lectura/escritura`.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestCostoConCache`: con `Precio{3, 15, 0.3, 3.75}` y `Uso{TokensEntrada: 1_000_000, CacheLectura: 400_000,
    CacheEscritura: 100_000, TokensSalida: 200_000}`, el coste es 1.5 + 0.12 + 0.375 + 3 = 4.995;
  - `TestBuscarSinFecha`: `claude-haiku-4-5-20251001` encuentra `anthropic/claude-haiku-4-5`;
  - `TestPropioMandaSobreTabla`;
  - `TestPropiosMalFormados` (falta un campo o un número negativo: error);
  - `TestTablaNoCaducada`: con `TRAMTAI_COMPROBAR_PRECIOS=1`, falla si `fecha` tiene más de 60 días; sin la variable,
    se salta;
  - en `precios.test.ts`, el filtro deja solo `litellm_provider` anthropic, openai y gemini con `mode` chat o responses,
    y pasa de USD por token a USD por millón.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar.** `precios.mjs <commit>` baja
  `https://raw.githubusercontent.com/BerriAI/litellm/<commit>/model_prices_and_context_window.json` y escribe
  `precios.json` con la fecha de hoy. Un commit que no sea un SHA de 40 caracteres se rechaza.
- [ ] **Paso 4:** pruebas en verde. Generar la tabla con el commit de hoy y revisar el diff a ojo. Añadir
  `TRAMTAI_COMPROBAR_PRECIOS: 1` al job de pruebas de `.github/workflows/publicar.yml`.
- [ ] **Paso 5: commit**, `feat(precios): tabla de precios embebida desde LiteLLM`.

### Tarea 3: la medida en el almacén

**Archivos:**
- Modificar: `internal/nucleo/trabajo.go` (`Uso`), `internal/persistencia/almacen.go` (v11),
  `internal/persistencia/trabajos.go` (`Cambio`, `CambiarEstado`), `internal/persistencia/consumo.go`
- Modificar: `internal/agentes/{claude,copilot,antigravity}/*.go` (la caché aparte)
- Modificar: `internal/planificador/ejecucion.go` (`sumarUso`, coste y autenticación al terminar)
- Crear: `internal/persistencia/uso.go`
- Probar: `internal/persistencia/uso_test.go`, `internal/planificador/coste_test.go`

**Interfaces:**
- **Consume:** de la Tarea 2, `precios.Buscar`, `precios.Costo` y `precios.Propios`.
- **Produce:**
  - `nucleo.Uso` gana `CacheLectura, CacheEscritura int` (json `cache_lectura`, `cache_escritura`, omitempty), que
    también cuentan dentro de `TokensEntrada`.
  - `persistencia.Cambio` gana `Autenticacion string` (`clave_api`, `suscripcion` o `""`).
  - Migración v11:
    - `trabajos` gana `cache_lectura`, `cache_escritura` y `autenticacion`;
    - tabla `uso_diario(instancia, punto, agente, modelo, autenticacion, dia, peticiones, errores, respaldos,
      tokens_entrada, tokens_salida, cache_lectura, cache_escritura, ms_total)` con PK de las seis primeras;
    - tabla `cuotas(agente, ventana, clase, usado REAL NULL, estado, reinicio TEXT NULL, plan, creditos REAL NULL,
      origen, leida_en)` con PK `(agente, ventana)`;
    - `avisos_presupuesto` gana `clase TEXT NOT NULL DEFAULT 'presupuesto'`.
  - `CambiarEstado`, al pasar a un estado terminal, suma una fila en `uso_diario` en el día de aceptación. El agente sale
    de la solicitud guardada, el modelo de `modelo_efectivo` (o el enviado), `respaldos` es `intento > 1` y `ms_total`
    es `terminado − creado`.
  - `type FiltroUso struct { Instancia, Punto, Agente, Modelo string; Desde, Hasta string /* AAAA-MM-DD */ }` y
    `func (a *Almacen) UsoDiario(ctx, f FiltroUso) ([]FilaUso, error)`, donde `FilaUso` lleva las columnas de la tabla.
  - `Purgar` deja en `uso_diario` el mes en curso y los 12 anteriores, como en `consumo`.
  - En el planificador, al terminar con éxito o con fallo, `Uso.CostoUSD` se calcula en
    `func (p *Planificador) coste(c candidato, modelo string, u nucleo.Uso) float64`: con clave es `precios.Costo` del
    modelo efectivo; con suscripción, sin precio o sin modelo, 0.
    - Un modelo con clave y sin precio deja el evento `aviso` «sin precio para <modelo>: no cuenta para el presupuesto
      en USD».
    - `Cambio.Autenticacion` sale de `adaptadores.ConAutenticacion` del agente elegido.
  - Los precios propios se leen de la clave global `uso.precios_propios` (`tipoLista`), que se añade al esquema.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestMigracionV11ConservaConsumo`: una base v10 con filas en `consumo` y `trabajos` migra sin perder nada y
    `Consumo()` devuelve lo mismo;
  - `TestUsoDiarioAlTerminar`: un trabajo terminado suma su fila con agente, modelo, autenticación, caché y ms; uno
    fallido suma `errores`; uno con `intento` 2 suma `respaldos`;
  - `TestCosteSinPrecio`: con clave y un modelo que no está en la tabla, `CostoUSD` es 0 y hay evento `aviso`;
  - `TestSuscripcionNoSumaDinero`: Claude con suscripción que informa `total_cost_usd` 0.5 deja `CostoUSD` 0 y
    `consumo.usd` sin cambio;
  - `TestCosteConClave`: Claude con clave y `claude-haiku-4-5` deja el coste de la tabla;
  - en los agentes, que Claude, Copilot y Antigravity rellenan `CacheLectura` y `CacheEscritura` desde sus campos.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar** lo de Interfaces. `sumarUso` suma también la caché. Shortcut: el uso de los intentos
  fallidos del respaldo se valora con el precio del agente que respondió (suelen ser 0 tokens); se anota con un
  comentario `shortcut:`.
- [ ] **Paso 4:** pruebas en verde, con `-race`.
- [ ] **Paso 5: commit**, `feat(uso): tokens de caché, autenticación y uso por día`.

### Tarea 4: los lectores de cuota

**Archivos:**
- Modificar: `internal/nucleo/` (tipo nuevo), `internal/adaptadores/adaptador.go` (`Resultado`, interfaz nueva)
- Modificar: `internal/agentes/claude/claude.go`, `internal/agentes/codex/` (nuevo `cuota.go`) y
  `internal/agentes/copilot/` (nuevo `cuota.go`, solo si la M0 lo confirmó)
- Crear: `internal/persistencia/cuotas.go`
- Modificar: `internal/planificador/ejecucion.go` (guardar las lecturas al terminar)
- Probar: `internal/agentes/claude/cuota_test.go`, `internal/agentes/codex/cuota_test.go`,
  `internal/persistencia/cuotas_test.go`

**Interfaces:**
- **Consume:** de la Tarea 1, los datos de prueba; de la Tarea 3, la tabla `cuotas`.
- **Produce:**
  - `nucleo.LecturaCuota { Agente, Ventana, Clase string; Usado *float64; Estado string; Reinicio *time.Time; Plan
    string; Creditos *float64; Origen string; LeidaEn time.Time }`.
    - `Clase` es `5h`, `semanal` u `otras`.
    - `Estado` es `ok`, `aviso` (`allowed_warning`), `agotada` (`rejected`) o `""`.
    - `Origen` es `proveedor`.
  - `adaptadores.Resultado` gana `Cuotas []nucleo.LecturaCuota`.
  - `type LectorCuota interface { LeerCuota(ctx context.Context) ([]nucleo.LecturaCuota, error) }` en `adaptadores`.
    Lo implementan Codex (y Copilot si aplica), con un plazo de 10 s; al vencer, el proceso se mata.
  - Claude: en el bucle de `Ejecutar`, `case "rate_limit_event"` llama a
    `func lecturaClaude(linea []byte, ahora time.Time) (nucleo.LecturaCuota, bool)`. `five_hour` es `5h`, `seven_day`
    es `semanal` y el resto es `otras` con su nombre. `resetsAt > 1e11` se lee en milisegundos.
  - Claude: si un evento trae `rejected` y el resultado es un error, el error pasa a `nucleo.CodigoCuotaAgotada` (lo
    crea la Tarea 6; hasta entonces, `CodigoLimiteUsoProveedor`).
  - Codex: `func lecturaCodex(resp []byte, ahora time.Time) ([]nucleo.LecturaCuota, error)`. 300 min es `5h`, 10080 es
    `semanal` y el resto es `otras`. Admite `primary` o `secondary` nulos y `rateLimitsByLimitId`.
  - `func (a *Almacen) GuardarCuotas(ctx, []nucleo.LecturaCuota) error` (upsert por agente y ventana) y
    `func (a *Almacen) Cuotas(ctx) ([]nucleo.LecturaCuota, error)`.
  - El planificador guarda `res.Cuotas` de cada intento, aunque haya fallado.

- [ ] **Paso 1: escribir las pruebas que fallan**, con los datos de la M0 y además:
  - `TestEventoClaudeRaroNoRompe`: una línea con el evento cortado a la mitad, otra sin `utilization` y otra con
    `resetsAt` en milisegundos. La respuesta sale igual, la lectura sin `utilization` tiene `Usado == nil` y el
    reinicio en milisegundos coincide con el de segundos;
  - `TestCodexSinVentanaCincoHoras`: `primary` nulo da solo la semanal;
  - `TestCodexAppServerSinRespuesta`: un programa simulado que no contesta devuelve error en 10 s o menos y no deja
    el proceso vivo;
  - `TestGuardarCuotasSustituye`.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar** lo de Interfaces. El `app-server` se lanza con `EntornoLimpio` y el mismo `CODEX_HOME`
  con el que `Ejecutar` usa la suscripción; nunca con el `CODEX_HOME` privado de la clave.
- [ ] **Paso 4:** pruebas en verde.
- [ ] **Paso 5: commit**, `feat(uso): lectura de la cuota de Claude y Codex`.

### Tarea 5: presupuestos con semana, aviso ajustable y reacciones

**Archivos:**
- Modificar: `internal/nucleo/presupuesto.go`, `internal/persistencia/consumo.go`, `internal/contrato/contrato.go`
  (`ConfigPunto`), `internal/servicio/puntos.go` (`normalizarPunto`), `internal/configuracion/configuracion.go`,
  `internal/planificador/presupuesto.go`, `cmd/tramtai/acceso.go`
- Probar: `internal/nucleo/presupuesto_test.go`, `internal/persistencia/consumo_test.go`

**Interfaces:**
- **Produce:**
  - `Presupuesto` gana `Semanal Limites` (yaml `semanal`) y `Aviso int` (yaml `aviso`; 0 = 80; válido de 1 a 99).
    `EstadoPresupuesto` gana `Semanal`.
  - `func Semana(t time.Time, zona *time.Location) (lunes string /* AAAA-MM-DD */, fin time.Time)`.
  - `Comprobar(dia, semana, mes Consumo, ahora, zona, ambito)` y `Porcentaje(dia, semana, mes)`: se actualizan todos sus
    llamadores. `consumoEn` devuelve también la semana (suma de `dia >= lunes`).
  - `contrato.AlLimite { Presupuesto, Cuota []string }` en `ConfigPunto.AlLimite *AlLimite` (yaml `al_limite`).
    - Valores de `Presupuesto`: `avisar`, `parar`. Valores de `Cuota`: `respaldo`, `parar`.
    - nil equivale a `{Presupuesto: [avisar, parar], Cuota: []}`; una lista vacía explícita es «solo mostrar».
    - `normalizarPunto` rechaza otros valores con `configuracion_invalida`.
  - En la configuración del espacio: `presupuesto.semanal.{peticiones,tokens,usd}`, `presupuesto.aviso` y
    `presupuesto.al_limite` (lista, solo `avisar` y `parar`).
  - Sin `parar`, `comprobarPresupuesto` no rechaza.
  - Sin `avisar`, `sumarConsumo` no escribe avisos.
  - Los avisos se evalúan con los períodos del día de **aceptación** (`aceptado`), no de `ahora`. Períodos `D:`, `S:`
    (lunes de la semana) y `M:`. Niveles: `Aviso` y 100.
  - `func (a *Almacen) AvisosPresupuesto(ctx, instancia string) ([]Aviso, error)`, con
    `Aviso { Clase, Punto, Periodo string; Nivel int; En time.Time }`, solo los del período en curso. (`Almacen.Avisos`
    ya existe y es otra cosa: el canal de escrituras.)
  - CLI: `tramtai acceso crear|editar --presupuesto semanal.usd=5 --aviso 70 --al-limite presupuesto=avisar,parar`.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestSemanaCruzaAnio`: el miércoles 2026-12-30 tiene lunes 2026-12-28 y fin 2027-01-04;
  - `TestComprobarSemanal`: el límite semanal agotado rechaza con el reinicio del lunes siguiente;
  - `TestPresupuestoV061SigueParando`: un `Presupuesto` YAML de la v0.61 (sin `semanal`, `aviso` ni `al_limite`)
    rechaza al 100 % y avisa al 80 % y al 100 %;
  - `TestSoloMostrarNoParaNiAvisa`: `al_limite: {presupuesto: []}` no rechaza ni escribe avisos;
  - `TestAvisoAjustable`: con `aviso: 70` hay aviso al 70 % y no al 80 %;
  - `TestAvisoEnPeriodoDeAceptacion`: aceptado el 2026-11-15 23:59 y terminado el 16 00:01, el aviso es de `D:2026-11-15`;
  - `TestAvisoUnaVezPorPeriodo`;
  - `TestPresupuestoUSDConModeloSinPrecio`: un trabajo con clave sin precio no frena el presupuesto en USD y la API
    lleva el aviso «sin precio».
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar** lo de Interfaces.
- [ ] **Paso 4:** pruebas en verde, con `-race`.
- [ ] **Paso 5: commit**, `feat(presupuestos): período semanal, aviso ajustable y reacciones`.

### Tarea 6: la cuota como límite

**Archivos:**
- Modificar: `internal/nucleo/errores.go`, `internal/api/api.go` (`estadosHTTP`, errores de las fachadas),
  `internal/planificador/respaldo.go`, `internal/planificador/ejecucion.go`, `internal/planificador/presupuesto.go`
  (o un `cuota.go` nuevo), `internal/configuracion/configuracion.go`, `internal/agentes/claude/claude.go`,
  `cmd/tramtai/agentes.go`
- Probar: `internal/planificador/cuota_test.go`, `internal/api/api_test.go`

**Interfaces:**
- **Consume:** de la Tarea 4, `Almacen.Cuotas`; de la Tarea 5, `ConfigPunto.AlLimite` y `AvisosPresupuesto`.
- **Produce:**
  - `nucleo.CodigoCuotaAgotada = "cuota_agotada"`, HTTP 429, saltable al respaldo. Se rechaza con
    `&nucleo.PresupuestoAgotado{Err: NuevoError(CodigoCuotaAgotada, …), Reinicio: <reinicio del proveedor>}`, así que
    `EscribirError` pone `Retry-After` sin cambios. En OpenAI, `code: "cuota_agotada"`.
  - Claves `adaptadores.*.cuota_aviso` y `adaptadores.*.cuota_limite` (`tipoLista` de `clase=porcentaje`, por ejemplo
    `5h=70`). Por defecto, 80 y 100 en cada clase.
  - `func (p *Planificador) enLimite(agente string, ahora time.Time) (*nucleo.LecturaCuota, bool)`: una ventana del
    agente con `Estado == "agotada"` o `Usado >= limite`, con lectura de menos de 15 min o reinicio aún futuro. Una
    lectura con `Reinicio` pasado no cuenta.
  - **Parar:** `EnviarPorPunto` comprueba, antes de `Almacen.Aceptar`, si la API tiene `parar` en `Cuota`. Rechaza si
    el agente principal está en su límite y (sin `respaldo`, o sin un respaldo libre).
  - **Respaldo:** en `recorrerRespaldo`, si la API tiene `respaldo` en `Cuota`, los candidatos en su límite pasan al
    final de la lista, salvo que todos lo estén. El evento `aviso` dice «<agente> sin cuota (<ventana>, vuelve a las
    HH:MM); empieza <siguiente>».
  - **Avisos de cuota:** por agente, al guardar una lectura que cruza `cuota_aviso` o `cuota_limite`. Se guardan en
    `avisos_presupuesto` con `clase = 'cuota'`, `instancia = ''`, `punto = <agente>` y
    `periodo = 'C:<ventana>:<reinicio>'`, una vez por ciclo de la ventana. El umbral de aviso a 0 los apaga.
  - `--max-budget-usd`: en `SolicitudEjecucion` entra `MaxUSD float64` (sin JSON público), que pone el planificador
    con lo que queda del límite USD más estricto de la API y del espacio (día, semana y mes). Claude lo pasa como
    `--max-budget-usd` solo con clave y si es mayor que 0.
  - CLI: `tramtai agentes cuota <agente> [--aviso 5h=70] [--limite semanal=90]`; sin opciones, enseña la cuota y los
    umbrales.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestPararPorCuota`: Claude con una lectura `agotada` y la API con `cuota: [parar]` rechaza con `cuota_agotada` y
    `Retry-After` hasta el reinicio;
  - `TestRespaldoPorCuota`: con `cuota: [respaldo]` responde el respaldo y el evento lo dice;
  - `TestTodosSinCuota`: con `respaldo` y `parar` y todos en su límite, rechaza; solo con `respaldo`, se intenta el
    principal;
  - `TestCuotaPasadaDeReinicioNoPara`: una lectura `agotada` con reinicio hace 1 min no rechaza;
  - `TestCuotaPorDefectoSoloAvisa`: sin `al_limite`, nunca rechaza por cuota y deja un aviso de clase `cuota` una
    vez por ciclo;
  - `TestRechazoDelProveedorSaltaAlRespaldo`: un `rejected` en el intento salta al respaldo con `cuota_agotada`;
  - `TestMaxBudgetUSD`: con clave y 2 USD por gastar en el día, Claude recibe `--max-budget-usd 2`; con suscripción no
    lo recibe;
  - en `api_test.go`, el cuerpo de `cuota_agotada` en la API nativa, OpenAI y Anthropic.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar** lo de Interfaces.
- [ ] **Paso 4:** pruebas en verde, con `-race`.
- [ ] **Paso 5: commit**, `feat(cuota): parar o saltar al respaldo según la cuota de la suscripción`.

### Tarea 7: el informe de uso en el servicio

**Archivos:**
- Crear: `internal/servicio/uso.go`
- Modificar: `internal/servicio/admin.go` (rutas), `internal/configuracion/configuracion.go` (`uso.*`),
  `internal/credenciales/` (`Nombres`), `internal/api/openapi.json`, `internal/cliente/` (funciones nuevas)
- Crear: `internal/conciliacion/conciliacion.go`
- Probar: `internal/servicio/uso_test.go`, `internal/conciliacion/conciliacion_test.go`

**Interfaces:**
- **Consume:** `Almacen.UsoDiario` (T3), `Almacen.Cuotas` (T4), `Almacen.AvisosPresupuesto` (T5), `precios` (T2) y los
  umbrales de cuota (T6).
- **Produce:**
  - Claves globales `uso.moneda` (texto ISO 4217 de 3 letras o vacío), `uso.tipo_cambio` (decimal > 0),
    `uso.tipo_cambio_fecha` (AAAA-MM-DD) y `uso.precios_propios` (de la T3).
  - `credenciales.Nombres` gana `ANTHROPIC_ADMIN_KEY` y `OPENAI_ADMIN_KEY`, así que salen en Ajustes › Claves como las
    demás.
  - `InformeUso`, JSON estable:
    - `cuotas`: lecturas con sus umbrales;
    - `dinero`: `{hoy, semana, mes}`, cada uno `{usd, moneda?}`;
    - `facturado`: opcional, `{mes_usd, por_dia: [{dia, usd}], leido_en, error?}`;
    - `series`: `[{periodo, peticiones, tokens, usd, errores, respaldos, ms_medio}]`, por día o por semana;
    - `tabla`: `[{api, peticiones, tokens, usd, sin_precio, presupuesto?}]`;
    - `precios`: `{fecha, origen, propios}`;
    - `moneda`: `{codigo, cambio, fecha}` o null;
    - `avisos`.
  - Rutas:
    - `GET /api/v1/servicio/uso?instancia&desde&hasta&agrupar=dia|semana&api&agente&modelo`;
    - `GET /api/v1/servicio/uso/cuotas`;
    - `POST /api/v1/servicio/uso/cuotas/actualizar` con `{peticion_minima: bool}`. Llama a cada `LectorCuota`, y con
      `peticion_minima`, a Claude con «responde OK» y su modelo más barato;
    - `GET /api/v1/servicio/uso/avisos`;
    - `GET /api/v1/servicio/uso/precios`.
  - `conciliacion.Anthropic(ctx, clave, desde, hasta)` y `conciliacion.OpenAI(...)` devuelven `([]DiaFacturado, error)`.
    El servicio guarda el resultado una hora en memoria. Sin clave de administrador, `facturado` no sale.
  - Cliente Go: `Uso(ctx, FiltroUso) (InformeUso, error)`, `ActualizarCuotas(ctx, minima bool)` y `Avisos(ctx)`.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestInformeDineroSoloClave`: filas de clave y de suscripción; el dinero solo suma las de clave;
  - `TestInformeMoneda`: con `CLP` a 950, 2 USD salen como 1900 CLP con la fecha del cambio;
  - `TestInformeSemanal`: `agrupar=semana` junta lunes a domingo;
  - `TestInformeDesconocido`: una cuota sin `usado` sale con `usado: null`, no 0;
  - `TestInformeLecturaVieja`: una lectura de hace 20 min sale con `vieja: true`; una con el reinicio pasado sale con
    `usado: null` y `estado: ""`;
  - `TestConciliacionAnthropic` y `TestConciliacionOpenAI` contra `httptest` con las respuestas documentadas (centavos
    en Anthropic); un 404 da `error` y deja el estimado;
  - `TestConciliacionUnaVezPorHora`.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar** lo de Interfaces y el OpenAPI.
- [ ] **Paso 4:** pruebas en verde, más la prueba del OpenAPI que ya existe.
- [ ] **Paso 5: commit**, `feat(uso): informe de uso, moneda y conciliación con la factura`.

### Tarea 8: `tramtai uso`

**Archivos:**
- Crear: `cmd/tramtai/uso.go`
- Modificar: `cmd/tramtai/main.go`
- Probar: `cmd/tramtai/uso_test.go`

**Interfaces:**
- **Consume:** el cliente de la Tarea 7.
- **Produce:** `tramtai uso` (alias `usage`) con `--periodo 7d|30d|90d` (por defecto 30d), `--desde`, `--hasta`, `--api`,
  `--agente`, `--modelo`, `--instancia`, `--actualizar` y `--json` (escribe `InformeUso` tal cual).

- [ ] **Paso 1: escribir las pruebas que fallan:** la salida de texto con un servicio simulado, que lleva las barras de
  cuota con «desconocido», el dinero en USD y en la moneda, y la tabla; `--json` es JSON válido con las claves de
  `InformeUso`; `--periodo 5d` da error.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar.** Mismo estilo que `tramtai trabajos`.
- [ ] **Paso 4:** pruebas en verde.
- [ ] **Paso 5: commit**, `feat(cli): tramtai uso`.

### Tarea 9: la página Uso en la Web UI

**Archivos:**
- Crear: `web/src/paginas/Uso.vue`, `web/src/componentes/uso/{TarjetaCuota,DineroUso,TendenciasUso,TablaUso}.vue`
- Modificar: `web/src/ayuda/tipos.ts` (`PAGINAS`), `web/src/menu.ts` (grupo Conectar: conectar, historial, uso),
  `web/src/App.vue`, `web/src/paginas/Panel.vue` (resumen), `web/src/api/` (llamadas),
  `web/src/idiomas/{es,en}.ts`, `web/src/componentes/accesos/FormularioApi.vue` (reacciones y aviso),
  `web/src/componentes/EditorPresupuesto.vue` (semana y aviso), `web/src/paginas/Accesos.vue` (editar el presupuesto
  del espacio), `web/src/paginas/Agentes.vue` (umbrales de cuota), `web/src/paginas/Configuracion.vue` (moneda y
  precios propios)
- Probar: `web/src/**/*.test.ts` junto a cada componente nuevo

**Interfaces:**
- **Consume:** las rutas de la Tarea 7 y las claves de las Tareas 5 y 6.
- **Produce:** la página `uso` como en spec §2.1 y el resumen de Inicio de §2.2.
  - Gráficas en SVG a mano, sin librerías, siguiendo la guía `dataviz`.
  - Período 7, 30 o 90 días.
  - Filtros por API, agente, modelo y espacio.
  - La cuota más apurada en Inicio es la ventana con mayor `usado` (las «desconocido» no compiten).
  - El presupuesto del espacio se guarda con la misma llamada con la que Ajustes guarda las claves `presupuesto.*`.
  - En el formulario de la API, elegir «Pasar al respaldo» sin respaldo configurado enseña el aviso «esta API no tiene
    respaldo: añade uno o la petición seguirá con el principal».

- [ ] **Paso 1: escribir las pruebas que fallan:** `TarjetaCuota` enseña «desconocido» sin `usado`, la cuenta atrás,
  «leída hace 3 min» y, con `vieja`, «última lectura hace 20 min»; el formulario avisa del respaldo que falta; `DineroUso` sin moneda solo enseña USD; `TablaUso` ordena por columna; el resumen de Inicio elige
  la cuota más apurada.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** pruebas en verde y `npm run build`. En el navegador, con un servicio de pruebas en el puerto 17388:
  375 y 1440 px, claro y oscuro, es y en, teclado, y `impeccable` sobre la página. Ninguna captura todavía.
- [ ] **Paso 5: commit**, `feat(web): página Uso y resumen en Inicio`.

### Tarea 10: la página Uso en la terminal

**Archivos:**
- Crear: `internal/tui/paginas/uso/`
- Modificar: `internal/tui/menu.go`, `internal/tui/paginas/inicio/`, `internal/tui/paginas/conectar/` (presupuesto del
  espacio, semana, aviso y reacciones), `internal/tui/paginas/agentes/` (umbrales de cuota)
- Probar: `internal/tui/paginas/uso/*_test.go`

**Interfaces:**
- **Consume:** el cliente de la Tarea 7.
- **Produce:** la página Uso con las cuotas en barras, minigráficas de una línea (`▁▂▃▄▅▆▇█`), el dinero y la tabla, más el
  resumen en Inicio, con la misma navegación que las demás páginas.

- [ ] **Paso 1: escribir las pruebas que fallan,** con el patrón de las demás páginas (vista con datos fijos): a 80
  columnas cabe sin cortar palabras; «desconocido» sale tal cual; el resumen de Inicio.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** pruebas en verde.
- [ ] **Paso 5: commit**, `feat(terminal): página Uso`.

### Tarea 11: alias que fijan el esfuerzo

**Archivos:**
- Modificar: `internal/api/fachadas.go` (`traducirModelo`, `solicitudFachada`, `aceptarFachada`),
  `internal/api/openai.go` y `internal/api/anthropic.go` (prioridad sobre el esfuerzo de la petición),
  `internal/servicio/puntos.go` (`normalizarPunto`), `web/src/componentes/accesos/FormularioApi.vue` (filas),
  `internal/tui/paginas/conectar/formulario.go`
- Probar: `internal/api/fachadas_test.go`, `internal/servicio/puntos_test.go`, el test de `FormularioApi`

**Interfaces:**
- **Produce:**
  - `func (s *Servidor) traducirModelo(model string) (destino, esfuerzo string)`: parte el destino del alias con
    `nucleo.DividirRespaldo`.
  - `SolicitudEjecucion` gana `EsfuerzoFijado bool` (sin JSON público). Con alias con `+nivel`,
    `Esfuerzo = nivel` y `EsfuerzoFijado = true`, y ningún esfuerzo de la petición lo cambia; si la petición pedía
    otro, `Tramtai-Advertencia: esfuerzo_fijado_por_alias`.
  - `aceptarFachada` pone `Tramtai-Esfuerzo: <nivel>` cuando `sol.Esfuerzo != ""`.
  - `normalizarPunto` valida el `+nivel` de cada alias con `agentes.NivelesDeModelo(agente, modelo)`. Si no vale:
    `configuracion_invalida` con «el alias X pide el esfuerzo Y, que <agente>/<modelo> no admite».
  - Web UI: filas `nombre | destino | esfuerzo`. El selector solo ofrece `NivelesDeModelo` (del catálogo que ya baja
    la página) y se oculta si está vacío. Se guarda como `nombre=destino+nivel`.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `TestAliasFijaEsfuerzo`: `rapido=claude/haiku+bajo` con `reasoning_effort: high` ejecuta con `bajo`, con las
    cabeceras `Tramtai-Esfuerzo: bajo` y la advertencia;
  - `TestAliasSinNivelComoHoy`;
  - `TestAliasNivelNoAdmitido`: `x=copilot/auto+alto` se rechaza al guardar;
  - el test de las filas del formulario.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** pruebas en verde y `npm run build`.
- [ ] **Paso 5: commit**, `feat(apis): alias que fijan el esfuerzo`.

### Tarea 12: «Comprobar conexión»

**Archivos:**
- Crear: `internal/servicio/comprobar.go`
- Modificar: `internal/servicio/admin.go` y `internal/servicio/herramientas.go` (rutas), `internal/cliente/`,
  `cmd/tramtai/acceso.go`, `cmd/tramtai/herramientas.go`, `web/src/paginas/Accesos.vue` y la vista de herramientas,
  `internal/tui/paginas/conectar/`, `internal/api/openapi.json`, `internal/adaptadores/` (`Clasificar`)
- Probar: `internal/servicio/comprobar_test.go`, `cmd/tramtai/acceso_test.go`

**Interfaces:**
- **Consume:** `LectorCuota` y `enLimite` (T4 y T6), `estadoHerramienta`.
- **Produce:**
  - `type Paso struct { ID, Estado, Detalle, Arreglo string }`:
    - `ID` es uno de `servicio`, `api`, `clave`, `activa`, `modelo`, `agente`, `cuota`, `real`, y además `archivo`
      en las herramientas;
    - `Estado` es `bien`, `mal` o `no_aplica`;
    - `Arreglo` es la página (`conectar`, `agentes`, `uso`…) o la orden que lo arregla.
  - `POST /api/v1/servicio/puntos/{nombre}/comprobar` con `{clave?: string, real: bool}` → `{pasos: []Paso}`.
    - Con `clave`, el paso `clave` la compara con la huella guardada.
    - Sin ella, comprueba que la API tiene clave y queda `bien` con «no se comprobó la clave de tu app».
  - `POST /api/v1/herramientas/{id}/comprobar` con `{real: bool}`: primero `archivo` (el motivo de
    `estadoHerramienta`) y después los pasos de su API, con su clave del almacén.
  - El paso `real` hace la petición mínima con el agente de la API y clasifica el error:
    - `presupuesto_agotado`;
    - límite de ritmo, «reintenta en X s», con el `retry-after` del mensaje si viene;
    - `cuota_agotada`, con la hora;
    - tope de gasto (`enforced_spend_limit_reached`, que `Clasificar` reconoce como `limite_uso_proveedor` con su
      propio mensaje);
    - `autenticacion_agente`, sin sesión.
  - CLI: `tramtai acceso comprobar <api> [--clave X] [--real] [--json]` y
    `tramtai herramientas comprobar <herramienta> [--real] [--json]`. El paso `servicio` lo hace la CLI misma, si no
    conecta.
  - Web UI y terminal: un botón «Comprobar conexión» en cada API y en cada herramienta, con la lista de pasos, un
    campo opcional para la clave de la app y «Probar con una petición real».

- [ ] **Paso 1: escribir las pruebas que fallan:** cada paso bien y mal con agentes simulados; la clave mal da
  `clave: mal` con `Arreglo: conectar`; cada clase de error del paso `real`; la herramienta editada a mano da
  `archivo: mal`; la CLI sin servicio da `servicio: mal`.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** pruebas en verde y `npm run build`.
- [ ] **Paso 5: commit**, `feat(apis): Comprobar conexión`.

### Tarea 13: Ayuda, documentación, capturas y sitio

**Archivos:**
- Crear: `web/src/ayuda/temas/uso.ts`
- Modificar:
  - Ayuda: `web/src/ayuda/indice.ts`, `web/src/idiomas/ayuda.{es,en}.ts`, `web/src/ayuda/temas/{presupuestos,
    conectar,conectar-herramientas,problemas}.ts` y `web/scripts/capturas-lista.ts`;
  - terminal: `internal/tui/datos/*.json`, regenerado;
  - documentación: `docs/api.md`, `docs/desarrolladores.md` y `README.md`;
  - sitio: `site/producto.html` y la Ayuda del sitio (`npm run sitio`).

- [ ] **Paso 1:** tema «Uso y gasto» al estilo de la v0.52:
  - los tres orígenes;
  - la cuota y por qué Tramtai no ve todo el consumo;
  - el dinero con la tabla y su fecha;
  - la moneda y la conciliación;
  - lo que la M0 no confirmó, si algo.

  Al día:
  - Presupuestos: quitar la nota de que el límite en dólares solo cuenta Claude; añadir la semana, el aviso, la cuota
    y las reacciones;
  - Mis APIs: alias con esfuerzo y «Comprobar conexión»;
  - Herramientas;
  - Problemas: `cuota_agotada` en `ERRORES`.
- [ ] **Paso 2:** `docs/api.md` y la guía de desarrolladores: `cuota_agotada`, `Tramtai-Esfuerzo`, la semana,
  `al_limite` y las rutas de uso. En el README, la página Uso y «Comprobar conexión».
- [ ] **Paso 3:** capturas nuevas `uso` (y las del formulario de la API que cambiaron) en es y en, con `--solo`. Se
  cuentan las peticiones reales que hacen.
- [ ] **Paso 4:** regenerar la terminal y el sitio. Pasan `npm test`, `estilo-textos.test.ts` y
  `TestComandosDelSitioExisten`.
- [ ] **Paso 5: commit**, `docs(v0.62): Ayuda, guías, capturas y sitio`.

### Tarea 14: cierre y publicación

**Archivos:**
- Modificar: `docs/equipo/pendientes.md` (la fila de la v0.62 y el pendiente de los avisos en el período de aceptación),
  y la spec con los Rulings que cambien el diseño

- [ ] **Paso 1: auditoría** de la página Uso, el resumen de Inicio, el formulario de la API y «Comprobar conexión»:
  - con `impeccable`, `antislop:antislop-ui` y teclado;
  - a 375 y 1440 px, en claro y oscuro, en es y en, con contraste AA.
- [ ] **Paso 2: revisión de la rama entera** con `superpowers:requesting-code-review` y el foco de «Lo que más puede
  fallar». Los Minor se arreglan aquí.
- [ ] **Paso 3:** `go vet ./...`, `go test -race ./...` y `cd web; npm test; npm run build; npm run sitio` pasan, y no
  dejan cambios. La tabla de precios tiene menos de 60 días (`TRAMTAI_COMPROBAR_PRECIOS=1 go test ./internal/precios`).
- [ ] **Paso 4:** `pendientes.md` con la v0.62 lista. Notas del release en es y en (`notas-es.txt`, `notas-en.txt`).
- [ ] **Paso 5: commit**, `docs(v0.62): cierre`.
- [ ] **Paso 6: con el «sí» del usuario:**
  - fusionar los PR en orden, cambiando antes su base a `main`;
  - etiquetar con `scripts/version.ps1 -Etiquetar -Resumen notas-es.txt -ResumenEn notas-en.txt`;
  - esperar a Publicar, comprobar el release y el sitio publicado.

## Orden y dependencias

- **Pueden empezar ya:** la Tarea 1 (con el permiso del usuario), la Tarea 2 y la Tarea 11.
- **Medida y cuota:**
  - la 3 necesita la 2;
  - la 4 necesita la 1 y la 3;
  - la 5 necesita la 3;
  - la 6 necesita la 4 y la 5.
- **Superficies:**
  - la 7 necesita de la 2 a la 6;
  - la 8, la 9 y la 10 necesitan la 7;
  - la 12 necesita la 6 y la 7.
- **Cierre:** la 13 va cuando todo lo demás está hecho, y la 14 la última.
- Las ramas se apilan en ese orden. La 11 sale de `main` y se rebasa al final si hace falta.
