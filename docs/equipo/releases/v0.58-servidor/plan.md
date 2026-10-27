# v0.58: modo servidor — plan

> **Para agentes:** SUB-SKILL OBLIGATORIA: superpowers:subagent-driven-development (recomendada) o superpowers:executing-plans, tarea a tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** que Tramtai funcione como servidor en Linux y Windows Server. Eso incluye:
- un servicio del sistema aparte del Tramtai de escritorio;
- claves sin escritorio;
- HTTPS solo con TLS 1.3, detrás de un proxy, con certificado propio o con Let's Encrypt;
- un administrador para la Web UI remota;
- la suscripción personal con condiciones;
- los paquetes del servidor;
- los términos versión 2, aceptados en toda la aplicación;
- «Solo el servicio» y la tecla `W`.

**Arquitectura:**
- **Lo común:** una tercera raíz de datos (`internal/carpetas`) hace que el mismo servicio viva en una carpeta del sistema.
- **Lo propio de cada sistema:** instalar, desinstalar, la unidad systemd y el servicio de Windows van en un paquete nuevo, `internal/servidor`.
- **Las reglas de exposición:** una función pura, `servidor.ComprobarArranque`, decide si el servicio puede arrancar.
- **TLS, proxy y administrador:** son piezas de `internal/servicio` que se activan por configuración.

**Tecnología:** Go 1.27 (CLI cobra, servicio, terminal Bubble Tea), `golang.org/x/sys/windows/svc`, `golang.org/x/crypto` (argon2 y acme/autocert, la única dependencia nueva), Vue/Quasar (Web UI), nfpm (paquetes) y GitHub Actions.

**Spec:** `docs/equipo/releases/v0.58-servidor/spec.md`. Se lee junto con este plan; las secciones se citan como «spec §N».

## Restricciones globales

- **Go:** `go 1.27`, `toolchain go1.27.3`. Dependencia nueva solo `golang.org/x/crypto`.
- **TLS propio:** `MinVersion: tls.VersionTLS13`; HSTS `max-age=31536000`; certificado ECDSA P-256 con Let's Encrypt.
- **Rutas:**
  - Linux: datos en `/var/lib/tramtai`, programa en `/opt/tramtai/`, envoltorio `/usr/local/bin/tramtai-servidor` (`/usr/bin/tramtai-servidor` con el paquete), unidad `/etc/systemd/system/tramtai.service` (`/usr/lib/systemd/system/` con el paquete).
  - Windows: datos en `%ProgramData%\Tramtai`, programa en `C:\Program Files\Tramtai Servidor\`, servicio `Tramtai` («Tramtai (servidor)»), cuenta `NT SERVICE\Tramtai`.
- **Claves de configuración nuevas** (todas bajo prefijos que ya son de seguridad, `servicio` y `adaptadores`):

  | Clave | Tipo | Por defecto |
  |---|---|---|
  | `servicio.dominios` | `tipoLista` | `[]` |
  | `servicio.proxy.confiables` | `tipoLista` | `[]` |
  | `servicio.tls.modo` | texto: `ninguno`, `archivos` o `letsencrypt` | `ninguno` |
  | `servicio.tls.certificado` | texto | `""` |
  | `servicio.tls.clave` | texto | `""` |
  | `servicio.tls.letsencrypt.correo` | texto | `""` |
  | `adaptadores.<agente>.suscripcion_personal` | booleano | `false` |
  | `adaptadores.<agente>.suscripcion_simultaneas` | entero, de 1 a 3 | `1` |
- **Archivos nuevos en `datos/`** (0600, escritos con `configuracion.EscribirAtomico`):
  - `terminos.json`: `{"version":"2.0","fecha":"2026-11-02T14:05:00Z","como":"interfaz"}`;
  - `administrador`: una línea PHC, `$argon2id$v=19$m=65536,t=3,p=1$<sal b64>$<hash b64>`;
  - `administrador-recuperacion`: una huella `sha256:<hex>` por línea.
- **argon2id:** m=64 MiB, t=3, p=1, sal de 16 bytes, clave de 32. Contraseña de 12 caracteres como mínimo.
- **TOTP:** RFC 6238, SHA-1, 6 dígitos, pasos de 30 s, ±1 paso, sin reutilizar un paso ya usado. El secreto es base32 de 20 bytes y se guarda en el almacén con el nombre `TRAMTAI_TOTP`.
- **Frenos:**
  - 5 fallos por dirección dan 15 minutos de espera;
  - 100 fallos en 10 minutos en total dan 5 minutos de espera para todos;
  - como mucho 2 cálculos de argon2id a la vez;
  - las claves falsas en las APIs (401): 30 por dirección en un minuto dan un minuto de espera (429).
- **Comandos nuevos con su alias en inglés,** exactamente los de spec «Lo decidido». Se registran con `Aliases` de cobra.
- **Textos:**
  - todo en español y en inglés;
  - cada `idioma.T` nuevo lleva su inglés en `internal/idioma/en_*.go` (`TestCobertura`);
  - cada clave nueva de `web/src/idiomas/es.ts` lleva la suya en `en.ts`;
  - los textos de la terminal salen de ahí con `npm run ayuda-terminal`.
- **Los cinco riesgos** (spec §10) son un solo texto: `terminos.Riesgos(servidor bool)` en Go y `riesgos.*` en `es.ts`/`en.ts`. No se copian a mano a otro sitio.
- **Commits y PR:**
  - un PR por tarea desde la rama `v0.58/servidor`, con Auto-fix;
  - nunca push directo a main;
  - cada mensaje acaba con `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`;
  - nunca se sube `anti-slop/`;
  - datos de ejemplo neutros.
- **Verificación de cada tarea en Go:** `go vet ./... && TRAMTAI_IDIOMA=es go test ./...`, y `go vet -tags conversor ./...`.
  - Si toca `escritorio/`: `cd escritorio && go vet ./... && go test ./...`.
  - Si toca `web/`: `cd web && npm test && npm run build`, y después `go generate ./internal/contrato` si cambia el contrato.

## Revisión: lo que más puede fallar

1. **Quien actualiza desde la v0.57 con scripts que usan la CLI:** sin aceptar los términos, las órdenes siguen funcionando, con el aviso solo en stderr, y el servicio sigue en marcha. Prueba en la Tarea 2: `TestCLISinAceptarSigueFuncionando`.
2. **Escritorio y servidor en el mismo equipo:** la CLI sin `--servidor` nunca habla con el servicio del sistema ni lo detiene, aunque sea de otra versión y ocupe el 17373. Prueba en la Tarea 5: `TestSinServidorNoTocaElDelSistema`.
3. **Un proxy en el mismo equipo** que reenvía a loopback: esas peticiones no son locales y la suscripción no personal no se permite. Prueba en la Tarea 9: `TestProxyLoopbackNoEsLocal`.
4. **Cambiar una clave en Linux reinicia el servicio:** una petición en curso termina bien. Lo exige el arreglo de SIGTERM de la Tarea 6, y la prueba va en el job «Servidor» de la Tarea 17.
5. **Fuerza bruta con `X-Forwarded-For` falsificado** desde fuera del proxy: el freno cuenta la dirección real y no se evade cambiando la cabecera. Prueba en la Tarea 12: `TestFrenoNoSeEvadeConXFF`.

---

### Tarea 1: M0 del servidor

Sin código de producto. Decide qué entra según la tabla «M0 antes de implementar» (spec).

**Archivos:**
- Crear: `pruebas/m0servidor/sonda.go` (etiqueta `m0servidor`). Es un ejecutable que, como servicio de Windows o unidad systemd, escribe en un JSON:
  - su `HOME`/`USERPROFILE` y `os.UserConfigDir()`;
  - el resultado de `credenciales.Guardar("ANTHROPIC_API_KEY", "prueba-m0")` y de `Leer`;
  - en Linux, `systemd-creds --version`.
- Cambiar: `docs/equipo/m0-agentes.md` (los CLI) y `docs/equipo/releases/v0.58-servidor/spec.md` (el anexo A, las pruebas del sistema).

- [ ] **Paso 1:** en este Windows (administrador), registrar la sonda con `sc.exe create TramtaiM0 obj= "NT SERVICE\TramtaiM0"`, arrancarla, leer su JSON y borrarla. Anotar en el anexo A la carpeta de perfil y si el Administrador de credenciales funciona sin sesión.
- [ ] **Paso 2:** `systemd-creds --version` en contenedores `debian:12`, `debian:13`, `ubuntu:24.04` y `rockylinux:9`, desde un workflow manual de un solo uso o con Docker local. Anotar la versión.
- [ ] **Paso 3 (con el usuario, con sus cuentas):**
  - en Linux, `sudo -u tramtaim0 -H claude auth login` y `codex login --device-auth` (el usuario activa antes el código de dispositivo en ChatGPT), y el inicio de sesión de Copilot;
  - en Windows, buscar si la cuenta virtual puede iniciar sesión en cada CLI.
  - Anotar en `m0-agentes.md` el resultado y dónde guarda cada CLI su sesión.
- [ ] **Paso 4:** citas de los términos vigentes de Anthropic, OpenAI y GitHub sobre el uso remoto personal, con enlace y fecha, en `m0-agentes.md`.
- [ ] **Paso 5:** con la tabla de la spec, anotar en el anexo A qué queda fuera. Si algo queda fuera, `agentes.Ficha` recibe `ServidorSuscripcion bool`, que usa la Tarea 13 para no ofrecer la suscripción en el servidor a ese agente.
- [ ] **Paso 6:** borrar las cuentas y servicios de prueba. Commit `docs: M0 del modo servidor`.

### Tarea 2: términos versión 2 en toda la aplicación

**Archivos:**
- Crear: `internal/terminos/terminos.go` y `terminos_test.go`; `cmd/tramtai/terminos.go`; `web/src/componentes/AntesDeSeguir.vue`; `internal/tui/terminos.go`.
- Cambiar:
  - `TERMINOS.md` (el texto del anexo B de la spec, más la traducción fiel bajo `## Terms of use`, con «The Spanish version prevails»);
  - `cmd/tramtai/main.go:31-51` (registro y `PersistentPreRun` del raíz);
  - `internal/servicio/web.go:230-261` (rutas) e `internal/contrato/contrato.go`;
  - `web/src/App.vue`, `internal/tui/app.go`;
  - `scripts/instalador/tramtai.nsi:63-66` (`!define MUI_LICENSEPAGE_CHECKBOX` antes de la página de términos y, al terminar, `tramtai terminos aceptar --como instalador`);
  - `web/src/idiomas/{es,en}.ts` e `internal/idioma/en_cli.go`.

