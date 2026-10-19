# Archivos (v0.50)

Diseño del 2026-10-19, acordado con el usuario por secciones. Hoy markitdown 0.1.8 (un programa en Python) se instala
aparte con `uv tool install "markitdown[pdf,docx,pptx,xlsx]==0.1.8"`; si falta, Tramtai prueba `uvx` y, si tampoco está,
el trabajo **falla al convertir**, sin aviso previo. Además, solo la API nativa acepta archivos: las fachadas OpenAI y
Anthropic responden `parametro_no_soportado` («solo texto») a cualquier parte que no sea texto.

Medido en el equipo de desarrollo: markitdown con PDF, Word, PowerPoint y Excel ocupa **234 MB instalado** (pandas,
onnxruntime por magika, numpy, sympy…). Los paquetes de Tramtai pesan hoy de 7 a 28 MB, en 6 variantes.

Decisiones del usuario:
- **Todos los paquetes traen `uv`** y Tramtai prepara markitdown en su propia carpeta: solo la primera vez al iniciar,
  en segundo plano, y además siempre con el botón «Preparar archivos». No se instala nada en el sistema, así que vale en
  equipos de empresa sin permiso para instalar programas.
- **Un paquete aparte con el conversor ya listo** para Windows x64 (`…-conversor-windows-x64.zip`), para equipos cuya
  red bloquea la descarga. Un solo job más en Publicar.
- **Las tres puertas aceptan archivos**: la API nativa, la fachada OpenAI y la fachada Anthropic, con el mismo recorrido.
- **Aviso antes de enviar**, nunca un error a mitad del trabajo.
- **En este release también** (pedido a mitad del diseño): en Mis APIs, «Descargar OpenAPI» deja de ser el botón de la
  API propia, porque confunde (es un archivo para programas, no para personas). El botón pasa a ser «Cómo usarla» (§4).

Fuera de alcance (a la hoja de ruta):
- Imágenes (markitdown no las convierte en texto útil sin otro modelo).
- SDK oficial de la API propia en Python y TypeScript, sobre `openapi.json`: release propio.
- `file_id` y una API `/files` al estilo de OpenAI.
- Paquetes del conversor para Linux y macOS (allí suele haber red y Python; se suma si alguien lo pide).
- Paquetes base y Pro: más adelante, con la Pro/Team.

## 1. Cómo se incluye y se prepara markitdown

### Lo que va en cada paquete

- El binario de **`uv`** de su sistema y arquitectura, de la versión fijada en Publicar (la misma para todos), descargado
  de su release oficial con el **SHA-256 fijado**, como NSIS. Va junto a `tramtai.exe` en Windows (instalador y
  portable) y junto a `tramtai` en los `.tar.gz` y el `.dmg`; donde el binario va en una carpeta compartida (`.deb`/`.rpm`
  en `/usr/bin`, Homebrew en `bin`), en `../lib/tramtai/uv` desde esa carpeta (`/usr/lib/tramtai/uv`), para no chocar
  con otro `uv`. Tramtai lo busca en ese orden: junto al ejecutable, `../lib/tramtai/` y el PATH. Suma unos 15–18 MB
  comprimido. En Linux, la variante `musl` (estática, vale en cualquier distribución).
- **`conversor.lock`** incrustado en el binario (`go:embed`): todas las dependencias de `markitdown[pdf,docx,pptx,xlsx]==0.1.8`
  con sus hashes, generado con `uv pip compile --universal --generate-hashes` y guardado en el repo. Lo que se descarga es
  siempre lo mismo; una versión nueva de pandas u onnxruntime no se cuela. `onnxruntime<1.20`: la 1.20 solo trae ruedas
  para macOS 13+ y Tramtai admite macOS 11 (lo destapó la revisión final).

### Dónde queda

En la carpeta local del usuario, `<carpeta>/conversor/` (`documentos.RaizConversor`): `%LOCALAPPDATA%\tramtai` en
Windows, para no cargar un perfil móvil con ~300 MB de Python; `$XDG_DATA_HOME/tramtai` o `~/.local/share/tramtai` en
Linux, fuera de `~/.config`; la carpeta de Tramtai en macOS; y en la portable, `datos-tramtai/` (cambio tras la
revisión final de la rama):

