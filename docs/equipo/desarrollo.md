# Guía del equipo

Documentación interna para desarrollar Tramtai. La de usuario está en el [README](../../README.md) y en [docs/](../inicio.md).

| Documento | Contenido |
|---|---|
| [pendientes.md](pendientes.md) | Hitos y lista de tareas (incluye CI/CD) |
| [versionado.md](versionado.md) | Versión `vX.Y.Z+DDMMAAAA` calculada desde los commits, y etiquetas con notas |
| [M0B.md](M0B.md) | Validación de los modos con clave de API (pendiente con claves reales) |

El documento maestro (Word) vive en `documentos/` y no se versiona.

## Compilar

```powershell
./scripts/compilar.ps1     # bin/tramtai.exe y bin/tramtai-escritorio.exe, con icono y versión en sus propiedades
go run ./scripts/icono     # regenera el icono (escritorio/icono/tramtai.png); compilar.ps1 genera el .ico y los .syso
```

`compilar.ps1` necesita `wails3` (en `%USERPROFILE%\go\bin`) para el icono y la versión de los `.exe`; sin él compila igual, sin esos recursos.

## Web UI

```powershell
cd web
npm install
npm run dev      # servidor de desarrollo que llama al servicio en marcha (tramtai servicio iniciar -d)
npm run build    # compila en internal/recursos/web, que se incorpora al ejecutable con go:embed
```

La compilación de la Web UI se versiona para que `go build` no necesite Node.

**Idiomas.** Los textos están en `web/src/idiomas/`: `es.ts` es el principal y define las claves; `en.ts` tiene el tipo `Textos`, así que `npx tsc --noEmit -p .` falla si le falta alguna. Para añadir un idioma, crea `idiomas/<código>.ts` con el mismo tipo y añádelo a `idiomas` en `web/src/i18n.ts` junto al paquete de Quasar (`quasar/lang/<código>`). La preferencia se recuerda en el navegador; sin ella se usa la del navegador y, si no está disponible, español. La CLI, la terminal y los mensajes de error del servicio siguen solo en español.

## Probar

```powershell
go vet ./...
go test ./...                                    # sin agentes reales: simulado, procesos, persistencia, servicio
$env:CGO_ENABLED = '1'; go test -race ./...      # necesita un compilador C (llvm-mingw, instalado con winget)
$env:TRAMTAI_PRUEBA_AUTENTICACION = 'suscripcion'
go test -tags agentes -v ./internal/adaptadores  # CLIs reales: respuesta, uso y cancelación (consume cuota)
go test -tags m0b -v -count=1 ./pruebas/m0b      # modos con clave de API, ver M0B.md
```

Para probar el servicio con la suscripción sin tocar la configuración global, arráncalo con `TRAMTAI_ADAPTADORES__CLAUDE__AUTENTICACION=suscripcion` (y la de Codex) en el entorno.

## Estructura

```
cmd/tramtai/           CLI (Cobra); sin argumentos abre la terminal interactiva
internal/nucleo/       estados, solicitud, errores, configuración por capas y perfiles
internal/persistencia/ SQLite (trabajos, eventos, idempotencia, retención)
internal/planificador/ cola, concurrencia, cancelación, drenaje y recuperación
internal/procesos/     supervisión de procesos (Job Objects / grupos Unix)
internal/adaptadores/  Claude, Codex, Gemini API, Antigravity y agente simulado
internal/credenciales/ claves de API en el Administrador de credenciales de Windows
internal/documentos/   cargas, conversión con markitdown y ZIP seguro
internal/salidas/      validación JSON Schema
internal/api/          HTTP: API nativa, SSE y fachadas OpenAI/Anthropic
internal/servicio/     servicio único: instancias, puntos de acceso, Web UI y entrada local
internal/cliente/      cliente del servicio (CLI, terminal y escritorio)
internal/tui/          terminal interactiva (Bubble Tea)
internal/recursos/     Web UI compilada (go:embed)
web/                   Web UI: Vue 3 + Quasar
escritorio/            aplicación de escritorio Wails v3 (módulo Go aparte) e icono
pruebas/m0b/           pruebas con claves reales (build tag m0b)
scripts/               compilar, versionar e icono
```

## Publicar una versión

Commits con prefijo `feat:`, `fix:` o `docs:` y la lista de cambios en el cuerpo; después:

```powershell
./scripts/version.ps1 -Etiquetar -Resumen "…"
git push origin HEAD; git push origin --tags
```
