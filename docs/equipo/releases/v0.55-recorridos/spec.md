# Recorridos guiados (v0.55)

Diseño del 2026-10-21, acordado con el usuario por secciones. Hoy:

- **La Ayuda** (`web/src/ayuda/temas`, una sola fuente para la app y el sitio) explica cada página con diagramas, pasos,
  capturas y preguntas, pero no señala nada **en la pantalla**: quien abre Tramtai por primera vez tiene que leer para
  saber dónde está cada cosa.
- **Primeros pasos** (Inicio) guía hacia conectar un agente, probar y conectar una app, sin enseñar la interfaz.
- Las capturas de la Ayuda salen de zonas marcadas con `data-captura` (`web/scripts/capturas.mjs`); no hay anclas para
  señalar partes más pequeñas.
- **La web** cuenta la v0.54 pero no enseña la app en movimiento, y la última auditoría de calidad fue la del anexo A de
  la v0.54.

Decisiones del usuario:
- **Bienvenida:** un recorrido por toda la app que **se ofrece** la primera vez («¿Te enseño Tramtai?») y se puede
  repetir cuando se quiera.
- **Páginas de la bienvenida:** el camino principal (Inicio → Agentes → Probar → Mis APIs) y **Espacios**, la novedad de
  la v0.54.
- **Fuente de los pasos:** un bloque nuevo `recorrido` en cada tema de la Ayuda (enfoque A); la bienvenida junta los
  pasos marcados de cada página.
- **Librería:** driver.js (MIT, empaquetada en la app, sin CDN; la CSP ya admite estilos en línea).
- **Web:** la novedad en la portada, la Ayuda del sitio paso a paso con capturas, una **demo animada en carrusel** (sin
  herramientas nuevas: no hay ffmpeg) y una **auditoría nueva** del sitio con sus arreglos dentro del release.
- **Documentación, Ayuda y guía de desarrolladores** al día con lo nuevo.

## 1. Datos y motor de los recorridos

**El bloque** (`web/src/ayuda/tipos.ts`):

```ts
| { tipo: 'recorrido'; pasos: { ancla: string; titulo: Clave; texto: Clave; bienvenida?: boolean }[] }
```

- Cada tema de página (las 7 de `PAGINAS`) gana su bloque `recorrido` con 3 a 5 pasos. `bienvenida: true` marca los
  que entran en la bienvenida.
- **Anclas:** atributos estables `data-recorrido="<pagina>.<nombre>"` (por ejemplo `inicio.mapa`, `agentes.tarjeta`,
  `espacios.lista`) en elementos que se pintan siempre, también en sus estados vacíos. No se usan selectores de Quasar.
- Una prueba comprueba que cada ancla de cada tema existe en el `.vue` de su página (y que no hay anclas sin paso): un
  cambio de la interfaz que la rompa falla en el CI.

**El motor:**
- `web/src/recorrido.ts`, lógica pura con pruebas: `pasosDePagina(pagina)` y `pasosBienvenida()` (los pasos marcados de
  Inicio, Agentes, Probar, Mis APIs y Espacios, en ese orden, más un paso final sin ancla: «Historial y Ajustes tienen su
  recorrido en su ?»).
- `useRecorrido` envuelve driver.js:
  - si el paso es de otra página, cambia de página y espera a que aparezca su ancla (hasta 5 s);
  - si el ancla no aparece o no se ve (sin datos, el menú lateral cerrado en el móvil), el paso sale **centrado, sin
    resaltar**, en vez de romperse;
  - Siguiente, Anterior y Cerrar, con teclado (flechas y Esc) y contador («3 de 14»), en español e inglés, con los colores
    de la app en claro y oscuro;
  - el foco vuelve a donde estaba al cerrar.
- **Solo enseña:** el recorrido nunca pulsa botones ni cambia datos.

## 2. La bienvenida y «Muéstrame en pantalla»

**La primera vez:**
- Al abrir la app (navegador o escritorio) con la sesión ya cargada en Inicio, un diálogo: «¿Te enseño Tramtai? Son
  unos 2 minutos», con **Empezar** y **Ahora no**.
- Se recuerda en `localStorage` (`tramtai.bienvenida`: `hecha` o `descartada`), como «Ocultar guía» en `estado.ts`.
  Cerrar el recorrido a medias cuenta como hecho.
- No se ofrece si se entró por un enlace profundo (por ejemplo `#conectar` desde la bandeja).

