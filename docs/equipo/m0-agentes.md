# Evidencia M0: Copilot CLI, OpenCode y Hermes Agent

Pruebas con los CLI reales el 2026-10-18 en Windows 11, con las cuentas del usuario y modelos pequeños o gratuitos.
Sirven a la spec de agentes por módulos (`releases/v0.43-agentes-modulos/spec.md`) y a los releases de cada agente.

## GitHub Copilot CLI 1.0.98 (`npm install -g @github/copilot`)

- **Plan:** Copilot Free incluye la CLI (docs.github.com, planes), pero solo con el modelo `auto` (en las pruebas eligió
  `mai-code-1.1-flash` y `gpt-6-luna`). Cada petición gasta créditos del plan: 1 «premium request» por prompt.
- **Autenticación:** `copilot login` (lo hace el usuario). La cuenta queda en `~/.copilot/config.json`
  (`loggedInUsers`, `lastLoggedInUser`, sin secretos) y el token en el almacén del sistema. Con `COPILOT_HOME` privado
  no hay sesión, salvo copiar esas dos claves al `config.json` privado: así funciona. El token clásico de `gh` (`ghp_`)
  se rechaza; valen `COPILOT_GITHUB_TOKEN`/`GH_TOKEN` con token fine-grained u OAuth. Sin GitHub: BYOK con
  `COPILOT_PROVIDER_BASE_URL`, `COPILOT_PROVIDER_TYPE` (openai, azure, anthropic; Ollama) y `COPILOT_PROVIDER_API_KEY`.
- **No interactivo:** `copilot -p <texto> --output-format json` escribe JSONL:
  - `session.auto_mode_resolved` → `chosenModel` (el modelo efectivo con `auto`);
  - `assistant.message_delta` → `deltaContent` (texto en vivo);
  - `assistant.message` → `content`, `model`, `toolRequests`;
  - `tool.execution_start` / `tool.execution_complete`;
  - `result` → `exitCode`, `usage.premiumRequests`.
- **Uso:** `--usage-output-file <archivo>` deja un JSON con tokens de entrada, salida, caché y razonamiento por modelo,
  créditos (`totalNanoAiu`) y premium requests. Coste fijo ≈ 12 200 tokens de entrada por llamada; ≈ 4–6 s una
  respuesta corta.
- **Esfuerzo:** `--reasoning-effort none|minimal|low|medium|high|xhigh|max`; con `auto` falla («Model "auto" does
  not support reasoning effort configuration»).
- **Aislamiento (canario):**
  - **Falla** con `--available-tools=` (vacío se ignora): usó PowerShell y leyó el archivo trampa; los comandos de
    solo lectura se ejecutan sin pedir permiso. La orden y la URL se negaron solo por no haber nadie que aprobara.
  - **Pasa** con `--available-tools none --disallow-temp-dir`, más `--no-custom-instructions --disable-builtin-mcps
    --no-ask-user --no-auto-update`: ninguna llamada a herramientas y «NO PUEDO» a las tres tareas.
- **Persistencia:** guarda la sesión (`session-store.db`, `session-state/`) en `COPILOT_HOME`: carpeta privada por
  ejecución que se borra al terminar.
- **Entrada y errores (pruebas del 2026-10-18):** el mensaje por la entrada estándar funciona (tildes y eñes bien, 1
  premium request). Un modelo inexistente sale con código 1 y `Error: Model "…" from --model flag is not available.`
  por stderr, sin gastar créditos. No hay orden para listar modelos.
- **Con el adaptador de Tramtai (v0.44):** el canario pasa (`gpt-6-luna`, «NO PUEDO» a las tres tareas) y la prueba
  real responde y se cancela en 4 s. Con `--no-custom-instructions` y sin MCP la entrada baja a unos 1 700–6 300 tokens.
  Gastaron 3 créditos del plan gratuito (canario, respuesta y cancelación). En Windows, npm deja `copilot.cmd`; el
  adaptador usa el `copilot.exe` nativo de `@github/copilot-win32-x64`.
- **Archivos:** `--attachment` admite imágenes y documentos nativos; Tramtai seguirá enviando el texto convertido.

## OpenCode 1.19.3 (`npm install -g opencode-ai`)

- El `postinstall` (bloqueado por npm) solo copia el binario del paquete de la plataforma y comprueba su versión; se
  ejecutó a mano tras leerlo.
