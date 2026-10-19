# Archivos (v0.50): plan de implementación

> **Para agentes:** SUB-SKILL REQUERIDA: superpowers:subagent-driven-development (recomendada) o
> superpowers:executing-plans, tarea por tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** markitdown viene con Tramtai (uv incluido y preparación en su carpeta), se avisa antes de enviar si falta,
y las tres puertas (nativa, OpenAI, Anthropic) aceptan archivos; en Mis APIs, «Cómo usarla» sustituye a «Descargar
OpenAPI».

**Arquitectura:** `internal/documentos` localiza el conversor (configurado → paquete → preparado → sistema) y lo prepara
con el `uv` incluido y un `conversor.lock` incrustado, en un Python independiente sin venv. Un único `Preparacion` vive
en el servicio y lo comparten todas las instancias; el planificador rechaza con 503 al aceptar si una carga necesita
conversor y no está listo. Las fachadas guardan los archivos en base64 como cargas y siguen el recorrido nativo.

**Tecnologías:** Go 1.x (cobra, Bubble Tea), Vue 3 + Quasar + vitest, PowerShell (scripts y CI), uv 0.12.23,
markitdown 0.1.8, Python 3.12.

**Spec:** [spec.md](spec.md)

## Restricciones globales

- markitdown `markitdown[pdf,docx,pptx,xlsx]==0.1.8`; Python `3.12`; uv `0.12.23`, fijado con SHA-256 por objetivo:
  `x86_64-pc-windows-msvc`, `x86_64-unknown-linux-musl`, `aarch64-unknown-linux-musl`, `x86_64-apple-darwin`,
  `aarch64-apple-darwin`.
- Estados: `listo`, `preparando`, `falta`, `error`; orígenes: `configurado`, `paquete`, `preparado`, `sistema`; etapas:
  `python`, `paquetes`, `comprobacion`.
- Código nuevo `conversor_no_disponible` → HTTP 503; con `preparando`, `Retry-After: 60`.
- Configuración nueva `conversion.preparar_al_iniciar` (booleano, `true` por defecto).
- uv con `UV_NO_CONFIG=1`, `UV_NATIVE_TLS=1`, `UV_PYTHON_INSTALL_DIR` y `UV_CACHE_DIR` en carpetas de Tramtai; `uv python
  install … --no-bin --no-registry`; `uv pip install … --break-system-packages --require-hashes --no-deps`.
- Entorno de uv y markitdown: `entornoConversor` (sin secretos ni `TRAMTAI_*`); se heredan `HTTPS_PROXY`/`NO_PROXY`.
- Ubicación de uv: junto al ejecutable → `<dir ejecutable>/../lib/tramtai/uv` → PATH.
- Carpeta preparada: `<RaizUsuario>/conversor/{python/, listo.json}`; paquete: `<dir ejecutable>/conversor/listo.json`;
  trabajo en `conversor.nuevo/` y renombrado al final.
- Fachadas: tope de 16 MB de cuerpo (≈12 MB de archivos); máximo 20 cargas por petición contando todos los turnos.
- Todo texto nuevo de app, CLI, terminal, web, Ayuda, README y notas en **español e inglés** (`internal/idioma`,
  `web/src/idiomas/{es,en}.ts`).