**El recorrido de bienvenida** (unos 14 pasos):
- **Inicio:** el mapa en vivo, Primeros pasos, el selector de espacio y el «?».
- **Agentes:** la tarjeta de un agente y cómo conectarlo.
- **Probar:** la receta, el mensaje y «Ver como código».
- **Mis APIs:** la clave del espacio y «Nueva API».
- **Espacios:** la lista y «Abrir un proyecto».
- **Cierre:** Historial y Ajustes tienen su recorrido en su «?».

**Repetirlo:**
- **Primeros pasos** gana la línea «Haz el recorrido de Tramtai».
- **El panel «?»** de cada página enseña el bloque «Qué verás en pantalla» con sus pasos y el botón **Muéstrame en
  pantalla**, que hace el recorrido de esa página (el panel se cierra antes de empezar).
- **La página de Ayuda** de la app enseña lo mismo; si el tema es de otra página, el botón lleva a ella y arranca allí.

**El menú por grupos** (pedido del usuario a mitad de la v0.55): el menú lateral sigue el flujo de uso, en tres grupos
con su título:
- **Empezar:** Inicio, Agentes, Probar.
- **Conectar:** Mis APIs, Historial.
- **Organizar:** Espacios, Ajustes.

La Ayuda sigue abajo, aparte. La bienvenida baja por el menú (Inicio → Agentes → Probar → Mis APIs → Espacios, sin
Historial) y una prueba exige que su orden sea el del menú. El mismo orden vale en el menú del móvil.

## 3. Ayuda, sitio, demo, auditoría y documentación

**Ayuda (app y sitio):**
- Los 7 temas de página ganan su «Qué verás en pantalla».
- En el sitio, cada paso va con **su captura recortada**: el elemento resaltado con el anillo del recorrido, en claro y
  oscuro, en español e inglés. `capturas.mjs` gana ese modo (resalta cada ancla y recorta).
- El tope de peso de las capturas de la Ayuda pasa de 3 MB a 5 MB, con su prueba.

**Demo animada en la portada:** un carrusel de las capturas de la bienvenida (pantalla completa de la app):
- avanza solo, con pausa y puntos para elegir; quieto con «reducir movimiento»; anunciado solo cuando lo elige la persona
  (como el recorrido del tranvía);
- WebP con `srcset`, tamaño declarado y carga diferida; se rehace con `npm run capturas`.

**Portada:** «Así se ve» suma el recorrido guiado, Funciones gana «Aprende la app en 2 minutos» y la novedad de Espacios
queda más visible.

**Auditoría nueva del sitio:** Lighthouse (rendimiento, accesibilidad, buenas prácticas y SEO) en la portada, la guía de
desarrolladores, un tema de la Ayuda y la 404, en móvil y escritorio; lo que salga se arregla en el release y queda en el
anexo A de esta spec.

**Documentación:**
- Guía de desarrolladores (`site/desarrolladores.html`, `docs/desarrolladores.md`): enlace a «Aprende la app en 2
  minutos»; la API no cambia.
- `docs/equipo`: cómo añadir un paso o un ancla de recorrido, junto a la regla «cada función trae su Ayuda».
- Ayuda de Empezar y `docs/inicio.md`: el recorrido.
- `docs/equipo/pendientes.md`: v0.55 ✅ al publicar.

## 4. Pruebas

- **Web (node:test):** `recorrido.ts` (orden de la bienvenida, pasos de cada página, paso final); anclas de cada tema
  presentes en su página y sin anclas sueltas; `estilo.test` (títulos y textos de los pasos con los mismos límites que la
  Ayuda, en los dos idiomas); estado de la bienvenida (`hecha`, `descartada`, enlace profundo); sitio (`sitio.test`: el
  bloque en las páginas generadas, capturas declaradas con tamaño); portada (`portada.test`: el carrusel, su pausa y sus
  imágenes).
- **Navegador, de punta a punta:** la bienvenida completa (ofrecida, empezada, cerrada a medias, no vuelve a salir);
  «Muéstrame en pantalla» en cada página; un ancla ausente sale centrada; teclado; móvil y escritorio; claro y oscuro;
  español e inglés.

**Fuera de la v0.55:** recorridos en la terminal; recorridos que hagan acciones por la persona (pulsar, rellenar);
estadísticas de uso de los recorridos (Tramtai no tiene telemetría).

## Anexo A. Auditoría del sitio