**Interfaces que deja:**
- `terminos.Version = "2.0"`, y `terminos.Mayor(v string) int` (`"2.0"` → 2; vacío o inválido → 0).
- `type Aceptacion struct { Version string `json:"version"`; Fecha time.Time `json:"fecha"`; Como string `json:"como"` }`.
- Las constantes de `Como`: `ComoInstalador = "instalador"`, `ComoServidorInstalar = "servidor-instalar"`, `ComoOpcion = "opcion"`, `ComoInterfaz = "interfaz"` y `ComoCLI = "cli"`.
- `terminos.Leer(datos string) (Aceptacion, error)`: sin archivo, devuelve `Aceptacion{}` y nil.
- `terminos.Aceptar(datos, como string) error`.
- `terminos.Pendiente(datos string) (bool, error)`: verdadero si falta la aceptación o si `Mayor(a.Version) < Mayor(Version)`.
- `terminos.Riesgos(servidor bool) []string`: los cinco riesgos de spec §10. Con `servidor=false`, los riesgos 3 y 4 empiezan por «Si usas el modo servidor, ».
- `contrato.EstadoTerminos{Version string json:"version"; Pendiente bool json:"pendiente"; Riesgos []string json:"riesgos"}`.
- `GET /ui/terminos` (`conSesion`) devuelve `EstadoTerminos`; `POST /ui/terminos` (`conSesion` y CSRF) llama a `terminos.Aceptar(datos, ComoInterfaz)` y responde 204.
- `tramtai terminos` (`terms`) imprime `TERMINOS.md`. Como `go:embed` no sube de carpeta, se incrusta una copia en `internal/terminos/TERMINOS.md`. La prueba `TestCopiaTerminosAlDia` falla si difiere de la de la raíz.
- `tramtai terminos aceptar` (`accept`) [`--como`, oculta, por defecto `cli`].

- [ ] **Paso 1: las pruebas que fallan.**
  - `TestPendiente`: sin archivo, `true`; tras `Aceptar`, `false`; con `{"version":"1.0"}`, `true`; con `{"version":"2.1"}` y `Version="2.0"`, `false`.
  - `TestAceptarEscribe`: el JSON tiene los tres campos, la fecha en UTC y permisos 0600 (en Unix).
  - `TestRiesgosCinco`: devuelve 5 y, con `servidor=false`, el 3 y el 4 empiezan por «Si usas el modo servidor».
  - `TestTerminosUI` (en `internal/servicio`): GET sin aceptar da `pendiente:true`; POST con CSRF da 204; después, `pendiente:false`. Sin sesión, 401.
  - `TestCLISinAceptarSigueFuncionando` (Revisión 1, `cmd/tramtai`): sin `terminos.json`, `tramtai version` termina con código 0, stdout sin cambios y stderr con una sola línea que contiene «tramtai terminos».
  - `TestTerminosAceptarCLI`: escribe `como:"cli"`; con `--como instalador`, `como:"instalador"`.
  - Web (`AntesDeSeguir.test.ts`): con `pendiente:true` se ve el diálogo con los 5 riesgos, «Acepto» llama al POST y «Salir» deja la app en la pantalla «Has salido» sin cargar las páginas.
  - TUI: `TestTerminalPideTerminos`: con estado pendiente, la primera vista es la de los términos; `enter` acepta, `q` sale.
- [ ] **Paso 2:** `go test ./internal/terminos/ ./internal/servicio/ ./cmd/tramtai/ -run 'Terminos|Pendiente|Riesgos|CLISinAceptar'`; `cd web && npm test`. Esperado: no compila o FAIL.
- [ ] **Paso 3:** implementar lo de «Interfaces que deja».
  - El aviso de la CLI va en el `PersistentPreRun` del raíz y se salta en `terminos`, `version` y `completion`.
  - La Web UI consulta `GET /ui/terminos` al arrancar, en `App.vue`, antes de montar las páginas.
  - La terminal, antes del menú.
  - `TERMINOS.md` se sustituye por el anexo B y su traducción, y se regenera `bin/TERMINOS.rtf` con `go run ./scripts/rtf`.
- [ ] **Paso 4:** la verificación global; `cd web && npm test && npm run build`; `go generate ./internal/contrato`. Esperado: PASS.
- [ ] **Paso 5:** commit `feat: términos versión 2 aceptados en toda la aplicación`.

### Tarea 3: «Solo el servicio»

La Tarea 13 de `releases/v0.56-terminal/plan.md` (líneas 499-541), tal cual: archivos, interfaces (`autoarranque.Modo`, `ModoApp`, `ModoServicio`, `Elegir`, `Configurado`) y pasos. Hay tres precisiones:
- En `escritorio/main.go:64-68`, la opción `-solo-servicio` se lee con `flag` junto a `-oculto` y se atiende antes de crear la app de Wails.
- En las pruebas, las variables que se sustituyen son las de hoy: `HOME`/`XDG_CONFIG_HOME` (`autoarranque_archivo_test.go:13-18`) y `conCopia` (`autoarranque_windows_test.go:15`).
- La Web UI muestra debajo de la elección: «¿Sin iniciar sesión? Eso es el servicio del sistema, otro entorno», con `tramtai-servidor instalar` y el enlace a la guía.

- [ ] **Pasos 1-7:** los de esa tarea. Commit `feat: arranque «Solo el servicio», sin ventana ni bandeja`.

### Tarea 4: la tecla `W` en la terminal

**Archivos:** cambiar `internal/tui/app.go:360-385`, `internal/tui/cabecera.go:221` (`comunes()`), `internal/tui/datos/textos.json` (vía `es.ts`/`en.ts` y `npm run ayuda-terminal`) y el tema de teclas de la Ayuda en `web/src/ayuda/temas/`. Pruebas en `internal/tui/app_test.go`.

