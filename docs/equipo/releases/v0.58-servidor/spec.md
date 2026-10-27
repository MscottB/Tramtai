# v0.58: modo servidor

Diseñado con el usuario el 2026-10-26 (brainstorming por secciones). Parte de lo investigado el 2026-10-20 en
`docs/equipo/pendientes.md` y de la Tarea 13 y la Tarea 14 de la v0.56 (`releases/v0.56-terminal/plan.md`), que pasaron
a esta versión.

## Por qué

Hoy Tramtai ya corre sin escritorio (`tramtai servicio iniciar -d`) y tiene el acceso remoto apagado
(`servicio.permitir_remoto`). Pero solo arranca cuando el usuario inicia sesión, en Linux sin sesión no hay almacén de
claves, y abrirlo a la red sin HTTPS haría que las claves viajaran en claro. Esta versión lo convierte en un servidor de
verdad para Linux y Windows Server, sin bajar la seguridad.

## Lo decidido

- **Alcance:** sin sesión iniciada y servidor personal, con **un solo administrador**. Varias cuentas, permisos por
  persona, inicio de sesión con la cuenta de la empresa (OIDC) y registro de quién cambió qué quedan para la Pro/Team.
- **Enfoque:** un Tramtai del sistema, **aparte** del Tramtai de escritorio del usuario: cuenta propia sin privilegios,
  datos en una carpeta del sistema y su propio puerto. Una imagen de contenedor, más adelante, encima de esto.
- **También en esta versión:** «Solo el servicio» (T13 de la v0.56), las capturas de la terminal en la portada (T14) y
  una tecla en la terminal para abrir la Web UI.
- **HTTPS de tres formas:** detrás de un proxy (la recomendada), certificado propio y Let's Encrypt integrado
  (opcional). Solo TLS 1.3.
- **La Web UI del servidor** se abre desde cualquier navegador con la contraseña del administrador (TOTP opcional,
  obligatorio con suscripción personal en remoto).
- **Suscripción:** con clave de API por defecto; la «suscripción personal» se permite también en remoto, con las
  condiciones de la sección 5, y una petición a la vez por agente por defecto (hasta 3).
- **Dos entornos:** tu Tramtai y el del servidor no comparten nada, y eso se dice en el sistema, la documentación y el
  sitio.
- **Linux:** se instala desde el `.tar.gz` que ya se publica con `tramtai servidor instalar`, o con los paquetes
  `.deb` y `.rpm` del servidor (`tramtai-servidor`, sin la app de escritorio), nuevos en esta versión.
- **Descargo de responsabilidad:** `TERMINOS.md` versión 2 (borrador revisado en el anexo B), con ley chilena y
  tribunales de Santiago, sin quitar las protecciones imperativas del país de cada usuario. Se acepta de forma expresa
  y se muestra al instalar, al activar la suscripción y en el README, el repositorio, el sitio y la Ayuda. Vale para
  toda la aplicación: también el escritorio la pide una vez al abrir la Web UI, la app o la terminal. Los riesgos se le
  dicen claro al usuario (sección 10).
- **macOS:** solo «Solo el servicio»; no tiene servicio del sistema en esta versión.
- Todo lo nuevo, en español y en inglés.
- **Los comandos nuevos nacen con su par en inglés** (alias de cobra; los dos funcionan siempre):
  `servidor`/`server` (`instalar`/`install`, `desinstalar`/`uninstall`, `estado`/`status`, `reiniciar`/`restart`,
  `registro`/`log`, `actualizar`/`update`), `administrador`/`admin` (`contraseña`/`password`, `totp`), `terminos`/`terms`
  (`aceptar`/`accept`), `agentes entrar`/`agents login`, `agentes suscripcion`/`agents subscription`,
  `claves guardar`/`keys set` y las opciones `--servidor`/`--server`, `--acepto-terminos`/`--accept-terms`,
  `--borrar-datos`/`--delete-data` y `--personal`. `tramtai-servidor` no lleva alias: es el nombre de un programa. Los
  alias en inglés de los comandos de antes son de la v0.62.

## Lo que cambia

### 1. Tres formas de arrancar

| Forma | Quién la arranca | Cuenta | Datos | Ventana y bandeja |
|---|---|---|---|---|
| La app en la bandeja (hoy) | el inicio de sesión | la del usuario | su carpeta de Tramtai | sí |
| **Solo el servicio** | el inicio de sesión | la del usuario | su carpeta de Tramtai | no |
| **Servicio del sistema** | el arranque del equipo | `tramtai` / `NT SERVICE\Tramtai` | `/var/lib/tramtai` · `%ProgramData%\Tramtai` | no |

- **«Solo el servicio»** sigue el diseño de la Tarea 13 de la v0.56:
  - `autoarranque.Elegir(modo)` con `ModoApp` y `ModoServicio`, y `autoarranque.Configurado()`;
  - Windows: `"<tramtai-escritorio.exe>" --solo-servicio` en `HKCU\…\Run`, que arranca el servicio en segundo plano y
    termina sin abrir ventana ni bandeja;
  - macOS: el LaunchAgent con `ProgramArguments [<tramtai>, servicio, iniciar, -d]`;
  - Linux: el `.desktop` con `Exec="<tramtai>" servicio iniciar -d`;
  - `tramtai servicio autoarranque activar --modo app|servicio` y `/ui/autoarranque` con `{activo, modo}`.
- **Ajustes → Arranque con el sistema** (Web UI y terminal) ofrece: «No arrancar», «La app en la bandeja» y «Solo el
  servicio». Debajo: «¿Sin iniciar sesión? Eso es el servicio del sistema, otro entorno», con la orden que lo instala
  y el enlace a la guía. No se instala desde la Web UI: necesita ser administrador.

### 2. El servicio del sistema

- **Una tercera raíz en `internal/carpetas`.** Además de la del usuario y la portable, la del servidor:
  `/var/lib/tramtai` en Linux y `%ProgramData%\Tramtai` en Windows. Se elige con la opción global `--servidor` de la CLI
  o con `TRAMTAI_SERVIDOR=1`, que ponen la unidad y el servicio. Dentro, lo de siempre (`config.yaml`, `datos/`, el
  registro y el conversor): el servicio no cambia por dentro, solo dónde vive.
- **La carpeta** solo la usan la cuenta del servicio y los administradores: 0700 con dueño `tramtai` en Linux; en
  Windows, una DACL con `SYSTEM`, `Administradores` y `NT SERVICE\Tramtai`.
- **`tramtai servidor`** (pide ser administrador; si no, lo dice y no hace nada):
  - `instalar`: sección 9;
  - `desinstalar [--borrar-datos]`;
  - `estado`, `reiniciar`, `registro [-f]` y `actualizar`.
- **Linux:** `/etc/systemd/system/tramtai.service` generado por Tramtai:
  - `User=tramtai`, `Group=tramtai`, `Environment=TRAMTAI_SERVIDOR=1`, `StateDirectory=tramtai`,
    `ExecStart=/opt/tramtai/tramtai servicio iniciar --registro /var/lib/tramtai/datos/servicio.log`;
  - `Restart=on-failure` y el plazo de parada mayor que `servicio.plazo_drenaje_segundos`;
  - endurecida: `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`, `PrivateDevices`,
    `ReadWritePaths` solo en su carpeta, `RestrictSUIDSGID`, `LockPersonality`. Sin `MemoryDenyWriteExecute`, que rompe
    el JIT de Node (Claude Code y Copilot son de Node; Codex es un binario nativo);
  - `AmbientCapabilities=CAP_NET_BIND_SERVICE` solo con Let's Encrypt o un puerto menor de 1024.
