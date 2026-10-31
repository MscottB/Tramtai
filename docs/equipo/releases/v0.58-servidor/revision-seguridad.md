# Revisión de seguridad del modo servidor (v0.58)

Revisión aparte del diff de la rama (`v0.57.0+26102026..91655a1`), hecha por un revisor de seguridad que no escribió
el código, sobre la lista de spec «Revisión de seguridad». Resultado: sin Critical, 3 Important, 6 Minor y 3 Minor
que venían aplazados de tareas anteriores. Todos se arreglan en esta versión, cada uno con su prueba (primero la prueba
que falla y después el arreglo).

## Los diez puntos y sus pruebas

| # | Punto | Estado tras los arreglos | Pruebas |
|---|---|---|---|
| 1 | Cabeceras reenviadas falsificadas | Cubierto (antes, hueco M6) | `red_test.go`: TestClienteReal (también `Forwarded`, `Via`, `X-Real-Ip` y `[v6]` sin puerto), TestCabecerasDeProxyNoSonLocales, TestEsHTTPS, TestProxyLoopbackNoEsLocal, TestOyenteLocalNoCreeCabeceras, TestTokenServicioSoloDesdeEsteEquipo; `administrador_test.go`: TestFrenoNoSeEvadeConXFF, TestAdminTOTP; `admin_claves_test.go`: TestAdminClavesSoloLocal; `origen_test.go`; job Proxies (`probar-proxies.sh`, que ahora crea la cuenta `tramtai` y arranca el servicio con ella) |
| 2 | Host y Origin | Cubierto | `api_test.go` TestSeguridad, `web_test.go` TestWebUI, `cmd/tramtai/servicio_test.go` TestDominiosEnHosts y TestDominiosNormalizados |
| 3 | Fuerza bruta y frenos | Cubierto (antes, fallo I3) | `administrador_test.go`: TestFrenoPorIP, TestFrenoPorIPUnitario, TestFrenoGlobal, TestFrenoGlobalConocidas, TestFrenoAccederALaVez, TestColaArgonLlena, TestContadorSaturado, TestFrenoClavesFalsas, TestClavesFalsasSaturado, TestClavesFalsasDesdeEsteEquipo, TestCodigoActualEnFrenos |
| 4 | TLS y ACME (`HostPolicy`, caché) | Cubierto | `tls_test.go`: TestTLSRechaza12, TestTLSPoscuantico, TestTLSRecarga, TestLetsEncryptHostPolicy, TestLetsEncryptCarpetaExistente, TestLetsEncryptECDSAConTLS13, TestPuntoConPuertoPropioHablaTLS, TestHSTS, TestConfigTLSModos; `tls_pebble_test.go` |
| 5 | Cookie y CSRF | Cubierto (antes, M1, M3 y el aplazado de la cookie) | TestAccederSoloHTTPS, TestCookieConPuerto, TestWebUI, TestCambiarContrasenaCierraSesiones, TestCambiarContrasenaDesdeLaCLI, TestContrasenaCambiaMientrasSeEntra, TestSesionRemotaNoEsLocal, TestEnlaceAbrirConProxyEsLocal, TestActivarTOTPEnDosPasos, TestFlujoTerminaAlCerrarSesiones |
| 6 | Redirecciones del inicio de sesión | Cubierto | TestAccederSinRedireccion, TestCodigoDeUnSoloUso |
| 7 | Tiempos de respuesta | Cubierto por diseño (`subtle.ConstantTimeCompare`, argon2id siempre) | TestAccederMismoMensaje, TestAccederConRecuperacion |
| 8 | Agotamiento de recursos (argon2id, conexiones, flujos) | Cubierto (antes, M2 y M6) | TestColaArgonLlena, TestFrenoAccederALaVez, TestCuerpoUIAcotado, TestServidorHTTP |
| 9 | Permisos de las carpetas y de la unidad | Cubierto (antes, fallos I1 y M4) | `linux_test.go` (TestInstalarNoSigueEnlaces, TestAjustarDueno, TestGuardarClaveSystemd, TestGuardarClaveSinSystemdCreds, TestDesinstalarBorraClaves, TestSinPaqueteQuitaDropIn, TestInstalarPaqueteSobreTarGz), `editar_test.go`, `permisos_windows_test.go`, TestCarpetaPublicaWindows, TestPuertoPublicado, TestPuertoPublicadoNoRegular, TestUnidadSystemd, TestUnidadLetsEncrypt, TestBorrarCredencialesSinSalirDelPerfil, TestLeerCredentialsDirectory, TestExtraerTarSinEnlaces, TestConversionElevadaEnServidor, TestEjecucionElevadaEnServidor, jobs Servidor |
| 10 | La suscripción expuesta | Cubierto | TestComprobarArranque, TestArranqueConTOTP, TestQuitarTOTPConSuscripcionPersonal, TestSuscripcionPersonalCLI, TestSuscripcionServidorExpuesto, TestProxyLoopbackConSuscripcion, TestSimultaneasServidor, TestSuscripcionMientrasEspera, TestSimultaneasTope, TestAutenticacionRemotaProhibida |

