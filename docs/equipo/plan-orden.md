# Orden del código (v0.49): plan de implementación

**Objetivo:** que el código sea más fácil de leer y de cambiar sin que cambie lo que hace. Es la base de la v0.54
(la terminal a la par de la Web UI).

**Spec:** `docs/equipo/orden.md`, aprobada. **Rama:** `feat/orden`, con PR a `main`.

## Restricciones generales

- **Sin cambios de comportamiento.** Las únicas excepciones son las tres de la spec:
  - capacidades más rápido;
  - la app de escritorio más ligera;
  - `tramtai acceso` inicia el servicio si no está en marcha.
- **Mover sin reescribir.** Lo que cambia de archivo conserva sus comentarios, su manejo de errores y sus validaciones.
  Un commit que solo mueve código no mezcla cambios de lógica; así la revisión puede ver qué se movió.
- **Nombres públicos iguales,** salvo lo que pasa a `contrato` (tarea 1), que conserva alias en `servicio`.
- **Textos nuevos:**
  - en `es.ts`/`en.ts`;
  - y en `internal/idioma/en_*.go`, que comprueba `TestCobertura`.
- **Comprobaciones en cada tarea:**
  - `go vet ./...` en Windows, con `GOOS=linux` y con `GOOS=darwin`;
  - `go test ./...`;
  - `go test -race` en los paquetes que toca la tarea;
  - `cd escritorio && go vet ./... && go test ./...`;
  - `cd web && npm run build && npm test`, y la Web UI compilada igual que la del repositorio;
  - `graphify update .`.
- **Pruebas reales:**
  - solo con Claude haiku;
  - con una carpeta de datos aislada (`TRAMTAI_CONFIG_GLOBAL` y una portable de prueba), nunca con el `%APPDATA%` real.
- **Release:** PR, revisión, docs, CI en verde, `gh pr merge`, etiqueta. Notas cortas y sin detalles internos.

## Dónde revisar con más cuidado

1. **`PUT /api/v1/puntos/{nombre}` (tarea 2):**
   - aplica las mismas reglas que crear (`Normalizar`, `ValidarPunto`);
   - no deja cambiar el nombre ni mover el punto a otra instancia;
   - solo se usa con la sesión de la Web UI o el token de servicio, nunca con el token de una app.
2. **`web.go` con tabla de rutas (tarea 5):** cada ruta conserva su permiso:
   - pública;
   - con sesión y CSRF;
   - o solo con sesión local.

   `web_test.go` pasa sin cambios.
3. **El mapa de modelos efectivos (tarea 6):** sin carreras (`-race`), y lo mismo que devolvía la consulta.
4. **La CLI y la terminal (tareas 3 y 4):** los mismos errores y la misma salida que hoy, también con `--local`.

---

### Tarea 1: El contrato entre cliente y servicio

**Archivos:**
- nuevos: `internal/contrato/contrato.go`, `internal/contrato/contrato_test.go`;
- cambian: `internal/servicio/{servicio,admin,puntos}.go` (alias), `internal/cliente/cliente.go`,
  `escritorio/*.go`, y quien llama a los métodos de `ConfigPunto`.

- [ ] **Antes de tocar nada:** compilar `tramtai-escritorio.exe` y anotar su tamaño para el PR.
- [ ] **`contrato`:**
  - mueve sin cambios `Estado`, `EstadoInstancia`, `EstadoPunto`, `ActivacionInstancia`, `EntradaConfiguracion`,
    `ConfigPunto`, `EjecucionPunto`, `Descriptor`, `RutaDescriptor` y `LeerDescriptor`;
  - mueve también `NombrePuntoValido` (con `reNombrePunto`) y `ConfigPunto.EstaActivo`, que no dependen del servicio;
  - solo puede importar `nucleo` e `idioma`.
- [ ] **En `servicio`:**
  - alias para cada tipo (`type EstadoPunto = contrato.EstadoPunto`) y variables para las funciones movidas;
  - `ConfigPunto.Normalizar` y `politica` pasan a funciones `normalizar(p, instancia, puerto)` y `politica(p)`,
    porque un alias no admite métodos nuevos. Se actualizan sus llamadas.
