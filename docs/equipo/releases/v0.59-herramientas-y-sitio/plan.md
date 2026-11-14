# v0.59: tus herramientas apuntadas a Tramtai y sitio nuevo — plan

> **Para agentes:** SUB-SKILL OBLIGATORIA: superpowers:subagent-driven-development (recomendada) o superpowers:executing-plans, tarea a tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:**
- Claude Code, Codex, Continue y opencode/Kilo se activan y desactivan contra Tramtai (una a una o todas), cada una con su
  API, con copia de su configuración y, en Claude Code y Codex, sin clave en disco.
- El sitio se renueva: portada en diez secciones, la terminal como texto real, el servidor presentado, las ventajas con
  cifras y capturas y comandos en el idioma elegido.

**Arquitectura:**
- **Los archivos de cada herramienta:** un paquete nuevo, `internal/apuntar`, sabe leerlos y escribirlos (un escritor por
  formato), anota lo que cambia y guarda la copia. No sabe nada del servicio.
- **La orquestación:** `internal/servicio` crea la API, guarda la clave en el almacén (`internal/credenciales`), llama a
  `apuntar` y expone `/api/v1/herramientas` a la Web UI, la terminal y la CLI.
- **La clave en Claude Code y Codex:** `tramtai herramientas clave <id>` la lee del almacén sin pasar por el servicio.
- **El sitio:** sigue escrito a mano (`site/index.html`, `servidor.html`). `web/scripts/sitio-ayuda.mjs` rellena las
  zonas comunes y genera la Ayuda, y `web/scripts/capturas.mjs` saca las capturas y, ahora, la terminal en HTML.

**Tecnología:** Go 1.27 (cobra, Bubble Tea), `go.yaml.in/yaml/v3`, nuevas `github.com/tailscale/hujson` y
`github.com/pelletier/go-toml/v2`, Vue/Quasar, Node para los scripts del sitio y GitHub Actions.

**Spec:** `docs/equipo/releases/v0.59-herramientas-y-sitio/spec.md`. Se lee junto con este plan; sus secciones se citan
como «spec Herramientas › …» y «spec §N» (las del sitio).

## Restricciones globales

- **Ramas y PR:** cada tarea va en su rama `v0.59/tNN-<tema>` desde `main` y su PR con Auto-fix. Nunca push a `main`. Se
  fusiona y se publica solo con autorización del usuario. Los commits terminan con
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. `anti-slop/` nunca se sube.
- **Dependencias nuevas:** solo `github.com/tailscale/hujson` (JSON/JSONC conservando comentarios) y
  `github.com/pelletier/go-toml/v2` (solo para validar el TOML que se escribe).
- **Herramientas e identificadores:**

  | id (y nombre de su API) | Nombre en la UI | Archivo global | Archivo en un proyecto | Clave |
  |---|---|---|---|---|
  | `claude-code` | Claude Code | `~/.claude/settings.json` (o `$CLAUDE_CONFIG_DIR/settings.json`) | `<proyecto>/.claude/settings.local.json` | almacén, `apiKeyHelper` |
  | `codex` | Codex | `$CODEX_HOME/config.toml` (`~/.codex/config.toml`) | — | almacén, `auth.command` |
  | `continue` | Continue | `~/.continue/config.yaml` | — | en el archivo |
  | `opencode` | opencode | `~/.config/opencode/opencode.json` (o `.jsonc` si existe) | — | en el archivo |
  | `kilo` | Kilo Code | `~/.config/kilo/kilo.jsonc` | — | en el archivo |

- **URLs:** las de la API de la herramienta, porque su clave solo vale ahí (M0, `m0.md`): Anthropic
  `http://127.0.0.1:<puerto>/p/<api>/anthropic`; OpenAI `http://127.0.0.1:<puerto>/p/<api>/openai/v1`. En `/anthropic`
  y `/openai/v1` solo vale la clave del espacio. El puerto es el del servicio en marcha.
- **Nombres en el almacén:** `TRAMTAI_HERRAMIENTA_CLAUDE_CODE` y `TRAMTAI_HERRAMIENTA_CODEX`. Se añaden a `Internos` en
  `internal/credenciales/credenciales.go:20`.
- **Archivos de Tramtai** (0600, con `configuracion.EscribirAtomico`):
  - `datos/herramientas/<id>.json`, la anotación: `{"id":"codex","ruta":"…","alcance":"global","modelo":"claude/sonnet","api":"Codex","orden":"C:\\…\\tramtai.exe","puerto":17373,"escrito":{"model":"claude/sonnet"},"previos":{"model":"gpt-5"},"noExistia":false,"fecha":"2026-11-05T14:05:00Z"}`;
  - `datos/herramientas/copias/<id>-<AAAAMMDD-HHMMSS>.<ext>`, la copia del archivo entero, hecha antes del primer cambio
    de cada activación.
- **Marcas** en TOML: las líneas `# >>> tramtai` y `# <<< tramtai`. En YAML, el modelo de Continue lleva `name: Tramtai`.
  En opencode/Kilo, el proveedor se llama `tramtai`.
- **API de cada herramienta:** grupo `Herramientas`, nombre = su id (los nombres de API cumplen
  `^[a-z0-9][a-z0-9_-]{0,39}$`; el nombre en la UI es el de la tabla), `ModelosPermitidos` solo el elegido.
- **CLI:** `tramtai herramientas` (alias `tools`), con `listar` (`list`), `activar` (`enable`), `desactivar` (`disable`),
  `olvidar` (`forget`), `restaurar` (`restore`) y `clave` (`key`). `activar` y `desactivar` aceptan `<id>…` o `--todas`
  (`--all`), y `activar` acepta además `--modelo` (`--model`).
- **Modo servidor:** las herramientas no se ofrecen (la cuenta del servicio no es la del usuario). La API responde
  `409 no_disponible_en_servidor`.
- **Textos:**
  - todo en español y en inglés;
  - cada `idioma.T` nuevo lleva su inglés en `internal/idioma/en_*.go` (`TestCobertura`);
  - cada clave nueva de `web/src/idiomas/es.ts` lleva la suya en `en.ts`;
  - la terminal los recibe con `npm run ayuda-terminal`;
  - el glosario y `estilo-textos.test.ts` mandan.
- **Capturas completas:** hacen dos peticiones reales a `claude/haiku` con la suscripción del usuario. Se le avisa antes.
  Nunca se usa el Tramtai instalado del usuario ni su puerto 17373; las copias de prueba van en `bin/capturas-*` y
  `bin/m0-*`, en el puerto 17388.
