# Arquitectura

El diseño completo está en el documento maestro (v12, en `documentos/`). Resumen de lo implementado:

## Paquetes

| Paquete | Responsabilidad |
|---|---|
| `internal/nucleo` | Tipos de dominio: `SolicitudEjecucion` (validación y huella SHA-256), `Trabajo`, estados y transiciones, códigos de error estables, configuración por capas. |
| `internal/persistencia` | SQLite en modo WAL sin CGO (`modernc.org/sqlite`). Tablas `trabajos`, `eventos`, `idempotencia`. Aceptación atómica y recuperación tras reinicio. |
| `internal/planificador` | Cola acotada, concurrencia por instancia, semáforo por adaptador, tiempos máximos de cola y ejecución, cancelación idempotente. |
| `internal/adaptadores` | Interfaz `Adaptador` y agente `Simulado` para pruebas. Los adaptadores reales (Claude, Codex, Gemini, Antigravity) llegan en M2–M3. |
| `cmd/tramtai` | CLI con Cobra. |

## Estados del trabajo

```
en_cola → preparando_entrada → esperando_agente → ejecutando → validando_salida → completado
                                                                                 ↘ fallido
cualquiera no terminal → cancelacion_solicitada → cancelado
ejecutando al reiniciar el servicio → interrumpido   (no se relanza)
en_cola al reiniciar → se reencola
```

## Precedencia de configuración

De menor a mayor: interno → global → proyecto → local → entorno → punto de acceso → solicitud. Las claves de seguridad (`prefijosAdministrados`) se rechazan en la capa de proyecto, que es la que va en el repositorio.

## Cumplimiento de proveedores

Por defecto cada adaptador usa clave de API. El modo suscripción es opcional, solo local y sin acceso remoto. Google se usa vía Gemini API directa; `agy` solo con `modelProvider: gemini` + `GEMINI_API_KEY`. Detalle en §2.4 del documento maestro y en [M0B.md](M0B.md).