- **Windows:** un servicio con `golang.org/x/sys/windows/svc` (ya es dependencia):
  - nombre `Tramtai`, nombre visible «Tramtai (servidor)», cuenta virtual `NT SERVICE\Tramtai`;
  - arranque automático retrasado, y reinicio si se cae;
  - detener el servicio drena como `tramtai servicio detener`;
  - los Job Objects de los agentes funcionan igual dentro del servicio.
- **Administrar desde el propio servidor:** con **`tramtai-servidor <orden>`**, que equivale a `tramtai --servidor
  <orden>` con el programa del servidor: `sudo tramtai-servidor …` en Linux y una consola de administrador en Windows.
  Lo mismo con la terminal (`tramtai-servidor` sin más). Es un nombre aparte para no tapar el `tramtai` de escritorio,
  que puede ser de otra versión.
  - En Linux, `/usr/local/bin/tramtai-servidor` (o `/usr/bin/tramtai-servidor` con el paquete) lanza
    `/opt/tramtai/tramtai --servidor`; en Windows, `tramtai-servidor.cmd` en la carpeta del programa, que se añade al
    PATH del sistema.
  - La CLI lee el descriptor y `token-servicio` de la carpeta del servidor y, con `--servidor`, acepta un servicio que
    corre con la cuenta del servidor (hoy rechaza los de otro usuario).
  - **Con `--servidor`, la CLI nunca detiene ni sustituye el servicio del sistema** por ser de otra versión (lo que hoy
    hace con un servicio anterior del usuario): avisa de la diferencia y remite a `tramtai-servidor actualizar`.
- **Convivencia:** el servicio del sistema tiene su puerto (17373 o el siguiente libre si el del escritorio lo ocupa),
  su descriptor y su cerrojo. La CLI sin `--servidor` sigue hablando con el Tramtai del usuario, como la portable
  desde la v0.48.
- **Los CLI de los agentes** tienen que estar instalados para todo el equipo (p. ej. `npm install -g`), o con su ruta en
  `comandos_agentes.<agente>`. Su configuración y su sesión van en la carpeta de la cuenta del servicio, nunca en la del
  usuario.

### 3. Dos entornos

Tu Tramtai y el del servidor son **entornos distintos**: cada uno con sus espacios, APIs, claves, sesiones de los CLI,
historial y ajustes. Nada pasa de uno a otro. Se dice en todas partes:

- **Web UI:** una marca fija en la cabecera, «Servidor» o «Este equipo».
- **Terminal:** la misma marca en la línea de estado.
- **Acerca de:** entorno, cuenta y carpeta.
- **CLI:** con `--servidor`, cada salida que crea o cambia algo empieza por «Servidor:».
- **Aviso cruzado:** si en el mismo equipo hay un Tramtai del sistema, la app, la terminal y `tramtai servicio estado`
  del usuario lo avisan: «Hay otro Tramtai, del sistema, en el puerto X: es otro entorno, con sus propias APIs y
  claves».
- **Documentación y sitio:** el apartado «Dos entornos», con la tabla de qué es de cada uno, en la guía del servidor, la
  Ayuda y `docs/operacion.md`.

### 4. Las claves del servidor

Tramtai sigue sin guardar claves en claro.

| Clave | Windows Server | Linux |
|---|---|---|
| De proveedor y token de GitHub para Copilot | Administrador de credenciales de la cuenta del servicio (DPAPI, sin sesión) | credenciales cifradas de systemd (`systemd-creds`, con TPM2 si lo hay), leídas de `$CREDENTIALS_DIRECTORY` |
| Secreto TOTP del administrador | igual | igual |
| Contraseña del administrador y códigos de recuperación | solo su huella argon2id en `datos/` | igual |
| De los espacios y de las APIs | solo su huella `sha256:` (v0.57) | igual |
| `token-servicio` | archivo en `datos/`, que solo leen la cuenta del servicio y los administradores | igual (0600, dueño `tramtai`) |

- **Guardar:** `tramtai --servidor claves guardar <NOMBRE>` pide el valor sin mostrarlo. También desde la terminal del
  servidor.
  - **Windows:** la CLI se la entrega al servicio por la API de administración (loopback y `token-servicio`) y el
    servicio la escribe en su propio almacén. Vale sin reiniciar, como hoy.
  - **Linux:** la CLI la cifra con `systemd-creds encrypt --name=tramtai.<NOMBRE>` en
    `/etc/credstore.encrypted/tramtai.<NOMBRE>`, añade su `LoadCredentialEncrypted=` en
    `/etc/systemd/system/tramtai.service.d/claves.conf` y reinicia el servicio con drenaje (systemd entrega las
    credenciales al arrancar).
  - **Sin `systemd-creds`** (systemd anterior a la 250): `/etc/tramtai/claves.env` de root, 0600, cargado con
    `EnvironmentFile=`. Tramtai avisa al guardarla y en `servidor estado` de que esa clave está en claro en el disco.
- **`claves borrar` y `claves lista`** funcionan igual. La lista nunca muestra valores.
- **No se guardan desde la Web UI**, como se decidió el 2026-10-15. La Web UI dice qué clave falta y qué orden la pone.
- **`internal/credenciales` en Linux** lee primero `$CREDENTIALS_DIRECTORY/tramtai.<NOMBRE>`; si no está, Secret
  Service y la variable de entorno, como hoy.

### 5. Suscripción en el servidor

- **Por defecto,** clave de API.
- **Lo que no se hace nunca:** guardar en Tramtai un token de sesión de Claude (p. ej. el de `claude setup-token`).
  Anthropic prohíbe que un tercero «recoja, guarde o intermedie» credenciales o tokens de sesión de Claude.ai y exige
  que el inicio de sesión pase por su propio flujo
  ([Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)).
- **El inicio de sesión lo hace siempre el CLI**, con su propio flujo y con la cuenta del servicio:
  `tramtai --servidor agentes entrar <agente>` lo lanza así.
  - **Claude:** `claude auth login` (en Linux, como `sudo -u tramtai -H`).
  - **Codex:** `codex login --device-auth`, con su `CODEX_HOME` privado en la carpeta del servidor.
  - **Copilot:** su propio inicio de sesión por dispositivo, o un token de grano fino con «Copilot Requests» guardado con
    `claves guardar COPILOT_GITHUB_TOKEN`, como lo documenta GitHub.
  - Tramtai no lee ni copia lo que guarde el CLI.
- **Suscripción en local** (el servicio solo en loopback y `servicio.proxy.confiables` vacío): se activa como hoy.
- **Suscripción personal en remoto** (el servicio expuesto, sección 6):
  - se activa por agente con `tramtai --servidor agentes suscripcion <agente> --personal`, que antes muestra las
    condiciones (solo la usa el titular de la cuenta, no se reparten las claves de las APIs que usan ese agente, uso
    ordinario), los enlaces a los términos del proveedor y el riesgo de suspensión, y pide confirmarlo;
  - queda en `adaptadores.<agente>.suscripcion_personal: true`, una **clave de seguridad** (no se acepta en el
    `local.yaml` de un proyecto ni desde una sesión remota de la Web UI);
  - exige administrador con **TOTP**: sin él, el servicio no arranca con esa combinación y lo explica; quitar el TOTP
    se rechaza mientras haya suscripción personal;
  - aviso fijo en la Web UI y en la terminal: «Este servidor usa tu suscripción de <agente>: solo para ti».
- **Peticiones a la vez:** `adaptadores.<agente>.suscripcion_simultaneas` (1 por defecto, de 1 a 3) limita las peticiones
  simultáneas con suscripción de ese agente en todo el servidor, sumando todas las APIs y todos los espacios, porque
  todas usan la misma cuenta. Las demás esperan en cola, en orden. Subirlo muestra el aviso de riesgo y pide confirmarlo.
  Con clave de API no cambia nada, y el Tramtai de escritorio sigue como hoy.
