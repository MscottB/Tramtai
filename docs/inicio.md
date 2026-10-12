# Inicio

## Requisitos

- Go 1.27 (instalado en `C:\Program Files\Go\bin`). Si `go` no se reconoce, añádelo al PATH del usuario:

  ```powershell
  [Environment]::SetEnvironmentVariable('Path', "$([Environment]::GetEnvironmentVariable('Path','User'));C:\Program Files\Go\bin;$env:USERPROFILE\go\bin", 'User')
  ```

  y abre una terminal nueva. `scripts/compilar.ps1` lo añade por su cuenta solo para esa ejecución.
- Git.
- Los agentes que vayas a usar: `claude` (Claude Code) y/o `codex` (Codex CLI), instalados como `.exe`.

## Compilar

```powershell
./scripts/compilar.ps1
```

Genera `bin/tramtai.exe` con la versión calculada desde git (ver [versionado.md](versionado.md)).

## Ejecutar

```powershell
./bin/tramtai.exe --version
./bin/tramtai.exe agentes                                     # versión, autenticación y términos de cada agente
./bin/tramtai.exe ejecutar "Resume en una frase qué es Go"    # agente y modelo predeterminados
./bin/tramtai.exe ejecutar -a claude -p rapido "Hola"         # perfil de modelo (rapido = haiku)
./bin/tramtai.exe ejecutar -a claude -m sonnet -s "Responde en inglés" "Hola"
./bin/tramtai.exe ejecutar -a codex "Hola"
Get-Content prompt.txt | ./bin/tramtai.exe ejecutar -a claude  # prompt desde stdin
```

`ejecutar` muestra la respuesta en vivo (Claude por tokens; Codex al final) y un resumen con el modelo efectivo, los tokens y el coste. Ctrl+C cancela el trabajo y termina el árbol de procesos del agente. Los trabajos se guardan en `%APPDATA%\tramtai\datos\cli.db`.

### Autenticación de los agentes

Por defecto cada agente usa **clave de API** (sección 2.4 del documento maestro):

| Agente | Variable |
|---|---|
| claude | `ANTHROPIC_API_KEY` |
| codex | `CODEX_API_KEY` u `OPENAI_API_KEY` |

Para uso **personal y local** con tu suscripción (Claude Pro/Max, cuenta ChatGPT), actívalo explícitamente en la configuración global `%APPDATA%\tramtai\config.yaml`:

```yaml
adaptadores:
  claude:
    autenticacion: suscripcion
  codex:
    autenticacion: suscripcion
```

o solo para una terminal: `$env:TRAMTAI_ADAPTADORES__CLAUDE__AUTENTICACION = 'suscripcion'`. La configuración del proyecto (`.tramtai/config.yaml`) no puede activarlo. Tramtai nunca lee ni copia las credenciales de los CLIs.

### Modelos

- `-m` pasa un identificador o alias literal (`haiku`, `claude-sonnet-5-5`, el modelo de Codex…). No es una lista cerrada: el CLI decide si existe.
- `-p` usa un perfil de `modelos.perfiles`. Por defecto: `rapido`/`equilibrado`/`profundo` → `haiku`/`sonnet`/`opus` para Claude. Para Codex hay que definirlos:

  ```yaml
  modelos:
    perfiles:
      rapido:
        codex: <identificador>
  ```

- Si un perfil no tiene modelo para el agente, el trabajo se rechaza con `modelo_no_configurado`. Nunca se cambia el modelo en silencio.

## Configuración

Se lee de (menor a mayor prioridad): valores internos, `%APPDATA%\tramtai\config.yaml`, `.tramtai/config.yaml`, `.tramtai/local.yaml` y variables `TRAMTAI_SECCION__CLAVE`.

```powershell
./bin/tramtai.exe configuracion mostrar --efectiva     # cada valor con su capa de origen
./bin/tramtai.exe configuracion explicar adaptadores.claude.autenticacion
```

## Probar

```powershell
go vet ./...
go test ./...                                   # sin agentes reales: simulado, procesos, persistencia
$env:TRAMTAI_PRUEBA_AUTENTICACION = 'suscripcion'
go test -tags agentes -v ./internal/adaptadores # CLIs reales: respuesta, uso y cancelación (consume cuota)
go test -tags m0b -v -count=1 ./pruebas/m0b     # modos con clave de API, ver M0B.md
```

## Qué falta para "verlo" como servicio

El servidor HTTP (`/api/v1`, SSE) llega en M4 y la interfaz (Web UI y aplicación de escritorio Wails + Quasar) en M6. Hasta entonces se usa `tramtai ejecutar`.
