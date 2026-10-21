# La terminal a la par de la Web UI (v0.56)

Diseño del 2026-10-21, acordado con el usuario por secciones.

## Situación de partida

**La terminal interactiva** (`tramtai` sin argumentos, `internal/tui`) es un solo modelo de Bubble Tea v2 de unas 1400 líneas, con los colores escritos a mano y 10 pantallas:
- espacios: ver todos, activar y detener;
- Probar: modelo, archivos y mensaje;
- historial: los últimos 50 y su detalle;
- Mis APIs a medias: crear con nombre, puerto y modelo, activar, eliminar y regenerar la clave;
- configuración y agentes, solo para leer;
- estado del servicio.

**Lo que le falta frente a la Web UI:**
- la red en vivo;
- el consumo y el presupuesto;
- editar una API;
- la receta completa, las plantillas, «Ver como código» y guardar como API en Probar;
- filtros y páginas en el historial;
- conectar agentes y cambiar su autenticación;
- confiar, buscar la carpeta y olvidar un espacio;
- casi todos los Ajustes;
- la Ayuda, las novedades y Acerca de.

**Cómo funciona hoy:**
- **Conexión con el servicio:** la terminal habla con él solo por `internal/cliente`, con el token de servicio, y nunca por `/ui/*`.
- **Lógica duplicada con la CLI:** la CLI (`cmd/tramtai`) no se puede importar, así que la terminal repite parte de su lógica: las rutas permitidas, las opciones de una API y elegir el espacio.
- **Uso solo con la terminal:** en Windows ya se puede usar Tramtai sin la app (`tramtai servicio iniciar -d` y `tramtai terminal`). Pero el arranque automático solo registra la app de escritorio con su bandeja: no hay forma de que arranque solo el servicio.
- **La Ayuda no llega a Go:** sus temas son TypeScript (`web/src/ayuda/temas`), y Go no los puede leer.

## Decisiones del usuario

- **«A la par»:** las 8 páginas (Inicio, Agentes, Probar, Mis APIs, Historial, Espacios, Ajustes y Ayuda), más las novedades con su botón de actualizar, y Acerca de. Lo que no cabe en texto se adapta.
- **«Solo terminal en Windows»:** Tramtai se instala igual que hoy, pero se puede usar entero sin abrir la app, y el arranque automático puede ser solo del servicio, sin ventana ni bandeja. La instalación sin la app queda para el modo servidor.
- **Ayuda:** dentro de la terminal, generada de la misma fuente que la app y la web. Funciona sin conexión, en es/en y con búsqueda.
- **Construcción (opción A):** se rehace la terminal por páginas, con componentes compartidos, `bubbles` y `lipgloss` v2. La lógica común con la CLI sube a `internal`.
- **Orden:** el modo servidor (Linux y Windows Server) queda para la v0.57. Lo demás se corre un número: configurar clientes pasa a la v0.58, uso y diagnóstico a la v0.59, y la actualización con vuelta atrás a la v0.60.
- **Cada release** pone al día, si hace falta, la Ayuda, la guía de desarrolladores, el README y la web como presentación del producto.

## 1. Estructura de la terminal

**Paquetes:**
- `internal/tui` es la aplicación: arranque, menú, navegación entre páginas, barra de estado, idioma y tema.
- `internal/tui/paginas/<página>` tiene un modelo por página: `inicio`, `agentes`, `probar`, `apis`, `historial`, `espacios`, `ajustes` y `ayuda`. Cada uno lleva su vista, sus teclas y sus pruebas, y no depende de las otras páginas. Se comunican con la aplicación por mensajes, por ejemplo «ir a Probar con esta API» o «ir al historial de esta API».
- `internal/tui/comp` tiene las piezas compartidas:
  - lista con buscador, filtros y orden;
  - formulario con validación al escribir;
  - tabla;
  - visor con desplazamiento;
  - selectores de modelo, agente y esfuerzo;
  - confirmación para lo que borra;
  - avisos;
  - panel de Ayuda.
- La lógica común de la CLI y la terminal pasa a `internal` y la usan las dos: opciones de una API, elegir el espacio, rutas permitidas para archivos, puerto, arranque automático y actualizar.

**Navegación** (como la Web UI):
- Menú lateral con los mismos grupos y el mismo orden que `web/src/menu.ts`: Empezar (Inicio, Agentes, Probar), Conectar (Mis APIs, Historial) y Organizar (Espacios, Ajustes), con Ayuda aparte. Una prueba exige que el orden sea el de la Web UI.
- Con menos de 100 columnas, el menú se pliega y se abre con Tab.
- Cabecera: el espacio activo, con un cambio rápido, y el estado del servicio. Pie: las teclas de la pantalla.