**Interfaces:**
- Usa `(c *cliente.Cliente) URLWeb(ctx, instancia string) (string, error)` (`cliente.go:593`) y `navegador.Abrir`.
- Deja `var hayNavegador = func() bool`:
  - en Linux, falso si `SSH_CONNECTION` está definida o si faltan `DISPLAY` y `WAYLAND_DISPLAY`;
  - en Windows y macOS, falso solo con `SSH_CONNECTION`.

- [ ] **Paso 1:** las pruebas:
  - `TestTeclaWAbreNavegador`: con `hayNavegador=true` y `abrir` sustituido, `W` llama a `abrir` con la URL de `URLWeb`.
  - `TestTeclaWSinNavegador`: muestra una ventana con la URL y el código, y no llama a `abrir`.
  - `TestTeclaWEnCampo`: con `EnCampo()==true`, `W` no hace nada.
  - `TestPieAnunciaW`.
- [ ] **Paso 2:** `go test ./internal/tui/ -run TeclaW`. Esperado: FAIL.
- [ ] **Paso 3:** implementar. La Tarea 14 añade el caso del servidor con administrador, en el que basta la URL.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(terminal): la tecla W abre la Web UI`.

### Tarea 5: la raíz del servidor y `--servidor`

**Archivos:**
- Cambiar: `internal/carpetas/raiz.go`, `internal/cliente/cliente.go:64-150`, `cmd/tramtai/main.go` (opción persistente), `cmd/tramtai/servicio.go:385-394`, `internal/servicio/servicio.go:35-79` (`Opciones.Entorno`), `internal/servicio/acerca.go` e `internal/contrato/contrato.go`.
- Pruebas: `internal/carpetas/raiz_test.go`, `internal/cliente/cliente_test.go`, `cmd/tramtai/servicio_test.go`.

**Interfaces que deja:**
- `carpetas.UsarServidor()`: hace que la raíz sea la del servidor. Se llama antes del primer `RaizUsuario`; si la raíz ya se había calculado con otro modo, entra en pánico con un mensaje claro (es un error de programación).
- `carpetas.Servidor() bool`: verdadero tras `UsarServidor()` o con `TRAMTAI_SERVIDOR=1`.
- `carpetas.RaizServidor() string`: `/var/lib/tramtai` o `filepath.Join(os.Getenv("ProgramData"), "Tramtai")`. En modo servidor, `raizDe` devuelve esta ruta sin crearla ni protegerla, porque eso lo hace la instalación.
- `carpetas.Entorno() string`: `"servidor"`, `"portable"` o `"escritorio"`.
- La opción global `--servidor`/`--server` del raíz (`PersistentFlags`), atendida en un `PersistentPreRunE` que llama a `carpetas.UsarServidor()` antes de nada.
- `cliente.ErrServidorOtraVersion`: en modo servidor, `Conectar` y `DetenerAnterior` nunca detienen el servicio. Si responde uno de otra versión, devuelven este error con el mensaje «el servicio del sistema es de otra versión (X): actualízalo con tramtai-servidor actualizar».
- En modo servidor, la comprobación de dueño (`duenoPropio`) acepta un servicio de otro usuario.
- `contrato.Acerca` gana `Entorno string json:"entorno"` y `Cuenta string json:"cuenta,omitempty"`. `servicio.Opciones` gana `Entorno string`.

- [ ] **Paso 1:** las pruebas:
  - `TestRaizServidor`: con `TRAMTAI_SERVIDOR=1` (en un proceso hijo, porque la raíz se calcula una vez), `RaizUsuario()` es `RaizServidor()`.
  - `TestUsarServidorTarde`: el pánico.
  - `TestSinServidorNoTocaElDelSistema` (Revisión 2): con un servicio falso de otra versión en el descriptor de una raíz de servidor temporal, la CLI de escritorio no lo ve. Con `--servidor`, `iniciarServicio` devuelve `ErrServidorOtraVersion` y el servicio falso sigue respondiendo.
  - `TestAcercaEntorno`: `/ui/acerca` lleva `entorno`.
- [ ] **Paso 2:** `go test ./internal/carpetas/ ./internal/cliente/ ./cmd/tramtai/ ./internal/servicio/ -run 'Servidor|Entorno'`. Esperado: FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat: raíz de datos del servidor y la opción --servidor`.

### Tarea 6: `tramtai servidor` en Linux (systemd)

**Archivos:**
- Crear: `internal/servidor/{servidor.go, unidad.go, linux.go, otros.go}`, `internal/servidor/testdata/{tramtai.service, tramtai-letsencrypt.service}` y `cmd/tramtai/servidor.go`.
- Cambiar: `cmd/tramtai/servicio.go:456` (`signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`; en Windows no existe `SIGTERM`, así que va con etiquetas de compilación) y `internal/idioma/en_cli.go`.
- Pruebas: `internal/servidor/unidad_test.go`, `linux_test.go` y `cmd/tramtai/servidor_test.go`.

**Interfaces que deja:**
- `servidor.Rutas() Rutas{Programa, Datos, Envoltorio, Unidad string}`, con los valores de «Restricciones globales».
- `servidor.Opciones{Origen string /* carpeta del programa que se copia */; AceptoTerminos bool; Conversor string; LetsEncrypt bool; PuertoBajo bool}`.
- `servidor.Unidad(o UnidadOpciones) string`. Es pura y genera el texto de spec §2:
  - `ExecStart=/opt/tramtai/tramtai servicio iniciar --registro /var/lib/tramtai/datos/servicio.log`;
  - `TimeoutStopSec=` igual a `servicio.plazo_drenaje_segundos` más 30;
  - `RuntimeDirectory=tramtai` con `RuntimeDirectoryMode=0755` (lo usa la Tarea 14);
  - `AmbientCapabilities=CAP_NET_BIND_SERVICE` solo con `LetsEncrypt || PuertoBajo`.
- `servidor.Instalar(o Opciones) error`, `Desinstalar(borrarDatos bool) error`, `Estado(ctx) (EstadoSistema, error)` y `Reiniciar() error`. Los de Linux están en `linux.go`; en `otros.go` devuelven un error traducido: «el modo servidor está disponible en Linux con systemd y en Windows».
- `servidor.EsAdministrador() bool`: `os.Geteuid()==0` en Linux.
- `var ejecutar = func(nombre string, args ...string) ([]byte, error)`: se sustituye en las pruebas.
- `cmd/tramtai/servidor.go`: `comandoServidor()` con `instalar`, `desinstalar [--borrar-datos]`, `estado`, `reiniciar`, `registro [-f] [-n]` y sus alias.
  - `instalar` muestra `terminos.Riesgos(true)` y el enlace, y pide escribir «acepto» (o `--acepto-terminos`); después llama a `terminos.Aceptar(datos, ComoServidorInstalar | ComoOpcion)`.
  - `registro` reutiliza `seguirRegistro`.
- El envoltorio es un script `#!/bin/sh\nexec /opt/tramtai/tramtai --servidor "$@"`.

- [ ] **Paso 1:** las pruebas:
  - `TestUnidadSystemd` y `TestUnidadLetsEncrypt`: comparan con `testdata/` byte a byte.
  - `TestInstalarLinuxOrdenes`, con `ejecutar` falso y un `Rutas` en una carpeta temporal. El orden es:
    1. `useradd --system --home /var/lib/tramtai --shell /usr/sbin/nologin tramtai` (solo si `id tramtai` falla);
    2. copiar `tramtai` y `uv`;
    3. escribir la unidad;
    4. `systemctl daemon-reload`;
    5. `systemctl enable --now tramtai`.
  - `TestInstalarSinAceptar`: falla sin hacer ninguna orden.
  - `TestInstalarRepetido`: con la misma versión no copia ni reinicia.
  - `TestDesinstalarConservaDatos`, y con `--borrar-datos`, `userdel tramtai` y el borrado de la carpeta tras confirmar.
  - `TestServidorNoAdministrador`: el mensaje traducido.
  - `TestSIGTERMDrena` (solo Unix): un proceso hijo con un trabajo simulado recibe SIGTERM y termina el trabajo antes de salir.
