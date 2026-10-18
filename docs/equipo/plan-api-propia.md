# API propia `/api/v1` explicada (v0.42) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que quien conecta una app sepa qué URL usar y cómo hablar con la API propia (guía, ejemplos, «Ver como
código», OpenAPI), y que el aviso de antes de actualizar muestre cada versión que se salta.

**Architecture:** Casi todo es Web UI: un módulo `web/src/ejemplos.ts` genera el código de los ejemplos (Mis APIs) y
de la prueba actual (Probar); un componente `GuiaUrls.vue` explica los tres contratos. En Go solo hay dos cambios: el
OpenAPI incluido en el ejecutable y servido en `GET /api/v1/openapi.json` (registrado dentro de `contrato("nativa")`,
así hereda la protección del resto) y el parámetro `hasta` de las novedades.

**Tech Stack:** Go 1.27 (`embed`), Vue 3 + Quasar + TypeScript.

**Spec:** `docs/equipo/api-propia.md`

## Global Constraints

- El comportamiento de la API no cambia; ninguna ruta nueva salvo `GET /api/v1/openapi.json`.
- `openapi.json`: OpenAPI 3.1; las rutas van sin el prefijo (`/trabajos/{id}`) y `servers[0].url` es
  `<esquema>://<host><prefijo>/api/v1`, con el prefijo sacado de `r.RequestURI` (en un punto `/p/<nombre>` lo quita
  `http.StripPrefix` de `URL.Path`, no de `RequestURI`) y el esquema `https` si `r.TLS != nil`.
- Las operaciones que solo existen en la instancia (`DELETE /trabajos`) llevan `x-tramtai-solo-instancia: true`; un
  punto de acceso las quita al servir el documento, y la Web UI al descargarlo para una API.
- Probar usa siempre `<tu clave>`; Mis APIs muestra la clave como hoy (oculta salvo que se pida verla).
- Textos nuevos en `es.ts` y `en.ts`; mensajes nuevos del servidor con traducción en `internal/idioma/en_*.go`.
- Comprobaciones: `go vet ./...` (también `GOOS=linux` y `GOOS=darwin`), `go test ./...`, `cd escritorio && go vet ./...`,
  `cd web && npm run build`, `graphify update .`.

## Review Focus

1. `TestOpenAPICubreLasRutas` falla si se añade una ruta `/api/v1` en `api.go` sin describirla, o al revés → Tarea 1.
2. `servers` es correcto en la instancia, en `/p/<nombre>` y en un punto con puerto propio → Tarea 1.
3. El código de «Ver como código» con esquema, respaldo y archivo funciona tal cual al copiarlo (curl y Python) → Tarea 8.
4. El aviso de actualización con `hasta` no muestra versiones posteriores a la nueva ni la instalada → Tarea 2.

---

### Tarea 1: OpenAPI incluido y servido

**Files:**
- Create: `internal/api/openapi.json`, `internal/api/openapi.go`
- Modify: `internal/api/api.go` (`Rutas`: `mux.HandleFunc("GET /api/v1/openapi.json", s.openapi)` dentro de
  `contrato("nativa")`)
- Test: `internal/api/openapi_test.go`

**Interfaces:**
- Produces: `//go:embed openapi.json var documentoOpenAPI []byte`; `func (s *Servidor) openapi(w, r)`;
  `func documentoPara(base string, punto bool) ([]byte, error)` (decodifica, fija `servers`, quita las operaciones
  `x-tramtai-solo-instancia` si `punto`, codifica).

- [ ] Escribir `openapi.json` a partir de `nativa.go` y `docs/api.md`: las rutas `/api/v1` de `Rutas` más
  `/openapi.json`; `components.schemas` `SolicitudEjecucion`, `Mensaje`, `ConfigSalida`, `ConfigEjecucion`, `Trabajo`,
  `Carga`, `Evento`, `Limites`, `Modelo`, `Capacidad`, `Error`; `securitySchemes` `bearer` (`http`, `bearer`) y
  `x-api-key` (`apiKey`, cabecera); respuestas de error con `$ref: Error`; `/trabajos/{id}/eventos` con
  `text/event-stream` y la cabecera `Last-Event-ID`; `/cargas` con `multipart/form-data` y el campo `archivo`;
  `Idempotency-Key` en `POST /trabajos` y `/ejecutar`.
