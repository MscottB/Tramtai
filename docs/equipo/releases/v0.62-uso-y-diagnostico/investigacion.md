# Gasto y presupuesto fiables en Tramtai: qué existe y qué tomar

Investigación para la v0.62 «Uso y diagnóstico». Fecha: 14-11-2026. Solo lectura de fuentes públicas, sin iniciar sesión en ningún sitio.

## Idea central

Hay **dos contadores distintos** que no se deben mezclar:

| | Suscripción (Claude Pro/Max, ChatGPT Plus/Pro, Copilot, Antigravity) | Clave de API (Anthropic, OpenAI, Gemini) |
|---|---|---|
| Límite real | Porcentaje de una ventana del proveedor (5 h, semanal; en Copilot, créditos al mes) | Dinero (y límites de RPM/TPM) |
| Quién sabe la verdad | Solo el proveedor | El proveedor (factura). Tramtai puede estimarla bien: tokens × precio |
| Fuente fiable | Lo que el propio CLI recibe del proveedor: `rate_limit_event`, `account/rateLimits/read`, `account.getQuota`, `/usage` | Tokens que devuelve el proveedor y una tabla de precios. Para conciliar: Usage/Cost API con clave de administrador |
| Qué hacen los contadores de tokens con suscripción | Son solo una aproximación. Anthropic no publica cuántos tokens caben en una ventana, y la cuota se comparte con claude.ai, la app de escritorio y otros equipos | Exactos si se usan los campos `usage` de cada respuesta |

Los presupuestos actuales de Tramtai (peticiones y tokens por API y por espacio) sirven como **límites propios**. No dicen cuánto queda de la cuota del proveedor. Para eso hay que leer lo que expone cada CLI.

---

## 1. Lectores del uso local de los CLI

### ccusage
- **URL:** https://github.com/ryoppippi/ccusage (npm `ccusage`).
- **Licencia:** MIT.
- **Mantenimiento:** muy activo. Último release v20.0.51, publicado el 14 de noviembre de 2026. La v20 está reescrita en Rust.
- **Qué mide:** tokens y coste **estimado** por día, semana, mes, sesión y bloque de 5 h. Lee 19 fuentes, entre ellas Claude Code, Codex, GitHub Copilot CLI, Gemini CLI y Antigravity.
- **Cómo lo obtiene:**
  - lee los JSONL locales: `~/.claude/projects/**.jsonl` y `~/.codex/sessions/**`;
  - calcula el coste con la tabla de precios de LiteLLM, que va embebida o se descarga al compilar; `--offline` usa la copia en caché;
  - admite precios propios en `ccusage.json`;
  - `ccusage blocks` reconstruye las ventanas de 5 h de Claude a partir de las marcas de tiempo.
- **Fiabilidad:**
  - **Tokens:** buena, si se eliminan duplicados. Claude Code escribe varias líneas por mensaje en streaming.
  - **Coste:** estimado a precio de lista.
  - **Cuota restante:** no la conoce. Los «bloques» los deduce él; no son los del proveedor.
  - No ve el uso que se hace desde otros equipos ni desde claude.ai.
- **Encaje con Tramtai:** como **idea y referencia**:
  - cómo eliminar duplicados (por `message.id` y `requestId`);
  - qué campos leer de cada JSONL;
  - cómo dividir las ventanas de 5 h.
  
  No sirve como biblioteca desde Go. Ejecutarlo como binario añadiría una dependencia que no compensa.

### Claude-Code-Usage-Monitor
- **URL:** https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor (PyPI `claude-monitor`).
- **Licencia:** MIT.
- **Mantenimiento:** activo. Último release v4.0.0, de junio de 2026.
- **Qué mide:** el consumo en vivo dentro de la ventana de 5 h y una predicción de cuándo se agotará.
- **Cómo lo obtiene:**
  - **por defecto, estima:** usa límites fijos por plan (pro 19 000, max5 88 000 y max20 220 000 tokens) o el percentil 90 de tu historial;
  - **con `--statusline`:** usa el `rate_limits` real que Claude Code pasa a la statusline, y marca como «estimación local» lo que esté caducado;
  - **con `--api`** (experimental): llama al endpoint OAuth no documentado.
- **Fiabilidad:**
  - el modo por defecto es solo heurístico; las cifras por plan son del autor, no de Anthropic;
  - el modo statusline es fiable.
