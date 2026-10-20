# Ayuda renovada (v0.52) — plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Una sola Ayuda, por temas y visual, que la app muestra (con un **?** en cada página) y el sitio genera desde los mismos datos.

**Architecture:** Los temas son datos TypeScript en `web/src/ayuda/` con bloques de tipos fijos y textos por clave en
`idiomas/ayuda.es.ts`/`ayuda.en.ts`. La app los dibuja con componentes Vue (inicio, tema, panel lateral); un script de
Node genera `site/ayuda/*.html` desde los mismos datos y el CI comprueba que el HTML guardado coincide. Una prueba de
estilo convierte la regla («corto, con diagrama, con su tema por página») en un fallo del CI.

**Tech Stack:** Vue 3 + Quasar, TypeScript, `node:test` (`npm test`), Node 24 (lee TypeScript), HTML/CSS estáticos del
sitio, CDP de Edge/Chrome para las capturas.

**Spec:** `docs/equipo/releases/v0.52-ayuda/spec.md`

## Global Constraints

- Todo texto nuevo en **español e inglés**; las claves de `ayuda.en.ts` son las de `ayuda.es.ts` (tipo `Textos`).
- **Sin dependencias nuevas** (ni npm ni Go). Las pruebas son `node:test` sobre `.ts`; no hay infraestructura para
  montar `.vue` en pruebas, así que **la lógica de los componentes va en `.ts` probados** y el dibujo se comprueba con
  `vue-tsc` y en el navegador (tarea 9).
- Resumen de un tema: **≤ 140 caracteres** por idioma. Paso, nota, parada, respuesta: **≤ 200 caracteres**.
- Capturas: **WebP**, `web/src/ayuda/capturas/<id>.<claro|oscuro>.<es|en>.webp`, **total ≤ 3 MB**.
- Nombres de ejemplo **neutros** («Asistente de soporte», «Resumen de notas»); nada que suene a costos o datos reales.
- La Web UI compilada (`internal/recursos/web`) y el HTML generado (`site/ayuda/`, `site/ayuda.html`) **van en el
  repo**; el CI compara ambos.
- Nunca push a `main`: todo en `feat/v0.52-ayuda` y por PR.
- Comentarios y nombres en español, con la densidad del código vecino.

## Review Focus

1. `#ayuda/<id>` con un id que no existe, o `#ayuda/` vacío → abre el inicio de la Ayuda, no una página en blanco
   (prueba en tarea 5).
2. Búsqueda con tildes, mayúsculas, espacios de más o caracteres de expresión regular (`(`, `?`, `*`) → encuentra sin
   romperse (prueba en tarea 4).
3. Títulos o ejemplos con `<`, `>`, `&` o comillas en el sitio generado → salen escapados, sin HTML roto (prueba en
   tarea 7).
4. Ancla vieja de `ayuda.html` desconocida (o ninguna) → lleva a `ayuda/` (prueba en tarea 7).
5. Abrir el **?** y cambiar de idioma o de tema claro/oscuro con el panel abierto → textos y captura cambian sin cerrar
   el panel (`varianteCaptura` probada en tarea 5; comprobación en el navegador en tarea 9).

---

### Task 1: Textos de la Ayuda en su propio archivo

**Files:**
- Create: `web/src/idiomas/ayuda.es.ts`, `web/src/idiomas/ayuda.en.ts`
- Modify: `web/src/idiomas/es.ts`, `web/src/idiomas/en.ts` (quitar las claves `ayuda.*` y sumar `...ayudaEs`/`...ayudaEn`)
- Test: `web/src/idiomas/ayuda.test.ts`

**Interfaces:**
- Produces: `ayuda.es.ts` exporta `default` un objeto con todas las claves `ayuda.*` (las que hoy están repartidas
  en `es.ts`, por ejemplo líneas 212, 444–534, 553–589, 938–981). `ayuda.en.ts` exporta las mismas con el tipo
  `Record<keyof typeof ayudaEs, string>`. `Clave` y `Textos` de `es.ts` siguen incluyéndolas.

