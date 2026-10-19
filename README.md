<div align="center">

<img src="escritorio/icono/tramtai.png" alt="" width="96">

# Tramtai

**Tus agentes de IA, como una API local.**

Tramtai convierte Claude Code, Codex, GitHub Copilot CLI, Antigravity y la API de Gemini en una API compatible con OpenAI y Anthropic que funciona en tu propio equipo.<br>
Cualquier aplicación o SDK puede usarlos cambiando solo la URL base.

[![CI](https://github.com/MscottB/Tramtai/actions/workflows/ci.yml/badge.svg)](https://github.com/MscottB/Tramtai/actions/workflows/ci.yml)
[![Versión](https://img.shields.io/github/v/release/MscottB/Tramtai?label=versi%C3%B3n&color=0b6e64)](https://github.com/MscottB/Tramtai/releases/latest)
[![Windows, macOS y Linux](https://img.shields.io/badge/Windows%20%C2%B7%20macOS%20%C2%B7%20Linux-0078d4)](https://github.com/MscottB/Tramtai/releases/latest)
[![Licencia](https://img.shields.io/badge/licencia-PolyForm%20Noncommercial-55666a)](LICENSE.md)

[**Descargar**](https://mscottb.github.io/Tramtai/#descargar) (Windows, macOS y Linux) · [Página del producto](https://mscottb.github.io/Tramtai/) · [Primeros pasos](docs/inicio.md) · [English](#english)

<br>

<img src="docs/imagenes/inicio.png" alt="Inicio de Tramtai: el mapa de la red con tu app, Tramtai y una línea por agente" width="880">

</div>

## Qué ofrece

- **Compatible con lo que ya usas.** SDK de OpenAI y Anthropic (Python, Node, curl…), con la respuesta en vivo y **herramientas** (`tools`): el agente pide las funciones de tu aplicación como con OpenAI o Anthropic. También una API propia con eventos que se pueden reanudar.
- **Agente y modelo en cada petición.** `claude/sonnet`, `codex`, `copilot`, `gemini/gemini-flash-latest` o un perfil como `perfil/rapido`; en Claude, Codex y GitHub Copilot (con un modelo concreto) también eliges cuánto razona el agente.
- **Archivos y respuestas con forma.** PDF, Word, Excel, PowerPoint o ZIP como entrada; JSON validado con un esquema como salida. El esquema se arma con plantillas listas (extraer datos, clasificar, resumen, tareas…) o campo a campo, sin escribir JSON Schema; si la respuesta no cumple, ves qué campo falló y la corriges con un clic, o eliges que se corrija sola hasta 3 veces.
- **Una API por aplicación,** con su propia clave, sus modelos permitidos o fijos e instrucciones propias (por ejemplo, «responde siempre en español»). Se activan o detienen sin reiniciar.
- **Recetas: de la prueba a la API en un clic.** En Probar defines instrucciones, agente con su esfuerzo, esquema JSON y respaldo, lo pruebas con tus archivos, ves la respuesta con formato y cuánto tardó cada paso, y lo guardas como API: tu aplicación envía el mensaje y recibe la respuesta con esa forma.
- **Respaldo entre agentes.** Si el agente principal llega a su límite, no tiene clave o falla antes de responder, la petición sigue con el siguiente de la lista (hasta 3), sin que tu aplicación lo note.
- **Presupuestos.** Cada API, y el espacio entero, pueden tener un límite diario o mensual de peticiones, tokens o dólares: aviso al 80 % y, al 100 %, error 429 hasta que empieza el periodo siguiente o reinicias el consumo.
- **Tu suscripción o tu clave de API.** Claude, Codex y GitHub Copilot se conectan con un clic a la sesión que ya tienes (uso personal; en GitHub Copilot, cualquier plan: Free, Pro o de empresa) o con una clave.
- **Cuatro formas de usarlo** sobre los mismos trabajos: app de escritorio con icono en la bandeja, Web UI con ayuda integrada, terminal interactiva y línea de órdenes. En español y en inglés.
- **Local y privado.** Escucha solo en `127.0.0.1` y los resultados caducan a las 24 horas. Sin telemetría.
- **Claves bien guardadas.** Las de tus proveedores quedan cifradas en el almacén del sistema (Administrador de credenciales, Llavero de macOS o Secret Service de Linux), nunca en los archivos de Tramtai. La clave de cada API solo se ve al crearla o regenerarla, y la Web UI se abre desde la app o con `tramtai abrir`, no con esa clave.

## Empezar

1. **Instálalo** en Windows, macOS o Linux (abajo, cómo en cada uno). Las sumas SHA-256 de cada archivo están en `SHA256SUMS.txt` del release.
2. **Abre Tramtai**: queda en la bandeja del sistema con el servicio en marcha.
3. En la app, **Agentes** te dice qué le falta a cada agente (conectarlo con tu suscripción o con una clave) y **Mis APIs** te da la URL, la clave y ejemplos listos para copiar.

**En Windows 10 y 11** descarga `Tramtai-…-instalador-windows-x64.exe` desde [Releases](https://github.com/MscottB/Tramtai/releases/latest) y ejecútalo. Es solo para tu usuario, no pide permisos de administrador y se actualiza con un clic. Si prefieres no instalar nada, `Tramtai-…-portable-windows-x64.zip` trae la misma app para descomprimir y usar (esa copia no se actualiza sola).

**En Linux** está la app completa, con su icono en la bandeja: descarga el `.deb` (Debian, Ubuntu, Mint…) o el `.rpm` (Fedora, openSUSE…) de [Releases](https://github.com/MscottB/Tramtai/releases/latest) e instálalo con `sudo apt install ./Tramtai-…-linux-amd64.deb` o `sudo dnf install ./Tramtai-…-linux-amd64.rpm`. Necesita GTK 4 y WebKitGTK 6.0, que se instalan solos (Ubuntu 24.04 o posterior, Debian 12 o posterior, Fedora).

**En macOS** descarga `Tramtai-…-macos.dmg` (Intel y Apple Silicon), ábrelo y arrastra Tramtai a Aplicaciones. La app no está firmada por un desarrollador de Apple, así que la primera vez macOS la bloquea: ábrela, cierra el aviso y ve a *Ajustes del Sistema → Privacidad y seguridad → Abrir igualmente* (o, en la Terminal, `xattr -dr com.apple.quarantine /Applications/Tramtai.app`). Después se abre con normalidad.

**Con Homebrew (macOS y Linux):** el propio repositorio hace de *tap*. La app de macOS se instala sin el bloqueo de Gatekeeper, y la CLI sola sirve también en Linux:

```bash
brew tap mscottb/tramtai https://github.com/MscottB/Tramtai
brew install --cask tramtai   # la app de escritorio (macOS)
brew install tramtai          # o solo la CLI y el servicio (macOS y Linux)
```

Para actualizar: `brew upgrade`.

**Sin app de escritorio (un servidor Linux, o solo la CLI en macOS)** se instala el servicio con la CLI y la Web UI:

```bash
curl -fsSL https://raw.githubusercontent.com/MscottB/Tramtai/main/scripts/instalar.sh | sh
tramtai abrir   # inicia el servicio y abre la Web UI en el navegador
```

El script elige el archivo de tu sistema (Intel o ARM), comprueba su suma SHA-256 y deja `tramtai` en `~/.local/bin`; para actualizar, vuelve a ejecutarlo.

Necesitas al menos un agente instalado: [Claude Code](https://docs.anthropic.com/claude-code), [Codex CLI](https://developers.openai.com/codex/cli), [GitHub Copilot CLI](https://docs.github.com/copilot/how-tos/set-up/install-copilot-cli) o una clave de la Gemini API. Para leer PDF y archivos de Office, instala también [markitdown](https://github.com/microsoft/markitdown) (`uv tool install "markitdown[pdf,docx,pptx,xlsx]==0.1.8"`).

## Conectar una aplicación

```python
from openai import OpenAI

cliente = OpenAI(base_url="http://127.0.0.1:17373/openai/v1", api_key="<clave de Mis APIs>")
r = cliente.chat.completions.create(model="claude/sonnet", messages=[{"role": "user", "content": "Hola"}])
print(r.choices[0].message.content)
```

Para enviar archivos, seguir tareas largas o reintentar, usa la API propia (`/api/v1`, con su OpenAPI en
`/api/v1/openapi.json`). En la app, **Mis APIs → ¿Qué URL uso?** compara las tres URL con ejemplos para copiar.

Con el SDK de Anthropic, la URL base es `http://127.0.0.1:17373/anthropic`. Todas las rutas y opciones están en la [referencia de la API](docs/api.md).

<p align="center"><img src="docs/imagenes/mis-apis-oscuro.png" alt="Mis APIs en modo oscuro: tus APIs por grupo, con buscador, orden y el botón Conectar de cada una" width="820"></p>

## Actualizaciones

Tramtai comprueba cada hora si hay una versión nueva, así el aviso aparece sin reiniciar la app (solo pide a GitHub la ficha pública de la última versión; no envía ningún dato tuyo). Cuando la hay:

- La cabecera de la app muestra **Actualizar**, y el menú del icono de la bandeja indica tu versión y **Actualizar a vX…**.
- El diálogo enseña las **novedades** sin salir de la app. En Windows, si la instalaste con el instalador, **Actualizar ahora** descarga la versión nueva, comprueba que lleva la firma de Tramtai y su suma SHA-256, y la instala sin preguntar nada más: la app se cierra, las peticiones en curso terminan y vuelve a abrirse ya actualizada, con tu configuración y tus datos.
- La copia portable, la de Linux y la de macOS muestran el aviso con el enlace de descarga: ahí se actualiza instalando el paquete nuevo (o volviendo a ejecutar `instalar.sh`).
- Para no esperar: **Ajustes → Buscar actualización ahora**, o **Buscar actualizaciones** en la bandeja.
- Si instalas una versión a mano con Tramtai abierto, el instalador lo cierra, instala y lo vuelve a abrir.
- Desde la terminal: `tramtai actualizar` (o `tramtai actualizar --comprobar` para solo mirar). Se desactiva en **Ajustes → Buscar actualizaciones**.

## Desde la terminal

```powershell
tramtai                                        # terminal interactiva: todo desde un menú
tramtai ejecutar "Resume en una frase qué es Go"
tramtai ejecutar -f acta.pdf -e compromisos.schema.json "Extrae los compromisos"
tramtai claves guardar ANTHROPIC_API_KEY       # guarda una clave sin mostrarla
tramtai acceso crear tienda --modelo claude/haiku   # una API propia para otra aplicación
```

La CLI sale en el idioma de tu sistema; `TRAMTAI_IDIOMA=en` o `es` lo fija.

## Documentación

| Guía | Contenido |
|---|---|
| [Primeros pasos](docs/inicio.md) | Instalar, dejar listos los agentes, archivos, modelos, proyectos y configuración |
| [Operación](docs/operacion.md) | Servicio, espacios global y por proyecto, APIs por aplicación, arranque con el sistema y retención |
| [API](docs/api.md) | Rutas nativas, compatibles con OpenAI y Anthropic, perfiles, esfuerzo y errores |
| [Arquitectura](docs/arquitectura.md) | Cómo funciona por dentro: agentes, aislamiento, seguridad y recuperación |
| [Desarrollo](docs/equipo/desarrollo.md) | Compilar desde el código, pruebas, versiones y publicación |

## Plataformas

| | Windows 10 y 11 | Linux | macOS |
|---|---|---|---|
| Servicio, API, CLI y terminal | ✅ | ✅ | ✅ |
| Web UI | ✅ | ✅ | ✅ |
| App de escritorio con bandeja | ✅ (instalador o portable) | ✅ (.deb y .rpm) | ✅ (.dmg sin firmar) |
| Arranque con el sistema | ✅ | ✅ | ✅ |
| Actualización con un clic | ✅ (instalada) | Aviso con enlace | Aviso con enlace |
| Claves en el almacén del sistema | ✅ Administrador de credenciales | ✅ Secret Service (`secret-tool`) | ✅ Llavero |
| Entrar en la Web UI | Directo (mismo usuario) | Directo (mismo usuario) | Desde la app o con `tramtai abrir` |

## Compilar desde el código

Necesitas Go 1.27 y, para la Web UI, Node.js. En Windows (en Linux y macOS, `scripts/linux/empaquetar.sh` y `scripts/macos/empaquetar.sh`):

```powershell
./scripts/compilar.ps1              # bin/tramtai.exe y bin/tramtai-escritorio.exe
./scripts/compilar.ps1 -Instalador  # además, el instalador en dist/ (requiere NSIS 3)
```

Los detalles, las pruebas y el flujo de versiones están en la [guía de desarrollo](docs/equipo/desarrollo.md).

## Términos de uso y de los proveedores

Al usar Tramtai aceptas sus [términos de uso](TERMINOS.md). En resumen: Tramtai no es un proveedor de IA ni está afiliado a ninguno; necesitas tu propia cuenta o clave de API y eres tú quien debe cumplir los términos de cada proveedor. Usar tu **suscripción** (Claude Pro/Max, ChatGPT) es solo para uso personal y local: los términos de los proveedores pueden no permitir usarla para alimentar otras aplicaciones y tu cuenta podría limitarse o suspenderse. Para tus aplicaciones, usa una **clave de API**.

## Licencia

Tramtai se publica con la licencia [PolyForm Noncommercial 1.0.0](LICENSE.md): puedes usarlo, estudiarlo y modificarlo gratis para fines personales y no comerciales. Para usarlo en una empresa o en un producto comercial hace falta una licencia aparte; escribe a [MscottB](https://github.com/MscottB).

Claude, Codex, GitHub Copilot, Gemini y Antigravity son marcas de sus respectivos dueños. Tramtai no está afiliado a ninguno de ellos y usa sus CLI oficiales tal como se instalan; con Gemini, su API pública y tu propia clave.

---

## English

**Tramtai turns Claude Code, Codex, GitHub Copilot CLI, Antigravity and the Gemini API into an OpenAI- and Anthropic-compatible API that runs on your own computer.** Any app or SDK can use them by changing just the base URL.

- **Install:** download the Windows installer (or the portable `.zip`) from [Releases](https://github.com/MscottB/Tramtai/releases/latest); it installs for your user only and needs no administrator rights. On Linux, install the desktop app from the `.deb` or `.rpm` package. On macOS, open the `.dmg` and drag Tramtai to Applications; it is not signed by an Apple developer, so the first time allow it in *System Settings → Privacy & Security → Open Anyway*. With Homebrew: `brew tap mscottb/tramtai https://github.com/MscottB/Tramtai` and then `brew install --cask tramtai` (app, no Gatekeeper prompt) or `brew install tramtai` (CLI only, macOS and Linux). CLI only: `curl -fsSL https://raw.githubusercontent.com/MscottB/Tramtai/main/scripts/instalar.sh | sh`.
- **Updates:** every hour Tramtai checks for a new version, so the notice shows up without restarting. The installed Windows app updates with one click (*Update now*), showing what's new inside the app; the other copies show the notice with the download link. `tramtai actualizar` does the same from the terminal.
- **Connect an agent:** use your Claude, ChatGPT or GitHub Copilot subscription (personal use; for GitHub Copilot, any plan: Free, Pro or business) or an API key; the agent's CLI must be installed. The agents your apps use have no terminal and can't write files or reach the network (Claude and GitHub Copilot without tools, Codex read-only), and each one is checked with its real CLI before your apps can use it; Antigravity runs without tools too and, until its test passes with a Gemini API key, is used from *Try*, the terminal and the CLI.
- **Point your app** at `http://127.0.0.1:17373/openai/v1` (or `/anthropic`) with the key from *My APIs*, and pick agent and model in the `model` field, e.g. `claude/sonnet`. To send files, follow long jobs or retry, use the Tramtai API (`/api/v1`, with its OpenAPI at `/api/v1/openapi.json`); *My APIs → Which URL do I use?* compares the three with examples to copy.
- **Shaped responses:** ask for JSON with a template (extract data, classify, summary, tasks…) or field by field, without writing JSON Schema. If the response does not match, you see which field failed and fix it with one click, or let it be corrected on its own up to 3 times.
- **Recipes and fallback:** in *Try*, set instructions, agent, a JSON schema and fallback agents, test them with your files and save them as an API in one click. If the main agent hits its limit or fails before answering, the request continues with the next one.
- **Budgets:** each API, and the whole workspace, can have a daily or monthly limit in requests, tokens or dollars: a warning at 80% and, at 100%, a 429 error until the next period starts or you reset the usage.
- **Help inside the app:** step-by-step guides, what to do about each error and a search box; *Try → View as code* gives you each test ready in curl, Python or Node.
- **Release notes in English:** what's new in each version shows in the app's language, with a link to every change on GitHub.
- The desktop app, Web UI, interactive terminal and CLI are available in English and Spanish. The CLI follows your system language; set `TRAMTAI_IDIOMA=en` to force English.
- Everything stays local: it listens only on `127.0.0.1`, has no telemetry, and responses expire after 24 hours.
- **Keys:** provider keys (`tramtai claves guardar NAME`) are stored encrypted in the system store (Windows Credential Manager, macOS Keychain or the Linux Secret Service), never in Tramtai's files. Each API's key is shown only when it is created or regenerated, and the Web UI opens from the app or with `tramtai abrir`, not with that key.

The detailed documentation is in Spanish. See the [product page](https://mscottb.github.io/Tramtai/) for an English overview. Free for personal and non-commercial use under [PolyForm Noncommercial 1.0.0](LICENSE.md); contact [MscottB](https://github.com/MscottB) for commercial licensing.