- **Encaje con Tramtai:** como **idea**. Dos cosas a copiar:
  - la etiqueta «dato del proveedor» frente a «estimación local»;
  - volver a la estimación cuando el dato real caduca.

### CodexBar y herramientas parecidas (codexusage, codex-limit, burnrate, abtop)
- **URL:** https://github.com/steipete/CodexBar (macOS). No verifiqué la licencia.
- **Qué mide:** el porcentaje usado de las ventanas de 5 h y semanal de Codex y Claude, y los créditos.
- **Cómo lo obtiene (en Codex):**
  - prefiere el RPC local `codex app-server`;
  - si no puede, extrae la salida de `/status` en una PTY.
- **Encaje con Tramtai:** como **idea**. Confirma que la vía buena en Codex es `app-server` (ver la sección 4).

---

## 2. Tablas de precios mantenidas por la comunidad

### LiteLLM `model_prices_and_context_window.json`
- **URL:** https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json
  - Versión cruda: `https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json`
- **Licencia:** MIT. El repositorio es MIT salvo la carpeta `enterprise/`.
- **Mantenimiento:** muy activo. Es el estándar de facto: lo usan ccusage y varios paquetes de Go. No verifiqué la fecha del último commit del archivo.
- **Qué contiene:** precio por token de cada modelo y proveedor. Pesa unos **3,9 MB**. Algunos campos:
  - `input_cost_per_token`, `output_cost_per_token`;
  - `cache_read_input_token_cost`, `cache_creation_input_token_cost` y `..._above_1hr`;
  - `output_cost_per_reasoning_token`;
  - `max_input_tokens`, `max_output_tokens`;
  - `litellm_provider`, `mode`, `deprecation_date`;
  - los recargos por tramo, con sufijos `_above_200k_tokens`.
- **Fiabilidad:** buena en los modelos populares, pero mantenida a mano por la comunidad. A veces va días por detrás de un cambio de precio y tiene errores puntuales.
- **Encaje con Tramtai:** **sí, como dato**:
  - un script de release filtra solo los proveedores y modelos que Tramtai usa (Anthropic, OpenAI, Gemini) y lo embebe con `go:embed`;
  - queda en unos pocos KB y no necesita red.

  No hace falta ninguna dependencia de Go. Los paquetes que lo leen (`voocel/litellm/pricing`, `agentuity/go-common/llm`, `adamallcock/runcost`) son pequeños y no compensan.

### models.dev (sst)
- **URL:** https://github.com/sst/models.dev y la API `https://models.dev/api.json`.
- **Licencia:** MIT.
- **Mantenimiento:** activo. Más de 15 000 commits, 7 600 estrellas; es el catálogo que usa opencode.
- **Qué contiene:**
  - precios **por millón de tokens**: entrada, salida, razonamiento, lectura y escritura de caché;
  - límites de contexto y fechas.
  
  Cada modelo es un archivo TOML revisado por PR.
- **Encaje con Tramtai:** como **segunda fuente para contrastar** en el script de release. Si LiteLLM y models.dev difieren en un modelo, el script avisa antes de publicar.

### tokencost (AgentOps)
- **URL:** https://github.com/AgentOps-AI/tokencost
- **Licencia:** MIT.
- **Mantenimiento:** **flojo**. Último release 0.1.26, de agosto de 2025. Su tabla deriva de la de LiteLLM.
- **Encaje con Tramtai:** **ninguno.** Es Python y está desactualizado. Mejor ir a la fuente (LiteLLM).

### OpenRouter models API
- **URL:** `GET https://openrouter.ai/api/v1/models`. No requiere autenticación.
- **Licencia:** es un servicio, no un proyecto abierto.
- **Qué contiene:** `pricing.prompt` y `pricing.completion` como cadenas, en USD por token, con campos de caché y razonamiento. Usa centinelas: `"0"` significa gratis y `"-1"` significa que el precio es dinámico.
- **Fiabilidad:** son los precios a los que **OpenRouter revende**. Casi siempre coinciden con la lista oficial, pero no tienen por qué.
- **Encaje con Tramtai:** solo como **comprobación manual**. No depender de él.