- [ ] **Step 1: Escribir la prueba**

```ts
test('los textos de la Ayuda viven en su archivo y en los dos idiomas', () => {
  assert.deepEqual(Object.keys(ayudaEn).sort(), Object.keys(ayudaEs).sort())
  assert.ok(Object.keys(ayudaEs).every((k) => k.startsWith('ayuda.')))
  assert.equal(Object.keys(es).filter((k) => k.startsWith('ayuda.')).length, Object.keys(ayudaEs).length)
})
```

- [ ] **Step 2:** `cd web && npm test` → FAIL (no existe `ayuda.es.ts`).
- [ ] **Step 3:** Mover las claves `ayuda.*` de `es.ts`/`en.ts` a los archivos nuevos y esparcirlas en los objetos
  `es`/`en`. Sin cambiar ningún texto.
- [ ] **Step 4:** `npm test && npm run build` → PASS; la Ayuda se ve igual.
- [ ] **Step 5:** Commit `refactor(web): textos de la Ayuda en su propio archivo`.

### Task 2: Tipos, índice, regla del estilo y los temas de las páginas

**Files:**
- Create: `web/src/ayuda/tipos.ts`, `web/src/ayuda/indice.ts`, `web/src/ayuda/estilo.test.ts`,
  `web/src/ayuda/temas/{empezar,agentes,probar,conectar,historial,ajustes}.ts`
- Modify: `web/src/idiomas/ayuda.es.ts`, `ayuda.en.ts` (textos nuevos y reescritos), `web/src/componentes/TablaApis.vue`

**Interfaces:**
- Produces (`tipos.ts`):

```ts
export type Pagina = 'inicio' | 'agentes' | 'ejecutar' | 'conectar' | 'historial' | 'configuracion'
export interface Parada { icono: string; titulo: Clave; nota?: Clave }
export type Bloque =
  | { tipo: 'diagrama'; paradas: Parada[]; ramas?: { desde: number; hacia: number; etiqueta: Clave }[] }
  | { tipo: 'pasos'; pasos: Clave[] }
  | { tipo: 'captura'; id: string; alt: Clave }
  | { tipo: 'ejemplo'; prueba: PruebaEjemplo; lenguajes: string[] }   // PruebaEjemplo: el argumento de ejemplos()
  | { tipo: 'tabla'; columnas: Clave[]; filas: { titulo: Clave; celdas: Clave[] }[] }
  | { tipo: 'preguntas'; preguntas: { id: string; pregunta: Clave; respuesta: Clave }[] }
  | { tipo: 'errores'; codigos: string[] }
  | { tipo: 'nota'; texto: Clave }
  | { tipo: 'ir'; pagina: Pagina }
  | { tipo: 'descargas' }
export interface Tema { id: string; icono: string; titulo: Clave; resumen: Clave; paginaApp?: Pagina;
  conDiagrama: boolean; relacionados: string[]; bloques: Bloque[] }
```

- Produces (`indice.ts`): `export const temas: Tema[]` (en el orden de la spec, sección 4) y
  `export function temaDe(pagina: Pagina): Tema | undefined`, `export function temaPorId(id: string): Tema | undefined`.
  Mientras falten temas (tarea 3), `temas` lista solo los creados.
- Produces: `export const tablaApis: Bloque` (tipo `tabla`) en `temas/conectar.ts`; `TablaApis.vue` lo dibuja.
- Los temas solo importan **tipos** de `idiomas/es` (`import type`), nunca `i18n.ts`, para que Node los lea en la
  tarea 7.

