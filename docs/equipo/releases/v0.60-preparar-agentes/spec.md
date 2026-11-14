# v0.60: tus agentes, preparados por Tramtai

Tramtai instala, conecta y mantiene los CLI de los agentes que usa por debajo (Claude Code, Codex, GitHub Copilot y
Antigravity), desde la app, la Web UI, la terminal o la CLI, también en el modo servidor y sin abrir otra consola.

Decisiones del usuario (2026-11-12):
- **Para qué:** «Las tres cosas»: el primer uso sin fricción, mantener los CLI que ya hay y el servidor sin consola.
- **Ajustes de los CLI:** «Solo lo que Tramtai necesite»: se toca su configuración únicamente cuando es imprescindible,
  con copia antes. Lo demás sigue aislado por petición, como hoy.
- **Dónde quedan:** «Instalación normal»: el instalador oficial de cada CLI, por usuario; el CLI sirve también fuera de
  Tramtai.
- **Inicio de sesión:** «Dentro de la app»: Tramtai lanza el del propio CLI y enseña su enlace y su código.
- **Agentes:** Claude Code, Codex, GitHub Copilot y Antigravity. Gemini no tiene CLI (es la API) y no entra.
- **Servidor:** «Linux y Windows Server».
- **Enfoque:** A, lo hace el servicio como una tarea con estados (frente a hacerlo en cada pantalla o un mixto).
- **Añadido al alcance:** desinstalar, saber la última versión antes de actualizar e instalar los requisitos (Node), este
  último solo si el M0 confirma que algún CLI lo necesita.
- **Versiones comprobadas:** antes de cada acción automática se comprueba que la versión del CLI está comprobada con
  Tramtai; si no, esa acción queda desactivada para ese agente, avisando de que no es compatible, hasta que Tramtai lo
  compruebe.

## Por qué

Hoy, quien instala Tramtai y no tiene ningún CLI se encuentra en Agentes con «Instala su CLI y pulsa «Volver a
comprobar»» y un enlace, y después con «inicia sesión con `claude auth login`». Tiene que salir de Tramtai, abrir una
consola, buscar el instalador de cada fabricante y volver. En el modo servidor es peor: en Linux hay que instalar con
`sudo npm install -g` y entrar con `sudo -u <cuenta> …`; en Windows Server no hay manera de iniciar sesión con la cuenta
del servicio.

## Lo que hay hoy

- **Detección:** `adaptadores.ResolverPrograma` solo busca en el PATH o en `comandos_agentes`; un servicio en marcha no ve
  el PATH nuevo que deja un instalador. La versión sale de `<cli> --version`.
- **Sesión:** `ComprobarSesion` ya existe para Claude (`claude auth status --json`), Codex (`codex login status`) y
  Copilot (su `config.json`); Antigravity y Gemini van solo con clave. La sesión caducada se detecta como «sesión no
  responde».
- **Ajustes:** Tramtai no escribe nada en la configuración de los CLI: aísla cada petición (carpetas temporales,
  `--setting-sources ""`, HOME privado de Antigravity…). Eso no cambia.
- **Instalar algo:** el conversor (`internal/documentos/preparacion.go`) es el patrón que se reutiliza: un estado
  (`falta` → `preparando` → `listo`/`error`) con etapa y progreso, una sola preparación a la vez, plazo, y su barra en
  Ajustes y en la CLI.
- **Copia de lo que se toca:** `internal/apuntar` (v0.59): copia del archivo entero antes del primer cambio, anotación de
  lo escrito, tres copias y Restaurar.
- **Servidor:** en Linux, `tramtai agentes entrar` lanza el inicio de sesión del CLI con la cuenta del servicio
  (`servidor.OrdenEntrar`); en Windows Server da un error que manda a la clave de API. La M0 de esos inicios de sesión
  sigue abierta (`docs/equipo/m0-agentes.md`).

## Qué entra