### Tablas oficiales en tiempo de ejecución
**Copilot:** el SDK expone `models.list`, con `billing.multiplier` y `billing.tokenPrices`. Es la única tabla oficial que se puede leer en tiempo de ejecución. Úsala si Tramtai habla con Copilot por SDK o ACP.

---

## 3. Pasarelas y proxies con presupuestos

Ninguna de estas herramientas sabe nada de la cuota de una suscripción: miden tráfico de API que pasa por ellas. Tramtai ya **es** la pasarela, así que interesan como fuente de ideas, no como dependencias.

| Herramienta | URL | Licencia | Mantenimiento | Qué ofrece | Encaje |
|---|---|---|---|---|---|
| **LiteLLM Proxy** | https://docs.litellm.ai/docs/proxy/users | MIT, salvo `enterprise/` | Muy activo. En mar. 2026, versiones 1.82.7 y 1.82.8 **comprometidas en PyPI** (TeamPCP) | Claves virtuales con `max_budget`, `soft_budget`, `budget_duration` ("30d"), `tpm_limit`, `rpm_limit` y `max_parallel_requests`. Presupuestos por clave, usuario, equipo, etiqueta y proveedor. Necesita Postgres, y Redis si hay varias instancias | **Idea:** modelo de presupuesto con duración que se reinicia, umbral blando para avisar y duro para cortar, y varias ventanas a la vez. No como dependencia |
| **Portkey Gateway** | https://github.com/Portkey-AI/gateway | MIT | Activo. Con «Gateway 2.0» los límites de presupuesto pasaron al código abierto. Una sola fuente dice que Palo Alto Networks lo compró (may. 2026); sin confirmar | Límites de presupuesto y de ritmo, pero solo en modelos con precio conocido | **Idea:** no aplicar un presupuesto en dinero a un modelo sin precio; avisar en su lugar |
| **Helicone** | https://github.com/Helicone/helicone | Apache-2.0. La pasarela aparte figura como GPL-3.0 | **Solo mantenimiento** desde que Mintlify lo compró (mar. 2026) | Coste por petición, usuario y modelo | **Ninguno** |
| **Langfuse** | https://github.com/langfuse/langfuse | MIT en el núcleo | Activo. ClickHouse lo compró (ene. 2026) | Trazas, coste deducido con definiciones de modelo que puede editar el usuario | **Idea:** tabla de precios editable por el usuario, que prevalece sobre la incluida |
| **OpenLLMetry** | https://github.com/traceloop/openllmetry | Apache-2.0 | Activo, v0.62.4 (sep. 2026). ServiceNow compró Traceloop | Instrumentación OTel con atributos `gen_ai.usage.*` | **Ninguno.** Python o JS para la app que llama, no para el proxy |
| **OpenMeter** | https://github.com/openmeterio/openmeter | Apache-2.0 | Activo, dentro de Kong desde sep. 2025 | Medición por eventos, derechos de uso y facturación | **Idea:** guardar eventos crudos (tokens por petición) y agregarlos al leer. No como dependencia: necesita Kafka o ClickHouse |
| **Claude apps gateway** (Anthropic) | https://code.claude.com/docs/en/claude-apps-gateway | De Anthropic | Activo | Límites de gasto por usuario delante de Claude Code. Claude Code ve el resultado en `rate_limits.spend_limit` | Solo de referencia |

---

## 4. Cómo exponen el uso real los proveedores

### Anthropic, con clave de API

- **Usage and Cost API** (legado), y su sucesora, la **Usage and Cost Analytics API**:
  - Endpoints: `GET /v1/organizations/usage_report/messages` y `GET /v1/organizations/cost_report`.
  - Necesita una **clave de administrador** (`sk-ant-admin01-…`). No vale una clave normal.
  - **No está disponible** para cuentas individuales de la Console sin organización, ni para el plan Team de claude.ai.
  - Los datos tardan unos 5 minutos en aparecer.
  - Se puede consultar una vez por minuto.
  - El coste va en centavos, solo con granularidad diaria.
  - Sirve para **conciliar** el gasto, no para controlarlo en tiempo real.
  - Doc: https://platform.claude.com/docs/en/manage-claude/usage-cost-api
- **Cabeceras de rate limit:**
  - `anthropic-ratelimit-{requests,tokens,input-tokens,output-tokens}-{limit,remaining,reset}`;
  - `retry-after`;
  - `anthropic-workspace-id`.
  
  Miden RPM e ITPM, que **no son dinero**. Las lecturas de caché no cuentan para el ITPM.
