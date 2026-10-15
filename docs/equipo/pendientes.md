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

## Tareas

- [ ] **Activar CI/CD y la página al hacer público el repositorio.** Los flujos ya están en `.github/workflows/`: `ci.yml` (vet y pruebas en Windows, Linux y macOS, y la Web UI), `publicar.yml` (en cada etiqueta `v*` compila, genera el instalador y crea el release con `SHA256SUMS.txt`) y `paginas.yml` (publica `site/` en GitHub Pages). En repositorios públicos los runners estándar no consumen minutos ni el almacenamiento de la cuenta. Falta: hacer público el repo, elegir *Settings → Pages → Source: GitHub Actions* y comprobar la primera ejecución de cada flujo (macOS nunca se ha probado en un equipo real y su resultado no bloquea).
- [ ] Firma de código del instalador y de los `.exe` (SmartScreen avisa de «aplicación desconocida» sin ella): Azure Trusted Signing o un certificado OV.
- [ ] Aviso de versión nueva en la app y `tramtai actualizar` (descarga el instalador del release, comprueba su SHA-256 y lo ejecuta en silencio).
- [ ] Ejecutar M0b con claves reales y registrar el resultado en [M0B.md](M0B.md).
- [ ] Compilar y probar la aplicación de escritorio en macOS y Linux (necesita CGO y WebKit; hoy solo Windows).
- [ ] Revisar Wails en cada beta (fijado en v3.0.0-beta.28) y migrar a la versión estable cuando salga.
- [ ] Historial permanente, cifrado propio y cache documental (`retencion.guardar_historial`, `cifrado_aplicacion`, `cache_conversion_habilitada`): hoy la configuración los rechaza con «no soportado todavía».
- [ ] Probar con una clave real los adaptadores `gemini` y `antigravity` (hoy con servidor y CLI simulados). En `agy`, comprobar si acepta el prompt por stdin (`--input-format stream-json`) para quitar el límite de 28 000 caracteres por línea de órdenes, y validar `permissions.deny` y `toolPermission: strict` en su `settings.json` (junto a M0b).
- [ ] Almacén de credenciales en macOS (Llavero) y Linux (Secret Service); hoy `tramtai claves` solo funciona en Windows.
- [ ] Entrada sin clave de la Web UI en macOS (identificar al dueño de la conexión con libproc); hoy pide la clave o `tramtai abrir`.
- [ ] **Antigravity con la cuenta personal (evaluado 2026-10-12, no se implementa por ahora).** OmniRoute ofrece `agy` por OAuth: importa el archivo de token del CLI, detecta su sesión local o inicia sesión con Google, y usa el backend de Antigravity (incluidos los modelos Claude). Tramtai no lo hace por dos motivos: nunca lee ni copia credenciales de los CLIs, y la FAQ de Antigravity dice que usar software de terceros para acceder a Antigravity viola sus términos (hay suspensiones de cuentas con 403 documentadas en el foro de Google AI); Google recomienda una clave de Vertex o AI Studio, que es lo que Tramtai ya usa. Revisar si Google publica un modo permitido para herramientas de terceros; mientras tanto, como mucho, un modo `suscripcion` opt-in que solo ejecute el `agy` oficial con su propia sesión (sin tocar tokens), con aviso explícito de riesgo de suspensión.
- [ ] Memoria entre conversaciones (engram u otra vía MCP): evaluar una opción por proyecto (en `consciente_proyecto`, que ya carga la configuración del proyecto) y si tiene sentido una global; hoy Tramtai es solo puente, sin memoria compartida entre aplicaciones.
- [ ] Arranque con el sistema en Linux (unidad de usuario de systemd o `~/.config/autostart`) y macOS (LaunchAgent); hoy solo Windows.