- [ ] **Paso 2:** `go test ./internal/servidor/ ./cmd/tramtai/ -run 'Unidad|Instalar|Desinstalar|Servidor|SIGTERM'`. Esperado: FAIL.
- [ ] **Paso 3:** implementar. La configuración inicial (`config.yaml` en la raíz del servidor) se escribe solo si no existe, con `servicio.host: 127.0.0.1`.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): instalar el servicio del sistema en Linux`.

### Tarea 7: `tramtai servidor` en Windows

**Archivos:**
- Crear: `internal/servidor/windows.go`, `cmd/tramtai/servicio_windows.go` y `servicio_otros.go`.
- Cambiar: `internal/carpetas/permisos_windows.go` (`ProtegerServidor`) y `cmd/tramtai/instalacion.go` (el PATH del sistema).
- Pruebas: `internal/servidor/windows_test.go`.

**Interfaces que deja:**
- `func ejecutarComoServicio(cmd *cobra.Command, archivo string) (bool, error)`:
  - si `svc.IsWindowsService()`, ejecuta `iniciarServicio` bajo `svc.Run("Tramtai", manejador)`;
  - `svc.Stop` y `svc.Shutdown` cancelan el contexto que ya provoca el drenaje;
  - informa `StartPending`, `Running` y `StopPending`.
  - `iniciarServicio` lo llama al principio; en otros sistemas devuelve `false, nil`.
- `servidor.Instalar` en Windows:
  - copia a `C:\Program Files\Tramtai Servidor\` (`tramtai.exe`, `uv.exe` y `tramtai-servidor.cmd`, con `@"%~dp0tramtai.exe" --servidor %*`);
  - usa `mgr.Connect` y `CreateService("Tramtai", ruta, mgr.Config{DisplayName: "Tramtai (servidor)", StartType: mgr.StartAutomatic, DelayedAutoStart: true, ServiceStartName: `NT SERVICE\Tramtai`}, "--servidor", "servicio", "iniciar", "--registro", <datos>\datos\servicio.log)`;
  - `SetRecoveryActions` con tres reinicios a los 10 s;
  - `carpetas.ProtegerServidor(datos, "NT SERVICE\\Tramtai")`, con el SDDL `D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;<SID del servicio>)`;
  - añade la carpeta del programa al PATH de `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, con el mismo código que `instalacion agregar-ruta` pero en HKLM, y difunde `WM_SETTINGCHANGE`;
  - `Start`.
- `servidor.EsAdministrador()` en Windows: `windows.GetCurrentProcessToken().IsElevated()`.
- `Opciones.Conversor` (`--conversor <zip>`, en los dos sistemas): descomprime el paquete del conversor sin red en `documentos.RaizConversor()` de la raíz del servidor, con la misma comprobación que el botón «Preparar archivos». Prueba: `TestInstalarConConversor`, con un `.zip` mínimo de prueba.

- [ ] **Paso 1:** las pruebas, con `//go:build windows` y omitidas si no hay elevación (`t.Skip`), porque en el CI de Windows sí la hay:
  - `TestInstalarServicioWindows`: instala con un nombre de prueba (`TramtaiPrueba<pid>`, mediante una variable sustituible) y comprueba la cuenta, el arranque automático retrasado, las acciones de recuperación y la DACL de la carpeta. Después, desinstala.
  - `TestManejadorDrena`: un `svc.Stop` simulado al manejador cancela el contexto.
- [ ] **Paso 2:** `go test ./internal/servidor/ ./cmd/tramtai/ -run 'Windows|Manejador'` en una consola de administrador. Esperado: FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** la verificación global, más una comprobación a mano en este Windows: `tramtai servidor instalar --acepto-terminos`, `tramtai-servidor estado` en marcha y `tramtai-servidor desinstalar --borrar-datos`.
- [ ] **Paso 5:** commit `feat(servidor): instalar el servicio de Windows`.

### Tarea 8: las claves del servidor

**Archivos:**
- Cambiar:
  - `internal/credenciales/credenciales_linux.go:45-57` (Leer) y `credenciales.go` (nombres internos);
  - `internal/servicio/admin.go:14-188` (rutas);
  - `internal/cliente/cliente_ajustes.go`;
  - `cmd/tramtai/claves.go:17-80`;
  - `internal/servidor/linux.go` (`GuardarClave`).
- Pruebas: `credenciales_unix_test.go`, `internal/servicio/admin_claves_test.go`, `internal/servidor/linux_test.go` y `cmd/tramtai/claves_test.go`.

**Interfaces que deja:**
- `credenciales.Internos = []string{"TRAMTAI_TOTP"}` y `credenciales.AdmitidoInterno(nombre string) bool`. `claves lista` no los enseña.
- En Linux, `Leer(nombre)` mira primero el archivo `filepath.Join(os.Getenv("CREDENTIALS_DIRECTORY"), "tramtai."+nombre)`. Si no está, sigue como hoy. `Almacen()` dice «credenciales de systemd» cuando `CREDENTIALS_DIRECTORY` está definida.
- La API de administración, con clave de servicio y además `desdeEsteEquipo(r)`:
  - `GET /api/v1/servicio/claves` → `[]contrato.ClaveGuardada{Nombre string json:"nombre"; Guardada bool json:"guardada"}`;
  - `PUT /api/v1/servicio/claves/{nombre}` con `{"valor":"…"}` → 204;
  - `DELETE /api/v1/servicio/claves/{nombre}` → 204.
- `(c *Cliente) Claves(ctx)`, `GuardarClave(ctx, nombre, valor string) error` y `BorrarClave(ctx, nombre string) error`.
- `servidor.GuardarClave(nombre, valor string) error` en Linux:
  1. `systemd-creds encrypt --name=tramtai.<N> - /etc/credstore.encrypted/tramtai.<N>`, con el valor por stdin;
  2. reescribe `/etc/systemd/system/tramtai.service.d/claves.conf` con una línea `LoadCredentialEncrypted=tramtai.<N>:/etc/credstore.encrypted/tramtai.<N>` por clave;
  3. `systemctl daemon-reload` y `systemctl restart tramtai`.
- **Sin `systemd-creds` o con versión < 250:** reescribe `/etc/tramtai/claves.env` (0600, root), con `EnvironmentFile=-/etc/tramtai/claves.env` en `claves.conf`, y devuelve `servidor.ErrClaveEnClaro` (un aviso, no un fallo) para que la CLI avise.
- `servidor.BorrarClave(nombre string) error`, simétrico.
- `cmd/tramtai/claves.go` con `carpetas.Servidor()`:
  - en Windows, por el cliente;
  - en Linux, con `servidor.GuardarClave`;
  - sin modo servidor, como hoy;
  - el alias `keys set`.

- [ ] **Paso 1:** las pruebas:
  - `TestLeerCredentialsDirectory`: con un archivo en una carpeta temporal y la variable, `Leer` lo devuelve sin llamar a `secret-tool`.
  - `TestAdminClavesSoloLocal`: con clave de servicio y `X-Forwarded-For`, 403; desde loopback, 204, y la clave queda guardada (con un almacén falso).
  - `TestGuardarClaveSystemd`: las órdenes y el contenido de `claves.conf` con dos claves.
  - `TestGuardarClaveSinSystemdCreds`: escribe `claves.env` 0600 y devuelve `ErrClaveEnClaro`.
  - `TestClavesListaSinInternos`.
  - `TestNoSeGuardaTokenClaude`: `claves guardar CLAUDE_CODE_OAUTH_TOKEN` se rechaza con un mensaje traducido que explica que la sesión de Claude la hace su propio CLI (spec §5), y `credenciales.Admitido` devuelve `false` para ese nombre.
