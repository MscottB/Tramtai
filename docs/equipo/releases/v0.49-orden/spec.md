# Orden del código (v0.49)

Diseño del 2026-10-19. La revisión completa del 2026-10-19 (`releases/v0.47-correcciones/spec.md`) dejó para este release lo que es
estructura, pruebas, CI y rendimiento: lo que no falla hoy, pero hace el código más difícil de leer y de cambiar. Las
correcciones de errores reales fueron la v0.47 y la portable la v0.48.

Decisiones del usuario:
- **Hoja de ruta** (2026-10-19), antes de más agentes:
  1. v0.49, este orden del código;
  2. v0.50, archivos: markitdown incluido con Tramtai, avisos y el recorrido de un archivo por cada API;
  3. v0.51, herramientas (`tools`) y guía del desarrollador;
  4. v0.52, Ayuda renovada en la app y en la web;
  5. v0.53, diagramas vivos;
  6. v0.54, la terminal a la par de la Web UI y Tramtai solo terminal en Windows;
  7. v0.55, configurar los clientes solos;
  8. v0.56, uso y diagnóstico;
  9. v0.57, actualización con vuelta atrás;
  10. después, OpenCode y Hermes Agent.
- **La licencia sigue igual** (PolyForm Noncommercial) y una versión Pro o Team queda para más adelante, sin fecha.

Este release no cambia lo que ve quien usa Tramtai, salvo tres cosas:
- Agentes y `/api/v1/capacidades` responden antes;
- la app de escritorio pesa menos;
- `tramtai acceso` inicia el servicio si no está en marcha, como ya hace `tramtai ejecutar`.

Fuera de alcance:
- **Pantallas nuevas en la terminal:** son de la v0.54. Aquí la terminal solo cambia por dentro (§2 y §3), y eso es lo
  que deja preparada la v0.54.
- **Cambiar el formato de la API o de `config.yaml`:** todo lo que hoy funciona sigue igual.
- **Reescribir lo que ya está bien:** solo se divide lo que tiene varias responsabilidades en un archivo.

## 1. Contrato ligero entre cliente y servicio

**Problema:** `internal/cliente` usa los tipos de `internal/servicio` (`Estado`, `EstadoInstancia`, `EstadoPunto`,
`ActivacionInstancia`, `EntradaConfiguracion`, `Descriptor`). Como Go enlaza paquetes enteros, la app de escritorio,
que solo habla con el servicio por HTTP, lleva dentro el servicio completo: el planificador, SQLite, las fachadas de
API y todos los adaptadores (`go list -deps ./escritorio` lo muestra).

**Cambio:**
- un paquete nuevo, `internal/contrato`, con los tipos que viajan entre el servicio y sus clientes y sin dependencias
  del servicio. `servicio` los usa con alias (`type EstadoPunto = contrato.EstadoPunto`), así que su código no cambia;
- `cliente` y `escritorio` importan `contrato` en lugar de `servicio`. `LeerDescriptor` pasa a `contrato` porque solo
  lee un JSON;
- una prueba de dependencias falla si `cliente` o `escritorio` vuelven a enlazar `servicio`, `planificador`,
  `persistencia` o `modernc.org/sqlite` (`go list -deps`).

**Se mide:** el tamaño de `tramtai-escritorio.exe` antes y después, y va en el PR.

## 2. La CLI y la terminal sobre la API del servicio

**Problema:** `tramtai acceso` (`cmd/tramtai/acceso.go`) y la terminal (`internal/tui`) editan `config.yaml` y los
tokens directamente con `servicio.EditarPuntos`, `EliminarPunto` y `RotarTokenPunto`, y después piden recargar. La
Web UI hace lo mismo por HTTP (`/api/v1/puntos`). Son dos caminos para lo mismo: en la v0.47 hubo que igualar sus reglas
a mano (`ConReglas`), y la Web pone a cero el consumo de una API borrada, pero la CLI y la terminal no.

**Cambio:**
- `cliente` gana `Puntos`, `CrearPunto`, `EditarPunto`, `EliminarPunto`, `RotarTokenPunto` y `ActivarPunto` (ya
  existe), sobre las rutas que usa la Web UI. Esas rutas ya aceptan el token de servicio;
- `tramtai acceso` y la terminal usan solo `cliente`. Si el servicio no está en marcha, lo inician en segundo plano
  (`cliente.IniciarEnSegundoPlano`), como `tramtai ejecutar`;
- las reglas (`ConReglas`, `ValidarPunto`) y la limpieza del consumo quedan solo en el servicio. Los tres clientes
  reciben los mismos errores con los mismos códigos;
- la CLI y la terminal dejan de importar `servicio`, salvo `tramtai servicio`, que es el propio servicio.

**Lo que no cambia:** las opciones y la salida de `tramtai acceso`, y lo que muestra la terminal.

## 3. Archivos grandes

Se dividen por responsabilidad, en el mismo paquete y sin cambiar nombres públicos. Lo que se mueve conserva sus
comentarios, su manejo de errores y sus validaciones.

