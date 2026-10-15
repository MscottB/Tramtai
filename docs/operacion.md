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
| Puntos de acceso | Activa o desactiva con Enter, crea con `n` (nombre, puerto y modelo), elimina con `x` y muestra la ruta del token con `t`. |
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
- **Probar:** modelo del catálogo, archivos, mensaje y JSON Schema opcional; muestra la respuesta en vivo y permite cancelar.
- **Historial:** detalle, seguir en vivo, cancelar, reintentar, eliminar y **vaciar el historial** (los trabajos terminados y sus archivos).
- **Mis APIs:** URL, clave y ejemplos; puntos de acceso con grupos que se activan o desactivan juntos.
- **Agentes**, **Ajustes** (arranque con Windows, **borrar datos** y valores efectivos) y **Ayuda** (cómo empezar y preguntas frecuentes).

También se puede abrir `http://127.0.0.1:17373` directamente: en este equipo entra sin clave.

La aplicación de escritorio inicia el servicio si no está en marcha y muestra la misma Web UI en su ventana. Al cerrar la ventana queda en la bandeja del sistema vigilando el servicio. Clic en el icono abre la ventana; **clic derecho** abre el menú, sin abrir la app:

- **Estado**: círculo verde «Servicio en marcha · 127.0.0.1:17373» o rojo «Servicio detenido» (también al pasar el ratón por el icono).
- **Mis APIs**: las APIs de todos los espacios en marcha (global y proyectos), con un círculo verde si está activa y rojo si está desactivada o con error. Un clic la desactiva o la vuelve a activar; se guarda y se aplica sin reiniciar. Al final, «Gestionar en Mis APIs» abre esa sección.
- **Copiar la dirección para tus apps**.
- **Iniciar el servicio** (si está detenido), o **Reiniciar** y **Detener el servicio…** (si está en marcha). Tras detenerlo, la app sigue en la bandeja para volver a iniciarlo.
- **Iniciar con Windows**, el mismo ajuste que en Ajustes.
- **Salir y detener el servicio…**: cierra la app y detiene el servicio, así que tus aplicaciones dejan de funcionar hasta que vuelvas a abrirla. Pide confirmación con un aviso que se puede marcar como «No volver a mostrar». Para recuperar el aviso, borra `%APPDATA%\tramtai\escritorio.json`.

Los avisos de Detener y Salir se muestran dentro de la ventana, con el estilo de la app.

### Arranque con Windows

Activado en **Ajustes** o con `tramtai servicio autoarranque activar`, Tramtai se registra en `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` (solo para tu usuario). Al iniciar sesión arranca `tramtai-escritorio.exe --oculto`: inicia el servicio y queda en la bandeja sin abrir ninguna ventana ni terminal. Se quita con `tramtai servicio autoarranque desactivar`. Necesita `tramtai-escritorio.exe` junto a `tramtai.exe`; en Linux y macOS aún no está disponible.

### Actualizaciones

Cada hora, el servicio pregunta a GitHub cuál es la última versión publicada (solo esa ficha pública; no envía ningún dato tuyo); la ventana se lo vuelve a preguntar cada 15 minutos y al volver a ella, y la bandeja, cada pocos segundos. Así el aviso aparece sin reiniciar la app. Si hay una más nueva:

- La cabecera de la Web UI muestra **Actualizar** y el menú de la bandeja, **Actualizar a vX…**. **Actualizar ahora** descarga el instalador, comprueba su suma SHA-256 con `SHA256SUMS.txt` y lo ejecuta en silencio: la app se cierra, el servicio se detiene tras terminar las peticiones en curso (`servicio.plazo_drenaje_segundos`) y la app vuelve a abrirse con la versión nueva, con tu configuración y tus datos.
- Desde la terminal: `tramtai actualizar` hace lo mismo (pide confirmación; `--si` la omite) y `tramtai actualizar --comprobar` solo informa.
- Solo se actualiza sola una copia instalada con el instalador de Windows; una compilada a mano o en Linux y macOS muestra el aviso con el enlace de descarga.
- Se desactiva en **Ajustes → Buscar actualizaciones** o con `servicio.buscar_actualizaciones: false`; `tramtai actualizar` sigue funcionando porque lo pides tú.

