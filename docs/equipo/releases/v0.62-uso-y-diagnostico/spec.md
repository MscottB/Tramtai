# v0.62: uso y diagnóstico

Tramtai muestra cuánto queda de la cuota de cada suscripción y cuánto dinero gastan las claves de API. Avisa, salta al
respaldo o para cada API al llegar a sus límites. Los alias pueden fijar también el esfuerzo, y «Comprobar conexión» dice
paso a paso por qué una API o una herramienta no responde.

Decisiones del usuario (2026-11-14 y 2026-11-15):
- **Objetivo:** «para mi todas las opciones son igual de importantes»: no pasarse de la cuota, saber cuánto cuesta y ver
  qué app gasta qué.
- **Dónde se ve:** «Página «Uso» + resumen»: una página Uso en la Web UI y en la terminal, y un resumen en Inicio.
- **Al llegar al límite:** las cuatro reacciones, que se combinan por API: avisar, pasar al respaldo, parar y solo
  mostrar.
- **Moneda:** «Dólares + tu moneda»: la cifra exacta en USD y, al lado, su equivalente en la moneda elegida, con un tipo
  de cambio que pone el usuario y su fecha.
- **Diseño:** las partes 1 (la medida), 2 (dónde se ve), 3 (presupuestos), 4 (alias con esfuerzo y «Comprobar
  conexión») y 5 (M0, pruebas, documentación y lo que queda fuera) aprobadas una a una; «Sí, escribe la spec».
- **Pendiente de su permiso:** la M0 usa sus sesiones reales de los CLI. Se le pide permiso explícito antes de empezarla
  (ver «Comprobación con los programas reales (M0)»).

La investigación previa, con fuentes y riesgos, está en [investigacion.md](investigacion.md).

## Por qué

Hoy Tramtai cuenta peticiones, tokens y un coste en dólares, pero:
- no sabe cuánto queda de la cuota de una suscripción (la ventana de 5 h y la semanal de Claude, las de Codex o los
  créditos de Copilot), que es lo que de verdad corta a quien usa su plan;
- el límite en dólares solo suma el `total_cost_usd` que estima Claude Code, también con suscripción, donde no hay dinero
  real; Codex, Copilot, Antigravity y Gemini no suman nada;
- no hay ningún sitio donde ver el uso a lo largo del tiempo ni repartido por API, agente o modelo;
- los avisos del 80 % y del 100 % se guardan, pero nadie los enseña;
- cuando una API falla, el error llega sin decir si es la clave, la sesión, la cuota, el presupuesto o el ritmo del
  proveedor.

## Lo que hay hoy

- **Presupuestos (v0.35):** `nucleo.Presupuesto{Diario, Mensual Limites}` con `Limites{Peticiones, Tokens, USD}`, por API
  (`ConfigPunto.Presupuesto`) y por espacio (claves `presupuesto.*`). Se comprueban en `Almacen.Aceptar` antes de la
  petición y se suman en `CambiarEstado` al terminar, en la tabla `consumo(instancia, punto, dia, …)`. Al agotarse
  responden `presupuesto_agotado` (HTTP 429 con `Retry-After`). El respaldo no lo evita: se rechaza antes de elegir
  agente.
- **Avisos:** la tabla `avisos_presupuesto` recibe los cruces del 80 % y del 100 %, pero nadie la lee. Además se evalúan
  con el período de la hora actual mientras el consumo va al día de aceptación (pendiente anotado en `pendientes.md`).
- **Tokens:** `trabajos.tokens_entrada` ya incluye la caché (Claude suma `cache_creation` y `cache_read`; Copilot y
  Antigravity, la suya). No se sabe qué parte fue caché, que tiene otro precio.
- **Cómo se ejecuta cada agente:**
  - **Claude:** `claude -p --output-format stream-json …`, con suscripción o con clave (`--bare`).
  - **Codex:** `codex exec --json …`, con suscripción o con clave.
  - **Copilot:** su CLI con `--usage-output-file`; siempre con la cuenta de GitHub. `premiumRequests` solo sale como aviso.
  - **Antigravity:** `agy` solo con clave (`GEMINI_API_KEY`).
  - **Gemini:** la API HTTP, con clave.

  Ningún lector mira hoy `rate_limit_event` ni ninguna cuota del proveedor.
