# Túneles delante del servidor de Tramtai

Un túnel publica el servidor sin abrir puertos ni tener un certificado propio: un programa del proveedor corre en el
mismo equipo que Tramtai, se conecta hacia fuera y reenvía las peticiones a `http://127.0.0.1:17373`. Para Tramtai es un
proxy más en este equipo, así que la configuración es la de los ejemplos de [proxies](proxies):

```yaml
# /var/lib/tramtai/config.yaml (Linux) o %ProgramData%\Tramtai\config.yaml (Windows)
servicio:
  host: 127.0.0.1
  dominios: [tramtai.example.com]   # el nombre público del túnel
  proxy:
    confiables: [127.0.0.1/32]      # el programa del túnel, en este mismo equipo
```

Tramtai solo deja iniciar sesión si la petición llega con `X-Forwarded-Proto: https` desde una dirección de
`servicio.proxy.confiables`, y solo acepta los nombres de `servicio.dominios` en `Host`. Después de configurar el túnel,
inicia sesión una vez y mira el registro (`sudo tramtai-servidor registro` en Linux; en Windows, `tramtai-servidor
registro` desde una consola de administrador): la línea «acceso correcto del
administrador desde …» debe tener tu dirección y no `127.0.0.1`. Si no puedes entrar, el túnel no envía esas cabeceras
o cambia el `Host`.

## Cloudflare Tunnel

`cloudflared tunnel` con un nombre público que apunte a `http://127.0.0.1:17373`.

- **El HTTPS termina en Cloudflare:** Cloudflare ve las peticiones en claro, también las claves de tus APIs, la
  contraseña del administrador y las respuestas de los agentes. Úsalo solo si aceptas eso; si no, un proxy en tu equipo
  con tu certificado (nginx, Caddy…) o el TLS propio de Tramtai.
- El tramo entre Cloudflare y tu equipo va cifrado por el túnel; el último, de `cloudflared` a Tramtai, no sale del
  equipo.
- Con Cloudflare Access delante pides además una identidad antes de llegar a la página de inicio de sesión.
- **Las respuestas sin flujo se cortan:** si Tramtai no empieza a responder en el plazo de lectura de Cloudflare
  (125 s, según su [documentación del error 524](https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-5xx-errors/error-524/);
  solo en Enterprise se puede alargar), la petición acaba en el error 524. En las peticiones largas usa `stream: true`,
  que empieza a responder en seguida, o la API nativa, que crea la petición y se consulta después.

## Tailscale Serve y Funnel

- **Serve** publica el servidor solo dentro de tu tailnet (tus equipos y las personas que invites), con un certificado
  de `*.ts.net`: `tailscale serve --bg 17373`. Es la opción más cerrada.
- **Funnel** lo abre a internet con el mismo nombre: `tailscale funnel --bg 17373`. El HTTPS termina en tu equipo
  (`tailscaled`), no en los servidores de Tailscale.
- En `servicio.dominios` va el nombre de la máquina en la tailnet (`equipo.tailnet-xxxx.ts.net`).

---

# Tunnels in front of the Tramtai server

A tunnel publishes the server without opening ports or having your own certificate: a program from the provider runs
on the same computer as Tramtai, connects outward and forwards requests to `http://127.0.0.1:17373`. To Tramtai it’s
just another proxy on this computer, so the settings are the same as in the [proxy examples](proxies):

```yaml
# /var/lib/tramtai/config.yaml (Linux) or %ProgramData%\Tramtai\config.yaml (Windows)
servicio:
  host: 127.0.0.1
  dominios: [tramtai.example.com]   # the tunnel’s public name
  proxy:
    confiables: [127.0.0.1/32]      # the tunnel program, on this same computer
```

Tramtai only lets you sign in when the request arrives with `X-Forwarded-Proto: https` from an address in
`servicio.proxy.confiables`, and it only accepts the names in `servicio.dominios` as `Host`. After setting up the
tunnel, sign in once and check the log (`sudo tramtai-servidor log` on Linux; on Windows, `tramtai-servidor log` from an
administrator console): the line “acceso correcto del administrador
desde …” must show your address, not `127.0.0.1`. If you can’t sign in, the tunnel isn’t sending those headers or it
changes the `Host`.

## Cloudflare Tunnel

`cloudflared tunnel` with a public hostname pointing to `http://127.0.0.1:17373`.

- **HTTPS ends at Cloudflare:** Cloudflare sees the requests in plain text, including your API keys, the
  administrator password and the agents’ responses. Only use it if you accept that; otherwise, a proxy on your computer
  with your own certificate (nginx, Caddy…) or Tramtai’s own TLS.
- The leg between Cloudflare and your computer is encrypted by the tunnel; the last one, from `cloudflared` to Tramtai,
  never leaves the computer.
- With Cloudflare Access in front, you also require an identity before anyone reaches the sign-in page.
- **Responses without streaming get cut off:** if Tramtai doesn’t start responding within Cloudflare’s read timeout
  (125 s, according to its [error 524 documentation](https://developers.cloudflare.com/support/troubleshooting/http-status-codes/cloudflare-5xx-errors/error-524/);
  only Enterprise can raise it), the request ends in error 524. For long requests, use `stream: true`, which starts
  responding right away, or the native API, which creates the request so you can check on it later.

## Tailscale Serve and Funnel

- **Serve** publishes the server only inside your tailnet (your devices and the people you invite), with a `*.ts.net`
  certificate: `tailscale serve --bg 17373`. It’s the most closed option.
- **Funnel** opens it to the internet under the same name: `tailscale funnel --bg 17373`. HTTPS ends on your computer
  (`tailscaled`), not on Tailscale’s servers.
- `servicio.dominios` takes the machine’s tailnet name (`machine.tailnet-xxxx.ts.net`).
