# Portable de verdad (v0.48)

Diseño del 2026-10-19, revisado por arquitectura el mismo día. Hoy el `.zip` portable de Windows trae la app y el
servicio sin instalar, pero guarda todo donde la instalada: configuración, historial y tokens en `%APPDATA%\tramtai`, y
el perfil de WebView2 (~13 MB) en `%APPDATA%\tramtai-escritorio.exe`. Por eso, en un equipo con las dos, comparten datos,
servicio y puerto, y llevada a otro equipo empieza de cero.

Decisiones del usuario:
- **La portable guarda sus datos junto al `.exe`.**
- **En este release también:**
  - «Iniciar con Windows» solo se marca si apunta a esta copia;
  - y la ayuda de la CLI nombra a GitHub Copilot.

Va después de las correcciones de la revisión (v0.47, `correcciones.md`). OpenCode pasa a la v0.50 y Hermes Agent a la v0.51.

Comprobado en la v0.46.1, con la portable aislada en un `%APPDATA%` vacío:
- arranca el `tramtai.exe` de su carpeta;
- abre la Web UI;
- completa una ejecución real (Claude haiku);
- y muestra el aviso de actualización manual.

Lo que cambia es dónde guarda los datos.

Fuera de alcance:
- **Portable en Linux y macOS:** allí no hay `.zip` portable. El código del modo funciona en cualquier sistema, pero solo
  se publica en Windows.
- **Importar los datos de la instalada con un botón:** basta con copiar la carpeta (§7).
- **Actualizar la portable sola:** sigue con el aviso y el enlace de descarga.

## 1. Cómo sabe que es portable

El `.zip` trae un archivo `portable.txt` junto a `tramtai.exe` y `tramtai-escritorio.exe`. Lo copia
`scripts/compilar.ps1`. El archivo explica en español y en inglés:
- para qué sirve;
- que borrarlo (con Tramtai cerrado) hace que esa copia vuelva a usar `%APPDATA%`.

**Cuándo es portable:** si la carpeta del ejecutable tiene un `portable.txt` que es un archivo regular. La carpeta es la
de `os.Executable` con `EvalSymlinks`; si `EvalSymlinks` falla, se usa la ruta sin resolver, como en
`actualizacion.Instalada`.

**Por qué un archivo y no otra señal:**
- no sirve la ausencia de `desinstalar.exe`, porque una copia compilada a mano no es portable;
- `go run` y las pruebas corren en carpetas temporales sin `portable.txt`, así que quedan en modo normal;
- la CLI que el instalador copia a `$PLUGINSDIR` tampoco lo tiene, así que detiene la instalada, que es lo que se quiere.

## 2. Dónde quedan los datos

En modo portable, la raíz de usuario pasa de `%APPDATA%\tramtai` a `<carpeta>\datos-tramtai`, con la misma estructura de
dentro:

| Hoy | Portable |
|---|---|
| `%APPDATA%\tramtai\config.yaml` y `profiles\` | `<carpeta>\datos-tramtai\config.yaml` y `profiles\` |
| `%APPDATA%\tramtai\datos\` (servicio.json, tokens, instancias, cli.db, registros, cargas) | `<carpeta>\datos-tramtai\datos\` |
| `%APPDATA%\tramtai\escritorio.json` | `<carpeta>\datos-tramtai\escritorio.json` |
| `%APPDATA%\tramtai-escritorio.exe\EBWebView` | `<carpeta>\datos-tramtai\webview\` (`Options.Windows.WebviewUserDataPath`) |

### Un solo punto en `internal/nucleo`

`nucleo` solo depende de `idioma`; `autoarranque`, `servicio`, `cliente` y `escritorio` pueden importarlo sin ciclos.

- **`raizDe(ejecutable string) (raiz string, portable bool, err error)`:** función pura y sin estado. Las pruebas la
  llaman directamente, con carpetas temporales.
  - **Sin `portable.txt`:** devuelve `UserConfigDir()\tramtai`.
  - **Con `portable.txt`:**
    - devuelve `<carpeta>\datos-tramtai`;
    - hace el `MkdirAll` y, en Windows, la DACL del §3;
    - si algo falla, devuelve el error «Descomprime Tramtai en una carpeta donde puedas escribir», sin volver a
      `%APPDATA%` para no mezclar los datos de dos copias.
  - **Carpeta temporal:** si la carpeta está bajo `os.TempDir()`, da el mismo error. Es el caso del Explorador cuando
    abre el `.zip` sin descomprimir: extrae a `%TEMP%\Temp1_….zip\` y esos datos se borrarían.
- **`RaizUsuario() (string, error)` y `Portable() bool`:** calculan `raizDe(os.Executable())` una sola vez por proceso
  (`sync.OnceValues`). Así un proceso no cambia de raíz a mitad de vida, y `repasar` deja de hacer un `Stat` cada 3 s.
- **`DirDatos() (string, error)`:** devuelve `Join(raiz, "datos")` con `MkdirAll 0o700`. Sustituye a las dos `dirDatos`,
  la de `cmd/tramtai/servicio.go` y la de `escritorio/main.go`, que se borran. La del escritorio no creaba la carpeta.
- **`RutaGlobal`:** primero `TRAMTAI_CONFIG_GLOBAL` (que mueve `config.yaml` y `profiles\`, como hoy) y después
  `Join(raiz, "config.yaml")`.
- **`rutaPreferencias` del escritorio:** pasa a ser `Join(raiz, "escritorio.json")`.

**Lo que no cambia:**
- Todo lo que hoy sale de `dirDatos` o `RutaGlobal` sigue igual y se mueve solo:
  - `cli.db` y `cargas\cli` de `ejecutar --directo`;
  - las rutas de Acerca de;
  - la TUI;
  - `cliente.Lanzar` e `IniciarEnSegundoPlano`.
- `autoarranque_linux.go` sigue usando `UserConfigDir` para `autostart`, que es del sistema y no de Tramtai.

### El error en la app de escritorio

Wails cierra la app con su propio cuadro si `WebviewUserDataPath` no es válido. Por eso el escritorio resuelve la raíz
**antes** de `application.New`:
- si va bien, usa `<raiz>\webview`;
- si da error, usa una carpeta de `os.MkdirTemp` y muestra `paginaError(err)`, como hoy cuando falla la conexión.

**Sin el `tramtai.exe` de al lado:** en modo portable, `buscarTramtai` no lo busca en el PATH y devuelve un error. Si lo
buscara, lanzaría la CLI instalada, que escribe en `%APPDATA%`, mientras la app la espera en `datos-tramtai`.

**Los agentes no notan nada:** Tramtai no cambia `APPDATA` ni otras variables de su entorno. Claude Code, Codex, GitHub
Copilot y Antigravity siguen usando sus sesiones de siempre.

## 3. Seguridad de la carpeta

`%APPDATA%` es privado del usuario, pero una carpeta portable hereda los permisos de su carpeta padre. Por ejemplo, en
`C:\Tramtai` los «Usuarios autenticados» pueden modificar. Dentro está `token-servicio`, que administra el servicio.

- **Permisos:** al crear `datos-tramtai` en Windows, se le pone una DACL protegida (sin herencia) que solo da acceso al
  usuario actual y a SYSTEM. Se hace con SDDL y `SetNamedSecurityInfo` de `golang.org/x/sys/windows`.
- **Si falla:** cuando el sistema de archivos no tiene ACL (FAT o exFAT de un pendrive), el error no impide usar la
  portable. La Ayuda avisa de que, en esos pendrives, los tokens quedan legibles para quien tenga el pendrive.
- **Claves de API:** se quedan en el Administrador de credenciales de Windows.
  - **Por qué:** son secretos de tu usuario de Windows, no de la carpeta. Dejarlas en un archivo junto al `.exe` las
    expondría a quien copie el pendrive, y cifrarlas con DPAPI las ataría igualmente al equipo.
  - **En el mismo equipo:** instalada y portable comparten las claves, y `tramtai claves borrar` en una las borra para
    las dos.
  - **En otro equipo:** hay que volver a guardarlas, con `tramtai claves guardar` o en Ajustes.
  - La Ayuda lo dice.

## 4. Instalada y portable a la vez

Con datos separados son dos Tramtai independientes.

- **Servicios:**
  - cada copia tiene su `servicio.json` y su token;
  - si el puerto está ocupado, la segunda usa otro libre, como hoy con `puerto_automatico`;
  - las APIs con puerto fijo que choquen dan el error de puerto ocupado de siempre.
- **Reiniciar desde la Web UI:**
  - **Problema:** hoy `/ui/servicio/reiniciar` devuelve el puerto configurado (`web.go`). Si ahí escucha la otra copia,
    el navegador salta a ella.
  - **Cambio:** devuelve el puerto configurado solo si es distinto del actual y está libre; si no, el actual. El servicio
    nuevo, en cambio, vuelve a buscar en orden y cae otra vez en el actual.
- **Cookie de sesión:** `tramtai_sesion` vale para todo `127.0.0.1`, sin distinguir puerto, así que dos Web UIs en el
  mismo navegador se echarían la una a la otra. El nombre lleva el puerto (`tramtai_sesion_<puerto>`). Las sesiones
  viven en memoria, así que no se pierde nada.
- **App de escritorio:**
  - `SingleInstance.UniqueID` sigue siendo `cl.tramtai.escritorio` en la instalada, y en la portable pasa a
    `cl.tramtai.escritorio.portable.p<8 hex>`. Así, abrir una no trae al frente a la otra.
  - El hash es el SHA-256 de la carpeta con `Clean` y, fuera de Linux, en minúsculas.
  - La `p` evita que la parte empiece por un dígito, que D-Bus rechaza en Linux.
- **Cómo se distinguen:** el título de la ventana y el texto de la bandeja dicen «Tramtai portable».
- **Instalador y desinstalador:**
  - **Problema:** hoy `DetenerTramtai`:
    - detecta los procesos por nombre (`tasklist`), así que una portable abierta hace saltar el aviso, agota la espera
      de 10 s y hace que al final se abra la instalada;
    - y luego cierra a la fuerza todo `tramtai.exe` del usuario, lo que mataría sin drenaje un servicio portable.
  - **Cambio:** dos comandos ocultos de la CLI, junto a `instalacion agregar-ruta`:
    - `tramtai instalacion abierto <carpeta> <exe>` sale con 0 si hay un proceso de ese nombre dentro de la carpeta y
      sustituye a `HayProceso`;
    - `tramtai instalacion cerrar <carpeta> <exe>` cierra solo esos y sustituye a los dos `taskkill`.
  - **Cómo funcionan:**
    - recorren los procesos con `CreateToolhelp32Snapshot` y sacan su ruta con `QueryFullProcessImageName`;
    - comparan las rutas con `dentroDe`, que ya ignora las mayúsculas fuera de Linux;
    - terminan los procesos con `TerminateProcess`, como `internal/procesos`;
    - ignoran los procesos que no pueden abrir.
  - **Quién los ejecuta:** el `$PLUGINSDIR\tramtai.exe` nuevo al instalar y `$INSTDIR\tramtai.exe` al desinstalar.
  - **Lo que no cambia:** la parada ordenada `servicio detener --si` del servicio de `%APPDATA%` sigue igual, así que las
    portables antiguas, sin `portable.txt`, se detienen bien como hoy.
  - No se usa PowerShell con CIM, porque es lento en un bucle, depende de WMI, se rompe con rutas que llevan `'` y hay
    políticas que lo bloquean.