- **Cada versión pone al día:** Ayuda, guía de desarrolladores (`docs/equipo/desarrollo.md`), README y sitio. Los Minor
  de las revisiones se arreglan dentro de la versión.

## Revisión: lo que más puede fallar

1. **La herramienta reescribe su archivo mientras Tramtai escribe** (Claude Code con `/config`, Codex al guardar). Lo
   esperado es no pisarlo: se relee, se aplica y, si la huella del archivo cambió entre leer y renombrar, se reintenta una
   vez; si no, error «el archivo cambió, vuelve a intentarlo». Prueba en la Tarea 2.
2. **Tramtai se mueve o se actualiza a otra ruta** y la orden de `apiKeyHelper` / `auth.command` deja de existir, o la
   ruta tiene espacios (`C:\Users\Ana María\…`). Lo esperado: la ruta va entre comillas y bien escapada, y el estado pasa a
   «Desactualizada» si el ejecutable anotado no existe. Pruebas en las Tareas 3, 4 y 6.
3. **La herramienta se detecta por el PATH pero no tiene archivo de configuración.** Lo esperado: se crea con 0600, se
   anota `noExistia: true` y, al desactivar, se borra si solo quedaba lo de Tramtai. Pruebas en la Tarea 2.
4. **El usuario tocó a mano lo que escribió Tramtai** (borró el bloque, cambió el modelo) o borró la API en Mis APIs. Lo
   esperado: desactivar no falla; quita lo que siga siendo de Tramtai, deja lo cambiado por el usuario y el estado lo
   dice. Pruebas en las Tareas 3, 4 y 6.
5. **Tramtai en modo servidor.** Lo esperado: la pestaña dice que no está disponible en el servidor y la API no escribe
   nada en la cuenta del servicio. Prueba en la Tarea 6.

---

### Tarea 1: M0 de las herramientas con los programas reales

Antes de construir, comprobar que Claude Code y Codex reales hablan con Tramtai tal como dice la spec, y arreglar lo que
no encaje en las rutas de Tramtai.

**Archivos:**
- Crear: `docs/equipo/releases/v0.59-herramientas-y-sitio/m0.md` (la evidencia)
- Modificar lo que haga falta en `internal/api/anthropic*.go` y `internal/api/openai*.go`, con su prueba en el `_test.go`
  vecino

**Interfaces:** nada nuevo para otras tareas. Fija los valores exactos que escribirán las Tareas 3 a 5.

- [ ] **Paso 1:** avisar al usuario de que la M0 hace peticiones reales con su suscripción, contra una copia de prueba en
  el puerto 17388.
- [ ] **Paso 2:** preparar la copia portable de prueba en `bin/m0-herramientas/`, como hace `capturas.mjs:327-350`:
  `portable.txt`, `config.yaml` con el puerto 17388 y `terminos aceptar`. Crear una API con
  `tramtai acceso crear m0 --grupo Herramientas` y apuntar su clave (se ve una vez).
- [ ] **Paso 3:** Claude Code con `CLAUDE_CONFIG_DIR` apuntando a una carpeta temporal, un `settings.json` con `env`
  (`ANTHROPIC_BASE_URL`, `ANTHROPIC_MODEL=claude/haiku`) y un `apiKeyHelper` que devuelva la clave. Ejecutar
  `claude -p "di hola"`. Anotar si llega a `/anthropic/v1/messages`, si trae la cabecera `anthropic-beta` y si responde.
- [ ] **Paso 4:** Codex con `CODEX_HOME` temporal y el `config.toml` de la spec (`wire_api = "responses"`,
  `auth.command`). Ejecutar `codex exec "di hola"`. Anotar la ruta, el nombre del modelo recibido y si el streaming
  funciona.
- [ ] **Paso 5:** lo mismo con Continue (`config.yaml` con `provider: openai`) y opencode (`opencode run`), si están
  instalados. Si no, anotarlo en `m0.md`; las Tareas 4 y 5 se quedan con la documentación.
- [ ] **Paso 6:** para cada fallo, una prueba en Go en `internal/api` que lo reproduzca (por ejemplo,
  `TestAnthropicAceptaCabeceraBeta`). Verla fallar, arreglar y verla pasar con `go test ./internal/api/...`.
- [ ] **Paso 7:** escribir `m0.md`: la versión de cada programa, el archivo usado, qué funcionó y qué se arregló.
- [ ] **Paso 8:** commit `test(v0.59): M0 de las herramientas con Claude Code y Codex reales`.

### Tarea 2: `internal/apuntar`, el núcleo

**Archivos:**
- Crear: `internal/apuntar/apuntar.go`, `internal/apuntar/anotacion.go`, `internal/apuntar/apuntar_test.go`
- Modificar: `go.mod`, `go.sum` (hujson y go-toml/v2)

**Interfaces:**
- Produce:
  ```go
  type Alcance string // "global" | "proyecto"
  type Valores struct {
      URLAnthropic, URLOpenAI string   // base de cada contrato
      Modelo                  string   // "agente/modelo"
      Clave                   string   // solo para las herramientas con la clave en el archivo
      Orden                   []string // ejecutable absoluto + args, para apiKeyHelper / auth.command
      Puerto                  int
  }
  type Anotacion struct {
      ID, Ruta, Modelo, API, Orden string; Alcance Alcance; Puerto int
      Escrito, Previos map[string]any; NoExistia bool; Fecha time.Time
  }
  type Formato interface {
      Aplicar(actual []byte, v Valores) (nuevo []byte, escrito, previos map[string]any, err error)
      Quitar(actual []byte, a Anotacion) (nuevo []byte, vacio bool, err error)
      Contiene(actual []byte, a Anotacion) bool // ¿sigue lo que escribió Tramtai?
  }
  type Herramienta struct {
      ID, Nombre string; ClaveEnArchivo, EnProyecto bool; Programa string
      Ruta      func(alcance Alcance, proyecto string) (string, error)
      Gestionado func() bool // nil salvo Claude Code
      Formato   Formato
  }
  func Lista() []Herramienta                 // en el orden de la tabla de restricciones
  func Detectada(h Herramienta) bool         // programa en el PATH o carpeta de config
  func Aplicar(datos string, h Herramienta, alcance Alcance, proyecto string, v Valores) (Anotacion, error)
  func Quitar(datos string, h Herramienta, a Anotacion) error
  func Restaurar(datos string, a Anotacion) (copia string, err error)
  func Leer(datos, id string) (Anotacion, bool, error)
  func Borrar(datos, id string) error
  var ErrArchivoIlegible = errors.New("archivo ilegible")
  var ErrCambioMientras = errors.New("el archivo cambió mientras se escribía")
  ```
