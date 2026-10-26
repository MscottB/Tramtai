# v0.57: claves de los espacios sin guardar — plan

> **Para agentes:** SUB-SKILL OBLIGATORIA: superpowers:subagent-driven-development (recomendada) o superpowers:executing-plans, tarea a tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** que la clave de cada espacio se guarde solo como huella `sha256:` y se vea una sola vez, al crearla o regenerarla, en el servicio, la API, la Web UI, la terminal y la CLI.

**Arquitectura:**
- **Servicio:** guarda la huella en el archivo y en `Instancia.huella`, y compara la huella de la clave que llega.
- **API:** el GET de la clave dice solo si hay clave y de dónde viene. El POST crea o regenera y devuelve la clave una vez.
- **Web UI y terminal:** enseñan la clave recién hecha en un sitio que pregunta antes de cerrarse.

**Tecnología:** Go (servicio, CLI, terminal Bubble Tea) y Vue/Quasar (Web UI). Los textos de la terminal se generan desde `web/src/idiomas`.

**Spec:** `docs/equipo/releases/v0.57-claves/spec.md`

## Restricciones globales

- **Formatos:**
  - Clave nueva: `"tt_" + aleatorio(32)` (64 hexadecimales).
  - Huella: `api.HuellaToken(clave)` = `"sha256:" + hex(sha256)`.
  - Archivos: escritos con `guardarSecreto` (0600, atómico).
- **Lo que no se toca:** `token-servicio` (`LeerOCrearToken`, `RotarToken` y `servicio.go:165-170`) y las claves de las APIs. Solo cambia el nombre de `RotarTokenPunto`.
- **`TRAMTAI_TOKEN_GLOBAL`:** 32 caracteres como mínimo, como hoy (`largoMinimoEntorno`).
- **Textos:**
  - Todo en español. Cada `idioma.T` nuevo lleva su inglés en `internal/idioma/en_*.go`, y cada clave nueva de `web/src/idiomas/es.ts` lleva la suya en `en.ts`.
  - Los textos de la terminal salen de `es.ts`/`en.ts` con `npm run ayuda-terminal`, desde `web/`.