- [ ] **`cliente` y `escritorio`** importan `contrato` y dejan de importar `servicio`.
- [ ] **`TestDependencias`** (en `contrato_test.go`): con `go list -deps`, ni `tramtai/internal/cliente` ni el módulo
  `escritorio` enlazan `servicio`, `planificador`, `persistencia`, `api` ni `modernc.org/sqlite`. Se salta si no hay
  `go` en el PATH.
- [ ] **Después:** compilar de nuevo `tramtai-escritorio.exe` y anotar el tamaño.
- [ ] Commit: `refactor: contrato ligero entre el servicio y sus clientes`.

### Tarea 2: Rutas completas de puntos en el servicio

**Archivos:** `internal/servicio/web.go`, `web_test.go`, `internal/cliente/cliente.go`, `cliente_test.go` (nuevo si
no existe).

La Web UI ya usa `POST /api/v1/puntos`, `PATCH`/`DELETE /api/v1/puntos/{nombre}` y `POST …/token`. El `PATCH` solo
cubre lo que edita la Web UI. `tramtai acceso editar` cambia además:
- el puerto o la ruta;
- el grupo, la instancia, los contratos y los alias;
- el agente, el modelo y el perfil tal cual;
- la ejecución.

- [ ] `GET /api/v1/puntos/{nombre}/config`: devuelve el `ConfigPunto` tal como está guardado.
- [ ] `PUT /api/v1/puntos/{nombre}`: reemplaza el `ConfigPunto` con el del cuerpo y responde con su `EstadoPunto`.
  - Rechaza un nombre distinto del de la ruta.
  - La instancia se fija igual que en `crearPuntoUI`.
  - Valida con `normalizar` y `validarPunto(…, s.existe)` antes de escribir, dentro de `EditarPuntos`, y después recarga.
  - `shortcut:` sin control de versión, así que dos ediciones del mismo punto a la vez se pisan, la última gana. Pasar a
    `If-Match` si la terminal y la Web UI llegan a editar el mismo punto a menudo.
- [ ] Las dos rutas exigen la sesión de la Web UI o el token de servicio (`privilegiada`), como las demás de `/api/v1/puntos`.
- [ ] **`cliente`:**
  - `Puntos(ctx, inst)`;
  - `ConfigDePunto(ctx, inst, nombre)`;
  - `CrearPunto(ctx, inst, p) (contrato.EstadoPunto, string, error)` (el token, que solo se ve una vez);
  - `ReemplazarPunto(ctx, inst, p)`;
  - `EliminarPunto(ctx, inst, nombre)`;
  - `RotarTokenPunto(ctx, inst, nombre) (string, error)`.
- [ ] **Pruebas:**
  - el `PUT` con un nombre distinto da 400;
  - el `PUT` con Antigravity en un punto para apps, o con un respaldo que no existe, da el mismo error que crear;
  - el `PUT` con el token de una app da 401;
  - borrar por la API y volver a crear con el mismo nombre empieza con consumo cero;
  - `cliente` contra un servicio de prueba en cada método.
- [ ] Commit: `feat(servicio): editar un punto entero por la API`.

### Tarea 3: `tramtai acceso` sobre el cliente

**Archivos:** `cmd/tramtai/acceso.go` y sus pruebas.

- [ ] **Conectarse:** una función `clienteAcceso(cmd)` que conecta con el servicio o lo inicia en segundo plano
  (`cliente.Conectar` y, si da `ErrServicioInactivo`, `cliente.IniciarEnSegundoPlano`, como `tramtai ejecutar`), y
  devuelve la instancia.
  - Con `--local`, activa la del proyecto con `cli.Activar("proyecto", raíz)`, como hace hoy la terminal.
  - Sin `--local`, usa `global`.
- [ ] **Cada subcomando:**
  - `listar` → `Puntos`;
  - `crear` → `ConfigPunto` con las banderas y `CrearPunto`;
  - `editar` → `ConfigDePunto`, las banderas (`aplicar`, sin cambios) y `ReemplazarPunto`;
  - `activar` y `desactivar` (también `--grupo`) → `ActivarPunto`;
  - `eliminar` → `EliminarPunto`;
  - `token --rotar` → `RotarTokenPunto`;
  - `reiniciar-presupuesto` → el método que ya existe.
