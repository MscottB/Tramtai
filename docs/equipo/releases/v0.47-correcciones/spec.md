# Correcciones de la revisión (v0.47)

Diseño del 2026-10-19. El 2026-10-19 cinco revisores leyeron el repositorio completo:
- arquitectura;
- Go del núcleo;
- Go de la CLI, el escritorio y los agentes;
- Web UI;
- repo y CI.

Su veredicto fue que el núcleo está bien diseñado y que los problemas están en los bordes. Este release arregla los
**errores reales y de seguridad** que encontraron. Cada hallazgo de esta lista se comprobó leyendo el código.

Decisiones del usuario:
- **Orden de la hoja de ruta:**
  1. v0.47, estas correcciones;
  2. v0.48, la portable de verdad (`releases/v0.48-portable/spec.md`);
  3. v0.49, orden del código: dividir archivos grandes, centralizar los puntos de acceso en el cliente, contrato
     cliente/servidor, CI reutilizable, índice de `docs/equipo/`, pruebas de Codex, Claude y la Web UI, y rendimiento;
  4. v0.50, OpenCode;
  5. v0.51, Hermes Agent.
- **Lo que queda fuera:** lo que solo es estructura, estilo o rendimiento va a la v0.49. Aquí entra solo lo que hoy puede
  fallar, perder datos o abrir una puerta.

Las notas de versión serán cortas y sin detalles internos, porque el repo es público.

## 1. Seguridad de la publicación

### 1.1 Permisos de Publicar

**Problema:** `publicar.yml` da `contents: write` a todos sus jobs. Varios de esos jobs ejecutan código descargado:
`go install nfpm` y `wails3`, `choco install nsis`, `npm ci` y las propias pruebas. Además, `actions/checkout` deja el token
en `.git/config`.

**Cambio:**
- `permissions: contents: read` a nivel del workflow;
- `contents: write` solo en los jobs `publicar` y `paquetes`;
- `persist-credentials: false` en los checkouts de los jobs que no hacen push.

### 1.2 wingetcreate fijado

**Problema:** hoy se descarga `https://aka.ms/wingetcreate/latest` y se ejecuta con `WINGET_TOKEN`, que está en el entorno
de todo el job.

**Cambio:**
- se fija una versión concreta y se comprueba su SHA-256 antes de ejecutarla;
- `WINGET_TOKEN` pasa solo al paso que envía el manifiesto.

**Lo que haces tú:** es un paso de tu cuenta y la spec no lo hace. Conviene cambiar `WINGET_TOKEN` por un token
fine-grained limitado a tu fork de `winget-pkgs`. El clásico con `public_repo` puede escribir en todos tus repos
públicos.

### 1.3 Actualizaciones firmadas

**Problema:** la actualización de un clic comprueba que el instalador coincide con `SHA256SUMS.txt`, pero los dos vienen
del mismo release. Quien pudiera escribir en los releases cambiaría ambos, y el instalador se ejecuta en silencio.

**Cambio:**
- **Firma:** Publicar firma `SHA256SUMS.txt` con ed25519 y sube `SHA256SUMS.txt.sig`.
- **Clave privada:** es un secreto del repo (`FIRMA_ACTUALIZACIONES`). Se genera en local, tú apruebas el
  `gh secret set` y no se guarda en ningún archivo.
- **Claves públicas:** van incrustadas en `internal/actualizacion` como una lista. Una firma vale si la verifica
  cualquiera de ellas; así se puede cambiar de clave sin dejar atrás a las copias instaladas. Publicar comprueba la
  firma con esas mismas claves (`scripts/firmar -comprobar`) antes de crear el release, en un job sin caché de Go.
- **Al instalar:** `Instalar` exige una firma válida antes de usar las sumas. Sin firma, o con una firma inválida, no
  instala y muestra el aviso con el enlace de descarga.
  - No se acepta una versión sin firma, porque entonces bastaría con quitarla.
  - Desde la v0.47 todo release va firmado, y el cliente solo actualiza hacia versiones más nuevas.
  - La v0.46.1 no comprueba firmas. La primera actualización real que se verifica es la de la v0.47 a la v0.48. Hasta
    entonces se prueba con un servidor de releases falso (`httptest`), con firma válida, inválida y ausente.
