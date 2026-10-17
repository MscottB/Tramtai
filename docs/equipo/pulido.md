# Pulido: cuenta atrás al instalar, conectar cada API y todos los pendientes

Diseño acordado el 2026-10-17. Antes del release de Probar, el usuario pidió:

1. Que al instalar una actualización la app no se cierre de golpe: una cuenta atrás con «Reiniciar ahora».
2. Que Mis APIs explique cómo conectar cada API (no solo la principal) y ordene sus acciones.
3. Cerrar todos los pendientes anotados de las revisiones (`docs/equipo/pendientes.md`), y desde ahora no aplazar los
   menores: lo que encuentre la revisión se arregla en el mismo release.

Fuera de alcance, porque piden una decisión o un recurso del usuario: hacer público el repositorio (CI y Pages),
el certificado de firma de código, las claves reales (M0b, Gemini, Antigravity), probar la app de escritorio en un Mac
o en Linux, el historial permanente y la memoria entre conversaciones. El pendiente de webhooks va en su release.

## 1. Cuenta atrás al instalar

- `Servicio.actualizar` (descarga y comprobación) ya no ejecuta el instalador al terminar: pasa a la fase `listo`
  con `instala_en` (ahora + 10 s) y espera el primero de estos:
  - el plazo vence → instala, como hoy;
  - `POST /ui/actualizar/instalar` → instala al momento;
  - `POST /ui/actualizar/cancelar` → no instala, borra la carpeta de la descarga y la fase vuelve a vacía.
- Las dos rutas exigen la sesión local y el CSRF, como `POST /ui/actualizar`. Fuera de la fase `listo` responden 409.
- La cuenta atrás la lleva el servicio: se cumple aunque se cierre la ventana.
- Web UI: en la fase `listo` el diálogo dice «Tramtai se reiniciará en {n} s» (calculado con `instala_en`), con
  **Reiniciar ahora** y **Cancelar**; el diálogo deja de ser persistente en esa fase.
- `tramtai actualizar` no cambia: ya pide confirmación y no usa el servicio.

## 2. Mis APIs: conectar cada API y acciones claras

- **Fila:** nombre, estado, grupo, URL, uso y presupuesto como hoy, y un solo botón **Conectar**. Salen de la fila
  «Ajustes de la API» y el menú «⋯»; el componente `MenuAcciones` deja de usarse y se borra. Pulsar la fila también
  abre el detalle.
- **Detalle de la API** (diálogo; a pantalla completa en el móvil), en tres bloques:
  - **Conectar:**
    - las URL base según los contratos de la API: OpenAI (`<url>/openai/v1`), Anthropic (`<url>/anthropic`) y la API
      propia (`<url>/api/v1`), cada una con su botón de copiar;
    - la clave: Tramtai guarda solo su huella, así que no se puede volver a mostrar. El detalle lo dice («La clave se
      ve una vez, al crearla o regenerarla») y ofrece **Regenerar clave**, que avisa de que la anterior deja de valer,
      la muestra una vez y la usa en los ejemplos;
    - los ejemplos para copiar de la API principal (Python y Node con OpenAI, Python con Anthropic, curl, variables
      de entorno), con la URL, el modelo y, si la hay, la clave de esta API (si no, un marcador `<clave de tienda>`).
  - **Acciones:** Editar ajustes (el diálogo de ajustes actual), Abrir en Probar, Ver peticiones y Activar o detener.
  - **Zona de peligro**, aparte y en color de error: Eliminar la API, con confirmación.
- **Nueva API:** al crearla se abre su detalle con la clave recién creada en el ejemplo (en vez del aviso actual).
- Los ejemplos de la API principal y los de cada API salen de la misma función, con base, clave y modelo.

## 3. Pendientes de las revisiones

### Grupos e historial

- La purga con tope calcula el corte con `SELECT terminado_en … ORDER BY terminado_en DESC LIMIT 1 OFFSET ?` y borra
  `terminado_en < corte` (con sus claves de idempotencia y eventos), en vez de `NOT IN`. Los empatados en el corte
  pueden dejar unos pocos de más.
