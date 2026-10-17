# Seguridad y límites: que nada malicioso use a Tramtai contra los CLI y que lo que no cabe no llegue al CLI

Diseño acordado el 2026-10-17 (v0.41). El usuario pidió dos cosas:

1. **Seguridad:** que un contenido malintencionado que llega desde una app (mensaje, instrucciones, documentos,
   esquema) no pueda usar a Tramtai para atacar a los CLI o al equipo. Lo que preocupa es la interacción de Tramtai con
   los CLI: que un problema no venga de Tramtai. La configuración propia de cada CLI (por ejemplo los servidores MCP de
   `~/.codex` con la suscripción) es cosa de ese CLI y del usuario: se documenta, no se cambia.
2. **Límites:** que tras convertir los documentos Tramtai mida la entrada y, si no cabe en el modelo, la rechace él (o
   pase a un respaldo que sí la admita, avisando), en lugar de gastar una ejecución que el CLI iba a rechazar.

Parte de una auditoría de solo lectura de todo el camino del contenido (adaptadores, cargas, conversión, validación,
salida y secretos). Nada crítico; los riesgos y su arreglo se listan en la sección 3.

Decisiones del usuario: Antigravity queda bloqueado para las APIs y apps hasta validarlo con el CLI real; si la entrada
no cabe en el principal, se salta al respaldo que la admita con un aviso claro; las pruebas canario con los CLI reales
se corren solo con modelos pequeños (Claude haiku y sonnet, el más pequeño de Codex con esfuerzo bajo).

Fuera de alcance: cambiar la configuración del usuario de cada CLI (MCP, hooks, reglas) y validar Antigravity con el CLI
real (necesita la clave del usuario; queda bloqueado para apps mientras tanto).

## 1. Límites antes de llamar al CLI

### Medir la entrada

- Al terminar la etapa de preparación (o al empezar, si no hay documentos), el planificador arma el texto que recibirá
  el agente (instrucciones, mensajes y documentos convertidos, como hace hoy `conDocumentos`) y lo mide: caracteres y
  una estimación de tokens = caracteres ÷ 4, redondeado hacia arriba. Es una estimación sin el tokenizador de cada
  proveedor: solo se rechaza lo que claramente no cabe.
- Se guarda un evento `entrada` (sin contenido) con `{"caracteres", "tokens_estimados", "documentos": [{"nombre",
  "tokens_estimados"}]}`, ordenado de mayor a menor; la purga lo conserva como `estado` e `intento`.

### Contexto de cada modelo

En este orden:

1. **Configurado:** `limites.contexto_tokens`, un mapa `agente` o `agente/modelo` → tokens (por ejemplo
   `claude/claude-haiku-4-5: 200000`). Manda sobre lo demás.
2. **Informado por el CLI o la API:**
   - Claude Code: `modelUsage.<modelo>.contextWindow` del resultado.
   - La API de Gemini: `inputTokenLimit` de `GET /v1beta/models/{modelo}`, guardado 24 h en memoria.
   - Codex: si su salida `--json` lo informa (se comprueba en los canarios); si no, solo configurado.
   - Se guarda por modelo concreto en una tabla nueva `modelos_vistos(agente, modelo, contexto_tokens, visto_en)`
     (migración v9). Un alias se resuelve con la última versión vista, como el catálogo.
3. **Desconocido:** no se comprueba; decide el CLI, como hoy.

- Reserva para la respuesta: `limites.reserva_salida_tokens` (8.192 por defecto). Cabe si
  `tokens_estimados + reserva <= contexto`.
- Antigravity, además, no admite más de 32.767 unidades UTF-16 en su línea de órdenes de Windows: se cuenta el prompt ya
  escapado (comillas y barras dobladas, caracteres fuera del BMP dobles), no las runas, y se compara con 28.000 para
  dejar margen.

### Si no cabe

- Se prueba con el siguiente candidato del respaldo, sin ejecutar el que no cabe, y queda un `aviso`: «La entrada
  (~45.000 tokens; informe.pdf ~38.000) no cabe en claude/haiku (contexto 32.000); sigue con codex, que sí la admite».
- Si ningún candidato la admite, el trabajo falla con el código nuevo `entrada_demasiado_grande` y un mensaje con el
  tamaño, el límite del último candidato y el documento que más pesa. En las fachadas: OpenAI 400
  `context_length_exceeded`; Anthropic 400 `invalid_request_error`; `/api/v1/ejecutar` 413.