- **Fuera de alcance:** Homebrew, winget, `.deb`/`.rpm` e `instalar.sh` siguen con sus sumas. Lo que se firma es la
  actualización automática de Windows.

### 1.4 Comprobaciones que hoy no paran un release

- **Web UI compilada:** el job `web` del CI hace `git diff --exit-code -- internal/recursos/web` después del build. Así no
  se puede etiquetar con la Web UI compilada vieja (pasó entre `a371c09` y `ed58ff5`).
- **wails3 obligatorio en el CI:** `compilar.ps1` falla, en lugar de avisar, si falta wails3 con `-Instalador` o con
  `CI` definido. Sin wails3, el `.exe` sale sin icono ni versión.
- **govulncheck en el escritorio:** el CI pasa govulncheck también a `escritorio/`, como ya hace Publicar.

## 2. Antigravity sin herramientas

**Problema:** el hook que niega las herramientas se registra con la ruta absoluta de `denegar.cmd` dentro de `%TEMP%`,
sin comillas. Con un usuario de Windows con espacios (`C:\Users\Ana María\…`), cmd puede no encontrarlo. La segunda
barrera solo corta después de que la herramienta se ejecutó.

**Cambio:**
- **Prueba:** una prueba con `TMP` en una carpeta con espacios y un `agy` falso que lea `hooks.json` y lance el
  `command` como lo haría cmd. Debe recibir el `deny`.
- **Arreglo:** si falla, el guion se escribe con su nombre corto (`windows.GetShortPathName`) o en una carpeta sin
  espacios dentro de los datos de Tramtai.
- **El canario real** con `agy` queda para cuando haya `GEMINI_API_KEY`, como ya está anotado.

## 3. Puntos de acceso: las mismas reglas en la Web, la CLI y la TUI

- **Consumo heredado:** la TUI borra la clave de una API **después** de recargar (`tui.go`). El servicio solo libera el
  consumo si la clave ya no existe al recargar, así que una API nueva con el mismo nombre hereda el consumo diario y
  mensual del presupuesto de la anterior.
- **Validación incompleta:** `tramtai acceso crear|editar` guarda sin `ValidarParaApps` ni `validarRespaldo`. Por eso
  puede guardar una API con Antigravity que después rechaza todas sus peticiones. La TUI no aplica `validarRespaldo`.

**Cambio:**
- `servicio.ConReglas(editar)` envuelve cualquier edición con `ValidarPunto` (respaldo con el que funcione y agentes
  habilitados para apps) en los puntos nuevos o que cambian algo más que `activo`. Lo usan la CLI y la terminal; la Web
  aplica las mismas funciones en sus rutas. No se metió dentro de `EditarPuntos` porque `Normalizar` necesita la
  instancia y el puerto del servicio, que solo conoce quien llama;
- una sola `servicio.EliminarPunto(datos, ruta, origen, nombre)` quita el punto del archivo y después borra su clave
  (404 si no estaba). La usan la Web, la CLI y la TUI, que después recargan. La Web, además, pone a cero su consumo con
  `EliminarConsumo`;
- se añade una prueba de borrar y volver a crear con el mismo nombre, que empieza con consumo cero.

Que la CLI y la TUI pasen por la API del servicio es el cambio grande y va a la v0.49.

## 4. Configuración sin cambios perdidos

- **`editarYAML` (`puntos.go`):**
  - **Problema:** escribe directamente sobre `config.yaml`, valida después y restaura si falla. Mientras tanto, otro
    lector ve un YAML a medias, y dos ediciones a la vez se pisan; la restauración puede borrar el cambio de la otra.
  - **Cambio:** valida los bytes en memoria antes de escribir (`nucleo.CargarCapa` admitirá bytes), escribe en un
    temporal y lo renombra, y un `sync.Mutex` de proceso serializa leer, modificar y escribir.
