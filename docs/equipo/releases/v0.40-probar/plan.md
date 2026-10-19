# Probar (v0.40) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Esfuerzo a la vista y por agente de respaldo, registro con tiempos por etapa, pruebas recientes y respuesta
en Markdown en Probar, y una ventana de progreso al instalar una actualización.

**Architecture:** El esfuerzo del respaldo viaja en la propia entrada (`agente/modelo+nivel`), así no cambia ningún
tipo de la API. El planificador deja un evento `intento` por intento y una ruta JSON nueva devuelve los eventos sin
texto con su hora; la Web UI arma con eso el registro. Dos componentes nuevos (`SalidaTrabajo`, `RegistroTrabajo`) se
comparten entre Probar y el Historial. El instalador deja de ser silencioso cuando lo lanza una actualización.

**Tech Stack:** Go 1.27, SQLite (modernc), Vue 3 + Quasar, `marked` + `dompurify` (nuevas), NSIS.

**Spec:** `docs/equipo/releases/v0.40-probar/spec.md`

## Global Constraints

- Niveles de esfuerzo: `bajo, medio, alto, muy_alto, maximo` (`nucleo.NivelesEsfuerzo`); sufijo `+nivel`.
- Sin sufijo, el respaldo hereda el esfuerzo del principal (comportamiento actual, no cambia).
- `/registro` nunca devuelve eventos `texto`; mismos permisos que `GET /api/v1/trabajos/{id}`.
- La purga conserva `estado` e `intento` con los metadatos.
- Pruebas recientes: 10, origen `probar`.
- Markdown: sin imágenes externas (su texto alternativo), enlaces con `target="_blank" rel="noopener noreferrer"`,
  HTML pasado por DOMPurify; JSON como hoy; «Copiar» copia el texto original; preferencia en `localStorage` con try/catch.
- Dependencias con versión exacta en `package.json` (`npm install --save-exact`).
- Textos nuevos en `es.ts` y `en.ts`; mensajes nuevos del servidor con su traducción en `internal/idioma/en_*.go`.
- Comprobaciones: `go vet ./...`, `go test ./...`, `cd escritorio && go vet ./...`, `cd web && npm run build`,
  `graphify update .`.

## Review Focus

1. Reintentar tras un salto a un respaldo con esfuerzo propio debe usar ese esfuerzo, no el del principal → prueba en Tarea 1.
2. Una API con `modelos_permitidos: [otro]` acepta el respaldo `otro+alto` → prueba en Tarea 1.
3. Un trabajo cuyo resultado venció conserva sus estados e intentos para el registro → prueba de purga en Tarea 2.
4. Markdown con `<script>`, `onerror=` o `javascript:` no ejecuta nada → comprobación de navegador en Tarea 4.
5. «Usar de nuevo» de una prueba con las entradas borradas no deja la receta a medias: el botón queda desactivado → Tarea 6.

---

### Tarea 1: Respaldo con esfuerzo propio

**Files:**
- Modify: `internal/nucleo/solicitud.go` (`ValidarRespaldo`, `DividirRespaldo`)
- Modify: `internal/planificador/planificador.go` (`candidato`, `candidatos`, `intentar`, bucle de respaldo, `aceptar`)
- Modify: `internal/persistencia/almacen.go` (`CambiarAgente`)
- Modify: `internal/api/punto.go:97` (`PermitidoRespaldo`), `internal/servicio/puntos.go:167`
- Modify: `internal/idioma/en_servicio.go`
- Test: `internal/nucleo/solicitud_test.go`, `internal/planificador/planificador_test.go`, `internal/api/api_test.go`

**Interfaces:**
- Produces: `nucleo.DividirRespaldo(entrada string) (agente, modelo, esfuerzo string)`;
  `candidato{agente, modelo, esfuerzo string}` con `String()` = `agente[/modelo][+esfuerzo]`;
  `(*Almacen).CambiarAgente(ctx, id, agente, modeloEnviado, esfuerzo string, restante []string) error`.

