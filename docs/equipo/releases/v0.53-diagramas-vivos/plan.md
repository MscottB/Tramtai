# Diagramas vivos (v0.53) — plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Las peticiones en curso se ven moverse por el mapa de Inicio y la portada del sitio muestra el recorrido completo hasta los agentes con cuatro ejemplos.

**Architecture:** Un SSE nuevo por instancia (`GET /api/v1/eventos`) reusa el aviso de cambios del almacén y lee los
trabajos cambiados por `actualizado_en`. La Web UI lo consume con un composable; la lógica (eventos → tranvías →
posiciones) va en `web/src/vivo.ts`, puro y probado, y `MapaRed.vue` solo dibuja. La portada anima con la Web
Animations API sobre su SVG, con los ejemplos como JSON dentro de `index.html`.

**Tech Stack:** Go (`net/http`, SQLite modernc), Vue 3 + Quasar, `node:test`, HTML/CSS/JS del sitio.

**Spec:** `docs/equipo/releases/v0.53-diagramas-vivos/spec.md`

## Global Constraints

- Ningún evento lleva mensaje, respuesta, archivos ni instrucciones: solo `id, estado, etapa, agente, modelo, api, origen, intento, codigo_error, actualizado`.
- Foto inicial: no terminados + terminados hace **menos de 10 s**. Avisos juntados: **una lectura cada 200 ms** como mucho. Latido **15 s**. Tope **8 flujos por instancia**, el siguiente `429` código `limite_flujos`, `Retry-After: 5`.
- Etapas: `cola`=en_cola · `preparacion`=preparando_entrada, esperando_agente · `ejecucion`=ejecutando, cancelacion_solicitada · `validacion`=validando_salida · `llegada`=completado, fallido, cancelado, interrumpido.
- Mapa: hasta **12** tranvías y «+N»; llegada visible **2 s**, fallo **4 s** en rojo, cancelado **2 s** en gris; animación **~600 ms**, sin animar con `prefers-reduced-motion`. Reconexión: **1, 2, 4, 8, 16, 30, 30… s**.
- Portada: **4 ejemplos**, **~6 s** cada uno, con **Pausa** y puntos; sin JavaScript, el mapa fijo y el primer ejemplo.
- Todo texto nuevo en español e inglés; regla de la v0.52: cada función con su Ayuda y su página del sitio.
- Sin dependencias nuevas (npm ni Go). Nunca push a `main`: rama `feat/v0.53-diagramas-vivos` y PR.

## Review Focus

1. La clave de una API por aplicación abre `/api/v1/eventos` → solo ve sus trabajos, también en la foto inicial (prueba en tarea 3).
2. Un trabajo dura menos de 200 ms (en cola → completado entre dos lecturas) → igual llega un evento con su estado final (prueba en tarea 3).
3. El flujo se corta (servicio reiniciado, sesión caducada) → el mapa reintenta con espera creciente y, si no puede, vuelve a la consulta de cada 5 s sin dejar tranvías congelados (prueba en tarea 4: `esperaReconexion` y `limpiar`).
4. Llega un evento de un trabajo que ya se borró del mapa (tarde o repetido) → no reaparece un tranvía fantasma terminado (prueba en tarea 4).
5. Portada con «reducir movimiento» o en una pestaña oculta → no se anima ni acumula animaciones al volver (comprobación en el navegador, tarea 6; `document.hidden` pausa el ciclo).

---

### Task 1: Trabajos cambiados desde un instante (almacén)

**Files:**
- Modify: `internal/persistencia/almacen.go` (índice nuevo en el esquema), `internal/persistencia/trabajos.go`
- Test: `internal/persistencia/vivos_test.go`

**Interfaces:**
- Produces:
  - `func (a *Almacen) Vivos(ctx context.Context, instancia, punto string, terminadosDesde time.Time) ([]nucleo.Trabajo, error)` — no terminados y terminados con `terminado_en >= terminadosDesde`; `punto` vacío = todos los de la instancia; orden `actualizado_en`.
  - `func (a *Almacen) CambiadosDesde(ctx context.Context, instancia, punto string, desde time.Time) ([]nucleo.Trabajo, error)` — `actualizado_en > desde`, orden `actualizado_en`, como mucho 500.
  - Índice `CREATE INDEX IF NOT EXISTS trabajos_instancia_actualizado ON trabajos (instancia, actualizado_en)`.