- [ ] **Step 1: Escribir `estilo.test.ts`** (lee los textos de `ayuda.es.ts`/`ayuda.en.ts`):
  - `resumen` presente y ≤ 140 en es y en;
  - cada paso, nota, `Parada.titulo`/`nota`, respuesta y celda ≤ 200 en es y en;
  - `conDiagrama` ⇒ tiene bloque `diagrama`;
  - las 6 páginas de `Pagina` tienen `temaDe(p)` (la Ayuda no);
  - cada `relacionados` existe; cada `errores.codigos` está en `CODIGOS_ERROR` (exportado desde `temas/problemas.ts`
    en la tarea 3; hasta entonces la comprobación se salta si el tema no está);
  - ids de tema únicos y en minúsculas (`/^[a-z]+$/`).
- [ ] **Step 2:** `npm test` → FAIL (no hay temas).
- [ ] **Step 3:** Escribir `tipos.ts`, `indice.ts` y los seis temas con su contenido según la tabla «Qué pasa con cada
  texto actual» de la spec (filas de `empezar`, `agentes`, `probar`, `conectar`, `historial`, `ajustes`, más lo que
  viene del sitio: Instalar, Conectar los agentes con `#terminos`, Conectar tu aplicación, Una API por aplicación,
  Actualizaciones). Reescribir al estilo. Mover los datos de `TablaApis.vue` a `tablaApis` y que el componente los
  dibuje. Los bloques `captura` ya citan sus ids (la existencia se comprueba en la tarea 8).
- [ ] **Step 4:** `npm test && npm run build` → PASS.
- [ ] **Step 5:** Commit `feat(ayuda): temas como datos y la regla del estilo como prueba`.

### Task 3: Los demás temas y el contenido viejo retirado

**Files:**
- Create: `web/src/ayuda/temas/{archivos,json,recetas,herramientas,presupuestos,problemas,conceptos,preguntas}.ts`
- Modify: `web/src/ayuda/indice.ts`, `web/src/idiomas/ayuda.es.ts`, `ayuda.en.ts`

**Interfaces:**
- Consumes: `Tema`, `Bloque` (tarea 2).
- Produces: `export const CODIGOS_ERROR: readonly string[]` en `temas/problemas.ts` (los 13 códigos de hoy, de
  `Ayuda.vue:260-274`); `conceptos.ts` con un bloque `preguntas` (id `c-<término>`) por concepto, cuya respuesta
  termina nombrando su tema; `indice.temas` completo con los 14 temas en el orden de la spec;
  `export const CLAVES_UI: Clave[]` en `indice.ts` (textos del inicio, del panel y de la vista de tema).

- [ ] **Step 1:** Añadir a `estilo.test.ts`: `temas.length === 14`; ninguna clave `ayuda.*` sin usar (las de los
  bloques de todos los temas más `CLAVES_UI`); los ids de pregunta son únicos en toda la Ayuda.
- [ ] **Step 2:** `npm test` → FAIL.
- [ ] **Step 3:** Escribir los ocho temas según la tabla de la spec (guías `g2`–`g6`, preguntas restantes, conceptos,
  errores, sitio: Herramientas, Recetas, Archivos y JSON, «¿Puedo usarlo en mi empresa?»). Borrar las claves que
  ya nadie usa.
- [ ] **Step 4:** `npm test && npm run build` → PASS.
- [ ] **Step 5:** Commit `feat(ayuda): los 14 temas con el contenido de la app y del sitio`.

### Task 4: Búsqueda

**Files:**
- Create: `web/src/ayuda/buscar.ts`, `web/src/ayuda/buscar.test.ts`

**Interfaces:**
- Consumes: `temas` (tarea 3).
- Produces:

```ts
export interface Resultado { tema: string; ancla: string; titulo: string; donde: string; puntos: number }
export function normalizar(s: string): string            // minúsculas, sin tildes (NFD), espacios colapsados
export function buscar(consulta: string, temas: Tema[], textos: Record<string, string>, tituloError: (c: string) => string): Resultado[]
```

  `ancla` es el id de la pregunta o del error (`'p-formatos'`, `'e-conversion_fallida'`) o `''` para el tema;
  `donde` es el texto a mostrar («¿Qué formatos admite?»). Puntos: título del tema 3, título de pregunta, error o
  código 2, cuerpo 1; orden por puntos y después por el orden de `temas`. Máximo 30 resultados.