- **Límite de gasto:**
  - Al llegar al tope mensual del nivel, la API devuelve 429 con `error.details.error_code = "enforced_spend_limit_reached"` y sin `retry-after`.
  - Si el límite lo fijó el usuario, devuelve 400 `invalid_request_error`.
  - Tramtai debería distinguir estos casos en «Comprobar conexión». Doc: https://platform.claude.com/docs/en/api/rate-limits
- **Tramtai no ve estas cabeceras** cuando la petición pasa por el CLI: las recibe el CLI. Solo las vería si llamara a la API directamente.

### Claude Code con suscripción

Lo que Claude Code da de forma documentada:

- **`/usage`:**
  - barras de uso del plan (5 h y semanal);
  - desglose por skills, subagentes y MCP;
  - créditos de uso extra.
  
  La cifra en dólares de la sesión es una estimación a precio de lista y, según la documentación, «no es relevante para la facturación» de los suscriptores. Desde v2.1.208, si el endpoint de uso está limitado, muestra la última lectura con el aviso «Showing last-known usage». Doc: https://code.claude.com/docs/en/costs
- **JSON de entrada de la statusline:** documentado oficialmente.
  - `rate_limits.five_hour.used_percentage`, de 0 a 100, y `rate_limits.five_hour.resets_at`, en segundos Unix;
  - lo mismo para `seven_day`;
  - `rate_limits.spend_limit.*` detrás de un Claude apps gateway;
  - `cost.total_cost_usd`, estimado.
  
  Condiciones: **solo para suscriptores Pro, Max y Ultra**, y **solo después de la primera respuesta** de la sesión. Cada ventana puede faltar y se descarta al pasar `resets_at`. Doc: https://code.claude.com/docs/en/statusline
- **`claude -p --output-format json` o `stream-json`:**
  - el `result` trae `total_cost_usd`, el desglose por modelo y `usage`, todos **estimados en el cliente**;
  - en `stream-json` llegan además eventos `rate_limit_event` con `rate_limit_info`: `status` (`allowed`, `allowed_warning` o `rejected`), `resetsAt`, `rateLimitType` (por ejemplo `five_hour`), `isUsingOverage` y, a veces, `utilization` o `unifiedWindows`.
  
  Cuidado:
  - los tipos `SDKRateLimitEvent` y `SDKRateLimitInfo` existen desde CLI v2.1.45, pero su documentación es escasa;
  - varios issues dicen que `utilization` **suele faltar con `allowed`**;
  - un bug informa de que el evento puede intercalarse en medio de otra línea NDJSON;
  - las unidades de `resetsAt` difieren según la fuente (segundos o milisegundos).
  
  Doc: https://code.claude.com/docs/en/headless
- **`--max-budget-usd`:** tope de gasto estimado por ejecución del CLI.
- **OpenTelemetry**, con `CLAUDE_CODE_ENABLE_TELEMETRY=1`:
  - la métrica `claude_code.cost.usage` (USD) y la métrica `claude_code.token.usage`, con el atributo `type`;
  - el evento `claude_code.api_request`, con `cost_usd`, `cost_usd_micros`, los tokens, `request_id`, `effort` y `speed`;
  - protocolo `http/json` disponible.
  
  Es un coste **estimado**, igual que el resto. Doc: https://code.claude.com/docs/en/monitoring-usage

Lo que no está documentado:

- **Endpoint `GET https://api.anthropic.com/api/oauth/usage`** con `anthropic-beta: oauth-2025-04-20`. Devuelve `five_hour`, `seven_day`, `seven_day_sonnet` y `extra_usage`, con `utilization` y `resets_at`. Lo usan muchas herramientas (cship, claude-usage, ai-usagebar).
- **Cabeceras `anthropic-ratelimit-unified-*`** que reciben las peticiones con OAuth de suscripción. Hay informes de que faltan en algunos planes (Team, Max).
- **Términos de uso:** la página Legal and compliance dice que los desarrolladores «may not collect, store, or intermediate Claude.ai credentials or session tokens». Leer el token OAuth del usuario para llamar a ese endpoint choca con esa frase. **No hacerlo.**

### Codex con ChatGPT Plus/Pro

