# Espacios de trabajo (v0.54) — plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** espacios de trabajo gestionables y estables (registro con id fijo, confianza, página «Espacios», Ajustes
claros, coherencia), agentes que detectan los cambios de su CLI y se pueden desconectar, la «Clave del espacio»
explicada, y la web y la Ayuda al día y mejoradas.

**Architecture:** el servicio pasa de `instancias.json` (solo activos) a un registro `datos/espacios.json` con todos
los espacios conocidos, una marca `.tramtai/espacio` en cada proyecto y rutas `/ui/espacios…` para gestionarlos. Los
agentes ganan una comprobación de sesión sin coste y un estado de conexión compartido (`adaptadores.Conexiones`) que
respetan el planificador y la API. La Web UI añade la página «Espacios», divide Ajustes y amplía Agentes y Mis APIs,
con su lógica en módulos puros probados; el sitio se genera y prueba como hoy.

**Tech Stack:** Go (net/http, cobra, SQLite modernc), Vue 3 + Quasar, `node:test`, sitio estático `site/` generado en
parte por `web/scripts/sitio.ts`.

**Spec:** `docs/equipo/releases/v0.54-espacios-de-trabajo/spec.md`

## Global Constraints

- Todo texto nuevo de app, Ayuda, web y CLI va en **español e inglés** (`web/src/idiomas/es.ts`/`en.ts`,
  `ayuda.es.ts`/`ayuda.en.ts`, `internal/idioma/en_*.go` para cada `idioma.T`; `TestCobertura` lo exige).
- Ayuda con el estilo de la v0.52: resumen ≤ 140 caracteres, textos de bloque ≤ 200, ninguna clave `ayuda.*` sin usar,
  anclas únicas (`web/src/ayuda/estilo.test.ts`).
- «Espacio de trabajo» en todo texto visible; las rutas `/api/v1/instancia…` y el campo `instancia` **no cambian**.
- La «API principal» se llama **«Clave del espacio»**.
- Nombres de ejemplo neutros (nada de «facturas»).
- Antes de cada commit de código: `go vet ./...`, `TRAMTAI_IDIOMA=es go test ./...` y, si toca la web, `cd web; npm
  test` y `npm run build`; si toca la Ayuda o `sitio.ts`, `npm run sitio` (el CI compara `site/ayuda`).
- Nunca push a `main`; todo por PR. Un commit por tarea como mínimo, mensajes en español.

## Review Focus

1. Una marca `.tramtai/espacio` con un id ajeno (un repositorio clonado que la versiona, o con `global`) no debe
   adueñarse del historial ni de las APIs de otro espacio: `global` y los ids que no son de un proyecto registrado se
   ignoran, y un id cuya ruta registrada sigue existiendo es una copia (Tarea 1: `TestMarcaAjenaNoAdopta`).
2. «Olvidar» con un id manipulado (`../`, rutas absolutas) no borra nada fuera de `datos/`, y nunca la carpeta del
   proyecto (Tarea 3: `TestOlvidarSoloDentroDeDatos`, `TestOlvidarNoTocaLaCarpeta`).
3. `espacios.json` dañado o a medio escribir: el servicio arranca con la global, guarda el dañado aparte y lo dice
   (Tarea 1: `TestRegistroDanado`).
4. Abrir un proyecto no confiado cuyo `local.yaml` trae claves administradas no debe fallar ni aplicarlas: se abre sin
   ellas y lo avisa (Tarea 2: `TestAbrirSinConfiarIgnoraClavesAdministradas`).
5. El 403 por CSRF viejo en otra pestaña renueva la sesión **una vez** y recarga; nunca entra en un bucle (Tarea 8:
   prueba de `cliente.test.ts` «un 403 de CSRF renueva una sola vez»).

---

### Task 1: Registro de espacios, id fijo y marca

**Files:**
- Create: `internal/servicio/espacios.go`, `internal/servicio/espacios_test.go`
- Modify: `internal/servicio/servicio.go` (campo `espacios *registro` en `Servicio`, cargado en `Nuevo`),
  `internal/servicio/instancias.go` (`ActivarProyecto`, `guardarActivas`, `Restaurar`, `Estado`),
  `internal/contrato/contrato.go` (`EstadoInstancia.UltimoUso`)

**Interfaces:**
- Produces:
  - `type Espacio struct { ID, Ambito, Ruta, Nombre string; Activo bool; UltimoUso time.Time; UltimoError string }`
    con tags JSON `id, ambito, ruta, nombre, activo, ultimo_uso, ultimo_error` (`omitempty` en los tres últimos).
  - `type registro struct` (privado) con `cargarRegistro(datos string) (*registro, error)`, `(r) guardar() error`,
    `(r) lista() []Espacio` (global primero, después por nombre), `(r) buscar(id string) (Espacio, bool)`,
    `(r) poner(e Espacio)`, `(r) quitar(id string)`. Protegido por su propio `sync.Mutex`.
  - `func leerMarca(raiz string) string` y `func escribirMarca(raiz, id string) error` (también añade la línea
    `espacio` al `.tramtai/.gitignore` si falta).
  - `func (s *Servicio) idDe(raiz string, c *nucleo.Configuracion) (id string, err error)`: resuelve el id de una
    carpeta (marca → registro → `IDProyecto`), detecta movido/copia y actualiza el registro.
  - `func (s *Servicio) marcarUso(id string)`: pone `UltimoUso = ahora` y guarda.
  - `contrato.EstadoInstancia` gana `UltimoUso time.Time \`json:"ultimo_uso,omitempty"\``.

- [ ] **Step 1: Write the failing tests** en `espacios_test.go` (usa `entorno(t, …)` de `servicio_test.go:26`):
  - `TestMigrarInstanciasJSON`: con `datos/instancias.json` = `{"global":true,"proyectos":[<proyecto>]}`, tras
    `Nuevo` existe `datos/espacios.json` con la global y el proyecto (`Activo: true`, `Nombre: "Demo App"`) y
    `instancias.json` ya no existe.
  - `TestIdFijoAlRenombrar`: activar el proyecto, cambiar `proyecto.nombre` en su `config.yaml`, desactivar y volver a
    activar → mismo `ID`; `leerMarca(proyecto) == ID`.
  - `TestMoverConMarca`: activar, desactivar, mover la carpeta (`os.Rename`) y `ActivarProyecto(nueva)` → mismo `ID`,
    `Ruta` nueva en el registro, mismo token (`RutaTokenInstancia`).
  - `TestCopiaEsNueva`: copiar la carpeta (con su marca) a otra ruta y activar la copia → `ID` distinto; la marca de
    la copia tiene el nuevo; el original conserva el suyo.
  - `TestMarcaAjenaNoAdopta` (Review Focus 1): una carpeta con marca `global`, y otra con un id que no está en el
    registro → ambas reciben un id calculado con `IDProyecto` y se les reescribe la marca.
  - `TestDatosSueltosNoEncontrado`: un `datos/instancias/viejo_1234abcd.db` sin entrada → `lista()` incluye
    `{ID: "viejo_1234abcd", Ambito: "proyecto", Ruta: "", Nombre: "viejo_1234abcd", Activo: false}`.
  - `TestRegistroDanado` (Review Focus 3): `espacios.json` con `{` → arranca con la global activa y existe
    `espacios.json.dañado`.
  - `TestRestaurarCarpetaBorradaDejaError`: registro con un proyecto activo cuya carpeta no existe → `Restaurar()`
    devuelve un error, el espacio queda `Activo: false` y `UltimoError` no vacío; `Estado()` lo incluye.
  - `TestGitignoreMarca`: tras activar un proyecto creado antes de la v0.54 (sin `espacio` en su `.gitignore`), el
    `.gitignore` contiene `espacio` una sola vez.

