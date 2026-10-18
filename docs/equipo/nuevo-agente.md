# Cómo añadir un agente

Cada agente vive en su módulo, `internal/agentes/<nombre>`, con su adaptador, su ficha y sus pruebas. El resto del código
(servicio, planificador, API, Web UI) pregunta al catálogo, `internal/agentes`, y no nombra agentes: lo comprueba
`TestSinNombresFuera`. Un agente es un release (v0.44 Copilot CLI, v0.45 OpenCode, v0.46 Hermes Agent).

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
  | `Streaming`, `SalidaEstructurada`, `ModeloEfectivoInformado` | Lo que garantiza, según la M0. |
  | `ValoresEsfuerzo`, `EsfuerzoPredeterminado` | El valor del CLI para cada nivel de `nucleo.NivelesEsfuerzo` (hasta su máximo) y, si se conoce, el que usa por defecto con cada modelo. |
  | `Autenticaciones`, `VariablesClave` | Los modos que admite y las variables de su clave, la preferida primero. Cada variable debe estar en `variablesCredenciales` (`adaptadores/comun.go`): se retira del entorno de los demás CLI. |
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
  - que el prompt no empiece por `-`.
- **Del catálogo:** `TestFichasCompletas`, `TestRegistroYValoresPredeterminados` y `TestSinNombresFuera` pasan sin
  tocarlos.
- **Canario:** el caso del agente en `pruebas/canario/canario_test.go` (`go test -tags canario`), con su modelo más
  pequeño.
- **Prueba real opcional:** `internal/agentes/real_test.go` (`go test -tags agentes`).

## 5. Textos y documentación

- Mensajes nuevos en Go con `idioma.T`/`Tf` y su traducción en `internal/idioma/en_*.go` (`TestCobertura`).
- Ayuda (`web/src/idiomas`), README, `site/ayuda.html`, `docs/arquitectura.md` (sus diferencias en la tabla de
  adaptadores) y `docs/equipo/pendientes.md`.
