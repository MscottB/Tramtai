# Guía del servidor

El modo servidor es un Tramtai del sistema: arranca con el equipo, sin que nadie inicie sesión, con su propia cuenta,
sus datos y su puerto, y se abre desde otros equipos por HTTPS con la contraseña del administrador. Funciona en Linux
con systemd y en Windows Server (también Server Core). macOS no tiene servicio del sistema: allí está «Solo el
servicio», que arranca al iniciar sesión (ver [operación](operacion.md#arranque-con-el-sistema)).

Es **un servidor personal, con un solo administrador**. Varias cuentas, permisos por persona y el inicio de sesión con
la cuenta de la empresa no están en esta versión.

[English below](#server-guide)

- [Aviso](#aviso)
- [Dos entornos](#dos-entornos)
- [Instalar en Linux](#instalar-en-linux)
- [Instalar en Windows Server](#instalar-en-windows-server)
- [Administrar desde el propio servidor](#administrar-desde-el-propio-servidor)
- [Abrir a la red](#abrir-a-la-red): proxy, túneles, certificado propio y Let’s Encrypt
- [Administrador y TOTP](#administrador-y-totp)
- [Claves y suscripción](#claves-y-suscripción)
- [Actualizar](#actualizar) y [desinstalar](#desinstalar)
- [Si algo falla](#si-algo-falla)

## Aviso

Antes de instalarlo, `tramtai servidor instalar` enseña estos cinco riesgos y pide escribir «acepto»:

<!-- riesgos-servidor:es -->
1. Tu cuenta con un proveedor puede limitarse, suspenderse o cerrarse si usas tu suscripción con Tramtai, y el proveedor puede cambiar sus reglas sin avisar. El autor no puede recuperarla.
2. Las condiciones de Tramtai (TOTP, una petición a la vez) bajan el riesgo, pero no lo quitan ni son una opinión legal sobre los términos del proveedor.
3. Lo que abres a la red es responsabilidad tuya: HTTPS, cortafuegos, contraseñas, copias y actualizaciones.
4. Si otras personas usan tu servidor, tú respondes de sus datos ante la ley que te aplique.
5. Tramtai es gratuito y se entrega tal cual, sin garantías.
<!-- /riesgos-servidor:es -->

El texto completo está en los [términos de uso](../TERMINOS.md) (versión 2, [día de publicación]). La aceptación queda en
`datos/terminos.json` del servidor; la de tu Tramtai de escritorio no cuenta para él.

## Dos entornos

Tu Tramtai y el del servidor **no comparten nada**: cada uno tiene sus espacios, APIs, claves, sesiones de los CLI,
historial y ajustes. Una API creada en tu equipo no existe en el servidor, ni al revés.

| | Tu Tramtai | El del servidor |
|---|---|---|
| Arranca | Al iniciar sesión («La app en la bandeja» o «Solo el servicio») o cuando lo abres | Con el equipo, sin sesión |
| Cuenta | La tuya | `tramtai` (Linux) o `NT SERVICE\Tramtai` (Windows) |
| Datos | Tu carpeta de Tramtai (o `datos-tramtai` en la portable) | `/var/lib/tramtai` o `%ProgramData%\Tramtai` |
| Programa | Donde lo instalaste | `/opt/tramtai/` o `C:\Program Files\Tramtai Servidor\` |
| Puerto | 17373 por defecto | 17373, o el siguiente libre si el tuyo lo ocupa |
| Claves de los agentes | El almacén cifrado de tu sesión | El de la cuenta del servicio (Windows) o `systemd-creds` (Linux) |
| Se administra con | `tramtai …`, la app, la terminal y la Web UI | `tramtai-servidor …` como administrador, y la Web UI |
| Web UI desde otro equipo | No, por defecto | Sí, por HTTPS y con la contraseña del administrador |

Siempre se ve en cuál estás:

- la cabecera de la Web UI y la línea de estado de la terminal dicen «Servidor» o «Este equipo»;
- Acerca de enseña el entorno, la cuenta y la carpeta;
- con `--servidor`, cada salida de la CLI que crea o cambia algo empieza por «Servidor:»;
- si en tu equipo también corre el del sistema, tu app, tu terminal y `tramtai servicio estado` avisan: «Hay otro
  Tramtai, del sistema, en el puerto X: es otro entorno, con sus propias APIs y claves».

## Instalar en Linux

Necesitas systemd y ser root (`sudo`).

**Con el `.tar.gz`.** Descarga de [Releases](https://github.com/MscottB/Tramtai/releases/latest) el de tu arquitectura
(`tramtai-…-linux-amd64.tar.gz` o `…-linux-arm64.tar.gz`) y `SHA256SUMS.txt`, comprueba la suma y lanza la instalación:

```sh
sha256sum -c --ignore-missing SHA256SUMS.txt
tar xzf tramtai-*-linux-amd64.tar.gz
cd tramtai
sudo ./tramtai servidor instalar
```

**Con el paquete del servidor** (`Tramtai-…-servidor-linux-amd64.deb` o `.rpm`, también para arm64; se puede tener
junto al paquete de escritorio, no chocan):

```sh
sha256sum -c --ignore-missing SHA256SUMS.txt
sudo apt install ./Tramtai-*-servidor-linux-amd64.deb     # o: sudo dnf install ./Tramtai-*-servidor-linux-amd64.rpm
sudo tramtai-servidor instalar
```

- **El paquete no arranca nada:** no puede pedir que aceptes los términos. Deja el programa en `/opt/tramtai/`,
  `/usr/bin/tramtai-servidor`, la unidad en `/usr/lib/systemd/system/tramtai.service`, la cuenta (`sysusers.d`) y la
  carpeta (`tmpfiles.d`); `sudo tramtai-servidor instalar` pide la aceptación, crea la configuración, habilita y arranca.
- **Sus ajustes van en un drop-in:** `/etc/systemd/system/tramtai.service.d/10-tramtai.conf` (el plazo de parada y, con
  Let’s Encrypt o un puerto menor de 1024, el permiso para escucharlo), que Tramtai reescribe al instalar y al
  reiniciar. Para cambiar la unidad, usa `sudo systemctl edit tramtai`: su drop-in se lee después y gana.
- **Si antes lo instalaste con el `.tar.gz`,** instalar con el paquete quita lo que dejó aquella instalación (la unidad
  de `/etc/systemd/system`, el envoltorio de `/usr/local/bin` y los `.anterior`), pero solo si son de Tramtai: una
  unidad ajena con el mismo nombre no se toca.

**Qué hace `instalar`:**

1. comprueba que eres root y que hay systemd;
2. enseña el aviso y pide escribir «acepto» (`--acepto-terminos` para instalar sin preguntas);
3. copia `tramtai` y `uv` a `/opt/tramtai/` (con el paquete, ya están) y deja `tramtai-servidor` en `/usr/local/bin`
   (`/usr/bin` con el paquete);
4. crea la cuenta `tramtai`, sin shell ni contraseña, y `/var/lib/tramtai` (0700, de esa cuenta) con una configuración
   que solo escucha en este equipo;
5. escribe la unidad, la habilita, la arranca y dice cómo seguir: la contraseña, HTTPS o un proxy, y las claves.

Repetirlo es seguro: con la misma versión no cambia nada; con otra, actualiza. La unidad corre con la cuenta `tramtai`,
`NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome` y escritura solo en su carpeta, entre otras protecciones
(`systemd-analyze security tramtai` las enseña).

## Instalar en Windows Server

Necesitas una consola de administrador. Sirve en Server Core: el servicio no necesita WebView2 ni escritorio.

```powershell
# En la carpeta con el .zip portable y SHA256SUMS.txt de Releases
(Get-FileHash .\Tramtai-*-portable-windows-x64.zip -Algorithm SHA256).Hash   # compárala con SHA256SUMS.txt
Expand-Archive .\Tramtai-*-portable-windows-x64.zip -DestinationPath .\tramtai
.\tramtai\tramtai.exe servidor instalar
```

También vale desde el instalador de Windows o desde cualquier copia de `tramtai.exe`. La instalación:

- copia el programa a `C:\Program Files\Tramtai Servidor\`, con `tramtai-servidor.cmd` en su carpeta `bin`, y añade solo`n  `bin` al PATH del sistema (abre una consola nueva para usarlo): así el `tramtai.exe` del servidor no tapa al de`n  escritorio;
- crea el servicio `Tramtai` («Tramtai (servidor)») con la cuenta virtual `NT SERVICE\Tramtai`, arranque automático
  retrasado y reinicio si se cae;
- crea `%ProgramData%\Tramtai`, que solo pueden usar `SYSTEM`, los administradores y la cuenta del servicio.

Sin internet, `tramtai servidor instalar --conversor <Tramtai-…-conversor-windows-x64.zip>` prepara el conversor de
archivos desde el paquete. La cuenta virtual guarda su perfil en `C:\Windows\ServiceProfiles\Tramtai`, que es donde
los CLI dejarán su sesión (está pendiente de comprobar con cada uno: ver [suscripción](#suscripción)).

## Administrar desde el propio servidor

`tramtai-servidor <orden>` equivale a `tramtai --servidor <orden>` con el programa del servidor: `sudo tramtai-servidor
…` en Linux, o desde una consola de administrador en Windows. Es un nombre aparte para no tapar el `tramtai` de tu
usuario, que puede ser de otra versión.

```sh
sudo tramtai-servidor estado              # en marcha, versión, HTTPS, avisos (también si hay una versión nueva)
sudo tramtai-servidor registro -f         # el registro del servicio, siguiéndolo
sudo tramtai-servidor reiniciar           # aplica los cambios de config.yaml, dejando terminar las peticiones
sudo tramtai-servidor                     # la terminal del servidor, con la marca «Servidor»
sudo tramtai-servidor abrir               # la Web UI del servidor, ya dentro, con un código de un solo uso
```

- **Las órdenes en inglés también valen:** `install`, `uninstall`, `status`, `restart`, `log`, `update`, `admin
  password`, `keys set`, `agents login`…
- **La configuración** está en `/var/lib/tramtai/config.yaml` o `%ProgramData%\Tramtai\config.yaml`. Las claves
  `servicio.*` se cambian ahí y se aplican con `tramtai-servidor reiniciar`. No se editan desde la Web UI: un cambio de
  red mal hecho te dejaría fuera.
- **Los agentes los ejecuta el servicio, con su cuenta.** Por eso, en el servidor, se rechazan `sudo tramtai-servidor
  agentes` a secas (el diagnóstico, que ejecuta cada CLI), `ejecutar` sin el servicio y `servicio iniciar` en primer
  plano: con privilegios, ejecutarían como root (o como administrador) los programas que dice una configuración que la
  cuenta del servicio puede escribir. `agentes entrar`, `agentes suscripcion` y `agentes desconectar` sí valen:
  `entrar` lanza el CLI con la cuenta del servicio, `suscripcion` cambia la configuración y `desconectar` se lo pide al
  servicio.
- **Con `--servidor`, la CLI nunca detiene ni sustituye el servicio del sistema** por ser de otra versión: avisa y remite
  a `tramtai-servidor actualizar`.

## Abrir a la red

Al instalarlo, el servicio solo escucha en este equipo. Queda **abierto a la red** (expuesto) si `servicio.host` no es
de este equipo o si `servicio.proxy.confiables` no está vacío. El servicio **no arranca**, y dice qué hacer, si:

- `servicio.host` no es de este equipo y falta `servicio.permitir_remoto`;
- `servicio.host` no es de este equipo y no hay HTTPS propio (`servicio.tls.modo: ninguno`): las claves viajarían en
  claro. Un proxy en otra máquina también necesita HTTPS hasta Tramtai;
- está abierto a la red con una suscripción que no es la personal, o con la personal sin TOTP;
- `letsencrypt` sin `servicio.dominios`, sin correo o sin `servicio.puerto: 443`;
- `archivos` con el certificado o la clave ilegibles, o con un certificado que no cubre `servicio.dominios`.

El motivo queda en el registro (`tramtai-servidor registro`). Con la configuración mal, el sistema no lo vuelve a
arrancar una y otra vez (fallaría igual): corrígela y `sudo tramtai-servidor reiniciar`.

**Cambio de la v0.58:** quien usaba `servicio.permitir_remoto` por HTTP plano verá que el servicio ya no arranca. El
mensaje ofrece las salidas: un proxy, el HTTPS propio o un túnel SSH.

En todos los modos, `Host` y `Origin` solo valen para este equipo y los nombres de `servicio.dominios`, y las claves
falsas en las APIs tienen freno (30 por dirección en un minuto dan un minuto de espera, con 429). El primer arranque
abierto a la red escribe el riesgo 3 en el registro y en `servidor estado`.

### Detrás de un proxy (la forma recomendada)

El proxy pone el HTTPS y Tramtai escucha solo en este equipo:

```yaml
# /var/lib/tramtai/config.yaml (Linux) o %ProgramData%\Tramtai\config.yaml (Windows)
servicio:
  host: 127.0.0.1
  dominios: [tramtai.example.com]
  proxy:
    confiables: [127.0.0.1/32]   # la dirección exacta del proxy
```

Pon la dirección exacta de cada proxy (como `/32` o `/128`). Una red entera también vale, pero cualquier equipo de esa
red podría falsificar las cabeceras: Tramtai solo rechaza las más anchas que /8 en IPv4 o /32 en IPv6, y eso es un
límite, no una recomendación.

`X-Forwarded-For`, `X-Forwarded-Proto` y `X-Forwarded-Host` solo se creen si la conexión viene de una dirección de
`servicio.proxy.confiables`; de cualquier otra se ignoran. La dirección del cliente (para el registro y los frenos) es
la última de `X-Forwarded-For` que no es de confianza. Sin `X-Forwarded-Proto: https` no se puede iniciar sesión.

Un ejemplo por proxy, cada uno con TLS 1.3 y sin búfer para el flujo en vivo (SSE):

| Proxy | Ejemplo | Comprobado |
|---|---|---|
| Caddy | [`proxies/caddy/Caddyfile`](servidor/proxies/caddy/Caddyfile) | En el CI, en cada cambio (la prueba «Proxies») |
| nginx | [`proxies/nginx/tramtai.conf`](servidor/proxies/nginx/tramtai.conf) | En el CI, en cada cambio |
| Apache httpd | [`proxies/apache/tramtai.conf`](servidor/proxies/apache/tramtai.conf) | En el CI, en cada cambio |
| Traefik | [`proxies/traefik/tramtai.yml`](servidor/proxies/traefik/tramtai.yml) | En el CI, en cada cambio |
| HAProxy | [`proxies/haproxy/haproxy.cfg`](servidor/proxies/haproxy/haproxy.cfg) | En el CI, en cada cambio |
| IIS (ARR y URL Rewrite) | [`proxies/iis/web.config`](servidor/proxies/iis/web.config) | Pendiente: se comprueba a mano antes de publicar |

El CI comprueba con cada uno que solo acepta TLS 1.3, que hay una sola cabecera HSTS, que se inicia sesión por el
proxy, que el flujo en vivo llega a trozos, que Tramtai anota la dirección real del cliente y que un `X-Forwarded-For`
falso enviado directamente a Tramtai no cuela.

**Cualquier otro proxy** sirve si:

- termina el HTTPS con TLS 1.3;
- envía `X-Forwarded-Proto: https` y **sustituye** (no añade a) el `X-Forwarded-For` que mande el cliente;
- conserva el `Host` original (o envía `X-Forwarded-Host`), que debe estar en `servicio.dominios`;
- no guarda en búfer las respuestas (el flujo en vivo llega trozo a trozo) y no corta las peticiones largas;
- no añade HSTS: lo pone Tramtai.

**El proxy en el mismo equipo:**

- con proxies de confianza, abrir a mano el puerto público de Tramtai en el propio servidor (por ejemplo,
  `http://127.0.0.1:17373` en su navegador) cuenta como entrar desde fuera: pide la contraseña y HTTPS;
- `tramtai-servidor abrir`, la tecla `W` de la terminal y la app entran por la entrada local, que el servicio abre
  aparte en `127.0.0.1`;
- sin contraseña de administrador no se entra desde fuera;
- el proxy tiene que apuntar al puerto real de Tramtai: si el 17373 está ocupado, el servidor usa el siguiente libre.
  Fíjalo con `servicio.puerto` y `servicio.puerto_automatico: false`, o mira el que usa en `tramtai-servidor estado`.

**El proxy en otra máquina:** Tramtai necesita además su propio HTTPS (`servicio.host` fuera de este equipo,
`servicio.permitir_remoto: true` y `servicio.tls.modo: archivos`), el proxy habla con él por `https://` y **su
dirección va en `servicio.proxy.confiables`**. Si falta, Tramtai ignora su `X-Forwarded-For`: todos los clientes
cuentan como la dirección del proxy, comparten los frenos (5 fallos de uno bloquean a todos) y el registro anota la IP
del proxy en lugar de la del cliente.

### Túneles

Cloudflare Tunnel y Tailscale Serve o Funnel publican el servidor sin abrir puertos. Están en
[túneles](servidor/tuneles.md), con lo que hay que saber de cada uno:

- con Cloudflare, el HTTPS termina en Cloudflare, que ve las peticiones y las claves;
- Cloudflare corta a los 125 s una respuesta que no ha empezado (error 524): en las peticiones largas, usa
  `stream: true` o la API propia.

Cloudflare Tunnel y Tailscale Serve: Pendiente: se comprueba a mano antes de publicar.

### Certificado propio

Tramtai pone su HTTPS con tu certificado y tu clave en PEM:

```yaml
servicio:
  host: 0.0.0.0
  permitir_remoto: true
  dominios: [tramtai.example.com]
  tls:
    modo: archivos
    certificado: /etc/tramtai/fullchain.pem
    clave: /etc/tramtai/privkey.pem
```

- La cuenta del servicio tiene que poder leer los dos archivos, y el certificado tiene que cubrir `servicio.dominios`.
- Se vuelven a leer al cambiar, sin reiniciar: basta con sustituirlos al renovar.
- Solo TLS 1.3, con intercambio de claves poscuántico (X25519MLKEM768, y X25519 si el cliente no lo tiene) y HSTS de un
  año. Un cliente sin TLS 1.3 no conecta: ponlo detrás de un proxy.

### Let’s Encrypt

Opcional: Tramtai saca y renueva el certificado solo, con el desafío TLS-ALPN-01 en el puerto 443 (no abre el 80).

```yaml
servicio:
  host: 0.0.0.0
  puerto: 443
  permitir_remoto: true
  dominios: [tramtai.example.com]
  tls:
    modo: letsencrypt
    letsencrypt:
      correo: tu-correo@example.com
```

- El 443 tiene que llegar desde internet hasta este equipo, y el nombre tiene que apuntar a él.
- Solo pide certificados para los nombres de `servicio.dominios`. Los guarda en `datos/acme` (0700); son ECDSA P-256.
- En Linux, `servidor reiniciar` da a la unidad el permiso de escuchar el 443 (`CAP_NET_BIND_SERVICE`); sin Let’s
  Encrypt ni un puerto menor de 1024, no lo tiene.

## Administrador y TOTP

```sh
sudo tramtai-servidor administrador contraseña     # la pide dos veces; 12 caracteres o más
sudo tramtai-servidor administrador totp activar   # el secreto, el enlace otpauth:// y 10 códigos de recuperación
sudo tramtai-servidor administrador totp quitar
```

- De la contraseña solo se guarda su huella argon2id, en `datos/administrador`. Cambiarla cierra todas las sesiones.
- **Sin contraseña no se entra desde otros equipos:** la Web UI solo se abre en el propio servidor, con `abrir` o la
  tecla `W`.
- El TOTP (RFC 6238) es opcional, salvo con la suscripción personal abierta a la red. Su secreto va al almacén cifrado
  del servicio. Los 10 códigos de recuperación valen una vez cada uno y se guardan como huella.
- **En Linux** el TOTP se activa y se quita solo con la CLI, porque el servicio no puede escribir en `systemd-creds`. **En
  Windows**, también en **Ajustes → Seguridad**.
- **Ajustes → Seguridad** cambia la contraseña (pide la actual), crea códigos de recuperación nuevos y cierra todas las
  sesiones. Quitar el TOTP o crear códigos nuevos pide la contraseña y un código de la app o de recuperación.
- **Ajustes → Servidor** enseña, solo para leer, el modo de HTTPS, los dominios, los proxies de confianza, cuándo vence
  el certificado y si hay administrador con TOTP, cada dato con la orden que lo cambia.

**La página de inicio de sesión** solo acepta la contraseña por HTTPS (el propio o un proxy de confianza con
`X-Forwarded-Proto: https`); si no, explica por qué y ofrece el código de un solo uso. Dice lo mismo si falla la
contraseña o el código. Tras 5 fallos desde una dirección, esa dirección espera 15 minutos; con 100 fallos en 10 minutos
en total, todos esperan 5 minutos, salvo las redes que entraron bien en los últimos 30 días. La sesión caduca tras 12
horas sin uso y no es local: no guarda claves ni ve las carpetas del servidor. El registro anota cada inicio de
sesión, bueno o malo, y cada bloqueo, con la dirección del cliente.

## Claves y suscripción

### Claves de los agentes

Se guardan con la CLI, en el propio servidor; nunca desde la Web UI, que dice qué clave falta y qué orden la pone:

```sh
sudo tramtai-servidor claves guardar ANTHROPIC_API_KEY   # la pide sin mostrarla
sudo tramtai-servidor claves listar                      # de dónde sale cada una, sin mostrarla
sudo tramtai-servidor claves borrar ANTHROPIC_API_KEY
```

- **Linux:** se cifra con `systemd-creds` en `/etc/credstore.encrypted/tramtai.<NOMBRE>` (con TPM2 si lo hay), se añade
  a `/etc/systemd/system/tramtai.service.d/claves.conf` y el servicio se reinicia dejando terminar las peticiones;
  systemd la entrega al arrancar en `$CREDENTIALS_DIRECTORY`. Con un systemd anterior a la 250, va a
  `/etc/tramtai/claves.env` (de root, 0600) y Tramtai avisa al guardarla y en `servidor estado` de que esa clave está en
  claro en el disco.
- **Windows:** la CLI se la entrega al servicio por su API de administración y el servicio la guarda en el Administrador
  de credenciales de su cuenta. Vale sin reiniciar.

**La clave del servicio** (`datos/token-servicio`) es la que usan la CLI, la terminal y la app para administrar el
servicio. Solo la leen la cuenta del servicio y los administradores, y con el servicio abierto a la red **solo vale
desde el propio equipo**: desde fuera, cualquier ruta responde 403.

### Los CLI de los agentes

- Instálalos para todo el equipo (por ejemplo, `sudo npm install -g @anthropic-ai/claude-code`), o pon su ruta en
  `comandos_agentes.<agente>`.
- Su configuración y su sesión van en la carpeta de la cuenta del servicio, nunca en la tuya.

### Suscripción

Por defecto, los agentes usan su clave de API. Tramtai **nunca guarda la credencial de sesión de Claude** (como la de
`claude setup-token`: Anthropic lo prohíbe) ni lee lo que guarda un CLI: el inicio de sesión lo hace siempre el propio
CLI, con la cuenta del servicio.

```sh
sudo tramtai-servidor agentes entrar claude    # claude auth login, como la cuenta tramtai
sudo tramtai-servidor agentes entrar codex     # codex login --device-auth, con su CODEX_HOME privado
sudo tramtai-servidor agentes entrar copilot   # el inicio de sesión por dispositivo de Copilot
```

- **Codex** necesita que actives el inicio por código de dispositivo en la seguridad de tu cuenta de ChatGPT (en beta).
- **Copilot** también vale con un «fine-grained personal access token» con el permiso «Copilot Requests»:
  `sudo tramtai-servidor claves guardar COPILOT_GITHUB_TOKEN`.
- **En Windows Server** el inicio de sesión con la cuenta del servicio está pendiente de comprobar: `agentes entrar` lo
  dice y, mientras, usa la clave de API del agente.

**Solo en este equipo** (el servicio sin abrir a la red), la suscripción se activa como en el escritorio.

**Abierto a la red,** solo la **suscripción personal**, por agente:

```sh
sudo tramtai-servidor agentes suscripcion claude --personal
```

- Antes enseña las condiciones y pide confirmarlas: solo la usas tú, el titular de la cuenta; no repartes las claves de
  las APIs que usan ese agente; uso ordinario. También los enlaces a los términos del proveedor
  ([Anthropic](https://code.claude.com/docs/en/legal-and-compliance), [OpenAI](https://openai.com/policies/),
  [GitHub](https://docs.github.com/en/site-policy/github-terms/github-terms-of-service)) y el riesgo de suspensión.
- Exige administrador con TOTP: sin él, el servicio no arranca con esa combinación, y quitar el TOTP se rechaza
  mientras haya suscripción personal.
- Queda en `adaptadores.<agente>.suscripcion_personal: true`, una clave de seguridad: no se acepta en el `local.yaml` de
  un proyecto ni desde una sesión remota de la Web UI.
- La Web UI y la terminal lo dicen siempre: «Este servidor usa tu suscripción de <agente>: solo para ti».
- `adaptadores.<agente>.suscripcion_simultaneas` (1 por defecto, hasta 3) limita las peticiones a la vez con esa
  suscripción en todo el servidor, sumando todas las APIs y los espacios; las demás esperan en orden. Subirlo enseña el
  aviso de riesgo y pide confirmarlo.

### Codex en el servidor

Codex se ejecuta en solo lectura (`-s read-only`), y en solo lectura **lee todo lo que lee la cuenta del servicio**. Quien
tenga una clave que pueda usar Codex (la de una API que lo permita o la **clave del espacio**, que puede usar cualquier
agente) puede pedirle que lea las claves de los proveedores de `$CREDENTIALS_DIRECTORY`, el secreto `TRAMTAI_TOTP` y
`datos/administrador` (la huella de la contraseña). Tramtai tapa las claves en las respuestas, pero no detiene una clave
codificada (por ejemplo, en base64). Por eso:

- da la clave del espacio y la de una API que pueda usar Codex solo a personas de confianza, como si les dieras esas
  claves;
- para las APIs de otras personas, pon en **Modelos permitidos** de esa API (`modelos_permitidos`) solo modelos de
  agentes que no son Codex (vacía, deja usar cualquier agente, también Codex) y no pongas Codex en su respaldo;
- o desconecta Codex en el servidor (`sudo tramtai-servidor agentes desconectar codex`).

## Actualizar

El servidor comprueba cada hora si hay una versión nueva y lo avisa en la Web UI, la terminal y `servidor estado`.

```sh
sudo tramtai-servidor actualizar
```

- Comprueba la firma de Tramtai de `SHA256SUMS.txt` (ed25519) y la suma SHA-256 del `.tar.gz` de su arquitectura o del
  `.zip` portable de Windows; sin una firma válida, no instala nada.
- Detiene el servicio dejando terminar las peticiones, cambia el programa, guarda el anterior como `.anterior` y arranca
  la versión nueva, que regenera la unidad.
- Si lo lanzas desde otra copia de `tramtai`, delega en el programa instalado.
- **Con el paquete `.deb` o `.rpm`,** `actualizar` remite a `apt` o `dnf`: el paquete nuevo reinicia el servicio solo si
  ya estaba habilitado, dejando terminar las peticiones.
- No se actualiza desde la Web UI: el servicio no puede escribir en la carpeta de su programa.

## Desinstalar

```sh
sudo tramtai-servidor desinstalar                  # quita el servicio, el programa y la unidad; conserva los datos
sudo tramtai-servidor desinstalar --borrar-datos   # también los datos, las claves cifradas y la cuenta (pide confirmar)
```

- **Con el paquete,** `desinstalar` deshabilita el servicio y quita su drop-in, pero el programa lo quita `apt remove` o
  `dnf remove`. Quitar el paquete (también con `apt purge`) conserva `/var/lib/tramtai` y las credenciales: para
  borrarlos, `desinstalar --borrar-datos` antes.
- **En Windows,** el perfil de la cuenta virtual (`C:\Windows\ServiceProfiles\Tramtai`, con las sesiones de los CLI)
  queda cargado hasta reiniciar el equipo; reinstalar antes de reiniciar recupera esas sesiones.

## Si algo falla

| Qué pasa | Qué hacer |
|---|---|
| El servicio no arranca | `sudo tramtai-servidor estado` y `registro`: el mensaje dice qué regla no se cumple y qué cambiar en `config.yaml`. |
| «Aquí no se entra con contraseña» | La conexión no llega por HTTPS. Usa `https://`, revisa que el proxy envíe `X-Forwarded-Proto: https` y que su dirección esté en `servicio.proxy.confiables`. |
| Por el proxy no se puede entrar | Mira el registro (`tramtai-servidor registro`): `web ui: acceso correcto del administrador desde X`, `web ui: acceso fallido desde X` o `web ui: intento de acceso sin HTTPS desde X`. Si X es la dirección del proxy y no la tuya, el proxy no envía `X-Forwarded-For` o no está en `servicio.proxy.confiables`; si sale «sin HTTPS», no envía `X-Forwarded-Proto: https`. El `Host` tiene que estar en `servicio.dominios`. |
| «Demasiados intentos» | Tras 5 fallos, esa dirección espera 15 minutos. Desde el servidor, `abrir` sigue entrando. |
| Error 524 de Cloudflare | La respuesta no empezó en 125 s: usa `stream: true` o la API propia en las peticiones largas. |
| Un cliente no conecta al HTTPS propio | No tiene TLS 1.3. Pon un proxy delante. |
| La CLI dice que el servidor es de otra versión | `sudo tramtai-servidor actualizar` (o `apt`/`dnf` con el paquete). La CLI no lo detiene por ti. |
| `agentes`, `ejecutar` o `servicio iniciar` se rechazan | En el servidor, los agentes los ejecuta el servicio con su cuenta: usa la Web UI, la terminal o una API. |
| `agentes entrar` falla en Windows Server | Su inicio de sesión con la cuenta del servicio está pendiente de comprobar: usa su clave de API (`claves guardar`). |
| «este systemd no tiene systemd-creds 250 o posterior: las claves del servidor están sin cifrar en /etc/tramtai/claves.env (0600, solo las lee root)» | Lo dicen `claves guardar` y `servidor estado`: tu systemd es anterior a la 250 y la clave queda sin cifrar en ese archivo, que solo lee root. Con systemd 250 o posterior, las claves se cifran con `systemd-creds`. |

Más en la Ayuda de la app (temas «Modo servidor» y «Dos entornos») y en [arquitectura](arquitectura.md#el-servidor).

---

# Server guide

Server mode is a system-wide Tramtai: it starts with the computer, with nobody signed in, with its own account, data
and port, and opens from other computers over HTTPS with the administrator password. It runs on Linux with systemd and
on Windows Server (Server Core too). macOS has no system service: there you have “Just the service”, which starts when
you sign in.

It’s **a personal server with a single administrator**. Several accounts, per-person permissions and signing in with
your company account aren’t in this version.

## Notice

Before installing, `tramtai server install` shows these five risks and asks you to type “acepto” (Spanish for “I
accept”):

<!-- riesgos-servidor:en -->
1. Your account with a provider may be limited, suspended or closed if you use your subscription with Tramtai, and the provider may change its rules without notice. The author cannot recover it.
2. Tramtai’s conditions (TOTP, one request at a time) lower the risk but do not remove it, and they are not a legal opinion on the provider’s terms.
3. What you open to the network is your responsibility: HTTPS, firewall, passwords, backups and updates.
4. If other people use your server, you answer for their data under the law that applies to you.
5. Tramtai is free and provided as is, without warranties.
<!-- /riesgos-servidor:en -->

The full text is in the [terms of use](../TERMINOS.md#terms-of-use) (version 2, [publication date]). The acceptance
is stored in the server’s `datos/terminos.json`; your desktop Tramtai’s doesn’t count for it.

## Two environments

Your Tramtai and the server’s **share nothing**: each has its own workspaces, APIs, keys, CLI sessions, history and
settings. An API created on your computer doesn’t exist on the server, or the other way around.

| | Your Tramtai | The server’s |
|---|---|---|
| Starts | When you sign in (“The app in the tray” or “Just the service”) or when you open it | With the computer, without a session |
| Account | Yours | `tramtai` (Linux) or `NT SERVICE\Tramtai` (Windows) |
| Data | Your Tramtai folder (or `datos-tramtai` in the portable copy) | `/var/lib/tramtai` or `%ProgramData%\Tramtai` |
| Program | Wherever you installed it | `/opt/tramtai/` or `C:\Program Files\Tramtai Servidor\` |
| Port | 17373 by default | 17373, or the next free one if yours is using it |
| Agents’ keys | Your session’s encrypted store | The service account’s store (Windows) or `systemd-creds` (Linux) |
| Managed with | `tramtai …`, the app, the terminal and the Web UI | `tramtai-servidor …` as administrator, and the Web UI |
| Web UI from another computer | No, by default | Yes, over HTTPS with the administrator password |

You can always tell which one you’re in: the Web UI header and the terminal’s status line say “Server” or “This
device”; About shows the environment, the account and the folder; with `--server`, every CLI output that creates or
changes something starts with “Server:”; and if the system one also runs on your computer, your app, your terminal and
`tramtai servicio estado` warn you that it’s another environment, with its own APIs and keys.

## Install on Linux

You need systemd and root (`sudo`).

**With the `.tar.gz`:** download your architecture’s file and `SHA256SUMS.txt` from
[Releases](https://github.com/MscottB/Tramtai/releases/latest), check the sum and run the installer:

```sh
sha256sum -c --ignore-missing SHA256SUMS.txt
tar xzf tramtai-*-linux-amd64.tar.gz
cd tramtai
sudo ./tramtai server install
```

**With the server package** (`Tramtai-…-servidor-linux-amd64.deb` or `.rpm`, arm64 too; it can sit next to the desktop
package without clashing): `sudo apt install ./Tramtai-*-servidor-linux-amd64.deb` (or `dnf install` with the `.rpm`)
and then `sudo tramtai-servidor install`.

- **The package starts nothing:** it can’t ask you to accept the terms. `sudo tramtai-servidor install` asks for the
  acceptance, creates the settings, enables and starts the service.
- **Its settings go in a drop-in,** `/etc/systemd/system/tramtai.service.d/10-tramtai.conf`, which Tramtai rewrites on
  install and restart. To change the unit, use `sudo systemctl edit tramtai`: its drop-in is read later and wins.
- **If you installed from the `.tar.gz` before,** installing with the package removes what that installation left (the
  unit in `/etc/systemd/system`, the wrapper in `/usr/local/bin` and the `.anterior` files), but only if they are
  Tramtai’s: someone else’s unit with the same name is left alone.

`install` checks you’re root, shows the notice, copies `tramtai` and `uv` to `/opt/tramtai/` (the package already has
them), creates the `tramtai` account and `/var/lib/tramtai` (0700) with settings that only listen on this computer,
writes and starts the hardened unit, and tells you what’s next: the password, HTTPS or a proxy, and the keys. Running it
again is safe: same version, nothing changes; another version, it updates.

## Install on Windows Server

From an administrator console (Server Core works too):

```powershell
(Get-FileHash .\Tramtai-*-portable-windows-x64.zip -Algorithm SHA256).Hash   # compare it with SHA256SUMS.txt
Expand-Archive .\Tramtai-*-portable-windows-x64.zip -DestinationPath .\tramtai
.\tramtai\tramtai.exe server install
```

It copies the program to `C:\Program Files\Tramtai Servidor\`, with `tramtai-servidor.cmd` in its `bin` folder, and adds
only `bin` to the system PATH (open a new console to use it), so the server’s `tramtai.exe` doesn’t shadow the desktop one;
it creates the `Tramtai` service (“Tramtai (servidor)”) with the virtual account
`NT SERVICE\Tramtai`, delayed automatic start and restart on failure, and creates `%ProgramData%\Tramtai`, usable only by
`SYSTEM`, administrators and the service account. Offline, `--conversor <zip>` prepares the file converter from its
package.

## Manage from the server itself

`tramtai-servidor <command>` is `tramtai --server <command>` with the server’s program: `sudo tramtai-servidor …` on
Linux, or from an administrator console on Windows. `status`, `log -f`, `restart` (applies `config.yaml` changes, letting
requests finish), `abrir` (the server’s Web UI, already signed in) and no command at all (the server’s terminal) are the
everyday ones.

- **The settings** live in `/var/lib/tramtai/config.yaml` or `%ProgramData%\Tramtai\config.yaml`; `servicio.*` changes
  apply with `tramtai-servidor restart`. They aren’t edited from the Web UI: a bad network change would lock you out.
- **The service runs the agents, with its own account.** On the server, plain `sudo tramtai-servidor agents` (the
  diagnosis, which runs each CLI), `ejecutar` without the service and `servicio iniciar` in the foreground are refused
  (`agents login`, `agents subscription` and `agents desconectar` do work): with privileges, they would run as root (or
  administrator) the programs named in settings the service account can write.
- **With `--server`, the CLI never stops or replaces the system service** for being another version: it warns you and
  points to `tramtai-servidor update`.

## Open to the network

After installing, the service only listens on this computer. It’s **open to the network** (exposed) when
`servicio.host` isn’t on this computer or `servicio.proxy.confiables` isn’t empty. The service **won’t start**, and says
what to do, with a non-local `servicio.host` and no `servicio.permitir_remoto`; with a non-local `servicio.host` and no
HTTPS of its own (keys would travel unencrypted; a proxy on another machine also needs HTTPS to Tramtai); exposed with a
subscription that isn’t the personal one, or personal without TOTP; `letsencrypt` without domains, email or port 443;
and `archivos` with unreadable files or a certificate that doesn’t cover `servicio.dominios`. The reason goes to the
log (`tramtai-servidor log`). With a bad configuration the system doesn’t keep restarting it (it would fail the same way):
fix it and run `sudo tramtai-servidor restart`.

**v0.58 change:** if you used `servicio.permitir_remoto` over plain HTTP, the service no longer starts. The message
offers the ways out: a proxy, its own HTTPS or an SSH tunnel.

### Behind a proxy (recommended)

The proxy provides HTTPS and Tramtai listens only on this computer: `servicio.host: 127.0.0.1`, your name in
`servicio.dominios` and the proxy’s exact address in `servicio.proxy.confiables` (as in the Spanish example above; a
whole network also works, but any computer in it could forge the headers: the /8 or /32 limit Tramtai enforces is a
limit, not a recommendation). Forwarded
headers only count from those addresses; without `X-Forwarded-Proto: https` nobody can sign in. The examples, each with
TLS 1.3 and no buffering for the live stream (SSE): [Caddy](servidor/proxies/caddy/Caddyfile),
[nginx](servidor/proxies/nginx/tramtai.conf), [Apache httpd](servidor/proxies/apache/tramtai.conf),
[Traefik](servidor/proxies/traefik/tramtai.yml) and [HAProxy](servidor/proxies/haproxy/haproxy.cfg) are checked by CI on
every change; [IIS](servidor/proxies/iis/web.config) (ARR and URL Rewrite): Pending: checked by hand before release.

**Any other proxy** works if it ends HTTPS with TLS 1.3, sends `X-Forwarded-Proto: https`, **replaces** (doesn’t append
to) the client’s `X-Forwarded-For`, keeps the original `Host` (which must be in `servicio.dominios`), doesn’t buffer
responses or cut long requests, and doesn’t add HSTS (Tramtai does).

**A proxy on the same computer:** with trusted proxies, opening Tramtai’s public port by hand on the server itself counts
as coming from outside (it asks for the password and HTTPS); `tramtai-servidor abrir`, the terminal’s `W` key and the
app come in through the local entrance the service opens separately on `127.0.0.1`; and without an administrator
password nobody gets in from outside. The proxy must point at Tramtai’s real port: if 17373 is taken, the server uses
the next free one, so pin it with `servicio.puerto` and `servicio.puerto_automatico: false`, or check it in
`tramtai-servidor status`.

**A proxy on another machine:** Tramtai also needs its own HTTPS (`servicio.tls.modo: archivos`), the proxy talks to
it over `https://`, and **its address goes in `servicio.proxy.confiables`**. Without it, Tramtai ignores its
`X-Forwarded-For`: every client counts as the proxy’s address, they all share the brakes (5 failures from one lock out
everyone) and the log notes the proxy’s IP instead of the client’s.

### Tunnels

Cloudflare Tunnel and Tailscale Serve or Funnel are in [tunnels](servidor/tuneles.md). With Cloudflare, HTTPS ends at
Cloudflare, which sees the requests and the keys, and a response that hasn’t started within 125 s ends in error 524: use
`stream: true` or the native API for long requests.
Cloudflare Tunnel and Tailscale Serve: Pending: checked by hand before release.

### Your own certificate

`servicio.tls.modo: archivos` with `servicio.tls.certificado` and `servicio.tls.clave` in PEM (and a non-local host with
`servicio.permitir_remoto: true`). The service account must be able to read both files and the certificate must cover
`servicio.dominios`. They’re read again when they change, without a restart. TLS 1.3 only, with post-quantum key
exchange (X25519MLKEM768, falling back to X25519) and one-year HSTS; a client without TLS 1.3 can’t connect: put a proxy
in front.

### Let’s Encrypt

Optional: `servicio.tls.modo: letsencrypt` with `servicio.tls.letsencrypt.correo`, your names in `servicio.dominios` and
`servicio.puerto: 443`. Tramtai gets and renews the certificate itself with the TLS-ALPN-01 challenge on port 443 (it
doesn’t open port 80), only for the names in `servicio.dominios`, cached in `datos/acme` (0700), ECDSA P-256. Port 443
must be reachable from the internet. On Linux, `server restart` gives the unit permission to listen on 443; without
Let’s Encrypt or a port below 1024, it doesn’t have it.

## Administrator and TOTP

`sudo tramtai-servidor admin password` sets or changes the password (asked twice, 12 characters or more; only its
argon2id hash is stored, and changing it closes every session). `admin totp enable` shows the secret, the `otpauth://`
link and 10 single-use recovery codes; `admin totp disable` removes it. **Without a password nobody signs in from
other computers.** TOTP is optional, except with the personal subscription open to the network. **On Linux** TOTP is
turned on and off only with the CLI (the service can’t write to `systemd-creds`); **on Windows**, in **Settings →
Security** too. Security also changes the password (asking for the current one), creates new recovery codes and closes
every session; removing TOTP or creating new codes asks for the password and a code. **Settings → Server** shows,
read-only, the HTTPS mode, domains, trusted proxies, certificate expiry and the administrator, each with the command that
changes it.

The sign-in page only takes the password over HTTPS; it says the same whether the password or the code fails; 5
failures from one address make it wait 15 minutes, and 100 failures in 10 minutes in total make everyone wait 5 minutes,
except networks that signed in fine in the last 30 days. Sessions expire after 12 hours idle and aren’t local: they
don’t store keys or see the server’s folders. The log notes every sign-in, good or bad, and every lockout, with the
client’s address.

## Keys and subscription

**Agents’ keys** are stored with the CLI on the server itself (`sudo tramtai-servidor keys set ANTHROPIC_API_KEY`),
never from the Web UI. On Linux they’re encrypted with `systemd-creds` and the service restarts, letting requests
finish (with systemd older than 250 they go to `/etc/tramtai/claves.env`, in plain text, and Tramtai warns you); on
Windows the service stores them in its account’s Credential Manager, with no restart.

**The service key** (`datos/token-servicio`), used by the CLI, the terminal and the app, is readable only by the service
account and administrators and, with the service open to the network, **only works from the server itself**: from
outside, every route answers 403.

**Agents’ CLIs** must be installed for the whole computer (for example `sudo npm install -g …`) or have their path in
`comandos_agentes.<agent>`; their sessions live in the service account’s folder.

**Subscription:** by default, agents use their API key. Tramtai **never stores a Claude session key** and never reads
what a CLI stores: the CLI signs in itself, with the service account, through `sudo tramtai-servidor agents login
<agent>` (Codex needs device-code sign-in turned on in your ChatGPT security settings; Copilot also takes a fine-grained
personal access token with “Copilot Requests” via `keys set COPILOT_GITHUB_TOKEN`). On Windows Server, signing in with
the service account is still pending a check: use the agent’s API key meanwhile. Open to the network, only the
**personal subscription** (`agents subscription <agent> --personal`): it shows the conditions (only you, the account
holder; you don’t hand out the keys of APIs that use that agent; ordinary use), the provider’s terms
([Anthropic](https://code.claude.com/docs/en/legal-and-compliance), [OpenAI](https://openai.com/policies/),
[GitHub](https://docs.github.com/en/site-policy/github-terms/github-terms-of-service)) and the suspension risk; it
requires an administrator with TOTP; the Web UI and the terminal always show “This server uses your <agent>
subscription: just for you”; and `adaptadores.<agent>.suscripcion_simultaneas` (1 by default, up to 3) caps requests at
a time with it across the whole server.

**Codex on the server:** Codex runs read-only (`-s read-only`), and read-only means it **reads everything the service
account reads**. Whoever has a key that can use Codex (that of an API that allows it, or the **workspace key**, which can use any
agent) can ask it to read the provider keys in `$CREDENTIALS_DIRECTORY`, the `TRAMTAI_TOTP` secret and
`datos/administrador` (the password hash). Tramtai hides keys in responses, but it doesn’t stop an encoded key (base64,
for example). Give the workspace key and the key of any API that can use Codex only to people you trust, as if you were
giving them those keys. For other people’s APIs, put only models of agents other than Codex in that API’s **Allowed
models** (`modelos_permitidos`; left empty, it allows any agent, Codex included) and don’t put Codex in its fallback; or
disconnect Codex on the server (`sudo tramtai-servidor agents desconectar codex`).

## Update

The server checks every hour and tells you in the Web UI, the terminal and `server status`. `sudo tramtai-servidor
update` checks Tramtai’s signature on `SHA256SUMS.txt` (ed25519) and the SHA-256 sum of the package, stops the service
letting requests finish, swaps the program (keeping the old one as `.anterior`) and starts the new version, which
regenerates the unit. Run from another copy of `tramtai`, it hands over to the installed program. **With the `.deb` or
`.rpm`,** it points you to `apt` or `dnf`. It doesn’t update from the Web UI.

## Uninstall

`sudo tramtai-servidor uninstall` removes the service, the program and the unit and keeps the data; `--delete-data`
also deletes the data, the encrypted keys and the account, after asking. With the package, `uninstall` disables the
service and removes its drop-in, and `apt remove` or `dnf remove` removes the program; removing the package (even with
`apt purge`) keeps `/var/lib/tramtai`. On Windows, the virtual account’s profile stays loaded until the computer
restarts.

## If something fails

- **The service won’t start:** `sudo tramtai-servidor status` and `log`; the message says which rule fails.
- **“You can’t sign in with a password here”:** the connection isn’t HTTPS; check that the proxy sends
  `X-Forwarded-Proto: https` and that its address is in `servicio.proxy.confiables`.
- **Can’t sign in through the proxy:** look at the log (`tramtai-servidor log`; its lines are in Spanish):
  `web ui: acceso correcto del administrador desde X`, `web ui: acceso fallido desde X` or `web ui: intento de acceso
  sin HTTPS desde X`. If X is the proxy’s address instead of yours, the proxy doesn’t send `X-Forwarded-For` or isn’t in
  `servicio.proxy.confiables`; “sin HTTPS” means it doesn’t send `X-Forwarded-Proto: https`. The `Host` must be in
  `servicio.dominios`.
- **“this systemd does not have systemd-creds 250 or later: the server keys are unencrypted in /etc/tramtai/claves.env
  (0600, only root reads it)”:** said by `keys set` and `server status`: your systemd is older than 250, so the key stays
  unencrypted in that root-only file. With systemd 250 or later, keys are encrypted with `systemd-creds`.
- **Cloudflare error 524:** the response didn’t start within 125 s; use `stream: true` or the native API.
- **A client can’t connect to Tramtai’s own HTTPS:** it lacks TLS 1.3; put a proxy in front.
- **The CLI says the server is another version:** `sudo tramtai-servidor update` (or `apt`/`dnf`).
- **`agents login` fails on Windows Server:** use the agent’s API key for now.

The app’s Help has the “Server mode” and “Two environments” topics.
