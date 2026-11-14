# v0.61, la web nueva: plan de implementación

> **Para quien lo ejecuta:** se sigue tarea a tarea con superpowers:subagent-driven-development (o
> superpowers:executing-plans). Los pasos usan casillas (`- [ ]`).

**Objetivo:** rehacer todo el sitio (`site/`) con el sistema visual nuevo y siete páginas con la misma barra, sin cambiar
la app.

**Arquitectura:** sigue siendo un sitio estático en GitHub Pages. `web/scripts/sitio.ts` pinta la barra, el pie, el
buscador, la Ayuda y la plantilla de documentación. Inicio, Producto, Agentes, Descargar, Servidor y Desarrolladores se
escriben a mano con zonas `<!-- comun:x -->` que `npm run sitio` rellena. `site/estilo.css` y `site/sitio.js` se
reescriben, sin librerías.

**Tecnología:**
- TypeScript con node:test (`cd web; npm test`);
- Go para `TestComandosDelSitioExisten` (`cmd/tramtai/sitio_test.go`);
- HTML, CSS y SVG a mano.

**Spec:** `docs/equipo/releases/v0.61-web-nueva/spec.md`. Se lee junto con este plan.

## Restricciones globales

- **Solo el sitio:** no se toca la app, la Web UI, la terminal ni la CLI. Los textos de la Ayuda siguen saliendo de
  `web/src/idiomas/ayuda.*.ts` sin cambiar.
- **Dependencias:** ninguna nueva (ni npm ni Go). Las letras se recortan en la máquina de quien hace la tarea con
  `uv tool run --from "fonttools[woff]" pyftsubset`, como la de Cascadia (`docs/equipo/desarrollo.md:124-135`).
- **Letras:**
  - Inter variable y Instrument Serif cursiva, OFL, con su licencia al lado (`INTER-OFL.txt`, `INSTRUMENT-OFL.txt`);
  - Cascadia se queda como `Tramtai Mono`;
  - Overpass se quita (`overpass.woff2`, `OVERPASS-OFL.txt`);
  - las letras que carga Inicio pesan en total **150 KB o menos**.
- **Colores:**
  - oscuro por defecto, con el acento `#2dd4bf`;
  - claro con el acento oscurecido;
  - variables con `light-dark()`;
  - se mantienen el tema del sistema y el selector Claro/Oscuro (`data-tema`, `localStorage` `tramtai.tema`).
- **Textos:**
  - las dos lenguas en la misma página (`lang="es"` / `lang="en"`), con el selector ES/EN de hoy (`tramtai.idioma`);
  - el apóstrofo tipográfico (’) y el glosario, que vigila `estilo-textos.test.ts`;
  - las capturas y los comandos, en el idioma elegido.
- **Direcciones estables:** `index.html`, `servidor.html`, `desarrolladores.html`, `ayuda/`, `ayuda/<tema>.html` y
  `#descargar` siguen funcionando (spec §7).
- **Movimiento:**
  - con `prefers-reduced-motion: reduce` todo queda quieto y en su estado final;
  - sin JavaScript, cada página se ve completa y las animaciones, en su estado final;
  - nunca se mueve el texto que se está leyendo.
- **Accesibilidad:**
  - contraste AA en los dos temas;
  - foco visible;
  - controles de 44 px o más, salvo ES/EN y tema con ratón en escritorio;
  - las pestañas, con teclado y ARIA de pestañas.
- **Capturas:**
  - solo con copias en `bin/capturas-*` y el puerto 17388, nunca con el Tramtai del usuario ni el 17373;
  - las completas hacen peticiones reales a claude/haiku con la suscripción del usuario, así que solo se rehacen las
    necesarias, con `--solo`, y se cuentan.
- **Git:**
  - cada tarea en su rama `v0.61/tNN-<tema>` y su PR con Auto-fix, apilado sobre la anterior; nunca push a `main`;
  - se fusiona y se publica solo con el «sí» del usuario;
  - los commits terminan con `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`;
  - `anti-slop/` nunca se sube.
- **Entorno:** Windows. La herramienta Bash está rota, así que se usa PowerShell. Un hook (GateGuard) pide datos antes
  de editar.