- [ ] **Step 1: Pruebas** (`vivos_test.go`): `TestVivosTraeEnCursoYRecientes` (uno en cola, uno ejecutando, uno terminado hace 3 s, otro hace 30 s → con `terminadosDesde = ahora-10s` vuelven 3); `TestVivosFiltraPorPunto` (dos puntos → con `punto="a"` solo los de `a`); `TestCambiadosDesdeOrdenYCorte` (tras cambiar el estado de uno, `CambiadosDesde(antes)` lo trae y `CambiadosDesde(después)` no); `TestCambiarAgenteActualiza` (`CambiarAgente` mueve `actualizado_en`, así el respaldo llega al flujo).
- [ ] **Step 2:** `go test ./internal/persistencia -run 'Vivos|Cambiados|CambiarAgenteActualiza'` → FAIL (no existen).
- [ ] **Step 3:** Implementar; si `CambiarAgente` no actualiza `actualizado_en`, que lo haga.
- [ ] **Step 4:** `go test ./internal/persistencia` → PASS.
- [ ] **Step 5:** Commit `feat(persistencia): trabajos vivos y cambiados desde un instante`.

### Task 2: Cupo de flujos por instancia (planificador)

**Files:**
- Modify: `internal/planificador/planificador.go`; `internal/nucleo/errores.go` (código `CodigoLimiteFlujos = "limite_flujos"`, HTTP 429)
- Test: `internal/planificador/flujos_test.go`

**Interfaces:**
- Produces: `func (p *Planificador) ReservarFlujo() (liberar func(), ok bool)` con tope `MaximoFlujos = 8`; `liberar` es idempotente. Pasarelas: `func (p *Planificador) Vivos(ctx context.Context, punto string, desde time.Time) ([]nucleo.Trabajo, error)` y `func (p *Planificador) CambiadosDesde(ctx context.Context, punto string, desde time.Time) ([]nucleo.Trabajo, error)` sobre la instancia del planificador.