- **Escritura atómica:**
  - **Problema:** `escribirJSONArchivo` (`servicio.go`) usa siempre `ruta + ".tmp"`, y dos escrituras del descriptor a la
    vez chocan en ese archivo.
  - **Cambio:** una sola `escribirAtomico(ruta, datos, permisos)`, con temporal único y limpieza, para el JSON, los
    secretos y el YAML. Sustituye a `escribirJSONArchivo` y al patrón de `guardarSecreto`.
- **`Recargar`:**
  - **Problema:** lee la configuración fuera del lock y la aplica dentro, así que dos recargas pueden aplicar la más
    vieja.
  - **Cambio:** un `muRecargar` serializa la función completa.
- **`Configuracion.Fijar`:**
  - **Problema:** carga, copia y guarda sin comparar; dos ajustes a la vez dejan en memoria solo uno.
  - **Cambio:** un bucle con `CompareAndSwap`.
- **Ruta de la base de datos:**
  - **Problema:** el DSN de SQLite se arma concatenando la ruta (`almacen.go`). Una carpeta con `?`, `#` o `%` rompe los
    pragmas, y con la portable de la v0.48 esas rutas serán más frecuentes.
  - **Cambio:** la ruta se escribe con `%`, `?` y `#` escapados (`rutaURI`).

## 5. Trabajos que no quedan colgados

- **Resolver modelos:**
  - **Problema:** al resolver el modelo concreto (`nativa.go`, `ModeloConcreto`) se quita la cancelación y también el
    plazo. Si el CLI de Claude se cuelga antes de `init`, su proceso y su goroutine viven para siempre, y cada
    `GET /api/v1/modelos` con la caché vacía lanza otro.
  - **Cambio:** un plazo de 60 s dentro de la goroutine, y las resoluciones en curso se comparten entre peticiones
    (una por agente y modelo).
- **Fallos de la base de datos en el planificador:**
  - **Problema:** si la base falla al cambiar de estado (disco lleno, base ocupada), `planificador.cambiar` devuelve
    falso sin registrar nada, y el trabajo queda en `ejecutando` con latidos sin fin. Lo mismo pasa con los
    `_ = AgregarEvento(...)`.
  - **Cambio:** registrar en `Registros` todo error que no sea `transicion_invalida`, y cerrar el trabajo como
    `interrumpido` con un intento de mejor esfuerzo.
- **Fachadas OpenAI y Anthropic:**
  - **Problema:** cualquier error al seguir el trabajo (también uno de la base) se trata como una desconexión del
    cliente: se cancela el trabajo y, sin streaming, se devuelve un 200 vacío.
  - **Cambio:** solo es desconexión si `r.Context().Err() != nil`. En otro caso se devuelve el error en el formato de
    cada fachada.
- **`Servicio.activar`:**
  - **Problema:** abre la base (migraciones, `VACUUM`), termina huérfanos y recupera trabajos con el lock de escritura
    tomado, así que bloquea todas las peticiones de todas las instancias.
  - **Cambio:** el lock solo cubre comprobar e insertar, y la apertura va fuera, compartida por quien espere la misma
    instancia.
- **Goroutines sueltas:**
  - **Problema:** `LimpiarDescargas` y la purga tras cambiar la retención no tienen dueño, y la purga puede correr con la
    base ya cerrada.
  - **Cambio:**
    - `LimpiarDescargas` va en un `sync.WaitGroup` del servicio que `Cerrar` espera;
    - la purga tras cambiar la retención la hace el bucle de purga de la instancia (`purgarPronto`), que `cerrar`
      (Desactivar y Cerrar) ya espera;
    - la comprobación de actualizaciones no toca las bases y sigue aparte.