- Comentarios y nombres en español como el resto del código; `gofmt`; mensajes de commit como los del repo, con la
  línea `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Tras cambiar código: `graphify update .`.
- No se cambian los checks obligatorios del ruleset de `main`: el job nuevo del CI se suma sin exigirlo.

## Foco de revisión

1. **Dos preparaciones a la vez** (botón mientras corre la del arranque): solo corre una; la segunda devuelve el estado
   en curso. Prueba en la tarea 2.
2. **Servicio cortado a media preparación:** queda `conversor.nuevo/`; al arrancar se borra, el estado es `falta` (o
   sigue `listo` el anterior) y nunca `listo` por lo a medias. Prueba en la tarea 2.
3. **Red que cuelga la descarga** (proxy que no responde): la preparación termina en `error` con un mensaje legible tras
   su plazo (30 min), no queda `preparando` para siempre. Prueba en la tarea 2 con un uv falso que no termina y un plazo
   corto.
4. **El mismo PDF en dos turnos** de una conversación por una fachada: una sola carga y un solo bloque de documento, no
   dos. Prueba en la tarea 5.
5. **Carpeta de usuario con espacios y tildes** (`C:\Users\José Pérez\…`): uv y python reciben las rutas como argumentos
   separados, sin pasar por un shell. Prueba en la tarea 2 (el uv falso registra sus argumentos y se comprueban).

---

### Tarea 1: Localizar el conversor y el lock incrustado

**Archivos:**
- Crear: `internal/documentos/conversor/conversor.in` (una línea: `markitdown[pdf,docx,pptx,xlsx]==0.1.8`)
- Crear: `internal/documentos/conversor/conversor.lock` (generado, ver paso 1)
- Crear: `internal/documentos/localizar.go`
- Modificar: `internal/documentos/conversor.go` (`Conversor`, `Desde`, `programa`, se quita el recurso a `uvx` y
  `EspecificacionMarkitdown` pasa a usarse solo en mensajes)
- Modificar: `internal/nucleo/errores.go` (código nuevo), `internal/api/api.go` (`estadosHTTP`)
- Modificar: `internal/idioma/en_servicio.go` (mensajes nuevos, se quita el de `uv tool install`)
- Prueba: `internal/documentos/localizar_test.go`

**Interfaces:**
- Produce:
  - `nucleo.CodigoConversorNoDisponible CodigoError = "conversor_no_disponible"` → `http.StatusServiceUnavailable`.
  - `documentos.LockConversor []byte` (`//go:embed conversor/conversor.lock`) y `func HuellaLock() string` (sha256 hex).
  - `type Origen string` con `OrigenConfigurado`, `OrigenPaquete`, `OrigenPreparado`, `OrigenSistema`.
  - `type Ubicacion struct { Origen Origen; Version string; Programa string; Argumentos []string; Lock string }`
    (`Lock` es el `lock_sha256` de `listo.json`, vacío fuera de paquete/preparado).
  - `type Listo struct { Markitdown, LockSHA256, Python string; PreparadoEn time.Time }` con etiquetas JSON
    `markitdown`, `lock_sha256`, `python`, `preparado_en`.
  - `func Localizar(comando, raiz string) (Ubicacion, bool)` (usa `os.Executable` + `EvalSymlinks` y `exec.LookPath`);
    `func localizarEn(comando, dirEjecutable, raiz string, buscar func(string) (string, error)) (Ubicacion, bool)`.
  - `func interprete(dir string) string`: `dir/python/python.exe` en Windows, `dir/python/bin/python3` en el resto.
  - `Conversor.Raiz string` (raíz de usuario; `Desde` la toma de `nucleo.RaizUsuario()`).

- [ ] **Paso 1: generar el lock.** `uv pip compile internal/documentos/conversor/conversor.in --universal
  --generate-hashes --python-version 3.12 -o internal/documentos/conversor/conversor.lock`. Esperado: el archivo
  lista `markitdown==0.1.8` y cada dependencia con `--hash=sha256:`.
- [ ] **Paso 2: pruebas que fallan** en `localizar_test.go`:
  - `TestLocalizarConfigurado`: `localizarEn("/x/mi-markitdown", …)` → `Origen == OrigenConfigurado`,
    `Programa == "/x/mi-markitdown"`, sin argumentos.
  - `TestLocalizarPaquete`: con `<dirEjecutable>/conversor/listo.json` y su intérprete creados →
    `OrigenPaquete`, `Argumentos == []string{"-m", "markitdown"}`, `Version == "0.1.8"`.
  - `TestLocalizarPreparadoAntesQueSistema`: preparado en `raiz/conversor` y `buscar` que encuentra `markitdown` →
    gana `OrigenPreparado`.
  - `TestLocalizarSistema`: solo `buscar` → `OrigenSistema`, `Version == ""`.
  - `TestLocalizarNada`: → `ok == false`.
  - `TestLocalizarListoSinInterprete`: `listo.json` sin intérprete → no cuenta.
  - `TestHuellaLock`: `len(HuellaLock()) == 64`.
- [ ] **Paso 3:** `go test ./internal/documentos -run 'Localizar|HuellaLock'` → FALLA (no compila).
- [ ] **Paso 4: implementar** `localizar.go` y que `Conversor.programa()` use `Localizar(c.Comando, c.Raiz)`; sin
  conversor, `nucleo.NuevoError(CodigoConversorNoDisponible, idioma.T("no hay conversor de archivos: prepáralo en
  Ajustes → Archivos o con «tramtai conversor preparar»"))`. Las pruebas existentes con markitdown falso usan un
  `Comando` distinto de `markitdown`, así que siguen como `configurado`.
- [ ] **Paso 5:** `go test ./internal/documentos ./internal/api ./internal/nucleo` → PASA.
- [ ] **Paso 6: commit** `feat(documentos): localizar el conversor (configurado, paquete, preparado, sistema) y lock fijado`.

### Tarea 2: Preparar el conversor con uv

**Archivos:**
- Crear: `internal/documentos/preparacion.go`, `internal/documentos/preparacion_test.go`
- Crear: `internal/documentos/real_conversor_test.go` (etiqueta `conversor`)
- Crear: `internal/documentos/testdata/muestra.pdf`, `muestra.docx`, `muestra.xlsx` (cada uno con el texto
  `Tramtai muestra`; se generan una vez con un script en el scratchpad y se versionan)
