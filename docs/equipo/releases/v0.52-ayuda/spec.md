# Ayuda renovada (v0.52)

Diseño del 2026-10-20, acordado con el usuario por secciones. Hoy hay dos ayudas que dicen cosas distintas y se
mantienen por separado:

- **App:** `web/src/paginas/Ayuda.vue` (350 líneas), una sola página larga con ocho bloques (cómo funciona, archivos,
  APIs, primeros pasos, guías, conceptos, problemas y preguntas), buscador y unas 160 claves `ayuda.*` en
  `web/src/idiomas/es.ts` y `en.ts`.
- **Sitio:** `site/ayuda.html` (424 líneas), diez secciones de texto (instalar, agentes, conectar tu app, herramientas,
  APIs, recetas, archivos y JSON, actualizaciones, preguntas y más documentación) sin capturas; las cuatro capturas
  del sitio están solo en la portada.

Decisiones del usuario:
- Lo que la v0.52 resuelve es **todo**: mucho texto, cuesta encontrar y app y web distintas.
- **Una sola fuente**: los temas como **datos** con bloques de tipos fijos; la app los muestra con componentes Vue y el
  sitio se **genera** desde ellos en HTML estático.
- Botón **?** en cada página de la app que abre su tema en un panel lateral, dentro de esta release.
- **Diagramas como datos** (paradas con ícono, título y ramas), sin dependencias; son la base de los diagramas vivos de
  la v0.53.
- **Capturas con un script** del repo (claro/oscuro, es/en, recortes WebP) y control en el CI.
- **Regla permanente** (también en la memoria del proyecto): toda función nueva llega con su tema de Ayuda en este
  estilo, la página del sitio al día y código con buenas prácticas.

Éxito: quien abre la Ayuda llega en dos clics, o con una búsqueda, a la respuesta de lo que quiere hacer, la entiende
mirando el diagrama, los pasos y la captura sin leer párrafos, y encuentra lo mismo en la app y en la web. Quien
agrega una función no puede olvidar su ayuda: el CI lo detiene.

## 1. Estructura y datos

```
web/src/ayuda/
  tipos.ts          forma de un tema y de cada bloque
  temas/            un archivo por tema (sección 4)
  indice.ts         orden de los temas y página de la app de cada uno
  buscar.ts         búsqueda sobre todos los temas (app y sitio)
  capturas/         <id>.<claro|oscuro>.<es|en>.webp (sección 4)
  estilo.test.ts    la regla del estilo como prueba
  buscar.test.ts
web/src/idiomas/ayuda.es.ts, ayuda.en.ts   textos de la Ayuda (salen de es.ts/en.ts)
web/src/componentes/ayuda/                 un componente por tipo de bloque, la vista de tema y el panel
scripts/sitio-ayuda.mjs                    genera site/ayuda/
scripts/capturas.mjs                       saca las capturas
```

**Un tema** (`tipos.ts`):

```ts
interface Tema {
  id: string                 // 'archivos'; también el nombre de su página en el sitio
  icono: string              // ícono de Material (o_…)
  titulo: Clave
  resumen: Clave             // una línea
  paginaApp?: Pagina         // página de la app cuyo ? abre este tema
  relacionados: string[]     // ids de otros temas (2 o 3)
  bloques: Bloque[]
}
```

**Bloques** (tipos fijos; un tema usa solo los que le corresponden, en el orden en que los declara):

| Tipo | Contenido | En la app | En el sitio |
|---|---|---|---|
| `diagrama` | paradas `{icono, titulo, nota?}` y ramas `{desde, hacia, etiqueta}` | horizontal en ancho, vertical en el móvil | igual, HTML y CSS |
| `pasos` | lista de claves | numerados | numerados |
| `captura` | id de la captura y texto alternativo | según tema e idioma de la app | `<picture>` claro/oscuro |
| `ejemplo` | prueba para `ejemplos.ts` y lenguajes | pestañas y copiar | pestañas con JS; sin JS, uno debajo de otro |
| `tabla` | filas y columnas de claves | tabla | tabla |
| `preguntas` | pares pregunta/respuesta | plegadas | `<details>` |
| `errores` | códigos de error | título de la app y qué hacer | igual |
| `nota` | una clave | caja informativa | caja informativa |
| `ir` | página de la app | botón «Ir a …» | no se muestra |
| `descargas` | — | no se muestra | botones por sistema de `sitio.js` |

