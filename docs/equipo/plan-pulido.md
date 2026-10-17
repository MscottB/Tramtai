# Pulido: plan de implementación

> **Para agentes:** SUB-SKILL REQUERIDA: superpowers:executing-plans, tarea por tarea. Los pasos usan casillas.

**Objetivo:** cuenta atrás antes de instalar, detalle de cada API en Mis APIs y todos los pendientes de las revisiones.

**Arquitectura:** la cuenta atrás vive en el servicio (fase `listo` con `instala_en` y dos rutas), separada de la
descarga para poder probarla con un instalador simulado. Mis APIs gana un diálogo de detalle que reutiliza los
ejemplos de la API principal con una función común. El resto son arreglos acotados en persistencia, planificador,
actualización y Web UI.

**Stack:** Go 1.27, SQLite (modernc), Vue 3 + Quasar (vue-tsc), HTML estático (`site/`).

**Spec:** docs/equipo/pulido.md

## Restricciones globales

- Cuenta atrás de 10 s. Rutas `POST /ui/actualizar/instalar` y `POST /ui/actualizar/cancelar` con sesión local y
  CSRF; 409 `solicitud_invalida` fuera de la fase `listo`.
- Las claves de las APIs nunca se leen: solo se muestran al crear o regenerar.
- Textos en `es.ts` y `en.ts` con las mismas claves; mensajes Go nuevos traducidos en `internal/idioma`.
- Tras Go: `go vet ./...` y `go test ./...`; tras la Web UI: `cd web && npm run build`.
- Los Minor que encuentre la revisión final se arreglan, no se aplazan.

## Puntos de revisión

- **Cancelar justo cuando vence la cuenta atrás:** gana el primero; nunca se instala tras un cancelar confirmado ni
  queda la fase colgada. Prueba en la tarea 1.
- **Regenerar la clave de una API desde el detalle** invalida la anterior: el aviso lo dice antes de confirmar.
  Se verifica en el navegador (tarea 5).
- **Consumo huérfano:** nunca se borran las filas especiales `''`, `*` ni `~`. Prueba en la tarea 3.
- **Purga con corte por OFFSET** con menos terminados que el tope: no borra nada. Prueba en la tarea 2.
- **Plurales** con 0, 1 y 2. Se verifica en el navegador (tarea 6).

---

### Tarea 1: Cuenta atrás antes de instalar

**Archivos:** `internal/servicio/actualizacion.go`, `internal/servicio/web.go` (rutas junto a `/ui/actualizar`),
`internal/servicio/actualizacion_test.go` (crear si no existe), `web/src/App.vue`, `web/src/api.ts`, `es.ts`, `en.ts`.

**Interfaces:**
- `ProgresoActualizacion.InstalaEn *time.Time` (`json:"instala_en,omitempty"`); fase nueva `"listo"`.
- `var instalar = actualizacion.Instalar` (para sustituirlo en la prueba) y `var esperaInstalacion = 10 * time.Second`.
- `func (s *Servicio) esperarEInstalar(v actualizacion.Version, instalador, dir, carpeta string)`: fija `listo` e
  `InstalaEn`, espera el primero de: el plazo, `s.ordenInstalar` (`chan bool`: true instala, false cancela); al
  cancelar borra `dir` y deja la fase vacía; al instalar pone `instalando` y llama a `instalar`.
- `func (s *Servicio) decidirInstalacion(instalarYa bool) error`: 409 si la fase no es `listo`; si no, envía la orden
  sin bloquear (canal con búfer 1, creado al entrar en `listo`).

- [ ] **Paso 1: Prueba que falla** (`TestCuentaAtrasInstalar`): con `esperaInstalacion = 50 ms` e `instalar`
  sustituido por un contador:
  - sin orden: tras 100 ms se instaló una vez y la fase es `instalando`;
  - con `decidirInstalacion(true)` al momento: instala antes del plazo;
  - con `decidirInstalacion(false)`: no instala, la carpeta ya no existe y la fase es `""`;
  - `decidirInstalacion` fuera de `listo` devuelve `CodigoSolicitudInvalida`.
- [ ] **Paso 2:** `go test ./internal/servicio -run CuentaAtras` → FAIL (no definido).
- [ ] **Paso 3:** implementar; `actualizar` llama a `esperarEInstalar` en vez de `actualizacion.Instalar`. Rutas en
  `web.go` con `sesionLocal` + `csrfValido`; 204.
- [ ] **Paso 4:** Web UI: `api.instalarYa()` y `api.cancelarActualizacion()`; en `App.vue`, la fase `listo` muestra
  «Tramtai se reiniciará en {n} s» con un contador local de `instala_en` y los botones **Reiniciar ahora** y
  **Cancelar**; `persistent` solo fuera de `listo` y `error`. Claves `actualizacion.reiniciaraEn`,
  `actualizacion.reiniciarAhora`.