- **`tramtai ejecutar --directo`:**
  - **Problema:** no purga nunca `cli.db` ni `cargas/cli` (en contra de `retencion`).
  - **Cambio:** al terminar aplica la retención a todas las instancias de `cli.db` con la misma función que el servicio
    (`servicio.PurgarBase`). Además, un segundo Ctrl+C sale aunque el trabajo no confirme la cancelación.
  - **Lo que no aplica:** el presupuesto, `retencion.max_trabajos` y la ventana de idempotencia de la instancia. Cada
    ejecución directa es una instancia nueva (para no relanzar trabajos de otra) con un solo trabajo, y no manda
    `Idempotency-Key`. Sí aplican los plazos de `retencion`.
  - **Segundo Ctrl+C:** detiene el planificador (que termina el árbol del agente, con su propio grupo en Unix) con un
    tope de 5 s y sale con código 130.

## 6. Errores pequeños del Go

- **Cargas:**
  - `Cargas.purgar` no toma el cerrojo que sí toma `Guardar`: una carga resubida mientras corre la purga puede quedar
    borrada.
  - Una carga repetida renombra igualmente su temporal encima del contenido, lo que falla en Windows si el archivo está
    abierto. Si ya existe, no se renombra.
- **Conversor de documentos:** el plazo se comprueba con `errors.Is`, y solo con el contexto del conversor, para no
  echarle la culpa del plazo de otro.
- **Procesos en Windows:** si `reanudar` no encuentra ningún hilo, devuelve un error en lugar de dejar el proceso
  suspendido.
- **Procesos en Linux:**
  - **Problema:** el tope de memoria se aplica después de `Start`, así que los hijos que se crean antes escapan.
  - **Cambio:** el hijo se lanza con `/bin/sh -c 'ulimit -v …; exec "$@"'`, de modo que el tope rige desde el primer
    instante para él y para lo que lance, con su mismo PID y grupo. Los argumentos van como posicionales, nunca dentro
    del guion. No hace falta un comando oculto de `tramtai`.
- **Registro:** si la rotación no puede abrir el archivo nuevo, se vuelve a abrir el anterior en modo añadir. Así no se
  pierde el registro sin aviso.
- **Navegador:** `navegador.Abrir` espera al proceso en segundo plano, para que no quede zombi en Unix.
- **Arranque automático en Linux:** la línea `Exec` escapa `"`, `` ` ``, `$`, `\` y `%`, como pide freedesktop.
- **TUI:** el flujo de eventos usa un contexto que se cancela al salir o con Esc.
- **Errores tragados:**
  - `guardarActivas` en `admin.go` devuelve su error, como ya hace `web.go`;
  - un `instancias.json` dañado se registra y se conserva como `.dañado`, en lugar de quedarse solo con la global sin
    aviso;
  - los errores de los `PRAGMA` de mantenimiento se registran.
- **`Idempotency-Key`:** el límite de 255 caracteres pasa a `Servidor.aceptar`, así que también lo cumplen las fachadas.
- **Cuerpos lentos:** cada lectura de cuerpo tiene un plazo de lectura (`ResponseController.SetReadDeadline`), mayor para
  las cargas. Un cuerpo de 16 MB ya no puede llegar a cuentagotas.

## 7. Web UI

- **Sesión:**
  - **Problema:** si varias peticiones reciben 401 a la vez, cada una abre una sesión nueva. El token CSRF que queda
    puede no ser el de la cookie, y la siguiente acción falla.
  - **Cambio:** una sola renovación compartida.
- **Probar:**
  - si un trabajo termina después de abrir otro, sus datos ya no pisan los del nuevo; `terminar(id)` comprueba que `id`
    sigue siendo el trabajo mostrado;
  - si la página se cierra mientras carga, no se abre la conexión en vivo;
  - la salida en vivo deja de anunciar cada fragmento al lector de pantalla: lleva `aria-busy` y el estado final va en un
    `role="status"` aparte.
- **`EditorReceta`:** no borra el esfuerzo guardado cuando el catálogo no cargó.
- **Enlace de las notas de versión:** el «Ver todos los cambios» solo acepta `https://`, como el botón de descarga.
- **Textos (`t()`):** con una clave que no existe devuelve la propia clave, en lugar de `undefined`.
- **Progreso de la actualización:** el sondeo no lanza una consulta nueva mientras la anterior no termina.
- **Clave de una API nueva:** si no se encuentra la API recién creada, la clave se muestra en un aviso fijo con botón de
  copiar, y no en uno que se cierra a los 2 s.