- **`codex app-server`, RPC `account/rateLimits/read`:** **documentado y sin la marca de experimental**. Doc: https://learn.chatgpt.com/docs/app-server (antes developers.openai.com/codex/app-server).
  - Devuelve `rateLimits`, y `rateLimitsByLimitId` cuando hay varios grupos.
  - Dentro de `rateLimits`:
    - `primary` y `secondary`, cada uno con `usedPercent`, `windowDurationMins` (300 para 5 h y 10080 para la semana) y `resetsAt` en segundos Unix;
    - `credits`, `planType`, `rateLimitReachedType` y `rateLimitResetCredits`.
  - Notificación `account/rateLimits/updated` cuando cambian los límites.
  - Notificación `thread/tokenUsage/updated` con el uso del hilo.
- **Archivos de sesión en `~/.codex/sessions/`**, sin documentación formal:
  - eventos `{"type":"event_msg","payload":{"type":"token_count", ...}}`;
  - `total_token_usage` acumulado y `last_token_usage`;
  - `rate_limits.primary` y `rate_limits.secondary`, con `used_percent`, `window_minutes` y `resets_at`.
  
  Sirve de alternativa si no se usa `app-server`.
- **`/status`:** muestra el porcentaje **restante** de las ventanas de 5 h y semanal. Necesita al menos un mensaje en la sesión.
- **Cambios de plan:**
  - OpenAI volvió a poner la ventana de 5 h en Plus el 25 de agosto de 2026;
  - en Pro sigue sin ventana de 5 h «durante los próximos meses».

  Por eso Tramtai **no debe dar por hecho que siempre hay dos ventanas**.
- **Problemas conocidos:** hay informes de «límite alcanzado» mientras `/status` marcaba un 97 % libre. El dato del proveedor también puede fallar.

### OpenAI, con clave de API

- **Usage API:** `GET /v1/organization/usage/completions`, más los de embeddings y otros.
- **Costs API:** `GET /v1/organization/costs`.
- Ambas necesitan una **clave de administrador** de la organización. Hay informes de 404 y de datos vacíos.
- Cookbook: https://developers.openai.com/cookbook/examples/completions_usage_api
- **Cabeceras:** `x-ratelimit-limit-requests`, `x-ratelimit-remaining-requests`, `x-ratelimit-reset-requests` y sus equivalentes `-tokens`. Son ritmo, no dinero.

### GitHub Copilot

- **Facturación desde el 1 de junio de 2026:** pasa a **AI Credits** según tokens (entrada, salida y caché) a precio de API de cada modelo, con 1 crédito = 0,01 USD.
  - Pro incluye 10 USD al mes y Pro+ incluye 39 USD.
  - Los planes anuales anteriores siguen con *premium requests* y multiplicadores hasta que vencen.
  - Los administradores fijan presupuestos por empresa, centro de coste o usuario.
  - Fuente: https://github.blog/changelog/2026/04/27/github-copilot-is-moving-to-usage-based-billing/
- **Copilot SDK**, documentado: https://docs.github.com/en/copilot/how-tos/copilot-sdk/features/usage-and-billing
  - `account.getQuota` devuelve `quotaSnapshots`. Contiene `premium_interactions`, `chat` y `completions`, cada uno con `entitlementRequests` (`-1` significa ilimitado), `usedRequests`, `remainingPercentage` y `resetDate`.
  - El evento `assistant.usage` trae, por llamada, `model`, `inputTokens`, `outputTokens` y `cost` (el multiplicador). Es efímero: no se repite al reanudar la sesión.
  - `session.usage.getMetrics` da `totalNanoAiu` y `totalPremiumRequestCost`, con desglose por modelo. **Es experimental.**
  - `session.quota.get` y `session.quota.refresh` son experimentales. La documentación insiste en que un valor que falta se deja vacío, nunca como cero, y que un fallo de lectura no significa cuota agotada.
  - `models.list` devuelve `billing.tokenPrices`.
- **Copilot CLI:**
  - `/usage` muestra lo consumido en la sesión, el desglose de tokens por modelo y la duración;
  - `/limits set max-ai-credits N` fija un tope por sesión, en vista previa.