- [ ] **Step 2: Run them to verify they fail**
  Run: `TRAMTAI_IDIOMA=es go test ./internal/servicio -run "Migrar|IdFijo|Mover|Copia|MarcaAjena|Sueltos|RegistroDanado|CarpetaBorrada|GitignoreMarca"`
  Expected: FAIL (no compila: `leerMarca`, `registro`… no existen).

- [ ] **Step 3: Implement** `espacios.go` y los cambios:
  - Escritura con `escribirJSONArchivo` (atómica, `escritura.go:54`); el dañado se renombra a `espacios.json.dañado`
    como hace hoy `Restaurar` con `instancias.json`.
  - `ActivarProyecto` usa `idDe` en vez de `IDProyecto` directo y llama a `escribirMarca`.
  - `idDe`: una marca vale si su id es de un proyecto registrado; si la `Ruta` registrada ya no tiene
    `.tramtai/config.yaml` → movido (actualiza `Ruta`); si la tiene y es otra carpeta → copia (id nuevo con
    `IDProyecto` y, si colisiona, sufijo `-2`, `-3`…). Rutas comparadas como hoy (`instancias.go:51-53`: minúsculas en
    Windows y macOS).
  - `guardarActivas` pasa a actualizar `Activo` en el registro y guardarlo (se mantiene el nombre para no tocar a sus
    cuatro llamadores).
  - `Restaurar` recorre los espacios con `Activo: true` del registro (sin registro: la global, como hoy) y guarda
    `UltimoError` del que falla.
  - `Nombre`: `proyecto.nombre` o `filepath.Base(raiz)`.

- [ ] **Step 4: Run the tests to verify they pass**, y la suite del paquete
  Run: `TRAMTAI_IDIOMA=es go test ./internal/servicio ./internal/contrato`
  Expected: PASS (incluida `TestRestaurarConInstanciasDanado`, que se adapta al registro).

- [ ] **Step 5: Commit** — `feat(servicio): registro de espacios con id fijo y marca en la carpeta`

### Task 2: Confianza al abrir un proyecto

**Files:**
- Modify: `internal/servicio/proyecto.go` (`AbrirProyecto`), `internal/nucleo/configuracion.go`,
  `internal/nucleo/errores.go`, `internal/api/api.go` (`estadosHTTP`), `internal/servicio/web_servicio.go`
  (`abrirProyectoUI`), `internal/idioma/en_servicio.go`
- Test: `internal/servicio/confianza_test.go`, `internal/nucleo/configuracion_test.go`

**Interfaces:**
- Consumes: `idDe`, `escribirMarca` (Tarea 1).
- Produces:
  - `nucleo.CodigoConfianzaRequerida = "confianza_requerida"` y `nucleo.CodigoDentroDeProyecto =
    "dentro_de_proyecto"`, ambos **409** en `estadosHTTP`.
  - `func nucleo.ClavesAdministradas(ruta string) ([]string, error)`: claves de primer y segundo nivel de un YAML
    que caen en `prefijosAdministrados` (`configuracion.go:133`), ordenadas; archivo ausente → `nil, nil`.
  - `type AbrirOpciones struct { Confiar *bool; CrearAqui bool }` y
    `func (s *Servicio) AbrirProyecto(raiz string, o AbrirOpciones) (*Instancia, error)`.
  - `type ErrorConfianza struct { Raiz string; Claves []string }` y `type ErrorDentroDeProyecto struct { Proyecto
    string }` (implementan `error`; `abrirProyectoUI` los responde 409 con
    `{"error":{"codigo","mensaje"},"claves":[…]}` / `{"error":…,"proyecto":"…"}`).
  - `func (s *Servicio) FijarConfianza(raiz string, confiar bool) error`: añade o quita `raiz` de
    `proyectos_confiables` en la configuración global (`FijarValor`, `puntos.go:528`) y, si el espacio está activo,
    recarga su configuración.
  - Cuerpo de `POST /ui/proyectos`: `{raiz, confiar?: bool, crear_aqui?: bool}`.

- [ ] **Step 1: Write the failing tests**
  - `TestClavesAdministradas` (nucleo): un `local.yaml` con `comandos_agentes.claude`, `puntos_acceso` y
    `predeterminados.agente` → `["comandos_agentes", "puntos_acceso"]`.
  - `TestAbrirCreaYConfia`: carpeta sin `.tramtai/` → `AbrirProyecto(dir, AbrirOpciones{})` la crea y
    `ProyectoConfiable(dir)` es verdadero; crear una API en ese espacio (`crearPuntoUI`) funciona.
  - `TestAbrirExistentePideConfianza`: carpeta con `.tramtai/` y un `local.yaml` con `comandos_agentes` →
    `*ErrorConfianza` con `Claves == ["comandos_agentes"]`; con `Confiar: &true` se abre y queda confiada.
  - `TestAbrirSinConfiarIgnoraClavesAdministradas` (Review Focus 4): el mismo caso con `Confiar: &false` → se abre,
    no queda confiada, `comandos_agentes` del `local.yaml` **no** se aplica y el espacio tiene `UltimoError` con el
    aviso «local.yaml tiene claves que solo valen en carpetas confiables».
  - `TestAbrirDentroDeOtroProyecto`: subcarpeta de un proyecto → `*ErrorDentroDeProyecto{Proyecto: <raíz>}`; con
    `CrearAqui: true` crea `.tramtai/` en la subcarpeta.
  - `TestDejarDeConfiar`: `FijarConfianza(raiz, false)` la quita de `proyectos_confiables`.
  - `TestAbrirProyectoUI409`: `POST /ui/proyectos` sobre una carpeta existente sin confiar responde 409 con
    `claves` en el cuerpo.

- [ ] **Step 2: Run them to verify they fail** — `TRAMTAI_IDIOMA=es go test ./internal/nucleo ./internal/servicio -run "ClavesAdministradas|Abrir|DejarDeConfiar"` → FAIL.

- [ ] **Step 3: Implement.** Si `CargarConfiguracion` hoy falla al encontrar claves administradas en un `local.yaml`
  no confiable, cambiar a **ignorarlas** con un aviso (que `abrirInstancia` copia a `UltimoError`). `AbrirProyecto`
  ya no sube a un proyecto superior en silencio: devuelve `ErrorDentroDeProyecto` salvo `CrearAqui`.

