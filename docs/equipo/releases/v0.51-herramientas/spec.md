# Herramientas y guía del desarrollador (v0.51)

Diseño del 2026-10-20, acordado con el usuario por secciones. Hoy las herramientas (`tools`) funcionan solo por las
fachadas OpenAI (`chat/completions`, `responses`) y Anthropic (`messages`): Tramtai las **emula** en
`internal/api/herramientas.go`. Le describe al agente las funciones, le pide la respuesta como un sobre JSON
`{"content", "tool_calls"}` con la salida estructurada de su CLI y lo traduce a `tool_calls` o `tool_use`. Todo ocurre
en la capa de la fachada, durante una petición que espera la respuesta: si los argumentos no cumplen el esquema de la
herramienta, la respuesta es 422 sin corrección posible. La API propia (`/api/v1/trabajos`) no las admite, la app no
las menciona y solo `docs/api.md` las explica, en una sección corta.

Decisiones del usuario:
- La **guía del desarrollador** es para **quien conecta su app** a Tramtai (la del equipo ya está en `docs/equipo`).
- La **API propia también tendrá herramientas**, con un **solo motor** para las tres APIs (enfoque A: sin ruta
  síncrona nueva y sin migrar la base de datos).
- La guía vive en el **sitio** (página «Desarrolladores», es/en) y en el **repo** (`docs/desarrolladores.md`); la app
  enlaza a ella.
- La prueba de punta a punta **no depende de un agente concreto**: recorre todos los que haya disponibles y salta los
  que falten.

Éxito: quien no conoce las herramientas entiende la ida y vuelta (tu app ofrece funciones → el agente pide una → tu app
la ejecuta → le devuelve el resultado → el agente responde) y la hace funcionar a la primera copiando el ejemplo de su
lenguaje, por cualquiera de las tres APIs.

## 1. Herramientas en la API propia

**Primer turno** (`POST /api/v1/trabajos`):

```json
{
  "mensajes": [{ "rol": "usuario", "contenido": "¿Qué tiempo hace en Santiago?" }],
  "herramientas": [{
    "nombre": "clima",
    "descripcion": "El tiempo actual de una ciudad",
    "parametros": { "type": "object", "properties": { "ciudad": { "type": "string" } }, "required": ["ciudad"] }
  }],
  "eleccion_herramienta": "auto"
}
```

- `herramientas`: lista de funciones con `nombre` (obligatorio y único), `descripcion` y `parametros` (JSON Schema
  2020-12, opcional).
- `eleccion_herramienta`: `"auto"` (predeterminada), `"ninguna"`, `"obligatoria"` o `{"nombre": "clima"}`.
- `llamadas_paralelas`: `true` por defecto; con `false`, como mucho una llamada por respuesta.

**El trabajo terminado** (`GET /api/v1/trabajos/{id}`) trae, además de lo de siempre:

```json
"salida": "",
"llamadas": [{ "id": "llamada_0", "nombre": "clima", "argumentos": { "ciudad": "Santiago" } }]
```

- `salida` lleva solo el texto para el usuario, nunca el sobre JSON interno.
- `argumentos` va como **objeto** (no como texto, a diferencia de OpenAI), ya validado contra `parametros`.
- Un trabajo sin herramientas no cambia: no trae `llamadas`.

**Segundo turno:** la app ejecuta la función y reenvía la conversación con el resultado (y las mismas `herramientas`):

```json
"mensajes": [
  { "rol": "usuario", "contenido": "¿Qué tiempo hace en Santiago?" },
  { "rol": "asistente", "contenido": "", "llamadas": [{ "id": "llamada_0", "nombre": "clima", "argumentos": { "ciudad": "Santiago" } }] },
  { "rol": "herramienta", "id_llamada": "llamada_0", "contenido": "18 °C, despejado" }
]
```

`"es_error": true` en un mensaje `herramienta` marca un resultado fallido.

**Reglas** (las mismas de las fachadas):
- Solo funciones. No se combinan con una respuesta en JSON (`salida.formato: json` o `salida.esquema`): 400
  `parametro_no_soportado`.
- Definiciones inválidas (sin nombre, repetidas, esquema que no compila, elección que nombra una herramienta que no
  está, elección que exige herramienta sin ninguna): 400 `solicitud_invalida` al aceptar.