- **Alias:** `ConfigPunto.AliasModelos` (`nombre → agente/modelo`), solo en las fachadas OpenAI y Anthropic. No pueden
  fijar esfuerzo. Se editan como texto `nombre=destino` en la Web UI, la terminal y `tramtai acceso --alias`.
- **Esfuerzo:** cinco niveles (`bajo` … `maximo`) traducidos por la ficha de cada agente; el respaldo ya admite
  `agente/modelo+nivel`.
- **Herramientas apuntadas:** `estadoHerramienta` comprueba la anotación, la API, el modelo, el puerto, la orden, la
  clave y el archivo, sin ninguna llamada de red.
- **Superficies:** la Web UI no tiene página Uso; el presupuesto del espacio solo se edita en YAML. La CLI de las APIs es
  `tramtai acceso` (no hay `apis` ni `uso`). El tema de Ayuda Presupuestos dice que el límite en dólares solo cuenta lo
  que informa Claude.

## Qué entra

| Parte | Qué |
|---|---|
| 1. La medida | Cuota de cada suscripción leída del CLI, dinero de las claves con una tabla de precios y conciliación opcional con la factura |
| 2. Dónde se ve | Página Uso (Web UI y terminal), resumen en Inicio, `tramtai uso` y el tema de Ayuda «Uso y gasto» |
| 3. Presupuestos | Dinero por API o espacio (día, semana y mes), cuota por agente y reacciones por API |
| 4. Diagnóstico | Alias que fijan el esfuerzo y «Comprobar conexión» de cada API y herramienta |
| 5. Cierre | M0 con los CLI reales, pruebas, documentación, sitio y release de la app |

## 1. La medida

### 1.1 Tres orígenes

Cada cifra que enseña Tramtai lleva su origen:
- **Del proveedor:** la cuota leída del CLI, con la hora de la lectura («leída hace 3 min»).
- **Estimado por Tramtai:** el dinero calculado con la tabla de precios.
- **Límite propio:** los presupuestos que pone el usuario.

Reglas:
- Un dato que no se conoce sale como «desconocido», nunca como 0. Un fallo de lectura no significa cuota agotada.
- Una lectura del proveedor con más de 15 minutos sale como «última lectura hace X». Pasada la hora de reinicio de esa
  ventana, la lectura se descarta y la ventana queda «desconocido» hasta la siguiente.
- Tramtai nunca ve todo el consumo: la cuota se comparte con claude.ai, la web de ChatGPT, otros equipos y el IDE. La
  Ayuda y la página Uso lo dicen.

### 1.2 La cuota de cada suscripción

| Agente | Cuándo usa suscripción | De dónde sale la cuota |
|---|---|---|
| Claude Code | `autenticacion: suscripcion` | Los eventos `rate_limit_event` de su salida `stream-json`, en cada petición |
| Codex | `autenticacion: suscripcion` | `account/rateLimits/read` de un `codex app-server` de vida corta, lanzado con la misma sesión |
| Copilot | Siempre (cuenta de GitHub) | La vía documentada del SDK de Copilot (`account.getQuota`) si la M0 confirma que se alcanza desde el CLI instalado sin dependencias nuevas; si no, «desconocido» |
| Antigravity | Nunca: en Tramtai va solo con clave | No aplica; cuenta como dinero (precios de Gemini) |
| Gemini | Nunca: es la API | No aplica; cuenta como dinero |

Detalle por agente:
- **Claude:** se guarda el último `rate_limit_event` de cada `rateLimitType` (`five_hour`, `seven_day`, …): `status`
  (`allowed`, `allowed_warning`, `rejected`), `resetsAt`, `utilization` si viene e `isUsingOverage`. El lector es
  tolerante:
  - todos los campos son opcionales; sin `utilization` el porcentaje queda «desconocido», pero el estado y el reinicio
    sí se guardan;
  - `resetsAt` se acepta en segundos o en milisegundos (un valor mayor que 10¹¹ son milisegundos);
  - una línea NDJSON partida o con el evento intercalado se salta sin romper la respuesta.
