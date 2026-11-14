# v0.60: tus agentes, preparados por Tramtai — plan

> **Para agentes:** SUB-SKILL OBLIGATORIA: superpowers:subagent-driven-development (recomendada) o superpowers:executing-plans, tarea a tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** Tramtai instala, conecta, actualiza y desinstala los CLI de Claude Code, Codex, GitHub Copilot y
Antigravity desde la app, la Web UI, la terminal y la CLI, también en el modo servidor, y solo hace cada acción
automática con versiones comprobadas.

**Arquitectura:**
- **Los datos:** cada ficha (`internal/agentes/<nombre>/ficha.go`) suma una `adaptadores.Receta`: las órdenes oficiales
  por sistema, dónde queda el ejecutable, el lector del inicio de sesión y las versiones comprobadas.
- **La ejecución:** un paquete nuevo, `internal/preparar`, ejecuta la receta como una tarea por agente con estados (como
  `internal/documentos/preparacion.go`), sobre `procesos.Ejecutar`. No sabe nada de HTTP.
- **Las versiones:** `internal/preparar` también sabe la última versión publicada y si una versión está comprobada
  (receta + lista firmada del release `compatibilidad` + marcas locales).
- **La orquestación:** `internal/servicio`/`internal/api` exponen `/api/v1/agentes/{id}/preparacion` y
  `/api/v1/agentes/versiones`; la Web UI, la terminal y la CLI las usan con un sondeo corto, como el conversor.

**Tecnología:** Go 1.27 (cobra, Bubble Tea), Vue/Quasar, GitHub Actions; ed25519 de `internal/actualizacion`. Posible
dependencia nueva para la pseudoterminal (solo si el M0 la pide; ver Restricciones).

**Spec:** `docs/equipo/releases/v0.60-preparar-agentes/spec.md`. Se lee junto con este plan; sus secciones se citan como
«spec › …».

## Restricciones globales

- **Ramas y PR:** cada tarea va en su rama `v0.60/tNN-<tema>` y su PR con Auto-fix, apilado sobre la anterior. Nunca push
  a `main`. Se fusiona y se publica solo con autorización del usuario. Los commits terminan con
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. `anti-slop/` nunca se sube. Al fusionar PR apilados, cambiar
  antes su base a `main` y borrar las ramas al final (borrar la base de un PR apilado lo cierra).
- **Dependencias nuevas:** ninguna, salvo la pseudoterminal si el M0 demuestra que un CLI no inicia sesión sin ella: en
  Unix `github.com/creack/pty`; en Windows, ConPTY con `golang.org/x/sys/windows`. Se pregunta al usuario antes de
  añadirla.
- **Agentes (id de ficha):** `claude` (Claude Code), `codex` (Codex), `copilot` (GitHub Copilot), `antigravity`
  (Antigravity, solo instalar/actualizar/desinstalar; sin inicio de sesión). `gemini` no tiene receta.
- **Acciones:** `instalar`, `actualizar`, `iniciar_sesion`, `desinstalar`.
- **Estados de una preparación:** `instalando`, `actualizando`, `desinstalando`, `comprobando`, `esperando_usuario`,
  `listo`, `error`, `cancelado`.
- **Códigos de error:** `requisito_falta`, `version_no_comprobada`, `peticiones_en_curso`, `ya_en_curso`,
  `instalador_fallo`, `no_encontrado_tras_instalar`, `plazo_agotado`, `salida_no_reconocida`, `sin_red`,
  `no_disponible_en_servidor`, `interrumpida` (el servicio se reinició a mitad).
- **Plazos:** instalar, actualizar y desinstalar 15 min; iniciar sesión 15 min; sin enlace ni código del lector en 30 s
  → `salida_no_reconocida`; consulta de última versión 10 s por CLI.
- **Rutas:**
  - `GET|POST|DELETE /api/v1/agentes/{id}/preparacion`, `POST /api/v1/agentes/{id}/preparacion/codigo`;
  - `GET /api/v1/agentes/versiones`.
- **Archivos de Tramtai** (0600, con `configuracion.EscribirAtomico`):
  - `datos/agentes/compatibilidad.json`: lo comprobado que se bajó y las marcas locales,
    `{"lista":{"version":3,"agentes":{"claude":{"iniciar_sesion":["2.1.288"]}}},"no_compatibles":{"claude":["2.2.0"]},"leida":"2026-11-20T10:00:00Z"}`;
  - `datos/agentes/preparaciones/<id>.json` (el estado, para recuperarlo tras un reinicio) y `<id>.log` (el registro
    corto de la última preparación).
- **Lista firmada:** archivos `compatibilidad.json` y `compatibilidad.json.sig` del release fijo de GitHub
  `compatibilidad`, firmados con la clave de `internal/actualizacion/firma.go` (`VerificarFirma`).
- **Ajuste:** `servicio.buscar_versiones_agentes` (booleano, `true` por defecto), junto a
  `servicio.buscar_actualizaciones`.