- Un mensaje `herramienta` sin `id_llamada`, o con uno que no aparece en una llamada anterior: 400 `solicitud_invalida`.
- Si el agente no responde con el sobre, pide una herramienta que no existe, da argumentos que no cumplen su esquema o
  no llama a ninguna cuando la elección lo exige, la salida **no cumple**: entra en el ciclo de correcciones
  (`salida.reintentos`, también el de la receta) y, al final, el trabajo falla con `salida_no_cumple_esquema`. El
  reintento y «Corregir» (`POST /trabajos/{id}/reintentar` con `{"corregir": true}`) funcionan como con un esquema.
- Con herramientas no hay texto en vivo: los eventos dicen el estado y la respuesta llega al terminar (igual que hoy en
  las fachadas).
- El agente nunca ejecuta nada.
- Las herramientas van **en cada petición**: las recetas (APIs por aplicación) no las guardan; siguen aportando
  instrucciones, agente, respaldo y correcciones.

## 2. Un solo motor

**Paquete nuevo `internal/herramientas`** con la lógica que hoy está en `internal/api/herramientas.go`:

- `Validar(sol)`: comprueba las definiciones, la elección y la conversación (la regla de `id_llamada`).
- `Preparar(sol) nucleo.SolicitudEjecucion`: añade a las instrucciones la descripción de las funciones (en inglés, como
  hoy), fija el sobre JSON como esquema de salida y escribe las llamadas y resultados anteriores como turnos que el
  agente entiende (los de hoy: el sobre para el asistente y «Tool result …» para el resultado).
- `Interpretar(sol, salida) (texto string, llamadas []nucleo.Llamada, err error)`: lee el sobre, valida cada llamada y
  devuelve un error de `salidas` con sus **problemas**, para que la corrección se los pase al agente como pasa hoy con
  un esquema.

**Tipos en el núcleo** (`internal/nucleo`), para que viajen con la solicitud: `Herramienta{Nombre, Descripcion,
Parametros}`, `Llamada{ID, Nombre, Argumentos json.RawMessage}`; `SolicitudEjecucion` gana `Herramientas`,
`EleccionHerramienta` y `LlamadasParalelas`; `Mensaje` gana `Llamadas`, `IDLlamada` y `EsError`. La solicitud ya se
guarda entera con el trabajo: **no hay migración** y los trabajos anteriores se leen igual.

**Dónde actúa:**

| Momento | Quién | Qué hace |
|---|---|---|
| Aceptar el trabajo | API (las tres) | `Validar`; error 400 antes de encolar |
| Ejecutar | planificador | `Preparar` arma lo que recibe el agente (como ya se hace con documentos y correcciones); la solicitud guardada no cambia |
| Validar la salida | planificador (`validarSalida`) | `Interpretar`; si no cumple, el ciclo de correcciones y respaldo de siempre |
| Leer el trabajo | API propia | `salida` = el texto y `llamadas`, con `Interpretar` sobre la salida ya validada |
| Responder | fachadas | las mismas llamadas como `tool_calls` (OpenAI) o `tool_use` (Anthropic) |

Las fachadas quedan como **traductoras** entre su formato y los campos del núcleo. Sus pruebas actuales siguen pasando
sin cambiar lo que esperan; lo único que cambia para ellas, y a mejor, es que unos argumentos inválidos ahora pasan por
las correcciones antes de dar 422.

## 3. Guía del desarrollador y ejemplos

**Dónde:** `site/desarrolladores.html` (es/en, con el estilo y el índice lateral de `ayuda.html`, en el menú del sitio)
y `docs/desarrolladores.md`. Los enlazan el README, la Ayuda de la app, Mis APIs y la página de Ayuda del sitio.

**Temas**, cortos, con pestañas **Python · JavaScript · Go · curl** donde haya código:

1. **Empezar:** URL, clave (`TRAMTAI_URL`, `TRAMTAI_CLAVE`) y qué API elegir (enlace a la comparación).
2. **Tu primer mensaje** y la respuesta en vivo.
3. **Archivos:** formatos admitidos, cómo se envían por cada API y sus límites.
4. **Respuestas en JSON** con esquema.
5. **Herramientas**, el centro del release:
   - qué son, con un diagrama de la ida y vuelta;
   - el ciclo completo en los cuatro lenguajes por la URL de OpenAI con los SDK oficiales (Python `openai`,
     JavaScript `openai`, Go `github.com/openai/openai-go`) y en curl;
   - el mismo ciclo por la API propia (Python y curl) y en qué cambia el formato de Anthropic;
   - qué agentes las atienden (la tabla comprobada de la sección 4).