- **Si un CLI no pasa su M0** (inicio de sesión con la cuenta del servicio, o sus términos no lo permiten), ese agente
  no ofrece suscripción en el servidor y lo dice.

### 6. Red y HTTPS

- **Expuesto** quiere decir: `servicio.host` no es loopback, o `servicio.proxy.confiables` no está vacío.
- **Reglas que impiden arrancar** (el servicio no arranca y dice qué hacer):
  - `host` no loopback sin `servicio.permitir_remoto` (como hoy);
  - `host` no loopback con `servicio.tls.modo: ninguno`: las claves viajarían en claro. Esto incluye un proxy en otra
    máquina: entre el proxy y Tramtai también hace falta HTTPS;
  - expuesto con suscripción que no sea personal, o con personal sin TOTP (sección 5);
  - `letsencrypt` sin `servicio.dominios` o sin correo;
  - `archivos` con el certificado o la clave ilegibles, o con un certificado que no cubre `servicio.dominios`.
- **Cambio de comportamiento:** quien hoy usa `permitir_remoto` por HTTP plano verá que el servicio no arranca, con el
  mensaje que ofrece el proxy, HTTPS o el túnel SSH. Va en las notas de la versión.
- **Detrás de un proxy (la recomendada):**
  - `servicio.proxy.confiables`: direcciones o redes (CIDR) de los proxies. `X-Forwarded-For`, `X-Forwarded-Proto` y
    `X-Forwarded-Host` solo se creen si la conexión viene de una de ellas; de cualquier otra se ignoran y la petición no
    cuenta como local, como hoy;
  - la dirección del cliente para el registro y los frenos es la de `X-Forwarded-For` que añadió el proxy de confianza
    (la última que no es de confianza, leyendo de derecha a izquierda);
  - sin `X-Forwarded-Proto: https`, no se puede iniciar sesión.
- **`servicio.tls.modo: archivos`:** `servicio.tls.certificado` y `servicio.tls.clave` en PEM. Se vuelven a leer cuando
  cambian, sin reiniciar.
- **`servicio.tls.modo: letsencrypt`** (opcional):
  - `servicio.tls.letsencrypt.correo` y los nombres de `servicio.dominios`;
  - `golang.org/x/crypto/acme/autocert`, con desafío TLS-ALPN-01 en el 443: no abre el 80;
  - solo acepta los nombres de `servicio.dominios` (`HostPolicy`);
  - caché en `datos/acme` (0700); certificado ECDSA P-256.