| Agente | Instalar | Iniciar sesión | Actualizar | Desinstalar |
|---|---|---|---|---|
| Claude Code | sí | sí (`claude auth login`) | sí | sí |
| Codex | sí | sí (código de dispositivo, `codex login --device-auth`) | sí | sí |
| GitHub Copilot | sí | sí (`copilot login --device-code`, código de GitHub) | sí | sí |
| Antigravity | sí | no: solo clave de API (la sesión personal no se usa) | sí | sí |

Los argumentos de inicio de sesión los fijó el M0 (`m0.md`):
- **Codex** inicia sesión siempre por código de dispositivo, en el escritorio y en el servidor; la tarjeta y la Ayuda avisan
  de que hay que activarlo en ChatGPT › Seguridad, con su enlace (Ruling 8).
- **Copilot** se lanza con `COPILOT_DEBUG_BROWSER` puesto: si no, abre el navegador del sistema y copia el código por su
  cuenta (Ruling 9). Cada receta lleva las variables que su inicio de sesión necesita (Ruling 11).
- **Claude** imprime el enlace y pide pegar el código que da la página.

## Cómo funciona

### La receta de cada agente

Cada ficha (`internal/agentes/<nombre>`) suma una receta, con un valor por sistema (Windows, macOS, Linux):
- **Instalar:** la orden oficial, sin Node (el M0 vio que ninguno lo necesita). Claude Code, Codex y Antigravity, su
  instalador oficial (`install.ps1`/`install.sh`); Copilot, en macOS y Linux su instalador y en Windows su descarga
  directa comprobada (zip y `SHA256SUMS.txt`, x64 o arm64) en `%LOCALAPPDATA%\GitHubCopilotCLI`, sin winget ni
  administrador, que sirve también para la cuenta del servicio (Ruling 6).
- **Dónde queda:** la ruta donde deja el ejecutable (por ejemplo `~/.local/bin`), escrita con `~` y `/`; `~` es el
  perfil de la cuenta que corre el servicio (Ruling 14).
- **Fijar versión:** si el instalador admite elegir versión, cómo se le pasa (Claude, Codex y Copilot; Antigravity no).
- **Actualizar:** la orden del propio CLI (`claude update`, `codex update`, `copilot update`, `agy update`).
- **Desinstalar:** ninguno tiene orden propia: la receta dice qué rutas se borran (Ruling 16).
- **Última versión publicada:** Claude, su dirección `latest`; Codex y Copilot, sus versiones de GitHub (Codex con el
  prefijo `rust-v`, Ruling 15); Antigravity, el manifiesto que usa su instalador (Ruling 12).
- **Lector del inicio de sesión:** cómo se reconocen en su salida el enlace, el código y la petición de pegar algo. Ninguno
  necesita una terminal de verdad (pty): todos funcionan con tuberías.
- **Versiones comprobadas:** ver abajo.

Las órdenes están fijas en el código: nada de lo que escribe el usuario entra en ellas.

### La preparación

Un paquete nuevo, `internal/preparar`, ejecuta la receta como una tarea por agente:
- **Estados:** `instalando`, `actualizando`, `desinstalando`, `comprobando`, `esperando_usuario` (con enlace, código y si
  pide pegar), `listo`, `error` (con motivo y qué hacer) y `cancelado`.
- **Una por agente a la vez,** cancelable, con plazo (instalar y actualizar: 15 min; iniciar sesión: 15 min) y el árbol
  de procesos atado como en las peticiones (Job Objects en Windows, grupo de procesos en Unix).
- **Sigue aunque cierres la pantalla:** vive en el servicio; al volver, la pantalla ve dónde va. Si dos pantallas lanzan
  la misma a la vez, la segunda recibe `ya_en_curso` y ve la de la primera.
- **Al parar el servicio** se cancelan las preparaciones en marcha y se espera a que su árbol termine (Ruling 19); si el
  servicio se cae a mitad, al volver la preparación sale como `error` con `interrumpida`.
- **Mientras instala, actualiza o desinstala un agente,** el servicio lo trata como no disponible: las peticiones nuevas
  reciben el `agente_no_disponible` de siempre (Ruling 18). Y nunca a la vez que una petición en curso: se vuelve a
  contar con la reserva ya visible (Ruling 39).
