# Primeros pasos

## Qué necesitas

- **Windows 10 u 11** (x64), **Linux** (x64 o ARM) o **macOS** 11 o posterior (Intel o Apple Silicon). La app de escritorio existe para los tres.
- **Al menos un agente:** [Claude Code](https://docs.anthropic.com/claude-code) (`claude`), [Codex CLI](https://developers.openai.com/codex/cli) (`codex`), [GitHub Copilot CLI](https://docs.github.com/copilot/how-tos/set-up/install-copilot-cli) (`copilot`, con cualquier plan de GitHub Copilot) o una clave de la Gemini API, que también sirve para Antigravity (`agy`).
- **Para archivos de Office y PDF no hace falta instalar nada:** Tramtai prepara su conversor (markitdown, con su
  propio Python) en su carpeta la primera vez que arranca; lo ves y lo repites en **Ajustes → Archivos** o con
  `tramtai conversor preparar`. Ocupa unos 300 MB y no pide permisos de administrador. Si la red de tu empresa bloquea
  la descarga, baja el **paquete del conversor** para Windows (`Tramtai-…-conversor-windows-x64.zip`, unos 100 MB) de
  la misma página de descargas y descomprímelo junto a `tramtai.exe`.

| | Windows 10 y 11 | Linux | macOS |
|---|---|---|---|
| Servicio, API, CLI, terminal y Web UI | ✅ | ✅ | ✅ |
| App de escritorio con bandeja | ✅ (instalador o portable) | ✅ (.deb y .rpm) | ✅ (.dmg sin firmar) |
| Arranque con el sistema | ✅ | ✅ | ✅ |
| Actualización con un clic | ✅ (instalada) | Aviso con enlace | Aviso con enlace |
| Claves en el almacén del sistema | ✅ Administrador de credenciales | ✅ Secret Service (`secret-tool`) | ✅ Llavero |
| Entrar en la Web UI | Directo (mismo usuario) | Directo (mismo usuario) | Desde la app o con `tramtai abrir` |

## 1. Instalar y abrir

Descarga `Tramtai-…-instalador-windows-x64.exe` desde [Releases](https://github.com/MscottB/Tramtai/releases/latest) y ejecútalo:

- Se instala solo para tu usuario, en `%LOCALAPPDATA%\Programs\Tramtai`, sin permisos de administrador.
- Opciones: el comando `tramtai` en la terminal (añade la carpeta a tu PATH), iniciar con Windows y un acceso directo en el escritorio.
- Para actualizar no hace falta descargar nada a mano: cuando hay una versión nueva, la app lo avisa y **Actualizar ahora** la instala (también `tramtai actualizar`). Ver [actualizaciones](operacion.md#actualizaciones).
- Se desinstala desde *Configuración → Aplicaciones*; al final pregunta si borrar también tus datos (`%APPDATA%\tramtai`).
- Para comprobar la descarga, la suma SHA-256 de cada archivo está en `SHA256SUMS.txt` del mismo release.

**En Linux, con la app de escritorio:** instala el `.deb` (`sudo apt install ./Tramtai-…-linux-amd64.deb`) o el `.rpm` (`sudo dnf install ./Tramtai-…-linux-amd64.rpm`) de [Releases](https://github.com/MscottB/Tramtai/releases/latest). Trae el servicio, la CLI, la app con su icono en la bandeja y el acceso en el menú de aplicaciones; necesita GTK 4 y WebKitGTK 6.0 (Ubuntu 24.04+, Debian 12+, Fedora), que el gestor de paquetes instala solo. En GNOME, el icono de la bandeja necesita la extensión *AppIndicator*; sin ella, Tramtai se abre desde el menú de aplicaciones.

**Portable en Windows:** `Tramtai-…-portable-windows-x64.zip` trae la app y el servicio sin instalar nada; descomprímelo en una carpeta tuya y abre `tramtai-escritorio.exe`. Guarda todo en la carpeta `datos-tramtai`, a su lado (por el archivo `portable.txt`), así que no se mezcla con una instalación y se puede llevar a otro equipo. No se actualiza sola: se descomprime la versión nueva encima. Más en [operación](operacion.md#versión-portable).

**En macOS:** abre `Tramtai-…-macos.dmg` y arrastra Tramtai a Aplicaciones. No está firmada por un desarrollador de Apple: la primera vez macOS la bloquea; ábrela, cierra el aviso y ve a *Ajustes del Sistema → Privacidad y seguridad → Abrir igualmente* (o `xattr -dr com.apple.quarantine /Applications/Tramtai.app` en la Terminal). Para usar `tramtai` en la Terminal: `ln -s /Applications/Tramtai.app/Contents/MacOS/tramtai ~/.local/bin/tramtai` (o instala solo la CLI con el script de abajo).

**Con Homebrew (macOS y Linux):** `brew tap mscottb/tramtai https://github.com/MscottB/Tramtai` y después `brew install --cask tramtai` (la app de macOS, sin el bloqueo de Gatekeeper) o `brew install tramtai` (solo la CLI y el servicio). Se actualiza con `brew upgrade`.

**Sin app de escritorio (un servidor Linux, o solo la CLI en macOS):** `curl -fsSL https://raw.githubusercontent.com/MscottB/Tramtai/main/scripts/instalar.sh | sh` descarga el archivo de tu sistema, comprueba su suma SHA-256 y deja `tramtai` en `~/.local/bin` (otra carpeta con `TRAMTAI_DESTINO`). Para actualizar, vuelve a ejecutarlo. Después, `tramtai abrir` inicia el servicio y abre la Web UI en el navegador.

Después abre **Tramtai** desde el menú Inicio (Windows), la carpeta Aplicaciones (macOS) o el menú de aplicaciones (Linux), o con `tramtai abrir` en una terminal nueva. Si prefieres compilarlo tú, la [guía de desarrollo](equipo/desarrollo.md) explica cómo (`./scripts/compilar.ps1`, con Go 1.27).

- **Entrar en la Web UI:** la app de escritorio entra sola. En el navegador (Windows, Linux y macOS) entras directamente: Tramtai comprueba que es de tu mismo usuario del sistema. Si no se puede comprobar, `tramtai abrir` (o la terminal interactiva) la abre ya dentro con un código de un solo uso. La clave de las aplicaciones no sirve para entrar.
- **Que arranque solo:** en **Ajustes → Arranque con Windows** (o con macOS, o con Linux, según tu sistema) o con `tramtai servicio autoarranque activar`, Tramtai se inicia oculto en la bandeja al iniciar sesión y la Web UI está siempre disponible, sin abrir ninguna terminal.
- **Ayuda, idioma y apariencia:** la página **Ayuda** resume estos pasos y responde las dudas comunes. En la cabecera, un botón cambia entre español e inglés y otro elige la apariencia: clara, oscura o como el sistema. La guía de **Inicio** es opcional: «Ocultar guía» la quita y **Ayuda** la vuelve a mostrar.
- **El recorrido de bienvenida:** la primera vez que abres Tramtai, ya en **Inicio**, un diálogo pregunta «¿Te enseño Tramtai?» (unos 2 minutos): **Empezar** te enseña el mapa, Agentes, Probar, Mis APIs y Espacios en 14 pasos, y **Ahora no** no vuelve a preguntar; lo tienes en **Primeros pasos** («Haz el recorrido de Tramtai») para cuando quieras, también para repetirlo. No sale si entraste por un enlace directo a una página. En cada página, el botón **?** abre su ayuda y **Muéstrame en pantalla** hace el recorrido de esa página (también en **Ayuda**). Con Esc lo cierras y con las flechas pasas de paso. Solo enseña: no pulsa nada ni cambia tus datos.
- **El menú, por grupos:** **Empezar** (Inicio, Agentes, Probar), **Conectar** (Mis APIs, Historial) y **Organizar** (Espacios, Ajustes); Ayuda queda aparte, abajo.
- **Aplicación de escritorio:** Tramtai abre la Web UI en su propia ventana, en Windows, macOS y Linux. Cerrar la ventana la deja en la bandeja con el servicio en marcha; el menú del clic derecho muestra el estado y tus APIs, y «Salir y detener el servicio…» la cierra del todo (con un aviso).
- **En un proyecto:** en la Web UI, el selector de espacio de la cabecera → **Abrir un proyecto…**: escribes la carpeta (o, en la app de escritorio, la eliges con **Examinar…**) y Tramtai la prepara si hace falta (crea `.tramtai/`) y la abre, con su historial, sus APIs y sus ajustes separados de la global. Desde la terminal, dentro de su carpeta, `tramtai iniciar` y `tramtai abrir` hacen lo mismo.

## 2. Dejar listos los agentes

La página **Agentes** dice qué falta en cada uno. Hay dos formas de autenticarlos:

**Clave de API** (la predeterminada). `tramtai claves guardar` la pide sin mostrarla y la guarda cifrada en el almacén del sistema; Tramtai nunca la escribe en sus archivos:

| Sistema | Dónde queda |
|---|---|
| Windows | Administrador de credenciales |
| macOS | Llavero (vía `/usr/bin/security`) |
| Linux | Secret Service de GNOME o KDE, vía `secret-tool` (paquete `libsecret-tools`) |

```powershell
tramtai claves guardar ANTHROPIC_API_KEY   # Claude; la pide sin mostrarla
tramtai claves guardar OPENAI_API_KEY      # Codex
tramtai claves guardar COPILOT_GITHUB_TOKEN  # GitHub Copilot (token fine-grained con «Copilot Requests»)
tramtai claves guardar GEMINI_API_KEY      # Gemini y Antigravity
tramtai claves listar                      # de dónde sale cada una, sin mostrarla
```

- **Sin reiniciar:** la clave del almacén se lee en cada petición, así que guardarla o cambiarla vale desde la siguiente.
- **Variable de entorno:** si existe una con el mismo nombre, tiene prioridad sobre el almacén, pero el servicio solo la ve si arranca con ella. En macOS, una variable de `~/.zshrc` no llega al servicio que inicia la app o el arranque automático: usa el Llavero. En Linux sin `secret-tool`, la variable es la única opción.
- La página **Agentes** dice dónde queda la clave en tu sistema.

**Tu suscripción** (Claude Pro/Max o cuenta de ChatGPT), solo para uso **personal y local**. Lo más fácil es el botón **Usar mi suscripción** de la página Agentes: lo aplica al momento, sin reiniciar. Equivale a escribir esto en la configuración global `config.yaml` (ver [configuración](#configuración)):

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

- `-f` admite PDF, Word, PowerPoint, Excel, HTML y texto (txt, md, json, csv…), y se puede repetir. Un formato que no
  se admite se avisa antes de enviar nada; un ZIP, descomprímelo (lista completa en [la API](api.md#formatos-admitidos)).
- `-e` recibe un JSON Schema: la respuesta será un JSON que lo cumpla, o la petición fallará con `salida_no_cumple_esquema` y el primer problema («Falta el campo `total`»). En Probar el esquema se arma con plantillas o campo a campo, y la respuesta que no cumple se corrige con un clic (o hasta 3 veces sola, con «Si la respuesta no cumple»).

### Modelos

- `-m` (o `model` en la API) acepta el nombre que entienda el CLI: `haiku`, `sonnet`, `claude-sonnet-5-5`, `gemini-flash-latest`…
- **GitHub Copilot:** `copilot` (o `copilot/auto`) usa el modelo automático (el único del plan Free): GitHub Copilot elige el modelo,
  Tramtai muestra cuál usó y cada petición gasta créditos del plan. Con un plan de pago, `copilot/<modelo>`. Inicia
  sesión con `copilot login` y elige en Agentes «Usar mi suscripción», o guarda `COPILOT_GITHUB_TOKEN`.
- `-p` usa un perfil: `rapido`, `equilibrado` o `profundo`. Para Claude equivalen a `haiku`, `sonnet` y `opus`, y para Gemini a los modelos *flash-lite*, *flash* y *pro*. Para Codex defínelos tú:

  ```yaml
  modelos:
    perfiles:
      rapido:
        codex: <modelo>
  ```

- Si un perfil no tiene modelo para el agente, la petición se rechaza: Tramtai nunca cambia el modelo en silencio.
- **Modelo predeterminado:** el que se usa cuando una aplicación no elige agente ni modelo. Se cambia en **Ajustes → Modelo predeterminado** (al momento, sin reiniciar) o con `predeterminados.agente` y `predeterminados.modelo` (o `predeterminados.perfil_modelo`) en la configuración. En un proyecto se guarda en su `.tramtai/local.yaml`. Debajo se elige su **esfuerzo** (`predeterminados.esfuerzo`): se usa con el modelo predeterminado cuando la petición tampoco elige esfuerzo; con otro agente, modelo o perfil, cada CLI usa el suyo. Si el modelo nuevo no admite ese esfuerzo, vuelve al del agente.
- **Versión real:** los selectores muestran bajo cada alias (`sonnet`, `haiku`…) la versión que informó el agente la última vez que se usó, por ejemplo `claude-haiku-4-5-20251001`. Codex no informa el modelo efectivo; GitHub Copilot sí, también el que elige el automático.

## Proyectos

```powershell
tramtai iniciar     # en la carpeta de un repositorio: crea .tramtai/ con su configuración y un perfil de ejemplo
tramtai abrir       # abre el espacio de ese proyecto
```

- **Perfiles de ejecución:** `.tramtai/profiles/<nombre>.yaml` guarda instrucciones, formato y esquema reutilizables. Se usan con `--perfil-trabajo <nombre>`, o con `perfil` en la API.
- **Modo consciente del proyecto:** el agente trabaja en la carpeta del proyecto y lee sus reglas (`CLAUDE.md`, `AGENTS.md`). Solo se permite en proyectos que declares confiables en la configuración global (`config.yaml`, ver [configuración](#configuración)):

  ```yaml
  proyectos_confiables:
    - C:\ruta\al\proyecto
  ```

  `proyectos_confiables` solo se lee de la configuración global: un proyecto no puede declararse confiable a sí mismo. Por eso un `.tramtai/local.yaml` con claves de seguridad (comandos de los agentes, `conversion.comando`, APIs, `servicio`, `adaptadores`…) solo se acepta si la carpeta del proyecto figura en esa lista; si no, Tramtai rechaza la configuración e indica que la añadas. Las demás claves, como el modelo predeterminado del proyecto, valen siempre.

  Se pide con `tramtai ejecutar --contexto consciente_proyecto "…"`.

## Idioma

La CLI, la terminal interactiva y la app de escritorio salen en el idioma de tu sistema: en español si está en español y en inglés si no. Para elegirlo, define la variable de entorno `TRAMTAI_IDIOMA` con `es` o `en`. La Web UI tiene su propio selector de idioma, arriba a la derecha.

## Configuración

Se lee de menor a mayor prioridad: valores internos, `config.yaml` global, `.tramtai/config.yaml` (proyecto, se versiona), `.tramtai/local.yaml` (solo este equipo) y variables de entorno `TRAMTAI_SECCION__CLAVE`.

La configuración global y los datos (subcarpeta `datos`) están en:

| Sistema | Carpeta |
|---|---|
| Windows | `%APPDATA%\tramtai` (la portable, en `datos-tramtai`, junto a sus archivos) |
| macOS | `~/Library/Application Support/tramtai` |
| Linux | `~/.config/tramtai` (o `$XDG_CONFIG_HOME/tramtai`) |

```powershell
tramtai configuracion mostrar --efectiva     # cada valor y de dónde sale (también en la Web UI)
tramtai configuracion explicar adaptadores.claude.autenticacion
```

Más detalle en [operación](operacion.md) y en la [API](api.md).
