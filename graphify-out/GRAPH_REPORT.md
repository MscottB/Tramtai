# Graph Report - Tramtai  (2026-10-12)

## Corpus Check
- 17 files · ~10,536 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 1 file(s) not represented in the graph (top: (none) 1)

## Summary
- 202 nodes · 618 edges · 10 communities (9 shown, 1 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 21 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- configuracion.go
- almacen.go
- adaptador.go
- m0b_test.go
- testing.T
- planificador.go
- almacen_test.go
- M0b: validación de los modos con clave de API
- solicitud_test.go
- tramtai

## God Nodes (most connected - your core abstractions)
1. `Almacen` - 21 edges
2. `Planificador` - 21 edges
3. `iniciar()` - 18 edges
4. `SolicitudEjecucion` - 15 edges
5. `abrir()` - 14 edges
6. `esperar()` - 14 edges
7. `CodigoDe()` - 13 edges
8. `Trabajo` - 13 edges
9. `opciones()` - 12 edges
10. `enviar()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `comandoConfiguracion()` --calls--> `NuevoError()`  [EXTRACTED]
  cmd/tramtai/main.go → internal/nucleo/errores.go
- `cargarConfiguracion()` --calls--> `CapaEntorno()`  [EXTRACTED]
  cmd/tramtai/main.go → internal/nucleo/configuracion.go
- `cargarConfiguracion()` --calls--> `CargarCapa()`  [EXTRACTED]
  cmd/tramtai/main.go → internal/nucleo/configuracion.go
- `cargarConfiguracion()` --references--> `Configuracion`  [EXTRACTED]
  cmd/tramtai/main.go → internal/nucleo/configuracion.go
- `cargarConfiguracion()` --calls--> `Resolver()`  [EXTRACTED]
  cmd/tramtai/main.go → internal/nucleo/configuracion.go

## Import Cycles
- None detected.

## Communities (10 total, 1 thin omitted)

### Community 0 - "configuracion.go"
Cohesion: 0.10
Nodes (23): cargarConfiguracion(), comandoConfiguracion(), main(), aplanar(), CapaEntorno(), CargarCapa(), convertir(), errorConfiguracion() (+15 more)

### Community 1 - "almacen.go"
Cohesion: 0.12
Nodes (15): CodigoError, NuevoError(), Estado, ValidarTransicion(), Trabajo, agregarEvento(), fecha(), Almacen (+7 more)

### Community 2 - "adaptador.go"
Cohesion: 0.13
Nodes (10): Simulado, Uso, Adaptador, Evento, Resultado, ConfigSalida, SolicitudEjecucion, ConfigEjecucion (+2 more)

### Community 3 - "m0b_test.go"
Cohesion: 0.21
Nodes (14): binario(), clave(), comprobarAislado(), contenido(), ejecutar(), entorno(), exigirOK(), modeloGemini() (+6 more)

### Community 4 - "testing.T"
Cohesion: 0.44
Nodes (18): CodigoDe(), TestTransicionesDeLaTabla(), abrir(), enviar(), esperar(), iniciar(), opciones(), solicitud() (+10 more)

### Community 5 - "planificador.go"
Cohesion: 0.18
Nodes (7): Nuevo(), NuevosSemaforos(), OpcionesDesde(), validarSalida(), Opciones, Planificador, Semaforos

### Community 6 - "almacen_test.go"
Cohesion: 0.26
Nodes (13): NuevoID(), Abrir(), abrir(), aceptar(), avanzar(), TestAceptacionPersisteTrasReabrir(), TestColaLlenaNoReservaClave(), TestIdempotencia() (+5 more)

### Community 7 - "M0b: validación de los modos con clave de API"
Cohesion: 0.29
Nodes (6): Cómo ejecutarlo, Cómo interpretar el resultado, M0b: validación de los modos con clave de API, Qué valida cada prueba, Resultado parcial (12-10-2026, sin claves reales), Seguridad y coste

### Community 8 - "solicitud_test.go"
Cohesion: 0.38
Nodes (4): TestEstadosTerminalesSinSalida(), solicitudValida(), TestHuellaEstableYSensible(), TestValidarSolicitud()

## Knowledge Gaps
- **1 isolated node(s):** `tramtai`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 38 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Planificador` connect `planificador.go` to `almacen.go`, `adaptador.go`, `testing.T`?**
  _High betweenness centrality (0.091) - this node is a cross-community bridge._
- **Are the 2 inferred relationships involving `iniciar()` (e.g. with `Nuevo()` and `NuevosSemaforos()`) actually correct?**
  _`iniciar()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `tramtai` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `configuracion.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10336817653890824 - nodes in this community are weakly interconnected._
- **Why does `Almacen` connect `almacen.go` to `adaptador.go`, `testing.T`, `planificador.go`, `almacen_test.go`?**
  _High betweenness centrality (0.075) - this node is a cross-community bridge._
- **Should `almacen.go` be split into smaller, more focused modules?**
  _Cohesion score 0.11614401858304298 - nodes in this community are weakly interconnected._
- **Why does `SolicitudEjecucion` connect `adaptador.go` to `solicitud_test.go`, `almacen.go`, `testing.T`, `configuracion.go`?**
  _High betweenness centrality (0.050) - this node is a cross-community bridge._