- **Commits:**
  - Comentarios con la densidad y el tono del código de alrededor.
  - Cada mensaje acaba con `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
  - Nunca se sube `anti-slop/`.
- **Datos de ejemplo:** neutros.

## Revisión: lo que más puede fallar

1. **Reinicio tras migrar:** un archivo en claro pasa a huella, el servicio se reinicia (`Recargar` o un servicio nuevo sobre los mismos datos) y la misma clave sigue entrando. Prueba en la Tarea 2.
2. **`--rotar` con el servicio parado:** la clave impresa entra al iniciar el servicio después. Prueba en la Tarea 3.
3. **Olvidar un proyecto y volver a abrirlo:** sin clave, y la anterior ya no entra. Prueba en la Tarea 2.
4. **Archivo dañado:**
   - corto: error al activar, sin crear otra clave en silencio;
   - con `sha256:` y basura detrás: no entra ninguna clave y no rompe nada.

   Prueba en la Tarea 1.
5. **Esc en el diálogo de la clave recién creada (Web UI) o salir de la pestaña (terminal) sin copiarla:** pregunta antes de perderla. Comprobación en las Tareas 3 y 4.

---

### Tarea 1: lectura y creación de huellas en `claves.go`

**Archivos:**
- Cambiar: `internal/servicio/claves.go`, `internal/servicio/puntos_admin.go:107,228`, `internal/servicio/web_instancia.go:239,279`
- Pruebas: `internal/servicio/claves_test.go`

**Interfaces que deja:**
- `NuevaClave(ruta string) (string, error)`: genera una clave, guarda solo su huella y devuelve la clave. Es `RotarTokenPunto` con otro nombre; se cambian sus cuatro llamadores.
- `leerHuella(ruta string) (string, error)`:
  - con `sha256:` delante, devuelve el contenido;
  - en claro, lo convierte en huella, lo reescribe y devuelve la huella;
  - sin archivo, devuelve un error con `fs.ErrNotExist`;
  - con un archivo corto, el error de `leerSecreto`.
- `LeerHuellaPunto` pasa a ser `leerHuella` más «sin archivo → `NuevaClave`», como hoy.
- `HuellaInstancia(datos, id string, c *configuracion.Configuracion) (huella, origen string, err error)`:
  - en el global con la variable definida, devuelve `api.HuellaToken(v)` y el origen `"variable " + nombre`, con el error de largo de hoy;
  - si no, devuelve `leerHuella(RutaTokenInstancia(...))` y la ruta como origen;
  - sin archivo, devuelve `"", ruta, nil`: el espacio no tiene clave.
- `TokenInstancia` y su uso de `LeerOCrearToken` desaparecen en la Tarea 2. En esta tarea `HuellaInstancia` se añade junto a ellos.

- [ ] **Paso 1: escribir las pruebas que fallan, en `claves_test.go`**
  - `TestHuellaInstanciaMigra`: escribe `tt_`+64hex en `tokens/pry`. `HuellaInstancia` devuelve `api.HuellaToken(esa)`, y el archivo pasa a contener esa huella más `\n`. Una segunda llamada devuelve lo mismo sin reescribir.
  - `TestHuellaInstanciaSinArchivo`: sin archivo, devuelve `huella == ""`, `err == nil` y `origen == RutaTokenInstancia(dir, "pry")`, y no crea el archivo.
  - `TestHuellaInstanciaDanada`:
    - un archivo `"corta"` da error y el archivo sigue igual;
    - un archivo `"sha256:zz…"` (71 caracteres) se devuelve tal cual, sin error.
  - Variable de entorno: adaptar las pruebas actuales (`claves_test.go:66-77`) a `HuellaInstancia`. Con la clave generada de la variable, devuelve `api.HuellaToken(generada)` y el origen `"variable TRAMTAI_TOKEN_GLOBAL"`. Con `"1234"`, el error de largo.
  - `TestNuevaClave`: devuelve `tt_`+64hex y el archivo solo contiene `api.HuellaToken(clave)`.
  - `claves_test.go:16` sigue probando `LeerHuellaPunto`.

- [ ] **Paso 2:** `go test ./internal/servicio/ -run 'HuellaInstancia|NuevaClave'`. Esperado: no compila (funciones sin definir).
- [ ] **Paso 3:** implementar lo de «Interfaces que deja» y renombrar `RotarTokenPunto` a `NuevaClave` en sus llamadores.
- [ ] **Paso 4:** `go test ./internal/servicio/`. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servicio): leer y crear la huella de la clave de un espacio`.

### Tarea 2: el servicio y la API guardan y comparan solo la huella

**Archivos:**
- Cambiar:
  - `internal/servicio/servicio.go:92` (struct `Instancia`: el campo `token` pasa a `huella`) y `internal/servicio/claves.go` (`RotarTokenInstancia`, `tokenDe`, `instanciaPorToken`, quitar `TokenInstancia`);
  - `internal/servicio/instancias.go:168,203`, `internal/servicio/web_instancia.go:43-53`, `internal/servicio/admin.go:238`;
  - `internal/contrato/contrato.go:119-123` y `internal/contrato/gents/main.go:32-50` (después, `go generate ./internal/contrato`);
  - `internal/api/api.go:160` e `internal/idioma/en_servicio.go:50`.
- Pruebas: todos los `internal/servicio/*_test.go` que usan `inst.token`, `global.token` o `pry.token` (unos 60 usos en 14 archivos).