- [ ] **Step 4: Run the tests to verify they pass** — misma orden más `go test ./internal/api -run OpenAPI` → PASS.

- [ ] **Step 5: Commit** — `feat(servicio): confianza al abrir un proyecto`

### Task 3: Gestión de espacios en el servicio (activar, detener, ubicar, olvidar)

**Files:**
- Modify: `internal/servicio/espacios.go`, `internal/servicio/web.go` (`rutasDeUI`), `internal/servicio/admin.go`,
  `internal/servicio/web_servicio.go` (`activarGlobalUI`, `abrirProyectoUI` llaman a `actualizarDescriptor`),
  `internal/servicio/claves.go`, `internal/persistencia/` (conteo sin activar)
- Test: `internal/servicio/espacios_test.go`

**Interfaces:**
- Consumes: registro (Tarea 1), `FijarConfianza` (Tarea 2).
- Produces:
  - `type EspacioUI struct { Espacio; Estado string \`json:"estado"\`; Confiado bool \`json:"confiado"\`; APIs int
    \`json:"apis"\`; Peticiones int \`json:"peticiones"\` }`, con `Estado` ∈ `activo | detenido | no_encontrado`.
  - `func (s *Servicio) Espacios(ctx context.Context) []EspacioUI`
  - `func (s *Servicio) ActivarEspacio(id string) (*Instancia, error)`, `func (s *Servicio) DetenerEspacio(id string)
    error`, `func (s *Servicio) Ubicar(id, ruta string) (*Instancia, error)`, `func (s *Servicio) Olvidar(id, nombre
    string) error`.
  - `func EliminarTokenInstancia(datos, id string)` (junto a `EliminarTokenPunto`, `claves.go:101`).
  - `func persistencia.ContarEn(ruta string) (int, error)`: abre la base solo para leer, cuenta los trabajos y cierra.
  - Rutas `/ui/` (permiso `soloLocal`): `GET /ui/espacios`, `POST /ui/espacios/{id}/activar`, `POST
    /ui/espacios/{id}/detener`, `POST /ui/espacios/{id}/confiar` (`{confiar}`), `POST /ui/espacios/{id}/ubicar`
    (`{ruta}`), `DELETE /ui/espacios/{id}` (`{nombre}`). Las que cambian de espacio responden la sesión
    (`responderSesion`) y las demás, la lista.
  - Administración (token de servicio): `GET /api/v1/servicio/espacios` (la lista) y `DELETE
    /api/v1/servicio/espacios/{id}` (`{nombre}`), para la CLI.

- [ ] **Step 1: Write the failing tests**
  - `TestEspaciosListaEstados`: global activa, un proyecto detenido y uno con la carpeta borrada → estados `activo`,
    `detenido`, `no_encontrado`; `APIs` y `Peticiones` del detenido salen de su configuración y de `ContarEn`.
  - `TestDetenerYActivarPorId`: `DetenerEspacio(id)` → `Activo: false` en el registro y fuera de `s.instancias`;
    `ActivarEspacio(id)` lo vuelve a abrir con el mismo token.
  - `TestUbicarReasocia`: espacio `no_encontrado` + `Ubicar(id, nuevaRuta)` (carpeta con `.tramtai/config.yaml`) →
    activo, mismo id y token, marca escrita; una ruta sin `.tramtai/config.yaml` → `CodigoSolicitudInvalida`.
  - `TestOlvidarNoTocaLaCarpeta` (Review Focus 2): proyecto con historial y una API → `Olvidar(id, "Demo App")`
    borra `datos/instancias/<id>.db*`, `datos/cargas/<id>`, su token y el de su API, y su entrada del registro; la
    carpeta del proyecto y todo `.tramtai/` (con la marca) siguen intactos.
  - `TestOlvidarPideNombre`: nombre distinto → `CodigoSolicitudInvalida` y nada borrado; `Olvidar("global", …)` →
    `CodigoSolicitudInvalida`.
  - `TestOlvidarSoloDentroDeDatos` (Review Focus 2): ids `../x`, `C:\x`, `/x` → `CodigoSolicitudInvalida` (los ids
    válidos cumplen `^[a-z0-9][a-z0-9_-]*$`) y cada ruta a borrar se comprueba dentro de `Datos` antes de borrar.
  - `TestRutasEspaciosUI`: con `abrirSesion` (`presupuesto_test.go:17`) y CSRF: `GET /ui/espacios` 200; `POST
    …/detener` 200; sin CSRF 401; `DELETE` con nombre correcto 200.
  - `TestDescriptorTrasActivarDesdeUI`: tras `POST /ui/instancias`, `servicio.json` lista la global en
    `instancias_activas`.

- [ ] **Step 2: Run them to verify they fail** — `TRAMTAI_IDIOMA=es go test ./internal/servicio ./internal/persistencia -run "Espacios|PorId|Ubicar|Olvidar|RutasEspacios|Descriptor"` → FAIL.

- [ ] **Step 3: Implement.** `Olvidar`: valida id y nombre, `Desactivar` si está activo, borra la base (con `-wal` y
  `-shm`), las cargas, el token del espacio y los de sus APIs (nombres leídos de su configuración si se puede), y
  quita la entrada. Nunca llama a nada con `Ruta`.

- [ ] **Step 4: Run the tests to verify they pass** — misma orden → PASS; y `go test ./internal/...`.

- [ ] **Step 5: Commit** — `feat(servicio): gestionar espacios (detener, activar, ubicar, olvidar)`

### Task 4: Coherencia del servicio, la CLI, la terminal y la app de escritorio

**Files:**
- Modify: `internal/servicio/web.go` (`instanciaPredeterminada`, `derivarSesion`, `cambiarSesionUI`),
  `internal/servicio/web_instancia.go` (nueva ruta `POST /api/v1/instancia/heredar`), `internal/servicio/puntos.go`
  (`QuitarValor`), `cmd/tramtai/{main.go,instancia.go,trabajos.go,ejecutar_servicio.go,conversor.go}`, nuevo
  `cmd/tramtai/espacios.go`, `internal/cliente/cliente.go`, `escritorio/main.go` (`prepararSesion`),
  `internal/tui/inicio.go`, `internal/nucleo/configuracion.go` (fuera `instancia.ambito`), `internal/idioma/en_*.go`,
  `internal/api/openapi.json`
- Test: `internal/servicio/web_test.go`, `cmd/tramtai/*_test.go`, `internal/servicio/web_instancia_test.go`