- **Codex:** el lector admite `primary` o `secondary` nulos (Pro no tiene ventana de 5 h) y varios grupos en
  `rateLimitsByLimitId`. La ventana se nombra por `windowDurationMins` (300 → 5 h; 10080 → semanal; otra → «N min»).
  También guarda `planType` y `credits` si vienen.
- **Copilot:** de cada `quotaSnapshot` se guarda el porcentaje restante, las peticiones usadas, el derecho (`-1` es
  ilimitado) y `resetDate`. Las `premiumRequests` de cada petición siguen quedando, como hoy, en el evento del
  trabajo.

Cuándo se lee:
- Al terminar cada petición, si el CLI lo trae (Claude).
- Con «Actualizar» en la página Uso, en `tramtai uso --actualizar` y al «Comprobar conexión»:
  - Codex y Copilot: una consulta, sin gastar cuota;
  - Claude: no hay consulta sin petición. «Actualizar» enseña la última lectura y ofrece aparte «Leer con una petición
    mínima», que gasta muy poca cuota y solo se hace si el usuario la pide.
- No hay sondeo continuo.

Lo que Tramtai nunca hace: leer las credenciales de los CLI (`~/.claude/.credentials.json` y similares), llamar a
endpoints privados (`api/oauth/usage`, `copilot_internal`, `chatgpt.com/backend-api`) ni leer los archivos de sesión de
los CLI. Solo usa el CLI oficial con la sesión del usuario, como piden los términos de Anthropic, OpenAI y GitHub.

Se guarda la última lectura por agente y ventana en una tabla nueva `cuotas` (global, no por espacio, porque la cuota es
de la cuenta): agente, ventana, porcentaje usado, estado, reinicio, plan, créditos, origen y hora de lectura.

### 1.3 El dinero

- **Solo las peticiones con clave cuestan dinero.** Con suscripción hay cuota, no dinero: el `total_cost_usd` que estima
  Claude con suscripción deja de sumarse al límite en dólares y no aparece como gasto. Copilot no cuesta dinero en
  Tramtai: su consumo son créditos de su plan y se enseña como cuota.
- **Tokens crudos:** cada trabajo guarda, además de lo de hoy, los tokens de caché leída y escrita por separado y el modo
  de autenticación con que se hizo. Las columnas actuales no cambian de significado (`tokens_entrada` sigue incluyendo la
  caché), para no romper el historial ni la API.
- **Uso por día:** una tabla nueva `uso_diario` agrega por espacio, API, agente, modelo, autenticación y día: peticiones,
  errores, saltos al respaldo, tokens de entrada, salida, caché leída y caché escrita, y la suma de los tiempos de
  respuesta. Se conserva como `consumo`: el mes en curso y los 12 anteriores, sin depender de la retención del historial.
- **El precio se calcula al mostrarlo,** desde `uso_diario` y la tabla de precios. Si cambia la tabla, cambian las cifras.
  Para hacer cumplir los presupuestos, al terminar cada petición se suma también su coste con la tabla de ese momento a
  `consumo.usd`.
- **La tabla de precios:**
  - Un JSON embebido con `go:embed`, generado desde `model_prices_and_context_window.json` de LiteLLM (MIT), filtrado a
    los modelos de Anthropic, OpenAI y Gemini. Lleva la fecha de la tabla y la versión de origen (el commit).
  - Por modelo: entrada, salida, caché leída y caché escrita, en USD por millón de tokens, y los recargos de contexto
    largo cuando existan.
  - El script que la genera fija el commit de origen; el diff del JSON se revisa en el PR de cada versión. No se ejecuta
    código ajeno ni se baja nada en tiempo de ejecución.
  - El usuario puede corregir el precio de un modelo en Ajustes; el suyo manda sobre el de la tabla y se marca como
    «precio propio».
  - Un modelo sin precio sale «sin precio» con un aviso, y su coste no cuenta para los presupuestos en dinero (se avisa
    de ello en la API afectada).
  - Si el agente no informa el modelo efectivo (Codex), se usa el modelo pedido o, si no hay, el predeterminado del
    agente según el catálogo; si no se puede saber, «sin precio».

