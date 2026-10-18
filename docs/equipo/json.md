# Respuestas JSON mejoradas (v0.46)

Diseño del 2026-10-18. Hoy pedir una respuesta con forma exige escribir un JSON Schema a mano en un cuadro de texto, y
si la respuesta no lo cumple el trabajo falla con un único mensaje técnico.

Decisiones del usuario:
- **Las cuatro mejoras:** plantillas listas, editor de campos, validación que explique el problema y reintento.
- **El reintento no es automático:**
  - se configura cuántas veces se corrige (0 por defecto, es decir, nunca);
  - y, si falla, Tramtai lo sugiere en lugar de hacerlo solo.

Este release reordena la hoja de ruta: OpenCode pasa a la v0.47 y Hermes Agent a la v0.48.

Fuera de alcance:
- JSON Schema completo en el editor visual (`oneOf`, `$ref`, patrones…): esos esquemas se siguen escribiendo en JSON.
- Plantillas propias guardadas por el usuario: una API ya guarda su esquema, y «Guardar como API» sirve para eso.

## 1. Plantillas listas

Un menú «Usar una plantilla» junto a «Respuesta con forma», en Probar y en Mis APIs (los dos usan `EditorReceta`).
Cada plantilla trae:
- su esquema;
- unas instrucciones sugeridas, que solo se ponen si las de la receta están vacías;
- un ejemplo de respuesta.

| Plantilla | Campos |
|---|---|
| Extraer datos | Los que elijas en el editor de campos. Empieza con `nombre`, `fecha` y `total`. |
| Clasificar | `categoria` (una de una lista que editas), `confianza` (0 a 1) y `motivo`. |
| Resumen en puntos | `resumen` (texto) y `puntos` (lista de textos, de 3 a 7). |
| Sentimiento | `sentimiento` (positivo, neutral o negativo) y `motivo`. |
| Sí o no | `respuesta` (sí o no) y `motivo`. |
| Lista de tareas | `tareas`: lista de `{tarea, responsable?, fecha?}`. |
| Entidades | `personas`, `organizaciones`, `lugares` y `fechas` (listas de textos). |

- Las plantillas viven en `web/src/plantillas.ts`, en español e inglés: los nombres de los campos van en el idioma de la
  interfaz al aplicarla.
- Aplicar una plantilla sustituye el esquema, tras confirmar si ya había uno.

## 2. Editor de campos

«Respuesta con forma» pasa a tener dos pestañas:
- **Campos** (predeterminada);
- **JSON**: el cuadro de hoy, para esquemas avanzados.

En **Campos**, cada campo tiene:
- nombre;
- tipo: texto, número, entero, sí/no, fecha, una de una lista, lista de (textos, números u objetos) u objeto;
- si es obligatorio;
- una descripción, que llega al modelo y ayuda a rellenarlo bien.

También:
- Los objetos y las listas de objetos tienen sus propios campos, hasta tres niveles.
- Se añaden, se quitan y se reordenan.
- El editor genera un esquema con `additionalProperties: false`, todos los campos en `properties` y los obligatorios en
  `required`. Los dos modos se sincronizan.
- Un esquema escrito en **JSON** que el editor no sabe representar se queda en esa pestaña, con el aviso «Este esquema
  usa opciones avanzadas: edítalo en JSON». No se pierde nada.
- Una vista previa muestra un ejemplo de respuesta con esa forma.

## 3. Validación que explica el problema

**Al escribir el esquema:**
- se compila en el servicio con la misma biblioteca que valida (`POST /api/v1/esquemas/validar`, sin ejecutar nada);
- el error se muestra junto al campo: «el esquema no es JSON válido (línea 4)», «`tipo` no es un tipo de JSON
  Schema»…

**Al validar la respuesta,** `salidas.Validar` devuelve también una lista de problemas a partir de los errores de
`santhosh-tekuri/jsonschema`:
- Cada problema tiene `campo` (ruta legible, `tareas[2].fecha`), `problema` (`falta`, `tipo`, `valor_no_permitido`,
  `sobra`, `formato`, `tamaño`) y un texto en es y en.