- **Registro corto** de cada preparación (órdenes, últimas líneas, resultado), sin secretos. «Ver registro» enseña las 20
  últimas líneas, ya tapadas, que da el estado de la última preparación; el registro entero queda en
  `datos/agentes/preparaciones/<id>.log`, sin ruta en la API (Ruling 25).

### Instalar

1. **Versión:** se instala la versión comprobada más reciente cuando el instalador deja fijarla; si no, la que dé el
   instalador.
2. **Instalador oficial** con el usuario (o la cuenta del servicio), mostrando su salida resumida, con la entrada estándar
   cerrada (si aun así se queda esperando, `plazo_agotado` y se cierra su árbol).
3. **Encontrarlo:** en el PATH, en la ruta de la receta y, en Windows, en el PATH de usuario guardado en el registro. Si
   solo está fuera del PATH del servicio, se guarda su ruta en `comandos_agentes` (configuración de Tramtai, no del CLI).
4. **Versión** con `--version` y, salvo Antigravity, se sigue con el inicio de sesión. En Antigravity se pide la clave de
   API como hoy.

### Iniciar sesión

1. Se lanza el inicio de sesión del CLI y se lee su salida con el lector de la receta, sobre el texto acumulado y sin
   secuencias ANSI. Sin enlace ni código en 30 s, `salida_no_reconocida`.
2. Estado `esperando_usuario` con:
   - el enlace y el código de dispositivo (Codex, Copilot): se espera a que el usuario lo apruebe en su navegador; o
   - el enlace y un campo «Pega aquí el código» (Claude, si lo pide así): lo pegado se pasa al CLI por su entrada.
3. Cuando el CLI termina, `ComprobarSesion` lo confirma. Si el agente admite la suscripción y sigue con clave de API sin
   clave guardada (`falta_clave`), aún no está listo: el panel, la bienvenida, la terminal (tecla `a`) y la CLI («acepto»;
   si no, sale con 3) ofrecen «Usar mi suscripción» con su aviso de riesgo; nunca se cambia solo. En el servidor no se
   acepta ahí: tras `tramtai-servidor agentes entrar <id>`, la orden `tramtai-servidor agentes suscripcion <id>` (con
   `--personal` si está abierto a la red), cada una en su bloque copiable (Rulings 34 y 35).
4. Si se cancela o se agota el plazo, se cierra el proceso. Si el CLI termina con error («Login failed», «Bad Request»),
   `inicio_sesion_fallo` (Ruling 21). La sesión la guarda el CLI; Tramtai nunca lee ni guarda tokens.

### Versiones comprobadas

Antes de instalar, actualizar o iniciar sesión, Tramtai comprueba que la versión del CLI (la instalada, o la que se va a
instalar) está **comprobada**:
- **La lista** de versiones comprobadas de cada acción va en la receta, dentro de Tramtai. Se completa con una lista
  firmada (ed25519, la misma clave que las actualizaciones): un job semanal del CI instala la última versión de cada CLI,
  comprueba que su instalador y su inicio de sesión siguen como dice la receta (lanza el inicio de sesión, lee el enlace
  y el código y lo cancela, sin cuenta) y, si pasa, la añade y la sube como archivo de un release fijo de GitHub,
  `compatibilidad` (el sitio solo se publica con cada versión, así que no sirve). Así una versión nueva queda comprobada
  sin esperar a otra versión de Tramtai. Tramtai lee esa lista con la búsqueda diaria de versiones. El job lee las
  últimas versiones de GitHub con su `GITHUB_TOKEN` de solo lectura, que nunca pasa a instaladores ni CLI; solo un límite
  de la API (403/429) cuenta como «sin concluir» (la versión ni entra ni sale y no se avisa); un 404, un tag que no vale
  o un fallo de red se avisan (Rulings 32 y 33).
- **Autoactualización:** Claude, Copilot y Antigravity se siguen actualizando solos (no es un ajuste imprescindible). Si
  saltan a una versión sin comprobar, solo queda desactivada la acción automática de ese agente (Ruling 7).