- La conversión única a `auto_vacuum` deja una línea en el registro del servicio y hace
  `PRAGMA wal_checkpoint(TRUNCATE)` después del `VACUUM`.
- «Ver más» descarta su respuesta si el filtro cambió mientras cargaba y cuenta en `enVuelo`.
- Mis APIs muestra la barra de filtros también con una sola API si hay un filtro activo.
- Ajustes · Historial: el registro se edita en días con decimales permitidos (por ejemplo 1,5) y se guarda en horas
  sin redondear a días enteros; un campo vacío no se envía (se avisa «completa los tres valores»).
- Plurales: «1 API», «1 activa», «1 día» en es y en (`mapa.grupo`, `mapa.activas`, `inicio.tableroPeriodo`,
  `inicio.peticionesTexto`, `puntos.uso`).
- El mapa en el móvil muestra las activas de cada grupo.
- `fijarRetencion` escribe en el registro del servicio el error de su purga, como `iniciarPurga`.
- `Purgar` cuenta en `eliminados` también lo que borra el tope.
- En 375 px, «Activar todas» y «Detener todas» pasan a botones de icono con su nombre accesible, en la misma línea.

### Actualizaciones

- `Novedades` guarda la lista de versiones de GitHub 10 minutos en memoria y filtra sobre ella; la ruta responde un
  mensaje fijo («GitHub no está disponible») en vez del error de red.
- El diálogo de novedades se abre al momento con un indicador mientras llegan las notas.
- Con una sola versión en la lista que no sea la instalada, se muestra su número.
- `aria-labelledby` en los diálogos de `App.vue` (novedades, actualizar y los demás que tengan título).
- El comentario del paquete `actualizacion` menciona la lista de versiones.

### Rediseño

- El logo-botón tiene un nombre accesible que empieza por el texto visible («Tramtai, ir a Inicio»), WCAG 2.5.3.
- Página del producto: textos alternativos de las capturas en inglés cuando la página está en inglés; niveles de
  título sin saltos; `theme-color` también para el tema oscuro.
- Se borran `.ayuda-campo` y la clave `puntos.activar`, que no se usan.
- Probar: se quita el doble relleno de la columna de la receta.

### Presupuestos

- Al arrancar, el consumo de una API que ya no está en la configuración (borrada con el servicio detenido o a mano) se
  borra como si se hubiera eliminado desde Tramtai.
- El uso de un trabajo suma los tokens y el coste que informaron los intentos fallidos antes del salto de respaldo, y
  el presupuesto los cuenta.

### Documentación desactualizada

- La Ayuda dice que el arranque con el sistema existe en Windows, macOS y Linux (como ya hace el código), y la tarea
  de `pendientes.md` que decía lo contrario se quita.

## Documentación

- `docs/operacion.md`: la cuenta atrás al instalar; el detalle de cada API en Mis APIs.
- `docs/equipo/pendientes.md`: se vacían las listas de pendientes de las revisiones y queda solo lo que necesita una
  decisión del usuario; fila del release en la hoja de ruta.

## Pruebas

- `internal/servicio`: la fase `listo` con `instala_en`; instalar al vencer, con «Reiniciar ahora» y no instalar con
  «Cancelar» (con un instalador simulado: `actualizacion.Instalar` se sustituye en la prueba); 409 fuera de `listo`;
  sin sesión local, 401.
- `internal/persistencia`: la purga con corte por `OFFSET` (mismos casos que `TestPurgarConTope` y la clave de
  idempotencia); `Purgar` cuenta lo del tope.
- `internal/actualizacion`: la caché de 10 minutos (una sola consulta para dos llamadas).
- `internal/planificador`: el uso suma los intentos fallidos antes del salto.
- `internal/servicio`: el consumo de una API ausente de la configuración se borra al arrancar.
- Navegador: cuenta atrás con Reiniciar ahora y Cancelar; detalle de cada API (URLs, Regenerar clave, ejemplos,
  acciones, eliminar); Nueva API abre el detalle; plurales; móvil y escritorio, claro y oscuro.
