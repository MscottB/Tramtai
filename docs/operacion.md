# Operación: servicio, instancias y puntos de acceso

Tramtai funciona como **un único servicio por usuario** (un proceso y un puerto, `127.0.0.1:17373`) con varias **instancias**, cada una con su configuración, token, cola, base de datos y cargas. El token de cada petición decide a qué instancia llega, así que una aplicación nunca ve los trabajos de otra.

## La forma cómoda: la terminal interactiva

```powershell
tramtai            # sin argumentos abre la terminal interactiva (o: tramtai terminal)
```

| Pantalla | Qué permite |
|---|---|
| Selector de ámbito | Proyecto actual, instancia global, todas las instancias o estado del servicio. Muestra si el servicio y cada instancia están activos. |
| Instancia | Endpoint, token, agente predeterminado y recuento de trabajos. Desde aquí se activa o desactiva la instancia; si el servicio no está en marcha, lo inicia en segundo plano. |
| Ejecutar | Elige el modelo con ←→ entre el catálogo, añade archivos (rutas separadas por comas), escribe el mensaje y ve la respuesta en vivo. Esc cancela el trabajo. |
| Trabajos | Lista, detalle con la salida, y acciones para cancelar (`c`), reintentar (`t`) o eliminar (`x`). |
| Puntos de acceso | Activa o desactiva con Enter, crea con `n` (nombre, puerto y modelo), elimina con `x` (también su clave) y genera o regenera la clave con `t`: se muestra esa única vez. |
| Configuración efectiva | Cada valor de la instancia y la capa de la que procede. |
| Agentes y capacidades | Versión, autenticación, términos y capacidades de cada agente. |
| Estado del servicio | Instancias, puntos de acceso y errores de configuración. Permite detener (con confirmación) y recargar. |

Se navega con ↑↓ o el número de la opción, Enter, Esc para volver y Ctrl+C para salir. Las acciones que afectan a otros (desactivar una instancia, detener el servicio, eliminar) piden confirmación.

## Web UI y aplicación de escritorio

```powershell
tramtai abrir                      # abre la app de escritorio (--navegador: la Web UI en el navegador); la del proyecto si estás en uno
tramtai global abrir               # la de la instancia global
tramtai proyecto abrir             # la del proyecto actual
tramtai-escritorio                 # aplicación de escritorio (o -proyecto <ruta>)
```

La Web UI está en español y en inglés (el botón de idioma de la cabecera) y tiene estas páginas:

- **Inicio:** resumen de trabajos, primeros pasos pendientes y estado de los agentes.
- **Probar:** modelo del catálogo con su esfuerzo justo debajo (también el que usa el agente si no eliges uno), respaldo con un esfuerzo propio por agente, archivos, mensaje y JSON Schema opcional; muestra la respuesta en vivo, con formato Markdown o como texto (la elección se recuerda), y permite cancelar. Bajo la respuesta, **Registro** da la hora de cada paso (cola, espera del agente, ejecución con qué agente, modelo y esfuerzo, avisos) y la duración de cada etapa; también está en el detalle del Historial. **Pruebas recientes** lista las 10 últimas pruebas de Probar: pulsar una la abre y **Usar de nuevo** recupera su mensaje y su receta (los adjuntos no).
- **Historial:** detalle, seguir en vivo, cancelar, reintentar, eliminar y **vaciar el historial** (los trabajos terminados y sus archivos).
- **Mis APIs:** URL, clave y ejemplos; puntos de acceso con grupos que se activan o desactivan juntos.
- **Agentes**, **Ajustes** (arranque con el sistema, **borrar datos** y valores efectivos) y **Ayuda** (cómo empezar y preguntas frecuentes).

También se puede abrir `http://127.0.0.1:17373` directamente: en Windows y Linux entra solo si el navegador es de tu mismo usuario del sistema. Si no (en macOS siempre), la página **Entrar** explica cómo abrirla: desde la app o con `tramtai abrir`, que generan un código de un solo uso. La clave de las aplicaciones no abre la Web UI.