- Modificar: `internal/contrato/` (tipo compartido, en el archivo de tipos que ya existe)

**Interfaces:**
- Consume: `Localizar`, `LockConversor`, `HuellaLock`, `interprete`, `entornoConversor` (tarea 1).
- Produce:
  - `contrato.EstadoConversor struct { Estado, Origen, Version, Etapa, Mensaje string; Progreso int;
    PrepararAlIniciar bool }` con etiquetas `estado`, `origen,omitempty`, `version,omitempty`, `etapa,omitempty`,
    `progreso,omitempty`, `mensaje,omitempty`, `preparar_al_iniciar`.
  - `func BuscarUV() (string, error)` (orden de las restricciones globales).
  - `type Preparacion struct` (con mutex) y `func NuevaPreparacion(raiz, comando, uv string) *Preparacion`;
    `func (p *Preparacion) Estado() contrato.EstadoConversor` (sin `PrepararAlIniciar`, lo pone la API);
    `func (p *Preparacion) Preparar() contrato.EstadoConversor` (lanza en segundo plano si no corre);
    `func (p *Preparacion) Esperar(ctx context.Context) contrato.EstadoConversor`;
    `func (p *Preparacion) Desactualizado() bool` (preparado con `Lock != HuellaLock()`);
    `func PrepararEn(ctx context.Context, destino, uv string, avance func(etapa string, progreso int)) error`
    (síncrona, la usan `Preparar` y el empaquetado de la tarea 9).
  - Campo de prueba `plazo time.Duration` (30 min por defecto).

- [ ] **Paso 1: pruebas que fallan** en `preparacion_test.go`, con un uv falso (el binario de prueba vía `TestMain`,
  como el markitdown falso de `documentos_test.go`): `python install` crea un intérprete falso que responde a
  `-m markitdown` y `pip install` escribe un marcador. El falso guarda sus argumentos en un archivo.
  - `TestPrepararPasaAListo`: `falta` → `Preparar()` → `Esperar` → `Estado == "listo"`, `Origen == "preparado"`,
    `listo.json` con `lock_sha256 == HuellaLock()`, sin `conversor.nuevo/`.
  - `TestPrepararArgumentosUV`: los argumentos incluyen `--no-bin`, `--no-registry`, `--break-system-packages`,
    `--require-hashes`, `--no-deps`; el entorno tiene `UV_NO_CONFIG=1` y `UV_NATIVE_TLS=1`; y una raíz
    `t.TempDir()+"/José Pérez"` llega como un único argumento (foco 5).
  - `TestPrepararDosALaVez`: dos `Preparar()` seguidos → el falso se ejecuta una vez (foco 1).
  - `TestPrepararFallaDejaError`: uv que sale con código 1 y «network unreachable» → `Estado == "error"`, `Mensaje`
    no vacío; el preparado anterior, si había, sigue `listo` tras `Estado()`.
  - `TestPrepararPlazo`: uv que no termina y `plazo = 200ms` → `error` (foco 3).
  - `TestRestosDeCorte`: `conversor.nuevo/` a medias sin `listo.json` → `NuevaPreparacion` lo borra y `Estado == "falta"`
    (foco 2).
  - `TestDesactualizado`: `listo.json` con otro lock → `Desactualizado() == true` y `Estado == "listo"`.
- [ ] **Paso 2:** `go test ./internal/documentos -run Preparar` → FALLA.
- [ ] **Paso 3: implementar** `preparacion.go`. `PrepararEn` hace los pasos 1–5 de la spec §1 (lock escrito en un
  temporal; comprobación con un HTML mínimo incrustado que debe devolver su texto); `Preparar` corre `PrepararEn` en una
  goroutine con `context.WithTimeout(plazo)` sobre `<raiz>/conversor.nuevo` y renombra a `<raiz>/conversor`
  (`os.RemoveAll` del anterior y `os.Rename`; en Windows, reintentos breves si el antivirus retiene). Progreso
  aproximado: `python` 0–40, `paquetes` 40–90, `comprobacion` 90–100. Mensaje de error: última línea de stderr de uv
  (`ultimaLinea`), traducible.
- [ ] **Paso 4:** `go test ./internal/documentos` → PASA.
- [ ] **Paso 5: prueba real** `real_conversor_test.go` (`//go:build conversor`): `TestConversorReal` toma uv de
  `TRAMTAI_UV` (o `BuscarUV`), hace `PrepararEn` en `t.TempDir()` y convierte las tres muestras con un `Conversor`
  apuntado a ese intérprete; cada salida contiene `Tramtai muestra`. Ejecutar en local: `go test -tags conversor
  -run TestConversorReal -v ./internal/documentos` → PASA.
- [ ] **Paso 6: commit** `feat(documentos): preparar markitdown con el uv incluido en un Python propio`.

### Tarea 3: Rechazo al aceptar si falta el conversor