- [ ] **Paso 2:** `go test ./internal/credenciales/ ./internal/servicio/ ./internal/servidor/ ./cmd/tramtai/ -run 'Credentials|AdminClaves|GuardarClave|ClavesLista|TokenClaude'`. Esperado: FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): claves sin escritorio (systemd-creds y almacén del servicio)`.

### Tarea 9: exposición, proxy de confianza y dominios

**Archivos:**
- Crear: `internal/servidor/reglas.go` y `reglas_test.go`; `internal/servicio/red.go` y `red_test.go`.
- Cambiar:
  - `internal/configuracion/configuracion.go:68-135,303-312` (las claves nuevas);
  - `cmd/tramtai/servicio.go:369-380,404-407` (las reglas y los hosts);
  - `internal/servicio/local.go:66-74` (`desdeEsteEquipo`);
  - `internal/servicio/servicio.go` (`Opciones.Confiables []netip.Prefix`, `Opciones.Dominios []string`).

**Interfaces que deja:**
- `servidor.Entrada{Host string; PermitirRemoto bool; Confiables []netip.Prefix; TLS string; Dominios []string; Correo string; Puerto int; Certificado, Clave string; Suscripciones []Suscripcion; TOTP bool}` y `servidor.Suscripcion{Agente string; Personal bool}`.
- `servidor.Expuesto(e Entrada) bool`: `!loopback(Host) || len(Confiables) > 0`.
- `servidor.ComprobarArranque(e Entrada) error`: las reglas de spec §6 en este orden, cada una con su mensaje traducido:
  1. remoto sin `PermitirRemoto`;
  2. remoto con TLS `ninguno` (el mensaje ofrece el proxy, HTTPS o el túnel SSH);
  3. expuesto con una suscripción no personal;
  4. expuesto con una personal sin TOTP;
  5. `letsencrypt` sin dominios, sin correo o con un puerto distinto de 443;
  6. `archivos` con el certificado ilegible o que no cubre los dominios.
  - Sustituye a `servicio.go:370-380`. La 4 y la 6 se comprueban con datos que dan las Tareas 10 y 12; hasta entonces, `TOTP` llega como `false` y la regla 4 no se puede cumplir, que es lo seguro.
- `servicio.ClienteReal(r *http.Request, confiables []netip.Prefix) netip.Addr`: si `RemoteAddr` es de confianza, recorre `X-Forwarded-For` de derecha a izquierda y devuelve la primera dirección que no es de confianza; si no, `RemoteAddr`.
- `servicio.EsHTTPS(r *http.Request, confiables []netip.Prefix) bool`: `r.TLS != nil`, o una conexión de confianza con `X-Forwarded-Proto: https`.
- `desdeEsteEquipo` no cambia de comportamiento: cualquier cabecera de proxy hace que la petición no sea local, también desde un proxy de confianza.
- Los hosts permitidos suman `servicio.dominios`.
- Con `Expuesto`, `iniciarServicio` escribe en el registro el riesgo 3 (`terminos.Riesgos(true)[2]`) al arrancar, y `servidor estado` lo muestra.

- [ ] **Paso 1:** las pruebas:
  - `TestAvisoExpuestoEnRegistro`: con `servicio.proxy.confiables: [127.0.0.1/32]`, el registro del arranque contiene el riesgo 3; en loopback sin proxy, no.
  - `TestComprobarArranque`: una tabla con una fila por regla, más dos combinaciones seguras (loopback sin nada; `0.0.0.0` con `archivos` y dominio).
  - `TestClienteReal`: confiable con `XFF: 203.0.113.9, 10.0.0.2`, siendo `10.0.0.0/8` de confianza → `203.0.113.9`; no confiable con XFF → `RemoteAddr`.
  - `TestEsHTTPS`.
  - `TestProxyLoopbackNoEsLocal` (Revisión 3): una petición desde 127.0.0.1 con `X-Forwarded-For` y `servicio.proxy.confiables: [127.0.0.1/32]` no abre la entrada local y, con Claude en suscripción no personal, `ComprobarArranque` falla.
  - `TestDominiosEnHosts`: `Host: tramtai.ejemplo.org` pasa con ese dominio y da 421/403 (lo que devuelva hoy `api.Proteger`) sin él.
- [ ] **Paso 2:** `go test ./internal/servidor/ ./internal/servicio/ -run 'ComprobarArranque|ClienteReal|EsHTTPS|ProxyLoopback|Dominios'`. Esperado: FAIL.
- [ ] **Paso 3:** implementar. Las claves nuevas van en la tabla de tipos y en `Internos()`.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): reglas de exposición, proxy de confianza y dominios`.

### Tarea 10: HTTPS con certificado propio (solo TLS 1.3)

**Archivos:** crear `internal/servicio/tls.go` y `tls_test.go`; cambiar `cmd/tramtai/servicio.go:409-442` (el oyente TLS) y el envoltorio del handler (HSTS).

**Interfaces que deja:**
- `servicio.ConfigTLS(o OpcionesTLS) (*tls.Config, error)`, con `OpcionesTLS{Modo, Certificado, Clave string; Dominios []string; Datos string; Correo string}`.
  - Con `archivos`: `MinVersion: tls.VersionTLS13` y `GetCertificate`, que vuelve a cargar el par cuando cambia la fecha de modificación de cualquiera de los dos archivos (como mucho una comprobación por segundo).
  - Con `ninguno`: `nil, nil`.
- `iniciarServicio` envuelve el oyente con `tls.NewListener` si hay configuración.
- Con TLS propio, o con `EsHTTPS(r)`, toda respuesta lleva `Strict-Transport-Security: max-age=31536000`.

- [ ] **Paso 1:** las pruebas, con un certificado generado en la prueba (`crypto/x509`, ECDSA P-256):
  - `TestTLSRechaza12`: un cliente con `MaxVersion: tls.VersionTLS12` falla el saludo.
  - `TestTLSPoscuantico`: un cliente Go por defecto negocia `tls.VersionTLS13` y `ConnectionState().CurveID == tls.X25519MLKEM768`.
  - `TestTLSRecarga`: se cambian los archivos por otro certificado y la conexión siguiente lo presenta.
  - `TestHSTS`.
- [ ] **Paso 2:** `go test ./internal/servicio/ -run 'TLS|HSTS'`. Esperado: FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): HTTPS con certificado propio, solo TLS 1.3`.

### Tarea 11: Let's Encrypt integrado (opcional)

**Archivos:** cambiar `internal/servicio/tls.go`, `go.mod`/`go.sum` (`golang.org/x/crypto`, la última publicada hace más de 7 días) y `.github/workflows/ci.yml` (el job `pebble`). Crear `internal/servicio/tls_pebble_test.go` (etiqueta `pebble`).

**Interfaces:**
- Con `letsencrypt`, `ConfigTLS` usa `autocert.Manager{Prompt: autocert.AcceptTOS, HostPolicy: autocert.HostWhitelist(dominios...), Cache: autocert.DirCache(filepath.Join(datos, "acme")), Email: correo}`.
  - De `m.TLSConfig()` toma `GetCertificate` y `NextProtos` (incluye `acme-tls/1`) y fija `MinVersion: tls.VersionTLS13`.
  - La carpeta `acme` se crea con 0700.
- `var clienteACME *acme.Client`: sustituible para apuntar a Pebble en las pruebas.
- Al activar el modo, la CLI y el registro dicen que se acepta el acuerdo de suscriptor de Let's Encrypt, con su enlace.

- [ ] **Paso 1:** `TestLetsEncryptPebble` (etiqueta `pebble`, Pebble en `PEBBLE_URL`): obtiene un certificado para `tramtai.prueba` por TLS-ALPN-01, que es ECDSA y queda en la caché. `TestLetsEncryptHostPolicy` (sin etiqueta): un SNI fuera de la lista no obtiene certificado.
- [ ] **Paso 2:** el job `pebble` en `ci.yml`, en Linux:
  - el servicio `ghcr.io/letsencrypt/pebble` con `PEBBLE_VA_ALWAYS_VALID=0`;
  - `go test -tags pebble ./internal/servicio/ -run Pebble`.
- [ ] **Paso 3:** implementar. Esperado: los dos PASS (en local, el de Pebble con Docker si lo hay).
- [ ] **Paso 4:** la verificación global. `govulncheck` sin avisos nuevos.
- [ ] **Paso 5:** commit `feat(servidor): Let's Encrypt integrado (opcional)`.

### Tarea 12: el administrador y la Web UI remota

**Archivos:**
- Crear: `internal/totp/totp.go` y `totp_test.go`; `internal/servicio/administrador.go`, `administrador_test.go` y `frenos.go`; `cmd/tramtai/administrador.go`.
- Cambiar:
  - `internal/servicio/web.go:20-80,212-261` (sesión, cookie y rutas);
  - `web/src/paginas/Entrar.vue` y `Configuracion.vue` (la pestaña Seguridad);
  - `internal/tui/paginas/configuracion`;
  - textos.