### 1.4 La moneda

- Todo se calcula en USD, que es lo que factura el proveedor.
- En Ajustes se puede elegir una moneda (código ISO 4217, por ejemplo CLP o EUR), su tipo de cambio (unidades por 1 USD)
  y la fecha de ese cambio. Con moneda elegida, cada cifra en dinero sale en USD y al lado en esa moneda, con la fecha
  del cambio a la vista.
- Sin moneda elegida, solo USD.
- Los presupuestos en dinero se fijan en USD y en pantalla se ven también convertidos.

### 1.5 Conciliación con la factura (opcional)

- Con una clave de administrador de Anthropic (`sk-ant-admin01-…`) u OpenAI, guardada en el almacén del sistema como el
  resto de claves, Tramtai consulta su API de costes (Anthropic `cost_report`, OpenAI `organization/costs`) como mucho
  una vez por hora.
- La página Uso enseña «facturado» al lado de «estimado» por día y mes, con el desfase de unos minutos que tienen esas
  API. Es de la organización entera, no por API.
- Si la consulta falla (404, sin datos, clave no válida), se dice y se sigue enseñando el estimado.

## 2. Dónde se ve

### 2.1 Página «Uso» (Web UI)

Nueva página `uso` en el grupo Conectar del menú, junto a Mis APIs e Historial:
- **Arriba, la cuota de las suscripciones:** una tarjeta por agente con suscripción. Cada ventana lleva una barra con el
  porcentaje, la cuenta atrás hasta el reinicio y el origen («del proveedor, leída hace 3 min»). Se marcan los umbrales
  del agente y el estado `allowed_warning` o `rejected`. Botón «Actualizar» (y, en Claude, «Leer con una petición
  mínima»).
- **Debajo, el dinero de las claves:** hoy, esta semana y este mes, en USD y en la moneda elegida, y el facturado al lado
  si la conciliación está activa. La fecha de la tabla de precios a la vista.
- **Tendencias:** gráficas por día o por semana de peticiones, tokens, dinero, errores, saltos al respaldo y tiempo medio
  de respuesta. Filtros por API, agente, modelo y espacio; período de 7, 30 o 90 días.
- **«Quién gasta qué»:** una tabla con cada API (y las peticiones sin API) con peticiones, tokens, dinero y su
  presupuesto. Se ordena por cualquier columna y cada fila lleva a la API en Mis APIs.
- Los avisos activos salen arriba de la página.

### 2.2 Inicio

Un resumen pequeño con un enlace a Uso:
- la cuota más apurada («Claude: 72 % de la ventana de 5 h, se reinicia en 1 h 20 min»);
- el dinero del mes;
- los avisos activos.

### 2.3 Terminal

Una página Uso nueva con lo mismo en texto: las cuotas en barras, las tendencias en minigráficas de una línea y la tabla
«quién gasta qué». El resumen de Inicio también aparece en la terminal.

### 2.4 CLI

- `tramtai uso` (alias `usage`): cuotas, dinero del día, la semana y el mes, y la tabla «quién gasta qué».
- Opciones: `--desde`/`--hasta` o `--periodo 7d|30d|90d`, `--api`, `--agente`, `--modelo`, `--instancia`,
  `--actualizar` (lee la cuota de Codex y Copilot antes de mostrarla) y `--json` para scripts.

### 2.5 API del servicio

Rutas nuevas bajo `/api/v1/servicio/`, con la misma autenticación que las de hoy, en el OpenAPI y en el cliente Go:
- `GET uso`: totales, series y la tabla, con los filtros de 2.1 y 2.4;
- `GET uso/cuotas` y `POST uso/cuotas/actualizar` (con `peticion_minima: true` para Claude);
- `GET uso/avisos`;
- `GET uso/precios` (la tabla con su fecha y los precios propios; los propios se guardan en la clave global
  `uso.precios_propios`, que se edita en Ajustes);
