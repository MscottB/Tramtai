# Portada renovada de la v0.56: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** rehacer cuatro secciones de la portada:
- «Los agentes que ya pagas»;
- la cabecera;
- el recorrido;
- «Una red con una línea por agente».

Así la portada enseña el problema, la solución y lo que hace la app.

**Architecture:**
- **Archivos:** la portada es HTML escrito a mano (`site/index.html`) con `site/estilo.css` y `site/sitio.js`.
- **Pruebas:** la vigila `web/scripts/portada.test.ts`, que lee el HTML y el JS como texto.
- **Capturas:** las del recorrido las saca `web/scripts/capturas.mjs`.
- **Mejora progresiva:** sin JavaScript se ve todo y el JS solo añade la interacción.

**Tech Stack:** HTML, CSS y JS sin dependencias; pruebas con `node:test` (`npm --prefix web test`); capturas con Edge por CDP.

**Spec:** `docs/equipo/releases/v0.56-terminal/portada.md`

## Global Constraints

- **Idiomas:** todo texto visible en `<span lang="es">…</span><span lang="en">…</span>`, como el resto de la portada.
- **Nombres para lectores:** los botones llevan su nombre en spans `solo-lectores` por idioma, nunca en `aria-label` con los dos idiomas (patrón de `portada.test.ts`).
- **Temas:** claro y oscuro con las variables de color que ya define `site/estilo.css` (tokens de DESIGN.md); ningún color nuevo suelto.
- **Sin brillos:** nada de `box-shadow` ni `text-shadow` de color.
- **Pantallas:** de 360 px a escritorio, sin desplazamiento horizontal; el texto de lectura, a 14 px como mínimo.
- **Teclado:** todo se maneja con el teclado y con el foco visible.
- **Movimiento:** con `prefers-reduced-motion`, nada gira ni avanza solo.
- **Textos:** glosario Espacio · API · Petición, «costo», «Playground» en inglés; sin promesas absolutas («nunca», «siempre», «sin cortes», «a nadie»).
- **Commits:** en español, con `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; uno por tarea como mínimo.

## Review Focus

1. **A 360 px de ancho**, ninguna sección nueva provoca desplazamiento horizontal (los SVG y el plano se adaptan). Lo comprueba la Tarea 6, paso 2.
2. **Sin JavaScript** se leen los seis motivos, el código de la primera escena, la lista de pasos con sus textos y las 12 funciones con su explicación. Lo comprueban las pruebas de las Tareas 1, 2, 4 y 5 (leen el HTML estático) y la Tarea 6, paso 4.
3. **Al cambiar a inglés** a mitad de la rotación, las pestañas, el panel y la leyenda salen en inglés: todo texto tiene sus dos spans (pruebas de las Tareas 2, 4 y 5).
4. **En tema claro**, las líneas de color y el texto sobre las paradas tienen contraste suficiente: al menos 3:1 para las líneas y 4,5:1 para el texto (Tarea 6, paso 3).
5. **Las flechas ← → del recorrido** solo actúan con el foco dentro de la sección; no secuestran el teclado del resto de la página (prueba de la Tarea 4).

## Decisión ya tomada

`portada.test.ts` tiene el test «“Por qué Tramtai” es la franja, sin motivos con iconos que repitan Funciones (auditoría 001, 53)». La spec aprobada recupera los motivos, así que ese test se sustituye en la Tarea 1:
- los motivos cuentan el beneficio en una línea;
- el detalle queda en el plano de Funciones (Tarea 5).

---

### Task 1: «Los agentes que ya pagas» dibujado, con sus seis motivos

**Files:**
- Modify: `site/index.html` (la sección de «Los agentes que ya pagas», que pasa justo después de la cabecera, antes de «Conecta estos agentes»)
- Modify: `site/estilo.css`
- Test: `web/scripts/portada.test.ts`

**Interfaces:**
- Produces: `<section id="pagas" class="destacado">` con:
  - `.pagas-diagramas`, que contiene dos `<figure class="pagas-diagrama">` («sin» y «con», cada uno con su SVG en línea y su `figcaption`);
  - `<ul class="motivos">`, con seis `<li class="motivo">`.

- [ ] **Step 1: Cambia la prueba de la auditoría 53 y añade las nuevas** en `portada.test.ts`. Borra el test «“Por qué Tramtai” es la franja…» y añade:

```ts
const pagas = seccion('pagas')
test('«Los agentes que ya pagas» va justo después de la cabecera, antes de los agentes', () => {
  assert.ok(pagas, 'falta <section id="pagas">')
  assert.ok(html.indexOf('id="pagas"') < html.indexOf('Conecta estos agentes'))
  assert.ok(html.indexOf('id="pagas"') < html.indexOf('id="como"'))
})
test('«Los agentes que ya pagas» dibuja el antes y el después, con texto visible', () => {
  const figuras = pagas.match(/<figure class="pagas-diagrama[^"]*">[\s\S]*?<\/figure>/g) ?? []
  assert.equal(figuras.length, 2)
  for (const f of figuras) { assert.match(f, /<svg[^>]*role="img"/); assert.match(f, /<figcaption>[\s\S]*lang="es"[\s\S]*lang="en"/) }
  assert.match(pagas, /Una clave, un SDK y una factura por proveedor/)
  assert.match(pagas, /Una URL local y la suscripción o la clave que ya tienes/)
})
test('los seis motivos, en los dos idiomas y sin promesas absolutas', () => {
  const motivos = pagas.match(/<li class="motivo">[\s\S]*?<\/li>/g) ?? []
  assert.equal(motivos.length, 6)
  for (const t of ['Lo que ya pagas', 'En tu equipo', 'Compatible', 'Respaldo', 'Por proyecto', 'Bajo control']) assert.match(pagas, new RegExp(t))
  for (const m of motivos) assert.match(m, /lang="es"[\s\S]*lang="en"/)
  assert.doesNotMatch(pagas, /\b(nunca|siempre|sin cortes|a nadie|never|always|no one)\b/i)
})
```

- [ ] **Step 2: Corre `npm --prefix web test`.** Expected: FAIL en los tres tests nuevos.
- [ ] **Step 3: Escribe la sección** con el contenido exacto de la spec (§1): pies de los diagramas, títulos y textos de los motivos.
  - Inglés de los motivos: «What you already pay for», «On your computer», «Compatible», «Fallback», «Per project», «In control».
  - Los iconos, en SVG de línea dentro de cada `li`, con `aria-hidden="true"`.
  - Los diagramas, en SVG con `role="img"`, y su texto en el `figcaption` en los dos idiomas; las líneas de cada agente, con sus variables de color.
  - En el móvil (<720 px), los diagramas uno sobre otro y los motivos en una columna; en tableta, en dos.
- [ ] **Step 4: Corre `npm --prefix web test`.** Expected: PASS.
- [ ] **Step 5: Commit:** `feat(sitio): «Los agentes que ya pagas» con el antes y el después y sus seis motivos`.

### Task 2: La cabecera con el código de cada escena

**Files:**
- Modify: `site/index.html`:
  - el JSON `#ejemplos-mapa`;
  - el bloque del mapa;
  - los `<pre id="codigo-mapa-N">`, que pasan al panel de la cabecera;
  - «Tres paradas», que se queda con un ejemplo fijo `<pre class="codigo" id="codigo-como">`, copia del primero.