**Interfaces:**
- Usa `HuellaInstancia` y `NuevaClave` de la Tarea 1.
- Deja:
  - `contrato.EstadoClave{TieneClave bool json:"tiene_clave"; Origen string json:"origen"` (`"archivo"` o `"variable"`)`; Variable string json:"variable,omitempty"; Archivo string json:"archivo,omitempty"}`. Se añade a `raices` con el nombre `EstadoClave`.
  - `contrato.ActivacionInstancia` queda solo con `Instancia`.
  - `GET /api/v1/instancia/token` → `EstadoClave`.
  - `POST /api/v1/instancia/token` → `{"token","archivo"}`, como hoy.
  - `(s *Servicio) RotarTokenInstancia(inst) (string, error)`, la misma firma: con la variable de entorno, el error de hoy; si no, `NuevaClave(inst.ArchivoToken)` e `inst.huella = api.HuellaToken(clave)`, bajo `s.mu.Lock`.
  - `(s *Servicio) estadoClave(inst) contrato.EstadoClave`, que lee `inst.huella` bajo `s.mu.RLock`. Sustituye a `tokenDe`.
  - `instanciaPorToken(token)`: calcula `h := api.HuellaToken(token)` una vez. Compara con `subtle.ConstantTimeCompare` contra cada `inst.huella` no vacía, sin cortar el bucle. Con un token vacío, `nil`.
- El 401 dice: «falta la clave o no es válida: envíala en la cabecera «Authorization: Bearer <clave>»; se crea o se regenera en Mis APIs › Clave del espacio de la Web UI». Su inglés va en `en_servicio.go`.

- [ ] **Paso 1: ayudante de pruebas.**
  - En un `_test.go` del paquete: `claveDe(t *testing.T, s *Servicio, inst *Instancia) string`, que llama a `s.RotarTokenInstancia(inst)` y falla la prueba si da error.
  - En cada prueba que hoy envía `X.token` como Bearer: `clave := claveDe(t, s, X)` justo después de activar, y `clave` donde estaba `X.token`.
  - Las pruebas que usan la clave para comprobar que algo NO entra (`admin_ajustes_test.go:144`, `admin_espacios_test.go:195`, `web_test.go:348`, `puntos_api_test.go:82`…) también la crean primero, para que el 401 lo cause la ruta y no la falta de clave.
- [ ] **Paso 2: pruebas nuevas que fallan.**
  - `TestClaveEspacioNuevo`: un servicio nuevo con `ActivarGlobal`.
    - `POST /api/v1/ejecutar` con `tt_`+64 ceros da 401.
    - `GET /api/v1/instancia/token` con sesión da `{"tiene_clave":false,"origen":"archivo","archivo":<ruta>}`.
    - El archivo no existe.
  - `TestClaveEspacioCrearYRegenerar`:
    - el POST con sesión y CSRF devuelve una clave que entra en `/api/v1`, `/openai/v1/models` y `/anthropic/v1/messages` (el que tenga la prueba más cercana);
    - el archivo contiene solo su huella;
    - el GET da `tiene_clave:true` y su cuerpo no contiene la clave;
    - un segundo POST deja fuera la primera (401) y deja entrar la segunda.
    - Reutiliza y sustituye la prueba de `web_test.go:226-238` y la de `web_test.go:345`.
  - `TestClaveEspacioMigradaTrasReinicio` (Revisión 1): escribe `tt_`+64hex en claro en `token-global` antes de `ActivarGlobal`.
    - Esa clave entra y el archivo ya contiene la huella.
    - Tras un servicio nuevo sobre el mismo `Datos` (o `Recargar`), sigue entrando.
  - `TestClaveEspacioVariable`: con `TRAMTAI_TOKEN_GLOBAL` (64 caracteres), su valor entra.
    - El GET da `origen:"variable"` y `variable:"TRAMTAI_TOKEN_GLOBAL"`.
    - El POST da 400.
    - `token-global` no se crea.
  - `TestClaveEspacioOlvidado` (Revisión 3): un proyecto con clave creada se olvida y se vuelve a abrir. No tiene clave, y la anterior da 401.
  - Activación: la respuesta de `POST /api/v1/servicio/instancias` no tiene el campo `token`.
- [ ] **Paso 3:** `go test ./internal/servicio/`. Esperado: no compila o FAIL en las pruebas nuevas.
- [ ] **Paso 4:** implementar lo de «Interfaces», borrar `TokenInstancia`, añadir `EstadoClave` a `raices` y ejecutar `go generate ./internal/contrato`.
- [ ] **Paso 5:** `go test ./internal/servicio/ ./internal/contrato/... ./internal/api/ ./internal/idioma/`. Esperado: PASS.
  - `go build ./...` puede fallar en `cmd/tramtai` (todavía llama a `TokenInstancia`). Lo arregla la Tarea 3.
  - Las pruebas de `internal/cliente` y de la terminal que leen la clave fallan hasta la Tarea 3; se anota en el registro.