- [ ] **Step 1: Prueba** `TestReservarFlujoTope`: 8 reservas `ok`, la novena `!ok`; tras un `liberar` (llamado dos veces) se puede reservar una más y no dos.
- [ ] **Step 2:** `go test ./internal/planificador -run ReservarFlujo` → FAIL. **Step 3:** implementar (contador con mutex). **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat(planificador): cupo de flujos de eventos por instancia`.

### Task 3: `GET /api/v1/eventos`

**Files:**
- Create: `internal/api/vivo.go`, `internal/api/vivo_test.go`
- Modify: `internal/api/api.go` (ruta junto a `/api/v1/trabajos`), `internal/api/openapi.json` (ruta, esquema `EventoTrabajo`, respuesta 429), `internal/idioma/en_*.go` (textos nuevos), `web/src/idiomas/es.ts`/`en.ts` (`error.limite_flujos`)

**Interfaces:**
- Consumes: tareas 1 y 2.
- Produces:
  - `type EventoTrabajo struct { ID, Estado, Etapa, Agente, Modelo, API, Origen string; Intento int; CodigoError string; Actualizado time.Time }` con JSON `id, estado, etapa, agente, modelo, api, origen, intento, codigo_error, actualizado`. `Modelo` = `ModeloEfectivo` o, si falta, el pedido.
  - `func EtapaDe(e nucleo.Estado) string` (tabla de Global Constraints).
  - Manejador `func (s *Servidor) eventosInstancia(w http.ResponseWriter, r *http.Request)`: reserva cupo (si no, `429` + `Retry-After: 5`); `punto := ""` o `s.Punto.Nombre`; envía la foto (`Vivos(ctx, punto, ahora-10s)`), luego un bucle igual al de `seguir` en `api.go`: tomar `Avisos()` antes de leer, `CambiadosDesde(ultimo)`, enviar solo si cambió `estado`, `agente` o `intento` respecto de lo último enviado por ese id, esperar aviso / latido 15 s / contexto; tras cada lectura, no volver a leer antes de 200 ms. `intento` lo lleva el flujo por id: 1 la primera vez y +1 cada vez que ve otro agente para ese id.
  - Eventos SSE con nombre `trabajo` y sin `id:` (el flujo no se reanuda: al reconectar llega otra foto).

- [ ] **Step 1: Pruebas** (`vivo_test.go`, con el servidor de prueba de `api_test.go`):
  - `TestEventosFotoInicial`: un trabajo en curso y uno terminado ya → la foto trae los dos con su `etapa`.
  - `TestEventosCambioDeEstado`: tras crear un trabajo, llegan eventos hasta `etapa: "llegada"` con `estado: "completado"`.
  - `TestEventosTrabajoRapido`: un trabajo que termina antes de la primera lectura igual aparece con su estado final (Review Focus 2).
  - `TestEventosRespaldoSubeIntento`: con un agente simulado que falla y respaldo → un evento con otro `agente` e `intento: 2`.
  - `TestEventosSoloSusTrabajos`: con la clave de una API, ni la foto ni los cambios traen trabajos de otra (Review Focus 1).
  - `TestEventosSinContenido`: el JSON de cada evento tiene exactamente las 10 claves de la constraint.
  - `TestEventosTope`: 8 flujos abiertos → el noveno `429` con `codigo: limite_flujos` y `Retry-After: 5`; al cerrar uno se puede abrir otro.
  - `TestEtapaDe`: los 10 estados de la tabla.
- [ ] **Step 2:** `go test ./internal/api -run 'Eventos|EtapaDe'` → FAIL. **Step 3:** implementar. **Step 4:** `go test ./internal/api ./internal/idioma` → PASS (`TestCobertura` exige el inglés de cada `idioma.T` nuevo).
- [ ] **Step 5:** Commit `feat(api): flujo de eventos de la instancia (GET /api/v1/eventos)`.

### Task 4: La lógica del mapa vivo (`vivo.ts`)

**Files:**
- Create: `web/src/vivo.ts`, `web/src/vivo.test.ts`

**Interfaces:**
- Produces:

```ts
export type Etapa = 'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'
export interface EventoTrabajo { id: string; estado: string; etapa: Etapa; agente: string; modelo: string; api: string; origen: string; intento: number; codigo_error: string; actualizado: string }
export interface Tranvia { id: string; agente: string; modelo: string; api: string; origen: string; etapa: Etapa; estado: string; intento: number; desde: number; avance: number; terminadoEn?: number }
export type Tranvias = Map<string, Tranvia>
export function aplicar(t: Tranvias, e: EventoTrabajo, ahora: number, borrados: Set<string>): void
export function limpiar(t: Tranvias, ahora: number, borrados: Set<string>): void   // borra llegadas (2 s), fallos (4 s), cancelados (2 s) y anota el id
export function posicion(tr: Tranvia): { tramo: 'entrada' | 'centro' | 'linea'; avance: number }
export function visibles(t: Tranvias, max?: number): { lista: Tranvia[]; resto: number }   // max 12, los más recientes
export function esperaReconexion(intento: number): number   // ms: 1000, 2000, 4000, 8000, 16000, 30000, 30000…
```

  `posicion`: `cola` → entrada 0.5; `preparacion` → centro; `ejecucion` → linea 0.45; `validacion` → linea 0.85;
  `llegada` → linea 1, salvo `fallido`/`interrumpido`/`cancelado`, que quedan donde iban (`aplicar` guarda el
  `avance` de la etapa anterior). `aplicar` ignora un evento de un id que está en `borrados` (Review Focus 4).

- [ ] **Step 1: Pruebas** (`vivo.test.ts`): alta y cambio de etapa; respaldo (otro `agente`, `intento` 2) cambia la línea; `posicion` de cada etapa y de un fallo en ejecución (queda en 0.45); `limpiar` a los 1999/2000 ms de una llegada y a los 3999/4000 de un fallo; evento tardío de un id borrado no vuelve; `visibles` con 15 → 12 y `resto: 3`; `esperaReconexion(0..7)` = 1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000.
- [ ] **Step 2:** `cd web && npm test` → FAIL. **Step 3:** implementar. **Step 4:** PASS.
- [ ] **Step 5:** Commit `feat(web): lógica del mapa vivo`.

### Task 5: El mapa vivo y «Siguiendo» en Inicio

**Files:**
- Create: `web/src/composables/useEventosInstancia.ts`, `web/src/componentes/SiguiendoTrabajo.vue`
- Modify: `web/src/componentes/MapaRed.vue`, `web/src/paginas/Panel.vue`, `web/src/paginas/Trabajos.vue` (abrir un trabajo pedido), `web/src/estado.ts` (`trabajoPorAbrir`), `web/src/estilos.css`, `web/src/idiomas/es.ts`/`en.ts` (`vivo.*`)

**Interfaces:**
- Consumes: `vivo.ts` (tarea 4), `/api/v1/eventos` (tarea 3).
- Produces: `useEventosInstancia(): { tranvias: Ref<Tranvias>; enVivo: Ref<boolean> }` — abre `EventSource('/api/v1/eventos')` al montar y lo cierra al desmontar; `onerror` cierra y reintenta tras `esperaReconexion(n)`; tras 3 fallos seguidos o sin `EventSource`, `enVivo=false` y llena los tranvías con los trabajos no terminados de `api.trabajos(20)` cada 5 s, sin dejar de reintentar el flujo cada 30 s; un temporizador de 500 ms llama a `limpiar`. `estado.ts`: `export const trabajoPorAbrir = ref('')`, que `Trabajos.vue` abre al montarse y vacía.
- `MapaRed.vue`: props nuevas `tranvias: Tranvia[]` (de `visibles`) y `resto: number`; emite `seguir(id)`. Cada tranvía es un `<g role="button" tabindex="0">` con `aria-label` `t('vivo.tranvia', { agente, etapa })`, un `rect` del color del agente (rojo `var(--error)` si falló, gris si se canceló) movido con CSS `offset-path` sobre el `d` del tramo (`entrada`: el camino de su estación al centro; `centro`: el punto del centro; `linea`: el camino de la línea de su agente) y `offset-distance: avance*100%`, con `transition: offset-distance 600ms` salvo `prefers-reduced-motion`. Estación de origen: la de su API o su grupo; sin API (principal, Probar, terminal, CLI), la «API principal». Varios en la misma línea se separan restando 4 % por cada uno. «+N» sobre Tramtai si `resto > 0`. La descripción del mapa suma `t('vivo.enCurso', { n })`.
- `SiguiendoTrabajo.vue`: prop `tranvia: Tranvia | undefined`; muestra API, agente y modelo, `Ruta` con el estado, tiempo desde `desde`, «Ver en el Historial» (pone `trabajoPorAbrir` y emite `ir('historial')`) y «Dejar de seguir»; región `aria-live="polite"` con la etapa. `Panel.vue` guarda el id seguido y lo pasa.

- [ ] **Step 1:** `npm test` y `npm run build` siguen en verde (la lógica ya está probada en la tarea 4); `vue-tsc` sin errores.
- [ ] **Step 2:** Navegador (copia de prueba como en la v0.52): petición desde la terminal a un agente real → el tranvía sale de «API principal», entra a Tramtai y recorre su línea; seguirlo muestra la ruta en vivo; un error (modelo que no existe) queda rojo 4 s; con respaldo, cambia de línea; al detener el servicio, el mapa reintenta y vuelve solo.
- [ ] **Step 3:** Commit `feat(web): el mapa de Inicio muestra las peticiones en curso`.

### Task 6: La portada con el recorrido completo

**Files:**
- Modify: `site/index.html` (sección `#como`: JSON de ejemplos, 4 bloques de código, leyenda, puntos y Pausa), `site/sitio.js`, `site/estilo.css`
- Create: `web/scripts/portada.test.ts`

