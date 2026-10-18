# Copilot CLI, novedades en inglés y web del producto (v0.44) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:**
- Copilot CLI como quinto agente: un módulo y una línea en el catálogo, sin tocar el resto del código.
- Las novedades de cada versión, también en inglés.
- La web del producto, al día con lo que hace Tramtai.

**Architecture:**
- `internal/agentes/copilot` tiene la ficha y el adaptador. Sigue el patrón de Codex: CLI con JSONL, mensaje por stdin
  y carpeta privada.
- El catálogo solo suma `copilot.Ficha`. La Web UI ya lee las fichas (v0.43), así que Agentes, Probar, Mis APIs y el
  mapa lo muestran solos.
- Las notas llevan un bloque `## English`, que `NotasVersion.vue` elige según el idioma.

**Tech Stack:** Go 1.27, Vue 3 + Quasar + TypeScript, HTML y CSS estáticos en `site/`.

**Spec:** `docs/equipo/copilot.md`. **Evidencia M0:** `docs/equipo/m0-agentes.md` y las capturas de `scratchpad/m0copilot`.

## Global Constraints

- **Lanzamiento exacto** (lo que pasó el canario en la M0):
  `--output-format json --model <m|auto> [--reasoning-effort v] --available-tools none --disallow-temp-dir
  --no-custom-instructions --disable-builtin-mcps --no-ask-user --no-auto-update --usage-output-file <raíz>/uso.json`.
  - Directorio de trabajo: el de `adaptadores.DirectorioTrabajo`.
  - Mensaje: `EnmarcarSistema(sistema, prompt)` por stdin.
  - Copilot avisa «Unknown tool name in the tool allowlist: "none"» y deja la lista vacía: es lo que se busca. Una
    prueba del módulo fija esas banderas.
- **Modelo:** vacío o `auto` → `--model auto`, sin `--reasoning-effort` (Copilot lo rechaza con `auto`).
- **Entorno:**
  - `EntornoLimpio(nil, extra...)` y, además, fuera toda variable `COPILOT_*` heredada (BYOK, `COPILOT_HOME`), menos las
    de `extra`;
  - `COPILOT_GITHUB_TOKEN`, `GH_TOKEN` y `GITHUB_TOKEN` se suman a `variablesCredenciales`;
  - `COPILOT_GITHUB_TOKEN` se suma a `credenciales.Nombres`.
- **Fixtures sin datos personales:** el JSONL y el `uso.json` de prueba salen de la M0 sin rutas del equipo, sin ids y
  sin `apiCallId`; solo los eventos que se leen y alguno que se ignora.
- **Textos:**
  - nuevos en `web/src/i18n.ts` (es/en);
  - mensajes Go con su traducción en `internal/idioma/en_*.go`;
  - todo lo nuevo de `site/`, con `<span lang="es">` y `<span lang="en">`.
- **Comprobaciones:**
  - `go vet ./...` (también con `GOOS=linux` y `GOOS=darwin`), `go test ./...`, `cd escritorio && go vet ./...`;
  - `cd web && npm run build`, `graphify update .`;
  - canarios: Claude haiku, Codex `gpt-5.6-terra` y Copilot `auto` (un crédito del plan gratuito).
- **Credenciales:** nunca se escribe una clave ni se inicia sesión: la sesión de Copilot ya la abrió el usuario.

## Review Focus

1. Copilot no puede usar herramientas, ni MCP, ni instrucciones del usuario, ni su configuración: banderas, entorno y
   `config.json` privado con solo `loggedInUsers` y `lastLoggedInUser` (Tarea 1).
2. Ninguna credencial llega a Copilot salvo la suya (`COPILOT_GITHUB_TOKEN` en modo clave), y los errores la redactan
   (Tarea 1).
3. La carpeta privada se borra siempre, también si el CLI falla o se cancela (Tarea 1).
4. Las notas en dos idiomas se ven bien en es y en en; las versiones viejas, sin bloque inglés, se siguen viendo
   (Tarea 4).
5. La web no tiene desplazamiento horizontal en el móvil y sus capturas no muestran datos personales (Tarea 5).

---

### Tarea 1: El módulo `internal/agentes/copilot`

**Files:**
- Create:
  - `internal/agentes/copilot/ficha.go` y `copilot.go`;
  - `copilot_test.go`;
  - `testdata/salida.jsonl` y `testdata/uso.json`.
- Modify:
  - `internal/agentes/catalogo.go` (una línea);
  - `internal/adaptadores/comun.go` (`variablesCredenciales`);
  - `internal/credenciales/credenciales.go` (`Nombres`);
  - `internal/idioma/en_*.go`.