- [ ] **Step 1: Pruebas que fallan**
  - `TestValidarRespaldo`: válidos `{"codex+alto"}`, `{"codex/gpt-5+maximo", "claude"}`; rechazados `{"codex+turbo"}`,
    `{"codex+"}`, `{"codex", "codex+alto"}` (repetida por esfuerzo). `DividirRespaldo("codex/gpt-5+alto")` →
    `"codex", "gpt-5", "alto"`; `DividirRespaldo("codex")` → `"codex", "", ""`.
  - `TestRespaldoConEsfuerzoPropio` (planificador): principal `simulado` falla por límite con `Esfuerzo: "bajo"`;
    respaldo `otro+alto` con un adaptador que guarda `s.Esfuerzo` en un canal → recibe `"alto"`; el trabajo completado
    tiene `Solicitud.Esfuerzo == "alto"` y `Solicitud.Respaldo` vacío. Con respaldo `otro` (sin sufijo) recibe `"bajo"`.
  - `TestRespaldoConEsfuerzoReintento`: tras el salto anterior, `Reintentar` ejecuta `otro` con `"alto"`.
  - `TestPuntoRespaldoNoPermitido` ampliado: `ModelosPermitidos: ["otro"]`, `Respaldo: ["otro+alto"]` se acepta.
- [ ] **Step 2:** `go test ./internal/nucleo ./internal/planificador ./internal/api` → FAIL (no compila: 2 valores).
- [ ] **Step 3: Implementar**
  - `DividirRespaldo`: primero `strings.Cut(entrada, "+")`, después agente/modelo.
  - `ValidarRespaldo`: el sufijo, si está, debe estar en `NivelesEsfuerzo` (mensaje nuevo
    `respaldo %q: esfuerzo desconocido %q`, con traducción); la comprobación de repetidas y de formato usa la base sin
    sufijo.
  - `candidatos`: el duplicado compara agente y modelo, nunca el esfuerzo. `intentar`: si `c.esfuerzo != ""`,
    `solicitud.Esfuerzo = c.esfuerzo`.
  - `CambiarAgente`: con `esfuerzo != ""` añade `'$.esfuerzo', ?` al `json_set`; `restante` usa `c.String()` (conserva el sufijo).
  - Los demás llamadores de `DividirRespaldo` ignoran el tercer valor.
- [ ] **Step 4:** `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat: esfuerzo propio para cada agente de respaldo`.

### Tarea 2: Evento `intento`, purga y ruta `/registro`

**Files:**
- Modify: `internal/planificador/planificador.go` (`intentar`)
- Modify: `internal/persistencia/almacen.go:660` (purga del resultado)
- Modify: `internal/api/api.go` (ruta), `internal/api/nativa.go` (handler)
- Test: `internal/planificador/planificador_test.go`, `internal/persistencia/almacen_test.go`, `internal/api/api_test.go`

**Interfaces:**
- Consumes: `candidato` de la Tarea 1; `adaptadores.NivelesEsfuerzo`, `adaptadores.EsfuerzoPredeterminado`.
- Produces: evento `intento` con datos JSON `{"agente","modelo","esfuerzo","del_agente"}` (`del_agente` omitido si
  false); `GET /api/v1/trabajos/{id}/registro` → `{"eventos":[{"numero","tipo","datos","creado_en"}]}`, `creado_en`
  en RFC 3339 con nanosegundos, UTC.

- [ ] **Step 1: Pruebas que fallan**
  - `TestEventoIntento`: un adaptador falso registrado como `codex` y solicitud con `Esfuerzo: "maximo"` → un evento
    `intento` con `agente: "codex"` y `esfuerzo` = último de `adaptadores.NivelesEsfuerzo("codex")`; con un salto de
    respaldo hay dos eventos `intento`, en orden. Con un agente sin niveles, `esfuerzo` vacío.
  - `TestIntentoDelAgente`: adaptador como `claude`, modelo `claude-opus-5-5`, sin esfuerzo → `esfuerzo: "medio"`,
    `del_agente: true`.
  - `TestPurgaConservaEstadoEIntento` (persistencia): tras vencer el resultado quedan `estado` e `intento`; `texto` y
    `aviso` se borran.
  - `TestRegistroSinTexto` (api, adaptador `trozos`): `/registro` devuelve `estado` e `intento`, ningún `texto`, con
    `creado_en` no decreciente; desde un servidor con `Punto` ajeno → 404.