- [ ] **Step 1: Pruebas**

```ts
test('sin tildes ni mayúsculas', () => assert.ok(buscar('CONVERSION', temas, es, te).some((r) => r.tema === 'archivos')))
test('el título pesa más', () => assert.equal(buscar('archivos', temas, es, te)[0].tema, 'archivos'))
test('por código de error', () => assert.ok(buscar('cola_llena', temas, es, te).some((r) => r.ancla === 'e-cola_llena')))
test('vacía o solo espacios no devuelve nada', () => assert.deepEqual(buscar('   ', temas, es, te), []))
test('caracteres de regex no rompen', () => assert.doesNotThrow(() => buscar('(*?', temas, es, te)))
test('en inglés también', () => assert.ok(buscar('files', temas, en, te).some((r) => r.tema === 'archivos')))
```

- [ ] **Step 2:** `npm test` → FAIL. **Step 3:** implementar (búsqueda con `includes` sobre `normalizar`, sin
  expresiones regulares). **Step 4:** `npm test` → PASS. **Step 5:** Commit `feat(ayuda): búsqueda en todos los temas`.

### Task 5: La Ayuda en la app (inicio, tema y dirección propia)

**Files:**
- Create: `web/src/ayuda/ruta.ts`, `web/src/ayuda/ruta.test.ts`, `web/src/ayuda/vista.ts`, `web/src/ayuda/vista.test.ts`,
  `web/src/componentes/ayuda/{BloqueDiagrama,BloquePasos,BloqueCaptura,BloqueEjemplo,BloqueTabla,BloquePreguntas,BloqueErrores,BloqueNota,VistaTema,TarjetasTemas}.vue`
- Modify: `web/src/paginas/Ayuda.vue` (se reescribe), `web/src/App.vue` (`desdeHash`, `ir`, `alNavegar`),
  `web/src/estilos.css` (estilos de los bloques; el recorrido de `Ayuda.vue:292-326` pasa a `BloqueDiagrama`)

**Interfaces:**
- Produces (`ruta.ts`): `export function leerHash(hash: string): { pagina: string; tema?: string }` —
  `'#ayuda/archivos'` → `{pagina:'ayuda', tema:'archivos'}`; `'#ayuda/noexiste'` y `'#ayuda/'` →
  `{pagina:'ayuda'}`; `'#panel'` → `{pagina:'inicio'}` (las `antiguas`); otro desconocido → `{pagina:'inicio'}`.
  `export function hashDe(pagina: string, tema?: string): string`. `App.vue` usa ambas y pasa `tema` a `Ayuda.vue`
  (prop `tema?: string`; el evento `ir` acepta `'ayuda/<id>'`). Volver funciona entre temas porque cada tema es una
  entrada del historial.
- Produces (`vista.ts`):
  `export function varianteCaptura(id: string, oscuro: boolean, idioma: 'es'|'en'): string` → `'<id>.oscuro.en.webp'`;
  `export function altDiagrama(paradas: string[]): string` → `'1. A → 2. B → 3. C'`;
  `export function bloquesVisibles(tema: Tema, contexto: 'app'|'panel'|'sitio', paginaActual?: Pagina): Bloque[]`
  (`descargas` solo en sitio; `ir` fuera del sitio y, en el panel, fuera si apunta a la página actual).
- `BloqueCaptura` resuelve el archivo con `import.meta.glob('../../ayuda/capturas/*.webp', { eager: true, query: '?url', import: 'default' })`
  y `varianteCaptura` (con `Dark.isActive` e `idioma`). Si el archivo aún no existe (antes de la tarea 8), no dibuja nada.
- `BloqueEjemplo` usa `ejemplos()` de `ejemplos.ts` como hoy `Ayuda.vue:220-223`.