**Archivos:**
- Modificar: `internal/documentos/preparador.go`, `internal/documentos/conversor.go` (exportar la lista de extensiones
  convertibles para reutilizarla)
- Modificar: `internal/planificador/planificador.go:198-203`
- Modificar: `internal/nucleo/` (error tipado, junto a `PresupuestoAgotado`), `internal/api/api.go` (`EscribirError`)
- Prueba: `internal/documentos/preparador_test.go`, `internal/api/api_test.go`

**Interfaces:**
- Consume: `Localizar`, `Preparacion.Estado` (tareas 1 y 2).
- Produce:
  - `func NecesitaConversor(nombre, ruta string) bool`: extensión convertible, o `.zip` cuyo índice (central directory,
    `zip.OpenReader`, sin extraer, recorriendo ZIP anidados por nombre solamente) tiene alguna.
  - `Preparador.Preparacion *Preparacion` (nil en la CLI local) y `func (p Preparador) ConversorListo(cargas []string) error`.
  - `type nucleo.ConversorPreparando struct { Err error }` con `Error()` y `Unwrap()`, como `PresupuestoAgotado`.

- [ ] **Paso 1: pruebas que fallan:**
  - `TestNecesitaConversor`: `informe.PDF` → true; `notas.md` → false; ZIP con `a.txt` → false; ZIP con `b.docx` → true.
  - `TestConversorListoSinConversor`: carga PDF, sin conversor → `CodigoDe(err) == conversor_no_disponible`.
  - `TestConversorListoPreparando`: con `Preparacion` en `preparando` → `errors.As(err, &*nucleo.ConversorPreparando)`.
  - `TestConversorListoSoloTexto`: cargas `.txt` sin conversor → nil.
  - En `api_test.go`, `TestCrearTrabajoSinConversor`: `POST /api/v1/trabajos` con una carga PDF → 503, código
    `conversor_no_disponible`; con `ConversorPreparando` → cabecera `Retry-After: 60`; el trabajo no queda creado.
  - `TestReintentarSinConversor`: reintentar un trabajo con PDF sin conversor → 503 (la comprobación también vale
    para `intento_de`).
- [ ] **Paso 2:** `go test ./internal/documentos ./internal/api ./internal/planificador` → FALLA.
- [ ] **Paso 3: implementar.** En el planificador, tras `Verificar`, una segunda comprobación por interfaz opcional
  `interface{ ConversorListo([]string) error }` para **toda** solicitud con cargas (nueva o reintento). En
  `EscribirError`, `Retry-After: 60` si `errors.As(err, &preparando)`.
- [ ] **Paso 4:** `go test ./...` → PASA.
- [ ] **Paso 5: commit** `feat: rechazar al aceptar los archivos que necesitan conversor si no está listo (503)`.

### Tarea 4: Rutas del conversor, configuración y arranque

**Archivos:**
- Modificar: `internal/nucleo/configuracion.go` (`conversion.preparar_al_iniciar`: tipo booleano y `true` por defecto)
- Modificar: `internal/api/api.go` (`Servidor.Conversor *documentos.Preparacion`, `Servidor.FijarPrepararAlIniciar
  func(bool) error`, rutas), `internal/api/nativa.go` (`capacidades` con bloque `conversor`), `internal/api/openapi.json`
- Crear: `internal/api/conversor.go`
- Modificar: `internal/servicio/servicio.go` (crear la `Preparacion` al iniciar y preparar si toca),
  `internal/servicio/instancias.go:150-158` (pasarla al `Preparador` y al `api.Servidor`)
- Prueba: `internal/api/conversor_test.go`, `internal/servicio/servicio_test.go`, `internal/api/openapi_test.go`

**Interfaces:**
- Consume: `Preparacion`, `contrato.EstadoConversor`, `BuscarUV` (tarea 2); `servicio.FijarValor` (ya existe, como en
  `fijarBusqueda`).
- Produce (HTTP):
  - `GET /api/v1/conversor` → 200 `EstadoConversor` (con `preparar_al_iniciar` de la configuración). En instancia y en
    puntos de acceso.
  - `POST /api/v1/conversor/preparar` → 202 `EstadoConversor`. **Solo si `s.Punto == nil`.**
  - `PUT /api/v1/conversor/ajustes` con `{"preparar_al_iniciar": bool}` → 204. **Solo si `s.Punto == nil`.**
  - `GET /api/v1/capacidades` añade `"conversor": EstadoConversor`.