## Lo que más puede fallar (foco de las revisiones)

1. **Un enlace de una app instalada que deja de funcionar** (`servidor.html`, `desarrolladores.html`, `ayuda/<id>.html`
   o `#descargar` del README): tiene que abrir su contenido. Lo prueba la Tarea 2 (`enlaces.test.ts`).
2. **`descargas.mjs` sigue escribiendo solo en `index.html`:** el workflow «Página» tiene que dejar la versión y los
   enlaces del release en Descargar y en el botón del arranque. Lo prueba la Tarea 5 (`descargas.test.ts`).
3. **El sistema no se detecta o es otro:** Linux ARM, un iPad, un navegador sin `userAgentData`. Lo esperado es que el
   botón diga «Descargar» sin sistema y lleve a Descargar, nunca que ofrezca un archivo equivocado. Lo prueba la Tarea 4
   (`sistemaDe`).
4. **Una página muy larga de la Ayuda o del servidor en el móvil:** el índice y «En esta página» no pueden tapar el
   texto ni provocar desplazamiento horizontal a 375 px. Lo comprueba la Tarea 8 en el navegador.
5. **Las animaciones con «reducir movimiento» o sin JavaScript:** la terminal tiene que verse entera y los diagramas
   dibujados. Lo prueba la Tarea 4 (`.vivo` sin `.js`) y lo comprueba el cierre.

---

### Tarea 1: el sistema visual (letras y estilo base)

**Archivos:**
- Crear: `site/inter.woff2`, `site/instrument-italica.woff2`, `site/INTER-OFL.txt`, `site/INSTRUMENT-OFL.txt`
- Borrar: `site/overpass.woff2`, `site/OVERPASS-OFL.txt`
- Reescribir: `site/estilo.css`
- Modificar: `web/scripts/sitio.ts`, en `cabeza()` (precarga de `inter.woff2` en lugar de `overpass.woff2`)
- Probar: `web/scripts/sitio.test.ts`

**Interfaces que deja** para las demás tareas, en `estilo.css`:
- **Variables:** `--fondo`, `--superficie`, `--superficie-suave`, `--borde`, `--borde-discontinuo`, `--texto`,
  `--texto-suave`, `--acento`, `--acento-texto`, y las de cada agente (`--claude`, `--codex`, `--copilot`, `--gemini`,
  `--antigravity`); `--fuente` (Inter), `--serifa` (Instrument Serif), `--mono` (Tramtai Mono) y `--radio: 10px`.
- **Clases:**
  - `.arranque`: el fondo con la cuadrícula y el brillo arriba de cada página;
  - `.titular` y `.titular em`: la parte en serifa cursiva y en acento;
  - `.tarjeta`: borde discontinuo;
  - `.mosaico`: una rejilla con `--col` y `.ancha` / `.alta`;
  - `.fig`: el rótulo «FIG n · …»;
  - `.bloque-codigo` con `.copiar`;
  - `.boton.principal` y `.boton.secundario`;
  - `.vivo`: arranca su animación cuando el `<html>` tiene `.js` y el elemento `.visto`;
  - `.solo-lectores`.
- `@media (prefers-reduced-motion: reduce)` deja `.vivo` en su estado final.

- [ ] **Paso 1: escribir las pruebas que fallan** en `sitio.test.ts`:
  - `'las letras de Inicio no pasan de 150 KB'`: suma el tamaño de cada `url(...)` de `@font-face` en
    `site/estilo.css` que no sea `cascadia.woff2` y comprueba `<= 150 * 1024`;
  - `'no queda Overpass'`: ni `estilo.css` ni `cabeza()` mencionan `overpass`;
  - `'cada letra lleva su licencia'`: existen `INTER-OFL.txt` e `INSTRUMENT-OFL.txt`;
  - `'reducir movimiento para lo vivo'`: `estilo.css` tiene un bloque `prefers-reduced-motion: reduce` que nombra
    `.vivo`.
