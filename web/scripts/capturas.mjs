// capturas saca las capturas de la Ayuda (web/src/ayuda/capturas: claro y oscuro, español e inglés), las de cada paso de
// los recorridos guiados (recorrido-*) y las de la portada (site/capturas: las cinco pantallas, la terminal, en HTML
// (terminal.json), y la demo de la bienvenida) con una Tramtai compilada en una carpeta temporal y portable, en su propio puerto (PUERTO_CAPTURAS; la
// nota-capturas de site/index.html dice el puerto que se ve en ellas: cámbiala si cambia): no toca la instalación de
// quien lo corre ni su puerto 17373. Las del modo servidor (Ajustes → Servidor y
// Seguridad, la entrada y la marca) salen de un Tramtai en modo servidor detrás de Caddy, en contenedores (Docker), con
// su propio puerto; sin Docker se avisa y se saltan. Se corre en el equipo de desarrollo, con un agente ya conectado
// (para el Historial):
//   cd web; npm run build; npm run capturas
//   (o: npm run capturas -- --solo receta, -- --recorridos, -- --bienvenida, -- --terminal, -- --servidor o -- --sitio)
// Usa Edge o Chrome sin ventana por su protocolo de depuración; TRAMTAI_NAVEGADOR elige otro ejecutable y
// TRAMTAI_CAPTURAS_MODELO el modelo de las peticiones de ejemplo (claude/haiku por defecto).
import { execFileSync, spawn } from 'node:child_process'
import { createHmac, randomBytes } from 'node:crypto'
import { existsSync, mkdirSync, mkdtempSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:net'
import { homedir, tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import en from '../src/idiomas/en.ts'
import es from '../src/idiomas/es.ts'
import { ANCHO_AYUDA, ANCLAS, BIENVENIDA, CAPTURAS, PORTADA, SIN_CAPTURA, TERMINAL, ansiAHtml, rutaPrivada } from './capturas-lista.ts'

const raiz = fileURLToPath(new URL('../../', import.meta.url))
const destinoAyuda = fileURLToPath(new URL('../src/ayuda/capturas/', import.meta.url))
const destinoPortada = fileURLToPath(new URL('../../site/capturas/', import.meta.url))
const args = process.argv.slice(2)
const solo = args.includes('--solo') ? args[args.indexOf('--solo') + 1] : ''
// --sitio saca solo los iconos y la imagen para compartir del sitio (favicon.svg y og.html), sin capturas de la app.
const soloSitio = args.includes('--sitio')
// --recorridos saca solo las capturas de los pasos de los recorridos.
const soloRecorridos = args.includes('--recorridos')
// --bienvenida saca solo la demo de la portada: la bienvenida de verdad, una pantalla por paso, sin peticiones a ningún
// agente.
const soloBienvenida = args.includes('--bienvenida')
// --terminal saca solo las de la terminal de la portada; --servidor, solo las del modo servidor.
const soloTerminal = args.includes('--terminal')
const soloServidor = args.includes('--servidor')
// completa es la corrida sin --solo ni ningún modo: lo saca todo.
const completa = !solo && !soloSitio && !soloRecorridos && !soloBienvenida && !soloTerminal && !soloServidor
const destinoSitio = fileURLToPath(new URL('../../site/', import.meta.url))
const PUERTO_CAPTURAS = 17388
const modelo = process.env.TRAMTAI_CAPTURAS_MODELO || 'claude/haiku'
const esperar = (ms) => new Promise((ok) => setTimeout(ok, ms))
// Nada del equipo de quien saca las capturas se publica: rutas de su carpeta personal, la temporal o la del repositorio.
const privada = rutaPrivada([homedir(), tmpdir(), raiz])
if (solo && ![...CAPTURAS, ...ANCLAS].some((c) => c.id === solo)) {
  console.error(`no hay una captura «${solo}»: usa uno de ${[...CAPTURAS, ...ANCLAS].map((c) => c.id).join(', ')}`)
  process.exit(1)
}

// puertoLibre prueba primero el preferido (así la dirección que sale en las capturas no cambia en cada corrida).
function puertoLibre(preferido = 0) {
  return new Promise((ok, mal) => {
    const s = createServer()
    s.once('error', () => (preferido ? puertoLibre().then(ok, mal) : mal(new Error('sin puertos libres'))))
    s.listen(preferido, '127.0.0.1', () => { const { port } = s.address(); s.close(() => ok(port)) })
  })
}

function navegador() {
  const candidatos = process.env.TRAMTAI_NAVEGADOR ? [process.env.TRAMTAI_NAVEGADOR] : {
    win32: ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
      'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'],
    darwin: ['/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge'],
  }[process.platform] ?? ['/usr/bin/google-chrome', '/usr/bin/chromium', '/usr/bin/chromium-browser', '/usr/bin/microsoft-edge']
  const ruta = candidatos.find((c) => existsSync(c))
  if (!ruta) throw new Error('no encontré Edge ni Chrome: indica uno con TRAMTAI_NAVEGADOR')
  return ruta
}

async function hasta(condicion, segundos, que) {
  for (let i = 0; i < segundos * 4; i++) {
    try { if (await condicion()) return } catch { /* todavía no */ }
    await esperar(250)
  }
  throw new Error('no llegó a tiempo: ' + que)
}

// conectar abre el protocolo de depuración de una pestaña y devuelve enviar(método, parámetros).
async function conectar(url) {
  const ws = new WebSocket(url)
  await new Promise((ok, mal) => { ws.onopen = ok; ws.onerror = mal })
  let n = 0
  const pendientes = new Map()
  ws.onmessage = (m) => {
    const d = JSON.parse(m.data)
    const p = d.id && pendientes.get(d.id)
    if (!p) return
    pendientes.delete(d.id)
    if (d.error) p.mal(new Error(d.error.message))
    else p.ok(d.result)
  }
  return {
    enviar: (method, params = {}) => new Promise((ok, mal) => { const id = ++n; pendientes.set(id, { ok, mal }); ws.send(JSON.stringify({ id, method, params })) }),
    cerrar: () => ws.close(),
  }
}

// La copia portable no arranca desde la carpeta temporal del sistema (se borraría con sus datos): va en bin/, que git
// ignora, y se borra al terminar.
mkdirSync(join(raiz, 'bin'), { recursive: true })
const temporal = mkdtempSync(join(raiz, 'bin', 'capturas-'))
const exe = join(temporal, process.platform === 'win32' ? 'tramtai.exe' : 'tramtai')
const procesos = []
// Con Ctrl+C se detiene igual lo que se arrancó (el finally no corre si el proceso termina por la señal).
process.on('SIGINT', () => {
  try { execFileSync(exe, ['servicio', 'detener'], { cwd: temporal, stdio: 'ignore' }) } catch { /* no estaba en marcha */ }
  for (const p of procesos) p.kill()
  process.exit(130)
})

// lanzar abre Edge o Chrome sin ventana, con su propio perfil y las opciones extra, y devuelve con qué hablarle.
async function lanzar(extra = []) {
  const depuracion = await puertoLibre()
  const nav = spawn(navegador(), ['--headless=new', `--remote-debugging-port=${depuracion}`, `--user-data-dir=${join(temporal, 'navegador-' + depuracion)}`,
    '--no-first-run', '--no-default-browser-check', '--hide-scrollbars', '--disable-extensions',
    '--disable-component-extensions-with-background-pages', ...extra, 'about:blank'], { stdio: 'ignore' })
  procesos.push(nav)
  let pestana
  await hasta(async () => {
    const lista = await (await fetch(`http://127.0.0.1:${depuracion}/json/list`)).json()
    pestana = lista.find((p) => p.type === 'page')
    return !!pestana
  }, 20, 'el navegador')
  const { enviar, cerrar } = await conectar(pestana.webSocketDebuggerUrl)
  const evaluar = async (expr) => (await enviar('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true })).result.value
  return { enviar, evaluar, cerrar }
}

// crearAbrir da abrir(pagina, tema, idioma, selector, ancho, alto, guardado): carga la página de esa Web UI con ese tema
// e idioma (guardados en localStorage) y espera a que aparezca el selector.
function crearAbrir(enviar, evaluar, base) {
  let carga = 0
  return async function abrir(pagina, tema, idioma, selector, ancho, alto, guardado = {}) {
    await enviar('Emulation.setDeviceMetricsOverride', { width: ancho, height: alto, deviceScaleFactor: 1, mobile: false })
    await enviar('Page.navigate', { url: `${base}/?c=${++carga}` })
    await hasta(() => evaluar('document.readyState === "complete"'), 20, 'la carga')
    // La pestaña de Ajustes que dejó una captura no debe pasar a la siguiente: cada una trae la suya en `guardado`.
    await evaluar(`localStorage.removeItem('tramtai.pestanaAjustes')`)
    // La bienvenida (se ofrece sola la primera vez en Inicio) no sale en las capturas: su fondo oscurecería la página.
    await evaluar(`localStorage.setItem('tramtai.tema', '${tema}'); localStorage.setItem('tramtai.idioma', '${idioma}'); localStorage.setItem('tramtai.bienvenida', 'descartada')`)
    for (const [clave, valor] of Object.entries(guardado)) await evaluar(`localStorage.setItem(${JSON.stringify(clave)}, ${JSON.stringify(JSON.stringify(valor))})`)
    await enviar('Page.navigate', { url: `${base}/?c=${++carga}#${pagina}` })
    await hasta(() => evaluar(`!!document.querySelector('${selector}')`), 20, `${selector} en #${pagina}`)
    // Hasta que no quede nada cargando dentro (esqueletos de Quasar).
    await hasta(() => evaluar(`!document.querySelector('${selector} .q-skeleton, ${selector} .q-inner-loading')`), 20, `${selector} cargado`)
    await esperar(1000) // animaciones y datos que llegan después
  }
}

// capturarZona saca la zona [data-captura=<id>] de una captura de la Ayuda, en ese tema e idioma. La cabecera se saca
// con la barra fija a la vista; las demás, sin ella.
async function capturarZona({ enviar, evaluar, abrir, sinBarra, ocultarRuta }, c, tema, idioma) {
  const selector = `[data-captura="${c.id}"]`
  await abrir(c.pagina, tema, idioma, selector, c.ancho ?? 1280, 900, c.guardado)
  await sinBarra(!c.cabecera)
  await ocultarRuta()
  const r = await evaluar(`(() => { const e = document.querySelector('${selector}'); e.scrollIntoView({ block: 'start' });
    const b = e.getBoundingClientRect(); return { x: b.left + scrollX, y: b.top + scrollY, w: b.width, h: Math.min(b.height, 720),
      texto: e.innerText + ' ' + [...e.querySelectorAll('input, textarea')].map((c) => c.value).join(' ') } })()`)
  if (privada.test(r.texto)) throw new Error(`${c.id}: la captura muestra una ruta real: ${r.texto.slice(0, 200)}`)
  // Más ancha que la columna de la Ayuda del sitio, se saca reducida a ese ancho (ANCHO_AYUDA).
  const { data } = await enviar('Page.captureScreenshot', { format: 'webp', quality: 80, captureBeyondViewport: true,
    clip: { x: r.x, y: r.y, width: r.w, height: r.h, scale: Math.min(1, ANCHO_AYUDA / r.w) } })
  writeFileSync(join(destinoAyuda, `${c.id}.${tema}.${idioma}.webp`), Buffer.from(data, 'base64'))
  console.log(`${c.id}.${tema}.${idioma}.webp`)
}

// capturasTerminal saca site/capturas/terminal.json: compila internal/tui/captura junto a la copia portable (portable.txt,
// así habla con su servicio y sus datos de ejemplo), pinta cada página de TERMINAL a 100×28 en español y en inglés, solo
// en el tema oscuro (las clases t-<token> valen para los dos temas, ver ansiAHtml) y guarda su HTML:
// {"es":{"inicio":"…","apis":"…","historial":"…"},"en":{…}}. La ruta del proyecto de ejemplo se cambia por la de siempre.
// Falla si una pantalla enseña una ruta real, si quedó en «Comprobando…» o si el Historial no tiene peticiones.
function capturasTerminal(ejemplo) {
  const programa = join(temporal, process.platform === 'win32' ? 'captura.exe' : 'captura')
  execFileSync('go', ['build', '-o', programa, './internal/tui/captura'], { cwd: raiz, stdio: 'inherit' })
  const textos = JSON.parse(readFileSync(join(raiz, 'internal', 'tui', 'datos', 'textos.json'), 'utf8'))
  const terminal = {}
  for (const idioma of ['es', 'en']) {
    terminal[idioma] = {}
    for (const p of TERMINAL) {
      // Sin NO_COLOR ni ASCII; TERM_PROGRAM dice que la salida acaba en un HTML con UTF-8, no en una consola antigua.
      const entorno = { ...process.env, TRAMTAI_IDIOMA: idioma, TERM_PROGRAM: 'tramtai-capturas' }
      delete entorno.NO_COLOR
      delete entorno.TRAMTAI_ASCII
      const ansi = execFileSync(programa, ['oscuro', idioma, '100', '28', ...p.teclas], { cwd: temporal, env: entorno, encoding: 'utf8', maxBuffer: 16 << 20 })
        .replaceAll(ejemplo, 'C:\\Proyectos\\mi-app')
      if (privada.test(ansi)) throw new Error(`terminal ${p.nombre} (${idioma}): la captura muestra una ruta real`)
      const html = ansiAHtml(ansi)
      const plano = html.replace(/<[^>]+>/g, '')
      const comprobando = textos[idioma]['inicio.comprobando']
      if (plano.includes(comprobando)) throw new Error(`terminal ${p.nombre} (${idioma}): quedó en «${comprobando}»`)
      if (p.nombre === 'historial' && plano.includes(textos[idioma]['historial.vacio']))
        throw new Error(`terminal historial (${idioma}): sin peticiones; ¿respondió ${modelo}?`)
      terminal[idioma][p.nombre] = html
      console.log(`terminal ${idioma} ${p.nombre}: ${html.length} bytes`)
    }
  }
  writeFileSync(join(destinoPortada, 'terminal.json'), JSON.stringify(terminal, null, 1) + '\n')
  console.log('site/capturas/terminal.json')
}

// totp da el código de 6 dígitos de un secreto en un instante (RFC 6238: SHA-1, pasos de 30 s), como la app de
// autenticación, para entrar en el servidor de prueba con su segundo factor.
function totp(secreto, instante = Date.now()) {
  const paso = Buffer.alloc(8)
  paso.writeBigUInt64BE(BigInt(Math.floor(instante / 30000)))
  const h = createHmac('sha1', secreto).update(paso).digest()
  const o = h[19] & 15
  return String((h.readUInt32BE(o) & 0x7fffffff) % 1e6).padStart(6, '0')
}
const base32 = (b) => [...b].map((x) => x.toString(2).padStart(8, '0')).join('').match(/.{1,5}/g)
  .map((g) => 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'[parseInt(g.padEnd(5, '0'), 2)]).join('')

// capturasServidor saca las capturas del modo servidor: un Tramtai en modo servidor (Linux) detrás de Caddy con HTTPS,
// los dos del ejemplo de docs/servidor/proxies (scripts/servidor/docker-compose.proxies.yml), con su contraseña y su
// TOTP de prueba. El navegador llega a tramtai.example.com por el puerto que publica el contenedor. También comprueba
// en el navegador el diálogo de Ajustes → Seguridad que pide la contraseña y el código antes de crear códigos nuevos.
async function capturasServidor() {
  try { execFileSync('docker', ['info'], { stdio: 'ignore' }) } catch {
    console.warn('aviso: sin Docker no salen las capturas del servidor (entrar, marca, servidor, seguridad)')
    return
  }
  const prueba = join(temporal, 'servidor')
  for (const d of ['bin', 'tls']) mkdirSync(join(prueba, d), { recursive: true })
  const linux = { ...process.env, CGO_ENABLED: '0', GOOS: 'linux', GOARCH: 'amd64' }
  console.log('Compilando el Tramtai del servidor (Linux)…')
  execFileSync('go', ['build', '-o', join(prueba, 'bin', 'tramtai'), './cmd/tramtai'], { cwd: raiz, env: linux, stdio: 'inherit' })
  execFileSync('go', ['build', '-o', join(prueba, 'bin', 'claude'), './scripts/servidor/claude-simulado'], { cwd: raiz, env: linux, stdio: 'inherit' })
  const goroot = execFileSync('go', ['env', 'GOROOT'], { encoding: 'utf8' }).trim()
  execFileSync('go', ['run', join(goroot, 'src', 'crypto', 'tls', 'generate_cert.go'), '--host', 'tramtai.example.com', '--ecdsa-curve', 'P256', '--ca'],
    { cwd: join(prueba, 'tls'), stdio: 'ignore' })
  renameSync(join(prueba, 'tls', 'cert.pem'), join(prueba, 'tls', 'fullchain.pem'))
  renameSync(join(prueba, 'tls', 'key.pem'), join(prueba, 'tls', 'privkey.pem'))
  const puerto = await puertoLibre(17489)
  writeFileSync(join(prueba, 'puertos.yml'), `services:\n  tramtai:\n    ports: ["127.0.0.1:${puerto}:443"]\n`)
  const compose = (args, opciones = {}) => execFileSync('docker', ['compose', '-p', 'tramtai-capturas', '-f', join(raiz, 'scripts', 'servidor', 'docker-compose.proxies.yml'),
    '-f', join(prueba, 'puertos.yml'), ...args], { cwd: raiz, env: { ...process.env, TRAMTAI_PRUEBA: prueba }, encoding: 'utf8', stdio: 'pipe', ...opciones })
  const enTramtai = (...a) => compose(['exec', '-T', 'tramtai', ...a])
  const contrasena = 'contrasena-de-las-capturas'
  const secreto = randomBytes(20)
  try {
    compose(['up', '-d', '--quiet-pull', 'tramtai', 'caddy'])
    // Como lo deja «tramtai-servidor instalar»: la cuenta del servicio, su configuración tras el proxy, los términos y
    // la contraseña. El servicio corre con su cuenta (como root no arranca) y el secreto TOTP le llega como se lo daría
    // systemd: en $CREDENTIALS_DIRECTORY/tramtai.TRAMTAI_TOTP (el contenedor no tiene systemd).
    enTramtai('useradd', '--system', '--home', '/var/lib/tramtai', '--shell', '/usr/sbin/nologin', 'tramtai')
    enTramtai('sh', '-c', `mkdir -p /var/lib/tramtai && printf 'servicio:\\n  host: 127.0.0.1\\n  dominios: [tramtai.example.com]\\n  proxy:\\n    confiables: [127.0.0.1/32]\\n' >/var/lib/tramtai/config.yaml`)
    enTramtai('sh', '-c', `mkdir -p /run/credenciales && printf %s ${base32(secreto)} >/run/credenciales/tramtai.TRAMTAI_TOTP && chown -R tramtai /run/credenciales && chmod -R go= /run/credenciales`)
    enTramtai('/opt/tramtai/tramtai', 'terminos', 'aceptar')
    compose(['exec', '-T', 'tramtai', '/opt/tramtai/tramtai', 'administrador', 'contraseña'], { input: `${contrasena}\n${contrasena}\n` })
    compose(['exec', '-d', '-u', 'tramtai', '-e', 'CREDENTIALS_DIRECTORY=/run/credenciales', 'tramtai', '/opt/tramtai/tramtai', 'servicio', 'iniciar',
      '--registro', '/var/lib/tramtai/datos/servicio.log'])
    await hasta(() => enTramtai('/opt/tramtai/tramtai', 'servicio', 'estado').includes('servicio activo'), 30, 'el servicio del servidor')

    const s = await lanzar([`--host-rules=MAP tramtai.example.com 127.0.0.1:${puerto}`, '--ignore-certificate-errors'])
    const nav = { enviar: s.enviar, evaluar: s.evaluar, abrir: crearAbrir(s.enviar, s.evaluar, 'https://tramtai.example.com'),
      sinBarra: (oculta = true) => s.evaluar(`(() => { let e = document.getElementById('sin-barra'); if (!e) { e = document.createElement('style'); e.id = 'sin-barra'; document.head.append(e) }
        e.textContent = ${oculta ? "'.q-header{display:none!important}'" : "''"} })()`),
      ocultarRuta: async () => {} }
    const deServidor = CAPTURAS.filter((c) => c.servidor && (completa || soloServidor || c.id === solo))
    // La página de entrada, antes de entrar; después, con la contraseña y el código, lo demás.
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const c of deServidor.filter((x) => x.id === 'entrar')) await capturarZona(nav, c, tema, idioma)
    await nav.abrir('inicio', 'claro', 'es', '[data-captura="entrar"]', 1280, 900)
    const estado = await s.evaluar(`fetch('/ui/acceder', { method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ contrasena: ${JSON.stringify(contrasena)}, codigo: ${JSON.stringify(totp(secreto))} }) }).then((r) => r.status)`)
    if (estado !== 200) throw new Error(`no se pudo entrar en el servidor de prueba: ${estado}`)
    const pasoEntrada = Math.floor(Date.now() / 30000)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const c of deServidor.filter((x) => x.id !== 'entrar')) await capturarZona(nav, c, tema, idioma)

    // El diálogo de Seguridad: «Crear códigos nuevos» pide la contraseña y, con TOTP, un código (de otro paso: uno ya
    // usado no vale), y da los códigos nuevos.
    if (completa || soloServidor) {
      await nav.abrir('configuracion', 'claro', 'es', '[data-captura="seguridad"]', 1280, 900, { 'tramtai.pestanaAjustes': 'seguridad' })
      await hasta(async () => Math.floor(Date.now() / 30000) > pasoEntrada, 35, 'un paso nuevo del TOTP')
      // rellenar escribe en el campo del diálogo y, cuando Vue ya habilitó «OK» (el último botón), lo pulsa.
      const rellenar = async (valor) => {
        await hasta(() => s.evaluar(`(() => { const i = document.querySelector('.q-dialog input'); if (!i) return false
          Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(i, ${JSON.stringify(valor)}); i.dispatchEvent(new Event('input', { bubbles: true })); return true })()`), 10, 'el campo del diálogo')
        await hasta(() => s.evaluar(`(() => { const b = [...document.querySelectorAll('.q-dialog .q-card__actions .q-btn')].at(-1)
          if (!b || b.disabled || b.classList.contains('disabled')) return false; b.click(); return true })()`), 10, '«OK» en el diálogo')
        return true
      }
      await hasta(() => s.evaluar(`(() => { const b = [...document.querySelectorAll('button')].find((b) => b.innerText.includes(${JSON.stringify(es['seguridad.regenerar'])}))
        b?.click(); return !!b })()`), 10, `el botón «${es['seguridad.regenerar']}» (¿el TOTP está activo?)`)
      await hasta(() => s.evaluar(`document.querySelector('.q-dialog')?.innerText.includes(${JSON.stringify(es['seguridad.confirmarTexto'])}) ?? false`), 10, 'el diálogo de la contraseña')
      await rellenar(contrasena)
      await hasta(() => s.evaluar(`document.querySelector('.q-dialog')?.innerText.includes(${JSON.stringify(es['seguridad.codigoActualTexto'])}) ?? false`), 10, 'el diálogo del código')
      await rellenar(totp(secreto))
      // Los códigos salen con el aviso «Guárdalos…», en un bloque con su botón de copiar (cuyo icono también es texto).
      const pantalla = join(tmpdir(), 'tramtai-dialogo-totp.png')
      const codigos = `(() => { if (!document.body.innerText.includes(${JSON.stringify(es['seguridad.guardalos'])})) return 0
        return [...document.querySelectorAll('.bloque-codigo')].map((b) => b.innerText.split('\\n').filter((l) => l.trim() && !l.includes('content_copy')).length).reduce((a, n) => Math.max(a, n), 0) })()`
      try {
        await hasta(async () => (await s.evaluar(codigos)) === 10, 15, 'los 10 códigos nuevos')
      } finally {
        const { data } = await s.enviar('Page.captureScreenshot', { format: 'png' })
        writeFileSync(pantalla, Buffer.from(data, 'base64'))
      }
      console.log(`el diálogo de Seguridad pidió la contraseña y el código y dio 10 códigos nuevos (pantalla en ${pantalla})`)
    }
    await s.enviar('Browser.close').catch(() => {})
    s.cerrar()
  } finally {
    try { compose(['down', '-t', '1']) } catch { console.warn('no pude parar los contenedores de tramtai-capturas: docker compose -p tramtai-capturas down') }
  }
}

try {
  console.log('Compilando Tramtai…')
  execFileSync('go', ['build', '-o', exe, './cmd/tramtai'], { cwd: raiz, stdio: 'inherit' })
  // Un puerto propio, el 17388, y no el 17373 de quien las saca: así nunca choca con su Tramtai. La nota-capturas de la
  // portada lo dice, porque algunas lo enseñan (Inicio, Mis APIs).
  const puerto = await puertoLibre(PUERTO_CAPTURAS)
  if (puerto !== PUERTO_CAPTURAS) console.warn(`aviso: el puerto ${PUERTO_CAPTURAS} está ocupado; las capturas enseñarán el ${puerto}`)
  const [agente] = modelo.split('/')
  writeFileSync(join(temporal, 'portable.txt'), '')
  mkdirSync(join(temporal, 'datos-tramtai'), { recursive: true })
  writeFileSync(join(temporal, 'datos-tramtai', 'config.yaml'),
    `servicio:\n  puerto: ${puerto}\nconversion:\n  preparar_al_iniciar: false\nadaptadores:\n  ${agente}:\n    autenticacion: suscripcion\n`)
  const servicio = spawn(exe, ['servicio', 'iniciar'], { cwd: temporal, stdio: ['ignore', 'inherit', 'inherit'] })
  procesos.push(servicio)
  const base = `http://127.0.0.1:${puerto}`
  // Responde algo (la Web UI pide sesión, así que no siempre es 200): ya está escuchando.
  await hasta(async () => (await fetch(base + '/')).status > 0, 30, 'el servicio')

  // Datos de ejemplo, con nombres neutros: dos APIs (una con presupuesto) y unas peticiones en el Historial. Los términos
  // se aceptan antes: si no, la Web UI y la terminal enseñarían «Antes de seguir».
  const cli = (...a) => execFileSync(exe, a, { cwd: temporal, stdio: 'pipe' })
  cli('terminos', 'aceptar')
  cli('acceso', 'crear', 'asistente-soporte', '--grupo', 'Soporte', '--presupuesto', 'diario.peticiones=50', '--presupuesto', 'mensual.tokens=200000')
  cli('acceso', 'crear', 'resumen-notas', '--grupo', 'Notas')
  // conApp: lo que abre la Web UI de escritorio (todas las capturas menos las de la terminal, el servidor y los iconos).
  const conApp = completa || !!solo || soloRecorridos || soloBienvenida
  for (const texto of completa || solo || soloRecorridos || soloTerminal ? ['Resume en una línea: Tramtai pone agentes de IA detrás de una API.', 'Di «hola» en tres idiomas.'] : []) {
    try { cli('ejecutar', '-a', agente, '-m', modelo.split('/')[1] ?? '', texto) } catch { console.warn(`aviso: ${modelo} no respondió; el Historial mostrará el error`) }
  }

  const { enviar, evaluar, cerrar } = await lanzar()
  const abrir = crearAbrir(enviar, evaluar, base)
  // sinBarra oculta la barra superior fija, que taparía el recorte de un elemento.
  const sinBarra = (oculta = true) => evaluar(`(() => { let s = document.getElementById('sin-barra'); if (!s) { s = document.createElement('style'); s.id = 'sin-barra'; document.head.append(s) }
    s.textContent = ${oculta ? "'.q-header{display:none!important}'" : "''"} })()`)

  // Los iconos del sitio salen de favicon.svg con fondo transparente, y la imagen para compartir de og.html.
  if (completa || soloSitio) {
    const sacar = async (url, ancho, alto, archivo) => {
      await enviar('Emulation.setDeviceMetricsOverride', { width: ancho, height: alto, deviceScaleFactor: 1, mobile: false })
      await enviar('Page.navigate', { url: new URL(url, import.meta.url).href })
      await hasta(() => evaluar('document.readyState === "complete" && document.fonts.status === "loaded"'), 20, url)
      await esperar(300)
      const { data } = await enviar('Page.captureScreenshot', { format: 'png' })
      writeFileSync(join(destinoSitio, archivo), Buffer.from(data, 'base64'))
      console.log('site/' + archivo)
    }
    await enviar('Emulation.setDefaultBackgroundColorOverride', { color: { r: 0, g: 0, b: 0, a: 0 } })
    await sacar('../../site/favicon.svg', 32, 32, 'favicon-32.png')
    await sacar('../../site/favicon.svg', 180, 180, 'apple-touch-icon.png')
    await enviar('Emulation.setDefaultBackgroundColorOverride', {})
    await sacar('./og.html', 1200, 630, 'og.png')
  }

  // Un proyecto de ejemplo para la captura de Espacios; la sesión vuelve a la global para las demás.
  const ejemplo = join(temporal, 'mi-app')
  mkdirSync(ejemplo, { recursive: true })
  if (conApp) await abrir('inicio', 'claro', 'es', '.pagina', 1280, 900)
  if (conApp) await evaluar(`(async () => {
    const pedir = async (ruta, cuerpo, csrf) => (await fetch(ruta, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
      body: JSON.stringify(cuerpo) })).json()
    const s = await (await fetch('/ui/sesion')).json()
    const p = await pedir('/ui/proyectos', { raiz: ${JSON.stringify(ejemplo)} }, s.csrf)
    await pedir('/ui/sesion', { instancia: 'global' }, p.csrf)
  })()`)

  // La ruta real del proyecto de ejemplo (la carpeta temporal de quien saca las capturas) no se publica.
  const ocultarRuta = () => evaluar(`(() => { const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    while (w.nextNode()) w.currentNode.nodeValue = w.currentNode.nodeValue.replaceAll(${JSON.stringify(ejemplo)}, 'C:\\\\Proyectos\\\\mi-app') })()`)

  mkdirSync(destinoAyuda, { recursive: true })
  for (const c of CAPTURAS.filter((x) => !x.servidor && (completa || x.id === solo)))
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en']) await capturarZona({ enviar, evaluar, abrir, sinBarra, ocultarRuta }, c, tema, idioma)

  // La terminal de la portada: cada página que pinta internal/tui/captura (compilado junto a la copia portable, así
  // habla con su servicio), convertida a HTML con ansiAHtml y guardada en site/capturas/terminal.json.
  if (completa || soloTerminal) capturasTerminal(ejemplo)

  // Un recorrido señala un elemento con el anillo de driver.js (.driver-active-element, ver estilos.css): se carga cada
  // página una vez por tema e idioma y se recorta cada ancla con 16 px de margen. Las de app.* (el selector de espacio
  // y el «?», que viven en una cabecera) recortan esa cabecera entera, que es el contexto que las hace reconocibles.
  // Una ancla que la demo no muestra no se inventa: se avisa y se apunta en SIN_CAPTURA, con el motivo.
  const visible = (ancla) => `[...document.querySelectorAll('[data-recorrido="${ancla}"]')].find((e) => { const b = e.getBoundingClientRect(); return b.width > 0 && b.height > 0 })`
  const ausentes = new Set()
  const aSacar = ANCLAS.filter((a) => (completa || soloRecorridos || a.id === solo) && !(a.ancla in SIN_CAPTURA))
  if (aSacar.length)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const pagina of [...new Set(aSacar.map((a) => a.pagina))]) {
          if (!aSacar.some((a) => a.pagina === pagina && !ausentes.has(a.ancla))) continue
          await abrir(pagina, tema, idioma, '.pagina', 1280, 900)
          // Las anclas que abren algo (preparar) van al final: lo abierto taparía a las demás.
          for (const a of aSacar.filter((x) => x.pagina === pagina && !ausentes.has(x.ancla)).sort((x, y) => !!x.preparar - !!y.preparar)) {
            const cabecera = a.ancla.startsWith('app.')
            await sinBarra(!cabecera)
            try {
              if (a.preparar) await hasta(() => evaluar(a.preparar), 10, `preparar ${a.ancla}`)
              await hasta(() => evaluar(`!!(${visible(a.ancla)})`), tema === 'claro' && idioma === 'es' ? 8 : 3, `${a.ancla} visible`)
              await esperar(300) // el panel que se abre llega con su animación
              await ocultarRuta()
            } catch {
              ausentes.add(a.ancla)
              console.warn(`aviso: ${a.ancla} no se ve en la demo de #${pagina}; apúntala en SIN_CAPTURA si no hay forma de mostrarla`)
              continue
            }
            const r = await evaluar(`(() => {
              const e = ${visible(a.ancla)}
              document.body.classList.add('driver-active'); e.classList.add('driver-active-element')
              const fijo = !!e.closest('.q-header, .q-dialog')
              if (fijo) scrollTo(0, 0); else e.scrollIntoView({ block: 'center', behavior: 'instant' })
              const c = ${cabecera} ? e.closest('.q-header, .cabecera-pagina') ?? e : e
              // Un panel de diálogo va pegado al borde de la ventana: solo el margen del anillo (2 px a 3 px del borde),
              // para no meter en el recorte trozos de la página de detrás.
              const m = ${cabecera} ? 0 : fijo ? 6 : 16
              const b = c.getBoundingClientRect()
              const x = Math.max(0, b.left + scrollX - m), y = Math.max(0, b.top + scrollY - m)
              const w = Math.min(document.documentElement.scrollWidth, b.right + scrollX + m) - x
              const h = Math.min(document.documentElement.scrollHeight, b.bottom + scrollY + m) - y
              // El texto de toda la zona recortada (también lo de detrás y los campos), no solo el del elemento.
              const dentro = (t) => t.width > 0 && t.right + scrollX > x && t.left + scrollX < x + w && t.bottom + scrollY > y && t.top + scrollY < y + h
              const textos = [], rango = document.createRange(), w2 = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
              while (w2.nextNode()) { rango.selectNodeContents(w2.currentNode); if (dentro(rango.getBoundingClientRect())) textos.push(w2.currentNode.nodeValue) }
              for (const campo of document.querySelectorAll('input, textarea')) if (dentro(campo.getBoundingClientRect())) textos.push(campo.value)
              return { x, y, w, h: Math.min(h, 1000), texto: textos.join(' ') }
            })()`)
            if (privada.test(r.texto)) throw new Error(`${a.ancla}: la captura muestra una ruta real: ${r.texto.slice(0, 200)}`)
            const { data } = await enviar('Page.captureScreenshot', { format: 'webp', quality: 80, captureBeyondViewport: true,
              clip: { x: r.x, y: r.y, width: r.w, height: r.h, scale: Math.min(1, ANCHO_AYUDA / r.w) } })
            writeFileSync(join(destinoAyuda, `${a.id}.${tema}.${idioma}.webp`), Buffer.from(data, 'base64'))
            console.log(`${a.id}.${tema}.${idioma}.webp`)
            await evaluar(`document.body.classList.remove('driver-active'); document.querySelectorAll('.driver-active-element').forEach((e) => e.classList.remove('driver-active-element'))`)
          }
        }
  if (ausentes.size) console.warn(`SIN CAPTURA: ${[...ausentes].join(', ')}`)

  if (completa)
    for (const p of PORTADA)
      for (const tema of ['claro', 'oscuro']) {
        await abrir(p.pagina, tema, 'es', '.pagina', 1440, 900)
        await ocultarRuta()
        const vista = await evaluar(`document.body.innerText + ' ' + [...document.querySelectorAll('input, textarea')].map((c) => c.value).join(' ')`)
        if (privada.test(vista)) throw new Error(`${p.nombre}: la captura muestra una ruta real: ${vista.slice(0, 200)}`)
        // La misma pantalla de 1440 px, también reducida a 960 (móviles) y a 640 (la columna de «Así se ve» en escritorio).
        for (const ancho of [1440, 960, 640]) {
          const { data } = await enviar('Page.captureScreenshot', { format: 'webp', quality: 80,
            clip: { x: 0, y: 0, width: 1440, height: 900, scale: ancho / 1440 } })
          writeFileSync(join(destinoPortada, `${p.nombre}-${tema}-${ancho}.webp`), Buffer.from(data, 'base64'))
          console.log(`site/capturas/${p.nombre}-${tema}-${ancho}.webp`)
        }
      }
  // zonaDelPaso es lo que señala el paso (.driver-active-element) con 32 px por lado, agrandado a 16:10 alrededor de su
  // centro (de 720 px de ancho a `maximo`) y encajado en la pantalla de 1440×900. El tope de 820 es lo que mide la imagen
  // en la portada: cada paso se ve a 1× o más. Un ancla más ancha (el mapa) se recorta desde su borde izquierdo, donde van
  // los nombres. En coordenadas del documento, las del recorte de Page.captureScreenshot. Solo para pasos con ancla: el
  // último no tiene, y driver.js deja en el DOM el resaltado del anterior.
  async function zonaDelPaso(maximo = 820) {
    return evaluar(`(() => {
      const b = document.querySelector('.driver-active-element').getBoundingClientRect(), maximo = ${maximo}
      const width = Math.min(maximo, Math.max(720, b.width + 64, (b.height + 64) * 1.6)), height = width / 1.6
      const x = Math.min(Math.max(0, b.width + 64 > width ? b.left - 32 : b.left + b.width / 2 - width / 2), 1440 - width)
      const y = Math.min(Math.max(0, b.top + b.height / 2 - height / 2), 900 - height)
      return { x: x + scrollX, y: y + scrollY, width, height }
    })()`)
  }
  // ENTERAS son las anclas que su paso explica enteras: las listas de filas (el estado va a la derecha) y el mapa (sus
  // agentes, a la derecha, son la mitad de lo que cuenta). Su zona abarca el ancho del ancla, a costa del aumento.
  const ENTERAS = new Set(['agentes.tarjeta', 'espacios.lista', 'inicio.mapa'])
  // huecoListo dice si el hueco del velo (el path evenodd de .driver-overlay) deja ver lo señalado entero: mira sus cuatro
  // esquinas, 2 px hacia dentro (dentro de la pantalla).
  const huecoListo = `(() => {
    const b = document.querySelector('.driver-active-element').getBoundingClientRect()
    const velo = document.querySelector('.driver-overlay path'), r = Math.min(b.right, 1440) - 2, a = Math.min(b.bottom, 900) - 2
    return [[b.left + 2, b.top + 2], [r, b.top + 2], [b.left + 2, a], [r, a]].every(([x, y]) => !velo.isPointInFill(new DOMPoint(x, y)))
  })()`
  // La demo de la portada es la bienvenida de verdad: cada tema e idioma cargan Inicio como la primera vez (sin la marca de
  // bienvenida), acepta la oferta y, en cada paso, saca la zona que señala el paso (zonaDelPaso), ampliada y sin la
  // ventana del recorrido; el «Siguiente» lo pulsa el script, como quien hace el recorrido. Los datos son los de
  // demostración de arriba. Sale en la corrida completa y con --bienvenida.
  if (soloBienvenida || completa)
    for (const tema of ['claro', 'oscuro'])
     for (const idioma of ['es', 'en']) {
      const textos = idioma === 'es' ? es : en
      await abrir('inicio', tema, idioma, '.pagina', 1440, 900)
      await evaluar(`localStorage.removeItem('tramtai.bienvenida')`)
      await enviar('Page.navigate', { url: `${base}/?c=b${Date.now()}#inicio` })
      const empezar = `[...document.querySelectorAll('.q-dialog button')].find((b) => b.innerText.trim() === ${JSON.stringify(textos['bienvenida.empezar'])})`
      await hasta(() => evaluar(`!!(${empezar})`), 20, 'la oferta de la bienvenida')
      await evaluar(`(${empezar}).click()`)
      for (const [i, paso] of BIENVENIDA.entries()) {
        const titulo = textos[paso.titulo]
        await hasta(() => evaluar(`document.querySelector('.driver-popover-title')?.innerText === ${JSON.stringify(titulo)}`), 20, `el paso ${i + 1} «${titulo}»`)
        if (paso.ancla) await hasta(() => evaluar(`!!document.querySelector('.driver-active-element')`), 10, `el resaltado de ${paso.ancla}`)
        await esperar(1000) // la ventana y el resaltado llegan con su animación, y los datos de la página con ella
        await ocultarRuta()
        // Se mira el texto de toda la pantalla y el de sus campos (más de lo que sale en el recorte).
        const visto = await evaluar(`document.body.innerText + ' ' + [...document.querySelectorAll('input, textarea')].map((c) => c.value).join(' ')`)
        if (privada.test(visto)) throw new Error(`el paso ${i + 1} muestra una ruta real: ${visto.slice(0, 200)}`)
        // Lo que señala el paso pudo crecer al llegar sus datos: driver.js recoloca el hueco del velo con un resize.
        if (paso.ancla) await hasta(async () => { await evaluar(`dispatchEvent(new Event('resize'))`); await esperar(200); return evaluar(huecoListo) }, 10, `el hueco del velo sobre ${paso.ancla}`)
        else await evaluar(`scrollTo(0, 0)`)
        const zona = paso.ancla ? await zonaDelPaso(ENTERAS.has(paso.ancla) ? 1440 : 820) : { x: 0, y: 0, width: 1440, height: 900 }
        // Sin el globo del recorrido; si el paso no señala nada, tampoco el velo (saldría la pantalla oscurecida).
        const tapan = paso.ancla ? '.driver-popover' : '.driver-popover, .driver-overlay'
        await evaluar(`document.querySelectorAll('${tapan}').forEach((e) => { e.style.visibility = 'hidden' })`)
        // La zona del paso a 1440 px de ancho (hasta 2× lo que mide en la pantalla). La de 960 para los móviles (srcset)
        // se reduce de esa en un canvas: un segundo recorte con scale < 1 salía a veces en mosaico.
        const { data } = await enviar('Page.captureScreenshot', { format: 'webp', quality: 80,
          clip: { ...zona, scale: 1440 / zona.width } })
        const reducida = await evaluar(`(async () => {
          const img = new Image(); img.src = 'data:image/webp;base64,${data}'; await img.decode()
          const lienzo = Object.assign(document.createElement('canvas'), { width: 960, height: 600 }), g = lienzo.getContext('2d')
          g.imageSmoothingQuality = 'high'; g.drawImage(img, 0, 0, 960, 600)
          return lienzo.toDataURL('image/webp', 0.8).split(',')[1]
        })()`)
        for (const [ancho, webp] of [[1440, data], [960, reducida]]) {
          writeFileSync(join(destinoPortada, `bienvenida-${i + 1}-${tema}-${idioma}-${ancho}.webp`), Buffer.from(webp, 'base64'))
          console.log(`site/capturas/bienvenida-${i + 1}-${tema}-${idioma}-${ancho}.webp`)
        }
        await evaluar(`document.querySelectorAll('${tapan}').forEach((e) => { e.style.visibility = '' })`)
        if (i < BIENVENIDA.length - 1) await evaluar(`document.querySelector('.driver-popover-next-btn').click()`)
      }
    }
  await enviar('Browser.close').catch(() => {})
  cerrar()

  if (completa || soloServidor || CAPTURAS.some((c) => c.servidor && c.id === solo)) await capturasServidor()
} finally {
  try { execFileSync(exe, ['servicio', 'detener'], { cwd: temporal, stdio: 'ignore' }) } catch { /* ya estaba detenido */ }
  for (const p of procesos) p.kill()
  await esperar(1500)
  try { rmSync(temporal, { recursive: true, force: true, maxRetries: 5, retryDelay: 500 }) } catch { console.warn('no pude borrar ' + temporal) }
}