- Ejemplos: «Falta el campo `total`», «`total` debería ser un número y llegó un texto», «`categoria` debe ser una de:
  consulta, queja, otro», «Sobra el campo `notas`».
- El trabajo fallido guarda la lista (`errores_salida`) y la API nativa la devuelve con el error.
- `salida_no_cumple_esquema` sigue siendo el código; el mensaje pasa a ser el primer problema y cuántos más hay.

**En Probar:**
- se ven la lista de problemas y la respuesta tal como llegó (Probar conserva siempre la salida inválida);
- se marca cada campo con problemas.

## 4. Corregir: configurable y sugerido

- **Nuevo ajuste `reintentos_salida`**, de 0 a 3, con 0 como predeterminado. Está:
  - en la receta (Probar y Mis APIs);
  - en el punto de acceso;
  - en la solicitud nativa (`salida.reintentos`);
  - en las rutas compatibles con OpenAI y Anthropic, el de la API.
- **Cómo se corrige:** si la respuesta no cumple y quedan reintentos, se pide de nuevo al mismo agente y modelo con:
  - la conversación;
  - la respuesta inválida, como turno del asistente;
  - un mensaje con los problemas: «Tu respuesta no cumple el formato pedido: … Devuelve solo el JSON corregido».
- **Registro y consumo:**
  - cada corrección es un evento `intento` más («corrección 1 de 2»);
  - sus tokens y su coste cuentan para el presupuesto;
  - como petición cuenta una sola vez, como hoy un respaldo.
- **No salta al respaldo:** la corrección no cambia de agente. Si se agotan las correcciones, el trabajo falla con los
  problemas del último intento.
- **Sugerido:** con 0 reintentos, Probar ofrece «Corregir» junto al error. Hace una corrección manual
  (`POST /api/v1/trabajos/{id}/reintentar` con `{"corregir": true}`), que crea un trabajo vinculado como el reintento de
  hoy. Solo existe si el trabajo conservó su salida.
- **Aviso de coste:** el control de reintentos dice que cada corrección es otra petición al agente. En Copilot, por
  ejemplo, gasta créditos.

## 5. Documentación

- **Ayuda:**
  - la guía «Respuestas con forma» (plantillas, campos, qué hacer si no cumple, reintentos);
  - el error `salida_no_cumple_esquema` con los pasos nuevos.
- **`docs/api.md`:** `salida.reintentos`, `errores_salida`, `POST /api/v1/esquemas/validar` y `corregir` en reintentar;
  también en `openapi.json`.
- **README (es/en) y la web:** la tarjeta «Respuestas con forma».
- **`pendientes.md`:**
  - la fila v0.46;
  - OpenCode pasa a v0.47 y Hermes a v0.48 (también en `nuevo-agente.md` y la spec de OpenCode).
- **Notas del release:** cortas y en los dos idiomas.

## Pruebas

- **Go:**
  - `salidas` traduce cada tipo de error a su problema y su ruta;
  - el planificador corrige hasta `reintentos_salida`, con la conversación y los problemas, y no salta al respaldo;
  - el uso se suma;
  - `esquemas/validar` (válido, JSON roto, tipo inválido, tope de tamaño);
  - reintentar con `corregir` (y sin salida conservada, error claro);
  - punto de acceso con `reintentos_salida`.
- **Web:**
  - del editor de campos al esquema y de vuelta, con un esquema avanzado que se queda en JSON;
  - cada plantilla genera un esquema que compila.
- **Real:** una corrección con Claude haiku y un esquema que suele fallar a la primera, y una con Copilot.
- **Navegador:**
  - Probar y Mis APIs con plantillas, campos, error explicado, «Corregir» y reintentos;
  - en móvil y escritorio, claro y oscuro, es y en.