- [ ] **Paso 2:** `cd web; node --import ./pruebas.mjs --test scripts/sitio.test.ts`. Tienen que fallar.
- [ ] **Paso 3: recortar las letras.**
  - Inter variable (rsms/inter, la última estable) e Instrument Serif Italic (Google Fonts), con el texto de todas las
    páginas en es/en más `«»’“”…·→—–`.
  - Se recortan con `pyftsubset --flavor=woff2`.
  - Se añade la receta a `docs/equipo/desarrollo.md`, junto a la de Cascadia.
- [ ] **Paso 4: reescribir `estilo.css`** con el sistema de la spec §1 y las interfaces de arriba.
  - Se mantienen las reglas de hoy que usan las páginas de la Ayuda (`.migas`, `.riesgos`, `.ventana-terminal`,
    `.pantalla` y las clases `t-*`) hasta que la Tarea 8 las sustituya.
  - Contraste AA comprobado con `mcp__plugin_antislop_antislop-contrast__check_contrast`, si está, para texto, texto
    suave y acento en los dos temas.
- [ ] **Paso 5:** `npm test` y `npm run sitio` pasan. En el navegador, las páginas de hoy se ven con las letras y los
  colores nuevos y sin roturas graves.
- [ ] **Paso 6: commit**, `feat(sitio): sistema visual nuevo, letras y colores`.

### Tarea 2: la barra, el pie, las páginas y los enlaces

**Archivos:**
- Modificar: `web/scripts/sitio.ts`, en concreto:
  - el tipo `Actual`;
  - `PAGINAS_FIJAS`;
  - `cabecera()`;
  - `pie()`;
  - `sitemap()`;
  - lo nuevo de `ANCLAS_PORTADA` y `destinoPortada()`.
- Crear: `site/producto.html`, `site/agentes.html`, `site/descargar.html` (esqueleto con sus zonas `comun:` y un titular)
- Crear: `web/scripts/enlaces.test.ts`
- Modificar: `cmd/tramtai/sitio_test.go`, solo si su lista de páginas está escrita a mano

**Interfaces:**
- **Produce:**
  - `type Actual = 'inicio' | 'producto' | 'agentes' | 'servidor' | 'desarrolladores' | 'ayuda' | 'descargar' | ''`;
  - `PAGINAS_FIJAS`, con `producto.html`, `agentes.html` y `descargar.html`;
  - `export const ANCLAS_PORTADA: Record<string, string>`, que es la tabla de la spec §7: `descargar` →
    `descargar.html`, `como` → `producto.html#como`, `funciones` → `producto.html#funciones`, `por-que` →
    `producto.html#por-que`, `privacidad` → `producto.html#privacidad`, `agentes` → `agentes.html`, `app` y `terminal` →
    `#formas`, `servidor` → `servidor.html`;
  - `export function destinoPortada(hash: string): string | null` devuelve el destino, o `null` si el ancla existe en la
    portada nueva o no está en la tabla.
- **La barra** (`cabecera`) lleva, en este orden:
  - Producto · Agentes · Servidor · Desarrolladores · Ayuda · Descargar, con `aria-current="page"` en la actual;
  - el botón del buscador (`data-buscar`, que abre la Tarea 3);
  - ES/EN, tema y GitHub;
  - «Descargar» hacia `descargar.html`.

- [ ] **Paso 1: escribir las pruebas que fallan** en `enlaces.test.ts`, que lee `site/` tal cual queda tras
  `npm run sitio`:
  - `'cada enlace interno lleva a un archivo que existe'`: cada `href`/`src` relativo de cada `.html`;
  - `'cada ancla existe en su página'`: cada `href="x.html#y"` y cada `#y` tienen un `id="y"` en su destino; las de
    `ANCLAS_PORTADA` cuentan como existentes en `index.html`;
  - `'las direcciones de las apps instaladas siguen'`: existen `servidor.html`, `desarrolladores.html`,
    `ayuda/index.html` y un `ayuda/<id>.html` por tema, y `destinoPortada('#descargar') === 'descargar.html'`;
  - `'cada página tiene la barra, el pie y los dos idiomas'`: las siete y la 404 tienen `comun:cabecera` y `comun:pie`
    rellenos, un `lang="es"` y un `lang="en"`;
  - `'el sitemap tiene las siete páginas'`: `index`, `producto`, `agentes`, `servidor`, `desarrolladores`, `ayuda/` y
    `descargar`, más los temas.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementarlo** con las interfaces de arriba. El pie lleva las siete páginas, más «Primeros pasos»,
  GitHub, Versiones, Licencia y Términos, y la línea de marcas de hoy.