- **Textos:** se usa el sistema actual (`t()`, claves y la prueba que exige las mismas claves en español e inglés). Los
  de la Ayuda pasan a `idiomas/ayuda.es.ts` y `ayuda.en.ts`, que `i18n.ts` junta con los demás. `es.ts` y `en.ts` no
  siguen creciendo.
- **La tabla de las APIs** (`TablaApis.vue`, en Mis APIs y en la Ayuda) pasa a ser un bloque `tabla` del tema
  «Conectar mi app». `TablaApis.vue` dibuja ese mismo bloque, así los datos quedan en un solo lugar.
- **Los ejemplos de código** del bloque `ejemplo` salen de `ejemplos.ts`, como hoy en la sección de archivos.

**La regla del estilo** (`estilo.test.ts`) falla si:
- un tema no tiene `resumen` o el resumen pasa de 140 caracteres en algún idioma;
- un paso, una nota, una parada o una respuesta pasa de 200 caracteres;
- un tema de la sección 4 marcado con diagrama no lo tiene;
- una página principal de la app (`principales` en `App.vue`; la Ayuda no lleva ?) no tiene tema con su `paginaApp`;
- una captura citada no existe en sus cuatro variantes, hay archivos en `capturas/` que nadie cita o el total pasa de
  3 MB;
- un tema cita un relacionado o un error que no existe.

## 2. La Ayuda dentro de la app

**Inicio** (`#ayuda`): buscador arriba y, debajo, tarjetas por tema (ícono, título, resumen); una columna en el móvil.
Al escribir, las tarjetas se cambian por **resultados**: tema y bloque donde aparece («Archivos › ¿Qué formatos
admite?»), cada uno lleva a ese punto. Sin resultados, un mensaje con el texto buscado. Al pie, enlaces a la guía del
desarrollador y a la documentación (como hoy).

**Tema** (`#ayuda/<id>`): dirección propia; Volver funciona y otras partes de la app pueden enlazar un tema. La
navegación por `#` de `App.vue` se extiende: `#ayuda/<id>` abre la página Ayuda con ese tema. Arriba, título y
resumen; luego los bloques; al final «Temas relacionados» y «Abrir en la web» (`…/ayuda/<id>.html`).

**El ? de cada página:** `CabeceraPagina` recibe `tema` y muestra un botón **?** junto al título
(`aria-label` «Ayuda sobre esta página» / «Help for this page»). Abre un panel lateral derecho con el tema en vista
compacta (sin el bloque `ir` a la página actual, con «Abrir en la Ayuda»). En el móvil ocupa toda la pantalla. Se
cierra con Esc o la X; el foco entra al panel al abrirlo y vuelve al ? al cerrarlo. No navega: lo que hay en la página
no se pierde.

**«¿Qué hago?» en los errores:** donde la app muestra un error con su código (Probar, Historial y el registro de un
trabajo), un botón abre el panel con el tema «Algo falló» en ese error.

**Accesibilidad:** el diagrama tiene un texto alternativo armado con sus paradas en orden; los botones solo con ícono
llevan `aria-label` en los dos idiomas; los resultados de la búsqueda se anuncian (`aria-live`).

## 3. La Ayuda en el sitio

- **Páginas:** `site/ayuda/index.html` (inicio con las mismas tarjetas) y una por tema (`site/ayuda/<id>.html`).
- **`site/ayuda.html`** queda como página de paso: un script lleva las anclas viejas a su tema nuevo
  (`#terminos` → `ayuda/agentes.html#terminos`, `#aplicaciones` y `#apis` → `ayuda/conectar.html`, `#instalar` →
  `ayuda/empezar.html`, y así cada sección) y, sin JavaScript, `<meta http-equiv="refresh">` a `ayuda/` más un enlace.
  Así siguen sirviendo los enlaces de versiones ya instaladas de la app (`ayuda.html#terminos`) y el de
  `desarrolladores.html#aplicaciones`.
- **Generación:** `scripts/sitio-ayuda.mjs` (Node 24, el del CI, que lee TypeScript) carga temas, índice y textos y
  escribe cada página con la cabecera, la barra y el pie del sitio. Los dos idiomas van en la misma página con
  `<span lang>` y `data-alt-en`, como el resto del sitio, y siguen el selector de idioma y de tema de `sitio.js`. El
  CSS de los bloques va en `site/estilo.css`. También escribe `site/ayuda/indice.json` para el buscador y copia las
  capturas a `site/ayuda/capturas/`.
