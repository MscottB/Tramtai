# Recorridos guiados (v0.55) — plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** recorridos guiados sobre la app (bienvenida por toda la app y «Muéstrame en pantalla» en cada página) que
salen de los datos de la Ayuda, con la Ayuda del sitio paso a paso, una demo en carrusel en la portada y una auditoría
nueva del sitio.

**Architecture:** un bloque nuevo `recorrido` en los temas de la Ayuda apunta a anclas `data-recorrido` de la interfaz;
`web/src/recorrido.ts` (puro) arma los pasos y `useRecorrido` los muestra con driver.js. El sitio pinta el mismo bloque
con capturas recortadas que saca `capturas.mjs`.

**Tech Stack:** Vue 3 + Quasar, driver.js (MIT, empaquetado), node:test, CDP (Edge) para capturas, Lighthouse.

**Spec:** `docs/equipo/releases/v0.55-recorridos/spec.md`

## Global Constraints

- Todo texto visible en español e inglés (`web/src/idiomas/{es,en}.ts`, `ayuda.{es,en}.ts`); títulos y textos de los
  pasos caben en los límites de la Ayuda (200 caracteres; los títulos, en 60).
- Una sola fuente: los pasos viven en los temas de `web/src/ayuda/temas`; ni la app ni el sitio repiten sus textos.
- Anclas `data-recorrido="<pagina>.<nombre>"` (o `app.<nombre>` para la cabecera); nunca selectores de Quasar.
- El recorrido solo enseña: no pulsa botones ni cambia datos.
- driver.js empaquetado por Vite (sin CDN); versión exacta fijada en `package.json`.
- Accesible: teclado (flechas, Esc), foco de vuelta al cerrar, «reducir movimiento», contraste AA en claro y oscuro.
- La API no cambia (ni `openapi.json` ni `docs/api.md`).
- Cada función trae su Ayuda y su página del sitio; el CI compara `site/` con `npm run sitio`.

## Review Focus

1. Un ancla que existe pero no se ve (pestaña de Ajustes sin elegir, `v-show` falso, menú lateral cerrado en el móvil,
   tamaño 0): el paso sale centrado y sin resaltar, nunca resalta un hueco vacío (Tarea 3: `elegirElemento`).
2. La sesión aún cargando, la pantalla de entrada o un enlace profundo (`#conectar`): la bienvenida no se ofrece hasta
   que hay sesión en Inicio (Tarea 4: `ofrecerBienvenida`).
3. `localStorage` bloqueado (ventana privada, política de empresa): la bienvenida se ofrece como mucho una vez por carga
   y nunca en bucle (Tarea 4).
4. Cambiar de idioma o de tema a mitad del recorrido: el paso siguiente sale en el idioma y los colores nuevos (los
   textos se resuelven al mostrar cada paso, no al empezar) (Tarea 3: `aDriver` perezoso).
5. Esc con el recorrido sobre un diálogo o con el panel «?» abierto: cierra el recorrido y no lo de debajo; el panel se
   cierra antes de empezar (Tarea 5).

---

### Task 1: El bloque `recorrido` y la lógica de los pasos

**Files:**
- Modify: `web/src/ayuda/tipos.ts` (bloque nuevo), `web/src/ayuda/estilo.test.ts` (`cortas` y límite de títulos)
- Create: `web/src/recorrido.ts`, `web/src/recorrido.test.ts`

**Interfaces:**
- Produces:
  - `type PasoRecorrido = { ancla: string; titulo: Clave; texto: Clave; bienvenida?: boolean }` y el bloque
    `{ tipo: 'recorrido'; pasos: PasoRecorrido[] }` en `Bloque`.
  - `interface PasoMostrado { pagina: Pagina; ancla?: string; titulo: Clave; texto: Clave }`.
  - `pasosDePagina(pagina: Pagina, temas?: Tema[]): PasoMostrado[]` — los del bloque `recorrido` del tema cuyo
    `paginaApp` es `pagina`.
  - `pasosBienvenida(temas?: Tema[]): PasoMostrado[]` — los marcados `bienvenida` de `ORDEN_BIENVENIDA` y, al final, el
    paso sin ancla `{ pagina: 'espacios', titulo: 'recorrido.finTitulo', texto: 'recorrido.finTexto' }`.
  - `const ORDEN_BIENVENIDA: Pagina[] = ['inicio', 'agentes', 'ejecutar', 'conectar', 'espacios']`.