La aplicación de escritorio inicia el servicio si no está en marcha y muestra la misma Web UI en su ventana. Al cerrar la ventana queda en la bandeja del sistema vigilando el servicio. Clic en el icono abre la ventana; **clic derecho** abre el menú, sin abrir la app:

- **Estado**: círculo verde «Servicio en marcha · 127.0.0.1:17373» o rojo «Servicio detenido» (también al pasar el ratón por el icono).
- **Mis APIs**: las APIs de todos los espacios en marcha (global y proyectos), con un círculo verde si está activa y rojo si está desactivada o con error. Un clic la desactiva o la vuelve a activar; se guarda y se aplica sin reiniciar. Al final, «Gestionar en Mis APIs» abre esa sección.
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

- **Acerca de Tramtai** (el menú lateral, el menú del estado o «Acerca de Tramtai» en la bandeja): es el único sitio que muestra la versión, tal como aparece en GitHub (`v0.42.0+17102026`), el desarrollador, el estado de las actualizaciones con **Buscar actualización ahora** y las novedades de esa versión en GitHub, los enlaces del proyecto (sitio, código, incidencias, licencia y términos) y **Detalles técnicos**: commit, sistema, Go, dirección y pid del servicio, almacén de claves y, en una sesión local, las carpetas de datos, configuración y registro. **Copiar datos para soporte** los copia como texto, sin claves ni tokens. La Web UI los pide a `GET /ui/acerca`.
- La cabecera de la Web UI muestra **Actualizar** y el menú de la bandeja, **Actualizar a vX…**. **Actualizar ahora** descarga el instalador, comprueba su suma SHA-256 con `SHA256SUMS.txt` y avisa «Tramtai se reiniciará en 10 s», con **Reiniciar ahora** y **Cancelar** (borra la descarga); al vencer lo ejecuta mostrando solo su ventana de progreso («Actualizar Tramtai»: el instalador dice Actualizar si ya hay una instalación registrada e Instalar si no), que se cierra sola: la app se cierra, el servicio se detiene tras terminar las peticiones en curso (`servicio.plazo_drenaje_segundos`) y la app vuelve a abrirse con la versión nueva, con tu configuración y tus datos.
- Para no esperar a la próxima comprobación: **Ajustes → Buscar actualizaciones → Buscar actualización ahora** o, en la bandeja, **Buscar actualizaciones**; consultan GitHub en el momento (como mucho una vez cada 10 segundos) aunque la búsqueda automática esté desactivada.
- Si instalas una versión a mano con Tramtai abierto (sea la instalada, la portable o una copia compilada), el instalador te avisa, cierra la app, detiene el servicio cuando terminan las peticiones en curso y, al acabar, vuelve a abrir lo que estaba abierto.
- Tras actualizar, la Web UI muestra una vez las novedades de cada versión desde la última que viste en ese navegador (hasta 10, la más nueva abierta), con el enlace a todas las versiones. Si la búsqueda de actualizaciones está desactivada, o GitHub no responde, solo dice la versión nueva y ofrece el enlace.
- El instalador descargado no puede borrarse mientras se ejecuta: el servicio, al arrancar, y cada descarga nueva borran las carpetas `tramtai-actualizacion*` del temporal con más de 10 minutos, así que no se acumulan.
- Desde la terminal: `tramtai actualizar` hace lo mismo (pide confirmación; `--si` la omite) y `tramtai actualizar --comprobar` solo informa.
- Solo se actualiza sola una copia instalada con el instalador de Windows; una compilada a mano o en Linux y macOS muestra el aviso con el enlace de descarga.
- Se desactiva en **Ajustes → Buscar actualizaciones** o con `servicio.buscar_actualizaciones: false`; `tramtai actualizar` sigue funcionando porque lo pides tú.

### Borrar datos

- **Historial:** «Vaciar historial» en la Web UI, o `DELETE /api/v1/trabajos`, borra los trabajos terminados de la instancia con sus eventos y las cargas que ya no usa ninguno. Lo que está en curso se conserva.
- **Puntos de acceso:** uno a uno en Mis APIs o con `tramtai acceso eliminar`.
- **Ajustes → Borrar datos** hace ambas cosas para la instancia actual. La configuración y las claves de proveedor guardadas no se tocan.

