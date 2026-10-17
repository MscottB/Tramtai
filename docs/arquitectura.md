# Arquitectura

Cómo está construido Tramtai por dentro:

## Paquetes

| Paquete | Responsabilidad |
|---|---|
| `internal/nucleo` | Tipos de dominio: `SolicitudEjecucion` (validación y huella SHA-256), `Trabajo` (modelo enviado, modelo efectivo y uso), estados y transiciones, códigos de error estables, configuración por capas y perfiles de modelo. |
| `internal/persistencia` | SQLite en modo WAL sin CGO (`modernc.org/sqlite`) con migraciones numeradas (`PRAGMA user_version`). Aceptación atómica, idempotencia, eventos numerados y recuperación tras reinicio. |
| `internal/planificador` | Cola acotada, concurrencia por instancia, semáforo por adaptador, tiempos máximos, cancelación idempotente y resolución del perfil de modelo al aceptar. |
| `internal/procesos` | Supervisión de procesos. En Windows crea el proceso suspendido, lo asigna a un Job Object con `KILL_ON_JOB_CLOSE` y lo reanuda; en Unix usa un grupo de procesos con SIGTERM y luego SIGKILL. Cancelar o agotar el tiempo termina el árbol completo; si no se confirma, devuelve `cancelacion_no_confirmada`. |
| `internal/adaptadores` | Contrato `Adaptador`, agente `Simulado` y los adaptadores reales: Claude, Codex, Gemini API y Antigravity. |
| `internal/documentos` | Cargas identificadas por SHA-256 (`carga_<hash>`) con vencimiento. Conversión a Markdown con markitdown 0.1.8 en un proceso supervisado con plazo y límite de salida. Los archivos de texto (txt, md, json, xml, csv, yaml…) pasan tal cual, validando que sean UTF-8. ZIP seguro con límites de archivos, tamaño descomprimido medido y profundidad, rechazando rutas con `..`, absolutas, con unidad o enlaces. |
| `internal/salidas` | Validación de la salida contra el JSON Schema del consumidor (`santhosh-tekuri/jsonschema`), con los `$ref` externos deshabilitados. |
| `internal/api` | Servicio HTTP: valida Host, Origin y token; API nativa con SSE reanudable por `Last-Event-ID`; fachadas OpenAI y Anthropic con política de parámetros y su formato de errores. Detalle en [api.md](api.md). |
| `internal/servicio` | Servicio único: instancias global y por proyecto con token, base de datos, cola y cargas propios; enrutado por token; administración con token de servicio; puntos de acceso con puerto o ruta `/p/<nombre>`, recarga sin reinicio y edición del YAML conservando comentarios; descriptor `servicio.json`. |
| `internal/cliente` | Cliente del servicio para la CLI y la terminal: descubre el servicio por su descriptor, lo inicia en segundo plano y sigue los eventos SSE. |
| `internal/tui` | Terminal interactiva con Bubble Tea v2. |
| `internal/recursos` | Web UI compilada incorporada con `go:embed`. Se versiona para que `go build` no necesite Node. |
| `internal/navegador` | Abre URLs en el navegador del sistema. |
| `internal/credenciales` | Claves de los proveedores en el almacén del sistema: Administrador de credenciales (Windows), Llavero (macOS, `/usr/bin/security`) y Secret Service (Linux, `secret-tool`). |
| `web/` | Web UI con Vue 3 y Quasar 2.35 (Vite). La compila `npm run build` en `internal/recursos/web`. |
| `escritorio/` | Aplicación de escritorio con Wails v3 en un módulo Go aparte, para que el núcleo siga sin CGO. Es un cliente del servicio: lo inicia si hace falta y muestra la misma Web UI en su ventana. |
| `internal/version` | Versión del ejecutable (inyectada al compilar) y commit de git. |
| `cmd/tramtai` | CLI con Cobra. Sin argumentos abre la terminal interactiva. Comandos: `servicio`, `global`, `proyecto`, `iniciar`, `acceso`, `claves`, `trabajos`, `ejecutar`, `convertir`, `agentes`, `configuracion`. Detalle en [operacion.md](operacion.md). |

## Agentes

| | Claude Code | Codex CLI |
|---|---|---|
| Invocación | `claude -p --output-format stream-json`, prompt por stdin | `codex exec --json -`, prompt por stdin |
| Aislamiento | `--setting-sources ""`, `--strict-mcp-config`, `--tools ""`, `--no-session-persistence` y espacio de trabajo temporal vacío. Con clave de API, también `--bare`. | `--ephemeral`, `--ignore-rules`, `-s read-only`, shell y búsqueda web desactivadas. Con clave de API, `CODEX_HOME` privado y vacío. |
| Sistema | `--system-prompt-file`: sustituye el prompt de Claude Code (≈650 tokens fijos en vez de ≈24 000) | primer bloque delimitado del prompt |
| Streaming | por tokens (`text_delta`) | solo el mensaje final |
| Modelo efectivo | sí (`modelUsage`) | no: queda como no verificable |
| Uso | tokens y coste en USD | tokens |