- Consume: `configuracion.EscribirAtomico(ruta string, datos []byte, permisos os.FileMode) error`.

- [ ] **Paso 1: pruebas que fallan** en `apuntar_test.go`, con un `Formato` de prueba que escribe `{"tramtai":1}`:
  - `TestAplicarHaceCopiaUnaVez`: con un archivo previo, `Aplicar` deja exactamente un archivo en
    `datos/herramientas/copias/` con el contenido original y permisos 0600. Se comprueba con
    `os.Stat(...).Mode().Perm() == 0o600`, salvo en Windows.
  - `TestAplicarCreaSiNoExiste`: sin archivo, se crea con 0600 y `Anotacion.NoExistia == true`.
  - `TestQuitarBorraSiQuedaVacio`: tras `Aplicar` sobre un archivo que no existía, `Quitar` lo borra.
  - `TestAnotacionIdaYVuelta`: `Leer` devuelve la misma `Anotacion` que escribió `Aplicar`.
  - `TestCambioMientrasEscribe`: un `Formato` que modifica el archivo en disco dentro de `Aplicar` provoca un
    reintento; si lo cambia otra vez, `ErrCambioMientras` sin tocar el archivo.
  - `TestArchivoIlegible`: si `Formato.Aplicar` devuelve `ErrArchivoIlegible`, el archivo no cambia y no hay copia.
- [ ] **Paso 2:** `go test ./internal/apuntar/` falla porque no compila.
- [ ] **Paso 3:** implementar.
  - `Aplicar` hace: leer, calcular la huella (sha256), copiar si no hay copia de esta activación, `Formato.Aplicar`,
    releer y comparar la huella, `EscribirAtomico(…, 0o600)` y escribir la anotación.
  - En Windows, si el renombrado falla por bloqueo, reintentar 3 veces con 200 ms de pausa.
- [ ] **Paso 4:** `go test ./internal/apuntar/` pasa.
- [ ] **Paso 5:** commit `feat(apuntar): núcleo con copia, anotación y escritura atómica`.

### Tarea 3: escritor JSON/JSONC (Claude Code, opencode y Kilo)

**Archivos:**
- Crear: `internal/apuntar/json.go`, `internal/apuntar/json_test.go`, `internal/apuntar/testdata/json/*`
- Modificar: `internal/apuntar/apuntar.go` (`Lista` con `claude-code`, `opencode` y `kilo`)

**Interfaces:**
- Consume: `Formato`, `Valores`, `Anotacion` (Tarea 2).
- Produce: `func ClaudeCode() Formato`, `func OpenCode() Formato` (vale para opencode y Kilo) y
  `func claudeGestionado(rutas []string) bool`.

**Lo que se escribe:**
- **Claude Code:**
  - `env.ANTHROPIC_BASE_URL = v.URLAnthropic`;
  - `env.ANTHROPIC_MODEL = v.Modelo`;
  - `apiKeyHelper` = `v.Orden` unida con espacios, con cada parte que tenga espacios entre comillas dobles.

  Si la M0 (Tarea 1) pidió `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1`, va también en `env`.
- **opencode/Kilo:** `provider.tramtai` = `{"npm":"@ai-sdk/openai-compatible","name":"Tramtai","options":{"baseURL":…,"apiKey":v.Clave},"models":{v.Modelo:{"name":v.Modelo+" (Tramtai)"}}}`
  y `model = "tramtai/"+v.Modelo`.

**Ajustes gestionados de Claude Code:** `Gestionado` mira si existe alguno de estos archivos con `env.ANTHROPIC_BASE_URL`
o `apiKeyHelper`:
- Windows: `C:\Program Files\ClaudeCode\managed-settings.json`;
- macOS: `/Library/Application Support/ClaudeCode/managed-settings.json`;
- Linux: `/etc/claude-code/managed-settings.json`.

- [ ] **Paso 1: pruebas que fallan,** con archivos de ejemplo en `testdata/json/`:
  - `TestClaudeCodeConservaLoDemas`: un `settings.json` con `permissions` y `env.OTRA=1` mantiene ambos y gana las
    claves de Tramtai.
  - `TestClaudeCodeGuardaPrevios`: con `env.ANTHROPIC_MODEL="opus"` previo, `previos["env.ANTHROPIC_MODEL"] == "opus"`, y
    `Quitar` lo devuelve.
  - `TestClaudeCodeRespetaCambioDelUsuario`: si tras activar el usuario pone `env.ANTHROPIC_MODEL="sonnet"`, `Quitar` no
    lo toca (solo quita lo que sigue igual a `escrito`) y `Contiene` da `false`.
  - `TestClaudeCodeOrdenConEspacios`: `Orden = ["C:\\Users\\Ana María\\Tramtai\\tramtai.exe","herramientas","clave","claude-code"]`
    produce `"\"C:\\Users\\Ana María\\Tramtai\\tramtai.exe\" herramientas clave claude-code"`.
  - `TestClaudeGestionado`: con un `managed-settings.json` temporal que fija `env.ANTHROPIC_BASE_URL`,
    `claudeGestionado([ruta])` da `true`; sin él, `false`.
  - `TestOpenCodeConservaComentarios`: un `opencode.jsonc` con `// comentario` lo conserva tras `Aplicar` y `Quitar`.
  - `TestJSONRoto`: un archivo con `{"env":` devuelve `ErrArchivoIlegible`.
- [ ] **Paso 2:** `go test ./internal/apuntar/ -run 'Claude|OpenCode|JSONRoto'` falla.
- [ ] **Paso 3:** implementar con `hujson.Parse` y `Patch` (RFC 6902) sobre el valor, y `hujson.Format` solo si el
  original estaba formateado. Las rutas anotadas usan `.` (`env.ANTHROPIC_MODEL`).
- [ ] **Paso 4:** pasa.
- [ ] **Paso 5:** commit `feat(apuntar): Claude Code, opencode y Kilo`.

### Tarea 4: escritor TOML (Codex)

**Archivos:** crear `internal/apuntar/toml.go`, `toml_test.go` y `testdata/toml/*`; modificar `apuntar.go` (`codex` en
`Lista`).