- [ ] **Paso 5:** `go test ./internal/servicio` y `npm run build` → PASS. Commit
  `feat(actualizacion): cuenta atrás antes de instalar, con reiniciar ahora o cancelar`.

### Tarea 2: Persistencia: corte por OFFSET, cuenta del tope y compactación con registro

**Archivos:** `internal/persistencia/almacen.go`, `internal/persistencia/historial_test.go`,
`internal/servicio/servicio.go` (`abrirInstancia`: aviso de compactación).

**Interfaces:** `Almacen.Compactada bool` (true si `Abrir` convirtió la base); el servicio escribe
«compactando la base de <instancia>» en su registro cuando lo ve.

- [ ] **Paso 1: Pruebas que fallan:** en `TestPurgarConTope`, `eliminados == 3`; caso nuevo con 2 terminados y
  `Maximo: 5` → no borra nada; en `TestAutoVacuumIncremental`, `Compactada` es true en la base vieja y false al
  reabrirla.
- [ ] **Paso 2:** `go test ./internal/persistencia` → FAIL.
- [ ] **Paso 3:** el tope calcula `corte` con `SELECT terminado_en … ORDER BY terminado_en DESC LIMIT 1 OFFSET ?`
  (Maximo − 1); sin fila, nada; si no, borra idempotencia, eventos y trabajos con `terminado_en < corte` y suma a
  `eliminados`. `compactable` devuelve si convirtió y hace `PRAGMA wal_checkpoint(TRUNCATE)` tras el `VACUUM`.
- [ ] **Paso 4:** `go test ./internal/persistencia ./internal/servicio` → PASS. Commit
  `fix(persistencia): tope con corte por fecha, cuenta y compactación registrada`.

### Tarea 3: Uso de intentos fallidos, consumo huérfano y errores de purga

**Archivos:** `internal/planificador/planificador.go` (bucle de candidatos, línea ~545),
`internal/persistencia/consumo.go` (`PuntosConConsumo`), `internal/servicio/puntos.go` (`Recargar`),
`internal/servicio/web.go` (`fijarRetencion`), pruebas en cada paquete.

**Interfaces:** `func (a *Almacen) PuntosConConsumo(ctx, instancia string) ([]string, error)` (distintos de `''`,
`*` y `~`); `Planificador.PuntosConConsumo(ctx)` lo expone.

- [ ] **Paso 1: Pruebas que fallan:**
  - planificador: un adaptador que falla con tokens informados (`limite_uso_proveedor`, 10→0) y un respaldo que
    responde (5→3): el trabajo completado tiene `tokens_entrada 15` y `tokens_salida 3`;
  - persistencia: `PuntosConConsumo` con consumo en `tienda`, `''`, `*` y `~` devuelve solo `tienda`;
  - servicio: consumo de una API `vieja` que no está en la configuración → tras `Recargar`, `Consumo` de `vieja` es
    cero.
- [ ] **Paso 2:** ejecutar → FAIL.
- [ ] **Paso 3:** sumar el `Uso` de cada intento fallido (si el adaptador lo devolvió) al del intento que termina,
  también cuando el trabajo termina fallido; `Recargar` borra el consumo de las APIs ausentes de cada instancia;
  `fijarRetencion` lanza la purga con su error al registro del servicio.
- [ ] **Paso 4:** `go test ./...` → PASS. Commit `fix: uso de intentos fallidos, consumo huérfano y errores de purga`.

### Tarea 4: Novedades con caché y diálogo inmediato

**Archivos:** `internal/actualizacion/actualizacion.go` (+ prueba), `internal/servicio/web.go` (mensaje fijo),
`web/src/App.vue`, `es.ts`, `en.ts`.

- [ ] **Paso 1: Prueba que falla** (`TestNovedadesCache`): dos llamadas a `Novedades` dentro de 10 minutos hacen una
  sola consulta al servidor de prueba; tras `reiniciarCache()` (solo para la prueba) vuelven a consultar.
- [ ] **Paso 2:** FAIL. **Paso 3:** caché en el paquete (`sync.Mutex`, lista y hora); el comentario del paquete
  menciona la lista; la ruta responde 502 con `idioma.T("GitHub no está disponible")`.
- [ ] **Paso 4:** Web UI: el diálogo se abre al detectar la versión nueva con un `q-spinner` mientras carga; con una
  sola versión distinta de la instalada, su número encima de las notas; `aria-labelledby` con un `id` en el título de
  cada `q-dialog` con título de `App.vue`.
- [ ] **Paso 5:** `go test ./internal/actualizacion ./internal/servicio` y `npm run build` → PASS. Commit
  `fix(actualizacion): novedades con caché y diálogo inmediato`.

### Tarea 5: Mis APIs: detalle de cada API

**Archivos:** `web/src/paginas/Accesos.vue`, borrar `web/src/componentes/MenuAcciones.vue`, `web/src/api.ts`
(`ejemplosConexion`), `es.ts`, `en.ts`.

