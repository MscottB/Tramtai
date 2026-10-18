# Agentes por módulos y listas que crecen (v0.43) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que añadir un agente sea crear un módulo y una línea en un catálogo (sin tocar el resto del código), y que Mis
APIs y Agentes sigan siendo cómodos con muchas APIs y muchos agentes.

**Architecture:**
- `internal/adaptadores` se queda con el contrato y lo común, con los ayudantes exportados.
- Cada agente pasa a `internal/agentes/<nombre>` con su adaptador y su ficha.
- `internal/agentes` es el catálogo: lista las fichas, crea el registro y contesta lo que hoy se decide por nombre.
- `nucleo` no puede importar `agentes` (sería un ciclo), así que el catálogo le registra los valores predeterminados de
  los agentes al iniciarse.
- La Web UI deja de nombrar agentes: lee las fichas de `GET /api/v1/agentes`.

**Tech Stack:** Go 1.27, Vue 3 + Quasar + TypeScript.

**Spec:** `docs/equipo/agentes-modulos.md`. **Evidencia M0:** `docs/equipo/m0-agentes.md`.

## Global Constraints

- **Mismo comportamiento:** mover un agente no cambia sus órdenes, su entorno ni sus eventos. Las pruebas actuales se
  mueven con su código y siguen comprobando lo mismo.
- **Contrato de la ficha** (`agentes.Ficha`):
  - `Nombre`, `NombreVisible`, `Programa` (el CLI, «Claude Code»), `Codigo`, `ColorClaro`, `ColorOscuro`, `Orden`;
  - `Streaming` (`tokens` o `final`), `SalidaEstructurada`, `ModeloEfectivoInformado`;
  - `ValoresEsfuerzo` (valor del CLI por cada nivel de `nucleo.NivelesEsfuerzo`, hasta su máximo),
    `EsfuerzoPredeterminado func(modelo string) string`;
  - `Autenticaciones` (`suscripcion`, `clave_api`), `VariablesClave` (las que guarda `tramtai claves`),
    `PrefijosPropios` (las variables que su CLI puede recibir);
  - `Comando` (predeterminado de `comandos_agentes.<nombre>`, vacío si no es un CLI), `Concurrencia`,
    `PermitidoEnApps`, `Instalacion`, `URLInstalacion`;
  - `Crear func(c *nucleo.Configuracion) adaptadores.Adaptador`.
- **Colores:** los de `estilos.css` pasan a las fichas tal cual (claro y oscuro).
- **Excepciones a «sin nombres fuera de `internal/agentes`»:** `predeterminados.agente: claude`, las pruebas, `idioma/`
  (textos) y `pruebas/`.
- **Textos nuevos** en `es.ts` y `en.ts`; mensajes Go nuevos con su traducción en `internal/idioma/en_*.go`.
- **Comprobaciones:**
  - `go vet ./...` (también con `GOOS=linux` y `GOOS=darwin`), `go test ./...`, `cd escritorio && go vet ./...`;
  - `cd web && npm run build`, `graphify update .`;
  - canarios: `go test -tags canario ./pruebas/canario`, solo Claude haiku y Codex `gpt-5.6-terra` con esfuerzo bajo.

## Review Focus

1. Ningún adaptador cambió lo que lanza: mismos argumentos y entorno. Lo comprueban las pruebas movidas (Tareas 3–4).
2. `Secretos()` sigue redactando las mismas variables, ahora sumadas desde las fichas (Tarea 4).
3. La suscripción solo se ofrece y acepta en agentes cuya ficha la admite: Web UI, `POST /ui/autenticacion` y la
   comprobación de acceso remoto (Tarea 4).
4. Un agente con `PermitidoEnApps` falso se sigue rechazando para apps y APIs, en el principal y en el respaldo; un
   `adaptadores.<nombre>.permitir_apps: true` lo habilita (Tarea 4).
5. La paginación de Mis APIs no pierde APIs entre páginas con grupos plegados, y un filtro vuelve a la página 1
   (Tarea 7).

---

### Tarea 1: Lo común de adaptadores, exportado