- [ ] Prueba (roja): `TestOpenAPICubreLasRutas` lee `api.go` con la expresión
  `HandleFunc\("(GET|POST|DELETE) /api/v1(/[^"]*)"`, convierte cada ruta a su clave y método de `paths` y comprueba
  las dos direcciones (ruta→documento y documento→ruta); valida que el documento es JSON y que `openapi` empieza
  por `3.1`.
- [ ] Prueba (roja): `TestOpenAPIServers`: con el servidor de prueba de `api_test.go`, `GET /api/v1/openapi.json` sin
  clave → 401; con clave → 200 y `servers[0].url == "http://<host>/api/v1"`; con un `Punto` montado detrás de
  `http.StripPrefix("/p/demo", …)` → `http://<host>/p/demo/api/v1` y sin `delete` en `/trabajos`.
- [ ] Implementar `openapi.go` y la ruta; pasar las pruebas.
- [ ] Commit `feat(api): descripción OpenAPI de /api/v1`.

### Tarea 2: Novedades hasta la versión nueva

**Files:**
- Modify: `internal/servicio/web.go` (`/ui/actualizacion/novedades`: `hasta := r.URL.Query().Get("hasta")`, vacío →
  `version.Texto()`)
- Test: `internal/servicio/web_test.go`, `internal/actualizacion/actualizacion_test.go`

- [ ] Prueba: con la lista falsa de versiones, `Novedades(ctx, "v0.40.0", "v0.42.0")` devuelve v0.42 y v0.41 (no
  v0.40 ni v0.43); si ya está cubierto, solo el caso con `hasta` posterior a la instalada.
- [ ] Prueba: en `web_test.go`, `?desde=v0.1.0&hasta=v0.2.0` con la búsqueda desactivada → `"versiones":[]` (no habla
  con GitHub).
- [ ] Implementar; commit `feat(actualizacion): novedades hasta una versión`.

### Tarea 3: Módulo de ejemplos

**Files:**
- Create: `web/src/ejemplos.ts`
- Modify: `web/src/paginas/Accesos.vue` (quita `ejemplosPara` y usa el módulo), `web/src/idiomas/es.ts`, `en.ts`

**Interfaces:**
- Produces: `interface Ejemplo { id: string; titulo: () => string; codigo: (clave: string) => string; nota?: string }`;
  `interface Prueba { mensaje: string; instrucciones?: string; modelo: string; esfuerzo?: string; respaldo?: string[];
  esquema?: unknown; archivos?: string[] }`;
  `function ejemplos(o: { url: string; modelo: string; contratos: string[]; prueba?: Prueba }): Ejemplo[]`.
- Sin `prueba`: los ejemplos de hoy (OpenAI, Anthropic, curl, entorno) y, si `contratos` incluye `nativa`,
  `curl-nativa`, `python-nativa` y `node-nativa` con mensaje `t('conectar.hola')`, un archivo `informe.pdf` y el
  esquema `{type: object, properties: {resumen: {type: string}}, required: [resumen]}`.
- Con `prueba`: los nativos y `python` / `curl` de OpenAI con sus datos; sin archivos se omite el paso de subida;
  en OpenAI, `instrucciones` va como mensaje `system`, `esfuerzo` como `reasoning_effort` y el esquema como
  `response_format: {type: "json_schema", json_schema: {name: "salida", schema}}`; con archivos o respaldo, las
  pestañas OpenAI llevan `nota = t('codigo.soloNativa')`.

- [ ] Mover `ejemplosPara` tal cual a `ejemplos.ts` y comprobar que Mis APIs muestra lo mismo (build + navegador).
- [ ] Añadir los tres nativos: (1) `POST /cargas` multipart `archivo=@informe.pdf` → `identificador_carga`;
  (2) `POST /trabajos` con `Idempotency-Key`, `modelo`, `mensajes`, `cargas`, `salida: {formato: "json", esquema}`;
  (3) `GET /trabajos/{id}/eventos` imprimiendo el texto de cada evento de salida; (4) `GET /trabajos/{id}/salida`.
  En curl, `jq -r` extrae los identificadores y un comentario lo dice; Python con `requests` (`stream=True`,
  `iter_lines`); Node con `fetch` y `body.getReader()`. Los nombres de campos y eventos, de `nativa.go`.
- [ ] Añadir la variante con `prueba`.
- [ ] `npm run build`; commit `feat(web): ejemplos de la API propia`.

### Tarea 4: Guía «¿Qué URL uso?»

**Files:**
- Create: `web/src/componentes/GuiaUrls.vue`
- Modify: `web/src/paginas/Accesos.vue` (sobre las URL de la API principal y en el bloque Conectar del detalle),
  `es.ts`, `en.ts`