- [ ] **Step 1: Write the failing tests** en `recorrido.test.ts` con temas de prueba (no los reales):
  `pasosDePagina('agentes', temas)` devuelve sus pasos en orden con `pagina: 'agentes'`; una página sin bloque devuelve
  `[]`; `pasosBienvenida(temas)` respeta `ORDEN_BIENVENIDA` (aunque el array de temas venga desordenado), solo toma los
  `bienvenida: true` y termina en el paso de cierre sin ancla. En `estilo.test.ts`, `cortas` incluye `titulo` y `texto`
  de cada paso y los títulos de paso caben en 60.
- [ ] **Step 2: Run** `cd web; npm test` → FAIL (no existe `recorrido.ts`).
- [ ] **Step 3: Implement** el tipo, `recorrido.ts` (por defecto usa `temas` de `ayuda/indice`) y las claves
  `recorrido.finTitulo`/`recorrido.finTexto` en es y en.
- [ ] **Step 4: Run** `npm test` → PASS.
- [ ] **Step 5: Commit** — `feat(web): el bloque recorrido de la Ayuda`

### Task 2: Los recorridos de las 7 páginas y sus anclas

**Files:**
- Modify: `web/src/ayuda/temas/{empezar,agentes,probar,conectar,historial,espacios,ajustes}.ts`,
  `web/src/idiomas/ayuda.{es,en}.ts`, `web/src/App.vue`, `web/src/componentes/CabeceraPagina.vue`,
  `web/src/paginas/{Panel,Agentes,Ejecutar,Accesos,Trabajos,Espacios,Configuracion}.vue`
- Create: `web/src/ayuda/anclas.test.ts`

**Interfaces:**
- Consumes: el bloque de la Tarea 1.
- Produces: las anclas. Las de bienvenida, en este orden:
  `inicio.mapa`, `inicio.primeros-pasos`, `app.espacio`, `app.ayuda`, `agentes.tarjeta`, `agentes.conectar`,
  `ejecutar.receta`, `ejecutar.mensaje`, `ejecutar.codigo`, `conectar.clave-espacio`, `conectar.nueva`,
  `espacios.lista`, `espacios.abrir` (13 pasos más el cierre = 14). Historial (`historial.filtros`, `historial.lista`,
  `historial.detalle`) y Ajustes (`configuracion.pestanas`, `configuracion.modelo`, `configuracion.datos`) solo en su
  recorrido de página.

- [ ] **Step 1: Write the failing test** `anclas.test.ts`: para cada paso de cada tema, `ancla` sigue
  `^(app|<pagina>)\.[a-z-]+$`; un ancla `<pagina>.x` aparece como `data-recorrido="<pagina>.x"` en el `.vue` de su
  página (mapa `inicio→Panel`, `agentes→Agentes`, `ejecutar→Ejecutar`, `conectar→Accesos`, `historial→Trabajos`,
  `espacios→Espacios`, `configuracion→Configuracion`) y un ancla `app.x`, en `App.vue` o `CabeceraPagina.vue`; ningún
  `data-recorrido` de esos archivos queda sin paso; las 7 páginas tienen recorrido de 3 a 5 pasos; `pasosBienvenida()`
  devuelve 14 pasos con las anclas de arriba en ese orden.
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement** los bloques `recorrido` (detrás del bloque `pasos` de cada tema), sus textos es/en
  (describen lo que se ve: «Aquí ves…», no instrucciones) y los `data-recorrido` en elementos que se pintan siempre,
  también en los estados vacíos (Mis APIs sin APIs, Historial vacío). El ancla va en el `.vue` de la página, en el
  elemento que envuelve al componente hijo si hace falta.
- [ ] **Step 4: Run** `npm test` y `npm run build` → PASS.
- [ ] **Step 5: Commit** — `feat(web): recorridos de las siete páginas`

### Task 3: El motor con driver.js

**Files:**
- Modify: `web/package.json` (+ `package-lock.json`), `web/src/estilos.css`, `web/src/idiomas/{es,en}.ts`
- Create: `web/src/composables/useRecorrido.ts`, `web/src/recorridoDom.ts`, `web/src/recorridoDom.test.ts`

**Interfaces:**
- Consumes: `PasoMostrado` (Tarea 1); `ir(pagina)` de `App.vue` (se pasa como función).
- Produces:
  - `elegirElemento(ancla: string | undefined, raiz: Pick<Document, 'querySelector'>): Element | undefined` — el
    elemento si existe y su `getBoundingClientRect()` tiene ancho y alto; si no, `undefined` (paso centrado).
  - `aDriver(paso: PasoMostrado, i: number, total: number, t: (k: string, p?: object) => string)` → la configuración
    de un paso de driver.js (`popover.title`, `description`, `progressText` «{i} de {total}»), llamada **al mostrar
    cada paso**.
  - `useRecorrido(ir: (pagina: Pagina) => void, paginaActual: Ref<Pagina>)` →
    `{ empezar(pasos: PasoMostrado[], alTerminar?: () => void): void; activo: Ref<boolean> }`.