- **El HTML generado se guarda en el repo** (como la Web UI compilada). El CI lo regenera y falla si cambia,
  indicando `node scripts/sitio-ayuda.mjs`.
- **Sin JavaScript** se lee todo (diagramas, pasos, capturas, `<details>`); **con JavaScript** se suman el buscador
  (sobre `indice.json`, en el idioma elegido), las pestañas de los ejemplos y copiar.
- **Navegación:** la barra superior apunta a `ayuda/`; cada tema lleva migas («Ayuda › Archivos»), relacionados al
  final y, en Conectar mi app, Herramientas y Archivos, un enlace a la guía del desarrollador.
- **Empezar** en el sitio suma el bloque `descargas` (instalar en Windows, macOS y Linux).

## 4. Contenido y capturas

**Temas** (14), en este orden:

| id | Tema | `paginaApp` | Diagrama | Capturas |
|---|---|---|---|---|
| `empezar` | Empezar | `inicio` | cómo funciona: app → clave → cola → agente → respuesta | Inicio |
| `agentes` | Agentes y modelos | `agentes` | suscripción o clave → agente → modelo y esfuerzo | fila de un agente |
| `probar` | Probar | `ejecutar` | mensaje → agente → respuesta | selector de modelo; respuesta |
| `conectar` | Conectar mi app | `conectar` | app → URL y clave → Tramtai → agente | tarjeta de una API |
| `archivos` | Archivos | — | app → carga → conversión → agente → respuesta | aviso del conversor |
| `json` | Respuestas JSON | — | esquema → agente → validación → corrección | editor de campos |
| `recetas` | Recetas y respaldo | — | receta → agente 1 ✗ → agente 2 ✓ | editor de receta |
| `herramientas` | Herramientas | — | tu app ofrece → el agente pide → tu app ejecuta → respuesta | — |
| `presupuestos` | Presupuestos | — | consumo → aviso al 80 % → tope | barras de presupuesto |
| `historial` | Historial | `historial` | — | lista con filtros |
| `ajustes` | Ajustes y actualizaciones | `configuracion` | — | actualizaciones |
| `problemas` | Algo falló | — | — | — |
| `conceptos` | Conceptos | — | — | — |
| `preguntas` | Preguntas frecuentes | — | — | — |

Los temas sin `paginaApp` se abren desde la Ayuda, desde los relacionados o desde un «Saber más» donde aparecen en la
app: el aviso del conversor (`archivos`), el editor de esquema (`json`), el editor de receta (`recetas`) y el de
presupuesto (`presupuestos`).

**Qué pasa con cada texto actual:**

| Hoy | Va a |
|---|---|
| App, «Cómo funciona» (`f1`–`f5`, `funcionaNota`) | `empezar`: diagrama y nota |
| App, «El recorrido de un archivo» (`a1`–`a5`, notas, ejemplos) | `archivos`: diagrama, notas, ejemplo |
| App, «Las APIs» (`TablaApis`) | `conectar`: bloque `tabla` |
| App, primeros pasos (`paso1`–`paso5`) | `empezar`: pasos (`paso4` → `conectar`, `paso5` → `ajustes`) |
| Guías `g1` conectar, `g2` documentos, `g3` JSON, `g4` respaldo, `g5` gasto, `g6` por qué falló | pasos de `conectar`, `archivos`, `json`, `recetas`, `presupuestos`, `problemas` |
| Conceptos (12) | `conceptos` (cada uno con enlace a su tema); `p6` (espacio de trabajo) se funde con su concepto |
| Problemas (13 errores) | `problemas`, bloque `errores` |
| `p1` clave para entrar, `p9` en marcha/detener, `p24` portable | `empezar` |
| `p5` suscripción o clave, `p19` Antigravity en APIs, `p22` qué agentes, `p23` plan de Copilot | `agentes` |
| `p21` prueba como código | `probar` |
| `p11` prueba → API, `p13` clave filtrada, `p20` qué URL | `conectar` |
| `p18` rechazo por tamaño | `archivos` |
| `p4` responder en español, `p12` agente en su límite | `recetas` |
| `p25` herramientas | `herramientas` |
| `p14` limitar el gasto | `presupuestos` (se funde con la guía `g5`) |
| `p7` borrar datos de prueba, `p8` estados, `p10` ver lo que procesa, `p15` tiempos por paso | `historial` |
| `p3` dónde están mis datos, `p16` al actualizar, `p17` qué versión | `ajustes` |
| `p2` chats en los CLI | `preguntas` |
| Sitio, Instalar | `empezar`: bloque `descargas` y pasos |
| Sitio, Conectar los agentes (suscripción o clave) | `agentes` |
| Sitio, Conectar tu aplicación y Una API por aplicación | `conectar` |
| Sitio, Herramientas | `herramientas` |
| Sitio, Recetas y respaldo | `recetas` |
| Sitio, Archivos y respuestas en JSON | `archivos` y `json` |
| Sitio, Actualizaciones | `ajustes` |
| Sitio, preguntas (incluidas `#terminos`, «¿Puedo usarlo en mi empresa?» y la clave filtrada) | `agentes` (`#terminos`), `preguntas` (empresa y licencia), `conectar` (clave filtrada) |
| Sitio, Más documentación | pie del inicio de la Ayuda |

