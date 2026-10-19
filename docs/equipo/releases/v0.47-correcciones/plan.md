# Correcciones de la revisión (v0.47): plan de implementación

**Objetivo:** arreglar los errores reales y de seguridad que encontró la revisión completa del 2026-10-19, sin cambiar
la estructura del código. La estructura es de la v0.49.

**Spec:** `docs/equipo/releases/v0.47-correcciones/spec.md`. Rama: `fix/correcciones`.

**Tecnología:** Go 1.27 y Vue 3 + Quasar + TypeScript. Las funciones puras de la web se prueban con `node --test`.

## Restricciones generales

- **Sin estructura:** no se mueven paquetes ni se dividen archivos. Si un arreglo pide una función nueva, va en el
  paquete donde ya vive el código que la usa.
- **Textos nuevos:** se añaden a `es.ts`/`en.ts` y a `internal/idioma/en_*.go` (los comprueba `TestCobertura`).
- **Comprobaciones:**
  - `go vet ./...` en Windows, con `GOOS=linux` y con `GOOS=darwin`;
  - `go test ./...` y `go test -race` en `servicio`, `api`, `planificador`, `documentos` y `registro`;
  - `cd escritorio && go vet ./... && go test ./...`;
  - `cd web && npm run build && npm test`;
  - `graphify update .`.
- **Web UI compilada:** si al compilar solo cambian los `data-v-` o los finales de línea, se restaura con
  `git checkout -- internal/recursos/web`.
- **Pruebas reales:** solo con Claude haiku.
- **Notas de versión:** cortas y sin detalles internos, porque el repo es público.
- **Cuenta del usuario:**
  - el `gh secret set` de la firma lo aprueba el usuario;
  - el token fine-grained de winget lo crea el usuario.

## Dónde revisar con más cuidado

1. **La firma (tarea 2):**
   - el cliente nunca instala sin una firma válida;
   - la clave privada no toca el disco ni los registros;
   - Publicar verifica la firma antes de crear el release.
2. **`config.yaml` (tarea 3):**
   - se valida en memoria antes de escribir;
   - el reemplazo es atómico;
   - dos ediciones a la vez no se pisan.
3. **Borrar una API (tarea 4):** el orden es editar → borrar la clave → consumo a cero, igual en la Web, la CLI y la TUI.
4. **El planificador (tarea 5):**
   - ningún error de la base deja un trabajo en `ejecutando`;
   - una desconexión real sigue cancelando.
5. **`Servicio.activar` (tarea 6):** fuera del lock no se crean dos instancias con el mismo id.

---

### Tarea 1: Publicación y CI

**Archivos:** `.github/workflows/publicar.yml`, `.github/workflows/ci.yml`, `scripts/compilar.ps1` y
`docs/equipo/versionado.md`.

- [ ] En `publicar.yml`:
  - `permissions: contents: read` arriba, y `permissions: contents: write` solo en los jobs `publicar` y `paquetes`;
  - `persist-credentials: false` en los `actions/checkout` del resto de jobs.
- [ ] wingetcreate:
  - descargarlo de una versión concreta del release de `microsoft/winget-create` y no de `aka.ms/…/latest`;
  - comprobar su SHA-256 (anotado en el YAML) antes de ejecutarlo;
  - poner `WINGET_TOKEN` solo en el `env:` del paso de envío.
- [ ] Job `web` de `ci.yml`: después de `npm run build`, un paso `git diff --exit-code -- internal/recursos/web` con un
  mensaje que diga «compila la Web UI y súbela».
  - Antes, comprobar en el CI que el build de Linux produce los mismos bytes que el versionado (LF). Si no, normalizar
    los finales de línea en la comparación con `git diff --ignore-cr-at-eol`.
- [ ] `compilar.ps1`: sin wails3, `throw` si `$Instalador` o `$env:CI`; si no, el `Write-Warning` de hoy.
- [ ] `ci.yml`: govulncheck también en `escritorio/`, igual que en `publicar.yml`.
- [ ] `versionado.md`: los permisos por job y por qué. Lo que tiene que hacer el usuario con `WINGET_TOKEN` (token
  fine-grained limitado al fork) se dice en la conversación y no en el repo.