- **REST:** `/users/{username}/settings/billing/premium_request/usage` da totales resumidos, pensados para el sistema antiguo.
- **`api.github.com/copilot_internal/user`:** **interno**. Muestra una instantánea en caché de hace unos minutos, puede cambiar sin aviso y no funciona con todos los tokens. Evitarlo.

### Antigravity

- **`/usage`**, alias `/quota`: vuelve a pedir la cuota al servidor y muestra las ventanas de 5 h y semanal por grupo de modelos (Gemini por un lado; Claude y GPT por otro).
- Según una guía que cita la documentación oficial, `agy -p "/usage"` funciona sin interfaz y admite `--json`. **No pude abrir la documentación oficial; hay que verificarlo.**
- En el foro hay quejas de que el porcentaje mostrado no coincide con los cortes, y una petición abierta de una «API oficial de cuota de solo lectura».

### API de Gemini

- Límites **por proyecto**: RPM, TPM y RPD. El RPD se reinicia a medianoche de la hora del Pacífico.
- **Límites de gasto:**
  - por ventana de 10 minutos: 10 USD en el nivel 1 y 200 USD en los niveles 2 y 3;
  - tope del proyecto: 250 USD en el nivel 1, 2 000 USD en el nivel 2 y de 20 000 a más de 100 000 USD en el nivel 3.
- **La documentación oficial no menciona cabeceras con la cuota restante.** Algunos tutoriales citan `x-ratelimit-*`, pero no está confirmado.
- Al agotar la cuota devuelve `429 RESOURCE_EXHAUSTED`, a veces con `retryDelay`.
- Doc: https://aistudio.google.com/docs/rate-limits
- **Conclusión:** con Gemini, Tramtai solo puede **contar por su cuenta** con `usageMetadata` y la tabla de precios, y mostrar el 429 cuando llegue.

---

## 5. Recomendación para Tramtai

1. **Separar «cuota» y «dinero» en el modelo de datos y en el panel.**
   - Cada cifra del panel lleva su origen: «del proveedor» (con la hora de lectura), «estimado por Tramtai» o «límite propio».
   - Una cuota del proveedor con más de unos 15 minutos de antigüedad pasa a «estimado» o «última lectura hace X».
   - Un dato que falta se muestra como «desconocido», nunca como 0. Esta regla la recomienda la documentación del SDK de Copilot.

2. **Leer la cuota real de cada suscripción por la vía documentada del propio CLI, sin tocar tokens OAuth:**
   - **Claude:** cuando ejecute `claude -p` con `--output-format stream-json`, guardar el último `rate_limit_event`: `status`, `rateLimitType`, `resetsAt`, `isUsingOverage` y `utilization` si viene. Leer de forma tolerante:
     - campos opcionales;
     - `resetsAt` en segundos o en milisegundos;
     - líneas NDJSON que se cortan y saltarlas.
     
     Como complemento opcional, ofrecer un comando que el usuario pueda poner como statusline de Claude Code, por ejemplo `tramtai statusline`, para recoger `rate_limits.five_hour` y `rate_limits.seven_day`, que tienen documentación oficial.
   - **Codex:** `codex app-server` y su RPC `account/rateLimits/read`, más la notificación `account/rateLimits/updated`. Si no hay `app-server`, leer el último `token_count.rate_limits` del archivo de sesión en `~/.codex/sessions`. Contemplar que `secondary` o `primary` sean `null`: Pro no tiene ventana de 5 h.
   - **Copilot:** si Tramtai habla con Copilot por SDK o ACP, usar `account.getQuota` para la cuota y `assistant.usage` para el coste por llamada. Si no, mostrar los créditos consumidos en la sesión y enlazar al panel de facturación de GitHub. No usar `copilot_internal`.
   - **Antigravity:** probar `agy -p "/usage" --json` en M0. Si funciona, leerlo en «Comprobar conexión»; si no, mostrar «sin dato del proveedor».

3. **Presupuestos de suscripción expresados en porcentaje de la ventana del proveedor.**
   - Por ejemplo: «avisar al 70 %, parar al 90 % de la ventana de 5 h de Claude».
   - Para la cuenta atrás, usar el `resetsAt` del proveedor; no calcularlo por cuenta propia.
   - Los presupuestos de peticiones y tokens que ya existen se quedan como límites propios por API y espacio, con el nombre bien puesto.

