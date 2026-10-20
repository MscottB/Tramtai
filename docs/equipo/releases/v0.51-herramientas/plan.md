# Herramientas y guía del desarrollador (v0.51) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Herramientas (`tools`) en las tres APIs con un solo motor, y una guía del desarrollador con ejemplos en Python, JavaScript, Go y curl que el CI ejecuta.

**Architecture:** La emulación de herramientas sale de `internal/api/herramientas.go` a un paquete `internal/herramientas` (Validar, Activas, Preparar, Interpretar). Los tipos viajan en `nucleo.SolicitudEjecucion` y `nucleo.Mensaje`, que ya se guardan enteros con el trabajo (sin migración). El planificador valida al aceptar, prepara al ejecutar e interpreta dentro de su ciclo de validación de salida (correcciones y respaldo); la API propia y las fachadas solo traducen.

**Tech Stack:** Go 1.27 (stdlib, `santhosh-tekuri/jsonschema` vía `internal/salidas`), Vue 3 + Quasar (`node:test`), HTML estático del sitio con `site/sitio.js`, GitHub Actions; ejemplos con Python `openai`/`requests`, npm `openai`, Go `github.com/openai/openai-go`.

**Spec:** `docs/equipo/releases/v0.51-herramientas/spec.md`

## Global Constraints

- Todo texto de usuario en español e inglés (`internal/idioma/en_*.go`, `web/src/idiomas/{es,en}.ts`, `<span lang>` en el sitio); `TestCobertura` de `internal/idioma` exige traducción de cada `idioma.T`/`Tf` y ninguna sin uso.
- Las instrucciones al agente van en inglés, como hoy (`instruccionesHerramientas`), y no dependen del idioma de la interfaz.
- Solo funciones; no se combinan con `salida.formato: json` ni `salida.esquema`: 400 `parametro_no_soportado` con el mensaje de hoy («no se pueden combinar herramientas con una respuesta en JSON»).
- `eleccion_herramienta`: `"auto"` (predeterminada), `"ninguna"`, `"obligatoria"` o `{"nombre": "<herramienta>"}`; `llamadas_paralelas` `true` por defecto.
- Identificadores de llamada: fachadas como hoy (`call_<trabajo>_<n>`, `toolu_<trabajo>_<n>`); API propia `llamada_<trabajo>_<n>` (Ruling: la spec muestra `llamada_0`; con el id del trabajo no se repiten entre turnos — costo si no: renombrar un prefijo).
- `Interpretar` recibe el prefijo de los ids (Ruling: la spec la escribe sin él; las fachadas lo necesitan para no cambiar sus respuestas).
- Sin migración de base de datos; un trabajo sin herramientas se ve exactamente igual que hoy.
- Las pruebas de las fachadas (`internal/api/*_test.go`) pasan sin cambiar lo que esperan.
- Los ejemplos leen `TRAMTAI_URL` (por defecto `http://127.0.0.1:17373`) y `TRAMTAI_CLAVE`.
- Antes de subir: `go vet ./...`, `go vet -tags canario ./...`, `go vet -tags ejemplos ./...`, `go vet -tags conversor ./...`, `go test ./...`, `cd web && npm test && npm run build`.

## Review Focus

1. **Dos rondas de llamadas en la misma conversación** (la app reenvía la primera y el agente pide otra): los ids nuevos no chocan con los anteriores y el `id_llamada` de cada resultado se resuelve al nombre correcto → prueba en la Tarea 5.
2. **La app reenvía `argumentos` como texto JSON** (como hace OpenAI) en vez de objeto en un mensaje `asistente` de la API propia: se acepta igual → prueba en la Tarea 1.
3. **Un trabajo con herramientas que falla, se cancela o expira:** leerlo no intenta interpretar la salida ni da 500; `salida`/`llamadas` solo en `completado` → prueba en la Tarea 5.
4. **Respaldo a otro agente con herramientas:** el respaldo recibe las mismas instrucciones y el sobre (Preparar no depende del agente) → prueba en la Tarea 3.
5. **Una receta (API por aplicación) con `salida.esquema` y una petición con herramientas:** se rechaza con 400 `parametro_no_soportado`, porque la comprobación corre tras aplicar la receta → prueba en la Tarea 5.