- **Tipos:** `Solicitud.mensajes` pasa a ser opcional, como en Go, porque la retención lo borra.
- **Accesibilidad:**
  - los diálogos de Accesos, Probar e Historial tienen `aria-labelledby`;
  - los buscadores de Historial y Ajustes llevan una etiqueta;
  - el identificador que se copia en Historial pasa a ser un botón que se alcanza con el teclado.
- **Idioma:**
  - se traducen `'API '`, `tokens` y `'OK'`;
  - las fechas de Ajustes usan el idioma de la interfaz y no el del navegador.

## 8. Bandeja: «Mis APIs» con muchas APIs

Añadido por el usuario a este release.

**Problema:** hoy el submenú «Mis APIs» de la bandeja (`escritorio/main.go`, `construir`) añade todas las APIs, sin
límite. Con decenas de APIs, Windows muestra un menú más alto que la pantalla, con flechas de desplazamiento, y las que
necesitan atención quedan perdidas. Además, si falla activar o desactivar una desde ahí, el error se descarta
(`_ = cli.ActivarPunto`) y no pasa nada visible.

**Cambio:**
- **Cuántas se muestran:** como mucho 15.
- **Orden:**
  1. primero las que tienen error;
  2. después las activas;
  3. por último las desactivadas.

  Dentro de cada bloque, por nombre.
- **Si hay más de 15:** debajo va «Ver las N restantes en Mis APIs…», que abre Mis APIs. El título sigue diciendo
  «Mis APIs (X de Y activas)» con los totales.
- **Fallos al cambiar una:** se abre la ventana y se muestra el error con el mismo aviso de la Web UI. Se hace con
  `tramtaiAviso`, un puente nuevo que se registra en `App.vue` junto a `tramtaiAcercaDe` y los demás.
- **La regla de orden y recorte** es una función pura (`menuApis(puntos, maximo)`) con su prueba: con 3, con 15 y con
  40 APIs, y con errores mezclados.

## 9. El mapa del sitio

Añadido por el usuario a este release. El mapa de la portada de `site/` llevaba `127.0.0.1` bajo Tramtai, y la dirección
es cosa de la documentación. Además, se veía peor que el mapa de Inicio de la app.

**Cambio:**
- el mapa del sitio usa la geometría de `MapaRed.vue`: tus apps a la izquierda, la cápsula de Tramtai («en tu equipo») y
  una estación de color por agente, con su proveedor debajo;
- en el móvil se dibuja en vertical, como en la app.

## 10. Pruebas

Cada arreglo lleva su prueba:
- **Paquetes Go afectados:**
  - edición concurrente del YAML sin cambios perdidos;
  - borrar y volver a crear un punto;
  - plazo de `ModeloConcreto` con un CLI falso que no responde;
  - `cambiar` con una base cerrada;
  - fachada con un error interno;
  - DSN con `#` y `%`;
  - carrera entre purgar y guardar una carga;
  - registro con un fallo al rotar;
  - escapado del `Exec` en Linux;
  - tope de memoria heredado en Linux, solo en el CI de Linux;
  - firma válida, inválida y ausente;
  - hook de Antigravity con espacios.
- **Web:** pruebas de `node --test` para la renovación compartida de la sesión, para `t()` sin clave y para la regla
  `https://` del enlace. Las demás esperan a que la v0.49 separe `api.ts`.
- **CI:** el paso de la Web UI compilada, y la firma verificada en Publicar antes de crear el release.
- **A mano:**
  - Probar con dos trabajos seguidos;
  - borrar y volver a crear una API en la TUI.
