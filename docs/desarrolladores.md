# Guía del desarrollador

Todo lo que necesitas para usar Tramtai desde tu código. Cada tema tiene ejemplos que funcionan tal cual en Python,
JavaScript, Go y curl, en [docs/ejemplos](ejemplos); el CI los ejecuta en cada cambio. La misma guía, con el código a
la vista, está en la [página del producto](https://mscottb.github.io/Tramtai/desarrolladores.html). Todas las rutas y
opciones están en la [referencia de la API](api.md).

## Empezar

Necesitas Tramtai en marcha con al menos un agente conectado, y la **URL** y la **clave** de tu API (en la app,
**Mis APIs**). Los ejemplos las leen de `TRAMTAI_URL` (por defecto `http://127.0.0.1:17373`), `TRAMTAI_CLAVE` y
`TRAMTAI_MODELO` (agente y modelo, por defecto `claude/haiku`).

**¿Qué API uso?** Si tu app ya usa el SDK de OpenAI o el de Anthropic, su URL (`/openai/v1` o `/anthropic`): no cambias
nada más. Si necesitas trabajos largos que sigues, cancelas o reintentas, la API propia (`/api/v1`). La
[comparación](api.md#qué-url-uso) dice qué permite cada una y por qué.

**¿Qué clave uso?** La de una API propia de tu app (**Mis APIs → Nueva API**): con sus modelos y su presupuesto,
y la regeneras sin tocar las demás. La **clave del espacio** es para ti y tus scripts. En la API, «instancia» es ese
espacio de trabajo: el global o el de un proyecto.

Si aún no conoces la app, [el recorrido guiado](https://mscottb.github.io/Tramtai/#demo) la enseña en la portada, y
en Tramtai el botón **?** de cada página tiene **Muéstrame en pantalla**. La API no cambia.

Desde la terminal, `tramtai espacios` lista los espacios, `tramtai espacio iniciar|detener|olvidar` los gestiona y
`tramtai agentes desconectar|conectar <agente>` deja un agente fuera sin desinstalar su CLI.

| Lenguaje | Instalar |
|---|---|
| Python | `pip install openai requests` |
| JavaScript (Node 20+) | `npm install openai` |
| Go | `go get github.com/openai/openai-go/v3` |
| curl | los ejemplos que encadenan pasos usan [jq](https://jqlang.org/) |

## Tu primer mensaje

Por la URL compatible con OpenAI, con la respuesta en vivo. El campo `model` elige agente y modelo.
Ejemplos: [Python](ejemplos/python/primer_mensaje.py) · [JavaScript](ejemplos/javascript/primer_mensaje.mjs) · [Go](ejemplos/go/primer_mensaje/main.go) · [curl](ejemplos/curl/primer_mensaje.sh).

## Archivos

Se admiten PDF, Word (`.docx`), PowerPoint (`.pptx`), Excel (`.xlsx`) y HTML, que Tramtai convierte a texto, y texto
tal cual (`.txt`, `.md`, `.json`, `.csv`, `.xml`, `.yaml`…); la lista completa está en
[Formatos admitidos](api.md#formatos-admitidos) y en `GET /api/v1/limites`. Por OpenAI y Anthropic el archivo va en
base64 dentro del mensaje (unos 12 MB en total); por la API propia se sube a `/cargas` (hasta 50 MB cada uno) y el
trabajo lo cita. Ejemplos: [Python](ejemplos/python/archivo.py) · [JavaScript](ejemplos/javascript/archivo.mjs) · [Go](ejemplos/go/archivo/main.go) · [curl](ejemplos/curl/archivo.sh).

## Respuestas en JSON

Pide la respuesta con un JSON Schema: Tramtai la valida antes de devolverla y, si no cumple, puede pedir al agente que
la corrija. Ejemplos: [Python](ejemplos/python/respuesta_json.py) · [JavaScript](ejemplos/javascript/respuesta_json.mjs) · [Go](ejemplos/go/respuesta_json/main.go) · [curl](ejemplos/curl/respuesta_json.sh).

## Herramientas

Las herramientas (`tools`) son funciones de **tu** aplicación: consultar un pedido, buscar en tu base de datos, mirar
el tiempo. El ciclo:

1. Tu app ofrece las funciones, con un JSON Schema para sus argumentos.
2. El agente, si necesita una, **pide** la llamada con sus argumentos.
3. Tu app la ejecuta.
4. Le devuelve el resultado en el turno siguiente.
5. El agente responde (o pide otra).

El agente nunca ejecuta nada: solo pide. Tramtai le describe las herramientas y valida los argumentos contra su
esquema; si el agente se equivoca, le pide que corrija (con las correcciones de la petición o de la receta) antes de
responder.

- **Por la URL de OpenAI**, con los SDK oficiales: `tools`, `tool_calls` y el rol `tool`.
  Ejemplos: [Python](ejemplos/python/herramientas.py) · [JavaScript](ejemplos/javascript/herramientas.mjs) · [Go](ejemplos/go/herramientas/main.go) · [curl](ejemplos/curl/herramientas.sh).
- **Por la API propia:** `herramientas` en la solicitud; el trabajo terminado trae `llamadas` con los argumentos como
  objeto, y el turno siguiente lleva un mensaje `{"rol": "herramienta", "id_llamada": …}` por resultado.
  Ejemplos: [Python](ejemplos/python/herramientas_api_propia.py) · [curl](ejemplos/curl/herramientas_api_propia.sh).
- **Por la URL de Anthropic**, igual que con su SDK: `tools` con `input_schema`, bloques `tool_use` en la respuesta y
  `tool_result` en el mensaje siguiente.

Con herramientas, la respuesta llega completa al terminar, aunque pidas `stream`. No se combinan con una respuesta en
JSON, y van en cada petición: las recetas no las guardan.

### Qué agentes las atienden

<!-- agentes-herramientas -->
Comprobado el 2026-10-20 con los agentes reales: el ciclo completo (pedir `clima`, recibir el resultado y responder)
por cada URL.

| Agente | Modelo probado | OpenAI | Anthropic | API propia |
|---|---|---|---|---|
| Claude Code | `haiku` | sí | sí | sí |
| Codex | el predeterminado de su CLI | sí | sí | sí |
| GitHub Copilot | `auto` (plan Free) | sí | sí | sí |
| Gemini API | ninguno | sin comprobar (falta clave) | | |
| Antigravity | ninguno | no se usa desde apps ni APIs | | |

Se repite con `go test -tags canario -run TestHerramientasConAgentesReales -v ./internal/api/` (salta los agentes que
no estén instalados o sin sesión).
<!-- /agentes-herramientas -->

## Trabajos largos

Solo por la API propia: creas el trabajo, lo sigues en vivo por eventos (si se corta la conexión, retomas con
`Last-Event-ID`) y lees el resultado. `POST /trabajos/{id}/cancelar` lo detiene y `POST /trabajos/{id}/reintentar` lo
vuelve a lanzar. Ejemplos: [Python](ejemplos/python/trabajo_largo.py) · [curl](ejemplos/curl/trabajo_largo.sh).

## Errores y reintentos

Los errores llegan en el formato de la API que usas, con un `code` estable. Para reintentar sin duplicar un trabajo,
envía la cabecera `Idempotency-Key`: la misma clave con la misma petición devuelve el mismo trabajo (en las tres APIs).

| HTTP | Código | Qué hacer |
|---|---|---|
| 400 | `solicitud_invalida`, `parametro_no_soportado`, `carga_invalida` | Corrige la petición: el mensaje dice qué campo falla. |
| 401 | `no_autorizado` | Revisa la clave (Mis APIs); si la regeneraste, la anterior ya no vale. |
| 409 | `conflicto_idempotencia` | Esa `Idempotency-Key` ya se usó con otra petición: usa otra. |
| 409 | `agente_desconectado` | El agente está desconectado en Tramtai: conéctalo en **Agentes** o con `tramtai agentes conectar <agente>`. |
| 422 | `salida_no_cumple_esquema` | La respuesta o las llamadas no cumplían el esquema: pide correcciones o «Corregir». |
| 429 | `cola_llena`, `presupuesto_agotado`, `limite_uso_proveedor` | Espera lo que diga `Retry-After` y reintenta. |
| 502 | `error_agente`, `autenticacion_agente` | El agente falló o no tiene sesión: revisa **Agentes** en la app. |
| 503 | `agente_no_disponible`, `conversor_no_disponible` | Algo no está listo; con `Retry-After`, reintenta después. |

`GET /api/v1/agentes` y `GET /api/v1/capacidades` traen `desconectado` por agente (y `/capacidades`, `cuenta`: con qué
cuenta inició sesión su CLI), así que tu app puede mirarlo antes de enviar. La lista completa de códigos está en la
[referencia de la API](api.md).

## Importar en Postman o generar un cliente

`GET /api/v1/openapi.json` describe la API propia en OpenAPI 3.1, con la URL de tu espacio o de tu API. Postman,
Insomnia o Bruno la importan tal cual («Import» → URL o archivo), y `openapi-generator` o `openapi-typescript` generan
un cliente. En la app, la guía «¿Qué URL uso?» de Mis APIs (y el detalle de cada API) la descarga como archivo.

```sh
curl -s "$TRAMTAI_URL/api/v1/openapi.json" -H "Authorization: Bearer $TRAMTAI_CLAVE" -o tramtai-openapi.json
```

## Límites

`GET /api/v1/limites` da los topes de una petición (archivos, tamaños, mensajes) y los formatos admitidos, para que tu
app los conozca antes de enviar. Ejemplo: [curl](ejemplos/curl/limites.sh).
