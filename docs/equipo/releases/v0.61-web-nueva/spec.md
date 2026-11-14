# v0.61: la web nueva

Rediseño a fondo de todo el sitio (`site/`): el estilo, la portada, páginas nuevas y la documentación. Es un release
**solo del sitio**: no cambia la app. Decidido con el usuario el 2026-11-13 (brainstorming con bocetos en el navegador).

## Por qué

Lo que el usuario ve mal en el sitio publicado de la v0.60:
- **El arranque es plano:** un titular centrado sobre fondo oscuro, sin nada que enganche.
- **Los gráficos se ven toscos:** el diagrama en forma de línea de metro (Tu app → Tramtai → agentes) y los esquemas.
- **No se ve el producto** en el primer vistazo.
- **El estilo general está anticuado:** tipografía, colores, tarjetas, densidad.
- **Es demasiado «landing»:** una sola página larga, con 10 secciones parecidas entre sí. El usuario quiere un sitio de
  producto con páginas propias.

El público que la portada tiene que convencer primero es **quien programa**: alguien que ya usa el SDK de OpenAI o de
Anthropic y quiere usar su suscripción de Claude o Codex.

**Éxito:** en los primeros segundos, quien programa entiende qué es Tramtai, ve el producto funcionando y quiere
descargarlo. El sitio se ve al nivel de las herramientas actuales para desarrolladores.

## Referencias

Se miraron diez sitios a 1440×900 (informe en `.superpowers/sdd/v0.61-plan/referencias-web-v061.md` del checkout
principal):
- Ollama, LM Studio, OpenRouter, Warp, Zed, Raycast, Tailscale, Supabase, Bun y Linear;
- además, en breve, AI SDK de Vercel y Cursor.

Se presentaron tres direcciones:
- **A · Consola viva:** Warp, Bun.
- **B · El producto delante:** LM Studio, Linear, OpenRouter.
- **C · Plano técnico:** Zed, Supabase, Tailscale.

El usuario eligió **la estructura de C, los colores de B y las animaciones vivas de A**.

## 1. Sistema visual

Común a todas las páginas; sustituye al de `docs/diseno/DESIGN.md`, que se reescribe.

- **Colores (los de B):**
  - Tema oscuro por defecto: fondo casi negro, superficies un punto más claras, texto blanco roto, gris suave para lo
    secundario.
  - Un único acento verde azulado, el `#2dd4bf` de Tramtai.
  - Los colores de cada agente (los de hoy) solo en sus puntos y líneas de estado.
  - Tema claro con el mismo cuidado: papel cálido, acento más oscuro para que se lea, contraste AA.
  - Se mantienen la elección del sistema y el selector Claro / Oscuro de hoy.
- **Letra:**
  - Texto y títulos: **Inter** variable, con pesos medios y letras algo juntas en los titulares.
  - La parte clave de cada titular en **Instrument Serif** cursiva, del color de acento («Tus agentes de IA, *como una
    API local.*»).
  - Código, comandos, rótulos y terminal: Cascadia, como hoy (`Tramtai Mono`).
  - Las tres van alojadas en el sitio (OFL, con su licencia al lado), recortadas a los caracteres de es/en. Overpass se
    quita.
  - Tope: las letras de la portada no pasan de 150 KB en total.
- **Fondo y tarjetas (la estructura de C):**
  - La cuadrícula de plano, muy tenue y solo en la parte de arriba de cada página, con el brillo del acento detrás del
    titular.
  - Tarjetas con borde fino discontinuo, esquinas de 10 px y sin sombra en oscuro.
  - Rótulos **«FIG n · …»** en mayúsculas pequeñas monoespaciadas sobre cada figura, diagrama o recorte.
- **Movimiento (lo vivo de A):**
  - La terminal se escribe sola.
  - Un flujo corre por las líneas de los diagramas.
  - Los estados de los agentes laten.
  - Los diagramas se dibujan al aparecer en pantalla.
  - Nunca se mueve el texto que se está leyendo.
- **Accesibilidad:**
  - Contraste AA en los dos temas.
  - Foco visible.
  - Controles de al menos 44 px (salvo ES/EN y tema con ratón en escritorio, como hoy).
  - Con «reducir movimiento» del sistema, todo queda quieto y en su estado final.
  - Sin JavaScript, cada página se ve completa, con las animaciones en su estado final.

## 2. Mapa del sitio

