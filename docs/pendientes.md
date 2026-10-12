# Pendientes

## Hitos

| Hito | Estado | Contenido |
|---|---|---|
| M0 | ✅ | Validación de CLIs, entorno y términos de uso (doc v11–v12) |
| M0b | 🟡 | Negativa OK; falta la parte positiva con claves reales ([M0B.md](M0B.md)) |
| M1 | ✅ | Núcleo, configuración por capas, SQLite, idempotencia, planificador |
| M2 | ⬜ | Adaptadores reales, supervisión de procesos (Job Objects), servicio HTTP `/api/v1` |
| M3 | ⬜ | Salida estructurada y validación con JSON Schema, fachadas `/openai/v1` y `/anthropic/v1` |
| M4+ | ⬜ | Puntos de acceso, conversión de cargas, TUI, app de escritorio Wails + Quasar, instalador NSIS |
| M7 | ⬜ | Cierre ordenado con plazo para trabajos en curso |

## Tareas

- [ ] **CI/CD con GitHub Actions** (en espera). Ejecutará `go vet` y `go test` en cada push, y en cada etiqueta `v*` compilará con `scripts/compilar.ps1` y creará el release de GitHub con `tramtai.exe` y, más adelante, el instalador. **Bloqueo:** el almacenamiento de GitHub Actions de la cuenta está lleno y se espera el reinicio del ciclo de facturación (unos 20 días desde el 12-10-2026). Al retomarla:
  - Revisar en *Settings → Billing → Usage* qué ocupa el espacio; normalmente son artefactos y cachés de otros repositorios, que se liberan al borrarlos y no al cambiar de ciclo.
  - Usar `retention-days: 1–5` en `upload-artifact` y adjuntar los binarios al release en lugar de dejarlos como artefactos.
- [ ] Ejecutar M0b con claves reales y registrar el resultado en el documento maestro.
- [ ] Añadir Go y `%USERPROFILE%\go\bin` al PATH del usuario.
- [ ] Probar con `-race` (necesita CGO y gcc) o en el CI de Linux.
- [ ] Registrar el avance de M1 y M0b en el documento maestro (v13).
