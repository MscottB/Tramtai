# Espacios de trabajo (v0.54)

Diseño del 2026-10-20, acordado con el usuario por secciones. Hoy:

- **Espacios de trabajo** (en el código, «instancias»): la global y una por proyecto (`internal/servicio/instancias.go`).
  El id de un proyecto sale de su ruta y de `proyecto.nombre`; `datos/instancias.json` solo recuerda los activos.
- **Abrir un proyecto desde la app** (`POST /ui/proyectos`) crea `.tramtai/`, pero crear una API o un presupuesto en él
  falla: `puntos_acceso` y `presupuesto` son claves administradas y `local.yaml` solo las acepta si la carpeta está en
  `proyectos_confiables`, que nadie rellena.
- **No hay gestión:** un proyecto detenido desaparece del selector; desde la app no se puede detener, borrar ni
  archivar; sus datos (`datos/instancias/<id>.db`, `datos/tokens/<id>`, `datos/cargas/<id>`) quedan para siempre.
- **Mover o renombrar** un proyecto crea otro espacio (token y historial nuevos); si la carpeta falta al arrancar, solo
  queda una línea en `servicio.log`.
- **Ajustes** mezcla lo del espacio y lo del servicio sin decirlo; no se ve qué hereda un proyecto de la global.
- **Rarezas:** tras reiniciar se vuelve a la global sin aviso; otras pestañas fallan al guardar tras cambiar de espacio;
  la CLI manda a la global sin avisar si el proyecto está detenido; la app dice «espacio» y la CLI «instancia».
- **Agentes** (`web/src/paginas/Agentes.vue`): «Cambiar a clave» solo aparece si el agente está listo o si eligió
  suscripción sin sesión; Claude no comprueba la sesión de su CLI; no se puede desconectar un agente.
- **La «API principal»** (la clave de cada espacio) se presenta como «Sirve para todas tus aplicaciones de este equipo»,
  y no se entiende que es una por espacio ni en qué se diferencia de las APIs por aplicación.
- **La web** cuenta lo de hace varias versiones y «Por qué Tramtai» son cuatro motivos genéricos.

Decisiones del usuario:
- La v0.54 son los **espacios de trabajo** y, dentro, la **conexión de los agentes**, las **claves claras** y la
  **mejora general de la web** (contenido, diseño visual, navegación y calidad técnica).
- Los recorridos guiados con driver.js pasan a la **v0.55**; lo que seguía se corre un número (v0.56–v0.59).
- **Exportar e importar APIs** queda para una release posterior, sin número, en `pendientes.md`, junto a la idea de la
  versión Pro de equipo.
- Confianza: la **mezcla** (sección 1). Identidad: **registro + marca**. Gestión en una **página «Espacios»**.
- La «API principal» pasa a llamarse **«Clave del espacio»**.

Éxito: quien abre un proyecto desde la app puede usarlo entero (APIs, presupuesto) sin editar archivos; ve todos sus
espacios, los detiene, los vuelve a abrir o los olvida desde una página; puede mover o renombrar un proyecto sin perder
nada; sabe en Ajustes qué es de su espacio y qué de todo Tramtai; si cierra sesión o cambia de cuenta en el CLI de un
agente, Tramtai se lo dice y le deja volver a la clave o desconectarlo; entiende qué clave usar; y la web cuenta con
claridad qué es Tramtai hoy.

## 1. Espacios: identidad, confianza y la página «Espacios»

**Registro** `datos/espacios.json` (reemplaza a `datos/instancias.json`, que se migra al arrancar y se borra después):
por espacio `id`, `ambito` (`global`/`proyecto`), `ruta`, `nombre`, `activo`, `ultimo_uso` (RFC 3339) y
`ultimo_error` (texto, vacío si no hay). La global está siempre. Escritura atómica (temporal + renombrar), como la
configuración.

- **Id fijo.** Los espacios que ya existen conservan el id de hoy (no cambia el token de las apps ni se pierde
  historial). Uno nuevo lo calcula como hoy al crearse y desde entonces no cambia.