- **Instaladores que no dejan elegir versión:** si la última versión publicada está comprobada, se instala sin más; si
  no, se avisa antes, como al actualizar, y si el usuario sigue, se instala y el inicio de sesión automático queda
  desactivado hasta que esa versión esté comprobada.
- **Si no está comprobada,** esa acción queda desactivada para ese agente y la tarjeta lo dice: «Claude Code 2.2.0 aún
  no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, puedes hacerlo en una
  consola con `claude auth login`». Se vuelve a habilitar sola cuando la versión entra en la lista (por la lista firmada
  o por una versión nueva de Tramtai).
- **Si el lector falla igualmente** (la salida no es la esperada en una versión comprobada), se cancela, se enseñan las
  últimas líneas tal cual con el enlace resaltado y la orden para la consola, y esa versión queda marcada como no
  compatible en este equipo (`datos/agentes/compatibilidad.json`) hasta que la lista la vuelva a confirmar. Nunca se
  queda colgado.
- **Antes de actualizar** a una versión que no está comprobada se avisa: «La versión nueva aún no está comprobada: si
  actualizas, el inicio de sesión automático quedará desactivado hasta que lo esté».
- **Seguir igualmente:** instalar, actualizar y desinstalar una versión sin comprobar se pueden confirmar (`confirmar` en
  la API, «Seguir» en las pantallas, `--confirmar` en la CLI); desinstalar también, porque Claude y Copilot se
  actualizan solos (Ruling 22). Iniciar sesión no se salta nunca: llega en el estado como `version_no_comprobada`.
- **El uso normal del agente no se bloquea:** las peticiones siguen funcionando con sus comprobaciones de siempre; lo
  que se desactiva es la preparación automática.

### Actualizar y versiones nuevas

- **Última versión publicada:** se consulta en la fuente oficial de cada CLI como mucho una vez al día y al pulsar
  «Volver a comprobar». La tarjeta dice «Hay una versión nueva: 2.1.300 (tienes 2.1.288)». No se envía nada que
  identifique al usuario; se puede desactivar en Ajustes. El servicio guarda el último resultado y lo sirve en el GET;
  solo con `?forzar=1` (o al arrancar y al cambiar de día, en segundo plano) vuelve a consultar, y las consultas
  simultáneas se agrupan en una.
- **Actualizar** ejecuta la orden de la receta y enseña la versión de antes y la de después. No actualiza si el agente
  tiene peticiones en curso: lo dice y espera.
- **Volver a iniciar sesión** aparece cuando la sesión caducó («sesión no responde»).

### Desinstalar

- Ningún CLI tiene orden oficial para desinstalar: se borran las rutas de la receta, siempre dentro de la casa de la
  cuenta y nunca la casa entera. Si entre la casa y la ruta hay un enlace o una junction, no se borra y se dice por qué
  (Ruling 20).
- Pide confirmación: el CLI también deja de estar para el usuario fuera de Tramtai.
- No borra la sesión ni los ajustes del CLI (`~/.claude`, `~/.codex`, `~/.copilot`…) salvo con la casilla «Borrar
  también su sesión y ajustes». Las líneas del PATH que dejaron los instaladores se quedan (no son de Tramtai).
- Espera si hay peticiones en curso con ese agente. Quita su ruta de `comandos_agentes` si la puso Tramtai.

### Requisitos

No se construye: el M0 vio que los cuatro CLI tienen instalador o descarga oficial sin Node (`m0.md`, Ruling 16). Sin
receta que pida Node, no hay `requisito_falta` ni instalación de Node; si un CLI futuro lo necesita, se añade entonces.

### Ajustes imprescindibles

Ninguno: el M0 vio que los cuatro se instalan e imprimen su enlace y su código sin tocar su configuración. Tramtai no
escribe nada en la de los CLI.

### Modo servidor

- El servicio ya corre con la cuenta del servicio, así que instala e inicia sesión para esa cuenta: «instalación
  normal» de esa cuenta, en su carpeta. El panel lo recuerda: «Para la cuenta del servicio (tramtai)».