**Interfaces:**
- Consumes: `marcarUso`, `Espacios`, rutas de administración (Tareas 1 y 3).
- Produces:
  - `instanciaPredeterminada()` devuelve el espacio activo con `UltimoUso` más reciente; si no hay, la global activa;
    si no, el primero.
  - `func QuitarValor(ruta string, origen nucleo.Origen, clave string) error` (junto a `FijarValor`).
  - `POST /api/v1/instancia/heredar` `{clave}`: solo en un proyecto (en la global, 400) y solo claves con prefijo
    `predeterminados.`, `retencion.` o `presupuesto.`; quita la clave de `.tramtai/local.yaml` y recarga.
  - `nucleo.CodigoEspacioDetenido = "espacio_detenido"`.
  - `func instanciaActual(ctx context.Context, c *cliente.Cliente, preferida string, global bool) (string, error)`.
  - CLI: `tramtai espacios`, `tramtai espacio iniciar|detener [ruta|id]`, `tramtai espacio olvidar <id> --nombre
    <nombre>`; `tramtai ejecutar` y `tramtai trabajos` aceptan `--global`. `comandoInstancia("global"|"proyecto")`
    sigue registrado como alias.
  - Cliente: `func (c *Cliente) Espacios(ctx) ([]contrato.Espacio, error)` y `func (c *Cliente) Olvidar(ctx, id,
    nombre string) error` (con `contrato.Espacio` = la forma de `EspacioUI`).

- [ ] **Step 1: Write the failing tests**
  - `TestSesionNuevaVuelveAlUltimoEspacio`: con la global y un proyecto activos, cambiar al proyecto
    (`POST /ui/sesion`), crear un navegador nuevo sin cookie → `GET /ui/sesion` responde `instancia` = el proyecto.
  - `TestHeredarQuitaDeLocal`: en un proyecto con `predeterminados.agente: codex` en `local.yaml`, `POST
    /api/v1/instancia/heredar {"clave":"predeterminados.agente"}` → 204, la clave ya no está y la configuración
    efectiva tiene origen `global` o `interno`; clave `comandos_agentes` → 400; en la global → 400.
  - `TestCLINoCaeEnLaGlobal` (cmd/tramtai): con un proyecto detenido como directorio actual, `instanciaActual(…,
    false)` → error con código `espacio_detenido`; con `global=true` → `"global"`.
  - `TestComandoEspacios`: `tramtai espacios` contra un servicio de prueba imprime una fila por espacio con su estado.
  - `TestPrepararSesionNoReactivaLaGlobal` (escritorio): con un proyecto activo y la global detenida,
    `prepararSesion("")` no llama a `Activar` y abre el proyecto (más reciente por `UltimoUso`).

- [ ] **Step 2: Run them to verify they fail** → FAIL.

- [ ] **Step 3: Implement.** `derivarSesion`/`cambiarSesionUI`/`crearSesion` llaman a `marcarUso`. Textos visibles de
  la CLI, la TUI (`inicio.go:18-177`), la bandeja y los errores pasan de «instancia» a «espacio de trabajo», con su
  inglés en `internal/idioma/en_*.go`. `openapi.json` documenta `heredar` y el código `espacio_detenido` no (es de la
  CLI). Fuera la clave de esquema `instancia.ambito`.

- [ ] **Step 4: Run the tests to verify they pass** — `go vet ./... && TRAMTAI_IDIOMA=es go test ./...` → PASS.

- [ ] **Step 5: Commit** — `feat: recordar el espacio, heredar ajustes y CLI de espacios`

### Task 5: Agentes desconectados

**Files:**
- Modify: `internal/adaptadores/registro.go` (`Conexiones`), `internal/agentes/catalogo.go` (valor interno
  `adaptadores.<x>.desconectado: false`), `internal/nucleo/configuracion.go` (`"adaptadores.*.desconectado":
  tipoBool`), `internal/nucleo/errores.go`, `internal/api/api.go`, `internal/api/nativa.go` (`fichasAgentes`,
  `capacidades`), `internal/planificador/{planificador.go,respaldo.go,ejecucion.go}`,
  `internal/servicio/{servicio.go,web.go,web_servicio.go}`, `cmd/tramtai/agente.go` (nuevo), `internal/idioma/en_*.go`,
  `internal/api/openapi.json`
- Test: `internal/adaptadores/registro_test.go`, `internal/planificador/respaldo_test.go`,
  `internal/api/capacidades_test.go`, `internal/servicio/web_test.go`

**Interfaces:**
- Produces:
  - `type Conexiones struct` (atómica, compartida como hoy `Modo`): `func NuevasConexiones(c *nucleo.Configuracion,
    agentes []string) *Conexiones`, `func (c *Conexiones) Desconectado(agente string) bool`, `func (c *Conexiones)
    Fijar(agente string, desconectado bool)`. `nil` = todos conectados.
  - `nucleo.CodigoAgenteDesconectado = "agente_desconectado"` → **409**.
  - El planificador recibe `Conexiones` en sus opciones: `aceptar` rechaza un principal desconectado con
    `agente_desconectado`; `candidatos` salta los de respaldo desconectados; si no queda ninguno, `agente_desconectado`.
  - `/api/v1/agentes` y `/api/v1/capacidades` añaden `"desconectado": bool` por agente.
  - `POST /ui/agentes/{agente}/conexion` `{conectado: bool}` (`soloLocal`): persiste
    `adaptadores.<agente>.desconectado` en la configuración global (`FijarValor`) y aplica `Conexiones.Fijar`.
  - CLI: `tramtai agente desconectar|conectar <agente>` (vía una ruta de administración `POST
    /api/v1/servicio/agentes/{agente}/conexion`).

- [ ] **Step 1: Write the failing tests**
  - `TestConexiones`: desde una configuración con `adaptadores.codex.desconectado: true` → `Desconectado("codex")`;
    `Fijar("codex", false)` lo conecta.
  - `TestPrincipalDesconectado409`: `POST /api/v1/trabajos` con agente `claude` desconectado → 409
    `agente_desconectado`.
  - `TestRespaldoSaltaDesconectado`: principal que falla con `limite_uso_proveedor`, respaldo `["codex","otro"]` con
    `codex` desconectado → lo ejecuta `otro`.
  - `TestRespaldoTodoDesconectado`: principal falla y su único respaldo está desconectado → el trabajo termina con
    `agente_desconectado`, no con el error del principal.
  - `TestCapacidadesInformaDesconectado`: el campo aparece en las dos rutas.
  - `TestConexionUIPersiste`: `POST /ui/agentes/codex/conexion {"conectado":false}` → 204, la configuración global
    tiene `adaptadores.codex.desconectado: true`, y una petición a codex da 409 sin reiniciar.

- [ ] **Step 2: Run them to verify they fail** → FAIL.
- [ ] **Step 3: Implement** con el patrón de `CambiarAutenticacion` (`registro.go:319`) y `cambiarAutenticacion`
  (`web_servicio.go:127`).
- [ ] **Step 4: Run the tests to verify they pass** — `go vet ./... && TRAMTAI_IDIOMA=es go test ./...` → PASS.
- [ ] **Step 5: Commit** — `feat(agentes): desconectar un agente sin desinstalarlo`

### Task 6: Sesión del CLI: evidencia M0, detección activa y pasiva

**Files:**
- Modify: `docs/equipo/m0-agentes.md` (secciones nuevas de Claude Code y Codex; Copilot y Antigravity ampliadas),
  `internal/adaptadores/comun.go` (`Estado.Cuenta`, problemas nuevos, `ComprobadorSesion`, `Salud`),
  `internal/agentes/{claude,codex,copilot,antigravity}/*.go` según la evidencia, `internal/planificador/ejecucion.go`
  (marca la salud), `internal/api/nativa.go` (`capacidades`), `internal/idioma/en_*.go`
