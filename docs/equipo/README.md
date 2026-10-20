# Documentación del equipo

Lo interno para desarrollar Tramtai. La documentación para usuarios está en el [README](../../README.md) y en
[docs/](../inicio.md).

## Guías

| Documento | Contenido |
|---|---|
| [desarrollo.md](desarrollo.md) | Compilar, la Web UI, las pruebas y cómo se trabaja |
| [versionado.md](versionado.md) | Versión `vX.Y.Z+DDMMAAAA`, el flujo con PR y cómo se publica un release |
| [nuevo-agente.md](nuevo-agente.md) | Cómo añadir un agente |
| [m0-agentes.md](m0-agentes.md) | Evidencia M0 de los CLI de cada agente |
| [M0B.md](M0B.md) | Validación de los modos con clave de API |
| [conversor.md](conversor.md) | El conversor de archivos: lock, uv fijado y cómo cambiar sus versiones |
| [pendientes.md](pendientes.md) | Hitos, hoja de ruta por releases y tareas |
| [Diseño](../diseno/DESIGN.md) | Sistema visual de la Web UI y de la página |

## Releases

Cada release tiene su spec (el diseño aprobado) y su plan (las tareas), en `releases/<versión>-<tema>/`.

| Versión | Tema | Spec | Plan |
|---|---|---|---|
| v0.34 | Recetas y respaldo entre agentes | [spec](releases/v0.34-recetas-respaldo/spec.md) | [plan](releases/v0.34-recetas-respaldo/plan.md) |
| v0.35 | Presupuestos por API y por instancia | [spec](releases/v0.35-presupuestos/spec.md) | [plan](releases/v0.35-presupuestos/plan.md) |
| v0.36 | Rediseño de la presentación | [spec](releases/v0.36-rediseno/spec.md) | [plan](releases/v0.36-rediseno/plan.md) |
| v0.37 | Actualizaciones: limpieza y novedades | [spec](releases/v0.37-actualizaciones/spec.md) | [plan](releases/v0.37-actualizaciones/plan.md) |
| v0.38 | Grupos, origen y límites del historial | [spec](releases/v0.38-grupos-historial/spec.md) | [plan](releases/v0.38-grupos-historial/plan.md) |
| v0.39 | Pulido | [spec](releases/v0.39-pulido/spec.md) | [plan](releases/v0.39-pulido/plan.md) |
| v0.40 | Probar | [spec](releases/v0.40-probar/spec.md) | [plan](releases/v0.40-probar/plan.md) |
| v0.41 | Seguridad y límites | [spec](releases/v0.41-seguridad/spec.md) | [plan](releases/v0.41-seguridad/plan.md) |
| v0.42 | API propia `/api/v1` explicada | [spec](releases/v0.42-api-propia/spec.md) | [plan](releases/v0.42-api-propia/plan.md) |
| v0.43 | Agentes por módulos | [spec](releases/v0.43-agentes-modulos/spec.md) | [plan](releases/v0.43-agentes-modulos/plan.md) |
| v0.44 | GitHub Copilot CLI | [spec](releases/v0.44-copilot/spec.md) | [plan](releases/v0.44-copilot/plan.md) |
| v0.46 | Respuestas JSON mejoradas | [spec](releases/v0.46-json/spec.md) | [plan](releases/v0.46-json/plan.md) |
| v0.47 | Correcciones de la revisión completa | [spec](releases/v0.47-correcciones/spec.md) | [plan](releases/v0.47-correcciones/plan.md) |
| v0.48 | Portable de verdad | [spec](releases/v0.48-portable/spec.md) | [plan](releases/v0.48-portable/plan.md) |
| v0.49 | Orden del código | [spec](releases/v0.49-orden/spec.md) | [plan](releases/v0.49-orden/plan.md) |
| v0.50 | Archivos | [spec](releases/v0.50-archivos/spec.md) | [plan](releases/v0.50-archivos/plan.md) |
| v0.51 | Herramientas y guía del desarrollador | [spec](releases/v0.51-herramientas/spec.md) | [plan](releases/v0.51-herramientas/plan.md) |

Los releases sin carpeta (v0.45 y los parches) se diseñaron dentro de otro o no tuvieron spec propia; su contenido
está en la hoja de ruta de [pendientes.md](pendientes.md).

Un release nuevo crea su carpeta con `spec.md` y `plan.md`, y añade su fila aquí.