- **Linux:** la Web UI remota y `tramtai agentes …` sustituyen a `sudo npm install -g` y `sudo -u …` en la guía.
- **Windows Server:** el M0 vio que, con la cuenta virtual del servicio, los cuatro se instalan, se actualizan y se
  desinstalan en `C:\Windows\ServiceProfiles\<servicio>`, y los tres con sesión imprimen enlace y código; que la sesión
  aprobada quede guardada se comprueba en la prueba final (Ruling 10). Esa cuenta no tiene consola: donde un mensaje
  daría la orden para hacerlo a mano, remite a la clave de API.
- **La orden para la consola** que dan los mensajes (por ejemplo, con una versión sin comprobar) es, en el servidor
  Linux, la de la cuenta del servicio: `sudo -u <cuenta> -H env -i HOME=… <cli> …`, conservando `TERM` y `LANG` si
  no necesitan comillas (Ruling 31). En el escritorio es la del CLI con su nombre si la consola lo encuentra; si no (el
  instalador de Claude y la descarga de Copilot en Windows no tocan el PATH), con su ruta completa, entre comillas si
  lleva espacios: `& "C:\Users\Ana María\.local\bin\claude.exe" auth login` en PowerShell (en cmd, sin el `&`).
- Solo el administrador, con su sesión (y TOTP si está activado), puede preparar agentes.

## API

Bajo la instancia, con la sesión de la Web UI o la clave del servicio; la clave del espacio no basta:

| Ruta | Qué hace |
|---|---|
| `GET /api/v1/agentes/<id>/preparacion` | Estado de la preparación (o `null`) |
| `POST /api/v1/agentes/<id>/preparacion` | `{"accion": "instalar"\|"actualizar"\|"iniciar_sesion"\|"desinstalar", "borrar_datos": false, "confirmar": false}` (202) |
| `POST /api/v1/agentes/<id>/preparacion/codigo` | `{"codigo": "…"}`: lo pegado, al CLI |
| `DELETE /api/v1/agentes/<id>/preparacion` | Cancela |
| `GET /api/v1/agentes/versiones` | Instalada, última publicada, comprobada o no, y acciones habilitadas por agente (`?forzar=1` vuelve a consultar) |

Con la clave del espacio (la de las apps), estas rutas responden 403 `no_autorizado`: con ella una app podría desinstalar
un CLI, borrar su sesión o colar su código de dispositivo (Rulings 23 y 24).

- Las pantallas siguen el estado con un sondeo corto mientras hay una preparación en marcha, como el conversor
  (`seguirPreparacion` en `web/src/conversor.ts`); el canal de eventos de la instancia solo lleva peticiones y no cambia.
- Errores con código: `version_no_comprobada`, `peticiones_en_curso`, `ya_en_curso`, `instalador_fallo`,
  `no_encontrado_tras_instalar`, `plazo_agotado`, `salida_no_reconocida`, `inicio_sesion_fallo` (el inicio de sesión del
  CLI terminó con error), `sin_red` e `interrumpida` (el servicio se reinició a mitad). `requisito_falta` no existe: no
  hay requisitos (Ruling 16), y `no_disponible_en_servidor` es solo de las herramientas: preparar no lo da (Ruling 38).
- **La orden a mano va aparte** en `orden_consola` (en el estado de la preparación y en las versiones) cuando el mensaje
  dice cómo iniciar sesión a mano; las pantallas arman la frase en su idioma (Ruling 37). Vacía ahí, la cuenta del
  servicio no tiene consola y queda la clave de API.
- Los tipos salen de `internal/contrato` (`go generate ./internal/contrato`).

## Pantallas