**Teclas:**
- Flechas e Intro para moverse y elegir.
- Comunes a todas las páginas: `/` busca, `n` crea, `e` edita, `x` borra (con confirmación), `?` abre la Ayuda de la página, `Esc` vuelve y `q` sale.
- Del `1` al `7` se va directo a cada página.

**Aspecto:**
- Tema claro y oscuro, según el fondo de la terminal, con contraste suficiente en los dos.
- Funciona igual en Windows Terminal, PowerShell clásico, Linux, macOS y por SSH.
- Sin color con `NO_COLOR` o en una terminal que no lo admite.

**Idioma:** todos los textos van con `idioma.T`/`Tf` y `TestCobertura` exige la traducción al inglés de cada uno.

## 2. Las páginas

**Inicio**
- Estado del espacio: su dirección, la API, el servicio y los agentes listos.
- **La red en vivo:** una lista que se actualiza sola con `/api/v1/eventos`. Cada petición en curso enseña su API, su agente, la etapa y el tiempo, y al terminar se queda unos segundos marcada como hecha o fallida.
- Contadores de completadas, en curso y con error.
- Primeros pasos, donde «Haz el recorrido» se convierte en «Ver la Ayuda de Empezar».
- Avisos: espacios con problemas, el conversor y la sugerencia de una API propia.

**Agentes**
- Lista con el estado de cada uno: listo, sin sesión, desconectado o sin instalar.
- Acciones: volver a comprobar, conectar o desconectar, y pasar de suscripción a clave de API.
- Detalles, catálogo de modelos y cómo instalar el que falte.
- «Probar con este agente» lleva a Probar con ese agente elegido.

**Probar**
- La receta completa:
  - instrucciones, modelo, forzar, esfuerzo y respaldo;
  - salida estructurada con campos o JSON, plantillas y validación del esquema (`/api/v1/esquemas/validar`);
  - reintentos, perfil y tiempo.
- El mensaje y los archivos, con los límites a la vista.
- La respuesta en vivo, con cancelar y corregir. Después se puede copiar, «Ver como código» (curl, Python y JavaScript, para copiar) y guardar como API o actualizar una que ya existe.
- Pruebas recientes con «Usar de nuevo», y un borrador que se guarda mientras escribes.

**Mis APIs**
- **Pestaña APIs:**
  - lista con buscador, filtros (grupo, estado, agente, contrato, uso) y orden;
  - grupos plegables y activar o desactivar un grupo entero.
- **Crear y editar una API:** nombre, grupo, dirección (ruta o puerto), contratos, permitidos, alias, receta y presupuesto.
- **Detalle de una API:**
  - copiar la URL y la clave, y regenerar la clave;
  - consumo y presupuesto del día y del mes (peticiones, tokens y USD) con barras, y reiniciar el consumo;
  - ver sus peticiones, abrirla en Probar y eliminarla.
- **Pestaña «Conexión directa»:** la clave del espacio (ver, copiar y regenerar), sus ejemplos y el presupuesto del espacio.

**Historial**
- Filtros por origen, API, grupo y texto, y «Ver más» para cargar más páginas.
- Detalle con la solicitud, la salida, el registro y, mientras corre, el progreso en vivo.
- Reintentar, corregir, cancelar, eliminar y vaciar.

**Espacios**
- Lista con el estado de cada espacio.
- Activar, detener, confiar, buscar la carpeta y olvidar.
- Abrir un proyecto escribiendo su carpeta, con autocompletado de rutas.

**Ajustes**
- **Pestaña «Este espacio»:**
  - modelo y esfuerzo predeterminados;
  - configuración efectiva, que dice de dónde viene cada valor y permite volver al de Global;
  - retención;
  - datos: eliminar APIs y vaciar.
- **Pestaña «Todo Tramtai»:** puerto, conversor, arranque automático (sección 3), buscar actualizaciones y Acerca de.

**Ayuda**
- Lista de temas y búsqueda.
- Cada tema en texto: pasos, tablas, preguntas, «Qué hago» ante cada error y «Qué verás en pantalla».
- «Ver en la web» abre el tema en el navegador para ver las capturas y los diagramas.
- El `?` de cada página abre su tema.

**Novedades:** si hay una versión nueva, la terminal avisa al abrirse, enseña sus notas y la instala, igual que `tramtai actualizar`.

## 3. Servicio, cliente, arranque y Ayuda