- [ ] **Step 1: Write the failing tests** en `recorridoDom.test.ts` (raíz falsa con `querySelector`): ancla presente y
  con tamaño → el elemento; ancla ausente, sin ancla o con rect 0×0 → `undefined`; `aDriver` usa la `t` que recibe
  (dos `t` distintas dan textos distintos: el idioma se resuelve al mostrar) y el contador «3 de 14».
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement**:
  - `npm install driver.js@<última estable, exacta>`;
  - `useRecorrido`: por paso, si `paso.pagina !== paginaActual.value` llama a `ir`, espera el ancla con
    `elegirElemento` cada 100 ms hasta 5 s y muestra el paso (centrado si no hay elemento); Siguiente/Anterior/Cerrar con
    los textos `recorrido.siguiente|anterior|cerrar|hecho`; Esc cierra; al cerrar devuelve el foco al elemento que lo
    tenía y llama `alTerminar`;
  - estilos de driver.js con las variables de la app en claro y oscuro (`body.body--dark`), contraste AA, sin animación
    con `prefers-reduced-motion`.
- [ ] **Step 4: Run** `npm test`, `npm run build` → PASS.
- [ ] **Step 5: Commit** — `feat(web): motor de los recorridos con driver.js`

### Task 4: La bienvenida

**Files:**
- Modify: `web/src/estado.ts`, `web/src/App.vue`, `web/src/paginas/Panel.vue`, `web/src/idiomas/{es,en}.ts`
- Create: `web/src/bienvenida.ts`, `web/src/bienvenida.test.ts`

**Interfaces:**
- Consumes: `useRecorrido`, `pasosBienvenida`.
- Produces:
  - `ofrecerBienvenida(o: { sesion: boolean; pagina: Pagina; entradaPorEnlace: boolean; estado: 'hecha' | 'descartada' | null; yaOfrecida: boolean }): boolean`
    — true solo con sesión, en `inicio`, sin enlace profundo, sin estado guardado y sin haberla ofrecido en esta carga.
  - En `estado.ts`: `bienvenida: Ref<'hecha' | 'descartada' | null>` (lee `tramtai.bienvenida` con try/catch) y
    `marcarBienvenida(valor: 'hecha' | 'descartada')` (guarda con try/catch; sin almacenamiento, solo en memoria).

- [ ] **Step 1: Write the failing tests** `bienvenida.test.ts`: la tabla de casos de `ofrecerBienvenida` (sin sesión,
  otra página, enlace profundo, `hecha`, `descartada`, ya ofrecida → false; el caso limpio → true).
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement**: el diálogo de `App.vue` («¿Te enseño Tramtai? Son unos 2 minutos», **Empezar** / **Ahora
  no**; claves `bienvenida.titulo|texto|empezar|ahoraNo`); `entradaPorEnlace` es si el hash inicial no estaba vacío ni
  era `#inicio`; empezar o cerrar a medias marca `hecha`, «Ahora no» marca `descartada`; Primeros pasos (Panel.vue)
  gana la línea «Haz el recorrido de Tramtai» (`bienvenida.repetir`), visible también con la guía completa.
- [ ] **Step 4: Run** `npm test`, `npm run build` → PASS.
- [ ] **Step 5: Commit** — `feat(web): la bienvenida se ofrece la primera vez`

### Task 5: «Muéstrame en pantalla» en el panel «?» y en la Ayuda

**Files:**
- Create: `web/src/componentes/ayuda/BloqueRecorrido.vue`
- Modify: `web/src/componentes/ayuda/VistaTema.vue`, `web/src/componentes/ayuda/PanelAyuda.vue`,
  `web/src/paginas/Ayuda.vue`, `web/src/App.vue`, `web/src/ayuda/panel.ts`, `web/src/idiomas/{es,en}.ts`

**Interfaces:**
- Consumes: `pasosDePagina`, `useRecorrido` (la instancia vive en `App.vue`).
- Produces: `VistaTema` emite `recorrido: [pagina: Pagina]`; `App.vue` cierra el panel, va a la página si hace falta y
  empieza `pasosDePagina(pagina)`.

