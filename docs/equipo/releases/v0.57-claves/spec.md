# v0.57: claves de los espacios sin guardar

Decidido el 2026-10-26 (`docs/equipo/pendientes.md`) y diseñado con el usuario ese mismo día.

## Por qué

Tramtai no debe almacenar claves:
- **Las de los proveedores** ya van al almacén cifrado del sistema.
- **Las de las APIs por aplicación** se guardan solo como huella `sha256:`.
- **Las de los espacios,** `datos/token-global` y `datos/tokens/<id>`, se guardan hoy en claro, en una carpeta solo del usuario, y la Web UI y la terminal pueden volver a mostrarlas.

Con esta versión pasan a ser como las de las APIs: se guarda la huella y la clave se ve una sola vez, al crearla o al regenerarla.

`token-servicio` se queda en su archivo. Lo leen la CLI y la app de escritorio del mismo usuario, y el llavero no lo protegería más frente a ese usuario.

## Lo que cambia

### 1. El servicio

- **El archivo de la clave** de un espacio guarda solo `sha256:<hex>`, calculado con `api.HuellaToken` como en las APIs.
- **Migración sin romper:** al leer un archivo con la clave en claro, la convierte en huella y reescribe el archivo, como hace `LeerHuellaPunto`.
  - Las apps que ya usan esa clave siguen entrando: la misma clave da la misma huella.
  - Un archivo ilegible o truncado da error y no se toca, como hoy.
- **Espacio nuevo, sin clave:** si el archivo no existe, el espacio no tiene clave.
  - No se crea una clave que nadie conoce.
  - Nadie puede entrar en el espacio con una clave hasta que se cree la suya.
  - La Web UI (por sesión), la terminal, la CLI y la app de escritorio (por `token-servicio`) siguen entrando como hoy.
- **Validación:** `instanciaPorToken` compara la huella de la clave que llega con la de cada espacio, en tiempo constante y sin cortar el bucle, como hoy. En memoria solo queda la huella; un espacio sin clave no coincide con nada.
- **`TRAMTAI_TOKEN_GLOBAL`:** si existe la variable, el espacio global usa su valor (32 caracteres como mínimo). El servicio calcula la huella al arrancar y no guarda el valor. Esa clave no se puede crear ni regenerar desde Tramtai, como hoy.
- **Crear y regenerar son la misma operación:** se genera `tt_` más 64 caracteres hexadecimales al azar, se guarda la huella en el archivo y en memoria, a la vez y bajo el mismo cerrojo, y la clave se devuelve una sola vez.
- **Olvidar un espacio** borra su archivo, como hoy.

### 2. La API

- **`GET /api/v1/instancia/token`** (sesión de la Web UI o `token-servicio`) deja de devolver la clave. Devuelve:
  - `tiene_clave`: si el espacio tiene clave;
  - `origen`: `archivo` o `variable`, con el nombre de la variable;
  - `archivo`: la ruta del archivo, como hoy.
- **`POST /api/v1/instancia/token`** crea la clave si no hay y la regenera si ya hay. Devuelve la clave una sola vez. Con la variable de entorno responde con error, como hoy.
- **`ActivacionInstancia`** (`POST /api/v1/servicio/instancias`) deja de llevar el campo con la clave. Nadie lo usa.
- **El mensaje del 401** dice que la clave se crea o se regenera en Mis APIs › Clave del espacio.
- **Contrato:** los tipos cambian en `internal/contrato` y se regeneran para la Web UI (`go generate`).

### 3. Mis APIs › «Clave del espacio» (Web UI y terminal)

- **Sin clave:**
  - qué es la clave del espacio y para qué sirve;
  - «Crear la clave del espacio».
  - Los ejemplos de código llevan el marcador `<tu clave>`.
- **Con clave:**
  - el espacio ya tiene clave, y solo se vio al crearla;
  - «Regenerar», con la confirmación de hoy: las apps que la usan dejarán de funcionar.
  - Los ejemplos llevan `<tu clave>`.
