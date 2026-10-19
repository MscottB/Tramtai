# Respuestas JSON mejoradas (v0.46) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pedir una respuesta con forma sin escribir JSON Schema a mano, entender por qué una respuesta no cumple y
poder corregirla: cuántas veces lo decide el usuario (0 por defecto), y si no hay reintentos, Tramtai lo sugiere.

**Architecture:**
- `internal/salidas` traduce los errores de `santhosh-tekuri/jsonschema` a una lista de problemas legibles.
- El planificador guarda esa lista en un evento `validacion` del trabajo (sin migrar la base) y, si la configuración
  lo permite, pide correcciones al mismo agente añadiendo a la conversación la respuesta inválida y los problemas.
- La Web UI añade plantillas y un editor de campos que generan el esquema; el cuadro JSON de hoy queda para lo
  avanzado.

**Tech Stack:** Go 1.27, Vue 3 + Quasar + TypeScript. Las funciones puras de la web se prueban con `node --test`, que
ejecuta TypeScript sin compilar.

**Spec:** `docs/equipo/releases/v0.46-json/spec.md`.

## Global Constraints

- **Límites:**
  - `reintentos` (solicitud nativa, `salida.reintentos`) y `reintentos_salida` (punto de acceso y receta) van de 0 a 3,
    con 0 por defecto. Fuera de rango: 400 en la API y error de configuración en el punto.
  - Una corrección nunca cambia de agente ni salta al respaldo.
- **Problemas:** `{ "campo": "tareas[2].fecha", "problema": "falta|tipo|valor_no_permitido|sobra|formato|tamaño|otro",
  "texto": "…" }`, con el texto en el idioma del servicio (`idioma.T/Tf`). Como mucho 20 por trabajo.
- **Consumo:** los tokens y el coste de cada corrección se suman al trabajo (cuentan para el presupuesto); la petición
  cuenta una vez.
- **Textos nuevos:** en `es.ts`/`en.ts` y `internal/idioma/en_*.go` (`TestCobertura`).
- **API:**
  - campos nuevos en `docs/api.md` y `internal/api/openapi.json`, cuya prueba compara las rutas registradas con la
    especificación;
  - el modelo de error no cambia (`codigo_error` sigue siendo `salida_no_cumple_esquema`).
- **Comprobaciones:**
  - `go vet ./...` (también con `GOOS=linux` y `GOOS=darwin`), `go test ./...`, `cd escritorio && go vet ./...`;
  - `cd web && npm run build` y `node --test web/src/*.test.ts`;
  - `graphify update .`;
  - pruebas reales: solo Claude haiku y GitHub Copilot `auto` (un crédito).

## Review Focus

1. Una corrección manda la conversación original, la respuesta inválida como turno del asistente y los problemas.
   Nunca cambia de agente, y el contador no permite más de 3 (Tarea 3).
2. `salida_no_cumple_esquema` llega con el primer problema en el mensaje y la lista completa en `errores_salida`, y una
   salida inválida solo se conserva cuando se pidió (Tareas 1, 3 y 4).
3. «Corregir» solo funciona si el trabajo falló por el esquema y conservó su salida; si no, da un error claro (Tarea 3).
4. El editor de campos no destruye un esquema avanzado: si no sabe representarlo, se queda en JSON (Tarea 5).

---

### Tarea 1: Problemas legibles en `internal/salidas`

**Files:** Modify: `internal/salidas/salidas.go` y `salidas_test.go`; `internal/idioma/en_servicio.go`.

- [ ] `type Problema struct { Campo, Problema, Texto string }` y `type ErrorSalida struct { *nucleo.Error; Problemas
  []Problema }`.
  - `Validar` lo devuelve: `nucleo.CodigoDe` y `MensajeDe` siguen funcionando.
  - El mensaje es el primer problema, con «(y N más)» si hay más.
- [ ] Recorrer `ValidationError.Causes` hasta las hojas y traducir cada `ErrorKind`:
  - `kind.Required` → `falta`, uno por campo («Falta el campo `x`»);
  - `kind.Type` → `tipo` («`x` debería ser un número y llegó un texto»);
  - `kind.Enum` y `kind.Const` → `valor_no_permitido` («`x` debe ser una de: a, b»);
  - `kind.AdditionalProperties` → `sobra`;
  - `kind.Format` → `formato`;
  - `MinItems`, `MaxItems`, `MinLength`, `MaxLength`, `Minimum` y `Maximum` → `tamaño`;
  - el resto → `otro`, con el mensaje de la biblioteca.

  La ruta sale de `InstanceLocation`, con los índices entre corchetes (`tareas[2].fecha`); la raíz es «la respuesta».
- [ ] Si la respuesta no es JSON: un único problema `tipo`, «La respuesta no es JSON válido».
- [ ] `CompilarDetallado(esquema) error` para la ruta de validar: en un JSON roto, la línea y la columna del error
  (`json.SyntaxError.Offset`).
- [ ] Pruebas: una tabla con un esquema y una salida por cada tipo de problema (ruta y texto), el tope de 20 y el
  mensaje con «y N más».