- **CLI:** `tramtai agentes instalar|install <id>…`, `entrar|login <id>`, `actualizar|update <id>…|--todos (--all)`,
  `desinstalar|uninstall <id> [--borrar-datos (--delete-data)]`, `versiones|versions`.
- **Textos:**
  - todo en español y en inglés;
  - cada `idioma.T` nuevo lleva su inglés en `internal/idioma/en_*.go` (`TestCobertura`);
  - cada clave nueva de `web/src/idiomas/es.ts` lleva la suya en `en.ts`;
  - la terminal los recibe con `npm run ayuda-terminal`;
  - el glosario y `estilo-textos.test.ts` mandan.
- **Pruebas con programas reales:** nunca con el Tramtai instalado del usuario ni su puerto 17373; copias en `bin/m0-*`,
  puerto 17388; carpetas de configuración temporales (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `COPILOT_HOME`, `HOME`,
  `USERPROFILE`…) para no tocar las sesiones reales; avisar al usuario antes de cada inicio de sesión real.
- **Cada versión pone al día:** Ayuda, guía de desarrolladores (`docs/equipo/desarrollo.md`), README y sitio. Los Minor
  de las revisiones se arreglan dentro de la versión.

## Revisión: lo que más puede fallar

1. **El servicio no ve el CLI recién instalado** porque su PATH es de antes. Lo esperado: se encuentra por la ruta de la
   receta o por el PATH de usuario del registro (Windows) y se guarda en `comandos_agentes`. Prueba en la Tarea 3.
2. **Un instalador se queda esperando una respuesta** (pregunta interactiva). Lo esperado: la entrada estándar va cerrada
   y, si aun así no termina, `plazo_agotado` con el árbol de procesos cerrado. Prueba en la Tarea 3.
3. **Dos pantallas lanzan la misma preparación a la vez** (la app y la Web UI remota). Lo esperado: la segunda recibe
   `ya_en_curso` y ve el estado de la primera. Prueba en la Tarea 3.
4. **La salida del inicio de sesión llega partida o con secuencias ANSI.** Lo esperado: el lector trabaja sobre el texto
   acumulado y sin ANSI, y encuentra el enlace y el código igual. Prueba en la Tarea 4.
5. **El servicio se reinicia a mitad de una preparación.** Lo esperado: el proceso hijo no queda huérfano
   (`procesos.ConAlIniciar`/`TerminarHuerfano`, como las peticiones) y la preparación aparece como `error` con
   `interrumpida`. Prueba en la Tarea 3.

---

### Tarea 1: M0 de los CLI con los programas reales

Antes de construir, fijar las órdenes y salidas reales de cada CLI en cada sistema.

**Archivos:**
- Crear: `docs/equipo/releases/v0.60-preparar-agentes/m0.md` (la evidencia),
  `.github/workflows/agentes-compatibilidad.yml` (solo `workflow_dispatch` por ahora; la Tarea 13 lo completa),
  `scripts/agentes/probar-instalacion.sh` y `scripts/agentes/probar-instalacion.ps1` (instalan un CLI en una carpeta de
  usuario temporal e imprimen dónde quedó y su `--version`)
- Crear: `internal/agentes/<id>/testdata/sesion-*.txt`, la salida real de cada inicio de sesión, sin datos personales

**Interfaces:** nada nuevo para otras tareas; fija los valores de las recetas de la Tarea 2.

- [ ] **Paso 1:** avisar al usuario de que habrá inicios de sesión reales (aprobará en su navegador dos o tres veces),
  siempre con carpetas de configuración temporales.
- [ ] **Paso 2:** en este equipo (Windows), para cada CLI: la orden oficial de instalación que no necesita Node (si la
  hay), dónde deja el ejecutable, si se puede fijar la versión, cómo se actualiza y se desinstala, de dónde se lee la
  última versión publicada y si el fabricante publica sumas o firmas.
- [ ] **Paso 3:** con el workflow manual en macOS, Linux y Windows Server (runners de GitHub), lo mismo con los scripts
  `probar-instalacion.*`. En Windows Server, además: con una cuenta virtual de servicio de prueba (como
  `TestInstalarServicioWindows`), ¿se puede instalar para ella y lanzar su inicio de sesión? Anotar qué sí y qué no.
- [ ] **Paso 4:** inicio de sesión de Claude Code, Codex y Copilot en este equipo, con carpetas temporales: ¿necesita pty?
  ¿qué enlace y código imprime, en qué líneas? ¿pide pegar algo? ¿cómo termina? Guardar cada salida en
  `testdata/sesion-*.txt` con los correos, nombres y códigos reales sustituidos por valores de ejemplo del mismo formato.
- [ ] **Paso 5:** qué ajuste imprescindible hace falta, si alguno (spec › Ajustes imprescindibles), y si algún CLI necesita
  Node (spec › Requisitos).