Siete páginas con la misma barra arriba:

| Página | Archivo | Qué lleva |
|---|---|---|
| Inicio | `index.html` | Arranque, mosaico «FIG», la ventana de cuatro formas y «sigue por aquí» |
| Producto | `producto.html` (nueva) | Cómo funciona, funciones, por qué Tramtai, comparativa, privacidad y riesgos |
| Agentes | `agentes.html` (nueva) | La tarjeta «router», cada agente y cómo se prepara |
| Servidor | `servidor.html` | El modo servidor (documentación) |
| Desarrolladores | `desarrolladores.html` | La guía con ejemplos en Python, JavaScript, Go y curl (documentación) |
| Ayuda | `ayuda/` | Los 20 temas y el buscador (documentación) |
| Descargar | `descargar.html` (nueva) | Sistema detectado, todos los paquetes, requisitos y novedades de la versión |

Además se mantienen `404.html`, `ayuda.html` (que lleva a `ayuda/`), `sitemap.xml` (con las páginas nuevas), el
manifiesto, los iconos y `og.png`, que se rehace con el estilo nuevo.

**El contenido de hoy no se pierde:** cada sección actual de la portada pasa a una página (tabla en «Direcciones
estables»). Los textos se acortan donde se repiten, pero no se quita información. La Ayuda mantiene sus temas y sus
textos (vienen de `web/src/idiomas/ayuda.*.ts`, compartidos con la app).

## 3. La barra, el pie y el buscador

- **Barra arriba**, igual en todas las páginas:
  - el logo, que lleva a Inicio;
  - Producto · Agentes · Servidor · Desarrolladores · Ayuda · Descargar;
  - el buscador;
  - ES/EN, el tema y GitHub;
  - el botón «Descargar».

  La página actual se marca. En el móvil, el menú se abre con un botón.
- **Buscador (Ctrl K o /):**
  - Busca en los títulos y apartados de Ayuda, Desarrolladores y Servidor, en el idioma elegido.
  - El índice lo genera `sitio.ts` al construir el sitio (`buscar.json`, pequeño). Busca en el navegador, sin servicios
    externos.
  - Se usa con teclado (flechas y Enter) y cierra con Esc.
  - Sustituye al buscador propio de la Ayuda de hoy.
- **Pie:** las páginas, la Ayuda, el repositorio, la licencia y la versión publicada, como hoy.

## 4. Inicio

Corta, con un solo propósito: enseñar qué es y llevar a cada página. Cuatro bloques.

1. **Arranque:**
   - El titular «Tus agentes de IA, *como una API local.*», con la segunda parte en serifa cursiva y en verde.
   - Una frase: qué agentes, compatible con OpenAI y Anthropic, en tu equipo, solo cambias la URL y la clave.
   - «Descargar para <tu sistema> ▾» (con el sistema detectado y el resto en el menú) y «Cómo funciona», que lleva a
     Producto.
   - Cómo se instala en el sistema detectado, con «copiar» si es un comando:
     - Windows: el instalador (Tramtai aún no está en el catálogo de winget: `winget show MscottB.Tramtai` no lo
       encuentra; si entra, se añade el comando);
     - macOS: `brew tap mscottb/tramtai https://github.com/MscottB/Tramtai` y `brew install --cask tramtai`, los de hoy;
     - Linux: el `.deb` o el `.rpm` de Descargar.
   - Detrás, el brillo y la cuadrícula.
2. **Mosaico «FIG»:**
   - **FIG 1, grande:** la terminal viva. El cambio de dos líneas (`- base_url` del proveedor y `+` la de Tramtai), luego
     la orden que arranca el servicio (`tramtai servicio iniciar`, en inglés `tramtai service start`; `tramtai serve` no
     existe) y las peticiones que llegan con su agente, modelo y tiempo.
   - **FIG 2:** el camino tu app → Tramtai → agentes, en líneas en perspectiva con el flujo corriendo.
   - **FIG 3:** el estado de los agentes, con puntos que laten (listo, versión nueva, sin instalar).
   - **FIG 4 y 5:** dos recortes reales de la app, Mis APIs y el mapa del servidor.
   - Los datos de FIG 1 y 3 son de ejemplo, fijos y verosímiles: la página no llama a nada.
