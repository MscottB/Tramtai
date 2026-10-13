# API HTTP

`tramtai servicio iniciar` levanta el servicio en `http://127.0.0.1:17373` (`servicio.host` y `servicio.puerto`). Hay tres contratos, cada uno bajo su propia ruta:

| Contrato | Base | Para |
|---|---|---|
| API nativa | `/api/v1` | Trabajos asíncronos, eventos, cargas de archivos y esquemas |
| OpenAI | `/openai/v1` | SDK y herramientas de OpenAI (`base_url`) |
| Anthropic | `/anthropic` | SDK de Anthropic (`base_url`) |

## Seguridad

- **Clave (token):** `Authorization: Bearer <clave>` o `x-api-key: <clave>`. La forma más fácil de verla es la página **Conectar apps** de la Web UI; también `tramtai global token` (o `tramtai proyecto token`). Se crea la primera vez en `%APPDATA%\tramtai\datos\token-global`, o sale de la variable `TRAMTAI_TOKEN_GLOBAL`.
- **Web UI:** en el propio equipo entra sin clave (mismo usuario del sistema); ver [arquitectura](arquitectura.md#interfaces).
- **Host:** solo `127.0.0.1`, `localhost` y `::1`. Cualquier otro nombre en la cabecera `Host` se rechaza con 403 antes de autenticar (protección frente a DNS rebinding).
- **Navegadores:** no hay CORS. Una petición con `Origin` de otro sitio se rechaza con 403.
- **Acceso remoto:** está desactivado. Solo se permite con `servicio.permitir_remoto: true`, token obligatorio y ningún agente en modo suscripción. Lo recomendado es un túnel SSH.
- `/healthz` no exige token y no expone datos.

## API nativa `/api/v1`

| Método y ruta | Función |
|---|---|
| `POST /trabajos` | Crea un trabajo (`SolicitudEjecucion`). Responde 202 con el trabajo y `Location`; si la `Idempotency-Key` ya apuntaba a la misma solicitud, responde 200. |
| `GET /trabajos/{id}` | Estado, error, modelo enviado y efectivo, uso, salida y `disponibilidad` (`disponible` o `expirada`). Tras la retención de metadatos (7 días) responde 404. |
| `GET /trabajos/{id}/salida` | La salida con su tipo (`text/markdown`, `text/plain` o `application/json`). Si no ha terminado, 409 `trabajo_no_terminado`; si la retención ya la borró, 410 `resultado_expirado`. |
| `GET /trabajos/{id}/eventos` | SSE con `id` monotónico. `Last-Event-ID` (o `?desde=`) reanuda tras ese evento. Envía latidos cada 15 s y termina con un evento `fin`. Si el resultado expiró, empieza con un evento `aviso`: solo quedan los eventos de estado. |
| `POST /trabajos/{id}/cancelar` | Cancelación idempotente; responde 202 sin afirmar que ya terminó. |
| `POST /trabajos/{id}/reintentar` | Crea un intento nuevo (`intento_de`) a partir de un trabajo terminado. Si sus entradas ya vencieron, 410 `entrada_no_disponible`. |
| `DELETE /trabajos/{id}` | Borra un trabajo terminado: 204, o 409 si sigue activo. |
| `DELETE /trabajos` | Vacía el historial: borra los trabajos terminados de la instancia y las cargas que ya no usa ninguno; conserva los activos. Responde `{"eliminados": n}`. No existe en los puntos de acceso. |
| `POST /ejecutar` | Igual que `POST /trabajos`, pero espera el resultado. Envía `Location` antes de esperar; si se corta la conexión, el trabajo no se cancela. |
| `POST /cargas` | Multipart con el campo `archivo`, o el binario en el cuerpo con `?nombre=`. Devuelve `identificador_carga`, `sha256`, `tamano_bytes` y `vence_en`. Un trabajo nuevo que cite una carga vencida se rechaza con 410; las cargas de un trabajo aceptado se conservan mientras se pueda reintentar. |
| `GET /capacidades` | Por agente: streaming (`tokens` o `final`), salida estructurada, modelo efectivo, versión detectada, autenticación y términos. Incluye el formato de serialización. |
| `GET /modelos` | Catálogo: agentes, `agente/modelo` configurados y perfiles. |

Ejemplo:

```bash
curl -s http://127.0.0.1:17373/api/v1/trabajos \
  -H "Authorization: Bearer $TOKEN" -H "Idempotency-Key: informe-42" \
  -d '{"agente":"claude","perfil_modelo":"rapido","cargas":["carga_…"],
       "mensajes":[{"rol":"usuario","contenido":"Extrae los compromisos"}],
       "salida":{"formato":"json","esquema":{"type":"object","required":["compromisos"]}}}'
curl -sN http://127.0.0.1:17373/api/v1/trabajos/trabajo_…/eventos -H "Authorization: Bearer $TOKEN"
```

Los campos desconocidos en la API nativa son un error (`solicitud_invalida`).

## Fachadas compatibles

| Endpoint | Soporte |
|---|---|
| `POST /openai/v1/chat/completions` | Mensajes de texto (`system`, `developer`, `user` y `assistant`), `stream` con `stream_options.include_usage` y `response_format` (`json_object` o `json_schema`). |
| `POST /openai/v1/responses` | `input` como texto o mensajes, `instructions`, `text.format` y streaming básico (`response.created`, `response.output_text.delta`, `response.completed`). |
| `GET /openai/v1/models` | El mismo catálogo que `/api/v1/modelos`. |
| `POST /anthropic/v1/messages` | `system`, mensajes de texto, `stream` con la secuencia de eventos de Anthropic. `max_tokens` es obligatorio, pero se ignora con aviso. |

### Campo `model`

- `claude/sonnet`, `codex/<id>`: agente y modelo literal.
- `perfil/rapido`: perfil de `modelos.perfiles` con el agente predeterminado.
- `claude`: agente con el modelo predeterminado de su CLI.
- `haiku`: sin prefijo, solo si ese modelo está configurado en un único agente. Si está en varios, se rechaza con 400 `modelo_ambiguo`.

### Política de parámetros (sección 8.1)

- **No soportados (error 400 `parametro_no_soportado`):** `tools`, `tool_choice`, `n` mayor que 1, `logprobs`, `stop`, imágenes o audio, `stop_sequences`, `thinking`… Con un valor sin efecto (`null`, `false`, `[]`, `n: 1`) se aceptan.
- **Ignorables (se aceptan con aviso):** `temperature`, `top_p`, `max_tokens`, `seed`, `user`, `metadata`… El aviso llega en la cabecera `Tramtai-Advertencia: parametros ignorados: …`.
- **Desconocidos:** error, salvo con `compatibilidad.modo_tolerante: true`.
- **Uso:** si el agente no informa tokens, se envían ceros con `Tramtai-Advertencia: uso_no_verificado`.
- **Desconexión:** si el cliente se desconecta, el trabajo se cancela (`compatibilidad.cancelar_al_desconectar: true`). En la API nativa no.

### Ejemplos con los SDK oficiales

```python
from openai import OpenAI
cliente = OpenAI(base_url="http://127.0.0.1:17373/openai/v1", api_key=TOKEN)
r = cliente.chat.completions.create(model="claude/haiku", messages=[{"role": "user", "content": "Hola"}])

import anthropic
cliente = anthropic.Anthropic(base_url="http://127.0.0.1:17373/anthropic", api_key=TOKEN)
m = cliente.messages.create(model="claude/sonnet", max_tokens=1024, messages=[{"role": "user", "content": "Hola"}])
```

Probados con los SDK oficiales `openai` y `anthropic` de Python y Node (chat, streaming, Responses y Messages con y sin streaming). Con el SDK de Anthropic, `base_url` termina en `/anthropic`: el SDK añade `/v1`.

### Perfiles de ejecución y modo de contexto

- **`perfil`** (API nativa) o `--perfil-trabajo` (CLI): aplica `profiles/<nombre>.yaml` del proyecto (`.tramtai/profiles/`) o de la configuración global. Aporta instrucciones, formato, esquema y `conservar_salida_invalida`; los campos explícitos de la solicitud prevalecen. `tramtai iniciar` crea el perfil de ejemplo `resumen`.
- **`salida.conservar_salida_invalida`:** si la salida no cumple el esquema, el trabajo falla con `salida_no_cumple_esquema`, pero conserva la salida bruta para revisarla.
- **`modo_contexto: consciente_proyecto`** (o `--contexto`): el agente trabaja en la raíz del proyecto y carga sus reglas. Solo se admite en proyectos incluidos en `proyectos_confiables`, en la configuración global o en `local.yaml`; si no, devuelve 403 `contexto_no_permitido`. Antes de ejecutar se registra un evento `aviso` con el contexto que se carga.

## Puntos de acceso y administración

Un punto de acceso expone la misma API bajo su propia URL y token: `http://127.0.0.1:17373/p/<nombre>/openai/v1`, o su propio puerto. Solo expone los contratos que tiene habilitados, aplica sus alias, modelos permitidos y modelo forzado (cabecera `Tramtai-Modelo-Forzado`) y solo ve sus trabajos. Con `ejecucion.maximo_trabajos` limita sus ejecuciones simultáneas, siempre dentro del límite de su instancia. La administración (`/api/v1/servicio`) y la API de cada instancia con `Tramtai-Instancia` se describen en [operacion.md](operacion.md).

`GET /api/v1/trabajos?limite=20` lista los trabajos recientes de la instancia, o los del punto de acceso si se llama desde uno.

## Errores

API nativa: `{"error": {"codigo": "…", "mensaje": "…"}}`. Las fachadas usan el formato de error de OpenAI o de Anthropic, con el código de Tramtai en `code` (OpenAI).

| HTTP | Códigos |
|---|---|
| 400 | `solicitud_invalida`, `parametro_no_soportado`, `modelo_ambiguo`, `modelo_no_permitido`, `modelo_no_disponible`, `modelo_no_configurado`, `carga_invalida` |
| 401 / 403 | `no_autorizado` / `origen_no_permitido`, `contexto_no_permitido` |
| 404 | `no_encontrado` |
| 409 | `conflicto_idempotencia`, `trabajo_no_terminado`, `trabajo_sin_salida` |
| 410 | `entrada_no_disponible`, `resultado_expirado` |
| 422 | `salida_no_cumple_esquema`, `conversion_fallida` |
| 429 | `cola_llena` (con `Retry-After`), `limite_uso_proveedor` |
| 502 / 503 / 504 | `error_agente`, `autenticacion_agente` / `agente_no_disponible` / `tiempo_ejecucion_agotado` |
