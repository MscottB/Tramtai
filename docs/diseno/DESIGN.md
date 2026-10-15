---
name: Tramtai
description: Pasarela local que convierte los CLI de IA en API; su Web UI se lee como la señalética de una red de tranvía.
colors:
  acento: "#0b6e64"
  acento-texto: "#ffffff"
  fondo: "#eef2f1"
  lateral: "#f7f9f8"
  superficie: "#ffffff"
  superficie-suave: "#f4f7f6"
  borde: "#dce3e2"
  borde-fuerte: "#c2cccb"
  texto: "#0f1b1e"
  texto-suave: "#55666a"
  ok: "#15803d"
  error: "#c42b2b"
  aviso: "#a16207"
  linea-claude: "#c4561d"
  linea-codex: "#2563eb"
  linea-gemini: "#7c3aed"
  linea-antigravity: "#db2777"
  linea-otro: "#64748b"
  acento-noche: "#2dd4bf"
  acento-texto-noche: "#042f2b"
  fondo-noche: "#0a1116"
  lateral-noche: "#0c151b"
  superficie-noche: "#111c23"
  superficie-suave-noche: "#15222a"
  borde-noche: "#1f2f38"
  borde-fuerte-noche: "#2d404b"
  texto-noche: "#e5edf0"
  texto-suave-noche: "#93a5ac"
  ok-noche: "#4ade80"
  error-noche: "#f87171"
  aviso-noche: "#fbbf24"
  linea-claude-noche: "#f28a4f"
  linea-codex-noche: "#5b8def"
  linea-gemini-noche: "#a78bfa"
  linea-antigravity-noche: "#f472b6"
  linea-otro-noche: "#8a9aa8"
  tablero-fondo: "#0f1b1e"
  tablero-fondo-noche: "#060b0e"
  tablero-texto: "#eef3f2"
  tablero-suave: "#9fb3b8"
  tablero-cifra: "#ffcf5c"
typography:
  display:
    fontFamily: "'Overpass Variable', 'Segoe UI Variable Display', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1.75rem"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.022em"
  cifra:
    fontFamily: "'Overpass Variable', 'Segoe UI Variable Display', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1.75rem"
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: "-0.02em"
    fontFeature: "'tnum'"
  title:
    fontFamily: "'Overpass Variable', 'Segoe UI Variable Display', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 650
    lineHeight: 1.3
    letterSpacing: "-0.008em"
  body:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', system-ui, -apple-system, Roboto, 'Helvetica Neue', sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.5
    fontFeature: "'ss01'"
  estacion:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.9rem"
    fontWeight: 600
    lineHeight: 1.25
  label:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.78rem"
    fontWeight: 600
    lineHeight: 1.3
  tablero-cabecera:
    fontFamily: "'Overpass Variable', 'Segoe UI Variable Display', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.8rem"
    fontWeight: 700
    letterSpacing: "0.06em"
  mono:
    fontFamily: "'Cascadia Code', 'Cascadia Mono', ui-monospace, Consolas, 'SF Mono', monospace"
    fontSize: "0.84rem"
    fontWeight: 400
    lineHeight: 1.55
rounded:
  etiqueta: "6px"
  control: "8px"
  caja: "10px"
  tarjeta: "12px"
  pastilla: "999px"
spacing:
  xs: "6px"
  sm: "12px"
  md: "16px"
  seccion: "20px"
  lg: "24px"
  pila: "28px"