- [ ] **Paso 1: pruebas que fallan:**
  - `TestConversorEstado`: `GET /api/v1/conversor` → 200 y `estado` en `{listo,preparando,falta,error}`.
  - `TestConversorPrepararSoloInstancia`: en un `Servidor` con `Punto` → `POST /api/v1/conversor/preparar` da 404;
    sin `Punto` → 202.
  - `TestConversorAjustes`: `PUT …/ajustes {"preparar_al_iniciar": false}` llama a `FijarPrepararAlIniciar(false)`.
  - `TestCapacidadesConversor`: la respuesta tiene la clave `conversor`.
  - `TestOpenapiConversor`: `openapi.json` describe las tres rutas (`openapi_test` ya compara rutas con el enrutador).
  - En servicio: `TestArranqueNoPreparaSiDesactivado`: `conversion.preparar_al_iniciar: false` y estado `falta` → el uv
    falso no se ejecuta; `TestArranquePreparaSiFalta`: `true` → se ejecuta una vez.
- [ ] **Paso 2:** `go test ./internal/api ./internal/servicio` → FALLA.
- [ ] **Paso 3: implementar.** Una sola `Preparacion` por servicio (`uv` de `BuscarUV`; si no hay uv, la preparación
  termina en `error` con «no se encontró uv junto a Tramtai»). Al iniciar: si `preparar_al_iniciar` y (`falta` o
  `Desactualizado()`), `Preparar()`. Registrar en `s.opc.Registros` el inicio, el fin y el error de la preparación.
- [ ] **Paso 4:** `go test ./...` → PASA.
- [ ] **Paso 5: commit** `feat(api): estado y preparación del conversor; se prepara solo al iniciar`.

### Tarea 5: Archivos por las fachadas OpenAI y Anthropic

**Archivos:**
- Crear: `internal/api/archivos.go`
- Modificar: `internal/api/fachadas.go` (`textoDe` queda para system y resultados de herramientas), `internal/api/openai.go`
  (chat y responses), `internal/api/herramientas.go` (bucle de partes de Anthropic), `internal/api/anthropic.go`
- Modificar: `internal/idioma/en_servicio.go`
- Prueba: `internal/api/archivos_test.go`

**Interfaces:**
- Consume: `Servidor.Cargas.Guardar(nombre, punto, r)`, `s.nombrePunto()`, `nucleo.MaximoCargas`.
- Produce:
  - `type archivoEnLinea struct { Nombre string; Datos []byte }`.
  - `func archivoDeParte(p parteContenido, mensaje, parte int, n *int) (archivoEnLinea, bool, error)`: reconoce
    `file` (chat), `input_file` (responses) y `document` (Anthropic: `source.type` `base64` o `text`); `bool` falso si
    la parte no es un archivo. `parteContenido` reúne los campos JSON de las tres formas (`type`, `text`,
    `file{filename,file_data,file_id}`, `filename`, `file_data`, `file_id`, `file_url`, `title`,
    `source{type,media_type,data}`).
  - `func extensionDeMIME(mime string) string` para la tabla de la spec §3.
  - `func (s *Servidor) guardarEnLinea(archivos []archivoEnLinea, sol *nucleo.SolicitudEjecucion) error`: guarda cada
    archivo y agrega su id a `sol.Cargas` **sin repetir** ids; más de `nucleo.MaximoCargas` → `solicitud_invalida`.

- [ ] **Paso 1: pruebas que fallan** en `archivos_test.go` (con `Cargas` en `t.TempDir()` y el markitdown falso):
  - `TestChatConArchivo`: `/openai/v1/chat/completions` con una parte `file` y `file_data` `data:application/pdf;base64,…`
    → 200 y el trabajo tiene 1 carga llamada `informe.pdf`.
  - `TestResponsesConArchivo`: `input_file` → 1 carga.
  - `TestAnthropicDocumento`: `document` base64 con `title: "Contrato"` y `media_type: application/pdf` → carga
    `Contrato.pdf`; `source.type: "text"` → carga `.txt` con ese texto.
  - `TestArchivoSinNombre`: sin `filename` y MIME `application/vnd.openxmlformats-officedocument.wordprocessingml.document`
    → `documento-1.docx`.
  - `TestArchivoMismoEnDosTurnos`: el mismo PDF en los turnos 1 y 3 → `len(trabajo.Solicitud.Cargas) == 1` (foco 4).
  - `TestArchivoRechazos` (tabla): base64 roto y `data:` sin `;base64,` → 400 `solicitud_invalida` con «mensaje 1,
    parte 2»; `file_id` → 400 cuyo mensaje contiene `file_data`; `file_url` y `source.type: "url"` → 400
    `parametro_no_soportado`; `image_url`, `input_image`, `image` → 400 con «imágenes»; MIME `application/x-foo` sin
    nombre → `carga_invalida`; 21 archivos → `solicitud_invalida`.
  - `TestArchivoSinConversorFachada`: PDF sin conversor → 503 en formato OpenAI (`code: conversor_no_disponible`) y
    en formato Anthropic.
  - `TestNombreConRuta`: `filename: "../../x.pdf"` → la carga se llama `x.pdf` (lo hace `nombreSeguro`).