- [ ] Commit: `feat(salidas): problemas legibles cuando la respuesta no cumple el esquema`.

### Tarea 2: `reintentos` en la solicitud y en el punto de acceso

**Files:**
- Modify:
  - `internal/nucleo/solicitud.go` y su prueba;
  - `internal/servicio/puntos.go`, `internal/servicio/web.go` y `internal/api/punto.go`;
  - sus pruebas.

- [ ] `ConfigSalida.Reintentos int json:"reintentos,omitempty"`. La validación admite de 0 a 3 y exige formato json o
  esquema si es mayor que 0.
- [ ] El punto de acceso:
  - `ConfigPunto.ReintentosSalida` (`reintentos_salida`, validado de 0 a 3) y `EstadoPunto.ReintentosSalida`;
  - el PATCH de la Web UI lo cambia (ausente no cambia);
  - `api.Punto.ReintentosSalida` se aplica si la solicitud no trae uno (como el esquema).
- [ ] Pruebas: validación, aplicación desde el punto (también por las rutas de OpenAI y Anthropic) y el PATCH.
- [ ] Commit: `feat: reintentos de la salida configurables por petición y por API (0 por defecto)`.

### Tarea 3: El planificador corrige y guarda los problemas

**Files:**
- Modify: `internal/planificador/planificador.go` y `planificador_test.go`, `internal/persistencia` (solo si
  `AgregarEvento` necesita un tipo nuevo en la lista del registro), `internal/idioma/en_servicio.go`.

- [ ] **Al fallar la validación:**
  - evento `validacion` con `{"problemas": […], "correccion": n}`;
  - el tipo se añade a los que devuelve `Registro`.
- [ ] **Corrección:** mientras queden reintentos:
  - nueva `SolicitudEjecucion` con los mensajes originales más `{asistente: salida inválida}` y `{usuario: texto de
    corrección con los problemas}`;
  - `intentar` con el mismo candidato que respondió;
  - evento `intento` con `"correccion": n`;
  - uso sumado;
  - volver a validar.

  El texto de corrección va al modelo: no se traduce y no se muestra al usuario.
- [ ] **Si se agotan:** `Fallido` con los problemas del último intento y la salida conservada solo si
  `ConservarSalidaInvalida`.
- [ ] **Corrección manual:** `Reintentar(ctx, id, corregir bool)`. Con `corregir`:
  - exige que el anterior fallara por `salida_no_cumple_esquema` y conservara su salida;
  - si no, `CodigoTrabajoNoTerminado` o un código nuevo `correccion_no_disponible` (400) con un mensaje claro;
  - la solicitud nueva es la anterior más los dos turnos de corrección, con los problemas de su evento `validacion`, y
    `Reintentos` a 0.
- [ ] **Pruebas:** con un adaptador simulado que responde mal y luego bien:
  - corrige y completa;
  - con 0 no corrige;
  - con 2, falla tras 3 intentos;
  - la conversación de la corrección lleva la respuesta y los problemas;
  - no salta al respaldo;
  - uso sumado;
  - evento `validacion`;
  - reintentar con `corregir` (válido, sin salida conservada y con otro error).
- [ ] Commit: `feat(planificador): corrección de la salida con los problemas, configurable y manual`.

### Tarea 4: API

**Files:**
- Modify:
  - `internal/api/nativa.go` (y `api.go` para el código nuevo);
  - `internal/api/openapi.json`;
  - `docs/api.md`;
  - las pruebas de `internal/api`.

- [ ] `POST /api/v1/esquemas/validar` con `{"esquema": …}`:
  - responde `{"valido": true}` o `{"valido": false, "error": "…", "linea": n, "columna": n}`;
  - el tope es el de los esquemas (`MaximoEsquemaPeticion`);
  - exige token, como el resto, y no ejecuta nada.
- [ ] `GET /api/v1/trabajos/{id}`: `errores_salida`, del último evento `validacion`, cuando el código es
  `salida_no_cumple_esquema`.
- [ ] `POST /api/v1/trabajos/{id}/reintentar` acepta `{"corregir": true}` (el cuerpo es opcional, como hoy).
- [ ] `openapi.json` y `docs/api.md` al día; las pruebas de la API cubren las tres rutas.
- [ ] Commit: `feat(api): validar esquemas, errores_salida y corregir al reintentar`.

### Tarea 5: Plantillas y editor de campos (funciones puras)

**Files:**
- Create:
  - `web/src/plantillas.ts`;
  - `web/src/esquemaCampos.ts`;
  - `web/src/esquemaCampos.test.ts`.

- [ ] **`esquemaCampos.ts`:**
  - `type Campo = { nombre, tipo: 'texto'|'numero'|'entero'|'si_no'|'fecha'|'opcion'|'lista'|'objeto', obligatorio,
    descripcion, opciones?: string[], elemento?: 'texto'|'numero'|'objeto', campos?: Campo[] }`;
  - `aEsquema(campos)`: `type: object`, `properties`, `required` y `additionalProperties: false`; fecha es
    `string` + `format: date`; opción es `enum`;
  - `desdeEsquema(esquema): Campo[] | null`: `null` si usa algo que el editor no representa (`oneOf`, `$ref`,
    `pattern`, más de 3 niveles…);
  - `ejemploDe(campos)`: la vista previa.