- `POST puntos/{nombre}/comprobar` (ver 4.2).

Las herramientas apuntadas se comprueban con `POST /api/v1/herramientas/{id}/comprobar`, junto a las rutas de
herramientas que ya existen (no disponibles en el modo servidor, como hoy).

## 3. Presupuestos

### 3.1 Tres clases de límite

- **Dinero:** el campo `USD` de `Limites`, que ya existe, pasa a contar el coste con la tabla de precios y solo de las
  peticiones con clave.
- **Peticiones y tokens:** siguen como hoy.
- **Umbral de aviso de los presupuestos:** por API y por espacio, en porcentaje del límite (por defecto 80). El aviso del
  100 % sale siempre.
- **Período semanal:** `Presupuesto` gana `Semanal` junto a `Diario` y `Mensual`, para las tres unidades. La semana va de
  lunes a domingo en la hora local del espacio, como el día y el mes.
- **Cuota de la suscripción (nuevo):** se fija **por agente**, porque la cuota es de la cuenta y no de una API. Para cada
  agente y cada clase de ventana (`5h`, `semanal` y `otras`) hay dos umbrales en porcentaje del proveedor: **aviso**
  (por defecto 80) y **límite** (por defecto 100). Se miden sobre la última lectura y su hora de reinicio.
- El presupuesto del espacio se puede editar en la Web UI y la terminal, donde ya se ven sus barras, con el mismo editor
  que el de cada API. Los umbrales de cuota de cada agente se editan desde su tarjeta en Uso y desde Agentes.

### 3.2 Qué hace cada API al llegar a un límite

Se elige por API y se combinan:
- **Avisar:** aviso en la página Uso, en Inicio y en la terminal, al cruzar el umbral de aviso y el límite.
- **Pasar al respaldo (solo cuota):** si la última lectura del agente principal está en su límite o en `rejected`, la
  petición empieza directamente por el primer respaldo cuyo agente no esté también en su límite. Si no queda ninguno, se
  aplica «Parar» si está elegida; si no, se intenta con el principal. Sin respaldo configurado, se avisa en la API.
- **Parar:** la API rechaza las peticiones nuevas con un error claro y la hora en que vuelve a estar disponible:
  - `presupuesto_agotado` (HTTP 429, el de hoy) para dinero, peticiones y tokens;
  - `cuota_agotada` (HTTP 429, nuevo) para la cuota de la suscripción, con `Retry-After` hasta el reinicio del
    proveedor.
- **Solo mostrar:** sin avisos ni cambios; el dato se ve en Uso.

Las reacciones se eligen por separado para los presupuestos y para la cuota, en el formulario de la API (Web UI y
terminal) y con `tramtai acceso crear|editar --al-limite presupuesto=avisar,parar --al-limite cuota=respaldo,parar`.
«Solo mostrar» es elegir ninguna. Los avisos de la cuota son del agente, no de cada API (ver Ruling 9).

Valores por defecto:
- **Presupuestos propios (dinero, peticiones y tokens):** avisar y parar, como hasta hoy. Un presupuesto que ya tenga
  alguien sigue parando igual.
- **Cuota de la suscripción:** solo avisar.

Los umbrales de cuota de cada agente se editan también con `tramtai agentes cuota <agente> --aviso 5h=70
--limite semanal=90`.

### 3.3 Cómo se aplica

- **Antes de la petición:** como hoy, en la transacción de `Aceptar`, con lo gastado más la reserva de los trabajos en
  curso. La cuota se mira con la última lectura del proveedor.
- **Al terminar:** se suma lo real (`consumo` y `uso_diario`) y se guarda la lectura de cuota que traiga la respuesta.
- **Si el proveedor corta de verdad** aunque Tramtai creyera que quedaba (`rate_limit_event` con `rejected`, o el error
  de cuota del CLI), se guarda esa lectura y se trata como agotada: el error del intento es `cuota_agotada`, que salta al
  respaldo como hoy lo hace `limite_uso_proveedor`; sin respaldo, la app recibe `cuota_agotada` con la hora de reinicio.
  `limite_uso_proveedor` queda para los límites de ritmo y los casos que no se pueden distinguir.