3. **Una ventana, cuatro formas de usarlo:**
   - Una ventana grande con pestañas **Web UI · Terminal · CLI · Servidor**.
   - Lleva capturas reales de la app en el idioma y el tema elegidos, y la terminal como texto real, como hace hoy la
     sección «terminal».
   - Las pestañas funcionan con teclado y sin JavaScript se ven las cuatro una debajo de otra.
4. **Sigue por aquí:** una tarjeta por página (Producto, Agentes, Servidor, Desarrolladores, Ayuda, Descargar), con una
   línea cada una.

## 5. Producto, Agentes y Descargar

Páginas de presentación, con el arranque corto, la cuadrícula arriba y el mosaico.

- **Producto:**
  - **Cómo funciona, en tres pasos:** instala Tramtai → prepara tus agentes (instala e inicia sesión desde la app) →
    cambia la URL y la clave. Un diagrama en perspectiva que se dibuja paso a paso al bajar.
  - **Funciones:** tarjetas de distinto tamaño, cada una con un recorte real. Sale de la sección «funciones» de hoy.
  - **Por qué Tramtai:** una franja de cifras sobrias y las siete ventajas en un mosaico (salen de «por-que» y de la
    línea de ventajas). La comparativa de hoy, plegada debajo.
  - **Privacidad y riesgos:** el contenido de la sección «privacidad» de hoy, completo.
- **Agentes:**
  - **La tarjeta «router»** sustituye a la línea de metro. De Tramtai salen líneas de un píxel a Claude Code, Codex,
    GitHub Copilot, Gemini API y Antigravity. Cada uno lleva su color, lo que hace, cómo se conecta (suscripción o clave)
    y un pulso animado.
  - **Una tarjeta por agente:** cómo se instala e inicia sesión desde Tramtai, las versiones comprobadas y su enlace a la
    Ayuda «Preparar tus agentes». Sale de la sección «agentes» de hoy.
- **Descargar:**
  - **El sistema detectado delante,** con pestañas Windows · macOS (Homebrew o `.dmg`) · Linux (`.deb`, `.rpm` y CLI) y
    «copiar» en los comandos.
  - **Debajo:** todas las descargas del release (instalador, portable, `.deb`, `.rpm`, `.dmg`, CLI y servidor), los
    requisitos y las novedades de la versión publicada. Sale de la sección «descargar» de hoy.

## 6. Servidor, Desarrolladores y Ayuda: documentación

Una plantilla de tres columnas, común:
- **Izquierda:** el índice de la página o de la Ayuda, con grupos.
- **En medio:** el contenido.
- **Derecha:** «En esta página», con los apartados y el actual marcado al bajar.

En el móvil se ve una sola columna y el índice se abre con un botón.

Cada página empieza con su titular en dos partes (la cursiva en verde) y usa:
- los rótulos «FIG» en figuras y ejemplos;
- los bloques de código con «copiar»;
- diagramas vivos donde explican algo: en Servidor, el mapa de hoy con el estilo nuevo; en Desarrolladores, tu app →
  Tramtai → agente.

El contenido de Servidor y Desarrolladores se mantiene; cambian la forma y el orden donde haga falta para la plantilla.

## 7. Direcciones estables

Las apps instaladas enlazan al sitio:
- `servidor.html` (Ajustes, terminal);
- `desarrolladores.html` (Ayuda, Mis APIs);
- `ayuda/<tema>.html` (Ayuda de la app y de la terminal);
- la portada (Acerca de).

El README enlaza a `#descargar`. **Esas direcciones no cambian.**

GitHub Pages no redirige, así que los anclajes viejos de la portada (`#descargar`, `#como`, `#agentes`, `#funciones`,
`#por-que`, `#privacidad`, `#servidor`, `#app`, `#terminal`) llevan a su sitio nuevo con un salto pequeño en
`index.html`. Sin JavaScript, un enlace visible en Inicio hace lo mismo.

| Anclaje de hoy | Va a |
|---|---|
| `#descargar` | `descargar.html` |
| `#como`, `#funciones`, `#por-que`, `#privacidad` | `producto.html#…` |
| `#agentes` | `agentes.html` |
| `#app`, `#terminal` | la ventana de Inicio (`#formas`) |
| `#servidor` | `servidor.html` |

## 8. Cómo se construye

