# Tramtai

Pasarela local en Go que expone Claude Code, Codex CLI y Antigravity (y la Gemini API) como una API HTTP propia, con cola de trabajos, idempotencia, configuración por capas y selección de modelo por solicitud.

> Estado: **M1 completo** (núcleo, configuración, SQLite, planificador). El servidor HTTP y los adaptadores reales llegan en M2–M3. Hoy se puede compilar, ejecutar la CLI y correr las pruebas.

## Inicio rápido

```powershell
./scripts/compilar.ps1                       # genera bin/tramtai.exe
./bin/tramtai.exe version
./bin/tramtai.exe configuracion mostrar --efectiva
go test ./...
```

Guía completa en [docs/inicio.md](docs/inicio.md).

## Documentación

| Documento | Contenido |
|---|---|
| [docs/inicio.md](docs/inicio.md) | Requisitos, compilar, ejecutar y probar |
| [docs/arquitectura.md](docs/arquitectura.md) | Paquetes, estados del trabajo y precedencia de configuración |
| [docs/versionado.md](docs/versionado.md) | Esquema `vX.Y.Z+DDMMAAAA` y cómo crear etiquetas |
| [docs/pendientes.md](docs/pendientes.md) | Hitos y lista de tareas (incluye CI/CD) |
| [docs/M0B.md](docs/M0B.md) | Validación de los modos con clave de API |

El documento maestro (Word) vive en `documentos/` y no se versiona.

## Estructura

```
cmd/tramtai/          CLI (Cobra)
internal/nucleo/      estados, solicitud, errores, configuración
internal/persistencia SQLite (trabajos, eventos, idempotencia)
internal/planificador cola, concurrencia, cancelación, recuperación
internal/adaptadores  contrato de adaptador + agente simulado
pruebas/m0b/          pruebas con claves reales (build tag m0b)
scripts/              compilar y versionar
docs/                 documentación
```