- [ ] **Validación local:** `aplicar` deja de llamar a `Normalizar`, porque ahora valida el servicio; un error de
  validación llega con su código y su mensaje.
- [ ] `acceso.go` deja de importar `servicio`.
- [ ] **Pruebas:** las de `acceso` pasan contra un servicio de prueba. Se añade una de `crear` con el servicio parado
  que comprueba que se inicia.
- [ ] Commit: `refactor(cli): tramtai acceso habla con el servicio por su API`.

### Tarea 4: La terminal sobre el cliente

**Archivos:** `internal/tui/tui.go` (todavía sin dividir) y `tui_test.go`.

- [ ] `editarPuntos`, `teclaAccesos` (activar y eliminar), `regenerarClave` y `crearPunto` usan `CrearPunto`,
  `ActivarPunto`, `EliminarPunto` y `RotarTokenPunto` del cliente. Desaparecen `rutaConfig` y las llamadas a
  `servicio.EditarPuntos`, `EliminarPunto` y `RotarTokenPunto`.
- [ ] La terminal usa `contrato` y deja de importar `servicio`.
- [ ] **Pruebas:** las de la terminal pasan. Se añade la de borrar y volver a crear con el mismo nombre (consumo cero).
- [ ] Commit: `refactor(tui): la terminal edita las APIs por la API del servicio`.

### Tarea 5: Dividir los archivos grandes de Go

Cada archivo en su commit (`refactor(<paquete>): dividir <archivo>`). Solo se mueve código; los nombres de destino son
los de la spec (§3).

- [ ] `internal/tui/tui.go` → `tui.go`, `vista.go`, `inicio.go`, `ejecutar.go`, `trabajos.go`, `accesos.go`.
- [ ] `internal/planificador/planificador.go` → `planificador.go`, `ejecucion.go`, `respaldo.go`, `entrada.go`,
  `presupuestos.go`.
- [ ] `internal/persistencia/almacen.go` → `almacen.go` (abrir, migrar, cerrar), `trabajos.go`, `eventos.go`,
  `consumo.go`, `modelos.go`, `purga.go`.
- [ ] `internal/servicio/servicio.go` → `servicio.go`, `instancias.go`, `escritura.go`.
- [ ] `internal/servicio/web.go` → `web.go`, `web_servicio.go`, `web_actualizacion.go`, `web_instancia.go`,
  `web_estaticos.go`.
- [ ] **Tabla de rutas de `web.go`,** en un commit aparte (`refactor(servicio): rutas de la Web UI en una tabla`):
  - el `switch` de `/ui/…` y el de `rutaInstancia` pasan a una lista `rutaUI{metodo, ruta, permiso, atender}`;
  - `permiso` es `publica`, `sesion` (con CSRF en lo que no es GET) o `local`;
  - un solo envoltorio comprueba el permiso antes de `atender`, con los mismos errores de hoy;
  - `web_test.go` pasa sin cambios, y se añade una prueba que recorre la tabla: cada ruta con `sesion` o `local`
    responde 401 sin sesión.
- [ ] Al terminar, ningún archivo de Go que no sea de pruebas pasa de 700 líneas. Se comprueba con `wc -l`.

### Tarea 6: Rendimiento

**Archivos:** `internal/api/nativa.go`, `internal/planificador/{planificador,ejecucion}.go`,
`internal/api/openai.go` y sus pruebas.

- [ ] **Medir antes:** el tiempo de `GET /api/v1/capacidades` con los agentes de este equipo, y el de crear 100
  trabajos con un agente simulado (`BenchmarkCrear`). Se anota para el PR.
- [ ] **`capacidades`:** un `sync.WaitGroup` lanza cada `Detectar` a la vez, con su plazo de 10 s y su propio contexto,
  y cada uno escribe en su posición de `lista`, así que el orden no cambia.
  - **Prueba:** tres agentes simulados que tardan 300 ms cada uno; la respuesta llega en menos de 600 ms y en orden
    alfabético.
