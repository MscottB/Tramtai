# GitHub Copilot CLI como agente nuevo, y la web del producto al día (v0.44)

Diseño del 2026-10-18. Es el primer agente que se añade con el catálogo por módulos de la v0.43. Sigue
`docs/equipo/nuevo-agente.md` y la evidencia de `docs/equipo/m0-agentes.md`.

Decisiones del usuario:
- Un agente por release: este es Copilot CLI.
- El orden por agente es el del código: cada agente en su módulo, como dejó la v0.43.
- El release actualiza todo lo que toca al agente: código, Ayuda, documentación, notas de la versión y la web de
  presentación, que además se mejora.
- El usuario tiene Copilot Free: solo modelo automático y cada petición gasta créditos del plan.

Fuera de alcance:
- OpenCode (v0.45) y Hermes Agent (v0.46).
- Herramientas de Copilot (MCP, shell, edición): Tramtai lo usa siempre sin herramientas.
- Adjuntos nativos (`--attachment`): Tramtai envía el texto convertido, como con los demás.

## 1. El módulo `internal/agentes/copilot`

**Ficha:**

| Campo | Valor |
|---|---|
| Nombre, nombre visible, programa | `copilot`, «Copilot», «GitHub Copilot CLI» |
| Código y colores | `GH`; claro `#1f883d`, oscuro `#3fb950` (verde de GitHub, distinto de los demás) |
| Orden | 25 (después de Codex) |
| Streaming | `tokens` (`assistant.message_delta`) |
| Salida estructurada | no nativa: el esquema va en las instrucciones y Tramtai valida la respuesta, como con cualquier agente |
| Modelo efectivo informado | sí: `session.auto_mode_resolved` (con `auto`) o el `model` de `assistant.message` |
| Esfuerzo | bajo → `low`, medio → `medium`, alto → `high`, muy alto → `xhigh`, máximo → `max` |
| Autenticación | `suscripcion` (la sesión de `copilot login`) y `clave_api` (`COPILOT_GITHUB_TOKEN`, un token fine-grained con el permiso «Copilot Requests») |
| Comando y concurrencia | `copilot`, 1 |
| Permitido en apps | sí: su canario pasa con las opciones de abajo |
| Instalación | `https://docs.github.com/copilot/how-tos/set-up/install-copilot-cli` |

Con `auto`, el esfuerzo no se envía: Copilot lo rechaza («Model "auto" does not support reasoning effort»). La ficha
lo dice con `ModelosSinEsfuerzo` (`""` y `auto`): el catálogo de modelos no ofrece niveles y el intento se registra sin
esfuerzo.

**Cómo se lanza:**
`copilot --output-format json --model <modelo|auto> [--reasoning-effort <nivel>] --available-tools none
--disallow-temp-dir --no-custom-instructions --disable-builtin-mcps --no-ask-user --no-auto-update
--usage-output-file <privado>/uso.json`, con el espacio temporal como directorio de trabajo.
- **El mensaje:** va por la entrada estándar, comprobado con tildes y eñes. No aparece en la lista de procesos y no
  tiene el límite de la línea de órdenes.
- **Las instrucciones:** como primer bloque (`EnmarcarSistema`), igual que en los CLI sin mecanismo propio.

**Carpeta privada:** `COPILOT_HOME` temporal por ejecución, borrada al terminar, con las sesiones, los registros y
`session-store.db`.
- Con suscripción, el `config.json` privado solo copia `loggedInUsers` y `lastLoggedInUser` de `~/.copilot/config.json`
  (sin secretos: el token está en el almacén del sistema). Así la sesión del usuario vale sin cargar su configuración,
  sus plugins ni sus MCP.
- Con clave: `COPILOT_GITHUB_TOKEN` en el entorno y nada más.
- `GH_TOKEN` y `GITHUB_TOKEN` no le llegan: se retiran como cualquier credencial. Un `ghp_` clásico, además, lo rechaza.

**Lectura de la salida (JSONL):**
- `assistant.message_delta` → texto en vivo;
- `assistant.message` → texto final y modelo;
- `session.auto_mode_resolved` → modelo elegido por `auto`;
- `result.exitCode`;
- uso: tokens de entrada, salida y caché sumados por modelo desde `--usage-output-file`. En el aviso del registro, los
  premium requests como «créditos de Copilot». No hay coste en USD.

**Errores:**
- Salida distinta de 0 → stderr clasificado con `Clasificar` (sin sesión → `autenticacion_agente`; modelo no disponible
  → `modelo_no_disponible`; límite → `limite_uso_proveedor`).
- Un modelo inexistente falla sin gastar créditos.

**Detección (`Detectar`):**
- versión con `copilot --version`;
- sin `copilot` en el PATH → `no_instalado`;
- en suscripción, sin `loggedInUsers` → `falta_clave` («inicia sesión con copilot login»);
- en clave, sin `COPILOT_GITHUB_TOKEN` (entorno o almacén) → `falta_clave`;
- términos: «Términos de GitHub Copilot: tu cuenta y tu plan; cada petición gasta créditos».

