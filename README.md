<div align="center">

<img src="escritorio/icono/tramtai.png" alt="" width="96">

# Tramtai

**Tus agentes de IA, como una API local.**

Tramtai convierte Claude Code, Codex, GitHub Copilot CLI y la API de Gemini en una API compatible con OpenAI y Anthropic que funciona en tu propio equipo.<br>
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

- **Compatible con lo que ya usas:** los SDK de OpenAI y Anthropic, con respuesta en vivo y herramientas (`tools`), y una API propia para trabajos largos.
- **Agente y modelo en cada petición:** `claude/sonnet`, `codex`, `copilot`, `gemini/…` o un perfil, con respaldo a otro agente si el primero falla.
- **Archivos y respuestas con forma:** PDF, Word, Excel o PowerPoint como entrada y JSON validado como salida, sin escribir JSON Schema.
- **Una API por aplicación,** con su clave, sus modelos, sus instrucciones y su presupuesto.
- **Tu suscripción o tu clave de API,** en la app de escritorio, la Web UI, la terminal o la CLI, en español e inglés.
- **Local y privado:** por defecto escucha solo en `127.0.0.1`, sin telemetría, y las claves quedan cifradas en el almacén del sistema.

## Empezar

1. [**Descarga Tramtai**](https://mscottb.github.io/Tramtai/#descargar) para Windows, macOS o Linux y ábrelo: queda en la bandeja con el servicio en marcha.
2. En **Agentes**, conecta al menos uno con tu suscripción o con una clave.
3. En **Mis APIs**, copia la URL y la clave en tu aplicación:

```python
from openai import OpenAI

cliente = OpenAI(base_url="http://127.0.0.1:17373/openai/v1", api_key="<clave de Mis APIs>")
r = cliente.chat.completions.create(model="claude/sonnet", messages=[{"role": "user", "content": "Hola"}])
print(r.choices[0].message.content)
```

Con el SDK de Anthropic, la URL es `http://127.0.0.1:17373/anthropic`. **Mis APIs → ¿Qué URL uso?** compara las tres
APIs y trae ejemplos para copiar.

## Documentación

| Guía | Contenido |
|---|---|
| [Primeros pasos](docs/inicio.md) | Instalar en cada sistema (también portable, Homebrew y solo CLI), agentes, archivos y modelos |
| [Operación](docs/operacion.md) | Servicio, APIs por aplicación, actualizaciones, arranque con el sistema y retención |
| [Desarrolladores](docs/desarrolladores.md) | Conectar tu app: primer mensaje, archivos, JSON, herramientas y errores, con ejemplos en Python, JavaScript, Go y curl |
| [API](docs/api.md) | Las tres APIs, sus rutas, opciones y errores |
| [Arquitectura](docs/arquitectura.md) | Cómo funciona por dentro: agentes, aislamiento y seguridad |
| [Desarrollo](docs/equipo/desarrollo.md) | Compilar desde el código, pruebas y publicación |

La app trae su propia **Ayuda**, con guías paso a paso y qué hacer ante cada error.

## Términos y licencia

Tramtai no es un proveedor de IA ni está afiliado a ninguno: necesitas tu propia cuenta o clave y cumplir los términos de
cada proveedor. Usar tu **suscripción** es solo para uso personal; para tus aplicaciones, usa una **clave de API**. Más
en los [términos de uso](TERMINOS.md).

Gratis para uso personal y no comercial con la licencia [PolyForm Noncommercial 1.0.0](LICENSE.md); para una empresa o
un producto comercial, escribe a [MscottB](https://github.com/MscottB). Claude, Codex, GitHub Copilot, Gemini y
Antigravity son marcas de sus respectivos dueños.

---

## English

**Tramtai turns Claude Code, Codex, GitHub Copilot CLI and the Gemini API into an OpenAI- and Anthropic-compatible
API that runs on your own computer.** If your app already uses the OpenAI or Anthropic SDK, you only change the base
URL. Antigravity, for now, only from the Playground, the terminal and the CLI.

- **Compatible:** the OpenAI and Anthropic SDKs with streaming and tools, plus a native API for long jobs.
- **Agent and model per request,** with fallback to another agent if the first one fails.
- **Files in, shaped JSON out:** PDF, Word, Excel or PowerPoint as input, validated JSON as output.
- **One API per app,** with its own key, models, instructions and budget.
- **Local and private:** listens only on `127.0.0.1` by default, no telemetry, keys encrypted in the system store.

[**Download**](https://mscottb.github.io/Tramtai/#descargar) for Windows, macOS or Linux, connect an agent in *Agents*
and copy the URL and key from *My APIs* into your app. The app, Web UI, terminal and CLI are in English and Spanish;
the detailed documentation is in Spanish, and the [product page](https://mscottb.github.io/Tramtai/) has an English
overview. Free for personal and non-commercial use under [PolyForm Noncommercial 1.0.0](LICENSE.md); contact
[MscottB](https://github.com/MscottB) for commercial licensing.