- **No interactivo:** `opencode run --format json -m proveedor/modelo [--variant esfuerzo] [-f archivo] <texto>`
  escribe JSONL:
  - `step_start`;
  - `text` → `part.text` (el texto de la parte completo, no por fragmentos);
  - `tool_use` → `part.tool`, `part.state.status|output|error`;
  - `step_finish` → `part.tokens` (`input`, `output`, `reasoning`, `cache`) y `part.cost`;
  - `error` → `error.data.message` y `statusCode`.
- **Modelos:** `opencode models <proveedor>`. OpenCode Zen trae 12 modelos gratis sin cuenta (por ejemplo
  `opencode/deepseek-v4.1-flash-free`, `opencode/big-pickle`, `opencode/glm-5.3-flash-free`). Coste fijo ≈ 8 900
  tokens de entrada.
- **Zen gratis exige parecer OpenCode normal:** con el permiso `read` denegado (o `"*": "deny"`) responde 403
  `FreeTierError` («OpenCode's free tier can only be used from within OpenCode»). Los demás permisos sí se pueden
  denegar. Con proveedores de pago no se probó (sin clave).
- **Aislamiento (canario pasa):**
  - carpetas privadas `XDG_DATA_HOME`, `XDG_CONFIG_HOME`, `XDG_CACHE_HOME` y `XDG_STATE_HOME` (sesiones y `auth.json`
    van a la de datos);
  - permisos por `OPENCODE_CONFIG_CONTENT`: `deny` en glob, grep, list, edit, bash, webfetch, websearch, codesearch,
    task, external_directory, todowrite y skill;
  - `read` queda permitido solo dentro de la carpeta de trabajo, vacía (`external_directory` denegado).
  
  La lectura del archivo trampa fuera de la carpeta se bloqueó, sin terminal ni web.
- **Pendiente para su release:**
  - OpenCode lee `~/.claude/CLAUDE.md` si no hay AGENTS.md global (`OPENCODE_DISABLE_CLAUDE_CODE`);
  - las claves de proveedores irán por variables de entorno;
  - el modelo efectivo no aparece en los eventos de texto.

## Antigravity CLI 1.4.2 (repaso del 2026-10-18)

- **Mensaje por stdin:** `agy --input-format stream-json --output-format stream-json -p=` lee una línea
  `{"event":"user","message":{"role":"user","content":"…"}}`. El `-p` vacío va al final: con un valor aparte, agy toma la
  opción siguiente como mensaje.
- **Salida:** `init` (herramientas disponibles), `step_update` (`step_type` `agent_response` con `text_delta` en vivo, o
  `tool` con `state` `ACTIVE`/`DONE`/`ERROR`) y `result` (`status`, `response`, `error`, `usage` con `input_tokens`,
  `output_tokens`, `thinking_tokens` y `cache_read_tokens`). No informa el modelo efectivo.
- **Sin herramientas:** un hook en `.agents/hooks.json` de la carpeta de trabajo, `PreToolUse` con `matcher: "*"` y una
  orden que imprime `{"decision":"deny",…}`, niega todas: el canario (leer un archivo, ejecutar una orden y abrir una
  URL) termina con las tres herramientas en `ERROR` «denied by pre-tool hook» y nada filtrado. Se carga aunque la
  carpeta no sea de confianza. La orden no puede llevar comillas (en Windows llegan como `\"` a `cmd` y el hook falla);
  se usa un `.cmd` o `.sh` aparte, que admite rutas con espacios.
- **Esquema:** con `--json-schema`, agy entrega el JSON con su herramienta `finish`, que el hook niega; el formato va en
  las instrucciones.
- **Esfuerzo:** `--effort low|medium|high|xhigh|max`.
- **Pendiente:** el canario con `GEMINI_API_KEY` y `modelProvider: gemini`, que es como lo ejecuta Tramtai (estas pruebas
  usaron la sesión del usuario directamente con agy, fuera de Tramtai).

## Hermes Agent 0.22.2

- **Instalación:** instalador oficial de Nous Research en PowerShell (revisado antes: uv, Git portable con SHA-256,
  `%LOCALAPPDATA%\hermes`, PATH del usuario). Funcionó; un segundo instalador en paralelo falla («Another Hermes update
  is already running»).
- **Pendiente:** necesita proveedor (`hermes setup`, lo hace el usuario) y queda para el final, por decisión del
  usuario. Según su documentación, `hermes -z <texto>` da solo la respuesta final por stdout.