- [ ] **Paso 6:** escribir `m0.md` con una tabla por CLI y sistema y, al final, las decisiones que fija: recetas, si hace
  falta pty (y entonces preguntar al usuario por la dependencia), si se construye «Requisitos» y qué se puede en
  Windows Server.
- [ ] **Paso 7:** commit `test(v0.60): M0 de la instalación y el inicio de sesión de los CLI`.

### Tarea 2: las recetas en las fichas

**Archivos:**
- Modificar: `internal/adaptadores/ficha.go` (tipos de la receta y campo `Receta *Receta` en `Ficha`)
- Modificar: `internal/agentes/{claude,codex,copilot,antigravity}/ficha.go`
- Crear: `internal/agentes/recetas_test.go`

**Interfaces:**
- Produce (en `internal/adaptadores`):
  ```go
  type OrdenReceta struct{ Programa string; Argumentos []string } // fijas; "{version}" se sustituye al fijar versión
  type FuenteVersion struct{ Tipo, Nombre string }                // Tipo: "npm" | "github" | "url"
  type DescargaVerificada struct{ URL, URLSumas string }          // "{version}" en las dos; sumas SHA-256 del fabricante
  type LectorSesion struct {
      Argumentos []string // los del inicio de sesión (Ficha.InicioSesion se rellena desde aquí)
      PTY        bool
      Enlace, Codigo, PidePegar *regexp.Regexp
  }
  type Receta struct {
      Instalar, Actualizar, Desinstalar map[string]OrdenReceta // por runtime.GOOS; Actualizar vacío: reinstalar
      Requisito   map[string]string   // por GOOS: "" | "npm" | "winget" | "brew"
      Rutas       map[string][]string // por GOOS: dónde puede quedar el ejecutable ("~" = casa del usuario)
      DatosCLI    []string            // carpetas de sesión y ajustes del CLI ("~/.claude"…), para «Borrar también»
      FijaVersion bool                // Instalar admite "{version}"
      Descarga    map[string]DescargaVerificada // por GOOS, solo si Tramtai baja el binario él mismo (no un instalador)
      Ultima      FuenteVersion
      Sesion      *LectorSesion       // nil: sin inicio de sesión (antigravity)
      Comprobadas map[string][]string // por acción: versiones exactas o con comodín de parche ("2.1.*")
  }
  ```
- Consume: los valores de `m0.md`.

- [ ] **Paso 1: pruebas que fallan** en `recetas_test.go`:
  - `TestRecetasCompletas`: `claude`, `codex`, `copilot` y `antigravity` tienen `Receta` con `Instalar` y `Rutas` para
    `windows`, `darwin` y `linux` (donde el M0 dijo que no hay, la entrada falta y un comentario lo dice); `gemini` no
    tiene.
  - `TestRecetasSinSesionAntigravity`: la receta de `antigravity` tiene `Sesion == nil`; las otras tres no.
  - `TestLectoresConSalidaReal`: cada `LectorSesion` encuentra en `testdata/sesion-*.txt` el enlace y el código de
    ejemplo que puso la Tarea 1.
  - `TestComprobadasTienenLaDelM0`: cada receta incluye en `Comprobadas` la versión probada en el M0 para cada acción.
- [ ] **Paso 2:** `go test ./internal/agentes/...` falla.
- [ ] **Paso 3:** implementar los tipos y las cuatro recetas con los valores de `m0.md`. `Ficha.InicioSesion` se
  mantiene, rellenado desde `Receta.Sesion.Argumentos`, para no romper `servidor.OrdenEntrar`.
- [ ] **Paso 4:** `go test ./internal/agentes/... ./internal/adaptadores/...` pasa.
- [ ] **Paso 5:** commit `feat(agentes): recetas de instalación e inicio de sesión de cada CLI`.

### Tarea 3: `internal/preparar`, instalar, actualizar y desinstalar

**Archivos:**
- Crear: `internal/preparar/preparar.go`, `internal/preparar/buscar.go`, `internal/preparar/buscar_windows.go`,
  `internal/preparar/buscar_otros.go`, `internal/preparar/preparar_test.go`
- Modificar: `internal/contrato/contrato.go` (tipos de abajo) y `go generate ./internal/contrato`

