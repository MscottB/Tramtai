# API propia `/api/v1` explicada: ¿qué URL uso?, ejemplos, «Ver como código» y OpenAPI

Diseño acordado el 2026-10-17 (v0.42). El usuario no veía en qué se diferencia la API propia de Tramtai de las
compatibles con OpenAI y Anthropic, ni cómo conectarse a ella. Hoy Mis APIs solo dice «API propia de Tramtai» con una
línea, los ejemplos para copiar son de OpenAI y Anthropic, y `docs/api.md` es una tabla de rutas.

Decisiones del usuario: Probar tiene «Ver como código»; se publica una descripción OpenAPI con una prueba que la
mantiene al día.

Fuera de alcance: cambiar el comportamiento de la API; SDK propios de Tramtai.

## 1. Guía «¿Qué URL uso?»

En Mis APIs, en la API principal y en el detalle de cada API (bloque Conectar), sobre las URL:

- Tres tarjetas, una por contrato que la API acepte:
  - **OpenAI** (`/openai/v1`): tu app ya usa el SDK de OpenAI o una herramienta que deja cambiar la «URL base» (por
    ejemplo LangChain, n8n o un editor con IA). Cambias la URL y la clave; el resto sigue igual.
  - **Anthropic** (`/anthropic`): tu app usa el SDK de Anthropic.
  - **API propia** (`/api/v1`): necesitas lo que las otras no tienen: enviar archivos (PDF, Word, Excel, ZIP…) que
    Tramtai convierte, tareas largas que sigues en vivo y retomas si se corta la conexión, cancelar y reintentar,
    elegir respaldo y esfuerzo en cada petición, idempotencia, el registro de cada paso y los límites.
- Debajo, plegada, una tabla «Qué permite cada una»:

  | | OpenAI | Anthropic | API propia |
  |---|---|---|---|
  | SDK oficial y herramientas existentes | sí | sí | no (HTTP y JSON) |
  | Respuesta en vivo | sí | sí | sí, y se retoma tras un corte |
  | Respuesta JSON con esquema | sí (`response_format`) | no | sí (`salida.esquema`) |
  | Enviar archivos | no | no | sí |
  | Tareas largas: seguir, cancelar, reintentar | no | no | sí |
  | Respaldo y esfuerzo por petición | esfuerzo | no | sí |
  | Registro de cada paso y límites | no | no | sí |

- Los textos van en `es.ts` y `en.ts`; la tabla sale de los mismos datos en los dos idiomas.

## 2. Ejemplos de la API propia

- Los ejemplos para copiar pasan de `Accesos.vue` a un módulo `web/src/ejemplos.ts` (una sola función con la URL, la
  clave, el modelo, los contratos y, opcionalmente, una receta y un mensaje), que usan Mis APIs y Probar.
- Con el contrato `nativa`, tres pestañas nuevas: **curl · API propia**, **Python · API propia** (con `requests`) y
  **Node · API propia** (con `fetch`). Cada una muestra el flujo completo:
  1. subir un archivo (`POST /cargas`, multipart) y quedarse con `identificador_carga`;
  2. crear el trabajo (`POST /trabajos`) con el modelo, el mensaje, la carga, `Idempotency-Key` y un esquema de
     ejemplo;
  3. seguirlo en vivo (`GET /trabajos/{id}/eventos`, SSE) imprimiendo el texto;
  4. leer el resultado (`GET /trabajos/{id}/salida`).

  Cada paso lleva un comentario de una línea; el de curl usa `jq` solo si hace falta extraer un campo, y lo dice.

## 3. «Ver como código» en Probar

- Un botón en la tarjeta de la receta abre un diálogo con la prueba actual como código: el mensaje, la receta
  (instrucciones, modelo, esfuerzo, respaldo, esquema) y, si hay, los archivos adjuntos (por su nombre).
- Pestañas: API propia (curl, Python, Node) y OpenAI (Python, curl).
  - En OpenAI se usa lo que ese contrato admite: instrucciones como mensaje de sistema, `reasoning_effort` y
    `response_format` con el esquema. Si la prueba tiene archivos o respaldo, una nota dice que solo los admite la
    API propia.
  - La URL es la de la API que se está editando (si se abrió desde Mis APIs) o la de la API principal. La clave va
    como marcador `<tu clave>`: Probar nunca muestra una clave.
- Se genera con el mismo módulo de ejemplos.

## 4. OpenAPI

- `internal/api/openapi.json` (OpenAPI 3.1), escrito a mano e incluido en el ejecutable, describe todas las rutas de
  `/api/v1`, sus cuerpos (`SolicitudEjecucion`, el trabajo, la carga, los límites, el catálogo), la seguridad
  (`Authorization: Bearer` y `x-api-key`) y el formato de error.
- `GET /api/v1/openapi.json` lo devuelve con los mismos permisos que el resto de `/api/v1`, cambiando `servers` por la
  URL base real de la petición (la de la instancia o la de la API, `/p/<nombre>/api/v1` o su puerto).
- Mis APIs, en el detalle de cada API con el contrato `nativa`: botón **Descargar OpenAPI** (lo pide con la sesión y
  lo guarda como `tramtai-<api>-openapi.json`).
- Prueba `TestOpenAPICubreLasRutas`: compara las rutas `/api/v1` que registra `api.go` (leyendo sus `HandleFunc`) con
  las de `openapi.json`, en los dos sentidos, y comprueba que el JSON es válido y que `servers` se reescribe.

## Documentación

- `docs/api.md`: sección «¿Qué URL uso?» (la tabla de la sección 1) y una guía paso a paso de la API propia: subir,
  crear, seguir (con reconexión `Last-Event-ID`), resultado, cancelar, reintentar, idempotencia, errores frecuentes
  y dónde está el OpenAPI.
- Ayuda: pregunta «¿Qué URL uso para conectar mi app?» y «¿Cómo veo mi prueba como código?».
- `site/ayuda.html`: sección «Qué URL usar» en español e inglés.
- `docs/equipo/pendientes.md`: fila del release.

## Pruebas

- `internal/api`: `TestOpenAPICubreLasRutas`; `GET /api/v1/openapi.json` con y sin clave (401), y desde un punto de
  acceso con su base en `servers`.
- Web UI (navegador): la guía y la tabla en Mis APIs y en el detalle; las tres pestañas de la API propia con la URL,
  la clave y el modelo de la API; «Ver como código» con una receta con esquema, respaldo y un archivo (nota en
  OpenAI); Descargar OpenAPI; móvil y escritorio, claro y oscuro.
- Los ejemplos de la API propia se ejecutan una vez contra el servidor de prueba (curl y Python) para comprobar que
  funcionan tal como se copian.
