# Grupos, origen y límites del historial

Diseño acordado el 2026-10-16. Tres temas que ordenan una instancia con muchas APIs y mucho tráfico:

1. **Grupos de APIs.** Ya existen (campo `grupo` de cada API, cabecera por grupo en Mis APIs con activas y
   activar o detener todas), pero no se pliegan, no se filtran y el mapa de Inicio dibuja una estación por API
   (máximo 3 y «y N más»), así que con muchas APIs esconde la mayoría.
2. **Origen de cada petición.** El trabajo solo guarda qué API lo recibió (`punto_acceso`): «API principal» mezcla
   Probar, la terminal, la CLI y cualquier app con la clave principal.
3. **Límites del historial.** Hay plazos (resultado 24 h, registro 7 días, purga cada 5 min), pero no hay tope de
   cantidad, los plazos solo se cambian en `config.yaml` y sin máximo, la base no se compacta tras borrar y el
   Historial muestra las últimas 200 sin avisar.

Fuera de alcance: las mejoras de Probar (esfuerzo a la vista, respaldo con esfuerzo, registro con tiempos,
historial breve, Markdown) van en el release siguiente y usarán el origen de este; la API propia explicada, en el
otro. Un tope por tamaño en MB (difícil de medir por trabajo) y filtrar el tablero de Inicio por grupo.

## 1. Grupos

### Mis APIs

- **Plegables:** la cabecera de cada grupo pliega y despliega su lista. Los grupos plegados se recuerdan en el
  navegador (`localStorage` `tramtai.gruposPlegados`, lista de nombres); sin almacenamiento, todo abierto.
- **Filtro:** con más de una API, encima de la lista, un selector de grupo («Todos», cada grupo y «Sin grupo») y un
  buscador por nombre de API. Ocultar por el filtro no pliega ni cambia nada guardado.
- Se puede abrir Mis APIs ya filtrada por un grupo, con una variable compartida como `origenHistorial`
  (`grupoInicial` en `api.ts`).

### Mapa de Inicio (`MapaRed.vue`)

- Estaciones de origen: la API principal, una por grupo («web · 3 APIs», debajo «2 activas») y las APIs sin grupo,
  sueltas. Se mantiene el tope de 3 estaciones más «y N más».
- La línea de un grupo va continua si alguna de sus APIs está activa y punteada si todas están detenidas.
- Pulsar un grupo abre Mis APIs filtrada por ese grupo; una API suelta abre Mis APIs.

## 2. Origen de cada petición

- **Columna nueva** `trabajos.origen` (migración v8, `TEXT NOT NULL DEFAULT ''`), con estos valores:
  - `probar`: la petición llegó con la sesión de la Web UI (cookie). La marca la pone el servicio, que autentica la
    petición; el cliente no la puede elegir.
  - `terminal` y `cli`: llegó con la clave de la instancia y el encabezado `X-Tramtai-Cliente: terminal` o `cli`,
    que envía `internal/cliente` según quién lo use (la TUI o los comandos).
  - `app`: la clave de la instancia sin ese encabezado (o con otro valor).
  - En una API extra (`punto_acceso` no vacío) el origen es `api`.
  - Los trabajos anteriores quedan con `''` y se muestran como hoy.
- El encabezado es solo una etiqueta: una app que lo ponga figura como «CLI», pero no gana ningún permiso.
- Un reintento (`POST /trabajos/{id}/reintentar`) conserva el origen del trabajo original, igual que ya conserva su
  API.
- El trabajo lo expone como `origen` en `GET /api/v1/trabajos` y `GET /api/v1/trabajos/{id}`.

## 3. Historial

- **Columna Origen y detalle:** «Probar», «Terminal», «CLI», «App (clave principal)», «API tienda · web» (con el
  grupo actual de la API, si tiene) o «API principal» para los antiguos.
- **Selector Origen:** «Todos», «Probar», «Terminal», «CLI», «App (clave principal)», cada grupo («Grupo web») y
  cada API.
- **Servicio:** `GET /api/v1/trabajos` acepta además:
  - `?origen=probar|terminal|cli|app`;
  - `?grupo=<nombre>`: las APIs de ese grupo según la configuración actual (si una API cambia de grupo, su historial
    pasa al nuevo; una API eliminada no está en ningún grupo, pero sigue con `?punto=` mientras quede historial);
  - `?antes=<creado_en>`: los trabajos creados antes de esa fecha, para pedir la página siguiente.
  - En un punto de acceso se siguen viendo solo los suyos: `grupo` y `origen` se ignoran ahí.
