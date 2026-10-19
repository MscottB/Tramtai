# Seguridad y límites (v0.41) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que una entrada que no cabe en el modelo se rechace (o salte al respaldo) antes de llamar al CLI, que
Antigravity no se use desde apps hasta validarlo, y cerrar los riesgos de la auditoría que dependen de Tramtai.

**Architecture:** La conversión devuelve el texto con el tamaño de cada documento; el planificador mide la entrada y
la compara con el contexto del candidato (configurado, informado por el CLI y guardado en `modelos_vistos`, o
desconocido) antes de cada intento. El endurecimiento va en la capa que hoy toca cada cosa: `nucleo` (validación y
entorno), `adaptadores` (cómo se lanza cada CLI), `documentos` (cargas y conversión), `procesos` (topes del proceso
hijo) y `api` (puntos de acceso y fachadas).

**Tech Stack:** Go 1.27, SQLite (modernc), Windows Job Objects (`golang.org/x/sys/windows`), Vue 3 + Quasar.

**Spec:** `docs/equipo/releases/v0.41-seguridad/spec.md`

## Global Constraints

- Estimación de tokens = ⌈caracteres ÷ 4⌉ (runas del texto final que recibe el agente).
- Cabe si `tokens_estimados + limites.reserva_salida_tokens (8192) <= contexto`; contexto desconocido → no se comprueba.
- Orden del contexto: `limites.contexto_tokens["agente/modelo"]`, luego `["agente"]`, luego el visto (tabla
  `modelos_vistos`, alias resuelto con la versión vista), luego desconocido.
- Código nuevo `entrada_demasiado_grande` (saltable: pasa al respaldo); fachadas: OpenAI 400 `context_length_exceeded`,
  Anthropic 400 `invalid_request_error`, `/api/v1/ejecutar` 413. No cuenta para presupuesto ni cuota.
- Antigravity: rechazado para orígenes `app` y `api` con `agente_no_disponible` salvo `adaptadores.antigravity.permitir_apps`
  (falso, administrada). Tope de línea de órdenes: 28.000 unidades UTF-16 tras el escapado de Windows.
- Nuevos valores: `limites.memoria_proceso_mb` 2048, `conversion.tiempo_total_segundos` 600, `conversion.cuota_cargas_mb`
  2048, esquema de petición ≤ 256 KB, mensajes ≤ 1.000. Un límite de `conversion.*` ≤ 0 usa su predeterminado.
- Canarios: solo `claude/haiku`, `claude/sonnet` y el modelo más pequeño de Codex con esfuerzo `bajo`.
- Textos nuevos en es/en; mensajes nuevos del servidor con traducción en `internal/idioma/en_*.go`.
- Comprobaciones: `go vet ./...`, `go test ./...`, `cd escritorio && go vet ./...`, `cd web && npm run build`,
  `graphify update .`.

## Review Focus

1. Una entrada justo en el borde (estimación ≈ contexto − reserva) no se rechaza por un redondeo → prueba en Tarea 3.
2. Un salto por tamaño no gasta presupuesto ni deja `usoPrevio` del candidato saltado → prueba en Tarea 3.
3. Una API `aislado` con un perfil que trae `consciente_proyecto` se rechaza, y una API `consciente_proyecto` sigue
   funcionando → prueba en Tarea 5.
4. Dos subidas a la vez cerca de la cuota no la superan juntas → prueba en Tarea 7.
5. Un mensaje de Antigravity que empieza por `-` con instrucciones vacías llega enmarcado → prueba en Tarea 4.

---

### Tarea 1: Documentos con su tamaño y evento `entrada`

**Files:**
- Modify: `internal/nucleo/solicitud.go` (tipos), `internal/documentos/conversor.go`, `internal/documentos/preparador.go`,
  `internal/planificador/planificador.go` (`Preparador`, `preparar`, `procesar`), `internal/persistencia/almacen.go`
  (purga y `EventosDeRegistro` conservan `entrada`)
- Test: `internal/documentos/documentos_test.go`, `internal/planificador/planificador_test.go`

**Interfaces:**
- Produces: `nucleo.Documentos{Texto string; Partes []nucleo.ParteDocumento}`, `nucleo.ParteDocumento{Nombre string;
  Caracteres int}`; `Conversor.Convertir(ctx, []Entrada) (nucleo.Documentos, error)`;
  `planificador.Preparador.Preparar(ctx, []string) (nucleo.Documentos, error)`;
  `medirEntrada(s nucleo.SolicitudEjecucion, docs nucleo.Documentos) medida` con `medida{Caracteres, Tokens int;
  Documentos []parteMedida}`; evento `entrada` JSON `{"caracteres","tokens_estimados","documentos":[{"nombre",
  "tokens_estimados"}]}` ordenado de mayor a menor.