components:
  button-primary:
    backgroundColor: "{colors.acento}"
    textColor: "{colors.acento-texto}"
    rounded: "{rounded.control}"
    typography: "{typography.label}"
  button-primary-noche:
    backgroundColor: "{colors.acento-noche}"
    textColor: "{colors.acento-texto-noche}"
    rounded: "{rounded.control}"
  button-outline:
    backgroundColor: "transparent"
    textColor: "{colors.acento}"
    rounded: "{rounded.control}"
  card:
    backgroundColor: "{colors.superficie}"
    rounded: "{rounded.tarjeta}"
    padding: "18px 20px 14px"
  input-outlined:
    backgroundColor: "transparent"
    textColor: "{colors.texto}"
    rounded: "{rounded.control}"
  estacion:
    backgroundColor: "transparent"
    textColor: "{colors.texto}"
    rounded: "{rounded.caja}"
    padding: "6px 12px"
    height: "52px"
  estacion-activa:
    backgroundColor: "rgba(11, 110, 100, 0.09)"
    textColor: "{colors.acento}"
    rounded: "{rounded.caja}"
  etiqueta:
    backgroundColor: "rgba(85, 102, 106, 0.1)"
    textColor: "{colors.texto-suave}"
    rounded: "{rounded.pastilla}"
    padding: "2px 10px 2px 8px"
    typography: "{typography.label}"
  etiqueta-ok:
    backgroundColor: "rgba(21, 128, 61, 0.1)"
    textColor: "{colors.ok}"
    rounded: "{rounded.pastilla}"
  segmentado:
    backgroundColor: "{colors.superficie-suave}"
    rounded: "{rounded.caja}"
    padding: "3px"
  tablero:
    backgroundColor: "{colors.tablero-fondo}"
    textColor: "{colors.tablero-texto}"
    rounded: "{rounded.tarjeta}"
  bloque-codigo:
    backgroundColor: "rgba(11, 110, 100, 0.05)"
    textColor: "{colors.texto}"
    typography: "{typography.mono}"
    rounded: "{rounded.caja}"
    padding: "12px 44px 12px 14px"
---

# Design System: Tramtai

## Overview

**Creative North Star: "La red de tranvía"**

La Web UI de Tramtai se dibuja como la señalética de una red de transporte. Los agentes son líneas con su propio color, tus apps son paradas de origen, Tramtai es la estación central y cada petición es un trayecto que avanza por estaciones (cola, preparación, ejecución, validación, llegada). El vocabulario visual sale de ese mundo: trazos de 3 a 4 px, estaciones como círculos con aro, giros de 45°, números tabulares y un tablero de salidas en tinta con cifras ámbar.

De día es un andén gris frío con superficies blancas y tinta profunda; de noche, una red azul tinta donde el acento verde azulado se enciende. Los dos modos tienen la misma calidad y comparten cada regla; solo cambian los valores. La densidad es equilibrada: herramienta de operador, no panel de marketing. Las tarjetas son contenedores con borde fino y sombra casi imperceptible; la jerarquía la dan el trazo de las líneas, el color del estado y la tipografía Overpass en títulos y cifras.

El sistema rechaza el panel SaaS de tarjetas iguales con un icono dentro de un recuadro tintado: cuando algo necesita marca visual, lleva la barra o el punto de su línea, no un icono decorativo.

**Key Characteristics:**
- Navegación lateral como una línea vertical continua con estaciones; la actual se rellena del acento.
- Un color de línea por agente, reservado para identificar a ese agente.
- Estados escritos siempre como punto de color más texto, nunca solo color.
- Tablero de salidas oscuro con cifras ámbar tabulares como único bloque de tinta en modo claro.
- Overpass para títulos, cifras y el mapa; la fuente del sistema para el cuerpo; Cascadia Code para código y direcciones.

## Colors

Un acento verde azulado de marca sobre neutros fríos, más una paleta de líneas de agente que solo identifica agentes; todo token tiene par de noche.

### Primary
- **Verde azulado Tramtai** (acento): botón principal, estación activa del menú, hub del mapa de red, enlaces, foco, cursor de texto y estaciones pendientes de la guía. De noche pasa a **Turquesa nocturno** (acento-noche), con texto oscuro (acento-texto-noche) encima, nunca blanco.
- **Velo de acento** (`rgba(11,110,100,0.09)` / noche `rgba(45,212,191,0.11)`): fondo de la estación activa y de la etiqueta informativa.

