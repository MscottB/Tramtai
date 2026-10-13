# Tramtai

<img src="escritorio/icono/tramtai.png" alt="" width="72" align="right" />

**Tus agentes de IA como una API en tu propio equipo.** Tramtai convierte Claude Code, Codex CLI y Gemini en un servicio local compatible con OpenAI y Anthropic, así que cualquier aplicación o SDK puede usarlos cambiando solo la URL base.

- **Compatible con lo que ya usas:** SDK de OpenAI y Anthropic (Python, Node, curl…), con respuesta en vivo.
- **Elige agente y modelo en cada petición:** `claude/sonnet`, `codex`, `gemini/gemini-flash-latest` o un perfil como `perfil/rapido`.
- **Archivos y respuestas estructuradas:** PDF, Word, Excel, PowerPoint o ZIP como entrada; JSON validado con un esquema como salida.
- **Una URL y una clave por aplicación,** con modelos permitidos o fijos, agrupadas para activarlas o detenerlas juntas.
- **Cuatro formas de usarlo** sobre los mismos trabajos: Web UI (en español e inglés, con ayuda integrada), aplicación de escritorio, terminal interactiva y línea de órdenes.
- **Arranca solo con Windows,** oculto en la bandeja: sin abrir ninguna terminal.
- **Local y privado:** escucha solo en `127.0.0.1`; los resultados se guardan 24 horas y se pueden borrar cuando quieras, y las claves de API van al Administrador de credenciales de Windows. Las consultas no quedan como chats en el historial de Claude Code ni de Codex.

## Empezar en tres pasos

```powershell
./scripts/compilar.ps1          # 1. compila bin/tramtai.exe (requiere Go 1.27)
./bin/tramtai.exe abrir         # 2. inicia el servicio y abre la Web UI
```

3. En la Web UI, **Agentes** te dice qué falta para cada agente (instalar su CLI o guardar su clave) y **Conectar apps** te da la URL, la clave y ejemplos listos para copiar.

En este equipo entras sin escribir ninguna clave: Tramtai reconoce que el navegador es tuyo.

## Conectar una aplicación

```python
from openai import OpenAI

cliente = OpenAI(base_url="http://127.0.0.1:17373/openai/v1", api_key="<clave de Conectar apps>")
r = cliente.chat.completions.create(model="claude/sonnet", messages=[{"role": "user", "content": "Hola"}])
print(r.choices[0].message.content)
```

Con el SDK de Anthropic, la URL base es `http://127.0.0.1:17373/anthropic`.

## Otras formas de usarlo

```powershell
tramtai                                        # terminal interactiva: todo desde un menú
tramtai ejecutar "Resume en una frase qué es Go"
tramtai ejecutar -f acta.pdf -e compromisos.schema.json "Extrae los compromisos"
tramtai claves guardar ANTHROPIC_API_KEY       # guarda una clave sin mostrarla
bin/tramtai-escritorio.exe                     # aplicación de escritorio con icono en la bandeja
```

## Documentación

| Guía | Contenido |
|---|---|
| [Primeros pasos](docs/inicio.md) | Instalar, dejar listos los agentes, ejecutar, archivos, modelos y configuración |
| [Operación](docs/operacion.md) | Servicio, instancias global y por proyecto, puntos de acceso, retención |
| [API](docs/api.md) | Rutas, fachadas OpenAI y Anthropic, perfiles, errores |
| [Arquitectura](docs/arquitectura.md) | Cómo funciona por dentro: agentes, aislamiento, seguridad y recuperación |

¿Desarrollas Tramtai? La guía del equipo está en [docs/equipo](docs/equipo/desarrollo.md).