- [ ] **Step 2:** `go test` de los tres paquetes → FAIL.
- [ ] **Step 3: Implementar**
  - En `intentar`, justo después de pasar a `Ejecutando`, `AgregarEvento(fondo, id, "intento", json)`. El nivel:
    pedido (`solicitud.Esfuerzo` tras aplicar `c.esfuerzo`) ajustado al último de `NivelesEsfuerzo(c.agente)`; sin
    pedido, `EsfuerzoPredeterminado(c.agente, c.modelo)` con `del_agente: true` si no es vacío; agente sin niveles → `""`.
  - Purga: `tipo <> 'estado'` → `tipo NOT IN ('estado', 'intento')`.
  - Handler `registroTrabajo`: `s.trabajo(r, id)` para el permiso, `s.Planificador.Eventos(ctx, id, 0)`, filtra `texto`.
- [ ] **Step 4:** `go test ./...` → PASS.
- [ ] **Step 5: Commit** `feat: evento intento y registro de cada petición con su hora`.

### Tarea 3: Web UI — esfuerzo junto al modelo y por agente de respaldo

**Files:**
- Modify: `web/src/componentes/EditorReceta.vue`, `web/src/componentes/SelectorEsfuerzo.vue`
- Modify: `web/src/api.ts`, `web/src/idiomas/es.ts`, `web/src/idiomas/en.ts`

**Interfaces:**
- Produces: `dividirRespaldo(e: string): { base: string; esfuerzo: string }` y
  `unirRespaldo(base: string, esfuerzo: string): string` en `api.ts`; prop `vacio?: string` en `SelectorEsfuerzo`
  (texto de la opción `''`, si se da).

- [ ] **Step 1: Implementar**
  - Modelo y esfuerzo en una fila (`col-12 col-sm-8` / `col-12 col-sm-4`), fuera del plegable; el plegable conserva
    forzar, respaldo y salida estructurada (`conOpciones` deja de mirar el esfuerzo).
  - El `q-select` de respaldo trabaja con las bases; al cambiar conserva el esfuerzo de las que siguen.
  - Debajo, por cada entrada con `esfuerzosDe(catalogo, base).length > 0`, un `SelectorEsfuerzo` con etiqueta = la base y
    `vacio` = `receta.respaldoIgual` («Igual que el principal ({nivel})») si la receta tiene esfuerzo, si no
    `esfuerzo.agenteCon` con `esfuerzoPredeterminadoDe(catalogo, base)` (o `esfuerzo.agente`). Sin niveles:
    `receta.respaldoSinEsfuerzo` («{agente}: no admite esfuerzo»).
- [ ] **Step 2:** `cd web && npm run build` → sin errores.
- [ ] **Step 3: Navegador** (servidor de prueba 17999, binario recompilado): en Probar el esfuerzo se ve junto al modelo
  con «Del agente (…)»; con respaldo `codex` en `alto` y `claude` sin cambiar, «Guardar como API» deja en Mis APIs →
  ajustes el respaldo `codex+alto, claude`; móvil 375 px.
- [ ] **Step 4: Commit** `feat(web): esfuerzo junto al modelo y para cada agente de respaldo`.

### Tarea 4: Web UI — `SalidaTrabajo` con Markdown

**Files:**
- Create: `web/src/componentes/SalidaTrabajo.vue`
- Modify: `web/package.json` (`marked`, `dompurify`), `web/src/paginas/Ejecutar.vue`, `web/src/paginas/Trabajos.vue:90`,
  `web/src/estilos.css`, `es.ts`, `en.ts`

**Interfaces:**
- Produces: `<SalidaTrabajo :texto="string" />`: JSON (`esJSON`) en `pre` como hoy; si no, conmutador
  «Markdown | Texto» (`q-btn-toggle`, clave `localStorage` `tramtai.salida`) y el HTML limpio en `div.markdown`.

- [ ] **Step 1:** `npm install --save-exact marked@15 dompurify@3` en `web/`.
- [ ] **Step 2: Implementar** `marked.parse(texto, { gfm: true, breaks: true, async: false })` con un renderer cuyo
  `image` devuelve el texto alternativo escapado; `DOMPurify.sanitize` con un hook `afterSanitizeAttributes` que da a cada
  `a` `target="_blank" rel="noopener noreferrer"`. Estilos `.markdown` (títulos, listas, tablas con borde, `code` y `pre`
  con el fondo de `.bloque-codigo`, `max-width: 100%` y desplazamiento horizontal en `pre`). Sustituye el `pre.salida`
  de Probar (conserva `aria-live="polite"` en el contenedor) y el `div.bloque-texto` del Historial.