- **Tope nativo de Claude:** cuando una petición de Claude va con clave y su API o su espacio tienen límite en dinero, se
  pasa `--max-budget-usd` con lo que queda del más estricto.
- **Avisos:** uno por umbral, período y API (o agente en la cuota). Se evalúan con el período de aceptación del trabajo,
  que es donde se suma su consumo, y así se corrige el pendiente anotado. Se reutiliza la tabla `avisos_presupuesto`,
  que gana una clase (`presupuesto` o `cuota`) y se lee por fin desde la API.

## 4. Diagnóstico

### 4.1 Alias que fijan el esfuerzo

- El destino de un alias admite `+nivel` como el respaldo: `rapido=claude/haiku+bajo`, `pensar=claude/sonnet+alto`.
- Sin `+nivel`, el alias se comporta como hoy.
- Con `+nivel`, manda el alias aunque la app pida otro esfuerzo, como ya pasa con el modelo forzado. La respuesta de las
  fachadas lleva la cabecera `Tramtai-Esfuerzo` con el nivel usado, y si la app pidió otro, `Tramtai-Advertencia` lo dice.
- Al guardar, el nivel se valida contra el catálogo de ese agente y modelo (`NivelesDeModelo`); un modelo sin esfuerzo no
  admite `+nivel`.
- **Web UI:** el editor de alias pasa de un área de texto a filas con nombre, destino y un selector de esfuerzo que solo
  ofrece los niveles de ese agente y modelo (y no sale si no admite esfuerzo).
- **Terminal y CLI:** la misma sintaxis `nombre=agente/modelo+nivel`, con el error de validación si el nivel no vale.
- Como hoy, los alias solo se aplican en las fachadas OpenAI y Anthropic.

### 4.2 «Comprobar conexión»

Dónde:
- un botón en cada API de Mis APIs y en cada herramienta apuntada (Web UI y terminal);
- `tramtai acceso comprobar <api>` y `tramtai herramientas comprobar <herramienta>`, con `--real` y `--json`.

Qué revisa, en orden, marcando cada paso bien, mal o no aplica:
1. el servicio responde;
2. la API existe, su URL y su contrato (OpenAI o Anthropic) están habilitados;
3. la clave de la API es válida;
4. la API está activa: no está en pausa y tiene presupuesto;
5. el modelo o el alias se resuelve y está permitido;
6. el agente está instalado y tiene sesión o clave;
7. le queda cuota de la suscripción (con la lectura disponible, o una consulta en Codex y Copilot).

Prueba real, opcional: al final se ofrece «Probar con una petición real», una pregunta mínima que gasta muy poca cuota.
Solo se hace si se pide (`--real` en la CLI).

Si algo falla, dice exactamente qué y cómo arreglarlo, con el enlace a la pantalla o la orden que lo arregla. Distingue:
- límite propio de Tramtai (`presupuesto_agotado`, con el límite y el reinicio);
- límite de ritmo del proveedor, con «reintenta en X s»;
- cuota de la suscripción agotada, con la hora de reinicio;
- tope de gasto del proveedor alcanzado (en Anthropic, `enforced_spend_limit_reached`);
- sin sesión o sesión caducada.

Para las herramientas apuntadas comprueba además, con lo que ya revisa `estadoHerramienta`, que su archivo sigue
apuntando a Tramtai con su API y su clave, y después recorre los pasos de su API.

## API: errores y cabeceras

- **`cuota_agotada`** (nuevo, HTTP 429): la cuota de la suscripción del agente está en su límite o el proveedor la
  rechazó. Lleva `Retry-After` hasta el reinicio y el mensaje dice el agente, la ventana y la hora. En OpenAI va como
  `rate_limit_error` con `code: "cuota_agotada"`; en Anthropic, como `rate_limit_error`. Es saltable al respaldo.
- **`presupuesto_agotado`:** igual que hoy; el mensaje distingue dinero, peticiones y tokens.
- **`Tramtai-Esfuerzo`** (nueva cabecera de respuesta en las fachadas): el nivel de esfuerzo usado.
- Todo se documenta en `docs/api.md`, en la guía de desarrolladores, en el OpenAPI y en la lista de errores de la Ayuda.

