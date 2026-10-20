# Cómo añadir un agente

Cada agente vive en su módulo, `internal/agentes/<nombre>`, con su adaptador, su ficha y sus pruebas. El resto del código
(servicio, planificador, API, Web UI) pregunta al catálogo, `internal/agentes`, y no nombra agentes: lo comprueba
`TestSinNombresFuera`. Un agente es un release (v0.44 Copilot CLI, v0.47 OpenCode, v0.48 Hermes Agent).

## 1. M0 con el CLI real

Antes de escribir código, se prueba el CLI tal como lo instala el usuario y se anota en `docs/equipo/m0-agentes.md`
(ver las de Copilot CLI y OpenCode):

- **Modo no interactivo:** la orden, la salida (texto, JSON o JSONL) y los eventos, con un ejemplo de cada uno.
- **Datos de la respuesta:**
  - el uso (tokens de entrada, salida, caché y razonamiento; coste);
  - el modelo efectivo;
  - el coste fijo por llamada.
- **Modelos y esfuerzo:** cómo se eligen el modelo y el esfuerzo, y qué niveles admite.
- **Autenticación:**
  - suscripción o clave;
  - dónde guarda la sesión;
  - qué variables lee.
- **Carpeta privada:**
  - la variable que mueve su configuración, sesiones y credenciales (`COPILOT_HOME`, `XDG_*`, `HOME`…);
  - si sigue autenticado con ella.
- **Persistencia:** qué deja guardado (sesiones, historial, memoria) y cómo evitarlo.
- **Canario** (`pruebas/canario`): un mensaje hostil que pide leer un archivo, ejecutar una orden y abrir una URL local.
  Se anota con qué opciones falla y con cuáles pasa. Sin un canario que pase, la ficha va con `PermitidoEnApps: false`.
  Comprueba que la opción que quita las herramientas de verdad se aplica: con Copilot, `--available-tools=` (vacío) se
  ignoraba y el agente leyó el archivo trampa; `--available-tools none` sí deja la lista vacía.
- **Si el CLI tiene hooks** (como Antigravity), un hook `PreToolUse` que responda `deny` a todo es la forma más firme de
  dejarlo sin herramientas. Comprueba que niega de verdad (el error debe decir que lo negó el hook, no que el hook
  falló) y que su orden no lleva comillas: en Windows llegan mal escapadas a `cmd`.
- **Salvaguarda:** si el CLI informa de las herramientas que ejecuta, corta la ejecución cuando una se complete, como
  hacen Copilot y Antigravity.
- **En Windows:** si se instala con npm, en el PATH queda un envoltorio `.cmd`, que `ResolverPrograma` rechaza. Busca el
  ejecutable nativo del paquete de la plataforma, como hace `programa` en `internal/agentes/copilot`.

## 2. El módulo

- `internal/agentes/<nombre>/<nombre>.go`: el adaptador (`adaptadores.Adaptador`). Si se puede, también
  `adaptadores.Detectable` (estado sin gastar cuota), `adaptadores.Resolvedor` (modelo concreto de un alias) y
  `adaptadores.ConAutenticacion` (suscripción cambiable en marcha). Usa lo común de `internal/adaptadores`:
  - `Separar`, `EnmarcarSistema`, `EntornoLimpio`, `ClaveEntorno`, `ResolverPrograma`;
  - `EspacioTemporal`, `DirectorioTrabajo`, `Clasificar`, `RedactarCon`, `ValorEsfuerzo`.
- `internal/agentes/<nombre>/ficha.go`: su `adaptadores.Ficha`.

  | Campo | Qué es |
  |---|---|
  | `Nombre` | El id, el que va delante de `agente/modelo` (`claude`). |
  | `NombreVisible`, `Programa` | Cómo se muestra («Claude») y el CLI o servicio que hay detrás («Claude Code»). |
  | `Codigo`, `ColorClaro`, `ColorOscuro`, `Orden` | Las dos letras y el color de su línea en la Web UI, y su lugar en las listas. Únicos. |
  | `Streaming`, `SalidaEstructurada`, `ModeloEfectivoInformado` | Lo que garantiza, según la M0. Sin salida estructurada propia, el adaptador pide el formato en las instrucciones (ver `instruccionesSalida` en Copilot) y Tramtai valida igual. |
  | `ValoresEsfuerzo`, `EsfuerzoPredeterminado`, `ModelosSinEsfuerzo` | El valor del CLI para cada nivel de `nucleo.NivelesEsfuerzo` (hasta su máximo); si se conoce, el que usa por defecto con cada modelo; y los modelos con los que no se puede elegir (`""` es su predeterminado; en Copilot, el automático). |
  | `Autenticaciones` (si incluye `suscripcion`, el adaptador cumple `adaptadores.ConAutenticacion`), `VariablesClave` | Los modos que admite y las variables de su clave, la preferida primero. Cada variable debe estar en `variablesCredenciales` (`adaptadores/comun.go`): se retira del entorno de los demás CLI. La primera es la que Agentes muestra en `tramtai claves guardar` y debe estar en `credenciales.Nombres` (`internal/credenciales`). |
  | `Comando`, `Concurrencia` | Los valores predeterminados de `comandos_agentes.<nombre>` y `concurrencia_servicio.adaptadores.<nombre>`. |
  | `PermitidoEnApps` | `true` solo si su canario pasó. |
  | `URLInstalacion` | Dónde se instala, o dónde se consigue la clave si no hay CLI. |
  | `Crear` | Construye el adaptador con la configuración efectiva. |

## 3. El catálogo

Una línea en `fichas` de `internal/agentes/catalogo.go`. Con eso:

- el servicio lo registra y le aplica sus valores predeterminados;
- `GET /api/v1/agentes` lo publica;
- la Web UI lo muestra con su nombre y su color en Agentes, Probar, Mis APIs y el mapa;
- el planificador y Mis APIs deciden con su ficha si se permite en apps.

## 4. Pruebas

- **Del módulo:** con un CLI simulado que emite el JSONL capturado en la M0. El ejemplo es el binario de prueba que hace
  de `agy` en `antigravity_test.go`. Cubren:
  - texto en vivo, uso, modelo efectivo y errores clasificados;
  - el entorno sin claves ajenas;
  - la carpeta privada;
  - que el mensaje vaya por stdin y no en la línea de órdenes;
  - si el CLI tiene hooks, que el hook niegue de verdad, y que tras una herramienta no se emita nada más.
- **Del catálogo:** `TestFichasCompletas`, `TestRegistroYValoresPredeterminados` y `TestSinNombresFuera` pasan sin
  tocarlos.
- **Canario:** el caso del agente en `pruebas/canario/canario_test.go` (`go test -tags canario`), con su modelo más
  pequeño.
- **Prueba real opcional:** `internal/agentes/real_test.go` (`go test -tags agentes`).

## 5. Textos y documentación

- Mensajes nuevos en Go con `idioma.T`/`Tf` y su traducción en `internal/idioma/en_*.go` (`TestCobertura`).
- Ayuda (`web/src/idiomas`), README (también su sección en inglés), `site/index.html` (mapa, sección de agentes y
  marcas), el tema `agentes` de la Ayuda (`web/src/ayuda/temas/agentes.ts`, que genera también el sitio), `TERMINOS.md` si admite suscripción, `docs/arquitectura.md` (sus diferencias en la tabla
  de adaptadores) y `docs/equipo/pendientes.md`.
- Textos que nombren agentes en la Web UI: búscalos (`Claude y Codex`…) y déjalos genéricos o al día.
