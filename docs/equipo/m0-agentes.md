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

## Sesión del CLI sin coste (v0.54, 2026-10-20)

Cómo sabe Tramtai, sin enviar ningún mensaje, si el CLI de un agente con suscripción sigue con sesión, si pasó a una
clave de API y qué cuenta usa (`releases/v0.54-espacios-de-trabajo/spec.md`, §3). Probado en Windows 11 con los CLI
reales; las cuentas se anonimizan.

- **Claude Code 2.1.288 — `claude auth status --json`** (también sin `--json`: es lo predeterminado). JSON con
  `loggedIn`, `authMethod`, `email`, `orgName` y `subscriptionType`. Con sesión de la suscripción:
  `{"loggedIn": true, "authMethod": "claude.ai", "email": "<correo>", "subscriptionType": "max", …}`. Sin sesión
  (`CLAUDE_CONFIG_DIR` vacío): `{"loggedIn": false, "authMethod": "none", …}`. Con `ANTHROPIC_API_KEY` en el entorno
  añade `"apiKeySource": "ANTHROPIC_API_KEY"`, pero Tramtai ejecuta los CLI con el entorno limpio. No gasta mensajes ni
  red apreciable (lee su configuración). **Sesión sin coste:** sí — `loggedIn: false` → sin sesión; `authMethod` distinto
  de `claude.ai` → usa clave de API; la cuenta es `email`.
- **Codex CLI 0.160.0 — `codex login status`.** Texto: `Logged in using ChatGPT` (suscripción, salida 0), `Logged in using
  an API key - sk-prueb***falsa` (clave, salida 0; probado con una clave falsa en un `CODEX_HOME` temporal) y `Not logged
  in` (salida 1). No informa la cuenta (está en el token de `auth.json`, que Tramtai no lee). **Sesión sin coste:** sí,
  sin cuenta.
- **GitHub Copilot CLI 1.0.98 — `~/.copilot/config.json`** (o `$COPILOT_HOME`), que Tramtai ya leía para saber si hay
  sesión. JSONC con `loggedInUsers: [{host, kind, login}]` y `lastLoggedInUser`; no guarda tokens. No tiene comando de
  estado. **Sesión sin coste:** sí; la cuenta es `login`. No se puede saber si «pasó a clave»: con clave Tramtai usa
  `COPILOT_GITHUB_TOKEN`, no la sesión del CLI.
- **Antigravity CLI 1.4.2:** solo admite clave de API (`GEMINI_API_KEY`) en Tramtai; no hay sesión que comprobar.
- **Para todos:** si una petición real falla por autenticación, el agente queda con «La sesión del CLI no responde» hasta
  que otra funciona (detección pasiva).

## v0.58: servidor

M0 de la suscripción de los CLI en el servicio del sistema (`releases/v0.58-servidor/spec.md`, §5 y «M0 antes de
implementar»). Las pruebas del sistema (perfil de la cuenta virtual, Administrador de credenciales, `systemd-creds`) están
en el anexo A de esa spec.

### Pendiente del usuario: inicios de sesión de los CLI con la cuenta del servicio

No se hicieron: necesitan las cuentas del usuario (Claude, ChatGPT, GitHub). **Hasta que se hagan, la Tarea 13 usa los
valores del plan** (los tres CLI con suscripción en el servidor, cada uno con su inicio de sesión propio) y esta sección
sigue abierta. Si alguno falla, se aplica la tabla de la spec (ese agente sin suscripción en el servidor, y entonces sí se
añade `ServidorSuscripcion bool` a `agentes.Ficha`).

**Linux** (una cuenta de prueba que se borra al acabar):

```sh
sudo useradd --system --create-home --home-dir /var/lib/tramtaim0 --shell /usr/sbin/nologin tramtaim0
sudo -u tramtaim0 -H claude auth login
sudo -u tramtaim0 -H codex login --device-auth     # antes: activar el código de dispositivo en ChatGPT (Seguridad)
sudo -u tramtaim0 -H copilot login                 # o un token de grano fino con «Copilot Requests» en COPILOT_GITHUB_TOKEN
sudo -u tramtaim0 -H claude auth status --json     # loggedIn, authMethod "claude.ai"
sudo -u tramtaim0 -H codex login status            # «Logged in using ChatGPT»
sudo -u tramtaim0 -H sh -c 'ls -a ~ ~/.claude ~/.codex ~/.copilot'   # dónde guarda cada CLI su sesión
sudo userdel -r tramtaim0                          # al terminar
```