### Borrar datos

- **Historial:** «Vaciar historial» en la Web UI, o `DELETE /api/v1/trabajos`, borra los trabajos terminados de la instancia con sus eventos y las cargas que ya no usa ninguno. Lo que está en curso se conserva.
- **Puntos de acceso:** uno a uno en Mis APIs o con `tramtai acceso eliminar`.
- **Ajustes → Borrar datos** hace ambas cosas para la instancia actual. La configuración y las claves de API guardadas no se tocan.

## Comandos equivalentes

```powershell
tramtai servicio iniciar -d        # segundo plano (sin -d: primer plano, Ctrl+C lo detiene)
tramtai servicio estado            # instancias, puntos de acceso y errores
tramtai servicio recargar          # aplica cambios de puntos de acceso sin reiniciar
tramtai servicio detener           # pide confirmación (-s para no pedirla); espera a lo que está en curso
tramtai servicio autoarranque [activar|desactivar]   # arrancar con Windows, oculto en la bandeja
tramtai servicio registro [-f] [-n 30]               # lo que procesa el servicio en segundo plano; -f lo sigue en vivo
tramtai servicio puerto [número] [--automatico=false] # muestra o cambia el puerto (comprueba que esté libre)
tramtai servicio iniciar --abrir   # modo desarrollo: primer plano con el registro a la vista y la interfaz abierta

tramtai global iniciar|detener|estado|token
tramtai iniciar                    # crea .tramtai/config.yaml en el repositorio actual
tramtai proyecto iniciar|detener|estado|token   # desde la carpeta del proyecto

tramtai trabajos listar|mostrar|cancelar|reintentar|eliminar   # -i <instancia> para elegir otra
```

- `proyecto iniciar` inicia el servicio si no está en marcha.
- **¿Sigue en marcha?** `tramtai abrir` y la app de escritorio dejan el servicio en segundo plano: sigue atendiendo a tus aplicaciones aunque cierres la terminal o la ventana. La cabecera de la Web UI muestra «En marcha» o «Sin conexión», y `tramtai servicio estado` lo confirma.
- **Puerto:** `servicio.puerto` (17373). Si al iniciar está ocupado y `servicio.puerto_automatico` está activado (lo está por defecto), Tramtai usa el siguiente libre y lo avisa en el registro; la CLI y la app de escritorio lo encuentran solas (la ventana de la app vuelve a entrar en la nueva dirección en unos segundos, cambie como cambie el puerto), pero tus aplicaciones deben usar la nueva dirección. Con `servicio.puerto_automatico: false` no arranca y explica cómo elegir otro. Se cambia en **Ajustes → Puerto del servicio** o con `tramtai servicio puerto <número>`: ambos comprueban que esté libre y se aplica al reiniciar (en Ajustes, con «Reiniciar ahora»; la página sigue al servicio a su nuevo puerto).
- **Iniciar, reiniciar y detener:** desde la Web UI, el indicador «En marcha» tiene «Reiniciar el servicio» y «Detener el servicio…» (solo en este equipo y con confirmación). Iniciarlo no puede hacerse desde la página, porque es el propio servicio el que la sirve: se inicia con `tramtai abrir`, con `tramtai servicio iniciar -d` o desde la bandeja de la app de escritorio (Iniciar el servicio). La página se reconecta sola cuando vuelve.
- **Registro:** cada petición deja una línea al recibirla y otra al terminar (agente, modelo efectivo, API de origen, duración, tokens o el error). Se limita con `servicio.registro_mb` (5 MB por archivo) y `servicio.registro_copias` (2 copias: `servicio.log.1` y `.2`); con 0 copias el archivo se vacía al llenarse. En segundo plano va a `%APPDATA%\tramtai\datos\servicio.log` (`tramtai servicio registro -f`); en primer plano, a la propia terminal.
- Las instancias activas se recuerdan: al reiniciar el servicio vuelven a abrirse y recuperan sus trabajos.
- Detener una instancia cancela lo que tenía en cola; lo que se estaba ejecutando queda `interrumpido`. Las demás instancias no se ven afectadas.
- **Detener el servicio:**
  - Deja de aceptar trabajos y espera hasta `servicio.plazo_drenaje_segundos` (30 por defecto) a que terminen los que están en marcha. Después interrumpe los que queden.
  - Lo que estaba en cola se reanuda al volver a iniciarlo.
  - En primer plano, un segundo Ctrl+C corta la espera.