- [ ] **Paso 2:** `go test ./internal/api -run 'Archivo|ConArchivo|Documento|NombreConRuta'` → FALLA.
- [ ] **Paso 3: implementar** y engancharlo donde hoy cada fachada arma `sol.Mensajes` (antes de `aceptarFachada`).
- [ ] **Paso 4:** `go test ./...` → PASA.
- [ ] **Paso 5: commit** `feat(api): las fachadas OpenAI y Anthropic aceptan archivos y los convierten`.

### Tarea 6: CLI y terminal

**Archivos:**
- Crear: `cmd/tramtai/conversor.go`; Modificar: `cmd/tramtai/main.go` (registrar el grupo)
- Modificar: `internal/cliente/cliente.go`, `cmd/tramtai/ejecutar.go` (modo directo), `cmd/tramtai/ejecutar_servicio.go`
- Modificar: `internal/tui/ejecutar.go`, `internal/tui/vista.go`
- Modificar: `internal/idioma/` (textos en)
- Prueba: `cmd/tramtai/conversor_test.go`, `internal/tui/tui_test.go`

**Interfaces:**
- Consume: rutas de la tarea 4; `NecesitaConversor`, `Localizar`, `PrepararEn` (tareas 1–3).
- Produce:
  - `func (c *Cliente) Conversor(ctx context.Context, instancia string) (contrato.EstadoConversor, error)` y
    `func (c *Cliente) PrepararConversor(ctx context.Context, instancia string) (contrato.EstadoConversor, error)`.
  - `tramtai conversor estado` (imprime estado, origen y versión), `tramtai conversor preparar` (lanza y sondea cada
    1 s mostrando etapa y %; sale con 1 si termina en `error`), `tramtai conversor preparar --en <carpeta>` (sin
    servicio, `PrepararEn` directo en esa carpeta; lo usa la tarea 9).

- [ ] **Paso 1: pruebas que fallan:**
  - `TestConversorEstadoCLI`: con un servicio de prueba en `falta` → la salida contiene `falta` y la instrucción.
  - `TestEjecutarPDFSinConversor`: `tramtai ejecutar --archivo x.pdf` en modo directo sin conversor → error antes de
    `Guardar` (no queda carga en `cargas/cli`).
  - En la TUI, `TestAvisoConversorAlAdjuntar`: con archivo `.pdf` y conversor `falta` → la vista muestra el aviso y
    `ctrl+p` llama a `PrepararConversor`.
- [ ] **Paso 2:** `go test ./cmd/... ./internal/tui ./internal/cliente` → FALLA.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** `go test ./...` → PASA.
- [ ] **Paso 5: commit** `feat(cli): tramtai conversor estado|preparar y aviso en la terminal`.

### Tarea 7: Web UI: estado, Ajustes, Probar e Inicio

**Archivos:**
- Modificar: `web/src/api/tipos.ts` (`EstadoConversor`), `web/src/api/cliente.ts` (`conversor`, `prepararConversor`,
  `ajustesConversor`)
- Crear: `web/src/conversor.ts` (`necesitaConversor(nombre: string): boolean` y un sondeo compartido
  `usarConversor()` que consulta cada 2 s mientras el estado es `preparando`), `web/src/conversor.test.ts`
- Crear: `web/src/componentes/AvisoConversor.vue` (texto según estado, botón «Preparar archivos»/«Preparar de nuevo»,
  barra de progreso con etapa)
- Modificar: `web/src/paginas/Ejecutar.vue` (aviso bajo Adjuntar; «Enviar» deshabilitado con motivo si un adjunto lo
  necesita y no está `listo`; con un `.zip` solo avisa, sin deshabilitar, porque decide el servicio),
  `web/src/paginas/Configuracion.vue` (tarjeta «Archivos»: estado, origen, versión, botón y `q-toggle` «Preparar al
  iniciar»), `web/src/paginas/Panel.vue` (aviso discreto con enlace a Ajustes si `falta` o `error`)
- Modificar: `web/src/idiomas/es.ts`, `web/src/idiomas/en.ts`

- [ ] **Paso 1: pruebas que fallan** en `conversor.test.ts`: `necesitaConversor('a.PDF') === true`,
  `('a.md') === false`, `('a.zip') === true`, `('a.xlsx') === true`; el sondeo se detiene al pasar a `listo`
  (temporizadores falsos de vitest).
- [ ] **Paso 2:** `npm --prefix web test -- conversor` → FALLA.
- [ ] **Paso 3: implementar**; `textos.test.ts` ya comprueba que cada clave de `es.ts` está en `en.ts`.
- [ ] **Paso 4:** `npm --prefix web test` y `npm --prefix web run build` → PASA.
- [ ] **Paso 5: navegador** (preview de la Web UI con el servicio local): con el conversor movido aparte, adjuntar un PDF
  en Probar muestra el aviso y deshabilita «Enviar»; «Preparar archivos» muestra el progreso y al terminar se habilita;
  Ajustes → Archivos muestra `listo` y el origen. Captura como prueba.