- Modify: `site/sitio.js` (el bloque `// Recorrido de la portada`)
- Modify: `site/estilo.css`
- Test: `web/scripts/portada.test.ts`

**Interfaces:**
- Produces:
  - **Las pestañas:** `<div class="mapa-pestanas" role="tablist">` con cuatro `<button type="button" role="tab" class="mapa-pestana" aria-selected="true|false" aria-controls="mapa-panel">`.
  - **El panel:** `<div class="mapa-panel" id="mapa-panel" role="tabpanel">`, con el título, `p.mapa-leyenda` y el `pre` de la escena.
  - **El JSON:** cada ejemplo añade `"titulo_es"` y `"titulo_en"`.

- [ ] **Step 1: Pruebas.** En `portada.test.ts`, sustituye «el recorrido se puede pausar y elegir» y el test de los puntos (`mapa-punto`) por:

```ts
test('la cabecera tiene cuatro pestañas con nombre y un panel con el código de la escena', () => {
  const pestanas = html.match(/<button type="button" role="tab" class="mapa-pestana"[\s\S]*?<\/button>/g) ?? []
  assert.equal(pestanas.length, 4)
  const titulos = [['Resume un contrato con Claude', 'Summarize a contract with Claude'], ['Un PDF a JSON con Codex', 'A PDF to JSON with Codex'],
    ['Traduce con GitHub Copilot', 'Translate with GitHub Copilot'], ['Respaldo: de Claude a Codex', 'Fallback: from Claude to Codex']]
  pestanas.forEach((p, i) => { assert.match(p, new RegExp(titulos[i][0])); assert.match(p, new RegExp(titulos[i][1])) })
  assert.match(pestanas[0], /aria-selected="true"/)
  const panel = html.slice(html.indexOf('<div class="mapa-panel" id="mapa-panel" role="tabpanel"'))
  assert.ok(panel.length > 0, 'falta el panel de la escena')
  assert.match(panel.slice(0, 4000), /id="codigo-mapa-1"/, 'sin JavaScript se ve el código de la primera escena')
  assert.match(html, /<pre class="codigo[^"]*" id="codigo-como"/, '«Tres paradas» tiene su ejemplo fijo')
  assert.match(html, /class="mapa-pausa"/)
})
test('las pestañas se manejan con las flechas y paran la rotación', () => {
  assert.match(tranvia, /ArrowRight/)
  assert.match(tranvia, /aria-selected/)
  assert.match(tranvia, /pausaManual = /)
})
```

  - El test del JSON añade `assert.ok(e.titulo_es && e.titulo_en)`.
  - Los tests que siguen valiendo se quedan: `escribirLeyenda`, la pausa, «reducir movimiento» y el `IntersectionObserver`.

