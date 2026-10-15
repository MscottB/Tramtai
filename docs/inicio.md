# Primeros pasos

## Qué necesitas

- **Windows 10 u 11** (x64), **Linux** o **macOS** (Intel o Apple Silicon). En Linux y macOS se instalan el servicio, la API, la CLI y la Web UI; la app de escritorio, de momento, solo existe para Windows.
- **Al menos un agente:** [Claude Code](https://docs.anthropic.com/claude-code) (`claude`), [Codex CLI](https://developers.openai.com/codex/cli) (`codex`) o una clave de la Gemini API. Antigravity (`agy`) es experimental.
- **Opcional, para archivos de Office y PDF:** `uv tool install "markitdown[pdf,docx,pptx,xlsx]==0.1.8"`.

## 1. Instalar y abrir

Descarga `Tramtai-…-instalador-windows-x64.exe` desde [Releases](https://github.com/MscottB/Tramtai/releases/latest) y ejecútalo:

- Se instala solo para tu usuario, en `%LOCALAPPDATA%\Programs\Tramtai`, sin permisos de administrador.
- Opciones: el comando `tramtai` en la terminal (añade la carpeta a tu PATH), iniciar con Windows y un acceso directo en el escritorio.
- Para actualizar no hace falta descargar nada a mano: cuando hay una versión nueva, la app lo avisa y **Actualizar ahora** la instala (también `tramtai actualizar`). Ver [actualizaciones](operacion.md#actualizaciones).
- Se desinstala desde *Configuración → Aplicaciones*; al final pregunta si borrar también tus datos (`%APPDATA%\tramtai`).
- Para comprobar la descarga, la suma SHA-256 de cada archivo está en `SHA256SUMS.txt` del mismo release.

**En Linux o macOS:** `curl -fsSL https://raw.githubusercontent.com/MscottB/Tramtai/main/scripts/instalar.sh | sh` descarga el archivo de tu sistema, comprueba su suma SHA-256 y deja `tramtai` en `~/.local/bin` (otra carpeta con `TRAMTAI_DESTINO`). Para actualizar, vuelve a ejecutarlo. Después, `tramtai abrir` inicia el servicio y abre la Web UI en el navegador.

Después abre **Tramtai** desde el menú Inicio, o `tramtai abrir` en una terminal nueva. Si prefieres compilarlo tú, la [guía de desarrollo](equipo/desarrollo.md) explica cómo (`./scripts/compilar.ps1`, con Go 1.27).

- **Sin clave:** en tu propio equipo entras directamente; Tramtai comprueba que el navegador es de tu mismo usuario de Windows. Desde otro equipo o en macOS, la página explica cómo entrar.
- **Que arranque solo:** en **Ajustes → Arranque con Windows** (o `tramtai servicio autoarranque activar`), Tramtai se inicia oculto en la bandeja al entrar en Windows y la Web UI está siempre disponible, sin abrir ninguna terminal.
- **Ayuda, idioma y apariencia:** la página **Ayuda** resume estos pasos y responde las dudas comunes. En la cabecera, un botón cambia entre español e inglés y otro elige la apariencia: clara, oscura o como el sistema. La guía de **Inicio** es opcional: «Ocultar guía» la quita y **Ayuda** la vuelve a mostrar.
- **Aplicación de escritorio:** Tramtai, en el menú Inicio, abre la Web UI en su propia ventana. Cerrar la ventana la deja en la bandeja con el servicio en marcha; el menú del clic derecho muestra el estado y tus APIs, y «Salir y detener el servicio…» la cierra del todo (con un aviso).
- **En un proyecto:** en la Web UI, el selector de espacio de la cabecera → **Abrir un proyecto…**: escribes la carpeta (o, en la app de escritorio, la eliges con **Examinar…**) y Tramtai la prepara si hace falta (crea `.tramtai/`) y la abre, con su historial, sus APIs y sus ajustes separados de la global. Desde la terminal, dentro de su carpeta, `tramtai iniciar` y `tramtai abrir` hacen lo mismo.

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

## Idioma

La CLI, la terminal interactiva y la app de escritorio salen en el idioma de tu sistema: en español si está en español y en inglés si no. Para elegirlo, define la variable de entorno `TRAMTAI_IDIOMA` con `es` o `en`. La Web UI tiene su propio selector de idioma, arriba a la derecha.

## Configuración

Se lee de menor a mayor prioridad: valores internos, `%APPDATA%\tramtai\config.yaml` (global), `.tramtai/config.yaml` (proyecto, se versiona), `.tramtai/local.yaml` (solo este equipo) y variables de entorno `TRAMTAI_SECCION__CLAVE`.

```powershell
tramtai configuracion mostrar --efectiva     # cada valor y de dónde sale (también en la Web UI)
tramtai configuracion explicar adaptadores.claude.autenticacion
```

Más detalle en [operación](operacion.md) y en la [API](api.md).