**Interfaces:** produce `func Codex() Formato`.

**Lo que se escribe:**
- Arriba, antes de la primera tabla, un bloque con `model = "<v.Modelo>"` y `model_provider = "tramtai"`.
- Al final, un bloque con `[model_providers.tramtai]` (`name = "Tramtai"`, `base_url = v.URLOpenAI`,
  `wire_api = "responses"`) y `[model_providers.tramtai.auth]` (`command = v.Orden[0]`, `args = v.Orden[1:]`).
- Cada bloque va entre `# >>> tramtai` y `# <<< tramtai`.
- Las líneas `model =` / `model_provider =` de nivel superior que ya existían se comentan con el prefijo `# tramtai: `
  (TOML no admite claves repetidas) y se anotan en `previos`.

- [ ] **Paso 1: pruebas que fallan:**
  - `TestCodexBloquesYValidez`: el resultado lo acepta `toml.Unmarshal` y tiene `model_provider == "tramtai"`.
  - `TestCodexComentaYDevuelvePrevios`: con `model = "gpt-5"` previo, queda `# tramtai: model = "gpt-5"`, y `Quitar` lo
    descomenta tal cual.
  - `TestCodexConservaComentariosYTablas`: un archivo con `# mi nota` y `[mcp_servers.x]` los mantiene iguales, byte a
    byte, fuera de los bloques.
  - `TestCodexBloqueBorradoAMano`: si el usuario borró los bloques, `Quitar` no falla y descomenta lo que siga comentado;
    `Contiene` da `false`.
  - `TestCodexRutaWindows`: `command` con `C:\Users\Ana María\…` sale como cadena TOML literal (`'…'`) válida.
- [ ] **Paso 2:** falla.
- [ ] **Paso 3:** implementar por líneas, sin reserializar. Se valida con `toml.Unmarshal` antes de devolver; si no es
  válido, `ErrArchivoIlegible`.
- [ ] **Paso 4:** pasa.
- [ ] **Paso 5:** commit `feat(apuntar): Codex`.

### Tarea 5: escritor YAML (Continue)

**Archivos:** crear `internal/apuntar/yaml.go`, `yaml_test.go` y `testdata/yaml/*`; modificar `apuntar.go` (`continue`
en `Lista`).

**Interfaces:** produce `func Continue() Formato`.

**Lo que se escribe:** en `models:`, un elemento `{name: Tramtai, provider: openai, model: v.Modelo, apiBase: v.URLOpenAI,
apiKey: v.Clave, roles: [chat, edit, apply], capabilities: [tool_use]}`. Si el archivo no existía, también
`name: Local`, `version: 1.0.0` y `schema: v1`.

- [ ] **Paso 1: pruebas que fallan:**
  - `TestContinueAnadeModelo`: con dos modelos previos quedan tres, el último el de Tramtai.
  - `TestContinueConservaComentarios`: los comentarios de cabeza y de línea siguen tras `Aplicar` y `Quitar`.
  - `TestContinueReemplazaElSuyo`: activar dos veces deja un solo modelo `Tramtai`.
  - `TestContinueYAMLRoto`: `ErrArchivoIlegible`.
- [ ] **Paso 2:** falla.
- [ ] **Paso 3:** implementar con `yaml.Node` de `go.yaml.in/yaml/v3`, buscando el elemento por `name: Tramtai`.
- [ ] **Paso 4:** pasa.
- [ ] **Paso 5:** commit `feat(apuntar): Continue`.

### Tarea 6: orquestación en el servicio y rutas `/api/v1/herramientas`

**Archivos:**
- Crear: `internal/servicio/herramientas.go`, `internal/servicio/herramientas_test.go`
- Modificar:
  - `internal/servicio/web_instancia.go` (rutas en el `switch` de `rutaInstancia`);
  - `internal/credenciales/credenciales.go:20` (`Internos`);
  - `internal/contrato/contrato.go` (tipos);
  - `internal/cliente/cliente.go` (métodos);
  - `internal/servicio/servicio.go` (campos de credenciales);
  - `internal/idioma/en_*.go`.

**Interfaces:**
- Consume:
  - `apuntar.*` (Tarea 2);
  - `configuracion.EditarPuntos`, `NuevaClave(RutaTokenPunto(datos, nombre))`, la lógica de `adminActivarPuntos`;
  - `credenciales.Guardar/Leer/Borrar`.
- Produce:
  ```go
  // contrato
  type EstadoHerramienta struct {
      ID, Nombre, Estado, Modelo, Ruta, Motivo string // Estado: "no_detectada"|"detectada"|"activa"|"desactivada"|"desactualizada"|"solo_global"
      ClaveEnArchivo, Gestionado bool
  }
  type ActivarHerramientas struct { IDs []string; Todas bool; Modelo string }
  // cliente
  func (c *Cliente) Herramientas(ctx context.Context, inst string) ([]contrato.EstadoHerramienta, error)
  func (c *Cliente) ActivarHerramientas(ctx context.Context, inst string, p contrato.ActivarHerramientas) ([]contrato.EstadoHerramienta, error)
  func (c *Cliente) DesactivarHerramientas(ctx context.Context, inst string, ids []string, todas bool) ([]contrato.EstadoHerramienta, error)
  func (c *Cliente) OlvidarHerramienta(ctx context.Context, inst, id string) error
  func (c *Cliente) RestaurarHerramienta(ctx context.Context, inst, id string) (copia string, err error)
  // servicio: campos que las pruebas sustituyen (por defecto, las funciones de internal/credenciales)
  guardarClave func(nombre, valor string) error
  leerClave    func(nombre string) (string, error)
  borrarClave  func(nombre string) error
  ```
- **Rutas** de la instancia (privilegiadas, como `/api/v1/instancia/token`):
  - `GET /api/v1/herramientas`;
  - `POST /api/v1/herramientas/activar` (`ActivarHerramientas`);
  - `POST /api/v1/herramientas/desactivar` (`{"ids":[],"todas":false}`);
  - `POST /api/v1/herramientas/{id}/olvidar`;
  - `POST /api/v1/herramientas/{id}/restaurar`.