**Interfaces:**
- Decisión: los ejemplos van como JSON dentro de `index.html` (la spec dice `sitio.js`) para que la prueba los lea sin ejecutar el sitio; `sitio.js` los toma de ahí.
- En `index.html`: `<script type="application/json" id="ejemplos-mapa">` con 4 objetos `{ "origen": 0|1|2, "agentes": [índices], "pausaCentro": bool, "es": "…", "en": "…", "codigo": "codigo-mapa-N" }` (`agentes` con dos índices = respaldo: el primero falla a mitad). Agentes por índice: 0 Claude, 1 Codex, 2 Copilot, 3 Gemini, 4 Antigravity. Ejemplos de la spec: (0 → [0]), (1 → [1], `pausaCentro`), (2 → [2]), (0 → [0, 1]).
- 4 `<pre class="codigo" id="codigo-mapa-N">`; del 2 al 4 con `hidden`. Bajo el mapa, `<div class="mapa-ejemplos" hidden>` con la leyenda (`aria-live="polite"`), 4 botones de punto y el botón Pausa/Seguir, bilingües; el JavaScript le quita `hidden`.
- `sitio.js`: arma el camino de cada ejemplo con los `d` del SVG (camino de la estación de origen + recta por Tramtai hasta `460 <y de la línea>` + camino de la línea; en `mapa-alto`, `M 18 <y origen> L 18 <y línea> L 52 <y línea>`), anima el `.tranvia` con `element.animate` sobre `offset-distance` (respaldo: hasta el 50 % de la línea 1 en rojo, vuelta a Tramtai, línea 2), cambia leyenda y código, pasa al siguiente a los 6 s, se detiene con Pausa o con `document.hidden`, y con `prefers-reduced-motion` no anima: el punto elegido deja el tranvía en la estación final. Quita la animación CSS fija de `.tranvia` (`@keyframes viaje`) cuando el JavaScript toma el control.
- `portada.test.ts`: lee `site/index.html`; el JSON tiene 4 ejemplos con `es` y `en` no vacíos y cada `codigo` existe como `id` de un `<pre>`; los agentes son índices de 0 a 4; hay un botón de pausa.

