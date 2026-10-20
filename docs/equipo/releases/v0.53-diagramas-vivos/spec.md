# Diagramas vivos (v0.53)

Diseño del 2026-10-20, acordado con el usuario por secciones. Hoy:

- **Mapa de Inicio** (`web/src/componentes/MapaRed.vue`): tus APIs → Tramtai → una línea por agente (de color si está
  listo, punteada si no). Es estático: Inicio pide los datos cada 5 s (`Panel.vue`) pero el mapa no muestra los
  trabajos que están pasando.
- **Ruta de un trabajo** (`Ruta.vue`): 5 paradas (cola → preparación → ejecución → validación → llegada) en el detalle
  del Historial y en Probar. Cada trabajo tiene su SSE (`GET /api/v1/trabajos/{id}/eventos`), pero no hay uno de la
  instancia.
- **Portada del sitio** (`site/index.html`, `.mapa`): dibuja apps → Tramtai → las cinco líneas, pero el «tranvía»
  animado se detiene en Tramtai, y el ejemplo de código de al lado es fijo.

Decisiones del usuario:
- El mapa vivo sirve **para las dos cosas**: ver qué pasa ahora (monitor) y entender el recorrido (seguir una petición).
- Los datos llegan por un **flujo de eventos de la instancia** (SSE), no por consultas más seguidas.
- En la portada, el recorrido **llega a los agentes** y se turnan **4 ejemplos** con su código.
- Los **recorridos guiados con driver.js** van en una release propia, la **v0.54**; lo que seguía se corre un número.

Éxito: al enviar una petición (desde Probar, la terminal o una app) se ve en Inicio un tranvía que sale de su API,
pasa por Tramtai y recorre la línea de su agente etapa por etapa, también si dura dos segundos o si salta al respaldo;
y quien visita la web entiende en unos segundos qué hace Tramtai mirando cómo los ejemplos llegan a cada agente.

## 1. El flujo de eventos de la instancia (servicio)

**`GET /api/v1/eventos`** (SSE), con la misma autenticación que el resto de `/api/v1`:

- **Quién ve qué:** la Web UI (sesión) y la clave de la instancia, todos los trabajos de la instancia; la clave de una
  API por aplicación, solo los de esa API (como `GET /api/v1/trabajos`).
- **Al conectarse**, una **foto**: un evento `trabajo` por cada trabajo no terminado y por cada uno terminado hace menos
  de 10 s.
- **Después**, un evento `trabajo` por cada cambio, con estos datos y ningún otro (nada del mensaje, la respuesta ni los
  archivos):

```json
{ "id": "trabajo_…", "estado": "ejecutando", "etapa": "ejecucion", "agente": "claude", "modelo": "claude/sonnet",
  "api": "asistente-soporte", "origen": "app", "intento": 1, "codigo_error": "", "actualizado": "2026-10-20T19:22:50Z" }
```

  - `etapa`: `cola` (en_cola), `preparacion` (preparando_entrada, esperando_agente), `ejecucion` (ejecutando,
    cancelacion_solicitada), `validacion` (validando_salida) y `llegada` (completado, fallido, cancelado,
    interrumpido), la misma correspondencia que `Ruta.vue`.
  - `agente` es el del intento en curso e `intento` empieza en 1 y sube cuando el respaldo pasa a otro agente.
  - `api` vacía es la API principal; `origen` es el de hoy (probar, terminal, cli, app, api).
- **Cómo se arma:** reusa el aviso de cambios de la base (`Almacen.Avisos`, el mismo del SSE de un trabajo). Con cada
  aviso lee los trabajos de la instancia con `actualizado_en` posterior al último leído (índice nuevo
  `(instancia, actualizado_en)`) y envía los que cambiaron de estado, agente o intento. Los avisos seguidos se juntan:
  como mucho una lectura cada 200 ms. Latido cada 15 s, como el SSE de un trabajo.
- **Límites:** hasta 8 flujos abiertos por instancia; el siguiente recibe `429 limite_flujos` con `Retry-After: 5`. Al
  desconectarse el cliente, el flujo se libera.
- **Documentación:** `internal/api/openapi.json` (ruta y esquema `EventoTrabajo`), `docs/api.md` y la guía del
  desarrollador (sección corta «Seguir lo que pasa», con `curl -N`).

## 2. El mapa vivo en Inicio (app)

**Datos:** `useEventosInstancia()` (composable) abre `EventSource('/api/v1/eventos')` mientras Inicio está a la vista y
lo cierra al salir. Si se corta, reconecta con espera creciente (1, 2, 4… hasta 30 s); si no puede abrirlo, el mapa
usa la consulta de cada 5 s de hoy (solo los trabajos en curso de `GET /trabajos`). La lógica va en `web/src/vivo.ts`,
funciones puras probadas.

**El tranvía de cada petición en curso** (rectángulo redondeado del color de su agente, como el de la portada):

| Etapa | Dónde está |
|---|---|
| `cola` | sale de la estación de su API (la principal, una por aplicación o «Probar y terminal») hacia Tramtai |
| `preparacion` | en Tramtai |
| `ejecucion` | avanzando por la línea de su agente |
| `validacion` | cerca del final de la línea |
| `llegada` | en la estación del agente; se desvanece en 2 s |

