# Superficie: Web UI de Tramtai (modo Operate, code-led)

Público: desarrolladores y usuarios avanzados en Windows que conectan sus CLI de IA como API locales. Tarea: ver si todo funciona, probar, conectar apps y revisar peticiones. Densidad equilibrada. Claro y oscuro de igual calidad.

## Direction contract

THESIS: Tramtai es una red de tranvía: los agentes son líneas, tus apps son paradas de origen y cada petición es un trayecto con estaciones. Rechaza el panel SaaS de tarjetas iguales con icono en recuadro tintado.

OWN-WORLD: señalética de red de transporte. Andén gris frío (#EEF2F1) con superficies blancas y tinta profunda (#0F1B1E); de noche, red nocturna azul tinta (#0A1116). Acento verde azulado de la marca. Cada agente tiene su color de línea (Claude naranja, Codex azul, Gemini violeta, Antigravity magenta). Barras de línea de 4 px, estaciones como círculos con aro, números tabulares, estados escritos con su punto.

STORY: el usuario ve de un vistazo la red (apps → Tramtai → agentes) y su estado, sigue cada petición como un trayecto y entiende dónde se detuvo.

FIRST VIEWPORT: en Inicio, cabecera con la dirección del servicio; debajo, el mapa de la red a todo el ancho; luego un tablero de salidas con las cifras de la semana y la lista de trayectos recientes.

FORM: mapa de red de transporte (1.º de la lista). seed: none (unavailable)

Firma: el menú lateral es una línea con estaciones; el trayecto de una petición avanza por sus estaciones (cola, preparación, ejecución, validación, llegada) en Probar y en el Historial.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