**Interfaces:** `export function ejemplosConexion(base: string, clave: string, modelo: string): { id: string; titulo: string; codigo: string }[]`
en `api.ts`, usada por la API principal y por el detalle (sale de los ejemplos actuales de `Accesos.vue`).

- [ ] **Paso 1:** fila: el botón «Ajustes de la API» y `MenuAcciones` se cambian por **Conectar**; la fila entera
  abre el detalle (`@click` con `cursor-pointer`; el botón con `@click.stop`).
- [ ] **Paso 2:** diálogo de detalle (`maximized` en `$q.screen.lt.sm`): cabecera con nombre, estado y grupo;
  **Conectar** (URLs según `p.contratos`, clave con el aviso y Regenerar clave con confirmación, ejemplos con
  `ejemplosConexion(p.url…, claveVista || '<clave de ' + p.nombre + '>', modelo)`); **Acciones** (Editar ajustes,
  Abrir en Probar, Ver peticiones, Activar/Detener); **Zona de peligro** (Eliminar con confirmación). Las acciones
  reutilizan las funciones de `accionesDe/ejecutarAccion`, que dejan de ser un arreglo paralelo.
- [ ] **Paso 3:** `crear` abre el detalle de la API nueva con su clave, en vez del aviso.
- [ ] **Paso 4:** la barra de filtros se ve si `puntos.length > 1 || busqueda || filtroGrupo !== null`; en
  `$q.screen.lt.sm`, «Activar todas» y «Detener todas» son botones de icono con `aria-label`.
- [ ] **Paso 5:** `npm run build`; navegador: detalle de `tienda` (URLs, Regenerar clave y ejemplo con la clave,
  acciones, Eliminar con confirmación sobre una API de prueba), Nueva API abre el detalle, móvil y escritorio, claro y
  oscuro. Commit `feat(web): detalle de cada API con cómo conectarla y sus acciones`.

### Tarea 6: Web UI: el resto de pendientes

**Archivos:** `web/src/paginas/Trabajos.vue`, `Configuracion.vue`, `Ejecutar.vue`, `Ayuda.vue`, `App.vue`,
`web/src/componentes/MapaRed.vue`, `web/src/estilos.css`, `es.ts`, `en.ts`.

- [ ] **Paso 1:** «Ver más» con `const n = pedidoLista` y `enVuelo++/--`: descarta la respuesta si cambió el filtro.
- [ ] **Paso 2:** Ajustes · Historial: días con `step="0.5"` y horas = `Math.round(dias * 24)`; si un campo está
  vacío, aviso `historialAjustes.completa` y no se envía; los días se muestran con `horas_metadatos / 24` sin redondear.
- [ ] **Paso 3:** plurales: claves `…Uno` para 1 (`mapa.grupoUno`, `mapa.activaUno`, `inicio.tableroPeriodoUno`,
  `inicio.peticionesTextoUno`; `puntos.usoUno` ya existe) elegidas con `n === 1`; probar 0, 1 y 2 en el navegador.
- [ ] **Paso 4:** mapa móvil: el texto de origen añade « · {activas}» si hay `detalle`.
- [ ] **Paso 5:** logo: `aria-label` «Tramtai, ir a Inicio» (clave `app.irInicio` cambia su texto); Probar sin el
  doble relleno; borrar `.ayuda-campo` y `puntos.activar`; `ayuda.paso5Texto` dice Windows, macOS y Linux.
- [ ] **Paso 6:** `npm run build` y navegador. Commit `fix(web): pendientes de Historial, Ajustes, mapa y textos`.

### Tarea 7: Página, documentación y cierre

**Archivos:** `site/index.html`, `site/sitio.js`, `site/ayuda.html`, `docs/operacion.md`,
`docs/equipo/pendientes.md`, capturas si cambia Mis APIs.

- [ ] **Paso 1:** página: `alt` en inglés con `data-en-alt` (y `sitio.js` lo aplica como los `data-en`); los
  `alt` de Mis APIs describen la vista nueva; títulos sin saltos de nivel; `<meta name="theme-color">` con
  `media="(prefers-color-scheme: dark)"` además del claro.
- [ ] **Paso 2:** capturas de Mis APIs (claro y oscuro) rehechas con la fila nueva; README si la usa.
- [ ] **Paso 3:** `operacion.md`: cuenta atrás y detalle de cada API. `pendientes.md`: se vacían las listas de
  pendientes de revisiones (quedan webhooks y lo que necesita una decisión), se quita la tarea del arranque en
  macOS/Linux y se añade la fila del release.
- [ ] **Paso 4:** `go vet ./...`, `go test ./...`, `cd escritorio && go vet ./...`, `npm run build`,
  `graphify update .` → todo verde. Commit `docs: cuenta atrás, detalle de cada API y pendientes cerrados`.