```
conversor/
  python/        Python independiente (python-build-standalone, el que descarga uv) con markitdown instalado dentro
  listo.json     {"markitdown": "0.1.8", "lock_sha256": "…", "python": "3.12.x", "preparado_en": "…"}
```

**Sin `venv`:** un entorno virtual guarda la ruta absoluta de su Python y deja de funcionar si se mueve la carpeta. Se
instala dentro del Python independiente, que sí se puede mover: así sirve el paquete del conversor (hecho en el CI) y la
portable llevada a otro equipo. Se ejecuta con `python -m markitdown <archivo>`.

### Cómo se prepara

1. Se trabaja en `conversor.nuevo/` (se borra antes si quedó de un intento anterior).
2. `uv python install 3.12 --install-dir conversor.nuevo/descarga --no-bin --no-registry` (sin accesos en el PATH ni
   registro de Windows) y se mueve el intérprete a `conversor.nuevo/python`.
3. `uv pip install --python conversor.nuevo/python/… --break-system-packages --require-hashes --no-deps -r conversor.lock`
   (el Python de uv se marca como gestionado; aquí es de Tramtai), con la caché de uv en una carpeta temporal que se
   borra al final.
4. Se comprueba convirtiendo un HTML mínimo incrustado.
5. Se escribe `listo.json` y se renombra `conversor.nuevo/` a `conversor/` (sustituyendo el anterior). Un corte a medias
   nunca deja un conversor que parezca listo.

Variables de uv: `UV_NO_CONFIG=1` (no lee la configuración del usuario), `UV_PYTHON_INSTALL_DIR` y `UV_CACHE_DIR` en
carpetas de Tramtai, y `UV_NATIVE_TLS=1` para usar los certificados del sistema (las redes de empresa que inspeccionan
TLS). Hereda el proxy (`HTTPS_PROXY`, `NO_PROXY`) y el entorno sin secretos (`entornoConversor`).

**Cuándo:**
- **Solo, la primera vez:** al arrancar el servicio, si el estado es `falta` y `conversion.preparar_al_iniciar` es `true`
  (nuevo, `true` por defecto), en segundo plano. También si `listo.json` es de otro `conversor.lock` (una versión nueva de
  Tramtai cambió markitdown): se prepara de nuevo y mientras tanto sigue sirviendo el anterior.
- **Siempre a mano:** «Preparar archivos» (Ajustes, aviso de Probar), `tramtai conversor preparar` y la terminal. Sirve
  también para reintentar tras un error o reparar.
- **Nunca a mitad de una petición:** bajar cientos de MB haría vencer el plazo de la app.

Una sola preparación a la vez por instalación; pedirla mientras corre devuelve el estado en curso.

### Orden de búsqueda al convertir

1. `conversion.comando`, si alguien lo cambió del valor por defecto (`markitdown`): se usa tal cual.
2. El **paquete del conversor**: `conversor/listo.json` junto al ejecutable.
3. El conversor **preparado** en la raíz de usuario.
4. `markitdown` en el PATH (quien ya lo tenía instalado).

Se quita el recurso a `uvx`: lo sustituye la preparación con el `uv` incluido y la versión fijada.

### El paquete del conversor (Windows x64)

Un job de Publicar en Windows hace los pasos 1–5 con el mismo `uv` y el mismo `conversor.lock`, comprueba que convierte
las muestras (§4) **sin red** y comprime `conversor/` en `Tramtai-vX-conversor-windows-x64.zip`. Va en `SHA256SUMS` (y su
firma) como los demás. Se descomprime junto a `tramtai.exe` (instalada o portable). El `README` del zip lo explica en
español e inglés.

## 2. Estado del conversor y avisos

### Un solo estado

| Estado | Significa | Datos |
|---|---|---|
| `listo` | Hay conversor | `origen` (`configurado`, `paquete`, `preparado`, `sistema`), `version` (la de `listo.json`; vacía si es del sistema o configurado) |
| `preparando` | Se está preparando | `etapa` (`python`, `paquetes`, `comprobacion`) y `progreso` (0–100, aproximado por etapa) |
| `falta` | No hay conversor y no se está preparando | — |
| `error` | Falló la última preparación | `mensaje` legible (sin conexión, proxy, certificado, espacio en disco) |

