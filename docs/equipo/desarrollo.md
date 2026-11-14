# Guía del equipo

Documentación interna para desarrollar Tramtai. La de usuario está en el [README](../../README.md) y en [docs/](../inicio.md).

| Documento | Contenido |
|---|---|
| [README.md](README.md) | Índice de esta carpeta y de la spec y el plan de cada release |
| [pendientes.md](pendientes.md) | Hitos y lista de tareas (incluye CI/CD) |
| [versionado.md](versionado.md) | Versión `vX.Y.Z+DDMMAAAA` calculada desde los commits, y etiquetas con notas |
| [M0B.md](M0B.md) | Validación de los modos con clave de API (pendiente con claves reales) |
| [Diseño](../diseno/DESIGN.md) | Sistema visual del sitio (plano técnico, v0.61) y de la Web UI («Red de tranvía»), y [contexto de producto](../diseno/PRODUCT.md) |

El documento de diseño original del proyecto no forma parte del repositorio.

## Compilar

```powershell
./scripts/compilar.ps1     # bin/tramtai.exe y bin/tramtai-escritorio.exe, con icono y versión en sus propiedades
go run ./scripts/icono     # regenera el icono (escritorio/icono/tramtai.png); compilar.ps1 genera el .ico y los .syso
```

`compilar.ps1` necesita `wails3` (en `%USERPROFILE%\go\bin`) para el icono y la versión de los `.exe`; sin él compila igual, sin esos recursos.

**Instalador.** `./scripts/compilar.ps1 -Instalador` deja además en `dist/` el instalador de Windows (NSIS, `scripts/instalador/tramtai.nsi`), el `.zip` portable y `SHA256SUMS.txt`. Necesita NSIS 3. Es por usuario y sin permisos de administrador; el PATH lo edita `tramtai instalacion agregar-ruta|quitar-ruta` (comando oculto), porque NSIS corta las cadenas a 1024 caracteres. Las imágenes del asistente se regeneran con `scripts/instalador/imagenes.ps1`.

## Web UI

```powershell
cd web
npm install
npm run dev      # servidor de desarrollo que llama al servicio en marcha (tramtai servicio iniciar -d)
npm run build    # comprueba los tipos (vue-tsc, también los .vue) y compila en internal/recursos/web, que se incorpora con go:embed
```

La compilación de la Web UI se versiona para que `go build` no necesite Node.

**Entorno.** Se trabaja con Node 26, la misma versión que el CI, que toma siempre su último parche (`check-latest`). En
Linux, el CI guarda en caché los paquetes de GTK 4 y WebKitGTK 6.0 por arquitectura (`.github/actions/gtk`), con
reintentos de apt, y cada job tiene un tope de tiempo.

**Tipos de la API.** Los tipos TypeScript de lo que el servicio envía y recibe (`web/src/api/tipos.gen.ts`) se
generan desde los structs de Go con `go generate ./internal/contrato` (`internal/contrato/gents`); `web/src/api/tipos.ts`
los reexporta y solo escribe a mano los de las rutas `/ui` de la sesión. Si cambias un tipo del contrato, regenera y sube
el archivo: una prueba de Go y el CI fallan si no está al día. Lo que no se deduce de Go (una unión de literales) se dice
con la etiqueta `ts:"…"` del campo. Lo derivado (el estado de un agente, la etapa y si se puede corregir una petición,
los Primeros pasos) lo calcula el servicio y la Web UI y la terminal solo lo pintan
([ADR 001](decisiones/001-estado-del-agente.md)).

**Dependencias.** `web/.npmrc` protege las instalaciones:

- `min-release-age=7`: npm no instala versiones publicadas hace menos de 7 días, la ventana en que suelen detectarse y retirarse los paquetes comprometidos. Para una corrección urgente: `npm install --min-release-age=0 …`.
- `ignore-scripts=true`: ningún paquete ejecuta scripts al instalarse.
- `save-exact=true`: las versiones quedan fijas, sin `^` ni `~`.

El CI falla si una dependencia de la Web UI tiene una vulnerabilidad alta o crítica conocida (`npm audit --audit-level=high`) o si su firma del registro no se verifica (`npm audit signatures`). Las de Go se revisan con `govulncheck`, que solo avisa de las vulnerabilidades que el código llega a usar.

**Idiomas.** Los textos están en `web/src/idiomas/`: `es.ts` es el principal y define las claves; `en.ts` tiene el tipo `Textos`, así que `npm run tipos` falla si le falta alguna. Para añadir un idioma, crea `idiomas/<código>.ts` con el mismo tipo y añádelo a `idiomas` en `web/src/i18n.ts` junto al paquete de Quasar (`quasar/lang/<código>`). La preferencia se recuerda en el navegador; sin ella se usa la del navegador y, si no está disponible, español. 

La CLI, la terminal interactiva, la app de escritorio y los mensajes de error del servicio usan `internal/idioma`: el texto se escribe en español dentro de `idioma.T("…")` o `idioma.Tf("… %s", x)` (siempre un literal) y su inglés va en `internal/idioma/en_*.go`. `TestCobertura` falla si un texto marcado no tiene traducción, si cambian los verbos de `fmt`, si sobra una traducción o si un mismo texto tiene dos. No se traducen los nombres de comandos y opciones, las claves de configuración, los códigos de error, los registros del servicio ni lo que se envía a los agentes. El idioma sale de `TRAMTAI_IDIOMA` (`es` o `en`) o del sistema; las pruebas comparan textos en español, así que el CI las ejecuta con `TRAMTAI_IDIOMA=es`.

## Ayuda y sitio en cada función

**Regla:** ninguna función se da por terminada sin su tema de Ayuda (en la app y en la web, en el estilo de abajo), la
página del sitio al día y el código con buenas prácticas (archivos acotados, nombres claros, pruebas, sin duplicar).
Va en la spec y el plan de cada release, no para después.