- Test: `internal/agentes/*/…_test.go` (CLI falso con `TestMain`, como `claude_test.go:28`),
  `internal/adaptadores/comun_test.go`, `internal/api/capacidades_test.go`

**Interfaces:**
- Produces:
  - `type SesionCLI struct { Estado string; Cuenta string }`, con `Estado` ∈ `ok | sin_sesion | usa_clave_api |
    desconocido`; `type ComprobadorSesion interface { ComprobarSesion(ctx context.Context) SesionCLI }`.
  - `Estado` gana `Cuenta string`; constantes `ProblemaUsaClave = "usa_clave_api"` y `ProblemaSesion =
    "sesion_no_responde"`. En suscripción, `Detectar` llama a `ComprobarSesion` (si el agente lo implementa):
    `sin_sesion` → `ProblemaFaltaClave` (el camino de la tarjeta que ya existe); `usa_clave_api` → `ProblemaUsaClave`;
    `ok` → `Cuenta`.
  - `type Salud struct` (atómica): `func (s *Salud) Fallo(agente string)`, `func (s *Salud) Exito(agente string)`,
    `func (s *Salud) SinRespuesta(agente string) bool`. El planificador llama a `Fallo` cuando un intento termina con
    `autenticacion_agente` y a `Exito` cuando uno termina bien; `capacidades` pone `ProblemaSesion` mientras
    `SinRespuesta` sea verdadero (si `Detectar` no dio otro problema).
  - `capacidades` añade `"cuenta"` cuando no está vacía.

- [ ] **Step 1: M0 (sin código).** En este equipo, con cada CLI real, averiguar y anotar en `m0-agentes.md` (versión,
  comando o archivo, salidas de ejemplo **con la cuenta anonimizada**, si gasta mensajes): cómo saber sin gastar
  mensajes si hay sesión, si se usa clave de API en vez de suscripción y qué cuenta es. Candidatos a verificar:
  `claude auth status` (o el archivo de credenciales de Claude Code), `codex login status`, el `config.json` de
  Copilot que ya lee `sesion()` (`copilot.go:178`, `loggedInUsers`) y lo que ofrezca Antigravity. Si un CLI no
  ofrece nada fiable y gratis, anotarlo: ese agente se queda solo con la detección pasiva.
  Expected: una subsección por CLI con «**Sesión sin coste:**» y la decisión.

- [ ] **Step 2: Write the failing tests**, con las salidas reales anotadas en el paso 1 guardadas en
  `testdata/` de cada agente:
  - Por cada agente con comprobación: `TestComprobarSesionOk` (con `Cuenta` esperada), `TestComprobarSesionSinSesion`,
    `TestComprobarSesionUsaClave` (si el CLI lo distingue) y `TestComprobarSesionNoGastaMensajes` (el registro del CLI
    falso solo contiene el comando de estado, nunca uno de ejecución).
  - `TestDetectarSuscripcionSinSesion` (claude): `Detectar` en suscripción con el CLI falso diciendo que no hay sesión
    → `ProblemaCodigo == ProblemaFaltaClave`.
  - `TestSaludPasiva`: `Fallo("claude")` → `SinRespuesta` verdadero; `Exito("claude")` → falso.
  - `TestCapacidadesSesionNoResponde`: un trabajo que falla con `autenticacion_agente` deja `problema_codigo:
    "sesion_no_responde"` en `capacidades` hasta que otro trabajo de ese agente termina bien.

- [ ] **Step 3: Run them to verify they fail** → FAIL.
- [ ] **Step 4: Implement** según la evidencia; la comprobación usa `EntornoLimpio` y un plazo de 5 s.
- [ ] **Step 5: Run the tests to verify they pass** — `go vet ./... && TRAMTAI_IDIOMA=es go test ./...` → PASS.
- [ ] **Step 6: Commit** — `feat(agentes): detectar los cambios de sesión del CLI`

### Task 7: Página «Espacios», selector y diálogos de abrir

**Files:**
- Create: `web/src/espacios.ts`, `web/src/espacios.test.ts`, `web/src/paginas/Espacios.vue`
- Modify: `web/src/api/tipos.ts` (`EspacioUI`), `web/src/api/cliente.ts`, `web/src/App.vue` (selector :15-37,
  diálogo de proyecto :164-184 y :417-444, `principales` :284), `web/src/ayuda/tipos.ts` (`PAGINAS` + `'espacios'`),
  `web/src/paginas/Panel.vue` (aviso de espacio con error), `web/src/idiomas/es.ts`, `en.ts`

**Interfaces:**
- Consumes: rutas `/ui/espacios…` y el 409 de `/ui/proyectos` (Tareas 2 y 3).
- Produces:
  - `interface EspacioUI { id; ambito: 'global' | 'proyecto'; ruta?; nombre; activo; ultimo_uso?; ultimo_error?;
    estado: 'activo' | 'detenido' | 'no_encontrado'; confiado: boolean; apis: number; peticiones: number }`.
  - En `espacios.ts` (puro): `type Accion = 'abrir' | 'detener' | 'activar' | 'buscar' | 'olvidar' | 'confiar' |
    'dejarDeConfiar'`; `accionesDe(e: EspacioUI, actual: string): Accion[]`; `ordenar(lista: EspacioUI[]):
    EspacioUI[]` (global primero, después activos por `ultimo_uso`, después el resto por nombre); `nombreConfirmado(e,
    escrito: string): boolean` (sin distinguir mayúsculas ni espacios de los extremos).
  - `api.espacios()`, `api.activarEspacio(id)`, `api.detenerEspacio(id)`, `api.confiarEspacio(id, confiar)`,
    `api.ubicarEspacio(id, ruta)`, `api.olvidarEspacio(id, nombre)`; `api.abrirProyecto(raiz, opciones?: { confiar?:
    boolean; crear_aqui?: boolean })`.

- [ ] **Step 1: Write the failing tests** en `espacios.test.ts`:
  - `accionesDe`: global activa y actual → `['detener']` (nunca `olvidar`); proyecto activo no actual y confiado →
    `['abrir','detener','dejarDeConfiar']`; detenido sin confiar → `['activar','confiar','olvidar']`;
    `no_encontrado` → `['buscar','olvidar']`.
  - `ordenar` con global, dos activos y un detenido → el orden indicado.
  - `nombreConfirmado(e, ' demo app ')` con nombre «Demo App» → verdadero; «Demo» → falso.