- [ ] **Step 2: Corre `npm --prefix web test`.** Expected: FAIL en los nuevos.
- [ ] **Step 3: Implementa.**
  - **Disposición:** en el escritorio (≥980 px), el mapa a la izquierda y el panel a la derecha; por debajo, el panel bajo el mapa.
  - **Pestañas:** en `sitio.js`, las pestañas sustituyen a `.mapa-punto`:
    - pulsar una pestaña elige la escena, la marca con `aria-selected` y fija `pausaManual = true`;
    - ← → mueven el foco y la elección entre pestañas (patrón de tablist);
    - la rotación pasa a la siguiente pestaña.
  - **Panel:** enseña el `pre` de la escena y oculta los demás. La línea que la explica es el `es` y el `en` del JSON, en la leyenda con `escribirLeyenda`.
  - **«Tres paradas»** deja de cambiar con las escenas.
- [ ] **Step 4: Corre `npm --prefix web test`.** Expected: PASS.
- [ ] **Step 5: Commit:** `feat(sitio): la cabecera enseña el código de cada escena junto al mapa`.

### Task 3: Las capturas del recorrido, recortadas a la zona del paso

**Files:**
- Modify: `web/scripts/capturas.mjs` (el bucle de la bienvenida, líneas ~289-317)
- Test: `web/scripts/portada.test.ts`. Se queda el test de nombres y pesos de `bienvenida-<n>-<tema>-<idioma>-<960|1440>.webp`, que sigue igual.

**Interfaces:**
- Produces: las mismas rutas de archivo, con un recorte de proporción 16:10 (por eso los `<img>` siguen con `width="1440" height="900"`).
- Consumes: el `.driver-active-element` del recorrido de la app.

- [ ] **Step 1: Implementa en `capturas.mjs` una función `zonaDelPaso()`** que devuelve `{ x, y, width, height }`. Se evalúa en la página y hace esto:
  1. Toma el rectángulo de `.driver-active-element` (sin elemento, la pantalla entera 1440×900).
  2. Le suma 32 px por lado y lo agranda a proporción 16:10 alrededor de su centro, con un ancho mínimo de 720 px.
  3. Lo encaja dentro de 1440×900.
  - Antes de capturar oculta `.driver-popover` (`visibility:hidden`) y después lo devuelve.
  - Cada captura usa `clip: { ...zona, scale: ancho / zona.width }` para `ancho` en `[1440, 960]`.
  - Actualiza el comentario del bloque: ya no es la pantalla completa con la ventana encima.
- [ ] **Step 2: Corre las capturas de la bienvenida.**
  - Comando: `npm --prefix web run build`; después, en `web/`, `npm run capturas -- --bienvenida`.
  - Usa una copia portable y temporal en un puerto libre, y no hace peticiones a agentes.
  - Expected: líneas «site/capturas/bienvenida-…» para los 14 pasos × 2 temas × 2 idiomas × 2 anchos.
- [ ] **Step 3: Corre `npm --prefix web test`.** Expected: PASS en el test de pesos (≤150 KB cada una de 1440, ≤6 MB en total). Si alguna pesa más, baja `quality` de 80 a 72.
- [ ] **Step 4: Mira a ojo los pasos 1, 5 y 14** en claro y en oscuro: la zona señalada se lee.
- [ ] **Step 5: Commit:** `feat(capturas): el recorrido de la portada recorta cada paso a la zona que señala`.