### Secondary
- **Líneas de agente**: Claude naranja (linea-claude), Codex azul (linea-codex), Gemini violeta (linea-gemini), Antigravity magenta (linea-antigravity), gris pizarra para cualquier otro (linea-otro). Cada una tiene variante de noche más clara. Aparecen como barra de 16×4 px junto al nombre, barra vertical de 5×36 px en listas de agentes, trazo y estación final del mapa.

### Tertiary
- **Ámbar de tablero** (tablero-cifra): solo para las cifras del tablero de salidas.

### Neutral
- **Andén** (fondo / fondo-noche): fondo de la aplicación.
- **Lateral** (lateral / lateral-noche): cajón de navegación; también el relleno de los puntos de estación, para que el trazo parezca pasar por detrás.
- **Superficie** (superficie / superficie-noche): tarjetas, menús, diálogos.
- **Superficie suave** (superficie-suave / superficie-suave-noche): hover de filas, columna de etiquetas de la ficha, control segmentado, bloques de texto.
- **Borde** (borde) y **Borde fuerte** (borde-fuerte): separadores y contornos; el borde fuerte es además el color de la vía sin servicio (trazos del menú, del trayecto y líneas punteadas del mapa).
- **Tinta** (texto) y **Tinta suave** (texto-suave): texto principal y secundario.
- **Estados**: ok, error y aviso, cada uno con su velo al 9-13 % para el fondo de etiquetas y cajas.

### Named Rules
**The Una Línea, Un Color Rule.** Los colores de línea identifican agentes y nada más. No se usan para estados, decoración ni categorías ajenas a un agente.

**The Punto y Texto Rule.** Un estado siempre es un punto de color seguido de su palabra (En marcha, Completado, Sin conectar). El color nunca va solo.

**The Vía Muerta Rule.** Lo que no tiene servicio se dibuja con borde-fuerte y trazo punteado; no se oculta ni se pinta en rojo.

## Typography

**Display Font:** Overpass Variable (autoalojada, con Segoe UI Variable Display de respaldo)
**Body Font:** Segoe UI Variable Text (con la pila del sistema)
**Label/Mono Font:** Cascadia Code (con Cascadia Mono, Consolas)

**Character:** Overpass, nacida de la señalética vial, da a títulos, cifras y nombres del mapa la voz de un cartel de red; el cuerpo usa la fuente del sistema de Windows para que la lectura sea nativa y densa.

### Hierarchy
- **Display** (700, 1.75rem, 1.2, -0.022em, text-wrap: balance): título de página, uno por pantalla.
- **Cifra** (700, 1.75rem, 1.1, tabular): cifras destacadas; en el tablero, 1.6rem en ámbar.
- **Title** (650, 1.0625rem, 1.3): título de sección dentro de una tarjeta, seguido de una línea de explicación en tinta suave.
- **Body** (400, 0.9375rem, ss01 activado): texto general. La salida de un agente se lee a 0.95rem, interlineado 1.65 y máximo 75ch.
- **Estación** (600, 0.9rem, 1.25) con detalle (0.75rem, tinta suave): nombres del menú y pasos.
- **Label** (600, 0.78rem): etiquetas de estado; cabeceras de tabla a 0.75rem, 600, +0.02em, en tinta suave.
- **Cabecera de tablero** (Overpass 700, 0.8rem, +0.06em, mayúsculas): solo la franja superior del tablero de salidas, como en un panel de andén.
- **Mono** (0.84rem, 1.55): bloques de código, JSON, direcciones del servicio, identificadores.

### Named Rules
**The Cifras Tabulares Rule.** Toda cifra (tablas, metadatos, versiones, números de paso, tablero) usa `font-variant-numeric: tabular-nums`.

**The Mayúsculas Solo en el Tablero Rule.** Las mayúsculas espaciadas pertenecen a la cabecera del tablero de salidas. Ningún otro título, etiqueta o sección las usa.

