# Recetas y respaldo entre agentes

Diseño acordado el 2026-10-15. Objetivo: que una persona pruebe una tarea en Probar (instrucciones, agente,
esquema JSON) y la publique en un clic como API con token, y que esa API siga respondiendo cuando un agente
llega a su límite o falla.

## Alcance

- **Receta = punto de acceso ampliado** con `esquema` y `respaldo`. No hay una entidad aparte: para quien usa
  la app sigue siendo "una API".
- **Respaldo entre agentes** en el planificador, sin cambiar el identificador del trabajo.
- **Probar como taller**: edita la receta completa y la publica ("Guardar como API") o actualiza la que abrió
  desde Mis APIs. Mis APIs también crea y edita recetas con el mismo componente.

Fuera de alcance: plantillas con variables, recetas reutilizadas por varios puntos, comparar agentes, MCP,
webhooks y presupuestos.

## 1. Modelo de datos

`ConfigPunto` (internal/servicio/puntos.go) gana:

| Campo | Tipo | Regla |
|---|---|---|
| `esquema` | `map[string]any` (JSON Schema escrito como YAML) | Se compila con `salidas.Compilar` en `Normalizar`; error de configuración si no es válido. |
| `respaldo` | `[]string` | Máximo 3; cada entrada `agente`, `agente/modelo` o `perfil/nombre`; sin repetir el modelo principal. |

`api.Punto` copia ambos campos (el esquema ya serializado a `json.RawMessage`). `EstadoPunto` los devuelve
para la Web UI.

`nucleo.SolicitudEjecucion` gana `Respaldo []string` (`json:"respaldo,omitempty"`). `Validar` aplica las
mismas reglas de formato y el máximo de 3. Forma parte de la huella, como el resto de la solicitud.

### Aplicación en `Servidor.ajustar` (internal/api/punto.go)

- Si la solicitud no trae esquema y el punto sí: `Salida.Esquema = punto.Esquema` y `Salida.Formato = "json"`.
- Si la solicitud no trae respaldo: `Respaldo = punto.Respaldo`.
- Con `modelos_permitidos`, cada entrada del respaldo debe estar permitida (`modelo_no_permitido` si no).

Las fachadas OpenAI y Anthropic no exponen `respaldo`: lo heredan del punto. La API nativa lo acepta.

### Web UI (internal/servicio/web.go)

- `crearPuntoUI` acepta `esquema` y `respaldo` (ya decodifica `ConfigPunto` entero).
- `puntoUI` PATCH acepta `esquema` (`null` o `{}` lo quita) y `respaldo`, y valida con `Normalizar` antes de
  escribir.

## 2. Respaldo en el planificador

En `Planificador.procesar`, la toma de semáforos y la ejecución del adaptador pasan a ser un bucle sobre los
candidatos: el agente principal y después cada entrada de `Respaldo`.

Un intento salta al siguiente candidato si se cumplen las dos condiciones:

1. El error es uno de: `limite_uso_proveedor`, `autenticacion_agente`, `agente_no_disponible`,
   `modelo_no_disponible`, `error_agente`, `error_protocolo_agente`, `tiempo_ejecucion_agotado`.
2. El intento no emitió texto (el agrupador lleva la cuenta).

Nunca salta con la cancelación, el cierre del servicio ni un esquema no cumplido (este siempre llega tras haber
texto). Si no quedan candidatos, el trabajo falla con el error del último intento.

Al saltar:

- Se sueltan los semáforos del intento (punto, agente, plaza) y se vuelven a tomar para el siguiente, en el
  mismo orden de hoy.
- Se resuelve el modelo del candidato con `resolverModelo`; si no se puede resolver, se pasa al siguiente.
- Un método nuevo del almacén (`CambiarAgente(id, agente, modeloEnviado)`) actualiza el agente de la solicitud
  guardada y `modelo_enviado`, para que el historial, el uso y el registro digan quién respondió.
- Se añade un evento `aviso`: «<agente> falló (<código>); sigue con <siguiente>».
- El tiempo límite de ejecución cuenta por intento. El plazo en cola no se reinicia.

## 3. Interfaz

### `EditorReceta.vue` (web/src/componentes)

Componente compartido con: instrucciones, agente/modelo (`SelectorModelo`) con "Forzar", esfuerzo
(`SelectorEsfuerzo`), respaldo (hasta 3, ordenado) y salida estructurada (interruptor más esquema JSON con
validación al escribir). Emite un objeto `Receta` que Probar convierte en solicitud y Mis APIs en punto.

### Probar (`Ejecutar.vue`)

- Columna izquierda: `EditorReceta`, después el mensaje de prueba y los adjuntos; "Avanzadas" conserva perfil
  y tiempo.
- Las instrucciones viajan como `instrucciones` de la solicitud y el respaldo como `respaldo`.
- Botón "Guardar como API": diálogo con nombre y grupo; crea el punto y muestra URL, token y ejemplo `curl`.
- Abierta desde Mis APIs ("Abrir en Probar"), muestra "Editando: <nombre>" y el botón pasa a
  "Actualizar <nombre>" (PATCH).
- La respuesta muestra los avisos del trabajo (salto de respaldo) y el agente que respondió.

### Mis APIs (`Accesos.vue`)

- "Nueva API" y "Editar" usan `EditorReceta`.
- Etiquetas "JSON" y "Respaldo: …" en cada API; botón "Abrir en Probar".

### Textos

`es.ts` y `en.ts`, página Ayuda (sección "Recetas y respaldo", incluido que el tiempo límite cuenta por
intento) y README.

## 4. Pruebas

Go:

- `Normalizar`: esquema no válido, respaldo con formato no válido o más de 3.
- `ajustar`: aplica esquema y respaldo del punto; la solicitud prevalece; respaldo fuera de
  `modelos_permitidos` da `modelo_no_permitido`.
- Planificador con dos `Simulado`: el primero falla por límite y responde el segundo (agente y modelo
  actualizados, evento `aviso`); fallo tras emitir texto no salta; cancelación no salta; sin candidatos queda
  el error del último.
- PATCH de la Web UI con `esquema` y `respaldo`.

Web: `vue-tsc` y compilación; prueba en el navegador integrado con el servicio en marcha: crear una receta
desde Probar, llamarla con `curl` y ver el aviso de respaldo.