- **Nombre** que se muestra: `proyecto.nombre` de `config.yaml`, o el de la carpeta si no tiene.
- **Marca** `.tramtai/espacio`: una línea con el id, no versionada (Tramtai añade `espacio` al `.tramtai/.gitignore`,
  también en los proyectos que ya existen, al activarlos).
  - Al abrir o restaurar una carpeta cuya marca tiene un id registrado con **otra ruta**: si la ruta vieja ya no tiene
    `.tramtai/config.yaml`, se **movió**: el espacio actualiza su ruta y lo conserva todo. Si la ruta vieja sigue
    siendo un proyecto con esa marca, es una **copia**: la copia es un espacio nuevo y se reescribe su marca.
  - Sin marca (proyectos de antes de la v0.54), se reconoce por el id de hoy y se le escribe.
- **Datos sueltos** de versiones anteriores (una base en `datos/instancias/` sin entrada en el registro) aparecen como
  espacios «No encontrado» con su id como nombre, para reasociarlos u olvidarlos.

**Confianza** (la mezcla):
- Si la app **crea** `.tramtai/` (no existía), el proyecto queda confiado: se añade a `proyectos_confiables`.
- Si la carpeta **ya traía** `.tramtai/`, la app pregunta «¿Confías en esta carpeta?» y antes muestra las claves
  administradas que encontró en su `local.yaml` (por ejemplo `comandos_agentes`, `adaptadores`). Si no, se abre igual,
  sin APIs ni presupuesto propios, y la app dice por qué y cómo confiar después.
- La confianza vive en `proyectos_confiables` de la configuración global, nunca en el repositorio; «Dejar de confiar»
  la quita.
- `local.yaml` mantiene su nombre: vive en `.tramtai/`, no choca con otras herramientas y sigue la costumbre de
  `.env.local` (lo de esta máquina, no versionado).

**Estados de un espacio:** **Activo**, **Detenido**, **No encontrado** (falta la carpeta o su `config.yaml`) y, en
proyectos, **Sin confiar** (se combina con los otros).

**Página «Espacios»** (nueva en el menú, ruta `espacios`): una tarjeta por espacio con nombre, ámbito, ruta, estado,
confianza, número de APIs y de peticiones, y último uso. Acciones según el estado:
- Activo: **Abrir** (cambiar a él), **Detener**. Detenido: **Activar**. No encontrado: **Buscar la carpeta**
  (reasocia: valida que sea un proyecto y conserva id, token e historial) y **Olvidar**.
- Proyecto: **Confiar** / **Dejar de confiar**.
- **Olvidar** (no la global): pide escribir su nombre; detiene el espacio y borra **sus datos en Tramtai** (base,
  cargas, token y tokens de sus APIs) y su entrada del registro; **nunca toca la carpeta del proyecto**.
- **Abrir un proyecto…** también desde aquí.
- Un espacio que no se pudo restaurar al arrancar muestra `ultimo_error` en su tarjeta, y Inicio avisa con un enlace a
  «Espacios».

El selector de arriba sigue para cambiar rápido entre los activos y termina con **«Gestionar espacios»**.

**Servicio** (rutas de la Web UI, solo sesión local y CSRF): `GET /ui/espacios` (lista con estados y cuentas),
`POST /ui/espacios/{id}/activar`, `POST /ui/espacios/{id}/detener`, `POST /ui/espacios/{id}/confiar` (cuerpo
`{confiar: bool}`), `POST /ui/espacios/{id}/ubicar` (cuerpo `{ruta}`), `DELETE /ui/espacios/{id}` (cuerpo
`{nombre}` para confirmar). `POST /ui/proyectos` responde, si la carpeta ya traía `.tramtai/`, `409
confianza_requerida` con las claves administradas encontradas; la app pregunta y repite con `{ruta, confiar}`. Las
rutas de administración (`/api/v1/servicio/instancias…`) siguen y usan el mismo registro.

## 2. Ajustes claros y coherencia

