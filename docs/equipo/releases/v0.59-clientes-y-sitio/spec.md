# v0.59: clientes configurados solos y sitio nuevo

La v0.59 junta dos cosas, por decisión del usuario (2026-10-31):
1. **Configurar los clientes solos** (Claude Code, Codex, Cline…), con copia de su configuración antes de tocarla, como
   en `pendientes.md`.
2. **El sitio nuevo** (`site/`): la terminal rehecha, el modo servidor bien presentado y las ventajas remarcadas.

El sitio solo se publica al terminar Publicar, así que sale con la versión. Va en las **últimas tareas**: así sus capturas
y textos ya incluyen la configuración de clientes y se sacan una sola vez.

Este documento diseña la parte del sitio. La parte de clientes se diseña en su propia sesión de diseño y se añade aquí,
como sección «Clientes», antes de escribir el plan. Lo único que el sitio necesita saber de ella está en la sección 3.

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

## Decisiones

### 1. Estructura de la portada

**Diez secciones**, en este orden (hoy son 14):

| # | Sección | Qué cambia |
|---|---|---|
| 1 | Portada | El mapa con las escenas, como ahora. Debajo de los botones, tres accesos: Escritorio · Terminal · Servidor, cada uno enlazado a su sección. |
| 2 | Por qué Tramtai | El «Sin/Con Tramtai» de hoy, la franja de cifras y la línea de ventajas (sección 4). Sustituye a los seis motivos. |
| 3 | Cómo funciona | Las tres paradas y el código, con pestañas OpenAI · Anthropic · curl. El paso 3 dice que Tramtai configura tus clientes por ti (sección 3). |
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
  falta al menos `servicio iniciar` (`service start`); el plan saca la lista completa.
- **Prueba nueva (Go, `cmd/tramtai`):** lee los bloques de código de `site/index.html` y `site/servidor.html`, toma cada
  línea que empieza por `tramtai`, `sudo tramtai`, `tramtai-servidor` o `.\tramtai.exe`, y comprueba con el árbol de
  órdenes de cobra que la orden existe en su idioma. Si alguien cambia o quita una orden, la prueba falla.
- **`servidor.html` en inglés:** hoy enseña `tramtai-servidor install` y `.\tramtai.exe server install`. Con los alias
  existen, pero la página tiene que usar la misma forma en todos sus bloques en inglés: se repasa entera.

### 3. Lo que el sitio cuenta de los clientes

- En «Cómo funciona», el paso 3 («Apunta tu aplicación») añade que Tramtai puede configurar tus clientes (Claude Code,
  Codex, Cline…) por ti y guarda antes una copia de su configuración.
- En la línea de ventajas, la parada «Sin tocar tu código» lo menciona en su frase.
- Los nombres de clientes, la orden y la página de la app salen de la sección «Clientes» de este documento cuando exista;
  el plan pone estas tareas después de las de clientes.

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
  2. **Sin tocar tu código:** si tu app usa el SDK de OpenAI o el de Anthropic, cambias la URL. Y Tramtai configura tus
     clientes por ti.
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
    traduce a su token y **un solo HTML por página e idioma** sigue el tema del sitio.
  - Un color que no esté en la tabla hace fallar la captura, para que no se cuele un color suelto.
- **Páginas:** Inicio, Agentes, Mis APIs e Historial, con los datos de ejemplo de la copia portable (puerto 17388).
  - La captura **espera a que la página termine de cargar**: si queda «Comprobando…» en pantalla, falla.
  - Se mantiene el bloqueo de rutas reales de hoy.
- **En la portada:**
  - Una ventana oscura en los dos temas, como una terminal, con tres puntos de semáforo, pestañas «1 Inicio · 2 Agentes ·
    4 Mis APIs · 5 Historial» y un rótulo «tramtai terminal · ssh».
  - Las pestañas cambian con el ratón y, con el foco en ellas, con las flechas y con las teclas de su número (1, 2, 4,
    5, como en la TUI). Fuera de la ventana las teclas no hacen nada, para no robarle atajos a la página. Siguen el
    patrón de pestañas accesibles de las escenas del mapa.
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

- Una imagen Docker de Tramtai.
- Vídeos o GIF de la terminal (VHS) y SVG generados con `freeze`: se descartaron frente al HTML real.
- Testimonios o cifras de uso.
- En la parte del sitio, cambios en la Web UI o en la terminal, salvo los alias de la CLI (los de clientes van en su
  sección).

## Criterios de aceptación

- La portada tiene las diez secciones en el orden de la sección 1 y la barra superior enlaza a Terminal y Servidor.
- La terminal de la portada es texto seleccionable en una ventana con cuatro pestañas, sigue el tema y el idioma sin
  recargar y no muestra «Comprobando…». No queda ninguna imagen `terminal-*.webp`.
- La sección del servidor y la cabecera de `servidor.html` tienen el mapa, las dos tarjetas, los pasos por sistema y las
  capturas reales en es/en × claro/oscuro. Los cinco riesgos siguen completos antes de «Instalar».
- Las seis paradas y las cuatro cifras sustituyen a los seis motivos.
- Al cambiar a inglés, cambian todas las capturas y todos los comandos, y la prueba de comandos pasa.
- `npm test`, `go test ./...` y la auditoría pasan, y el sitio se ve bien a 375 px y a 1440 px en los dos temas.