## Hallazgos y arreglos

### Important

**I1. La CLI como root o administrador ejecutaba binarios elegidos por la configuración del servicio.**
`comandos_agentes.<agente>` sale de `config.yaml`, que puede escribir la cuenta del servicio (`tramtai` o
`NT SERVICE\Tramtai`). Con `sudo tramtai-servidor agentes`, `ejecutar --directo` (o `ejecutar` sin el servicio en
marcha) y `servicio iniciar` en primer plano, la CLI ejecutaba esos binarios, o el servicio entero, como root.
Arreglo (R36): `ejecucionLocalPermitida`, junto a `conversionLocalPermitida` (R20), en `cmd/tramtai/ejecutar.go`. En el
modo servidor y con privilegios, el diagnóstico de `agentes`, la ejecución directa de `ejecutar` e `iniciarServicio`
(después de `ejecutarComoServicio`, para que el servicio de Windows siga arrancando; bajo systemd el proceso es la
cuenta `tramtai`) lo rechazan y remiten al servicio o a `tramtai-servidor reiniciar`. `ejecutar` por el servicio, que es
lo que usan los jobs «Servidor (Linux)» y «Servidor (Windows Server)» y `probar-servidor.sh`/`.ps1`, sigue igual. El
job «Proxies» (`scripts/servidor/probar-proxies.sh`) arrancaba el servicio como root en su contenedor: ahora crea la
cuenta `tramtai` como la instalación (`useradd --system`), comprueba que las órdenes de administrador le dejan `datos/` a
su nombre (`AjustarDueno`) y arranca el servicio con ella (`docker compose exec -u tramtai`).
Prueba: TestEjecucionElevadaEnServidor (`agentes`, `agents`, `ejecutar --directo`, `ejecutar` sin servicio y
`servicio iniciar`) y el job «Proxies».

**I2. El token de servicio valía desde fuera, y un agente con permiso de lectura puede leerlo.**
Con el token, `POST /api/v1/servicio/sesiones` daba un código de entrada a la Web UI sin contraseña ni TOTP.
Arreglo (R37): en `enrutar`, con el servicio expuesto, el token de servicio que no llega desde este equipo
(`desdeEsteEquipo`) recibe 403 `origen_no_permitido`, antes de mirar la ruta. Cubre `/api/v1/servicio/*` y también las
rutas de un espacio con `Tramtai-Instancia`, que con el mismo token daban acceso de administrador a ese espacio. La CLI,
la terminal y la app usan el oyente local (R24 y R35). Prueba: TestTokenServicioSoloDesdeEsteEquipo (por el proxy, con y
sin `X-Forwarded-For`, 403 en `GET /api/v1/servicio`, `POST /api/v1/servicio/sesiones` y `GET /api/v1/capacidades`; por
el oyente local, 200; sin exponer, nada cambia).

**I3. El freno de claves falsas fallaba cerrado al saturarse.**
Con el contador lleno (10 000 direcciones, p. ej. desde una /48 de IPv6), toda dirección sin cuenta esperaba, también
las apps con su clave válida. Arreglo: `frenarClaves` usa `esperaPropia`. Las claves son de 256 bits y el freno es contra
el abuso, así que falla abierto; el freno de la contraseña sigue fallando cerrado. Prueba: TestClavesFalsasSaturado.

### Minor

**M1. Quitar el TOTP y regenerar los códigos pedían solo la contraseña.** Además, al activarlo, el fallo del código
actual se contaba fuera del hueco de argon2id. Arreglo: `conCodigo`, el `siVale` de `comprobarActual`, comprueba el
segundo factor dentro del hueco y cuenta el fallo con él tomado. Lo usan activar (paso 1), quitar y regenerar; el cuerpo
de quitar y regenerar lleva `codigo`. Quitar mira antes si se puede (`puedeQuitarTOTP`: suscripción personal o Linux),
para no gastar el código en un 409. En Ajustes → Seguridad, con TOTP, la Web UI pide el código después de la
contraseña (`seguridad.codigoActual` y `seguridad.codigoActualTexto` en `es.ts` y `en.ts`). Pruebas:
TestActivarTOTPEnDosPasos (sin código, 400; con uno de recuperación, vale), TestCodigoActualEnFrenos (5 códigos malos
dan 429) y TestQuitarTOTPConSuscripcionPersonal (el código de recuperación sigue valiendo tras el 409).