**Interfaces:**
- Produce:
  ```go
  // contrato
  type PedidoPreparacion struct {
      Accion      string `json:"accion"` // "instalar" | "actualizar" | "iniciar_sesion" | "desinstalar"
      BorrarDatos bool   `json:"borrar_datos"`
      Confirmar   bool   `json:"confirmar"` // seguir con una versión no comprobada (Tarea 5)
  }
  type EstadoPreparacion struct {
      Agente, Accion, Estado, Etapa string
      Enlace, Codigo string; PidePegar bool
      MotivoCodigo, Mensaje string
      Lineas []string // las 20 últimas, redactadas
      VersionAntes, VersionDespues string
      Inicio, Fin *time.Time
  }
  // preparar
  type Opciones struct {
      Datos     string
      Ficha     func(id string) (adaptadores.Ficha, bool)
      EnCurso   func(agente string) int             // peticiones en curso del agente
      Guardar   func(agente, ruta string) error     // fija o quita ("") comandos_agentes.<agente>
      Comprobar func(ctx context.Context, agente string) adaptadores.SesionCLI
  }
  type Preparador struct{ /* privado */ }
  func Nuevo(o Opciones) *Preparador
  func (p *Preparador) Iniciar(agente string, pedido contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error)
  func (p *Preparador) Estado(agente string) (contrato.EstadoPreparacion, bool)
  func (p *Preparador) Cancelar(agente string) error
  func (p *Preparador) Recuperar() // al arrancar: termina huérfanos y deja `error`/`interrumpida`
  type Error struct{ Codigo, Mensaje string } // con los códigos de las Restricciones
  func (e *Error) Error() string
  ```
- Consume: `procesos.Ejecutar`, `procesos.ConAlIniciar`, `procesos.TerminarHuerfano`, `adaptadores.VersionDe`,
  `adaptadores.EntornoLimpio`, `adaptadores.RedactarCon`, `configuracion.EscribirAtomico`.

- [ ] **Paso 1: pruebas que fallan** en `preparar_test.go`, con CLI falsos (programas Go compilados en `TestMain`, como
  las pruebas de `internal/adaptadores`) y una ficha de prueba:
  - `TestInstalarYEncontrar` (Revisión 1): el instalador falso deja el ejecutable en una ruta de `Rutas` fuera del PATH;
    el estado termina `listo` (o sigue con `iniciar_sesion` si hay `Sesion`), `Guardar` recibe esa ruta y
    `VersionDespues` es la del falso.
  - `TestInstaladorFalla`: código de salida 1 → `error` con `instalador_fallo` y sus últimas líneas en `Lineas`.
  - `TestNoEncontradoTrasInstalar` → `no_encontrado_tras_instalar`.
  - `TestInstaladorEsperaEntrada` (Revisión 2): un falso que lee de stdin termina (stdin cerrado); uno que se queda en un
    bucle → `plazo_agotado` con un plazo de prueba de 2 s y el proceso cerrado.
  - `TestYaEnCurso` (Revisión 3): dos `Iniciar` seguidos del mismo agente → el segundo devuelve `ya_en_curso` y el estado
    del primero.
  - `TestNoActualizaConPeticiones`: `EnCurso` devuelve 1 → `peticiones_en_curso`, sin lanzar nada (también al
    desinstalar).
  - `TestDesinstalarNoBorraDatos`: sin `BorrarDatos`, las carpetas `DatosCLI` del falso siguen; con él, desaparecen; y
    `Guardar(agente, "")` si la ruta la puso Tramtai.
  - `TestRequisitoFalta`: receta con `Requisito[runtime.GOOS]="npm"` y sin npm en el PATH → `requisito_falta`.
  - `TestRecuperarTrasCaida` (Revisión 5): un estado guardado «en marcha» con el PID y la firma de un proceso vivo →
    `Recuperar` lo termina y deja `error`/`interrumpida`.
  - `TestRegistroRedactado`: una línea con forma de clave (`sk-ant-…`) no aparece en `<id>.log` ni en `Lineas`.
  - `TestDescargaSumaNoCoincide`: con `Receta.Descarga` y un servidor de prueba cuyas sumas no casan con el binario →
    `error` con `instalador_fallo`, sin dejar el binario en `Rutas`.
- [ ] **Paso 2:** `go test ./internal/preparar/` falla.
- [ ] **Paso 3:** implementar.
  - Una goroutine por preparación con su `context` y su plazo; el estado en memoria y en
    `datos/agentes/preparaciones/<id>.json` (con PID y firma, para `Recuperar`).
  - Stdin cerrado salvo en el inicio de sesión.
  - `buscar`: PATH del servicio, `Receta.Rutas` (expandiendo `~`) y, en Windows (`buscar_windows.go`), el `Path` de
    `HKCU\Environment` leído con `golang.org/x/sys/windows/registry`.
  - Ajustes imprescindibles: si el M0 encontró alguno, se aplican aquí con `internal/apuntar` (con su copia) y se quitan
    al desinstalar; si no, nada.
- [ ] **Paso 4:** `go test ./internal/preparar/` pasa, también con `-race`.
- [ ] **Paso 5:** commit `feat(preparar): instalar, actualizar y desinstalar los CLI con estados`.

### Tarea 4: el inicio de sesión

**Archivos:**
- Crear: `internal/preparar/sesion.go`, `internal/preparar/sesion_test.go` y, solo si el M0 pidió pty,
  `internal/preparar/pty_unix.go` y `internal/preparar/pty_windows.go`

**Interfaces:**
- Consume: `adaptadores.LectorSesion`, `Opciones.Comprobar`, `(*Preparador).Marcar` (Tarea 5; aquí, un stub que la Tarea 5
  completa).