- [ ] **Paso 6:** commit `feat(servicio): la clave del espacio se guarda solo como huella`.

### Tarea 3: cliente, CLI y terminal

**Archivos:**
- Cambiar:
  - `internal/cliente/cliente_puntos.go:58-65` y `cmd/tramtai/instancia.go:37-88,163`;
  - `cmd/tramtai/servicio.go:264` e `internal/idioma/en_cli.go`;
  - `internal/tui/paginas/conectar/pagina.go` y `vista.go`;
  - `web/src/idiomas/es.ts`, `en.ts` e `internal/tui/datos/` (generados).
- Pruebas:
  - `internal/cliente/cliente_test.go:318-336`;
  - `internal/tui/paginas/conectar/conectar_test.go:424,461` y `modelo_test.go:106`;
  - una prueba de la CLI junto a las de `cmd/tramtai` (seguir el patrón de las que ya ejecutan un comando de cobra con salida a un buffer).

**Interfaces:**
- Usa `contrato.EstadoClave`, el GET y el POST de la Tarea 2, y `servicio.HuellaInstancia` y `servicio.NuevaClave` de la Tarea 1.
- Deja:
  - `(c *Cliente) EstadoClave(ctx, instancia string) (contrato.EstadoClave, error)`, que sustituye a `ClaveInstancia`.
  - `RotarClave(ctx, instancia)` no cambia.
- **Textos nuevos** en `es.ts` y `en.ts`, que la Web UI usa en la Tarea 4:

| clave | es | en |
|---|---|---|
| `conectar.sinClave` | Este espacio aún no tiene clave. Créala para conectar tus apps con la clave del espacio: solo se verá una vez. | This workspace has no key yet. Create it to connect your apps with the workspace key: it is shown only once. |
| `conectar.crearClave` | Crear la clave del espacio | Create the workspace key |
| `conectar.conClave` | El espacio ya tiene clave. Solo se vio al crearla: si no la tienes, regénerala. | The workspace already has a key. It was shown only when it was created: if you do not have it, regenerate it. |
| `conectar.claveVariable` | La clave viene de la variable {variable}: Tramtai no la guarda, ni la muestra ni la regenera. Cámbiala en esa variable. | The key comes from the {variable} variable: Tramtai does not store, show or regenerate it. Change it in that variable. |
| `conectar.claveCreada` | Clave creada: cópiala ahora | Key created: copy it now |

  - El aviso de «una sola vez» reutiliza `puntos.claveUnaVez`.
  - `conectar.mostrarClave` y `conectar.ocultarClave` se borran si nadie más las usa.
  - `claves.espacioTexto` (es) pasa a: «Una por espacio, con acceso a cualquier agente y modelo, sin límites propios. Para tus apps y scripts; se ve solo al crearla o regenerarla.» Su inglés va a juego.

**CLI:** `tramtai global|proyecto token` y `tramtai servicio token`, sin `--rotar`, imprimen una línea según `HuellaInstancia`. Cada una tiene su inglés en `en_cli.go`.
- **Sin clave:** «El espacio aún no tiene clave: créala con tramtai %s token --rotar o en Mis APIs › Clave del espacio».
- **Con clave en archivo:** «El espacio tiene clave (en %s solo se guarda su huella). Se ve solo al crearla o regenerarla: tramtai %s token --rotar».
- **Variable:** «La clave del espacio viene de la %s: Tramtai no la guarda ni la muestra».

**`--rotar`:** como hoy, pero sin servicio escribe con `servicio.NuevaClave(servicio.RutaTokenInstancia(datos, id))`. El `Short` de ambos comandos pasa a: «Dice si el espacio tiene clave; con --rotar la crea o la regenera y la muestra una vez».

**Terminal (`conectar`):**
- **Estado:**
  - `p.clave` y `p.verClave` desaparecen.
  - `p.estadoClave contrato.EstadoClave` llega en la carga completa.
  - `p.claveEspacio string` guarda la clave recién creada o regenerada.