- No consume cuota ni presupuesto: no llegó a ejecutarse.

### Dónde se ven los límites

- **Probar**, bajo «Adjuntar archivos»: «Hasta 20 archivos de 50 MB; convertidos a texto, hasta 20 MB. Los archivos
  ocupan contexto del modelo: si no caben en el principal, se prueba con el respaldo» (con los valores reales de la
  configuración).
- **Registro** (Probar e Historial): una línea «Entrada: ~12.300 tokens (informe.pdf ~9.800) · contexto de
  claude/haiku: 200.000».
- **Agentes**: el contexto de cada modelo cuando se conoce, y de dónde sale (configurado o informado).
- **`GET /api/v1/capacidades`**: un bloque `limites` con cargas por petición, tamaño por archivo, texto convertido,
  cuerpo de la petición, esquema, mensajes y la reserva de salida.

## 2. Antigravity, bloqueado para apps

- Una petición de una API o una app (origen `app` o `api`) con Antigravity como principal o en el respaldo se rechaza
  al aceptarla con `agente_no_disponible`: «Antigravity no está habilitado para aplicaciones: todavía no se ha
  comprobado con el CLI real que no pueda usar herramientas. Úsalo desde Probar o la terminal». Probar, la terminal y la
  CLI lo siguen usando.
- Mis APIs no ofrece Antigravity como agente ni como respaldo y explica por qué; una API guardada con él se marca con
  error en su detalle.
- Clave `adaptadores.antigravity.permitir_apps` (falso; administrada: una configuración de proyecto no la cambia) para
  habilitarlo cuando pase su canario.
- Endurecimiento que depende de Tramtai, se use desde donde se use:
  - el prompt nunca empieza por `-` (siempre va enmarcado), para que no se lea como una opción del CLI;
  - `APPDATA`, `LOCALAPPDATA` y `XDG_*` apuntan a su carpeta privada, como ya `HOME` y `USERPROFILE`;
  - rechaza `consciente_proyecto`, como Gemini;
  - el tope de la línea de órdenes de la sección 1.

## 3. Endurecimiento de Tramtai

| # | Riesgo | Arreglo |
|---|---|---|
| M2 | Una app activa `consciente_proyecto` aunque su API sea `aislado`, y un perfil puede ampliar el modo | En un punto de acceso manda su `modo_contexto` (`aislado` si no tiene); una petición o un perfil que pida otro se rechaza con `contexto_no_permitido` |
| M3 | Un Office o PDF hostil agota la memoria al convertirlo | Tope de memoria por proceso hijo (Job Object en Windows: `JOB_OBJECT_LIMIT_PROCESS_MEMORY`; `RLIMIT_AS` en Linux y macOS), `limites.memoria_proceso_mb` (2048); antes de convertir un `.docx`, `.xlsx` o `.pptx`, Tramtai suma los tamaños descomprimidos de su ZIP contra `conversion.maximo_descomprimido_mb` |
| M4 | Una línea enorme en la salida de un proceso crece sin tope en memoria | El búfer de línea se corta en el tope de salida (u 8 MB) y el proceso falla con un error claro |
| M5 | La conversión no tiene plazo total y ocupa el único hueco de conversión | Plazo total de preparación: `conversion.tiempo_total_segundos` (600) |
| M6 | Las cargas pueden llenar el disco | Cuota total por instancia `conversion.cuota_cargas_mb` (2048) comprobada antes de guardar (507 `cuota_cargas_agotada`) |
| M7 | Una configuración de proyecto desactiva los límites de ZIP y conversión (0 = sin límite) | Un valor ≤ 0 usa el predeterminado y cada límite tiene un máximo; los de `conversion.*` pasan a administrados |
| L1 | Un documento imita los delimitadores `<<<turno …>>>` o `<<<fin archivo>>>` | Los delimitadores llevan un código aleatorio por petición |
| L2/L9 | Esquemas de petición sin tope; el de Claude va por la línea de órdenes | Tope de 256 KB en `Validar`; se compila una vez |
| L4 | El filtro de variables de entorno deja pasar `*_KEY`, `*_PASS`, `*_PWD`, `*_CLAVE`, `PRIVATE`, URL con usuario y clave, `SSH_AUTH_SOCK`; el conversor no quita `TRAMTAI_*` ni la variable del token | Se amplía la lista (no una lista blanca: rompería los CLI que usan proxies u otras variables) y el conversor filtra igual que los agentes |
| L5 | Los secretos solo se tapan en los errores | También en la salida final y en los eventos `texto`, con las claves de proveedor cargadas |
| L6 | Un mensaje de error del CLI puede falsificar líneas del registro del servicio | Se escapan saltos de línea y se recorta |
| L7 | El conversor acepta formatos que `/capacidades` no anuncia | Solo los anunciados; el resto, `carga_invalida`; la documentación indica instalar markitdown con versiones fijas |
| L8 | Las cargas se comparten entre las APIs de una instancia y sus metadatos se sobrescriben | El identificador incluye la API que la subió y los metadatos no se sobrescriben |
| L10 | Sin tope de número de mensajes | Máximo de 1.000 mensajes por petición; el tamaño ya lo limita el cuerpo (16 MB) y ahora el contexto |