## Comandos equivalentes

```powershell
tramtai servicio iniciar -d        # segundo plano (sin -d: primer plano, Ctrl+C lo detiene)
tramtai servicio estado            # instancias, puntos de acceso y errores
tramtai servicio recargar          # aplica cambios de puntos de acceso sin reiniciar
tramtai servicio detener           # pide confirmación (-s para no pedirla); espera a lo que está en curso
tramtai servicio autoarranque [activar|desactivar]   # arrancar al iniciar sesión, oculto en la bandeja
tramtai servicio registro [-f] [-n 30]               # lo que procesa el servicio en segundo plano; -f lo sigue en vivo
tramtai servicio puerto [número] [--automatico=false] # muestra o cambia el puerto (comprueba que esté libre)
tramtai servicio iniciar --abrir   # modo desarrollo: primer plano con el registro a la vista y la interfaz abierta

tramtai global iniciar|detener|estado|token [--rotar]
tramtai iniciar                    # crea .tramtai/config.yaml en el repositorio actual
tramtai proyecto iniciar|detener|estado|token [--rotar]   # desde la carpeta del proyecto

tramtai trabajos listar|mostrar|cancelar|reintentar|eliminar   # -i <instancia> para elegir otra
```

- `proyecto iniciar` inicia el servicio si no está en marcha.
- **¿Sigue en marcha?** `tramtai abrir` y la app de escritorio dejan el servicio en segundo plano: sigue atendiendo a tus aplicaciones aunque cierres la terminal o la ventana. La cabecera de la Web UI muestra «En marcha» o «Sin conexión», y `tramtai servicio estado` lo confirma.
- **Puerto:** `servicio.puerto` (17373). Si al iniciar está ocupado y `servicio.puerto_automatico` está activado (lo está por defecto), Tramtai usa el siguiente libre y lo avisa en el registro; la CLI y la app de escritorio lo encuentran solas (la ventana de la app vuelve a entrar en la nueva dirección en unos segundos, cambie como cambie el puerto), pero tus aplicaciones deben usar la nueva dirección. Con `servicio.puerto_automatico: false` no arranca y explica cómo elegir otro. Se cambia en **Ajustes → Puerto del servicio** o con `tramtai servicio puerto <número>`: ambos comprueban que esté libre y se aplica al reiniciar (en Ajustes, con «Reiniciar ahora»; la página sigue al servicio a su nuevo puerto).
- **Iniciar, reiniciar y detener:** desde la Web UI, el indicador «En marcha» tiene «Reiniciar el servicio» y «Detener el servicio…» (solo en este equipo y con confirmación). Iniciarlo no puede hacerse desde la página, porque es el propio servicio el que la sirve: se inicia con `tramtai abrir`, con `tramtai servicio iniciar -d` o desde la bandeja de la app de escritorio (Iniciar el servicio). La página se reconecta sola cuando vuelve.
- **Registro:** cada petición deja una línea al recibirla y otra al terminar (agente, modelo efectivo, API de origen, duración, tokens o el error). Se limita con `servicio.registro_mb` (5 MB por archivo) y `servicio.registro_copias` (2 copias: `servicio.log.1` y `.2`); con 0 copias el archivo se vacía al llenarse. En segundo plano va a `datos/servicio.log` en la [carpeta de Tramtai](inicio.md#configuración) (`tramtai servicio registro -f`); en primer plano, a la propia terminal.
- Las instancias activas se recuerdan: al reiniciar el servicio vuelven a abrirse y recuperan sus trabajos.
- Detener una instancia cancela lo que tenía en cola; lo que se estaba ejecutando queda `interrumpido`. Las demás instancias no se ven afectadas.
- **Detener el servicio:**
  - Deja de aceptar trabajos y espera hasta `servicio.plazo_drenaje_segundos` (30 por defecto) a que terminen los que están en marcha. Después interrumpe los que queden.
  - Lo que estaba en cola se reanuda al volver a iniciarlo.
  - En primer plano, un segundo Ctrl+C corta la espera.
- **Resultados:** se conservan 24 horas (`retencion.horas_resultado`); después la salida responde 410 y se borra lo que escribió el usuario. El registro del trabajo dura 7 días (`retencion.horas_metadatos`). Ver [arquitectura](arquitectura.md#retención-cierre-y-recuperación).
- **Límites del historial:** como mucho se guardan 10 000 peticiones terminadas por instancia (`retencion.max_trabajos`, de 100 a 100 000); en cada purga se borran las más antiguas que pasen del tope. Las respuestas se guardan de 0 a 720 horas (0 no las guarda) y el registro de 0 a 90 días (admite decimales: 1,5 días son 36 horas), nunca menos que las respuestas. Si varias peticiones terminaron en el mismo instante del corte, el tope puede conservar unas pocas de más. Un valor fuera de rango en `config.yaml` se ajusta al límite y deja un aviso en el registro del servicio. Tras cada purga la base devuelve al disco el espacio liberado; una base anterior se convierte una sola vez al actualizar, y el registro del servicio lo dice («se compactó la base de …»). Los tres valores se cambian en **Ajustes → Historial**, que también muestra cuántas peticiones hay y cuánto ocupa la base.
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
| La de la instancia (la de la *API principal*) | **Regenerar la clave** en Mis APIs → API principal, o `tramtai global token --rotar` (`tramtai proyecto token --rotar` en un proyecto). Vale sin reiniciar; si sale de una variable de entorno, cámbiala allí. |
| La de un proveedor (Anthropic, OpenAI, Gemini) | Revócala en la consola del proveedor y guarda la nueva con `tramtai claves guardar NOMBRE`. |

En los dos primeros casos, pon la clave nueva en las aplicaciones que usaban la anterior.

**Si vuelves a una versión anterior** a la que guarda las claves de las APIs como huella: esa versión leería el archivo `sha256:…` como si fuera la clave. Al volver, regenera la clave de cada API (`tramtai acceso token <nombre> --rotar`).

## Instancias

| | Global | Proyecto |
|---|---|---|
| Configuración | `config.yaml` de la [carpeta de Tramtai](inicio.md#configuración) | La global, más `.tramtai/config.yaml` (versionada) y `.tramtai/local.yaml` (de esta máquina) |
| Identificador | `global` | `<nombre>_<hash de la ruta>`, por ejemplo `demo_app_1a2b3c4d` |
| Token | `datos/token-global` o la variable `TRAMTAI_TOKEN_GLOBAL` | `datos/tokens/<id>` |
| Base de datos | `datos/instancias/global.db` | `datos/instancias/<id>.db` |

Las rutas son relativas a la [carpeta de Tramtai](inicio.md#configuración). La clave de la instancia y el token de servicio quedan en claro en archivos con permisos 0600, porque la CLI y la app los leen; las claves de los puntos de acceso, en cambio, solo como huella.

La configuración versionada del proyecto no puede definir claves de seguridad: ni puertos, ni comandos, ni autenticación, ni puntos de acceso. `.tramtai/local.yaml` tampoco, salvo que la carpeta del proyecto figure en `proyectos_confiables` de la configuración global: un repositorio clonado puede traer un `local.yaml` (forzado con `git add -f`) y no debe poder cambiar qué programas ejecuta Tramtai. `proyectos_confiables` solo se admite en la configuración global.

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

- **Cargas:** hasta 20 por petición y `conversion.maximo_carga_mb` (50) cada una. Las de una instancia no pasan de
  `conversion.cuota_cargas_mb` (2048) en total: la siguiente responde 507 `cuota_cargas_agotada` hasta que venzan. Cada
  carga pertenece a la API que la subió (o a la instancia): otra API no puede citarla aunque conozca su identificador.
- **Formatos:** texto, Markdown, JSON, XML, CSV, HTML, PDF, Word, PowerPoint, Excel y ZIP; el resto se rechaza con
  `carga_invalida`, también dentro de un ZIP. Un Word, PowerPoint o Excel que se descomprimiría en más de
  `conversion.maximo_descomprimido_mb` se rechaza antes de convertirlo.
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
bajo) y a Copilot (automático; gasta un crédito del plan), con tu suscripción, que lean un archivo trampa, ejecuten una orden y abran una URL local, y comprueba que no lo
consiguen. Gasta unas pocas peticiones pequeñas.

## Puntos de acceso

Un punto de acceso es otra URL, con su propio token, para una aplicación concreta. Fija el agente y el modelo, restringe los modelos permitidos y traduce los nombres fijos que envía la aplicación.

```powershell
tramtai acceso crear app_contable --puerto 17401 --contratos openai `
  --agente claude --modelo sonnet --permitidos claude/sonnet,claude/haiku --alias gpt-4o-mini=claude/haiku
tramtai acceso crear resumenes --local --forzar --perfil rapido   # ruta /p/resumenes en la instancia del proyecto
tramtai acceso crear tienda --grupo cliente_x --agente claude   # grupo libre
tramtai acceso crear soporte --instrucciones "Responde siempre en español."
tramtai acceso listar
tramtai acceso editar app_contable --permitidos claude --grupo contabilidad
tramtai acceso desactivar app_contable resumenes   # uno o varios
tramtai acceso activar --grupo cliente_x           # todos los de un grupo
tramtai acceso token app_contable --rotar        # regenera la clave y la muestra esta única vez
tramtai acceso editar tienda --presupuesto diario.peticiones=500 --presupuesto mensual.usd=50
tramtai acceso reiniciar-presupuesto tienda    # sin nombre, el de la instancia
```

**Mis APIs** tiene dos pestañas, que el navegador recuerda:

- **APIs por aplicación:**
  - **Filtros:** buscador (por nombre, grupo, modelo o instrucciones), grupo, estado (activas, detenidas, con error) y agente.
  - **Más filtros:** qué acepta (OpenAI, Anthropic, API propia), qué tiene (respuesta JSON, respaldo, presupuesto, instrucciones, modelo forzado, modelos restringidos) y si tiene peticiones.
  - **Orden:** por nombre, más usadas o última petición.
  - **Páginas:** de 20; una página que empieza en medio de un grupo repite su cabecera con «(continúa)».
  - **Grupos:** cada uno se pliega desde su cabecera y el navegador recuerda cuáles quedaron plegados.
- **API principal:** la guía «¿Qué URL uso?», la clave, los modelos, los ejemplos y el presupuesto del espacio. En el mapa de **Inicio**, cada grupo es una estación («web · 3 APIs», con sus activas debajo) que abre Mis APIs filtrada por ese grupo.

Cada API tiene un botón **Conectar** (también se abre pulsando su fila) con su detalle: las URL base según lo que acepta (OpenAI, Anthropic, API propia), la clave —que solo se ve al crearla o regenerarla; si no la tienes, **Regenerar la clave** muestra una nueva y la anterior deja de valer—, los ejemplos para copiar con su URL y su modelo, las acciones (ajustes, Probar, peticiones, activar o detener) y, aparte, **Eliminar la API**. Al crear una API se abre su detalle con la clave nueva.

- **Clave:** `tramtai acceso crear`, Mis APIs y «Guardar como API» en Probar muestran la clave una sola vez, al crearla. Tramtai guarda solo su huella SHA-256 (`datos/tokens/acceso_<nombre>` contiene `sha256:…`), así que no se puede volver a ver: si se pierde, se regenera (`--rotar` o **Regenerar la clave**) y la anterior deja de valer. `tramtai acceso token <nombre>` sin `--rotar` lo explica. Las claves antiguas guardadas en claro se convierten solas a huella y siguen valiendo. Eliminar una API borra también su clave.
- **Instrucciones:** `--instrucciones` (o el campo del mismo nombre en Mis APIs, editable después) se antepone a las instrucciones de cada petición que llega por ese punto, venga de la aplicación que venga. Sirve, por ejemplo, para que una app que no lo permite configurar responda siempre en español.
- **Uso:** Mis APIs muestra de cada API cuántas peticiones tuvo en los 7 días que conserva la retención, cuántas con error y la última; al pulsarlo se abre el Historial filtrado por esa API (`GET /api/v1/trabajos?punto=<nombre>`).
- **Presupuesto:** `--presupuesto periodo.unidad=valor` (periodo `diario` o `mensual`; unidad `peticiones`, `tokens` o `usd`; repetible; 0 quita ese límite), o los campos de presupuesto en Mis APIs. Al 100 % las peticiones nuevas reciben 429 `presupuesto_agotado` hasta que reinicia el periodo (medianoche o día 1, hora local); Mis APIs muestra una barra por límite, amarilla desde el 80 %. El límite en USD solo suma el coste que informa el agente (hoy Claude). Tokens y USD se conocen al terminar, así que se reserva lo que gastarían los trabajos en curso al promedio de los terminados: es un límite aproximado (tras reiniciar con trabajos en curso, la reserva queda algo corta hasta el periodo siguiente). El de la instancia va en la configuración (`presupuesto.diario.*`, `presupuesto.mensual.*`; global o `local.yaml`, nunca la versionada), se aplica sin reiniciar con `tramtai servicio recargar` y cuenta todas sus peticiones. **Reiniciar consumo** (Mis APIs o `tramtai acceso reiniciar-presupuesto`) pone a cero el día y el mes en curso; reiniciar o eliminar una API no devuelve su consumo al presupuesto de la instancia. El consumo se guarda aparte de los trabajos y se conserva 13 meses.
- **Grupos:** `--grupo` es una etiqueta libre (hasta 40 caracteres) para ordenar los puntos y activarlos o desactivarlos juntos, desde la CLI o desde Mis APIs. La separación por proyecto ya la dan las instancias: un punto creado con `--local` pertenece a la del proyecto.

- **Dirección:** sin `--puerto`, el punto queda en `http://127.0.0.1:17373/p/<nombre>`. Con `--puerto`, escucha en su propio puerto; si ese puerto está ocupado, el punto no se activa (no se elige otro) y el estado lo indica.
- **Dónde se guarda:** en la configuración global, o con `--local` en `.tramtai/local.yaml` del proyecto (pertenece a su instancia; exige que la carpeta figure en `proyectos_confiables` de la configuración global). Se conservan los comentarios del archivo, y un cambio inválido se rechaza sin tocarlo.
- **Contratos:** cada punto expone solo los suyos (`nativa`, `openai`, `anthropic`), nunca la administración.
- **Modelos:**
  - `--alias` traduce nombres que la aplicación no deja cambiar, como `gpt-4o-mini`.
  - `--permitidos` restringe lo que se puede pedir (`agente`, `agente/modelo` o `perfil/nombre`); si se pide otro modelo, se rechaza con `modelo_no_permitido`.
  - `--forzar` ignora el modelo de la solicitud y lo informa en la cabecera `Tramtai-Modelo-Forzado`.
- **Trabajos:** cada trabajo guarda el punto que lo creó (`punto_acceso`), y por ese punto solo se ven sus propios trabajos.
- **Sin reinicios:** los cambios se aplican al momento. Los puntos que no cambian siguen abiertos y no se cortan los trabajos en curso.

## Administración (para la CLI y la terminal)

El token de servicio (`datos/token-servicio`) habilita `/api/v1/servicio`:

| Ruta | Para qué |
|---|---|
| `GET /api/v1/servicio` | Estado del servicio, sus instancias y sus puntos. |
| `POST /api/v1/servicio/instancias` | Activar una instancia, con `{"ambito":"global"}` o `{"ambito":"proyecto","raiz":"…"}`. |
| `DELETE /api/v1/servicio/instancias/{id}` | Desactivar una instancia. |
| `GET /api/v1/servicio/instancias/{id}/configuracion` | Ver su configuración efectiva. |
| `POST /api/v1/servicio/recargar` | Aplicar los cambios de puntos de acceso. |
| `POST /api/v1/servicio/detener` | Detener el servicio. |

Con ese mismo token y la cabecera `Tramtai-Instancia: <id>`, la administración puede usar la API de cualquier instancia. El descriptor `datos/servicio.json` indica dónde escucha el servicio. Antes de usar el servicio, la CLI comprueba con `/healthz` que responde ese mismo servicio y no otro proceso que ocupe el puerto.