---

### Task 1: Tipos de herramientas en el núcleo

**Files:**
- Modify: `internal/nucleo/solicitud.go` (tipos, campos, `rolesValidos`, `Validar`)
- Test: `internal/nucleo/solicitud_test.go`

**Interfaces:**
- Produces:
  - `type Herramienta struct { Nombre string \`json:"nombre"\`; Descripcion string \`json:"descripcion,omitempty"\`; Parametros json.RawMessage \`json:"parametros,omitempty"\` }`
  - `type Llamada struct { ID string \`json:"id"\`; Nombre string \`json:"nombre"\`; Argumentos json.RawMessage \`json:"argumentos"\` }` — `UnmarshalJSON` acepta `argumentos` como objeto o como texto con un objeto JSON y lo guarda como objeto compacto.
  - `type EleccionHerramienta struct { Modo string; Nombre string }` — Modo `""`/`auto`/`ninguna`/`obligatoria`/`una`; JSON: cadena para los tres primeros, `{"nombre": …}` para `una`; cero se omite (`omitzero`).
  - `SolicitudEjecucion` gana `Herramientas []Herramienta \`json:"herramientas,omitempty"\``, `EleccionHerramienta EleccionHerramienta \`json:"eleccion_herramienta,omitzero"\``, `LlamadasParalelas *bool \`json:"llamadas_paralelas,omitempty"\``.
  - `Mensaje` gana `Llamadas []Llamada \`json:"llamadas,omitempty"\``, `IDLlamada string \`json:"id_llamada,omitempty"\``, `EsError bool \`json:"es_error,omitempty"\``.
  - `rolesValidos` admite `"herramienta"`.

- [ ] **Step 1: Write the failing tests**

```go
func TestEleccionHerramientaJSON(t *testing.T) {
	// "auto" → {Modo:auto}; {"nombre":"clima"} → {Modo:una, Nombre:clima}; "otra" → error; ida y vuelta igual.
}
func TestLlamadaArgumentosTextoUObjeto(t *testing.T) {
	// {"id":"a","nombre":"clima","argumentos":"{\"ciudad\": \"Santiago\"}"} y con el objeto dan Argumentos == `{"ciudad":"Santiago"}`;
	// "[1]" o "x" dan error.
}
func TestValidarMensajesDeHerramientas(t *testing.T) {
	// rol herramienta sin id_llamada → CodigoSolicitudInvalida; llamadas en un mensaje de usuario → CodigoSolicitudInvalida;
	// asistente con llamadas + herramienta con id_llamada → nil; una solicitud sin herramientas serializa igual que antes (sin claves nuevas).
}
```

- [ ] **Step 2: Run** `go test ./internal/nucleo/ -run 'Eleccion|LlamadaArgumentos|MensajesDeHerramientas'` — Expected: FAIL (tipos sin definir).
- [ ] **Step 3: Implement** los tipos y la validación en `internal/nucleo/solicitud.go`; mensajes de error con `idioma.T`/`Tf` y su traducción en `internal/idioma/en_servicio.go`.
- [ ] **Step 4: Run** `go test ./internal/nucleo/ ./internal/idioma/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(nucleo): herramientas, llamadas y el rol herramienta en la solicitud`.

### Task 2: Paquete `internal/herramientas`

**Files:**
- Create: `internal/herramientas/herramientas.go`, `internal/herramientas/herramientas_test.go`
- Modify: `internal/salidas/problemas.go` (exportar el constructor)
- Move: la lógica de `prepararHerramientas`, `instruccionesHerramientas`, `interpretar`, `objetoJSON`, `sobreLlamadas`, `turnoResultado` desde `internal/api/herramientas.go` (la Tarea 4 borra los originales)