- [ ] **Step 1: Write the failing test** en `web/src/ayuda/vista.test.ts`: `bloquesVisibles` deja el bloque
  `recorrido` en los tres contextos (`app`, `panel`, `sitio`).
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement** `BloqueRecorrido.vue` («Qué verás en pantalla» = `ayuda.ui.recorridoTitulo`, la lista
  numerada de pasos y el botón **Muéstrame en pantalla** = `ayuda.ui.mostrar`); el panel se cierra antes de empezar; Esc
  con el recorrido activo lo cierra a él y no al panel ni al diálogo de debajo (`PanelAyuda.alTeclear` lo deja pasar si
  `activo`).
- [ ] **Step 4: Run** `npm test`, `npm run build` → PASS.
- [ ] **Step 5: Commit** — `feat(web): «Muéstrame en pantalla» en la Ayuda`

### Task 6: La Ayuda del sitio paso a paso

**Files:**
- Modify: `web/scripts/sitio.ts`, `web/scripts/sitio.test.ts`, `web/scripts/capturas.mjs`,
  `web/scripts/capturas-lista.ts`, `web/scripts/capturas.test.ts`, `site/estilo.css`
- Generated: `web/src/ayuda/capturas/recorrido-*.webp`, `site/ayuda/*`

**Interfaces:**
- Consumes: los bloques de la Tarea 2.
- Produces: capturas `recorrido-<ancla con . → ->.<claro|oscuro>.<es|en>.webp` (p. ej.
  `recorrido-inicio-mapa.oscuro.en.webp`); en `sitio.ts`, el bloque `recorrido` como sección «Qué verás en pantalla»
  (entra en «En esta página»), una lista numerada con título, texto y su captura (con `width`/`height`).

- [ ] **Step 1: Write the failing tests**: `sitio.test` — una página con recorrido tiene `<section id="s-recorrido">`,
  un `<li>` por paso y su `<img>` con tamaño cuando la captura existe; `capturas.test` — cada ancla de cada recorrido
  tiene sus 4 capturas y el tope total pasa a 5 MB.
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement** el modo de capturas (abre la página, desplaza el ancla a la vista, la rodea con el anillo
  de driver.js y recorta el elemento con 16 px de margen; las de `app.*` recortan la cabecera) y el bloque en el sitio.
- [ ] **Step 4: Run** `npm run build`, `npm run capturas`, `npm run sitio`, `npm test` → PASS.
- [ ] **Step 5: Commit** — `feat(sitio): la Ayuda paso a paso con capturas`

### Task 7: La portada: novedad y demo en carrusel

**Files:**
- Modify: `site/index.html`, `site/estilo.css`, `site/sitio.js`, `web/scripts/portada.test.ts`,
  `web/scripts/capturas.mjs`, `web/scripts/capturas-lista.ts`
- Generated: `site/capturas/bienvenida-<n>-<claro|oscuro>-<960|1440>.webp`

**Interfaces:**
- Consumes: `pasosBienvenida` (para el número y el orden de pasos de la demo).
- Produces: el carrusel `#demo` (una imagen por paso con el recorrido sobre la app a pantalla completa).

- [ ] **Step 1: Write the failing tests** en `portada.test.ts`: existe `<section id="demo">` con una figura por paso
  de la bienvenida (14), cada `<img>` WebP con `srcset` 960/1440, `width`/`height` y `loading="lazy"`; hay botón de
  pausa sin `aria-pressed` y puntos con nombre por idioma (como el recorrido del tranvía); la región no se anuncia sola
  (`aria-live="off"`); Funciones tiene «Aprende la app en 2 minutos»; «Así se ve» enlaza la demo; Espacios sale en la
  entrada de la portada y en Funciones (la novedad de la v0.54, más visible).
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement** las capturas de la bienvenida (el recorrido de verdad, paso a paso, a 1440×900 y 960),
  el carrusel en `sitio.js` (6 s por paso, pausa, puntos, quieto con «reducir movimiento», sin animar con la pestaña
  oculta; sin JavaScript se ve la primera imagen y la lista de pasos) y los textos de la portada en es/en.
- [ ] **Step 4: Run** `npm run capturas`, `npm test`; navegador (`pagina`, 8765) en 375/800/1280, claro y oscuro.
- [ ] **Step 5: Commit** — `feat(sitio): la demo del recorrido en la portada`

### Task 8: Auditoría del sitio con Lighthouse

**Files:**
- Modify: `docs/equipo/releases/v0.55-recorridos/spec.md` (anexo A) y lo que la auditoría pida en `site/`,
  `web/scripts/sitio.ts`, `site/estilo.css`, `site/sitio.js`