**Bienvenida (app y Web UI):** si no hay ningún agente listo, ofrece «Prepara tus agentes» con Claude Code y Codex
marcados; los instala uno tras otro en un solo panel, que en cada inicio de sesión enseña enlace («Abrir»), código
(«Copiar») y, si hace falta, el campo para pegar. Termina con «Listo: ya puedes usar …» para los que quedaron listos y,
para los que inician sesión y aún van con clave de API, «Falta un paso: usar tu suscripción.» con el botón «Usar mi
suscripción» (o su orden, donde no hay botón). Si un agente no acaba en listo, el panel ofrece «Reintentar» (también con
un solo agente) y «Saltar», y dice cuáles quedan (Rulings 26 y 27). Quien ya respondió la bienvenida en una versión
anterior no la vuelve a ver: prepara sus agentes desde Agentes y Primeros pasos (Ruling 28).

**Agentes:** cada tarjeta, su estado y una sola acción principal:
- No instalado → «Instalar».
- Instalado sin sesión → «Iniciar sesión» (Antigravity: «Poner la clave»).
- Listo → su versión y, si la hay, «Hay una versión nueva…» con «Actualizar».
- Sesión caducada → «Volver a iniciar sesión».
- Versión no comprobada → el aviso de «Versiones comprobadas», sin el botón de esa acción.
- En «…»: «Comprobar versión», «Actualizar», «Desinstalar» y «Ver registro».

**Ajustes:** «Comprobar versiones nuevas de los agentes» (activada, una vez al día), junto a las actualizaciones de
Tramtai, con a qué sitios se conecta. Como el de las actualizaciones, solo se cambia en el propio equipo: en la Web UI
remota del servidor sale desactivado.

**Terminal:** en Agentes, `i` instalar, `s` iniciar sesión, `u` actualizar, `x` desinstalar y `X` desinstalar borrando
sus datos (con confirmación), `y` seguir con una versión sin comprobar, `k` cancelar, `l` ver registro, `e` volver al
campo de pegar, `c` volver a comprobar y `a` usar tu suscripción tras iniciar sesión; `v` en Ajustes cambia el ajuste de
versiones. El panel enseña enlace y código como texto seleccionable y un campo para pegar; el enlace va sin saltos de
línea propios y como hipervínculo OSC 8, para copiarlo entero (Ruling 29).

**CLI** (con alias en inglés desde el principio):
- `tramtai agentes instalar|install <id>…`
- `tramtai agentes entrar|login <id>` (hoy solo en el servidor; pasa a funcionar siempre)
- `tramtai agentes actualizar|update <id>…|--todos`
- `tramtai agentes desinstalar|uninstall <id> [--borrar-datos]`
- `tramtai agentes versiones|versions [--comprobar]`

Imprimen el progreso, el enlace y el código, leen el código pegado por la entrada estándar sin mostrarlo y terminan con
un código de salida claro: 0 listo; 1 error; 2 versión sin comprobar que no se confirmó, o acción que ahora no se puede;
3 hecho, pero falta un paso (queda sin sesión, sin clave o sin usar tu suscripción, o no se pudo comprobar), con la
orden que falta (Ruling 30); 130 cancelado. `--confirmar` se salta la pregunta de la versión sin comprobar y la de
desinstalar, nunca el inicio de sesión ni el «acepto» de la suscripción.

De paso (Ruling 36), las órdenes que la Ayuda en inglés aún citaba en español llevan su alias: `actualizar|update`,
`conversor|converter` (`estado|status`, `preparar|prepare`), `espacio|workspace` (`iniciar|start`, `detener|stop`,
`olvidar|forget`), `espacios|workspaces` y `proyecto|project` (con `global`, que se escribe igual: `iniciar|start`,
`detener|stop`, `estado|status`, `abrir|open`); `servicio iniciar` admite `--open` (oculta) como `--abrir`. Los textos en
inglés citan las órdenes con su alias.

Todos los textos en español e inglés, con el glosario y `estilo-textos.test.ts`.

## Seguridad

- Órdenes fijas en el código; descargas solo de los orígenes oficiales por HTTPS; se comprueban sumas o firmas donde el
  fabricante las publica.
- Tramtai nunca lee ni guarda tokens. Del inicio de sesión solo saca el enlace y el código; lo pegado va al CLI por su
  entrada y no se registra; el registro tapa cualquier cosa con forma de secreto.