**Interfaces:**
- Consumes: tipos de la Tarea 1.
- Produces:
  - `salidas.NoCumple(problemas []Problema) error` (lo que hoy es `errorSalida`, exportado).
  - `herramientas.Validar(sol nucleo.SolicitudEjecucion) error` — reglas de la spec §1 (nombres, repetidas, esquema que compila con `salidas.Compilar`, elección, «obligatoria»/«una» sin herramientas, combinación con JSON → `CodigoParametroNoSoportado`, `id_llamada` que no aparece en una llamada anterior → `CodigoSolicitudInvalida`). La combinación con JSON solo se rechaza si `Activas`.
  - `herramientas.Activas(sol nucleo.SolicitudEjecucion) bool` — hay herramientas y la elección no es `ninguna`.
  - `herramientas.Preparar(sol nucleo.SolicitudEjecucion) nucleo.SolicitudEjecucion` — escribe los mensajes `asistente` con `Llamadas` como el sobre (`sobreLlamadas`) y los `herramienta` como turno de `usuario` (`turnoResultado`, con el nombre resuelto por id); si `Activas`, además añade las instrucciones y fija `Salida.Formato="json"` y el sobre como `Salida.Esquema` (idéntico al de hoy, `enum` de nombres; solo el nombre elegido con `una`); `LlamadasParalelas=false` equivale al `SoloUna` de hoy. No modifica la entrada (copia los slices).
  - `herramientas.Interpretar(sol nucleo.SolicitudEjecucion, salida, prefijo string) (string, []nucleo.Llamada, error)` — si no `Activas`, devuelve `salida, nil, nil`. Si no, las reglas de `interpretar` de hoy; cada fallo como `salidas.NoCumple([]salidas.Problema{{Problema: "otro", Texto: …}})`; ids `fmt.Sprintf("%s_%d", prefijo, i)`.

- [ ] **Step 1: Write the failing tests** — trasladar a `herramientas_test.go` los casos de preparar e interpretar de `internal/api/herramientas_test.go` (mismos valores y mensajes), adaptados a las firmas nuevas, y añadir:

```go
func TestPrepararEscribeLaConversacion(t *testing.T)   // asistente con Llamadas → Contenido == sobre JSON exacto de hoy; herramienta (es_error) → usuario con el texto de turnoResultado y el nombre resuelto
func TestPrepararSinHerramientasNoCambiaNada(t *testing.T) // reflect.DeepEqual(Preparar(sol), sol) sin herramientas ni mensajes de herramienta
func TestPrepararNoModificaLaEntrada(t *testing.T)
func TestInterpretarDevuelveProblemas(t *testing.T)      // argumentos que no cumplen → salidas.ProblemasDe(err) no vacío y nucleo.CodigoDe == CodigoSalidaNoCumpleEsquema
func TestValidarIDLlamadaDesconocido(t *testing.T)
func TestValidarJSONSoloSiActivas(t *testing.T)          // eleccion ninguna + salida.esquema → nil; auto + esquema → CodigoParametroNoSoportado
```

- [ ] **Step 2: Run** `go test ./internal/herramientas/` — Expected: FAIL (paquete sin código).
- [ ] **Step 3: Implement** `internal/herramientas/herramientas.go` y `salidas.NoCumple` (sustituye al `errorSalida` privado en sus usos).
- [ ] **Step 4: Run** `go test ./internal/herramientas/ ./internal/salidas/ ./internal/idioma/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(herramientas): un solo motor para validar, preparar e interpretar`.

### Task 3: El planificador usa el motor

**Files:**
- Modify: `internal/planificador/planificador.go` (`aceptar`), `internal/planificador/ejecucion.go` (línea `solicitud, desde := t.Solicitud, …` y `validarSalida`)
- Test: `internal/planificador/planificador_test.go` (o `herramientas_test.go` en el mismo paquete)

**Interfaces:**
- Consumes: `herramientas.Validar`, `Preparar`, `Interpretar`, `Activas`.
- Produces: `validarSalida(sol nucleo.SolicitudEjecucion, salida string) error` — si `herramientas.Activas(sol)`, el error de `Interpretar(sol, salida, "")`; si no, lo de hoy con `sol.Salida`.

- [ ] **Step 1: Write the failing tests** (con adaptadores simulados que devuelven salidas en orden):

