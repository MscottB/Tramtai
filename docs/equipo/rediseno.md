# Rediseño de la presentación

Diseño acordado el 2026-10-16. Objetivo: **pulir** la Web UI y la página del producto sin cambiar la dirección
visual «red de tranvía» (`docs/diseno/DESIGN.md`, `.impeccable/superficie-web-ui.md`). Se mejora la jerarquía,
la legibilidad, la densidad de la información y la coherencia entre páginas, y se corrigen dos fallos que vio el
usuario: la pantalla en blanco al actualizar y el sitio del botón Volver.

Fuera de alcance: nueva identidad visual, cambios en la API o en la lógica del servicio (salvo lo necesario para
el fallo de actualización), nuevas funciones.

## 1. Navegación

- **Logo y nombre** (arriba a la izquierda) son un botón que lleva siempre a Inicio («Ir a Inicio»).
- **Volver** deja la cabecera global y pasa a la cabecera de cada página (`CabeceraPagina`), encima del título,
  como «← Volver a {página}». Retrocede un paso en la pila que ya existe (`pila`/`volver` en `App.vue`); sin pila
  no se muestra. Está en todas las páginas. El atrás del navegador y Alt+← siguen funcionando.

## 2. Base común

- **`CabeceraPagina`**: Volver, título, una línea de descripción y acciones alineadas a la derecha (debajo en
  pantallas estrechas).
- **`EstadoVacio`**: estación apagada en el estilo del mapa, titular que invita a actuar, una línea y un botón.
  Sustituye el triángulo de aviso del Historial y los vacíos sueltos.
- **`MenuAcciones`**: «⋯» («Más acciones») con icono y texto; las destructivas al final y en color de error.
- **Ayudas legibles**: texto bajo los campos a 12,5 px con interlineado 1,45 y como mucho una línea; el resto en
  un «?» con descripción emergente.

## 3. Páginas

- **Inicio**: `CabeceraPagina` y `EstadoVacio` en recientes.
- **Mis APIs**: fila en cuatro zonas (boceto aprobado: insignia; nombre, estado, etiquetas, URL con copiar y
  «agente · contratos»; uso y presupuesto; acciones). Botón «Ajustes» y `MenuAcciones` (Activar/Desactivar,
  Abrir en Probar, Regenerar la clave, Reiniciar consumo, Eliminar). API principal en «Conexión» y «Ejemplos».
  Modelos agrupados por agente con su color.
- **Agentes**: orden fijo Claude, Codex, Gemini API, Antigravity; tarjetas sin altura forzada.
- **Probar**: lo esencial arriba; esfuerzo, respaldo, JSON y «usar siempre este modelo» en «Opciones de la
  receta» plegable (abierta si alguna tiene valor); respuesta fija al desplazar en escritorio.
- **Historial**: cabecera común; vacío «Aún no hay peticiones» (Ir a Probar) o, con filtro, «Ninguna petición
  con este filtro» (Quitar filtros).
- **Ajustes**: una columna, nombre y explicación a la izquierda y control a la derecha en todas las secciones.
- **Ayuda**: corregir «Haz que arranque solo»: hoy solo en Windows.

## 4. Actualizar sin pantalla en blanco

Al pulsar «Actualizar» la ventana se queda en blanco sin mostrar la descarga. Se depura con
superpowers:systematic-debugging antes de corregir. Sospecha: al detener el servicio el instalador, la app de
escritorio recarga la ventana (`ventana.SetURL` en `escritorio/main.go`) hacia un servicio caído.

Esperado: el avance se ve en el acto; barra y texto durante descarga y verificación; al instalar, «Instalando la
versión X: Tramtai se cerrará y volverá a abrirse» sin recargar la página; la app de escritorio nunca muestra una
ventana vacía (página de estado propia hasta que el servicio responda); si falla, error y «Reintentar».

## 5. Página del producto

- Etiquetas del mapa a la derecha de cada estación, sin cruzar las líneas.
- Sección nueva con capturas reales (Inicio y Mis APIs) según el tema, con `alt` descriptivo.
- Funciones agrupadas en «Conecta», «Controla» y «Protege».
- **Selector de tema** junto al de idioma: Sistema, Claro y Oscuro. Hoy la página solo sigue el tema del sistema
  (`prefers-color-scheme`); el selector lo fuerza con un atributo en `<html>` y se recuerda en el navegador
  (`localStorage`, con su fallo tolerado). Las capturas de la app siguen el tema elegido. Los dos temas se revisan
  con la misma calidad.

## 6. Verificación

Cada página en claro, oscuro y a 375 px; `npm run build` y `TestCobertura` en verde; capturas nuevas para el
README y la página; revisión de diseño y accesibilidad de toda la rama.