### Task 4: La demo con la lista de pasos al lado y flechas a los costados

**Files:**
- Modify: `site/index.html` (`<section id="demo">`)
- Modify: `site/sitio.js` (el bloque `// Demo del recorrido guiado`)
- Modify: `site/estilo.css`
- Test: `web/scripts/portada.test.ts`

**Interfaces:**
- Consumes: las capturas de la Tarea 3 (mismos nombres) y `pasosBienvenida()` (`pagina`, `titulo`, `texto`).
- Produces, dentro de `.demo`:
  - **La lista:** `<nav class="demo-pasos">`, con un `<details class="demo-pasos-movil" open>` en el móvil. Dentro va `ol.demo-lista`, con un grupo por página: `<li class="demo-grupo">`, el nombre de la página y un `<ol>` de `<li><button type="button" class="demo-ir" data-paso="n">…</button></li>`.
  - **El escenario:** `.demo-escenario`, con `button.demo-flecha.demo-anterior`, las figuras y `button.demo-flecha.demo-siguiente`.
  - **El progreso:** `div.demo-progreso > span`.
  - **Cada figura:** `figcaption` con el título y el texto del paso, en los dos idiomas.

- [ ] **Step 1: Pruebas.**
  - Sustituye el test de la lista y el de los puntos de la demo por los de abajo.
  - Se queda el test de las figuras.

```ts
test('la lista de pasos va agrupada por página, con los títulos reales del recorrido', () => {
  const botones = [...demo.matchAll(/<button type="button" class="demo-ir" data-paso="(\d+)"><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/button>/g)]
  assert.deepEqual(botones.map((m) => [m[2], m[3]]), pasos.map((p) => [es[p.titulo], en[p.titulo]]))
  assert.ok((demo.match(/class="demo-grupo"/g) ?? []).length >= 5)
})
test('cada paso enseña su texto debajo de la imagen, en los dos idiomas', () => {
  const escapar = (s: string) => s.slice(0, 30).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  figuras.forEach((f, i) => {
    assert.match(f, new RegExp(`<figcaption>[\\s\\S]*${escapar(es[pasos[i].texto])}`))
    assert.match(f, new RegExp(`<figcaption>[\\s\\S]*${escapar(en[pasos[i].texto])}`))
  })
})
test('flechas a los costados, con su nombre en cada idioma, y sin puntos', () => {
  for (const [clase, es_, en_] of [['demo-anterior', 'Paso anterior', 'Previous step'], ['demo-siguiente', 'Paso siguiente', 'Next step']])
    assert.match(demo, new RegExp(`<button type="button" class="demo-flecha ${clase}"[\\s\\S]*?solo-lectores">${es_}<[\\s\\S]*?solo-lectores">${en_}<`))
  assert.doesNotMatch(demo, /class="demo-punto"/)
  assert.match(demo, /class="demo-progreso"/)
})
test('las flechas del teclado solo actúan con el foco en la demo', () => {
  assert.match(demoJs, /caja\.addEventListener\('keydown'/)
  assert.doesNotMatch(demoJs, /document\.addEventListener\('keydown'/)
})
```

- [ ] **Step 2: Corre `npm --prefix web test`.** Expected: FAIL en los nuevos.
- [ ] **Step 3: Implementa la sección.**
  - **Escritorio (≥900 px):** la lista a la izquierda, de unos 260 px; el escenario a la derecha, con las flechas de 44 px a los costados y el progreso debajo.
  - **Móvil:** la lista dentro del `details`, encima.
  - **Nombres de los grupos:** los de las páginas de la app (las mismas claves del menú que usa la Ayuda, en es y en).
  - **JS:**
    - los botones de la lista y las flechas eligen el paso;
    - la flecha que no lleva a ninguna parte se queda `disabled`;
    - ← → solo con un `keydown` en `caja`;
    - el avance solo, la pausa, `escribirLeyenda`, `precargar` y «reducir movimiento» siguen como hoy;
    - la leyenda dice «Paso n de 14 · Página».
- [ ] **Step 4: Corre `npm --prefix web test`.** Expected: PASS.
- [ ] **Step 5: Commit:** `feat(sitio): el recorrido de la portada con la lista de pasos y flechas a los costados`.