- [ ] **Ficha** con los valores de la tabla de la spec:
  - `GH`, `#1f883d` / `#3fb950`, orden 25, `Streaming: "tokens"`;
  - `ModeloEfectivoInformado: true`;
  - `ValoresEsfuerzo: low, medium, high, xhigh, max`;
  - las dos autenticaciones, `VariablesClave: COPILOT_GITHUB_TOKEN`;
  - `Comando: "copilot"`, concurrencia 1, `PermitidoEnApps: true` y la URL de instalación.

  `Crear` sigue el patrón de Codex (`Programa` y `Autenticacion`); `ModoAutenticacion` implementa `ConAutenticacion`.
- [ ] **`Detectar`:**
  - `ResolverPrograma` y `VersionDe`;
  - con suscripción, lee el `config.json` de `COPILOT_HOME` o `~/.copilot` (JSONC: se quitan las líneas que empiezan
    por `//`);
  - sin `loggedInUsers` → `ProblemaFaltaClave`: «inicia sesión con copilot login»;
  - con clave, `ClaveEntorno("COPILOT_GITHUB_TOKEN")`;
  - los términos de la spec.
- [ ] **`Ejecutar`:**
  - `EspacioTemporal` con `defer os.RemoveAll(raiz)` y `home := raiz/copilot-home` (0o700);
  - con suscripción: `config.json` privado (0o600) con solo esas dos claves, como `json.RawMessage`. Sin ellas, error
    `CodigoAutenticacionAgente`;
  - con clave: `COPILOT_GITHUB_TOKEN=<clave>`;
  - argumentos por una función `argumentos(s, raiz)`, que se prueba sola.
- [ ] **Lectura del JSONL** en una función `leer(linea, *estado)`:
  - `assistant.message_delta` → `emitir(texto)`;
  - `assistant.message` → contenido final y modelo;
  - `session.auto_mode_resolved` → `chosenModel`;
  - `result` → `exitCode` y `premiumRequests`.

  Al terminar:
  - se leen los tokens de `uso.json`, sumados de `modelMetrics.*.usage` (entrada = `inputTokens` + `cacheReadTokens` +
    `cacheWriteTokens`);
  - se emite un `aviso` «Copilot: N créditos del plan»;
  - se devuelve `Resultado{Salida, Uso, ModeloEfectivo}`.
- [ ] **Errores:**
  - salida distinta de 0 o sin `result`: `RedactarCon(stderr, clave)` y `Clasificar`;
  - sin texto: `CodigoErrorProtocoloAgente` («copilot terminó sin result»).
- [ ] **Catálogo y credenciales:** `copilot.Ficha` en `fichas`; las variables de credenciales; `TestFichasCompletas`
  y `TestSinNombresFuera` pasan sin cambios.
- [ ] **Pruebas** con el binario de prueba como CLI simulado: `TestMain` mira `TRAMTAI_COPILOT_SIMULADO` y, si está
  puesto, hace de Copilot:
  - escribe en un archivo stdin, argumentos, entorno y el `config.json` de `COPILOT_HOME`;
  - imprime `testdata/salida.jsonl` y copia `testdata/uso.json` donde diga `--usage-output-file`;
  - o, si se le pide, sale con 1 y el error de modelo.

  Casos:
  - texto en vivo y final, modelo efectivo, uso y aviso;
  - el mensaje con «piña» llega por stdin;
  - con `auto`, sin `--reasoning-effort`; con un modelo y nivel alto, `high`;
  - el entorno sin `GH_TOKEN`, `OPENAI_API_KEY` ni `COPILOT_PROVIDER_API_KEY` puestos en la prueba;
  - con suscripción, el `config.json` privado solo tiene las dos claves (la fuente lleva también otras);
  - con clave, `COPILOT_GITHUB_TOKEN` llega;
  - error de modelo → `modelo_no_disponible`, sin la clave en el mensaje;
  - la carpeta privada no queda.
- [ ] Commit: `feat(agentes): GitHub Copilot CLI como agente (módulo copilot)`.

### Tarea 2: Canario y prueba real

**Files:** Modify: `pruebas/canario/canario_test.go` y `internal/agentes/real_test.go`.

- [ ] El caso `copilot/auto` con `copilot.Copilot{Programa: "copilot", Autenticacion: suscripcion}` y modelo `auto`.
- [ ] El caso real (`-tags agentes`), con una respuesta corta.
- [ ] Ejecutar el canario de Copilot y la prueba real; anotar los créditos gastados en m0-agentes.md. La M0 ya probó
  las banderas.
- [ ] Commit: `test: canario y prueba real de Copilot`.

### Tarea 3: La Web UI con Copilot

**Files:** Modify: `web/src/paginas/Agentes.vue` y `Ayuda.vue` (solo si hace falta), y `web/src/i18n.ts`.

- [ ] Con el servicio de prueba, comprobar que Copilot aparece solo desde su ficha en:
  - Agentes: fila, versión, cambiar a suscripción, «Volver a comprobar»;
  - Probar: el selector y la respuesta en vivo con `auto`;
  - Mis APIs: el filtro por agente;
  - el mapa de Inicio.