- **Pestaña Clave:**
  - Con variable: `conectar.claveVariable`.
  - Sin clave: `conectar.sinClave`; `t` crea sin preguntar.
  - Con clave: `conectar.conClave`; `t` pregunta como hoy (`accRegenerarEspacio`).
  - Con `p.claveEspacio`: la clave, la advertencia `puntos.claveUnaVez` e `y` para copiarla.
  - La barra de teclas: ya no hay `v`; `y` solo aparece con la clave a la vista.
  - Crear sin preguntar avisa con `conectar.claveCreada`; regenerar, con `conectar.claveRegenerada`.
- **Al salir:**
  - `Pendiente()` también devuelve `cierre.clave` con `p.claveEspacio != ""`.
  - ←/→ desde la pestaña Clave con la clave a la vista preguntan, con `cierre.claveTitulo` y `cierre.clave`. Al confirmar se borra `p.claveEspacio` y se cambia de pestaña.
  - `Iniciar` con otro espacio borra `p.claveEspacio`; con el mismo, la conserva, como `claveVista`.

- [ ] **Paso 1: pruebas que fallan.**
  - `cliente_test`: `EstadoClave` empieza con `TieneClave:false`. Tras `RotarClave`, da `TieneClave:true`, y la clave devuelta entra en `/api/v1/instancia` como Bearer.
  - `conectar_test`:
    - sin clave, la pestaña dice el texto de `conectar.sinClave`;
    - `t` muestra una clave `tt_…` y `Pendiente() != ""`;
    - tras recargar, la clave sigue a la vista hasta salir;
    - ← pide confirmación y, al confirmar, la clave ya no está en la vista.
    - Las pruebas que hoy leen la clave con `ClaveInstancia` pasan a obtenerla con `RotarClave`.
  - CLI, en una carpeta de datos temporal sin servicio:
    - `token` dice «aún no tiene clave»;
    - `token --rotar` imprime `tt_`+64hex y el archivo solo tiene su huella;
    - `token` dice «tiene clave» y no imprime ninguna `tt_`.
  - CLI, Revisión 2: después, un servicio de prueba sobre esa carpeta acepta la clave impresa. Si `cmd/tramtai` no puede arrancar un servicio en pruebas, se cubre con `HuellaInstancia(datos, id) == api.HuellaToken(impresa)` y se anota la decisión.