### Tarea 2: Actualizaciones firmadas

**Archivos:**
- Nuevos: `scripts/firmar/main.go`, `internal/actualizacion/firma.go` y `firma_test.go`.
- Cambian: `internal/actualizacion/actualizacion.go`, `actualizacion_test.go` y `.github/workflows/publicar.yml`; los
  textos en `internal/idioma/en_*.go`; `docs/operacion.md` y `docs/equipo/versionado.md`.

- [ ] **`scripts/firmar` (Go, como `scripts/rtf`):**
  - `go run ./scripts/firmar SHA256SUMS.txt` lee la semilla ed25519 en base64 de `FIRMA_ACTUALIZACIONES`, desde el
    entorno y nunca desde un argumento, y escribe `SHA256SUMS.txt.sig` (firma en base64);
  - `-verificar <publica>` comprueba un archivo ya firmado;
  - `-generar` imprime un par nuevo: la pública por la salida estándar y la privada por stderr, para el `gh secret set`.
- [ ] **Clave:**
  - generar el par en local;
  - el usuario aprueba `gh secret set FIRMA_ACTUALIZACIONES` (la privada entra por stdin);
  - la pública queda como constante `clavePublica` en `firma.go`;
  - se borra cualquier rastro local.
- [ ] **En Publicar (job `publicar`):** después de calcular las sumas:
  - firmar con `FIRMA_ACTUALIZACIONES` en el `env:` solo de ese paso;
  - verificar con la pública del repo (`-verificar`);
  - subir el `.sig` como un archivo más del release. Las sumas no incluyen el `.sig`.
- [ ] **En `actualizacion`:**
  - `Ultima` guarda `urlFirma` (el archivo `SHA256SUMS.txt.sig`);
  - `TieneInstalador` también exige `urlFirma`;
  - `Descargar` baja la firma (como mucho 1 KiB), la valida con `ed25519.Verify` contra las sumas descargadas y solo
    entonces busca el instalador en ellas;
  - el prefijo de URL de la firma se comprueba como el de las demás.
- [ ] **Errores:** sin firma o con una inválida, error `idioma.T("la firma de esta versión no es válida: descárgala desde
  su página")`. El aviso de la Web UI ya muestra el enlace cuando `instalable` es falso.
- [ ] **Pruebas** (`httptest` como servidor de releases, con un par generado en la prueba y `clavePublica` sustituible
  por variable de paquete):
  - firma válida: instala;
  - firma de otro par: falla;
  - sin `.sig`: no es instalable;
  - sumas cambiadas tras firmar: falla.

### Tarea 3: Configuración y archivos sin cambios perdidos

**Archivos:** `internal/servicio/servicio.go`, `puntos.go` y sus pruebas; `internal/nucleo/configuracion.go` y sus
pruebas; `internal/persistencia/almacen.go` y sus pruebas.

- [ ] **`escribirAtomico(ruta string, datos []byte, permisos os.FileMode) error` en `servicio.go`:**
  - usa `os.CreateTemp(dir, base+".*.tmp")`, escribe, `Sync`, cierra y `Rename`, y borra el temporal si algo falla;
  - lo usan `escribirJSONArchivo`, `guardarSecreto` (que pierde su copia del patrón) y `editarYAML`.
- [ ] **`nucleo.CargarCapaDe(datos []byte, ruta string, origen Origen) (Capa, error)`:**
  - `CargarCapa` pasa a leer el archivo y llamar a esta;
  - `editarYAML` valida `salida` con `CargarCapaDe` **antes** de escribir y desaparece la restauración.
- [ ] **Cerrojo:** `var muYAML sync.Mutex` de paquete, tomado en `editarYAML` durante leer → editar → escribir.
- [ ] **Recargas:** `muRecargar sync.Mutex` en `Servicio`, tomado al principio de `Recargar`.
- [ ] **`Configuracion.Fijar`:** bucle `for { actual := c.estado.Load(); …; if c.estado.CompareAndSwap(actual, nuevo)
  { return } }`.
