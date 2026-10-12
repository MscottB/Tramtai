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
| `internal/version` | Versión del ejecutable (inyectada al compilar) y commit de git. |
| `cmd/tramtai` | CLI con Cobra: `version`, `configuracion`, `agentes` y `ejecutar`. |

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