**Modelos:** el catálogo ofrece `copilot` (el automático). En un plan de pago se escribe `copilot/<modelo>` en el
selector o en `model`. Copilot no tiene orden para listarlos, así que no se inventa una lista.

**Credenciales:** `COPILOT_GITHUB_TOKEN` se añade a `variablesCredenciales` y a `credenciales.Nombres`, para guardarlo
con `tramtai claves guardar COPILOT_GITHUB_TOKEN`.

## 2. Ayuda, documentación y release

- Ayuda:
  - «¿Qué agentes puedo usar?» con Copilot;
  - una pregunta «¿Puedo usar Copilot con el plan gratuito?» (sí, solo con el modelo automático; cada petición gasta
    créditos).
- `docs/arquitectura.md`: Copilot en la tabla de adaptadores (stdin, JSONL, `COPILOT_HOME` privado, sin herramientas).
- `docs/api.md` y `docs/inicio.md`: el agente en el campo `model` (`copilot`, `copilot/<modelo>`).
- README (es/en) y la ayuda del sitio (cómo conectarlo).
- `pendientes.md`: la fila de v0.44.
- Las notas del release (`-Resumen`) explican el agente y la web nueva.

## 3. Las novedades de cada versión, también en inglés

Pedido por el usuario durante la spec: las novedades deben tener inglés, como ya lo tienen la app, la web y el README.
Hoy las notas de cada versión (las que escribe `scripts/version.ps1` y la app muestra con `NotasVersion.vue` al
avisar y tras actualizar) solo están en español.

**Las notas:**
- `scripts/version.ps1` admite `-ResumenEn`, obligatorio con `-Etiquetar` si hay `-Resumen`.
- Las notas terminan con un bloque inglés tras la línea `## English`, con:
  - el título;
  - el resumen en inglés;
  - «Full list of changes (in Spanish) above».

  En la página del release de GitHub se ven las dos partes. La lista de cambios sale de los asuntos de los commits,
  escritos en español, así que no se traduce.

**La app:**
- `NotasVersion.vue` parte el texto por `## English` y muestra la parte del idioma de la interfaz.
- En inglés, debajo del resumen, un enlace «See every change on GitHub» a la página de la versión.
- Las versiones anteriores, sin bloque inglés, se ven en español como hoy.
- El recorte de las notas en el servicio sube de 6000 a 12000 caracteres para que quepan las dos partes.

**El resto, que ya es bilingüe, se mantiene así:**
- Todo texto nuevo de la app, la web y la ayuda del sitio va en es/en. La prueba de cobertura de traducciones lo
  comprueba en la app.
- La sección «English» del README se pone al día con:
  - los agentes, Copilot incluido;
  - la API propia y OpenAPI;
  - la Ayuda integrada.

## 4. La web de presentación (`site/index.html`)

- **Portada:** Copilot como quinta línea del mapa, con su color, y en el texto de entrada.
- **Sección nueva «Agentes»**, entre «Cómo funciona» y las capturas. Una tarjeta por agente con su chapa y color:
  - cómo se conecta (suscripción o clave);
  - si tiene esfuerzo y respuesta en vivo;
  - Antigravity marcado como experimental, solo desde Probar y la terminal;
  - Copilot con la nota del plan gratuito.

  Debajo, una línea: «el catálogo crece por módulos; cada agente se comprueba con su CLI real antes de usarse desde
  apps».
- **Funciones:** se añaden tres tarjetas:
  - «API propia con OpenAPI» (seguir, cancelar, reintentar, Ver como código);
  - «Agentes sin herramientas» (canarios: un mensaje hostil no les hace leer archivos ni ejecutar órdenes);
  - «Ayuda integrada» (guías paso a paso, errores y buscador).

  La de modelo y esfuerzo pasa a nombrar a los agentes que lo admiten.
- **Capturas:** a Inicio y Mis APIs se suman Agentes (filas con filtro) y Probar (con Ver como código), en claro y
  oscuro. Se capturan con un perfil limpio y nombres neutros.
- `meta description` y textos en inglés al día.
- En el móvil, la sección de agentes se ve en una columna sin desplazamiento horizontal.

## Pruebas

- **Módulo, con un CLI simulado** que emite el JSONL capturado en la M0:
  - texto en vivo, modelo efectivo con `auto`, uso desde el archivo;
  - error de sesión y de modelo;
  - mensaje por stdin;
  - `COPILOT_HOME` privado con solo `loggedInUsers`;
  - el entorno sin `GH_TOKEN` ni claves ajenas;
  - sin `--reasoning-effort` con `auto`.
- **Catálogo:** `TestFichasCompletas`, `TestSinNombresFuera` y `TestRegistroYValoresPredeterminados` sin tocarlos.
- **Canario:** `copilot/auto` en `pruebas/canario` (gasta un crédito del plan gratuito).
- **Prueba real** (`-tags agentes`): una respuesta corta con `auto`.
- **Notas:** `NotasVersion` con notas de dos idiomas, solo español y sin `## English` al final; `version.ps1 -Explicar` con `-ResumenEn`.
- **Navegador:** Copilot en Agentes, Probar (selector y respuesta en vivo) y Mis APIs; la web en móvil y escritorio,
  claro y oscuro.
