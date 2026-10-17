# Probar: esfuerzo a la vista, respaldo con esfuerzo, registro con tiempos, pruebas recientes y Markdown

Diseño acordado el 2026-10-17 (v0.40). Al probar una consulta, el usuario no sabía qué esfuerzo tenía el modelo, no
podía elegir el esfuerzo de los agentes de respaldo ni ver el que usan por defecto, las etapas pasaban demasiado rápido
para seguirlas, echaba en falta un histórico corto en Probar y la respuesta se veía como texto plano. Al actualizar,
además, la app desaparece unos segundos sin mostrar nada mientras instala.

Durante la implementación el usuario añadió dos cosas al mismo release: un «Acerca de» con la versión y otros datos, y
que el instalador diga «Actualizar» cuando ya hay una instalación (sección 6).

Fuera de alcance: la guía de la API nativa (`/api/v1`, siguiente release), comparar agentes y «Seguridad y límites»
(auditoría frente a contenido malicioso hacia los CLI y mostrar los límites de los adjuntos y de cada CLI o modelo),
que va en su propio release después de este.

## 1. El esfuerzo junto al modelo

- En `EditorReceta` el selector de esfuerzo sale de «Opciones de la receta» y va en la misma fila que el modelo
  (debajo en el móvil), en Probar y en Mis APIs.
- Siempre dice qué esfuerzo se usará: «Del agente (alto)» si no se elige uno. Sin modelo elegido toma el
  predeterminado del catálogo (`predeterminado: true`) para calcularlo.
- En los datos de la respuesta (Probar y detalle del Historial) el esfuerzo aparece siempre: el elegido, o el del
  agente con «(del agente)» si se conoce; si no se conoce, no se muestra.

## 2. Esfuerzo para cada agente de respaldo

- Cada entrada del respaldo admite un esfuerzo con el sufijo `+nivel`: `codex/gpt-5.5+alto`, `claude+bajo`. El `+` no
  puede aparecer en un agente ni en un modelo (`reParte` admite `._:@-`), así que no hay ambigüedad. Vale igual en
  `/api/v1`, en `respaldo:` del YAML de las APIs y en Mis APIs; lo guardado sin sufijo sigue valiendo.
- `nucleo.ValidarRespaldo` acepta el sufijo solo con un nivel de `NivelesEsfuerzo`; dos entradas que solo difieren en
  el esfuerzo cuentan como repetidas. `DividirRespaldo` devuelve también el esfuerzo.
- Sin sufijo, la entrada se comporta como hoy: usa el esfuerzo del principal (ajustado al máximo de ese agente) y, si
  el principal no tiene, el del propio CLI. `+agente` usa siempre el de su CLI (en el editor, «El del agente», solo si el
  principal tiene esfuerzo). Al saltar, el planificador escribe en el respaldo restante lo que heredaba (`+nivel` o
  `+agente`), porque la solicitud guardada pasa a llevar el esfuerzo del respaldo que respondió.
- El planificador lleva el esfuerzo en cada `candidato` y lo pasa al adaptador en su intento.
- Web UI: bajo el selector de respaldo, una línea por agente elegido con su selector de esfuerzo; la opción vacía dice
  «Igual que el principal ({nivel})» si el principal tiene uno, o «Del agente ({nivel})» con el predeterminado de ese
  modelo. Un agente sin esfuerzo regulable muestra «no admite esfuerzo».

## 3. Registro con tiempos por etapa

- Al empezar cada intento el planificador guarda un evento `intento` con JSON
  `{"agente","modelo","esfuerzo","del_agente"}`: `esfuerzo` es el nivel que se pidió al CLI (ya ajustado a su máximo)
  o, si no se pidió ninguno, el predeterminado conocido con `del_agente: true` (vacío si no se conoce).
- `intento` no lleva contenido del usuario: la purga lo conserva con los metadatos, como `estado`.
- El flujo SSE de `/api/v1/trabajos/{id}/eventos` también lo emite (los clientes que no lo esperan lo ignoran); las
  fachadas OpenAI y Anthropic solo leen `texto` y no cambian.
- Ruta nueva `GET /api/v1/trabajos/{id}/registro`, con los mismos permisos que `GET /api/v1/trabajos/{id}`: devuelve
  `{"eventos":[{"numero","tipo","datos","creado_en"}]}` con los eventos `estado`, `intento` y `aviso`, nunca `texto`.
- Componente `RegistroTrabajo` (bloque plegable «Registro», en Probar bajo la respuesta y en el detalle del Historial):
  - una línea por evento con la hora (con milisegundos) y el tiempo desde la creación: «En cola», «Esperando agente»,
    «Ejecutando · claude/sonnet · alto», «Aviso: …», «Completado»;
  - la duración de cada etapa (cola, preparación, ejecución, validación);
  - al final, tokens de entrada y salida, costo y modelo que respondió.
- En Probar se carga al terminar la petición; mientras corre, se va completando con los eventos en vivo.

## 4. Pruebas recientes en Probar

- Lista «Pruebas recientes» con las últimas 10 peticiones de origen `probar` (`GET /api/v1/trabajos?origen=probar`):
  hora, estado, modelo y el comienzo del mensaje (o «mensaje ya no guardado» si caducó o se borraron las entradas).
- Pulsar una la abre en el panel de respuesta, con su salida y su registro.
- **Usar de nuevo** carga el mensaje y la receta (instrucciones, modelo, esfuerzo, respaldo y esquema); los adjuntos
  no, porque caducan, y se avisa si los tenía.
- Se recarga al terminar cada prueba. Enlace «Ver todo en el Historial» con el filtro de origen Probar.

## 5. Respuesta en Markdown