- [ ] **Step 3:** `npm run build` → sin errores.
- [ ] **Step 4: Navegador:** una respuesta con `# Título`, `**negrita**`, una tabla, un bloque de código,
  `![foto](https://ejemplo.com/x.png)`, `<img src=x onerror=alert(1)>`, `<script>alert(1)</script>` y
  `[enlace](javascript:alert(1))` → título, negrita, tabla y código con formato; ni `img` ni `script` en el DOM; el
  enlace `javascript:` sin `href`; «Texto» muestra el original y la elección sobrevive a recargar; JSON igual que antes.
  (Con el adaptador simulado del servidor de prueba, o la `salida` de un trabajo fijada a mano en la base de prueba.)
- [ ] **Step 5: Commit** `feat(web): respuesta en Markdown con opción de texto`.

### Tarea 5: Web UI — `RegistroTrabajo` y esfuerzo en los datos

**Files:**
- Create: `web/src/componentes/RegistroTrabajo.vue`
- Modify: `web/src/api.ts` (`registro`, tipo `EventoRegistro`, `esfuerzoUsado`), `Ejecutar.vue`, `Trabajos.vue`, `es.ts`, `en.ts`

**Interfaces:**
- Consumes: `/registro` de la Tarea 2.
- Produces: `api.registro(id: string): Promise<EventoRegistro[]>`, `interface EventoRegistro { numero: number; tipo:
  string; datos: string; creado_en: string }`; `<RegistroTrabajo :trabajo="Trabajo" :version="number" />` que vuelve a
  pedir el registro cuando cambia `version`; `esfuerzoUsado(t: Trabajo, eventos: EventoRegistro[]): string`.

- [ ] **Step 1: Implementar**
  - Bloque plegable «Registro»: por evento, hora local `HH:MM:SS.mmm`, desplazamiento desde `trabajo.creado_en`
    («+15 ms», «+1,2 s»), y el texto: estado → `estadoInfo(datos).etiqueta`; un `estado ejecutando` seguido de su
    `intento` en una sola línea «Ejecutando · agente/modelo · nivel»; `aviso` → «Aviso: …».
  - Duraciones por etapa sumando los tramos entre estados, agrupados como en `Ruta.vue`: cola (`en_cola`), preparación
    (`preparando_entrada`, `esperando_agente`), ejecución (`ejecutando`, `cancelacion_solicitada`), validación
    (`validando_salida`).
  - Pie: tokens de entrada → salida, costo si lo hay y `modelo_efectivo`.
  - `esfuerzoUsado`: `solicitud.esfuerzo` → `nombreEsfuerzo`; si no, el último `intento` con `del_agente` →
    `esfuerzo.delAgente` («{nivel} (del agente)»); si no, `''`. Lo usan los datos de Probar y del Historial.
  - Probar: `version` sube con cada evento SSE `estado`, `aviso` e `intento` (nuevo `addEventListener('intento')`) y en `fin`.
- [ ] **Step 2:** `npm run build` → sin errores.
- [ ] **Step 3: Navegador:** enviar una prueba en el servidor 17999 → el registro muestra En cola, Esperando agente,
  Ejecutando · …, Completado con horas crecientes y duraciones; el mismo bloque en el detalle del Historial; el esfuerzo
  aparece en los datos.
- [ ] **Step 4: Commit** `feat(web): registro con tiempos por etapa en Probar y el Historial`.

### Tarea 6: Web UI — Pruebas recientes en Probar

**Files:**
- Modify: `web/src/paginas/Ejecutar.vue`, `web/src/api.ts` (`recetaDeSolicitud`), `web/src/paginas/Trabajos.vue`
  (filtro de origen al llegar), `es.ts`, `en.ts`

**Interfaces:**
- Produces: `recetaDeSolicitud(s: Solicitud): Receta` (inversa de `aplicarModelo` + `ajustesDeReceta`: modelo
  `perfil/<perfil_modelo>` o `agente[/modelo]`, esquema con `JSON.stringify(…, null, 2)`, `forzar: false`).