- [ ] **`plantillas.ts`:** las siete plantillas de la spec, cada una con:
  - `id`;
  - nombre y descripción con claves de `i18n`;
  - `campos(idioma)`, con los nombres de campo en el idioma;
  - `instrucciones(idioma)`.
- [ ] **Pruebas (`node --test`):**
  - de los campos al esquema y de vuelta, idéntico;
  - un esquema avanzado da `null`;
  - cada plantilla genera un esquema con `type: object` y sus obligatorios;
  - el ejemplo cumple el esquema (comprobación simple de tipos).
- [ ] Commit: `feat(web): plantillas y editor de campos para el esquema (funciones)`.

### Tarea 6: La Web UI

**Files:**
- Create: `web/src/componentes/CamposEsquema.vue`.
- Modify:
  - `web/src/componentes/EditorReceta.vue`;
  - `web/src/api.ts` (`Receta.reintentos`, `recetaDePunto`, `solicitudDeReceta`, el PATCH del punto,
    `api.validarEsquema` y `api.reintentar(id, {corregir})`);
  - `web/src/paginas/Ejecutar.vue` y `web/src/paginas/Accesos.vue`;
  - `web/src/idiomas/es.ts` y `en.ts`.

- [ ] **`CamposEsquema.vue`:**
  - filas con nombre, tipo, obligatorio y descripción;
  - opciones para «una de una lista»;
  - subcampos para objeto y lista de objetos (hasta 3 niveles);
  - añadir, quitar, subir y bajar;
  - accesible con teclado, con etiquetas.
- [ ] **`EditorReceta`, en «Respuesta con forma»:**
  - el botón «Usar una plantilla» abre un menú con las siete y su descripción, y pide confirmar si ya hay esquema;
  - las pestañas **Campos** y **JSON**. Con un esquema que `desdeEsquema` no representa, Campos se desactiva con el
    aviso;
  - la validación del JSON con `api.validarEsquema` (tras una pausa al escribir), con la línea del error;
  - la vista previa de un ejemplo;
  - «Reintentos si no cumple»: 0, 1, 2 o 3, con el aviso «cada corrección es otra petición al agente».
- [ ] **Probar:**
  - envía `conservar_salida_invalida: true`;
  - con `salida_no_cumple_esquema`, muestra la lista de problemas, la respuesta tal como llegó (con los campos con
    problemas resaltados) y el botón «Corregir» (si quedan 0 reintentos), que llama a reintentar con `corregir`;
  - en el registro, las correcciones como intentos («corrección 1 de 2»).
- [ ] **Mis APIs:** la receta guarda `reintentos_salida` en el punto (PATCH) y lo muestra en el detalle.
- [ ] `npm run build`; confirmar los recursos compilados. Commit: `feat(web): plantillas, editor de campos, errores
  explicados y corregir en Probar y Mis APIs`.

### Tarea 7: Ayuda, documentación y web

**Files:**
- Modify:
  - `web/src/idiomas/es.ts`/`en.ts` (Ayuda);
  - `README.md`;
  - `site/index.html`, `site/ayuda.html`;
  - `docs/inicio.md`, `docs/api.md`;
  - `docs/equipo/pendientes.md`, `nuevo-agente.md` y `opencode.md` (en la rama `feat/opencode` se renumera al
    retomarla).

- [ ] **Ayuda:**
  - la guía «Respuestas con forma» (plantilla, campos, JSON, qué pasa si no cumple, reintentos y «Corregir»);
  - el error `salida_no_cumple_esquema` con los pasos nuevos.
- [ ] **README (es/en) y web:** la tarjeta «Respuestas con forma» con plantillas, campos y corrección. Si cambia lo que
  muestra Probar, la captura de Probar.
- [ ] **`pendientes.md`:** la fila v0.46; OpenCode pasa a v0.47 y Hermes a v0.48.
- [ ] Commit: `docs: respuestas con forma (plantillas, campos y corrección)`.

### Tarea 8: Verificación

- [ ] **Comprobaciones:** todas las de Global Constraints.
- [ ] **Real:**
  - con Claude haiku, un esquema exigente (por ejemplo `puntos` con exactamente 5 elementos y `categoria` en una lista
    corta) y `reintentos: 1`: ver una corrección en el registro, o comprobar «Corregir» a mano;
  - lo mismo una vez con GitHub Copilot `auto`.
- [ ] **Navegador**, en Probar y Mis APIs (escritorio y móvil, claro y oscuro, es y en):
  - plantillas;
  - campos ⇄ JSON;
  - esquema avanzado;
  - error explicado;
  - «Corregir»;
  - reintentos guardados en la API.
- [ ] `graphify update .`. Después, el flujo del release:
  - revisión de toda la rama (`ecc:code-reviewer`, opus) y todos los arreglos;
  - fusión;
  - notas cortas con `-Resumen` y `-ResumenEn`;
  - etiqueta, CI y Publicar, y paquetes.
