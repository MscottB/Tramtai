# Portable de verdad (v0.48): plan de implementación

**Objetivo:** que la portable de Windows guarde todo junto a su `.exe` y conviva con la instalada sin pisarse.

**Spec:** `docs/equipo/portable.md`, aprobada y revisada por arquitectura. **Rama:** `feat/portable`, con PR a `main`.

## Restricciones generales

- **Un solo punto para la raíz de usuario:** `internal/nucleo`. Nadie más llama a `os.UserConfigDir` para los datos de
  Tramtai. Se exceptúa `autoarranque_linux.go`, que lo usa para `autostart`, del sistema.
- **Modo normal intacto:** sin `portable.txt` todo sigue en `%APPDATA%\tramtai`, como hoy.
- **Textos nuevos:**
  - en `es.ts`/`en.ts`;
  - y en `internal/idioma/en_*.go`, que comprueba `TestCobertura`.
- **Comprobaciones:**
  - `go vet ./...` en Windows, con `GOOS=linux` y con `GOOS=darwin`;
  - `go test ./...`;
  - `go test -race` en `nucleo`, `servicio` y `autoarranque`;
  - `cd escritorio && go vet ./... && go test ./...`;
  - `cd web && npm run build && npm test`;
  - `graphify update .`.
- **Pruebas reales:**
  - solo con Claude haiku;
  - la portable y la instalada en este equipo, con datos de prueba aislados, nunca con los `%APPDATA%` reales del usuario.
- **Release:** PR, CI en verde, `gh pr merge`, etiqueta. Notas cortas y sin detalles internos.

## Dónde revisar con más cuidado

1. **`raizDe` (tarea 1):** nunca cae en `%APPDATA%` si hay `portable.txt` y algo falla, y calcula la raíz una sola vez
   por proceso.
2. **La DACL de `datos-tramtai` (tarea 1):**
   - solo el usuario actual y SYSTEM;
   - un error en FAT o exFAT no impide usar la portable.
3. **`instalacion abierto` y `cerrar` (tarea 4):** solo tocan procesos de la carpeta indicada; nunca la portable desde
   el instalador.
4. **El autoarranque (tarea 3):**
   - `Activo` solo es cierto para esta copia;
   - `Desactivar` nunca borra el valor de otra.

---

### Tarea 1: La raíz de usuario en `nucleo`

**Archivos:**
- nuevos: `internal/nucleo/raiz.go`, `raiz_test.go`, `permisos_windows.go`, `permisos_otros.go` y `permisos_windows_test.go`;
- cambia: `internal/nucleo/archivos.go` (`RutaGlobal`).

- [ ] `raizDe(ejecutable string) (raiz string, portable bool, err error)`:
  - resuelve `EvalSymlinks`, y si falla usa la ruta tal cual;
  - es portable si la carpeta tiene un `portable.txt` regular;
  - **sin él:** devuelve `UserConfigDir()/tramtai`;
  - **con él:**
    - si la carpeta está bajo `os.TempDir()`, da error;
    - si no, `MkdirAll(<carpeta>/datos-tramtai, 0o700)` y `protegerCarpeta`;
    - cualquier fallo es el error «Descomprime Tramtai en una carpeta donde puedas escribir» (con el detalle), sin
      caer en `%APPDATA%`.
- [ ] Las funciones públicas:
  - `RaizUsuario() (string, error)` y `Portable() bool`: `sync.OnceValues` sobre `raizDe(os.Executable())`;
  - `DirDatos() (string, error)`: `Join(raiz, "datos")` con `MkdirAll 0o700`;
  - `RutaGlobal`: primero `TRAMTAI_CONFIG_GLOBAL`, después `Join(raiz, "config.yaml")`.
- [ ] `protegerCarpeta(ruta)`:
  - **Windows:** `SetNamedSecurityInfo` con una DACL protegida desde SDDL `D:P(A;OICI;FA;;;<SID del usuario>)(A;OICI;FA;;;SY)`;
  - **resto:** `os.Chmod(0o700)`;
  - el error se ignora si el volumen no tiene ACL (FAT o exFAT): se comprueba con `GetVolumeInformation` y su
    `FILE_PERSISTENT_ACLS`.
- [ ] **Pruebas:**
  - sin `portable.txt`;
  - con él, la carpeta se crea;
  - un `portable.txt` que es una carpeta no cuenta;
  - un **archivo** `datos-tramtai` da el error;
  - una carpeta bajo `os.TempDir()` da el error;
  - en Windows, la DACL resultante tiene exactamente dos entradas, el usuario actual y SYSTEM, y está protegida
    (`SE_DACL_PROTECTED`).

