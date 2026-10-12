# Arquitectura

El diseño completo está en el documento maestro (v12, en `documentos/`). Resumen de lo implementado:

## Paquetes

| Paquete | Responsabilidad |
|---|---|
| `internal/nucleo` | Tipos de dominio: `SolicitudEjecucion` (validación y huella SHA-256), `Trabajo` (modelo enviado, modelo efectivo y uso), estados y transiciones, códigos de error estables, configuración por capas y perfiles de modelo. |
| `internal/persistencia` | SQLite en modo WAL sin CGO (`modernc.org/sqlite`) con migraciones numeradas (`PRAGMA user_version`). Aceptación atómica, idempotencia, eventos numerados y recuperación tras reinicio. |
| `internal/planificador` | Cola acotada, concurrencia por instancia, semáforo por adaptador, tiempos máximos, cancelación idempotente y resolución del perfil de modelo al aceptar. |
| `internal/procesos` | Supervisión de procesos. En Windows crea el proceso suspendido, lo asigna a un Job Object con `KILL_ON_JOB_CLOSE` y lo reanuda; en Unix usa un grupo de procesos con SIGTERM y luego SIGKILL. Cancelar o agotar el tiempo termina el árbol completo; si no se confirma, devuelve `cancelacion_no_confirmada`. |
| `internal/adaptadores` | Contrato `Adaptador`, agente `Simulado` y los adaptadores reales `Claude` y `Codex` (M2). Antigravity y la Gemini API llegan después. |
| `internal/documentos` | Cargas identificadas por SHA-256 (`carga_<hash>`) con vencimiento. Conversión a Markdown con markitdown 0.1.8 en un proceso supervisado con plazo y límite de salida. Los archivos de texto (txt, md, json, xml, csv, yaml…) pasan tal cual, validando que sean UTF-8. ZIP seguro con límites de archivos, tamaño descomprimido medido y profundidad, rechazando rutas con `..`, absolutas, con unidad o enlaces. |
| `internal/salidas` | Validación de la salida contra el JSON Schema del consumidor (`santhosh-tekuri/jsonschema`), con los `$ref` externos deshabilitados. |
| `internal/api` | Servicio HTTP (sección 7 y 8): valida Host, Origin y token; API nativa con SSE reanudable por `Last-Event-ID`; fachadas OpenAI y Anthropic con política de parámetros y su formato de errores. Detalle en [api.md](api.md). |
| `internal/servicio` | Servicio único (sección 5): instancias global y por proyecto con token, base de datos, cola y cargas propios; enrutado por token; administración con token de servicio; puntos de acceso con puerto o ruta `/p/<nombre>`, recarga sin reinicio y edición del YAML conservando comentarios; descriptor `servicio.json`. |
| `internal/cliente` | Cliente del servicio para la CLI y la terminal: descubre el servicio por su descriptor, lo inicia en segundo plano y sigue los eventos SSE. |
| `internal/tui` | Terminal interactiva con Bubble Tea v2 (sección 13.2). |
| `internal/recursos` | Web UI compilada incorporada con `go:embed`. Se versiona para que `go build` no necesite Node. |
| `internal/navegador` | Abre URLs en el navegador del sistema. |
| `web/` | Web UI con Vue 3 y Quasar 2.35 (Vite). La compila `npm run build` en `internal/recursos/web`. |
| `escritorio/` | Aplicación de escritorio con Wails v3 en un módulo Go aparte, para que el núcleo siga sin CGO. Es un cliente del servicio: lo inicia si hace falta y muestra la misma Web UI en su ventana. |
| `internal/version` | Versión del ejecutable (inyectada al compilar) y commit de git. |
| `cmd/tramtai` | CLI con Cobra. Sin argumentos abre la terminal interactiva. Comandos: `servicio`, `global`, `proyecto`, `iniciar`, `acceso`, `trabajos`, `ejecutar`, `convertir`, `agentes`, `configuracion`. Detalle en [operacion.md](operacion.md). |

## Adaptadores (M2)

