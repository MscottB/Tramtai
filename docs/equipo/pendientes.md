# Pendientes

## Hitos

| Hito | Estado | Contenido |
|---|---|---|
| M0 | ✅ | Validación de CLIs, entorno y términos de uso |
| M0b | 🟡 | Negativa OK; falta la parte positiva con claves reales ([M0B.md](M0B.md)) |
| M1 | ✅ | Núcleo, configuración por capas, SQLite, idempotencia, planificador |
| M2 | ✅ | Supervisión de procesos (Job Objects y grupos Unix), adaptadores Claude y Codex, streaming, timeout, cancelación y perfiles de modelo |
| M3 | ✅ | Cargas por hash, conversión con markitdown 0.1.8, ZIP seguro con límites, salidas con JSON Schema (nativo en Claude y Codex) |
| M4 | ✅ | Servicio HTTP: API nativa `/api/v1` con SSE reanudable, cargas, capacidades y modelos; fachadas OpenAI (chat, responses, models) y Anthropic (messages) con política de parámetros |
| M5 | ✅ | Servicio único con instancias global y por proyecto por token, descriptor y segundo plano, puntos de acceso con recarga en caliente, terminal interactiva |
| M6 | ✅ | Web UI embebida (Vue 3 + Quasar) con sesión por cookie y CSRF, aplicación de escritorio Wails v3 con bandeja, y recorrido compartido: CLI, terminal, Web y escritorio sobre los mismos trabajos |
| M7 | ✅ | Retención con purga periódica (resultado, entradas, eventos, metadatos, idempotencia y cargas), 410 `resultado_expirado`, cierre con plazo de drenaje y recuperación verificada tras matar el servicio |
| M8 | ✅ | Distribución (instalador, portable, .deb, .rpm, .dmg, Homebrew, winget), documentación y matriz multiplataforma |

## Hoja de ruta por releases

Cada tema va en su propio release, con su spec, plan, revisión y documentación (Ayuda, README, sitio y docs):