- [ ] **Step 2: Run them to verify they fail** — `cd web; node --test src/espacios.test.ts` → FAIL.
- [ ] **Step 3: Implement** la lógica y la UI:
  - `Espacios.vue`: `CabeceraPagina tema="espacios"`, una tarjeta por espacio (`data-captura="espacios"` en la lista),
    con estado, ruta, «Confiado/Sin confiar», APIs, peticiones y último uso, y sus acciones; «Olvidar» abre un
    diálogo que pide escribir el nombre y explica que la carpeta del proyecto no se toca; «Buscar la carpeta» pide la
    ruta (con «Examinar…» en la app de escritorio); «Abrir un proyecto…».
  - Selector: termina con «Gestionar espacios» (`ir('espacios')`).
  - Diálogo de proyecto: ante `confianza_requerida` muestra «¿Confías en esta carpeta?» con las claves recibidas y
    tres salidas (Confiar y abrir · Abrir sin confiar · Cancelar); ante `dentro_de_proyecto`, «Esta carpeta está
    dentro de {proyecto}: ¿abrir ese o crear uno aquí?».
  - Inicio: si algún espacio tiene `ultimo_error`, un aviso con enlace a «Espacios».
  - `menu.espacios` / `menuDetalle.espacios` en es/en.
  - Limpieza (spec §2): fuera las claves sin uso `app.proyectoAntes` y `app.proyectoDespues`, y `api.activarGlobal`
    deja de enviar `{ambito}`.
- [ ] **Step 4: Run the tests to verify they pass** — `npm test` y `npm run build` → PASS (la prueba de estilo de la
  Ayuda fallará por el tema de `espacios` hasta la Tarea 11: crear ya el tema mínimo con `paginaApp: 'espacios'` y
  completarlo allí).
- [ ] **Step 5: Commit** — `feat(web): página Espacios y confianza al abrir un proyecto`

### Task 8: Ajustes en dos pestañas, herencia y varias pestañas

**Files:**
- Create: `web/src/herencia.ts`, `web/src/herencia.test.ts`
- Modify: `web/src/paginas/Configuracion.vue`, `web/src/composables/useSesion.ts`, `web/src/api/cliente.ts`,
  `web/src/api/cliente.test.ts`, `web/src/idiomas/es.ts`, `en.ts`

**Interfaces:**
- Consumes: `POST /api/v1/instancia/heredar` (Tarea 4).
- Produces:
  - `herencia.ts`: `type Procedencia = 'propio' | 'global'`; `procedencia(entradas: EntradaConfiguracion[], clave:
    string, ambito: 'global' | 'proyecto'): Procedencia | undefined` (en la global, `undefined`: no se marca; en un
    proyecto, origen `proyecto`/`local` → `propio`, `global`/`interno` → `global`, `entorno` → `undefined`);
    `heredable(clave: string): boolean` (prefijos `predeterminados.`, `retencion.`, `presupuesto.`).
  - `api.heredar(clave)`.
  - `cliente.ts`: un 403 con código `origen_no_permitido` en una petición no GET renueva la sesión **una vez**
    (`renovarSesion`) y lanza `ErrorApi('espacio_cambiado', …)`; `useSesion` lo atiende recargando con el aviso.
  - `useSesion`: `BroadcastChannel('tramtai-sesion')`; al cambiar de espacio publica `{instancia, nombre}`; al recibir
    otro distinto del actual, guarda el aviso en `sessionStorage` y recarga; tras recargar muestra «Cambiaste a
    {nombre} en otra pestaña».

- [ ] **Step 1: Write the failing tests**
  - `herencia.test.ts`: los casos de `procedencia` y `heredable` de arriba.
  - `cliente.test.ts` (Review Focus 5): «un 403 de CSRF renueva una sola vez»: con una respuesta 403
    `origen_no_permitido` repetida, `pedir('POST', …)` llama a `/ui/sesion` una sola vez y lanza `espacio_cambiado`.
- [ ] **Step 2: Run them to verify they fail** → FAIL.
- [ ] **Step 3: Implement.** `Configuracion.vue` en `q-tabs` «Este espacio» (modelo :5-24, historial :104-126,
  presupuesto del espacio, datos :127-156, valores técnicos :159-194) y «Todo Tramtai» (puerto :26-53, conversor
  :55-73, actualizaciones :75-90, arranque :93-103), recordando la pestaña como `Accesos.vue:138`; chip
  «Propio»/«De Global» y «Volver al de Global» donde `heredable`; aviso en la global; textos del presupuesto e
  historial corregidos («en cada espacio por separado», «de este espacio»).
- [ ] **Step 4: Run the tests to verify they pass** — `npm test`, `npm run build` → PASS.
- [ ] **Step 5: Commit** — `feat(web): Ajustes por espacio y de todo Tramtai, con herencia`

### Task 9: Agentes en la Web UI (suscripción, sesión, desconectar)

**Files:**
- Modify: `web/src/agentes.ts` (+ prueba), `web/src/api/tipos.ts` (`Adaptador.cuenta?`, `desconectado?`), `web/src/api/cliente.ts`,
  `web/src/paginas/Agentes.vue`, `web/src/componentes/MapaRed.vue` (`lineas` :120-128), `web/src/componentes/EditorReceta.vue`
  (`usables` :106), `web/src/componentes/accesos/*` (aviso de API con solo un agente desconectado),
  `web/src/paginas/Panel.vue` y `Agentes.vue` (refresco cada 5 min), `web/src/idiomas/es.ts`, `en.ts`

**Interfaces:**
- Consumes: `desconectado`, `cuenta`, `ProblemaUsaClave`, `ProblemaSesion` (Tareas 5 y 6); `POST
  /ui/agentes/{agente}/conexion`.
- Produces (en `agentes.ts`, puro):
  - `type EstadoAgente = 'listo' | 'desconectado' | 'noInstalado' | 'faltaClave' | 'sinSesion' | 'usaClave' |
    'sesionNoResponde' | 'revisar'`; `estadoAgente(a: Adaptador): EstadoAgente` (desconectado primero; en
    suscripción `falta_clave` → `sinSesion`).
  - `ofrecerDejarSuscripcion(a: Adaptador, admiteSuscripcion: boolean): boolean` (verdadero en **cualquier** estado si
    `a.autenticacion === 'suscripcion'` y la ficha admite las dos).
  - `agenteListo` pasa a ser falso si `desconectado`.
  - `api.conectarAgente(agente, conectado)`.

- [ ] **Step 1: Write the failing tests** en `agentes.test.ts`: `estadoAgente` para cada código
  (`no_instalado`, `falta_clave` con clave y con suscripción, `usa_clave_api`, `sesion_no_responde`, `aviso`,
  desconectado); `ofrecerDejarSuscripcion` verdadero con `problema_codigo` vacío, `falta_clave`, `usa_clave_api` y un
  problema genérico; falso con `clave_api`; `agenteListo` falso si `desconectado`.
- [ ] **Step 2: Run them to verify they fail** → FAIL.
- [ ] **Step 3: Implement.** La tarjeta de `Agentes.vue` se arma desde `estadoAgente`:
  - «Dejar de usar la suscripción» donde `ofrecerDejarSuscripcion`.
  - Textos y acciones de `sinSesion` (con el comando para iniciar sesión), `usaClave` y `sesionNoResponde`.
  - «Suscripción de {cuenta}».
  - «Comprobar ahora» (`cargar()`).
  - «Desconectar» / «Conectar».
  - Refresco de `api.capacidades()` cada 5 min mientras la página está a la vista (con `useSondeo` si sirve).
  - Mapa: línea gris con «Desconectado».
  - `EditorReceta` no ofrece agentes desconectados.
  - Mis APIs avisa si una API solo permite un agente desconectado.