- [ ] **Paso 6: commit** `feat(web): estado del conversor, aviso al adjuntar y Ajustes → Archivos`.

### Tarea 8: Web UI: «Cómo usarla» y ejemplos con archivos

**Archivos:**
- Modificar: `web/src/componentes/GuiaUrls.vue` (el botón de la tarjeta `nativa` pasa a «Cómo usarla» y emite `usar`;
  debajo, una línea secundaria «Avanzado: importar en Postman o Insomnia» con «Descargar OpenAPI» y una frase de para
  qué sirve; fila `guia.fArchivos` a «sí» en las tres con la nota `≈12 MB`)
- Modificar: `web/src/paginas/Accesos.vue` y `web/src/componentes/accesos/DetalleApi.vue` (al recibir `usar`, eligen el
  ejemplo de la API propia y desplazan hasta los ejemplos)
- Modificar: `web/src/ejemplos.ts` (`chatOpenAI`, `mensajesAnthropic` y sus generadores: con archivos, la parte `file` o
  `document` con el archivo leído y en base64 en curl, Python y Node; `notaFachada` deja de listar los archivos);
  ejemplo de muestra de la API propia con `informe.pdf`
- Crear: `web/src/ejemplos.test.ts`
- Modificar: `web/src/idiomas/es.ts`, `web/src/idiomas/en.ts`

- [ ] **Paso 1: pruebas que fallan** en `ejemplos.test.ts`: con `prueba.archivos = ['informe.pdf']`, el ejemplo de OpenAI
  en Python contiene `base64` y `"type": "file"`, el de Anthropic contiene `"type": "document"`, y ninguno tiene `nota`;
  el ejemplo nativo en curl contiene `/api/v1/cargas` y `informe.pdf`.
- [ ] **Paso 2:** `npm --prefix web test -- ejemplos` → FALLA.
- [ ] **Paso 3: implementar.**
- [ ] **Paso 4:** `npm --prefix web test` y `npm --prefix web run build` → PASA.
- [ ] **Paso 5: navegador:** en Mis APIs, «Cómo usarla» lleva a los ejemplos de la API propia; «Descargar OpenAPI» sigue
  funcionando desde la línea Avanzado; en tema claro y oscuro y a 375 px de ancho. Captura.
- [ ] **Paso 6: commit** `feat(web): «Cómo usarla» en la API propia y ejemplos con archivos en las tres APIs`.

### Tarea 9: Paquetes con uv, paquete del conversor y CI

**Archivos:**
- Crear: `scripts/uv.ps1` (`$VersionUv = '0.12.23'`, tabla objetivo → SHA-256 y `Obtener-Uv -Objetivo <t> -Destino
  <dir>` que descarga de `https://github.com/astral-sh/uv/releases/download/<v>/uv-<t>.{zip|tar.gz}` con tres intentos,
  verifica el hash y extrae solo `uv`/`uv.exe`; si algo falla, `throw`)
- Modificar: `scripts/compilar.ps1` (portable con `uv.exe`), `scripts/instalador/tramtai.nsi` (instala y desinstala
  `uv.exe`), `scripts/linux/nfpm.yaml` (`/usr/lib/tramtai/uv`), `scripts/linux/empaquetar.sh` y
  `scripts/macos/empaquetar.sh` (`.tar.gz` y `.dmg` con `uv`), `scripts/paquetes.ps1` (la fórmula instala
  `(lib/"tramtai").install "uv"`), `scripts/linux/probar-paquete.sh` y `scripts/macos/probar-app.sh` (comprueban `uv`)
- Modificar: `.github/workflows/ci.yml` (job `conversor`, matriz windows/ubuntu/macos: `Obtener-Uv` y
  `TRAMTAI_UV=… go test -tags conversor -run TestConversorReal -v ./internal/documentos`)
- Modificar: `.github/workflows/publicar.yml` (cada job de empaquetado llama a `Obtener-Uv`; job nuevo
  `conversor-windows`: `tramtai conversor preparar --en <dir>/conversor`, prueba sin red y `Compress-Archive` a
  `Tramtai-<versión>-conversor-windows-x64.zip` con un `LEEME.txt` es/en; `publicar` lo incluye en `SHA256SUMS`)
- Crear: `scripts/conversor-leeme.txt`

- [ ] **Paso 1: hashes.** Para cada objetivo, descargar `uv-<t>.<ext>.sha256` del release 0.12.23 y copiarlo a la
  tabla de `scripts/uv.ps1`. Comprobar: `pwsh -c ". scripts/uv.ps1; Obtener-Uv -Objetivo x86_64-pc-windows-msvc -Destino
  $env:TEMP/uvp"` → existe `uv.exe` y `uv.exe --version` dice `0.12.23`. Con un hash cambiado a mano → falla con el
  mensaje del hash.