- [ ] **DSN de `persistencia.Abrir`:** `(&url.URL{Scheme: "file", Path: filepath.ToSlash(ruta), RawQuery: …}).String()`.
  En Windows, comprobar que el driver acepta `file:C:/…`; si pide `file:///C:/…`, ajustar con la prueba delante.
- [ ] **Pruebas:**
  - 20 `FijarValor` concurrentes sobre claves distintas, y al final están todas;
  - un YAML inválido no llega a escribirse (el archivo queda igual, byte a byte);
  - `escribirAtomico` concurrente sobre la misma ruta, sin errores;
  - `Abrir` en una carpeta `a#b%c d`;
  - `Fijar` concurrente.

### Tarea 4: Puntos de acceso con las mismas reglas

**Archivos:** `internal/servicio/puntos.go`, `web.go` y sus pruebas; `cmd/tramtai/acceso.go`; `internal/tui/tui.go` y
sus pruebas.

- [ ] **`validarRespaldo` pasa a ser una función de paquete:** comprueba el agente con el catálogo
  (`agentes.Buscar`), no con `s.opc.Registro`. Antes, verificar que los dos conjuntos coinciden; si no coinciden, se
  pasa el registro como parámetro.
- [ ] **`EditarPuntos`:** después de `editar`, para cada punto nuevo o cambiado (el mismo diff que hace hoy la TUI),
  aplica:
  - `Normalizar`;
  - `ValidarParaApps`;
  - `validarRespaldo`.

  Quitar esas llamadas de `web.go` y de `tui.go`, que pasan a usar las de `EditarPuntos`.
- [ ] **`EliminarPunto(datos, ruta string, origen nucleo.Origen, nombre string) error`:**
  - quita el punto con `EditarPuntos`;
  - `EliminarTokenPunto(datos, nombre)`;
  - `EliminarConsumo` de la base global, si está disponible para quien llama. Si no, basta el orden «clave antes de
    recargar», que es el que el servicio ya entiende.
- [ ] **Quién usa `EliminarPunto`:**
  - `acceso eliminar` (CLI): recarga después, como hoy;
  - la tecla `x` de la TUI: recarga después; se arregla el orden;
  - el borrado de la Web (`web.go`): sigue llamando a `EliminarConsumo`.
- [ ] **Pruebas:**
  - **Borrar y recrear en la TUI:** crear `p` con consumo y borrarlo con el modelo de la TUI; al crear otra `p`, el
    consumo es 0.
  - **CLI:** `acceso crear` con Antigravity y `permitir_apps: false` da error y no guarda.

### Tarea 5: Planificador y API

**Archivos:** `internal/adaptadores/adaptador.go`, `internal/api/nativa.go`, `fachadas.go`, `openai.go`,
`anthropic.go`, `punto.go`, `api.go` y sus pruebas; `internal/planificador/planificador.go` y sus pruebas.

- [ ] **`ModeloConcreto`:**
  - si `ctx` no trae plazo, le pone uno de 60 s;
  - deduplica las resoluciones en curso con `golang.org/x/sync/singleflight`, que ya es dependencia indirecta y pasa a
    directa (`go mod tidy`). La clave es `agente/modelo`.
  - En `nativa.go`, la goroutine sigue usando `WithoutCancel`; el plazo lo pone `ModeloConcreto`.
- [ ] **`planificador.cambiar`:** si `CambiarEstado` falla con un código que no es `transicion_invalida`:
  - lo registra en `opc.Registros`;
  - intenta una vez `→ interrumpido` con `codigo_error: error_interno`;
  - devuelve falso.

  Los `_ = p.almacen.AgregarEvento(...)` pasan por un `p.evento(...)` que registra el error.
- [ ] **Fachadas:**
  - en `seguir` y sus usos (`openai.go`, `anthropic.go`), solo es desconexión si `r.Context().Err() != nil`;
  - en otro caso, `EscribirError` (sin streaming) o el `fallar` de cada fachada (streaming).
- [ ] **`Idempotency-Key`:** el límite de 255 pasa de `enviar` (`nativa.go`) a `Servidor.aceptar`, para que lo cumplan
  las tres rutas.