- [ ] **Modelos efectivos en memoria:**
  - el planificador guarda `efectivos map[string]string` con un `sync.RWMutex`;
  - lo carga con `almacen.ModelosEfectivos` la primera vez que lo necesita (`sync.Once`);
  - cuando un trabajo informa su modelo efectivo, el planificador actualiza la entrada `agente/modelo enviado`;
  - `versionVista`, `ModelosEfectivos` y las fachadas (`nativa.go:559`, `openai.go:544`) leen del mapa;
  - `ModelosEfectivos` devuelve una copia;
  - **prueba:** el mapa coincide con la consulta después de varios trabajos, con `-race`.
- [ ] **Medir después** y anotar.
- [ ] Commit: `perf: capacidades en paralelo y modelos efectivos en memoria`.

### Tarea 7: Pruebas de Codex y Claude

**Archivos:**
- nuevos: `internal/agentes/codex/codex_test.go`, `codex/testdata/*.jsonl`;
- cambian: `internal/agentes/claude/claude_test.go`, más `claude/testdata/*` si hace falta.

Mismo patrón que `copilot_test.go`: el binario de pruebas hace de CLI falso, apunta lo que recibe y responde con las
muestras de `testdata`.

- [ ] **Codex:**
  - los argumentos de `codex exec`: solo lectura, sin red, modelo y esfuerzo, y nada de la configuración del usuario;
  - la lectura de los eventos JSONL (texto, uso de tokens, modelo efectivo, error del CLI y JSONL cortado);
  - `ModoAutenticacion` con clave y con suscripción;
  - que el entorno no lleva secretos de otros agentes.
- [ ] **Claude:**
  - los argumentos: sin herramientas, modelo, esfuerzo y `--json-schema` con salida estructurada;
  - el resultado con uso y modelo efectivo;
  - un error del CLI;
  - el entorno sin secretos.
- [ ] Las muestras salen del formato que ya leen los adaptadores. Si hace falta una salida real, solo con Claude haiku.
- [ ] Commit: `test: Codex y Claude con un CLI simulado`.

### Tarea 8: Web UI

**Archivos:**
- `web/src/api.ts` → `web/src/api/cliente.ts`, `web/src/api/tipos.ts`, `web/src/estado.ts`, `web/src/recetas.ts`,
  `web/src/formato.ts`, `web/src/agentes.ts`;
- `web/src/paginas/Accesos.vue` → la página y `web/src/componentes/accesos/{ListaApis,DetalleApi,FormularioApi}.vue`;
- `web/src/App.vue` → `web/src/composables/{useSesion,useSondeo,useActualizacion}.ts`;
- los 23 archivos que importan `api.ts`;
- y `package.json` (`test` también en `src/**/*.test.ts`).

- [ ] **Separar `api.ts`** por responsabilidad, sin cambiar firmas. Cada importador pasa a importar del módulo que usa,
  y desaparece `api.ts`.
- [ ] **Separar `Accesos.vue`:** props y eventos explícitos; el estado compartido sigue en la página.
- [ ] **Separar `App.vue`:** el sondeo en curso, la sesión y la actualización pasan a composables. La plantilla queda
  en `App.vue`.
- [ ] **Pruebas `node --test`:**
  - `recetas.test.ts` (receta de un punto y de una solicitud, esquema y configuración de una receta);
  - `formato.test.ts` (fechas, `haceCuanto`, duraciones, estados);
  - `agentes.test.ts` (opciones de modelo, agrupar, esfuerzos y su predeterminado, orden);
  - `api/cliente.test.ts` (renovar la sesión con una respuesta 401 simulada).
- [ ] **Ningún archivo de `web/src` pasa de 500 líneas,** salvo los de `idiomas/`.
- [ ] **A mano, en el navegador** con la Web UI de prueba:
  - Mis APIs: lista, filtros, detalle, crear, editar y borrar;
  - Probar;
  - el aviso de actualización;
  - la sesión caducada.
- [ ] Commits: `refactor(web): separar api.ts`, `refactor(web): componentes de Mis APIs`,
  `refactor(web): composables de App`, `test(web): …`.

### Tarea 9: CI y Dependabot

**Archivos:** `.github/workflows/ci.yml`, `.github/workflows/publicar.yml`, `.github/dependabot.yml` (nuevo).

- [ ] **`ci.yml`:**
  - añade `workflow_call:` a `on`;
  - el `concurrency` usa `github.workflow`, así que una llamada desde Publicar no cancela un CI de la rama;
  - los nombres de los jobs no cambian.