```go
func TestHerramientasInvalidasAlAceptar(t *testing.T)        // Enviar con herramienta sin nombre → CodigoSolicitudInvalida, nada encolado
func TestHerramientasCorreccionLuegoBien(t *testing.T)       // 1ª salida con argumentos inválidos, 2ª válida, salida.reintentos=1 → Completado; el 2º Ejecutar recibe el mensaje de corrección con el problema
func TestHerramientasSinCorreccionesFalla(t *testing.T)      // reintentos=0 → Fallido con CodigoSalidaNoCumpleEsquema
func TestHerramientasRespaldoRecibeElSobre(t *testing.T)     // principal falla antes de texto; el respaldo recibe Instrucciones con "Available tools" y Salida.Esquema del sobre
func TestSolicitudGuardadaSinPreparar(t *testing.T)          // el trabajo guardado conserva Herramientas y los mensajes originales (sin el sobre en Instrucciones)
func TestHerramientasSinTextoEnVivo(t *testing.T)            // con herramientas activas no se guarda ningún evento "texto" (el sobre interno no se ve en /eventos); sin ellas, sí
```

- [ ] **Step 2: Run** `go test ./internal/planificador/ -run Herramientas` — Expected: FAIL.
- [ ] **Step 3: Implement** `herramientas.Validar` tras `solicitud.Validar()` en `aceptar`; `herramientas.Preparar(t.Solicitud)` donde hoy se toma `t.Solicitud` para ejecutar; `validarSalida(t.Solicitud, …)`; si `herramientas.Activas(t.Solicitud)`, el callback que guarda los eventos del agente descarta los de tipo `texto`.
- [ ] **Step 4: Run** `go test ./internal/planificador/ ./internal/api/` — Expected: PASS (las fachadas aún preparan por su cuenta; `Preparar` no cambia una solicitud ya preparada sin `Herramientas`).
- [ ] **Step 5: Commit** `feat(planificador): herramientas validadas al aceptar y corregidas como un esquema`.

### Task 4: Las fachadas solo traducen

**Files:**
- Modify: `internal/api/openai.go`, `internal/api/anthropic.go`, `internal/api/herramientas.go` (queda solo lo propio de cada formato: `soloFunciones`, `eleccionOpenAI`, la lectura de `contenidoDe`)
- Modify: `docs/arquitectura.md` (paquete `herramientas` y la tabla «Dónde actúa» de la spec §2)
- Test: `internal/api/herramientas_test.go` (lo trasladado a la Tarea 2 sale de aquí), `internal/api/openai_test.go`

**Interfaces:**
- Consumes: Tareas 1–3.
- Produces: las fachadas rellenan `sol.Herramientas`, `sol.EleccionHerramienta`, `sol.LlamadasParalelas` y mensajes con `Llamadas`/`IDLlamada`/`EsError` (rol `herramienta`) en lugar de escribir el texto; responden con `herramientas.Interpretar(final.Solicitud, final.Salida, "call_"+t.ID)` (`"toolu_"+t.ID` en Anthropic) y `string(Argumentos)` donde OpenAI pide texto.

- [ ] **Step 1: Write the failing test**

```go
func TestFachadaCorrigeArgumentosInvalidos(t *testing.T) // chat/completions con tools y salida.reintentos=1 en la receta; el simulado responde primero argumentos inválidos y luego válidos → 200 con tool_calls (hoy 422)
```

- [ ] **Step 2: Run** `go test ./internal/api/ -run FachadaCorrige` — Expected: FAIL (422).
- [ ] **Step 3: Implement** la traducción en `openai.go` y `anthropic.go`; borrar de `internal/api/herramientas.go` lo que ya vive en `internal/herramientas`.
- [ ] **Step 4: Run** `go test ./internal/api/ ./...` — Expected: PASS, **sin tocar** lo que esperan las pruebas existentes de las fachadas.
- [ ] **Step 5: Commit** `refactor(api): las fachadas traducen y el motor común decide`.

### Task 5: Herramientas en la API propia