### Tarea 2: Quien usa la raíz

**Archivos:** `cmd/tramtai/servicio.go` (y sus usos de `dirDatos`), `escritorio/main.go` y su prueba.

- [ ] **`dirDatos` de `cmd/tramtai`:** se borra y sus usos pasan a `nucleo.DirDatos()`.
- [ ] **`dirDatos` y `rutaPreferencias` del escritorio:** se borran;
  - los datos pasan a `nucleo.DirDatos()`;
  - `escritorio.json` a `Join(raiz, "escritorio.json")`.
- [ ] **El escritorio, antes de `application.New`:**
  - **`Windows.WebviewUserDataPath`:** si `nucleo.RaizUsuario()` da error, se usa una carpeta de `os.MkdirTemp` y,
    al arrancar, se muestra `paginaError(err)`. Si va bien y es portable, se usa `<raiz>\webview`; en modo normal, el
    valor predeterminado de Wails.
  - **`SingleInstance.UniqueID`:** `uniqueID(carpeta string, portable bool)`. En modo normal es `cl.tramtai.escritorio`;
    en portable, `cl.tramtai.escritorio.portable.p<8 hex>`, con el SHA-256 de la carpeta con `Clean` y, fuera de Linux,
    en minúsculas.
  - **El nombre visible:** en portable, el título de la ventana y el texto de la bandeja dicen «Tramtai portable».
- [ ] **`buscarTramtai`:** en portable no busca en el PATH y, sin el `tramtai.exe` de al lado, devuelve un error claro.
- [ ] **Pruebas:**
  - `uniqueID` es igual para `C:\X` y `c:\x`;
  - todas sus partes son válidas para D-Bus, también cuando el hash empieza por un dígito.

### Tarea 3: «Iniciar con Windows» por copia

**Archivos:** `internal/autoarranque/autoarranque_windows.go`, `autoarranque_archivo.go`, `autoarranque_linux.go`,
`autoarranque_darwin.go` y sus pruebas.

- [ ] **Windows:**
  - `nombreValor()` da `Tramtai` en la instalada y `Tramtai portable` en la portable (`nucleo.Portable()`); las
    pruebas lo sustituyen como hoy;
  - `Activo()` es cierto si el valor es el que escribiría `Activar(Escritorio())`, comparando con `EqualFold` y, si
    las dos rutas existen, con `os.SameFile`;
  - **portable de la 0.46:** en modo portable, un valor `Tramtai` que apunta a esta copia también cuenta; `Activar`
    lo borra y escribe el nuevo, y `Desactivar` quita los dos;
  - `Desactivar` solo borra los valores que apuntan a esta copia.
- [ ] **Linux y macOS:** `Activo` es cierto solo si el archivo de arranque contiene la ruta de esta copia escapada
  (como la escribe `contenido`).
- [ ] **Pruebas:**
  - otra ruta da falso y la propia, cierto, aunque cambien las mayúsculas;
  - el valor viejo de la portable se migra;
  - `Desactivar` no toca el valor de otra copia.

### Tarea 4: El instalador no toca la portable

**Archivos:**
- nuevos: `internal/instalacion/procesos_windows.go`, `procesos_otros.go` y `procesos_windows_test.go`;
- cambian: `cmd/tramtai/instalacion.go` y `scripts/instalador/tramtai.nsi`.

- [ ] **Funciones de `instalacion`:**
  - `ProcesosEn(carpeta, exe string) ([]uint32, error)`: recorre los procesos con `CreateToolhelp32Snapshot`
    (`TH32CS_SNAPPROCESS`), saca la ruta con `QueryFullProcessImageName` y se queda con los de `exe` cuya carpeta es
    `carpeta`, sin distinguir mayúsculas;
  - `Cerrar(carpeta, exe string) error`: `TerminateProcess` de esos procesos, ignorando los que no se pueden abrir;
  - fuera de Windows, `ProcesosEn` devuelve un error de «solo Windows».
- [ ] **Comandos ocultos:**
  - `tramtai instalacion abierto <carpeta> <exe>` sale con 0 si hay alguno y con 1 si no;
  - `tramtai instalacion cerrar <carpeta> <exe>`.
