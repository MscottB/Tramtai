# Arquitectura

Cómo está construido Tramtai por dentro. Aquí se usan los nombres del código: una *instancia* es lo que la app llama
espacio, un *trabajo* es una petición, un *punto de acceso* es una API por aplicación y su *token* es la clave. La
correspondencia completa está en el [glosario](GLOSARIO.md).

## Paquetes

| Paquete | Responsabilidad |
|---|---|
| `internal/nucleo` | Tipos de dominio: `SolicitudEjecucion` (validación y huella SHA-256), `Trabajo` (modelo enviado, modelo efectivo y uso), estados y transiciones, códigos de error estables y presupuestos. No sabe de archivos ni del sistema operativo. |
| `internal/carpetas` | Las tres raíces: la de usuario (`RaizUsuario`, `DirDatos`: `%APPDATA%\tramtai` o, con `portable.txt` junto al ejecutable, `datos-tramtai` a su lado, solo para el usuario; en Windows, con su ACL) y la del servidor (`RaizServidor`: `/var/lib/tramtai` o `%ProgramData%\Tramtai`), que eligen `--servidor` o `TRAMTAI_SERVIDOR=1` (`Servidor`, `UsarServidor`); `Entorno` dice cuál es (servidor, portable o escritorio). También compara rutas como el sistema (`MismaRuta`, `Dentro`, `ExpandirCasa`). |
| `internal/servidor` | El modo servidor: instala, actualiza y quita el servicio del sistema (la unidad systemd en Linux, con su paquete `.deb`/`.rpm`; el servicio `Tramtai` con su cuenta virtual en Windows), guarda las claves del servidor (`systemd-creds` o la API de administración), aplica las reglas de exposición antes de arrancar (`ComprobarArranque`), lee las cabeceras de los proxies de confianza (`ClienteReal`, `EsHTTPS`) y pone el TLS propio y Let’s Encrypt. Ver [el servidor](#el-servidor). |
| `internal/terminos` | La aceptación expresa de `TERMINOS.md` en `datos/terminos.json` de cada entorno (versión, fecha y cómo) y los cinco riesgos (`Riesgos`), el mismo texto que `riesgos.*` de la Web UI. |
| `internal/totp` | El segundo factor del administrador del servidor: RFC 6238 con la biblioteca estándar. |
| `internal/autoarranque` | El arranque al iniciar sesión, de dos maneras (`ModoApp`, la app en la bandeja, y `ModoServicio`, solo el servicio): `HKCU\…\Run`, el LaunchAgent o el `.desktop`. |
| `internal/persistencia` | SQLite en modo WAL sin CGO (`modernc.org/sqlite`) con migraciones numeradas (`PRAGMA user_version`). Aceptación atómica, idempotencia, eventos numerados y recuperación tras reinicio. |
| `internal/planificador` | Cola acotada, concurrencia por instancia, semáforo por adaptador, tiempos máximos, cancelación idempotente y resolución del perfil de modelo al aceptar. |
| `internal/procesos` | Supervisión de procesos. En Windows crea el proceso suspendido, lo asigna a un Job Object con `KILL_ON_JOB_CLOSE` y lo reanuda; en Unix usa un grupo de procesos con SIGTERM y luego SIGKILL. Cancelar o agotar el tiempo termina el árbol completo; si no se confirma, devuelve `cancelacion_no_confirmada`. |
| `internal/adaptadores` | Contrato con los agentes (`Adaptador`, `Ficha`, `Detectable`, `Resolvedor`, `ConAutenticacion`), el agente `Simulado` y lo común a todos: enmarcado de turnos, entorno sin claves ajenas, carpeta temporal, clasificación de errores y redacción de secretos. |
| `internal/agentes` | Catálogo de agentes: cada uno en su módulo (`internal/agentes/claude`, `codex`, `copilot`, `gemini`, `antigravity`) con su adaptador y su ficha (nombre, color, capacidades, esfuerzo, autenticación, comando, si se permite en apps). El servicio, el planificador, la API (`GET /api/v1/agentes`) y la Web UI le preguntan en lugar de nombrar agentes. Cómo añadir uno: [nuevo-agente.md](equipo/nuevo-agente.md). |
| `internal/documentos` | Cargas identificadas por SHA-256 (`carga_<hash>`) con vencimiento. Conversión a Markdown con markitdown 0.1.8 en un proceso supervisado con plazo y límite de salida. El conversor se localiza (configurado, paquete junto al ejecutable, preparado en la carpeta del usuario o del PATH) y se prepara con el `uv` que viene con Tramtai y un lock con hashes incrustado (`conversor/conversor.lock`), en un Python independiente sin venv; un único `Preparacion` por servicio lleva su estado y el planificador rechaza con 503 `conversor_no_disponible` lo que no se podría convertir. Los archivos de texto (txt, md, json, xml, csv, yaml…) pasan tal cual, validando que sean UTF-8. `Formatos()` es la única lista de extensiones admitidas: la usan `Cargas.Guardar` (que rechaza el resto al subir), `/limites`, la CLI y la terminal. Los ZIP no se admiten desde la v0.50.2. |
| `internal/herramientas` | Herramientas (`tools`) emuladas para los agentes CLI, igual en las tres APIs: `Validar` al aceptar, `Preparar` (las funciones descritas en las instrucciones, el sobre JSON `{"content", "tool_calls"}` como esquema y la conversación con llamadas y resultados escrita como texto) al ejecutar, e `Interpretar` (texto y llamadas, con los argumentos validados contra el esquema de cada herramienta) al validar la salida y al responder. El agente nunca ejecuta nada. |
| `internal/salidas` | Validación de la salida contra el JSON Schema del consumidor (`santhosh-tekuri/jsonschema`), con los `$ref` externos deshabilitados. |
| `internal/api` | Servicio HTTP: valida Host, Origin y token; API nativa con SSE reanudable por `Last-Event-ID` y un SSE de toda la instancia (`GET /api/v1/eventos`, que alimenta el mapa vivo de Inicio a partir de los avisos de cambio del almacén); fachadas OpenAI y Anthropic con política de parámetros y su formato de errores. Detalle en [api.md](api.md). |
| `internal/servicio` | Servicio único: instancias global y por proyecto con token, base de datos, cola y cargas propios; enrutado por token; administración con token de servicio; puntos de acceso con puerto o ruta `/p/<nombre>` y recarga sin reinicio; descriptor `servicio.json`. Las rutas `/ui/` de la Web UI están en una tabla (`rutasDeUI`) con el permiso de cada una: pública, con sesión o solo con una sesión local. Las reglas para guardar una API (respaldo que funcione, agentes permitidos en apps) se aplican solo aquí. |
| `internal/configuracion` | Configuración por capas (interna, global, proyecto, local y entorno) con su esquema y los perfiles de modelo. La capa interna de los agentes la da `agentes.Internos()` y quien carga la configuración la pasa a `CargarConfiguracion`: no hay registro global. Edición de los archivos de configuración (`config.yaml` y `local.yaml`) conservando el resto del archivo y sus comentarios: `FijarValor`, `QuitarValor` y `EditarPuntos` validan el resultado con el esquema antes de guardarlo con `EscribirAtomico` (temporal y renombrado). La usan el servicio y la CLI. |
| `internal/contrato` | Los tipos que viajan entre el servicio y sus clientes (estado, instancias, puntos de acceso, descriptor). No depende del servicio, así que un cliente no enlaza el planificador, la base ni los adaptadores; una prueba lo comprueba. |
| `internal/cliente` | Cliente del servicio para la CLI, la terminal y la app de escritorio: descubre el servicio por su descriptor, lo inicia en segundo plano, gestiona instancias y puntos de acceso por la API y sigue los eventos SSE. |
| `internal/tui` | Terminal interactiva con Bubble Tea v2. |
| `internal/recursos` | Web UI compilada incorporada con `go:embed`. Se versiona para que `go build` no necesite Node. |
| `internal/navegador` | Abre URLs en el navegador del sistema. |
| `internal/credenciales` | Claves de los proveedores en el almacén del sistema: Administrador de credenciales (Windows), Llavero (macOS, `/usr/bin/security`) y Secret Service (Linux, `secret-tool`). |
| `web/` | Web UI con Vue 3 y Quasar 2.35 (Vite). La compila `npm run build` en `internal/recursos/web`. La Ayuda son datos (`web/src/ayuda`): la app los muestra y `npm run sitio` genera con ellos `site/ayuda/`. |
| `escritorio/` | Aplicación de escritorio con Wails v3 en un módulo Go aparte, para que el núcleo siga sin CGO. Es un cliente del servicio: lo inicia si hace falta y muestra la misma Web UI en su ventana. Solo usa `cliente` y `contrato`, no el servicio. |
| `internal/version` | Versión del ejecutable (inyectada al compilar) y commit de git. |
| `cmd/tramtai` | CLI con Cobra. Sin argumentos abre la terminal interactiva. `tramtai acceso` guarda las APIs por la API del servicio (`/api/v1/servicio/puntos`, con el token de servicio), como la terminal y la app de escritorio usan `internal/cliente`; `tramtai ejecutar` sin servicio usa su propia base, y `tramtai servicio` es el propio servicio. Comandos: `servicio`, `global`, `proyecto`, `iniciar`, `acceso`, `claves`, `trabajos`, `ejecutar`, `convertir`, `agentes`, `configuracion`, `terminos` y, en el modo servidor, `servidor` y `administrador` (los nuevos llevan su alias en inglés). Detalle en [operacion.md](operacion.md). |

## Agentes

| | Claude Code | Codex CLI | GitHub Copilot CLI |
|---|---|---|---|
| Invocación | `claude -p --output-format stream-json`, prompt por stdin | `codex exec --json -`, prompt por stdin | `copilot --output-format json`, prompt por stdin; en Windows, el `copilot.exe` nativo del paquete de npm (no el envoltorio `.cmd`) |
| Aislamiento | `--setting-sources ""`, `--strict-mcp-config`, `--tools ""`, `--no-session-persistence` y espacio de trabajo temporal vacío. Con clave de API, también `--bare`. | `--ephemeral`, `--ignore-rules`, `-s read-only`, shell y búsqueda web desactivadas. Con clave de API, `CODEX_HOME` privado y vacío. | `--available-tools none` (lista vacía), `--disable-builtin-mcps`, `--no-custom-instructions`, `--disallow-temp-dir`, `--no-ask-user`, `--no-auto-update` y `COPILOT_HOME` privado por ejecución, que se borra al terminar. Con suscripción, su `config.json` solo lleva la cuenta de `copilot login` (el token queda en el almacén del sistema); con clave, `COPILOT_GITHUB_TOKEN`. |
| Sistema | `--system-prompt-file`: sustituye el prompt de Claude Code (≈650 tokens fijos en vez de ≈24 000) | primer bloque delimitado del prompt | primer bloque delimitado del prompt |
| Esquema | nativo (`--json-schema`) | nativo (`--output-schema`) | en las instrucciones; se quita el bloque ```` ```json ```` si llega |
| Streaming | por tokens (`text_delta`) | solo el mensaje final | por tokens (`assistant.message_delta`) |
| Modelo efectivo | sí (`modelUsage`) | no: queda como no verificable | sí, también el que elige `auto` (`session.auto_mode_resolved`) |
| Esfuerzo | `--effort`, hasta máximo | `model_reasoning_effort`, hasta muy alto | `--reasoning-effort`, hasta máximo; no con `auto` |
| Uso | tokens y costo en USD | tokens | tokens (`--usage-output-file`) y un aviso con los créditos del plan |

Además hay dos adaptadores de Google, probados con un servidor y un CLI simulados:

- **`gemini`:** Gemini API directa por HTTP con `GEMINI_API_KEY`, que viaja en cabecera, nunca en la URL.
  - Streaming por fragmentos (`streamGenerateContent`) y sistema en `systemInstruction`.
  - Salida estructurada nativa (`responseJsonSchema`), modelo efectivo (`modelVersion`) y uso.
  - Perfiles: `gemini-flash-lite-latest`, `gemini-flash-latest` y `gemini-pro-latest`.
- **`antigravity`:** `agy --input-format stream-json --output-format stream-json`, con el mensaje por stdin.
  - Funciona solo con `modelProvider: gemini` y la clave del usuario, en un `HOME` privado y temporal; rechaza la suscripción por los términos de Antigravity.
  - **Sin herramientas:** un hook `PreToolUse` (`.agents/hooks.json` en la carpeta de trabajo, `matcher: "*"`) responde `deny` a todas; probado con el CLI real en Windows (lectura, órdenes y web), con la sesión del usuario y fuera de Tramtai. Falta su canario con `GEMINI_API_KEY` y en Linux y macOS. Si una herramienta llegara a completarse, Tramtai corta la ejecución.
  - Respuesta en vivo (`text_delta`), esfuerzo (`--effort`) y uso. El formato JSON va en las instrucciones: con `--json-schema` agy entrega el JSON con su herramienta `finish`, que el hook niega.
  - No admite `consciente_proyecto`.

Las conversaciones de varios turnos se enmarcan con el formato `tramtai-turnos-v2`: un bloque `<<<turno-<código> rol=…>>>` por turno, con el contenido intacto y un código aleatorio por petición para que un mensaje no pueda cerrar su turno. Los errores del proveedor se traducen a `limite_uso_proveedor`, `autenticacion_agente`, `modelo_no_disponible` o `error_agente`. Ningún secreto llega a los mensajes: se redactan antes.

## Entradas y salidas

Con cargas, el trabajo pasa por `preparando_entrada`, que tiene su propio límite de conversiones simultáneas (`concurrencia.maximo_conversiones_instancia`). Cada archivo se convierte por separado y se antepone al último mensaje del usuario en un bloque delimitado:

```
<<<archivo-3f9a1c2e nombre="acta.docx" sha256="…">>>
…Markdown…
<<<fin archivo-3f9a1c2e>>>
```

El código tras el guion es aleatorio por petición (igual en los turnos `<<<turno-…>>>`): un documento que escriba
`<<<fin archivo>>>` no cierra su bloque.

Si una carga venció, el trabajo falla con `entrada_no_disponible`. Una conversión vacía o fallida falla con `conversion_fallida`; nunca se presenta como éxito.

Con herramientas, la solicitud del cliente guarda `herramientas`, `eleccion_herramienta` y los mensajes con `llamadas` o
de rol `herramienta`; cada paquete hace su parte:

| Momento | Quién | Qué hace |
|---|---|---|
| Aceptar el trabajo | planificador (las tres APIs) | `herramientas.Validar`; error 400 antes de encolar |
| Ejecutar | planificador | `herramientas.Preparar` arma lo que recibe el agente; el trabajo guarda la solicitud del cliente |
| Validar la salida | planificador (`validarSalida`) | `herramientas.Interpretar`; si no cumple, el ciclo de correcciones y respaldo de siempre |
| Leer el trabajo | API propia | `salida` = el texto y `llamadas` |
| Responder | fachadas | las mismas llamadas como `tool_calls` (OpenAI) o `tool_use` (Anthropic) |

Con herramientas no se guarda texto en vivo: sería el sobre interno; la respuesta llega al terminar.

Con un esquema, se usa primero la salida estructurada nativa del agente (`--json-schema` y `structured_output` en Claude; `--output-schema` en Codex); GitHub Copilot, que no la tiene, lo recibe en las instrucciones. Después Tramtai valida el resultado. Si no cumple, el trabajo falla con `salida_no_cumple_esquema` y la salida solo se conserva con `conservar_salida_invalida`. Un esquema inválido se rechaza al aceptar el trabajo.

## Modelo de amenazas

Lo que llega de una app (mensaje, instrucciones, documentos, esquema) se trata como hostil. No se puede impedir que
intente manipular al modelo; sí que un modelo manipulado haga daño a través de Tramtai:

- **Agentes:** Claude Code sin herramientas, sin configuración del usuario y con el mensaje por stdin; Codex en solo
  lectura, sin terminal ni búsqueda web, en una carpeta temporal y con el mensaje por stdin; GitHub Copilot sin herramientas ni
  MCP, con un `COPILOT_HOME` privado y el mensaje por stdin. Las pruebas canario (`pruebas/canario`) lo comprueban con
  los CLI reales de Claude, Codex y Copilot: un mensaje les pide leer un archivo fuera de su carpeta, ejecutar una orden
  y abrir una URL local, y no lo consiguen. Gemini es HTTP, sin herramientas, y no pasa por el canario. Antigravity va sin herramientas por un hook, con el mensaje por stdin
  y la configuración privada; queda fuera de las apps hasta que su canario pase con una clave de la Gemini API.
- **Entrada:** delimitadores con código por petición; topes de mensajes (1.000), esquema (256 KB), cuerpo (16 MB) y
  la comprobación del contexto antes de llamar al CLI.
- **Cargas y conversión:** identificadores por API, cuota por instancia, solo formatos anunciados (el resto se rechaza
  al subir), sin ZIP, Office comprobado antes de convertir, plazo por archivo y total, y tope de
  memoria para markitdown. El conversor se prepara con uv fijado por SHA-256 y dependencias con hashes
  (`--require-hashes`), sin tocar el PATH ni el registro y sin la configuración de uv del usuario.
- **Procesos:** cada CLI en un grupo que se termina entero (Job Object en Windows), sin shell, con un tope de 8 MB por
  línea de salida y sin los secretos del entorno (claves, contraseñas, URL con credenciales, `TRAMTAI_*`).
- **Salida:** las claves de proveedor se tapan; la Web UI muestra Markdown solo como HTML limpio (DOMPurify) bajo una
  CSP estricta; un error del CLI no puede escribir líneas falsas en el registro. Una clave codificada (base64, por
  ejemplo) no se reconoce: ver el riesgo de Codex en [el modelo de amenazas del servidor](#modelo-de-amenazas-del-servidor).

## El servidor

El modo servidor (v0.58) no cambia el servicio por dentro, solo dónde vive y quién llega a él. Guía de uso:
[servidor.md](servidor.md).

### Tercera raíz y dos entornos

`internal/carpetas` suma la raíz del servidor a la de usuario y la portable: `/var/lib/tramtai` en Linux y
`%ProgramData%\Tramtai` en Windows, con lo de siempre dentro (`config.yaml`, `datos/`, el registro y el conversor). La
eligen `--servidor` (o `--server`) en la CLI y `TRAMTAI_SERVIDOR=1`, que ponen la unidad y el servicio. El servicio del
sistema tiene su puerto (17373, o el siguiente libre si el del usuario lo ocupa), su descriptor y su cerrojo, así que
convive con el Tramtai del usuario: son dos entornos que no comparten nada, y la CLI sin `--servidor` sigue hablando con
el del usuario. La carpeta solo la usan la cuenta del servicio y los administradores: 0700 con dueño `tramtai` en Linux;
en Windows, una DACL protegida con `SYSTEM`, `Administradores` y `NT SERVICE\Tramtai`. Lo que la CLI escribe como root
en ella pasa por `os.Root` (sin seguir enlaces) y queda a nombre de `tramtai` al terminar cada orden.

### El servicio del sistema

- **Linux:** `tramtai.service` (en `/etc/systemd/system`, o en `/usr/lib/systemd/system` con el paquete y sus ajustes en
  el drop-in `10-tramtai.conf`), con `User=tramtai`, `StateDirectory=tramtai`, `Restart=on-failure`, `KillMode=mixed` (el
  SIGTERM va al servicio, que drena, y no a los CLI) y un plazo de parada mayor que el de drenaje. Va endurecida:
  `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`, `PrivateDevices`, escritura solo en su carpeta,
  `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK`, `CapabilityBoundingSet=` vacío y `UMask=0077`, entre
  otras. Sin `MemoryDenyWriteExecute`, `RestrictNamespaces`, `SystemCallFilter`, `ProcSubset=pid` ni `PrivateUsers`, que
  rompen los CLI de los agentes (el JIT de Node de Claude Code y Copilot). `CAP_NET_BIND_SERVICE` solo con Let’s Encrypt
  o un puerto menor de 1024. `servidor instalar` y `servidor reiniciar` la regeneran con la configuración del momento.
- **Windows:** un servicio con `golang.org/x/sys/windows/svc`, `Tramtai` («Tramtai (servidor)»), con la cuenta virtual
  `NT SERVICE\Tramtai`, arranque automático retrasado y reinicio si se cae (también tras un «Reiniciar», que sale con un
  código propio distinto de cero). Detenerlo drena como `tramtai servicio detener`, y los Job Objects funcionan igual.
- **Administración:** `tramtai-servidor <orden>` es `tramtai --servidor <orden>` con el programa del servidor
  (`/opt/tramtai/tramtai` o `C:\Program Files\Tramtai Servidor\tramtai.exe`). Con privilegios en el servidor, la CLI no
  ejecuta agentes ni el servicio en primer plano (`agentes`, `ejecutar` sin el servicio, `servicio iniciar`): ejecutaría
  como root los programas de `comandos_agentes`, que la cuenta del servicio puede cambiar en `config.yaml`. Con
  `--servidor`, nunca detiene ni sustituye un servicio del sistema de otra versión.
- **Actualizar:** `servidor actualizar` reutiliza `internal/actualizacion` (firma ed25519 de `SHA256SUMS.txt` y suma
  SHA-256), drena, cambia el programa dejando el anterior como `.anterior` y ejecuta el nuevo con `servidor reiniciar`
  para regenerar la unidad. Lanzado desde otra copia, delega en el programa instalado; con el paquete, remite a apt o dnf.

### Claves del servidor

| Clave | Windows Server | Linux |
|---|---|---|
| De proveedor y token de GitHub para Copilot | Administrador de credenciales de la cuenta del servicio (DPAPI, sin sesión); la CLI se la entrega al servicio por la API de administración | `systemd-creds` (con TPM2 si lo hay) en `/etc/credstore.encrypted/tramtai.<NOMBRE>`, entregadas en `$CREDENTIALS_DIRECTORY`; sin `systemd-creds` (systemd < 250), `/etc/tramtai/claves.env` de root, con aviso |
| Secreto TOTP del administrador (`TRAMTAI_TOTP`) | igual | igual (solo la CLI lo activa o lo quita) |
| Contraseña del administrador y códigos de recuperación | solo su huella argon2id (m=64 MiB, t=3, p=1) en `datos/administrador` y `datos/administrador-recuperacion` | igual |
| De los espacios y de las APIs | solo su huella `sha256:` | igual |
| `token-servicio` | archivo en `datos/` que solo leen la cuenta del servicio y los administradores | igual (0600, dueño `tramtai`) |

`internal/credenciales` en Linux lee primero `$CREDENTIALS_DIRECTORY/tramtai.<NOMBRE>` y después Secret Service y la
variable de entorno. Ninguna clave se guarda desde la Web UI.

### Modelo de amenazas del servidor

- **Red:** expuesto es `servicio.host` fuera de loopback o `servicio.proxy.confiables` no vacío. `ComprobarArranque`
  impide arrancar con un host fuera de loopback sin `permitir_remoto` o sin TLS propio, con una suscripción que no es la
  personal (o personal sin TOTP), con Let’s Encrypt incompleto o con un certificado que no cubre `servicio.dominios`.
  `Host` y `Origin` solo valen para loopback y `servicio.dominios`; los plazos de lectura y el tope de cabeceras son los
  de siempre, en un constructor común del `http.Server`. TLS propio solo 1.3 (X25519MLKEM768 de Go 1.27), HSTS de un año.
- **Proxy:** `X-Forwarded-For`, `-Proto` y `-Host` solo se creen desde `servicio.proxy.confiables` (nunca redes más
  anchas que /8 o /32 en IPv6); la dirección del cliente es la última de `X-Forwarded-For` que no es de confianza.
  En el oyente público, una petición desde un proxy de confianza nunca es local, aunque no traiga cabeceras; la CLI, la
  terminal y la app usan un oyente local aparte en `127.0.0.1`. `Forwarded`, `Via` y `X-Real-Ip` desde fuera no cambian
  nada.
- **Inicio de sesión:** solo con TLS propio o desde un proxy de confianza con `X-Forwarded-Proto: https`; el mismo
  mensaje para contraseña y código; 5 fallos por dirección dan 15 minutos de espera y 100 en 10 minutos, 5 minutos para
  todos salvo las redes que entraron bien en 30 días; como mucho 2 cálculos de argon2id a la vez, dentro de los que se
  cuenta cada fallo; TOTP sin reutilizar un paso; cookie `__Host-` con `Secure`, `HttpOnly` y `SameSite=Strict`, más
  `X-CSRF-Token`; caducidad de 12 h sin uso; cambiar la contraseña o «Cerrar todas las sesiones» cierran también los
  flujos SSE abiertos; cuerpos de `/ui/acceder` y `/ui/sesion` de 64 KB como mucho; sin redirecciones a URL de la
  petición. Expuesto, el token de servicio solo vale desde este equipo (desde fuera, 403 en cualquier ruta). Las claves
  falsas en las APIs: 30 por dirección en un minuto dan un minuto de espera (429); ese freno falla abierto si se satura,
  porque las claves son de 256 bits.
- **ACME:** `autocert` con TLS-ALPN-01 en el 443 (no abre el 80), `HostPolicy` limitada a `servicio.dominios` y caché en
  `datos/acme` (0700).
- **Suscripción remota:** solo la personal, con TOTP obligatorio, `suscripcion_simultaneas` (1 a 3) por agente en todo
  el servidor y clave de seguridad (no desde `local.yaml` ni desde una sesión remota). Tramtai nunca guarda un token de
  sesión de Claude: el inicio de sesión lo hace el CLI con la cuenta del servicio.
- **Riesgo residual de Codex:** Codex corre con `-s read-only`, que lee todo lo que lee la cuenta del servicio. Quien
  tenga la clave de una API con Codex puede pedirle que lea `$CREDENTIALS_DIRECTORY` (las claves de proveedor y
  `TRAMTAI_TOTP`) y `datos/administrador`, y la redacción de secretos no detiene una clave codificada. No hay arreglo en
  Tramtai sin quitarle a Codex la lectura: la guía pide dar esas APIs solo a personas de confianza o dejar Codex fuera
  de las APIs de otros. Detalle en [revisión de seguridad](equipo/releases/v0.58-servidor/revision-seguridad.md).

## Interfaces

Las cuatro interfaces trabajan sobre el mismo servicio, así que un trabajo creado en una se ve y se controla en las demás:

- **CLI:** `tramtai ejecutar` usa el servicio si está en marcha (`--directo` para no usarlo).
- **Terminal interactiva:** `tramtai` sin argumentos.
- **Web UI:** servida por el propio servicio en `http://127.0.0.1:17373`.
- **Aplicación de escritorio:** `tramtai-escritorio`.

La Web UI se enlaza a una instancia (un espacio de trabajo: el global o el de un proyecto) con una sesión:

- **Entrada sin clave en el propio equipo:** si la conexión llega por loopback desde un programa del mismo usuario del sistema (el navegador o la app de escritorio), la Web UI entra sola y puede cambiar entre instancias. En Windows se identifica al dueño de la conexión con la tabla TCP del sistema y el SID de su proceso; en Linux, con el uid de `/proc/net/tcp`; en macOS, con el `so_uid` de `net.inet.tcp.pcblist_n` (la tabla que lee `netstat`). Se comprueba una vez por conexión. No se aplica si la petición trae cabeceras de proxy (`Forwarded`, `X-Forwarded-For`, `X-Forwarded-Host`, `X-Real-Ip`, `Via`), ni con `servicio.permitir_remoto`, porque un proxy o túnel del mismo usuario reenviaría peticiones de otros equipos. Se desactiva con `servicio.entrada_local: false`.
- **Contraseña del administrador (modo servidor):** desde otros equipos, por HTTPS, con TOTP opcional. Ver
  [el servidor](#modelo-de-amenazas-del-servidor).
- **Código de un solo uso:** la otra forma de entrar. Lo piden `tramtai abrir` o la app de escritorio con el token de servicio (un minuto de validez). Es lo que se usa cuando no se puede entrar directamente: otro usuario del equipo, `servicio.entrada_local: false` o un sistema donde no se identifica al dueño. Un código canjeado desde este equipo (loopback y sin cabeceras de proxy) abre una sesión local, como la entrada directa: solo el dueño del servicio puede pedirlo. Con el servicio expuesto en red, la Web UI remota solo se abre con un código de `tramtai abrir` hecho en el equipo del servicio, y esa sesión no es local.
- **La clave de las aplicaciones no abre la Web UI:** `POST /ui/sesion` con una clave responde 401. Quien tuviera esa clave podría, si no, crear APIs y cambiar la configuración.
- **Por qué es seguro:** la entrada directa protege frente a otros usuarios del equipo, y eso es justo lo que se comprueba. Una página web ajena no puede aprovecharla: Host, Origin, la cookie `SameSite=Strict` y el CSRF la bloquean, y las aplicaciones siguen necesitando su clave para la API.
- **Cookie:** `HttpOnly` y `SameSite=Strict`; las peticiones que modifican exigen además `X-CSRF-Token`.
- **Navegador:** se valida `Origin`, no hay CORS y se aplica una CSP estricta.
- **El token nunca viaja en la URL** y no se guarda en el navegador.
- **Configuración y puntos de acceso:** solo se gestionan con una sesión de la Web UI o el token de servicio, no con el token que usan las aplicaciones.

### Claves

| Clave | Cómo se guarda |
|---|---|
| De cada punto de acceso | Solo su huella SHA-256 (`datos/tokens/acceso_<nombre>` contiene `sha256:…`), comparada en tiempo constante. Se muestra al crearla o regenerarla; las antiguas en claro se convierten solas a huella. Eliminar el punto borra su clave. |
| Del espacio | Solo su huella SHA-256 (`datos/token-global` o `datos/tokens/<id>` contiene `sha256:…`), comparada en tiempo constante; la de una variable de entorno (`TRAMTAI_TOKEN_GLOBAL`) se reduce a huella al arrancar y no se guarda. Un espacio nuevo no tiene clave: se crea (o se regenera, sin reiniciar) con `POST /api/v1/instancia/token`, que la devuelve una sola vez; `GET` solo dice si hay y de dónde viene. Las antiguas en claro se convierten solas a huella. |
| Clave de servicio | En claro, en `datos/token-servicio` con permisos 0600, porque la CLI y la app de escritorio del mismo usuario la leen para administrar el servicio, y un almacén cifrado no la protegería más frente a ese usuario. |
| De los proveedores | Cifradas en el almacén del sistema: Administrador de credenciales (Windows), Llavero vía `/usr/bin/security` (macOS) o Secret Service vía `secret-tool` (Linux). Tramtai nunca las escribe en sus archivos. Se leen en cada petición, así que un cambio vale sin reiniciar; una variable de entorno con el mismo nombre tiene prioridad. |

### Espacios y agentes

- **Registro de espacios** (`datos/espacios.json`, `internal/servicio/espacios.go`): todos los espacios conocidos con un
  id fijo, también los detenidos y los que perdieron su carpeta. El id de un proyecto se guarda en `.tramtai/espacio`
  (la marca): al mover la carpeta se reconoce, y una copia no se adopta. Detalle en
  [operación](operacion.md#espacios-de-trabajo).
- **Confianza:** `local.yaml` solo puede tocar claves de seguridad si la carpeta está en `proyectos_confiables`; al
  abrir una carpeta clonada, la Web UI pregunta (`confianza_requerida`) y, sin confianza, las ignora y lo avisa.
- **Agentes desconectados:** `adaptadores.<agente>.desconectado` deja un agente fuera sin desinstalar su CLI; el
  planificador rechaza sus peticiones con 409 `agente_desconectado` y el respaldo lo salta.
- **Sesión del CLI:** cada adaptador con suscripción pregunta a su CLI con qué cuenta entró (`claude auth status`,
  `codex login status`, la configuración de Copilot) sin leer sus credenciales; un fallo de autenticación en una
  petición marca el agente como «la sesión no responde» hasta la siguiente que salga bien. Evidencia en
  [m0-agentes](equipo/m0-agentes.md).

## Estados del trabajo

```
en_cola → preparando_entrada → esperando_agente → ejecutando → validando_salida → completado
                                                                                 ↘ fallido
ejecutando → esperando_agente   (salto de respaldo: el agente falló antes de emitir texto)
cualquiera no terminal → cancelacion_solicitada → cancelado
ejecutando al reiniciar el servicio → interrumpido   (no se relanza)
en_cola o esperando_agente al reiniciar → se reencola (tras un salto, con el respaldo que queda)
```

## Retención, cierre y recuperación

La retención se aplica a cada instancia al abrirla y después cada 5 minutos. Los plazos cuentan desde que el trabajo termina, y los trabajos activos nunca se purgan:

| Clave | Por defecto | Qué borra al vencer |
|---|---|---|
| `retencion.horas_resultado` | 24 (0 a 720) | La salida, los mensajes, las instrucciones, las cargas citadas y los eventos con contenido. El trabajo sigue `completado`, con `disponibilidad: expirada`, y la salida responde 410. |
| `retencion.horas_metadatos` | 168 (0 a 2160, y no menos que `horas_resultado`) | El trabajo entero y sus eventos de estado. Desde entonces responde 404. |
| `retencion.max_trabajos` | 10000 (100 a 100000) | Los trabajos terminados más antiguos que pasen del tope, con sus eventos. Los activos no cuentan. |
| `retencion.horas_idempotencia` | 24 | La clave `Idempotency-Key`; después la misma clave crea otro trabajo. |
| `retencion.horas_carga_huerfana` | 24 | Las cargas que ningún trabajo necesita. Las de un trabajo que aún se puede reintentar se conservan aunque hayan vencido. |
| `retencion.eliminar_entradas_al_terminar` | false | Con `true`, las entradas se borran al terminar y el reintento exige reenviarlas. |

**Cierre:** al detener el servicio se dejan de aceptar trabajos (503 `servicio_detenido`). Lo que se está ejecutando tiene hasta `servicio.plazo_drenaje_segundos` (30) para terminar, mientras la API sigue respondiendo para recoger resultados. Al agotarse el plazo, lo que quede pasa a `interrumpido`. Lo que estaba en cola se conserva y se reanuda al reiniciar. Desactivar una instancia no espera: interrumpe lo que tenga en curso.

**Caída del servicio:**

- En Windows, el sistema cierra el Job Object de cada agente y termina su árbol de procesos. Se verificó matando el servicio con 6 procesos de Codex en marcha.
- Al reiniciar, lo que estaba ejecutándose pasa a `interrumpido` y nunca se relanza.
- En Unix el grupo de procesos sobrevive a la caída; al arrancar, el servicio lo termina si su firma de inicio coincide.

**Límite:** el vencimiento borra los datos de forma lógica (SQLite y archivos). No garantiza un borrado físico seguro en SSD, copias de seguridad ni páginas libres de SQLite.

## Modo consciente del proyecto

Por defecto, cada agente se ejecuta aislado en una carpeta temporal vacía. Con `modo_contexto: consciente_proyecto`, en un proyecto declarado en `proyectos_confiables` (solo en la configuración global; la versionada y `local.yaml` lo rechazan):

| Agente | Qué cambia |
|---|---|
| claude | Trabaja en la raíz del proyecto con `--setting-sources project`. |
| codex | Trabaja en la raíz del proyecto (`-C`) sin `--ignore-rules`, así que carga su `AGENTS.md`. |
| antigravity | No lo admite (`contexto_no_permitido`): trabaja siempre en la carpeta temporal, donde está su hook. |
| gemini | No lo admite, porque no tiene proyecto. |

**Límites comprobados (Claude):**
- Carga también los `CLAUDE.md` de las carpetas superiores.
- Con suscripción, carga además el `CLAUDE.md` del usuario.
- Con clave de API usa un `CLAUDE_CONFIG_DIR` privado, así que las reglas del usuario no se cargan.

**En todos los agentes:** siguen desactivados las herramientas, MCP y la sesión persistente, y antes de ejecutar se registra un `aviso` con el contexto que se carga.

## Supervisión y recuperación

- **Firma de inicio:** cada agente registra su PID y su firma de inicio (migración v5).
- **Al arrancar:** el servicio termina los procesos huérfanos de una caída, solo si su firma coincide; un PID reutilizado nunca se toca.
  - Windows: el Job Object ya los cerró.
  - Unix: se mata su grupo de procesos.
- **Seguir un trabajo:** el SSE espera avisos en memoria del almacén en lugar de sondear la base; el sondeo queda como respaldo cada 5 s.

## Precedencia de configuración

De menor a mayor: interno → global → proyecto → local → entorno → punto de acceso → solicitud. Las claves de seguridad (`servicio`, `comandos_agentes`, `adaptadores`, `concurrencia_servicio`…) se rechazan en la capa de proyecto, que es la que va en el repositorio, y en `local.yaml` salvo que la carpeta figure en `proyectos_confiables` de la configuración global (un `local.yaml` también puede llegar con un clon). `proyectos_confiables` solo se lee de la global.

Los agentes no heredan secretos del entorno: se retiran las credenciales conocidas de los proveedores, todas las `TRAMTAI_*` (el token del servicio) y cualquier variable cuyo nombre contenga `API_KEY`, `TOKEN`, `SECRET`, `PASSWORD` o `CREDENTIAL` (`GITHUB_TOKEN`, `AWS_SECRET_ACCESS_KEY`…). Cada CLI solo conserva las variables secretas con los prefijos de su propio proveedor (`CLAUDE_` y `ANTHROPIC_` para Claude, `CODEX_` y `OPENAI_` para Codex, `GEMINI_` para Antigravity; GitHub Copilot no conserva ninguna `COPILOT_*` heredada, ni `GH_TOKEN` ni `GITHUB_TOKEN`), y la clave que necesita se la pasa Tramtai según el modo de autenticación. La consulta `--version` se ejecuta sin ninguna clave.

## Cumplimiento de proveedores

Por defecto cada adaptador usa clave de API. El modo suscripción es opcional, solo local y sin acceso remoto. Google se usará vía Gemini API directa; `agy` solo con `modelProvider: gemini` + `GEMINI_API_KEY`.