- **Recién creada o regenerada:**
  - la clave se ve una vez, con «Copiar» y el aviso «Cópiala ahora: no se volverá a ver»;
  - mientras está a la vista, los ejemplos la llevan dentro;
  - al salir de la pestaña o de la página se pierde, y salir pregunta como al cerrar la clave de una API nueva.
- **Con la variable de entorno:** la clave viene de `TRAMTAI_TOKEN_GLOBAL` y no se puede crear ni regenerar desde aquí.
- **Terminal (`internal/tui/paginas/conectar`), las mismas cuatro situaciones:**
  - `t` crea o regenera;
  - la clave recién hecha se ve con `y` para copiarla;
  - fuera de ese momento no hay `v` («ver»), porque no hay nada que ver.
- **Inicio:** el aviso de las apps que entran con la clave del espacio (`app_con_clave_24h`) no cambia.

### 4. La CLI

- **`tramtai global token` y `tramtai proyecto token`** (y su alias `tramtai servicio token`) dicen:
  - si el espacio tiene clave;
  - de dónde viene (archivo o variable);
  - que la clave solo se ve al crearla o regenerarla, con `--rotar`.

  No imprimen ninguna clave.
- **Con `--rotar`,** crean o regeneran la clave y la imprimen una vez.
  - Si el servicio está en marcha, por el servicio; si no, escribiendo la huella en el archivo, como hoy.
  - Con la variable de entorno, el error de hoy.

### 5. Documentación y textos

Todo en español y en inglés:
- **La Ayuda:**
  - los temas Conectar y Claves;
  - la línea «En la terminal» de Mis APIs;
  - quitar que «la CLI usa la clave del espacio», que no es cierto.
- **`docs/`:**
  - `docs/api.md`: el GET y el POST de la clave;
  - `docs/operacion.md`: dónde está la clave, cómo crearla o regenerarla y qué pasa al volver a la v0.56;
  - `docs/arquitectura.md`.
- **Los textos de la Web UI y de la terminal:** `web/src/idiomas`, `internal/tui/datos` e `internal/idioma`.
- **Regenerar:** la Web UI compilada y el sitio.

### 6. Volver a una versión anterior

- La v0.56 o una anterior leería la huella como si fuera la clave, así que las apps dejarían de entrar.
- Hay que regenerar la clave con esa versión. Ya pasa así con las claves de las APIs desde que son huella.
- Lo dicen `docs/operacion.md` y las notas de la versión.

## Pruebas

- **Migración:** un archivo con la clave en claro pasa a huella, y la misma clave sigue entrando en `/api/v1`, `/openai/v1` y `/anthropic`.
- **Espacio nuevo:** no tiene clave, ninguna clave entra, y la sesión y `token-servicio` sí.
- **Crear y regenerar:** crear devuelve la clave una vez y el archivo solo guarda su huella; regenerar deja fuera la anterior.
- **El GET** no devuelve la clave en ningún caso, y dice si la hay y de dónde viene.
- **La variable de entorno:** su clave entra, en memoria solo queda la huella, y crear o regenerar dan error.
- **`ActivacionInstancia`** ya no lleva la clave.
- **Web UI y terminal:** las cuatro situaciones de «Clave del espacio» (sin clave, con clave, recién creada, variable) y los ejemplos con `<tu clave>` o con la clave recién creada.
- **CLI:** `token` sin `--rotar` no imprime ninguna clave, y con `--rotar` la imprime una vez, con el servicio y sin él.
- **Las pruebas de hoy** que usan la clave en claro como `Bearer` pasan a crear la clave con la operación nueva y usar la que devuelve.

## Fuera

- Las claves de los proveedores (ya en el almacén cifrado) y las de las APIs (ya huella).
- `token-servicio`.
- Cambiar el formato `tt_` de las claves.