- [ ] **Step 1: Implementar**
  - Tarjeta «Pruebas recientes» bajo la de la respuesta: `api.trabajos(10, { origen: 'probar' })` al montar y en cada `fin`.
  - Fila: hora (`fecha`), estado (`estadoInfo`), `modeloDe(t)` y `textoDe(t)` recortado a una línea; pulsar →
    `seguir(t)`. Botón **Usar de nuevo** → mensaje = último mensaje de usuario, `receta = recetaDeSolicitud(...)`,
    `editando = ''`; con `solicitud.cargas?.length` avisa `ejecutar.sinAdjuntos`. Desactivado (con tooltip
    `trabajo.entradaEliminada`) si `entradas_eliminadas` o sin mensajes.
  - «Ver todo en el Historial»: `origenHistorial.value = 'o:probar'` y `emit('ir', 'historial')`; `Trabajos.vue`
    acepta en `origenHistorial` un valor con prefijo `o:`/`g:`/`p:` tal cual y un nombre suelto como `p:<nombre>`.
  - Estado vacío: «Aún no hay pruebas».
- [ ] **Step 2:** `npm run build` → sin errores.
- [ ] **Step 3: Navegador:** dos pruebas → aparecen arriba en la lista; abrir una muestra su salida y registro; «Usar de
  nuevo» restaura mensaje, modelo, esfuerzo y respaldo; «Ver todo» abre el Historial filtrado por Probar; móvil.
- [ ] **Step 4: Commit** `feat(web): pruebas recientes en Probar`.

### Tarea 7: Ventana de progreso al instalar una actualización

**Files:**
- Modify: `scripts/instalador/tramtai.nsi`

- [ ] **Step 1: Implementar**
  - `Var actualizando`. `Function .onInit`: con `/abrir` en `${GetParameters}` y `${Silent}`, `SetSilent normal` y
    `StrCpy $actualizando 1`.
  - `Function SaltarSiActualiza` (`${If} $actualizando == 1` → `Abort`) como `MUI_PAGE_CUSTOMFUNCTION_PRE` de bienvenida,
    las dos licencias, componentes, carpeta y final.
  - Al inicio de `SecPrograma`: con `$actualizando == 1`, `SetAutoClose true`.
  - `DetenerTramtai`: el `MessageBox` solo si no es silencioso y `$actualizando != 1`.
  - `.onInstSuccess`: abre la app con `-actualizado` si (silencioso o `$actualizando == 1`) y `$abierta == 1`; el resto igual.
- [ ] **Step 2:** `scripts/compilar.ps1 -Instalador` → genera `dist/Tramtai-instalador.exe` sin avisos de NSIS.
- [ ] **Step 3: Prueba manual — pedir permiso antes al usuario:** ejecutar `dist\Tramtai-instalador.exe /S /abrir
  /D=<carpeta temporal>` cierra el Tramtai que esté abierto. Comprobar: se ve solo la página de progreso, se cierra sola,
  la app de la carpeta temporal se abre con el diálogo de novedades. Después, desinstalar esa copia
  (`desinstalar.exe /S`) y volver a abrir la instalación normal del usuario.
- [ ] **Step 4: Commit** `feat(instalador): ventana de progreso al actualizar`.

### Tarea 8: Documentación, capturas y comprobación final

**Files:**
- Modify: `web/src/paginas/Ayuda.vue` (y sus claves en `es.ts`/`en.ts`), `docs/operacion.md`, `docs/api.md`,
  `README.md` y `site/index.html` + `site/ayuda.html` si nombran Probar o la actualización, `docs/equipo/pendientes.md`
- Modify: `site/capturas/*.png` y `docs/imagenes/*.png` que muestren Probar

- [ ] **Step 1:** Ayuda: Probar (esfuerzo junto al modelo, respaldo `+nivel`, registro, pruebas recientes, Markdown) y
  Actualizar (ventana de progreso). `api.md`: sufijo `+nivel` en `respaldo`, evento `intento` en el flujo SSE, ruta
  `/registro` con un ejemplo de respuesta. `operacion.md`: registro y ventana al actualizar. `pendientes.md`: fila
  «Pulido» → v0.39.0 ✅ y fila v0.40.0 Probar.
- [ ] **Step 2:** Capturas de Probar en claro y oscuro con Edge sin cabeza (dos pasadas: la primera cierra las novedades).
- [ ] **Step 3:** `go vet ./...`, `go test ./...`, `cd escritorio && go vet ./...`, `cd web && npm run build`,
  `graphify update .` → todo verde.
- [ ] **Step 4: Commit** `docs: Probar con registro, pruebas recientes y Markdown; ventana al actualizar`.

