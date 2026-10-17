# Actualizaciones: limpieza y novedades

Diseño acordado el 2026-10-16. Dos mejoras al flujo de actualización y el cierre de un pendiente:

1. El instalador descargado no se borra nunca tras una actualización correcta: en el equipo del usuario hay cinco
   carpetas `%TEMP%\tramtai-actualizacion*` de 11,8 MB (59 MB), una por actualización.
2. Tras actualizar solo aparece un aviso corto que se va en segundos. El usuario quiere ver los cambios, y el
   registro muestra que salta versiones (de la 0.32.0 a la 0.35.1 sin ver la 0.33, la 0.34 ni la 0.35.0).
3. La pantalla en blanco al pulsar Instalar quedó resuelta en v0.36.0 (el avance se muestra al instante): el usuario
   lo verificó al pasar de la 0.36.0 a la 0.36.1.

Fuera de alcance: borrar el instalador justo después de instalar (no se puede mientras se ejecuta, y la limpieza al
arrancar logra lo mismo), guardar las notas antes de instalar (no sirve si se actualiza desde la terminal, winget u
otro equipo) y un botón para releer las novedades en Ajustes (nadie lo pidió; el enlace a las versiones sigue).

## 1. Limpieza del instalador

- `actualizacion.LimpiarDescargas(antes time.Time)` borra en `os.TempDir()` las carpetas `tramtai-actualizacion*`
  modificadas antes de `antes`. Lo que no se pueda borrar (un archivo en uso) se deja para la próxima vez; los
  errores no se informan, nunca detienen nada.
- Se llama con `time.Now().Add(-10 * time.Minute)`, para no tocar una descarga en curso:
  - al arrancar el servicio (`servicio.Nuevo`), en segundo plano: la versión nueva limpia lo que dejó la anterior;
  - antes de cada descarga, en el servicio (`Servicio.actualizar`) y en `tramtai actualizar`.
- Prueba: con `TMP`/`TEMP` apuntando a una carpeta de prueba, borra las carpetas viejas de Tramtai y respeta las
  recientes y las que no son de Tramtai.

## 2. Novedades tras actualizar

### Servicio

- `actualizacion.Novedades(ctx, desde, hasta string) ([]Version, error)` pide a GitHub
  `GET /repos/MscottB/Tramtai/releases?per_page=30` (una consulta) y devuelve las versiones publicadas, sin borradores
  ni previas, con `MasNueva(v, desde)` y no `MasNueva(v, hasta)`: de la más nueva a la más vieja, como mucho 10.
  Cada una con `version`, `url` (solo páginas del repositorio, como en `Ultima`) y `notas` (recortadas a 6000 runas).
- `GET /ui/actualizacion/novedades?desde=<versión>` responde `{"versiones": [...]}` hasta la versión instalada. Con
  `desde` vacío o inválido, solo la instalada.
- Si `servicio.buscar_actualizaciones` está desactivado, no consulta GitHub y responde `{"versiones": []}`: quien
  apagó la búsqueda no quiere que Tramtai hable con GitHub por su cuenta.
- Un fallo de red o de GitHub responde 502 con el error; la Web UI lo trata como «sin notas».

### Web UI

- En `App.vue`, el aviso corto (`actualizacion.hecha`) se reemplaza por un diálogo cuando cambia la versión:
  - título «Tramtai se actualizó a la {version}»;
  - las notas con `NotasVersion`; si hay varias versiones, la más nueva abierta y las demás plegadas
    (`q-expansion-item` con su número);
  - el enlace «Ver todas las versiones» (la página de versiones del repositorio) y un botón **Aceptar**;
  - sin notas (sin red, búsqueda desactivada o respuesta vacía), solo el título, una línea y el enlace.
- Sale una sola vez por versión, como hoy: la última versión vista sigue en `localStorage` (`tramtai.version`). Una
  compilación `dev` no lo muestra (no hay versión con la que comparar).

## 3. Pantalla en blanco

- `docs/equipo/pendientes.md`: se quita el pendiente del rediseño y se anota que quedó resuelto en v0.36.0,
  verificado por el usuario en la actualización 0.36.0 → 0.36.1.

## Documentación

- `docs/operacion.md` (Actualizaciones): la limpieza de descargas y el diálogo de novedades. Las rutas `/ui/` son
  internas de la Web UI y no van en `docs/api.md`, y la Ayuda no tiene una respuesta sobre actualizaciones.
- `docs/equipo/pendientes.md`: la fila del release en la hoja de ruta.

## Pruebas

- `go test ./internal/actualizacion`: `LimpiarDescargas` (viejas, recientes, ajenas) y `Novedades` contra un servidor
  local (filtro por rango, orden, tope de 10, borradores y previas fuera, URL ajena descartada).
- `go test ./internal/servicio`: el endpoint con la búsqueda desactivada no consulta GitHub.
- Navegador: con una versión vista anterior en `localStorage`, el diálogo aparece con las notas y no vuelve a salir
  tras Aceptar; sin red, aparece sin notas.