- Produce: `func (p *Preparador) Codigo(agente, codigo string) error` y el estado `esperando_usuario` con `Enlace`,
  `Codigo` y `PidePegar`.

- [ ] **Paso 1: pruebas que fallan** en `sesion_test.go`, con un CLI falso que imprime la salida de
  `testdata/sesion-*.txt`:
  - `TestSesionCodigoDispositivo`: estado `esperando_usuario` con el enlace y el código de ejemplo; cuando el falso
    termina con 0 y `Comprobar` devuelve `SesionOK` → `listo`.
  - `TestSesionPegarCodigo`: con `PidePegar`, `Codigo(agente, "abc-123")` llega a la entrada del falso, que lo escribe en
    un archivo de la prueba; ni el estado ni el registro contienen `abc-123`.
  - `TestSesionSalidaPartidaYANSI` (Revisión 4): el falso escribe el enlace en tres trozos con `\x1b[1m…\x1b[0m` → se
    encuentra igual.
  - `TestSesionNoReconocida`: el falso imprime otra cosa y espera → `error` con `salida_no_reconocida` (plazo de prueba
    2 s), con `Lineas`, el proceso cerrado y `Marcar` llamado con su versión.
  - `TestSesionCancelar`: `Cancelar` → `cancelado` y el proceso cerrado.
- [ ] **Paso 2:** `go test ./internal/preparar/ -run Sesion` falla.
- [ ] **Paso 3:** implementar: el lector quita ANSI y busca sobre el texto acumulado; con `PTY`, la pseudoterminal de la
  dependencia aprobada; sin ella, tuberías.
- [ ] **Paso 4:** `go test ./internal/preparar/` pasa, también con `-race`.
- [ ] **Paso 5:** commit `feat(preparar): inicio de sesión del propio CLI con enlace, código y pegar`.

### Tarea 5: versiones nuevas y versiones comprobadas

**Archivos:**
- Crear: `internal/preparar/versiones.go`, `internal/preparar/compatibilidad.go`,
  `internal/preparar/versiones_test.go`
- Modificar: `internal/contrato/contrato.go` (`VersionesAgente`), `internal/configuracion/configuracion.go`
  (`servicio.buscar_versiones_agentes`)

**Interfaces:**
- Produce:
  ```go
  // contrato
  type VersionesAgente struct {
      Agente, Instalada, Ultima string
      Comprobadas  map[string]bool // por acción, para la versión que se usaría
      Acciones     []string        // las habilitadas ahora
      MotivoCodigo, Motivo string  // por qué falta alguna
  }
  // preparar
  func (p *Preparador) Versiones(ctx context.Context, forzar bool) []contrato.VersionesAgente
  func (p *Preparador) Comprobada(agente, accion, version string) bool
  func (p *Preparador) Marcar(agente, version string) error // no compatible en este equipo
  func (p *Preparador) LeerLista(ctx context.Context) error  // baja y verifica la lista firmada
  var URLLista = "https://github.com/MscottB/Tramtai/releases/download/compatibilidad/compatibilidad.json"
  ```
- Consume: `actualizacion.VerificarFirma(datos, firma []byte) error`.

- [ ] **Paso 1: pruebas que fallan** en `versiones_test.go`, con un `httptest.Server` para npm, GitHub y la lista:
  - `TestUltimaVersion`: cada `FuenteVersion` (`npm`, `github`, `url`) da la versión esperada; sin red → `Ultima` vacía
    y `MotivoCodigo` `sin_red`, sin error.
  - `TestComprobadaComodin`: `"2.1.*"` cubre `2.1.300` y no `2.2.0`.
  - `TestListaFirmada`: una lista con firma válida añade versiones; con firma inválida se ignora y se conserva la
    anterior.
  - `TestMarcaLocal`: tras `Marcar("claude","2.1.288")`, `Comprobada` es falsa aunque la receta la tenga, hasta que una
    lista firmada con `version` mayor la vuelva a traer.
  - `TestIniciarConVersionNoComprobada`: `Iniciar` con `iniciar_sesion` y una versión no comprobada →
    `version_no_comprobada` sin lanzar nada, también con `Confirmar` (no se inicia sesión sin comprobar).
  - `TestInstalarSinFijarVersionPideConfirmar`: `instalar` con `FijaVersion=false` y `Ultima` no comprobada → sin
    `Confirmar`, `version_no_comprobada`; con `Confirmar`, instala y el inicio de sesión queda desactivado.
  - `TestSinBusqueda`: con `servicio.buscar_versiones_agentes=false`, `Versiones` no hace ninguna petición de red.
- [ ] **Paso 2:** `go test ./internal/preparar/ -run Version` falla.
- [ ] **Paso 3:** implementar. Consulta como mucho una vez al día (campo `leida` de `compatibilidad.json`) salvo con
  `forzar`; 10 s por CLI; solo la cabecera `User-Agent: Tramtai/<versión>`. `Iniciar` (Tarea 3) consulta `Comprobada`
  antes de lanzar. Al instalar con `FijaVersion`, se elige la versión comprobada más reciente.
