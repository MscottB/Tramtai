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
| M8 | ⬜ | Distribución, documentación y matriz multiplataforma |

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
| v0.44 | Copilot CLI como agente nuevo (M0 hecha: `m0-agentes.md`; el usuario tiene Copilot Free, solo modelo automático) | ⬜ |
| v0.45 | OpenCode como agente nuevo (`opencode run`; M0 hecha) | ⬜ |
| v0.46 | Hermes Agent como agente nuevo (instalado; falta `hermes setup` del usuario y su M0) | ⬜ |
| — | Webhooks al terminar un trabajo y avisos de presupuesto (`presupuesto.aviso`, `presupuesto.agotado`) | ⬜ |
| — | Comparar agentes en Probar | ⬜ |
| — | Tramtai como servidor MCP | ⬜ |
| — | Cursor CLI | ⬜ |

Pendientes ya asignados:

- **Webhooks:** los avisos del 80 % y del 100 % se evalúan con los periodos de la hora actual, pero el consumo va al día de aceptación: un cruce del día anterior causado por un trabajo que termina pasada la medianoche no se registra. Evaluar también el periodo de aceptación al sumar.

## Tareas

- [ ] **Activar CI/CD y la página al hacer público el repositorio.** Los flujos ya están en `.github/workflows/`: `ci.yml` (vet y pruebas en Windows, Linux y macOS, y la Web UI), `publicar.yml` (en cada etiqueta `v*` compila, genera el instalador y crea el release con `SHA256SUMS.txt`) y `paginas.yml` (publica `site/` en GitHub Pages). En repositorios públicos los runners estándar no consumen minutos ni el almacenamiento de la cuenta. Falta: hacer público el repo, elegir *Settings → Pages → Source: GitHub Actions* y comprobar la primera ejecución de cada flujo (macOS nunca se ha probado en un equipo real y su resultado no bloquea).
- [ ] Firma de código del instalador y de los `.exe` (SmartScreen avisa de «aplicación desconocida» sin ella): Azure Trusted Signing o un certificado OV.
- [ ] Ejecutar M0b con claves reales y registrar el resultado en [M0B.md](M0B.md).
- [ ] Compilar y probar la aplicación de escritorio en macOS y Linux (necesita CGO y WebKit; hoy solo Windows).
- [ ] Revisar Wails en cada beta (fijado en v3.0.0-beta.28) y migrar a la versión estable cuando salga.
- [ ] Historial permanente, cifrado propio y cache documental (`retencion.guardar_historial`, `cifrado_aplicacion`, `cache_conversion_habilitada`): hoy la configuración los rechaza con «no soportado todavía».
- [ ] Probar con una clave real los adaptadores `gemini` y `antigravity` (hoy con servidor y CLI simulados). En `agy`, comprobar si acepta el prompt por stdin (`--input-format stream-json`) para quitar el límite de 28 000 caracteres por línea de órdenes, y validar `permissions.deny` y `toolPermission: strict` en su `settings.json` (junto a M0b). **Riesgo acotado (v0.41):** Antigravity ya no se usa desde apps ni APIs (`adaptadores.antigravity.permitir_apps` lo habilitaría), rechaza `consciente_proyecto`, su mensaje nunca empieza por `-` y su configuración es privada. Para habilitarlo hace falta su canario (`pruebas/canario`) con la clave real. Hasta validarlo, Antigravity (experimental) se ejecuta sin restricción de herramientas comprobada (`agy -p` puede usar herramientas, shell o MCP en su carpeta de trabajo, la raíz del proyecto en `consciente_proyecto`) y recibe el prompt como argumento, visible en la lista de procesos. No se añaden opciones sin validarlas con el CLI real.
- [ ] Entrada directa de la Web UI en el navegador en macOS (identificar al dueño de la conexión con libproc); hoy se entra desde la app o con `tramtai abrir`.
- [ ] **Antigravity con la cuenta personal (evaluado 2026-10-12, no se implementa por ahora).** OmniRoute ofrece `agy` por OAuth: importa el archivo de token del CLI, detecta su sesión local o inicia sesión con Google, y usa el backend de Antigravity (incluidos los modelos Claude). Tramtai no lo hace por dos motivos: nunca lee ni copia credenciales de los CLIs, y la FAQ de Antigravity dice que usar software de terceros para acceder a Antigravity viola sus términos (hay suspensiones de cuentas con 403 documentadas en el foro de Google AI); Google recomienda una clave de Vertex o AI Studio, que es lo que Tramtai ya usa. Revisar si Google publica un modo permitido para herramientas de terceros; mientras tanto, como mucho, un modo `suscripcion` opt-in que solo ejecute el `agy` oficial con su propia sesión (sin tocar tokens), con aviso explícito de riesgo de suspensión.
- [ ] Memoria entre conversaciones (engram u otra vía MCP): evaluar una opción por proyecto (en `consciente_proyecto`, que ya carga la configuración del proyecto) y si tiene sentido una global; hoy Tramtai es solo puente, sin memoria compartida entre aplicaciones.