**Reescritura:** cada texto se acorta al estilo (resumen de una línea, pasos de 200 caracteres como máximo) y se quita
lo que el diagrama o la captura ya muestran y lo repetido entre secciones (por ejemplo, cómo se elige el modelo, que
hoy está en tres lugares). Todo en español e inglés.

**Capturas** (`scripts/capturas.mjs`):
- Arranca el binario compilado en una carpeta temporal en modo portable (`portable.txt`), sin tocar la instalación del
  usuario, y carga datos de ejemplo por la API con nombres neutros («Asistente de soporte», «Resumen de notas»).
- Se conecta a Edge o Chrome por el protocolo de depuración (CDP, con el `WebSocket` de Node), sin dependencias nuevas.
- Cada captura se declara en una lista del script: id, página, selector del elemento que se recorta y acción previa
  (por ejemplo, abrir el selector de modelo).
- Saca cada una en claro y oscuro y en español e inglés, en WebP, en `web/src/ayuda/capturas/`
  (`<id>.<claro|oscuro>.<es|en>.webp`).
- También saca las de la portada del sitio (`site/capturas/`).
- Se corre al cerrar cada release que cambie pantallas (`node scripts/capturas.mjs`, con `--solo <id>` para una).
- Cálculo: unas 15 capturas × 4 ≈ 60 archivos de ~40 KB, unos 2,5 MB, bajo el tope de 3 MB.

## 5. Pruebas, documentación y alcance

**Pruebas (CI):**
- `estilo.test.ts` (sección 1).
- `buscar.test.ts`: encuentra por título, paso, pregunta y código de error; sin distinguir mayúsculas ni tildes
  («conversion» encuentra «conversión»); el título pesa más que el cuerpo; búsqueda vacía no devuelve nada.
- Componentes: cada tipo de bloque se dibuja en los dos idiomas; el ? de una página abre su tema; el panel devuelve el
  foco al ? al cerrarse; `#ayuda/<id>` abre el tema y Volver vuelve; un id desconocido lleva al inicio de la Ayuda.
- Sitio: el CI corre `scripts/sitio-ayuda.mjs` y falla si cambia algo en `site/ayuda/`; comprueba que los enlaces
  internos de `site/ayuda/` existen y que cada ancla vieja de `ayuda.html` tiene destino.
- Revisión en el navegador antes del PR: app y sitio en escritorio y móvil, claro y oscuro, español e inglés; el sitio
  sin JavaScript.

**Documentación:**
- `docs/equipo/desarrollo.md`, sección «Ayuda y sitio en cada función»: cómo se agrega un tema o un bloque, qué
  controla `estilo.test.ts`, cómo se corren `capturas.mjs` y `sitio-ayuda.mjs`, y la regla: ninguna función se da por
  terminada sin su tema de Ayuda, su página del sitio al día y código con buenas prácticas.
- `docs/equipo/nuevo-agente.md`: «`site/ayuda.html`» pasa a «el tema `agentes`».
- `docs/arquitectura.md`: de dónde sale la Ayuda (datos → app y sitio).
- README y portada del sitio: enlaces a `ayuda/`; `docs/equipo/pendientes.md`: v0.52 hecha.
- Notas de la versión en español e inglés.

**Fuera de esta release:** diagramas vivos (v0.53), capturas sacadas por el CI, Ayuda en la terminal (v0.54, con
esta misma fuente), la guía del desarrollador (solo se enlaza).

**Riesgos:**
- El binario crece por las capturas: recortes WebP y tope de 3 MB en el CI.
- El HTML del sitio se desfasa de los temas: el CI lo regenera y compara.
- Se pierde contenido al reescribir: la tabla «Qué pasa con cada texto actual» se revisa al final contra lo hecho.