**Files:**
- Modify: `internal/adaptadores/comun.go`, `claude.go`, `codex.go`, `gemini.go`, `antigravity.go` y sus pruebas (solo
  nombres)

- [ ] Exportar los ayudantes que usarán los módulos, cambiando solo el nombre y sus llamadas:
  - `separar` → `Separar`;
  - `enmarcarSistema` → `EnmarcarSistema`;
  - `entornoLimpio` → `EntornoLimpio`;
  - `claveEntorno` → `ClaveEntorno`;
  - `resolverPrograma` → `ResolverPrograma`;
  - `directorioTrabajo` → `DirectorioTrabajo`;
  - `consciente` → `Consciente`;
  - `espacioTemporal` → `EspacioTemporal`;
  - `redactar` → (ya existe `Redactar`; la interna pasa a llamarse `RedactarCon`);
  - `clasificar` → `Clasificar`;
  - `version` → `VersionDe`;
  - `codigoDelimitador` → `CodigoDelimitador`.
  
  Más los que aparezcan al mover: lo que use otro paquete se exporta.
- [ ] `go test ./internal/adaptadores` igual que antes; commit `refactor(adaptadores): exportar lo común`.

### Tarea 2: Ficha y catálogo

**Files:**
- Create: `internal/agentes/ficha.go`, `internal/agentes/catalogo.go`, `internal/agentes/catalogo_test.go`
- Modify: `internal/nucleo/configuracion.go` (`AgregarInternos`)

**Interfaces:**
- `type Ficha struct { … }` (Global Constraints).
- `func Todos() []Ficha`, ordenadas por `Orden`.
- `func Buscar(nombre string) (Ficha, bool)`.
- `func Registro(c *nucleo.Configuracion) (map[string]adaptadores.Adaptador, map[string]int)`.
- `func Niveles(nombre string) []string`.
- `func ValorEsfuerzo(nombre, nivel string) string`.
- `func EsfuerzoPredeterminado(nombre, modelo string) string`.
- `func AdmiteSuscripcion(nombre string) bool`.
- `func PermitidoEnApps(c *nucleo.Configuracion, nombre string) bool`: la ficha o `adaptadores.<nombre>.permitir_apps`.
- `func Secretos() []string`, `func PrefijosPropios(nombre string) []string`.
- En `nucleo`: `func AgregarInternos(valores map[string]any)`, para los valores predeterminados que aporta cada ficha
  (`comandos_agentes.*`, `concurrencia_servicio.adaptadores.*`). La llama `agentes` en su `init`; los valores ya
  escritos en `Internos` no se pisan.

- [ ] `TestFichasCompletas`:
  - toda ficha tiene nombre, nombre visible, código de 2 letras, colores `#rrggbb`, orden y `Crear`;
  - el nombre, el código y el orden son únicos;
  - los `ValoresEsfuerzo` no superan los niveles de `nucleo`.
- [ ] Catálogo vacío por ahora (las fichas llegan en la Tarea 3); commit `feat(agentes): ficha y catálogo`.

### Tarea 3: Cada agente en su módulo

**Files:**
- Create:
  - `internal/agentes/claude/claude.go` (+ `ficha.go`, pruebas);
  - igual en `codex/`, `gemini/` y `antigravity/`.
- Move: el código y las pruebas de `internal/adaptadores/{claude,codex,gemini,antigravity}.go` y sus `_test.go`; la
  parte de cada agente de `esfuerzo.go` y `real_test.go`.
- Modify: `internal/agentes/catalogo.go` (las cuatro fichas)

- [ ] Mover agente por agente con `git mv` (para conservar el historial). Ajustar el paquete, los imports y las
  llamadas a los ayudantes exportados.
- [ ] Cada ficha con los datos de hoy:
  - **Nombres y códigos:** los de `api.ts`.
  - **Colores:** los de `estilos.css`.
  - **Capacidades:** las de `nativa.go`.
  - **Esfuerzo:** el de `esfuerzo.go`.
  - **Autenticación:** suscripción solo en Claude y Codex.
  - **Variables:** las de `variablesCredenciales` y `propios*`.
  - **Comandos y concurrencia:** los de `configuracion.go`.
  - **`PermitidoEnApps`:** falso solo en Antigravity.
  - **Instalación:** los textos de Agentes.