**Regla de la terminal:** cada función nueva llega también a la terminal (`tramtai terminal`), con los mismos nombres y
textos que la Web UI, su tecla en el pie y su línea «En la terminal: …» en el tema de Ayuda. Si algo no cabe en texto
(una captura, un diagrama), la terminal lleva a la web; nunca se queda sin decir cómo se hace. Cómo añadir una página
está en [Terminal: cómo añadir una página](#terminal-cómo-añadir-una-página).

La Ayuda es **una sola fuente** en `web/src/ayuda/`: cada tema (`temas/<id>.ts`) es una lista de bloques de tipos
fijos (`diagrama`, `pasos`, `captura`, `ejemplo`, `tabla`, `preguntas`, `errores`, `nota`, `ir`, `descargas`, en
`tipos.ts`) y sus textos van por clave en `idiomas/ayuda.es.ts` y `ayuda.en.ts`. La app los dibuja con
`componentes/ayuda/` (inicio por temas, `#ayuda/<tema>` y el panel del botón **?** de cada página) y el sitio se genera
desde los mismos datos.

- **Añadir un tema:** su archivo en `temas/`, su lugar en `indice.ts` (y `paginaApp` si es el de una página) y sus
  textos en los dos idiomas, también su línea `terminal` («En la terminal: …», con sus teclas). **Añadir un tipo de
  bloque:** en `tipos.ts`, su componente `Bloque<Tipo>.vue`, su HTML en `web/scripts/sitio.ts`, su forma para la
  terminal en `web/scripts/terminal-datos.ts` (`bloqueDe`) y en `internal/tui/paginas/ayuda/vista.go`, y, si tiene texto,
  en `buscar.ts` (`campos`) y en `campos` de `internal/tui/paginas/ayuda/pagina.go`.
- **Una sección con título:** un bloque `pasos` con `titulo` abre su propia sección (en el sitio, su h2 con el id
  `s-<última parte de la clave>`), como «Solo con la terminal» en Empezar.
- **La regla del estilo es una prueba** (`estilo.test.ts`): resumen de una línea (140 caracteres), pasos, notas y
  respuestas de 200 como máximo, diagrama donde hay un recorrido, un tema por página de la app, anclas únicas y ningún
  texto sin usar. `capturas.test.ts` exige las cuatro variantes de cada captura, ninguna huérfana y 5 MB en total (las de los pasos de los recorridos, `recorrido-<ancla>`, incluidas).
- **Sitio:** `cd web; npm run sitio` escribe `site/ayuda/`, la página de paso `site/ayuda.html`, `404.html`,
  `sitemap.xml`, `buscar.json` y las zonas comunes de las páginas escritas a mano (abajo, en **El sitio**); el CI lo
  regenera y falla si lo guardado no coincide.
- **Capturas:** `cd web; npm run build; npm run capturas` (o `-- --solo <id>`, varios separados por comas) compila Tramtai en `bin/`, la arranca
  portable en otro puerto con datos de ejemplo y saca cada zona `[data-captura="<id>"]` en claro y oscuro y en español e
  inglés, el elemento de cada paso de los recorridos con su anillo (`-- --recorridos` saca solo esas; una ancla que la demo no muestra va en `SIN_CAPTURA`, con su motivo) y las de la portada. Necesita Edge o Chrome y un agente conectado (`TRAMTAI_CAPTURAS_MODELO`, por defecto
  `claude/haiku`); se corre en el equipo de desarrollo cuando cambia una pantalla que sale en la Ayuda. Las de la Ayuda
  más anchas que su columna del sitio salen reducidas a `ANCHO_AYUDA` (736 px), y las de la portada a 640, 960 y 1440 px.
  Si una captura muestra una ruta del equipo (la carpeta personal, la temporal o la del repositorio), el script se para
  (`rutaPrivada` en `capturas-lista.ts`). Una pantalla de la portada sale sola con `-- --solo portada-<nombre>`
  (`portada-agentes`). Cada petición real a un agente sale en la consola («petición real a …»), para contarlas.
- **Recortes del sitio:** FIG 4 y 5 de Inicio (`portada-fig-apis`, `portada-fig-servidor`) y las tarjetas de funciones
  de Producto (`funcion-*`) son entradas de `CAPTURAS` con `sitio`: `zona` (el selector de lo que se recorta; con varios
  elementos, el recorte los abarca a todos), `preparar` (JavaScript que la deja a la vista: abre un diálogo, el panel de
  la Ayuda o las opciones de la receta), `alto` y `margen`. Salen en una ventana de 420 px, para que se lean también en
  el móvil, a 2× en `site/capturas/<id>-<es|en>-<claro|oscuro>.webp`, y la página los usa con `width` y `height` de la
  mitad de sus píxeles. No enseñan el Historial: `npm run capturas -- --solo funcion-sdk` no hace ninguna petición real
  (`portada-fig-servidor` sale del Tramtai en modo servidor, con Docker y su `claude` simulado). Se paran si el recorte
  enseña una ruta, un correo o el nombre del equipo. FIG 17, la terminal, no es una imagen: es la pantalla de Inicio de
  `terminal.json` (`comun:terminal-recorte`, que pinta `sitio.ts`). `site/og.png` sale de `web/scripts/og.html` con
  `-- --sitio`.
- **Los agentes en la demo:** las capturas no instalan ni quitan ningún CLI. `SIMULAR` (`capturas-lista.ts`) corre en
  cada página antes que la app y cambia lo que el servicio responde de los agentes: en Agentes, Antigravity sale sin
  instalar (así se ve «Instalar», el ancla `agentes.instalar`, en cualquier equipo), y con
  `tramtai.capturas.sinListos` en `localStorage` ningún agente está listo, para la oferta «Prepara tus agentes» de la
  bienvenida (la captura `preparar`).
- **Demo de la portada:** sale en la corrida completa y sola con `npm run capturas -- --bienvenida`; lanza la bienvenida de verdad (acepta la oferta de la primera
  vez y pulsa «Siguiente») y saca una pantalla completa de 1440×900 por paso, con la ventana del recorrido encima, en
  claro y oscuro y en español e inglés: `site/capturas/bienvenida-<n>-<claro|oscuro>-<es|en>-<960|1440>.webp` (`BIENVENIDA` en `capturas-lista.ts`, uno
  por paso de `pasosBienvenida()`). Solo usa los datos de demostración y no pide nada a ningún agente; si cambia la
  bienvenida o una pantalla que sale en ella, se rehace. `portada.test.ts` exige una figura por paso en `site/index.html`,
  150 KB como máximo por imagen de 1440 y 6 MB en total; el carrusel es `.demo` en `sitio.js`.
- **La terminal de la portada es texto:** `npm run capturas -- --terminal` (también sale en la corrida completa) pinta
  Inicio, Mis APIs e Historial (`TERMINAL` en `capturas-lista.ts`) a 100×28, en español y en inglés, y guarda su HTML en
  `site/capturas/terminal.json` (`{"es":{"inicio":"…","apis":"…","historial":"…"},"en":{…}}`). Hace dos peticiones reales
  por idioma a `claude/haiku` con la suscripción de quien la corre (para que el Historial tenga entradas en el idioma de
  la captura: `PETICIONES` en `capturas-lista.ts`; antes de las del inglés vacía el Historial) y se para si una pantalla
  muestra una ruta del equipo, si quedó en «Comprobando…», si la red de Inicio sigue en movimiento («N peticiones en
  curso» o filas ya terminadas como «terminada 9 s», que Inicio deja a la vista unos segundos antes de quitarlas) o si
  el Historial está vacío. `tui.Captura` no da por asentada una vista así: espera a que la red quede vacía.
  `ansiAHtml` escribe clases, no colores: `t-<token>` (tinta), `tf-<token>` (fondo) y `t-b` (negrita), con
  los tokens de `DESIGN.md` (`TOKENS_TERMINAL`); la TUI usa exactamente la paleta de cada tema, así que un solo HTML por
  página e idioma sirve en los dos. Un color sin token hace fallar la captura.
  Cada flecha, forma o símbolo que Cascadia Mono no trae entero (`← ↑ → ↓ ▸ ▾ ○ ● ✓ ✗`, los rangos `U+2190–21FF` y
  `U+25A0–27BF`) va en su propio `<span class="g">`. La regla `.g { display: inline-block; width: 1ch; text-align: center }`
  la define la hoja del sitio (tarea 15), para que las columnas no se descuadren.
  `terminal.json` no lo regenera el CI (necesita peticiones reales): se guarda en el repositorio, como las capturas.
- **Fuente de la terminal:** `site/cascadia.woff2` es un subconjunto de Cascadia Mono (OFL-1.1, licencia en
  `site/CASCADIA-OFL.txt`) con las letras que salen en `terminal.json` y el ASCII imprimible, de 60 KB como máximo. Se
  rehace cuando `terminal.json` trae letras nuevas. `CascadiaMono.ttf` sale de `ttf/` en el
  [lanzamiento oficial](https://github.com/microsoft/cascadia-code/releases) (`CascadiaCode-<versión>.zip`, la 2407.24
  al hacerlo), en una carpeta aparte y sin guardarla en el repositorio. `caracteres.txt` es cada carácter distinto del
  texto de `terminal.json` (sin las etiquetas ni `&lt;`, `&gt;` y `&amp;`) más los 95 de `0x20` a `0x7e`:

  ```
  uv tool run --from "fonttools[woff]" pyftsubset CascadiaMono.ttf --flavor=woff2 --text-file=caracteres.txt --output-file=site/cascadia.woff2
  ```

  Cascadia Mono no tiene `✗` (U+2717): sale en la lista de caracteres pero el subconjunto no puede llevarlo; por eso
  los glifos van en `span.g` (arriba).
- **Fuentes del texto (v0.61):** `site/inter.woff2` es Inter variable entera (ejes `wght` 100–900 y `opsz` 14–32) e
  `site/instrument-italica.woff2`, Instrument Serif cursiva, las dos OFL-1.1 con su licencia al lado
  (`site/INTER-OFL.txt`, `site/INSTRUMENT-OFL.txt`). `InterVariable.ttf` sale del
  [lanzamiento oficial](https://github.com/rsms/inter/releases) (`Inter-<versión>.zip`, la 4.1 al hacerlo) e
  `InstrumentSerif-Italic.ttf` y su `OFL.txt`, de `ofl/instrumentserif/` en el
  [repositorio de Google Fonts](https://github.com/google/fonts), en una carpeta aparte y sin guardarlos en el
  repositorio. La lista de caracteres se guarda en `web/scripts/letras-caracteres.txt`: cada carácter distinto del
  texto de `site/*.html`, `site/ayuda/*.html` y `site/sitio.js` (sin las etiquetas y con las entidades resueltas) más
  los 95 de `0x20` a `0x7e`, `«»’“”…·→—–` y las mayúsculas y minúsculas acentuadas del español (`ÁÉÍÓÚÜÑáéíóúüñ¡¿`).
  `tnum` se añade a las funciones que se guardan porque las cifras usan `font-variant-numeric: tabular-nums`:

  ```
  uv tool run --from "fonttools[woff]" pyftsubset InterVariable.ttf --flavor=woff2 --text-file=web/scripts/letras-caracteres.txt --layout-features+=tnum --output-file=site/inter.woff2
  uv tool run --from "fonttools[woff]" pyftsubset InstrumentSerif-Italic.ttf --flavor=woff2 --text-file=web/scripts/letras-caracteres.txt --output-file=site/instrument-italica.woff2
  ```

  Las dos juntas pesan unos 65 KB (49 y 17); el tope de las letras de Inicio, sin Cascadia, es de 150 KB y lo vigila
  `sitio.test.ts`. La misma prueba comprueba que cada carácter del texto de `site/**/*.html` (sin `<pre>`, `<code>` ni
  scripts, que pinta Tramtai Mono) está en la lista. Si falta alguno, se añade a la lista y se rehacen las dos.
- **El sitio (v0.61):** siete páginas con la misma barra: Inicio (`index.html`), Producto, Agentes, Servidor,
  Desarrolladores, Ayuda (`ayuda/`) y Descargar. El sistema visual está en [DESIGN.md](../diseno/DESIGN.md) (Parte 1).
  - **Escritas a mano:** `index.html`, `producto.html`, `agentes.html`, `servidor.html`, `desarrolladores.html` y
    `descargar.html`. `npm run sitio` solo pone al día sus zonas comunes (`<!-- comun:x --> … <!-- /comun:x -->`, en
    `rellenar` de `web/scripts/sitio.ts`): `cabeza`, `cabecera` y `pie` en todas; `anclas` (el salto de los anclajes
    viejos) en Inicio; `riesgos` y `riesgos-servidor`; `terminal` y `terminal-recorte`; `camino`, y `indice-boton` y
    `en-esta-pagina` en las de documentación. Las opcionales solo se rellenan donde la página las trae.
  - **Generadas:** `ayuda/*.html` y `ayuda.html` salen de `web/src/ayuda`, y `404.html`, `sitemap.xml` y `buscar.json`,
    de `sitio.ts`.
  - **Documentación en tres columnas:** `columnasDoc` (índice, contenido y «En esta página») es la plantilla de la Ayuda;
    `servidor.html` y `desarrolladores.html` la escriben a mano con el mismo marcado y `rellenar` les pone el botón del
    índice y «En esta página» desde sus `<section id>`.
  - **Buscador:** Ctrl K, ⌘K o `/` abren un diálogo que busca en `site/buscar.json` (una entrada por tema y apartado de
    Ayuda, Desarrolladores y Servidor, en los dos idiomas; 100 KB como máximo). Se genera con `npm run sitio`; no hay
    servicio externo.
  - **Anclajes viejos:** `#descargar`, `#como`, `#funciones`, `#por-que`, `#privacidad`, `#agentes`, `#servidor`, `#app`,
    `#terminal` (y los de antes de la v0.59) llevan a su página con `ANCLAS_PORTADA` y `destinoPortada`, que Inicio
    incrusta como salto en el `<head>`. `servidor.html`, `desarrolladores.html`, `ayuda/<tema>.html` y `descargar.html`
    son direcciones que enlazan las apps y el README: no se cambian de nombre. `enlaces.test.ts` comprueba cada enlace
    interno y cada anclaje.
  - **Versión y descargas:** `web/scripts/descargas.mjs` escribe el número de versión (`[data-version]`) y el JSON
    `#descargas` con la URL de cada archivo del último release en las páginas que se le pasan. El workflow «Página» le
    pasa `site/index.html` (el botón del arranque) y `site/descargar.html`; sin páginas, las mismas dos. Una página nueva
    con descargas se añade a esa lista y a la del workflow, y su prueba es `descargas.test.ts`.
  - **`sitio.js`, sin librerías:** el tema (`tramtai.tema`) y el idioma (`tramtai.idioma`), el menú, el buscador, las
    pestañas, Copiar, «En esta página» y el índice, lo vivo (`.vivo` pasa a `.visto` al entrar en pantalla) y la
    demo. `sistemaDe`, `pestanaDe` y `destinoDescarga` eligen el sistema, la pestaña de Descargar y el archivo del
    botón; sin un sistema seguro (móvil, tableta, ChromeOS, Linux ARM o sin datos) el botón dice «Descargar» y lleva a
    `descargar.html`, nunca a un archivo equivocado.
  - **Pruebas:** `sitio.test.ts` (páginas, `buscar.json`, `sitemap.xml`, letras), `portada.test.ts` (Inicio,
    Producto y Agentes), `enlaces.test.ts`, `descargas.test.ts` y `estilo-textos.test.ts`; en Go,
    `TestComandosDelSitioExisten` (`cmd/tramtai/sitio_test.go`): cada orden de `tramtai` que enseña un `<pre>` existe.
- **La ventana en `site/index.html`:** no se escribe a mano. `npm run sitio` rellena la zona `<!-- comun:terminal -->`
  (`ventanaTerminal` en `web/scripts/sitio.ts`) con el semáforo, las pestañas «1 Inicio · 4 Mis APIs · 5 Historial», el
  rótulo y un panel por página e idioma tomado de `terminal.json`; el CI falla si lo guardado no coincide. Los colores
  `.t-*`, `.tf-velo`, `.g` y la fuente `Tramtai Mono` están en `site/estilo.css`, y el tema de la ventana sigue el del
  sitio (`light-dark()`). Si `terminal.json` cambia, se regenera el HTML (`npm run sitio`) y, si trae letras nuevas, la fuente.

### Recorridos: cómo añadir un paso o un ancla

Los recorridos guiados (driver.js) salen de la misma fuente que la Ayuda: el bloque `recorrido` de cada tema de
página (`web/src/ayuda/temas/<id>.ts`). No hay otra lista de pasos que tocar.

- **Un paso:** una línea en el bloque `recorrido` del tema de esa página, `{ ancla, titulo, texto, bienvenida? }`, con
  `titulo` y `texto` como claves de `ayuda.es.ts` y `ayuda.en.ts` (`estilo.test.ts` limita el título a 60 caracteres y
  el texto a 200). Cada página tiene de 3 a 5 pasos. Con `bienvenida: true` el paso entra también en la bienvenida, que
  los toma en el orden del menú (Inicio, Agentes, Probar, Mis APIs, Espacios) más un cierre; hoy son 14 pasos y
  `anclas.test.ts` fija su orden.
- **Dos temas en una página:** Agentes tiene «Agentes y modelos» y «Preparar tus agentes». Su **?** abre el primero de
  `indice.ts` y su recorrido junta los pasos de los dos, en ese orden.
- **Un ancla:** el atributo `data-recorrido="<pagina>.<nombre>"` en el elemento del `.vue` que señala el paso (minúsculas
  y guiones; `app.<nombre>` para lo que está en todas las páginas, como el selector de espacio o el «?», que viven en
  `App.vue` o `componentes/CabeceraPagina.vue`).
- **`anclas.test.ts` lo comprueba:** que cada paso apunte a un `data-recorrido` que existe en el `.vue` de su página y
  que no sobre ninguno (toda ancla de la interfaz tiene su paso). Si quitas o renombras un elemento, la prueba falla
  hasta que el paso y el ancla vuelvan a coincidir.
- **Ajustes:** tiene dos pestañas y la que no se ve esconde sus elementos. Si el ancla está en una de ellas, añádela a
  `web/src/pestanasAjustes.ts`, que dice qué pestaña abrir antes de señalarla (tiene su prueba).
- **El recorrido solo enseña:** no pulsa nada ni cambia datos. Si un elemento no está en pantalla, el paso sale
  centrado; Esc lo cierra y las flechas pasan de paso. El elemento señalado no se puede pulsar mientras tanto, y Atrás
  (o perder la sesión) cierra el recorrido. Lo único que hace solo es cambiar de página y, en Ajustes, abrir la pestaña
  de un paso.
- **La primera vez** se recuerda en `localStorage` (`tramtai.bienvenida`: `hecha` o `descartada`); no se ofrece si se
  entró por un enlace profundo. Para que vuelva a ofrecerse, borra esa clave y recarga la app en Inicio; para hacerla
  sin más, «Haz el recorrido de Tramtai» en Primeros pasos.
- **Capturas del sitio:** un paso nuevo necesita su captura y, si entra en la bienvenida, también la demo de la portada;
  cómo sacarlas está arriba, en **Capturas** y **Demo de la portada**.

## Probar

```powershell
go vet ./...
go test ./...                                    # sin agentes reales: simulado, procesos, persistencia, servicio
$env:CGO_ENABLED = '1'; go test -race ./...      # necesita un compilador C (llvm-mingw, instalado con winget)
$env:TRAMTAI_PRUEBA_AUTENTICACION = 'suscripcion'
go test -tags agentes -v ./internal/agentes      # CLIs reales: respuesta, uso y cancelación (consume cuota)
go test -tags m0b -v -count=1 ./pruebas/m0b      # modos con clave de API, ver M0B.md
```

**Modo desarrollo:** `tramtai servicio iniciar --abrir` corre el servicio en primer plano con una línea por petición en la terminal y abre la app de escritorio (`--navegador` para el navegador); Ctrl+C lo detiene. Si ya está en segundo plano, `tramtai servicio registro -f` sigue el mismo registro.

Para probar el servicio con la suscripción sin tocar la configuración global, arráncalo con `TRAMTAI_ADAPTADORES__CLAUDE__AUTENTICACION=suscripcion` (y la de Codex) en el entorno.

### Las páginas de la terminal

Las pruebas de las páginas (`internal/tui/paginas/*`) son de caja negra (`package <página>_test`): la tocan por su
interfaz, contra el servicio de `paginas/prueba`. `prueba.App` la lleva como la terminal (cada Cmd en su goroutine,
`Pulsar`, `Escribir`, `Pegar`, `Entregar` y `Hasta`/`HastaVer` para lo que llega en vivo) y guarda en `Fuera` lo que es de
la app (avisos, preguntas, `IrA`). Cada paso vuelve cuando nada en el proceso puede avanzar solo (ni un Cmd, ni el
servicio, ni datos sin leer en una conexión) y, si en 10 s no se asienta, la prueba falla. Toda página nueva se prueba
con él, y la terminal entera también, con `prueba.ConducirApp`. Una respuesta a destiempo se fabrica por la vía de verdad: `Retener` guarda los Cmd que
la página pide, la prueba cambia la versión, el espacio o el contexto, y `Soltar` los corre. `prueba.Reloj` es la hora de
la página: el sondeo y las reconexiones esperan a `Pasar`. Lo único que una prueba cambia por dentro está en el
`export_test.go` de la página: el reloj y de dónde lee los eventos. Dentro del paquete solo quedan las pruebas de
funciones puras y las que pintan estados que el servicio de prueba no da (un conversor con error de red, por ejemplo).

### Terminal: cómo añadir una página

Cada página vive en `internal/tui/paginas/<página>` y no importa `internal/tui` ni otras páginas: habla con la app por
los mensajes de `internal/tui/pagina` (`IrA`, `Volver`, `CambioEspacio`, `ErrorServicio`…).

1. **La interfaz `pagina.Pagina`:** `Titulo`, `Iniciar(pagina.Contexto)` (carga sus datos con `Contexto.Cli`, el
   cliente con la clave de servicio, y nunca por `/ui/*`), `Update`, `Vista` (como mucho `Ancho`×`Alto`), `Teclas` (las
   del pie, además de las comunes), `EnCampo` (con un campo enfocado no valen las teclas comunes ni la `q`) y `Ayuda`
   (el `paginaApp` de su tema). Si tiene algo en vivo, `pagina.Ocultable`; si se puede perder algo al salir,
   `pagina.Pendiente`. Cada respuesta lleva su `pagina.Origen` y se descarta si ya no es `Vigente`.
2. **Registrarla:** su clave en `internal/tui/pagina` (la de la Web UI), su `Nueva` en `fabricas` de
   `internal/tui/app.go` y su lugar en `gruposMenu` de `internal/tui/menu.go`, en el orden de `web/src/menu.ts`
   (`TestTodasRegistradas` y `TestMenuComoLaWeb` lo comprueban).
3. **Los textos:**
   - los que también tiene la Web UI se toman de ella con `datos.Texto` y `datos.Textof`: la clave va en su lista
     `CLAVES_<PÁGINA>` de `web/scripts/terminal-datos.ts` y `npm run sitio` regenera `internal/tui/datos`;
   - los propios de la terminal van con `idioma.T`/`Tf` y un literal, y su inglés en
     `internal/idioma/en_terminal.go` (`TestCobertura`).
4. **La Ayuda:** la línea `terminal` del tema de la página dice cómo se hace allí, con sus teclas; el `?` la abre con
   todas las teclas de la página.
5. **Las pruebas** (caja negra, `package <página>_test`): `cli, inst := prueba.Servicio(t)` levanta el servicio real en
   `httptest`; `a := prueba.Abrir(t, pagina.Nueva("es"), prueba.Contexto(t, cli, inst))` la abre y espera su carga
   (`a.EsperarCarga()` si algo llega más tarde); `a.Pulsar`, `a.Escribir`, `a.Pegar` y `a.HastaVer("…")` la recorren.
   Prueba también la ventana de 60×20, el servicio caído a mitad (avisa y `r` reintenta, nada se cierra) y
   `TRAMTAI_ASCII=1`.

### Herramientas con los agentes reales

`go test -tags canario -run TestHerramientasConAgentesReales -v -count=1 ./internal/api/` recorre el ciclo de
herramientas por OpenAI, Anthropic y la API propia con cada agente instalado y con sesión o clave (Claude `haiku`,
Codex, GitHub Copilot `auto` y Gemini si hay `GEMINI_API_KEY`); salta los que falten. Usa la cuota de tus suscripciones
(Copilot Free gasta un crédito por petición). Su resultado es la tabla «Qué agentes las atienden» de
[la guía del desarrollador](../desarrolladores.md).

### Herramientas de terceros: `internal/apuntar`

`tramtai herramientas` (CLI), la pestaña Herramientas de Mis APIs y la de la terminal apuntan Claude Code, Codex,
Continue, opencode y Kilo Code a Tramtai. Las tres interfaces usan las mismas rutas del servicio
(`internal/servicio/herramientas.go`), y estas llaman a `internal/apuntar`, que es el único código que toca los archivos
de esas herramientas.

**Rutas** (`/api/v1/herramientas`, las que lee la Web UI y la terminal; en el modo servidor responden
`409 no_disponible_en_servidor`):

| Ruta | Qué hace |
|---|---|
| `GET /api/v1/herramientas` | Lista de `contrato.EstadoHerramienta`: estado, modelo, archivo, motivo (`motivo_codigo` + `motivo_datos`, y `motivo` de respaldo), `clave_en_archivo`, `gestionado`, `hay_copia` |
| `POST /api/v1/herramientas/activar` | `{ids|todas, modelo}`: crea o reanuda la API de cada herramienta y escribe su archivo |
| `POST /api/v1/herramientas/desactivar` | `{ids|todas}`: quita lo de Tramtai y detiene su API |
| `POST /api/v1/herramientas/<id>/olvidar` | Desactiva y borra la clave del almacén y la anotación (204) |
| `POST /api/v1/herramientas/<id>/restaurar` | Repone la copia entera del archivo; responde `{copia}` |

Activar y desactivar devuelven la lista al día como `{"herramientas":[…]}`, igual que el `GET`. `tramtai herramientas clave <id>` no pasa por el servicio: lee el
almacén (`servicio.NombreClaveHerramienta`) y escribe solo la clave. Los tipos de la API salen de `internal/contrato`
(`go generate ./internal/contrato`).

**Qué hace el núcleo** (`apuntar.go`, `anotacion.go`): para activar relee el archivo, guarda la copia entera
(`datos/herramientas/copias/<id>-<AAAAMMDD-HHMMSS>.<ext>`, 0600, las 3 más recientes por herramienta; la hora del nombre
va en UTC para que el orden por nombre sea estable y sin saltos de horario, y la Web UI y la terminal enseñan la fecha de
la copia en hora local), pide al `Formato`
el archivo nuevo, comprueba que su huella no cambió desde que lo leyó (si cambió, reintenta una vez y, si no, devuelve
`ErrCambioMientras`), lo escribe con `configuracion.EscribirAtomico` y por último anota en
`datos/herramientas/<id>.json` lo escrito y lo que había (`Escrito`, `Previos`). La anotación nunca lleva secretos: para
una clave en un archivo guarda `sha256:<hex>`. Si la herramienta ya está activa en otra ruta devuelve
`ErrOtraActivacion`: quien llama la quita antes. `Quitar` deshace solo lo que sigue siendo de Tramtai; `Restaurar` repone
la copia.

**El contrato `Formato`** (una implementación por tipo de archivo: `json.go` para JSON/JSONC con `hujson`, `toml.go`,
`yaml.go`; no leen ni escriben en disco):

- `Aplicar(actual, Valores)` devuelve el archivo con Tramtai puesto, lo escrito y lo que había en esas claves (las claves
  de `previos` son un subconjunto de las de `escrito`);
- `Quitar(actual, Anotacion)` lo devuelve sin lo de Tramtai y dice si ya no queda nada más;
- `Contiene(actual, Anotacion)` dice si sigue lo que se escribió (compara huellas en las claves).

Conservan lo que no es suyo: comentarios y orden en JSON/JSONC; en TOML y YAML, un bloque entre `# >>> tramtai` y
`# <<< tramtai` (o el modelo `name: Tramtai` de Continue) sin reescribir el resto. Rechazan un archivo roto con
`ErrArchivoIlegible` sin tocarlo.

**Añadir una herramienta:**

1. Una entrada en `apuntar.Lista()` con su `ID` (también el nombre de su API: `^[a-z0-9][a-z0-9_-]{0,39}$`), el nombre de
   la UI, `Programa` (para detectarla en el PATH), `Ruta(alcance, proyecto)`, `EnProyecto` si admite un archivo de
   proyecto, `ClaveEnArchivo` y, si hace falta, `Gestionado` o `Previa` (algo que impida activar, antes de escribir nada).
2. Su `Formato`, o uno de los existentes si el archivo es del mismo tipo (opencode y Kilo comparten `OpenCode()`), con
   archivos de ejemplo en `testdata/<formato>/`: las pruebas comprueban que se conservan los comentarios, que se deshace
   con los valores previos, que se respeta lo que el usuario cambió después y que un archivo roto no se toca.
3. Si la clave no va en el archivo, la herramienta tiene que poder pedirla a una orden (`tramtai herramientas clave <id>`):
   añade `TRAMTAI_HERRAMIENTA_<ID>` a `credenciales.Internos` y la orden a `Valores.Orden`. Rutas con `%` se rechazan
   porque `cmd.exe` las expandiría.
4. Los textos van por `idioma.T` con su inglés en `internal/idioma/en_apuntar.go`, y los de la Web UI en `es.ts` y
   `en.ts` (`herramientas.*`); la terminal los recibe con `npm run ayuda-terminal`.
5. El tema de Ayuda `conectar-herramientas` y la prueba con el programa real (la comprobación al estilo M0 de
   `docs/equipo/releases/v0.59-herramientas-y-sitio/m0.md`, con `HOME`, `CLAUDE_CONFIG_DIR` y `CODEX_HOME` temporales y
   nunca el Tramtai instalado ni el puerto 17373).

### Preparar los CLI de los agentes: `internal/preparar`

Desde la v0.60, Tramtai instala, conecta, actualiza y desinstala los CLI de Claude Code, Codex, GitHub Copilot y
Antigravity. La Web UI (Agentes y la bienvenida), la terminal y la CLI (`tramtai agentes instalar|entrar|actualizar|
desinstalar|versiones`, alias `install|login|update|uninstall|versions`) usan las mismas rutas del servicio
(`internal/servicio/preparacion.go`), y estas llaman a `internal/preparar`, el único código que lanza instaladores. La
spec y las decisiones están en `docs/equipo/releases/v0.60-preparar-agentes/` (`spec.md`, `m0.md`).

**Rutas** (con la sesión de la Web UI o la clave del servicio; la clave del espacio no basta: 403 `no_autorizado`):

| Ruta | Qué hace |
|---|---|
| `GET /api/v1/agentes/{id}/preparacion` | Estado de la última preparación del agente, o `null` |
| `POST /api/v1/agentes/{id}/preparacion` | `{"accion": "instalar"\|"actualizar"\|"iniciar_sesion"\|"desinstalar", "borrar_datos": false, "confirmar": false}` (202) |
| `POST /api/v1/agentes/{id}/preparacion/codigo` | `{"codigo": "…"}`: lo pegado, al CLI por su entrada; nunca se registra |
| `DELETE /api/v1/agentes/{id}/preparacion` | Cancela y cierra el árbol de procesos |
| `GET /api/v1/agentes/versiones` | Por agente: instalada, última, comprobadas por acción y acciones habilitadas; `?forzar=1` repite la consulta |

Los tipos salen de `internal/contrato` (`go generate ./internal/contrato`). Las pantallas siguen el estado con un sondeo
corto (`seguirPreparacion` en `web/src/preparacion.ts`), como el conversor.

**La receta** (`adaptadores.Receta`, en la ficha de cada agente, `internal/agentes/<nombre>/ficha.go`) es fija en el
código, por sistema (`windows`, `darwin`, `linux`): nada de lo que escribe el usuario entra en una orden.

- `Instalar` y `Actualizar`: la orden oficial (sin entrada en `Actualizar`, se reinstala); `Descarga`, si Tramtai baja el
  binario él mismo y comprueba su suma (Copilot en Windows, con `{version}` y `{arq}`); `FijaVersion` si admiten
  `{version}`. La descarga no pisa el ejecutable: lo aparta a `.viejo` y pone el nuevo (`colocar`), porque Windows
  deja renombrar un `copilot.exe` en marcha pero no reemplazarlo; el `.viejo` se borra entonces o en la próxima.
- `Rutas`: dónde deja el ejecutable (`~` es la casa de la cuenta que corre el servicio). Si queda fuera del PATH del
  servicio, se guarda en `comandos_agentes.<agente>` y se quita al desinstalar.
- `Borrar` (lo que se quita al desinstalar: ningún CLI tiene orden propia) y `DatosCLI` (sesión y ajustes, solo con
  «Borrar también su sesión y ajustes»). Desinstalar se niega si la ruta pasa por un enlace o una junction.
- `Ultima`: de dónde sale la última versión publicada (`url` con un manifiesto, `github` con `Prefijo`, o `npm`).
- `Sesion` (`LectorSesion`, `nil` en Antigravity): argumentos, `Entorno` (por ejemplo `BROWSER` o
  `COPILOT_DEBUG_BROWSER` para que no abra el navegador del sistema), y las expresiones del enlace, el código y la
  petición de pegar. El lector trabaja sobre el texto acumulado y sin ANSI; sin enlace ni código en 30 s desde que el
  proceso arrancó, `salida_no_reconocida`.
- **La orden a mano** (lector sin reconocer, inicio de sesión fallido o versión sin comprobar) sale de
  `Preparador.ordenConsola`: en el escritorio, el CLI por su nombre si el PATH lo encuentra y, si no, su ruta completa
  (entre comillas si hace falta; en Windows con `& ` delante, para PowerShell); en el servidor, `servidor.OrdenConsola`.
  Va también aparte, en `orden_consola`, para que la Web UI arme la frase en su idioma (Ruling 37).
- `Comprobadas`: las versiones probadas por acción, exactas o con comodín de parche (`2.1.*`).

**La preparación** es una tarea por agente con estados (`instalando`, `actualizando`, `desinstalando`, `comprobando`,
`esperando_usuario`, `listo`, `error`, `cancelado`), una a la vez por agente (`ya_en_curso`), plazo de 15 minutos y el
árbol de procesos atado como el de las peticiones. Sobrevive a cerrar la pantalla; si el servicio se reinicia a mitad,
queda en `error` con `interrumpida`. Guarda su estado en `datos/agentes/preparaciones/<id>.json` y las últimas líneas
(redactadas) en `<id>.log`. No actualiza ni desinstala un agente con peticiones en curso (`peticiones_en_curso`) y,
mientras lo instala, actualiza o desinstala, el planificador lo trata como no disponible. La invariante es que un
instalador nunca corre a la vez que una petición (Ruling 39), y por eso `Iniciar` cuenta dos veces: la primera con la
reserva invisible, para no frenar peticiones si al final no se lanza; la segunda con la reserva ya visible, porque una
petición que entre entre medias ya no se lanza (el planificador mira `preparando` al aceptar y al lanzar) y una que
entrara antes sale en esta cuenta. No juntes las dos: con una sola, una petición puede colarse con el instalador. El coste
es un `agente_no_disponible` de unos milisegundos si la reserva se deshace en la segunda cuenta.

**Versiones comprobadas y la lista firmada.** `Preparador.Comprobada(agente, accion, version)` mira la receta y la lista
firmada (`URLLista`: `compatibilidad.json` y `compatibilidad.json.sig` del release fijo `compatibilidad`, firmados con la
clave de `internal/actualizacion`), salvo que la versión esté marcada como no compatible en este equipo (`Marcar`, cuando
el lector falla en una versión comprobada). Lo bajado y las marcas quedan en `datos/agentes/compatibilidad.json`. La lista
se lee con la consulta diaria de versiones (`servicio.buscar_versiones_agentes`, `true` por defecto); una lista cuya firma
no vale se rechaza.

**El job semanal** (`.github/workflows/agentes-compatibilidad.yml`, los lunes y a mano con `workflow_dispatch`) corre
`go run ./cmd/compatibilidad comprobar <resultado.json>` en Linux, macOS y Windows: con las fichas reales y el mismo
`Preparador` que el servicio, instala la última versión de cada CLI, lanza su inicio de sesión hasta el enlace y el código
(sin cuenta) y lo cancela, actualiza y desinstala. `cmd/compatibilidad juntar` añade a la lista publicada las versiones
que pasaron en los tres sistemas, sube `version` y solo escribe si cambia algo; el job la firma con
`FIRMA_ACTUALIZACIONES`, comprueba la firma y la sube al release. Si algo falla, abre o comenta un issue con lo que hay
que ajustar en la receta. Publicar y avisar solo pasan desde `main` (la ejecución semanal y la manual sobre `main`): en
otra rama el job prueba, pero no publica la lista ni abre issues. Un límite de la API de GitHub cuenta como «sin concluir»: la versión no entra ni sale.

**Añadir una versión comprobada a mano** (si el job no puede, o para una versión de Tramtai):
1. Lo normal es lanzar el job a mano (Actions › «Compatibilidad de los agentes» › Run workflow): prueba y publica.
2. Para que viaje con Tramtai, añádela a `Comprobadas` de la receta, en las acciones que probaste, y publica la versión.
3. Solo si hace falta corregir la lista publicada sin el job: baja `compatibilidad.json` del release, añade la versión
   en `agentes.<id>.<accion>`, sube `version` en 1, fírmala (`go run ./scripts/firmar compatibilidad.json` con
   `FIRMA_ACTUALIZACIONES` en el entorno), comprueba la firma (`go run ./scripts/firmar -comprobar compatibilidad.json`)
   y sube los dos archivos con `gh release upload compatibilidad compatibilidad.json compatibilidad.json.sig --clobber`.

**Pruebas:** CLI falsos que imitan el instalador y el inicio de sesión (con las salidas reales del M0 en
`internal/agentes/<nombre>/testdata/sesion-*.txt`), sin red ni CLI reales. Una prueba con un CLI real se hace como en el
M0: copia de Tramtai en `bin/`, puerto 17388, `HOME`, `USERPROFILE`, `CLAUDE_CONFIG_DIR`, `CODEX_HOME` y `COPILOT_HOME`
temporales, y nunca con el Tramtai instalado ni el puerto 17373. Los textos de la CLI y del servicio van con `idioma.T`
(`internal/idioma/en_preparar.go`), los de la Web UI en `preparar.*` y `bienvenida.*`, y la Ayuda es el tema `preparar`.

### Ejemplos de la guía

`go test -tags ejemplos -run TestEjemplos -v ./internal/api/` ejecuta `docs/ejemplos` contra un agente simulado (el
job «Ejemplos» del CI). En local necesita Python con `openai` y `requests` (`TRAMTAI_PYTHON` apunta a su intérprete),
`npm install` en `docs/ejemplos/javascript` y, para curl, `sh`, `curl` y `jq`; lo que falte se salta fuera del CI.

Cada ejemplo tiene su versión inglesa en la carpeta `en/` de su lenguaje (`python/en/tools.py`, `go/en/tools/main.go`…),
con nombres, comentarios, mensajes y variables en inglés (`TRAMTAI_KEY`, `TRAMTAI_MODEL`); los campos y las rutas de la
API propia (`mensajes`, `rol`, `/api/v1/trabajos`…) no se traducen, porque son lo que la API acepta. Van dentro de la
carpeta de su lenguaje para compartir el módulo de Go, `node_modules` y `requirements.txt`, y `en/file` lee
`docs/ejemplos/sample.txt`. El agente simulado contesta en inglés cuando la conversación no lleva ¿ ni tildes, y la prueba
exige los 21 de cada idioma. Al añadir un ejemplo, escribe los dos, ponlos en `esperado` y en las dos mitades de
`site/desarrolladores.html` (`<code lang="es" data-ejemplo>` y `<code lang="en" data-ejemplo>`): la página en inglés carga
y copia los de `en/`, y `portada.test.ts` falla si uno trae español.

## Estructura

```
cmd/tramtai/           CLI (Cobra); sin argumentos abre la terminal interactiva
internal/nucleo/       estados, solicitud, trabajo, errores y presupuestos
internal/carpetas/     raíz del usuario (normal o portable), permisos de sus carpetas y comparación de rutas
internal/persistencia/ SQLite (trabajos, eventos, idempotencia, retención)
internal/planificador/ cola, concurrencia, cancelación, drenaje y recuperación
internal/procesos/     supervisión de procesos (Job Objects / grupos Unix)
internal/adaptadores/  Claude, Codex, Gemini API, Antigravity y agente simulado
internal/credenciales/ claves de proveedor en el almacén del sistema (Windows, Llavero de macOS, Secret Service de Linux)
internal/documentos/   cargas, formatos admitidos, conversión con markitdown y preparación del conversor
internal/salidas/      validación JSON Schema
internal/api/          HTTP: API nativa, SSE y fachadas OpenAI/Anthropic
internal/apuntar/      apunta Claude Code, Codex, Continue y opencode/Kilo a Tramtai: copias, anotación y un Formato por tipo de archivo
internal/preparar/     instala, conecta, actualiza y desinstala los CLI de los agentes con su receta; versiones comprobadas
cmd/compatibilidad/    el job semanal que prueba los CLI y arma la lista firmada de versiones comprobadas
internal/servicio/     servicio único: instancias, puntos de acceso, Web UI y entrada local
internal/configuracion/ configuración por capas y perfiles; edición de config.yaml y local.yaml y escritura atómica
internal/cliente/      cliente del servicio (CLI, terminal y escritorio)
internal/tui/          terminal interactiva (Bubble Tea)
internal/recursos/     Web UI compilada (go:embed)
web/                   Web UI: Vue 3 + Quasar
escritorio/            aplicación de escritorio Wails v3 (módulo Go aparte) e icono
pruebas/m0b/           pruebas con claves reales (build tag m0b)
scripts/               compilar, versionar e icono
```

## Publicar una versión

Commits con prefijo `feat:`, `fix:` o `docs:` y la lista de cambios en el cuerpo; después:

```powershell
./scripts/version.ps1 -Etiquetar -Resumen notas-es.txt -ResumenEn notas-en.txt   # notas cortas, para quien usa Tramtai
git push origin HEAD; git push origin --tags
```
