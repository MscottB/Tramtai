# Ejemplos

Ejemplos que funcionan tal cual contra un Tramtai en marcha, en Python, JavaScript, Go y curl. La explicación de cada
uno está en la [guía del desarrollador](../desarrolladores.md).

Todos leen tres variables:

| Variable | Qué es | Por defecto |
|---|---|---|
| `TRAMTAI_URL` | La URL de tu API (Mis APIs) | `http://127.0.0.1:17373` |
| `TRAMTAI_CLAVE` | Su clave | Ninguno: es obligatoria |
| `TRAMTAI_MODELO` | Agente y modelo | `claude/haiku` |

- **Python:** `pip install -r python/requirements.txt` (todas las dependencias fijadas, generadas desde
  `requirements.in` con `uv pip compile`) y `python python/herramientas.py`.
- **JavaScript (Node 20+):** `cd javascript && npm install` y `node herramientas.mjs`.
- **Go:** `cd go && go run ./herramientas`.
- **curl:** `sh curl/herramientas.sh` (los que encadenan pasos usan [jq](https://jqlang.org/)).

El CI los ejecuta en cada cambio contra un agente simulado (`go test -tags ejemplos ./internal/api/`).

## En inglés

Cada lenguaje tiene en `en/` los mismos ejemplos en inglés, los que enseña la guía del sitio en inglés:
`first_message`, `file`, `json_response`, `tools`, `tools_native_api`, `long_job` y `limits`. Leen `TRAMTAI_URL`,
`TRAMTAI_KEY` y `TRAMTAI_MODEL`, y el de archivos usa `sample.txt`. Los campos y las rutas de la API propia
(`mensajes`, `rol`, `contenido`, `/api/v1/trabajos`…) se quedan como están, porque son los que la API acepta. Comparten
las dependencias de su lenguaje: `python python/en/tools.py`, `cd javascript && node en/tools.mjs`,
`cd go && go run ./en/tools` y `sh curl/en/tools.sh`.

## In English

Each language has the same examples in English under `en/`: `first_message`, `file`, `json_response`, `tools`,
`tools_native_api`, `long_job` and `limits`. They read `TRAMTAI_URL`, `TRAMTAI_KEY` and `TRAMTAI_MODEL`. The native
API's field names and routes (`mensajes`, `rol`, `contenido`, `/api/v1/trabajos`…) stay as they are, because they are
what the API accepts. Install the dependencies as above, then run `python python/en/tools.py`,
`cd javascript && node en/tools.mjs`, `cd go && go run ./en/tools` or `sh curl/en/tools.sh`.
