# Guía del equipo

Documentación interna para desarrollar Tramtai. La de usuario está en el [README](../../README.md) y en [docs/](../inicio.md).

| Documento | Contenido |
|---|---|
| [README.md](README.md) | Índice de esta carpeta y de la spec y el plan de cada release |
| [pendientes.md](pendientes.md) | Hitos y lista de tareas (incluye CI/CD) |
| [versionado.md](versionado.md) | Versión `vX.Y.Z+DDMMAAAA` calculada desde los commits, y etiquetas con notas |
| [M0B.md](M0B.md) | Validación de los modos con clave de API (pendiente con claves reales) |
| [Diseño](../diseno/DESIGN.md) | Sistema visual «Red de tranvía» de la Web UI y la página, y [contexto de producto](../diseno/PRODUCT.md) |

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
- **Sitio:** `cd web; npm run sitio` escribe `site/ayuda/` y la página de paso `site/ayuda.html`; el CI lo regenera y
  falla si lo guardado no coincide.
- **Capturas:** `cd web; npm run build; npm run capturas` (o `-- --solo <id>`) compila Tramtai en `bin/`, la arranca
  portable en otro puerto con datos de ejemplo y saca cada zona `[data-captura="<id>"]` en claro y oscuro y en español e
  inglés, el elemento de cada paso de los recorridos con su anillo (`-- --recorridos` saca solo esas; una ancla que la demo no muestra va en `SIN_CAPTURA`, con su motivo) y las de la portada. Necesita Edge o Chrome y un agente conectado (`TRAMTAI_CAPTURAS_MODELO`, por defecto
  `claude/haiku`); se corre en el equipo de desarrollo cuando cambia una pantalla que sale en la Ayuda. Las de la Ayuda
  más anchas que su columna del sitio salen reducidas a `ANCHO_AYUDA` (736 px), y las de la portada a 640, 960 y 1440 px.
  Si una captura muestra una ruta del equipo (la carpeta personal, la temporal o la del repositorio), el script se para
  (`rutaPrivada` en `capturas-lista.ts`).
- **Demo de la portada:** sale en la corrida completa y sola con `npm run capturas -- --bienvenida`; lanza la bienvenida de verdad (acepta la oferta de la primera
  vez y pulsa «Siguiente») y saca una pantalla completa de 1440×900 por paso, con la ventana del recorrido encima, en
  claro y oscuro y en español e inglés: `site/capturas/bienvenida-<n>-<claro|oscuro>-<es|en>-<960|1440>.webp` (`BIENVENIDA` en `capturas-lista.ts`, uno
  por paso de `pasosBienvenida()`). Solo usa los datos de demostración y no pide nada a ningún agente; si cambia la
  bienvenida o una pantalla que sale en ella, se rehace. `portada.test.ts` exige una figura por paso en `site/index.html`,
  150 KB como máximo por imagen de 1440 y 6 MB en total; el carrusel es `.demo` en `sitio.js`.
- **La terminal de la portada es texto:** `npm run capturas -- --terminal` (también sale en la corrida completa) pinta
  Inicio, Mis APIs e Historial (`TERMINAL` en `capturas-lista.ts`) a 100×28, en español y en inglés, y guarda su HTML en
  `site/capturas/terminal.json` (`{"es":{"inicio":"…","apis":"…","historial":"…"},"en":{…}}`). Hace dos peticiones reales
  a `claude/haiku` con la suscripción de quien la corre (para que el Historial tenga entradas) y se para si una pantalla
  muestra una ruta del equipo, si quedó en «Comprobando…» o si el Historial está vacío. `ansiAHtml` escribe clases, no
  colores: `t-<token>` (tinta), `tf-<token>` (fondo), `t-b` (negrita) y `t-d` (tenue), con los tokens de `DESIGN.md`
  (`TOKENS_TERMINAL`); la TUI usa exactamente la paleta de cada tema, así que un solo HTML por página e idioma sirve en
  los dos. Un color sin token hace fallar la captura. `terminal.json` no lo regenera el CI (necesita peticiones reales):
  se guarda en el repositorio, como las capturas.
- **Fuente de la terminal:** `site/cascadia.woff2` es un subconjunto de Cascadia Mono (OFL-1.1, licencia en
  `site/CASCADIA-OFL.txt`) con las letras que salen en `terminal.json` y el ASCII imprimible, de 60 KB como máximo. Se
  rehace cuando `terminal.json` trae letras nuevas. `CascadiaMono.ttf` sale de `ttf/` en el
  [lanzamiento oficial](https://github.com/microsoft/cascadia-code/releases) (`CascadiaCode-<versión>.zip`, la 2407.24
  al hacerlo), en una carpeta aparte y sin guardarla en el repositorio. `caracteres.txt` es cada carácter distinto del
  texto de `terminal.json` (sin las etiquetas ni `&lt;`, `&gt;` y `&amp;`) más los 95 de `0x20` a `0x7e`:

  ```
  uv tool run --from "fonttools[woff]" pyftsubset CascadiaMono.ttf --flavor=woff2 --text-file=caracteres.txt --output-file=site/cascadia.woff2
  ```

  Cascadia Mono no tiene `✗` (U+2717): sale en la lista de caracteres pero el subconjunto no puede llevarlo.

### Recorridos: cómo añadir un paso o un ancla

Los recorridos guiados (driver.js) salen de la misma fuente que la Ayuda: el bloque `recorrido` de cada tema de
página (`web/src/ayuda/temas/<id>.ts`). No hay otra lista de pasos que tocar.

- **Un paso:** una línea en el bloque `recorrido` del tema de esa página, `{ ancla, titulo, texto, bienvenida? }`, con
  `titulo` y `texto` como claves de `ayuda.es.ts` y `ayuda.en.ts` (`estilo.test.ts` limita el título a 60 caracteres y
  el texto a 200). Cada página tiene de 3 a 5 pasos. Con `bienvenida: true` el paso entra también en la bienvenida, que
  los toma en el orden del menú (Inicio, Agentes, Probar, Mis APIs, Espacios) más un cierre; hoy son 14 pasos y
  `anclas.test.ts` fija su orden.
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

### Ejemplos de la guía

`go test -tags ejemplos -run TestEjemplos -v ./internal/api/` ejecuta `docs/ejemplos` contra un agente simulado (el
job «Ejemplos» del CI). En local necesita Python con `openai` y `requests` (`TRAMTAI_PYTHON` apunta a su intérprete),
`npm install` en `docs/ejemplos/javascript` y, para curl, `sh`, `curl` y `jq`; lo que falte se salta fuera del CI.

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