- **Fallo** (`fallido`, `interrumpido`): se pone rojo y queda 4 s donde falló. **Cancelado:** gris, 2 s.
- **Respaldo** (`intento` sube): vuelve a Tramtai y toma la línea del agente nuevo.
- **Movimiento:** se anima entre posiciones en unos 600 ms; con «reducir movimiento» del sistema salta sin animar.
- **Muchas a la vez:** hasta 12 tranvías; con más, Tramtai muestra «+N». Varias en la misma línea se separan.
- **Móvil:** el mapa vertical hace lo mismo sobre su eje.

**Seguir una petición:** al pulsar un tranvía aparece bajo el mapa la tarjeta **«Siguiendo»**: API de origen, agente y
modelo, la **ruta de 5 paradas** (`Ruta.vue`) en vivo, el tiempo transcurrido y «Ver en el Historial»
(`historial` con ese trabajo abierto). Al terminar queda con su resultado hasta «Dejar de seguir» o hasta elegir otro.
Sin mensaje ni respuesta: solo el recorrido.

**Accesibilidad:** cada tranvía es un botón con nombre («Petición a Claude, en ejecución»); la descripción del mapa suma
«N peticiones en curso»; una región `aria-live` anuncia solo los cambios de la petición seguida.

## 3. La portada del sitio con el recorrido completo

- El tranvía recorre **todo el camino**: app → Tramtai → línea del agente → su estación.
- **4 ejemplos** que se turnan (unos 6 s cada uno), con el bloque de código de al lado cambiando a la vez:
  1. **SDK de OpenAI → Claude:** «Resume este contrato en tres puntos» (el Python de OpenAI de hoy).
  2. **Un PDF por la API propia → Codex → JSON:** «Saca las fechas y los nombres de esta acta»; `curl` que sube el
     archivo y pide respuesta con forma. El tranvía se detiene un momento en Tramtai (la conversión).
  3. **SDK de Anthropic → GitHub Copilot:** «Traduce este texto al inglés» (Python de Anthropic).
  4. **Respaldo:** por la línea de Claude, se pone rojo a mitad («llegó a su límite»), vuelve a Tramtai y sigue por la
     de Codex; el código muestra la receta con `respaldo`.
- Bajo el mapa: una línea que dice qué pasa (es/en, «2 de 4 · Un PDF por la API propia → Codex, con respuesta JSON»),
  puntos para elegir un ejemplo y **Pausa** (obligatorio para movimiento automático de más de 5 s).
- **«Reducir movimiento»:** sin animación; cada punto muestra el tranvía en la estación de su agente y su código.
  **Sin JavaScript:** el mapa fijo y el primer ejemplo, como hoy. **Móvil:** lo mismo sobre `mapa-alto`.
- Los ejemplos son datos en `site/sitio.js` (camino sobre el SVG, texto es/en, id del bloque de código); la animación
  usa la Web Animations API con `offset-path`, como el tranvía de hoy, sin librerías. Los 4 bloques de código están en
  el HTML (se leen sin JavaScript) y se muestra el que toca.

## 4. Pruebas, Ayuda, documentación y alcance

**Pruebas:**
- **Go (`/api/v1/eventos`):** la foto trae los en curso y los terminados hace menos de 10 s; un cambio de estado llega
  como evento con su `etapa`; el respaldo sube `intento` y cambia `agente`; la clave de una API solo ve sus trabajos;
  el flujo 9 recibe 429; ningún evento lleva campos de contenido; al desconectarse se libera el cupo; avisos seguidos
  dan una sola lectura.
- **Web (`vivo.ts`):** aplicar eventos (alta, cambio, respaldo, llegada); etapa → posición; límite de 12 y «+N»;
  borrado de una llegada a los 2 s y de un fallo a los 4 s (reloj inyectado); esperas de reconexión (1, 2, 4… 30 s).
- **Sitio:** una prueba comprueba que los 4 ejemplos de la portada tienen su bloque de código y su texto en los dos
  idiomas.
- **Navegador:** peticiones reales desde Probar y la terminal (tranvía, respaldo, «Siguiendo»); portada con pausa,
  puntos, móvil, «reducir movimiento» y sin JavaScript.

**Ayuda y sitio** (regla de la v0.52): preguntas nuevas en «Empezar» («¿Qué son los tranvías del mapa?»), «Historial»
(«¿Puedo seguir una petición en vivo?») y «Conectar mi app» («¿Mi app puede ver lo que pasa en vivo?», con
`/api/v1/eventos`); el sitio se regenera. La captura de Inicio sigue siendo el mapa sin tranvías (sacar una con un
tranvía a medio camino dependería del momento justo).

**Documentación:** `docs/api.md`, `openapi.json`, `docs/arquitectura.md` (el flujo de la instancia) y
`docs/equipo/pendientes.md` (v0.53 hecha; v0.54 recorridos guiados con driver.js; v0.55 terminal a la par, v0.56
configurar clientes, v0.57 uso y diagnóstico, v0.58 actualización con vuelta atrás). Notas de la versión es/en.

**Fuera de esta release:** recorridos guiados con driver.js (v0.54), animar los diagramas de la Ayuda, mostrar el
contenido de las peticiones en el mapa.

**Riesgos:**
- Muchas conexiones abiertas: tope de 8 por instancia y latido.
- Lecturas de la base con mucho movimiento: avisos juntados cada 200 ms, solo los trabajos que cambiaron, con índice.
- Animación molesta: «reducir movimiento» y pausa en la portada.