**Reglas:**
- **Activar:**
  1. Crea la API `ID` (por ejemplo, `codex`) en el grupo `Herramientas` del espacio, o la reanuda si existe. Las URLs
     de `Valores` son las suyas, `/p/<ID>/anthropic` y `/p/<ID>/openai/v1`.
  2. Genera la clave con `NuevaClave`.
  3. Si `!ClaveEnArchivo`, la guarda con `guardarClave("TRAMTAI_HERRAMIENTA_"+ID_MAYUS, clave)` y pasa en `Valores.Orden`
     el `os.Executable()` + `herramientas clave <id>`. Si no, la pasa en `Valores.Clave`.
  4. Llama a `apuntar.Aplicar`.
  5. Recarga.
- **Modelo por defecto:** el primer agente conectado en el orden de `ordenAgentes`, con su modelo por defecto.
- **Alcance:** desde un espacio de proyecto, solo las herramientas con `EnProyecto`; las demás, `solo_global`.
- **Desactivar:** `apuntar.Quitar` y detiene la API.
- **Olvidar:** desactivar más `borrarClave` y `apuntar.Borrar`.
- **Desactualizada:** la anotación existe pero la API ya no existe, `Puerto` no es el actual, `Orden` no existe en disco o
  `Formato.Contiene` da `false`. `Motivo` dice cuál.
- **Gestionado:** si `h.Gestionado()` da `true`, `Gestionado` y `Motivo` lo avisan; activar se permite igual.
- **Modo servidor:** `409 no_disponible_en_servidor` en todas las rutas.

- [ ] **Paso 1: pruebas que fallan** en `herramientas_test.go`, con un servicio de prueba, `HOME`/`CODEX_HOME`
  temporales y un mapa en memoria como almacén:
  - `TestActivarCodexCreaAPIYGuardaClave`: tras activar, existe la API `codex` en el grupo «Herramientas» con
    `ModelosPermitidos == ["claude/haiku"]`. La clave guardada como `TRAMTAI_HERRAMIENTA_CODEX` autentica contra
    `/p/codex/openai/v1`, y el `config.toml` no la contiene.
  - `TestActivarTodasSoloDetectadas`: con solo `codex` y `continue` detectadas, `Todas` activa esas dos.
  - `TestDesactivarDetieneAPI`: la API queda con `Activo == false` y la clave sigue en el almacén.
  - `TestOlvidarBorraClave`: tras olvidar, la clave ya no está en el almacén y no hay anotación.
  - `TestDesdeProyectoSoloClaudeCode`: desde un espacio de proyecto, `codex` sale `solo_global`, y activar Claude Code
    escribe `<proyecto>/.claude/settings.local.json` y nunca `.claude/settings.json`.
  - `TestDesactualizada`: borrar la API, cambiar el puerto o apuntar `Orden` a un ejecutable que no existe deja el estado
    en `desactualizada` con su `Motivo`, y desactivar no falla.
  - `TestServidorNoOfreceHerramientas`: con el servicio en modo servidor, `GET` responde 409 `no_disponible_en_servidor`.
- [ ] **Paso 2:** `go test ./internal/servicio/ -run Herramienta` falla.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** pasa, junto con `go test ./internal/credenciales/ ./internal/cliente/`.
- [ ] **Paso 5:** commit `feat(servicio): activar y desactivar herramientas`.

### Tarea 7: `tramtai herramientas` en la CLI

**Archivos:**
- Crear: `cmd/tramtai/herramientas.go`, `cmd/tramtai/herramientas_test.go`
- Modificar: `cmd/tramtai/main.go` (registrar el comando), `internal/idioma/en_*.go`

**Interfaces:**
- Consume: los métodos del cliente (Tarea 6), `instanciaActual`, `credenciales.Leer`.
- Produce: `func comandoHerramientas() *cobra.Command`.

- [ ] **Paso 1: pruebas que fallan:**
  - `TestHerramientasAlias`: `tools list`, `tools enable --all --model claude/haiku`, `tools disable codex`,
    `tools forget codex`, `tools restore codex` y `tools key codex` resuelven al mismo comando que su forma en español.
  - `TestClaveSoloLaClave`: con una clave en el almacén (doble de prueba), `herramientas clave codex` escribe exactamente
    la clave más `\n` en stdout, nada en stderr, y sale con 0.
  - `TestClaveSinActivar`: sin clave, sale con 1 y un mensaje en stderr, sin nada en stdout.
  - `TestRestaurarPideConfirmacion`: sin `--si`, `restaurar` pregunta, como `servicio detener`.
- [ ] **Paso 2:** `go test ./cmd/tramtai/ -run 'Herramientas|Clave|Restaurar'` falla.
- [ ] **Paso 3:** implementar. `clave` no arranca ni contacta con el servicio: lee el almacén directamente, porque Codex
  la llama cada 300 s.
- [ ] **Paso 4:** pasa.
- [ ] **Paso 5:** commit `feat(cli): tramtai herramientas`.

### Tarea 8: pestaña Herramientas en la Web UI

**Archivos:**
- Crear: `web/src/componentes/accesos/Herramientas.vue`, `web/src/componentes/accesos/Herramientas.test.ts`
- Modificar:
  - `web/src/paginas/Accesos.vue:26-78` (tercera pestaña `name="herramientas"`) y `:148` (el tipo de `pestana` admite
    `'herramientas'`);
  - `web/src/api/cliente.ts` (`herramientas`, `activarHerramientas`, `desactivarHerramientas`, `olvidarHerramienta`,
    `restaurarHerramienta`);
  - `web/src/api/tipos.ts` (`EstadoHerramienta`);
  - `web/src/idiomas/es.ts` y `en.ts` (claves `herramientas.*`).

**Interfaces:** consume las rutas de la Tarea 6. Produce las claves `herramientas.*` que usará la terminal (Tarea 9).

**Pantalla:**
- **Encima de la lista:** «Activar todas» y «Desactivar todas».
- **Cada fila:**
  - el nombre;
  - el estado como `.etiqueta` (punto y palabra);
  - el modelo;
  - la ruta del archivo en mono;
  - un `q-toggle` con su texto.
- **Menú `MenuAcciones` de cada fila:** «Cambiar modelo», «Olvidar» y «Restaurar la copia» (destructiva, al final, en
  color de error, con diálogo `persistent`).
- **Avisos:**
  - una fila `solo_global` dice «Solo desde el espacio global»;
  - con `ClaveEnArchivo`, una nota bajo la fila: «La clave queda en su archivo, solo para ti»;
  - con `Gestionado`, su aviso;
  - activar abre un diálogo con el selector de agente y modelo (el de Probar), con el primero conectado elegido.