## Layout

Estructura de aplicación con cabecera fija de 60 px (fondo del andén al 86 % con desenfoque de 12 px y borde inferior), cajón lateral de 256 px que se oculta por debajo de 1024 px y contenido centrado a 1200 px como máximo. El relleno de página es 16 px, 24 px desde pantallas grandes. Las secciones de una página se apilan con 28 px de separación; las cabeceras de sección dentro de tarjeta usan 18/20/14 px.

La primera vista de Inicio es la cabecera de página con la dirección del servicio, el mapa de red a todo el ancho, el tablero de salidas y la lista de trayectos recientes. El detalle de un elemento se abre como hoja lateral derecha a toda la altura, pegada al borde, sin radio.

Responsive (punto de corte 600 px): el mapa de red pasa de horizontal (apps → Tramtai → agentes, con giros de 45°) a un único eje vertical; el tablero oculta su columna de detalle; la guía de pasos se apila en filas. Por debajo de 600 px, el mapa horizontal nunca se encoge más de 620 px: se desplaza.

## Elevation & Depth

Híbrido plano: la profundidad la dan los bordes y el tono de superficie, con sombras mínimas de día y ninguna en reposo de noche.

### Shadow Vocabulary
- **Reposo** (`box-shadow: 0 1px 2px rgba(15,27,30,0.05), 0 2px 6px rgba(15,27,30,0.04)`): tarjetas, tablero, opción pulsada del segmentado. De noche, `none`.
- **Alta** (`box-shadow: 0 12px 32px rgba(15,27,30,0.14), 0 2px 6px rgba(15,27,30,0.06)`; noche `0 16px 40px rgba(0,0,0,0.5)`): menús y diálogos.
- **Halo de estación** (`0 0 0 4px` del fondo + `0 0 0 6px` del acento al 35 %): la estación activa del menú; el trayecto usa un halo de 4 px del color del resultado al 22 %.

### Named Rules
**The Borde Antes que Sombra Rule.** Un contenedor se separa con un borde de 1 px; la sombra de reposo solo acompaña, y de noche desaparece.

## Shapes

Esquinas suaves y escalonadas por tamaño: 12 px para tarjetas, menús, notificaciones y tablero; 10 px para cajas internas (código, avisos, fichas, opciones, segmentado, estaciones del menú); 8 px para controles (botones, campos, elementos de lista); 6 px para tooltips y la dirección del servicio; píldora completa para etiquetas de estado y el indicador del servicio. Los botones redondos son círculos.

La geometría propia del mundo: trazos de vía de 3 px (menú, trayecto, origen del mapa) y 4 px (líneas de agente) con extremos redondeados; estaciones de 15 px con aro de 3 px; giros a 45° entre tramos rectos en el mapa; barras de línea de 16×4 px con radio de 2 px.

## Components

### Buttons
- **Shape:** control con esquinas suaves (8 px); los de icono son redondos.
- **Primary:** acento sólido, texto blanco de día y texto oscuro de noche; peso 600, sin mayúsculas, con un brillo interior de 1 px y una sombra de 1-2 px.
- **Outline:** contorno del acento sobre transparente, para la acción secundaria junto a la principal (Mis APIs junto a Probar).
- **Flat:** acciones terciarias y de cabecera; en diálogos de confirmación, la opción secundaria va en tinta suave para que destaque la principal.
- **Hover / Focus:** el de Quasar; foco con contorno de 2 px del acento separado 2 px.

### Chips (etiquetas de estado)
- **Style:** píldora con velo del estado, punto de 6 px en `currentColor` y texto 0.78rem/600. Nunca muestran icono.
- **State:** neutra, ok, aviso, error e info; la info (en proceso) late su punto cada 1.6 s.