- **Ver más:** bajo la tabla, «Mostrando las {n} más recientes» y **Ver más**, que pide 200 más con `antes`. Sin más
  resultados, el botón desaparece.

## 4. Límites del historial

### Configuración

| Clave | Por defecto | Rango |
|---|---|---|
| `retencion.horas_resultado` | 24 | 0 a 720 (30 días); 0 no guarda las respuestas |
| `retencion.horas_metadatos` | 168 | 0 a 2160 (90 días), y nunca menor que `horas_resultado` |
| `retencion.max_trabajos` (nueva) | 10000 | 100 a 100000 |

- Un valor fuera de rango en `config.yaml` se ajusta al límite y deja un aviso en el registro del servicio: no
  impide arrancar. Por la API o Ajustes, se rechaza con el motivo (400 `solicitud_invalida`).

### Tope de cantidad

- En cada purga (cada 5 min y al guardar los ajustes), `Purgar` borra los trabajos terminados más antiguos que
  pasen de `max_trabajos` en la instancia, con sus eventos. Los activos nunca se borran ni cuentan para el corte.
  Entre dos purgas puede pasarse un poco del tope.
- Las cargas que ya no usa ningún trabajo se van con la purga de cargas que ya existe.

### Compactación

- Al abrir la base, si `PRAGMA auto_vacuum` no es incremental, se pone `PRAGMA auto_vacuum = INCREMENTAL` y se
  hace un `VACUUM` único, fuera de la transacción de las migraciones. Tarda según el tamaño, que la retención ya
  acota.
- Tras cada purga, `PRAGMA incremental_vacuum` devuelve al disco las páginas libres.

### Ajustes · Historial

- Tres campos: «Guardar respuestas y archivos» (horas), «Guardar el registro» (días; se guarda en horas) y «Máximo
  de peticiones guardadas». Debajo, el uso actual: «1 234 peticiones · 12 MB» (trabajos de la instancia y tamaño del
  archivo de la base). Sigue el «Vaciar historial» que ya existe.
- `GET /api/v1/instancia/retencion` →
  `{"horas_resultado", "horas_metadatos", "max_trabajos", "uso": {"trabajos", "bytes"}}`;
  `PUT /api/v1/instancia/retencion` con los tres valores. Con la misma protección que
  `/api/v1/instancia/predeterminado`.
- Se guarda como el modelo predeterminado (`fijarPredeterminado`): en la instancia global, en la configuración
  global (y alcanza a los proyectos sin valor propio); en un proyecto, en su `local.yaml`. Se aplica al momento y
  lanza una purga.
- El «últimos 7 días» fijo del tablero de Inicio pasa a decir los días configurados.

## 5. Capturas y nombres de ejemplo

- La instancia de prueba usa APIs `tienda` y `soporte` en el grupo `web` (no «facturas», que se lee como cobro).
- Se rehacen las capturas de la página (`site/capturas/`) y del README (`docs/imagenes/`).

## Documentación

- `docs/operacion.md`: grupos (plegar, filtrar, mapa), retención (rangos, tope, compactación, Ajustes).
- `docs/arquitectura.md`: la tabla de retención con `max_trabajos` y los rangos.
- `docs/api.md`: `origen` en el trabajo; `?origen=`, `?grupo=` y `?antes=` en la lista; `X-Tramtai-Cliente`;
  `GET/PUT /api/v1/instancia/retencion`.
- Ayuda de la Web UI: una respuesta sobre cuánto se guarda el historial y dónde se cambia.
- `docs/equipo/pendientes.md`: la fila del release en la hoja de ruta.

## Pruebas

- `internal/persistencia`: migración v8 (columna `origen` y `auto_vacuum` incremental en una base vieja); `Purgar`
  con tope (borra los terminados más antiguos, nunca los activos); `Listar` por origen, por lista de APIs y con
  `antes`.
- `internal/nucleo`: rangos de `retencion.*` (ajuste al límite al cargar; error al fijar).
- `internal/api` y `internal/servicio`: el origen según la autenticación y el encabezado (y que una API extra da
  `api` aunque mande el encabezado); `?grupo=` con la configuración actual; `GET/PUT /api/v1/instancia/retencion`
  (valores, rechazo fuera de rango, purga al guardar).
- `internal/cliente`: el encabezado según quién lo usa.
- Navegador: grupos plegables y recordados, filtro de Mis APIs, mapa por grupo y su enlace, Historial por origen y
  grupo con Ver más, Ajustes · Historial; claro y oscuro, 375 px.