**Interfaces que deja:**
- **`totp`:**
  - `Generar() (string, error)`;
  - `Codigo(secreto string, t time.Time) (string, error)`;
  - `Valido(secreto, codigo string, t time.Time, ultimoPaso int64) (paso int64, ok bool)`;
  - `URI(secreto, cuenta string) string` → `otpauth://totp/Tramtai:<cuenta>?secret=<s>&issuer=Tramtai`.
- **El administrador, en el servicio:**
  - `FijarContrasena(datos, c string) error` (menos de 12 caracteres → error traducido);
  - `ComprobarContrasena(datos, c string) (bool, error)` (en tiempo constante; sin archivo → `false, nil`);
  - `NuevosCodigosRecuperacion(datos string) ([]string, error)`: 10 códigos `XXXXX-XXXXX` en base32, guardados como huella;
  - `UsarCodigoRecuperacion(datos, c string) (bool, error)`: lo borra al usarlo.
- **El secreto TOTP** se lee y escribe con `credenciales.Leer`/`Guardar("TRAMTAI_TOTP")`.
  - En Windows, el servicio lo escribe en su propio almacén.
  - En Linux, el servicio no puede escribir en `systemd-creds`. Por eso activar o quitar el TOTP es solo de la CLI (`servidor.GuardarClave`), y la Web UI muestra la orden. Es una consecuencia de spec §4 que se anota en la Ayuda.
- **`frenos`:**
  - `(f *frenos) Permitido(ip netip.Addr, ahora time.Time) (espera time.Duration)`;
  - `Fallo(ip, ahora)` y `Exito(ip)`, con los valores de «Restricciones globales»;
  - un semáforo `chan struct{}` de 2 para argon2id.
- **Rutas:**
  - `GET /ui/entrada` (pública) → `contrato.EntradaRemota{Contrasena bool json:"contrasena"; TOTP bool json:"totp"; HTTPS bool json:"https"}`;
  - `POST /ui/acceder` (pública) con `{contrasena, codigo}`:
    - solo si `EsHTTPS` y hay administrador; si no, 403 con el porqué;
    - si falla, 401 «contraseña o código incorrectos», siempre igual;
    - bloqueado, 429 con `Retry-After`;
    - si sale bien, una sesión **no local** de la instancia global;
  - `POST /ui/administrador/contrasena` (`conSesion` y CSRF) con `{actual, nueva}`: cierra todas las sesiones;
  - `POST /ui/administrador/totp` y `DELETE /ui/administrador/totp` (solo Windows; en Linux, 409 con la orden);
  - `POST /ui/administrador/recuperacion`;
  - `POST /ui/sesiones/cerrar`.
- **`sesion`** gana `generacion uint64`. El servicio guarda la generación vigente, y cambiar la contraseña o cerrar las sesiones la sube.
- **La cookie,** cuando la petición es `EsHTTPS`: nombre `__Host-tramtai_sesion_<puerto>`, con `Secure`, `Path=/` y sin `Domain`.
- **CLI:** `administrador contraseña` (`admin password`) escribe directamente en `datos/` del servidor y pide la contraseña dos veces sin eco. `administrador totp activar|quitar` (`admin totp enable|disable`) va por la API en Windows y por `servidor.GuardarClave` en Linux.
- **Registro:** cada acceso bueno o malo y cada bloqueo, con `ClienteReal`.

- [ ] **Paso 1:** las pruebas:
  - `TestTOTPVectores`: el secreto ASCII `12345678901234567890` a t=59 da `287082` (RFC 6238 en 6 dígitos), y a t=1111111109 da `081804`.
  - `TestTOTPNoReutiliza` y `TestTOTPMargen`: ±30 s valen y ±90 s no.
  - `TestContrasenaArgon2`: el archivo empieza por `$argon2id$v=19$m=65536,t=3,p=1$`; la buena es `true` y la mala `false`; con 11 caracteres, error.
  - `TestRecuperacionUnSoloUso`.
  - `TestAccederSoloHTTPS`: sin TLS y sin proxy, 403; con `httptest.NewTLSServer`, 200 y la cookie `__Host-` con `Secure`.
  - `TestAccederMismoMensaje`: el cuerpo es igual con la contraseña mala y con el código malo.
  - `TestFrenoPorIP`: el sexto intento devuelve 429. `TestFrenoGlobal`.
  - `TestFrenoNoSeEvadeConXFF` (Revisión 5): 6 intentos desde una dirección no confiable con `X-Forwarded-For` distinto cada vez dan 429.
  - `TestCambiarContrasenaCierraSesiones`, y `TestSesionRemotaNoEsLocal` (no puede `PUT` claves ni ver las carpetas en `/ui/acerca`).
  - `TestAccederSinRedireccion`: `POST /ui/acceder?siguiente=https://otro.ejemplo` responde JSON sin cabecera `Location`.
  - `TestQuitarTOTPConSuscripcionPersonal`: con `adaptadores.claude.suscripcion_personal: true`, `DELETE /ui/administrador/totp` y `administrador totp quitar` responden 409 con el porqué.
  - `TestFrenoClavesFalsas`: 31 peticiones con claves falsas desde una dirección en un minuto → la 31 da 429, y otra dirección sigue recibiendo 401.
  - Web: `Entrar.test.ts` muestra el formulario con `contrasena:true` y `https:true`, y el porqué con `https:false`.
- [ ] **Paso 2:** `go test ./internal/totp/ ./internal/servicio/ ./cmd/tramtai/ -run 'TOTP|Contrasena|Recuperacion|Acceder|Freno|Sesion'`; `cd web && npm test`. Esperado: FAIL.
- [ ] **Paso 3:** implementar. En la Tarea 9, `ComprobarArranque` recibe `TOTP: <hay secreto>`.
- [ ] **Paso 4:** la verificación global y la de web. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): administrador con contraseña y TOTP para la Web UI remota`.

### Tarea 13: la suscripción en el servidor

**Archivos:**
- Cambiar:
  - `internal/agentes/catalogo.go:36,57,100` (límites y `Ficha`);
  - las fichas de claude, codex y copilot (`InicioSesion`);
  - `internal/servicio/web_servicio.go:124,218-225` (`cambiarAutenticacion`);
  - `cmd/tramtai/ejecutar.go:30-83` (el grupo `agentes`);
  - `cmd/tramtai/servicio.go` (`Entrada.Suscripciones`).
- Pruebas: `internal/agentes/catalogo_test.go`, `internal/servicio/web_servicio_test.go` y `cmd/tramtai/agentes_test.go`.

**Interfaces que deja:**
- `Ficha.InicioSesion []string`: claude `["auth","login"]`, codex `["login","--device-auth"]`, copilot `["login"]`. Si la M0 cambia algo, manda la M0.
- `agentes entrar <agente>` (`agents login`), solo en modo servidor.
  - En Linux: `sudo -u tramtai -H env HOME=/var/lib/tramtai <comando del agente> <InicioSesion…>`, con la terminal conectada y, en codex, el `CODEX_HOME` privado del adaptador.
  - En Windows, lo que diga la M0; si la M0 la deja fuera, el error «no disponible en Windows Server para <agente>».
- `agentes suscripcion <agente> --personal [--simultaneas N]` (`agents subscription`):
  - enseña las condiciones de spec §5 con `terminos.Riesgos(true)[0:2]`, pide escribir «acepto» y escribe `adaptadores.<a>.autenticacion: suscripcion`, `suscripcion_personal: true` y `suscripcion_simultaneas: N`, con `configuracion.FijarValor` en la configuración global del servidor;
  - después, `servidor.Reiniciar()`;
  - si N es mayor que 1, avisa del riesgo y pide confirmarlo;
  - sin `--personal`, la suscripción local de siempre.
- En `agentes.Registro(c)`, con `carpetas.Servidor()` y el agente en suscripción, el límite es `min(concurrencia, suscripcion_simultaneas)`. La cuenta es por agente en todo el servicio, porque `Semaforos` ya es por agente, sumando todas las APIs y todos los espacios.
- `cambiarAutenticacion`, en modo servidor: desde una sesión remota, 403; desde una local con el servicio expuesto, 409 con la orden `agentes suscripcion`.

- [ ] **Paso 1:** las pruebas:
  - `TestSimultaneasServidor`: con `suscripcion_simultaneas: 1`, dos trabajos de dos APIs distintas con Claude simulado en suscripción se ejecutan uno tras otro (el segundo empieza después de que el primero termina); con clave de API, a la vez.
  - `TestSimultaneasTope`: el valor 4 se rechaza al cargar la configuración (`errorConfiguracion`, de 1 a 3).
  - `TestSuscripcionPersonalCLI`: sin «acepto» no escribe nada; con «acepto», las tres claves.
  - `TestAutenticacionRemotaProhibida`.
  - `TestEntrarSoloServidor`: sin modo servidor, el error traducido.
- [ ] **Paso 2:** `go test ./internal/agentes/ ./internal/servicio/ ./cmd/tramtai/ -run 'Simultaneas|SuscripcionPersonal|AutenticacionRemota|Entrar'`. Esperado: FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): suscripción personal con condiciones y peticiones a la vez`.