- [ ] **`publicar.yml`:**
  - cambia `pruebas-windows` y `pruebas-unix` por un job `ci` con `uses: ./.github/workflows/ci.yml` y
    `permissions: contents: read`;
  - `publicar` pasa a `needs: [dependencias, ci, unix, linux-app, macos-app, windows]`;
  - se mantiene el resto, con `persist-credentials: false`.
- [ ] **`dependabot.yml`** (versión 2), semanal y con un grupo `menores` (`minor` y `patch`) en cada ecosistema:
  - `gomod` en `/`;
  - `gomod` en `/escritorio`, con `ignore: github.com/wailsapp/wails/v3`;
  - `npm` en `/web`;
  - `github-actions` en `/`.
- [ ] **Comprobar:**
  - `actionlint`, si está disponible; si no, que GitHub lo acepte en el PR;
  - los 6 checks del ruleset aparecen con su nombre en el PR;
  - la llamada desde Publicar se prueba en la etiqueta del release (tarea 11).
- [ ] Commit: `ci: Publicar reutiliza el CI y Dependabot semanal`.

### Tarea 10: Documentación

- [ ] **`docs/equipo/releases/`:**
  - cada spec de release con su plan pasa a `releases/v0.XX-<tema>/spec.md` y `plan.md` con `git mv`;
  - las parejas: recetas-respaldo 0.34, presupuestos 0.35, rediseno 0.36, actualizaciones 0.37, grupos-historial 0.38,
    pulido 0.39, probar 0.40, seguridad 0.41, api-propia 0.42, agentes-modulos 0.43, copilot 0.44, json 0.46,
    correcciones 0.47, portable 0.48 y orden 0.49;
  - antes de mover cada una, se mira su encabezado: si es una guía permanente y no la spec de un release, se queda donde
    está;
  - `grep` de cada nombre viejo en `docs/`, el README, el código y `CLAUDE.md`, y se arreglan los enlaces.
- [ ] **`docs/equipo/README.md`:**
  - el índice de las guías permanentes;
  - una tabla de releases con versión, tema y enlaces a spec y plan.
- [ ] **`docs/arquitectura.md`:**
  - el paquete `contrato`;
  - que los cuatro clientes hablan con el servicio solo por HTTP;
  - la tabla de rutas de la Web UI.
- [ ] **Donde se explica `tramtai acceso`** (`docs/operacion.md`, el README, `site/ayuda.html` y la Ayuda, si sale):
  que inicia el servicio si hace falta. En es y en.
- [ ] **`pendientes.md`:** v0.49 ✅ al publicar.
- [ ] **Notas** (`notas-049-es.txt` y `notas-049-en.txt`, sin subir):
  - Agentes responde antes;
  - la app de escritorio pesa menos;
  - `tramtai acceso` inicia el servicio solo.
- [ ] Commit: `docs: índice del equipo, releases por carpeta y arquitectura`.

### Tarea 11: Verificación y release

- [ ] Todas las comprobaciones generales.
- [ ] **A mano, con datos aislados:**
  - `tramtai acceso crear|editar|activar|desactivar|eliminar|token --rotar`, con el servicio parado y en marcha;
  - borrar y volver a crear desde la CLI y la terminal, con consumo cero;
  - una ejecución con Claude haiku desde la terminal;
  - la app de escritorio: bandeja, Mis APIs y abrir la Web UI;
  - la página Agentes con los tiempos de la tarea 6.
- [ ] **Revisión** con `ecc:code-reviewer` (opus). Se arregla todo lo que encuentre, también lo menor.
- [ ] PR (no borrador) con:
  - los tamaños de la app de escritorio;
  - los tiempos de antes y después;
  - y las pruebas a mano.
- [ ] CI en verde, `gh pr merge --merge --delete-branch`, `git pull`, después
  `scripts/version.ps1 -Etiquetar -Resumen notas-049-es.txt -ResumenEn notas-049-en.txt`, y `git push origin <etiqueta>`.
- [ ] **Publicar:**
  - el job `ci` llamado desde Publicar pasa;
  - el release trae sus 13 archivos con la firma válida;
  - el commit de paquetes llega a `main`.