| Release | Contenido | Estado |
|---|---|---|
| v0.34.0 | Recetas y respaldo entre agentes, claves cifradas y huella de las claves de API, instalador con RTF | ✅ |
| v0.35.0 | Presupuestos por API y por instancia; mapa de la red sin cortes | ✅ |
| v0.35.1 | «Gemini API» en los textos: el agente `gemini` llama a la API de Gemini directamente, no a Gemini CLI, así que sigue disponible y no se quita | ✅ |
| v0.36.0 | Rediseño de la presentación: cabecera común con Volver, Mis APIs, estados vacíos, esfuerzo predeterminado por modelo; página del producto con tema claro u oscuro y capturas nuevas | ✅ |
| v0.36.1 | Probar: la línea de la ruta ya no se sale de la tarjeta mientras la petición avanza | ✅ |
| v0.37.0 | Actualizaciones: se borran los instaladores descargados y, tras actualizar, un diálogo con las novedades de las versiones saltadas | ✅ |
| v0.38.0 | Mis APIs con la lista primero (buscador, grupo, orden), grupos plegables y mapa de Inicio por grupo; Historial por origen y grupo con Ver más; límites del historial (tope de peticiones, plazos en Ajustes, compactar la base); origen de cada petición (Probar, terminal, CLI, app o API) | ✅ |
| v0.39.0 | Pulido: cuenta atrás de 10 s antes de instalar (Reiniciar ahora o Cancelar); detalle de cada API en Mis APIs con cómo conectarla y sus acciones; todos los pendientes de las revisiones cerrados | ✅ |
| v0.40.0 | Probar: esfuerzo junto al modelo, esfuerzo por agente de respaldo con su predeterminado, registro con tiempos por etapa, historial breve de pruebas y respuesta con formato Markdown; ventana de progreso al instalar una actualización (dice Actualizar si ya está instalado); Acerca de Tramtai | ✅ |
| v0.41.0 | Seguridad y límites: la entrada que no cabe en el modelo no llega al CLI (salta al respaldo o falla con entrada_demasiado_grande); Antigravity solo desde Probar y la terminal; cuota y formatos de cargas, topes de conversión y procesos, delimitadores con código, entorno sin secretos, salida sin claves; canarios con los CLI reales; límites a la vista | ✅ |
| v0.42.0 | API propia explicada: guía «¿Qué URL uso?» en Mis APIs, ejemplos de la API propia en curl, Python y Node, «Ver como código» en Probar y descripción OpenAPI (`/api/v1/openapi.json`, Descargar OpenAPI); el aviso de actualización muestra cada versión que se salta; la versión solo en Acerca de, como en GitHub; selector de modelo agrupado por agente con la versión exacta (también en el respaldo); Ayuda con recorrido, guías paso a paso, conceptos, errores y buscador; agentes no disponibles apartados en el selector y esfuerzo predeterminado en Ajustes; «Buscar actualización ahora» | ✅ |
| v0.43.0 | Agentes por módulos: cada agente en `internal/agentes/<nombre>` con su ficha, un catálogo que consulta el resto del código y `GET /api/v1/agentes`; la Web UI saca nombres y colores de las fichas; Mis APIs en pestañas (APIs por aplicación y API principal) con filtro por estado y agente y páginas de 20; Agentes en filas con buscador, filtro y los no instalados aparte; cómo añadir un agente (`nuevo-agente.md`) y la M0 de Copilot CLI, OpenCode y Hermes Agent | ✅ |
| v0.44.0 | GitHub Copilot CLI como agente (`internal/agentes/copilot`: sin herramientas ni MCP, `COPILOT_HOME` privado, texto en vivo, modelo efectivo también con el automático, esquema en las instrucciones, `copilot.exe` nativo en Windows; canario pasado con Copilot Free); modelos sin esfuerzo en la ficha; novedades de cada versión también en inglés (`-ResumenEn`); web del producto con sección de agentes, funciones nuevas y capturas de Agentes y Probar; la cola respeta el orden por agente con la máquina cargada | ✅ |
| v0.45.0 | Antigravity sin herramientas (hook que las niega, comprobado con el CLI real), con el mensaje por stdin, respuesta en vivo y esfuerzo; textos de Copilot para cualquier plan; notas de versión cortas, para quien usa Tramtai y en dos idiomas; página del producto renovada con el logo de GitHub | ✅ |
| v0.46 | Respuestas JSON mejoradas (`releases/v0.46-json/spec.md`): plantillas listas y editor de campos en Probar y Mis APIs, validación que explica cada problema (`errores_salida`, `POST /api/v1/esquemas/validar`), correcciones configurables (`salida.reintentos`, `reintentos_salida`, 0 por defecto) y «Corregir» a mano; GitHub Copilot con su nombre completo en todas partes | ✅ |
| v0.46.1 | macOS identifica al dueño de la conexión (`net.inet.tcp.pcblist_n`): las etiquetas `cli` y `terminal` valen y el navegador entra a la Web UI sin código; el CI de macOS vuelve a bloquear; prueba con carreras arreglada; Publicar más corto (wails3 en caché desde `main` y pruebas de Windows en paralelo) | ✅ |
| v0.47 | Correcciones de la revisión completa del código (`releases/v0.47-correcciones/spec.md`): Publicar con permisos mínimos y actualizaciones firmadas, configuración atómica, las mismas reglas para las APIs en la Web, la CLI y la terminal, trabajos que no quedan colgados, errores pequeños de Go y de la Web UI, Mis APIs en la bandeja con muchas APIs y el mapa del sitio como el de la app | ✅ |
| v0.48 | Portable de verdad (`releases/v0.48-portable/spec.md`): los datos junto al `.exe` con `portable.txt`, carpeta solo para el usuario, instalada y portable a la vez sin pisarse (servicio, ventana, «Iniciar con Windows», cookie y reinicio), el instalador solo cierra lo de su carpeta, la ayuda de la CLI nombra a todos los agentes | ✅ |
| v0.49 | Orden del código (`releases/v0.49-orden/spec.md`): contrato ligero entre cliente y servicio (la app de escritorio deja de enlazar el servicio), la CLI y la terminal sobre la API del servicio, archivos grandes divididos, capacidades en paralelo y modelos efectivos con un índice, pruebas de Codex, Claude y la Web UI, CI reutilizable desde Publicar, Dependabot e índice de `docs/equipo/` | ⬜ |
| v0.50 | Archivos: markitdown incluido con Tramtai, aviso antes de trabajar archivos si falta, el recorrido de un archivo por cada API y cómo lo usa quien desarrolla | ⬜ |
| v0.51 | Herramientas (`tools`) explicadas para cualquier lenguaje, con ejemplos en Python, JavaScript, Go y curl, y guía del desarrollador | ⬜ |
| v0.52 | Ayuda renovada en la app y en la web: menos texto, con diagramas, ejemplos, búsqueda y capturas | ⬜ |
| v0.53 | Diagramas vivos: en la web, el recorrido llega a los agentes con ejemplos; en la app, el mapa muestra el recorrido de un trabajo en curso | ⬜ |
| v0.54 | La terminal a la par de la Web UI (Panel, consumo, Mis APIs completo, Probar, plantillas, Ajustes, Ayuda) y Tramtai solo terminal en Windows | ⬜ |
| v0.55 | Configurar los clientes solos (Claude Code, Codex, Cline…), con copia de su configuración antes de tocarla | ⬜ |
| v0.56 | Uso y diagnóstico: panel de uso con tendencias, alias que también fijan el esfuerzo y «Comprobar conexión» | ⬜ |
| v0.57 | Actualización que vuelve a la versión anterior si la nueva no arranca | ⬜ |
| — | OpenCode como agente nuevo (spec en `opencode.md`, rama `feat/opencode`; M0 hecha) | ⬜ |
| — | Hermes Agent como agente nuevo (instalado; falta `hermes setup` del usuario y su M0) | ⬜ |
| — | Proveedor compatible con Anthropic en el adaptador de Claude (DeepSeek, Kimi, GLM) | ⬜ |
| — | Webhooks al terminar un trabajo y avisos de presupuesto (`presupuesto.aviso`, `presupuesto.agotado`) | ⬜ |
| — | Comparar agentes en Probar | ⬜ |
| — | Tramtai como servidor MCP | ⬜ |
| — | Cursor CLI | ⬜ |