### Cards / Containers
- **Corner Style:** 12 px.
- **Background:** superficie.
- **Shadow Strategy:** reposo (ver Elevation & Depth); diálogos y menús, alta.
- **Border:** 1 px de borde; las tarjetas pulsables pasan a borde fuerte al pasar el cursor, y las ideas para probar al acento.
- **Internal Padding:** cabecera de sección 18/20/14 px, separada del contenido por un separador de borde.

### Inputs / Fields
- **Style:** campos con contorno de Quasar, 8 px de radio, trazo en borde fuerte; los numéricos sin flechas.
- **Focus:** el trazo pasa al acento a 1.5 px.

### Navigation
- **Style:** el cajón lateral es una línea vertical de 3 px en borde fuerte que pasa por cada estación. Cada entrada tiene punto con aro de 15 px, nombre (0.9rem/600) y detalle (0.75rem, tinta suave), 52 px de alto mínimo.
- **Hover:** velo neutro de fondo y el aro se oscurece a tinta suave.
- **Activa:** velo del acento, nombre en acento, punto relleno con halo y escala 1.05.
- **Aparte:** Ayuda vive abajo, fuera de la línea, con icono en lugar de estación.
- **Móvil:** el cajón se abre desde el botón de menú por debajo de 1024 px.

### Mapa de red (firma)
SVG a todo el ancho en Inicio: tus apps a la izquierda como estaciones con aro, Tramtai al centro como un bloque del acento (radio 16 px) con su dirección, y los agentes a la derecha unidos por su línea de color de 4 px. Los tramos van rectos, giran a 45° y salen en paralelo, separados 7 px. Lo que no está en servicio va punteado en borde fuerte con la estación vacía y su estado en aviso. Cada grupo es un enlace enfocable.

### Trayecto (firma)
Cinco estaciones en fila unidas por una vía de 3 px: cola, preparación, ejecución, validación, llegada. Las pasadas se rellenan, la actual late con halo cada 1.4 s, y al terminar toda la vía toma el color del resultado (ok, error o tinta suave si se canceló). La misma pieza sirve para la guía de inicio, donde las estaciones pendientes llevan aro del acento y las hechas se rellenan de ok.

### Tablero de salidas (firma)
Banda de tinta con radio de 12 px, una fila por estado: punto de 10 px, nombre (650), detalle en tinta clara, cifra ámbar en Overpass 1.6rem tabular y chevron. Cabecera en mayúsculas espaciadas. Es oscuro en ambos modos; de noche se oscurece más y gana un borde.

### Línea de agente
Barra de 16×4 px del color del agente, nombre en 600 y modelo en tinta suave recortado con elipsis.

### Ficha
Rejilla de dos columnas con borde de 1 px y radio de 10 px: etiqueta en tinta suave sobre superficie suave, valor alineado a la derecha.

### Control segmentado
Filtros como un control segmentado de 10 px con 3 px de relleno sobre superficie suave; la opción pulsada sube a superficie con sombra de reposo y borde.

## Do's and Don'ts

### Do:
- **Do** identificar cada agente con su color de línea (barra de 16×4 px, barra vertical de 5×36 px o trazo de 4 px).
- **Do** escribir cada estado como punto más palabra, con el velo del estado de fondo.
- **Do** dibujar progresos y secuencias como estaciones unidas por una vía de 3 px.
- **Do** usar Overpass para títulos de página, títulos de sección, cifras y textos del mapa, y cifras tabulares en todo número.
- **Do** definir cada color nuevo como variable con su par de noche en `body.body--dark`.
- **Do** respetar `prefers-reduced-motion`: todas las animaciones y transiciones se anulan.

### Don't:
- **Don't** construir paneles de tarjetas iguales con un icono dentro de un recuadro tintado.
- **Don't** usar los colores de línea para estados, categorías o decoración.
- **Don't** comunicar un estado solo con color.
- **Don't** usar mayúsculas espaciadas fuera de la cabecera del tablero de salidas.
- **Don't** poner texto blanco sobre el acento de noche; va el texto oscuro del acento.