Documentación, sin cambio de código: con la suscripción, Codex y Claude usan la configuración del usuario (MCP, hooks,
reglas) en lo que sus CLI no permiten aislar; en `consciente_proyecto` Claude carga los hooks del proyecto, por eso solo
vale en `proyectos_confiables`.

## 4. Pruebas canario con los CLI reales

- `pruebas/canario` (con la etiqueta de compilación `canario`; no corre en CI): por cada CLI pide, con un mensaje
  hostil, (a) leer un archivo trampa junto a la carpeta de trabajo, (b) ejecutar `echo` con una marca, (c) abrir una URL
  local que registra si la llaman. Comprueba que la respuesta no contiene el contenido trampa ni la marca y que la URL no
  recibió nada.
- Solo modelos pequeños: Claude `haiku` y `sonnet`; Codex el modelo más pequeño del catálogo con esfuerzo bajo.
  Antigravity no se prueba (bloqueado para apps).
- De paso confirman qué informa cada CLI sobre el contexto (sección 1).

## Documentación

- Ayuda: «¿Por qué se rechazó mi petición por tamaño?», «¿Por qué no puedo usar Antigravity en mis APIs?».
- `docs/api.md`: `entrada_demasiado_grande`, `cuota_cargas_agotada`, `contexto_no_permitido`, el evento `entrada` y el
  bloque `limites` de `/capacidades`.
- `docs/operacion.md`: límites y claves nuevas (`limites.*`, `conversion.tiempo_total_segundos`,
  `conversion.cuota_cargas_mb`, `adaptadores.antigravity.permitir_apps`) y la nota sobre la configuración de cada CLI.
- `docs/arquitectura.md`: el modelo de amenazas en una sección corta.
- `docs/equipo/pendientes.md`: fila del release; el riesgo abierto de Antigravity pasa a «bloqueado para apps».

## Pruebas

- `internal/planificador`:
  - sin respaldo, una entrada que no cabe falla con `entrada_demasiado_grande` y no llama al adaptador;
  - con respaldo, salta al que cabe con su aviso;
  - con el contexto desconocido no se comprueba;
  - el evento `entrada` con los documentos ordenados.
- `internal/adaptadores`:
  - Claude guarda el `contextWindow` informado;
  - Antigravity enmarca un mensaje que empieza por `-`, redirige `APPDATA`, rechaza `consciente_proyecto` y cuenta
    UTF-16 escapado;
  - la redacción de secretos en la salida.
- `internal/api`:
  - una API rechaza Antigravity para apps;
  - una API `aislado` rechaza `consciente_proyecto` y un perfil que lo pida;
  - el bloque `limites`;
  - los códigos en las fachadas.
- `internal/documentos`:
  - cuota de cargas;
  - formatos no anunciados rechazados;
  - OOXML con descomprimido excesivo rechazado antes de markitdown;
  - plazo total;
  - límites ≤ 0 usan el predeterminado;
  - delimitadores con código aleatorio.
- `internal/procesos`: el búfer de línea con tope; el tope de memoria (Windows y Unix).
- `internal/nucleo`: tope de esquema y de mensajes; filtro de entorno ampliado.
- Canarios con los CLI reales (sección 4), con el permiso ya dado.
- Navegador: el aviso de salto por tamaño, el error sin respaldo, la línea «Entrada» del registro, la ayuda bajo
  Adjuntar, Agentes con el contexto, Mis APIs sin Antigravity.