El estado se calcula al arrancar y tras cada preparación, y se guarda en memoria: consultarlo es instantáneo. Para
`sistema` basta con `exec.LookPath`.

### En la API

- `GET /api/v1/conversor`: el estado, al momento. Para seguir el progreso.
- `GET /api/v1/capacidades`: el mismo estado en un bloque `conversor`.
- `POST /api/v1/conversor/preparar`: lanza la preparación y responde 202 con el estado. **Solo con la clave de la
  instancia** (la Web UI, la CLI y la terminal); no existe en los puntos de acceso, como `DELETE /trabajos`: una app no
  puede lanzar una descarga de cientos de MB.
- Ambas rutas en `openapi.json`.

### Aviso antes, no error a mitad

Un archivo **necesita conversor** si es HTML, PDF, Word, PowerPoint o Excel, o un ZIP con alguno de esos dentro (se mira
el índice del ZIP, sin descomprimir). Texto, Markdown, JSON, XML, CSV, YAML… no lo necesitan.

- **Al aceptar un trabajo** (las tres puertas y `POST /ejecutar`): si alguna carga necesita conversor y el estado no es
  `listo`, se rechaza al instante con **503 `conversor_no_disponible`** (código nuevo) y un mensaje que dice cómo
  prepararlo. Si está `preparando`, con `Retry-After: 60`. Nunca se acepta un trabajo que va a fallar después.
- **Subir una carga** (`POST /cargas`) sigue funcionando sin conversor: solo guarda el archivo.
- **Probar:** al adjuntar un archivo que lo necesita, si no está `listo`, un aviso **antes de enviar** con «Preparar
  archivos» y la barra de progreso; «Enviar» queda deshabilitado mientras tanto, con el motivo a la vista.
- **Ajustes → Archivos:** estado, versión, origen, «Preparar archivos» (o «Preparar de nuevo») e interruptor «Preparar
  al iniciar».
- **Inicio:** un aviso discreto mientras el estado sea `falta` o `error`, con enlace a Ajustes.
- **CLI:** `tramtai conversor estado` y `tramtai conversor preparar` (espera mostrando el progreso). `tramtai ejecutar`
  con un archivo que necesita conversor y sin conversor listo falla **antes de subirlo**, con la instrucción.
- **Terminal:** el mismo aviso al adjuntar y la opción de preparar.

## 3. Archivos por las fachadas OpenAI y Anthropic

Un archivo que llega **dentro del mensaje** (base64) se guarda como una carga de la instancia (mismo almacén, misma
cuota, mismo hash, mismo vencimiento y la misma API como dueña) y se suma a `Cargas` de la solicitud. Desde ahí el
recorrido es el de la API nativa.

| Puerta | Se acepta | Se rechaza con un mensaje claro |
|---|---|---|
| OpenAI chat (`/chat/completions`) | `{"type":"file","file":{"filename":…,"file_data":"data:<mime>;base64,…"}}` | `file_id` («Tramtai no tiene /files: envía file_data»), `image_url` |
| OpenAI responses (`/responses`) | `{"type":"input_file","filename":…,"file_data":"data:…;base64,…"}` | `file_id`, `file_url` (Tramtai no descarga URLs), `input_image` |
| Anthropic (`/messages`) | `{"type":"document","source":{"type":"base64","media_type":…,"data":…}}`; `source.type: "text"` como documento de texto; `title` como nombre | `source.type` `url` o `file`, `image` |

Reglas:
- **Nombre:** el `filename` o `title`; si falta, `documento-<n>` con la extensión del tipo MIME (`application/pdf` →
  `.pdf`, los de Office, `text/html`, `text/plain` → `.txt`, `text/markdown`, `text/csv`, `application/json`,
  `application/zip`). Un tipo que no está en la lista y sin nombre con extensión admitida da `carga_invalida`.
- **Límites:** los mismos (20 archivos por petición contando todos los turnos, `carga_mb` por archivo, cuota). Además, el
  cuerpo tiene un tope de 16 MB y base64 ocupa un 33 % más: por las fachadas caben unos **12 MB** de archivos en total.
  Se documenta; para más, la API nativa.