- **Desinstalar:**
  - si el usuario elige borrar los datos, también se borra `%APPDATA%\tramtai-escritorio.exe`, el perfil de WebView2 que
    hoy queda huérfano;
  - `SecPrograma` borra `$INSTDIR\portable.txt`, por si alguien descomprimió la portable en la carpeta de instalación.
- **CLI en la terminal:** en una terminal, `tramtai` es el del PATH, es decir, la instalada.
  - Para hablar con la portable, se usa `.\tramtai.exe` desde su carpeta.
  - `cmd.exe` busca primero en la carpeta actual y PowerShell no.
  - La Ayuda lo explica.

## 5. «Iniciar con Windows»

**Problema:** hoy `autoarranque.Activo()` solo mira si existe el valor `Tramtai` en
`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. Por eso la portable y la instalada se pisan el valor y las dos se
ven marcadas.

**Nombre del valor:**
- es `Tramtai` en la instalada y `Tramtai portable` en la portable;
- activar en una no toca la otra;
- un solo nombre para todas las portables evita valores huérfanos al mover la carpeta. A cambio, dos portables distintas
  se pisan; es un caso raro y se acepta.

**Cuándo se ve marcado:**
- `Activo()` es verdadero solo si el valor guardado es exactamente lo que escribiría `Activar` para esta copia
  (`"<ruta>" --oculto`), comparado con `strings.EqualFold`;
- si las dos rutas existen, además con `os.SameFile`;
- si la carpeta portable se movió, la casilla aparece desmarcada y, al marcarla, el valor se reescribe con la ruta nueva.

**`Desactivar`:** solo borra el valor si apunta a esta copia. El desinstalador llama a `servicio autoarranque desactivar`
de la instalada, así que solo quita el suyo.

**Portables de la 0.46:** esas copias escribieron el valor `Tramtai`. En modo portable:
- `Activo` también reconoce el valor `Tramtai` si apunta a esta copia;
- `Activar` lo borra y escribe `Tramtai portable`;
- y `Desactivar` quita los dos.

**Linux y macOS:** se aplica la misma regla. `Activo` es verdadero solo si el archivo de arranque contiene la ruta de
esta copia entre comillas.

La bandeja, Ajustes y `tramtai servicio autoarranque` usan las mismas funciones; no hay cambios en la Web UI.

## 6. Ayuda de la CLI

- **`tramtai --help`** y **`tramtai ejecutar --agente`** sacan la lista del catálogo de agentes: los programas («Claude
  Code, Codex CLI, GitHub Copilot CLI, Gemini API y Antigravity CLI») y los identificadores («claude, codex, copilot,
  gemini o antigravity»). Así no falta ninguno ni faltará con los agentes nuevos.
- El texto de bienvenida del instalador también nombra a GitHub Copilot.

## 7. Acerca de, migración y documentación

- **Acerca de:**
  - `acerca()` añade `"portable": true|false` y la Web UI muestra «Portable» junto a la versión;
  - las rutas ya van en `rutas` solo a una sesión local, y ahí la carpeta de datos ya dice cuál se está usando;
  - en `/api/v1/instancia`, que leen las aplicaciones con token de instancia, no se añade nada.
- **Pasar a la portable lo que ya tienes:**
  - con las dos copias cerradas, se copia el contenido de `%APPDATA%\tramtai` en `datos-tramtai`;
  - la estructura es la misma, así que se conservan configuración, APIs, tokens e historial;
  - si el `config.yaml` copiado tiene `puerto_automatico: false` y la instalada usa ese puerto, la portable dirá
    «puerto ocupado»: hay que cambiar uno de los dos.
- **Quien ya usa la portable 0.46:** al descomprimir la 0.47 encima, la copia empieza con datos vacíos y tokens nuevos,
  así que sus aplicaciones conectadas necesitan el token nuevo. Para conservarlo todo, hay que copiar `%APPDATA%\tramtai`
  como en el punto anterior. Lo avisan las notas de versión y la Ayuda.
- **Actualizar la portable:**
  - se descomprime el `.zip` nuevo sobre la carpeta;
  - `datos-tramtai` no viene en el `.zip`, así que no se pisa.
- **Dónde se explica:**
  - Ayuda de la Web UI;
  - `site/ayuda.html` (es/en);
  - README;
  - `docs/inicio.md`;
  - `docs/operacion.md`;
  - `docs/arquitectura.md`.

## 8. Pruebas

- **`nucleo.raizDe`:** con carpetas temporales:
  - sin `portable.txt` devuelve `UserConfigDir()\tramtai`;
  - con el archivo devuelve `<carpeta>\datos-tramtai` y lo crea;
  - un `portable.txt` que es una carpeta no cuenta;
  - un **archivo** llamado `datos-tramtai` hace fallar el `MkdirAll` y da el error sin caer en `%APPDATA%`. Se usa un
    archivo porque en Windows el atributo de solo lectura de una carpeta no impide crear archivos;
  - una carpeta bajo `os.TempDir()` da el error.
- **DACL (Windows):** después de crear `datos-tramtai`, su descriptor está protegido y solo tiene al usuario actual y a
  SYSTEM.
- **`autoarranque`:** en Windows, con nombres de valor propios de la prueba, como hoy:
  - `Activo` es falso con otra ruta y verdadero con la propia, aunque cambien las mayúsculas;
  - el valor antiguo `Tramtai` de una portable se migra;
  - `Desactivar` no borra el valor de otra copia.

  En Linux y macOS, la misma regla con el archivo de arranque.
- **`instalacion abierto` y `cerrar`:** el filtro por carpeta acepta un proceso de esta carpeta (el de la propia prueba)
  y rechaza uno de otra.
- **Servicio:**
  - el reinicio devuelve el puerto actual cuando el configurado está ocupado;
  - la cookie lleva el puerto.
- **Escritorio:** el `UniqueID` es válido para D-Bus (también cuando el hash empieza por un dígito) y es igual para
  `C:\X` y `c:\x`.
- **CI (Publicar, job `windows`):** después de armar el `.zip`, una orden por paso, como exige pwsh:
  - se descomprime en una carpeta temporal;
  - se comprueba que trae `portable.txt`;
  - se ejecutan `tramtai.exe servicio iniciar -d`, `servicio estado` y `servicio detener --si`;
  - debe existir `datos-tramtai\datos\servicio.json` y no debe haberse creado `%APPDATA%\tramtai`.
- **A mano, con haiku:**
  - la portable y la instalada a la vez, cada una con su servicio y su ventana;
  - reiniciar desde la Web UI de la portable sin que salte a la instalada;
  - «Iniciar con Windows» marcado solo en la que lo activó;
  - reinstalar con la portable abierta, sin aviso y sin que se cierre.