- [ ] **Step 1: Pruebas** de `leerHash`, `hashDe` (ida y vuelta para los 14 temas), `varianteCaptura`, `altDiagrama`
  y `bloquesVisibles` (los tres contextos), incluidos los casos 1 y 5 de Review Focus.
- [ ] **Step 2:** `npm test` → FAIL. **Step 3:** implementar `ruta.ts`, `vista.ts` y los componentes; `Ayuda.vue`
  muestra `TarjetasTemas` + buscador (resultados con `aria-live="polite"`, cada uno navega a `ayuda/<tema>` y hace
  scroll a `ancla`) o `VistaTema` si hay `tema`; pie con guía del desarrollador y documentación como hoy; `VistaTema`
  termina con relacionados y «Abrir en la web» (`https://mscottb.github.io/Tramtai/ayuda/<id>.html`).
- [ ] **Step 4:** `npm test && npm run build` → PASS.
- [ ] **Step 5:** Commit `feat(ayuda): inicio por temas, vista de tema y #ayuda/<tema>`.

### Task 6: El ? de cada página, «¿Qué hago?» y «Saber más»

**Files:**
- Create: `web/src/ayuda/panel.ts`, `web/src/ayuda/panel.test.ts`, `web/src/componentes/ayuda/PanelAyuda.vue`
- Modify: `web/src/componentes/CabeceraPagina.vue` (prop `tema?: Pagina`), las 6 páginas principales (pasan su
  `tema`), `web/src/App.vue` (monta `PanelAyuda` una vez), los lugares que muestran un error con código (el
  implementador los localiza con `grep -rn "tituloError" web/src`), y los cuatro «Saber más»
  (`AvisoConversor.vue` → `archivos`, `CamposEsquema.vue` → `json`, `EditorReceta.vue` → `recetas`,
  `EditorPresupuesto.vue` → `presupuestos`)

**Interfaces:**
- Produces (`panel.ts`): `export const panel = reactive({ abierto: false, tema: '', ancla: '' })`,
  `export function abrirAyuda(tema: string, ancla?: string): void`, `export function cerrarAyuda(): void`. Abrir con un
  tema desconocido no abre.
- `PanelAyuda.vue`: `q-drawer side="right" overlay` (ancho 420 px; a pantalla completa bajo 600 px) con `VistaTema` en
  contexto `'panel'`, botón X y «Abrir en la Ayuda» (navega a `ayuda/<tema>` y cierra). Esc cierra. Al abrir, foco en
  el título del panel; al cerrar, vuelve al elemento que lo abrió (`document.activeElement` guardado en `abrirAyuda`).
- `CabeceraPagina`: botón `q-btn flat round icon="o_help_outline"` junto al título, `aria-label` `t('ayuda.abrirPanel')`
  («Ayuda sobre esta página» / «Help for this page»).
- «¿Qué hago?»: `abrirAyuda('problemas', 'e-' + codigo)`. «Saber más»: `abrirAyuda('<tema>')`.

- [ ] **Step 1: Pruebas** de `panel.ts`: abrir y cerrar, tema desconocido no abre, el ancla se guarda, abrir otro tema
  con el panel abierto lo cambia.
- [ ] **Step 2:** `npm test` → FAIL. **Step 3:** implementar. **Step 4:** `npm test && npm run build` → PASS.
- [ ] **Step 5:** Commit `feat(ayuda): botón ? en cada página y ayuda junto a los errores`.

### Task 7: La Ayuda del sitio, generada

**Files:**
- Create: `web/scripts/sitio-ayuda.mjs`, `web/scripts/sitio.ts` (funciones puras del generador), `web/scripts/sitio.test.ts`,
  `site/ayuda/index.html`, `site/ayuda/<id>.html` (14), `site/ayuda/indice.json` (generados)
- Modify: `site/ayuda.html` (página de paso, generada), `site/estilo.css` (bloques), `site/sitio.js` (buscador,
  pestañas, copiar), `site/index.html` y `site/desarrolladores.html` (enlaces a `ayuda/`), `web/package.json`
  (`"sitio": "node --import ./pruebas.mjs scripts/sitio-ayuda.mjs"` y `test` también sobre `scripts/**/*.test.ts`),
  `.github/workflows/ci.yml` (job `web`)