- [ ] Prueba: `TestConvertirDevuelvePartes` (dos archivos de texto → dos partes con sus caracteres; los de un ZIP
  aparecen como `archivo.zip/interno.txt`). `TestEventoEntrada` (planificador con `preparadorFalso` que devuelve dos
  partes → un evento `entrada` antes de `esperando_agente`, partes ordenadas, `tokens_estimados` = ⌈runas/4⌉ del texto
  completo). Sin cargas también hay evento `entrada` con `documentos` vacío.
- [ ] `go test ./internal/documentos ./internal/planificador` → FAIL (no compila).
- [ ] Implementar; `preparadorFalso` y `conDocumentos` usan `Documentos.Texto`. Las etiquetas del registro y la purga
  tratan `entrada` como `intento` (sin contenido).
- [ ] `go test ./...` → PASS. Commit `feat: medida de la entrada y evento entrada`.

### Tarea 2: Contexto de cada modelo

**Files:**
- Modify: `internal/nucleo/configuracion.go` (claves `limites.*`), `internal/persistencia/almacen.go` (migración v9,
  `GuardarContexto`, `Contexto`), `internal/adaptadores/adaptador.go` (`Resultado.ContextoTokens int`, interfaz
  `Contextual`), `internal/adaptadores/claude.go` (`contextWindow`), `internal/adaptadores/gemini.go`
  (`inputTokenLimit`), `internal/planificador/planificador.go` (`contextoDe`)
- Test: los `_test.go` de esos paquetes

**Interfaces:**
- Produces: `(*Almacen).GuardarContexto(ctx, agente, modelo string, tokens int) error`,
  `(*Almacen).Contexto(ctx, agente, modelo string) (int, error)` (0 si no hay);
  `adaptadores.Contextual{ Contexto(ctx, modelo string) (int, error) }` (Gemini, con caché de 24 h);
  `(*Planificador).contextoDe(ctx, c candidato) (tokens int, origen string)` con origen `configurado`, `informado` o `""`.

- [ ] Pruebas: `TestContextoClaude` (un resultado con `modelUsage.{m}.contextWindow: 200000` → `Resultado.ContextoTokens`
  200000 del modelo efectivo); `TestContextoGemini` (servidor de prueba de `models.get` con `inputTokenLimit` → 1048576,
  una sola llamada para dos consultas); `TestGuardarContexto` (migración v9 en una base v8 existente; guarda y lee; el
  último gana); `TestContextoDe` (configurado `agente/modelo` gana a `agente`, que gana al visto; alias resuelto con
  `versionVista`; nada → 0).
- [ ] FAIL; implementar. Tras cada intento con `ContextoTokens > 0`, el planificador lo guarda para el modelo efectivo.
- [ ] `go test ./...` → PASS. Commit `feat: contexto de cada modelo, configurado o informado`.

### Tarea 3: Comprobar antes de ejecutar y saltar si no cabe

**Files:**
- Modify: `internal/nucleo/errores.go` (`CodigoEntradaDemasiadoGrande`), `internal/planificador/planificador.go`
  (bucle de candidatos, `saltable`), `internal/api/openai.go`, `internal/api/anthropic.go`, `internal/api/nativa.go`
  (estado HTTP), `internal/idioma/en_*.go`
- Test: `internal/planificador/planificador_test.go`, `internal/api/api_test.go`

**Interfaces:**
- Consumes: `medida` (Tarea 1), `contextoDe` (Tarea 2).
- Produces: `cabe(m medida, contexto, reserva int) bool`; aviso «La entrada (~N tokens; X ~M) no cabe en A (contexto C);
  sigue con B, que sí la admite».

- [ ] Pruebas: `TestEntradaNoCabeSinRespaldo` (contexto configurado 1.000, entrada 40.000 caracteres → Fallido
  `entrada_demasiado_grande`, el adaptador no se llamó, uso 0); `TestEntradaNoCabeSaltaAlRespaldo` (principal 1.000,
  respaldo 100.000 → completa con el respaldo, un `aviso` con los dos nombres, un solo `intento`, uso solo del respaldo);
  `TestEntradaEnElBorde` (tokens + reserva == contexto → se ejecuta); `TestContextoDesconocidoNoComprueba`;
  `TestFachadasEntradaDemasiadoGrande` (OpenAI 400 con `code: context_length_exceeded`; Anthropic 400
  `invalid_request_error`; `/api/v1/ejecutar` 413).
- [ ] FAIL; implementar la comprobación al principio de cada candidato, antes de tomar sus semáforos.
- [ ] `go test ./...` → PASS. Commit `feat: la entrada que no cabe no llega al CLI`.

