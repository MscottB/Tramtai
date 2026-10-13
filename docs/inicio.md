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

### Archivos y salida estructurada

```powershell
./bin/tramtai.exe convertir informe.pdf datos.zip              # solo muestra el texto que recibiría el agente
./bin/tramtai.exe ejecutar -f informe.pdf -f anexo.xlsx "Resume ambos documentos"
./bin/tramtai.exe ejecutar -f acta.docx -e compromisos.schema.json "Extrae compromisos; null si no hay fecha"
```

- `-f` es repetible y admite PDF, DOCX, PPTX, XLSX, HTML, texto (txt, md, json, xml, csv…) y ZIP. Los archivos de texto se entregan tal cual. El resto se convierte con markitdown, que debe estar instalado: `uv tool install "markitdown[pdf,docx,pptx,xlsx]==0.1.8"`. Si no lo está, Tramtai usa `uvx`, más lento.
- `-e` recibe un JSON Schema; la salida es JSON validado contra él.
- Límites configurables en `conversion.*`: `maximo_carga_mb` (50), `maximo_archivos_zip` (50), `maximo_descomprimido_mb` (200), `profundidad_maxima_zip` (3), `tiempo_limite_segundos` (120) y `maximo_salida_mb` (20).

`ejecutar` muestra la respuesta en vivo (Claude por tokens; Codex al final) y un resumen con el modelo efectivo, los tokens y el coste. Ctrl+C cancela el trabajo y termina el árbol de procesos del agente. Los trabajos se guardan en `%APPDATA%\tramtai\datos\cli.db`.

### Autenticación de los agentes

Por defecto cada agente usa **clave de API** (sección 2.4 del documento maestro):

| Agente | Variable |
|---|---|
| claude | `ANTHROPIC_API_KEY` |
| codex | `CODEX_API_KEY` u `OPENAI_API_KEY` |
| gemini (Gemini API directa) | `GEMINI_API_KEY` |
| antigravity (`agy`, experimental) | `GEMINI_API_KEY`; nunca la sesión personal |

En Windows, guarda las claves en el Administrador de credenciales en lugar de dejarlas en variables de entorno:

```powershell
tramtai claves guardar ANTHROPIC_API_KEY   # la pide sin mostrarla
tramtai claves listar                      # de dónde sale cada una, sin mostrarla
tramtai claves borrar ANTHROPIC_API_KEY
```

Una variable de entorno con el mismo nombre tiene prioridad. Reinicia el servicio después de cambiar una clave.

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

## Uso diario: la terminal interactiva

```powershell
./bin/tramtai.exe          # o: tramtai terminal
```

Desde el menú se elige el ámbito (proyecto actual, global, todas las instancias o servicio) y se ejecuta, se ven los trabajos y se gestionan los puntos de acceso sin escribir comandos. Si el servicio no está en marcha, la terminal lo inicia en segundo plano. Detalle en [operacion.md](operacion.md).

## Web UI y escritorio

```powershell
./bin/tramtai.exe global abrir          # Web UI en el navegador, sin escribir el token
./bin/tramtai-escritorio.exe            # aplicación de escritorio
```

Para cambiar la Web UI: `cd web; npm install; npm run dev`. Esto abre un servidor de desarrollo que llama al servicio en marcha. Después, `npm run build` y `./scripts/compilar.ps1` la incorporan al ejecutable.

## Servicio HTTP

```powershell
./bin/tramtai.exe servicio iniciar -d  # segundo plano (sin -d: primer plano, Ctrl+C lo detiene)
./bin/tramtai.exe global token         # token de la instancia global para los clientes
./bin/tramtai.exe iniciar              # en un repositorio: crea .tramtai/config.yaml
./bin/tramtai.exe proyecto iniciar     # instancia del proyecto, con su propio token
```

Configura tus aplicaciones con `base_url` `http://127.0.0.1:17373/openai/v1` (OpenAI) o `http://127.0.0.1:17373/anthropic` (Anthropic), con el token como API key y `model` del tipo `claude/sonnet` o `perfil/rapido`. Contrato completo en [api.md](api.md).

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
