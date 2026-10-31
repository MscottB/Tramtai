# v0.59: tus herramientas apuntadas a Tramtai y sitio nuevo

La v0.59 junta dos cosas, por decisión del usuario (2026-10-31):
1. **Herramientas:** Tramtai apunta solo a Claude Code, Codex, Continue y opencode/Kilo a su API, con copia de su
   configuración antes de tocarla y un interruptor para activarlas o desactivarlas, una a una o todas.
2. **El sitio nuevo** (`site/`): la terminal rehecha, el modo servidor bien presentado y las ventajas remarcadas.

Preparar los CLI de los agentes (instalarlos, iniciar sesión, dejar sus ajustes), que también se pensó como «configurar
los clientes», es la v0.60.

El sitio solo se publica al terminar Publicar, así que sale con la versión. Va en las **últimas tareas**: así sus capturas
y textos ya incluyen las herramientas y se sacan una sola vez.

## Por qué

Lo que se vio en el sitio publicado de la v0.58 (2026-10-31):
- **La terminal:**
  - Son dos capturas de pantalla completa (112×32), una al lado de la otra a unos 540 px, sin marco de ventana. La letra
    no se lee.
  - La de Inicio salió a medio cargar: dice «Comprobando…» dos veces.
  - La clara va sobre blanco y no parece una terminal.
  - Encima van dos párrafos de atajos de teclado.
- **El servidor:** en la portada es un título, dos párrafos y un bloque de código, entre la terminal y las capturas.
  `servidor.html` es buena documentación, pero abre con los cinco riesgos antes de decir qué ganas, y no tiene nada
  visual.
- **Las ventajas:** están repartidas en tres sitios (seis motivos de 5 a 10 palabras con icono, el plano de «Funciones»
  y el tablero de Privacidad). La portada tiene 14 secciones y la terminal y el servidor quedan escondidos en medio.
- **El idioma:** las cinco capturas de la app de la portada solo existen en español, y algunos comandos en inglés no
  existen en la CLI (`service start`) o no coinciden con la página en español.

## Referencias

Se revisaron unos 30 sitios de productos parecidos (2026-10-31). De cada uno se toma un patrón, adaptado a la dirección
«Red de tranvía» de `docs/diseno/DESIGN.md`; no se copia marca, texto ni imagen de nadie.

| Patrón | De dónde | Dónde se usa |
|---|---|---|
| Terminal como texto real dentro de una ventana | Ghostty, Atuin, Textual | Terminal |
| Un solo marco con pestañas por vista | Bun | Terminal |
| Teclas como pastillas junto a lo que hacen | Raycast, Atuin | Terminal |
| Pie de figura en texto | Warp | Terminal |
| «Quieres el escritorio si… / el servidor si…» | AnythingLLM, n8n | Servidor |
| Seguridad contada como virtud (HTTPS, 2FA, proxy) | Syncthing, Coolify | Servidor |
| Un comando por sistema, en pestañas | Ollama | Servidor |
| Responsabilidad con acciones, después de los beneficios | Plausible, Vaultwarden | `servidor.html` |
| Pilares de una frase en un diagrama propio | Raycast | Ventajas |
| Franja de cifras grandes con etiqueta pequeña | Ollama, Atuin | Ventajas |

Las maquetas de la terminal, el servidor y las ventajas se aprobaron con el usuario en el chat del 2026-10-31.

## Herramientas: apuntarlas a Tramtai

Decidido con el usuario el 2026-10-31. Los formatos salen de la documentación oficial de cada herramienta, consultada ese
día (enlaces en la tabla); ninguno se ha probado todavía contra el programa real (ver «Comprobación con los programas
reales»).

### Qué entra