**Ajustes en dos pestañas:**
- **Este espacio:** modelo predeterminado, historial y retención, presupuesto del espacio (también en proyectos
  confiados), borrar datos y valores efectivos.
- **Todo Tramtai:** puerto, conversor, actualizaciones y arranque con el sistema.

**Herencia a la vista:** en un proyecto, cada valor del espacio dice **«Propio»** o **«De Global»**; los propios tienen
**«Volver al de Global»** (quita la clave de `local.yaml`). En la global, un aviso: lo que cambies vale también para los
proyectos sin valor propio. El texto del presupuesto aclara que el de la global se cuenta **en cada espacio por
separado**. `GET /api/v1/instancia/configuracion` ya da el origen de cada valor; la app lo usa.

**Coherencia:**
- **Recordar el espacio:** el registro guarda el último usado (`ultimo_uso`); tras reiniciar el servicio, una sesión
  local nueva empieza en él si está activo (si no, en la global). La app de escritorio deja de reactivar la global si
  se detuvo a propósito (solo la activa si no hay ningún espacio activo).
- **Varias pestañas:** al cambiar de espacio, la pestaña avisa a las demás (`BroadcastChannel`) y estas se recargan
  con «Cambiaste a {nombre} en otra pestaña». Si una pestaña recibe un 403 por CSRF viejo, renueva la sesión y recarga
  con el mismo aviso.
- **CLI:** dentro de un proyecto detenido, `tramtai ejecutar` y `tramtai trabajos` fallan con «El espacio {nombre}
  está detenido: tramtai espacio iniciar» (código de salida distinto de cero) en vez de usar la global; `--global` la
  usa a propósito.
- **Abrir dentro de otro proyecto:** si la carpeta elegida está dentro de un proyecto, la app pregunta si abrir ese o
  crear uno nuevo en la carpeta elegida (`POST /ui/proyectos` con `{ruta, crear_aqui}`).
- **Un solo nombre, «espacio de trabajo»,** en app, CLI, terminal (TUI), bandeja, errores y documentación. CLI nueva:
  `tramtai espacios` (lista) y `tramtai espacio iniciar|detener|olvidar [ruta|id]`; `tramtai global|proyecto …`
  siguen como alias. Las rutas `/api/v1/instancia…` y el campo `instancia` no cambian (contrato); la documentación
  aclara que «instancia» es el espacio de trabajo.
- **Limpieza:** `activarGlobalUI` y `abrirProyectoUI` actualizan el descriptor `servicio.json`; fuera `app.proyectoAntes`,
  `app.proyectoDespues`, la clave sin uso `instancia.ambito` y el `{ambito}` que envía `activarGlobal`.

## 3. Agentes: suscripción, sesión del CLI y desconectar

**«Dejar de usar la suscripción» en cualquier estado:** si la ficha del agente admite las dos formas, el botón está
en todas las situaciones de su tarjeta (lista, sin sesión, con cualquier error). Al pasar a clave sin tenerla, la
tarjeta muestra cómo ponerla.

**Detección de cambios en el CLI:**
- **Activa** (los agentes que lo permitan): la ficha define una comprobación que **no gasta mensajes** (el comando de
  estado del CLI o la lectura de su archivo de sesión) y devuelve `sin_sesion`, `usa_clave_api` (el CLI pasó a clave)
  o `ok` con la **cuenta** si el CLI la informa. Se comprueba al abrir Agentes, cada 5 minutos en segundo plano (solo
  agentes con suscripción) y con **«Comprobar ahora»**. Cada caso tiene su texto y su acción: iniciar sesión (con el
  comando), «Dejar de usar la suscripción», o la cuenta actual a la vista («Suscripción de ana@…», solo en este
  equipo).
- **Pasiva** (todos): si una petición real falla por autenticación (`autenticacion_agente`), el agente queda con «La
  sesión del CLI no responde» en su tarjeta y en el mapa hasta que una petición vuelve a funcionar o se comprueba.