### Tarea 14: la Web UI y la terminal del servidor, y los dos entornos

**Archivos:**
- Cambiar: `web/src/App.vue:10-111` (la marca), `web/src/paginas/Configuracion.vue` (Servidor y Seguridad; sin «Arranque» ni «Instalar ahora» en el servidor), `web/src/paginas/Agentes.vue`, `web/src/paginas/Acerca*` (lo que muestre `/ui/acerca`), `internal/tui/cabecera.go:201` (`vistaEstado`), `internal/tui/paginas/{configuracion,agentes}`, `internal/servicio/web_servicio.go` y `cmd/tramtai/estado.go`.
- Crear: `internal/servidor/publico.go`.

**Interfaces que deja:**
- `contrato.EstadoServidor{TLS string json:"tls"; Dominios []string json:"dominios"; Confiables []string json:"confiables"; VenceCertificado *time.Time json:"vence_certificado,omitempty"; Administrador bool json:"administrador"; TOTP bool json:"totp"; Suscripciones []contrato.SuscripcionServidor json:"suscripciones"}` y `SuscripcionServidor{Agente string; Personal bool; Simultaneas int}`.
- `GET /ui/servidor` (`conSesion`) devuelve `EstadoServidor`; 404 si `Entorno != "servidor"`.
- **El aviso cruzado:**
  - `servidor.PublicarPuerto(puerto int) error`: el servicio del sistema escribe su puerto en `/run/tramtai/puerto` (0644) o en `%ProgramData%\Tramtai\publico\puerto` (lectura para `Usuarios`);
  - `servidor.PuertoDelSistema() (int, bool)` lo leen la app, la terminal y `tramtai servicio estado` del usuario;
  - si hay puerto, avisan: «Hay otro Tramtai, del sistema, en el puerto X: es otro entorno, con sus propias APIs y claves».
- **La marca:** «Servidor», «Este equipo» o «Portable», según `Acerca.Entorno`, en la cabecera de la Web UI (siempre visible, también en el móvil) y en la línea de estado de la terminal.
- **La CLI con `--servidor`:** cada salida que crea o cambia algo empieza por «Servidor: ».
- **El aviso fijo** de la suscripción personal (spec §5) en la Web UI (un `q-banner` como el de `App.vue:159`) y en la terminal.
- **La tecla `W` de la Tarea 4** en el servidor con `EstadoServidor.Administrador`: muestra solo la URL, sin pedir código. Prueba: `TestTeclaWServidorConAdministrador`.

- [ ] **Paso 1:** las pruebas:
  - `TestEstadoServidorUI`, `TestPuertoPublicado` y `TestAvisoCruzado` (con una ruta pública temporal sustituible).
  - Web (`App.test.ts`, `Configuracion.test.ts`, `Agentes.test.ts`): la marca según el entorno; Servidor y Seguridad solo en el servidor; sin «Arranque» ni «Instalar ahora» en el servidor; Agentes sin «Usar mi suscripción» en el servidor; el aviso fijo con suscripción personal.
  - TUI: `TestMarcaEntorno` y `TestConfiguracionServidorSoloLectura`.