| Herramienta | Archivo | Qué escribe Tramtai | Clave |
|---|---|---|---|
| Claude Code | `~/.claude/settings.json`; en un proyecto, `.claude/settings.local.json` ([docs](https://code.claude.com/docs/en/settings)) | `env.ANTHROPIC_BASE_URL` (`…/anthropic`), `env.ANTHROPIC_MODEL` y `apiKeyHelper` | Almacén cifrado; `apiKeyHelper` llama a `tramtai herramientas clave claude-code` |
| Codex CLI | `$CODEX_HOME/config.toml` (`~/.codex/config.toml`) ([docs](https://learn.chatgpt.com/docs/config-file/config-advanced)) | `model`, `model_provider = "tramtai"` y `[model_providers.tramtai]` con `base_url` (`…/openai/v1`), `wire_api = "responses"` y `auth.command` | Almacén cifrado; `auth.command` llama a `tramtai herramientas clave codex` |
| Continue | `~/.continue/config.yaml` ([docs](https://docs.continue.dev/reference)) | Un modelo `provider: openai` con `apiBase`, `model`, `roles` y `capabilities: [tool_use]` | En el archivo |
| opencode y Kilo Code | `~/.config/opencode/opencode.json(c)` y `~/.config/kilo/kilo.jsonc`, mismo formato ([docs](https://opencode.ai/docs/providers/)) | Un proveedor `tramtai` (`@ai-sdk/openai-compatible`) con `baseURL` y el modelo | En el archivo |

- **Se detecta** por su programa en el PATH o por su carpeta de configuración. Las rutas en Windows de opencode y Kilo se
  confirman en la primera tarea del plan.
- **Para más adelante:** Aider (falta probar cómo trata los nombres con «/»), Qwen Code, Zed (la clave va a su llavero),
  Cline (solo con `cline auth`, sin esquema documentado) y Roo Code (importación sin esquema documentado).

### Dónde se usa

- **App:** una pestaña «Herramientas» en Mis APIs, junto a «APIs por aplicación» y «Clave del espacio».
  - Una fila por herramienta, con su estado (punto y texto): No detectada, Detectada, Activa, Desactivada o
    Desactualizada.
  - Cada fila tiene un **interruptor** para activarla o desactivarla, y muestra el modelo y el archivo que toca.
  - Encima de la lista, «Activar todas» y «Desactivar todas», para las detectadas.
- **Terminal:** la misma pestaña en Mis APIs, con las mismas acciones y sus teclas.
- **CLI:** `tramtai herramientas` (alias `tools`), con estas órdenes:
  - `listar`;
  - `activar <id>|--todas [--modelo agente/modelo]`;
  - `desactivar <id>|--todas`;
  - `olvidar <id>`;
  - `restaurar <id>`;
  - `clave <id>`.

### Activar

1. **Modelo:** eliges agente y modelo; por defecto, el primero conectado. Con «Activar todas» se elige una vez para todas.
2. **API:** Tramtai crea en el espacio actual una API con el nombre de la herramienta («Claude Code», «Codex»…), en el
   grupo «Herramientas», limitada a ese agente y con su propia clave. Si ya existe (porque se activó antes), la reanuda.
3. **Clave:** Claude Code y Codex admiten pedirla a una orden. Para ellas, la clave se guarda cifrada en el almacén del
   sistema (donde ya están las de los proveedores) y su configuración llama a `tramtai herramientas clave <id>`, así que
   no queda ninguna clave en disco. Para Continue y opencode/Kilo, la clave va en su archivo, con permisos solo para el
   usuario, y la app lo dice.
4. **Copia:** el archivo entero se copia en `datos-tramtai/herramientas/copias/<id>-<fecha>`, con permisos solo para el
   usuario: puede tener otros secretos suyos.
5. **Escritura:** Tramtai escribe solo lo suyo y anota en `datos-tramtai/herramientas/<id>.json` qué escribió y qué
   valores había antes.
   - Relee el archivo justo antes de escribir, porque Claude Code y Codex reescriben sus propios archivos.
   - Escribe de forma atómica (temporal y renombrar) y reintenta si Windows tiene el archivo bloqueado.
   - Conserva los comentarios: JSON/JSONC con un lector que los respeta; TOML y YAML con un bloque entre
     `# >>> tramtai` y `# <<< tramtai`, sin reescribir el resto.

### Desactivar, olvidar y restaurar

- **Desactivar** quita solo lo que puso Tramtai y devuelve los valores de antes; lo que el usuario cambió después se
  respeta. La API se detiene (conserva su historial) y la clave sigue en el almacén, así que volver a activar es
  inmediato.
- **Olvidar** desactiva y además borra la clave del almacén y la anotación. La API queda detenida, para que el usuario
  decida si borrarla en Mis APIs.
- **Restaurar** repone la copia entera del archivo. Es el último recurso; pide confirmación y dice de qué fecha es la copia.

### Alcance según el espacio

- **Desde el espacio global:** la configuración del usuario de cada herramienta.
- **Desde el espacio de un proyecto:** solo Claude Code, en `.claude/settings.local.json` de la carpeta del proyecto, y
  la API se crea en ese espacio. Nunca se escribe en `.claude/settings.json`, que se comparte por git.
- **Solo global:** Codex (un `config.toml` de proyecto no admite proveedores), Continue y opencode/Kilo (la clave
  quedaría en una carpeta del repositorio). Desde un proyecto, su fila lo dice.

### Avisos y errores

- **Archivo ilegible** (JSON roto, TOML inválido): no se toca; la fila lo dice con la ruta y qué hacer.
- **Ajustes gestionados:** si unos ajustes de la empresa (managed settings de Claude Code) mandan sobre los del usuario, se
  avisa de que Tramtai no puede cambiarlos.
- **Desactualizada:** si cambia el puerto, la API ya no existe o el archivo ya no tiene lo que Tramtai escribió, la fila
  pasa a «Desactualizada» con «Volver a activar».
- **`tramtai herramientas clave <id>`:** solo sirve para herramientas activas y escribe la clave en la salida estándar,
  sin nada más. Nunca aparece en registros ni en el historial.
- **Sincronización:** la Ayuda avisa de que, si sincronizas `~/.claude` o tus dotfiles, la configuración llega a equipos
  sin Tramtai.

### Comprobación con los programas reales

Antes de cerrar la versión, una prueba al estilo M0 con Claude Code y Codex reales, apuntados a una copia de prueba de
Tramtai (nunca la del usuario ni su puerto 17373). Comprueba:
- `/anthropic/v1/messages` en vivo, aceptando la cabecera `anthropic-beta`;
- `/openai/v1/responses` en vivo;
- que los nombres `agente/modelo` llegan enteros;
- que `apiKeyHelper` y `auth.command` funcionan.

Lo que no encaje se arregla en la ruta de Tramtai, dentro de la versión. Continue y opencode se comprueban con su archivo
y una petición real.

### Pruebas

- **En Go,** un escritor por formato, con archivos de ejemplo. Comprueban que:
  - se conservan los comentarios;
  - se deshace con los valores previos;
  - se respetan los cambios del usuario hechos después de activar;
  - se rechaza un archivo roto sin tocarlo;
  - en un proyecto nunca se escribe `.claude/settings.json`;
  - las copias y los archivos con clave quedan con permisos solo para el usuario.
- **Servicio:** las rutas de la API para la Web UI y la terminal.
- **Paridad:** las pruebas de paridad de la terminal y de la Web UI.

### Ayuda

Un tema nuevo, «Conectar tus herramientas»:
- qué toca en cada una;
- dónde está la copia y cómo deshacer;
- por qué la clave de Continue y opencode va en su archivo;
- qué pasa con los equipos que sincronizan su configuración.

En el sitio, lo de la sección 3.

## El sitio: decisiones

### 1. Estructura de la portada

**Diez secciones**, en este orden (hoy son 14):

| # | Sección | Qué cambia |
|---|---|---|
| 1 | Portada | El mapa con las escenas, como ahora. Debajo de los botones, tres accesos: Escritorio · Terminal · Servidor, cada uno enlazado a su sección. |
| 2 | Por qué Tramtai | El «Sin/Con Tramtai» de hoy, la franja de cifras y la línea de ventajas (sección 4). Sustituye a los seis motivos. |
| 3 | Cómo funciona | Las tres paradas y el código, con pestañas OpenAI · Anthropic · curl. El paso 3 dice que Tramtai apunta tus herramientas por ti (sección 3). |
| 4 | La app | Las cinco capturas, ya en es/en, y el recorrido guiado justo después. |
| 5 | La terminal | La sección nueva (sección 5). |
| 6 | Tu servidor | La sección nueva (sección 6). |
| 7 | Agentes | Las cinco tarjetas de hoy; absorbe la franja «Conecta estos agentes / Desde tus apps y SDK». |
| 8 | Funciones | El plano de metro de hoy, para quien quiera el detalle. |
| 9 | Privacidad y riesgos | El tablero de Privacidad y los cinco riesgos, juntos. |
| 10 | Descargar y cierre | Las tres tarjetas de descarga y el cierre. |

- **Barra superior:** Cómo funciona · Terminal · Servidor · Agentes · Ayuda · Desarrolladores. En el panel del menú (hasta
  1100 px), las mismas.
- **Se mantiene:** la dirección «Red de tranvía», el bilingüe con `<span lang>`, el glosario, el selector de tema, la
  regla de mayúsculas solo en el tablero y todo lo que comprueba `npm test`.

### 2. Idioma coherente

Todo lo que se ve cambia con el idioma elegido: el texto, las imágenes y los comandos.
- **Capturas de la app:** las cinco de «La app» pasan a tener versión en inglés
  (`<pantalla>-<claro|oscuro>-<es|en>-<ancho>.webp`), como ya tienen la bienvenida y la terminal. Las del servidor nacen
  en los dos idiomas.
- **Comandos:** cada bloque de código enseña la orden en el idioma elegido, con `<span lang>`. En inglés se usan los alias
  en inglés de la CLI.
- **Alias que faltan:** se añaden en la CLI los alias en inglés de toda orden que salga en el sitio y no lo tenga. Hoy
  falta al menos `servicio iniciar` (`service start`); el plan saca la lista completa. Es un adelanto de la versión
  «Idiomas» de `pendientes.md`: allí siguen los alias del resto de órdenes y los ejemplos de la Ayuda y las guías.
- **Prueba nueva (Go, `cmd/tramtai`):** lee los bloques de código de `site/index.html` y `site/servidor.html`, toma cada
  línea que empieza por `tramtai`, `sudo tramtai`, `tramtai-servidor` o `.\tramtai.exe`, y comprueba con el árbol de
  órdenes de cobra que la orden existe en su idioma. Si alguien cambia o quita una orden, la prueba falla.
- **`servidor.html` en inglés:** hoy enseña `tramtai-servidor install` y `.\tramtai.exe server install`. Con los alias
  existen, pero la página tiene que usar la misma forma en todos sus bloques en inglés: se repasa entera.

### 3. Lo que el sitio cuenta de las herramientas

- En «Cómo funciona», el paso 3 («Apunta tu aplicación») añade que Tramtai apunta por ti Claude Code, Codex, Continue y
  opencode/Kilo, guarda antes una copia de su configuración y se desactiva con un interruptor.
- En la línea de ventajas, la parada «Sin tocar tu código» lo menciona en su frase.
- «La app» se queda en cinco capturas; la pestaña Herramientas se enseña con su captura en el tema de Ayuda «Conectar
  tus herramientas».
- El plan pone estas tareas después de las de herramientas.

### 4. Las ventajas: franja de cifras y línea de seis paradas

- **Franja de cuatro cifras**, número grande en Overpass tabular y etiqueta pequeña:
  - **4** agentes desde tus apps;
  - **3** API: OpenAI, Anthropic y la propia;
  - **0** telemetría y ningún servidor nuestro;
  - **1** cambio en tu app: la URL.

  Son cifras del producto, verificables en el propio sitio. No hay cifras de descargas ni testimonios: no se inventan.
- **Línea de seis paradas** con el trazo y las estaciones del sitio (acento, no colores de agente):
  1. **Usa lo que ya pagas:** tu clave de API o, solo para tu uso personal, tu suscripción de Claude, ChatGPT o GitHub
     Copilot. Sin cuentas nuevas.
  2. **Sin tocar tu código:** si tu app usa el SDK de OpenAI o el de Anthropic, cambias la URL. Y a Claude Code, Codex,
     Continue y opencode los apunta Tramtai por ti.
  3. **Si un agente falla, sigue otro:** llega a su límite y la petición continúa con el siguiente de tu lista.
  4. **Todo queda en tu equipo:** sin servidor nuestro ni telemetría; tus claves, cifradas en el almacén del sistema.
  5. **Cada app, con su clave y su límite:** presupuesto diario o mensual por app y cada petición a la vista.
  6. **Escritorio, terminal o servidor:** la misma Tramtai en la app, por SSH o siempre encendida en tu servidor.
- **Disposición:** en pantalla ancha, dos líneas verticales de tres paradas, lado a lado; en móvil, una sola línea de
  seis. Las frases no pasan de unas 25 palabras y no prometen de más, como pidió la auditoría del 2026-10-22.

### 5. La terminal: texto real en una ventana con pestañas

- **Cómo se saca:**
  - `capturas.mjs --terminal` deja de guardar `.webp`. Guarda la pantalla como HTML, una por página e idioma, en un
    archivo que lee el generador del sitio.
  - `ansiAHtml` pasa de escribir colores fijos a escribir clases que apuntan a variables del sitio (`--t-acento`,
    `--t-ok`, `--t-error`…). La TUI usa exactamente la paleta clara y oscura de `DESIGN.md`, así que cada color se
    traduce a su token y **un solo HTML por página e idioma** sigue el tema del sitio. Comprobado el 2026-10-31 con
    tres páginas en los dos temas: los 24 colores que salieron tienen token y cada página pesa unos 6 KB.
  - Un color que no esté en la tabla hace fallar la captura, para que no se cuele un color suelto.
- **Páginas:** Inicio, Mis APIs e Historial, con los datos de ejemplo de la copia portable (puerto 17388), a **100×28**
  (a 112 columnas no cabe con letra legible en una ventana de unos 1000 px).
  - **Agentes no se enseña:** enseña los CLI reales del equipo que saca la captura (versiones, cuenta, «Sin conectar»).
  - **Corrección en `tui.Captura`:** hoy da por asentada una vista con «Comprobando…» (solo espera a «Cargando…»), y
    por eso Inicio salió a medio cargar. Pasa a esperar también a `inicio.comprobando`; en la prueba del 2026-10-31,
    Inicio salió completo en 6 s. Si al acabar la espera queda «Comprobando…», la captura falla.
  - Se mantiene el bloqueo de rutas reales de hoy.
- **Fuente:** un subconjunto autoalojado de Cascadia Mono (OFL, como Overpass) con las letras y los símbolos que usa la
  TUI (`✓ ✗ ● ○ ▸ ▾ │ ·`, flechas y bloques). Con la fuente de reserva, `✓`/`✗` miden más que una letra y descuadran las
  columnas (visto en la prueba).
- **En la portada:**
  - Una ventana que **sigue el tema del sitio** (terminal clara de día, oscura de noche, con la paleta de cada tema de
    `DESIGN.md`), con tres puntos de semáforo, pestañas «1 Inicio · 4 Mis APIs · 5 Historial» y un rótulo «tramtai
    terminal · ssh».
  - Las pestañas cambian con el ratón y, con el foco en ellas, con las flechas y con las teclas de su número (1, 4, 5,
    como en la TUI). Fuera de la ventana las teclas no hacen nada, para no robarle atajos a la página. Siguen el patrón
    de pestañas accesibles de las escenas del mapa.
  - Sin JavaScript se ve Inicio y las otras páginas quedan en `<details>`.
  - En móvil, la pantalla se desplaza en horizontal dentro de su marco, sin escalar la letra por debajo de unos 11 px.
- **Alrededor:**
  - Título y una frase: las ocho páginas de la app, con el teclado, sin abrir la app y por SSH.
  - Pie de figura en texto: «la terminal en Inicio, con datos de ejemplo».
  - Una leyenda de teclas `<kbd>` (`Tab` menú, `1`–`7` páginas, `?` ayuda, `Esc` volver, `q` salir) en lugar de los dos
    párrafos. El resto de atajos se enlaza a la Ayuda («Solo con la terminal»).
  - El bloque de dos comandos: `tramtai servicio iniciar -d` y `tramtai terminal`, en el idioma elegido.
- **Se borran** las 16 imágenes `site/capturas/terminal-*.webp`, y su lista en `capturas-lista.ts` pasa a ser la de las
  páginas en HTML.

### 6. El servidor

**Sección en la portada:**
- Una frase que separa escritorio y servidor: «El escritorio es para ti; el servidor, para tu casa o tu equipo de
  trabajo, encendido aunque nadie inicie sesión».
- Un mapa de tranvía en SVG en línea: tus equipos (portátil, móvil, scripts) → tramo **HTTPS** en el acento → Tramtai en
  tu servidor → las líneas de los agentes. El texto del mapa va en `<tspan lang>`.
- Cuatro virtudes, cada una con una línea:
  - **Arranca con el equipo:** sin sesión iniciada.
  - **Solo por HTTPS:** sin él no arranca.
  - **Contraseña y TOTP:** segundo factor opcional.
  - **Detrás de tu proxy:** con ejemplos para los más usados.
- Dos tarjetas: «El escritorio, si…» y «El servidor, si…».
- Pestañas Linux · Windows Server, con la orden de instalar y la de la contraseña en el idioma elegido.
- Un enlace a la guía y, en la misma línea: «Es otro entorno: tus APIs y claves no pasan de uno a otro. Antes de
  instalar, lee los cinco riesgos».

**Cabecera nueva de `servidor.html`:**
- Un titular de lo que ganas, una entrada de unas 30 palabras y el mapa de la portada en grande.
- «¿Es para ti?» con las dos tarjetas.
- Tres pasos numerados (instalar, contraseña, HTTPS o proxy), en pestañas Linux · Windows Server.
- **Dos capturas reales:** la página de entrada con contraseña y TOTP, e Inicio con la cabecera «Servidor».

**Debajo, la documentación de hoy**, sin cambios de fondo:
- El Aviso de los cinco riesgos se mueve justo antes de «Instalar», completo y con su enlace a los términos: sigue
  delante de cualquier instrucción de instalación.
- La tabla «Dos entornos» se queda donde está.

**Capturas del servidor:**
- Salen del modo `capturas.mjs --servidor`, que ya existe y necesita Docker.
- Se añaden versiones para el sitio (`site/capturas/servidor-<entrar|inicio>-<claro|oscuro>-<es|en>-<ancho>.webp`) con
  las mismas protecciones de hoy.

**No se anuncia** una imagen Docker de Tramtai: no existe. Docker solo se usa para probar los proxies
(`scripts/servidor/docker-compose.proxies.yml`).

### 7. Calidad

- **Pruebas:**
  - `npm test` (`portada.test.ts`, `sitio.test.ts` y `estilo-textos.test.ts`) se pone al día con las secciones nuevas, el
    glosario y el bilingüe.
  - Las pruebas de `ansiAHtml` cubren la tabla de colores, un color desconocido y el escape de HTML.
  - La prueba de comandos de la sección 2.
- **Auditoría del sitio terminado:**
  - impeccable y antislop;
  - accesibilidad: pestañas con teclado, `aria`, contraste en los dos temas y `prefers-reduced-motion`;
  - móvil a 375 px;
  - los tres avisos de «cramped padding» de la caja `.aviso` de `servidor.html`: se revisan y se arreglan o se anotan
    como falso positivo, con su motivo.
- **Al día en esta versión:**
  - `DESIGN.md`: la ventana de terminal y la línea de ventajas como piezas del sitio.
  - La guía de desarrolladores (`docs/equipo/desarrollo.md`): cómo se generan las pantallas de la terminal en HTML.
  - El README, si enseña algo que cambie.
  - La Ayuda («Solo con la terminal», «Modo servidor»), si un texto deja de coincidir.
- **Capturas completas:** hacen dos peticiones reales a `claude/haiku` con la suscripción del usuario, como ahora. Se le
  avisa antes de sacarlas. Nunca se usa el Tramtai instalado del usuario ni su puerto 17373.

## Fuera de alcance

- Una imagen Docker de Tramtai: va como versión propia más adelante (`pendientes.md`), con su diseño (qué trae la imagen,
  cómo se añaden los CLI de cada agente sin redistribuirlos y cómo se aceptan los términos y se pone la contraseña sin
  consola).
- Preparar los CLI de agentes (instalarlos, iniciar sesión, dejar sus ajustes): es la v0.60.
- Vídeos o GIF de la terminal (VHS) y SVG generados con `freeze`: se descartaron frente al HTML real.
- Testimonios o cifras de uso.
- En la parte del sitio, cambios en la Web UI o en la terminal, salvo los alias de la CLI (los de las herramientas van
  en su sección).

## Criterios de aceptación

**Herramientas:**
- Con Claude Code y Codex reales, activar desde la app, la terminal o la CLI deja la herramienta hablando con Tramtai sin
  ninguna clave en su archivo. Desactivar la devuelve a como estaba, respetando lo que el usuario cambió después.
- Continue y opencode/Kilo quedan apuntados, con la clave en su archivo y permisos solo para el usuario.
- «Activar todas» y «Desactivar todas» hacen lo mismo con todas las detectadas.
- Siempre queda una copia del archivo antes del primer cambio, y «Restaurar» la repone.
- Desde un proyecto, solo se escribe `.claude/settings.local.json`.

**El sitio:**

- La portada tiene las diez secciones en el orden de la sección 1 y la barra superior enlaza a Terminal y Servidor.
- La terminal de la portada es texto seleccionable en una ventana con tres pestañas, sigue el tema y el idioma sin
  recargar, no muestra «Comprobando…» y sus columnas cuadran. No queda ninguna imagen `terminal-*.webp`.
- La sección del servidor y la cabecera de `servidor.html` tienen el mapa, las dos tarjetas, los pasos por sistema y las
  capturas reales en es/en × claro/oscuro. Los cinco riesgos siguen completos antes de «Instalar».
- Las seis paradas y las cuatro cifras sustituyen a los seis motivos.
- Al cambiar a inglés, cambian todas las capturas y todos los comandos, y la prueba de comandos pasa.
- `npm test`, `go test ./...` y la auditoría pasan, y el sitio se ve bien a 375 px y a 1440 px en los dos temas.