**Interfaces:**
- Consumes: `temas`, `bloquesVisibles(..., 'sitio')`, `altDiagrama`, `normalizar`, `ejemplos()`, textos es/en.
- Decisión: el generador vive en `web/scripts/` (la spec dice `scripts/` en la raíz) porque usa las fuentes y el
  gancho `pruebas.mjs` de la Web UI; se corre con `npm run sitio`.
- Produces (`sitio.ts`): `escapar(s)`, `bilingue(es, en)` → `<span lang="es">…</span><span lang="en">…</span>` (ambos
  escapados), `paginaTema(tema, textos)`, `paginaInicio(temas, textos)`, `indiceBusqueda(temas, textos)` (por idioma:
  tema, ancla, título y texto pasado por `normalizar`), `paginaDePaso()`, `destinoAncla(hash: string): string` y
  `export const ANCLAS_VIEJAS: Record<string, string>` (`instalar`→`empezar.html`, `agentes`→`agentes.html`,
  `terminos`→`agentes.html#terminos`, `aplicaciones`→`conectar.html`, `herramientas`→`herramientas.html`,
  `apis`→`conectar.html`, `recetas`→`recetas.html`, `archivos`→`archivos.html`, `actualizaciones`→`ajustes.html`,
  `preguntas`→`preguntas.html`, `mas`→`index.html`). `paginaDePaso()` incrusta `destinoAncla` y `ANCLAS_VIEJAS`.
- Las páginas usan la cabecera, la barra y el pie de `site/desarrolladores.html` (mismo `<head>`, script de tema
  inicial, selector de idioma y tema). Capturas como `<picture>` con `<source media="(prefers-color-scheme: dark)">`
  y `data-alt-en`. Ejemplos: `<div class="pestanas">` con un `<pre>` por lenguaje (sin JS se ven todos).
  `descargas` reusa el marcado de `site/index.html#descargar`. Cada tema lleva migas («Ayuda › Archivos»), sus
  relacionados al final y, en `conectar`, `herramientas` y `archivos`, un enlace a `../desarrolladores.html`. El
  generador copia `web/src/ayuda/capturas/` a
  `site/ayuda/capturas/` (las que existan).
- CI (job `web`, después de `npm run build`): `npm run sitio` y
  `git diff --exit-code --stat -- ../site/ayuda ../site/ayuda.html || { echo "::error::genera la Ayuda del sitio (cd web; npm run sitio) y súbela"; exit 1; }`.

- [ ] **Step 1: Pruebas** (`sitio.test.ts`): `escapar('<a & "b">')` → `'&lt;a &amp; &quot;b&quot;&gt;'`; una página de
  tema con un texto que contiene `<script>` no lo deja sin escapar; cada `href` interno de cada página generada
  apunta a un archivo que se genera; cada valor de `ANCLAS_VIEJAS` existe; `destinoAncla('#noexiste')` y
  `destinoAncla('')` → `'ayuda/'`; `indiceBusqueda` tiene los 14 temas en los dos idiomas.
- [ ] **Step 2:** `npm test` → FAIL. **Step 3:** implementar y correr `npm run sitio`.
- [ ] **Step 4:** `npm test` → PASS; abrir `site/ayuda/index.html` en el navegador con y sin JavaScript.
- [ ] **Step 5:** Commit `feat(sitio): Ayuda generada desde los temas de la app`.

### Task 8: Capturas

**Files:**
- Create: `web/scripts/capturas.mjs`, `web/scripts/capturas.test.ts`, `web/src/ayuda/capturas/*.webp` (generadas)
- Modify: `web/package.json` (`"capturas": "node scripts/capturas.mjs"`), `web/src/ayuda/estilo.test.ts`,
  `site/capturas/*` (portada, regeneradas con el mismo script), `site/index.html` si cambia algún nombre