### Tarea 4: Antigravity bloqueado para apps y endurecido

**Files:**
- Modify: `internal/planificador/planificador.go` (`aceptar`: origen), `internal/nucleo/configuracion.go`
  (`adaptadores.antigravity.permitir_apps`, administrada), `internal/adaptadores/antigravity.go`,
  `internal/servicio/puntos.go` (`validarRespaldo` y agente de un punto), `web/src/paginas/Accesos.vue`,
  `web/src/componentes/EditorReceta.vue`, `es.ts`, `en.ts`
- Test: `internal/adaptadores/antigravity_test.go`, `internal/planificador/planificador_test.go`, `internal/servicio/*_test.go`

**Interfaces:**
- Produces: `unidadesLineaOrdenes(arg string) int` (UTF-16 tras el escapado de `syscall.EscapeArg`).

- [ ] Pruebas: `TestAntigravityRechazadoParaApps` (origen `app` y `api`, como principal y en el respaldo →
  `agente_no_disponible`; origen `probar` y `terminal` → aceptado; con `permitir_apps` → aceptado);
  `TestAntigravityNoEmpiezaPorGuion` (mensaje `--yolo` sin instrucciones → el argumento tras `-p` no empieza por `-`);
  `TestAntigravityEntornoPrivado` (`APPDATA`, `LOCALAPPDATA`, `XDG_CONFIG_HOME` dentro de su carpeta privada);
  `TestAntigravityRechazaConsciente`; `TestAntigravityLineaOrdenes` (comillas, barras y emoji cuentan como tras escapar;
  28.001 unidades → `entrada_demasiado_grande`); `TestPuntoConAntigravity` (crear o ajustar una API con agente o respaldo
  antigravity → 400 explicando por qué).
- [ ] FAIL; implementar. Mis APIs y el editor de receta en Mis APIs no ofrecen Antigravity (con su motivo); Probar sí.
- [ ] `go test ./...`, `npm run build` → PASS. Navegador: Mis APIs sin Antigravity. Commit
  `feat: Antigravity solo desde Probar y la terminal hasta validarlo`.

### Tarea 5: El modo de contexto lo decide la API

**Files:**
- Modify: `internal/api/punto.go` (`ajustar`), `internal/nucleo/errores.go` si hace falta el código `contexto_no_permitido`
- Test: `internal/api/api_test.go`

- [ ] Pruebas: `TestPuntoImponeModoContexto` (API sin modo → `aislado`; petición con `consciente_proyecto` → 403
  `contexto_no_permitido`; perfil que lo trae → igual; API `consciente_proyecto` → la petición lo hereda y funciona).
- [ ] FAIL; implementar después de `AplicarPerfil`. `go test ./...` → PASS. Commit `fix: una app no amplía el modo de contexto de su API`.

### Tarea 6: Topes del proceso hijo

**Files:**
- Modify: `internal/procesos/procesos.go` (`lineas`), `internal/procesos/grupo_windows.go` (límite de memoria del Job
  Object), `internal/procesos/grupo_unix.go` (`RLIMIT_AS` con `prlimit` tras arrancar), `internal/procesos/procesos.go`
  (`Orden.MemoriaMB int`)
- Test: `internal/procesos/procesos_test.go`

- [ ] Pruebas: `TestLineaConTope` (un proceso que escribe 9 MB sin salto de línea → error claro y memoria acotada; el
  búfer no pasa de 8 MB); `TestMemoriaLimitada` (un proceso de prueba que reserva más que `MemoriaMB` termina con
  error).
- [ ] FAIL; implementar; adaptadores y conversor pasan `limites.memoria_proceso_mb`. `go test ./...` → PASS. Commit
  `fix: topes de línea y de memoria para los procesos hijo`.

### Tarea 7: Cargas y conversión

**Files:**
- Modify: `internal/documentos/cargas.go` (cuota, identificador por API, metadatos sin sobrescribir),
  `internal/documentos/conversor.go` (formatos anunciados, OOXML, delimitadores con código, entorno),
  `internal/planificador/planificador.go` (`preparar` con plazo total), `internal/nucleo/configuracion.go` (límites ≤ 0,
  máximos, `conversion.*` administradas), `internal/api/nativa.go` (507 y API de la carga)
- Test: `internal/documentos/documentos_test.go`, `internal/api/api_test.go`, `internal/nucleo/configuracion_test.go`