**Cliente** (`internal/cliente`): gana los métodos que faltan:
- editar una API, su presupuesto y reiniciar el consumo;
- el historial con filtros y cursor, su registro y vaciarlo;
- la configuración del espacio y volver al valor heredado;
- la retención y los valores predeterminados;
- validar un esquema y los límites;
- leer el flujo `/api/v1/eventos`, que la terminal cierra al salir de Inicio.

Casi todas estas rutas ya existen en `/api/v1` y aceptan el token de servicio.

**Servicio:** lo que hoy solo está en `/ui` y necesita al servicio pasa también a la API de administración, con el token de servicio y los mismos permisos que activar, olvidar y conectar un agente:
- confiar en un espacio;
- buscar su carpeta;
- abrir un proyecto;
- pasar un agente de suscripción a clave.

Las rutas `/ui` no cambian, y una clave de aplicación sigue sin poder usar ninguna de estas.

**En el propio equipo:** el puerto, el arranque automático y las actualizaciones las resuelve la terminal en el equipo, con el mismo código que la CLI, que se sube a `internal`.

**Arranque automático:**
- En Ajustes y con `tramtai servicio autoarranque activar` se elige qué arranca al iniciar sesión:
  - **«La app en la bandeja»**, como hasta ahora;
  - **«Solo el servicio»**, que atiende a las apps en segundo plano sin ventana ni bandeja.
- En Windows, «solo el servicio» arranca sin abrir ninguna ventana de consola. En macOS y Linux usa su mecanismo de inicio de sesión con un comando.
- `tramtai abrir` y `tramtai actualizar` funcionan igual con las dos opciones.

**Ayuda generada:**
- `npm run ayuda-terminal`, que también se lanza dentro de `npm run sitio`, convierte los temas en un archivo de texto es/en que va incrustado (`go:embed`) en `tramtai`.
- Las capturas, los diagramas y los recorridos se cambian por «Ver en la web».
- El CI comprueba que ese archivo no se desfase de los temas.

## 4. Documentación, web y pruebas

**Ayuda:**
- Empezar explica cómo usar Tramtai solo con la terminal.
- Los temas de cada página dicen, en una línea, cómo se hace lo mismo en la terminal.
- Ajustes explica «Solo el servicio».

**Portada:**
- La novedad «Todo desde la terminal» y una tarjeta en Funciones.
- «Así se ve» suma dos capturas de la terminal: Inicio con la red en vivo y Mis APIs. Las saca `npm run capturas`, en claro y oscuro.

**Guía de desarrolladores:** la sección «Administrar desde la terminal», con los atajos, `tramtai terminal` y cómo entrar por SSH al equipo donde corre Tramtai.

**README, `docs/inicio.md` y `docs/operacion.md`:** la terminal completa, «Solo el servicio» y los atajos.

**`docs/equipo`:**
- cómo añadir una página a la terminal;
- la regla «cada función nueva llega también a la terminal»;
- `pendientes.md` renumerado, con la v0.56 ✅ al publicar.

**Pruebas:**
- **Go:**
  - cada página, con teclas y vista, contra un servicio real en `httptest`;
  - los componentes: lista con filtros, formulario y validación, visor y confirmación;
  - el orden del menú, igual que el de la Web UI;
  - los métodos nuevos del cliente y la lectura de eventos;
  - las rutas de administración nuevas, y que una clave de aplicación no entra;
  - lo que registra «Solo el servicio» en cada sistema.
- **CI:** la Ayuda generada no se desfasa.
- **De punta a punta:**
  - Windows Terminal y PowerShell clásico;
  - una ventana estrecha y `NO_COLOR`;
  - claro y oscuro, español e inglés;
  - por SSH a Linux, si hay dónde;
  - cada página haciendo lo mismo que en la Web UI, con el servicio en marcha y parado.

**Lo que más puede fallar:**
1. **Una ventana muy estrecha o muy baja:** nada se corta sin avisar.
2. **El servicio se detiene a mitad, o el espacio cambia desde la app:** la terminal lo dice y se recupera sin cerrarse.
3. **Pegar textos largos o con saltos de línea** en Probar y en las instrucciones.
4. **La terminal clásica de Windows:** colores, tildes y redimensionado.
5. **La terminal y la Web UI dicen cosas distintas:** tienen que coincidir los nombres, el consumo y los errores.

**Pedido a mitad del release:** en el mapa de la portada, el tranvía pasa por debajo del cartel de Tramtai y no se ve
mientras lo cruza; vale para el mapa ancho y para el estrecho del móvil. Lo comprueba `portada.test`.

**Fuera de la v0.56:**
- instalar sin la app de escritorio y el modo servidor (v0.57);
- ratón en la terminal;
- capturas, diagramas y recorridos dentro de la terminal.