- La respuesta de Probar y la del detalle del Historial se muestran con formato Markdown por defecto, también mientras
  llegan; un botón alterna «Markdown» y «Texto», y la elección se recuerda (`localStorage`, con try/catch).
- Una salida JSON se sigue mostrando como hoy. «Copiar» copia siempre el texto original.
- Dependencias nuevas con versión fija: `marked` (Markdown a HTML) y `dompurify` (limpia el HTML antes de insertarlo).
- No se cargan imágenes externas (se muestra su texto alternativo): una URL externa en la respuesta podría rastrear a
  quien la abre. Los enlaces se abren en otra pestaña con `rel="noopener noreferrer"`.
- Componente `SalidaTrabajo`, compartido por Probar y el Historial.

## 6. Ventana de progreso al instalar una actualización

- Hoy la app lanza el instalador con `/S /abrir`: todo es silencioso y la app desaparece unos segundos.
- `tramtai.nsi`: con `/abrir` (lo pasan «Actualizar ahora» y `tramtai actualizar`), `.onInit` hace `SetSilent normal`
  y marca `$actualizando`. Las páginas de bienvenida, licencias, componentes, carpeta y final se saltan con su función
  `PRE` (`Abort`); solo queda la de progreso «Instalando Tramtai…», que se cierra sola (`SetAutoClose true`).
- Lo que hoy depende de `${Silent}` pasa a depender de «silencioso o actualizando»: no se pregunta antes de cerrar
  Tramtai, las secciones son las predeterminadas y al terminar se vuelve a abrir la app con `-actualizado`.
- Como el cambio va en el instalador nuevo, se ve aunque lo lance una versión anterior.
- Se descarta mostrar solo un cartel (`Banner`): no tiene barra de progreso.
- Con una instalación previa (`InstallLocation` en la clave de desinstalación de HKCU), la ventana y el asistente dicen
  «Actualizar» en vez de «Instalar»: título de la ventana «Actualizar Tramtai», página de bienvenida «Actualizar
  Tramtai a {versión}» y cabecera de la página de progreso «Actualizando Tramtai». Sin instalación previa, todo queda como
  hoy.
- La ventana de progreso se trae al frente al actualizar: Windows no deja que el instalador lanzado en segundo plano lo
  haga solo.

## 7. Acerca de Tramtai

- Diálogo «Acerca de Tramtai» (a pantalla completa en el móvil). Se abre desde:
  - un enlace bajo Ayuda en el menú lateral;
  - la versión de la cabecera, que pasa a ser un botón;
  - el menú del estado;
  - la bandeja del sistema: la línea «Versión …» deja de estar desactivada y abre la app con el diálogo.
- Contenido:
  - logo, nombre y la versión con su fecha de compilación (de `+DDMMAAAA`: «v0.40.0 · 17 de octubre de 2026»);
  - «Desarrollado por Michael Scott B.», con enlace a https://github.com/MscottB;
  - el estado de actualizaciones (el que ya conoce la app: «Tienes la última versión» o «Hay una versión nueva: vX»), con
    **Buscar ahora** y **Ver novedades** (la página de la versión instalada en GitHub Releases);
  - enlaces: sitio web (https://mscottb.github.io/Tramtai/), repositorio, informar de un problema (issues de GitHub),
    licencia (PolyForm Noncommercial 1.0.0) y términos de uso;
  - **Detalles técnicos**, plegado: commit (con enlace) y si tenía cambios sin confirmar, sistema y arquitectura, versión
    de Go, dirección del servicio, su pid y desde cuándo está en marcha, dónde se guardan las claves y, solo en una
    sesión local, las rutas de la carpeta de datos, la configuración global y el registro (`servicio.log`);
  - **Copiar datos para soporte**: copia los detalles técnicos como texto. Nunca lleva claves ni tokens.
- Ruta nueva `GET /ui/acerca`, con sesión (401 sin ella). Las rutas de carpetas solo van a una sesión local, porque
  contienen el nombre del usuario del sistema.

## Documentación

- Ayuda (Probar: esfuerzo, respaldo con esfuerzo, registro, pruebas recientes, Markdown; Actualizar: la ventana de
  progreso), `docs/operacion.md`, `docs/api.md` (sufijo `+nivel`, evento `intento`, ruta `/registro`), README y la
  página del producto si nombran algo de esto; `docs/equipo/pendientes.md` con la fila del release.

## Pruebas

- `internal/nucleo`: `ValidarRespaldo` y `DividirRespaldo` con `+nivel` (válido, nivel desconocido, repetida por
  esfuerzo).
- `internal/planificador`: el respaldo con esfuerzo propio lo pasa al adaptador; sin sufijo hereda el del principal;
  un evento `intento` por intento con el esfuerzo y `del_agente`.
- `internal/persistencia`: la purga del resultado conserva `estado` e `intento` y borra `texto` y `aviso`.
- `internal/api`: `/registro` sin `texto`, con 404 para un trabajo de otro punto y sin permiso.
- Instalador: compilar con `scripts/compilar.ps1 -Instalador` y ejecutarlo con `/S /abrir` sobre una carpeta de
  prueba, avisando antes al usuario porque cierra el Tramtai que tenga abierto; comprobar que se ve la ventana de
  progreso, que se cierra sola y que la app vuelve a abrirse.
- `internal/servicio`: `/ui/acerca` con sesión local (con rutas), sin sesión (401) y con una sesión remota (sin rutas).
- Instalador: con una instalación previa dice «Actualizar»; en una carpeta nueva sin instalación registrada, «Instalar».
- Navegador: las cinco partes de Probar y Acerca de (cada forma de abrirlo, Copiar datos), en móvil y escritorio,
  claro y oscuro.