4. **Coste con clave de API: tokens del proveedor por una tabla de precios embebida y versionada.**
   - Un script de release baja el JSON de LiteLLM (MIT), lo filtra a los modelos de Anthropic, OpenAI y Gemini que usa Tramtai y lo contrasta con models.dev.
   - Lo embebe con `go:embed` junto a un `prices_as_of`.
   - Sin dependencias nuevas y sin red en tiempo de ejecución.
   - El usuario puede sobrescribir precios en la configuración, que prevalece sobre la tabla incluida.
   - Si un modelo no tiene precio, se avisa con «modelo sin precio» y no se aplica el presupuesto en dinero, como hace Portkey.

5. **Guardar los tokens crudos y calcular el coste al leer, no al escribir.** Es el modelo de eventos de OpenMeter. Así:
   - se puede recalcular cuando cambian los precios;
   - las tendencias del panel salen de agregar esos eventos;
   - se pueden contemplar todas las dimensiones que cambian el precio: entrada sin caché, escritura de caché a 5 minutos y a 1 hora, lectura de caché, salida, razonamiento, recargo de contexto largo (más de 200k), lote, modo rápido y residencia de datos (×1,1 en Anthropic).

6. **Aplicar el presupuesto en dos tiempos y apoyarse en los topes nativos de cada CLI.**
   - **Antes de la petición:** comprobar lo gastado más una estimación.
   - **Después:** sumar lo real.
   - Topes nativos:
     - Claude Code: pasar `--max-budget-usd` con lo que queda del presupuesto del espacio.
     - Copilot CLI: `/limits set max-ai-credits`, en vista previa.
   - Del modelo de LiteLLM conviene copiar:
     - `budget_duration` con reinicio;
     - el umbral blando que avisa y el duro que corta;
     - varias ventanas a la vez, por ejemplo diaria y mensual.

7. **Conciliación opcional con clave de administrador**, en el caso de que el usuario tenga una organización de API:
   - Anthropic: Usage and Cost Analytics API.
   - OpenAI: Costs API.
   - Se consultan como mucho una vez por hora y se muestra «estimado frente a facturado» con el desfase.
   - «Comprobar conexión» distingue estos casos, que hoy acaban todos como un error genérico:
     - `enforced_spend_limit_reached`, sin `retry-after`;
     - límite propio, que devuelve 400;
     - límite de ritmo, que devuelve 429 con `retry-after`;
     - cuota de suscripción agotada (`rate_limit_event` con `rejected`).

8. **No adoptar ninguna pasarela como dependencia** (LiteLLM Proxy, Portkey, Helicone, Langfuse, OpenLLMetry ni OpenMeter):
   - miden tráfico de API, no suscripciones;
   - piden Postgres, Redis, ClickHouse o Kafka;
   - Tramtai ya es la pasarela;
   - LiteLLM sufrió un ataque a la cadena de suministro en marzo de 2026.

---

## 6. Riesgos

- **Precios que cambian:**
  - Anthropic, OpenAI y Google cambian precios y estructuras (tramos de contexto largo, TTL de caché, modo rápido, residencia de datos) sin avisar a terceros.
  - La tabla de LiteLLM puede ir días por detrás o tener errores.
  - Copilot cambió de *premium requests* a AI Credits el 1 de junio de 2026 y conserva el sistema antiguo para planes anuales.

  Mitigación: fecha visible en la tabla, precios editables por el usuario, aviso cuando la tabla tenga más de N días y recálculo desde los tokens crudos.
- **Cuotas de suscripción no documentadas o cambiantes:**
  - Anthropic no publica cuántos tokens caben en las ventanas de 5 h y semanal; varían por plan y por promociones (por ejemplo, +25 % semanal desde septiembre).
  - OpenAI quitó y volvió a poner la ventana de 5 h en Plus en 2026, y Pro sigue sin ella.
  - La cuota de Claude se comparte con claude.ai, la app de escritorio y otros equipos, y la de Codex con la web y el IDE. Por eso Tramtai **nunca** verá todo el consumo.
  - Los campos de `rate_limit_event`, de los archivos de sesión de Codex y del `/usage` de Antigravity están poco documentados o sin documentar: pueden cambiar de nombre o desaparecer en cualquier versión del CLI.
  - Hay casos de corte con cuota libre (Codex, Antigravity).

  Mitigación: lectores tolerantes, el estado «desconocido» y una prueba de contrato por CLI en «Comprobar conexión».
