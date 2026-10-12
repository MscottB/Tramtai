# Tramtai

Pasarela local en Go que expone Claude Code, Codex CLI y Antigravity (y la Gemini API) como una API HTTP propia, con cola de trabajos, idempotencia, configuración por capas y selección de modelo por solicitud.

> Estado: **M5 completo**: servicio único con instancias global y por proyecto separadas por token, puntos de acceso (puerto o ruta propios, alias, modelos permitidos o forzados), terminal interactiva, API nativa y fachadas OpenAI/Anthropic, adaptadores Claude y Codex, archivos y salidas con JSON Schema. La Web UI y la aplicación de escritorio llegan en M6.

## Inicio rápido

```powershell
./scripts/compilar.ps1                       # genera bin/tramtai.exe
./bin/tramtai.exe                            # terminal interactiva: todo desde un menú
./bin/tramtai.exe --version
./bin/tramtai.exe agentes                    # diagnóstico de Claude y Codex
./bin/tramtai.exe ejecutar -p rapido "Hola"   # trabajo real con respuesta en vivo
./bin/tramtai.exe ejecutar -f acta.pdf -e compromisos.schema.json "Extrae los compromisos"
./bin/tramtai.exe servicio iniciar -d          # servicio en segundo plano en http://127.0.0.1:17373
go test ./...
```

Guía completa en [docs/inicio.md](docs/inicio.md).

## Documentación

| Documento | Contenido |
|---|---|
| [docs/inicio.md](docs/inicio.md) | Requisitos, compilar, ejecutar y probar |
| [docs/operacion.md](docs/operacion.md) | Terminal interactiva, servicio, instancias global y por proyecto, puntos de acceso |
| [docs/api.md](docs/api.md) | API HTTP: rutas, seguridad, fachadas OpenAI y Anthropic, errores |
| [docs/arquitectura.md](docs/arquitectura.md) | Paquetes, estados del trabajo y precedencia de configuración |
| [docs/versionado.md](docs/versionado.md) | Versión `vX.Y.Z+DDMMAAAA` calculada desde los commits y etiquetas |
| [docs/pendientes.md](docs/pendientes.md) | Hitos y lista de tareas (incluye CI/CD) |
| [docs/M0B.md](docs/M0B.md) | Validación de los modos con clave de API |

El documento maestro (Word) vive en `documentos/` y no se versiona.

## Estructura

```
cmd/tramtai/          CLI (Cobra)
internal/nucleo/      estados, solicitud, errores, configuración
internal/persistencia SQLite (trabajos, eventos, idempotencia)
internal/planificador cola, concurrencia, cancelación, recuperación
internal/procesos     supervisión de procesos (Job Objects / grupos Unix)
internal/adaptadores  Claude, Codex y agente simulado
internal/documentos   cargas, conversión con markitdown y ZIP seguro
internal/salidas      validación JSON Schema
internal/api          HTTP: API nativa, SSE y fachadas OpenAI/Anthropic
internal/servicio     servicio único: instancias, puntos de acceso, administración
internal/cliente      cliente del servicio (CLI y terminal)
internal/tui          terminal interactiva (Bubble Tea)
internal/version      versión y commit del ejecutable
pruebas/m0b/          pruebas con claves reales (build tag m0b)
scripts/              compilar y versionar
docs/                 documentación
```