- [ ] **Paso 4:** `npm test`, `npm run sitio` y `go test ./cmd/tramtai -run TestComandosDelSitioExisten` pasan.
- [ ] **Paso 5: commit**, `feat(sitio): siete páginas con la misma barra y prueba de enlaces`.

### Tarea 3: el buscador

**Archivos:**
- Modificar: `web/scripts/sitio.ts`, en `indiceBusqueda()` y `generar()`
- Modificar: `site/sitio.js`
- Borrar: `site/ayuda/indice.json`, que sustituye `site/buscar.json`
- Probar: `web/scripts/sitio.test.ts`

**Interfaces:**
- **Consume:** de la Tarea 2, el botón `data-buscar` de la barra.
- **Produce:**
  - `buscar.json`: `{ es: Entrada[], en: Entrada[] }`, con `Entrada = { u: string; ti: string; d: string; x: string }`.
    `u` es la URL relativa a la raíz con su ancla, `ti` el título, `d` dónde está («Ayuda › Agentes», «Servidor»…) y `x`
    el texto normalizado como hoy (`normalizar`).
  - En `generar`, las entradas de los temas de la Ayuda, como hoy. Además, una por cada `<h2 id>` y `<h3 id>` de
    `servidor.html` y `desarrolladores.html`, leídos de `site/`.
  - En `sitio.js`, un `<dialog>` de búsqueda: se abre con Ctrl K, ⌘K, `/` o el botón; las flechas mueven, Enter abre,
    Esc cierra; resultados en el idioma activo; `role="listbox"`.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `'buscar.json tiene una entrada por tema y apartado'`: cada tema de la Ayuda aparece con sus anclas, y hay
    entradas de Servidor y Desarrolladores con `u` que empieza por `servidor.html#` o `desarrolladores.html#`;
  - `'cada u de buscar.json lleva a un ancla que existe'`.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementar** el índice y el diálogo. El buscador de la Ayuda de hoy (`#buscar-ayuda`) pasa a abrir el
  mismo diálogo.
- [ ] **Paso 4:** pruebas en verde. En el navegador, buscar «suscripción» encuentra el tema de la Ayuda y el apartado del
  servidor, todo con teclado.
- [ ] **Paso 5: commit**, `feat(sitio): buscador en todo el sitio`.

### Tarea 4: Inicio

**Archivos:**
- Reescribir: `site/index.html`
- Modificar: `site/sitio.js`, para el sistema detectado, las pestañas de «Una ventana, cuatro formas», el «copiar», lo
  vivo con `IntersectionObserver` y los anclajes viejos
- Modificar: `web/scripts/sitio.ts`, si `ventanaTerminal()` cambia de forma para ir dentro de la ventana de cuatro formas
- Reescribir: `web/scripts/portada.test.ts`

**Interfaces:**
- **Consume:**
  - de la Tarea 1, las clases;
  - de la Tarea 2, `ANCLAS_PORTADA` y `destinoPortada`, que `index.html` incrusta como `ayuda.html` incrusta las suyas;
  - de las Tareas 5, 6 y 7, los anclajes de destino.
- **Produce:**
  - En `sitio.js`, `sistemaDe(ua: string, plataforma?: string): 'windows' | 'macos' | 'linux' | null`, exportable para
    probarla con el patrón que ya usan las pruebas del sitio.
  - Los `id` de Inicio: `arranque`, `fig`, `formas` y `sigue`.

