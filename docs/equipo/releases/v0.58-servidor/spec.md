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
- **Linux:** se instala desde el `.tar.gz` que ya se publica con `tramtai servidor instalar`; sin paquetes `.deb`/`.rpm`
  del servidor en esta versión.
- **Descargo de responsabilidad:** `TERMINOS.md` versión 2, mostrado al instalar, al activar la suscripción y en el
  README, el repositorio, el sitio y la Ayuda.
- **macOS:** solo «Solo el servicio»; no tiene servicio del sistema en esta versión.
- Todo lo nuevo, en español y en inglés.

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
    el JIT de Node (Claude Code, Codex y Copilot son de Node);
  - `AmbientCapabilities=CAP_NET_BIND_SERVICE` solo con Let's Encrypt o un puerto menor de 1024.
- **Windows:** un servicio con `golang.org/x/sys/windows/svc` (ya es dependencia):
  - nombre `Tramtai`, nombre visible «Tramtai (servidor)», cuenta virtual `NT SERVICE\Tramtai`;
  - arranque automático retrasado, y reinicio si se cae;
  - detener el servicio drena como `tramtai servicio detener`;
  - los Job Objects de los agentes funcionan igual dentro del servicio.
- **Administrar desde el propio servidor:** `sudo tramtai --servidor <orden>` en Linux y una consola de administrador
  con `tramtai --servidor <orden>` en Windows. Lo mismo con la terminal (`tramtai --servidor` sin más). La CLI lee el
  descriptor y `token-servicio` de la carpeta del servidor y, con `--servidor`, acepta un servicio que corre con la
  cuenta del servidor (hoy rechaza los de otro usuario).
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
  guardados como huella. El secreto va al almacén cifrado (sección 4).
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
  2. muestra el resumen de `TERMINOS.md` y pide aceptarlo (`--acepto-terminos` para instalar sin preguntas);
  3. copia el programa a `/opt/tramtai/` (`tramtai` y `uv`, con el enlace `/usr/local/bin/tramtai`) o a
     `C:\Program Files\Tramtai Servidor\`. La cuenta del servicio solo puede leer esa carpeta;
  4. crea la cuenta (`useradd --system --home /var/lib/tramtai --shell /usr/sbin/nologin tramtai` en Linux; la cuenta
     virtual en Windows), la carpeta de datos con sus permisos y una configuración inicial solo en loopback;
  5. registra el servicio, lo arranca y dice cómo seguir: contraseña, HTTPS o proxy, y claves.
  - Repetirla es seguro: con la misma versión no cambia nada; con otra, actualiza.
  - Sirve en Windows Server Core: el servicio no necesita WebView2 ni escritorio.
  - `--conversor <zip>` prepara el conversor desde el paquete sin red (Windows x64), para servidores sin internet.
- **Actualizar:** el servidor comprueba cada hora, como hoy, y avisa en la Web UI, la terminal y `servidor estado`.
  `tramtai servidor actualizar`:
  - reutiliza `internal/actualizacion` (firma ed25519 de `SHA256SUMS.txt` y suma SHA-256);
  - toma el programa del `.tar.gz` de su arquitectura o del `.zip` portable de Windows;
  - detiene el servicio con drenaje, cambia el programa, guarda el anterior como `.anterior` y arranca;
  - volver solo a la versión anterior si la nueva no arranca es la v0.61.
  - No se actualiza desde la Web UI remota: el servicio no puede escribir en su carpeta de programa.
- **Desinstalar:** `tramtai servidor desinstalar` quita el servicio, el programa y la unidad, y conserva los datos.
  `--borrar-datos` borra también la carpeta, las credenciales cifradas y la cuenta, tras pedir confirmación.

### 10. Términos y avisos

- **`TERMINOS.md` versión 2** (es/en):
  - el modo servidor: tu propio servidor, administrado por ti;
  - la suscripción personal también en remoto, con sus condiciones;
  - el riesgo de limitación o suspensión de cuentas, también por cambios de los proveedores sin aviso;
  - abrir un servicio a la red es responsabilidad de quien lo administra;
  - el autor no responde por ninguno de esos daños;
  - la sección 1 deja de decir solo «en tu propio equipo».
- **Dónde se muestra:**
  - el instalador de Windows (su página de términos) y `tramtai servidor instalar`;
  - al activar la suscripción, en el escritorio y en el servidor, y al subir `suscripcion_simultaneas`;
  - el aviso fijo de la suscripción personal en remoto; Acerca de con el enlace;
  - un apartado «Aviso» corto en el README, el sitio y la Ayuda, con el enlace a `TERMINOS.md`.
- Sin revisión legal profesional; conviene una si llega la versión de pago.

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

- Imagen de contenedor (Docker), sobre este diseño.
- Paquetes `.deb` y `.rpm` del servidor.
- Servicio del sistema en macOS.
- Varias cuentas, permisos por persona, OIDC y registro de cambios por persona (Pro/Team).
- Editar la red, el HTTPS o las claves desde una sesión remota de la Web UI.
- Volver solo a la versión anterior si la nueva no arranca (v0.61).
- **i18n igual en todas partes y un tercer idioma:** release propio. Primero, que añadir un idioma sea añadir sus
  archivos y una entrada en un registro también en Go (`internal/idioma` hoy solo conoce es/en), la Ayuda y el sitio,
  con una prueba de cobertura común; después el idioma, traducido por el usuario. Recomendado: portugués de Brasil y,
  más adelante, chino simplificado. Plataformas como Weblate o Crowdin piden una licencia OSI; se verá con el paso a
  open core.