- [ ] **Paso 2:** `go test ./internal/cliente/ ./internal/tui/... ./cmd/...`. Esperado: FAIL.
- [ ] **Paso 3:** implementar. Desde `web/`, `npm run ayuda-terminal`.
- [ ] **Paso 4:** `go build ./... && go test ./...`. Desde `web/`, `npm test`. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(terminal,cli): la clave del espacio se ve una vez`.

### Tarea 4: Web UI, Mis APIs › Clave del espacio

**Archivos:**
- Crear: `web/src/componentes/accesos/ClaveEspacio.vue`.
- Cambiar: `web/src/paginas/Accesos.vue:46-63,80,125-127,218-221,293-307,336` y `web/src/api/cliente.ts:118-120`.

**Interfaces:**
- Usa `EstadoClave` (generado en la Tarea 2) y los textos de la Tarea 3.
- `cliente.ts`: `claveInstancia` pasa a `estadoClaveInstancia: () => pedir<EstadoClave>('GET', '/api/v1/instancia/token')`. `regenerarClaveInstancia` sigue igual.
- `ClaveEspacio.vue`:
  - Recibe la prop `codigo: (clave: string) => string`, el ejemplo del lenguaje elegido.
  - Carga el estado al montarse. Lleva la columna de la clave que hoy está en `Accesos.vue:47-62`, con las mismas cuatro situaciones que la terminal:
    - error, con «Reintentar»;
    - variable;
    - sin clave, con «Crear la clave del espacio», sin preguntar;
    - con clave, con «Regenerar la clave» y el diálogo de confirmación de hoy.
  - **La clave recién hecha** se muestra en un `q-dialog persistent`, como `DetalleApi.vue:5-6,27-28`: la clave con «Copiar», la advertencia `puntos.claveUnaVez`, y `codigo(clave)` con su botón de copiar.
  - Esc usa `escaparConAviso(evento, true, cerrar, true)`.
  - Al cerrar, la clave se borra de la memoria y se vuelve a cargar el estado.
- `Accesos.vue`:
  - Los ejemplos de la página llevan siempre `t('codigo.tuClave')`, y copiar el ejemplo copia eso.
  - Se quitan `clave`, `verClave`, `ocultar`, `cargarClave` y `regenerarClaveInstancia`.

**Decisión de diseño:** la spec pide que la clave recién hecha salga en los ejemplos mientras está a la vista, y que salir pregunte. Se cumple con el ejemplo dentro del diálogo, que es como lo hace `DetalleApi` con las APIs. La página sigue con `<tu clave>`.

- [ ] **Paso 1:** implementar.
- [ ] **Paso 2:** desde `web/`, `npm run build` y `npm test`. Esperado: sin errores de `vue-tsc` y PASS.
- [ ] **Paso 3: comprobación en vivo** con la copia portable (`C:\Users\Administrator\tramtai-e2e`, binario de esta rama) en el navegador integrado:
  - sin clave, crear;
  - el diálogo muestra la clave y el ejemplo con ella;
  - Esc pregunta (Revisión 5);
  - al cerrar, «ya tiene clave»;
  - regenerar pregunta y abre el diálogo.
  - Una captura de cada estado va al informe.
- [ ] **Paso 4:** commit `feat(web): Clave del espacio se ve una vez, al crearla o regenerarla`.

### Tarea 5: Ayuda, documentación, sitio y Web UI compilada

**Archivos:**
- La Ayuda: `web/src/idiomas/ayuda.es.ts`, `ayuda.en.ts` y `web/src/ayuda/temas/claves.ts` (temas Conectar y Claves).
- `docs/`: `api.md:58,383-384`, `operacion.md:181,294` (más un apartado «Volver a la v0.56»), `arquitectura.md:139`, `desarrolladores.md` y `GLOSARIO.md`.
- `site/index.html`, si promete ver la clave; `README.md`, si la menciona.
- Generados: `site/ayuda/*`, `internal/tui/datos/*` e `internal/recursos/web/*`.

**Contenido:**
- **La Ayuda (es y en):**
  - la clave del espacio se crea o regenera en Mis APIs › Clave del espacio, o con `tramtai global|proyecto token --rotar`, y se ve una sola vez;
  - un espacio nuevo no tiene clave hasta que se crea;
  - con `TRAMTAI_TOKEN_GLOBAL` no se puede crear ni regenerar desde Tramtai;
  - corregir que «la CLI usa la clave del espacio» (`ayuda.es.ts:248`): la CLI usa `token-servicio`.
- **`api.md`:** el GET devuelve `{"tiene_clave","origen","variable","archivo"}` y nunca la clave; el POST la crea o la regenera y la devuelve una vez.
- **`operacion.md`:**
  - la tabla de `:181` dice que el archivo guarda la huella `sha256:…`;
  - «Volver a la v0.56»: esa versión leería la huella como clave y las apps darían 401; hay que regenerar la clave con `tramtai global token --rotar` en esa versión.
- **`arquitectura.md:139`:** la del espacio pasa a la fila de las huellas; `token-servicio` sigue en claro, con su motivo.

- [ ] **Paso 1:** editar. Buscar `clave del espacio`, `workspace key` y `instancia/token` en `docs/`, `web/src/idiomas`, `site/index.html` y `README.md`, y corregir lo que prometa ver o mostrar la clave más de una vez.
- [ ] **Paso 2:** desde `web/`, `npm run sitio`, `npm run build` y `npm test`. Esperado: PASS; los generados cambian.
- [ ] **Paso 3:** `go test ./...`. Esperado: PASS, incluida la comprobación de que los datos de la terminal están al día.
- [ ] **Paso 4:** commit `docs: la clave del espacio se ve una vez (Ayuda, api, operación, sitio)`.