| | Claude Code | Codex CLI |
|---|---|---|
| Invocación | `claude -p --output-format stream-json`, prompt por stdin | `codex exec --json -`, prompt por stdin |
| Aislamiento | `--setting-sources ""`, `--strict-mcp-config`, `--tools ""`, `--no-session-persistence` y espacio de trabajo temporal vacío. Con clave de API, también `--bare`. | `--ephemeral`, `--ignore-rules`, `-s read-only`, shell y búsqueda web desactivadas. Con clave de API, `CODEX_HOME` privado y vacío. |
| Sistema | `--system-prompt-file`: sustituye el prompt de Claude Code (≈650 tokens fijos en vez de ≈24 000) | primer bloque delimitado del prompt |
| Streaming | por tokens (`text_delta`) | solo el mensaje final |
| Modelo efectivo | sí (`modelUsage`) | no: queda como no verificable |
| Uso | tokens y coste en USD | tokens |

Las conversaciones de varios turnos se enmarcan con el formato `tramtai-turnos-v1`: un bloque `<<<turno rol=…>>>` por turno, con el contenido intacto. Los errores del proveedor se traducen a `limite_uso_proveedor`, `autenticacion_agente`, `modelo_no_disponible` o `error_agente`. Ningún secreto llega a los mensajes: se redactan antes.

## Entradas y salidas (M3)

Con cargas, el trabajo pasa por `preparando_entrada`, que tiene su propio límite de conversiones simultáneas (`concurrencia.maximo_conversiones_instancia`). Cada archivo se convierte por separado y se antepone al último mensaje del usuario en un bloque delimitado:

```
<<<archivo nombre="paquete.zip/acta.docx" sha256="…">>>
…Markdown…
<<<fin archivo>>>
```

Si una carga venció, el trabajo falla con `entrada_no_disponible`. Una conversión vacía o fallida falla con `conversion_fallida`; nunca se presenta como éxito. Un ZIP que supera los límites falla con `carga_invalida`.

Con un esquema, se usa primero la salida estructurada nativa del agente (`--json-schema` y `structured_output` en Claude; `--output-schema` en Codex) y después Tramtai valida el resultado. Si no cumple, el trabajo falla con `salida_no_cumple_esquema` y no se conserva la salida. Un esquema inválido se rechaza al aceptar el trabajo.

## Interfaces (M6)

Las cuatro interfaces trabajan sobre el mismo servicio, así que un trabajo creado en una se ve y se controla en las demás:

- **CLI:** `tramtai ejecutar` usa el servicio si está en marcha (`--directo` para no usarlo).
- **Terminal interactiva:** `tramtai` sin argumentos.
- **Web UI:** servida por el propio servicio en `http://127.0.0.1:17373`.
- **Aplicación de escritorio:** `tramtai-escritorio`.

La Web UI se enlaza a una instancia con una sesión (sección 7.4):

- **Entrada:** se entra con el token de la instancia, o con un código de un solo uso que dura un minuto y piden la CLI, la terminal o la app de escritorio con el token de servicio.
- **Cookie:** `HttpOnly` y `SameSite=Strict`; las peticiones que modifican exigen además `X-CSRF-Token`.
- **Navegador:** se valida `Origin`, no hay CORS y se aplica una CSP estricta.
- **El token nunca viaja en la URL** y no se guarda en el navegador.
- **Configuración y puntos de acceso:** solo se gestionan con una sesión de la Web UI o el token de servicio, no con el token que usan las aplicaciones.

## Estados del trabajo

```
en_cola → preparando_entrada → esperando_agente → ejecutando → validando_salida → completado
                                                                                 ↘ fallido
cualquiera no terminal → cancelacion_solicitada → cancelado
ejecutando al reiniciar el servicio → interrumpido   (no se relanza)
en_cola al reiniciar → se reencola
```

## Precedencia de configuración

De menor a mayor: interno → global → proyecto → local → entorno → punto de acceso → solicitud. Las claves de seguridad (`servicio`, `comandos_agentes`, `adaptadores`, `concurrencia_servicio`…) se rechazan en la capa de proyecto, que es la que va en el repositorio.

## Cumplimiento de proveedores

Por defecto cada adaptador usa clave de API. El modo suscripción es opcional, solo local y sin acceso remoto. Google se usará vía Gemini API directa; `agy` solo con `modelProvider: gemini` + `GEMINI_API_KEY`. Detalle en §2.4 del documento maestro y en [M0B.md](M0B.md).