- **Evidencia M0:** antes de implementar, cada comprobación se verifica contra el CLI real de Claude Code, Codex,
  GitHub Copilot y Antigravity (comandos, salidas, archivos), y se anota en `docs/equipo/m0-agentes.md`. Si un CLI no
  ofrece nada fiable que no gaste mensajes, se queda con la detección pasiva y la Ayuda lo dice.

**Desconectar un agente** (ajuste de todo Tramtai, `adaptadores.<agente>.desconectado: true`, en marcha como la
autenticación):
- App: la tarjeta dice «Desconectado» con **«Conectar»**; en el mapa su línea va gris con «Desconectado»; no se ofrece
  en Probar ni en los selectores de modelo.
- Apps: una petición a ese agente recibe `agente_desconectado` (409) con qué hacer; el respaldo lo salta; Mis APIs avisa
  si una API solo permite ese agente.
- CLI: `tramtai agente desconectar|conectar <agente>`. API: `/api/v1/agentes` y `/api/v1/capacidades` informan
  `desconectado`.

## 4. Claves claras: la clave del espacio y las de cada app

- La «API principal» se llama **«Clave del espacio»** en app, Ayuda, web y documentación, con el espacio a la vista
  («Clave del espacio Global», «Clave del espacio mi-proyecto»). El historial nombra el origen igual.
- **Mis APIs** empieza con una explicación corta de las dos clases:
  - **Clave del espacio:** una por espacio, en la dirección del servicio; puede usar cualquier agente y modelo, sin
    límites propios; la usan la CLI y tus scripts.
  - **APIs por aplicación:** las que quieras, cada una con su dirección y su clave, sus agentes, modelos, instrucciones
    y presupuesto, y su historial; si una se filtra, se revoca solo esa.
- El texto «Sirve para todas tus aplicaciones de este equipo» se reemplaza por uno correcto («Una por espacio de
  trabajo, con acceso a todo: úsala para probar, la CLI y tus scripts; da a cada app su propia API»).
- **Aviso suave:** si en las últimas 24 h más de 20 peticiones con origen `app` llegaron con la clave del espacio,
  Inicio y Mis APIs sugieren crearles su propia API. No obliga a nada.

## 5. La web

**Novedades de la v0.54** en la portada («Así se ve», con capturas nuevas de Espacios y Agentes), en la guía de
desarrolladores y en la Ayuda (sección 6).

**«Por qué Tramtai»:** de cuatro motivos genéricos a **seis concretos**, cada uno con su icono y una frase que se puede
comprobar:
- **Lo que ya pagas:** usa tu suscripción o tu clave de cada agente, y cambias cuando quieras.
- **Local:** escucha solo en tu equipo y no envía estadísticas.
- **Compatible:** OpenAI, Anthropic y la API propia, con streaming y herramientas.
- **Sin cortes:** si un agente llega a su límite, sigue el siguiente de tu lista.
- **Por proyecto:** cada espacio con su historial, sus claves y sus ajustes.
- **Bajo control:** una API por app con su presupuesto, y el mapa en vivo.

Encima, una franja **«Sin Tramtai / Con Tramtai»**: una clave por proveedor en cada app frente a una sola URL local que
usa tus agentes.

**Mejora general** (contenido, diseño visual, navegación y calidad técnica): la lista concreta sale de la auditoría del
sitio (anexo A). Criterios de aceptación:
- Ningún texto cuenta algo que la app ya no hace, ni deja fuera lo de v0.50–v0.54; es y en dicen lo mismo.
- Misma jerarquía, espaciados y componentes en todas las páginas; modo oscuro y móvil revisados página por página.
- Barra y pie iguales en todas las páginas, con «Descargar», «Empezar», «Ayuda» y «Desarrolladores» a un clic.
- Accesibilidad: textos alternativos, foco visible, contraste AA y landmarks en todas las páginas.
- Rendimiento: capturas en un formato ligero con su tamaño declarado y carga diferida fuera de la primera pantalla.
- Metadatos: `description`, Open Graph y tarjeta para compartir con imagen, `canonical`, `sitemap.xml` y `robots.txt`.

## 6. Ayuda, desarrolladores, documentación y pruebas

