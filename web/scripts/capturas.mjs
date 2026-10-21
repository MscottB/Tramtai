// capturas saca las capturas de la Ayuda (web/src/ayuda/capturas, en claro y oscuro y en español e inglés) y las de la
// portada (site/capturas) con una Tramtai compilada en una carpeta temporal, portable y en otro puerto, así no toca la
// instalación de quien lo corre. Se corre en el equipo de desarrollo, con un agente ya conectado (para el Historial):
//   cd web; npm run build; npm run capturas            (o: npm run capturas -- --solo receta, o -- --sitio)
// Usa Edge o Chrome sin ventana por su protocolo de depuración; TRAMTAI_NAVEGADOR elige otro ejecutable y
// TRAMTAI_CAPTURAS_MODELO el modelo de las peticiones de ejemplo (claude/haiku por defecto).
import { execFileSync, spawn } from 'node:child_process'
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:net'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { CAPTURAS, PORTADA } from './capturas-lista.ts'

const raiz = fileURLToPath(new URL('../../', import.meta.url))
const destinoAyuda = fileURLToPath(new URL('../src/ayuda/capturas/', import.meta.url))
const destinoPortada = fileURLToPath(new URL('../../site/capturas/', import.meta.url))
const args = process.argv.slice(2)
const solo = args.includes('--solo') ? args[args.indexOf('--solo') + 1] : ''
// --sitio saca solo los iconos y la imagen para compartir del sitio (favicon.svg y og.html), sin capturas de la app.
const soloSitio = args.includes('--sitio')
const destinoSitio = fileURLToPath(new URL('../../site/', import.meta.url))
const modelo = process.env.TRAMTAI_CAPTURAS_MODELO || 'claude/haiku'
const esperar = (ms) => new Promise((ok) => setTimeout(ok, ms))
if (solo && !CAPTURAS.some((c) => c.id === solo)) {
  console.error(`no hay una captura «${solo}»: usa uno de ${CAPTURAS.map((c) => c.id).join(', ')}`)
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
try {
  console.log('Compilando Tramtai…')
  execFileSync('go', ['build', '-o', exe, './cmd/tramtai'], { cwd: raiz, stdio: 'inherit' })
  const puerto = await puertoLibre(17388)
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

  // Datos de ejemplo, con nombres neutros: dos APIs (una con presupuesto) y unas peticiones en el Historial.
  const cli = (...a) => execFileSync(exe, a, { cwd: temporal, stdio: 'pipe' })
  cli('acceso', 'crear', 'asistente-soporte', '--grupo', 'Soporte', '--presupuesto', 'diario.peticiones=50', '--presupuesto', 'mensual.tokens=200000')
  cli('acceso', 'crear', 'resumen-notas', '--grupo', 'Notas')
  for (const texto of soloSitio ? [] : ['Resume en una línea: Tramtai pone agentes de IA detrás de una API.', 'Di «hola» en tres idiomas.']) {
    try { cli('ejecutar', '-a', agente, '-m', modelo.split('/')[1] ?? '', texto) } catch { console.warn(`aviso: ${modelo} no respondió; el Historial mostrará el error`) }
  }

  const depuracion = await puertoLibre()
  const nav = spawn(navegador(), ['--headless=new', `--remote-debugging-port=${depuracion}`, `--user-data-dir=${join(temporal, 'navegador')}`,
    '--no-first-run', '--no-default-browser-check', '--hide-scrollbars', '--disable-extensions',
    '--disable-component-extensions-with-background-pages', 'about:blank'], { stdio: 'ignore' })
  procesos.push(nav)
  let pestana
  await hasta(async () => {
    const lista = await (await fetch(`http://127.0.0.1:${depuracion}/json/list`)).json()
    pestana = lista.find((p) => p.type === 'page')
    return !!pestana
  }, 20, 'el navegador')
  const { enviar, cerrar } = await conectar(pestana.webSocketDebuggerUrl)
  const evaluar = async (expr) => (await enviar('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true })).result.value
  let carga = 0
  // abrir carga la página con ese tema e idioma (guardados en localStorage) y espera a que aparezca el selector.
  async function abrir(pagina, tema, idioma, selector, ancho, alto, guardado = {}) {
    await enviar('Emulation.setDeviceMetricsOverride', { width: ancho, height: alto, deviceScaleFactor: 1, mobile: false })
    await enviar('Page.navigate', { url: `${base}/?c=${++carga}` })
    await hasta(() => evaluar('document.readyState === "complete"'), 20, 'la carga')
    await evaluar(`localStorage.setItem('tramtai.tema', '${tema}'); localStorage.setItem('tramtai.idioma', '${idioma}')`)
    for (const [clave, valor] of Object.entries(guardado)) await evaluar(`localStorage.setItem(${JSON.stringify(clave)}, ${JSON.stringify(JSON.stringify(valor))})`)
    await enviar('Page.navigate', { url: `${base}/?c=${++carga}#${pagina}` })
    await hasta(() => evaluar(`!!document.querySelector('${selector}')`), 20, `${selector} en #${pagina}`)
    // Hasta que no quede nada cargando dentro (esqueletos de Quasar).
    await hasta(() => evaluar(`!document.querySelector('${selector} .q-skeleton, ${selector} .q-inner-loading')`), 20, `${selector} cargado`)
    await esperar(1000) // animaciones y datos que llegan después
  }
  // sinBarra oculta la barra superior fija, que taparía el recorte de un elemento.
  const sinBarra = () => evaluar(`document.head.insertAdjacentHTML('beforeend', '<style>.q-header{display:none!important}</style>')`)

  // Los iconos del sitio salen de favicon.svg con fondo transparente, y la imagen para compartir de og.html.
  if (!solo) {
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
  if (!soloSitio) await abrir('inicio', 'claro', 'es', '.pagina', 1280, 900)
  if (!soloSitio) await evaluar(`(async () => {
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
  for (const c of CAPTURAS.filter((x) => !soloSitio && (!solo || x.id === solo)))
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en']) {
        const selector = `[data-captura="${c.id}"]`
        await abrir(c.pagina, tema, idioma, selector, 1280, 900, c.guardado)
        await sinBarra()
        await ocultarRuta()
        const r = await evaluar(`(() => { const e = document.querySelector('${selector}'); e.scrollIntoView({ block: 'start' });
          const b = e.getBoundingClientRect(); return { x: b.left + scrollX, y: b.top + scrollY, w: b.width, h: Math.min(b.height, 720) } })()`)
        const { data } = await enviar('Page.captureScreenshot', { format: 'webp', quality: 80, captureBeyondViewport: true,
          clip: { x: r.x, y: r.y, width: r.w, height: r.h, scale: 1 } })
        writeFileSync(join(destinoAyuda, `${c.id}.${tema}.${idioma}.webp`), Buffer.from(data, 'base64'))
        console.log(`${c.id}.${tema}.${idioma}.webp`)
      }

  if (!solo && !soloSitio)
    for (const p of PORTADA)
      for (const tema of ['claro', 'oscuro']) {
        await abrir(p.pagina, tema, 'es', '.pagina', 1440, 900)
        await ocultarRuta()
        // La misma pantalla de 1440 px, también reducida a 960 para los móviles (srcset).
        for (const ancho of [1440, 960]) {
          const { data } = await enviar('Page.captureScreenshot', { format: 'webp', quality: 80,
            clip: { x: 0, y: 0, width: 1440, height: 900, scale: ancho / 1440 } })
          writeFileSync(join(destinoPortada, `${p.nombre}-${tema}-${ancho}.webp`), Buffer.from(data, 'base64'))
          console.log(`site/capturas/${p.nombre}-${tema}-${ancho}.webp`)
        }
      }
  await enviar('Browser.close').catch(() => {})
  cerrar()
} finally {
  try { execFileSync(exe, ['servicio', 'detener'], { cwd: temporal, stdio: 'ignore' }) } catch { /* ya estaba detenido */ }
  for (const p of procesos) p.kill()
  await esperar(1500)
  try { rmSync(temporal, { recursive: true, force: true, maxRetries: 5, retryDelay: 500 }) } catch { console.warn('no pude borrar ' + temporal) }
}