- [ ] **Cuerpos lentos:**
  - `LeerJSON` y `leerFachada` fijan `http.NewResponseController(w).SetReadDeadline(time.Now().Add(30*time.Second))`;
  - las cargas de archivos, 5 minutos;
  - se ignora `http.ErrNotSupported`.
- [ ] **Pruebas:**
  - `ModeloConcreto` con un adaptador falso que bloquea: devuelve "" al vencer el plazo, y dos llamadas a la vez
    ejecutan una sola resolución;
  - `cambiar` con la base cerrada: el trabajo no queda colgado y hay una línea en el registro;
  - una fachada con un almacén que falla al seguir: 500 con el error, no un 200 vacío, y el trabajo no se cancela;
  - `Idempotency-Key` de 256 en `/openai/v1/chat/completions`: 400;
  - un cliente que manda el cuerpo byte a byte durante más que el plazo (en la prueba el plazo es inyectable): la
    conexión se corta.

### Tarea 6: Ciclo de vida del servicio y modo directo

**Archivos:** `internal/servicio/servicio.go`, `web.go`, `admin.go`, `actualizacion.go` y sus pruebas;
`internal/persistencia/almacen.go`; `cmd/tramtai/ejecutar.go`.

- [ ] **`activar`:**
  - con el lock, comprueba si la instancia ya existe o se está abriendo; un `map[string]*apertura` con `listo chan
    struct{}`, `inst` y `err`;
  - sin el lock, abre la base, termina los huérfanos y recupera los trabajos;
  - con el lock, publica la instancia;
  - quien llegue mientras tanto espera `listo`.
- [ ] **`Servicio.fondo sync.WaitGroup`:**
  - `LimpiarDescargas`, la purga tras `fijarRetencion` y la comprobación de actualizaciones van con `s.fondo.Go(...)`;
  - `Cerrar` hace `s.fondo.Wait()` antes de cerrar las bases;
  - la purga comprueba antes que la instancia siga abierta.
- [ ] **Errores que hoy se pierden:**
  - `admin.go`: los dos `_ = s.guardarActivas()` devuelven 500 con el error;
  - `Restaurar`: si `instancias.json` no se puede leer como JSON, lo renombra a `instancias.json.dañado`, lo registra y
    sigue;
  - `almacen.go`: los errores de los `PRAGMA` de mantenimiento se devuelven a quien llama, que los registra.
- [ ] **El constructor de la instancia:**
  - se extrae de `servicio.go` (lo que hoy está en `activar`) a una función exportada
    `AbrirInstancia(datos, id string, c *nucleo.Configuracion, o OpcionesInstancia) (*Instancia, error)`;
  - incluye la ventana de idempotencia y el presupuesto;
  - `ejecutar --directo` la usa, y al terminar llama a `Purgar` y `Cerrar`.
- [ ] **Ctrl+C en modo directo:** el primero cancela el trabajo; si llega un segundo antes de 5 s, sale con código 130.
- [ ] **Pruebas:**
  - dos `activar` simultáneos del mismo proyecto devuelven la misma instancia y abren la base una vez;
  - con `activar` abriendo una base lenta, `instanciaPara` de otra instancia responde (canal de bloqueo en la prueba);
  - `Cerrar` espera a una purga en curso;
  - `instancias.json` dañado;
  - `--directo` purga `cli.db` según `retencion`.

### Tarea 7: Errores pequeños del Go

**Archivos:**
- `internal/documentos/cargas.go` y `conversor.go`;
- `internal/procesos/grupo_windows.go`, `grupo_unix.go`, `memoria_linux.go` y uno nuevo, `limite_linux.go`;
- `cmd/tramtai/main.go`, para el comando oculto `__limite`;
- `internal/registro/registro.go`, `internal/navegador/navegador.go`, `internal/autoarranque/autoarranque_linux.go` e
  `internal/tui/tui.go`;
- `internal/agentes/antigravity/antigravity.go`;
- y las pruebas de cada uno.

- [ ] **Cargas:**
  - `purgar` toma `c.mu`;
  - en `Guardar`, si el contenido ya existe, se borra el temporal en lugar de renombrarlo encima.