- **Resultados:** se conservan 24 horas (`retencion.horas_resultado`); después la salida responde 410 y se borra lo que escribió el usuario. El registro del trabajo dura 7 días (`retencion.horas_metadatos`). Ver [arquitectura](arquitectura.md#retención-cierre-y-recuperación).

## Claves de API

`tramtai claves guardar|listar|borrar NOMBRE` las guarda en el Administrador de credenciales de Windows, como credenciales genéricas `tramtai/<NOMBRE>`. Los adaptadores las usan si no hay una variable de entorno con ese nombre, y nunca se muestran. Ver [primeros pasos](inicio.md#2-dejar-listos-los-agentes).

## Instancias

| | Global | Proyecto |
|---|---|---|
| Configuración | `%APPDATA%\tramtai\config.yaml` | La global, más `.tramtai/config.yaml` (versionada) y `.tramtai/local.yaml` (de esta máquina) |
| Identificador | `global` | `<nombre>_<hash de la ruta>`, por ejemplo `demo_app_1a2b3c4d` |
| Token | `%APPDATA%\tramtai\datos\token-global` o la variable `TRAMTAI_TOKEN_GLOBAL` | `%APPDATA%\tramtai\datos\tokens\<id>` |
| Base de datos | `datos\instancias\global.db` | `datos\instancias\<id>.db` |

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

## Puntos de acceso

Un punto de acceso es otra URL, con su propio token, para una aplicación concreta. Fija el agente y el modelo, restringe los modelos permitidos y traduce los nombres fijos que envía la aplicación.

```powershell
tramtai acceso crear app_contable --puerto 17401 --contratos openai `
  --agente claude --modelo sonnet --permitidos claude/sonnet,claude/haiku --alias gpt-4o-mini=claude/haiku
tramtai acceso crear resumenes --local --forzar --perfil rapido   # ruta /p/resumenes en la instancia del proyecto
tramtai acceso crear facturas --grupo cliente_x --agente claude   # grupo libre
tramtai acceso crear soporte --instrucciones "Responde siempre en español."
tramtai acceso listar
tramtai acceso editar app_contable --permitidos claude --grupo contabilidad
tramtai acceso desactivar app_contable resumenes   # uno o varios
tramtai acceso activar --grupo cliente_x           # todos los de un grupo
tramtai acceso token app_contable [--rotar]
```

- **Instrucciones:** `--instrucciones` (o el campo del mismo nombre en Mis APIs, editable después) se antepone a las instrucciones de cada petición que llega por ese punto, venga de la aplicación que venga. Sirve, por ejemplo, para que una app que no lo permite configurar responda siempre en español.
- **Uso:** Mis APIs muestra de cada API cuántas peticiones tuvo en los 7 días que conserva la retención, cuántas con error y la última; al pulsarlo se abre el Historial filtrado por esa API (`GET /api/v1/trabajos?punto=<nombre>`).
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

El token de servicio (`datos\token-servicio`) habilita `/api/v1/servicio`:

| Ruta | Para qué |
|---|---|
| `GET /api/v1/servicio` | Estado del servicio, sus instancias y sus puntos. |
| `POST /api/v1/servicio/instancias` | Activar una instancia, con `{"ambito":"global"}` o `{"ambito":"proyecto","raiz":"…"}`. |
| `DELETE /api/v1/servicio/instancias/{id}` | Desactivar una instancia. |
| `GET /api/v1/servicio/instancias/{id}/configuracion` | Ver su configuración efectiva. |
| `POST /api/v1/servicio/recargar` | Aplicar los cambios de puntos de acceso. |
| `POST /api/v1/servicio/detener` | Detener el servicio. |

Con ese mismo token y la cabecera `Tramtai-Instancia: <id>`, la administración puede usar la API de cualquier instancia. El descriptor `datos\servicio.json` indica dónde escucha el servicio. Antes de usar el servicio, la CLI comprueba con `/healthz` que responde ese mismo servicio y no otro proceso que ocupe el puerto.