- [ ] **Paso 2: paquetes locales.** `scripts/compilar.ps1 -Portable` → el `.zip` portable trae `uv.exe`. El instalador
  NSIS compilado en local instala `uv.exe` junto a `tramtai.exe` y lo quita al desinstalar.
- [ ] **Paso 3: prueba sin red del paquete del conversor** (paso del job): extraer el zip en una carpeta nueva junto a un
  `tramtai.exe` copiado, con `UV_OFFLINE=1`, `HTTPS_PROXY=http://127.0.0.1:9` y `HTTP_PROXY=http://127.0.0.1:9`, y
  convertir `internal/documentos/testdata/muestra.pdf` con `python -m markitdown` del paquete y con `tramtai conversor
  estado` (debe decir `listo`, origen `paquete`) → la salida contiene `Tramtai muestra`.
- [ ] **Paso 4: CI.** Push de la rama y PR (no borrador): el job `Conversor` pasa en los 3 sistemas. Los 6 checks
  obligatorios siguen en verde.
- [ ] **Paso 5: commit** `build: uv fijado en todos los paquetes, paquete del conversor para Windows y prueba real en CI`.

### Tarea 10: Documentación y Ayuda

**Archivos:**
- Modificar: `web/src/paginas/Ayuda.vue` (sección «El recorrido de un archivo» con un diagrama que reutiliza
  `Ruta.vue`, y por cada puerta un ejemplo de `ejemplos.ts` con PDF), `web/src/idiomas/{es,en}.ts`
- Modificar: `site/ayuda.html`, `site/index.html` (lo mismo en la web del producto, es/en)
- Modificar: `docs/api.md` («¿Qué URL uso?»: archivos sí en las tres, nota de ≈12 MB; rutas `/conversor`; tabla de
  errores con `conversor_no_disponible` 503; sección de archivos por fachada con un ejemplo por puerta; OpenAPI como
  opción avanzada tras los pasos), `docs/inicio.md` (sin `uv tool install`; «Preparar archivos» y el paquete del
  conversor), `docs/operacion.md` (red de empresa: proxy, `UV_NATIVE_TLS`, paquete sin conexión;
  `conversion.preparar_al_iniciar`), `docs/arquitectura.md` (`internal/documentos`: localizar y preparar), `README.md`
- Crear: `docs/equipo/conversor.md` (regenerar `conversor.lock`; cambiar markitdown o uv: `conversor.in`,
  `scripts/uv.ps1` y sus hashes, prueba real)
- Modificar: `docs/equipo/README.md` (enlace al plan y a `conversor.md`), `docs/equipo/pendientes.md` (v0.50 ✅ al
  publicar; filas nuevas: SDK oficial de la API propia; paquetes base y Pro)

- [ ] **Paso 1:** escribir los cambios. Comprobar con `grep -rn "uv tool install" README.md docs site web/src` → sin
  resultados fuera de `docs/equipo/conversor.md`.
- [ ] **Paso 2:** `npm --prefix web test` y `npm --prefix web run build` → PASA; la sección nueva de la Ayuda se ve en el
  navegador (es y en, claro y oscuro). Captura.
- [ ] **Paso 3: commit** `docs: el recorrido de un archivo por cada API y markitdown incluido`.

### Tarea 11: Verificación final y PR

- [ ] **Paso 1:** `go vet ./... && go test ./...` → PASA; `go test -tags conversor -run TestConversorReal ./internal/documentos`
  → PASA.
- [ ] **Paso 2:** `npm --prefix web test && npm --prefix web run build` → PASA; la Web UI compilada actualizada en
  `internal/recursos` como en los releases anteriores.
- [ ] **Paso 3: navegador, de punta a punta**, con el servicio compilado de la rama:
  - Probar con un PDF por la API nativa → respuesta que cita el PDF;
  - script Python con el SDK `openai` (`base_url` a Tramtai) enviando el PDF con `file_data` → respuesta;
  - script Python con el SDK `anthropic` enviando un `document` → respuesta.
- [ ] **Paso 4:** `graphify update .`; revisión de la rama (superpowers:requesting-code-review). Los Minor se arreglan
  en la rama.
- [ ] **Paso 5:** PR no borrador hacia `main` con resumen es/en y `gh pr merge --auto --merge --delete-branch`.
- [ ] **Paso 6:** tras la fusión, `git pull` y `scripts/version.ps1 -Etiquetar -Resumen … -ResumenEn …` con notas cortas
  para usuarios (archivos por las tres APIs, markitdown incluido, paquete del conversor), y `git push origin <tag>`.
  Comprobar que el release trae `Tramtai-…-conversor-windows-x64.zip` en `SHA256SUMS`.