## Comprobación con los programas reales (M0)

Leer la cuota exige una sesión iniciada, así que la M0 usa **las sesiones reales del usuario**. Antes de empezarla se le
pide permiso explícito; sin él, la M0 no se hace y los lectores se escriben con los formatos de la documentación y la
investigación, marcados como no comprobados.

Hace solo esto:
- **Claude:** dos o tres peticiones mínimas («responde OK») con `claude -p --output-format stream-json --verbose` para
  ver los `rate_limit_event` (con y sin `utilization`, la unidad de `resetsAt`). Comprobar que la versión instalada admite
  `--max-budget-usd` y qué salida da al pasarse.
- **Codex:** lanzar `codex app-server`, hacer el saludo y `account/rateLimits/read`, y cerrarlo. Ver también si
  `codex exec --json` trae algo de límites.
- **Copilot:** confirmar si `account.getQuota` se alcanza desde el CLI instalado por una vía documentada y sin
  dependencias nuevas; guardar un `--usage-output-file` y el `premiumRequests` de una petición mínima.
- **Antigravity y Gemini:** nada (solo clave).

Reglas:
- No se leen credenciales ni se cambia ningún ajuste de los CLI. Se calculan las huellas de sus archivos de sesión y
  configuración antes y después, como en la v0.60, y deben coincidir.
- No se toca el Tramtai instalado del usuario ni su puerto 17373.
- Las salidas se guardan en `m0.md` y, sin nada personal (cuentas, ids, correos), como datos de prueba en `testdata`.
  Desde ahí, las pruebas automáticas nunca vuelven a tocar los CLI.
- Lo que la M0 no confirme queda como «desconocido» en esa versión y se dice en la Ayuda.

## Pruebas

- **Lectores de cuota** contra las salidas reales de la M0 y los casos raros: campos que faltan, `resetsAt` en segundos y
  milisegundos, líneas partidas, `primary`/`secondary` nulos, derechos ilimitados, lecturas viejas y pasadas de reinicio.
- **Dinero:** el cálculo con la tabla, la caché leída y escrita, los precios propios, los modelos sin precio, la moneda y
  que la suscripción no sume dinero.
- **Presupuestos:** el período semanal, los umbrales de cuota, las cuatro reacciones y sus combinaciones, el respaldo por
  cuota, `cuota_agotada`, `--max-budget-usd`, los avisos una sola vez por período (también con trabajos que terminan
  pasada la medianoche) y las carreras de peticiones simultáneas con `-race`.
- **Migraciones:** las columnas y tablas nuevas sobre una base de la v0.61 con datos; los presupuestos existentes siguen
  parando igual.
- **Alias con esfuerzo:** sintaxis, validación por agente y modelo, la prioridad sobre la petición y las cabeceras.
- **«Comprobar conexión»:** cada paso bien y mal, cada clase de error y la prueba real con agentes simulados.
- **Tabla de precios:** el filtro del script y una prueba que falla si la tabla tiene más de 60 días al publicar.
- **Superficies:** la página Uso y el resumen de Inicio en la Web UI y en la terminal, a 375 y 1440 px, en claro y oscuro,
  en español e inglés; `tramtai uso --json` con su forma estable.
- Ninguna prueba toca los CLI ni las sesiones reales del usuario.

## Documentación, Ayuda y sitio

- **Ayuda:** tema nuevo «Uso y gasto» (los tres orígenes, la cuota, el dinero, la moneda, la conciliación y por qué
  Tramtai no ve todo el consumo), al estilo de la v0.52. Al día: Presupuestos (dinero con la tabla, semana, cuota por
  agente, reacciones), Mis APIs (alias con esfuerzo, «Comprobar conexión»), herramientas apuntadas y Problemas
  (`cuota_agotada`). Capturas nuevas en es y en, y la terminal regenerada desde los mismos textos.
- **Guía de desarrolladores y `docs/api.md`:** `cuota_agotada`, `Tramtai-Esfuerzo`, el período semanal, las reacciones y
  las rutas de uso.