- [ ] **Step 1:** Escribir `portada.test.ts` → `npm test` FAIL. **Step 2:** implementar. **Step 3:** `npm test` → PASS.
- [ ] **Step 4:** Navegador: los 4 ejemplos, pausa, puntos, móvil (375 px), «reducir movimiento», pestaña oculta y vuelta, sin JavaScript (solo el primero) y en inglés.
- [ ] **Step 5:** Commit `feat(sitio): el recorrido de la portada llega a los agentes, con cuatro ejemplos`.

### Task 7: Ayuda, documentación y revisión

**Files:**
- Modify: `web/src/ayuda/temas/{empezar,historial,conectar}.ts`, `web/src/idiomas/ayuda.es.ts`/`ayuda.en.ts`, `site/desarrolladores.html` (sección «Seguir lo que pasa» con `curl -N`), `docs/api.md`, `docs/arquitectura.md`, `docs/equipo/pendientes.md`, `site/ayuda/*` (regenerado), `internal/recursos/web`

- [ ] **Step 1:** Preguntas nuevas (ids `tranvias`, `seguir-en-vivo`, `eventos-app`) en empezar, historial y conectar, es/en, dentro del estilo (`npm test` las comprueba).
- [ ] **Step 2:** Documentación: `docs/api.md` (ruta, foto, evento, 429), `docs/arquitectura.md` (el flujo de la instancia), `pendientes.md` (v0.53 ✅; v0.54 recorridos guiados con driver.js; v0.55 terminal a la par; v0.56 configurar clientes; v0.57 uso y diagnóstico; v0.58 actualización con vuelta atrás).
- [ ] **Step 3:** Verificación: `cd web && npm test && npm run build && npm run sitio`; `go vet ./...` y `TRAMTAI_IDIOMA=es go test ./...`; `git status` sin diferencias pendientes en `internal/recursos/web` ni `site/ayuda`.
- [ ] **Step 4:** Revisión en el navegador de la Review Focus 3 y de la Ayuda nueva.
- [ ] **Step 5:** Commit `docs: diagramas vivos en la Ayuda, la guía y la documentación`.