### Task 5: «Una red con una línea por agente», un plano de metro

**Files:**
- Modify: `site/index.html` (`<section id="funciones">`)
- Modify: `site/sitio.js` (un bloque nuevo, `// Plano de metro de Funciones`)
- Modify: `site/estilo.css`
- Test: `web/scripts/portada.test.ts`

**Interfaces:**
- Produces: `div.metro`, con cuatro `section.metro-linea[data-linea="conecta|controla|protege|usa"]`.
  - Cada línea lleva un `h3` y un `ol` de tres `li.parada`.
  - Cada `li.parada` lleva `button.parada-boton` (el título) y `p.parada-texto` (el texto de hoy).
  - Hay además un `div.metro-panel` (`aria-live="polite"`) que solo usa el JS.

- [ ] **Step 1: Pruebas.**

```ts
test('el plano tiene cuatro líneas y doce paradas con su texto en el HTML (sin JavaScript se lee todo)', () => {
  const funciones = seccion('funciones')
  assert.equal((funciones.match(/<section class="metro-linea" data-linea="(conecta|controla|protege|usa)"/g) ?? []).length, 4)
  const paradas = funciones.match(/<li class="parada">[\s\S]*?<\/li>/g) ?? []
  assert.equal(paradas.length, 12)
  for (const p of paradas) { assert.match(p, /<button type="button" class="parada-boton"/); assert.match(p, /<p class="parada-texto">[\s\S]*lang="es"[\s\S]*lang="en"/) }
  for (const t of ['Compatible con tus SDK', 'Una API por aplicación', 'Respaldo entre agentes', 'La terminal, a la par de la app', 'Recorrido y Ayuda'])
    assert.match(funciones, new RegExp(t))
  assert.match(funciones, /href="#demo"/)
})
```

  - La sección `funciones` contiene `<section>` anidadas, así que la función `seccion()` (que corta en el primer `</section>`) no sirve aquí. Antes, sustitúyela por una versión que respete el anidamiento, o usa `html.slice` entre `id="funciones"` y `id="descargar"`.
  - Se quedan los tests de hoy que miran Funciones: «Recorrido y Ayuda», `href="#demo"` y Espacios.
- [ ] **Step 2: Corre `npm --prefix web test`.** Expected: FAIL.
- [ ] **Step 3: Implementa.**
  - **Sin JS, o por debajo de 900 px:** cada línea es vertical (borde izquierdo de 5 px con su color; cada parada, un círculo sobre la línea), con el texto visible.
  - **Con JS y ≥900 px:** `sitio.js` añade `metro--plano` a `.metro`.
    - El CSS pinta las cuatro líneas en horizontal, con las paradas encima y los títulos debajo, y oculta los `p.parada-texto`.
    - Pulsar una parada copia su texto al `metro-panel` y la marca con `aria-pressed="true"`.
    - Empieza elegida la primera.
  - **Colores:** las variables de color que ya usa el mapa.
- [ ] **Step 4: Corre `npm --prefix web test`.** Expected: PASS.
- [ ] **Step 5: Commit:** `feat(sitio): las funciones como un plano de metro`.

### Task 6: Comprobación final de la portada

**Files:** los de las tareas anteriores, solo si hay que arreglar algo.

- [ ] **Step 1: Captura la portada con Edge sin ventana** (`--headless=new --screenshot --window-size=<ancho>,11000`, servida con el servidor `pagina` de `.claude/launch.json`).
  - Tres anchos: 1440, 768 y 390 px.
  - En claro y en oscuro; en es y en.
  - Revisa las cuatro secciones.
- [ ] **Step 2: Mide el ancho de la página a 360 px** en el navegador integrado, emulando 360×800. Expected: `document.documentElement.scrollWidth <= 360`.
- [ ] **Step 3: Comprueba el contraste en claro** de las líneas del plano y del texto de las paradas y del panel (`check_contrast` de antislop). Expected: al menos 3:1 en las líneas y 4,5:1 en el texto.
- [ ] **Step 4: Comprueba sin JavaScript** (Edge `--blink-settings=scriptEnabled=false`): se ven los seis motivos, el código de la primera escena, la lista de pasos con sus textos y las 12 funciones con su texto.
- [ ] **Step 5: Corre `npm --prefix web test` y `npm --prefix web run sitio`.** Expected: PASS, y `git status` sin cambios tras `sitio`.
- [ ] **Step 6: Commit de lo que haya hecho falta arreglar:** `fix(sitio): …`.