- **README:** la página Uso y «Comprobar conexión».
- **Sitio:** Producto y la Ayuda mencionan la página Uso; se publica con el release.

## Publicación

Esta versión cambia la app: lleva su release con instaladores y notas en es y en, la tabla de precios al día y el sitio
publicado con él.

## Fuera de alcance

- Webhooks y avisos por correo o del sistema operativo (tienen su pendiente).
- Leer la cuota de claude.ai, de otros equipos o del IDE, que Tramtai no puede ver.
- Presupuestos por usuario del modo servidor.
- La conciliación con la factura de Google.
- Contrastar la tabla de precios con una segunda fuente (models.dev).
- Una statusline de Claude Code que alimente a Tramtai.
- Alias en la API nativa (siguen solo en las fachadas).

## Criterios de aceptación

- Con Claude por suscripción, tras una petición la página Uso enseña sus ventanas con porcentaje (o «desconocido»),
  reinicio y hora de lectura; con Codex por suscripción, «Actualizar» las trae sin gastar cuota.
- Una petición con clave enseña su coste con la tabla y su fecha; una con suscripción no suma dinero.
- Un presupuesto en dinero de una API para al llegar al 100 % y avisa al 80 %, una sola vez por período.
- Con «Pasar al respaldo» y el agente principal en su límite, la petición la responde el respaldo; con «Parar», la app
  recibe `cuota_agotada` y la hora de reinicio.
- Un alias `rapido=claude/haiku+bajo` responde con `Tramtai-Esfuerzo: bajo` aunque la app pida `high`.
- «Comprobar conexión» de una API con la clave mal dice que la clave no es válida y enlaza a dónde cambiarla.
- `tramtai uso --json` devuelve las cuotas, el dinero y la tabla.
- La M0 deja las huellas de los archivos de los CLI igual antes y después.

## Decisiones tomadas al revisar el código

Ajustes al diseño aprobado, por cómo está hecho Tramtai hoy:
- **Ruling 1:** la orden de las APIs es `tramtai acceso`, no `apis`; «Comprobar conexión» es `tramtai acceso comprobar`.
- **Ruling 2:** Antigravity solo funciona con clave en Tramtai, así que cuenta como dinero (precios de Gemini) y su
  `/usage` no se lee.
- **Ruling 3:** Copilot se ejecuta por su CLI, no por el SDK; su cuota se lee por la vía del SDK solo si la M0 confirma
  que es posible sin dependencias nuevas.
- **Ruling 4:** Codex se ejecuta con `exec`; la cuota se lee aparte con un `app-server` de vida corta.
- **Ruling 5:** el límite en dólares ya existía, pero sumaba el coste estimado de Claude también con suscripción; pasa a
  contar solo las peticiones con clave, con la tabla de precios.
- **Ruling 6:** «por defecto no para» se aplica a la cuota; los presupuestos propios siguen parando por defecto, para no
  cambiar el comportamiento de quien ya los tiene.
- **Ruling 7:** «Pasar al respaldo» solo tiene sentido con la cuota de un agente: un presupuesto de la API se agota igual
  con cualquier agente.
- **Ruling 8:** el presupuesto del espacio, que hoy solo se edita en YAML, pasa a editarse en la Web UI y la terminal.

Al escribir el plan:
- **Ruling 9:** los avisos de la cuota son por agente, con su umbral de aviso (0 los apaga), porque la cuota es de la
  cuenta y no de una API; por API se eligen «Pasar al respaldo» y «Parar».
- **Ruling 10:** Tramtai solo guarda la huella de la clave de cada API, así que «Comprobar conexión» admite escribir la
  clave que usa la app para comprobarla. Sin ella, solo comprueba que la API tiene clave.
- **Ruling 11:** los precios propios se guardan en la clave global `uso.precios_propios`, editada en Ajustes, sin
  rutas propias.
- **Ruling 12:** el coste estimado se guarda en `trabajos.costo_usd` y suma a `consumo.usd` al terminar, calculado por
  el planificador con la tabla; el `total_cost_usd` que informa Claude deja de usarse.