- **Estado vacío** (`EstadoVacio`) si no se detecta ninguna: «No encontramos ninguna herramienta compatible» y el enlace
  al tema de Ayuda.

- [ ] **Paso 1: pruebas que fallan** en `Herramientas.test.ts`:
  - pinta una fila por herramienta con su estado en texto;
  - el interruptor llama a `activarHerramientas({ids:['codex'],todas:false,modelo})`;
  - «Activar todas» manda `todas:true`;
  - una fila `solo_global` tiene el interruptor deshabilitado y el texto;
  - con el servicio respondiendo 409 `no_disponible_en_servidor` se ve «No disponible en el servidor».
- [ ] **Paso 2:** `cd web; npm test -- Herramientas` falla.
- [ ] **Paso 3:** implementar siguiendo `ListaApis.vue` y `ClaveEspacio.vue`.
- [ ] **Paso 4:** pasa, y `npm run build` y `npm test` en `web/` también.
- [ ] **Paso 5:** commit `feat(web): pestaña Herramientas en Mis APIs`.

### Tarea 9: pestaña Herramientas en la terminal

**Archivos:**
- Modificar:
  - `internal/tui/paginas/conectar/pagina.go:78-81` (`pestanaHerramientas` tras `pestanaClave`);
  - `vista.go:84` (`pestanas()`);
  - `conectar_test.go`;
  - `internal/tui/datos/textos.json` (con `npm run ayuda-terminal`).
- Crear: `internal/tui/paginas/conectar/herramientas.go`.

**Interfaces:** consume los métodos del cliente (Tarea 6) y las claves `herramientas.*` (Tarea 8).

**Teclas:**
- `Espacio` activa o desactiva la fila;
- `t` activa todas y `T` desactiva todas;
- `m` cambia el modelo;
- `r` restaura (pide confirmación);
- `o` olvida.

Van en la barra de teclas como las demás.

- [ ] **Paso 1: pruebas que fallan** en `conectar_test.go`:
  - `TestHerramientasPestana`: con las flechas se llega a la tercera pestaña y se ve una fila por herramienta con su
    estado;
  - `TestHerramientasEspacioActiva`: `Espacio` sobre Codex llama a `ActivarHerramientas` con `IDs == ["codex"]`;
  - `TestHerramientasComoLaWeb`: los textos de estado salen de las mismas claves `herramientas.*` que la Web UI.
- [ ] **Paso 2:** `go test ./internal/tui/paginas/conectar/` falla.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** pasa, y `cd web; npm test -- terminal-datos` también.
- [ ] **Paso 5:** commit `feat(tui): pestaña Herramientas en Mis APIs`.

### Tarea 10: Ayuda «Conectar tus herramientas»

**Archivos:**
- Crear: `web/src/ayuda/temas/herramientas.ts`
- Modificar:
  - `web/src/ayuda/indice.ts`;
  - `web/src/idiomas/ayuda.es.ts` y `ayuda.en.ts`;
  - `web/scripts/capturas-lista.ts` (captura `herramientas` de la página `conectar` con
    `guardado: {'tramtai.pestanaApis':'herramientas'}`);
  - `web/scripts/sitio.test.ts` (el número de temas pasa de 18 a 19).

**Contenido** (spec Herramientas › Ayuda):
- qué toca en cada herramienta (la tabla de restricciones);
- dónde está la copia y cómo desactivar, olvidar y restaurar;
- por qué la clave de Continue y opencode/Kilo va en su archivo;
- el aviso de sincronización.

`paginaApp: 'conectar'`, `relacionados: ['claves','conectar']`.

- [ ] **Paso 1:** poner a 19 los temas en `sitio.test.ts` y ver que falla.
- [ ] **Paso 2:** escribir el tema en es y en.
- [ ] **Paso 3:** `cd web; npm run sitio; npm test` pasa (estilo y glosario incluidos).
- [ ] **Paso 4:** commit `docs(ayuda): conectar tus herramientas`.

### Tarea 11: alias en inglés y prueba de los comandos del sitio

**Archivos:**
- Crear: `cmd/tramtai/sitio_test.go`
- Modificar: `cmd/tramtai/servicio.go:161,166` (`servicio`→`service`, `iniciar`→`start`, y los demás subcomandos de
  `servicio` que salgan en el sitio), `internal/idioma/en_*.go` si cambia algún texto

**Interfaces:** produce `TestComandosDelSitioExisten`, que las Tareas 15 y 16 mantienen verde.