**M2. `/ui/acceder` y `/ui/sesion` leían hasta 16 MB antes de los frenos.** Arreglo: `rutasUI` limita el cuerpo a
64 KiB con `http.MaxBytesReader`. Prueba: TestCuerpoUIAcotado (1 MB en `/ui/acceder` da 400 sin calcular argon2id).

**M3. Los flujos SSE sobrevivían a «Cerrar todas las sesiones» y al cambio de contraseña.** Arreglo: un contexto por
generación de sesiones (`finGeneracion`), que se cancela al subirla (`nuevaGeneracion`, desde `cerrarSesiones` y
`vigilarAdministrador`). Cada sesión guarda el de su generación y `enrutar` lo enlaza a la petición con
`context.AfterFunc`. Para que un cambio de contraseña desde la CLI, que escribe `datos/administrador` directamente,
corte los flujos sin esperar a otra petición con sesión, `vigilarEnFondo` mira el archivo cada 2 segundos hasta
`Cerrar`. Prueba: TestFlujoTerminaAlCerrarSesiones (cerrar las sesiones y cambiar la contraseña desde fuera).

**M4. `/run/tramtai/puerto` se abría con `os.Open`, que sigue enlaces y se queda colgado con un FIFO.** Arreglo:
`leerPublico` abre con `O_RDONLY|O_NONBLOCK|O_NOFOLLOW` y exige un archivo normal. Prueba (Linux, en Docker):
TestPuertoPublicadoNoRegular, con un FIFO y con un enlace.

**M5. `servicio.proxy.confiables` solo rechazaba `/0`.** Arreglo: se rechazan las redes más anchas que /8 en IPv4 y
/32 en IPv6, con su mensaje en es/en. Prueba: TestConfiables (`0.0.0.0/1`, `128.0.0.0/7`, `::ffff:10.0.0.0/103`,
`::/1`, `2000::/31`).

**M6. Sin prueba de `Forwarded`, `Via` y `X-Real-Ip` ni de los plazos del `http.Server`, repetidos a mano en tres
sitios.** Arreglo: `servicio.NuevoServidorHTTP`, el constructor común del oyente público, el local y el de cada punto con
puerto propio. Pruebas: TestServidorHTTP (plazos y 431 con cabeceras de 128 KiB), TestCabecerasDeProxyNoSonLocales,
TestClienteReal con las tres cabeceras y un caso más en TestProxyLoopbackNoEsLocal.

### Minor aplazados de tareas anteriores

- **La cookie de una sesión muerta.** Si la contraseña cambiaba mientras se comprobaba, `accederUI` respondía 401 con el
  `Set-Cookie` de esa sesión. Arreglo: se quita esa cabecera antes de responder. Prueba: TestContrasenaCambiaMientrasSeEntra.
- **`deSuscripcion` se miraba al empezar a esperar.** Una petición que ya esperaba su turno cuando el agente pasaba a
  suscripción no tomaba su semáforo. Arreglo: en `intentar`, la suscripción se mira al llegarle el turno (tras el de su
  API y el de su agente). No se vuelve a mirar con la plaza de la instancia tomada, porque esperar así la suscripción
  podía bloquear a dos trabajos hasta agotar el tiempo en cola. Queda un exceso pasajero solo si el cambio llega
  mientras el trabajo espera la plaza. Prueba: TestSuscripcionMientrasEspera (esperando su API y esperando la plaza,
  sin bloqueo).
- **`[v6]` sin puerto en `X-Forwarded-For` cortaba la cadena** en `ClienteReal`, y todos los clientes de ese proxy
  compartían freno. Arreglo: se quitan los corchetes antes de leer la dirección. Prueba: el caso nuevo de TestClienteReal.

## Riesgo residual

Codex se ejecuta con `-s read-only`, que, según su documentación, lee todo lo que lee la cuenta del servicio. Tras I2,
el token de servicio ya no sirve desde fuera, pero una clave de una API que use Codex aún puede pedirle que lea
`$CREDENTIALS_DIRECTORY` (las claves de los proveedores y `TRAMTAI_TOTP`) y `datos/administrador`. La redacción de
secretos de la salida no detiene una codificación como base64. Lo documentan la guía del servidor
(`docs/servidor.md`, «Codex en el servidor», y la Ayuda, tema «Modo servidor») y el modelo de amenazas del servidor de
`docs/arquitectura.md`.
