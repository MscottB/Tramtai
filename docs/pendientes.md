# Pendientes

## Hitos

| Hito | Estado | Contenido |
|---|---|---|
| M0 | ✅ | Validación de CLIs, entorno y términos de uso (doc v11–v12) |
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

- [ ] **CI/CD con GitHub Actions** (en espera). Ejecutará `go vet` y `go test` en cada push, y en cada etiqueta `v*` compilará con `scripts/compilar.ps1` y creará el release de GitHub con `tramtai.exe` y, más adelante, el instalador. **Bloqueo:** el almacenamiento de GitHub Actions de la cuenta está lleno y se espera el reinicio del ciclo de facturación (unos 20 días desde el 12-10-2026). Al retomarla:
  - Revisar en *Settings → Billing → Usage* qué ocupa el espacio; normalmente son artefactos y cachés de otros repositorios, que se liberan al borrarlos y no al cambiar de ciclo.
  - Usar `retention-days: 1–5` en `upload-artifact` y adjuntar los binarios al release en lugar de dejarlos como artefactos.
- [ ] Ejecutar M0b con claves reales y registrar el resultado en el documento maestro.
- [ ] Registrar el avance de M1, M2, M3 y M0b en el documento maestro (v13).
- [ ] `instancia_global.raices_permitidas`: limitar las rutas locales que la CLI puede enviar como archivos (hoy cualquier ruta legible por el usuario).
- [ ] Compilar y probar la aplicación de escritorio en macOS y Linux (necesita CGO y WebKit; hoy solo Windows).
- [ ] Revisar Wails en cada beta (fijado en v3.0.0-beta.28) y migrar a la versión estable cuando salga.
- [ ] Historial permanente, cifrado propio y cache documental (`retencion.guardar_historial`, `cifrado_aplicacion`, `cache_conversion_habilitada`): hoy la configuración los rechaza con «no soportado todavía».
- [ ] Probar con una clave real los adaptadores `gemini` y `antigravity` (hoy con servidor y CLI simulados). En `agy`, comprobar si acepta el prompt por stdin (`--input-format stream-json`) para quitar el límite de 28 000 caracteres por línea de órdenes, y validar `permissions.deny` y `toolPermission: strict` en su `settings.json` (junto a M0b).
- [ ] Almacén de credenciales en macOS (Llavero) y Linux (Secret Service); hoy `tramtai claves` solo funciona en Windows.