- [ ] **Paso 1: escribir las pruebas que fallan** en `portada.test.ts`:
  - `'Inicio tiene sus cuatro bloques'`, con los `id` de arriba;
  - `'el arranque lleva el titular en dos partes'`: un `h1` con un `em`, en es y en;
  - `'FIG 1 enseña el cambio de dos líneas'`: una línea `- base_url` y otra `+ base_url` con `127.0.0.1:17373`;
  - `'la ventana tiene cuatro pestañas'`: Web UI, Terminal, CLI y Servidor, con `role="tab"`; sin `.js`, los cuatro
    paneles se ven;
  - `'sistemaDe'`:
    - Windows → `windows`;
    - Mac → `macos`;
    - Linux x86_64 → `linux`;
    - iPad, Android, un texto vacío y Linux aarch64 → `null`;
  - `'lo vivo tiene su estado final sin JavaScript'`: las líneas de FIG 1 no llevan `opacity:0` fuera de `.js .vivo`.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: escribir `index.html`** según la spec §4.
  - **FIG 1** es la terminal viva, con datos fijos de ejemplo.
  - **FIG 2** es el camino en SVG isométrico, con su flujo.
  - **FIG 3** es el estado de los agentes; FIG 4 y 5 van con recortes de la Tarea 9, y mientras tanto con las capturas
    de hoy.
  - **La ventana:**
    - Web UI con la captura de Mis APIs;
    - Terminal con `comun:terminal`;
    - CLI en texto real, con órdenes que existen;
    - Servidor con la captura del mapa.
  - **«Sigue por aquí»**, con seis tarjetas.
  - El botón de descarga lleva `data-descarga` y el sistema detectado.
  - **Los comandos del arranque:**
    - macOS: `brew tap mscottb/tramtai https://github.com/MscottB/Tramtai` y `brew install --cask tramtai`;
    - Windows: «Descargar el instalador»;
    - Linux: «.deb o .rpm».
- [ ] **Paso 4:** `npm test`, `npm run sitio` y `go test ./cmd/tramtai -run TestComandosDelSitioExisten` pasan. En el
  navegador, a 375 y 1440 px, en claro y oscuro, en es y en, con «reducir movimiento» y sin JavaScript.
- [ ] **Paso 5: commit**, `feat(sitio): Inicio nuevo`.

### Tarea 5: Descargar

**Archivos:**
- Escribir: `site/descargar.html`, con el contenido de la sección «descargar» de hoy, que sale de `index.html`
- Modificar: `web/scripts/descargas.mjs`, para que acepte varias páginas
- Modificar: `.github/workflows/paginas.yml:48`, que pasa `site/index.html site/descargar.html`
- Probar: `web/scripts/descargas.test.ts`

**Interfaces:**
- **Consume:** de la Tarea 4, `sistemaDe`.
- **Produce:** `conDescargas(html, release)` no cambia. El ejecutable acepta `node descargas.mjs release.json
  <página>...` y aplica la función a cada página. `descargar.html` lleva el `<script id="descargas">`, los `data-archivo`
  y `data-arm`, y `<span data-version>`; Inicio lleva `data-version` y los `data-archivo` del botón del arranque.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `'descargas.mjs rellena varias páginas'`: con un release de ejemplo, las dos quedan con la versión y sus enlaces;
  - `'Descargar tiene cada archivo del release'`: instalador, portable, `.deb`, `.rpm`, `.dmg`, CLI y servidor
    (amd64/arm64), los mismos `data-archivo` que hoy;
  - `'las pestañas de Descargar'`: Windows, macOS y Linux.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementarlo** según la spec §5, «Descargar». Las novedades de la versión van como hoy.
- [ ] **Paso 4:** pruebas en verde, y `node web/scripts/descargas.mjs` con el `release.json` real de la v0.60 (`gh
  release view --json tagName,assets`) sobre una copia del sitio en el scratchpad deja los enlaces bien.
- [ ] **Paso 5: commit**, `feat(sitio): página Descargar`.

### Tarea 6: Producto

**Archivos:**
- Escribir: `site/producto.html`, con lo que sale de `index.html`: «como», «funciones», «por-que», la línea de ventajas
  y «privacidad»
- Modificar: `site/sitio.js`, solo si el diagrama que se dibuja paso a paso necesita más que `.vivo`
- Probar: `web/scripts/portada.test.ts`, en un `describe('producto')`