- [ ] **Paso 4:** `go test ./internal/preparar/ ./internal/configuracion/` pasa.
- [ ] **Paso 5:** commit `feat(preparar): versiones nuevas y versiones comprobadas con lista firmada`.

### Tarea 6: requisitos (Node), solo si el M0 lo pide

Si `m0.md` concluye que ningún CLI necesita Node, esta tarea es solo anotarlo en `m0.md` y cerrarla con ese commit.

**Archivos:**
- Crear: `internal/preparar/requisitos.go`, `internal/preparar/requisitos_test.go`
- Modificar: las recetas de la Tarea 2 que pidan npm

**Interfaces:**
- Produce: dentro de `Iniciar` con `instalar`, si falta npm: en Windows instala Node LTS con winget por usuario (orden
  exacta del M0, p. ej. `winget install --id OpenJS.NodeJS.LTS -e --scope user`) antes del CLI; en macOS
  `brew install node` si hay brew; en Linux, `requisito_falta` con la orden de la distribución en `Mensaje`.

- [ ] **Paso 1: pruebas que fallan:** `TestRequisitoWindowsConWinget` (winget falso → instala y sigue con el CLI),
  `TestRequisitoLinuxSoloOrden` (nunca ejecuta nada; `requisito_falta` con la orden).
- [ ] **Paso 2:** fallan. **Paso 3:** implementar. **Paso 4:** `go test ./internal/preparar/` pasa.
- [ ] **Paso 5:** commit `feat(preparar): Node para los CLI que lo necesitan`.

### Tarea 7: el servicio y la API

**Archivos:**
- Modificar: `internal/api/api.go` (rutas); crear `internal/api/preparacion.go` y `internal/api/preparacion_test.go`
- Modificar: `internal/servicio/servicio.go` (crear el `Preparador` con sus `Opciones`, `Recuperar()` al arrancar,
  `LeerLista` con la búsqueda diaria), `internal/cliente/` (métodos de abajo)

**Interfaces:**
- Consume: `preparar.Preparador` (Tareas 3–5).
- Produce:
  - Las rutas de las Restricciones. `POST …/preparacion` recibe `contrato.PedidoPreparacion`. Respuestas: 202 con
    `EstadoPreparacion`; 409 con `{"error":{"codigo":"ya_en_curso"|"peticiones_en_curso"|"version_no_comprobada",…}}`;
    422 con `requisito_falta`.
  - En `internal/cliente`:
    `Preparacion(id string) (contrato.EstadoPreparacion, error)`,
    `Preparar(id string, p contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error)`,
    `CodigoPreparacion(id, codigo string) error`, `CancelarPreparacion(id string) error`,
    `VersionesAgentes(forzar bool) ([]contrato.VersionesAgente, error)`.

- [ ] **Paso 1: pruebas que fallan** en `preparacion_test.go`:
  - `TestPrepararRutas`: el ciclo instalar → `GET` → `listo` con un `Preparador` de prueba.
  - `TestPrepararCodigoNoSeDevuelve`: tras `POST …/codigo`, ningún `GET` devuelve lo pegado.
  - `TestPrepararSoloAdministradorEnServidor`: en modo servidor, sin sesión de administrador → 401/403; con ella, 202.
  - `TestPrepararCSRF`: con sesión de la Web UI y sin `X-CSRF-Token` → 403.
  - `TestPeticionesEnCursoDelAgente`: `EnCurso` cuenta las peticiones `en_cola`/`ejecutando` de ese agente en todas las
    instancias.
- [ ] **Paso 2:** fallan. **Paso 3:** implementar, como las rutas de `/api/v1/conversor`; comprobar que el freno de
  peticiones de la Web UI cubre estas rutas.
- [ ] **Paso 4:** `go test ./internal/api/... ./internal/servicio/... ./internal/cliente/...` pasa y
  `go generate ./internal/contrato` no deja cambios.
- [ ] **Paso 5:** commit `feat(api): preparar agentes desde /api/v1/agentes/{id}/preparacion`.

### Tarea 8: la Web UI, página Agentes y Ajustes

**Archivos:**
- Crear: `web/src/componentes/PanelPreparacion.vue`, `web/src/preparacion.ts` (sondeo como `seguirPreparacion` de
  `web/src/conversor.ts`), `web/src/preparacion.test.ts`
- Modificar: `web/src/paginas/Agentes.vue`, `web/src/paginas/Configuracion.vue` (el ajuste), `web/src/idiomas/es.ts`,
  `en.ts`, `internal/recursos/web` (compilado)

**Interfaces:**
- Consume: las rutas de la Tarea 7 y los tipos de `web/src/tipos.gen.ts`.
- Produce: `<PanelPreparacion :agentes="['claude','codex']" @listo="…" />`, que la Tarea 9 reutiliza.