**Files:**
- Modify: `internal/api/nativa.go` (`obtenerTrabajo`, `listarTrabajos`, `salidaTrabajo`: una vista común), `internal/api/openapi.json` (campos de `SolicitudEjecucion`, `Mensaje`, `Trabajo`, esquemas `Herramienta`, `Llamada`), `docs/api.md` (sección «Herramientas en la API propia» con la spec §1, y la de las fachadas remitiendo a la guía), `web/src/api/tipos.ts` (campos opcionales nuevos)
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: Tareas 1–4.
- Produces: `vistaTrabajo(t nucleo.Trabajo) (nucleo.Trabajo, []nucleo.Llamada)` — solo si `t.Estado == Completado`, `Disponibilidad` no expirada y `herramientas.Activas(t.Solicitud)`: `Salida` = texto y llamadas con prefijo `"llamada_"+t.ID`; si no, el trabajo tal cual. El JSON del trabajo gana `llamadas` (omitido si vacío).

- [ ] **Step 1: Write the failing tests**

```go
func TestNativaHerramientasDosTurnos(t *testing.T)   // turno 1 → llamadas[0] = {id "llamada_<id>_0", nombre "clima", argumentos {"ciudad":"Santiago"}}, salida "";
                                                    // turno 2 con asistente+herramienta → salida con el texto final, sin llamadas; el simulado recibió "Tool result" con "18 °C"
func TestNativaDosRondasIDsDistintos(t *testing.T)  // Review Focus 1
func TestNativaHerramientasErrores(t *testing.T)     // sin nombre → 400 solicitud_invalida; con salida.esquema → 400 parametro_no_soportado; id_llamada desconocido → 400
func TestNativaRecetaConEsquemaYHerramientas(t *testing.T) // Review Focus 5: punto con esquema en la receta → 400 parametro_no_soportado
func TestNativaHerramientasTrabajoFallido(t *testing.T)    // Review Focus 3: fallido/cancelado → 200 sin llamadas, salida intacta
func TestNativaSinHerramientasIgualQueAntes(t *testing.T)  // el JSON de un trabajo normal no tiene "llamadas"; /salida devuelve el texto tal cual
func TestNativaCorregirConHerramientas(t *testing.T)       // falla con salida_no_cumple_esquema (conservar_salida_invalida); POST /reintentar {"corregir": true} → el nuevo trabajo termina con llamadas válidas
```

- [ ] **Step 2: Run** `go test ./internal/api/ -run Nativa` — Expected: FAIL.
- [ ] **Step 3: Implement** la vista en los tres manejadores (`/salida` con herramientas devuelve el texto) y los cambios de `openapi.json` (sin reformatear el archivo: insertar con su formato).
- [ ] **Step 4: Run** `go test ./internal/api/ && python -I -c "import json;json.load(open('internal/api/openapi.json',encoding='utf-8'))"` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(api): herramientas en la API propia`.

### Task 6: Ejemplos ejecutables y su job del CI

**Files:**
- Create: `docs/ejemplos/python/{primer_mensaje,archivo,json,herramientas,herramientas_api_propia,trabajo_largo}.py`, `docs/ejemplos/python/requirements.txt` (`openai` y `requests` con versión fija)
- Create: `docs/ejemplos/javascript/{primer_mensaje,archivo,json,herramientas}.mjs`, `docs/ejemplos/javascript/package.json` (+ `package-lock.json`, `openai` fijo)
- Create: `docs/ejemplos/go/go.mod` (módulo propio `tramtai.dev/ejemplos`, `github.com/openai/openai-go` fijo, `go.sum`), `docs/ejemplos/go/{primer_mensaje,archivo,json,herramientas}/main.go`
- Create: `docs/ejemplos/curl/{primer_mensaje,archivo,json,herramientas,herramientas_api_propia,trabajo_largo,limites}.sh`
- Create: `docs/ejemplos/muestra.txt` (el archivo que usan los ejemplos de archivos)
- Create: `internal/api/ejemplos_test.go` (`//go:build ejemplos`)
- Modify: `.github/workflows/ci.yml` (job `ejemplos`, ubuntu-latest: setup-go, setup-python, setup-node con versiones fijas como el resto; `pip install -r`, `npm ci`, `go test -tags ejemplos -run TestEjemplos -v ./internal/api/`)

