# v0.56: portada renovada (cuatro secciones)

Parte de la v0.56: la portada (`site/index.html`, `site/estilo.css`, `site/sitio.js`) se estrena con la versión. La web se
publica al terminar Publicar, así que sale a la vez que los binarios.

## Por qué

Quien llega a la portada tiene que entender en segundos qué problema resuelve Tramtai y cómo. Hoy:
- **La cabecera** cuenta cuatro escenas en una línea gris pequeña bajo el mapa, con cuatro puntos: el texto no se ve.
- **«Los agentes que ya pagas»**, la parte que enseña la solución, quedó en dos cajas de texto. La auditoría del
  2026-10-22 quitó sus seis motivos porque algunos prometían de más.
- **El recorrido** reduce capturas de 1440 px, con la pantalla oscurecida por el recorrido. El texto de la app y del
  globo del recorrido no se lee, y se navega con 14 puntos abajo.
- **«Una red con una línea por agente»** son 12 tarjetas de texto. Es lo que hace la app y no tiene nada visual.

## Decisiones (elegidas con maquetas, 2026-10-25)

### 1. «Los agentes que ya pagas»: antes y después dibujado (A)

- **Dónde:** pasa justo después de la cabecera, antes de «Conecta estos agentes».
- **Dos diagramas lado a lado:**
  - **«Sin Tramtai»:** tres apps; cada una con su clave de pago y su SDK hacia su proveedor. Pie: «Una clave, un SDK y una
    factura por proveedor».
  - **«Con Tramtai»:** las apps → Tramtai → los agentes que ya tienes, con las líneas de color de cada agente. Pie:
    «Una URL local y la suscripción o la clave que ya tienes».
- **Debajo, los seis motivos con icono, en 3×2,** escritos sin promesas absolutas:
  - **Lo que ya pagas:** tu suscripción de Claude, ChatGPT o Copilot, o tu clave de cada agente; cambias de uno a otro
    cuando quieras.
  - **En tu equipo:** Tramtai escucha solo en tu equipo y no tiene servidor propio.
  - **Compatible:** rutas de OpenAI y Anthropic y una API propia, con respuesta en vivo y herramientas.
  - **Respaldo:** si un agente llega a su límite o falla, la petición puede seguir con el siguiente de tu lista.
  - **Por proyecto:** cada espacio con su historial, sus APIs y sus ajustes.
  - **Bajo control:** una API por app con su presupuesto, y el mapa en vivo de cada petición.
- **En el móvil:** los diagramas van uno debajo de otro; los motivos, en una columna (dos en tableta).
- Los diagramas son SVG en línea con su texto visible, legibles sin imágenes.

### 2. La cabecera: el mapa y el código de la escena, lado a lado (B)

- **Arriba, cuatro pestañas con nombre** en lugar de los puntos:
  1. «Resume un contrato con Claude»;
  2. «Un PDF a JSON con Codex»;
  3. «Traduce con GitHub Copilot»;
  4. «Respaldo: de Claude a Codex».
- **Contenido:** el mapa a la izquierda y, a la derecha, el panel de la escena:
  - su título;
  - una línea que la explica;
  - el código que la provoca (los `codigo-mapa-N` que ya existen).
- **El tranvía** sigue cruzando el mapa como hoy.
- **El giro:**
  - las escenas cambian solas cada ~6,5 s;
  - se para al pasar el ratón o el foco por el bloque, o al elegir una pestaña;
  - con `prefers-reduced-motion` no gira solo.
- **Accesibilidad:** las pestañas son un `tablist` con `aria-selected` y se manejan con las flechas.
- **En el móvil:** el panel va debajo del mapa.

### 3. El recorrido: la zona del paso ampliada, con la lista de pasos al lado (B)

- **A la izquierda:** la lista de los 14 pasos, agrupada por página (Inicio, Agentes, Probar, Mis APIs, Espacios…).
  Cada paso es un botón y el actual está marcado.
- **A la derecha, la imagen del paso:** solo la zona que señala, ampliada y nítida (capturada a 2×), sin el globo del
  recorrido.
  - Debajo: «Paso n de 14 · Página», el título del paso y su explicación, con letra grande.
  - Una barra fina de progreso sustituye a los 14 puntos.
- **Flechas grandes a los costados** de la imagen (al menos 44 px), y ← → del teclado. En el primer y en el último paso,
  la flecha que no lleva a ninguna parte se desactiva.
- **En el móvil:** la lista pasa a ser un desplegable encima de la imagen; las flechas siguen a los costados.
- **Capturas:** `capturas.mjs` saca, para cada paso, la zona del elemento señalado con un margen y sin el globo
  (`bienvenida-<n>-<tema>-<idioma>-<ancho>.webp`, claro y oscuro, es y en).
  - Sustituyen a las de pantalla completa.
  - Los títulos y las explicaciones salen de los mismos textos que el recorrido de la app.

### 4. «Una red con una línea por agente»: un plano de metro (A)

- **Cuatro líneas de color, una por grupo** (Conecta, Controla, Protege, Usa), con una parada por función. Son las 12
  funciones de hoy.
- **Cada parada es un botón.** Al pulsarla, su explicación (los textos de hoy) aparece en un panel debajo del plano.
  Empieza elegida la primera.
- **En el móvil, o sin JavaScript:** se ve como lista. Cada grupo es una línea vertical con sus paradas y su texto
  completo debajo, así que todo se lee sin pulsar nada.
- **Colores:** los tokens de DESIGN.md, también en claro.

## Reglas para las cuatro

- **Idiomas y temas:**
  - todo en español y en inglés (`<span lang="es">` y `<span lang="en">`, como el resto de la portada);
  - en claro y en oscuro, con los tokens de DESIGN.md;
  - sin brillos de color ni sombras de neón.
- **Tamaños de pantalla:** de 360 px a escritorio, sin barra horizontal; el texto de lectura, a 14 px como mínimo.
- **Teclado y accesibilidad:** todo se usa con el teclado, con el foco visible; las imágenes tienen su texto
  alternativo.
- **Sin JavaScript:** se ve todo el contenido; solo se pierden la rotación y la interacción.
- **Textos:** con el glosario (Espacio · API · Petición, «costo», «Playground» en inglés) y sin promesas absolutas.
- **Lo demás no cambia:** el resto de la portada, la Ayuda y la Web UI quedan como están.

## Pruebas

- **`web/scripts/portada.test.ts`** comprueba:
  - el nuevo orden de las secciones;
  - los seis motivos y las palabras prohibidas;
  - las cuatro pestañas de la cabecera, en es y en, y su panel con código;
  - los 14 pasos del recorrido en la lista, las flechas con su `aria-label` y las capturas de zona de cada paso;
  - las 12 paradas del plano y la lista completa sin JavaScript.
- **A ojo,** con Edge sin ventana:
  - a 1440, 768 y 390 px de ancho;
  - en claro y en oscuro;
  - en es y en.
- **Lo de siempre:** `npm --prefix web test` y `npm run sitio`.

## Fuera

- Las capturas de la terminal (v0.58).
- Rehacer las demás secciones de la portada.