**Ayuda** (estilo v0.52: temas cortos y visuales, diagramas, pasos y capturas; `estilo.test.ts` vigente):
- Tema nuevo **«Espacios de trabajo»**: qué son, global y proyectos, qué se hereda, confiar en una carpeta, mover,
  renombrar u olvidar, «No encontrado».
- Tema nuevo **«Claves y APIs»**: clave del espacio frente a APIs por aplicación, qué pasa al regenerar cada una.
- **Agentes:** dejar de usar la suscripción, «la sesión del CLI cambió o no responde», desconectar un agente.
- **Ajustes:** las dos pestañas y la herencia.
- «¿Qué hago?» de los errores nuevos (`agente_desconectado`, `confianza_requerida`, espacio detenido).
- `npm run sitio` regenera `site/ayuda/`.

**Desarrolladores** (`site/desarrolladores.html`, `docs/desarrolladores.md`): qué clave usar; «instancia» es el
espacio de trabajo; `desconectado` y `agente_desconectado`; la CLI nueva.

**Documentación:** `docs/operacion.md`, `docs/arquitectura.md`, `docs/api.md`, `internal/api/openapi.json`,
`docs/equipo/m0-agentes.md` y `docs/equipo/pendientes.md` (v0.54 ✅ al publicar; v0.55 recorridos guiados con
driver.js; v0.56–v0.59 lo que seguía; exportar e importar APIs, sin número, con la nota de la Pro de equipo).

**Pruebas:**
- Go: migración `instancias.json` → `espacios.json`; id fijo al renombrar; mover y copiar con la marca; datos sueltos
  como «No encontrado»; confianza al crear y `409 confianza_requerida` al abrir uno existente; **olvidar borra los
  datos de Tramtai y nunca la carpeta del proyecto**; restaurar con la carpeta borrada deja `ultimo_error`; la CLI no
  cae en la global; detección de sesión con CLIs simulados (cada resultado); fallo de autenticación → estado pasivo y
  vuelta a `ok`; agente desconectado: 409, el respaldo lo salta; recordar el último espacio.
- Web: lógica pura (estados y acciones de un espacio, herencia de Ajustes, estado de un agente), Ayuda (`estilo.test`),
  portada (`portada.test`).
- Navegador, de punta a punta: abrir un proyecto nuevo y uno clonado (pregunta de confianza), crear una API en él,
  moverlo, detenerlo, olvidarlo; dos pestañas; cerrar sesión en un CLI real y ver el aviso; desconectar y conectar un
  agente; la web en móvil y escritorio, claro y oscuro.

**Fuera de la v0.54:** exportar e importar APIs; sincronización de equipo (Pro); cambiar el nombre de las rutas
`/api/v1/instancia…`; borrar la carpeta `.tramtai/` de un proyecto.

## Anexo A. Auditoría del sitio

Auditoría del 2026-10-20 (lectura de `site/` y `web/scripts/sitio.ts`; la Ayuda se genera, sus cambios van en el
generador). Todo entra en la v0.54; se marca la prioridad para ordenar el plan.

**Contenido y textos**
- [A] Capturas de la portada y de la Ayuda de Inicio anteriores al mapa vivo: rehacerlas (y las nuevas de Espacios y
  Agentes).
- [A] La portada no cuenta el mapa en vivo (v0.53) ni la Ayuda renovada (v0.52: «?» en cada página, ayuda junto a cada
  error, la misma Ayuda en la web).
- [A] Antigravity en el titular como «API» cuando desde apps no está disponible: matizarlo («por ahora desde Probar y
  la CLI»); fila en la tabla de herramientas de desarrolladores.
- [A] La guía de desarrolladores no menciona `GET /api/v1/openapi.json`: sección corta «Importar en Postman o generar
  un cliente».
- [M] «Con streaming y herramientas» es inexacto (con herramientas no hay streaming): «streaming; herramientas en las
  tres APIs».
- [M] El conversor sin conexión no dice para qué sirve («si tu red bloquea la descarga de markitdown»).
- [M] Mensajes repetidos en la portada («Una API por aplicación» y «De la prueba al código» dos veces; «Por qué» repite
  Funciones y Privacidad): fusionar o retitular.