**Interfaces:**
- Consumes: Tareas 4–5.
- Produces: `agenteEjemplos` (adaptador de prueba): si la última entrada contiene `Tool result` → sobre `{"content":"En Santiago hay 18 °C y está despejado.","tool_calls":[]}`; si `herramientas` activas en lo recibido (`Salida.Esquema` con `tool_calls`) → sobre con una llamada `clima {"ciudad":"Santiago"}`; si hay `Salida.Esquema` → `{"resumen":"Tramtai convierte agentes en una API local."}`; si no → `Hola desde Tramtai.` emitido como texto en vivo.

- [ ] **Step 1: Write the failing test** `TestEjemplos`: levanta `servidorCon(t, map{"simulado": agenteEjemplos{}}, …)` con `predeterminados.agente: simulado`, y para cada archivo de `docs/ejemplos/**` ejecuta su intérprete (`python`, `node`, `go run ./<dir>`, `sh`) con `TRAMTAI_URL=ts.URL`, `TRAMTAI_CLAVE=secreto`, plazo 60 s, y exige código 0 y que la salida contenga la frase esperada del ejemplo (`Hola desde Tramtai`, `resumen`, `18 °C`…). Un intérprete que falte → `t.Skip` solo fuera del CI (`os.Getenv("CI") == ""`); en el CI falla.
- [ ] **Step 2: Run** `go test -tags ejemplos -run TestEjemplos ./internal/api/` — Expected: FAIL (no hay ejemplos).
- [ ] **Step 3: Write** los ejemplos: cortos, comentados en español, sin dependencias fuera de las listadas; el de herramientas hace el ciclo completo (ofrecer `clima`, ejecutar la función local, devolver el resultado, imprimir la respuesta final).
- [ ] **Step 4: Run** el paso 2 — Expected: PASS con Python, Node y Go instalados.
- [ ] **Step 5: Commit** `feat(docs): ejemplos ejecutables en Python, JavaScript, Go y curl, probados en el CI`.

### Task 7: La guía del desarrollador

**Files:**
- Create: `docs/desarrolladores.md` (los ocho temas de la spec §3, con enlaces relativos a `docs/ejemplos/…` en lugar de copiar el código)
- Create: `site/desarrolladores.html` (es/en, estructura e índice lateral de `site/ayuda.html`; cada bloque de código es `<pre class="codigo" data-ejemplo="python/herramientas.py"></pre>` dentro de unas pestañas Python · JavaScript · Go · curl; diagrama de la ida y vuelta en SVG en línea con los tokens de color del sitio)
- Modify: `site/sitio.js` (pestañas accesibles con `role="tablist"`/`aria-selected` y teclado, y carga de `data-ejemplo` con `fetch('ejemplos/' + ruta)`; sin JavaScript o si falla, un enlace al archivo en GitHub)
- Modify: `.github/workflows/paginas.yml` (copiar `docs/ejemplos` a `site/ejemplos` antes de subir; añadir `docs/ejemplos/**` a `paths`), menú de `site/index.html` y `site/ayuda.html` («Desarrolladores» / «Developers»), `README.md` (enlace en la fila de enlaces y en la tabla de documentación), `docs/api.md` (enlace a la guía)

- [ ] **Step 1: Write** la guía y la página; la tabla «qué agentes atienden herramientas» queda con las filas del canario (Tarea 9) marcadas «por comprobar» hasta entonces.
- [ ] **Step 2: Verify** con la vista previa `pagina` (tras copiar `docs/ejemplos` a `site/ejemplos` en local, sin commitear la copia; `site/ejemplos/` va a `.gitignore`): las pestañas cambian con clic y con flechas, cada bloque carga su archivo, el idioma y el tema funcionan, y a 375 px no hay desplazamiento horizontal de la página.
- [ ] **Step 3: Commit** `docs: guía del desarrollador en el sitio y en el repo`.

### Task 8: La app enlaza y suma Go y herramientas