Pendientes ya asignados:

- **Webhooks:** los avisos del 80 % y del 100 % se evalúan con los periodos de la hora actual, pero el consumo va al día de aceptación: un cruce del día anterior causado por un trabajo que termina pasada la medianoche no se registra. Evaluar también el periodo de aceptación al sumar.

## Tareas

- [ ] Firma de código del instalador y de los `.exe` (SmartScreen avisa de «aplicación desconocida» sin ella): Azure Trusted Signing o un certificado OV.
- [ ] Ejecutar M0b con claves reales y registrar el resultado en [M0B.md](M0B.md).
- [ ] Revisar Wails en cada beta (fijado en v3.0.0-beta.28) y migrar a la versión estable cuando salga.
- [ ] Historial permanente, cifrado propio y cache documental (`retencion.guardar_historial`, `cifrado_aplicacion`, `cache_conversion_habilitada`): hoy la configuración los rechaza con «no soportado todavía».
- [ ] Probar con una clave real los adaptadores `gemini` y `antigravity` (hoy con servidor y CLI simulados). En `agy`, el canario con `GEMINI_API_KEY` y `modelProvider: gemini` (`go test -tags canario`), y la M0 del hook en Linux y macOS; si pasan, `PermitidoEnApps: true` en su ficha.
- [ ] **Antigravity con la cuenta personal (evaluado 2026-10-12, no se implementa por ahora).** OmniRoute ofrece `agy` por OAuth: importa el archivo de token del CLI, detecta su sesión local o inicia sesión con Google, y usa el backend de Antigravity (incluidos los modelos Claude). Tramtai no lo hace por dos motivos: nunca lee ni copia credenciales de los CLIs, y la FAQ de Antigravity dice que usar software de terceros para acceder a Antigravity viola sus términos (hay suspensiones de cuentas con 403 documentadas en el foro de Google AI); Google recomienda una clave de Vertex o AI Studio, que es lo que Tramtai ya usa. Revisar si Google publica un modo permitido para herramientas de terceros; mientras tanto, como mucho, un modo `suscripcion` opt-in que solo ejecute el `agy` oficial con su propia sesión (sin tocar tokens), con aviso explícito de riesgo de suspensión.
- [ ] Memoria entre conversaciones (engram u otra vía MCP): evaluar una opción por proyecto (en `consciente_proyecto`, que ya carga la configuración del proyecto) y si tiene sentido una global; hoy Tramtai es solo puente, sin memoria compartida entre aplicaciones.
