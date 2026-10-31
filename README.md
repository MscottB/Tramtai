<div align="center">

<img src="escritorio/icono/tramtai.png" alt="" width="96">

# Tramtai

**Tus agentes de IA, como una API local.**

Tramtai convierte Claude Code, Codex, GitHub Copilot CLI y la Gemini API en una API compatible con OpenAI y Anthropic que funciona en tu propio equipo.<br>
Si tu app ya usa el SDK de OpenAI o el de Anthropic, solo cambias la URL base. Antigravity, por ahora, solo desde Probar, la terminal y la CLI.

[![CI](https://github.com/MscottB/Tramtai/actions/workflows/ci.yml/badge.svg)](https://github.com/MscottB/Tramtai/actions/workflows/ci.yml)
[![Versión](https://img.shields.io/github/v/release/MscottB/Tramtai?label=versi%C3%B3n&color=0b6e64)](https://github.com/MscottB/Tramtai/releases/latest)
[![Windows, macOS y Linux](https://img.shields.io/badge/Windows%20%C2%B7%20macOS%20%C2%B7%20Linux-0078d4)](https://github.com/MscottB/Tramtai/releases/latest)
[![Licencia](https://img.shields.io/badge/licencia-PolyForm%20Noncommercial-55666a)](LICENSE.md)

[**Descargar**](https://mscottb.github.io/Tramtai/#descargar) · [Página del producto](https://mscottb.github.io/Tramtai/) · [Ayuda](https://mscottb.github.io/Tramtai/ayuda/) · [Desarrolladores](https://mscottb.github.io/Tramtai/desarrolladores.html) · [English](#english)

<br>

<img src="docs/imagenes/inicio.png" alt="Inicio de Tramtai: el mapa de la red con tu app, Tramtai y una línea por agente" width="880">

</div>

## Qué ofrece

- **Compatible con lo que ya usas:** los SDK de OpenAI y Anthropic, con respuesta en vivo y herramientas (`tools`), y una API propia para peticiones largas.
- **Agente y modelo en cada petición:** `claude/sonnet`, `codex`, `copilot`, `gemini/…` o un perfil, con respaldo a otro agente si el primero falla.
- **Archivos y respuestas con forma:** PDF, Word, Excel o PowerPoint como entrada y JSON validado como salida, sin escribir JSON Schema.
- **Una API por aplicación,** con su clave, sus modelos, sus instrucciones y su presupuesto.
- **Tu suscripción o tu clave de API,** en la app de escritorio, la Web UI, la terminal o la CLI, en español e inglés.
- **Todo desde la terminal:** `tramtai terminal` tiene las mismas páginas que la app, de Inicio a la Ayuda, y sirve sin abrir la app y por SSH.
- **Local y privado:** por defecto escucha solo en `127.0.0.1`, sin telemetría, y las claves quedan cifradas en el almacén del sistema.

## Empezar

1. [**Descarga Tramtai**](https://mscottb.github.io/Tramtai/#descargar) para Windows, macOS o Linux y ábrelo: queda en la bandeja con el servicio en marcha.
2. En **Agentes**, conecta al menos uno con tu suscripción o con una clave.
3. En **Mis APIs**, copia la URL y crea la clave (se ve una sola vez) para tu aplicación:

```python
from openai import OpenAI

cliente = OpenAI(base_url="http://127.0.0.1:17373/openai/v1", api_key="<clave de Mis APIs>")
r = cliente.chat.completions.create(model="claude/sonnet", messages=[{"role": "user", "content": "Hola"}])
print(r.choices[0].message.content)
```

Con el SDK de Anthropic, la URL es `http://127.0.0.1:17373/anthropic`. **Mis APIs → ¿Qué URL uso?** compara las tres
APIs y trae ejemplos para copiar.

**Solo con la terminal** (sin la app, también por SSH al equipo donde corre Tramtai):

```sh
tramtai servicio iniciar -d   # el servicio en segundo plano
tramtai terminal              # las mismas páginas que la app
```

Tab abre el menú, del 1 al 7 vas a cada página, `?` abre la ayuda de la página, Esc vuelve, `W` abre la Web UI en el
navegador, `r` reintenta si el servicio no responde y `q` sale. En Probar, `c` cancela la petición en curso. Necesita una
ventana de 60×20 como mínimo; con `NO_COLOR` va sin color y con `TRAMTAI_ASCII=1`, sin caracteres de dibujo.

## Modo servidor

Nuevo en la v0.58: un Tramtai del sistema para Linux con systemd y Windows Server, que arranca con el equipo sin que
nadie inicie sesión y se abre desde otros equipos **solo por HTTPS** (detrás de un proxy, con tu certificado o con
Let’s Encrypt) y con la contraseña del administrador, con TOTP opcional:

```sh
sudo ./tramtai servidor instalar              # desde el .tar.gz; con el paquete .deb o .rpm: sudo tramtai-servidor instalar
sudo tramtai-servidor administrador contraseña
```

En Windows Server, `tramtai.exe servidor instalar` desde una consola de administrador. **Es otro entorno:** tus APIs,
claves y ajustes de escritorio no pasan al servidor, ni al revés. Todo, en la [guía del servidor](docs/servidor.md).

## Documentación

| Guía | Contenido |
|---|---|
| [Primeros pasos](docs/inicio.md) | Instalar en cada sistema (también portable, Homebrew y solo CLI), agentes, archivos y modelos |
| [Operación](docs/operacion.md) | Servicio, la terminal y sus teclas, APIs por aplicación, actualizaciones, arranque con el sistema y retención |
| [Desarrolladores](docs/desarrolladores.md) | Conectar tu app: primer mensaje, archivos, JSON, herramientas y errores, con ejemplos en Python, JavaScript, Go y curl |
| [Servidor](docs/servidor.md) | El modo servidor: instalar, dos entornos, HTTPS y proxies, administrador, claves, suscripción y qué hacer si algo falla |
| [API](docs/api.md) | Las tres APIs, sus rutas, opciones y errores |
| [Arquitectura](docs/arquitectura.md) | Cómo funciona por dentro: agentes, aislamiento y seguridad |
| [Desarrollo](docs/equipo/desarrollo.md) | Compilar desde el código, pruebas y publicación |

La app trae su propia **Ayuda**, con guías paso a paso y qué hacer ante cada error.

## Aviso

Los cinco riesgos de usar Tramtai, los mismos que enseñan la app y el instalador:

<!-- riesgos:es -->
1. Tu cuenta con un proveedor puede limitarse, suspenderse o cerrarse si usas tu suscripción con Tramtai, y el proveedor puede cambiar sus reglas sin avisar. El autor no puede recuperarla.
2. Las condiciones de Tramtai (TOTP, una petición a la vez) bajan el riesgo, pero no lo quitan ni son una opinión legal sobre los términos del proveedor.
3. Si usas el modo servidor, lo que abres a la red es responsabilidad tuya: HTTPS, cortafuegos, contraseñas, copias y actualizaciones.
4. Si usas el modo servidor y otras personas lo usan, tú respondes de sus datos ante la ley que te aplique.
5. Tramtai es gratuito y se entrega tal cual, sin garantías.
<!-- /riesgos:es -->

## Términos y licencia

Tramtai no es un proveedor de IA ni está afiliado a ninguno: necesitas tu propia cuenta o clave y cumplir los términos de
cada proveedor. Usar tu **suscripción** es solo para uso personal; para tus aplicaciones, usa una **clave de API**. Se
aceptan de forma expresa al instalar o al abrir Tramtai por primera vez, una vez en cada entorno. Más en los
[términos de uso](TERMINOS.md).

Gratis para uso personal y no comercial con la licencia [PolyForm Noncommercial 1.0.0](LICENSE.md); para una empresa o
un producto comercial, escribe a [MscottB](https://github.com/MscottB). Claude, Codex, GitHub Copilot, Gemini y
Antigravity son marcas de sus respectivos dueños.

---

## English

**Tramtai turns Claude Code, Codex, GitHub Copilot CLI and the Gemini API into an OpenAI- and Anthropic-compatible
API that runs on your own computer.** If your app already uses the OpenAI or Anthropic SDK, you only change the base
URL. Antigravity, for now, only from the Playground, the terminal and the CLI.

- **Compatible:** the OpenAI and Anthropic SDKs with streaming and tools, plus a native API for long-running requests.
- **Agent and model per request,** with fallback to another agent if the first one fails.
- **Files in, shaped JSON out:** PDF, Word, Excel or PowerPoint as input, validated JSON as output.
- **One API per app,** with its own key, models, instructions and budget.
- **Everything from the terminal:** `tramtai terminal` has the same pages as the app, from Home to Help, and works
  without the app open, also over SSH (`tramtai servicio iniciar -d` starts the service in the background). Tab opens the
  menu, 1 to 7 go to each page, `?` opens the page’s Help, Esc goes back and `q` quits.
- **Local and private:** listens only on `127.0.0.1` by default, no telemetry, keys encrypted in the system store.
- **Server mode (new in v0.58):** a system-wide Tramtai for Linux with systemd and Windows Server that starts with the
  computer and opens from other computers over HTTPS only, with an administrator password and optional TOTP
  (`sudo ./tramtai server install`). It’s a separate environment: your desktop APIs, keys and settings don’t carry over.
  See the [server guide](docs/servidor.md#server-guide).

**Notice.** The five risks of using Tramtai, the same ones the app and the installer show:

<!-- riesgos:en -->
1. Your account with a provider may be limited, suspended or closed if you use your subscription with Tramtai, and the provider may change its rules without notice. The author cannot recover it.
2. Tramtai’s conditions (TOTP, one request at a time) lower the risk but do not remove it, and they are not a legal opinion on the provider’s terms.
3. If you use server mode, what you open to the network is your responsibility: HTTPS, firewall, passwords, backups and updates.
4. If you use server mode and other people use it, you answer for their data under the law that applies to you.
5. Tramtai is free and provided as is, without warranties.
<!-- /riesgos:en -->

[**Download**](https://mscottb.github.io/Tramtai/#descargar) for Windows, macOS or Linux, connect an agent in *Agents*
and copy the URL from *My APIs* into your app, with a key created there (shown only once). The app, Web UI, terminal
and CLI are in English and Spanish; the detailed documentation is in Spanish, and the
[product page](https://mscottb.github.io/Tramtai/) has an English overview. Free for personal and non-commercial use
under [PolyForm Noncommercial 1.0.0](LICENSE.md); contact [MscottB](https://github.com/MscottB) for commercial
licensing.