- [ ] `CambiarAutenticacion` deja de mirar tipos concretos. Una interfaz `adaptadores.ConAutenticacion` con
  `Modo() *Modo` la cumplen Claude y Codex; el error sale de la ficha.
- [ ] Todas las pruebas movidas pasan sin cambiar sus comprobaciones. Commit `refactor(agentes): un módulo por agente`.

### Tarea 4: El resto del código pregunta al catálogo

**Files:**
- Modify:
  - `internal/servicio/*.go` (`adaptadores.Registro` → `agentes.Registro`; `cambiarAutenticacion`, `puntos.go:183-200`);
  - `cmd/tramtai/servicio.go:461`;
  - `internal/api/nativa.go` (capacidades desde la ficha);
  - `internal/planificador/planificador.go` (`PermitirAntigravityApps` → `PermitidosEnApps map[string]bool` en
    `OpcionesDesde`);
  - `internal/adaptadores/comun.go` (`Secretos()` → `agentes.Secretos()` donde se llame);
  - `internal/nucleo/configuracion.go` (fuera los valores predeterminados por agente);
  - las llamadas de `escritorio/` y `cmd/`.
- Create: `internal/agentes/nombres_test.go`

- [ ] `TestSinNombresFuera` recorre los `.go` del repositorio (sin `_test.go`, `internal/agentes/`, `internal/idioma/`
  ni `pruebas/`). Falla si encuentra `"<nombre>"` o `'<nombre>'` de una ficha, salvo `predeterminados.agente`.
- [ ] Pruebas:
  - suscripción rechazada para Gemini (`POST /ui/autenticacion`);
  - acceso remoto rechazado con un agente en suscripción, recorriendo las fichas;
  - Antigravity rechazado en apps y en respaldo, y habilitado con `permitir_apps`;
  - `Secretos()` igual a la lista anterior.
- [ ] Commit `refactor: el código pregunta al catálogo de agentes`.

### Tarea 5: `GET /api/v1/agentes`

**Files:**
- Modify: `internal/api/api.go` (`Rutas`, dentro de `contrato("nativa")`), `internal/api/nativa.go`,
  `internal/api/openapi.json` (ruta y esquema `Agente`)
- Test: `internal/api/api_test.go`

**Interfaces:**
- Respuesta `{"agentes":[{"agente","nombre","programa","codigo","color":{"claro","oscuro"},"orden","streaming",
  "salida_estructurada","modelo_efectivo_informado","esfuerzos","autenticaciones","variables_clave",
  "permitido_apps","instalacion","url_instalacion"}]}`.
- Sin comprobar los CLI: debe responder al momento. `permitido_apps` ya cuenta `permitir_apps`.
- Solo los agentes del registro de la instancia: en las pruebas, los simulados con una ficha neutra.

- [ ] Pruebas: 401 sin clave; con clave, los del registro en orden; `TestOpenAPICubreLasRutas` sigue pasando.
- [ ] Commit `feat(api): fichas de los agentes`.

### Tarea 6: La Web UI lee las fichas

**Files:**
- Modify:
  - `web/src/api.ts` (`cargarFichas`, `fichas`, `nombreAgente`, `codigoLinea`, `ordenAgentes`, `colorAgente`);
  - `web/src/estilos.css` (fuera las clases `.linea--<agente>` y las variables `--linea-<agente>`; queda
    `.linea--otro`);
  - `MapaRed.vue`, `EditorReceta.vue`, `Agentes.vue`, `Accesos.vue`, `Ejecutar.vue`, `Ruta`/`Linea` si colorean por
    clase.

**Interfaces:**
- `fichas = ref<Record<string, FichaAgente>>({})`, cargadas con la sesión (como el catálogo).
- `estiloLinea(agente)` → `{ '--c': color según el tema }`, sustituye a `:class="'linea--' + agente"`.
- Sin ficha: id tal cual, código de dos letras y `--linea-otro`.

