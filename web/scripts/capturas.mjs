// capturas saca las capturas de la Ayuda (web/src/ayuda/capturas: claro y oscuro, español e inglés), las de cada paso de
// los recorridos guiados (recorrido-*) y las de la portada (site/capturas: las cinco pantallas y la demo de la
// bienvenida) con una Tramtai compilada en una carpeta temporal y portable, en el 17373 (el puerto por defecto, el de los
// ejemplos de código) si está libre y si no en otro (la nota-capturas de site/index.html dice el puerto que se ve en
// ellas: cámbiala si cambia): no toca la instalación de
// quien lo corre. Se corre en el equipo de desarrollo, con un agente ya conectado (para el Historial):
//   cd web; npm run build; npm run capturas
//   (o: npm run capturas -- --solo receta, -- --recorridos, -- --bienvenida o -- --sitio)
// Usa Edge o Chrome sin ventana por su protocolo de depuración; TRAMTAI_NAVEGADOR elige otro ejecutable y
// TRAMTAI_CAPTURAS_MODELO el modelo de las peticiones de ejemplo (claude/haiku por defecto).
import { execFileSync, spawn } from 'node:child_process'
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:net'
import { homedir, tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import en from '../src/idiomas/en.ts'
import es from '../src/idiomas/es.ts'
import { ANCHO_AYUDA, ANCLAS, BIENVENIDA, CAPTURAS, PORTADA, SIN_CAPTURA, rutaPrivada } from './capturas-lista.ts'

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
const destinoSitio = fileURLToPath(new URL('../../site/', import.meta.url))
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
try {
  console.log('Compilando Tramtai…')
  execFileSync('go', ['build', '-o', exe, './cmd/tramtai'], { cwd: raiz, stdio: 'inherit' })
  const puerto = await puertoLibre(17373)
  // La portada dice que sus capturas usan el puerto por defecto, y algunas lo enseñan (Inicio, el selector de espacio).
  if (puerto !== 17373) console.warn(`aviso: el puerto 17373 está ocupado (¿una Tramtai abierta?); las capturas enseñarán el ${puerto}`)
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
  for (const texto of soloSitio || soloBienvenida ? [] : ['Resume en una línea: Tramtai pone agentes de IA detrás de una API.', 'Di «hola» en tres idiomas.']) {
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
  // sinBarra oculta la barra superior fija, que taparía el recorte de un elemento.
  const sinBarra = (oculta = true) => evaluar(`(() => { let s = document.getElementById('sin-barra'); if (!s) { s = document.createElement('style'); s.id = 'sin-barra'; document.head.append(s) }
    s.textContent = ${oculta ? "'.q-header{display:none!important}'" : "''"} })()`)

  // Los iconos del sitio salen de favicon.svg con fondo transparente, y la imagen para compartir de og.html.
  if (!solo && !soloRecorridos && !soloBienvenida) {
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
  for (const c of CAPTURAS.filter((x) => !soloSitio && !soloRecorridos && !soloBienvenida && (!solo || x.id === solo)))
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en']) {
        const selector = `[data-captura="${c.id}"]`
        await abrir(c.pagina, tema, idioma, selector, 1280, 900, c.guardado)
        await sinBarra()
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

  // Un recorrido señala un elemento con el anillo de driver.js (.driver-active-element, ver estilos.css): se carga cada
  // página una vez por tema e idioma y se recorta cada ancla con 16 px de margen. Las de app.* (el selector de espacio
  // y el «?», que viven en una cabecera) recortan esa cabecera entera, que es el contexto que las hace reconocibles.
  // Una ancla que la demo no muestra no se inventa: se avisa y se apunta en SIN_CAPTURA, con el motivo.
  const visible = (ancla) => `[...document.querySelectorAll('[data-recorrido="${ancla}"]')].find((e) => { const b = e.getBoundingClientRect(); return b.width > 0 && b.height > 0 })`
  const ausentes = new Set()
  const aSacar = ANCLAS.filter((a) => !soloSitio && !soloBienvenida && (!solo || a.id === solo) && !(a.ancla in SIN_CAPTURA))
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

  if (!solo && !soloSitio && !soloRecorridos && !soloBienvenida)
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
  if (soloBienvenida || (!solo && !soloSitio && !soloRecorridos))
    for (const tema of ['claro', 'oscuro'])
     for (const idioma of ['es', 'en']) {
      const textos = idioma === 'es' ? es : en
      await abrir('inicio', tema, idioma, '.pagina', 1440, 900)
      await evaluar(`localStorage.removeItem('tramtai.bienvenida')`)
      await enviar('Page.navigate', { url: `${base}/?c=${++carga}#inicio` })
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
} finally {
  try { execFileSync(exe, ['servicio', 'detener'], { cwd: temporal, stdio: 'ignore' }) } catch { /* ya estaba detenido */ }
  for (const p of procesos) p.kill()
  await esperar(1500)
  try { rmSync(temporal, { recursive: true, force: true, maxRetries: 5, retryDelay: 500 }) } catch { console.warn('no pude borrar ' + temporal) }
}