- [ ] Si Agentes da instrucciones de conexión por agente, añadir las de Copilot (`copilot login` o
  `tramtai claves guardar COPILOT_GITHUB_TOKEN`) desde la ficha, sin nombrar el agente en el código de la página.
- [ ] En la Ayuda:
  - Copilot en «¿Qué agentes puedo usar?»;
  - la pregunta nueva «¿Puedo usar Copilot con el plan gratuito?», en es y en.
- [ ] `npm run build`; confirmar los recursos compilados. Commit: `feat(web): Copilot en Agentes, Probar y la Ayuda`.

### Tarea 4: Las novedades en inglés

**Files:**
- Modify:
  - `scripts/version.ps1`;
  - `web/src/componentes/NotasVersion.vue` (y quien le pase la página de la versión: `ListaNovedades.vue`, `App.vue`);
  - `internal/actualizacion/actualizacion.go` (recorte a 12000) y su prueba.

- [ ] **`version.ps1`:**
  - acepta `-ResumenEn`; con `-Etiquetar` y `-Resumen`, sin `-ResumenEn`, se detiene con un error claro;
  - las notas terminan en `` `n`n## English`n`nTramtai <v>`n`n<ResumenEn>`n`nFull list of changes (in Spanish) above. ``;
  - probarlo con `-Explicar`.
- [ ] **`NotasVersion.vue`:**
  - nueva prop `pagina?: string`;
  - se parte el texto por la línea `## English`;
  - en inglés, con bloque, se analiza el bloque inglés (sin la línea «Full list…») y se añade el enlace «See every
    change on GitHub» si hay página;
  - en español, o sin bloque, la parte de antes, como hoy.

  `ListaNovedades` y `App.vue` le pasan `n.url` y la de la actualización.
- [ ] **El recorte** pasa a 12000: la prueba de `armar` lo cubre.
- [ ] **En el navegador**, con notas de prueba de dos idiomas y otras viejas, en es y en.
- [ ] Commit: `feat: novedades de cada versión también en inglés`.

### Tarea 5: La web del producto

**Files:**
- Modify: `site/index.html`, `site/estilo.css` y, si hace falta, `site/sitio.js` y `site/ayuda.html`.
- Create: `site/capturas/agentes-{claro,oscuro}.png` y `probar-{claro,oscuro}.png`.

- [ ] **Portada:** la quinta línea del mapa SVG (Copilot, `GH`, su color) y Copilot en el texto de entrada.
- [ ] **Sección «Agentes»** (`#agentes`, enlazada en el menú) entre «Cómo funciona» y las capturas:
  - una tarjeta por agente con su chapa, cómo se conecta, esfuerzo y respuesta en vivo;
  - Antigravity experimental; Copilot con la nota del plan gratuito;
  - debajo, la línea del catálogo por módulos;
  - una columna en el móvil.
- [ ] **Funciones:**
  - tres tarjetas nuevas: API propia con OpenAPI, agentes sin herramientas y Ayuda integrada;
  - la de modelo y esfuerzo pasa a «en Claude, Codex, Copilot y Antigravity», o los que digan sus fichas.
- [ ] **Capturas** de Agentes y de Probar con Ver como código, en claro y oscuro (1440×900, Edge headless, perfil
  limpio y nombres neutros), en la galería con su alt y su pie es/en.
- [ ] **Textos al día:** `meta description` es/en y `ayuda.html` (cómo conectar Copilot).
- [ ] **Comprobar en el navegador:** escritorio y móvil (375 px, sin desplazamiento horizontal), claro y oscuro, es y
  en.
- [ ] Commit: `feat(site): sección de agentes, Copilot, funciones nuevas y capturas`.

### Tarea 6: Documentación y verificación

**Files:**
- Modify:
  - `docs/arquitectura.md`, `docs/api.md`, `docs/inicio.md`;
  - `README.md` (es y su sección English);
  - `docs/equipo/nuevo-agente.md` (aprendido: comprobar que una lista vacía de herramientas de verdad se aplica);
  - `docs/equipo/pendientes.md` (fila v0.44 ✅).
- [ ] **Docs:**
  - Copilot en la tabla de adaptadores y en los ejemplos de `model` (`copilot`, `copilot/<modelo>`);
  - cómo conectarlo;
  - las notas en dos idiomas (`-ResumenEn`) en `docs/desarrollo.md`.
- [ ] **README:** la sección English al día con los agentes (Copilot incluido), la API propia con OpenAPI y la Ayuda
  integrada.
- [ ] **Comprobaciones:** todas las de Global Constraints y los canarios de Claude, Codex y Copilot.
- [ ] `graphify update .`. Commit: `docs: Copilot CLI, notas en inglés y web del producto`.
- [ ] Después, el flujo del release: revisión de toda la rama (`ecc:code-reviewer`, opus), todos los arreglos, fusión,
  `version.ps1 -Etiquetar -Resumen … -ResumenEn …`, la etiqueta, CI y Publicar, y los paquetes.