- [M] Textos largos o técnicos (Privacidad «Claves», «un hook las niega todas», «vive en su propio módulo», el párrafo
  de eventos): partir y simplificar.
- [M] Un término por cosa: «Gemini API», «GitHub Copilot», «respuesta en vivo (streaming)», «la app» en vez de «Web
  UI» en español, «pasarela» frente a «puente» frente a «intercambiador»; y el mismo modelo de ejemplo en portada,
  desarrolladores y Ayuda.
- [B] Comillas inglesas en el texto en inglés; el ancla `agentes-herramientas` también en la tabla en inglés; «uso no
  comercial» igual en la portada y la nota; la versión escrita al publicar (no solo si responde la API de GitHub).

**Diseño visual**
- [A] En oscuro, `--tablero` casi igual que `--fondo`: borde en `.tablero` y `.codigo`.
- [M] Comandos de Homebrew en un `pre.codigo` (con Copiar) en vez de `<br><code>` que se corta en el móvil.
- [M] La Ayuda sin índice lateral: `h2` por bloque y un «En esta página» generado.
- [M] Unificar `.etiqueta` y `.grupo-funciones`; iconos de tema en SVG (los glifos ◐ ☀ ☾ salen como emoji).
- [B] `@media` y variables de tema duplicadas en `estilo.css`; el color fijo de `.codigo .k` a variable; el diagrama
  `.ciclo` en una columna desde 900 px.

**Navegación**
- [A] Entre 641 y 1100 px la barra pierde la navegación sin reemplazo: menú desplegable accesible.
- [A] Desarrolladores no se alcanza en el móvil: tercera tarjeta en el cierre de la portada.
- [A] El pie no enlaza a nada: Descargar, Ayuda, Desarrolladores, GitHub/Releases, Licencia y Términos, en todas las
  páginas.
- [M] «Cómo empezar» y «Lee la ayuda» a `ayuda/empezar.html`; botón de descarga compacto en el móvil; acceso al
  buscador desde cada tema de la Ayuda; `SHA256SUMS.txt` enlazado.
- [B] La misma barra (5 entradas) en portada, desarrolladores y Ayuda; claves distintas en `localStorage` para el
  lenguaje elegido en la guía y en la Ayuda.

**Calidad técnica**
- [A] Enlace «Saltar al contenido» en todas las páginas.
- [A] El idioma se aplica al final del body (parpadeo en inglés): aplicarlo en el `<head>`, como el tema, y traducir
  `document.title`.
- [A] `.doc table { display: block }` rompe la semántica: envolver las tablas en un contenedor con desplazamiento.
- [A] Capturas de la portada en PNG (~936 KB): WebP con `srcset` 960/1440, tamaño declarado y carga diferida.
- [A] `og:image` relativa: `og.png` 1200×630 absoluta, `og:url`, `og:type`, `og:site_name`, `og:locale` (+
  alternativa en inglés), `twitter:card`, y `canonical` absoluto en todas las páginas (también la Ayuda).
- [M] Bordes de campos, pestañas y ES/EN con contraste ≥ 3:1; zonas táctiles ≥ 24 px (puntos del recorrido, ES/EN,
  tema); `aria-label` y `placeholder` en el idioma de la página, no los dos a la vez.
- [M] Imágenes de la Ayuda con `width`/`height`; `logo` pequeño para la barra, `favicon.svg`/32 px y
  `apple-touch-icon` en vez del PNG de 1024 px.
- [M] `sitemap.xml` (portada, desarrolladores y temas de la Ayuda) y `404.html` bilingüe con enlaces útiles. (`robots.txt`
  no aplica: GitHub Pages de proyecto no lo sirve en la raíz del dominio.)
- [B] Precargar `overpass.woff2` y `sitio.js` con `defer` en el `<head>`; títulos reales (`h3`/`h4`) en vez de
  `role="heading"`; JSON-LD `SoftwareApplication`; `manifest.webmanifest` mínimo.