**Windows** (cuenta virtual): buscar si cada CLI inicia sesión con `NT SERVICE\TramtaiM0`. No hay consola interactiva
para una cuenta virtual, así que se lanza el mismo `claude auth login`, `codex login --device-auth` y `copilot login`
desde un servicio de prueba (`sc.exe create TramtaiM0 … obj= "NT SERVICE\TramtaiM0"`, como la sonda de
`pruebas/m0servidor`) con la salida en un archivo de una carpeta que esa cuenta pueda escribir, para leer el código de
dispositivo y la dirección. Su perfil es `C:\WINDOWS\ServiceProfiles\TramtaiM0` (anexo A): ahí hay que mirar qué
carpetas crea cada CLI (`.claude`, `.codex`, `.copilot`, `AppData`) y, tras `claude auth status --json`,
`codex login status` y el `config.json` de Copilot, si la sesión queda. Esta orden aún no se ha probado: si un servicio
de prueba no sirve para un inicio de sesión con código, la Tarea 13 lo resolverá con `tramtai --servidor agentes entrar`.

Anotar aquí el resultado de cada CLI y sistema (inicia sesión sí o no, carpeta donde guarda la sesión) y borrar las
cuentas, servicios y perfiles de prueba.

### Términos vigentes sobre el uso remoto personal (consultados el 2026-10-26)

Citas cortas, solo lo pertinente a usar la propia suscripción desde un servidor propio. Los textos de condiciones pueden
cambiar: se vuelven a mirar antes de publicar.

**Anthropic**

- [Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance): «Advertised usage limits for Pro and
  Max plans assume ordinary, individual usage». Los terceros no pueden enrutar peticiones por la suscripción de sus usuarios ni
  recoger sus credenciales: «developers may not collect, store, or intermediate Claude.ai credentials or session tokens».
  Pero sí vale que cada usuario inicie sesión él mismo: «signing in to the unmodified Claude Code binary with their own
  Claude subscription».
- [Consumer Terms](https://www.anthropic.com/legal/consumer-terms), §2 y §3: «You may not share your Account login
  information.» Y prohíben acceder «through automated or non-human means», salvo clave de API o «where we otherwise
  explicitly permit it». Es la zona gris: la página de Claude Code permite el CLI sin modificar con la cuenta del titular,
  pero no dice nada de manejarlo desde una API propia.

**OpenAI**

- [Codex, autenticación](https://developers.openai.com/codex/auth) (redirige a `learn.chatgpt.com/docs/auth`): el inicio por
  dispositivo es la vía documentada para equipos remotos: «Enable device code login in your ChatGPT security settings».
  Para automatizar recomienda otra: «API keys are still the recommended default for automation.» Avisa de dos cosas que
  afectan al servidor: «Treat `~/.codex/auth.json` like a password: it contains access tokens.» y «Don’t expose Codex
  execution in untrusted or public environments.»
- [Términos de uso de OpenAI](https://openai.com/es-419/policies/row-terms-of-use/) (versión para el resto del mundo, en
  español; consultados el 2026-10-26 con un navegador, porque las herramientas web reciben un 403). Apartado «Registro»:
  «No puedes compartir las credenciales de tu cuenta». Entre los usos prohibidos: «Extraer datos o Resultados … de forma
  automática o mediante programación». La primera es la regla de una cuenta para una persona. La segunda trata de extraer
  datos o resultados de los servicios (raspado), no del modo no interactivo documentado de Codex (`codex exec`); no se
  puede decir más de lo que dice el texto.

**GitHub**

- [Condiciones del servicio](https://docs.github.com/en/site-policy/github-terms/github-terms-of-service), B.3: «Your login
  may only be used by one person». Y las
  [políticas de uso aceptable](https://docs.github.com/en/site-policy/acceptable-use-policies/github-acceptable-use-policies),
  §4, prohíben «any form of excessive automated bulk activity».
- [Autenticar Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/authenticate-copilot-cli):
  el token con «Copilot Requests» «Must be owned by your personal account», y el uso con variables de entorno es «ideal for
  CI/CD pipelines, containers, or headless servers». No hay prohibición expresa de usarlo en un servidor propio.

**Lectura.** Ningún texto prohíbe expresamente que el titular use su propia suscripción, con el CLI sin modificar y su
propio inicio de sesión, desde un servidor suyo. Los tres condicionan lo que la spec ya exige: solo el titular, sin repartir
la cuenta, uso ordinario e individual (de ahí `suscripcion_simultaneas` en 1), y el riesgo de suspensión que Anthropic
se reserva sin aviso previo. Los términos de OpenAI tampoco prohíben que el titular use su propia cuenta desde su servidor:
prohíben compartir las credenciales o la cuenta, que es lo que la suscripción personal ya impide con sus condiciones. Por
los términos, ningún agente queda fuera de la suscripción personal en remoto.
