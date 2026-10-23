# Operación: servicio, espacios y APIs

Tramtai funciona como **un único servicio por usuario** (un proceso y un puerto, `127.0.0.1:17373`) con varios **espacios**, cada uno con su configuración, su clave, su cola, su base de datos y sus archivos subidos. La clave de cada petición decide a qué espacio llega, así que una aplicación nunca ve las peticiones de otra. Los nombres que se usan aquí están en el [glosario](GLOSARIO.md).

## La terminal interactiva

```powershell
tramtai            # sin argumentos abre la terminal interactiva (o: tramtai terminal)
```

Tiene el mismo menú que la Web UI, con los mismos nombres y textos: **Inicio**, **Agentes** y **Probar** (grupo
Empezar), **Mis APIs** e **Historial** (grupo Usar), **Espacios** y **Ajustes** (grupo Organizar) y **Ayuda**. Las
páginas Mis APIs, Espacios, Ajustes y Ayuda todavía dicen «Próximamente»: mientras tanto, usa la Web UI o la CLI.

| Tecla | Qué hace |
|---|---|
| Tab | Abre el menú (por debajo de 100 columnas está plegado). |
| 1 a 7 | Va a esa página del menú. |
| ? | La ayuda de la página, con todas sus teclas. |
| Ctrl+E | Cambia de espacio. |
| ! | Enseña entero el último error. |
| Esc | Vuelve o cierra. En Probar, una petición en marcha sigue en el servicio: `c` la cancela. |
| q | Sale de la terminal (pregunta si hay una petición en marcha). |

El pie de cada página dice sus teclas. Las acciones que afectan a otros (cancelar una petición, vaciar el historial,
desconectar un agente) piden confirmación con `s`; Enter no confirma.

## Web UI y aplicación de escritorio

```powershell
tramtai abrir                      # abre la app de escritorio (--navegador: la Web UI en el navegador); la del proyecto si estás en uno
tramtai global abrir               # la del espacio global
tramtai proyecto abrir             # la del proyecto actual
tramtai-escritorio                 # aplicación de escritorio (o -proyecto <ruta>)
```

La Web UI está en español y en inglés (el botón de idioma de la cabecera) y tiene estas páginas:

- **Inicio:** resumen de peticiones, primeros pasos pendientes y estado de los agentes.
- **Probar:** modelo del catálogo con su esfuerzo justo debajo (también el que usa el agente si no eliges uno), respaldo con un esfuerzo propio por agente, archivos, mensaje y JSON Schema opcional; muestra la respuesta en vivo, con formato Markdown o como texto (la elección se recuerda), y permite cancelar la petición. Bajo la respuesta, **Registro** da la hora de cada paso (cola, espera del agente, ejecución con qué agente, modelo y esfuerzo, avisos) y la duración de cada etapa; también está en el detalle del Historial. **Pruebas recientes** lista las 10 últimas pruebas de Probar: pulsar una la abre y **Usar de nuevo** recupera su mensaje y su receta (los adjuntos no).
- **Historial:** detalle, seguir en vivo, cancelar, reintentar, eliminar y **vaciar el historial** (las peticiones terminadas y sus archivos).
- **Mis APIs:** URL, clave y ejemplos; APIs por aplicación con grupos que se activan o detienen juntos.
- **Agentes**, **Ajustes** (arranque con el sistema, **borrar datos** y valores efectivos) y **Ayuda** (cómo empezar y preguntas frecuentes).

También se puede abrir `http://127.0.0.1:17373` directamente: en Windows, Linux y macOS entra solo si el navegador es de tu mismo usuario del sistema. Si no, la página **Entrar** explica cómo abrirla: desde la app o con `tramtai abrir`, que generan un código de un solo uso. La clave de las aplicaciones no abre la Web UI.

La aplicación de escritorio inicia el servicio si no está en marcha y muestra la misma Web UI en su ventana. Al cerrar la ventana queda en la bandeja del sistema vigilando el servicio. Clic en el icono abre la ventana; **clic derecho** abre el menú, sin abrir la app:

- **Estado**: círculo verde «Servicio en marcha · 127.0.0.1:17373» o rojo «Servicio detenido» (también al pasar el ratón por el icono).
- **Mis APIs**: las APIs de todos los espacios en marcha (global y proyectos), con un círculo verde si está activa y rojo si está detenida o con error. Un clic la detiene o la vuelve a activar; se guarda y se aplica sin reiniciar. Al final, «Gestionar en Mis APIs» abre esa sección.
- **Copiar la dirección para tus apps**.
- **Iniciar el servicio** (si está detenido), o **Reiniciar** y **Detener el servicio…** (si está en marcha). Tras detenerlo, la app sigue en la bandeja para volver a iniciarlo.
- **Iniciar con el sistema** (Windows, macOS o Linux), el mismo ajuste que en Ajustes.
- **Salir y detener el servicio…**: cierra la app y detiene el servicio, así que tus aplicaciones dejan de funcionar hasta que vuelvas a abrirla. Pide confirmación con un aviso que se puede marcar como «No volver a mostrar». Para recuperar el aviso, borra `escritorio.json` de la [carpeta de Tramtai](inicio.md#configuración).

Los avisos de Detener y Salir se muestran dentro de la ventana, con el estilo de la app.

### Arranque con el sistema

Activado en **Ajustes** o con `tramtai servicio autoarranque activar`, Tramtai se registra solo para tu usuario y, al iniciar sesión, arranca la app de escritorio con `--oculto`: inicia el servicio y queda en la bandeja sin abrir ninguna ventana ni terminal. Se quita con `tramtai servicio autoarranque desactivar`. Necesita la app de escritorio junto a `tramtai`.

| Sistema | Dónde se registra |
|---|---|
| Windows | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` |
| macOS | `~/Library/LaunchAgents/com.mscottb.tramtai.plist` |
| Linux | `~/.config/autostart/tramtai.desktop` (escritorios compatibles con XDG, como GNOME o KDE) |

### Actualizaciones

Cada hora, el servicio pregunta a GitHub cuál es la última versión publicada (solo esa ficha pública; no envía ningún dato tuyo); la ventana se lo vuelve a preguntar cada 15 minutos y al volver a ella, y la bandeja, cada pocos segundos. Así el aviso aparece sin reiniciar la app. Si hay una más nueva:

- **Acerca de Tramtai** (el menú lateral, el menú del estado o «Acerca de Tramtai» en la bandeja): es el único sitio que muestra la versión, tal como aparece en GitHub (`v0.42.0+17102026`), el desarrollador, el estado de las actualizaciones con **Buscar actualización ahora** y las novedades de esa versión en GitHub, los enlaces del proyecto (sitio, código, incidencias, licencia y términos) y **Detalles técnicos**: commit, sistema, Go, dirección y pid del servicio, almacén de claves y, en una sesión local, las carpetas de datos, configuración y registro. **Copiar datos para soporte** los copia como texto, sin claves. La Web UI los pide a `GET /ui/acerca`.
- La cabecera de la Web UI muestra **Instalar la versión nueva** y el menú de la bandeja, **Instalar la versión vX…**. **Instalar ahora** comprueba la firma de Tramtai de `SHA256SUMS.txt` (`SHA256SUMS.txt.sig`, ed25519), descarga el instalador, comprueba su suma SHA-256 con esas sumas y avisa «Tramtai se reiniciará en 10 s», con **Reiniciar ahora** y **Cancelar** (borra la descarga); al vencer lo ejecuta mostrando solo su ventana de progreso («Actualizar Tramtai»: el instalador dice Actualizar si ya hay una instalación registrada e Instalar si no), que se cierra sola: la app se cierra, el servicio se detiene tras terminar las peticiones en curso (`servicio.plazo_drenaje_segundos`) y la app vuelve a abrirse con la versión nueva, con tu configuración y tus datos. Si la versión no trae una firma válida, no se instala: el aviso muestra el enlace para descargarla desde su página.
- Para no esperar a la próxima comprobación: **Ajustes → Buscar actualizaciones → Buscar actualización ahora** o, en la bandeja, **Buscar actualizaciones**; consultan GitHub en el momento (como mucho una vez cada 10 segundos) aunque la búsqueda automática esté desactivada.
- Si instalas una versión a mano con Tramtai abierto (sea la instalada, la portable o una copia compilada), el instalador te avisa, cierra la app, detiene el servicio cuando terminan las peticiones en curso y, al acabar, vuelve a abrir lo que estaba abierto.
- Tras actualizar, la Web UI muestra una vez las novedades de cada versión desde la última que viste en ese navegador (hasta 10, la más nueva abierta), con el enlace a todas las versiones. Si la búsqueda de actualizaciones está desactivada, o GitHub no responde, solo dice la versión nueva y ofrece el enlace.
- El instalador descargado no puede borrarse mientras se ejecuta: el servicio, al arrancar, y cada descarga nueva borran las carpetas `tramtai-actualizacion*` del temporal con más de 10 minutos, así que no se acumulan.
- Desde la terminal: `tramtai actualizar` hace lo mismo (pide confirmación; `--si` la omite) y `tramtai actualizar --comprobar` solo informa.
- Solo se actualiza sola una copia instalada con el instalador de Windows; una compilada a mano o en Linux y macOS muestra el aviso con el enlace de descarga.
- Se desactiva en **Ajustes → Buscar actualizaciones** o con `servicio.buscar_actualizaciones: false`; `tramtai actualizar` sigue funcionando porque lo pides tú.

### Borrar datos

- **Historial:** «Vaciar historial» en la Web UI, o `DELETE /api/v1/trabajos`, borra las peticiones terminadas del espacio con sus eventos y las cargas que ya no usa ninguno. Lo que está en curso se conserva.
- **APIs por aplicación:** una a una en Mis APIs o con `tramtai acceso eliminar`.
- **Ajustes → Borrar datos** hace ambas cosas para el espacio actual. La configuración y las claves de proveedor guardadas no se tocan.

## Comandos equivalentes

```powershell
tramtai servicio iniciar -d        # segundo plano (sin -d: primer plano, Ctrl+C lo detiene)
tramtai servicio estado            # espacios, APIs y errores
tramtai servicio recargar          # aplica los cambios de las APIs sin reiniciar
tramtai servicio detener           # pide confirmación (-s para no pedirla); espera a lo que está en curso
tramtai servicio autoarranque [activar|desactivar]   # arrancar al iniciar sesión, oculto en la bandeja
tramtai servicio registro [-f] [-n 30]               # lo que procesa el servicio en segundo plano; -f lo sigue en vivo
tramtai servicio puerto [número] [--automatico=false] # muestra o cambia el puerto (comprueba que esté libre)
tramtai servicio iniciar --abrir   # modo desarrollo: primer plano con el registro a la vista y la interfaz abierta

tramtai global iniciar|detener|estado|token [--rotar]
tramtai iniciar                    # crea .tramtai/config.yaml en el repositorio actual
tramtai proyecto iniciar|detener|estado|token [--rotar]   # desde la carpeta del proyecto

tramtai espacios                   # los espacios de trabajo: activos, detenidos y no encontrados
tramtai espacio iniciar|detener [carpeta|id]
tramtai espacio olvidar <id>       # borra sus datos en Tramtai, nunca la carpeta del proyecto
tramtai agentes desconectar|conectar <agente>   # deja un agente fuera sin desinstalar su CLI

tramtai trabajos listar|mostrar|cancelar|reintentar|eliminar   # las peticiones; -i <espacio> para elegir otro; --global para la global
```

- `proyecto iniciar` inicia el servicio si no está en marcha.
- **¿Sigue en marcha?** `tramtai abrir` y la app de escritorio dejan el servicio en segundo plano: sigue atendiendo a tus aplicaciones aunque cierres la terminal o la ventana. La cabecera de la Web UI muestra «En marcha» o «Sin conexión», y `tramtai servicio estado` lo confirma.
- **Puerto:** `servicio.puerto` (17373). Si al iniciar está ocupado y `servicio.puerto_automatico` está activado (lo está por defecto), Tramtai usa el siguiente libre y lo avisa en el registro; la CLI y la app de escritorio lo encuentran solas (la ventana de la app vuelve a entrar en la nueva dirección en unos segundos, cambie como cambie el puerto), pero tus aplicaciones deben usar la nueva dirección. Con `servicio.puerto_automatico: false` no arranca y explica cómo elegir otro. Se cambia en **Ajustes → Puerto del servicio** o con `tramtai servicio puerto <número>`: ambos comprueban que esté libre y se aplica al reiniciar (en Ajustes, con «Reiniciar ahora»; la página sigue al servicio a su nuevo puerto).
- **Iniciar, reiniciar y detener:** desde la Web UI, el indicador «En marcha» tiene «Reiniciar el servicio» y «Detener el servicio…» (solo en este equipo y con confirmación). Iniciarlo no puede hacerse desde la página, porque es el propio servicio el que la sirve: se inicia con `tramtai abrir`, con `tramtai servicio iniciar -d` o desde la bandeja de la app de escritorio (Iniciar el servicio). La página se reconecta sola cuando vuelve.
- **Registro:** cada petición deja una línea al recibirla y otra al terminar (agente, modelo efectivo, API de origen, duración, tokens o el error). Se limita con `servicio.registro_mb` (5 MB por archivo) y `servicio.registro_copias` (2 copias: `servicio.log.1` y `.2`); con 0 copias el archivo se vacía al llenarse. En segundo plano va a `datos/servicio.log` en la [carpeta de Tramtai](inicio.md#configuración) (`tramtai servicio registro -f`); en primer plano, a la propia terminal.
- Los espacios activos se recuerdan: al reiniciar el servicio vuelven a abrirse y recuperan sus peticiones.
- Detener un espacio cancela lo que tenía en cola; lo que se estaba ejecutando queda `interrumpido`. Los demás espacios no se ven afectados.
- **Detener el servicio:**
  - Deja de aceptar peticiones y espera hasta `servicio.plazo_drenaje_segundos` (30 por defecto) a que terminen los que están en marcha. Después interrumpe los que queden.
  - Lo que estaba en cola se reanuda al volver a iniciarlo.
  - En primer plano, un segundo Ctrl+C corta la espera.
- **Resultados:** se conservan 24 horas (`retencion.horas_resultado`); después la salida responde 410 y se borra lo que escribió el usuario. El registro de la petición dura 7 días (`retencion.horas_metadatos`). Ver [arquitectura](arquitectura.md#retención-cierre-y-recuperación).
- **Límites del historial:** como mucho se guardan 10 000 peticiones terminadas por espacio (`retencion.max_trabajos`, de 100 a 100 000); en cada purga se borran las más antiguas que pasen del tope. Las respuestas se guardan de 0 a 720 horas (0 no las guarda) y el registro de 0 a 90 días (admite decimales: 1,5 días son 36 horas), nunca menos que las respuestas. Si varias peticiones terminaron en el mismo instante del corte, el tope puede conservar unas pocas de más. Un valor fuera de rango en `config.yaml` se ajusta al límite y deja un aviso en el registro del servicio. Tras cada purga la base devuelve al disco el espacio liberado; una base anterior se convierte una sola vez al actualizar, y el registro del servicio lo dice («se compactó la base de …»). Los tres valores se cambian en **Ajustes → Historial**, que también muestra cuántas peticiones hay y cuánto ocupa la base.
- **Origen:** cada petición guarda de dónde llegó: Probar (la Web UI), la terminal, la CLI, una aplicación con la clave principal o una API. El Historial lo muestra y filtra por origen, por grupo de APIs o por API, y trae las peticiones de 200 en 200 con **Ver más**.

## Claves de API

`tramtai claves guardar|listar|borrar NOMBRE` gestiona las claves de los proveedores (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `CODEX_API_KEY`, `GEMINI_API_KEY`). Se guardan cifradas en el almacén del sistema y Tramtai nunca las escribe en sus archivos ni las muestra:

| Sistema | Almacén |
|---|---|
| Windows | Administrador de credenciales, como credenciales genéricas `tramtai/<NOMBRE>` |
| macOS | Llavero, mediante `/usr/bin/security` |
| Linux | Secret Service (GNOME, KDE…) mediante `secret-tool`, del paquete `libsecret-tools`; sin él, usa una variable de entorno |

- Una variable de entorno con el mismo nombre tiene prioridad sobre el almacén.
- **No hace falta reiniciar:** la clave del almacén se lee en cada petición. Solo una variable de entorno exige que el servicio arranque con ella; en macOS, las de `~/.zshrc` no llegan al servicio que inicia la app o el arranque automático, por eso se recomienda el Llavero.
- La página **Agentes** dice dónde queda la clave en tu sistema. Ver [primeros pasos](inicio.md#2-dejar-listos-los-agentes).

## Si se filtra una clave

| Clave | Qué hacer |
|---|---|
| La de una API por aplicación | **Regenerar la clave** en Mis APIs, o `tramtai acceso token <nombre> --rotar`. La anterior deja de valer al momento. |
| La del espacio | **Regenerar la clave** en Mis APIs → Clave del espacio, o `tramtai global token --rotar` (`tramtai proyecto token --rotar` en un proyecto). Vale sin reiniciar; si sale de una variable de entorno, cámbiala allí. |
| La de un proveedor (Anthropic, OpenAI, Gemini) | Revócala en la consola del proveedor y guarda la nueva con `tramtai claves guardar NOMBRE`. |

En los dos primeros casos, pon la clave nueva en las aplicaciones que usaban la anterior.

**Si vuelves a una versión anterior** a la que guarda las claves de las APIs como huella: esa versión leería el archivo `sha256:…` como si fuera la clave. Al volver, regenera la clave de cada API (`tramtai acceso token <nombre> --rotar`).

**Si vuelves a la v0.55 o anterior desde la v0.56:** la v0.56 sube la base de cada espacio a la versión 10, que guarda el `intento` de cada petición. Una versión anterior no abre esa base («versión de base de datos 10 no soportada por este ejecutable»). No se pierde nada: el historial sigue intacto y la página Espacios todavía lo cuenta. Para recuperar el espacio, vuelve a instalar la v0.56 o una posterior, o restaura la copia de la carpeta de Tramtai que hiciste antes de actualizar. Las peticiones de antes de la migración quedan con intento 1, aunque ya hubieran pasado al respaldo, y un reintento de una de ellas también empieza en 1.

## Espacios de trabajo

Un espacio de trabajo (en la API propia, las rutas `/api/v1/instancia`) tiene su historial, sus APIs, su clave y sus ajustes. Hay uno global
y uno por cada proyecto que abras. La página **Espacios** de la app, `tramtai espacios` y `GET /api/v1/servicio/espacios`
los listan todos: activos, detenidos y no encontrados.

| | Global | Proyecto |
|---|---|---|
| Configuración | `config.yaml` de la [carpeta de Tramtai](inicio.md#configuración) | La global, más `.tramtai/config.yaml` (versionada) y `.tramtai/local.yaml` (de esta máquina) |
| Identificador | `global` | `<nombre>_<hash de la ruta>` (por ejemplo `demo_app_1a2b3c4d`), fijado la primera vez y guardado en `.tramtai/espacio` |
| Clave | `datos/token-global` o la variable `TRAMTAI_TOKEN_GLOBAL` | `datos/tokens/<id>` |
| Base de datos | `datos/instancias/global.db` | `datos/instancias/<id>.db` |
| Registro | `datos/espacios.json`: todos los espacios con su ruta, su último uso y su último error (reemplaza a `instancias.json`, que se migra solo) | |

Las rutas son relativas a la [carpeta de Tramtai](inicio.md#configuración). La clave del espacio y la clave de servicio quedan en claro en archivos con permisos 0600, porque la CLI y la app las leen; las claves de las APIs por aplicación, en cambio, solo como huella.

- **La marca** (`.tramtai/espacio`, fuera de git por el `.gitignore` de `.tramtai`) guarda el id del espacio: si mueves o renombras la carpeta, sigue siendo el mismo espacio con su historial. Una copia de la carpeta, en cambio, es un espacio nuevo, también para la CLI. La confianza no viaja con la marca (un repositorio ajeno podría copiarla): al abrir el proyecto en su ruta nueva la app vuelve a preguntar, diciendo de dónde viene.
- **No encontrado:** si la carpeta de un proyecto ya no está donde estaba, el espacio queda «No encontrado»; con **Buscar** (o `POST /ui/espacios/ubicar`) se le indica la ruta nueva, que no puede ser la de otro espacio. Buscar no traslada la confianza: si su `local.yaml` usa claves de seguridad, después se confía desde la página Espacios, que las enseña.
- **Al abrir una carpeta que ya trae `.tramtai/`** (un repositorio clonado, aunque le falte `config.yaml`), la app pregunta si confías en ella (409 `confianza_requerida`, con `claves` y, si parece un proyecto movido, `movido`) y no pisa lo que trae. Confiar la añade a `proyectos_confiables`; sin confiar, las claves de seguridad de su `local.yaml` se ignoran y se avisa cuáles (también tras reiniciar), y los demás ajustes del espacio se pueden guardar. Un proyecto cuyo `.tramtai/` crea Tramtai queda confiado. Si la carpeta está dentro de otro proyecto, la app pregunta cuál abrir (409 `dentro_de_proyecto`).
- **Olvidar** un espacio borra su base de datos, su clave y su registro en Tramtai, nunca la carpeta del proyecto. Se confirma escribiendo su nombre; la global no se olvida.
- **Ajustes** tiene dos pestañas, «Este espacio» y «Todo Tramtai»: cada valor dice si es propio del proyecto o viene de Global, y **Usar el de Global** lo devuelve (`POST /api/v1/instancia/heredar`).
- La app, la terminal y la CLI vuelven al último espacio usado; la CLI no cae en la global si el espacio del proyecto está detenido (`espacio_detenido`), salvo con `--global`.

La configuración versionada del proyecto no puede definir claves de seguridad: ni puertos, ni comandos, ni autenticación, ni APIs. `.tramtai/local.yaml` tampoco, salvo que la carpeta del proyecto figure en `proyectos_confiables` de la configuración global: un repositorio clonado puede traer un `local.yaml` (forzado con `git add -f`) y no debe poder cambiar qué programas ejecuta Tramtai. `proyectos_confiables` solo se admite en la configuración global.

### Carpetas desde las que se envían archivos

Por defecto, `tramtai ejecutar -f`, `--esquema`, `tramtai convertir` y la terminal interactiva pueden enviar cualquier archivo que tu usuario pueda leer. Para limitarlo, indica las carpetas en la configuración global (o en `.tramtai/local.yaml` de un proyecto confiable):

```yaml
instancia_global:
  raices_permitidas:
    - C:\Users\yo\Documentos\para-agentes
    - ~/proyectos        # en macOS y Linux
```

Se comprueba la ruta real del archivo, así que `..` o un enlace que salga de esas carpetas se rechaza con `ruta_no_permitida`. No afecta a la Web UI ni a la API, que solo reciben archivos subidos, nunca rutas.

### Límites y cargas

- **Cargas:** hasta 20 por petición y `conversion.maximo_carga_mb` (50) cada una. Las de un espacio no pasan de
  `conversion.cuota_cargas_mb` (2048) en total: la siguiente responde 507 `cuota_cargas_agotada` hasta que venzan. Cada
  carga pertenece a la API que la subió (o al espacio): otra API no puede citarla aunque conozca su identificador.
- **Formatos:** los de [Formatos admitidos](api.md#formatos-admitidos); el resto, también los ZIP, se rechaza al
  subirlo con `carga_invalida`. Un Word, PowerPoint o Excel que se descomprimiría en más de
  `conversion.maximo_descomprimido_mb` se rechaza antes de convertirlo. `conversion.maximo_archivos_zip` y
  `conversion.profundidad_maxima_zip` se aceptan todavía en la configuración, pero ya no hacen nada.
- **Conversor:** markitdown 0.1.8 con su propio Python, en la carpeta local: `%LOCALAPPDATA%\tramtai\conversor` en
  Windows (no viaja con un perfil móvil), `~/.local/share/tramtai/conversor` en Linux (o `$XDG_DATA_HOME`), la carpeta
  de Tramtai en macOS y `datos-tramtai/conversor` en la portable. Se prepara solo la primera vez que arranca el servicio (`conversion.preparar_al_iniciar`,
  `true` por defecto) y a mano con **Ajustes → Archivos** o `tramtai conversor preparar`; `tramtai conversor estado`
  dice si está listo. Se usa, por este orden: `conversion.comando` si lo cambiaste, el paquete del conversor junto al
  ejecutable, el preparado y `markitdown` del PATH. La descarga usa las variables `HTTPS_PROXY` y `NO_PROXY` del entorno del
  servicio y los certificados del sistema (redes que inspeccionan TLS). Un proxy configurado solo en las opciones del
  sistema o con un PAC no se lee: define esas variables para tu usuario (también las hereda el servicio que arranca con
  el sistema) o, si aun así se bloquea, descomprime el paquete del conversor para Windows junto a `tramtai.exe`.
  Mientras se prepara uno nuevo (una versión de Tramtai que cambia markitdown, o «Preparar de nuevo»), se sigue usando
  el anterior; cerrar Tramtai corta la preparación. Sin conversor listo, una petición con un archivo que lo necesita responde 503
  `conversor_no_disponible` antes de aceptarse.
- **Conversión:** `conversion.tiempo_limite_segundos` (120) por archivo y `conversion.tiempo_total_segundos` (600) para
  toda la petición; markitdown corre con un tope de memoria (`limites.memoria_proceso_mb`, 2048; en macOS no se
  aplica). Los límites de `conversion.*` solo se cambian en la configuración global: un valor 0 o negativo usa el
  predeterminado y cada uno tiene un máximo.
- **Contexto:** ver [límites de la entrada](api.md#límites-de-la-entrada). Para fijar el de un modelo que no lo informa
  (Codex, por ejemplo):

  ```yaml
  limites:
    contexto_tokens:
      - codex/gpt-5.6-terra=272000
    reserva_salida_tokens: 8192
  ```

- **Antigravity** va sin herramientas (un hook de agy las niega), pero solo se usa desde Probar, la terminal y la CLI
  hasta que su canario pase con una clave de la Gemini API; `adaptadores.antigravity.permitir_apps: true` lo habilita
  también para apps y APIs.
- **Modo de contexto de una API:** lo fija la API (`aislado` si no indica otro); una app no puede pedir
  `consciente_proyecto` en una API aislada.
- **Entorno de los CLI:** no heredan claves, contraseñas, `TRAMTAI_*` ni URL con credenciales, salvo los proxies
  (`*_PROXY`), que sí necesitan para llegar a su proveedor.
- **La configuración propia de cada CLI** es cosa suya y tuya: con la suscripción, Codex y Claude usan lo que sus CLI no
  permiten aislar (por ejemplo los servidores MCP de `~/.codex`), y en `consciente_proyecto` Claude carga los hooks
  del proyecto, por eso ese modo solo vale en `proyectos_confiables`.

### Comprobar el aislamiento de los agentes

`go test -tags canario -v -count=1 ./pruebas/canario` pide a Claude (haiku y sonnet), a Codex (gpt-5.6-terra, esfuerzo
bajo) y a GitHub Copilot (automático; gasta un crédito del plan), con tu suscripción, que lean un archivo trampa, ejecuten una orden y abran una URL local, y comprueba que no lo
consiguen. Gasta unas pocas peticiones pequeñas.

## APIs por aplicación

Una API por aplicación es otra URL, con su propia clave, para una aplicación concreta. El comando es `tramtai acceso` y la clave de configuración, `puntos_acceso`. Fija el agente y el modelo, restringe los modelos permitidos y traduce los nombres fijos que envía la aplicación.

```powershell
tramtai acceso crear app_contable --puerto 17401 --contratos openai `
  --agente claude --modelo sonnet --permitidos claude/sonnet,claude/haiku --alias gpt-4o-mini=claude/haiku
tramtai acceso crear resumenes --local --forzar --perfil rapido   # ruta /p/resumenes en el espacio del proyecto
tramtai acceso crear tienda --grupo cliente_x --agente claude   # grupo libre
tramtai acceso crear soporte --instrucciones "Responde siempre en español."
tramtai acceso listar
tramtai acceso editar app_contable --permitidos claude --grupo contabilidad
tramtai acceso desactivar app_contable resumenes   # detiene una o varias
tramtai acceso activar --grupo cliente_x           # todas las de un grupo
tramtai acceso token app_contable --rotar        # regenera la clave y la muestra esta única vez
tramtai acceso editar tienda --presupuesto diario.peticiones=500 --presupuesto mensual.usd=50
tramtai acceso reiniciar-presupuesto tienda    # sin nombre, el del espacio
```

`tramtai acceso` guarda los cambios a través del servicio, con las mismas reglas que Mis APIs y la terminal, y lo inicia en segundo plano si no está en marcha. Trabaja sobre el archivo (la configuración global o, con `--local`, `.tramtai/local.yaml` del proyecto), así que también llega a una API cuyo espacio no está activo o que no se pudo abrir: se puede detener o eliminar, pero no guardar otra vez con su error.

**Mis APIs** tiene dos pestañas, que el navegador recuerda:

- **APIs por aplicación:**
  - **Filtros:** buscador (por nombre, grupo, modelo o instrucciones), grupo, estado (activas, detenidas, con error) y agente.
  - **Más filtros:** qué acepta (OpenAI, Anthropic, API propia), qué tiene (respuesta JSON, respaldo, presupuesto, instrucciones, modelo forzado, modelos restringidos) y si tiene peticiones.
  - **Orden:** por nombre, más usadas o última petición.
  - **Páginas:** de 20; una página que empieza en medio de un grupo repite su cabecera con «(continúa)».
  - **Grupos:** cada uno se pliega desde su cabecera y el navegador recuerda cuáles quedaron plegados.
- **Clave del espacio:** la guía «¿Qué URL uso?», la clave, los modelos, los ejemplos y el presupuesto del espacio. Es para ti y tus scripts; cada app va mejor con su propia API, y Mis APIs lo sugiere si muchas peticiones de apps llegan con esta clave. En el mapa de **Inicio**, cada grupo es una estación («web · 3 APIs», con sus activas debajo) que abre Mis APIs filtrada por ese grupo.

Al pulsar el nombre o la fila de una API se abre su detalle, con **Dirección y clave**: las URL base según lo que acepta (OpenAI, Anthropic, API propia), la clave (solo se ve al crearla o regenerarla; si no la tienes, **Regenerar la clave** muestra una nueva y la anterior deja de valer), los ejemplos para copiar con su URL y su modelo, las acciones (ajustes, Probar, peticiones, activar o detener) y, aparte, **Eliminar la API**. Al crear una API se abre su detalle con la clave nueva.

- **Clave:** `tramtai acceso crear`, Mis APIs y «Guardar como API» en Probar muestran la clave una sola vez, al crearla. Tramtai guarda solo su huella SHA-256 (`datos/tokens/acceso_<nombre>` contiene `sha256:…`), así que no se puede volver a ver: si se pierde, se regenera (`--rotar` o **Regenerar la clave**) y la anterior deja de valer. `tramtai acceso token <nombre>` sin `--rotar` lo explica. Las claves antiguas guardadas en claro se convierten solas a huella y siguen valiendo. Eliminar una API borra también su clave.
- **Instrucciones:** `--instrucciones` (o el campo del mismo nombre en Mis APIs, editable después) se antepone a las instrucciones de cada petición que llega por esa API, venga de la aplicación que venga. Sirve, por ejemplo, para que una app que no lo permite configurar responda siempre en español.
- **Uso:** Mis APIs muestra de cada API cuántas peticiones tuvo en los 7 días que conserva la retención, cuántas con error y la última; al pulsarlo se abre el Historial filtrado por esa API (`GET /api/v1/trabajos?punto=<nombre>`).
- **Presupuesto:** `--presupuesto periodo.unidad=valor` (periodo `diario` o `mensual`; unidad `peticiones`, `tokens` o `usd`; repetible; 0 quita ese límite), o los campos de presupuesto en Mis APIs. Al 100 % las peticiones nuevas reciben 429 `presupuesto_agotado` hasta que reinicia el periodo (medianoche o día 1, hora local); Mis APIs muestra una barra por límite, amarilla desde el 80 %. El límite en USD solo suma el costo que informa el agente (hoy Claude). Tokens y USD se conocen al terminar, así que se reserva lo que gastarían las peticiones en curso al promedio de las terminadas: es un límite aproximado (tras reiniciar con peticiones en curso, la reserva queda algo corta hasta el periodo siguiente). El del espacio va en la configuración (`presupuesto.diario.*`, `presupuesto.mensual.*`; global o `local.yaml`, nunca la versionada), se aplica sin reiniciar con `tramtai servicio recargar` y cuenta todas sus peticiones. **Reiniciar consumo** (Mis APIs o `tramtai acceso reiniciar-presupuesto`) pone a cero el día y el mes en curso; reiniciar o eliminar una API no devuelve su consumo al presupuesto del espacio. El consumo se guarda aparte de las peticiones y se conserva 13 meses.
- **Grupos:** `--grupo` es una etiqueta libre (hasta 40 caracteres) para ordenar las APIs y activarlas o detenerlas juntas, desde la CLI o desde Mis APIs. La separación por proyecto ya la dan los espacios: una API creada con `--local` pertenece al del proyecto.

- **Dirección:** sin `--puerto`, la API queda en `http://127.0.0.1:17373/p/<nombre>`. Con `--puerto`, escucha en su propio puerto; si ese puerto está ocupado, la API no se activa (no se elige otro) y el estado lo indica.
- **Dónde se guarda:** en la configuración global, o con `--local` en `.tramtai/local.yaml` del proyecto (pertenece a su espacio; exige que la carpeta figure en `proyectos_confiables` de la configuración global). Se conservan los comentarios del archivo, y un cambio inválido se rechaza sin tocarlo.
- **Contratos:** cada API expone solo los suyos (`nativa`, `openai`, `anthropic`), nunca la administración.
- **Modelos:**
  - `--alias` traduce nombres que la aplicación no deja cambiar, como `gpt-4o-mini`.
  - `--permitidos` restringe lo que se puede pedir (`agente`, `agente/modelo` o `perfil/nombre`); si se pide otro modelo, se rechaza con `modelo_no_permitido`.
  - `--forzar` ignora el modelo que pide la petición y lo informa en la cabecera `Tramtai-Modelo-Forzado`.
- **Peticiones:** cada petición guarda la API que la creó (`punto_acceso`), y por esa API solo se ven sus propias peticiones.
- **Sin reinicios:** los cambios se aplican al momento. Las APIs que no cambian siguen abiertas y no se cortan las peticiones en curso.

## Administración (para la CLI y la terminal)

La clave de servicio (`datos/token-servicio`) habilita `/api/v1/servicio`:

| Ruta | Para qué |
|---|---|
| `GET /api/v1/servicio` | Estado del servicio, sus espacios y sus APIs. |
| `POST /api/v1/servicio/instancias` | Activar un espacio, con `{"ambito":"global"}` o `{"ambito":"proyecto","raiz":"…"}`. Si la carpeta trae la marca de un espacio cuya carpeta ya no está y no es confiable, responde 409 `confianza_requerida` con `movido`: se confía en ella (`/proyectos` con `confiar`) antes de quedarse con ese espacio. |
| `DELETE /api/v1/servicio/instancias/{id}` | Detener un espacio. |
| `GET /api/v1/servicio/espacios` | Todos los espacios del registro, también los detenidos y los no encontrados, con su estado, APIs y peticiones. |
| `DELETE /api/v1/servicio/espacios/{id}` | Olvidar un espacio, con `{"nombre": "…"}` igual a su nombre: borra su base, su clave y su registro, nunca la carpeta del proyecto. |
| `POST /api/v1/servicio/espacios/{id}/activar` | Vuelve a abrir un espacio detenido; responde el espacio (200). Un espacio que no existe, o sin su carpeta, da 404 `no_encontrado`. |
| `POST /api/v1/servicio/espacios/{id}/detener` | Detiene el espacio (lo que estaba en cola se cancela); sus datos y su configuración se quedan (204). |
| `POST /api/v1/servicio/espacios/{id}/confianza` | `{"confiar": true}` añade su proyecto a `proyectos_confiables`; `false` lo quita (204). Solo para el espacio de un proyecto con ruta: si no, 400 `solicitud_invalida`. |
| `POST /api/v1/servicio/espacios/{id}/ubicacion` | `{"ruta": "…"}` asocia un espacio cuya carpeta no se encuentra con una carpeta que tenga `.tramtai/config.yaml`: conserva su id, su clave y su historial, y lo activa. Responde el espacio (200). |
| `POST /api/v1/servicio/proyectos` | `{"raiz": "…", "confiar": true, "crear_aqui": false}` abre el proyecto (y lo crea si falta) como espacio activo; responde el espacio (200). Si hay que decidir responde 409: `confianza_requerida` con `claves` (las de seguridad de su `local.yaml`) y `movido` (dónde estaba, si parece uno movido), o `dentro_de_proyecto` con `proyecto`. Se repite con `confiar` o con `crear_aqui: true`. |
| `PUT /api/v1/servicio/agentes/{agente}/autenticacion` | `{"modo": "suscripcion"}` o `{"modo": "clave_api"}` en un agente que admite la suscripción, al momento y en todos los espacios (204). |
| `POST /api/v1/servicio/agentes/{agente}/conexion` | `{"conectado": false}` desconecta un agente sin desinstalar su CLI (`adaptadores.<agente>.desconectado`); `true` lo vuelve a conectar (204). |
| `GET /api/v1/servicio/instancias/{id}/configuracion` | Ver su configuración efectiva. |
| `PUT /api/v1/servicio/puerto` | Guardar `servicio.puerto` y `servicio.puerto_automatico` (`{"puerto": 18000, "automatico": true}`; con `puerto` 0 solo cambia el automático). Se aplica al reiniciar. |
| `POST /api/v1/servicio/recargar` | Aplicar los cambios de las APIs. |
| `POST /api/v1/servicio/detener` | Detener el servicio. |

Con esa misma clave y la cabecera `Tramtai-Instancia: <id>`, la administración puede usar la API de cualquier espacio. El descriptor `datos/servicio.json` indica dónde escucha el servicio. Antes de enviarle la clave, la CLI comprueba que quien responde lo conoce: manda un reto aleatorio a `/healthz?reto=<reto>` y el servicio contesta con el HMAC-SHA256 de la clave sobre su dirección y ese reto. Otro proceso que ocupe el puerto no puede contestar, aunque conozca el identificador del descriptor. Un solo servicio usa cada carpeta de datos: al arrancar toma el cerrojo `datos/servicio.lock`, y un segundo servicio sobre la misma carpeta no arranca. El servicio de una versión anterior no contesta el reto ni toma el cerrojo: tras actualizar, la CLI y la app de escritorio lo detienen, dejando terminar las peticiones en curso, y arrancan el nuevo. Solo le mandan la clave si el proceso que escucha en el puerto es del mismo usuario del sistema; si no se puede comprobar, piden detenerlo a mano.

## Versión portable

El `.zip` portable de Windows trae `portable.txt` junto a `tramtai.exe` y `tramtai-escritorio.exe`. Con ese archivo,
esa copia guarda todo en `datos-tramtai`, a su lado, en lugar de en `%APPDATA%\tramtai`:

- `config.yaml` y `profiles\`;
- `datos\`: descriptor del servicio, claves, bases de los espacios, registros y archivos subidos;
- `escritorio.json`;
- `webview\`: el perfil del navegador de la app.

Lo que conviene saber:

- **Permisos.** Al crearla, `datos-tramtai` queda solo para tu usuario y SYSTEM, sin heredar los de la carpeta de
  encima. En un pendrive FAT o exFAT no hay permisos: quien tenga el pendrive puede leer las claves de tus APIs.
- **Dónde no funciona.** Si la carpeta no se puede escribir, o es la temporal donde el Explorador abre un `.zip` sin
  descomprimir, la app y la CLI lo dicen y no usan `%APPDATA%`.
- **Claves de los proveedores.** Siguen en el almacén cifrado de Windows: se comparten con la instalación de este
  equipo (borrarlas en una las borra para las dos) y en otro equipo hay que volver a guardarlas.
- **Instalada y portable a la vez.** Son dos Tramtai independientes:
  - cada una tiene su servicio y su clave;
  - si el puerto está ocupado, la segunda usa otro libre (`puerto_automatico`);
  - el título de la ventana y la bandeja dicen «Tramtai portable»;
  - «Iniciar con Windows» se marca por separado en cada una;
  - el instalador no cierra la portable.
- **La CLI.** En una terminal, `tramtai` es el del PATH, la instalada. Para la portable, `.\tramtai.exe` desde su
  carpeta.
- **Pasar lo de la instalada.** Con las dos cerradas, copia el contenido de `%APPDATA%\tramtai` en `datos-tramtai`:
  configuración, APIs, claves e historial. Si ese `config.yaml` tiene `servicio.puerto_automatico: false` y la
  instalada usa ese puerto, cambia el de una de las dos.
- **Quien ya usaba la portable 0.47 o anterior.** Desde la 0.48 la portable empieza con datos propios y claves nuevas;
  para conservar lo anterior, copia `%APPDATA%\tramtai` como en el punto anterior.
- **Actualizar.** Descomprime la versión nueva encima: `datos-tramtai` no viene en el `.zip` y no se pisa.
- **Volver a %APPDATA%.** Borra `portable.txt` con Tramtai cerrado.

