# M0b: validación de los modos con clave de API

M0b comprueba que cada adaptador funciona en el modo que Tramtai usa por defecto: con clave de API y aislado de la configuración del usuario. Las pruebas están en `pruebas/m0b` y se omiten solas cuando falta la clave correspondiente.

## Qué valida cada prueba

| Prueba | Clave | Qué demuestra |
|---|---|---|
| `TestClaudeClaveAPI` | `ANTHROPIC_API_KEY` | Claude Code en modo `--bare`, que solo admite clave de API y no carga `CLAUDE.md`; informa uso y modelo efectivo. |
| `TestCodexHomePrivado` | `CODEX_API_KEY` (u `OPENAI_API_KEY`) | Codex con un `CODEX_HOME` privado y vacío: no carga tu `AGENTS.md` global y acepta las opciones de restricción. |
| `TestGeminiAPIDirecta` | `GEMINI_API_KEY` | Llamada directa a la Gemini API: salida con JSON Schema, uso de tokens, modelo efectivo y streaming SSE. |
| `TestAntigravityProveedorGemini` | `GEMINI_API_KEY` | `agy` con `modelProvider: "gemini"` y un home privado: falla con una clave inválida (no usa tu sesión OAuth) y responde con la válida. |

## Cómo ejecutarlo

En PowerShell, solo para esa sesión (no se guardan en ningún archivo):

```powershell
$env:ANTHROPIC_API_KEY = "<tu clave de Anthropic Console>"
$env:CODEX_API_KEY     = "<tu clave de OpenAI>"
$env:GEMINI_API_KEY    = "<tu clave de Google AI Studio>"
go test -tags m0b -v -count=1 ./pruebas/m0b
```

Puedes definir solo algunas: las pruebas sin clave aparecen como `SKIP`.

Variables opcionales: `M0B_CLAUDE_MODELO` (por defecto `haiku`), `M0B_CODEX_MODELO`, `M0B_GEMINI_MODELO` (por defecto, el primer modelo `flash` disponible), `M0B_AGY_MODELO`, y `M0B_CLAUDE`, `M0B_CODEX`, `M0B_AGY` para indicar la ruta de cada ejecutable.

## Seguridad y coste

- Las claves solo se leen del entorno. Cada CLI recibe únicamente la suya y se redactan de cualquier mensaje de error.
- La clave de Gemini viaja en la cabecera `x-goog-api-key`, nunca en la URL.
- Codex y Antigravity usan directorios temporales propios que se borran al terminar; tu `~/.codex` y tu `~/.gemini` no se tocan.
- Cada prueba hace una o dos llamadas cortas. El coste total es de céntimos, pero se factura en cada cuenta de API.

## Resultado parcial (12-10-2026, sin claves reales)

Comprobación negativa con una clave falsa en cada CLI. Si alguno hubiera respondido, seguiría usando la sesión de suscripción:

| CLI | Configuración | Resultado |
|---|---|---|
| Claude Code 2.1.288 | `--bare` con `ANTHROPIC_API_KEY` falsa | Error y 0 tokens: no recurre a la suscripción. |
| Codex CLI 0.160.0 | `CODEX_HOME` vacío con `CODEX_API_KEY` falsa | `401 Unauthorized` de `api.openai.com`: usa la clave, no la sesión de ChatGPT. |
| Antigravity 1.2.16 | Home privado, `modelProvider: "gemini"`, `GEMINI_API_KEY` falsa | `API_KEY_INVALID` de la Gemini API: respeta `modelProvider` y no usa el OAuth personal. |

Queda la mitad positiva: ejecutar las cuatro pruebas con claves válidas.

## Cómo interpretar el resultado

- `PASS` en las cuatro: el modo clave queda validado y M0b se da por cerrado.
- Fallo en `TestAntigravityProveedorGemini` con el mensaje "sigue usando la sesión OAuth": `agy` no respeta `modelProvider` desde un home privado y el adaptador de Antigravity debe quedar desactivado.
- Fallo por "cargó archivos de reglas": el aislamiento de ese CLI no es completo y debe revisarse su adaptador.
