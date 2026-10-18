# API HTTP

`tramtai servicio iniciar` levanta el servicio en `http://127.0.0.1:17373` (`servicio.host` y `servicio.puerto`). Hay tres contratos, cada uno bajo su propia ruta:

| Contrato | Base | Para |
|---|---|---|
| API nativa | `/api/v1` | Trabajos asíncronos, eventos, cargas de archivos y esquemas |
| OpenAI | `/openai/v1` | SDK y herramientas de OpenAI (`base_url`) |
| Anthropic | `/anthropic` | SDK de Anthropic (`base_url`) |

## ¿Qué URL uso?

- **OpenAI** (`/openai/v1`): tu app ya usa el SDK de OpenAI o una herramienta que deja cambiar la «URL base» (LangChain,
  n8n, un editor con IA). Cambias la URL y la clave; el resto sigue igual.
- **Anthropic** (`/anthropic`): tu app usa el SDK de Anthropic.
- **API propia** (`/api/v1`): necesitas lo que las otras no tienen: enviar archivos (PDF, Word, Excel, ZIP…) que
  Tramtai convierte, tareas largas que sigues en vivo y retomas si se corta la conexión, cancelar y reintentar, elegir
  respaldo y esfuerzo en cada petición, idempotencia, el registro de cada paso y los límites.

| | OpenAI | Anthropic | API propia |
|---|---|---|---|
| SDK oficial y herramientas existentes | sí | sí | no (HTTP y JSON) |
| Respuesta en vivo | sí | sí | sí, y se retoma tras un corte |
| Respuesta JSON con esquema | sí (`response_format`) | no | sí (`salida.esquema`) |
| Enviar archivos | no | no | sí |
| Tareas largas: seguir, cancelar, reintentar | no | no | sí |
| Respaldo y esfuerzo por petición | solo el esfuerzo | no | sí |
| Registro de cada paso y límites | no | no | sí |

En la app, **Mis APIs** muestra esta guía con las URL de cada API y ejemplos para copiar (curl, Python y Node, también
de la API propia), y **Probar → Ver como código** da la prueba actual como petición.

## Seguridad