**Interfaces:**
- **Consume:** las clases de la Tarea 1 y `comun:riesgos` de `rellenar()`.
- **Produce:** los `id` `como`, `funciones`, `por-que`, `comparativa` y `privacidad`.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `'Producto tiene sus secciones'`, con los `id`;
  - `'las siete ventajas siguen'`: el mismo número y los mismos textos que la línea de ventajas de hoy; antes de borrar
    la sección de `index.html` se copia su lista a la prueba;
  - `'la comparativa va plegada'`: dentro de un `<details>`;
  - `'privacidad completa'`: el texto de la sección de hoy, entero.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: escribir la página** según la spec §5, «Producto». El diagrama de tres pasos va en SVG y se dibuja con
  `.vivo`; las funciones, como tarjetas con recorte (de la Tarea 9; mientras tanto, las capturas de hoy).
- [ ] **Paso 4:** pruebas en verde y revisión en el navegador, como en la Tarea 4.
- [ ] **Paso 5: commit**, `feat(sitio): página Producto`.

### Tarea 7: Agentes

**Archivos:**
- Escribir: `site/agentes.html`, con la sección «agentes» de hoy, que sale de `index.html`
- Probar: `web/scripts/portada.test.ts`, en un `describe('agentes')`

**Interfaces:**
- **Consume:** las variables de color de cada agente (Tarea 1).
- **Produce:** el `id` `router` y un `id` por agente: `claude`, `codex`, `copilot`, `gemini` y `antigravity`.

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `'la tarjeta router llega a los cinco agentes'`: un SVG con cinco destinos, cada uno con su nombre visible;
  - `'cada agente dice cómo se conecta'`: suscripción o clave, como en la sección de hoy; Antigravity, solo con clave;
  - `'cada agente enlaza a Preparar tus agentes'`: un enlace a `ayuda/preparar.html`;
  - `'ya no hay línea de metro'`: no queda la clase de la línea de hoy; se busca su nombre en el `index.html` actual.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: escribir la página** según la spec §5, «Agentes».
- [ ] **Paso 4:** pruebas en verde y revisión en el navegador.
- [ ] **Paso 5: commit**, `feat(sitio): página Agentes con la tarjeta router`.

### Tarea 8: la plantilla de documentación (Ayuda, Servidor y Desarrolladores)

**Archivos:**
- Modificar: `web/scripts/sitio.ts`, en `pagina()`, `paginaTema()` y `paginaInicio()` (la Ayuda en tres columnas) y en lo
  nuevo de `columnasDoc()`
- Modificar: `site/servidor.html` y `site/desarrolladores.html`, en tres columnas y con su titular en dos partes
- Modificar: `site/sitio.js`, para marcar el apartado actual al bajar y abrir el índice en el móvil
- Quitar de `estilo.css`: las reglas de la Ayuda vieja que ya no use nadie
- Probar: `web/scripts/sitio.test.ts`

**Interfaces:**
- **Produce:** `export function columnasDoc(indice: string, contenido: string, enEstaPagina: { id: string; es: string;
  en: string }[]): string`. Devuelve `<div class="doc"><nav class="doc-indice">…</nav><article
  id="contenido">…</article><aside class="doc-pagina">…</aside></div>`. En el móvil, el índice se abre con un botón
  `.doc-indice-boton` (`aria-expanded`).

- [ ] **Paso 1: escribir las pruebas que fallan:**
  - `'cada tema de la Ayuda va en tres columnas'`: `.doc-indice` con todos los temas y el actual con `aria-current`, y
    `.doc-pagina` con sus apartados;
  - `'Servidor y Desarrolladores, en tres columnas'`: cada `<h2 id>` del contenido aparece en «En esta página»;
  - `'los textos de la Ayuda no cambian'`: el texto de cada tema generado contiene sus cadenas de `ayuda.es.ts`, como
    hoy.
- [ ] **Paso 2:** ejecutarlas. Tienen que fallar.
- [ ] **Paso 3: implementarlo** según la spec §6. Se mantienen los diagramas de hoy de `servidor.html` (el mapa) con el
  estilo nuevo, y en Desarrolladores se añade el de «tu app → Tramtai → agente».
- [ ] **Paso 4:**
  - `npm test`, `npm run sitio` y `go test ./cmd/tramtai -run TestComandosDelSitioExisten` pasan;
  - en el navegador: el tema más largo de la Ayuda y `servidor.html` a 375 px sin desplazamiento horizontal, con el
    índice abriéndose y cerrándose con teclado.