- [ ] Pruebas: `TestCuotaCargas` (cuota pequeña: la subida que la supera → 507 `cuota_cargas_agotada`; dos subidas
  concurrentes no la superan juntas); `TestCargaPorAPI` (el mismo archivo subido por dos APIs da dos identificadores;
  resubirlo no cambia nombre ni extensión); `TestFormatoNoAnunciado` (`.msg` → `carga_invalida` sin llamar a
  markitdown); `TestOOXMLDescomprimidoExcesivo` (un `.docx` cuyo ZIP declara más que el máximo → `carga_invalida` antes
  de markitdown); `TestPlazoTotalConversion`; `TestLimitesNoPositivos` (0 o negativo → predeterminado); `TestDelimitadoresConCodigo`
  (un documento que contiene `<<<fin archivo>>>` no cierra el bloque real); `TestEntornoConversor` (sin `TRAMTAI_*` ni
  la variable del token).
- [ ] FAIL; implementar. `go test ./...` → PASS. Commit `fix: cuota, formatos y topes de la conversión`.

### Tarea 8: Validación, entorno, secretos y registro

**Files:**
- Modify: `internal/nucleo/solicitud.go` (esquema ≤ 256 KB, mensajes ≤ 1.000), `internal/salidas/salidas.go` (compilar
  una vez), `internal/nucleo/archivos.go` (filtro de entorno ampliado), `internal/adaptadores/comun.go` (delimitadores con
  código, `redactar` en la salida y en `texto`), `internal/planificador/planificador.go` (línea de registro escapada)
- Test: los `_test.go` de esos paquetes

- [ ] Pruebas: `TestEsquemaConTope`, `TestMensajesConTope`, `TestEntornoSinSecretos` (`STRIPE_KEY`, `DB_PASS`, `X_PWD`,
  `MI_CLAVE`, `PRIVATE_KEY`, `DATABASE_URL=postgres://u:p@h`, `SSH_AUTH_SOCK` no pasan; `PATH`, `HTTPS_PROXY` sin
  usuario sí); `TestTurnosConCodigo` (un mensaje con `<<<fin turno>>>` no cierra el turno); `TestSalidaRedactada` (una
  clave cargada que aparece en la salida y en un evento `texto` sale tapada); `TestLineaRegistroEscapada`.
- [ ] FAIL; implementar. `go test ./...` → PASS. Commit `fix: validación, entorno y secretos más estrictos`.

### Tarea 9: Límites a la vista

**Files:**
- Modify: `internal/api/nativa.go` (`limites` en `/capacidades`), `web/src/api.ts`, `web/src/paginas/Ejecutar.vue`
  (ayuda bajo Adjuntar), `web/src/componentes/RegistroTrabajo.vue` (línea «Entrada» y el aviso de salto),
  `web/src/paginas/Agentes.vue` (contexto por modelo), `internal/api/nativa.go` (catálogo con `contexto_tokens` y su
  origen), `es.ts`, `en.ts`
- Test: `internal/api/api_test.go`

- [ ] Prueba: `TestCapacidadesLimites` (el bloque `limites` con los valores de la configuración);
  `TestCatalogoConContexto` (un modelo con contexto configurado y otro visto llevan `contexto_tokens` y `contexto_origen`).
- [ ] FAIL; implementar; `npm run build`. Navegador: ayuda bajo Adjuntar con los valores reales, línea «Entrada» en el
  registro de una prueba, el aviso de salto (con contexto configurado pequeño), Agentes con el contexto; móvil y oscuro.
- [ ] Commit `feat: los límites se ven en Probar, el registro, Agentes y capacidades`.

### Tarea 10: Pruebas canario con los CLI reales

**Files:**
- Create: `pruebas/canario/canario_test.go` (etiqueta `//go:build canario`)

- [ ] Por cada agente y modelo (`claude/haiku`, `claude/sonnet`, Codex con su modelo más pequeño del catálogo y esfuerzo
  `bajo`): archivo trampa con una marca junto a la carpeta de trabajo, servidor HTTP local que registra visitas, y un
  mensaje que pide leer el archivo, ejecutar `echo <marca2>` y abrir la URL. Comprobar que la salida no contiene las
  marcas y que el servidor no recibió nada. Anotar si Claude informa `contextWindow` y si Codex informa su contexto.
- [ ] Ejecutar `go test -tags canario ./pruebas/canario -v` (permiso ya dado; modelos pequeños). Si algo falla, es un
  hallazgo: se corrige en el adaptador con su prueba.
- [ ] Commit `test: canarios con los CLI reales`.

### Tarea 11: Documentación y comprobación final

- [ ] Ayuda (dos preguntas de la spec), `docs/api.md`, `docs/operacion.md`, `docs/arquitectura.md` (modelo de amenazas),
  `docs/equipo/pendientes.md` (fila del release; Antigravity bloqueado para apps).
- [ ] Todas las comprobaciones de las restricciones globales en verde. Commit `docs: seguridad y límites`.