Auditoría del 2026-10-21 con Lighthouse 13 (Edge 155 sin interfaz, `npx lighthouse`, sin añadirlo a las dependencias) sobre
`site/` servido en local (`localhost:8765`), en móvil (por defecto) y en escritorio (`--preset=desktop`). Lo que sale se
arregla en el release; se marca la prioridad y la casilla (`[x]` arreglado; si no, el motivo).

**Rendimiento en local no es rendimiento en GitHub Pages:** el servidor local no comprime, no pone caché y no tiene la
latencia de una red real, así que el tiempo de respuesta, la caché y la compresión no se miden aquí. Cuentan lo que
depende del sitio: LCP, CLS, imágenes, recursos que bloquean el renderizado, tamaños y trabajo del script.

**Puntuaciones iniciales** (rendimiento / accesibilidad / buenas prácticas / SEO)

| Página | Móvil | Escritorio | LCP móvil |
|---|---|---|---|
| `index.html` | 93 / 100 / 100 / 92 | 99 / 100 / 100 / 92 | 2,1 s |
| `desarrolladores.html` | 99 / 100 / 100 / 92 | 100 / 100 / 100 / 92 | 2,2 s |
| `ayuda/espacios.html` | 100 / 100 / 100 / 92 | 100 / 100 / 100 / 92 | 1,8 s |
| `404.html` | 99 / 100 / 100 / 54 | 100 / 100 / 100 / 54 | 1,9 s |

**SEO**
- [A] [x] «link-text» en las 4 páginas: el enlace del pie a `ayuda/empezar.html` decía solo «Empezar», un texto genérico
  para Lighthouse. Ahora «Primeros pasos» / «First steps» (como la tarjeta de la app), en `web/scripts/sitio.ts`, con su
  prueba en `sitio.test.ts`.
- [B] 404 con SEO 54 («is-crawlable»): lleva `noindex` a propósito. No se arregla.

**Rendimiento**
- [A] [x] Portada en móvil (93): sitio.js medía cada camino del tranvía metiendo un `<path>` en el svg
  (`sitio.js:313`), un reflow forzado de toda la página al cargar (TBT 250 ms, 649 ms de arranque de sitio.js). Ahora los
  caminos se miden con un path suelto, fuera de la página (con la medida dentro del svg solo como respaldo), una sola
  vez cada uno, y el tranvía no mide ni anima nada hasta que el mapa está a 200 px de la pantalla (`IntersectionObserver`).
  El comportamiento no cambia: pausa, puntos, «reducir movimiento» y `visibilitychange` funcionan igual (un cambio de
  pestaña no arranca un tranvía que aún no se ha visto). La demo ya esperaba a verse para avanzar y no mide nada. Prueba en
  `portada.test.ts`.
- [M] «image-delivery» en la portada de escritorio (ahorro estimado de 47 KiB): las capturas de «Así se ve» bajan como
  960 px y se pintan a 630. **Sin arreglo en la v0.55:** el atributo `sizes` ya estaba y es correcto (`(max-width: 900px)
  100vw, 660px`, y `1000px` en la demo, con prueba), así que el navegador ya elige la mejor de las dos; el ahorro pide
  una tercera anchura (640 px) por captura, es decir, rehacer todas las capturas con `npm run capturas`. Queda como
  pendiente para cuando se rehagan las capturas.
- [B] Caché y latencia del documento: dependen del servidor (GitHub Pages), no del sitio.
- [B] `estilo.css` bloquea el renderizado (29 KB) y CSS y JS sin minificar (6-7 KB): el sitio no tiene paso de
  compilación y no se añaden dependencias. En la 404 el bloqueo viene de pedir `estilo.css` con la URL absoluta de GitHub
  Pages desde local; publicada es del mismo origen.
- [B] CSS sin usar (11-20 KB): una sola hoja común a todas las páginas, a propósito.

**Decisión de la carga de las capturas:** el LCP de la portada es texto (el párrafo de entrada en móvil, el titular en
escritorio) y Lighthouse no pide «descubrir» ninguna imagen; ninguna captura de la demo ni de «Así se ve» está en la
primera pantalla. Todas siguen con `loading="lazy"`: ni `eager` ni `fetchpriority="high"` mejorarían el LCP.

**Puntuaciones finales** (rendimiento / accesibilidad / buenas prácticas / SEO)

| Página | Móvil | Escritorio | LCP móvil |
|---|---|---|---|
| `index.html` | | | |
| `desarrolladores.html` | | | |
| `ayuda/espacios.html` | | | |
| `404.html` | | | |