- [ ] **Paso 5: commit**, `feat(sitio): plantilla de documentación de tres columnas`.

### Tarea 9: los recortes y la imagen para compartir

**Archivos:**
- Modificar: `web/scripts/capturas-lista.ts`, para añadir los recortes `portada-fig-apis`, `portada-fig-servidor` y los
  de las tarjetas de funciones de Producto, en es/en × claro/oscuro
- Modificar: `web/scripts/og.html`, con el estilo nuevo, y rehacer `site/og.png` (1200×630)
- Modificar: `site/index.html` y `site/producto.html`, para usar los recortes en lugar de las capturas provisionales
- Probar: `web/scripts/capturas.test.ts`

**Interfaces:**
- **Consume:** los `id` de figura de las Tareas 4 y 6.
- **Produce:** los archivos `site/capturas/<nombre>-<es|en>-<claro|oscuro>.webp`, con el patrón de hoy.

- [ ] **Paso 1: escribir la prueba que falla:** `'los recortes nuevos están en la lista y en el sitio'`. Cada nombre
  nuevo está en `CAPTURAS` y cada `<img>` de Inicio y Producto apunta a un archivo que existe en los cuatro sabores.
- [ ] **Paso 2:** ejecutarla. Tiene que fallar.
- [ ] **Paso 3: sacar solo los recortes nuevos** con `npm run capturas -- --solo <nombre>` y contar las peticiones
  reales. Rehacer `og.png`.
- [ ] **Paso 4:** pruebas en verde. Los recortes no muestran datos personales.
- [ ] **Paso 5: commit**, `feat(sitio): recortes del mosaico y og.png nuevos`.

### Tarea 10: documentación

**Archivos:**
- Reescribir: `docs/diseno/DESIGN.md`, con el sistema nuevo
- Modificar: `README.md`, las partes en español y en inglés, con enlaces a las páginas nuevas; `#descargar` pasa a
  `descargar.html`
- Modificar: `docs/equipo/desarrollo.md`, donde habla del sitio (las páginas, `columnasDoc`, `buscar.json`,
  `descargas.mjs` con varias páginas y las letras)
- Modificar: `docs/desarrolladores.md`, solo los enlaces al sitio que hayan cambiado

- [ ] **Paso 1:** buscar `github.io/Tramtai` y `#descargar` en `README.md` y `docs/`, y apuntar cada uno a su página
  nueva.
- [ ] **Paso 2: escribir `DESIGN.md`** con las variables, las letras, las clases, el movimiento y las reglas de
  accesibilidad de la Tarea 1.
- [ ] **Paso 3:** `npm test` (el estilo de los textos también mira el README) pasa.
- [ ] **Paso 4: commit**, `docs(v0.61): sistema visual nuevo y enlaces al sitio`.

### Tarea 11: cierre

**Archivos:**
- Modificar: `docs/equipo/pendientes.md`, la fila de la v0.61
- Modificar: la spec, con los Rulings que cambien el diseño

- [ ] **Paso 1: auditoría de diseño y accesibilidad** de las siete páginas y la 404:
  - con `impeccable`, `antislop:antislop-ui` y teclado;
  - a 375 y 1440 px, en claro y oscuro, en es y en;
  - con contraste AA, con «reducir movimiento» y sin JavaScript.

  Se arregla lo real. Los avisos del detector que no son reales se ignoran con su motivo en `.impeccable/config.json`.
- [ ] **Paso 2: revisión de la rama entera.** Se usa `superpowers:requesting-code-review` con el foco de «Lo que más
  puede fallar». Los Minor se arreglan aquí.
- [ ] **Paso 3:** `go vet ./...`, `go test ./...` y `cd web; npm test; npm run build; npm run sitio` pasan, y
  `npm run sitio` no deja cambios.
- [ ] **Paso 4:** `pendientes.md` con la v0.61 lista. Se enseña al usuario el sitio en el navegador, desde una copia
  local de `site/`.
- [ ] **Paso 5: commit**, `docs(v0.61): cierre`.
- [ ] **Paso 6: con el «sí» del usuario**, fusionar los PR en orden (cambiando antes su base a `main`) y publicar con
  `gh workflow run paginas.yml`.