- **Base64 roto** o `data:` mal formado: `solicitud_invalida` con el índice del mensaje y de la parte.
- **Dónde los ve el agente:** en el bloque de documentos de la petición (como en la API nativa), no en el lugar exacto
  del mensaje. Un archivo de un turno anterior también entra. Se documenta.
- **Idempotencia y reintentos:** las cargas creadas así quedan en el trabajo como las de la API nativa, así que
  `reintentar` y la idempotencia funcionan igual.
- **Imágenes:** `parametro_no_soportado` con «Tramtai no admite imágenes todavía».
- Los errores salen en el formato de cada fachada (OpenAI con el código de Tramtai en `code`).

## 4. Documentación y pruebas

### Documentación (es/en)

- **Ayuda (app y web): «El recorrido de un archivo»**, con un diagrama (archivo → puerta → carga → markitdown → texto
  delimitado → CLI → respuesta) y, por cada puerta, un ejemplo para copiar en curl, Python y Node que envía un PDF.
- **«¿Qué URL uso?»** (Mis APIs y `docs/api.md`): «Enviar archivos» pasa a sí en las tres, con la nota de los ~12 MB de
  las fachadas; nuevo código `conversor_no_disponible` en la tabla de errores; rutas `/conversor` en la API nativa.
- **Mis APIs y «Ver como código» de Probar:** si la prueba lleva un archivo, el código lo incluye también en las fachadas.
- **«Cómo usarla» en la API propia (Mis APIs, `GuiaUrls.vue`):** sustituye al botón «Descargar OpenAPI». Abre los cuatro
  pasos que ya existen en los ejemplos (subir el archivo, crear el trabajo, seguirlo en vivo y leer el resultado) en
  curl, Python y Node, con la URL y la clave de esa API puestas y un PDF de ejemplo. «Descargar OpenAPI» queda como
  línea secundaria, «Avanzado: importar en Postman o Insomnia», con una frase que dice para qué sirve el archivo. En la
  Ayuda y en `docs/api.md`, lo mismo: primero los pasos, y OpenAPI como opción avanzada.
- **README, `docs/inicio.md`, `docs/operacion.md`, `docs/arquitectura.md` y el sitio:** markitdown deja de ser un
  requisito a mano; «Preparar archivos», el paquete del conversor y qué hacer si la red bloquea la descarga.
- **`docs/equipo/`:** cómo regenerar `conversor.lock` y cambiar la versión de markitdown o de uv; índice con la v0.50.
- **`pendientes.md`:** v0.50 hecha; filas nuevas para el SDK de la API propia y los paquetes base y Pro.
- **Notas de versión** cortas, en es/en.

### Pruebas

- **Go (`internal/documentos`, `internal/api`, `cmd/tramtai`):**
  - cada forma de archivo en las fachadas: válida, base64 roto, `data:` mal formado, sin nombre (MIME → extensión),
    tipo no admitido, más de 20 archivos, `file_id`, `file_url`, `url`, imagen;
  - el orden de búsqueda del conversor (configurado, paquete, preparado, sistema) y `listo.json` de otro lock;
  - los estados `falta` → `preparando` → `listo` y `error`, con un `uv` simulado, y que un corte a medias no deje `listo`;
  - el 503 `conversor_no_disponible` al aceptar (también un ZIP con un PDF dentro; no con un ZIP solo de texto);
  - que un punto de acceso no tenga `POST /conversor/preparar`;
  - `conversion.preparar_al_iniciar: false` no prepara.
- **Web UI:** el aviso y el botón en Probar, la sección Archivos de Ajustes con el progreso y el aviso de Inicio.
- **CI en los 3 sistemas:** un job que prepara **de verdad** el conversor con el `uv` fijado y `conversor.lock`, y
  convierte un PDF, un Word y un Excel de muestra (`internal/documentos/testdata`). Comprueba que el lock sirve en
  Windows, Linux y macOS.
- **Publicar:** construye el paquete del conversor, lo prueba sin red y comprueba que cada paquete trae `uv` con su hash.
- **Navegador:** Probar con un PDF por la API nativa y con el SDK de OpenAI y de Anthropic apuntando a Tramtai.