- [ ] Sustituir:
  - `puedeCambiar` → `autenticaciones` incluye `suscripcion`;
  - `cli` → `programa`;
  - `claves` → `variables_clave[0]`;
  - `sinAntigravity`/`usables` → `permitido_apps`;
  - los `'claude'` de último recurso → el predeterminado del catálogo de modelos.
- [ ] Una búsqueda de `'claude'|'codex'|'gemini'|'antigravity'` en `web/src` (sin `idiomas/`) no encuentra nada.
- [ ] `npm run build`; navegador: los colores en claro y oscuro iguales a los de antes. Commit `feat(web): los
  agentes salen de sus fichas`.

### Tarea 7: Mis APIs en pestañas, con filtro y páginas

**Files:**
- Modify: `web/src/paginas/Accesos.vue`, `es.ts`, `en.ts`

- [ ] **Pestañas** (`q-tabs` + `q-tab-panels`):
  - «APIs por aplicación (n)»: la lista;
  - «API principal»: la guía de URL, la clave, los modelos, los ejemplos y el presupuesto del espacio.
  
  La pestaña se recuerda en `localStorage` (`tramtai.pestanaApis`). `grupoInicial` (desde Inicio) abre «APIs por
  aplicación».
- [ ] **Filtros nuevos:** estado (todas, activas, detenidas, con error) y agente (de las fichas), junto a los de ahora.
- [ ] **Páginas:**
  - `porPagina = 20` sobre la lista ya filtrada y ordenada, y luego se agrupa la página;
  - los grupos plegados ocupan una fila: se pagina sobre filas visibles (cabeceras y APIs), para no esconder APIs;
  - `q-pagination` con «Mostrando a–b de n» solo si hay más de una página;
  - cambiar un filtro o la búsqueda vuelve a la página 1.
- [ ] Navegador: con 60 APIs de prueba (creadas en el servicio de prueba y borradas al terminar), en móvil y
  escritorio. Commit `feat(web): Mis APIs en pestañas con filtros y páginas`.

### Tarea 8: Agentes ordenado para crecer

**Files:**
- Modify: `web/src/paginas/Agentes.vue`, `es.ts`, `en.ts`

- [ ] **Arriba:** buscador; filtro por estado (Todos, Listos, Falta conectar, No instalados, con su número); «Volver a
  comprobar».
- [ ] **Lista:**
  - `q-list` de `q-expansion-item` (`group="agentes"`, uno abierto a la vez);
  - cabecera: chapa, nombre, versión, etiqueta de estado, esfuerzo y contexto;
  - cuerpo: lo que hoy tiene la tarjeta (conectar, cambiar autenticación y detalles);
  - orden: listos, falta conectar y luego el resto, en el orden del catálogo.
- [ ] **No instalados:** en un `q-expansion-item` plegado, «Otros agentes que puedes instalar (n)», con la instalación
  de su ficha.
- [ ] Navegador: con los cuatro agentes (Gemini y Antigravity sin clave en el servicio de prueba), en móvil y
  escritorio. Commit `feat(web): Agentes en filas con filtro`.

### Tarea 9: Documentación y verificación

**Files:**
- Create: `docs/equipo/nuevo-agente.md`
- Modify:
  - `docs/arquitectura.md`, `docs/api.md`, `docs/equipo/pendientes.md`;
  - Ayuda (`es.ts`/`en.ts` y `Ayuda.vue`: pregunta «¿Qué agentes puedo usar?»; ajustar las guías que hablan de Mis
    APIs o Agentes);
  - `site/ayuda.html` y las capturas de `site/` si muestran Mis APIs o Agentes.

- [ ] `nuevo-agente.md`: la lista de la spec, con la ficha comentada campo a campo y cómo grabar el JSONL de la M0
  para la prueba con el CLI simulado.
- [ ] `pendientes.md`: la fila de v0.43 y las previstas v0.44 Copilot CLI, v0.45 OpenCode y v0.46 Hermes Agent.
- [ ] Canarios: Claude haiku y Codex `gpt-5.6-terra` (esfuerzo bajo) tras el movimiento.
- [ ] Comprobaciones globales; commit `docs: agentes por módulos y cómo añadir uno`.