- **Términos de uso:**
  - **Anthropic:** la página Legal and compliance prohíbe a terceros recoger, guardar o intermediar credenciales o tokens de sesión de Claude.ai. También prohíbe enrutar peticiones con credenciales de planes Free, Pro, Max o Ultra «en nombre de sus usuarios».
    - Permite expresamente que el usuario inicie sesión con su propia suscripción en el binario **sin modificar** de Claude Code.
    - Los límites anunciados presuponen un uso «ordinario e individual» de Claude Code y del Agent SDK.
    - Desde el 4 de abril de 2026, las herramientas de terceros que usan la suscripción consumen «uso extra».
    
    Consecuencia: Tramtai debe seguir ejecutando el CLI oficial con la sesión del propio usuario, sin leer `~/.claude/.credentials.json` ni llamar a `api/oauth/usage`.
  - **OpenAI y GitHub:** términos parecidos sobre credenciales. `copilot_internal` y los endpoints de `chatgpt.com/backend-api` son internos.
  - Un uso intensivo y automatizado a través de una API local puede salirse de ese «uso ordinario» y provocar medidas del proveedor sin aviso. Conviene decirlo con claridad en la Ayuda.
- **Seguridad de la cadena de suministro:** el compromiso de LiteLLM en PyPI muestra el riesgo de depender de un proxy de terceros. Embeber datos (JSON) con una revisión del diff en cada release evita ejecutar código ajeno.

---

## Fuentes principales

- Claude Code, costes y `/usage`: https://code.claude.com/docs/en/costs
- Claude Code, statusline y `rate_limits`: https://code.claude.com/docs/en/statusline
- Claude Code sin interfaz (`-p`, json y stream-json): https://code.claude.com/docs/en/headless
- Claude Code, OpenTelemetry: https://code.claude.com/docs/en/monitoring-usage
- Claude Code, Legal and compliance: https://code.claude.com/docs/en/legal-and-compliance
- Anthropic, rate limits: https://platform.claude.com/docs/en/api/rate-limits
- Anthropic, Usage and Cost API: https://platform.claude.com/docs/en/manage-claude/usage-cost-api
- Codex app-server: https://learn.chatgpt.com/docs/app-server
- Esquema de los archivos de sesión de Codex (comunidad): https://lin-guanguo.github.io/llm-memory-research/agent-cli/codex-session-file-schema/
- OpenAI, Usage API y Costs API: https://developers.openai.com/cookbook/examples/completions_usage_api
- Copilot SDK, uso y facturación: https://docs.github.com/en/copilot/how-tos/copilot-sdk/features/usage-and-billing
- Copilot, facturación por uso: https://github.blog/changelog/2026/04/27/github-copilot-is-moving-to-usage-based-billing/
- Gemini, límites de ritmo: https://aistudio.google.com/docs/rate-limits
- ccusage: https://github.com/ryoppippi/ccusage
- Claude-Code-Usage-Monitor: https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor
- CodexBar: https://github.com/steipete/CodexBar
- Tabla de precios de LiteLLM: https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json
- models.dev: https://github.com/sst/models.dev
- tokencost: https://github.com/AgentOps-AI/tokencost
- OpenRouter, modelos: https://openrouter.ai/docs/api-reference/list-available-models
- LiteLLM, presupuestos: https://docs.litellm.ai/docs/proxy/users
- Compromiso de LiteLLM en PyPI: https://research.jfrog.com/post/litellm-compromised-teampcp/
- Portkey Gateway 2.0: https://portkey.ai/blog/gateway-2-0/
- Compra de Langfuse por ClickHouse: https://clickhouse.com/blog/clickhouse-acquires-langfuse-open-source-llm-observability
- Compra de OpenMeter por Kong: https://konghq.com/blog/news/kong-acquires-openmeter

**Sin verificar:**
- la licencia de CodexBar;
- la fecha del último commit de la tabla de LiteLLM;
- el comando `agy -p "/usage" --json`, que viene de una guía de terceros;
- la compra de Portkey por Palo Alto Networks (una sola fuente);
- el esquema exacto de `rate_limit_event`, que procede de issues y de SDKs de terceros.

Conviene comprobarlo en M0 con capturas reales de cada CLI.