- [ ] **En `tramtai.nsi`:**
  - `HayProceso` pasa a `"${CLI}" instalacion abierto "$INSTDIR" <exe>` y los dos `taskkill` a
    `"${CLI}" instalacion cerrar "$INSTDIR" <exe>`;
  - `servicio detener --si` queda igual;
  - `SecPrograma` borra `$INSTDIR\portable.txt`;
  - al desinstalar, si se borran los datos, también se borra `$APPDATA\tramtai-escritorio.exe`.
- [ ] **Pruebas:**
  - el proceso de la propia prueba aparece con su carpeta;
  - con otra carpeta, no aparece.

### Tarea 5: Servicio

**Archivos:** `internal/servicio/web.go`, `acerca.go` y sus pruebas.

- [ ] **Reiniciar desde la Web UI:** devuelve el puerto configurado solo si es distinto del actual y está libre
  (`PuertoLibre`); si no, el actual.
- [ ] **Cookie de sesión:** se llama `tramtai_sesion_<puerto>`. Se revisan todos los usos de `cookieSesion`.
- [ ] **`acerca()`:** añade `"portable": nucleo.Portable()`.
- [ ] **Pruebas:**
  - el reinicio devuelve el puerto actual cuando el configurado está ocupado;
  - la cookie lleva el puerto.

### Tarea 6: Empaquetado y CI

**Archivos:** nuevo `scripts/portable.txt`; cambian `scripts/compilar.ps1` y `.github/workflows/publicar.yml`.

- [ ] **`scripts/portable.txt`** (es/en): para qué sirve, dónde quedan los datos y que borrarlo con Tramtai cerrado
  vuelve a usar `%APPDATA%`.
- [ ] **`compilar.ps1`:** lo copia al `.zip`.
- [ ] **Publicar, job `windows`:** después de compilar, una orden por paso:
  - descomprimir el `.zip` en `$RUNNER_TEMP\portable`;
  - comprobar que trae `portable.txt`;
  - `tramtai.exe servicio iniciar -d`;
  - `servicio estado`;
  - `servicio detener --si`;
  - comprobar que existe `datos-tramtai\datos\servicio.json` y que no existe `$env:APPDATA\tramtai`.

### Tarea 7: Ayuda de la CLI

**Archivos:** `cmd/tramtai/main.go`, `ejecutar.go` e `internal/idioma/en_cli.go`.

- [ ] La lista de agentes de `--help` y de `--agente` sale del catálogo (`agentes.Todos()`), así que GitHub Copilot ya
  no falta y no volverá a faltar con los agentes nuevos.
- [ ] Se revisan las demás listas fijas de agentes de la CLI.

### Tarea 8: Web UI

**Archivos:** `web/src/componentes/AcercaDe.vue`, `api.ts` (tipo) e `idiomas/*.ts`.

- [ ] Acerca de muestra «Portable» junto a la versión cuando `portable` es cierto.

### Tarea 9: Documentación y notas

- [ ] **Dónde se explica:** Ayuda de la Web UI, `site/ayuda.html` (es/en), README, `docs/inicio.md`, `docs/operacion.md`
  y `docs/arquitectura.md`.
- [ ] **Qué se explica:**
  - dónde quedan los datos;
  - las claves compartidas;
  - cómo pasar los datos de la instalada;
  - qué pasa al actualizar;
  - qué hacer quien ya usa la portable 0.46;
  - FAT y exFAT;
  - la CLI desde su carpeta;
  - borrar `portable.txt` con Tramtai cerrado.
- [ ] **`pendientes.md`:** la v0.48 hecha.
- [ ] **Notas `notas-048-es.txt` / `-en.txt`:** cortas, con el aviso para quien ya usa la portable.

### Tarea 10: Verificación y release

- [ ] Todas las comprobaciones, en verde.
- [ ] **A mano, con haiku:**
  - la portable compilada en una carpeta de prueba y la instalada a la vez, cada una con su servicio y su ventana;
  - reiniciar desde la Web UI de la portable;
  - «Iniciar con Windows» en una sin que se marque la otra;
  - el instalador compilado con NSIS sobre la instalada con la portable abierta, sin aviso y sin cerrarla. Si NSIS no
    está, se prueban los comandos `instalacion abierto` y `cerrar`.
- [ ] Revisión de la rama con `ecc:code-reviewer` (opus), y se arregla todo.
- [ ] PR, CI en verde, `gh pr merge --merge --delete-branch`, `git pull`, etiqueta con las notas, Publicar, y
  comprobar el `.zip` portable del release.