**Files:**
- Modify: `web/src/ejemplos.ts` (`goNativa(url, prueba, k)` con `net/http`, el mismo flujo que `pythonNativa`; y un ejemplo `python-herramientas` · «Python · herramientas» por la URL de OpenAI, el ciclo de `docs/ejemplos/python/herramientas.py` con la URL y la clave puestas), `web/src/componentes/TablaApis.vue` (fila `guia.fHerramientas`: sí, sí, sí), `web/src/paginas/Ayuda.vue` y `web/src/componentes/GuiaUrls.vue` (enlace «Guía del desarrollador» a `https://mscottb.github.io/Tramtai/desarrolladores.html`), `web/src/idiomas/{es,en}.ts` (pregunta frecuente «¿Cómo uso herramientas?» en dos frases + enlace; claves de los ejemplos nuevos), `internal/recursos/web` (compilado)
- Test: `web/src/ejemplos.test.ts`

- [ ] **Step 1: Write the failing tests**

```ts
test('Go por la API propia sube, crea, sigue y lee el resultado', () => { /* ejemplos({ url, modelo, contratos: ['nativa'] }) incluye id 'go-nativa'; su código contiene 'net/http', '/api/v1/cargas', '/api/v1/trabajos', 'Authorization' */ })
test('el ejemplo de herramientas hace el ciclo completo', () => { /* id 'python-herramientas' con contratos ['openai']: contiene 'tools=', 'tool_calls', '"role": "tool"' y la URL /openai/v1 */ })
```

- [ ] **Step 2: Run** `cd web && npm test` — Expected: FAIL.
- [ ] **Step 3: Implement** y compilar (`npm run build`).
- [ ] **Step 4: Run** `cd web && npm test && npm run build`; vista previa: Mis APIs muestra «Go · API propia» y «Python · herramientas», la tabla tiene la fila nueva, la Ayuda enlaza a la guía en es y en.
- [ ] **Step 5: Commit** `feat(web): Go y herramientas en los ejemplos, y enlaces a la guía`.

### Task 9: Prueba con los agentes reales

**Files:**
- Create: `internal/api/herramientas_canario_test.go` (`//go:build canario`)
- Modify: `docs/desarrolladores.md` y `site/desarrolladores.html` (la tabla con el resultado), `docs/equipo/desarrollo.md` (cómo ejecutarla)

**Interfaces:**
- Consumes: el registro real de agentes (`claude.Claude`, `codex.Codex`, `copilot.Copilot`, `gemini`) como en `pruebas/canario`, y `servidorCon`.

- [ ] **Step 1: Write** `TestHerramientasConAgentesReales`: por cada agente, `Detectar` (versión y autenticación); si no está o no tiene sesión/clave → `t.Skip` con el motivo. Si está: el ciclo `clima` completo por `/openai/v1/chat/completions`, `/anthropic/v1/messages` y `/api/v1/trabajos`, con el modelo pequeño de cada agente (`claude/haiku`, `codex` predeterminado, `copilot/auto`, `gemini/gemini-flash-latest`); exige una llamada a `clima` con `ciudad` y una respuesta final que mencione «18».
- [ ] **Step 2: Run** `go test -tags canario -run TestHerramientasConAgentesReales -v -count=1 ./internal/api/` — Expected: PASS o SKIP por agente; anotar cuáles pasaron y con qué modelo.
- [ ] **Step 3: Write** la tabla comprobada en la guía (agente, modelo, OpenAI / Anthropic / propia, fecha).
- [ ] **Step 4: Commit** `test(canario): herramientas con cada agente disponible`.

### Task 10: Cierre

**Files:**
- Modify: `docs/equipo/pendientes.md` (v0.51 ✅ y «Fuera de la v0.51» de la spec), `docs/equipo/README.md` (enlace al plan)

- [ ] **Step 1: Run** todo lo de Global Constraints («Antes de subir») y la prueba de ejemplos — Expected: todo en verde.
- [ ] **Step 2: Commit** `docs(v0.51): hoja de ruta al día`.
- [ ] **Step 3:** revisión final de la rama y PR, como en cada release.
