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
- [ ] Añadir Go y `%USERPROFILE%\go\bin` al PATH del usuario.
- [ ] Probar con `-race` (necesita CGO y gcc) o en el CI de Linux.
- [ ] Registrar el avance de M1, M2, M3 y M0b en el documento maestro (v13).
- [ ] Probar las fachadas con los SDK oficiales de Python y Node (hoy probadas con curl y pruebas Go).
- [ ] Sustituir el sondeo de eventos (100 ms) por notificación en memoria si pesa con muchos clientes.
- [ ] Límite de concurrencia propio por punto de acceso (hoy usa el de su instancia).
- [ ] Modo consciente del proyecto (raíces permitidas, reglas del repositorio para proyectos confiables).
- [ ] Compilar y probar la aplicación de escritorio en macOS y Linux (necesita CGO y WebKit; hoy solo Windows).
- [ ] Revisar Wails en cada beta (fijado en v3.0.0-beta.28) y migrar a la versión estable cuando salga.
- [ ] Icono propio de Tramtai para la bandeja y el ejecutable (hoy el predeterminado de Wails).
- [ ] Unix: guardar pgid y firma de inicio de cada agente y terminar los grupos huérfanos al arrancar; hoy una caída del servicio los deja vivos (en Windows los cierra el Job Object).
- [ ] `retencion.guardar_historial` y `cifrado_aplicacion` se aceptan en la configuración, pero no tienen efecto: no hay historial permanente ni cifrado propio (sección 16.2).
- [ ] Perfiles de salida en configuración (`salida.esquema` por archivo, sección 22.3) y `conservar_salida_invalida`.
- [ ] Adaptadores Antigravity (`agy` con `modelProvider: gemini`) y Gemini API directa.
- [ ] Guardar las claves de API en el Administrador de credenciales de Windows en lugar de variables de entorno.
- [ ] Recurso de versión del `.exe` (Propiedades → Detalles en Windows) con la versión calculada.