**Interfaces:**
- `capturas.mjs`:
  1. Compila (`go build -o <tmp>/tramtai[.exe] ./cmd/tramtai`), crea `<tmp>/portable.txt` y una configuración con
     `servicio.puerto` libre (no 17373, para no chocar con la instalación del usuario), y arranca
     `tramtai servicio iniciar` en primer plano como proceso hijo.
  2. Entra a la Web UI con el mismo mecanismo que `abrirInterfaz` (`cmd/tramtai/abrir.go`). Si ese mecanismo solo abre
     el navegador del sistema, se añade a `tramtai abrir` la opción oculta `--solo-url` que imprime la dirección sin
     abrir nada (con su prueba en Go).
  3. Carga datos de ejemplo por la API: dos APIs («Asistente de soporte», «Resumen de notas»), una receta, un
     presupuesto y tres peticiones con el primer agente disponible en el equipo (si no hay ninguno, el script se para
     y dice que hace falta uno: se corre en el equipo de desarrollo, no en el CI).
  4. Lanza Edge o Chrome sin ventana (`--headless=new --remote-debugging-port`) y por CDP (`WebSocket` de Node) recorre
     la lista exportada `CAPTURAS` (`{ id, pagina, selector, antes? }`), fija tema e idioma por `localStorage`
     (`tramtai.tema`, `tramtai.idioma`) y recarga, y usa `Page.captureScreenshot` con `format: 'webp'` y el `clip` del
     elemento.
  5. `--solo <id>` saca solo esa. Al terminar detiene el servicio y borra `<tmp>`.
- `estilo.test.ts` suma: cada captura citada existe en sus 4 variantes; no hay `.webp` sin citar; suma ≤ 3 MB.
- `capturas.test.ts`: los ids de `CAPTURAS` son exactamente los citados por los temas más los de la portada
  (`inicio`, `agentes`, `mis-apis`, `probar`).

- [ ] **Step 1:** Pruebas → FAIL (no hay capturas). **Step 2:** escribir el script y correrlo (`npm run capturas`).
- [ ] **Step 3:** `npm test`, `npm run build`, `npm run sitio` → PASS; las capturas se ven en la app y en el sitio.
- [ ] **Step 4:** Commit `feat(ayuda): capturas con script, claras y oscuras, en dos idiomas`.

### Task 9: Documentación, regla en el repo y revisión en el navegador

**Files:**
- Modify: `docs/equipo/desarrollo.md` (sección «Ayuda y sitio en cada función»), `docs/equipo/nuevo-agente.md:88`,
  `docs/arquitectura.md`, `README.md` (enlace `ayuda/`), `docs/equipo/pendientes.md` (v0.52 ✅),
  `internal/recursos/web` (compilada)

- [ ] **Step 1:** `desarrollo.md`: cómo se agrega un tema o un bloque, qué controla `estilo.test.ts`, `npm run sitio`,
  `npm run capturas [-- --solo <id>]`, y la regla: ninguna función se da por terminada sin su tema de Ayuda, su página
  del sitio al día y código con buenas prácticas.
- [ ] **Step 2:** Resto de documentos según la spec, sección 5.
- [ ] **Step 3:** Verificación completa: `cd web && npm test && npm run build && npm run sitio`; `go vet ./...` y
  `go test ./...` desde la raíz; `git status` sin diferencias en `internal/recursos/web` ni `site/ayuda`.
- [ ] **Step 4:** Revisión en el navegador (app con `preview` y `site/ayuda/`): escritorio y 375 px, claro y oscuro,
  español e inglés, el **?** en las 6 páginas, «¿Qué hago?» en un error, Volver entre temas, sitio sin JavaScript, y la
  tabla «Qué pasa con cada texto actual» de la spec comparada fila por fila con los temas.
- [ ] **Step 5:** Commit `docs: Ayuda renovada y la regla de Ayuda y sitio en cada función`.
