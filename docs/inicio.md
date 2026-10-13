# Primeros pasos

## Qué necesitas

- **Windows 11** (Linux y macOS: el núcleo funciona; la aplicación de escritorio, de momento, solo en Windows).
- **Go 1.27** para compilar. Si `go` no se reconoce, añade `C:\Program Files\Go\bin` al PATH de tu usuario y abre una terminal nueva.
- **Al menos un agente:** [Claude Code](https://docs.anthropic.com/claude-code) (`claude`), [Codex CLI](https://developers.openai.com/codex/cli) (`codex`) o una clave de la Gemini API. Antigravity (`agy`) es experimental.
- **Opcional, para archivos de Office y PDF:** `uv tool install "markitdown[pdf,docx,pptx,xlsx]==0.1.8"`.

## 1. Compilar y abrir

```powershell
./scripts/compilar.ps1     # genera bin/tramtai.exe y bin/tramtai-escritorio.exe
./bin/tramtai.exe abrir    # inicia el servicio en segundo plano y abre la Web UI
```

- **Sin clave:** en tu propio equipo entras directamente; Tramtai comprueba que el navegador es de tu mismo usuario de Windows. Desde otro equipo o en macOS, la página explica cómo entrar.
- **Que arranque solo:** en **Ajustes → Arranque con Windows** (o `tramtai servicio autoarranque activar`), Tramtai se inicia oculto en la bandeja al entrar en Windows y la Web UI está siempre disponible, sin abrir ninguna terminal.
- **Ayuda e idioma:** la página **Ayuda** resume estos pasos y responde las dudas comunes; el botón de idioma de la cabecera cambia entre español e inglés.
- **Aplicación de escritorio:** `bin/tramtai-escritorio.exe` hace lo mismo en su propia ventana y queda en la bandeja. Cerrarla no detiene el servicio.
- **En un proyecto:** dentro de su carpeta, `tramtai abrir` abre su propia instancia, con trabajos y configuración separados de la global.

## 2. Dejar listos los agentes

La página **Agentes** dice qué falta en cada uno. Hay dos formas de autenticarlos:

**Clave de API** (la predeterminada). Guárdala en el Administrador de credenciales de Windows, sin dejarla en variables de entorno:

```powershell
tramtai claves guardar ANTHROPIC_API_KEY   # Claude; la pide sin mostrarla
tramtai claves guardar OPENAI_API_KEY      # Codex
tramtai claves guardar GEMINI_API_KEY      # Gemini y Antigravity
tramtai claves listar                      # de dónde sale cada una, sin mostrarla
```

Reinicia el servicio después de guardar una clave: `tramtai servicio detener` y otra vez `tramtai abrir`.

**Tu suscripción** (Claude Pro/Max o cuenta de ChatGPT), solo para uso **personal y local**. Lo más fácil es el botón **Usar mi suscripción** de la página Agentes: lo aplica al momento, sin reiniciar. Equivale a escribir esto en la configuración global `%APPDATA%\tramtai\config.yaml`:

```yaml
adaptadores:
  claude:
    autenticacion: suscripcion
  codex:
    autenticacion: suscripcion
```

Tramtai nunca lee ni copia las credenciales de los CLIs. Antigravity solo funciona con clave de la Gemini API: sus términos no permiten usar una cuenta personal desde otra herramienta.

## 3. Usarlo

- **Web UI → Probar:** escribe un mensaje, adjunta archivos si quieres y mira la respuesta en vivo.
- **Web UI → Mis APIs:** la URL, la clave y ejemplos en Python, Node y curl para tus aplicaciones.
- **Terminal interactiva:** `tramtai` sin argumentos abre un menú con todo lo anterior.
- **Línea de órdenes:**

```powershell
tramtai ejecutar "Resume en una frase qué es Go"               # agente y modelo predeterminados
tramtai ejecutar -a claude -p rapido "Hola"                    # perfil de modelo
tramtai ejecutar -a claude -m sonnet -s "Responde en inglés" "Hola"
Get-Content prompt.txt | tramtai ejecutar -a codex             # mensaje desde un archivo
tramtai trabajos listar                                        # historial (también en la Web UI)
```

### Archivos y respuestas en JSON

```powershell
tramtai ejecutar -f informe.pdf -f anexo.xlsx "Resume ambos documentos"
tramtai ejecutar -f acta.docx -e compromisos.schema.json "Extrae los compromisos; null si no hay fecha"
tramtai convertir informe.pdf     # muestra el texto que recibiría el agente, sin ejecutarlo
```

- `-f` admite PDF, Word, PowerPoint, Excel, HTML, texto (txt, md, json, csv…) y ZIP, y se puede repetir.
- `-e` recibe un JSON Schema: la respuesta será un JSON que lo cumpla, o el trabajo fallará con `salida_no_cumple_esquema`.

### Modelos

- `-m` (o `model` en la API) acepta el nombre que entienda el CLI: `haiku`, `sonnet`, `claude-sonnet-5-5`, `gemini-flash-latest`…
- `-p` usa un perfil: `rapido`, `equilibrado` o `profundo`. Para Claude equivalen a `haiku`, `sonnet` y `opus`, y para Gemini a los modelos *flash-lite*, *flash* y *pro*. Para Codex defínelos tú:

  ```yaml
  modelos:
    perfiles:
      rapido:
        codex: <modelo>
  ```

- Si un perfil no tiene modelo para el agente, el trabajo se rechaza: Tramtai nunca cambia el modelo en silencio.
- **Modelo predeterminado:** el que se usa cuando una aplicación no elige agente ni modelo. Se cambia en **Ajustes → Modelo predeterminado** (al momento, sin reiniciar) o con `predeterminados.agente` y `predeterminados.modelo` (o `predeterminados.perfil_modelo`) en la configuración. En un proyecto se guarda en su `.tramtai/local.yaml`.
- **Versión real:** los selectores muestran bajo cada alias (`sonnet`, `haiku`…) la versión que informó el agente la última vez que se usó, por ejemplo `claude-haiku-4-5-20251001`. Codex no informa el modelo efectivo.

## Proyectos

```powershell
tramtai iniciar     # en la carpeta de un repositorio: crea .tramtai/ con su configuración y un perfil de ejemplo
tramtai abrir       # abre la instancia de ese proyecto
```

- **Perfiles de trabajo:** `.tramtai/profiles/<nombre>.yaml` guarda instrucciones, formato y esquema reutilizables. Se usan con `--perfil-trabajo <nombre>`, o con `perfil` en la API.
- **Modo consciente del proyecto:** el agente trabaja en la carpeta del proyecto y lee sus reglas (`CLAUDE.md`, `AGENTS.md`). Solo se permite en proyectos que declares confiables en `.tramtai/local.yaml`:

  ```yaml
  proyectos_confiables:
    - C:\ruta\al\proyecto
  ```

  Se pide con `tramtai ejecutar --contexto consciente_proyecto "…"`.

## Configuración

Se lee de menor a mayor prioridad: valores internos, `%APPDATA%\tramtai\config.yaml` (global), `.tramtai/config.yaml` (proyecto, se versiona), `.tramtai/local.yaml` (solo este equipo) y variables de entorno `TRAMTAI_SECCION__CLAVE`.

```powershell
tramtai configuracion mostrar --efectiva     # cada valor y de dónde sale (también en la Web UI)
tramtai configuracion explicar adaptadores.claude.autenticacion
```

Más detalle en [operación](operacion.md) y en la [API](api.md).