- **TLS propio (archivos y Let's Encrypt): solo TLS 1.3.**
  - `MinVersion: tls.VersionTLS13`; no hay cifrados que elegir (todos AEAD con secreto hacia adelante) y Go no acepta
    datos 0-RTT en el servidor;
  - intercambio de claves híbrido poscuántico X25519MLKEM768, el de Go 1.27 por defecto, con X25519 si el cliente no lo
    admite;
  - HSTS de un año;
  - un cliente sin TLS 1.3 no conecta; la guía lo remite al proxy.
- **En todos los modos:**
  - Host y Origin solo valen para loopback y `servicio.dominios`;
  - los plazos de lectura y el tope de cabeceras de hoy se mantienen;
  - las claves falsas en las APIs también tienen freno, por abuso.
- **Configuración:** `servicio.dominios`, `servicio.proxy.confiables` y todo `servicio.tls.*` son claves de seguridad.

### 7. Administrador y Web UI remota

- **Contraseña:** `tramtai --servidor administrador contraseña` la pone o la cambia (pide repetirla; mínimo 12
  caracteres). Se guarda solo su huella argon2id en `datos/administrador`. Sin contraseña, la Web UI remota solo se abre
  con el código de un solo uso de `tramtai --servidor abrir`.
- **TOTP (RFC 6238, biblioteca estándar):** `tramtai --servidor administrador totp activar|quitar`, o ya dentro en
  **Ajustes → Seguridad**. Muestra el secreto y el enlace `otpauth://` y da 10 códigos de recuperación de un solo uso,
  guardados como huella. El secreto va al almacén cifrado (sección 4). En Linux se activa y se quita solo desde la CLI,
  porque el servicio no puede escribir en `systemd-creds`; la Web UI muestra la orden.
- **La página de inicio de sesión:**
  - solo con TLS propio o desde un proxy de confianza con `X-Forwarded-Proto: https`; si no, explica por qué no puede;
  - el mismo mensaje si falla la contraseña o el código;
  - tras 5 fallos desde una dirección, esa dirección espera 15 minutos; además, un freno global;
  - argon2id con un tope de cálculos a la vez, para que no sirva para agotar el servicio;
  - sin redirecciones a URL que vengan en la petición.
- **La sesión:**
  - cookie `__Host-` con `Secure`, `HttpOnly` y `SameSite=Strict`, y el `X-CSRF-Token` de hoy;
  - caduca tras 12 h sin uso;
  - cambiar la contraseña o «Cerrar todas las sesiones» las cierra todas;
  - no es local: no guarda claves ni ve las carpetas del servidor (como la sesión por código de hoy).
- **Registro:** cada inicio de sesión, bueno o malo, y cada bloqueo, con la dirección del cliente.

### 8. Web UI y terminal

- **En el servidor:**
  - **Ajustes → Servidor (solo lectura):** modo de HTTPS, dominios, proxies de confianza, vencimiento del certificado y
    si hay administrador con TOTP, cada dato con la orden que lo cambia. No se edita desde una sesión remota: un cambio
    de red mal hecho deja fuera sin vuelta;
  - **Ajustes → Seguridad:** cambiar la contraseña (pide la actual), TOTP, regenerar los códigos de recuperación y
    «Cerrar todas las sesiones»;
  - **Agentes:** en lugar de «Usar mi suscripción», lo de la sección 5 (local: el inicio de sesión del CLI con su orden;
    remoto: la suscripción personal y sus condiciones), y las claves que faltan con la orden que las pone;
  - no aparecen el arranque con el sistema, la bandeja ni «Instalar ahora» (sección 9).
- **La terminal del servidor** (`tramtai --servidor`): las 8 páginas con las mismas reglas que la Web UI y la marca
  «Servidor».
- **La tecla `W`**, común a todas las páginas de la terminal (la `w` minúscula sigue siendo «ver en la web» de la Ayuda):
  - con navegador en el equipo, pide un código de un solo uso y abre la Web UI ya dentro;
  - sin navegador (sesión SSH o sin `DISPLAY`/`WAYLAND_DISPLAY` en Linux), muestra la URL y el código; si el servidor
    tiene administrador, basta con la URL;
  - el pie de cada página la anuncia y la Ayuda la incluye en la lista de teclas.

### 9. Instalación, actualización y desinstalación

- **`tramtai servidor instalar`**, desde cualquier copia (el `.tar.gz` de Linux, el instalador o el `.zip` de Windows):
  1. comprueba que es administrador y que el sistema es Linux con systemd o Windows;
  2. muestra el resumen de riesgos y de `TERMINOS.md` y pide aceptarlo escribiendo «acepto»
     (`--acepto-terminos` para instalar sin preguntas); la aceptación queda registrada (sección 10);
  3. copia el programa a `/opt/tramtai/` (`tramtai` y `uv`, con `/usr/local/bin/tramtai-servidor`) o a
     `C:\Program Files\Tramtai Servidor\` (con `tramtai-servidor.cmd`). Si el programa ya está ahí porque lo instaló el
     paquete, no copia nada. La cuenta del servicio solo puede leer esa carpeta;
  4. crea la cuenta (`useradd --system --home /var/lib/tramtai --shell /usr/sbin/nologin tramtai` en Linux; la cuenta
     virtual en Windows), la carpeta de datos con sus permisos y una configuración inicial solo en loopback;
  5. registra el servicio, lo arranca y dice cómo seguir: contraseña, HTTPS o proxy, y claves.
  - Repetirla es seguro: con la misma versión no cambia nada; con otra, actualiza.
  - Sirve en Windows Server Core: el servicio no necesita WebView2 ni escritorio.
  - `--conversor <zip>` prepara el conversor desde el paquete sin red (Windows x64), para servidores sin internet.
- **Paquetes `.deb` y `.rpm` del servidor (Linux amd64 y arm64):**
  - `Tramtai-<versión>-servidor-linux-<arq>.deb` y `.rpm`, con nfpm como los de escritorio y una configuración aparte
    (`scripts/linux/nfpm-servidor.yaml`); nombre del paquete `tramtai-servidor`;
  - contienen `/opt/tramtai/tramtai`, `/opt/tramtai/uv`, `/usr/bin/tramtai-servidor`, la unidad
    `/usr/lib/systemd/system/tramtai.service`, la cuenta en `/usr/lib/sysusers.d/tramtai.conf` y la carpeta en
    `/usr/lib/tmpfiles.d/tramtai.conf`, `TERMINOS.md` y la licencia en `/usr/share/doc/tramtai-servidor/`;
  - sin dependencias (el programa es estático) y sin chocar con el paquete de escritorio, que usa `/usr/bin/tramtai` y
    `/usr/lib/tramtai`: se pueden tener los dos;
  - **al instalar no arranca nada:** el paquete no puede pedir que se acepten los términos. Lo dice al terminar y
    remite a `sudo tramtai-servidor instalar`, que pide la aceptación, crea la configuración, habilita y arranca;
  - **actualizar** con `apt`/`dnf`: el `postinst` reinicia el servicio solo si ya estaba habilitado, con drenaje.
    `tramtai-servidor actualizar` detecta que vino de un paquete y remite a `apt`/`dnf`;
  - **quitar** el paquete detiene y deshabilita el servicio y conserva los datos (`/var/lib/tramtai`) y las
    credenciales; `apt purge` tampoco los borra: eso es `tramtai-servidor desinstalar --borrar-datos` antes;
  - se publican con su suma en `SHA256SUMS.txt`, firmado como el resto. Sin repositorio de `apt`/`dnf` propio en esta
    versión.
- **Actualizar:** el servidor comprueba cada hora, como hoy, y avisa en la Web UI, la terminal y `servidor estado`.
  `tramtai servidor actualizar`:
  - reutiliza `internal/actualizacion` (firma ed25519 de `SHA256SUMS.txt` y suma SHA-256);
  - toma el programa del `.tar.gz` de su arquitectura o del `.zip` portable de Windows;
  - detiene el servicio con drenaje, cambia el programa, guarda el anterior como `.anterior` y arranca;
  - volver solo a la versión anterior si la nueva no arranca es la v0.61.
  - No se actualiza desde la Web UI remota: el servicio no puede escribir en su carpeta de programa.
- **Desinstalar:** `tramtai servidor desinstalar` quita el servicio, el programa y la unidad, y conserva los datos.
  `--borrar-datos` borra también la carpeta, las credenciales cifradas y la cuenta, tras pedir confirmación.

### 10. Términos, riesgos y avisos

- **`TERMINOS.md` versión 2** (es/en; manda el español): el texto del anexo B, revisado como lo haría un abogado
  (las razones de cada cambio están en B.1). La versión en inglés se traduce fiel en la implementación.
- **Los riesgos, dichos claro y en el momento justo.** Siempre los mismos cinco, con las mismas palabras en todas
  partes (un solo texto en `web/src/idiomas`, `internal/idioma` y la Ayuda):
  1. **Tu cuenta con un proveedor puede limitarse, suspenderse o cerrarse** si usas tu suscripción con Tramtai, y el
     proveedor puede cambiar sus reglas sin avisar. El autor no puede recuperarla.
  2. **Las condiciones de Tramtai** (TOTP, una petición a la vez) **bajan el riesgo, pero no lo quitan** ni son una
     opinión legal sobre los términos del proveedor.
  3. **Lo que abres a la red es responsabilidad tuya:** HTTPS, cortafuegos, contraseñas, copias y actualizaciones.
  4. **Si otras personas usan tu servidor,** tú respondes de sus datos ante la ley que te aplique.
  5. **Tramtai es gratuito y se entrega tal cual,** sin garantías.
- **Dónde se muestran:**
  - **al instalar:** página de términos del instalador de Windows (casilla «Acepto», no basta con avanzar) y
    `tramtai-servidor instalar` (hay que escribir «acepto»), con los cinco riesgos antes del enlace;
  - **al activar la suscripción** (escritorio y servidor) y al subir `suscripcion_simultaneas`: los riesgos 1 y 2 y
    confirmar; en el servidor remoto, además, la declaración de uso personal;
  - **al abrir el servidor a la red** (primer arranque expuesto): el riesgo 3 en el registro y en `servidor estado`;
  - **siempre a la vista:** el aviso fijo de la suscripción personal en remoto, y Acerca de con el enlace a los
    términos;
  - **en el README, el sitio, la guía del servidor y la Ayuda:** un apartado «Aviso» con los cinco riesgos y el enlace
    a `TERMINOS.md`.
- **Para toda la aplicación, no solo el servidor.** La versión 2 rige toda Tramtai, y la aceptación expresa se pide
  en cada entorno (escritorio, portable y servidor), cada uno con su propio registro:
  - **al instalar,** donde hay páginas: el instalador de Windows y `tramtai-servidor instalar`;
  - **al abrir la Web UI, la app de escritorio o la terminal,** si ese entorno no tiene aceptación o es de una versión
    mayor anterior: la pantalla «Antes de seguir», con los cinco riesgos y «Acepto» / «Salir» (que cierra sin usar
    nada). La ven una vez quienes actualicen a la v0.58 (versión 1 → 2) y quienes instalen por una vía sin páginas
    (`.dmg`, paquetes de escritorio de Linux, `.tar.gz`, `.zip` portable, Homebrew, winget o el instalador en
    silencio). En el escritorio, los riesgos 3 y 4 se dicen como «si usas el modo servidor…»;
  - **la CLI sin interfaz no se bloquea,** para no romper automatizaciones al actualizar: `tramtai terminos` los
    muestra y `tramtai terminos aceptar` los acepta; mientras falte, cada orden escribe un aviso de una línea en stderr
    (nunca en la salida normal) y el servicio sigue funcionando. La API de las apps no cambia.
- **Registro de la aceptación:** `datos/terminos.json` de cada entorno guarda la versión aceptada, la fecha (ISO 8601,
  UTC) y cómo se aceptó (`instalador`, `servidor-instalar`, `opcion` para `--acepto-terminos`, `interfaz`, `cli`). Un
  cambio de versión mayor de los términos vuelve a pedir la aceptación como se acaba de describir; uno menor (erratas,
  enlaces) solo se avisa en las novedades. La versión vigente y el resumen de riesgos van en un paquete nuevo,
  `internal/terminos` (constante `Version`, lectura y escritura de `terminos.json`), que usan la CLI, la terminal, el
  servicio y la app; `GET /ui/terminos` y `POST /ui/terminos` los sirven a la Web UI.
- **Límite honesto:** la revisión del anexo B no sustituye a un abogado habilitado en Chile. Antes de una versión de
  pago, hay que validarlo con uno (con pago, la Ley 19.496 sí aplicaría; ver B.1).

### 11. Documentación, sitio y capturas

Todo en español y en inglés:
- **Guía del servidor:** `docs/servidor.md` y `site/servidor.html` (generada como la de desarrolladores):
  - instalar en Linux (con `sha256sum -c SHA256SUMS.txt`) y en Windows Server;
  - dos entornos;
  - abrir a la red: proxy, certificado propio y Let's Encrypt;
  - proxies con su ejemplo: Caddy, nginx, Apache httpd, Traefik, HAProxy e IIS (ARR y URL Rewrite), cada uno con TLS
    1.3 y sin búfer para el flujo en vivo (SSE);
  - túneles: Cloudflare Tunnel (el HTTPS termina en Cloudflare, que ve las peticiones y las claves) y Tailscale
    Serve/Funnel;
  - qué debe cumplir cualquier otro proxy: TLS 1.3, `X-Forwarded-Proto`, Host conservado y sin búfer;
  - administrador y TOTP; claves y suscripción personal; actualizar y desinstalar; «Si algo falla».
- **Ayuda:** temas «Servidor» y «Dos entornos»; «Arranque con el sistema» con las tres formas; la tecla `W`.
- **`docs/arquitectura.md`:** la tercera raíz, el servicio del sistema, la tabla de claves y el modelo de amenazas del
  servidor (red, proxy, inicio de sesión, ACME y suscripción remota).
- **`docs/operacion.md`** (arranque, servidor, dos entornos), **`docs/inicio.md`** (instalar el servidor) y la **guía de
  desarrolladores** (la URL `https://` de un servidor en los SDK).
- **README** (modo servidor y Aviso) y **`docs/equipo/pendientes.md`** (la fila con su spec y las filas nuevas de
  «Fuera»).
- **Capturas:** la terminal en la portada (T14 de la v0.56; ajustar la nota del puerto 17388 frente al 17373) y, con
  `npm run capturas`, Ajustes → Arranque, Ajustes → Servidor y Seguridad, la página de inicio de sesión y la marca
  «Servidor».
- **Regenerar** la Web UI compilada y el sitio.

## M0 antes de implementar

Con evidencia en `docs/equipo/m0-agentes.md` (las de los CLI) y en un anexo A de esta spec (las del sistema). Si una
falla, se aplica lo de la derecha:

| Comprobación | Si falla |
|---|---|
| `claude auth login` con la cuenta del servicio en Linux y en Windows (cuenta virtual) | Claude sin suscripción en ese sistema |
| `codex login --device-auth` con `CODEX_HOME` privado (hay que activar el inicio por código de dispositivo en la seguridad de la cuenta de ChatGPT; está en beta) | Codex sin suscripción en el servidor |
| Copilot: inicio de sesión por dispositivo o token de grano fino con la cuenta del servicio | Copilot sin suscripción en el servidor |
| Términos vigentes de Anthropic, OpenAI y GitHub sobre uso remoto personal, con citas | ese agente sin suscripción personal en remoto |
| Administrador de credenciales con `NT SERVICE\Tramtai` (escribir y leer sin sesión) | claves por variable de entorno del servicio, con aviso |
| `systemd-creds` en Debian 12-13, Ubuntu 24.04 y RHEL 9 | el archivo de entorno de root, con aviso |
| La carpeta de perfil de la cuenta virtual y dónde guardan su estado los CLI | ajustar `HOME`/`USERPROFILE` del servicio a su carpeta |

## Pruebas

- **Carpetas:** `--servidor` y `TRAMTAI_SERVIDOR=1` eligen la raíz del servidor; sin ellos, la de hoy.
- **Unidad systemd:** el archivo generado es igual al esperado (con y sin Let's Encrypt).
- **Reglas de arranque:** cada combinación insegura de la sección 6 impide arrancar con su mensaje; las seguras
  arrancan.
- **Proxy:** las cabeceras reenviadas solo valen desde `servicio.proxy.confiables`; desde fuera, una cabecera falsa no
  cambia la dirección del cliente ni abre la entrada local.
- **TLS:** con TLS 1.2 la conexión se rechaza; con un cliente Go actual se negocia TLS 1.3 con X25519MLKEM768; el
  certificado se vuelve a leer al cambiar el archivo.
- **Let's Encrypt:** contra Pebble (el servidor de pruebas de Let's Encrypt) en un contenedor, con TLS-ALPN-01.
- **Inicio de sesión:** bloqueo tras 5 fallos, el mismo mensaje para contraseña y código, TOTP y códigos de recuperación
  de un solo uso, cookie `__Host-` con sus atributos, CSRF, caducidad, cerrar todas las sesiones, y que sin HTTPS no se
  puede entrar.
- **Claves:** en Windows por la API de administración; en Linux, `$CREDENTIALS_DIRECTORY` primero; nunca se guarda un
  token de Claude; la Web UI no guarda claves.
- **Suscripción:** las condiciones de la personal; `suscripcion_simultaneas` deja N a la vez sumando todas las APIs y
  todos los espacios, y el resto espera en orden; con clave de API no limita.
- **«Solo el servicio»:** las pruebas de la Tarea 13 de la v0.56 (`TestElegirServicio*`, `TestConfiguradoDistingueModos`,
  `TestAutoarranqueUIModo`).
- **Dos entornos:** la marca en la Web UI y la terminal, y el aviso cruzado.
- **Tecla `W`:** con navegador y sin él.
- **Job «Servidor» en el CI:**
  - Linux (systemd con sudo): instalar, arrancar, HTTPS con un certificado de prueba, contraseña e inicio de sesión,
    una petición con un agente simulado, actualizar desde un paquete local y desinstalar;
  - Windows (los runners de GitHub son Windows Server 2025): la misma secuencia con el servicio de Windows.
- **Paquetes del servidor (Publicar y CI, contenedores Debian y Rocky Linux):** `.deb` y `.rpm` se instalan junto al
  paquete de escritorio sin chocar, dejan los archivos, la cuenta y la unidad en su sitio, no arrancan nada, y
  quitarlos conserva `/var/lib/tramtai`. La secuencia completa con systemd (instalar, habilitar, actualizar) va en el
  job «Servidor» con el `.deb`.
- **Términos:** sin aceptar no se instala el servidor; `--acepto-terminos` y el instalador escriben
  `datos/terminos.json`; la Web UI, la app y la terminal muestran «Antes de seguir» sin aceptación o con una versión
  mayor anterior, y no después de aceptar; «Salir» no deja usar nada; la CLI sin aceptar avisa solo por stderr y no
  falla; `tramtai terminos aceptar` lo registra; cada entorno tiene su registro; un cambio de versión menor no vuelve a
  pedirla.
- **`tramtai-servidor`:** usa el programa del servidor y no el de escritorio; con `--servidor`, la CLI nunca detiene un
  servicio del sistema de otra versión.
- **Job «Proxies» en el CI (Linux, contenedores):** Caddy, nginx, Apache, Traefik y HAProxy con los ejemplos de la guía:
  hay que iniciar sesión, el flujo en vivo llega a trozos, Tramtai ve la dirección real y una cabecera falsa desde fuera
  no cuela.
- **A mano antes de publicar:** IIS en Windows Server, Cloudflare Tunnel y Tailscale Serve (en la guía, «comprobado en
  la v0.58»), y `testssl.sh` contra el HTTPS propio (solo TLS 1.3, intercambio poscuántico, HSTS).

## Revisión de seguridad

Una tarea del plan, hecha por un revisor de seguridad aparte, con esta lista y una prueba automática por punto:
cabeceras reenviadas falsificadas, Host y Origin, fuerza bruta y frenos, TLS y ACME (`HostPolicy`, caché), cookie y
CSRF, redirecciones del inicio de sesión, tiempos de respuesta, agotamiento de recursos (argon2id, conexiones, flujos),
permisos de las carpetas y de la unidad, y la suscripción expuesta. Después, la revisión final de la rama con opus, como
en las últimas versiones.

## Entrega

- Rama `v0.58/servidor`, un PR por tarea, con Auto-fix.
- Notas de la versión en español y en inglés, con el cambio de comportamiento de `permitir_remoto` por HTTP.

## Fuera

- Imagen de contenedor (Docker), sobre este diseño: release propio, el último de la hoja de ruta.
- Repositorio propio de `apt`/`dnf` (los paquetes se descargan del release).
- Servicio del sistema en macOS.
- Varias cuentas, permisos por persona, OIDC y registro de cambios por persona (Pro/Team).
- Editar la red, el HTTPS o las claves desde una sesión remota de la Web UI.
- Volver solo a la versión anterior si la nueva no arranca (v0.61).
- **i18n igual en todas partes y un tercer idioma:** release propio, la v0.62. Primero, que añadir un idioma sea añadir sus
  archivos y una entrada en un registro también en Go (`internal/idioma` hoy solo conoce es/en), la Ayuda y el sitio,
  con una prueba de cobertura común; después el idioma, traducido por el usuario. Recomendado: portugués de Brasil y,
  más adelante, chino simplificado. Plataformas como Weblate o Crowdin piden una licencia OSI; se verá con el paso a
  open core.

## Anexo A. M0 del sistema

Hecha el 2026-10-26 con la sonda `pruebas/m0servidor` (etiqueta `m0servidor`; se queda como evidencia, como `pruebas/m0b`).
Las pruebas de los CLI están en `docs/equipo/m0-agentes.md`, apartado «v0.58: servidor».

### Windows: cuenta virtual y Administrador de credenciales

Windows 11 Pro 10.0.26300, sesión de administrador. Servicio `TramtaiM0` (`sc.exe create … obj= "NT SERVICE\TramtaiM0"`), con
el JSON de la sonda en una carpeta con permiso explícito para esa cuenta
(`icacls <carpeta> /grant "NT SERVICE\TramtaiM0:(OI)(CI)(M)"`, ejecutado **después** de crear el servicio: antes, la cuenta
no existe para `icacls`).

| Dato | Resultado |
|---|---|
| Cuenta que ve el proceso | `NT SERVICE\TramtaiM0` (SID `S-1-5-80-…`) |
| `HOME` | vacío |
| `USERPROFILE` y `os.UserHomeDir()` | `C:\WINDOWS\ServiceProfiles\TramtaiM0` (existe) |
| `os.UserConfigDir()` y `APPDATA` | `…\ServiceProfiles\TramtaiM0\AppData\Roaming` |
| `LOCALAPPDATA` | `…\ServiceProfiles\TramtaiM0\AppData\Local` |
| `credenciales.Guardar("ANTHROPIC_API_KEY", "prueba-m0")` | correcto |
| `credenciales.Leer(…)` | correcto, el valor coincide |
| `credenciales.Borrar(…)` | correcto |
| Tras detener y volver a arrancar el servicio (un arranque guarda, el siguiente lee y borra) | el valor sigue ahí |

- **El Administrador de credenciales funciona sin sesión** con la cuenta virtual, sin cambiar `internal/credenciales`: se
  escribe, se lee y sobrevive a reiniciar el servicio. El almacén es de esa cuenta (su perfil), no el de quien administra:
  por eso la CLI entrega la clave al servicio por la API de administración (§4). No se probó que el administrador no lea
  la clave de la cuenta del servicio desde su propia sesión; se da por el diseño de Windows.
- **Carpeta de perfil:** no está bajo `%ProgramData%\Tramtai` sino en `C:\WINDOWS\ServiceProfiles\<nombre del servicio>`.
  `HOME` va vacío, `USERPROFILE` y `APPDATA` apuntan ahí. Se espera que los CLI, que en Windows suelen partir de `USERPROFILE`, dejen
  su `.claude`, `.codex` y `.copilot` en esa carpeta sin ajustar nada; no está comprobado. El ajuste de `HOME`/`USERPROFILE`
  de la tabla de arriba solo hará falta si un CLI no la usa (se verá con los inicios de sesión pendientes).
- **Consecuencias para el instalador (Tareas de `instalar` y `desinstalar`):**
  - crear el servicio **antes** de dar permisos a `NT SERVICE\Tramtai` en la carpeta de datos;
  - el perfil sobrevive al servicio: `sc.exe delete` no lo borra y volver a crear un servicio con el mismo nombre
    recupera la misma cuenta (mismo SID) y el mismo perfil, con las sesiones de los CLI dentro. `desinstalar
    --borrar-datos` tiene que decidir qué hace con él (la sesión de los CLI es una credencial) y decirlo;
  - Windows tardó en descargar el perfil al borrar el servicio: `Remove-CimInstance` sobre `Win32_UserProfile` devolvió
    «el archivo está siendo utilizado» y `reg unload` «acceso denegado». Se quita con un reinicio.
    **En la máquina de la prueba queda el perfil residual `C:\WINDOWS\ServiceProfiles\TramtaiM0`** (sin ninguna clave) hasta
    reiniciar; después se quita con `Get-CimInstance Win32_UserProfile | Where-Object LocalPath -like '*TramtaiM0' |
    Remove-CimInstance`.

### Linux: `systemd-creds`

`docker run --rm <imagen> …`. La imagen mínima no trae `systemd`; con el paquete `systemd` (apt o dnf) el resultado es:

| Imagen | `systemd-creds --version` |
|---|---|
| `debian:12` | systemd 252 (252.39-1~deb12u2) |
| `debian:13` | systemd 257 (257.13-1~deb13u1) |
| `ubuntu:24.04` | systemd 255 (255.4-1ubuntu8.18) |
| `rockylinux:9` (en lugar de RHEL 9) | systemd 252 (252-67.el9_8.6) |

Las cuatro dan versión 250 o mayor, así que el camino de `systemd-creds` vale en todas. Además, en `debian:12` y
`rockylinux:9`, `echo -n prueba-m0 | systemd-creds encrypt --name=tramtai.X - archivo` y `systemd-creds decrypt
--name=tramtai.X archivo -` devolvieron `prueba-m0` como root, con la clave del equipo (sin TPM2; en el contenedor avisa de
que la clave no está en un disco cifrado). Queda sin probar en un systemd real la entrega con `LoadCredentialEncrypted=`;
la prueba del job «Servidor» del CI (Linux) la cubre. El archivo de entorno de root sigue siendo el plan B de systemd
anterior a la 250 (aquí no hace falta en ninguna de las cuatro).

### Qué queda fuera (según la tabla de «M0 antes de implementar»)

| Comprobación | Resultado | Queda fuera |
|---|---|---|
| `claude auth login`, cuenta del servicio (Linux y Windows) | pendiente del usuario | nada por ahora |
| `codex login --device-auth` con `CODEX_HOME` privado | pendiente del usuario | nada por ahora |
| Copilot, inicio por dispositivo o token de grano fino | pendiente del usuario | nada por ahora |
| Términos de Anthropic, OpenAI y GitHub | leídos los tres, con citas (`m0-agentes.md`); no prohíben el uso personal de la propia cuenta desde un servidor | nada |
| Administrador de credenciales con la cuenta virtual | pasa | nada |
| `systemd-creds` en Debian 12-13, Ubuntu 24.04 y RHEL 9 | pasa en las cuatro | nada |
| Carpeta de perfil y estado de los CLI | `ServiceProfiles\<servicio>`; `USERPROFILE` ya apunta ahí | nada |

- **Ninguna fila deja un agente fuera con lo comprobado**, así que `agentes.Ficha` **no** recibe `ServidorSuscripcion`. Se
  añadirá solo si falla un inicio de sesión pendiente.
- Los términos de los tres no prohíben el uso personal de la propia cuenta desde un servidor. Prohíben compartir las
  credenciales o la cuenta (lo que la suscripción personal ya impide con sus condiciones). Lo de OpenAI de extraer datos
  «de forma automática o mediante programación» trata del raspado de los servicios, no del modo no interactivo de Codex.
  Tres cosas condicionan: solo el titular, uso ordinario e individual y riesgo de suspensión (Anthropic y su «automated or
  non-human means»), y que OpenAI pide no exponer Codex «in untrusted or public environments». Eso refuerza lo que la spec
  ya hace: la suscripción personal en remoto exige TOTP, aviso y `suscripcion_simultaneas` en 1.
- La **Tarea 13** usa los valores del plan hasta que el usuario haga los inicios de sesión.

## Anexo B. Borrador de `TERMINOS.md` versión 2

Sustituye a la versión 1 al publicar la v0.58; «[día de publicación]» se cambia por la fecha de la etiqueta. La versión
en inglés se traduce fiel, con la frase «The Spanish version prevails». Las razones de cada cambio, en B.1.

---

> # Términos de uso de Tramtai
>
> **Versión 2 · [día de publicación]** · [English below](#terms-of-use)
>
> **Resumen de riesgos (léelo antes de aceptar).** Tramtai es gratuito y se entrega tal cual. (1) Si usas tu
> suscripción de un proveedor con Tramtai, **ese proveedor puede limitar, suspender o cerrar tu cuenta**, y puede
> cambiar sus reglas sin avisar; el autor no puede recuperarla. (2) Las condiciones que pone Tramtai **bajan ese
> riesgo, pero no lo quitan**. (3) **Lo que abras a la red es responsabilidad tuya.** (4) **Si otras personas usan tu
> servidor, tú respondes de sus datos.** (5) **No hay garantías** y la responsabilidad del autor está limitada (cláusulas
> 10 a 12).
>
> ## 1. Partes y aceptación
>
> - Tramtai lo publica Michael Scott B. ([MscottB](https://github.com/MscottB)), persona natural, «el autor».
> - Aceptas estos términos y la [licencia](LICENSE.md) cuando los aceptas de forma expresa (en el instalador, en
>   `tramtai-servidor instalar` o en la aplicación) o cuando instalas o usas Tramtai. Tramtai guarda en tu equipo qué
>   versión aceptaste y cuándo. Si no estás de acuerdo, no lo instales ni lo uses.
> - Si aceptas en nombre de una organización, declaras que puedes obligarla; recuerda que el uso por una organización
>   o con fines comerciales necesita una licencia aparte (cláusula 3).
>
> ## 2. Qué es Tramtai
>
> - Tramtai es un programa que funciona en equipos que tú controlas: tu ordenador o un servidor que administras tú.
>   Conecta tus aplicaciones con agentes de IA de terceros: Claude Code, Codex, GitHub Copilot CLI y Antigravity
>   mediante sus programas oficiales, y la Gemini API con tu propia clave.
> - **Tramtai no es un proveedor de IA:** el autor no opera servidores para ti, no vende acceso a ningún modelo y no
>   forma parte de Anthropic, OpenAI, GitHub, Microsoft ni Google, que no lo respaldan ni lo patrocinan.
>
> ## 3. Licencia
>
> Puedes usar, estudiar y modificar Tramtai para fines personales y no comerciales según la licencia
> [PolyForm Noncommercial 1.0.0](LICENSE.md). Para usarlo en una empresa, en otra organización con fines comerciales o
> en un producto comercial necesitas una licencia aparte del autor. En lo que toca a tus derechos de uso del programa,
> manda la licencia.
>
> ## 4. Servicios de terceros
>
> - Para usar un agente necesitas **tu propia cuenta o clave de API** con su proveedor. **Tú aceptas y debes cumplir**
>   los términos y las políticas de uso de cada proveedor.
> - El autor no controla esos servicios: su disponibilidad, precios, límites, términos y decisiones sobre tu cuenta
>   dependen solo del proveedor, que puede cambiarlos en cualquier momento y sin aviso. Un cambio de un proveedor puede
>   hacer que Tramtai deje de funcionar con él.
> - Los costos que te cobre un proveedor son tuyos.
>
> ## 5. Uso con tu suscripción
>
> - Tramtai permite que Claude Code, Codex o GitHub Copilot CLI usen tu suscripción (por ejemplo, Claude Pro/Max,
>   ChatGPT o GitHub Copilot) en lugar de una clave de API: en tu equipo y, en el modo servidor, también desde otros
>   equipos, si activas la «suscripción personal» y aceptas sus condiciones.
> - Es **solo para ti, como titular de la suscripción**, para un uso personal y ordinario. No la compartas ni des a
>   otras personas las claves de las APIs que la usan.
> - **Los términos de los proveedores pueden no permitir** el uso automatizado o remoto de una suscripción personal, o
>   usarla para alimentar otras aplicaciones, y cambian con el tiempo. Antes de activarla, declaras que has leído los
>   términos vigentes de ese proveedor y que tu uso los cumple.
> - **Usarla puede llevar a que el proveedor limite, suspenda o cierre tu cuenta**, con la pérdida del acceso, de tus
>   datos en ese servicio o de lo que hayas pagado. El autor no puede intervenir ni recuperar tu cuenta. **Ese riesgo y
>   la decisión de usarla son exclusivamente tuyos.**
> - Las condiciones que pone Tramtai (segundo factor, pocas peticiones a la vez, avisos) **reducen el riesgo, pero no lo
>   eliminan**, y no son una interpretación de los términos de ningún proveedor ni una asesoría legal.
> - Tramtai no lee, copia ni guarda las credenciales de los programas de los proveedores: el inicio de sesión lo hace
>   el propio programa del proveedor. El autor puede cambiar o retirar esta función en cualquier momento, por ejemplo
>   si un proveedor lo pide.
> - Para aplicaciones de otras personas, automatizaciones o cualquier uso que no sea estrictamente personal, usa una
>   **clave de API**.
>
> ## 6. Modo servidor y seguridad
>
> - En el modo servidor **tú eres el administrador** y respondes de su configuración y su uso: dónde y cómo lo expones
>   (HTTPS, proxy, cortafuegos), las contraseñas y el segundo factor, a quién das claves, las copias de seguridad y las
>   actualizaciones.
> - Tramtai incluye protecciones (HTTPS solo con TLS 1.3, inicio de sesión con frenos, claves guardadas como huella o
>   cifradas), pero **ningún programa es invulnerable**. Abrir un servicio a la red es una decisión tuya.
> - Si encuentras un fallo de seguridad, avísalo en privado con el aviso de seguridad del
>   [repositorio](https://github.com/MscottB/Tramtai/security), no en un issue público.
>
> ## 7. Tus datos y los de otras personas
>
> - Tramtai guarda su configuración, su historial y tus archivos **solo en los equipos donde lo instalas**. El autor
>   no recibe nada de eso.
> - Tus peticiones van desde ese equipo al proveedor que elijas, y su tratamiento lo rige la política de privacidad de
>   ese proveedor.
> - **Si otras personas usan un Tramtai que administras,** tú decides qué datos suyos se tratan y para qué, y respondes
>   de cumplir la ley de protección de datos que te aplique (por ejemplo, en Chile, la Ley 19.628 y la Ley 21.719 que
>   la sustituye; en la Unión Europea, el RGPD).
> - Para avisar de versiones nuevas, Tramtai consulta la ficha pública del último release en GitHub, sin enviar datos
>   tuyos. Puedes desactivarlo.
>
> ## 8. Uso aceptable
>
> No uses Tramtai para:
> - infringir la ley ni los derechos de otras personas;
> - incumplir los términos o las políticas de uso de un proveedor;
> - revender, compartir o dar a terceros acceso a tus cuentas, suscripciones o claves;
> - eludir límites, controles de pago o medidas de seguridad de un proveedor;
> - acceder sin permiso a sistemas o datos ajenos.
>
> ## 9. Contenido generado
>
> Las respuestas las generan los modelos de los proveedores, no el autor. Pueden ser incorrectas, incompletas o
> inadecuadas. Revísalas antes de usarlas; cómo las usas es responsabilidad tuya.
>
> ## 10. Sin garantía
>
> **TRAMTAI SE ENTREGA «TAL CUAL» Y «SEGÚN DISPONIBILIDAD», SIN GARANTÍAS DE NINGÚN TIPO, NI EXPRESAS NI IMPLÍCITAS,**
> incluidas, entre otras, las de funcionamiento sin errores o sin interrupciones, seguridad, compatibilidad con los
> servicios de los proveedores o con sus términos, y adecuación a un fin concreto.
>
> ## 11. Riesgos que asumes
>
> Al aceptar reconoces que has leído el resumen de riesgos del principio y que asumes, en particular: la limitación,
> suspensión o cierre de tus cuentas con los proveedores; los cambios de sus términos o servicios; los costos que te
> cobren; los incidentes de seguridad derivados de cómo configures o expongas Tramtai; y las consecuencias de usar el
> contenido generado.
>
> ## 12. Limitación de responsabilidad
>
> - **En la máxima medida que permita la ley aplicable, el autor no responde de ningún daño** derivado del uso de
>   Tramtai o de la imposibilidad de usarlo, sea directo, indirecto, incidental, especial o consecuente, incluidos,
>   entre otros, el lucro cesante, la pérdida de datos, los costos de un proveedor, la limitación, suspensión o cierre
>   de cuentas, los incidentes de seguridad y los daños causados por el contenido generado.
> - Si, pese a lo anterior, se declarara responsable al autor, su responsabilidad total quedará limitada a lo que le
>   hayas pagado por Tramtai en los doce meses anteriores al hecho o a 50 dólares de los Estados Unidos, lo que sea
>   mayor.
> - **Nada de lo anterior excluye ni limita la responsabilidad del autor por dolo o culpa grave**, ni los derechos que
>   la ley de tu país declare irrenunciables.
>
> ## 13. Indemnidad
>
> En la medida que permita la ley aplicable, te comprometes a mantener indemne al autor frente a reclamaciones de
> terceros causadas por tu incumplimiento de estos términos, de la licencia o de los términos de un proveedor.
>
> ## 14. Marcas
>
> Claude, Claude Code, Codex, GitHub Copilot, Gemini, Antigravity, Anthropic, OpenAI, GitHub, Microsoft y Google son
> marcas de sus respectivos dueños. Se nombran solo para indicar con qué programas funciona Tramtai, sin sugerir
> respaldo ni asociación.
>
> ## 15. Cambios
>
> - El autor puede actualizar estos términos. La versión vigente es la publicada en el
>   [repositorio](https://github.com/MscottB/Tramtai/blob/main/TERMINOS.md), con su número de versión y su fecha.
> - Si un cambio afecta a tus derechos o a los riesgos que asumes, Tramtai te pedirá aceptarlo de nuevo al actualizar.
>   Los demás cambios se anuncian en las novedades, y seguir usando Tramtai significa que los aceptas.
>
> ## 16. Ley aplicable y tribunales
>
> - Estos términos se rigen por las leyes de la República de Chile, y cualquier controversia se somete a los
>   tribunales ordinarios de justicia de Santiago.
> - Esto no te priva de la protección de las normas imperativas de tu país de residencia que no puedan excluirse por
>   contrato, en especial las de protección de los consumidores.
>
> ## 17. Disposiciones finales
>
> - Si una cláusula no fuera válida, el resto sigue vigente, y esa cláusula se aplicará en la medida en que la ley lo
>   permita.
> - Estos términos y la licencia son el acuerdo completo entre tú y el autor sobre Tramtai.
> - Que el autor no exija una cláusula en un momento dado no significa que renuncie a ella.
> - Estos términos están en español y en inglés; si difieren, manda el español.
> - Consultas: [abre un issue](https://github.com/MscottB/Tramtai/issues) o escribe a
>   [MscottB](https://github.com/MscottB).

---

### B.1. Notas de la revisión

Revisado como lo haría un abogado en contratos de software y consumo, en Chile y con usuarios de otros países. No
sustituye la validación de un abogado habilitado en Chile.

- **Aceptación expresa y registrada (cláusulas 1 y 15).** La versión 1 se aceptaba con solo usar el programa. En
  derecho comparado, una aceptación con casilla u orden explícita y un registro de la versión aceptada se sostiene
  mucho mejor que la aceptación tácita, sobre todo frente a cláusulas de responsabilidad. Por eso el instalador pide
  una casilla, `servidor instalar` pide escribir «acepto» y se guarda `datos/terminos.json`.
- **Resumen de riesgos al principio y renuncia de garantías destacada (resumen y cláusula 10).** Para que una renuncia
  de garantías sea oponible, varias jurisdicciones (p. ej., EE. UU.) exigen que sea visible: por eso va en mayúsculas y
  en negrita. Los riesgos van antes que todo lo demás, con las mismas palabras que la aplicación.
- **Ley chilena y tribunales de Santiago, con la salvedad internacional (cláusula 16).** La versión 1 no fijaba ley
  aplicable ni tribunal, lo que deja abierta la discusión sobre qué ley rige. Con usuarios de otros países, una
  elección de ley no puede quitar a un consumidor las normas imperativas de su país (en la Unión Europea, el
  Reglamento Roma I, art. 6, y la Directiva 93/13 sobre cláusulas abusivas); declararlo evita que se impugne la
  cláusula entera.
- **Dolo y culpa grave (cláusula 12).** En Chile no vale la condonación anticipada del dolo (Código Civil, art. 1465) y
  la culpa grave se equipara al dolo en materia civil (art. 44). Una limitación sin esa salvedad arriesga que se anule
  completa; con ella, el resto se sostiene.
- **Tope de responsabilidad (cláusula 12).** Un tope de cero en un contrato de adhesión se lee como renuncia total y es
  más fácil de anular; un tope razonable (lo pagado en doce meses o 50 dólares) mejora la defensa sin exponer al
  autor.
- **Gratuidad y Ley 19.496.** La ley chilena del consumidor rige los actos a título oneroso; con Tramtai gratuito,
  probablemente no aplica. **Con una versión de pago sí aplicaría,** y su art. 16, letra e), deja sin efecto las
  cláusulas que limitan la responsabilidad del proveedor por deficiencias del servicio: la cláusula 12 tendría que
  rehacerse antes de vender nada.
- **Terceros y suscripción (cláusulas 4, 5 y 11).** No se afirma que un proveedor permita algo: el texto dice que sus
  términos pueden no permitirlo, que el usuario declara haberlos leído y que las condiciones de Tramtai no son una
  interpretación ni una asesoría legal. Así el autor no garantiza el cumplimiento de términos ajenos.
- **Modo servidor y datos de otras personas (cláusulas 6 y 7).** El autor no recibe datos, así que no es responsable
  de su tratamiento. Quien administra un servidor que usan otros sí lo es, ante la Ley 19.628 (y la Ley 21.719, que la
  sustituye) o el RGPD; decirlo traslada esa responsabilidad a quien de verdad la tiene.
- **Indemnidad acotada (cláusula 13).** Se limita a los daños causados por el incumplimiento del propio usuario; una
  indemnidad por cualquier uso sería más fácil de calificar como abusiva.
- **Organizaciones (cláusulas 1 y 3).** La licencia es no comercial. Quien acepta por una organización declara poder
  obligarla, y se recuerda que ese uso necesita licencia aparte.
- **Marcas (cláusula 14).** Se añade «Claude Code» y que no se sugiere respaldo, en línea con las reglas de marca de
  Anthropic para quien ejecuta Claude Code en su producto.
- **Seguridad (cláusula 6).** Un canal privado para avisar de fallos es buena práctica y reduce la exposición si
  alguien encuentra uno.