- **Clave (token):** `Authorization: Bearer <clave>` o `x-api-key: <clave>`.
  - **Clave de la instancia** (la de la *API principal*): se ve en **Mis APIs → API principal** o con `tramtai global token` (o `tramtai proyecto token`). Se crea la primera vez en `datos/token-global` (archivo con permisos 0600 en la [carpeta de Tramtai](inicio.md#configuración)), o sale de la variable que indique `instancia_global.token_variable_entorno` (`TRAMTAI_TOKEN_GLOBAL`).
  - **Clave de cada API por aplicación:** solo se muestra al crearla o regenerarla; Tramtai guarda únicamente su huella SHA-256. Ver [puntos de acceso](#puntos-de-acceso-y-administración).
  - Si se filtra alguna, ver [Si se filtra una clave](operacion.md#si-se-filtra-una-clave).
- **Web UI:** no se abre con la clave de las aplicaciones (`POST /ui/sesion` con una clave responde 401). Se entra en el propio equipo con el mismo usuario del sistema o con el código de un solo uso de la app, la terminal o `tramtai abrir`; ver [arquitectura](arquitectura.md#interfaces).
- **Host:** solo `127.0.0.1`, `localhost` y `::1`. Cualquier otro nombre en la cabecera `Host` se rechaza con 403 antes de autenticar (protección frente a DNS rebinding).
- **Navegadores:** no hay CORS. Una petición con `Origin` de otro sitio se rechaza con 403.
- **Acceso remoto:** está desactivado. Solo se permite con `servicio.permitir_remoto: true`, token obligatorio y ningún agente en modo suscripción. Lo recomendado es un túnel SSH. La Web UI remota solo se abre con un código de `tramtai abrir` hecho en el equipo del servicio.
- `/healthz` no exige token y no expone datos.
- **Contenido hostil:** lo que manda una app (mensaje, instrucciones, documentos, esquema) no puede hacer que un agente
  lea archivos, ejecute órdenes o salga a la red: Claude Code y Copilot van sin herramientas y Codex en solo lectura, sin
  terminal ni búsqueda web (comprobado con [canarios](operacion.md#comprobar-el-aislamiento-de-los-agentes)).
  Antigravity, sin esa comprobación, no se puede usar desde apps ni APIs (`agente_no_disponible`) salvo
  `adaptadores.antigravity.permitir_apps`. Los delimitadores de turnos y archivos llevan un código por petición, y una
  clave de proveedor que aparezca en la salida se tapa (`[REDACTADO]`).

## API nativa `/api/v1`

| Método y ruta | Función |
|---|---|
| `POST /trabajos` | Crea un trabajo (`SolicitudEjecucion`). Responde 202 con el trabajo y `Location`; si la `Idempotency-Key` ya apuntaba a la misma solicitud, responde 200. |
| `GET /trabajos/{id}` | Estado, error, modelo enviado y efectivo, uso, salida y `disponibilidad` (`disponible` o `expirada`). Tras la retención de metadatos (7 días) responde 404. |
| `GET /trabajos/{id}/salida` | La salida con su tipo (`text/markdown`, `text/plain` o `application/json`). Si no ha terminado, 409 `trabajo_no_terminado`; si la retención ya la borró, 410 `resultado_expirado`. |
| `GET /trabajos/{id}/eventos` | SSE con `id` monotónico: `estado`, `entrada` (antes de esperar al agente: lo que ocupa la entrada), `texto`, `aviso` e `intento` (al empezar cada intento, JSON con `agente`, `modelo`, `esfuerzo` y `del_agente`; un cliente que no lo espera lo ignora). `Last-Event-ID` (o `?desde=`) reanuda tras ese evento. Envía latidos cada 15 s y termina con un evento `fin`. Si el resultado expiró, empieza con un evento `aviso`: solo quedan los eventos de estado, entrada e intento. |
| `GET /limites` | Los topes de una petición: `cargas_por_peticion` (20), `carga_mb`, `texto_convertido_mb`, `cuota_cargas_mb`, `cuerpo_mb` (16), `esquema_kb` (256), `mensajes` (1.000) y `reserva_salida_tokens`. Es rápida; `/capacidades` trae el mismo bloque en `limites`. |
| `GET /trabajos/{id}/registro` | Los pasos de la petición con su hora, sin el texto de la respuesta: `{"eventos":[{"numero","tipo","datos","creado_en"}]}` con los eventos `estado`, `entrada` (caracteres, tokens estimados y cada documento de mayor a menor), `intento` y `aviso`. Mismos permisos que `GET /trabajos/{id}`. |
| `POST /trabajos/{id}/cancelar` | Cancelación idempotente; responde 202 sin afirmar que ya terminó. |
| `POST /trabajos/{id}/reintentar` | Crea un intento nuevo (`intento_de`) a partir de un trabajo terminado. Si sus entradas ya vencieron, 410 `entrada_no_disponible`. |
| `DELETE /trabajos/{id}` | Borra un trabajo terminado: 204, o 409 si sigue activo. |
| `DELETE /trabajos` | Vacía el historial: borra los trabajos terminados de la instancia y las cargas que ya no usa ninguno; conserva los activos. Responde `{"eliminados": n}`. No existe en los puntos de acceso. |
| `POST /ejecutar` | Igual que `POST /trabajos`, pero espera el resultado. Envía `Location` antes de esperar; si se corta la conexión, el trabajo no se cancela. |
| `POST /cargas` | Multipart con el campo `archivo`, o el binario en el cuerpo con `?nombre=`. Devuelve `identificador_carga`, `sha256`, `tamano_bytes` y `vence_en`. Un trabajo nuevo que cite una carga vencida se rechaza con 410; las cargas de un trabajo aceptado se conservan mientras se pueda reintentar. |
| `GET /agentes` | La ficha de cada agente, al momento y sin comprobar los CLI: `agente`, `nombre`, `programa`, `codigo`, `color` (`claro`, `oscuro`), `orden`, `streaming`, `salida_estructurada`, `modelo_efectivo_informado`, `esfuerzos`, `autenticaciones`, `variables_clave`, `permitido_apps` y `url_instalacion`. Un agente sin ficha (de pruebas) va al final con su id. |
| `GET /capacidades` | Por agente: streaming (`tokens` o `final`), salida estructurada, modelo efectivo, versión detectada, autenticación y términos. Incluye el formato de serialización. |
| `GET /openapi.json` | La descripción OpenAPI 3.1 de esta API, con `servers` apuntando a la URL de la petición (la de la instancia, `/p/<nombre>/api/v1` o el puerto de la API). Mismos permisos que el resto. En Mis APIs, **Descargar OpenAPI** la guarda con la URL de cada API. |
| `GET /modelos` | Catálogo: agentes, `agente/modelo` configurados y perfiles. Cada entrada indica `predeterminado: true` si es el modelo predeterminado, `version` con el último modelo efectivo informado por el agente y, en los perfiles, `resuelto` con el agente/modelo al que llevan. |

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

### Guía paso a paso

1. **Subir los archivos** (si hay): `POST /cargas` multipart con el campo `archivo`. Guarda `identificador_carga`.

   ```bash
   curl -s http://127.0.0.1:17373/api/v1/cargas -H "Authorization: Bearer $TOKEN" -F "archivo=@informe.pdf"
   ```

2. **Crear el trabajo**: `POST /trabajos` con `mensajes`, el modelo (`agente` y `modelo`, o `perfil_modelo`), las
   `cargas` y, si quieres JSON, `salida: {"formato": "json", "esquema": {…}}`. Responde 202 con el trabajo (`id`).
3. **Seguirlo en vivo**: `GET /trabajos/{id}/eventos` (SSE). Cada evento trae `id`, `event` y `data`: los `texto` son
   trozos de la respuesta (una línea `data:` por línea del texto); `estado`, `entrada`, `intento` y `aviso` cuentan lo
   que pasa, y `fin` (`{"estado","codigo_error"}`) cierra el flujo. Si se corta, vuelve a pedirlo con la cabecera
   `Last-Event-ID` (o `?desde=`) con el último `id` recibido: sigue desde ahí y el trabajo no se repite ni se cancela.
4. **Leer el resultado**: `GET /trabajos/{id}/salida` (con esquema, el JSON ya validado). `GET /trabajos/{id}` da el
   estado, el modelo efectivo y el uso de tokens.
5. **Cancelar o reintentar**: `POST /trabajos/{id}/cancelar` (idempotente) y `POST /trabajos/{id}/reintentar` (un
   trabajo nuevo con `intento_de`, con las mismas cargas mientras no venzan).
6. **Idempotencia**: repetir `POST /trabajos` con la misma `Idempotency-Key` y la misma solicitud devuelve el mismo
   trabajo (200) en lugar de crear otro; con otra solicitud, 409 `conflicto_idempotencia`. Útil para reintentar tras
   un corte de red sin duplicar el trabajo.

Errores frecuentes: 401 `no_autorizado` (falta la clave o no es de esta API), 400 `solicitud_invalida` (un campo que no
existe o un valor no válido; el mensaje dice cuál), 410 `entrada_no_disponible` (la carga venció: súbela otra vez), 409
`trabajo_no_terminado` (pediste la salida antes del final), 429 `presupuesto_agotado` o `cola_llena` (con
`Retry-After`). En Mis APIs y en Probar (**Ver como código**) tienes este flujo completo en curl, Python y Node.

## Fachadas compatibles

| Endpoint | Soporte |
|---|---|
| `POST /openai/v1/chat/completions` | Mensajes de texto (`system`, `developer`, `user` y `assistant`), `stream` con `stream_options.include_usage`, `response_format` (`json_object` o `json_schema`) y herramientas (`tools`, `tool_choice`). |
| `POST /openai/v1/responses` | `input` como texto o mensajes, `instructions`, `text.format`, herramientas (`function_call`) y streaming básico (`response.created`, `response.output_text.delta`, `response.completed`). |
| `GET /openai/v1/models` | El mismo catálogo que `/api/v1/modelos`. |
| `POST /anthropic/v1/messages` | `system`, mensajes de texto, herramientas (`tool_use` y `tool_result`), `stream` con la secuencia de eventos de Anthropic. `max_tokens` es obligatorio, pero se ignora con aviso. |

### Campo `model`

- `claude/sonnet`, `codex/<id>`: agente y modelo literal.
- `perfil/rapido`: perfil de `modelos.perfiles` con el agente predeterminado.
- `claude`: agente con el modelo predeterminado de su CLI.
- `haiku`: sin prefijo, solo si ese modelo está configurado en un único agente. Si está en varios, se rechaza con 400 `modelo_ambiguo`.

### Política de parámetros

- **No soportados (error 400 `parametro_no_soportado`):** `n` mayor que 1, herramientas que no son funciones (búsqueda web, archivos, código…), `logprobs`, `stop`, imágenes o audio, `stop_sequences`, `thinking`… Con un valor sin efecto (`null`, `false`, `[]`, `n: 1`) se aceptan.
- **Ignorables (se aceptan con aviso):** `temperature`, `top_p`, `max_tokens`, `seed`, `user`, `metadata`… El aviso llega en la cabecera `Tramtai-Advertencia: parametros ignorados: …`.
- **Desconocidos:** error, salvo con `compatibilidad.modo_tolerante: true`.
- **Uso:** si el agente no informa tokens, se envían ceros con `Tramtai-Advertencia: uso_no_verificado`.
- **Desconexión:** si el cliente se desconecta, el trabajo se cancela (`compatibilidad.cancelar_al_desconectar: true`). En la API nativa no.

### Herramientas (`tools`)

Las tres rutas aceptan herramientas de tipo función: `tools` y `tool_choice` en chat y responses (con `parallel_tool_calls`), y `tools` con `input_schema` y `tool_choice` (`auto`, `any`, `tool`, `none`, `disable_parallel_tool_use`) en Anthropic. Funciona igual que con OpenAI o Anthropic: el agente **pide** la llamada (`tool_calls` con `finish_reason: "tool_calls"`, o bloques `tool_use` con `stop_reason: "tool_use"`), tu aplicación la ejecuta y devuelve el resultado en el siguiente mensaje (rol `tool`, `function_call_output` o `tool_result`).

- El agente nunca ejecuta nada: Tramtai le describe las herramientas y le pide la respuesta con la salida estructurada de su CLI, y la traduce al formato de cada API.
- Los argumentos se validan con el JSON Schema de cada herramienta. Si el agente devuelve argumentos que no lo cumplen, o no llama a ninguna cuando `tool_choice` lo exige, la respuesta es 422 `salida_no_cumple_esquema`.
- La respuesta llega completa al terminar: con herramientas no hay texto en vivo, aunque `stream: true` envía los eventos de siempre.
- No se combina con `response_format` en JSON.

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

### Esfuerzo de razonamiento

Cuánto razona el modelo antes de responder. Es opcional: sin él, cada CLI usa su propia configuración.

- **API nativa:** `"esfuerzo": "bajo" | "medio" | "alto" | "muy_alto" | "maximo"`. Un valor desconocido devuelve 400.
- **OpenAI:** `reasoning_effort` (chat) o `reasoning.effort` (Responses). `minimal` y `low` equivalen a bajo; `medium` a medio; `high` a alto; `xhigh` a muy alto; `none` deja el del agente.
- **Por API:** el campo `esfuerzo` de un punto de acceso, que se edita en Mis APIs, se usa cuando la aplicación no pide uno.
- **Del espacio:** `predeterminados.esfuerzo` (en **Ajustes**, bajo el modelo predeterminado) va con el modelo predeterminado: se usa cuando la petición no elige agente, modelo, perfil ni esfuerzo (una API por aplicación sin modelo propio también lo hereda). `GET /api/v1/modelos` lo indica en `esfuerzo_espacio` del modelo predeterminado, y `POST /api/v1/instancia/predeterminado` acepta `{"modelo", "esfuerzo"}` (`""` vuelve al del agente; sin `esfuerzo`, no cambia; con `modelo` vacío solo cambia el esfuerzo). Un valor escrito a mano que no es un nivel se ignora.
- **Qué agentes lo admiten:**
  - Claude Code (`--effort`), hasta máximo.
  - Codex (`model_reasoning_effort`), hasta muy alto; si se pide máximo, usa muy alto.
  - Copilot (`--reasoning-effort`), hasta máximo, solo con un modelo concreto: con `copilot` o `copilot/auto` (el
    automático, el único del plan gratuito) se ignora y `GET /api/v1/modelos` no lo ofrece.
  - La API de Gemini y Antigravity lo ignoran.
- **Respaldo:** cada entrada puede llevar su propio esfuerzo con `+nivel` (`codex/gpt-5.5+alto`), o `+agente` para usar el de su CLI aunque el principal tenga uno; sin sufijo, usa el del principal. Tras un salto, el trabajo guarda el respaldo que queda con lo que heredaba ya escrito (`+alto` o `+agente`), así un reintento no hereda el esfuerzo del respaldo que respondió.
- **Dónde se ve:** `GET /api/v1/modelos` indica en `esfuerzos` los niveles de cada modelo, el trabajo guarda el esfuerzo pedido en `solicitud.esfuerzo` y cada evento `intento` dice el que se usó: el pedido, ajustado al máximo del agente, o con `del_agente: true` el que usa el CLI por su cuenta, si se conoce.

### Perfiles de ejecución y modo de contexto

En un punto de acceso el modo de contexto lo fija la API (`aislado` si no indica otro): una petición, o un perfil, que pida otro recibe 403 `contexto_no_permitido`.

- **`perfil`** (API nativa) o `--perfil-trabajo` (CLI): aplica `profiles/<nombre>.yaml` del proyecto (`.tramtai/profiles/`) o de la configuración global. Aporta instrucciones, formato, esquema y `conservar_salida_invalida`; los campos explícitos de la solicitud prevalecen. `tramtai iniciar` crea el perfil de ejemplo `resumen`.
- **`salida.conservar_salida_invalida`:** si la salida no cumple el esquema, el trabajo falla con `salida_no_cumple_esquema`, pero conserva la salida bruta para revisarla.
- **`modo_contexto: consciente_proyecto`** (o `--contexto`): el agente trabaja en la raíz del proyecto y carga sus reglas. Solo se admite en proyectos incluidos en `proyectos_confiables` de la configuración global; si no, devuelve 403 `contexto_no_permitido`. Antes de ejecutar se registra un evento `aviso` con el contexto que se carga.

## Puntos de acceso y administración

Un punto de acceso expone la misma API bajo su propia URL y token: `http://127.0.0.1:17373/p/<nombre>/openai/v1`, o su propio puerto. Solo expone los contratos que tiene habilitados, antepone sus `instrucciones` (si tiene) a las de cada solicitud, aplica sus alias, modelos permitidos y modelo forzado (cabecera `Tramtai-Modelo-Forzado`) y solo ve sus trabajos. Con `ejecucion.maximo_trabajos` limita sus ejecuciones simultáneas, siempre dentro del límite de su instancia.

Un punto de acceso también puede ser una **receta**: con `esquema` (un JSON Schema escrito como YAML), toda petición que no traiga su propio esquema responde con JSON validado, también por las fachadas sin `response_format`. Con `respaldo` (hasta 3 entradas `agente` o `agente/modelo`, cada una con un esfuerzo propio opcional `+nivel`), si el agente principal falla antes de emitir texto (`limite_uso_proveedor`, `autenticacion_agente`, `agente_no_disponible`, `modelo_no_disponible`, `error_agente`, `error_protocolo_agente` o `tiempo_ejecucion_agotado`), o si su entrada no cabe en su contexto (`entrada_demasiado_grande`, sin ejecutarlo), el mismo trabajo sigue con el siguiente: queda un evento `aviso`, el trabajo muestra el agente que respondió y el tiempo límite cuenta por intento. Tras un salto, el campo `model` de las respuestas de las fachadas OpenAI y Anthropic indica el agente/modelo que respondió (sin salto, sigue siendo el que pidió la aplicación). En streaming, los primeros fragmentos salen antes de saber si habrá salto y llevan el modelo pedido; el que respondió aparece en el fragmento final de uso de OpenAI (`stream_options.include_usage`) y en el trabajo (`GET /api/v1/trabajos/{id}`). El trabajo guarda el respaldo que le queda, así que al reencolarlo o reintentarlo no repite agentes. La API nativa acepta `"respaldo": [...]` en la solicitud; las fachadas lo heredan del punto. Un respaldo con un agente que no existe, o fuera de `modelos_permitidos`, se rechaza al guardar el punto.

```yaml
puntos_acceso:
  - nombre: tienda
    instrucciones: Extrae el total y el proveedor de la factura adjunta.
    agente: claude
    modelo: sonnet
    respaldo: [codex, gemini/gemini-flash-latest]
    esquema:
      type: object
      properties: { total: { type: number }, proveedor: { type: string } }
      required: [total, proveedor]
```

### Presupuestos

Un punto de acceso, y la instancia entera, pueden tener un `presupuesto` diario y mensual en peticiones, tokens (entrada + salida) y USD (solo el coste que informa el agente: hoy Claude, y con suscripción es estimado). Los periodos son el día y el mes de calendario en la hora local del equipo; 0 o ausente es sin límite.

- Al aceptar una petición se comprueban el presupuesto de su punto y el de la instancia. Si alguno está agotado, responde **429** `presupuesto_agotado` en el formato de cada fachada (`rate_limit_error` en OpenAI y Anthropic), con `Retry-After` en segundos hasta el reinicio del periodo agotado. El respaldo no lo evita: el límite es de la API, no del agente.
- Tokens y USD se conocen al terminar cada trabajo, así que el límite reserva lo que gastarían los trabajos en curso al promedio de los terminados del periodo. Sigue siendo aproximado: un trabajo puede gastar más que el promedio.
- Cada `POST` a una API o instancia con presupuesto devuelve `X-Tramtai-Presupuesto: <n>`, el porcentaje usado del límite más cercano a agotarse (el del punto o el de la instancia) al recibir la petición, sin contarla: la que agota el límite puede ver 99. Sirve para que la aplicación frene antes del corte.
- Si se agotan varios límites a la vez (el diario y el mensual, o el del punto y el de la instancia), `Retry-After` apunta al reinicio más tardío.
- Los cruces del 80 % y del 100 % se registran una vez por periodo, para avisar por webhook.
- El presupuesto de la instancia cuenta todas sus peticiones: las de sus puntos, las de la clave principal y las de Probar. Reiniciar o eliminar un punto no le devuelve su consumo.

```yaml
puntos_acceso:
  - nombre: tienda
    presupuesto:
      diario: { peticiones: 500 }
      mensual: { tokens: 20000000, usd: 50 }
presupuesto:                     # el de la instancia (configuración global o local.yaml)
  mensual: { peticiones: 10000 }
```

La administración (`/api/v1/servicio`) y la API de cada instancia con `Tramtai-Instancia` se describen en [operacion.md](operacion.md).

`GET /api/v1/trabajos?limite=20` lista los trabajos recientes de la instancia, o los del punto de acceso si se llama desde uno; `&punto=<nombre>` filtra por un punto de acceso, `&grupo=<nombre>` por las APIs de ese grupo (según la configuración actual; un grupo sin APIs devuelve una lista vacía), `&origen=probar|terminal|cli|app` por origen y `&antes=<creado_en>` (RFC 3339) con `&antes_id=<id>` del último que ya se vio trae la página siguiente sin saltarse los creados en el mismo instante. Desde un punto de acceso, `grupo` y `origen` se ignoran. Cada trabajo indica su `origen`: `probar` (la sesión de la Web UI), `terminal` o `cli` (la clave de la instancia con el encabezado `X-Tramtai-Cliente`, que es solo una etiqueta y no da permisos), `app` (la clave sin ese encabezado) o `api` (un punto de acceso); vacío en los trabajos anteriores a v0.38.

`GET /api/v1/instancia/retencion` devuelve `horas_resultado`, `horas_metadatos`, `max_trabajos` y `uso` (`trabajos` de la instancia y `bytes` de la base); `PUT` con esos tres números los guarda (en la configuración global para la instancia global, en `local.yaml` para un proyecto), los aplica al momento y purga. Fuera de rango responde 400 `solicitud_invalida` con el motivo. Ambos piden una sesión de la Web UI o el token de servicio, como `/api/v1/instancia/predeterminado`. Desde la Web UI, `GET /api/v1/puntos` incluye `uso` por punto (`peticiones`, `errores` y `ultima`, de lo que conserva la retención) y `presupuestos` (por punto con presupuesto: `diario` y `mensual` con `limites`, `consumo` y `reinicio`).

### Claves de los puntos y de la instancia

Estas rutas exigen una sesión de la Web UI o el token de servicio (con `Tramtai-Instancia`); la clave de las aplicaciones no sirve.

| Método y ruta | Función |
|---|---|
| `POST /api/v1/puntos` | Crea un punto de acceso. La respuesta trae su `token`: es la única vez que se ve. |
| `POST /api/v1/puntos/{nombre}/token` | Regenera la clave del punto y la devuelve una sola vez (`{"token": "…"}`); la anterior deja de valer al momento. Ya no existe `GET` en esta ruta: Tramtai solo guarda la huella (`datos/tokens/acceso_<nombre>` contiene `sha256:…`). Las claves antiguas guardadas en claro se convierten solas a huella y siguen valiendo. |
| `DELETE /api/v1/puntos/{nombre}` | Elimina el punto y su clave. |
| `POST /api/v1/puntos/{nombre}/presupuesto/reiniciar` | Pone a cero el consumo del día y del mes en curso del punto (204). |
| `GET /api/v1/instancia/presupuesto` | `{"presupuesto": {…}}` con el consumo y los límites de la instancia, o `{}` si no tiene. |
| `POST /api/v1/instancia/presupuesto/reiniciar` | Pone a cero el consumo del día y del mes en curso de la instancia (204). |
| `GET /api/v1/instancia/token` | La clave de la instancia, para mostrarla en *API principal* (Mis APIs). |
| `POST /api/v1/instancia/token` | Regenera la clave de la instancia sin reiniciar y la devuelve. Si viene de una variable de entorno (`instancia_global.token_variable_entorno`), responde 400 `solicitud_invalida`: hay que cambiarla en la variable. |

## Límites de la entrada

Tras convertir los documentos, Tramtai mide lo que recibirá el agente (instrucciones, mensajes y documentos) y estima
sus tokens (caracteres ÷ 4). Antes de cada agente lo compara con su contexto menos `limites.reserva_salida_tokens`
(8.192): si no cabe, pasa al respaldo sin ejecutarlo y deja un `aviso` («La entrada (~45000 tokens; informe.pdf
~38000) no cabe en claude/haiku…; sigue con codex»); si no cabe en ninguno, el trabajo falla con
`entrada_demasiado_grande`.

- **Contexto de cada modelo**, en este orden: `limites.contexto_tokens` (una lista de `agente=tokens` o
  `agente/modelo=tokens`, por ejemplo `- codex/gpt-5.6-terra=272000`), lo que informa el agente (Claude Code en cada
  respuesta; la API de Gemini con `inputTokenLimit`) o, si no se conoce, no se comprueba.
- `GET /api/v1/modelos` indica el contexto conocido de cada modelo en `contexto_tokens` y su origen en
  `contexto_origen` (`configurado` o `informado`); el evento `intento` lo lleva en `contexto_tokens`.
- En la API nativa no es un error HTTP: el trabajo termina `fallido` con `entrada_demasiado_grande` (también en
  `POST /ejecutar`, que responde 200 al aceptar). Las fachadas responden 400 (`context_length_exceeded` en OpenAI).
- Una petición que no cabe en ningún agente no llega a ejecutarse: no gasta tokens ni cuenta como petición del
  presupuesto.

## Errores

API nativa: `{"error": {"codigo": "…", "mensaje": "…"}}`. Las fachadas usan el formato de error de OpenAI o de Anthropic, con el código de Tramtai en `code` (OpenAI).

| HTTP | Códigos |
|---|---|
| 400 | `solicitud_invalida`, `parametro_no_soportado`, `modelo_ambiguo`, `modelo_no_permitido`, `modelo_no_disponible`, `modelo_no_configurado`, `carga_invalida`; en las fachadas también `entrada_demasiado_grande` (OpenAI: `context_length_exceeded`) |
| 401 / 403 | `no_autorizado` / `origen_no_permitido`, `contexto_no_permitido` |
| 404 | `no_encontrado` |
| 409 | `conflicto_idempotencia`, `trabajo_no_terminado`, `trabajo_sin_salida` |
| 410 | `entrada_no_disponible`, `resultado_expirado` |
| 422 | `salida_no_cumple_esquema`, `conversion_fallida` |
| 429 | `cola_llena` y `presupuesto_agotado` (con `Retry-After`), `limite_uso_proveedor` |
| 507 | `cuota_cargas_agotada` (las cargas guardadas llegan a `conversion.cuota_cargas_mb`) |
| 502 / 503 / 504 | `error_agente`, `autenticacion_agente` / `agente_no_disponible` / `tiempo_ejecucion_agotado` |