Además hay dos adaptadores de Google, probados con un servidor y un CLI simulados:

- **`gemini`:** Gemini API directa por HTTP con `GEMINI_API_KEY`, que viaja en cabecera, nunca en la URL.
  - Streaming por fragmentos (`streamGenerateContent`) y sistema en `systemInstruction`.
  - Salida estructurada nativa (`responseJsonSchema`), modelo efectivo (`modelVersion`) y uso.
  - Perfiles: `gemini-flash-lite-latest`, `gemini-flash-latest` y `gemini-pro-latest`.
- **`antigravity`:** `agy -p --output-format json`, experimental.
  - Funciona solo con `modelProvider: gemini` y la clave del usuario, en un `HOME` privado y temporal; rechaza la suscripción por los términos de Antigravity.
  - Usa `structured_output` si hay esquema; no hay streaming ni modelo efectivo.
  - Recibe el prompt como argumento (hasta 28 000 caracteres), así que otros procesos del equipo pueden verlo en la lista de procesos mientras se ejecuta.
  - **Sin restricción de herramientas validada:** a diferencia de Claude y Codex, a `agy` no se le desactivan herramientas, shell ni MCP, porque ninguna opción (`permissions.deny`, `toolPermission: strict`) se ha comprobado aún con el CLI real. Solo lo limitan el `HOME` privado y la carpeta temporal vacía; en `consciente_proyecto` trabaja en la raíz del proyecto. Úsalo solo con prompts de confianza.

Las conversaciones de varios turnos se enmarcan con el formato `tramtai-turnos-v2`: un bloque `<<<turno-<código> rol=…>>>` por turno, con el contenido intacto y un código aleatorio por petición para que un mensaje no pueda cerrar su turno. Los errores del proveedor se traducen a `limite_uso_proveedor`, `autenticacion_agente`, `modelo_no_disponible` o `error_agente`. Ningún secreto llega a los mensajes: se redactan antes.

## Entradas y salidas

Con cargas, el trabajo pasa por `preparando_entrada`, que tiene su propio límite de conversiones simultáneas (`concurrencia.maximo_conversiones_instancia`). Cada archivo se convierte por separado y se antepone al último mensaje del usuario en un bloque delimitado:

```
<<<archivo-3f9a1c2e nombre="paquete.zip/acta.docx" sha256="…">>>
…Markdown…
<<<fin archivo-3f9a1c2e>>>
```

El código tras el guion es aleatorio por petición (igual en los turnos `<<<turno-…>>>`): un documento que escriba
`<<<fin archivo>>>` no cierra su bloque.

Si una carga venció, el trabajo falla con `entrada_no_disponible`. Una conversión vacía o fallida falla con `conversion_fallida`; nunca se presenta como éxito. Un ZIP que supera los límites falla con `carga_invalida`.

Con un esquema, se usa primero la salida estructurada nativa del agente (`--json-schema` y `structured_output` en Claude; `--output-schema` en Codex) y después Tramtai valida el resultado. Si no cumple, el trabajo falla con `salida_no_cumple_esquema` y la salida solo se conserva con `conservar_salida_invalida`. Un esquema inválido se rechaza al aceptar el trabajo.

## Modelo de amenazas

Lo que llega de una app (mensaje, instrucciones, documentos, esquema) se trata como hostil. No se puede impedir que
intente manipular al modelo; sí que un modelo manipulado haga daño a través de Tramtai:

- **Agentes:** Claude Code sin herramientas, sin configuración del usuario y con el mensaje por stdin; Codex en solo
  lectura, sin terminal ni búsqueda web, en una carpeta temporal y con el mensaje por stdin; Gemini es HTTP. Las pruebas
  canario lo comprueban con los CLI reales. Antigravity, sin esa comprobación, queda fuera de las apps; su mensaje nunca
  empieza por `-` y su configuración es privada.
- **Entrada:** delimitadores con código por petición; topes de mensajes (1.000), esquema (256 KB), cuerpo (16 MB) y
  la comprobación del contexto antes de llamar al CLI.
- **Cargas y conversión:** identificadores por API, cuota por instancia, solo formatos anunciados, ZIP con límites de
  archivos, tamaño, profundidad y rutas, Office comprobado antes de convertir, plazo por archivo y total, y tope de
  memoria para markitdown.
- **Procesos:** cada CLI en un grupo que se termina entero (Job Object en Windows), sin shell, con un tope de 8 MB por
  línea de salida y sin los secretos del entorno (claves, contraseñas, URL con credenciales, `TRAMTAI_*`).
- **Salida:** las claves de proveedor se tapan; la Web UI muestra Markdown solo como HTML limpio (DOMPurify) bajo una
  CSP estricta; un error del CLI no puede escribir líneas falsas en el registro.

## Interfaces

Las cuatro interfaces trabajan sobre el mismo servicio, así que un trabajo creado en una se ve y se controla en las demás:

- **CLI:** `tramtai ejecutar` usa el servicio si está en marcha (`--directo` para no usarlo).
- **Terminal interactiva:** `tramtai` sin argumentos.
- **Web UI:** servida por el propio servicio en `http://127.0.0.1:17373`.
- **Aplicación de escritorio:** `tramtai-escritorio`.

La Web UI se enlaza a una instancia con una sesión:

- **Entrada sin clave en el propio equipo:** si la conexión llega por loopback desde un programa del mismo usuario del sistema (el navegador o la app de escritorio), la Web UI entra sola y puede cambiar entre instancias. En Windows se identifica al dueño de la conexión con la tabla TCP del sistema y el SID de su proceso; en Linux, con el uid de `/proc/net/tcp`. No se aplica si la petición trae cabeceras de proxy (`Forwarded`, `X-Forwarded-For`, `X-Forwarded-Host`, `X-Real-Ip`, `Via`), ni con `servicio.permitir_remoto`, porque un proxy o túnel del mismo usuario reenviaría peticiones de otros equipos. Se desactiva con `servicio.entrada_local: false`.
- **Código de un solo uso:** la otra forma de entrar. Lo piden `tramtai abrir`, la terminal interactiva o la app de escritorio con el token de servicio (un minuto de validez). Es lo que se usa en macOS, donde no se identifica al dueño de la conexión. Un código canjeado desde este equipo (loopback y sin cabeceras de proxy) abre una sesión local, como la entrada directa: solo el dueño del servicio puede pedirlo. Con el servicio expuesto en red, la Web UI remota solo se abre con un código de `tramtai abrir` hecho en el equipo del servicio, y esa sesión no es local.
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
| De la instancia y token de servicio | En claro, en archivos con permisos 0600, porque la CLI y la app los leen. La de la instancia se regenera en marcha (`POST /api/v1/instancia/token`), sin reiniciar. |
| De los proveedores | Cifradas en el almacén del sistema: Administrador de credenciales (Windows), Llavero vía `/usr/bin/security` (macOS) o Secret Service vía `secret-tool` (Linux). Tramtai nunca las escribe en sus archivos. Se leen en cada petición, así que un cambio vale sin reiniciar; una variable de entorno con el mismo nombre tiene prioridad. |

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
| antigravity | Trabaja en la raíz del proyecto y carga `AGENTS.md` y `GEMINI.md`. |
| gemini | No lo admite, porque no tiene proyecto. |

**Límites comprobados (Claude):**
- Carga también los `CLAUDE.md` de las carpetas superiores.
- Con suscripción, carga además el `CLAUDE.md` del usuario.
- Con clave de API usa un `CLAUDE_CONFIG_DIR` privado, así que las reglas del usuario no se cargan.

**En todos los agentes:** siguen desactivados las herramientas, MCP y la sesión persistente (salvo en antigravity, que no tiene restricción de herramientas validada; ver arriba), y antes de ejecutar se registra un `aviso` con el contexto que se carga.

## Supervisión y recuperación

- **Firma de inicio:** cada agente registra su PID y su firma de inicio (migración v5).
- **Al arrancar:** el servicio termina los procesos huérfanos de una caída, solo si su firma coincide; un PID reutilizado nunca se toca.
  - Windows: el Job Object ya los cerró.
  - Unix: se mata su grupo de procesos.
- **Seguir un trabajo:** el SSE espera avisos en memoria del almacén en lugar de sondear la base; el sondeo queda como respaldo cada 5 s.

## Precedencia de configuración

De menor a mayor: interno → global → proyecto → local → entorno → punto de acceso → solicitud. Las claves de seguridad (`servicio`, `comandos_agentes`, `adaptadores`, `concurrencia_servicio`…) se rechazan en la capa de proyecto, que es la que va en el repositorio, y en `local.yaml` salvo que la carpeta figure en `proyectos_confiables` de la configuración global (un `local.yaml` también puede llegar con un clon). `proyectos_confiables` solo se lee de la global.

Los agentes no heredan secretos del entorno: se retiran las credenciales conocidas de los proveedores, todas las `TRAMTAI_*` (el token del servicio) y cualquier variable cuyo nombre contenga `API_KEY`, `TOKEN`, `SECRET`, `PASSWORD` o `CREDENTIAL` (`GITHUB_TOKEN`, `AWS_SECRET_ACCESS_KEY`…). Cada CLI solo conserva las variables secretas con los prefijos de su propio proveedor (`CLAUDE_` y `ANTHROPIC_` para Claude, `CODEX_` y `OPENAI_` para Codex, `GEMINI_` para Antigravity), y la clave que necesita se la pasa Tramtai según el modo de autenticación. La consulta `--version` se ejecuta sin ninguna clave.

## Cumplimiento de proveedores

Por defecto cada adaptador usa clave de API. El modo suscripción es opcional, solo local y sin acceso remoto. Google se usará vía Gemini API directa; `agy` solo con `modelProvider: gemini` + `GEMINI_API_KEY`.