- [ ] **Step 4: Run the tests to verify they pass** — `npm test`, `npm run build` → PASS.
- [ ] **Step 5: Commit** — `feat(web): agentes con sesión del CLI a la vista y desconectar`

### Task 10: Clave del espacio

**Files:**
- Modify: `internal/servicio/web_instancia.go` y `internal/contrato/contrato.go` (dato del aviso suave),
  `internal/persistencia/` (conteo por origen y fecha), `web/src/paginas/Accesos.vue`, `web/src/paginas/Panel.vue`,
  `web/src/formato.ts` (+ prueba) o el módulo donde viva el nombre del origen del historial, `web/src/idiomas/es.ts`,
  `en.ts`, `internal/api/openapi.json`
- Test: `internal/servicio/web_instancia_test.go`, `web/src/claves.test.ts`

**Interfaces:**
- Produces:
  - `GET /api/v1/instancia` añade `"app_con_clave_24h": int`: trabajos de origen `app` sin `punto_acceso` en las
    últimas 24 h (`persistencia.(*Almacen).ContarOrigenDesde(ctx, instancia, origen string, sinPunto bool, desde
    time.Time) (int, error)`).
  - `web/src/claves.ts`: `sugerirApiPropia(n: number): boolean` (verdadero si `n > 20`).

- [ ] **Step 1: Write the failing tests**
  - `TestAppConClave24h`: 21 trabajos `app` sin punto en las últimas 24 h y 3 de hace 2 días → 21.
  - `claves.test.ts`: `sugerirApiPropia(20) === false`, `sugerirApiPropia(21) === true`.
- [ ] **Step 2: Run them to verify they fail** → FAIL.
- [ ] **Step 3: Implement.**
  - En la pestaña: «Clave del espacio {nombre}» (`conectar.directa`), el texto nuevo de `conectar.directaTexto`
    («Una por espacio de trabajo, con acceso a todo: úsala para probar, la CLI y tus scripts; da a cada app su propia
    API.») y la explicación de las dos clases al principio de Mis APIs.
  - El historial nombra el origen «Clave del espacio» y el mapa también.
  - El aviso suave aparece en Inicio y Mis APIs.
  - Busca y sustituye todas las apariciones visibles de «API principal» (`historial.origenPrincipal`,
    `historial.origenApp`, `puntos.principal`, `mapa.principal`, `puntos.vacio`, `presupuesto.espacioTexto`).
- [ ] **Step 4: Run the tests to verify they pass** — Go y `npm test`/`npm run build` → PASS.
- [ ] **Step 5: Commit** — `feat: la clave del espacio, explicada`

### Task 11: Ayuda

**Files:**
- Create: `web/src/ayuda/temas/espacios.ts`, `web/src/ayuda/temas/claves.ts`
- Modify: `web/src/ayuda/indice.ts`, `web/src/ayuda/temas/{agentes,ajustes,conectar,conceptos,problemas}.ts`,
  `web/src/idiomas/ayuda.es.ts`, `ayuda.en.ts`, `web/src/ayuda/estilo.test.ts` (14 → 16 temas),
  `web/scripts/sitio.test.ts` (14 → 16), `web/scripts/capturas-lista.ts` (capturas `espacios` y `agente-sesion`)
- Generated: `site/ayuda/*`, capturas `.webp` de la Ayuda

**Interfaces:**
- Consumes: páginas, errores y textos de las Tareas 7 a 10.
- Produces: temas `espacios` (`paginaApp: 'espacios'`, con diagrama global/proyectos/herencia) y `claves` (sin
  página propia; relacionado desde `conectar`).

- [ ] **Step 1: Write the failing test** — subir los conteos a 16 en `estilo.test.ts:73` y `sitio.test.ts:68`; añadir a
  `estilo.test.ts` que `ERRORES` tenga `agente_desconectado`, `confianza_requerida` y `espacio_detenido`.
- [ ] **Step 2: Run it to verify it fails** — `cd web; npm test` → FAIL (15 temas o faltan errores).
- [ ] **Step 3: Write the topics** (estilo v0.52):
  - **«Espacios de trabajo»:** qué son, global y proyectos, qué se hereda, confiar en una carpeta, mover, renombrar u
    olvidar, «No encontrado», con preguntas de anclas `confiar`, `mover`, `olvidar`, `no-encontrado`.
  - **«Claves y APIs»:** las dos clases, cuándo usar cada una y qué pasa al regenerar.
  - **Agentes:** preguntas `dejar-suscripcion`, `sesion-cli` y `desconectar`.
  - **Ajustes:** las dos pestañas y la herencia.
  - **`ERRORES`:** las entradas nuevas.
  - **Capturas:** `npm run capturas` (con el preview en marcha y datos de ejemplo neutros).
- [ ] **Step 4: Run** `npm test`, `npm run sitio`, `npm run build` → PASS y `site/ayuda` regenerado.
- [ ] **Step 5: Commit** — `docs(ayuda): espacios de trabajo, claves y agentes`

### Task 12: Portada: novedades, «Por qué Tramtai» y contenido

**Files:**
- Modify: `site/index.html`, `site/estilo.css`, `web/scripts/capturas.mjs` (portada en WebP), `web/scripts/capturas-lista.ts`
  (`PORTADA` + `espacios`), `web/scripts/capturas.test.ts`, `web/scripts/portada.test.ts`
- Generated: `site/capturas/*.webp` (las `.png` se borran)

**Interfaces:**
- Consumes: la app con todo lo anterior (para las capturas).

- [ ] **Step 1: Write the failing tests** en `portada.test.ts`:
  - «Por qué Tramtai» tiene **6** motivos con títulos es/en («Lo que ya pagas», «Local», «Compatible», «Sin cortes»,
    «Por proyecto», «Bajo control»).
  - Existe la franja «Sin Tramtai / Con Tramtai».
  - Ninguna `<img>` de `capturas/` es `.png`, y todas tienen `width`, `height`, `srcset` y, salvo la primera,
    `loading="lazy"`.
  - El texto de Inicio menciona el mapa en vivo.
  - No queda «API principal».
  - En `capturas.test.ts`: `PORTADA` con 5 entradas (añade `espacios`) y cada `<nombre>-<tema>-<ancho>.webp` existe,
    con `ancho` 960 y 1440.
- [ ] **Step 2: Run them to verify they fail** → FAIL.
- [ ] **Step 3: Implement.**
  - Las secciones de la spec §5 («Por qué» con iconos SVG en línea y la franja).
  - «Así se ve» con Espacios y Agentes.
  - Capturas en WebP: `format: 'webp', quality: 80` en `capturas.mjs:157`, a 1440 y a 960.
  - Los puntos de contenido del anexo A:
    - mapa en vivo y Ayuda v0.52;
    - Antigravity matizado;
    - «streaming; herramientas en las tres APIs»;
    - conversor sin conexión explicado;
    - mensajes repetidos fusionados;
    - textos partidos;
    - un término por cosa y el mismo modelo de ejemplo;
    - comillas inglesas;
    - «uso no comercial» unificado;
    - la versión escrita.