**Interfaces:**
- Produces: `<GuiaUrls :contratos="string[]" />`: una tarjeta por contrato aceptado (título, ruta, para qué) y un
  `q-expansion-item` «Qué permite cada una» con la tabla de la spec; las filas salen de una lista
  `[clave, openai, anthropic, nativa]` con textos `guia.*`.

- [ ] Componente y textos (es/en); en móvil, las tarjetas en columna y la tabla con su propio desplazamiento
  horizontal.
- [ ] `npm run build`; commit `feat(web): guía ¿Qué URL uso?`.

### Tarea 5: Descargar OpenAPI

**Files:**
- Modify: `web/src/api.ts` (`openapi: () => pedir<Record<string, any>>('GET', '/api/v1/openapi.json')`),
  `web/src/paginas/Accesos.vue` (botón en el detalle de una API con `nativa`)

- [ ] El botón pide el documento de la instancia con la sesión, fija `servers = [{url: punto.url + '/api/v1'}]`,
  quita las operaciones `x-tramtai-solo-instancia` y lo guarda con `Blob` y un `<a download>` como
  `tramtai-<api>-openapi.json`.
- [ ] `npm run build`; commit `feat(web): descargar OpenAPI`.

### Tarea 6: «Ver como código» en Probar

**Files:**
- Create: `web/src/componentes/VerComoCodigo.vue`
- Modify: `web/src/paginas/Ejecutar.vue` (botón en la tarjeta de la receta), `es.ts`, `en.ts`

**Interfaces:**
- Consumes: `ejemplos({ url, modelo, contratos: ['nativa', 'openai'], prueba })` de la Tarea 3.
- Produces: `<VerComoCodigo v-model="abierto" :prueba="Prueba" :url="string" />`: pestañas por ejemplo, bloque de
  código con copiar (el mismo `bloque-codigo` que Mis APIs) y la nota si la hay.

- [ ] La URL es la de la API que se edita (`editando`) o la de la API principal; la clave, `<tu clave>`.
- [ ] La prueba sale de `receta` (instrucciones, modelo, esfuerzo, respaldo, esquema), del mensaje y de los nombres
  de `archivos`.
- [ ] `npm run build`; commit `feat(web): ver la prueba como código`.

### Tarea 7: Aviso de actualización con cada versión

**Files:**
- Create: `web/src/componentes/ListaNovedades.vue` (el `q-list` de `App.vue:203-207`)
- Modify: `web/src/App.vue` (los dos diálogos), `web/src/api.ts` (`novedades(desde, hasta?)`)

- [ ] Sacar la lista a `ListaNovedades.vue` (`:versiones="Novedad[]"`) y usarla en `dialogoNovedades`.
- [ ] Al abrirse `dialogoActualizar` con `actualizacion.disponible`, pedir `api.novedades(actualizacion.actual,
  actualizacion.version)` (una vez por versión nueva; `catch` → `[]`); con dos o más, `ListaNovedades` y el enlace
  «Ver todas»; si no, `NotasVersion` con `actualizacion.notas` como hoy.
- [ ] `npm run build`; commit `feat(web): el aviso de actualización muestra cada versión`.

### Tarea 8: Documentación y verificación

**Files:**
- Modify: `docs/api.md`, `web/src/idiomas/es.ts` y `en.ts` (Ayuda: «¿Qué URL uso para conectar mi app?», «¿Cómo veo
  mi prueba como código?»), `site/ayuda.html` (sección «Qué URL usar», es/en), `docs/equipo/pendientes.md`,
  `README.md` (si habla de las URL, enlace a la guía)

- [ ] `docs/api.md`: «¿Qué URL uso?» con la tabla y la guía paso a paso (subir, crear, seguir con `Last-Event-ID`,
  resultado, cancelar, reintentar, idempotencia, errores frecuentes, OpenAPI).
- [ ] Ejecutar una vez, contra el servidor de prueba (`prueba-presupuestos`), el curl y el Python nativos copiados de
  Mis APIs y el de «Ver como código» con esquema, respaldo y un archivo, con `claude/haiku`.
- [ ] Navegador: guía y tabla en Mis APIs y en el detalle; las pestañas nativas con la URL, la clave y el modelo de
  la API; «Ver como código» con la nota en OpenAI; Descargar OpenAPI; el aviso de actualización con dos versiones
  saltadas (servidor de prueba compilado con una versión dos atrás); móvil y escritorio, claro y oscuro.
- [ ] Comprobaciones globales; commit `docs: API propia explicada`.