### Tarea 9: Acerca de Tramtai

**Files:**
- Modify: `internal/servicio/web.go` (ruta `/ui/acerca`), `internal/version/version.go` (si hace falta la versión de Go)
- Create: `web/src/componentes/AcercaDe.vue`
- Modify: `web/src/App.vue` (botón de versión, menú del estado, menú lateral, `window.tramtaiAcercaDe`), `web/src/api.ts`,
  `es.ts`, `en.ts`, `escritorio/main.go` (línea de versión de la bandeja)
- Test: `internal/servicio/web_test.go`

**Interfaces:**
- Produces: `GET /ui/acerca` → `{"version","commit","modificado","go","sistema","arquitectura","servicio":{"direccion",
  "pid","iniciado_en"},"almacen","rutas"?:{"datos","configuracion","registro"}}`; `api.acerca(): Promise<Acerca>`;
  `window.tramtaiAcercaDe()` abre el diálogo.

- [ ] **Step 1: Prueba que falla** `TestAcercaDe`: sin sesión → 401; sesión local → 200 con `version` = `version.Texto()`,
  `servicio.pid` = `os.Getpid()` y `rutas.datos` = la carpeta de datos del servicio; sesión no local (token de instancia)
  → 200 sin `rutas`.
- [ ] **Step 2:** `go test ./internal/servicio -run TestAcercaDe` → FAIL (404).
- [ ] **Step 3: Implementar** la ruta junto a las de `/ui/actualizacion` (mismo control de sesión), con `version.Commit()`,
  `runtime.Version()`, `runtime.GOOS`/`GOARCH`, `s.Base()`, `s.IniciadoEn`, `credenciales.Almacen()`, `nucleo.RutaGlobal()`
  y `filepath.Join(s.opc.Datos, "servicio.log")`.
- [ ] **Step 4: Web UI** según la spec, sección 7. Fecha de compilación: `+DDMMAAAA` → `toLocaleDateString(idioma, {
  dateStyle: 'long' })`; sin sufijo (versión `dev`) no se muestra. «Ver novedades»:
  `https://github.com/MscottB/Tramtai/releases/tag/` + `encodeURIComponent(version)`. Bandeja: la línea de versión
  llama a `mostrar()` y `ventana.ExecJS("window.tramtaiAcercaDe && window.tramtaiAcercaDe()")`.
- [ ] **Step 5:** `go test ./...`, `cd escritorio && go vet ./...`, `npm run build` → todo verde; navegador: abrirlo desde
  la cabecera, el menú del estado y el menú lateral; Copiar datos; móvil y oscuro.
- [ ] **Step 6: Commit** `feat: Acerca de Tramtai`.

### Tarea 10: el instalador dice Instalar o Actualizar

**Files:**
- Modify: `scripts/instalador/tramtai.nsi`

- [ ] **Step 1: Implementar** en `.onInit`: `ReadRegStr` de `InstallLocation` en `${DESINSTALAR}` (HKCU); si no está
  vacío, `StrCpy $hayInstalacion 1`. Textos con `LangString` (es/en): título de la ventana con `SendMessage $HWNDPARENT
  ${WM_SETTEXT}` en las funciones `SHOW` (bienvenida y progreso), cabecera de la página de progreso con
  `!insertmacro MUI_HEADER_TEXT`, y bienvenida con su título y texto de actualizar.
- [ ] **Step 2:** `scripts/compilar.ps1 -Instalador` sin avisos.
- [ ] **Step 3: Prueba manual (con el permiso ya dado por el usuario; mismo respaldo y restauración que en la Tarea 7):**
  con la instalación del usuario registrada, `/S /abrir /D=<temporal>` debe titular «Actualizar Tramtai». No se repite
  la prueba sin instalación previa: borrar la clave del registro del usuario no compensa; la rama «Instalar» es la de
  hoy.
- [ ] **Step 4: Commit** `feat(instalador): dice Actualizar si ya está instalado`.

### Tarea 11: Documentación de Acerca de

- [ ] Ayuda (pregunta «¿Qué versión tengo?»), `docs/operacion.md` (Acerca de y la ventana de actualizar), README si lo
  menciona; `docs/equipo/pendientes.md`: fila «Seguridad y límites» después de Probar. Commit
  `docs: Acerca de y el instalador al actualizar`.