- La lista firmada de versiones comprobadas se rechaza si su firma no vale.
- En el servidor, solo el administrador; CSRF y freno de peticiones como el resto de la Web UI.
- Desinstalar pide confirmación; la consulta de versiones no envía nada que identifique y se puede desactivar.

## Comprobación con los programas reales (M0)

Primera tarea, en `m0.md` de esta carpeta, con carpetas de configuración temporales (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`,
`COPILOT_HOME`, HOME…) para no tocar las sesiones reales del usuario:
- Por CLI y sistema: orden de instalación sin Node (si la hay), dónde queda, cómo se fija la versión, actualizar,
  desinstalar, de dónde sale la última versión y si publica sumas o firmas.
- Inicio de sesión: si necesita pty, qué enlace y código imprime, si pide pegar algo, cómo termina. Con un inicio de
  sesión real en Windows (el usuario aprueba en el navegador dos o tres veces; se le avisa antes); macOS y Linux, solo
  instalación, en los runners del CI.
- Qué ajuste imprescindible hace falta, si alguno.
- Si algún CLI necesita Node.
- Windows Server con la cuenta virtual del servicio (job de Windows Server del CI y, si hace falta, una máquina de
  prueba).

## Pruebas

- CLI falsos que imitan el instalador y el inicio de sesión de cada agente, con salidas reales del M0 limpias de datos.
- Pruebas del lector de cada receta, de la lista de versiones comprobadas (firmada, no firmada, versión fuera de la
  lista, fallo del lector), de la API y los eventos, de la Web UI, la terminal y la CLI.
- Un job del CI semanal (y manual) que instala los cuatro CLI en los tres sistemas, comprueba versión, instalador e
  inicio de sesión hasta el enlace y el código, y sube la lista firmada al release `compatibilidad`. Si una versión nueva
  no pasa, no se añade y el job abre un issue para ajustar la receta.
- Al cierre, una prueba final con los programas reales, como en la v0.59.

## Documentación, Ayuda y sitio

- Ayuda nueva «Preparar tus agentes» (es/en) y al día «Primeros pasos», «Agentes» y «Modo servidor».
- `docs/inicio.md`, `docs/servidor.md` (fuera `sudo npm install -g` y `sudo -u` donde Tramtai ya lo hace), README, guía
  de desarrolladores (`internal/preparar`, recetas, lista firmada) y `docs/equipo/nuevo-agente.md` (la receta es parte
  de un agente nuevo).
- El sitio: una línea de ventajas («Tramtai instala y conecta tus agentes») y capturas nuevas de Agentes y de la
  bienvenida en es/en × claro/oscuro.

## Fuera de alcance

- Agentes sin CLI (Gemini es la API) y los que aún no tiene Tramtai (OpenCode, Hermes, Cursor…).
- Iniciar sesión con la cuenta personal en Antigravity (solo clave).
- Instalar Node en Linux (necesita root), u otros gestores de paquetes.
- Ajustes recomendados de los CLI (telemetría, autoactualización…): solo los imprescindibles.
- Las herramientas que se apuntan a Tramtai (v0.59) y sus pendientes.

## Criterios de aceptación

- En un equipo sin ningún CLI, desde la app, la terminal o la CLI, se llega a Claude Code y Codex listos sin abrir otra
  consola; Copilot igual; Antigravity queda instalado y pide su clave.
- Un CLI instalado se actualiza, se vuelve a conectar y se desinstala desde Tramtai; la tarjeta enseña la versión nueva
  cuando la hay.
- Una versión no comprobada desactiva solo la acción automática de ese agente, con el aviso y la orden para la consola,
  y se vuelve a habilitar al entrar en la lista firmada.
- Un fallo del lector nunca deja una preparación colgada.
- En el servidor Linux todo lo anterior se hace desde la Web UI remota; en Windows Server, lo que diga el M0, y lo que no
  se pueda está explicado.
- Ningún token pasa por Tramtai; las sesiones reales del usuario no se tocan en las pruebas.
- `go test ./...`, `npm test`, `npm run sitio` y la auditoría pasan; la Ayuda, las guías, el README y el sitio están al
  día en es/en.