- [ ] **Paso 1: pruebas que fallan** en `preparacion.test.ts`:
  - la acción principal de cada tarjeta según su estado y `VersionesAgente` (spec › Pantallas › Agentes), incluida la
    versión no comprobada (aviso y sin botón);
  - el panel enseña enlace, código y campo para pegar con `esperando_usuario`, y el enlace se abre con
    `rel="noopener noreferrer"`;
  - el sondeo para con `listo`, `error` o `cancelado`.
- [ ] **Paso 2:** `cd web; npm test` falla.
- [ ] **Paso 3:** implementar. «…» de cada tarjeta: «Comprobar versión», «Actualizar», «Desinstalar» (diálogo con la
  casilla «Borrar también su sesión y ajustes») y «Ver registro». Antes de instalar o actualizar una versión no
  comprobada, el aviso de la spec y «Seguir» (`confirmar: true`). En modo servidor, el panel dice «Para la cuenta del
  servicio (<cuenta>)».
- [ ] **Paso 4:** `npm test`, `npm run build` y `npm run sitio` pasan; compilado commiteado.
- [ ] **Paso 5:** commit `feat(web): instalar, conectar y mantener los agentes desde Agentes`.

### Tarea 9: la bienvenida

**Archivos:**
- Modificar: el componente de bienvenida de la Web UI (la clave `tramtai.bienvenida`; búscalo en `web/src`), `es.ts`,
  `en.ts`, el compilado

**Interfaces:** consume `PanelPreparacion` (Tarea 8).

- [ ] **Paso 1: prueba que falla:** sin ningún agente listo, la bienvenida ofrece «Prepara tus agentes» con Claude Code y
  Codex marcados; con uno listo, no.
- [ ] **Paso 2:** falla. **Paso 3:** implementar: instala uno tras otro en un solo panel y termina con «Listo: ya puedes
  usar …». **Paso 4:** `npm test` y `npm run build` pasan.
- [ ] **Paso 5:** commit `feat(web): la bienvenida prepara tus agentes`.

### Tarea 10: la terminal

**Archivos:**
- Modificar: `internal/tui/paginas/agentes/pagina.go` y su `_test.go`, los textos de la terminal (con
  `npm run ayuda-terminal`) y su Ayuda de teclas

**Interfaces:** consume los métodos de `cliente` de la Tarea 7.

- [ ] **Paso 1: pruebas que fallan** (con el servicio de prueba de `internal/tui/paginas/prueba`): `i` lanza instalar,
  `s` iniciar sesión, `u` actualizar, `x` pide confirmar y desinstala; con `esperando_usuario` se ven el enlace y el
  código y un campo para pegar que envía `CodigoPreparacion`.
- [ ] **Paso 2:** fallan. **Paso 3:** implementar. **Paso 4:** `go test ./internal/tui/...` pasa.
- [ ] **Paso 5:** commit `feat(terminal): preparar agentes desde la página Agentes`.

### Tarea 11: la CLI

**Archivos:**
- Modificar: `cmd/tramtai/agentes.go` y su `_test.go`; `entrar` deja de ser solo del servidor

**Interfaces:** consume los métodos de `cliente` de la Tarea 7.

- [ ] **Paso 1: pruebas que fallan** en `agentes_test.go`, con el servicio de prueba de `cmd/tramtai`:
  - `TestAgentesInstalar`: imprime el progreso y termina con 0 en `listo`, con 1 en `error` y el motivo.
  - `TestAgentesEntrarPegaCodigo`: con `PidePegar`, lee el código de stdin y lo envía.
  - `TestAgentesAlias`: existen `install`, `login`, `update`, `uninstall`, `versions`, `--all` y `--delete-data`.
  - `TestAgentesVersiones`: tabla con instalada, última y comprobada.
- [ ] **Paso 2:** fallan. **Paso 3:** implementar. **Paso 4:** `go test ./cmd/tramtai/` pasa.
- [ ] **Paso 5:** commit `feat(cli): tramtai agentes instalar, entrar, actualizar, desinstalar y versiones`.

### Tarea 12: el modo servidor (Linux y Windows Server)

**Archivos:**
- Modificar: `internal/servidor/linux.go`, `internal/servidor/windows.go` y sus pruebas, `docs/servidor.md`, y lo que
  diga el M0 para Windows Server

**Interfaces:** consume `preparar` tal cual: el servicio ya corre con la cuenta del servicio.

- [ ] **Paso 1: pruebas que fallan:**
  - Linux: la preparación del servicio usa `HOME` = la carpeta de datos de la cuenta y la receta instala «por usuario»
    para ella (con la cuenta de prueba de `internal/servidor`, en el job «Servidor (Linux)» del CI).
  - Windows Server: lo que el M0 dijo que se puede; lo que no, `no_disponible_en_servidor` con su texto (es/en).
