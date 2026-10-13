# Operación: servicio, instancias y puntos de acceso

Tramtai funciona como **un único servicio por usuario** (un proceso y un puerto, `127.0.0.1:17373`) con varias **instancias**, cada una con su configuración, token, cola, base de datos y cargas. El token de cada petición decide a qué instancia llega, así que una aplicación nunca ve los trabajos de otra (sección 5 del documento maestro).

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
tramtai abrir                      # abre la Web UI (la del proyecto si estás en uno; si no, la global)
tramtai global abrir               # la de la instancia global
tramtai proyecto abrir             # la del proyecto actual
tramtai-escritorio                 # aplicación de escritorio (o -proyecto <ruta>)
```

La Web UI tiene un panel (instancia, endpoints para conectar aplicaciones y trabajos recientes) y estas páginas:

- **Ejecutar:** modelo del catálogo, archivos, mensaje y JSON Schema opcional; muestra la respuesta en vivo y permite cancelar.
- **Trabajos:** detalle, seguir en vivo, cancelar, reintentar y eliminar.
- **Puntos de acceso:** crear con alias, modelos permitidos y forzado; activar, desactivar, ver el token y eliminar.
- **Configuración efectiva** y **Agentes**.

También se puede abrir `http://127.0.0.1:17373` directamente: en este equipo entra sin clave.

La aplicación de escritorio inicia el servicio si no está en marcha y muestra la misma Web UI en su ventana. Al cerrarla queda en la bandeja del sistema, que permite abrirla, abrirla en el navegador, detener el servicio (con confirmación) o salir. **Salir no detiene el servicio.**

## Comandos equivalentes

```powershell
tramtai servicio iniciar -d        # segundo plano (sin -d: primer plano, Ctrl+C lo detiene)
tramtai servicio estado            # instancias, puntos de acceso y errores
tramtai servicio recargar          # aplica cambios de puntos de acceso sin reiniciar
tramtai servicio detener           # pide confirmación (-s para no pedirla); espera a lo que está en curso

tramtai global iniciar|detener|estado|token
tramtai iniciar                    # crea .tramtai/config.yaml en el repositorio actual
tramtai proyecto iniciar|detener|estado|token   # desde la carpeta del proyecto

tramtai trabajos listar|mostrar|cancelar|reintentar|eliminar   # -i <instancia> para elegir otra
```

- `proyecto iniciar` inicia el servicio si no está en marcha.
- Las instancias activas se recuerdan: al reiniciar el servicio vuelven a abrirse y recuperan sus trabajos (sección 4.3).
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

La configuración versionada del proyecto no puede definir claves de seguridad: ni puertos, ni comandos, ni autenticación, ni puntos de acceso.

## Puntos de acceso

Un punto de acceso es otra URL, con su propio token, para una aplicación concreta. Fija el agente y el modelo, restringe los modelos permitidos y traduce los nombres fijos que envía la aplicación (sección 5.4).

```powershell
tramtai acceso crear app_contable --puerto 17401 --contratos openai `
  --agente claude --modelo sonnet --permitidos claude/sonnet,claude/haiku --alias gpt-4o-mini=claude/haiku
tramtai acceso crear resumenes --local --forzar --perfil rapido   # ruta /p/resumenes en la instancia del proyecto
tramtai acceso listar
tramtai acceso editar app_contable --permitidos claude
tramtai acceso desactivar app_contable
tramtai acceso token app_contable [--rotar]
```

- **Dirección:** sin `--puerto`, el punto queda en `http://127.0.0.1:17373/p/<nombre>`. Con `--puerto`, escucha en su propio puerto; si ese puerto está ocupado, el punto no se activa (no se elige otro) y el estado lo indica.
- **Dónde se guarda:** en la configuración global, o con `--local` en `.tramtai/local.yaml` del proyecto (pertenece a su instancia). Se conservan los comentarios del archivo, y un cambio inválido se rechaza sin tocarlo.
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

Con ese mismo token y la cabecera `Tramtai-Instancia: <id>`, la administración puede usar la API de cualquier instancia. El descriptor `datos\servicio.json` (sección 14.2) indica dónde escucha el servicio. Antes de usar el servicio, la CLI comprueba con `/healthz` que responde ese mismo servicio y no otro proceso que ocupe el puerto.