| Archivo | Líneas | Queda en |
|---|---|---|
| `internal/tui/tui.go` | 1344 | `tui.go` (modelo, `Update`, navegación), `vista.go` (dibujo y estilos), `inicio.go`, `ejecutar.go`, `trabajos.go`, `accesos.go` (una pantalla o grupo de pantallas cada uno) |
| `internal/planificador/planificador.go` | 1249 | `planificador.go` (crear, cola, semáforos), `ejecucion.go` (lanzar un trabajo y sus eventos), `respaldo.go` (cadena de respaldo y esfuerzo), `entrada.go` (medida y tope de la entrada), `presupuestos.go` |
| `internal/servicio/web.go` | 1026 | `web.go` (sesión, cookie, CSRF), `web_servicio.go` (puerto, reiniciar, detener, autoarranque), `web_actualizacion.go`, `web_instancia.go` (`/api/v1/instancia` y `/api/v1/puntos`), `web_estaticos.go` |
| `internal/persistencia/almacen.go` | 912 | por tabla: `trabajos.go`, `eventos.go`, `consumo.go`, `modelos.go`, `purga.go` |
| `internal/servicio/servicio.go` | 906 | `servicio.go` (arranque y cierre), `instancias.go` (activar, desactivar, estado), `escritura.go` (`escribirAtomico`) |
| `web/src/api.ts` | 885 | `api/cliente.ts` (peticiones, sesión, CSRF), `api/tipos.ts`, `estado.ts` (refs compartidas), `recetas.ts`, `formato.ts` (fechas, duraciones, estados), `agentes.ts` (fichas, modelos, esfuerzo) |
| `web/src/paginas/Accesos.vue` | 749 | la página y tres componentes: lista con filtros, detalle de una API y formulario |
| `web/src/App.vue` | 612 | la estructura y composables: `useSesion`, `useSondeo` (estado en curso), `useActualizacion` |

En `web.go`, el `switch` de rutas pasa a una tabla de rutas del `http.ServeMux` (método y ruta, como `admin.go`), con
los mismos permisos por ruta. Las pruebas de `web_test.go` siguen pasando sin cambios.

Quienes importan `api.ts` (23 archivos) pasan a importar del módulo que usan. No queda un `api.ts` que lo reexporte todo.

## 4. Rendimiento

- **`/api/v1/capacidades`:** hoy detecta los agentes uno tras otro, y cada `Detectar` puede tardar hasta 10 s. Pasa a
  detectarlos a la vez, con el mismo plazo por agente y el resultado en el mismo orden.
- **Modelos efectivos:** el planificador consulta los 500 últimos trabajos (`ModelosEfectivos`) cada vez que crea uno
  (`versionVista`), y las fachadas lo hacen en cada `/models`. Pasa a un mapa en memoria del planificador, que se carga
  de la base una vez y se actualiza cuando un trabajo informa su modelo efectivo. *(Al implementarlo se usó un índice:
  ver «Cambios respecto al plan» en `plan.md`.)*
- Se mide antes y después: el tiempo de `/api/v1/capacidades` con los agentes instalados y el de crear 100 trabajos.

## 5. Pruebas que faltan

- **Codex** (`internal/agentes/codex` no tiene pruebas):
  - los argumentos de `codex exec` (solo lectura, sin red, modelo y esfuerzo);
  - la lectura de sus eventos JSONL con un CLI simulado (texto, uso de tokens, modelo efectivo y error);
  - la autenticación (`ModoAutenticacion`).
- **Claude:** las mismas tres comprobaciones donde falten hoy, con un CLI simulado.
- **Web UI:** `node --test` para lo que el §3 separa en módulos sin Vue: `recetas.ts`, `formato.ts`, `agentes.ts`
  (opciones de modelo, esfuerzos, orden) y la renovación de la sesión de `api/cliente.ts`.
- **Contrato:** la prueba de dependencias del §1.

## 6. CI

- **CI reutilizable:** `ci.yml` admite `workflow_call`, y Publicar lo llama en lugar de repetir sus pruebas
  (`pruebas-windows` y `pruebas-unix`). Los checks de los PR conservan sus nombres, así que el ruleset de `main` no cambia.
  Lo que solo hace Publicar (compilar, firmar, empaquetar) se queda en `publicar.yml`.
- **Dependabot** (`.github/dependabot.yml`), semanal:
  - `gomod` en `/` y en `/escritorio`, con Wails fuera (va fijado en una beta con `replace`);
  - `npm` en `/web`;
  - `github-actions`;
  - actualizaciones menores y de parche agrupadas en un PR por ecosistema. Pasan por los mismos checks que cualquier PR.

## 7. Documentación del equipo

- **`docs/equipo/README.md`:** un índice con:
  - lo que vale siempre: `desarrollo.md`, `versionado.md`, `nuevo-agente.md`, `m0-agentes.md`, `M0B.md` y
    `pendientes.md`;
  - y cada release con su spec y su plan.
- **Specs y planes por release:** pasan a `docs/equipo/releases/<versión>-<tema>/` (`spec.md` y `plan.md`). Se mueven
  con `git mv` para conservar su historial, y se actualizan los enlaces que apuntan a ellos.
- **`docs/arquitectura.md`:** el contrato (§1) y que la CLI, la terminal, la Web UI y la app de escritorio hablan con el
  servicio solo por HTTP.
- **Ayuda, README y sitio:** solo cambia que `tramtai acceso` inicia el servicio si hace falta. Se dice donde hoy se
  explica `tramtai acceso`.

## 8. Comprobación

- `go vet`, `go test ./...` en los dos módulos, `npm test` y la Web UI compilada igual que en el CI.
- **A mano**, con una carpeta de datos aislada:
  - `tramtai acceso crear|editar|eliminar|token` con el servicio parado y en marcha;
  - borrar y volver a crear una API con el mismo nombre desde la CLI: empieza con consumo cero;
  - la terminal: crear, editar y borrar una API, y una ejecución con Claude haiku;
  - la app de escritorio: bandeja, Mis APIs y abrir la Web UI;
  - la página Agentes, con los tiempos del §4.

Notas de versión: cortas, para quien usa Tramtai. Contarán que Agentes responde antes, que la app pesa menos y que
`tramtai acceso` inicia el servicio solo.