- **Generación:**
  - Sigue siendo un sitio estático en GitHub Pages, sin dependencias nuevas.
  - `web/scripts/sitio.ts` pinta la barra, el pie y el buscador en todas las páginas. Además genera la Ayuda con la
    plantilla de documentación, `buscar.json` y `sitemap.xml`.
  - Inicio, Producto, Agentes y Descargar se escriben a mano en `site/`, como hoy la portada; Servidor y Desarrolladores,
    también, sobre la plantilla.
- **Estilo:** `site/estilo.css` se reescribe con el sistema nuevo (variables con `light-dark()`, como hoy).
- **JavaScript:** `site/sitio.js` sigue siendo propio, sin librerías. Hace:
  - el tema y el idioma;
  - el buscador;
  - las pestañas;
  - el «copiar»;
  - el sistema detectado;
  - los anclajes viejos;
  - lo que aparece al bajar, con `IntersectionObserver`.
- **Bilingüe:** las dos lenguas en la misma página, como hoy (`lang` y el selector ES/EN); las capturas y los comandos
  salen en el idioma elegido.
- **Capturas:**
  - Las de la Web UI salen del proceso de siempre (`npm run capturas`, `capturas-lista.ts`), en es/en × claro/oscuro.
  - Se añaden los recortes de FIG 4 y 5 y de las tarjetas de funciones.
  - Las capturas completas hacen unas pocas peticiones reales a claude/haiku con la suscripción del usuario para el
    Historial de ejemplo, así que solo se rehacen las que hagan falta, con `--solo`.
  - Nunca con el Tramtai instalado del usuario ni su puerto 17373.
- **Diagramas:** SVG dentro de la página, en un solo color de línea (el acento) y con los colores de los agentes solo en
  sus puntos; la animación, en CSS.

## 9. Pruebas

- Siguen las de hoy, ajustadas a las páginas nuevas:
  - `TestComandosDelSitioExisten` (cada orden citada existe, también en `ayuda/*.html`);
  - el estilo de los textos (`estilo-textos.test.ts`);
  - los textos de la portada (`portada.test.ts`) y del generador (`sitio.test.ts`).
- Nuevas:
  - Cada enlace interno y cada anclaje existe.
  - Cada página tiene la barra, el pie y los dos idiomas.
  - Los anclajes viejos llevan a su sitio.
  - `buscar.json` tiene una entrada por tema y apartado.
  - `sitemap.xml` tiene las siete páginas.
  - Las letras no pasan del tope.
- Antes de cerrar, una revisión de diseño y accesibilidad de cada página:
  - con impeccable, antislop y teclado;
  - a 375 y 1440 px, en claro y oscuro, en es y en;
  - con contraste AA, con «reducir movimiento» y sin JavaScript.

## 10. Publicación

- El sitio lo publica el workflow «Página» al terminar Publicar o a mano (`workflow_dispatch`). En esta versión, como no
  hay release de la app, se publica a mano tras fusionar en main y con el «sí» del usuario.
- Los commits que solo tocan `site/` no suben la versión de la app (`scripts/version.ps1` los excluye). Los que tocan
  `web/scripts/` (el generador y las pruebas) sí cuentan, y saldrán con la próxima versión de la app sin cambiar nada de
  ella. Ninguna tarea cambia la app ni la Ayuda dentro de la app.
- Se ponen al día:
  - `docs/diseno/DESIGN.md` con el sistema nuevo;
  - el README (es/en), con enlaces a las páginas nuevas;
  - la guía de desarrolladores, donde habla del sitio.

## Fuera del alcance

- Cambios en la app, la Web UI, la terminal o la CLI. La Web UI no cambia de estilo en esta versión.
- Contenido nuevo del producto: solo se reparte y se presenta mejor el que hay.
- Un generador de sitios (Astro u otro) o dependencias nuevas.
- Vídeos: lo vivo es CSS y SVG, no grabaciones.

## Riesgos

- **Que la portada pese más:**
  - las letras tienen un tope;
  - las capturas van en WebP con `loading="lazy"` salvo la primera;
  - el SVG va dentro de la página y no hay librerías.
- **Que las animaciones molesten o distraigan:**
  - solo se mueve lo que explica algo;
  - nada se repite deprisa;
  - «reducir movimiento» lo para todo.
- **Romper enlaces de las apps instaladas:** las direcciones de la sección 7 no cambian y una prueba lo comprueba.
- **Que la Ayuda del sitio y la de la app se separen:** los textos siguen viniendo del mismo `ayuda.*.ts`; solo cambia la
  plantilla del sitio.