- [ ] **Step 1:** Lighthouse (rendimiento, accesibilidad, buenas prácticas, SEO) en móvil y escritorio sobre `index.html`,
  `desarrolladores.html`, `ayuda/espacios.html` y `404.html` servidos en local (preview `pagina`); anotar en el anexo A
  cada hallazgo con su prioridad.
- [ ] **Step 2:** Por cada hallazgo comprobable con una prueba (atributos, metadatos, tamaños), la prueba que falla en
  `sitio.test`/`portada.test` → arreglo → PASS; los demás, arreglo y nueva medida de Lighthouse.
- [ ] **Step 3: Run** `npm test`, `npm run sitio` → PASS; las puntuaciones finales en el anexo A.
- [ ] **Step 4: Commit** — `fix(sitio): lo que pidió la auditoría`

### Task 9: Documentación

**Files:**
- Modify: `site/desarrolladores.html`, `docs/desarrolladores.md`, `docs/inicio.md`, `web/src/ayuda/temas/empezar.ts`
  (+ textos), `docs/equipo/pendientes.md`, `docs/equipo/desarrollo.md` (sección «Ayuda y sitio en cada función»:
  añadir «Recorridos: cómo añadir un paso o un ancla» y subir el tope de capturas de 3 a 5 MB)

- [ ] **Step 1:** Escribir: en la guía, el enlace a «Aprende la app en 2 minutos» (la API no cambia); en `inicio.md` y
  en Empezar, el recorrido de bienvenida y cómo repetirlo; en `docs/equipo`, cómo añadir un paso y un ancla (y que
  `anclas.test` lo comprueba); `pendientes.md`, v0.55 ✅ (se marca al publicar, en esta rama).
- [ ] **Step 2: Run** `npm test`, `npm run sitio` → PASS (sin cambios pendientes).
- [ ] **Step 3: Commit** — `docs: recorridos guiados en la guía y la documentación`

### Task 11: El menú por grupos (se ejecuta justo después de la Tarea 2)

**Files:**
- Create: `web/src/menu.ts`, `web/src/menu.test.ts`
- Modify: `web/src/App.vue` (lista `principales` y su menú lateral), `web/src/idiomas/{es,en}.ts`, `web/src/recorrido.ts`

**Interfaces:**
- Produces:
  - `const GRUPOS_MENU: { grupo: 'empezar' | 'conectar' | 'organizar'; paginas: Pagina[] }[]` =
    `empezar: ['inicio', 'agentes', 'ejecutar']`, `conectar: ['conectar', 'historial']`,
    `organizar: ['espacios', 'configuracion']`.
  - `ordenMenu(): Pagina[]` — las páginas de `GRUPOS_MENU` en orden.
  - Claves `menu.grupo.empezar|conectar|organizar` («Empezar» / «Get started», «Conectar» / «Connect», «Organizar» /
    «Organize»).

- [ ] **Step 1: Write the failing tests** `menu.test.ts`: cada página de `PAGINAS` sale exactamente una vez en
  `ordenMenu()`; el orden es el de arriba; `ORDEN_BIENVENIDA` (de `recorrido.ts`) es una subsecuencia de `ordenMenu()`
  (mismo orden relativo).
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement** `menu.ts`; `App.vue` arma `principales` en el orden de `ordenMenu()` y pinta el menú lateral
  (también el del móvil) en tres grupos con su título, accesibles (`role="group"` con `aria-labelledby` al título del
  grupo); la línea del menú (las paradas) sigue siendo una sola; la Ayuda queda abajo, aparte. Si hay atajos o algo que
  dependa de la posición en el menú, sigue el orden nuevo.
- [ ] **Step 4: Run** `npm test`, `npm run build` → PASS; navegador: escritorio y móvil, claro y oscuro.
- [ ] **Step 5: Commit** — `feat(web): el menú por grupos, en el orden de uso`

### Task 10: Comprobación de punta a punta

- [ ] **Step 1:** `go vet ./...`, `TRAMTAI_IDIOMA=es go test ./...`, `cd web; npm test; npm run build; npm run sitio`
  (sin cambios pendientes), `graphify update .`.
- [ ] **Step 2: Navegador** (preview `v050` con el binario nuevo): la bienvenida ofrecida la primera vez, empezada,
  cerrada a medias y no vuelve a salir; «Ahora no»; un enlace profundo no la ofrece; «Muéstrame en pantalla» en las 7
  páginas; un ancla oculta sale centrada (Ajustes en la otra pestaña); Esc con el panel y con un diálogo; cambio de
  idioma a mitad; teclado; móvil y escritorio; claro y oscuro.
- [ ] **Step 3: Commit** de lo que falte y línea del ledger.
