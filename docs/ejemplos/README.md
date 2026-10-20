# Ejemplos

Ejemplos que funcionan tal cual contra un Tramtai en marcha, en Python, JavaScript, Go y curl. La explicación de cada
uno está en la [guía del desarrollador](../desarrolladores.md).

Todos leen tres variables:

| Variable | Qué es | Por defecto |
|---|---|---|
| `TRAMTAI_URL` | La URL de tu API (Mis APIs) | `http://127.0.0.1:17373` |
| `TRAMTAI_CLAVE` | Su clave | — |
| `TRAMTAI_MODELO` | Agente y modelo | `claude/haiku` |

- **Python:** `pip install -r python/requirements.txt` y `python python/herramientas.py`.
- **JavaScript (Node 20+):** `cd javascript && npm install` y `node herramientas.mjs`.
- **Go:** `cd go && go run ./herramientas`.
- **curl:** `sh curl/herramientas.sh` (los que encadenan pasos usan [jq](https://jqlang.org/)).

El CI los ejecuta en cada cambio contra un agente simulado (`go test -tags ejemplos ./internal/api/`).
