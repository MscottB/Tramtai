# Evidencia M0: Copilot CLI, OpenCode y Hermes Agent

Pruebas con los CLI reales el 2026-10-18 en Windows 11, con las cuentas del usuario y modelos pequeños o gratuitos.
Sirven a la spec de agentes por módulos (`agentes-modulos.md`) y a los releases de cada agente.

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

## Hermes Agent 0.22.2

- **Instalación:** instalador oficial de Nous Research en PowerShell (revisado antes: uv, Git portable con SHA-256,
  `%LOCALAPPDATA%\hermes`, PATH del usuario). Funcionó; un segundo instalador en paralelo falla («Another Hermes update
  is already running»).
- **Pendiente:** necesita proveedor (`hermes setup`, lo hace el usuario) y queda para el final, por decisión del
  usuario. Según su documentación, `hermes -z <texto>` da solo la respuesta final por stdout.