6. **Trabajos largos** (solo la API propia): seguir en vivo, retomar tras un corte, cancelar y reintentar.
7. **Errores y reintentos:** los códigos, `Retry-After`, la `Idempotency-Key` y qué hacer ante cada uno.
8. **Límites:** `GET /limites` para conocerlos antes de enviar.

**Ejemplos que siempre funcionan:** son archivos reales en `docs/ejemplos/{python,javascript,go,curl}/` que leen
`TRAMTAI_URL` (por defecto `http://127.0.0.1:17373`) y `TRAMTAI_CLAVE`. La página y el documento los muestran tal cual
(el workflow de la página los copia al publicar, sin duplicarlos a mano). Un **job nuevo del CI, «Ejemplos»**, corre
una prueba de Go con el tag `ejemplos`: levanta el servidor de la API (el mismo de las pruebas de `internal/api`) con
el agente simulado que responde texto y, si hay herramientas, una llamada; y **ejecuta cada ejemplo** con
`TRAMTAI_URL` y `TRAMTAI_CLAVE` apuntando a él. Si uno se rompe, el CI lo dice antes de publicar.

**En la app:**
- **Mis APIs:** los ejemplos para copiar suman **Go** y un ejemplo de herramientas.
- **La tabla «Qué permite cada una»** (`TablaApis`) gana la fila «Herramientas (`tools`)»: sí en las tres.
- **Ayuda:** una pregunta frecuente «¿Cómo uso herramientas?» que explica la idea en dos frases y enlaza a la guía; la
  Ayuda no crece más (la v0.52 la recorta).

## 4. Pruebas y documentación

### Pruebas (escritas antes del código)

- **`herramientas`:** las pruebas de preparar e interpretar pasan de `api` a este paquete; más las de la conversación
  (`llamadas`, rol `herramienta`, `es_error`, `id_llamada` desconocido).
- **Planificador:** unos argumentos inválidos piden una corrección al mismo agente y, si vale, el trabajo termina
  bien; sin correcciones falla con `salida_no_cumple_esquema`; el respaldo funciona igual con herramientas.
- **API propia:** primer turno con `llamadas`; segundo turno con el resultado y respuesta final; los 400 de la sección
  1; `salida` sin el sobre; un trabajo sin herramientas no cambia.
- **Fachadas:** sus pruebas actuales pasan sin tocar lo que esperan.
- **Ejemplos:** el job «Ejemplos» del CI, en Linux, con Python, Node y Go.
- **De punta a punta con agentes reales:** una prueba con el tag `canario` que detecta los agentes instalados y con
  sesión o clave y, con **cada uno**, recorre el ciclo de herramientas por OpenAI, Anthropic y la API propia. Los que
  falten se saltan avisando, sin fallar. En el equipo de desarrollo se ejecuta con Claude, Codex y GitHub Copilot, y
  con Gemini si hay clave. Su resultado da la tabla «qué agentes atienden herramientas, con qué modelo se probó» de la
  guía. El CI no la ejecuta (necesita cuentas).

### Documentación (es/en donde la lee el usuario)

- `docs/api.md`: herramientas en la API propia (campos, reglas y errores) y la sección actual de las fachadas,
  remitiendo a la guía.
- `internal/api/openapi.json`: los campos nuevos de la solicitud, el mensaje y el trabajo.
- `docs/arquitectura.md`: el paquete `herramientas` y la tabla de la sección 2.
- La guía nueva (repo y sitio), enlazada desde el README, la Ayuda y Mis APIs.
- `docs/equipo/pendientes.md`: v0.51 ✅ y lo que queda fuera.
- Notas del release en español e inglés.

## Fuera de la v0.51

- Herramientas en **Probar** y guardadas en las **recetas**.
- Ejecutar herramientas en el propio Tramtai o vía **MCP**: el agente sigue sin ejecutar nada.
- Texto en vivo mientras se usan herramientas.
- La CLI y la terminal: `tramtai ejecutar` no ofrece herramientas (no hay una app que las ejecute).

La versión la calcula el script: es una función nueva, **v0.51.0**.