- [ ] **Step 4: Run** `npm test` → PASS; comprobar en el navegador (`pagina`, 8765) móvil y escritorio, claro y oscuro.
- [ ] **Step 5: Commit** — `feat(sitio): portada al día y «Por qué Tramtai» renovado`

### Task 13: Sitio: navegación, diseño y calidad técnica

**Files:**
- Create: `site/404.html`, `site/og.png` (1200×630), `site/favicon.svg`, `site/favicon-32.png`,
  `site/apple-touch-icon.png`, `site/logo-64.png`, `site/manifest.webmanifest`, `web/scripts/og.html` (plantilla de la
  imagen para compartir, capturada con `capturas.mjs`)
- Modify: `site/index.html`, `site/desarrolladores.html`, `site/estilo.css`, `site/sitio.js`, `web/scripts/sitio.ts`
  (cabecera, pie, `<head>`, índice «En esta página», `sitemap.xml`), `web/scripts/sitio.test.ts`
- Generated: `site/ayuda/*`, `site/sitemap.xml`

**Interfaces:**
- Produces: en `sitio.ts`, `const URL_SITIO` (la URL de GitHub Pages, comprobada con `gh api
  repos/MscottB/Tramtai/pages --jq .html_url`), y `cabecera(rel: string, actual: string)` / `pie(rel: string)` como
  **única fuente** de la barra y el pie; `index.html` y `desarrolladores.html` llevan la misma barra y el mismo pie
  (la prueba lo exige).

- [ ] **Step 1: Write the failing tests** en `sitio.test.ts`:
  - Barra y pie de `index.html`, `desarrolladores.html` y una página generada tienen los mismos enlaces: Descargar,
    Empezar (`ayuda/empezar.html`), Agentes, Ayuda, Desarrolladores, GitHub/Releases, Licencia y Términos.
  - Cada página tiene enlace «Saltar al contenido» a `#contenido`, `canonical` absoluto con `URL_SITIO`, `og:url`,
    `og:image` absoluta a `og.png`, `og:type`, `og:site_name`, `og:locale` y `twitter:card`.
  - `sitemap.xml` lista la portada, desarrolladores y los 16 temas.
  - Existe `404.html` con enlaces a la portada, la Ayuda y Descargar.
  - Ninguna regla de `estilo.css` pone `display: block` a `table`.
  - El script del `<head>` aplica el idioma guardado (no solo `sitio.js` al final).
- [ ] **Step 2: Run them to verify they fail** → FAIL.
- [ ] **Step 3: Implement** los puntos de diseño, navegación y calidad técnica del anexo A:
  - menú desplegable accesible entre 641 y 1100 px y botón de descarga compacto en el móvil;
  - Desarrolladores en el cierre de la portada;
  - buscador accesible desde cada tema;
  - `SHA256SUMS.txt` enlazado;
  - borde de `.tablero`/`.codigo` en oscuro;
  - Homebrew en `pre.codigo`;
  - etiquetas unificadas;
  - iconos de tema en SVG;
  - CSS sin duplicados;
  - `.ciclo` en una columna desde 900 px;
  - contraste ≥ 3:1 en bordes;
  - zonas táctiles ≥ 24 px;
  - `aria-label` y `placeholder` en el idioma de la página;
  - títulos reales en vez de `role="heading"`;
  - `width`/`height` en las imágenes de la Ayuda;
  - favicons y logo pequeño;
  - precarga de la fuente y `defer`;
  - JSON-LD `SoftwareApplication`;
  - manifest;
  - claves de `localStorage` separadas para el lenguaje de la guía y de la Ayuda.
- [ ] **Step 4: Run** `npm test`, `npm run sitio` → PASS; navegador en 375, 800 y 1280 px, claro y oscuro, con teclado.
- [ ] **Step 5: Commit** — `feat(sitio): navegación, diseño y metadatos`

### Task 14: Guía de desarrolladores y documentación

**Files:**
- Modify: `site/desarrolladores.html`, `docs/desarrolladores.md`, `docs/api.md`, `docs/operacion.md`,
  `docs/arquitectura.md`, `docs/inicio.md` (si cita la «API principal» o `tramtai global|proyecto`),
  `internal/api/openapi.json`, `docs/equipo/pendientes.md`, `README.md` (si cita la «API principal»)

- [ ] **Step 1: Write**
  - **`desarrolladores`:** qué clave usar (la del espacio o la de una API por aplicación); «instancia» en la API = el
    espacio de trabajo; `desconectado` y `agente_desconectado`; sección «Importar en Postman o generar un cliente»
    con `GET /api/v1/openapi.json`; fila de Antigravity en la tabla de herramientas; la CLI nueva (`tramtai espacios`,
    `tramtai espacio …`, `tramtai agente desconectar|conectar`).
  - **`api.md` y `openapi.json`:** códigos `agente_desconectado`, `confianza_requerida` y `dentro_de_proyecto`;
    `heredar`, `app_con_clave_24h`, `cuenta` y `desconectado`; rutas de administración de espacios y agentes.
  - **`operacion.md` y `arquitectura.md`:** registro `espacios.json`, la marca, la confianza, olvidar y la tabla de
    archivos.
  - **`pendientes.md`:** v0.54 (se marca ✅ al publicar); v0.55 recorridos guiados con driver.js; v0.56–v0.59 lo que
    seguía; «Exportar e importar APIs» sin número, con la nota de la versión Pro de equipo.
- [ ] **Step 2: Run** `go test ./internal/api -run OpenAPI`, `cd web; npm test` → PASS.
- [ ] **Step 3: Commit** — `docs: espacios de trabajo, claves y agentes en la guía y la documentación`

### Task 15: Verificación de punta a punta

- [ ] **Step 1:** `go vet ./...`, `go vet -tags conversor ./...`, `TRAMTAI_IDIOMA=es go test ./...`, `cd web; npm test;
  npm run build; npm run sitio` (sin cambios pendientes en `site/ayuda`), `graphify update .`.
- [ ] **Step 2: Navegador** (preview `v050` con el binario nuevo en `.superpowers/vista`):
  - abrir un proyecto nuevo, que queda confiado, y crear una API en él;
  - abrir uno con `.tramtai/` y un `local.yaml` con `comandos_agentes`: aparece la pregunta de confianza;
  - mover la carpeta y abrirla en su nueva ruta: conserva el historial;
  - detenerlo, olvidarlo y comprobar que la carpeta sigue intacta;
  - dos pestañas;
  - reiniciar el servicio y volver al último espacio;
  - cerrar sesión en un CLI real y ver el aviso en Agentes;
  - desconectar y conectar un agente;
  - la clave del espacio y el aviso suave.
  Expected: todo como en la spec; capturas de prueba en el ledger.
- [ ] **Step 3: Commit** de lo que falte y línea del ledger.