- [ ] **Conversor:** `errors.Is(ctxConv.Err(), context.DeadlineExceeded)` con el contexto propio del conversor.
- [ ] **Windows:** `reanudar` cuenta los hilos reanudados y, si son 0, devuelve un error, y `Ejecutar` termina el grupo.
- [ ] **Linux, tope de memoria:**
  - si hay tope, el hijo se lanza como `tramtai __limite <MB> -- <programa> <args…>`, un comando oculto;
  - `__limite` hace `unix.Setrlimit(RLIMIT_AS)` y después `unix.Exec`, de modo que el agente y todos sus hijos lo
    heredan;
  - para la ruta de `tramtai` se usa `os.Executable()`; en las pruebas se sustituye por el binario de la prueba
    (`TestMain`);
  - se quita el `Prlimit` posterior a `Start`.
- [ ] **Registro:** si `abrir` falla en `rotar`, se reabre la ruta en modo añadir y se escribe una línea con el error.
- [ ] **Navegador:** `go cmd.Wait()` después de `Start()`.
- [ ] **Arranque en Linux:** una función `escaparExec(ruta)` que entrecomilla y escapa `"`, `` ` ``, `$` y `\`, y
  duplica `%`.
- [ ] **TUI:** `cli.Eventos` con un contexto que se cancela con Esc, al cambiar de pantalla y al salir.
- [ ] **Antigravity:**
  - primero la prueba: `TMP` en `t.TempDir()/con espacios`, y un `agy` falso (patrón `TestMain` de
    `antigravity_test.go`) que lee `hooks.json` y ejecuta el `command` con `cmd /c` como lo haría Antigravity;
  - si no recibe el `deny`, el guion se registra con su nombre corto (`windows.GetShortPathName`) y, si el volumen no
    tiene nombres cortos, en una carpeta sin espacios bajo los datos de Tramtai.
- [ ] **Pruebas:**
  - purgar mientras se guarda (`-race`);
  - `reanudar` sin hilos (Windows);
  - un hijo de un hijo que intenta reservar más memoria que el tope y falla (solo Linux, en el CI);
  - registro con la rotación fallida (la carpeta pasa a ser de solo lectura en Unix; en Windows, la ruta destino es una
    carpeta);
  - `escaparExec` con `%`, `$` y comillas.

### Tarea 8: Web UI

**Archivos:** `web/src/api.ts`, `i18n.ts`, `App.vue`, `componentes/NotasVersion.vue`, `componentes/EditorReceta.vue`,
`paginas/Ejecutar.vue`, `paginas/Accesos.vue`, `paginas/Trabajos.vue`, `paginas/Configuracion.vue` y `idiomas/*.ts`.
Nuevos: `web/src/sesion.ts` y `sesion.test.ts`, `i18n.test.ts` y `enlaces.test.ts`.

- [ ] **Renovación de la sesión:**
  - una función pura `compartida<T>(f: () => Promise<T>)` en `sesion.ts`, que devuelve siempre la promesa en curso;
  - `pedir` la usa para `api.sesion()`;
  - prueba: 5 llamadas a la vez ejecutan `f` una vez.
- [ ] **`Ejecutar.vue`:**
  - en `terminar(id)`, después del `await`, `if (trabajo.value?.id !== id) return`;
  - un indicador `desmontado`, y `seguir` no abre el `EventSource` si ya se salió de la página;
  - la salida pierde `aria-live` y lleva `aria-busy` mientras `activo`; el estado final va en un `role="status"`
    aparte.
- [ ] **`EditorReceta.vue`:** el watch de `niveles` no borra el esfuerzo si `catalogo` está vacío.
- [ ] **Enlaces seguros:** `enlaceSeguro(url)` en un módulo puro, que devuelve la URL solo si empieza por `https://`. La
  usan `NotasVersion.vue` y el botón de descarga de `App.vue`, que hoy lo comprueba aparte.
- [ ] **`t()`:** `textos[clave] ?? clave`. La función de búsqueda pura se prueba.
- [ ] **Progreso de la actualización (`App.vue`):** un indicador `enVuelo` en el sondeo.
- [ ] **`Accesos.vue`:** si no se encuentra la API recién creada, la clave va en un `Notify` con `timeout: 0` y una
  acción «Copiar».
- [ ] **Tipos:** `Solicitud.mensajes?: Mensaje[]`, y se ajustan los usos que no compilen.
- [ ] **Accesibilidad:**
  - `aria-labelledby` en los diálogos de Accesos (crear y ajustes), Probar (guardar como API y clave) e Historial;
  - `aria-label` en los buscadores de Historial y Ajustes;
  - el identificador que se copia en Historial pasa a ser un `q-btn`.
- [ ] **Idioma:**
  - `'API '`, `tokens` y `'OK'` pasan a ser claves de `es.ts`/`en.ts`;
  - las fechas de Ajustes usan `idioma.value`.

### Tarea 9: «Mis APIs» en la bandeja

**Archivos:** `escritorio/main.go` y uno nuevo, `escritorio/menu_apis.go`, con su prueba; `web/src/App.vue`; los textos
en `internal/idioma/en_*.go`.

- [ ] **`menuApis(puntos []servicio.EstadoPunto, maximo int) (mostrar []servicio.EstadoPunto, resto int)`:**
  - **Orden:**
    1. primero las que tienen error (habilitadas y no activas);
    2. después las activas;
    3. por último las desactivadas.

    Dentro de cada bloque, por nombre y después por instancia.
  - Devuelve las primeras `maximo` (15).
- [ ] **`construir`:**
  - usa `menuApis`;
  - si `resto > 0`, añade «Ver las %d restantes en Mis APIs…» → `irA("conectar")`;
  - el título sigue con los totales.
- [ ] **Si falla `ActivarPunto`:** se abre la ventana y se hace
  `ventana.ExecJS("window.tramtaiAviso && window.tramtaiAviso(" + json + ")")`. En `App.vue`, `tramtaiAviso` se registra
  junto a `tramtaiAcercaDe` y llama a `avisar` con el mensaje.
- [ ] **Pruebas de `menuApis`:**
  - con 3 APIs, todas;
  - con 15, todas y resto 0;
  - con 40 y errores mezclados, las que tienen error primero y resto 25.

### Tarea 10: Documentación y notas

- [ ] **`docs/operacion.md` y la Ayuda (Web UI y `site/ayuda.html`, es/en):** la actualización de un clic comprueba la
  firma de la versión, y si no es válida pide descargarla desde su página.
- [ ] **`docs/equipo/versionado.md`:** el secreto `FIRMA_ACTUALIZACIONES`, cómo rotarlo (`scripts/firmar -generar` más
  un release que incruste la clave pública nueva, firmado aún con la anterior) y los permisos por job.
- [ ] **`docs/equipo/pendientes.md`:** se cierra esta revisión y se añaden las tareas de la v0.49.
- [ ] **Notas (`notas-047-es.txt` / `-en.txt`, ignoradas por git):** un titular y unas pocas correcciones visibles para
  quien usa Tramtai, sin detalles internos:
  - actualizaciones verificadas;
  - Mis APIs en la bandeja;
  - Probar más estable;
  - accesibilidad.

### Tarea 11: Verificación y release

- [ ] Todas las comprobaciones de las restricciones generales, en verde.
- [ ] **A mano, con haiku y el servicio compilado:**
  - Probar con dos trabajos seguidos (abrir el segundo mientras el primero termina);
  - borrar y volver a crear una API en la TUI y comprobar que el consumo es 0;
  - la bandeja con 20 APIs de prueba en una instancia aislada (`APPDATA` temporal);
  - `tramtai ejecutar --directo` dos veces, y que `cli.db` se purga según `retencion`.
- [ ] Revisión de toda la rama con `ecc:code-reviewer` (opus), y arreglar cada hallazgo, también los menores.
- [ ] Flujo de release de siempre:
  1. merge a main y las pruebas;
  2. push;
  3. `version.ps1 -Etiquetar` con las notas;
  4. `git push origin <tag>`;
  5. vigilar CI y Publicar;
  6. comprobar en el release que están `SHA256SUMS.txt.sig` y la firma verificada;
  7. pull del commit de paquetes y borrar la rama.