- [ ] **Paso 1: prueba que falla,** `TestComandosDelSitioExisten`:
  - Lee `../../site/index.html` y `../../site/servidor.html`.
  - Dentro de cada `<pre>`, quita las etiquetas y toma cada línea que, tras `sudo ` o `.\`, empiece por `tramtai`,
    `tramtai.exe`, `./tramtai` o `tramtai-servidor`. Este último equivale a `tramtai --servidor`.
  - Las líneas dentro de un `lang="en"` se comprueban como inglés; el resto, como español.
  - A esas líneas suma una lista fija con las que enseñará el sitio nuevo: `tramtai service start -d`,
    `sudo ./tramtai server install`, `sudo tramtai-servidor admin password` y `.\tramtai.exe server install`.
  - Busca cada una con `comandoRaiz().Find(args)` y falla si no existe o si una línea en inglés solo se resuelve con un
    nombre en español.
- [ ] **Paso 2:** `go test ./cmd/tramtai/ -run ComandosDelSitio` falla con `service start`.
- [ ] **Paso 3:** añadir los alias.
- [ ] **Paso 4:** pasa.
- [ ] **Paso 5:** commit `feat(cli): alias en inglés de los comandos del sitio`.

### Tarea 12: la terminal en HTML para el sitio

**Archivos:**
- Modificar:
  - `internal/tui/captura.go:43` (esperar también a `inicio.comprobando`);
  - `internal/tui/captura_test.go` (crear si no existe);
  - `web/scripts/capturas-lista.ts` (`TERMINAL` y `ansiAHtml`);
  - `web/scripts/capturas.test.ts`;
  - `web/scripts/capturas.mjs:182-215`.
- Crear: `site/capturas/terminal.json` (generado), `site/cascadia.woff2` y `site/CASCADIA-OFL.txt`.
- Borrar: `site/capturas/terminal-*.webp` (16).

**Interfaces:**
- Produce:
  - `TERMINAL: { nombre: 'inicio'|'apis'|'historial'; teclas: string[] }[]` con `[]`, `['4']` y `['5']`;
  - `export function ansiAHtml(ansi: string): string`, que ya no recibe tema y devuelve `<span class="t-<token>">`;
  - `export const TOKENS_TERMINAL: Record<string, string>` (hex → token, los 24 de la spec §5);
  - el archivo `site/capturas/terminal.json`, con la forma `{"es":{"inicio":"<html>","apis":"…","historial":"…"},"en":{…}}`.

- [ ] **Paso 1: pruebas que fallan:**
  - En `capturas.test.ts`:
    - `ansiAHtml` convierte `\x1b[38;2;45;212;191mX` y `\x1b[38;2;11;110;100mX` al mismo
      `<span class="t-acento">X</span>`;
    - escapa `<&>`;
    - lanza un error con un color sin token (`#123456`).
  - En Go, `TestCapturaEsperaComprobando`: con un agente lento (como `TestPrimerosPasosComprobando` en
    `internal/tui/paginas/inicio/inicio_test.go:511`), `tui.Captura` no devuelve una vista con «Comprobando…» si el
    agente responde antes de `espera`.
- [ ] **Paso 2:** `go test ./internal/tui/ -run Captura` y `cd web; npm test -- capturas` fallan.
- [ ] **Paso 3:** implementar.
  - **Captura:** `capturasTerminal` pinta cada página a 100×28 en `es` y `en`, solo con el tema oscuro (las clases valen
    para los dos), y escribe `terminal.json`.
  - **Fallos:** si la vista contiene «Comprobando…» o «Checking…», falla. Si Historial no tiene peticiones, también.
- [ ] **Paso 4:** pasa.
- [ ] **Paso 5:** la fuente.
  - Generar `site/cascadia.woff2` con `uv tool run --from fonttools[woff] pyftsubset CascadiaMono.ttf --flavor=woff2`,
    con los caracteres de `terminal.json` más ASCII imprimible y un tope de 60 KB.
  - Poner la licencia OFL junto a la de Overpass.
  - El comando exacto queda en `docs/equipo/desarrollo.md`.
- [ ] **Paso 6:** `npm run capturas -- --terminal`, con aviso al usuario por las dos peticiones reales. Comprobar que
  `terminal.json` tiene las seis pantallas y que ninguna enseña una ruta real.
- [ ] **Paso 7:** commit `feat(sitio): la terminal en HTML con clases de tema`.

### Tarea 13: capturas de la app en es/en y del servidor para el sitio

**Archivos:**
- Modificar:
  - `web/scripts/capturas.mjs:463-477` (recorrer `['es','en']` y nombrar `<nombre>-<tema>-<idioma>-<ancho>.webp`);
  - `capturas.mjs:233-325` (`capturasServidor` también guarda `site/capturas/servidor-entrar-…` y `servidor-inicio-…` en
    es/en × claro/oscuro, a 1440 y 960);
  - `web/scripts/portada.test.ts:265`.
- Borrar: las `site/capturas/<nombre>-<tema>-<ancho>.webp` sin idioma.

`PORTADA` (`capturas-lista.ts:152`) no cambia: inicio, agentes, mis-apis, probar y espacios.

- [ ] **Paso 1: prueba que falla** en `portada.test.ts`: «cada captura de la app y del servidor tiene su versión `es` y
  `en`, con `lang` y `srcset`, y existe en disco».
- [ ] **Paso 2:** falla.
- [ ] **Paso 3:** implementar en `capturas.mjs`.
- [ ] **Paso 4:** `npm run capturas` (completa, con aviso al usuario) y `npm run capturas -- --servidor` (necesita
  Docker). Revisar a ojo una captura de cada idioma y tema.
- [ ] **Paso 5:** la prueba pasa (las referencias del HTML se cambian en la Tarea 14; hasta entonces, la prueba mira solo
  los archivos en disco).
- [ ] **Paso 6:** commit `feat(sitio): capturas de la app y del servidor en español e inglés`.

### Tarea 14: portada — estructura, ventajas y «Cómo funciona»

**Archivos:**
- Modificar:
  - `site/index.html`;
  - `site/estilo.css`;
  - `site/sitio.js`;
  - la barra común en `web/scripts/sitio.ts` (zona `cabecera`);
  - `web/scripts/portada.test.ts`.

**Qué cambia** (spec §1, §3 y §4):
- las diez secciones en su orden;
- la barra «Cómo funciona · Terminal · Servidor · Agentes · Ayuda · Desarrolladores»;
- los tres accesos bajo los botones;
- «Por qué Tramtai» con la franja de cuatro cifras y la línea de seis paradas, con el texto exacto de spec §4;
- «Cómo funciona» con pestañas OpenAI · Anthropic · curl y el paso 3 sobre las herramientas;
- «La app» con las capturas en es/en (Tarea 13);
- «Conecta estos agentes / Desde tus apps y SDK» dentro de «Agentes»;
- Privacidad y riesgos juntos.

Se quitan `.motivos` y su CSS.

- [ ] **Paso 1: pruebas que fallan** en `portada.test.ts`:
  - el orden de los `id` de `<section>` es exactamente `portada, por-que, como, app, terminal, servidor, agentes,
    funciones, privacidad, descargar`;
  - la barra tiene enlaces a `#terminal` y `#servidor`;
  - hay cuatro `.cifra` y seis `.parada-ventaja`, cada una con `lang="es"` y `lang="en"`;
  - no queda `.motivo`;
  - las pestañas de «Cómo funciona» son un `role="tablist"` con tres `role="tab"`;
  - cada `<img>` de «La app» tiene su pareja en el otro idioma.
- [ ] **Paso 2:** `cd web; npm test -- portada` falla.
- [ ] **Paso 3:** implementar.
  - Las cifras van en Overpass con `tabular-nums`.
  - La línea usa el trazo y las estaciones del sitio (acento, no colores de agente): dos columnas desde 900 px, una por
    debajo.
  - Las pestañas reutilizan el patrón accesible de `.mapa-pestana`.
  - Los enlaces internos a anclas viejas (`#capturas`, `#pagas`, `#aviso`) se cambian, y `sitio.test.ts` («anclas viejas»)
    lo vigila.
- [ ] **Paso 4:** `npm run sitio; npm test` pasa (estilo, glosario y anclas).
- [ ] **Paso 5:** commit `feat(sitio): portada en diez secciones con ventajas y cifras`.

### Tarea 15: portada — la terminal

**Archivos:**
- Modificar:
  - `site/index.html` (`#terminal`);
  - `site/estilo.css` (`.ventana-terminal`, `.t-*` con los tokens de `DESIGN.md` en claro y oscuro, `@font-face` de
    Cascadia);
  - `site/sitio.js` (pestañas);
  - `web/scripts/sitio.ts` (rellenar una zona `<!-- comun:terminal -->` con `terminal.json`);
  - `web/scripts/portada.test.ts`.

**Qué se ve** (spec §5):
- una ventana que sigue el tema, con semáforo, pestañas «1 Inicio · 4 Mis APIs · 5 Historial» y el rótulo;
- debajo, el pie de figura, la leyenda `<kbd>` y el bloque de dos comandos con su versión en inglés
  (`tramtai service start -d`).

Sin JavaScript, Inicio visible y las otras pantallas en `<details>`. Las teclas 1, 4 y 5 y las flechas solo funcionan con
el foco en las pestañas.

- [ ] **Paso 1: pruebas que fallan:**
  - la zona `comun:terminal` existe y `npm run sitio` la rellena con seis `<pre class="pantalla">` (3 páginas × 2
    idiomas) sin colores en línea (prohibido `style=` dentro);
  - hay un `role="tablist"` con tres pestañas;
  - no queda ningún `terminal-*.webp`.
- [ ] **Paso 2:** falla.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** `npm run sitio; npm test` y `go test ./cmd/tramtai/ -run ComandosDelSitio` pasan.
- [ ] **Paso 5:** commit `feat(sitio): la terminal como texto real en la portada`.

### Tarea 16: el servidor en la portada y en `servidor.html`

**Archivos:** modificar `site/index.html` (`#servidor`), `site/servidor.html`, `site/estilo.css`, `site/sitio.js` y
`web/scripts/portada.test.ts` y `sitio.test.ts`.

**Qué cambia** (spec §6):
- **En la portada:**
  - la frase;
  - el mapa SVG (tus equipos → HTTPS → Tramtai en tu servidor → agentes) con `<tspan lang>`;
  - las cuatro virtudes;
  - las dos tarjetas;
  - las pestañas Linux · Windows Server con su orden en es y en (`sudo ./tramtai server install`,
    `sudo tramtai-servidor admin password`, `.\tramtai.exe server install`);
  - la línea del entorno y los riesgos.
- **En `servidor.html`:**
  - la cabecera nueva (titular, entrada de unas 30 palabras, el mapa grande, «¿Es para ti?» y los tres pasos);
  - las capturas `servidor-entrar` y `servidor-inicio` (Tarea 13);
  - el Aviso con los cinco riesgos movido justo antes de «Instalar»;
  - el inglés de la página repasado entero, con las órdenes en su forma en inglés.

- [ ] **Paso 1: pruebas que fallan:**
  - `#servidor` tiene un `svg` con `role="img"`, dos tarjetas y un `role="tablist"` con «Linux» y «Windows Server»;
  - en `servidor.html`, `#aviso` va justo antes de `#instalar` y la zona `comun:riesgos-servidor` sigue completa;
  - no aparece la palabra «Docker» en la portada ni en la cabecera de `servidor.html`.
- [ ] **Paso 2:** falla.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** `npm run sitio; npm test` y `go test ./cmd/tramtai/` (incluida `TestComandosDelSitioExisten`) pasan.
- [ ] **Paso 5:** commit `feat(sitio): el modo servidor presentado`.

### Tarea 17: auditoría, prueba final con los programas reales, documentación y cierre

**Archivos:**
- Modificar:
  - `docs/diseno/DESIGN.md` (la ventana de terminal y la línea de ventajas en «Piezas comunes»);
  - `docs/equipo/desarrollo.md` (`internal/apuntar`, cómo se generan `terminal.json` y la fuente);
  - `README.md` (si enseña algo que cambió);
  - los temas de Ayuda «Solo con la terminal» y «Modo servidor» si un texto dejó de coincidir;
  - `docs/equipo/releases/v0.59-herramientas-y-sitio/m0.md`;
  - `docs/equipo/pendientes.md`;
  - las notas de la versión.

- [ ] **Paso 1: con los programas reales** (aviso al usuario por las peticiones reales). Con una copia de prueba en el
  puerto 17388 y `HOME`/`CODEX_HOME`/`CLAUDE_CONFIG_DIR` temporales:
  1. `tramtai herramientas activar claude-code codex --modelo claude/haiku`;
  2. `claude -p "di hola"` y `codex exec "di hola"` responden por Tramtai (se ven en su Historial);
  3. `tramtai herramientas desactivar --todas` deja los archivos como estaban.

  Anotarlo en `m0.md`.
- [ ] **Paso 2:** sitio en marcha (`.claude/launch.json` «pagina», apuntando a este worktree). Pasar las skills
  `impeccable` y `antislop-ui` sobre la portada y `servidor.html`. Arreglar lo que salga o anotarlo como falso positivo
  con su motivo.
- [ ] **Paso 3:** accesibilidad:
  - las pestañas con el teclado (Tab, flechas, Inicio/Fin);
  - `aria-selected`;
  - el contraste de los `.t-*` y de las cifras en los dos temas (`check_contrast`, 4,5:1 el texto);
  - `prefers-reduced-motion`.
- [ ] **Paso 4:** a 375 px y a 1440 px, en claro y oscuro y en es y en: la página no se desplaza en horizontal (la
  ventana de terminal se desplaza dentro de su marco). Capturas de comprobación en el PR.
- [ ] **Paso 5:** los tres avisos de «cramped padding» de `.aviso` en `servidor.html`: medir con `getComputedStyle` y
  arreglarlos o anotar el motivo en el PR.
- [ ] **Paso 6:** revisión de código de la rama entera (`superpowers:requesting-code-review`), con foco en
  `internal/apuntar`, las rutas nuevas y la sección «Revisión: lo que más puede fallar». Los Minor se arreglan aquí.
- [ ] **Paso 7:** `go test ./...` y `cd web; npm test; npm run build; npm run sitio` pasan, y `npm run sitio` no deja
  cambios.
- [ ] **Paso 8:** la documentación de la lista de archivos; `pendientes.md` con la v0.59 lista para publicar.
- [ ] **Paso 9:** commit `docs(v0.59): diseño, guía y cierre`. Las notas de la versión se enseñan al usuario antes de
  publicar.