- [ ] **Paso 2:** fallan. **Paso 3:** implementar; `OrdenEntrar` se queda para la orden manual de la guía.
- [ ] **Paso 4:** pasan en los jobs «Servidor (Linux)» y «Servidor (Windows Server)» del CI.
- [ ] **Paso 5:** `docs/servidor.md`: fuera `sudo npm install -g` y `sudo -u …` donde Tramtai ya lo hace, y corregir el
  «con su CODEX_HOME privado» que no coincide con el código. Commit
  `feat(servidor): preparar agentes para la cuenta del servicio`.

### Tarea 13: el job de compatibilidad y la lista firmada

**Archivos:**
- Modificar: `.github/workflows/agentes-compatibilidad.yml` (de la Tarea 1): `schedule` semanal y `workflow_dispatch`
- Crear: `cmd/compatibilidad/main.go` (instala la última versión de cada CLI con su receta, lanza el inicio de sesión
  hasta encontrar enlace y código con el lector, lo cancela y escribe `compatibilidad.json`) y su `_test.go`

**Interfaces:**
- Consume: las recetas (Tarea 2), el lector (Tarea 4), `scripts/firmar` con el secreto `FIRMA_ACTUALIZACIONES`.
- Produce: `compatibilidad.json` + `.sig` en el release `compatibilidad` (`gh release upload compatibilidad … --clobber`),
  con el formato que lee `LeerLista` (Tarea 5): `{"version":N,"agentes":{"<id>":{"<accion>":["<versión>",…]}}}`, con `N`
  creciente.

- [ ] **Paso 1: prueba que falla** en `cmd/compatibilidad`: con CLI falsos, una versión que pasa se añade y `version`
  sube; una que no pasa no se añade y el programa termina con 2 (el workflow abre entonces un issue con
  `gh issue create`).
- [ ] **Paso 2:** falla. **Paso 3:** implementar el programa y el workflow: matriz windows/macos/ubuntu; un job final
  junta, firma y sube; permisos mínimos (`contents: write` solo en ese job, `issues: write` para el aviso).
- [ ] **Paso 4:** `go test ./cmd/compatibilidad/` pasa; lanzar el workflow a mano una vez y comprobar que el release
  `compatibilidad` tiene los dos archivos y que `LeerLista` los acepta.
- [ ] **Paso 5:** commit `ci: lista semanal de versiones comprobadas de los CLI, firmada`.

### Tarea 14: Ayuda, guías, README y sitio

**Archivos:**
- Crear: el tema de Ayuda «Preparar tus agentes» en `web/src/ayuda/temas/` (es/en), con su `recorrido`
- Modificar: los temas «Primeros pasos», «Agentes» y «Modo servidor»; `docs/inicio.md`; README;
  `docs/equipo/desarrollo.md` (`internal/preparar`, recetas, lista firmada y el job); `docs/equipo/nuevo-agente.md` (la
  receta es parte de un agente nuevo); `site/index.html` (una línea de ventajas: «Tramtai instala y conecta tus
  agentes»); capturas de Agentes y de la bienvenida en es/en × claro/oscuro (`npm run capturas`, con las reglas de la
  v0.59)

- [ ] **Paso 1:** escribir; pasan `npm test` (estilo de textos), `npm run sitio` sin cambios pendientes y
  `go test ./cmd/tramtai -run TestComandosDelSitioExisten` (las órdenes nuevas en es y en).
- [ ] **Paso 2:** capturas: avisar al usuario de las peticiones reales; copia en `bin/capturas-*`, puerto 17388.
- [ ] **Paso 3:** commit `docs(v0.60): Ayuda, guías, README y sitio`.

### Tarea 15: cierre

**Archivos:** `m0.md` (sección «Prueba final»), `docs/equipo/pendientes.md`, las notas de la versión

- [ ] **Paso 1: prueba final con los programas reales** (avisar antes): en una copia de prueba con carpetas temporales,
  instalar, iniciar sesión, actualizar y desinstalar Claude Code y Codex desde la Web UI; Copilot y Antigravity al menos
  instalar; comprobar que las sesiones reales del usuario no cambian (huella de sus archivos antes y después).
- [ ] **Paso 2:** auditoría de diseño de la página Agentes y la bienvenida (impeccable, antislop-ui, teclado, contraste,
  375/1440 px, claro/oscuro, es/en).
- [ ] **Paso 3:** revisión de la rama entera (`superpowers:requesting-code-review`), con foco en «Revisión: lo que más
  puede fallar» y en spec › Seguridad. Los Minor se arreglan aquí.
- [ ] **Paso 4:** `go vet ./...`, `go test ./...`, `cd web; npm test; npm run build; npm run sitio` pasan y
  `npm run sitio` no deja cambios.
- [ ] **Paso 5:** `pendientes.md` con la v0.60 lista para publicar; notas de la versión en es/en enseñadas al usuario
  antes de publicar. Commit `docs(v0.60): cierre`.