- [ ] **Paso 2:** `go test ./internal/servidor/ ./internal/servicio/ ./internal/tui/... -run 'EstadoServidor|Puerto|Aviso|Marca|SoloLectura'`; `cd web && npm test`. Esperado: FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** la verificación global, la de web y `go generate ./internal/contrato`. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): Web UI y terminal del servidor, dos entornos a la vista`.

### Tarea 15: `tramtai servidor actualizar`

**Archivos:** cambiar `internal/actualizacion/actualizacion.go:40,295` (asset por sistema), `internal/servidor/{servidor,linux,windows}.go` y `cmd/tramtai/servidor.go`.

**Interfaces que deja:**
- `actualizacion.Asset(version, goos, goarch string) (string, error)`:
  - `linux` → `tramtai-<version>-linux-<goarch>.tar.gz`;
  - `windows/amd64` → el `.zip` portable (`Tramtai-<version>-portable-windows-x64.zip`; se confirma el nombre exacto en `publicar.yml`);
  - otros → error.
- `actualizacion.DescargarAsset(ctx, v Version, nombre, dir string, avance Avance) (string, error)`: generaliza `Descargar`, con la misma verificación de la firma de `SHA256SUMS.txt` y la suma.
- `servidor.Actualizar(ctx) error`:
  1. si existe `/opt/tramtai/.paquete`, devuelve `servidor.ErrInstaladoPorPaquete` («actualiza con apt o dnf»);
  2. descarga y verifica;
  3. extrae `tramtai` y `uv`;
  4. detiene el servicio (con drenaje);
  5. renombra los actuales a `.anterior`, pone los nuevos y arranca.
  - Si falla antes de arrancar, devuelve los `.anterior` a su sitio.

- [ ] **Paso 1:** las pruebas:
  - `TestAssetPorSistema`.
  - `TestActualizarServidor`: con un servidor HTTP de prueba que publica un `.tar.gz` firmado con la clave de prueba (como `TestDescargar`), con `ejecutar` falso, comprueba el orden detener → cambiar → arrancar y que quedan los `.anterior`.
  - `TestActualizarPaquete`.
  - `TestActualizarFirmaMala`: no toca nada.
- [ ] **Paso 2:** `go test ./internal/actualizacion/ ./internal/servidor/ -run 'Asset|Actualizar'`. Esperado: FAIL.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** la verificación global. Esperado: PASS.
- [ ] **Paso 5:** commit `feat(servidor): actualizar el servidor con verificación de firma`.

### Tarea 16: paquetes `.deb` y `.rpm` del servidor

**Archivos:**
- Crear: `scripts/linux/nfpm-servidor.yaml`, `scripts/linux/servidor/{tramtai.service, tramtai-servidor, sysusers.conf, tmpfiles.conf, postinst.sh, prerm.sh, postrm.sh}` y `scripts/linux/probar-paquete-servidor.sh`.
- Cambiar: `scripts/linux/empaquetar.sh:22-23` y `.github/workflows/publicar.yml` (job `linux-app` y la lista de assets).

**Interfaces:**
- **El paquete:** nombre `tramtai-servidor`, archivos `dist/Tramtai-$version-servidor-linux-$arq.{deb,rpm}`. Su contenido es el de spec §9. Además, `/opt/tramtai/.paquete` (vacío).
- **`scripts/linux/servidor/tramtai.service`** es igual a `servidor.Unidad(UnidadOpciones{})`. La prueba `TestUnidadDelPaqueteAlDia` (en `internal/servidor`) lo compara.
- **`sysusers.conf`:** `u tramtai - "Tramtai (servidor)" /var/lib/tramtai /usr/sbin/nologin`.
- **`tmpfiles.conf`:** `d /var/lib/tramtai 0700 tramtai tramtai -`.
- **`postinst.sh`:**
  - `systemd-sysusers` y `systemd-tmpfiles --create`;
  - si la unidad ya estaba habilitada, `systemctl daemon-reload` y `systemctl try-restart tramtai`;
  - si no, imprime «Para empezar: sudo tramtai-servidor instalar».
- **`prerm.sh`:** al quitar (no al actualizar), `systemctl disable --now tramtai`. **`postrm.sh`** no borra `/var/lib/tramtai` ni las credenciales, ni con purge.
- **`servidor.Instalar`** detecta `.paquete`: no copia el programa ni escribe la unidad; solo crea la configuración, acepta los términos, habilita y arranca.

- [ ] **Paso 1:** `TestUnidadDelPaqueteAlDia` y `TestInstalarConPaquete` (no copia ni escribe la unidad).
- [ ] **Paso 2:** `probar-paquete-servidor.sh <deb|rpm>`, que se ejecuta en `debian:13` y `rockylinux:9`:
  - instala el paquete de escritorio y el del servidor a la vez;
  - comprueba los archivos, la cuenta (`getent passwd tramtai`), que `/usr/bin/tramtai` es el de escritorio y que no se arrancó nada;
  - quita el del servidor y comprueba que sigue `/var/lib/tramtai`.
- [ ] **Paso 3:** implementar y añadir al job `linux-app` de `publicar.yml` la generación y la prueba. Los assets nuevos entran en `SHA256SUMS.txt` como el resto.
- [ ] **Paso 4:** la verificación global, y `sh scripts/linux/empaquetar.sh 0.0.0 amd64` en un contenedor o en WSL si lo hay. Si no se puede en local, el CI del PR lo comprueba (el job `gtk-para-publicar` de `ci.yml` ya compila lo de `linux-app`; se le añade el paquete del servidor).
- [ ] **Paso 5:** commit `feat(servidor): paquetes .deb y .rpm del servidor`.

### Tarea 17: CI del servidor y los ejemplos de proxies

**Archivos:**
- Crear: `docs/servidor/proxies/{caddy/Caddyfile, nginx/tramtai.conf, apache/tramtai.conf, traefik/tramtai.yml, haproxy/haproxy.cfg, iis/web.config}`, `docs/servidor/tuneles.md`, `scripts/servidor/probar-servidor.sh`, `scripts/servidor/probar-servidor.ps1`, `scripts/servidor/probar-proxies.sh` y `scripts/servidor/docker-compose.proxies.yml`.
- Cambiar: `.github/workflows/ci.yml` (jobs `servidor-linux`, `servidor-windows` y `proxies`).

**Interfaces:**
- **Cada ejemplo de proxy:**
  - TLS 1.3 (nginx: `ssl_protocols TLSv1.3; ssl_ecdh_curve X25519MLKEM768:X25519;`; Caddy: `protocols tls1.3`; Apache: `SSLProtocol -all +TLSv1.3`);
  - `X-Forwarded-Proto` y Host conservado;
  - sin búfer para SSE (nginx `proxy_buffering off`, Apache `flushpackets=on`, HAProxy `option http-server-close` con `timeout tunnel 1h`, IIS `responseBufferLimit="0"`);
  - en el lado de Tramtai: `servicio.proxy.confiables: [127.0.0.1/32]`.
- **`probar-servidor.sh`** (Linux, con sudo):
  1. instalar desde el `.deb` construido;
  2. contraseña de administrador;
  3. `servicio.tls.modo: archivos` con un certificado de prueba;
  4. iniciar sesión;
  5. una petición con el agente simulado;
  6. **guardar una clave con un trabajo largo en curso y comprobar que el trabajo termina** (Revisión 4);
  7. actualizar desde un paquete local;
  8. desinstalar.
- **`probar-servidor.ps1`:** la misma secuencia en el runner de Windows (Windows Server 2025), con `servidor instalar` desde el `.zip` portable.
- **`probar-proxies.sh`:** por cada proxy de Caddy, nginx, Apache, Traefik y HAProxy:
  - sin sesión, 401 en `/ui/…`;
  - el inicio de sesión funciona por el proxy;
  - el flujo SSE de un trabajo simulado llega en al menos 3 trozos separados en el tiempo;
  - el registro de Tramtai tiene la dirección del cliente;
  - un `X-Forwarded-For` enviado directamente al puerto de Tramtai, saltándose el proxy, no cambia la dirección registrada.

- [ ] **Paso 1:** escribir los ejemplos y los scripts.
- [ ] **Paso 2:** añadir los tres jobs a `ci.yml` (reutilizable, así que también corren desde Publicar). No pasan a ser obligatorios en el ruleset sin que lo decida el usuario.
- [ ] **Paso 3:** subir la rama y comprobar en el PR que los tres jobs pasan. Esperado: verde.
- [ ] **Paso 4:** commit `ci: servidor en Linux y Windows Server y ejemplos de proxies comprobados`.

### Tarea 18: revisión de seguridad

**Archivos:** los que salgan de la revisión, y `docs/equipo/releases/v0.58-servidor/revision-seguridad.md` (hallazgos y arreglos).

- [ ] **Paso 1:** lanzar un revisor de seguridad aparte (agente `ecc:security-reviewer` u opus) sobre el diff de la rama. Revisa la lista de spec «Revisión de seguridad» y, para cada punto, la prueba automática que lo cubre:
  - cabeceras reenviadas falsificadas;
  - Host y Origin;
  - fuerza bruta y frenos;
  - TLS y ACME (`HostPolicy`, caché);
  - cookie y CSRF;
  - redirecciones del inicio de sesión;
  - tiempos de respuesta;
  - agotamiento de recursos (argon2id, conexiones, flujos);
  - permisos de las carpetas y de la unidad;
  - la suscripción expuesta.
- [ ] **Paso 2:** para cada hallazgo, una prueba que falla y el arreglo (con TDD). Todos se arreglan en la versión, también los menores.
- [ ] **Paso 3:** la verificación global. Esperado: PASS.
- [ ] **Paso 4:** commit `fix(servidor): arreglos de la revisión de seguridad`.

### Tarea 19: documentación, Ayuda, sitio y capturas

**Archivos:**
- Crear: `docs/servidor.md`, `site/servidor.html` (generado como `site/desarrolladores.html`) y los temas `web/src/ayuda/temas/servidor.ts` y `dos-entornos.ts`.
- Cambiar:
  - `README.md` (modo servidor y el apartado «Aviso» con los cinco riesgos);
  - `docs/arquitectura.md` (tercera raíz, servicio del sistema, tabla de claves y modelo de amenazas del servidor);
  - `docs/operacion.md` (arranque con tres formas, servidor y dos entornos);
  - `docs/inicio.md` y `docs/desarrolladores.md` (URL `https://` en los SDK);
  - el tema de arranque de la Ayuda y la lista de teclas (`W`);
  - `web/src/idiomas/ayuda.{es,en}.ts`.
- Capturas: la Tarea 14 de `releases/v0.56-terminal/plan.md` (línea 543 en adelante; ajustar la nota del puerto 17388 frente al 17373) y, con `npm run capturas`, Ajustes → Arranque, Ajustes → Servidor y Seguridad, la página de inicio de sesión y la marca «Servidor».

- [ ] **Paso 1:** escribir la guía con los apartados de spec §11, en español y en inglés, enlazando los ejemplos de `docs/servidor/proxies/`. Los proxies comprobados a mano se marcan «comprobado en la v0.58» después de la Tarea 20.
- [ ] **Paso 2:** los temas de la Ayuda y el «Aviso» con `terminos.Riesgos`/`riesgos.*` (sin copiar el texto).
- [ ] **Paso 3:** `cd web && npm run sitio && npm run capturas && npm test && npm run build`; el CI compara `site/`. Esperado: PASS.
- [ ] **Paso 4:** marcar ✅ las filas de spec que correspondan en `docs/equipo/pendientes.md` solo al publicar (no aquí). Commit `docs: guía del servidor, Ayuda, sitio y capturas`.

### Tarea 20: cierre

- [ ] **Paso 1 (a mano):** en un Windows Server, IIS con `docs/servidor/proxies/iis/web.config`; Cloudflare Tunnel y Tailscale Serve con la cuenta del usuario (con su permiso). Anotar el resultado en la guía.
- [ ] **Paso 2 (a mano):** `testssl.sh` contra el HTTPS propio. Esperado: solo TLS 1.3, X25519MLKEM768 ofrecido y HSTS presente. Anotar la salida resumida en `revision-seguridad.md`.
- [ ] **Paso 3:** la revisión final de la rama con opus, como en las últimas versiones. Arreglar todos los hallazgos.
- [ ] **Paso 4:** las notas de la versión (español e inglés, cortas, para usuarios), con el cambio de comportamiento de `permitir_remoto` por HTTP y que la versión 2 de los términos se pide una vez. Se enseñan al usuario antes de publicar.
- [ ] **Paso 5:** publicar solo con la autorización del usuario.
