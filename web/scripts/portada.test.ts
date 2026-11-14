import { describe, test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { pasosBienvenida } from '../src/recorrido.ts'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { PORTADA, TOKENS_TERMINAL } from './capturas-lista.ts'
import { ANCLAS_PORTADA, AVISO_DESCARGA, destinoPortada } from './sitio.ts'
import { conDescargas } from './descargas.mjs'

const leer = (p: string) => readFileSync(new URL(`../../site/${p}`, import.meta.url), 'utf8')
// v0.61 (spec §4): Inicio tiene cuatro bloques. Lo que antes estaba en la portada pasó tal cual (ruling 3) a Producto
// («por-que», «como», «funciones» y «privacidad»), Agentes y Descargar: sus pruebas lo leen de ahí hasta que las Tareas 5
// a 7 rehagan esas páginas.
const inicio = leer('index.html')
const html = [inicio, leer('producto.html'), leer('agentes.html'), leer('descargar.html')].join('\n')
// seccion respeta las <section> anidadas (las líneas del plano de Funciones lo son): corta en el </section> de la suya.
const seccion = (id: string, de = html) => {
  const comienzo = de.search(new RegExp(`<section[^>]*id="${id}"`))
  if (comienzo < 0) return ''
  const etiquetas = /<section\b|<\/section>/g
  etiquetas.lastIndex = comienzo
  let nivel = 0
  for (let m = etiquetas.exec(de); m; m = etiquetas.exec(de)) {
    nivel += m[0] === '</section>' ? -1 : 1
    if (nivel === 0) return de.slice(comienzo, m.index + m[0].length)
  }
  return ''
}
const js = leer('sitio.js')
const css = leer('estilo.css')
// funcionDe saca una función de arriba del todo de sitio.js (sin sangría) para probarla sin navegador.
const funcionDe = (nombre: string) =>
  new Function(`${js.match(new RegExp(`\\nfunction ${nombre}\\([\\s\\S]*?\\n\\}`))![0]}\nreturn ${nombre}`)()
const pasos = pasosBienvenida()

// ---------------------------------------------------------------------------------------------------------------------
// Inicio (v0.61, spec §4)

test('Inicio tiene sus cuatro bloques', () => {
  assert.deepEqual([...inicio.matchAll(/<section\b[^>]*\bid="([^"]+)"/g)].map((m) => m[1]), ['arranque', 'fig', 'formas', 'sigue'])
  assert.match(inicio, /<section id="arranque" class="arranque">/, 'la cuadrícula y el brillo van detrás del arranque')
  assert.match(seccion('fig', inicio), /<div class="mosaico" style="--col: 3">/)
})

test('el arranque lleva el titular en dos partes', () => {
  const h1 = inicio.match(/<h1 class="titular">([\s\S]*?)<\/h1>/)?.[1] ?? ''
  assert.match(h1, /<span lang="es">Tus agentes de IA,<br> <em>como una API local\.<\/em><\/span>/)
  assert.match(h1, /<span lang="en">Your AI agents,<br> <em>as a local API\.<\/em><\/span>/)
  assert.equal((inicio.match(/<h1\b/g) ?? []).length, 1)
  // Una frase de qué es y otra de lo que cambia en tu app; sin pastillas.
  const entrada = (inicio.match(/<p class="entrada"><span lang="es">([^<]*)<\/span>/)?.[1] ?? '').split('. ')
  assert.equal(entrada.length, 2, entrada.join(' | '))
  assert.match(seccion('arranque', inicio), /solo cambias la URL y la clave\.<\/span>/)
  assert.match(seccion('arranque', inicio), /you only change the URL and the key\.<\/span>/)
  assert.doesNotMatch(seccion('arranque', inicio), /class="chapa"/)
  assert.match(seccion('arranque', inicio), /<a class="boton secundario" href="producto\.html#como">/, '«Cómo funciona» lleva a Producto')
})

// El botón de descarga empieza en «Descargar» (a Descargar) y sitio.js le pone el sistema detectado; el resto de
// sistemas va en el desplegable. Debajo, cómo se instala en cada sistema, con los comandos de hoy.
test('Descargar para tu sistema, con los demás en el desplegable y cómo se instala en cada uno', () => {
  const arranque = seccion('arranque', inicio)
  assert.match(arranque, /<a class="boton principal" href="descargar\.html" data-descarga>[\s\S]*?<span class="texto-descarga"><span lang="es">Descargar<\/span><span lang="en">Download<\/span><\/span><\/a>/)
  const menu = arranque.match(/<details class="desplegable">[\s\S]*?<\/details>/)?.[0] ?? ''
  assert.match(menu, /<summary class="boton principal" aria-label="Otros sistemas" data-aria-en="Other systems">/)
  assert.deepEqual([...menu.matchAll(/data-archivo="([^"]+)"/g)].map((m) => m[1]), ['instalador-windows-x64.exe', 'macos.dmg', 'linux-amd64.deb', 'linux-amd64.rpm'])
  assert.match(menu, /href="descargar\.html"/)
  // Cada sistema, su bloque: el primer data-archivo es el archivo del botón cuando ese es el sistema.
  const bloques = [...arranque.matchAll(/<div class="instalar-sistema" data-sistema="(\w+)">([\s\S]*?)\n {8}<\/div>/g)]
  assert.deepEqual(bloques.map((b) => [b[1], b[2].match(/data-archivo="([^"]+)"/)?.[1]]), [['macos', 'macos.dmg'], ['windows', 'instalador-windows-x64.exe'], ['linux', 'linux-amd64.deb']])
  assert.ok(bloques[0][2].includes('<pre class="bloque-codigo"><code>brew tap mscottb/tramtai https://github.com/MscottB/Tramtai\nbrew install --cask tramtai</code></pre>'))
  assert.match(bloques[1][2], /Descargar el instalador/)
  assert.match(bloques[2][2], /\.deb[\s\S]*\.rpm/)
  // Hasta la Tarea 5, descargas.mjs rellena Inicio (ruling 4).
  assert.match(arranque, /<span data-version><\/span>/)
  assert.match(arranque, /<script type="application\/json" id="descargas">\{\}<\/script>/)
  // Con el sistema conocido se ve solo el suyo; el desplegable se cierra con Escape o fuera.
  for (const s of ['windows', 'macos', 'linux'])
    assert.ok(css.includes(`html[data-tu-sistema="${s}"] .instalar-sistema:not([data-sistema="${s}"])`), s)
  assert.match(js, /document\.documentElement\.dataset\.tuSistema = sistema/)
  assert.match(js, /details\.desplegable[\s\S]*?e\.key !== 'Escape'/)
})

// Riesgo 3 del plan: con un sistema que no se reconoce, o sin paquete seguro, «Descargar» a secas, nunca un archivo
// equivocado.
test('sistemaDe', () => {
  const sistemaDe = funcionDe('sistemaDe')
  const casos: [string, string | undefined, string | null][] = [
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36', 'Win32', 'windows'],
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0', undefined, 'windows'],
    ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15', 'MacIntel', 'macos'],
    ['Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0', 'Linux x86_64', 'linux'],
    ['Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1', 'iPad', null],
    ['Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36', 'Linux armv81', null],
    ['', '', null],
    ['', undefined, null],
    ['Mozilla/5.0 (X11; Linux aarch64; rv:132.0) Gecko/20100101 Firefox/132.0', 'Linux aarch64', null],
    // Chromium en ARM dice x86_64 en su texto, pero no en la plataforma.
    ['Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36', 'Linux aarch64', null],
    ['Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36', 'Linux x86_64', null],
  ]
  for (const [ua, plataforma, quiere] of casos) assert.equal(sistemaDe(ua, plataforma), quiere, `${ua} | ${plataforma}`)
  // El iPad con Safari de escritorio dice Macintosh: la pantalla táctil lo delata.
  assert.match(js, /if \(sistema === 'macos' && navigator\.maxTouchPoints > 1\) sistema = null;/)
  // En Linux ARM (Chromium) el botón vuelve a «Descargar».
  assert.match(js, /if \(v\.architecture !== 'arm'\) return;\s*sistema = null;/)
})

// Con un release publicado, el botón lleva al archivo de Windows o de macOS; en Linux, a Descargar: .deb o .rpm depende
// de la distribución (revisión de la Tarea 4).
test('con el release relleno, el botón de Linux no apunta a ningún paquete', () => {
  const tag = 'v0.61.0+14112026'
  const assets = ['instalador-windows-x64.exe', 'macos.dmg', 'linux-amd64.deb', 'linux-amd64.rpm', 'linux-arm64.deb'].map((a) => ({ name: `Tramtai-${tag}-${a}`, url: `https://ejemplo.test/${a}` }))
  const relleno = conDescargas(inicio, { tagName: tag, assets })
  const archivos = JSON.parse(relleno.match(/<script type="application\/json" id="descargas">([^<]*)<\/script>/)![1]).archivos
  assert.equal(archivos['linux-amd64.deb'], 'https://ejemplo.test/linux-amd64.deb', 'el release sí trae el .deb')
  const destinoDescarga = funcionDe('destinoDescarga')
  const principal = (s: string) => relleno.match(new RegExp(`data-sistema="${s}">[\\s\\S]*?data-archivo="([^"]+)"`))?.[1]
  assert.equal(destinoDescarga('windows', principal('windows'), archivos), 'https://ejemplo.test/instalador-windows-x64.exe')
  assert.equal(destinoDescarga('macos', principal('macos'), archivos), 'https://ejemplo.test/macos.dmg')
  assert.equal(destinoDescarga('linux', principal('linux'), archivos), null)
  assert.equal(destinoDescarga(null, principal('windows'), archivos), null)
  assert.equal(destinoDescarga('windows', principal('windows'), {}), null, 'sin release, a Descargar')
  assert.match(js, /var destino = destinoDescarga\(sistema, principal && principal\.dataset\.archivo, archivos\);/)
})

// FIG 1: la terminal que se escribe sola, con datos de ejemplo fijos y agentes y modelos del catálogo.
const fig1 = inicio.match(/<pre class="terminal-viva">[\s\S]*?<\/pre>/)?.[0] ?? ''
test('FIG 1 enseña el cambio de dos líneas', () => {
  const lineas = fig1.replace(/<[^>]+>/g, '').split('\n')
  assert.ok(lineas.some((l) => l.startsWith('- base_url=') && !l.includes('127.0.0.1')), 'la línea que se quita')
  assert.ok(lineas.some((l) => l.startsWith('+ base_url=') && l.includes('127.0.0.1:17373')), 'la de Tramtai')
  // Se teclea a una letra por paso: --pasos es la longitud de la orden en cada idioma.
  const ordenes = [...fig1.matchAll(/<span class="escribe" lang="(es|en)" style="--retraso: [\d.]+s; --pasos: (\d+)">([^<]+)<\/span>/g)]
  assert.deepEqual(ordenes.map((m) => [m[1], m[3]]), [['es', 'tramtai servicio iniciar'], ['en', 'tramtai service start']])
  for (const m of ordenes) assert.equal(Number(m[2]), m[3].length, m[3])
  assert.match(css, /steps\(var\(--pasos, 24\), end\)/)
  // Cada petición con su agente y modelo (del catálogo: docs/api.md) y su tiempo en cada idioma.
  const modelos = [...fig1.replace(/<[^>]+>/g, '').matchAll(/● ([a-z]+\/[a-z0-9.-]+)/g)].map((m) => m[1])
  assert.deepEqual(modelos, ['claude/sonnet', 'codex/gpt-5.5', 'claude/haiku', 'copilot/auto'])
  assert.equal((fig1.match(/<span lang="es">\d,\d s<\/span><span lang="en">\d\.\d s<\/span>/g) ?? []).length, 4)
  assert.match(inicio, /<figure class="tarjeta ancha alta fig-terminal vivo">\s*<figcaption class="fig"><span lang="es">FIG 1 · /)
})

test('lo vivo tiene su estado final sin JavaScript', () => {
  // Las líneas de FIG 1 no se esconden en el HTML: solo el CSS de html.js .vivo las espera.
  assert.doesNotMatch(fig1, /style="[^"]*(opacity|display)|\shidden[\s>]/)
  for (const [, selector, cuerpo] of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const s = selector.replace(/\/\*[\s\S]*?\*\//g, '').trim()
    if (/^(from|to|\d+%)$/.test(s)) continue
    if (/opacity:\s*0(?![.\d])|clip-path:\s*inset\(0 100%|stroke-dashoffset:\s*1\b/.test(cuerpo)) assert.match(s, /^html\.js \.vivo:not\(\.visto\) /, s)
  }
  // Esos estados de partida solo existen sin «reducir movimiento».
  const sinMovimiento = css.slice(css.indexOf('@media (prefers-reduced-motion: no-preference)'))
  assert.match(sinMovimiento.slice(0, sinMovimiento.indexOf('\n}')), /html\.js \.vivo:not\(\.visto\) \.escribe \{ clip-path: inset\(0 100% 0 0\); \}/)
  // Las tres figuras vivas: la terminal, el camino (se dibuja, corre y late) y los agentes (laten).
  assert.equal((inicio.match(/<figure class="tarjeta[^"]* vivo">/g) ?? []).length, 3)
  const camino = inicio.match(/<figure class="tarjeta fig-camino vivo">[\s\S]*?<\/figure>/)?.[0] ?? ''
  assert.equal((camino.match(/class="dibuja" pathLength="1"/g) ?? []).length, 2)
  assert.equal((camino.match(/class="flujo" stroke-dasharray="4 6"/g) ?? []).length, 2, 'sin JavaScript el flujo se ve punteado y quieto')
  assert.equal((camino.match(/<circle class="pulso"/g) ?? []).length, 4)
  assert.match(camino, /<svg [^>]*role="img" aria-label="[^"]+" data-aria-en="[^"]+">/)
})

test('FIG 2 y 3: los colores de los agentes solo en sus puntos; FIG 3 con los tres estados', () => {
  const camino = inicio.match(/<figure class="tarjeta fig-camino vivo">[\s\S]*?<\/figure>/)?.[0] ?? ''
  for (const [, etiqueta] of camino.matchAll(/(<[^>]*var\(--(?:claude|codex|copilot|gemini|antigravity)\)[^>]*>)/g)) assert.match(etiqueta, /^<circle /, etiqueta)
  const agentes = inicio.match(/<ul class="estado-agentes">[\s\S]*?<\/ul>/)?.[0] ?? ''
  assert.deepEqual([...agentes.matchAll(/<\/span>([^<]+) <span class="estado/g)].map((m) => m[1]), ['Claude Code', 'Codex', 'GitHub Copilot', 'Gemini API', 'Antigravity'])
  for (const t of ['listo', 'versión nueva', 'sin instalar', 'ready', 'update available', 'not installed']) assert.ok(agentes.includes(`>${t}<`), t)
  assert.match(agentes, /<li class="sin-instalar"><span class="punto" style="--c: var\(--antigravity\)"><\/span>/, 'sin instalar no late')
  // Las figuras con datos de ejemplo y las capturas lo dicen, con el puerto de las capturas frente al de los ejemplos.
  const nota = inicio.match(/<p class="nota nota-figuras">[\s\S]*?<\/p>/)?.[0] ?? ''
  assert.match(nota, /datos de ejemplo[\s\S]*17388[\s\S]*17373/)
  assert.match(nota, /sample data[\s\S]*17388[\s\S]*17373/)
  const puerto = readFileSync(new URL('./capturas.mjs', import.meta.url), 'utf8').match(/const PUERTO_CAPTURAS = (\d+)/)?.[1]
  assert.equal(puerto, '17388')
  // Fuera de la terminal (una captura en texto) y de las notas, el puerto que se enseña es el de Tramtai.
  const sinCapturas = inicio.replace(/<!-- comun:terminal -->[\s\S]*?<!-- \/comun:terminal -->/, '').replace(/<p class="nota[^"]*">[\s\S]*?<\/p>/g, '')
  for (const [, p] of sinCapturas.matchAll(/127\.0\.0\.1:(\d+)/g)) assert.equal(p, '17373')
})

// Las capturas: WebP con su tamaño, en los dos idiomas y temas; la primera de la página carga sin esperar y el resto,
// con carga diferida (spec, Riesgos).
test('las capturas de Inicio son WebP con tamaño, pareja en el otro idioma y carga diferida salvo la primera', () => {
  const imgs = inicio.match(/<img [^>]*src="capturas\/[^>]*>/g) ?? []
  assert.equal(imgs.length, 16, 'FIG 4, FIG 5, Web UI y Servidor, cada una en claro y oscuro y en los dos idiomas')
  imgs.forEach((img, i) => {
    const m = img.match(/^<img class="solo-(claro|oscuro)" lang="(es|en)" src="capturas\/([a-z-]+)-\1-\2-1440\.webp" srcset="[^"]+" sizes="[^"]+" width="1440" height="900"( loading="lazy")? decoding="async" alt="[^"]+">$/)
    assert.ok(m, img.slice(0, 160))
    assert.equal(Boolean(m[4]), i >= 4, `${m[3]}: ${i < 4 ? 'la primera, sin' : 'con'} carga diferida`)
    const otro = m[2] === 'es' ? 'en' : 'es'
    assert.ok(imgs.some((x) => x.includes(`class="solo-${m[1]}" lang="${otro}" src="capturas/${m[3]}-${m[1]}-${otro}-1440.webp"`)), `${m[3]}: falta su pareja`)
    for (const [, archivo] of img.matchAll(/capturas\/([a-z0-9-]+\.webp)/g)) statSync(new URL(`../../site/capturas/${archivo}`, import.meta.url))
  })
  assert.deepEqual([...new Set(imgs.map((i) => i.match(/capturas\/([a-z-]+?)-(claro|oscuro)/)?.[1]))], ['mis-apis', 'servidor-inicio'])
})

// Una ventana, cuatro formas (spec §4): pestañas con el patrón de las demás; sin JavaScript se ven las cuatro.
const formas = seccion('formas', inicio)
test('la ventana tiene cuatro pestañas', () => {
  const lista = formas.match(/<div class="pestanas-lista" role="tablist"[^>]*>[\s\S]*?<\/div>/)?.[0] ?? ''
  assert.match(lista, /aria-label="Formas de usarlo" data-aria-en="Ways to use it" hidden>/, 'sin JavaScript no hay pestañas que no hagan nada')
  const pestanas = lista.match(/<button type="button" role="tab"[\s\S]*?<\/button>/g) ?? []
  assert.deepEqual(pestanas.map((p) => p.replace(/<span lang="en">[^<]*<\/span>/g, '').replace(/<[^>]+>/g, '')), ['Web UI', 'Terminal', 'CLI', 'Servidor'])
  pestanas.forEach((p, i) => {
    const [id, panel] = [p.match(/ id="([^"]+)"/)?.[1], p.match(/aria-controls="([^"]+)"/)?.[1]]
    assert.match(p, new RegExp(`aria-selected="${i === 0}" tabindex="${i === 0 ? 0 : -1}"`))
    // Sin JavaScript se ven los cuatro paneles: ninguno empieza oculto ni es tabpanel hasta que sitio.js enciende la lista.
    assert.match(formas, new RegExp(`<div class="forma" id="${panel}" aria-labelledby="${id}">`), panel)
  })
  assert.doesNotMatch(formas, /role="tabpanel"|<div class="forma"[^>]* hidden/)
  assert.match(formas, /<div class="ventana" data-pestanas="formas">/)
  // Web UI: Mis APIs; Terminal: comun:terminal; CLI: órdenes reales; Servidor: el mapa.
  const panel = (id: string) => {
    const i = formas.indexOf(`id="forma-${id}-panel"`)
    const j = formas.indexOf('<div class="forma"', i)
    return formas.slice(i, j < 0 ? undefined : j)
  }
  assert.match(panel('webui'), /capturas\/mis-apis-/)
  assert.match(panel('terminal'), /<!-- comun:terminal -->\n<div class="ventana-terminal">/)
  assert.match(panel('servidor'), /capturas\/servidor-inicio-/)
  assert.match(panel('servidor'), /href="servidor\.html"[\s\S]*href="producto\.html#riesgos"/)
  const cli = panel('cli').match(/<pre class="bloque-codigo">[\s\S]*?<\/pre>/)?.[0] ?? ''
  for (const [o, e] of [['tramtai servicio iniciar -d', 'tramtai service start -d'], ['tramtai agentes', 'tramtai agents'], ['tramtai herramientas activar --todas', 'tramtai tools enable --all'], ['tramtai servicio estado', 'tramtai service status']])
    assert.ok(cli.includes(`<span lang="es">${o}</span><span lang="en">${e}</span>`), e)
  for (const c of cli.match(/<span class="c"[^>]*>/g) ?? []) assert.match(c, /lang="(es|en)"/)
  // Las pestañas son las de su lista: la terminal de dentro lleva las suyas. La ventana abre siempre en Web UI.
  assert.match(js, /function pestanasDe\(g\) \{ return g\.querySelector\('\[role="tablist"\]'\)\.querySelectorAll\('\[role="tab"\]'\); \}/)
  assert.match(js, /if \(guardar && nombre !== 'formas' && nombre !== 'descargar'\)/)
  assert.match(js, /if \(nombre !== 'formas' && nombre !== 'descargar'\) try \{ elegido = localStorage\.getItem\('tramtai\.' \+ nombre\); \}/, 'tampoco lee lo que guardaba antes')
  // La clave del lenguaje no depende de una página: Producto, la guía y quien venga la comparten; la Ayuda, la suya.
  assert.match(js, /var clave = document\.querySelector\('\.tema-ayuda'\) \? 'tramtai\.lenguaje-ayuda' : 'tramtai\.lenguaje';/)
  assert.doesNotMatch(js, /lenguaje-portada/)
})

// v0.59 (spec §5): la terminal es texto real (site/capturas/terminal.json, de capturas --terminal) en una ventana con
// pestañas que sigue el tema: Inicio, Mis APIs e Historial, en los dos idiomas. Ahora va en la pestaña Terminal.
test('la terminal de la ventana son seis pantallas de texto de terminal.json, sin colores en línea', () => {
  const zona = formas.match(/<!-- comun:terminal -->([\s\S]*?)<!-- \/comun:terminal -->/)?.[1] ?? ''
  assert.ok(zona, 'falta la zona comun:terminal (cd web; npm run sitio)')
  const datos = JSON.parse(leer('capturas/terminal.json')) as Record<string, Record<string, string>>
  assert.equal((zona.match(/<pre class="pantalla"/g) ?? []).length, 6, '3 páginas × 2 idiomas')
  for (const idioma of ['es', 'en'])
    for (const p of ['inicio', 'apis', 'historial']) {
      const fin = zona.indexOf(`">${datos[idioma][p]}</pre>`)
      assert.ok(fin > 0, `${p}, ${idioma}`)
      // role="img" con su texto: el lector de pantalla oye qué enseña la página, no 28 filas de columnas.
      const pre = zona.slice(zona.lastIndexOf('<pre ', fin), fin + 2)
      assert.match(pre, new RegExp(`^<pre class="pantalla" lang="${idioma}" role="img" aria-label="${idioma === 'es' ? 'La terminal en' : 'The terminal on'} [^"]+">$`), `${p}, ${idioma}`)
    }
  assert.doesNotMatch(zona, /style=|<img /)
  assert.deepEqual(readdirSync(new URL('../../site/capturas/', import.meta.url)).filter((a) => /^terminal-.*\.webp$/.test(a)), [], 'quedan terminal-*.webp')
  // Semáforo, rótulo y tres pestañas; sin JavaScript, Inicio y las otras en <details>.
  assert.ok(zona.includes('tramtai terminal · ssh'), 'falta el rótulo')
  const pestanas = zona.match(/<button type="button" role="tab"[\s\S]*?<\/button>/g) ?? []
  assert.deepEqual(pestanas.map((p) => p.replace(/<span lang="en">[^<]*<\/span>/g, '').replace(/<[^>]+>/g, '')), ['1 Inicio', '4 Mis APIs', '5 Historial'])
  assert.deepEqual((zona.match(/<details class="ventana-mas">[\s\S]*?<\/details>/g) ?? []).map((d) => d.match(/id="([^"]+)"/)?.[1]), ['terminal-apis-panel', 'terminal-historial-panel'])
  const bloque = js.slice(js.indexOf("document.querySelector('.ventana-terminal')"))
  assert.match(bloque, /lista\.addEventListener\('keydown'/)
  assert.match(bloque, /replaceWith/)
  // El pie dice de dónde salen, con el puerto de las capturas frente al tuyo.
  const pie = formas.match(/<p class="nota forma-nota">[\s\S]*?<\/p>/)?.[0] ?? ''
  assert.match(pie, /lang="es">[^<]*datos de ejemplo y en el puerto 17388; la tuya usa el 17373/)
  assert.match(pie, /lang="en">[^<]*sample data and on port 17388; yours uses 17373/)
  assert.match(formas, /href="ayuda\/empezar\.html#s-soloterminal"/)
})

// Cada color de la TUI (TOKENS_TERMINAL) es una clase con su valor de día y de noche: la ventana sigue el tema del sitio.
test('la hoja pinta cada token de la terminal en claro y oscuro, con Cascadia autoalojada y los glifos en su celda', () => {
  const pares = new Map<string, string[]>()
  for (const [hex, token] of Object.entries(TOKENS_TERMINAL)) pares.set(token, [...(pares.get(token) ?? []), hex])
  // Claude de día va en el tono de su chapa, #ba521c (DESIGN.md, Excepciones): #c4561d se queda en 4,47:1.
  pares.set('claude', ['#ba521c', pares.get('claude')![1]])
  for (const [token, [dia, noche]] of pares) {
    const regla = token === 'velo' ? `.tf-velo { background: light-dark(${dia}, ${noche}); }` : `.t-${token} { color: light-dark(${dia}, ${noche}); }`
    assert.ok(css.includes(regla), regla)
  }
  const fondo = css.match(/\.ventana-terminal \{[^}]*background: light-dark\((#\w{6}), (#\w{6})\)/)?.slice(1) ?? []
  assert.equal(fondo.length, 2)
  for (const [token, colores] of pares) if (token !== 'velo') colores.forEach((c, i) => assert.ok(contraste(c, fondo[i]) >= 4.5, `${token} ${i ? 'de noche' : 'de día'}: ${contraste(c, fondo[i]).toFixed(2)}:1`))
  assert.ok(contraste('#0f1b1e', pares.get('velo')![0]) >= 4.5 && contraste('#e5edf0', pares.get('velo')![1]) >= 4.5, 'texto sobre el velo')
  assert.ok(css.includes('.g { display: inline-block; width: 1ch; text-align: center; }'))
  const fuente = css.match(/@font-face \{[^}]*cascadia\.woff2[^}]*\}/)?.[0] ?? ''
  assert.match(fuente, /font-family: 'Tramtai Mono'/, 'Cascadia Code es nombre reservado (OFL)')
  assert.match(css, /\.ventana-pantalla \{[^}]*overflow-x: auto/)
  assert.match(css, /\.pantalla \{[^}]*clamp\(11px,/)
})

const luz = (h: string) => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)).reduce((s, v, i) => s + v * [0.2126, 0.7152, 0.0722][i], 0)
const contraste = (a: string, b: string) => { const [x, y] = [luz(a), luz(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05) }

// FIG 1 va sobre la tinta del tablero en los dos temas: sus colores llegan a 4,5:1 sobre ella.
test('los colores de FIG 1 se leen sobre el tablero', () => {
  const tableros = css.match(/--tablero: light-dark\((#\w{6}), (#\w{6})\)/)?.slice(1) ?? []
  assert.equal(tableros.length, 2)
  const colores = [...css.matchAll(/\.terminal-viva \.[\w-]+ \{ color: (#\w{6}); \}/g)].map((m) => m[1])
  assert.ok(colores.length >= 6)
  for (const c of colores) for (const t of tableros) assert.ok(contraste(c, t) >= 4.5, `${c} sobre ${t}: ${contraste(c, t).toFixed(2)}:1`)
})

// Sigue por aquí: una tarjeta por página. Spec §7: los anclajes de la portada de antes saltan a su sitio nuevo y, sin
// JavaScript, caen en la tarjeta (o la ventana) que lleva allí.
test('«Sigue por aquí» lleva a las seis páginas, con una línea cada una', () => {
  const tarjetas = seccion('sigue', inicio).match(/<a class="tarjeta pagina"[\s\S]*?<\/a>/g) ?? []
  assert.deepEqual(tarjetas.map((t) => t.match(/href="([^"]+)"/)?.[1]), ['producto.html', 'agentes.html', 'servidor.html', 'desarrolladores.html', 'ayuda/', 'descargar.html'])
  for (const t of tarjetas) assert.match(t, /<h3><span lang="es">[^<]+<\/span><span lang="en">[^<]+<\/span><\/h3>\s*<p><span lang="es">[^<]+<\/span><span lang="en">[^<]+<\/span><\/p>/)
})

test('los anclajes viejos saltan a su sitio nuevo, y sin JavaScript caen en lo que lleva allí', () => {
  const script = inicio.match(/<!-- comun:anclas -->\s*<script>([\s\S]*?)<\/script>/)?.[1] ?? ''
  assert.ok(script, 'falta comun:anclas (cd web; npm run sitio)')
  assert.ok(inicio.indexOf('<!-- comun:anclas -->') < inicio.indexOf('</head>'), 'en el <head>, antes de pintar')
  for (const [hash, destino] of [['#descargar', 'descargar.html'], ['#como', 'producto.html#como'], ['#terminal', '#formas'], ['#pagas', 'producto.html#por-que'], ['#formas', ''], ['', ''], ['#__proto__', '']]) {
    let a = ''
    new Function('location', 'addEventListener', script)({ hash, replace: (u: string) => { a = u } }, () => {})
    assert.equal(a, destino, hash)
  }
  // El salto es JavaScript escrito tal cual, y lleva cada anclaje a lo mismo que destinoPortada; también si el ancla
  // cambia con la página abierta.
  assert.doesNotMatch(script, /function destinoPortada/)
  for (const ancla of Object.keys(ANCLAS_PORTADA)) {
    let a = ''
    let alCambiar: (() => void) | null = null
    const location = { hash: '', replace: (u: string) => { a = u } }
    new Function('location', 'addEventListener', script)(location, (evento: string, f: () => void) => { if (evento === 'hashchange') alCambiar = f })
    location.hash = '#' + ancla
    alCambiar!()
    assert.equal(a, destinoPortada('#' + ancla), ancla)
  }
  for (const ancla of Object.keys(ANCLAS_PORTADA)) assert.match(inicio, new RegExp(`\\sid="${ancla}"`), ancla)
  assert.doesNotMatch(js, /pagas: 'por-que'/, 'el salto de antes de la v0.59 va ahora en ANCLAS_PORTADA')
})

// ---------------------------------------------------------------------------------------------------------------------
// Lo común del sitio que cuidaba la portada

test('la barra lleva a Producto, Agentes, Servidor, Desarrolladores, Ayuda y Descargar', () => {
  const nav = inicio.match(/<nav aria-label="Secciones"[\s\S]*?<\/nav>/)?.[0] ?? ''
  assert.deepEqual([...nav.matchAll(/href="([^"]+)"/g)].map((m) => m[1]), ['producto.html', 'agentes.html', 'servidor.html', 'desarrolladores.html', 'ayuda/', 'descargar.html'])
})

test('la cuadrícula y el brillo, solo en el arranque', () => {
  for (const [, selector, cuerpo] of css.matchAll(/([^{}]+)\{([^}]*)\}/g))
    if (/radial-gradient/.test(cuerpo)) assert.match(selector.trim(), /^\.arranque::/, selector)
  assert.doesNotMatch(css, /repeating-linear-gradient/)
})

test('mayúsculas espaciadas solo en los rótulos «FIG» y la cabecera del tablero (35) y ámbar solo para cifras (33)', () => {
  const reglas = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].filter(([, , cuerpo]) => /text-transform:\s*uppercase/.test(cuerpo)).map(([, s]) => s.replace(/\/\*[\s\S]*?\*\//g, '').trim())
  assert.deepEqual(reglas.sort(), ['.fig', '.tablero-cabecera'])
  assert.doesNotMatch(css + html, /--cifra|#ffcf5c/)
})

test('cada control mide 44 px y la barra no tapa lo que enlaza un ancla (23, 62)', () => {
  for (const clase of ['.github', '.menu-boton'])
    assert.match(css, new RegExp(`\\${clase} \\{[^}]*width: 44px; height: 44px`), clase)
  for (const clase of ['.boton', '.copiar', '.pestanas-lista button', '.idioma button'])
    assert.match(css, new RegExp(`${clase.replace('.', '\\.')} \\{[^}]*min-height: 44px`), clase)
  assert.match(css, /\.desplegable a \{[^}]*min-height: 44px/)
  assert.match(css, /section\[id\] \{ scroll-margin-top: 72px; \}/)
  assert.match(css, /a\.pagina \{[^}]*scroll-margin-top: 80px/)
})

test('el foco se ve: sin recortes en ES/EN y en verde claro dentro del código (20)', () => {
  assert.doesNotMatch(css, /\.idioma \{[^}]*overflow: hidden/)
  assert.match(css, /\.codigo :focus-visible \{ outline-color: var\(--tablero-clave\); \}/)
  assert.doesNotMatch(css, /:focus-visible \{[^}]*border-radius/)
})

test('sin JavaScript el menú se ve y nada se mueve; el paso del cursor no levanta nada (60, 61)', () => {
  assert.match(css, /html\.js \.menu-panel \{ display: none;/)
  // Lo único que se anima es lo vivo, con JavaScript y ya en pantalla (v0.61, spec §1).
  for (const [, selector, cuerpo] of css.matchAll(/([^{}]+)\{([^}]*)\}/g))
    if (/animation:(?!\s*none)/.test(cuerpo)) assert.match(selector.trim(), /^html\.js \.vivo\.visto /, selector)
  assert.doesNotMatch(css, /translateY/)
})

test('la portada de antes no deja restos: ni tranvía ni accesos; el recorrido guiado está en Producto', () => {
  assert.doesNotMatch(js, /Recorrido de la portada/)
  assert.doesNotMatch(css, /\.(portada|accesos|escenas|mapa-pestana|tranvia|terminal-figura|teclas)\b/)
  assert.doesNotMatch(html, /class="(portada|accesos|escenas|mapa)[ "]/)
  assert.doesNotMatch(inicio, /class="demo|id="demo"/)
})

// ---------------------------------------------------------------------------------------------------------------------
// Producto (v0.61, spec §5, Tarea 6): el arranque corto; Cómo funciona en tres pasos con su diagrama; Funciones en
// tarjetas con captura, con «La app» y el recorrido guiado de 14 pasos dentro; Por qué Tramtai con sus cifras, las siete
// ventajas en un mosaico y la comparativa plegada; y Privacidad entera.
const producto = leer('producto.html')
const funcionesProducto = seccion('funciones', producto)
const corte = funcionesProducto.indexOf('<div id="demo"')
const asi = corte < 0 ? funcionesProducto : funcionesProducto.slice(0, corte)
const demo = corte < 0 ? '' : funcionesProducto.slice(corte)
const figuras = demo.match(/<figure class="captura demo-paso"[\s\S]*?<\/figure>/g) ?? []
// texto quita las etiquetas y junta los espacios: lo que lee quien visita la página.
const texto = (s: string) => s.replace(/<[^>]+>/g, '').replace(/\s+/g, ' ')

// Las siete ventajas, copiadas de la línea de ventajas de la portada de antes de la v0.61 (git show 2d00e09:site/index.html).
const VENTAJAS = [
  ['Usa lo que ya pagas', 'tu clave de API o, solo para tu uso personal, tu suscripción de Claude, ChatGPT o GitHub Copilot. Sin cuentas nuevas.'],
  // v0.60: la de preparar los agentes.
  ['Tramtai instala y conecta tus agentes', 'instala Claude Code, Codex, GitHub Copilot o Antigravity, te ayuda a iniciar sesión y los mantiene al día, sin abrir otra consola.'],
  ['Sin tocar tu código', 'con el SDK de OpenAI o Anthropic, cambias la URL y la clave. A Claude Code, Codex, Continue, opencode y Kilo Code los apunta Tramtai.'],
  ['Si un agente falla, sigue otro', 'llega a su límite y la petición continúa con el siguiente de tu lista.'],
  ['Todo corre en tu equipo', 'sin servidor nuestro ni telemetría; tus claves, cifradas en el almacén del sistema, salvo la de Continue, opencode y Kilo Code: en su archivo.'],
  ['Cada app, con su clave y su límite', 'presupuesto diario o mensual por app y cada petición a la vista.'],
  ['Escritorio, terminal o servidor', 'la misma Tramtai en la app, por SSH o siempre encendida en tu servidor.'],
]
// El texto de la sección «privacidad» de la portada de antes de la v0.61 (git show 2d00e09:site/index.html), entero; los
// cinco riesgos los rellena comun:riesgos y los vigila su prueba.
const PRIVACIDAD = [
  'Privacidad', 'Privacy', 'Tramtai no tiene servidor propio.', 'Tramtai has no server of its own.',
  'Funciona en tu equipo y no tiene cuentas. Tus peticiones van de tu equipo al proveedor del agente que elijas, no a Tramtai.',
  'It runs on your computer and has no accounts. Your requests go from your computer to the provider of the agent you choose, not to Tramtai.',
  'Red', 'Network',
  'Por defecto escucha solo en 127.0.0.1; abrirlo a otros equipos es un ajuste aparte, apagado de serie, y solo arranca con HTTPS. Cada API exige su propia clave.',
  'By default it listens only on 127.0.0.1; opening it to other computers is a separate setting, off out of the box, and it only starts with HTTPS. Every API requires its own key.',
  'Claves', 'Keys',
  'Tus claves de proveedor quedan cifradas en el almacén del sistema, no en los archivos de Tramtai. La clave de cada API y la de cada espacio solo se ven al crearlas o regenerarlas, Tramtai guarda solo su huella y no sirven para abrir la app.',
  'Your provider keys are encrypted in the system store, not in Tramtai’s files. Each API’s key and each workspace’s key are shown only when they are created or regenerated, Tramtai stores only a hash of them and they can’t open the app.',
  'Sesiones', 'Sessions',
  'Tramtai no lee las credenciales de los CLI: les pregunta con qué cuenta iniciaste sesión. Con GitHub Copilot copia a su carpeta privada qué cuenta es, sin su clave.',
  'Tramtai doesn’t read the CLIs’ credentials: it asks them which account you signed in with. With GitHub Copilot it copies which account it is to its private folder, without its key.',
  'Datos', 'Data',
  'Las respuestas se guardan 24 horas por defecto; lo cambias en Ajustes, donde también vacías el historial.',
  'Responses are kept for 24 hours by default; you change that in Settings, where you can also clear the history.',
  'Telemetría', 'Telemetry',
  'Ninguna. Tramtai no envía estadísticas de uso. Solo consulta GitHub para buscar versiones nuevas, y eso se apaga en Ajustes.',
  'None. Tramtai sends no usage statistics. It only checks GitHub for new versions, and you can turn that off in Settings.',
  'Antes de usarlo, cinco riesgos.', 'Before you use it, five risks.',
  'Los mismos que te enseña Tramtai antes de aceptar sus términos, en la app, la terminal y al instalar el servidor.',
  'The same ones Tramtai shows you before you accept its terms, in the app, the terminal and when you install the server.',
  'Los términos de uso completos', 'The full terms of use',
]

describe('producto', () => {
  test('Producto tiene sus secciones', () => {
    assert.deepEqual([...producto.matchAll(/<section\b[^>]*\bid="([^"]+)"/g)].map((m) => m[1]), ['como', 'funciones', 'por-que', 'privacidad'])
    assert.equal((producto.match(/<h1\b/g) ?? []).length, 1)
    assert.match(producto, /<main id="contenido" tabindex="-1" class="producto">\s*(<!--[^>]*-->\s*)?<div class="arranque">/, 'el arranque corto, con la cuadrícula y el brillo')
    assert.match(producto, /<p class="entrada"><span lang="es">Cómo funciona, qué hace, por qué usarlo y qué datos salen de tu equipo\.<\/span><span lang="en">How it works, what it does, why use it and which data leaves your computer\.<\/span><\/p>/)
    // La parte en serifa de los h2 sale de una sola regla, la del titular, en todas las páginas.
    assert.deepEqual([...css.matchAll(/([^{}]*\bem\b[^{}]*)\{[^}]*var\(--serifa\)/g)].map((m) => m[1].replace(/\/\*[\s\S]*?\*\//g, '').trim()), ['h2 em, .titular em'])
    assert.match(seccion('por-que', producto), /<details class="comparativa" id="comparativa">/)
    assert.match(funcionesProducto, /<div id="demo">/, 'el recorrido guiado va dentro de Funciones')
    for (const id of ['como', 'funciones', 'por-que', 'comparativa', 'privacidad', 'demo', 'riesgos'])
      assert.equal((producto.match(new RegExp(`\\sid="${id}"`, 'g')) ?? []).length, 1, id)
  })

  test('las siete ventajas siguen', () => {
    const porQueProducto = seccion('por-que', producto)
    const lista = porQueProducto.match(/<ol class="mosaico ventajas"[^>]*>[\s\S]*?<\/ol>/)?.[0] ?? ''
    const ventajas = lista.match(/<li class="tarjeta[^"]*">[\s\S]*?<\/li>/g) ?? []
    assert.equal(ventajas.length, VENTAJAS.length)
    ventajas.forEach((v, i) => {
      assert.ok(v.includes(`<h3><span lang="es">${VENTAJAS[i][0]}</span>`), VENTAJAS[i][0])
      const frase = VENTAJAS[i][1][0].toUpperCase() + VENTAJAS[i][1].slice(1)
      assert.ok(v.includes(`<p><span lang="es">${frase}</span>`), frase)
      const ingles = v.match(/<p>[\s\S]*?<span lang="en">([^<]*)<\/span>/)?.[1] ?? ''
      assert.ok(ingles && ingles.split(/\s+/).length <= 25, `25 palabras como mucho: ${ingles}`)
    })
    // Un mosaico de distinto tamaño: la primera y la última, anchas.
    assert.match(ventajas[0], /^<li class="tarjeta ancha">/)
    assert.match(ventajas[6], /^<li class="tarjeta ancha">/)
    // El número de cada una es adorno: el lector de pantalla ya dice el de la lista.
    for (const v of ventajas) assert.match(v, /<span class="fig" aria-hidden="true">0\d<\/span>/)
    assert.match(css, /\.pasos-como li::before \{ content: '0' counter\(paso\); content: '0' counter\(paso\) \/ '';/)
    assert.doesNotMatch(lista, COLORES_LINEA)
  })

  test('la comparativa va plegada', () => {
    const comparativa = producto.match(/<details class="comparativa" id="comparativa">[\s\S]*?<\/details>/)?.[0] ?? ''
    assert.ok(comparativa, 'falta el <details> de la comparativa')
    assert.doesNotMatch(comparativa, /<details[^>]* open/)
    assert.match(comparativa, /<summary><span lang="es">Sin Tramtai y con Tramtai<\/span><span lang="en">Without and with Tramtai<\/span><\/summary>/)
    assert.equal((comparativa.match(/<figure class="pagas-diagrama[^"]*">/g) ?? []).length, 2, 'Sin Tramtai y Con Tramtai')
    // Va debajo de las ventajas, y sitio.js la abre si se llega por su ancla.
    assert.ok(producto.indexOf('class="mosaico ventajas"') < producto.indexOf('id="comparativa"'))
    assert.match(js, /destino\.tagName === 'DETAILS'\) destino\.open = true/)
  })

  test('privacidad completa', () => {
    const privacidad = texto(seccion('privacidad', producto))
    for (const t of PRIVACIDAD) assert.ok(privacidad.includes(t), t)
    assert.match(seccion('privacidad', producto), /class="tablero"[\s\S]*<div class="cabeza riesgos-cabeza" id="riesgos">[\s\S]*<!-- comun:riesgos -->/)
  })

  // Cómo funciona: instala Tramtai → prepara tus agentes → cambia la URL y la clave, en un diagrama en perspectiva que se
  // dibuja con .vivo (como FIG 2 de Inicio); el texto de cada paso va en el HTML, debajo.
  test('Cómo funciona dibuja los tres pasos en SVG con .vivo', () => {
    const como = seccion('como', producto)
    const diagrama = como.match(/<figure class="tarjeta fig-camino vivo">[\s\S]*?<\/figure>/)?.[0] ?? ''
    assert.ok(diagrama, 'falta el diagrama')
    assert.match(diagrama, /<figcaption class="fig"><span lang="es">FIG 1 · /)
    assert.match(diagrama, /<svg [^>]*role="img" aria-label="[^"]+" data-aria-en="[^"]+">/)
    assert.equal((diagrama.match(/class="dibuja" pathLength="1"/g) ?? []).length, 3, 'una pieza por paso')
    assert.equal((diagrama.match(/class="flujo" stroke-dasharray="4 6"/g) ?? []).length, 2, 'sin JavaScript el flujo se ve punteado y quieto')
    assert.doesNotMatch(diagrama, /style="[^"]*(opacity|display)|\shidden[\s>]/)
    const pasosComo = [...como.matchAll(/<li>\s*<h3><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/h3>/g)].map((m) => [m[1], m[2]])
    assert.deepEqual(pasosComo, [['Instala Tramtai', 'Install Tramtai'], ['Prepara tus agentes', 'Set up your agents'], ['Cambia la URL y la clave', 'Change the URL and the key']])
    assert.doesNotMatch(como, COLORES_LINEA)
  })

  // Funciones: doce tarjetas en cuatro líneas (conecta, controla, protege y usa), cada una con un recorte real (los de la
  // Tarea 9; de momento, las zonas del recorrido guiado), y las cinco pantallas de «La app» con su captura entera.
  test('Funciones son tarjetas con captura: cuatro líneas de tres, sin perder ninguna función', () => {
    const lineas = asi.match(/<div class="linea-funciones">[\s\S]*?<\/div>\s*<\/div>/g) ?? []
    assert.deepEqual(lineas.map((l) => l.match(/<h3><span lang="es">([^<]+)<\/span>/)?.[1]), ['Conecta', 'Controla', 'Protege', 'Usa'])
    const tarjetas = lineas.flatMap((l) => l.match(/<article class="tarjeta funcion">[\s\S]*?<\/article>/g) ?? [])
    assert.equal(tarjetas.length, 12)
    for (const t of tarjetas) {
      assert.match(t, /<span class="fig"><span lang="es">FIG \d+ · [^<]+<\/span><span lang="en">FIG \d+ · [^<]+<\/span><\/span>/)
      assert.match(t, /<h4><span lang="es">[^<]+<\/span><span lang="en">[^<]+<\/span><\/h4>\s*<p><span lang="es">[\s\S]*<\/span><span lang="en">/)
      assert.equal((t.match(/<img class="solo-(claro|oscuro)" lang="(es|en)" src="capturas\/[a-z0-9-]+\.webp"[^>]* loading="lazy" decoding="async" alt="[^"]+">/g) ?? []).length, 4, t.slice(0, 160))
    }
    // Las doce funciones de antes, en su orden y en los dos idiomas.
    assert.deepEqual(tarjetas.map((x) => x.match(/<h4><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/h4>/)?.slice(1)), [
      ['Compatible con tus SDK', 'Works with your SDKs'], ['API propia con OpenAPI', 'Native API with OpenAPI'], ['Respuestas con forma', 'Shaped responses'],
      ['Espacios de trabajo', 'Workspaces'], ['Una API por aplicación', 'One API per app'], ['Modelo y esfuerzo en cada petición', 'Model and effort per request'],
      ['Respaldo entre agentes', 'Agent fallback'], ['Claves cifradas en el almacén del sistema', 'Keys encrypted in the system store'], ['Sin terminal ni web', 'No terminal, no web'],
      ['La terminal, a la par de la app', 'The terminal, on par with the app'], ['Adjunta PDF, Word y Excel', 'Attach PDF, Word and Excel'], ['Recorrido y Ayuda', 'Tour and Help'],
    ])
    // Cada rótulo nombra su función, no la captura provisional (la Tarea 9 trae los recortes sin tocarlos).
    assert.deepEqual(tarjetas.map((x) => x.match(/<span class="fig"><span lang="es">(FIG \d+ · [^<]+)<\/span>/)?.[1]), [
      'FIG 8 · Tus SDK', 'FIG 9 · API propia', 'FIG 10 · Respuestas con forma', 'FIG 11 · Espacios', 'FIG 12 · Una API por app', 'FIG 13 · Modelo y esfuerzo',
      'FIG 14 · Respaldo', 'FIG 15 · Claves cifradas', 'FIG 16 · Sin terminal ni web', 'FIG 17 · La terminal', 'FIG 18 · Adjuntos', 'FIG 19 · Recorrido y Ayuda',
    ])
    // La de la terminal lleva a la ventana de Inicio; la del recorrido, al recorrido de esta página.
    const terminal = tarjetas.find((x) => x.includes('La terminal, a la par de la app')) ?? ''
    assert.match(terminal, /href="\.\/#formas"/)
    assert.match(terminal, /SSH/)
    assert.match(funcionesProducto, /href="#demo">míralo paso a paso<\/a>/)
    assert.equal((leer('desarrolladores.html').match(/href="producto\.html#demo"/g) ?? []).length, 2)
    assert.doesNotMatch(asi, /\b(nunca|siempre|sin cortes|a nadie|never|always|no one)\b/i)
    // El plano de metro se fue con su JavaScript y su CSS.
    assert.doesNotMatch(producto + js + css, /\bmetro\b|\.metro|parada-(boton|titulo|texto)/)
  })

  test('Funciones guarda «La app»: sus cinco pantallas con su texto y su captura, en tarjetas de distinto tamaño', () => {
    const pantallas = asi.match(/<div class="mosaico pantallas"[^>]*>[\s\S]*?<\/article>\s*<\/div>/)?.[0] ?? ''
    const tarjetas = pantallas.match(/<article class="tarjeta funcion[^"]*">[\s\S]*?<\/article>/g) ?? []
    assert.deepEqual(tarjetas.map((t) => t.match(/<span class="fig"><span lang="es">FIG \d+ · ([^<]+)<\/span>/)?.[1]), ['Inicio', 'Espacios', 'Probar', 'Mis APIs', 'Agentes'])
    assert.ok(new Set(tarjetas.map((t) => t.match(/class="tarjeta funcion([^"]*)"/)?.[1])).size > 1, 'de distinto tamaño')
    for (const t of ['El mapa en vivo de tu red', 'Un espacio por proyecto', 'De la prueba al código en un clic', 'Cada app con su clave y su presupuesto', 'Suscripción o clave de API, en cada agente'])
      assert.ok(pantallas.includes(`<h3><span lang="es">${t}</span>`), t)
    assert.match(tarjetas[0], /Cada petición viaja en vivo por la línea de su agente/)
    const imgs = pantallas.match(/<img [^>]*>/g) ?? []
    assert.equal(imgs.length, 20, 'cinco pantallas, en claro y oscuro y en los dos idiomas')
    for (const img of imgs) {
      const m = img.match(/class="solo-(claro|oscuro)" lang="(es|en)" src="capturas\/([a-z-]+)-\1-\2-1440\.webp"/)
      assert.ok(m, img.slice(0, 120))
      const [, tema, idioma, nombre] = m
      const base = `capturas/${nombre}-${tema}-${idioma}`
      assert.ok(img.includes(`srcset="${base}-640.webp 640w, ${base}-960.webp 960w, ${base}-1440.webp 1440w"`), `${base}: srcset`)
      assert.match(img, /width="1440" height="900"/)
      assert.match(img, /loading="lazy"/)
      const otro = idioma === 'es' ? 'en' : 'es'
      assert.ok(imgs.some((i) => i.includes(`class="solo-${tema}" lang="${otro}" src="capturas/${nombre}-${tema}-${otro}-1440.webp"`)), `${base}: falta su pareja`)
    }
    const nota = asi.match(/<p class="nota-capturas[\s\S]*?<\/p>/)?.[0] ?? ''
    assert.match(nota, /datos de ejemplo[\s\S]*17388[\s\S]*17373/)
  })

  test('el recorrido es una figura por paso de la bienvenida, con su captura en claro y oscuro y en los dos idiomas', () => {
    assert.equal(figuras.length, 14)
    assert.equal(figuras.length, pasos.length, 'una figura por paso')
    assert.match(figuras[13], /Historial y Ajustes tienen su recorrido en su \?\. Ábrelo cuando llegues allí\./)
    assert.match(figuras[13], /History and Settings have their own tour behind their \? button\. Open it when you get there\./)
    figuras.forEach((figura, i) => {
      const n = i + 1
      for (const tema of ['claro', 'oscuro'])
        for (const idioma of ['es', 'en']) {
          const base = `capturas/bienvenida-${n}-${tema}-${idioma}`
          assert.match(figura, new RegExp(`<img class="solo-${tema}" lang="${idioma}" src="${base}-1440\\.webp" srcset="${base}-960\\.webp 960w, ${base}-1440\\.webp 1440w" sizes="[^"]+" width="1440" height="900" loading="lazy" decoding="async" alt="[^"]+">`), `paso ${n}, ${tema}, ${idioma}`)
        }
      const escapar = (s: string) => s.slice(0, 30).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      assert.match(figura, new RegExp(`<figcaption>[\\s\\S]*${escapar(es[pasos[i].texto])}`))
      assert.match(figura, new RegExp(`<figcaption>[\\s\\S]*${escapar(en[pasos[i].texto])}`))
    })
    // La primera se ve; el resto espera su turno (sin JavaScript se ven las 14, una debajo de otra).
    assert.doesNotMatch(figuras[0], /^<figure[^>]* hidden/)
    for (const figura of figuras.slice(1)) assert.match(figura, /^<figure[^>]* hidden/)
    const botones = [...demo.matchAll(/<button type="button" class="demo-ir" data-paso="(\d+)"><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/button>/g)]
    assert.deepEqual(botones.map((m) => [m[2], m[3]]), pasos.map((p) => [es[p.titulo], en[p.titulo]]))
  })

  // Con el recorrido va su JavaScript: sin él, con html.js, solo se vería el primer paso.
  const demoJs = js.slice(js.indexOf('// Demo del recorrido guiado'))
  test('el recorrido funciona con su JavaScript y su CSS: flechas con el foco dentro, pausa y leyenda', () => {
    assert.ok(demoJs.length > 1000, 'falta el bloque del recorrido en sitio.js')
    assert.match(demoJs, /caja\.addEventListener\('keydown'/)
    assert.doesNotMatch(demoJs, /document\.addEventListener\('keydown'/)
    assert.match(demoJs, /if \(e\.altKey \|\| e\.ctrlKey \|\| e\.metaKey \|\| e\.shiftKey\) return/)
    assert.match(demoJs, /pausaManual = pausado/)
    assert.match(demoJs, /escribirLeyenda\(leyenda, /)
    assert.match(js, /function escribirLeyenda\(leyenda, partes\) \{[\s\S]*?setTimeout/)
    assert.match(demoJs, /siguiente\.loading = 'eager'/)
    assert.match(demo, /<p class="demo-leyenda" aria-live="off">/)
    assert.match(css, /html\.js \.demo \{[^}]*grid-template-columns: 220px minmax\(0, 1fr\)/)
    assert.match(css, /html\.js \.demo-paso\[hidden\] \{[^}]*visibility: hidden/)
    assert.match(css, /\.demo-flecha \{[^}]*width: 44px; height: 44px/)
    assert.match(css, /\.demo-pausa \{[^}]*min-height: 44px/)
    assert.match(css, /#demo \{[^}]*scroll-margin-top: 72px/)
  })
})

// ---------------------------------------------------------------------------------------------------------------------
// Agentes (v0.61, spec §5, Tarea 7): la tarjeta «router», que sustituye a la línea de metro (de Tramtai salen líneas de un
// píxel a los cinco agentes, cada uno con su color, lo que hace, cómo se conecta y un pulso), y una tarjeta por agente con
// cómo se instala e inicia sesión desde Tramtai, las versiones comprobadas y su enlace a «Preparar tus agentes».
const agentesPagina = leer('agentes.html')
const AGENTES = ['claude', 'codex', 'copilot', 'gemini', 'antigravity']
// Cómo se conecta cada uno, copiado de la sección «agentes» de la portada de antes de la v0.61
// (git show 2d00e09:site/index.html); se contrasta con Autenticaciones de internal/agentes/*/ficha.go.
const CONEXION: Record<string, [string, string]> = {
  claude: ['Tu suscripción de Claude (uso personal) o una clave de API de Anthropic.', 'Your Claude subscription (personal use) or an Anthropic API key.'],
  codex: ['Tu cuenta de ChatGPT (uso personal) o una clave de API de OpenAI.', 'Your ChatGPT account (personal use) or an OpenAI API key.'],
  copilot: ['Tu cuenta de GitHub Copilot (Free, Pro o de empresa) con <code>copilot login</code>, o una clave de GitHub. Cada petición gasta créditos de tu plan.',
    'Your GitHub Copilot account (Free, Pro or business) with <code>copilot login</code>, or a GitHub key. Every request uses credits from your plan.'],
  gemini: ['Tu clave de la Gemini API, sin instalar ningún CLI.', 'Your Gemini API key, with no CLI to install.'],
  antigravity: ['El CLI de Google con tu clave de la Gemini API, sin herramientas.', 'Google’s CLI with your Gemini API key, without tools.'],
}
const tarjetaAgente = (id: string) => agentesPagina.match(new RegExp(`<article class="tarjeta agente[^"]*" id="${id}">[\\s\\S]*?</article>`))?.[0] ?? ''

describe('agentes', () => {
  const router = seccion('router', agentesPagina)
  const figura = router.match(/<figure class="tarjeta ancha router vivo">[\s\S]*?<\/figure>/)?.[0] ?? ''

  test('la tarjeta router llega a los cinco agentes', () => {
    assert.ok(figura, 'falta la tarjeta router')
    assert.match(figura, /<figcaption class="fig"><span lang="es">FIG 1 · /)
    // El SVG: cinco líneas de un píxel, que se dibujan, desde Tramtai; el flujo corre por las cuatro que llegan a tus apps.
    // Solo dibuja: lo que dice cada destino es texto de la lista de al lado, que leen también los lectores de pantalla.
    const svg = figura.match(/<svg [\s\S]*?<\/svg>/)?.[0] ?? ''
    assert.match(svg, /<svg [^>]*aria-hidden="true">/)
    assert.match(svg, /stroke-width="1"/)
    assert.equal((svg.match(/<path class="dibuja" pathLength="1"/g) ?? []).length, 5, 'cinco destinos')
    assert.equal((svg.match(/<path class="flujo" stroke-dasharray="4 6"/g) ?? []).length, 4, 'sin JavaScript el flujo se ve punteado y quieto')
    assert.match(svg, />Tramtai</)
    // Cada destino, con su nombre visible, su punto de color, lo que hace y cómo se conecta, y enlace a su tarjeta.
    const destinos = figura.match(/<li[^>]*>[\s\S]*?<\/li>/g) ?? []
    assert.deepEqual(destinos.map((d) => d.match(/href="#([a-z]+)"/)?.[1]), AGENTES)
    assert.deepEqual(destinos.map((d) => d.match(/<span class="router-nombre">([^<]+)<\/span>/)?.[1]), ['Claude Code', 'Codex', 'GitHub Copilot', 'Gemini API', 'Antigravity'])
    destinos.forEach((d, i) => {
      assert.match(d, new RegExp(`<span class="punto[^"]*" style="--c: var\\(--${AGENTES[i]}\\)`), AGENTES[i])
      assert.match(d, /<span class="router-hace"><span lang="es">[^<]+<\/span><span lang="en">[^<]+<\/span><\/span>/, AGENTES[i])
    })
    const conecta = destinos.map((d) => d.match(/<span class="router-conexion"><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/span>/)?.slice(1))
    assert.deepEqual(conecta, [...Array(3).fill(['suscripción o clave', 'subscription or key']), ['clave de API', 'API key'], ['solo clave de API', 'API key only']])
    // Los cuatro que llegan a tus apps laten; Antigravity, no.
    assert.equal((figura.match(/class="punto pulso"/g) ?? []).length, 4)
    assert.match(destinos[4], /fuera de tus apps/)
    assert.doesNotMatch(figura, /style="[^"]*(opacity|display)|\shidden[\s>]/)
    // Las filas crecen con el texto (WCAG 1.4.4 y 1.4.10): cinco iguales y el SVG, de su alto; en un teléfono estrecho, fuera.
    assert.match(css, /\.router-destinos \{[^}]*grid-auto-rows: minmax\(64px, 1fr\)/)
    assert.doesNotMatch(css, /\.router-destinos li \{[^}]*[^-]height:/)
    assert.match(css, /\.router-lineas \{[^}]*height: 100%/)
    assert.match(css, /@media \(max-width: 420px\) \{[^@]*\.router-lineas \{ display: none; \}/)
  })

  test('cada agente dice cómo se conecta', () => {
    for (const id of AGENTES) {
      const t = tarjetaAgente(id)
      assert.ok(t, `falta la tarjeta de ${id}`)
      assert.ok(t.includes(`<span lang="es">${CONEXION[id][0]}`), `${id} (es)`)
      assert.ok(t.includes(`<span lang="en">${CONEXION[id][1]}`), `${id} (en)`)
    }
    // Antigravity y la Gemini API van solo con clave: ni una palabra de suscripción.
    for (const id of ['gemini', 'antigravity']) assert.doesNotMatch(tarjetaAgente(id), /suscripción|subscription/, id)
    assert.match(tarjetaAgente('antigravity'), /Todavía no desde tus apps/)
    // Cómo se instala y se inicia sesión desde Tramtai (v0.60, Ayuda «Preparar tus agentes»): los tres con suscripción
    // inician sesión desde la app; Antigravity solo se instala; la Gemini API no se instala.
    for (const [id, es, en] of [['claude', 'Con un enlace en tu navegador', 'With a link in your browser'], ['codex', 'Con un código de dispositivo', 'With a device code'],
      ['copilot', 'Con un código de dispositivo de GitHub', 'With a GitHub device code'], ['antigravity', 'No inicia sesión: solo se instala', 'It doesn’t sign in: it is only installed'],
      ['gemini', 'Nada que instalar', 'Nothing to install']])
      assert.ok(tarjetaAgente(id).includes(es) && tarjetaAgente(id).includes(en), id)
    for (const id of ['claude', 'codex', 'copilot', 'antigravity'])
      assert.match(tarjetaAgente(id), /versiones comprobadas[\s\S]*checked versions/, id)
    // En la app, con el texto de ayuda.preparar.paso1 y paso2, en los tres que inician sesión.
    for (const id of ['claude', 'codex', 'copilot'])
      assert.match(tarjetaAgente(id), /<dt><span lang="es">En la app<\/span>[\s\S]*En Agentes, abre «Otros agentes que puedes instalar» y pulsa «Instalar»\. Tramtai sigue con el inicio de sesión:/, id)
    // Antigravity y la Gemini API: «La clave», con la variable de su ficha (VariablesClave).
    for (const id of ['gemini', 'antigravity']) {
      assert.match(tarjetaAgente(id), /<dt><span lang="es">La clave<\/span>[\s\S]*<code>tramtai claves guardar GEMINI_API_KEY<\/code>[\s\S]*<code>tramtai keys set GEMINI_API_KEY<\/code>/, id)
      assert.doesNotMatch(tarjetaAgente(id), /Iniciar sesión<\/span>/, id)
    }
  })

  test('cada agente enlaza a Preparar tus agentes', () => {
    for (const id of AGENTES) assert.match(tarjetaAgente(id), /<p class="agente-pie"><a href="ayuda\/preparar\.html">/, id)
    // Un enlace suelto mide 44 px de alto.
    assert.match(css, /\.agente-pie a \{ display: inline-flex; align-items: center; min-height: 44px; \}/)
  })

  test('ya no hay línea de metro', () => {
    // La sección «agentes» de antes era una parada por agente: el aro de su sigla y la ficha de «vía muerta».
    assert.doesNotMatch(agentesPagina, /\bmetro\b|class="(sigla|fichas|conexiones|panel|agentes)"|sin-apps|línea de ese agente|that agent’s line/)
    assert.doesNotMatch(css, /\.(sigla|fichas|conexiones|panel|panel-titulo|agentes)(?![\w-])|\.agente\.sin-apps/)
    // El color de cada agente solo en sus puntos.
    for (const [etiqueta] of agentesPagina.matchAll(/<[^>]*var\(--(?:claude|codex|copilot|gemini|antigravity)\)[^>]*>/g))
      assert.match(etiqueta, /^<span class="punto[^"]*" style="--c: var\(--[a-z]+\)[^"]*">$/, etiqueta)
  })
})

// ---------------------------------------------------------------------------------------------------------------------
// Lo que pasó de la portada a Producto, Agentes y Descargar (ruling 3), hasta que las Tareas 5 a 7 rehagan esas páginas.

// v0.59 (spec §4): la franja de cuatro cifras del producto y la línea de siete paradas, en lugar de los seis motivos.
const porQue = seccion('por-que')
test('«Por qué Tramtai» tiene cuatro cifras del producto, en Inter tabular', () => {
  const cifras = porQue.match(/<li class="cifra">[\s\S]*?<\/li>/g) ?? []
  assert.equal(cifras.length, 4)
  assert.deepEqual(cifras.map((c) => c.match(/<span class="cifra-numero">(\d+)<\/span>/)?.[1]), ['4', '3', '0', '2'])
  for (const c of cifras) assert.match(c, /lang="es"[\s\S]*lang="en"/)
  for (const t of ['agentes desde tus apps', 'OpenAI, Anthropic y la propia', 'telemetría y ningún servidor nuestro', 'cambios en tu app: la URL y la clave'])
    assert.ok(porQue.includes(t), t)
  const regla = css.match(/\.cifra-numero \{[^}]*\}/)?.[0] ?? ''
  assert.match(regla, /var\(--fuente\)/)
  assert.match(regla, /font-variant-numeric: tabular-nums/)
})

test('los seis motivos ya no están, ni su CSS', () => {
  assert.doesNotMatch(html, /class="motivos?"/)
  assert.doesNotMatch(css, /\.motivo/)
})

// v0.59 (spec §1, §3): «Cómo funciona» enseña el código con pestañas y el paso 3 cuenta lo de las herramientas.
const como = seccion('como')
test('«Cómo funciona» tiene pestañas OpenAI, Anthropic y curl, con su panel cada una', () => {
  const listas = como.match(/<div class="pestanas-lista" role="tablist"[\s\S]*?<\/div>/g) ?? []
  assert.equal(listas.length, 1)
  const pestanas = listas[0].match(/<button type="button" role="tab"[^>]*>[^<]*<\/button>/g) ?? []
  assert.deepEqual(pestanas.map((p) => p.replace(/<[^>]+>/g, '')), ['OpenAI', 'Anthropic', 'curl'])
  for (const p of pestanas) {
    const panel = p.match(/aria-controls="([^"]+)"/)?.[1] ?? ''
    assert.match(como, new RegExp(`<pre class="codigo" id="${panel}" aria-labelledby="${p.match(/id="([^"]+)"/)?.[1]}"`), panel)
  }
  assert.match(listas[0], /role="tablist"[^>]* hidden>/)
  assert.match(css, /\.pestanas-lista\[hidden\] \{ display: none; \}/)
  assert.equal((como.match(/<pre class="codigo"[^>]* hidden>/g) ?? []).length, 2)
  assert.match(js, /lista\.hidden = false/)
  assert.match(js, /setAttribute\('role', 'tabpanel'\)/)
  // M6: al cargar no se guarda nada; solo la pestaña que elige la persona.
  assert.match(js, /if \(guardar && nombre !== 'formas' && nombre !== 'descargar'\) try \{ localStorage\.setItem\(nombre \? 'tramtai\.' \+ nombre : clave, lenguaje\)/)
  assert.match(js, /elegir\(b\.dataset\.lenguaje, null, true\)/)
  // Cada juego de pestañas (data-pestanas="nombre") cambia y se recuerda aparte: elegir Windows no toca OpenAI · curl.
  assert.match(js, /if \(\(g\.dataset\.pestanas \|\| ''\) !== nombre\) return;/)
  const pre = (id: string) => html.match(new RegExp(`<pre class="codigo" id="${id}"[^>]*>([\\s\\S]*?)</pre>`))?.[1] ?? ''
  assert.match(pre('como-openai-panel').replace(/<[^>]+>/g, ''), /from openai import OpenAI/)
  assert.match(pre('como-openai-panel'), /stream=<span class="k">True/)
  assert.match(pre('como-anthropic-panel'), /anthropic/)
  assert.match(pre('como-curl-panel'), /^curl /m)
})

test('el paso 3 de «Cómo funciona» dice que Tramtai apunta tus herramientas y lleva a su Ayuda', () => {
  const paso = como.match(/<li>[\s\S]*?<\/li>/g)?.[2] ?? ''
  for (const t of ['Claude Code', 'Codex', 'Continue', 'opencode', 'copia de su configuración', 'interruptor', 'Herramientas', 'Tools', 'switch'])
    assert.ok(paso.includes(t), t)
  assert.match(paso, /href="ayuda\/conectar-herramientas\.html"/)
})

test('Agentes, Privacidad y Descargar conservan lo que absorbieron', () => {
  assert.match(seccion('agentes'), /Desde tus apps y SDK[\s\S]*LangChain[\s\S]*n8n[\s\S]*API propia \(OpenAPI\)/)
  assert.match(seccion('privacidad'), /class="tablero"[\s\S]*<!-- comun:riesgos -->/)
  assert.match(leer('descargar.html'), /Gratis para uso personal y no comercial/)
  assert.doesNotMatch(html, /href="#(pagas|capturas|aviso)"/)
})

// «Por qué Tramtai» (v0.59) conserva el antes y el después de «Los agentes que ya pagas» (v0.56).
test('«Por qué Tramtai» dibuja el antes y el después, con texto visible', () => {
  const figuras = porQue.match(/<figure class="pagas-diagrama[^"]*">[\s\S]*?<\/figure>/g) ?? []
  assert.equal(figuras.length, 2)
  for (const f of figuras) { assert.match(f, /<svg[^>]*role="img"/); assert.match(f, /<figcaption>[\s\S]*lang="es"[\s\S]*lang="en"/) }
  assert.match(porQue, /Una clave, un SDK y una factura por proveedor/)
  assert.match(porQue, /Una URL local y los agentes que ya tienes/)
  assert.match(porQue, /One local URL and the agents you already have/)
  const diagramas = porQue.match(/<div class="pagas-diagramas">[\s\S]*<\/figure>/)?.[0] ?? ''
  for (const t of ['Sin Tramtai', 'Con Tramtai', 'Without Tramtai', 'With Tramtai']) assert.ok(diagramas.includes(t), `falta «${t}»`)
  const con = porQue.match(/<figure class="pagas-diagrama con">[\s\S]*?<\/figure>/)?.[0] ?? ''
  for (const n of ['Claude Code', 'GitHub Copilot', 'Gemini API']) assert.match(con, new RegExp(`>${n}</text>`), n)
  assert.doesNotMatch(html, /var\(--acento\) 55%/)
})
test('la suscripción, solo para tu uso personal, y sin promesas absolutas fuera de la línea (I2, M7)', () => {
  assert.match(porQue, /<span lang="en">Your API key or, only for your personal use, your Claude, ChatGPT or GitHub Copilot subscription\. No new accounts\.<\/span>/)
  assert.doesNotMatch(porQue, /la suscripción o la clave que ya tienes|the subscription or key you already have/)
  const sinLinea = porQue.replace(/<ol class="mosaico ventajas"[\s\S]*?<\/ol>/, '')
  assert.ok(sinLinea.length < porQue.length, 'falta la línea de ventajas')
  assert.doesNotMatch(sinLinea, /\b(nunca|siempre|sin cortes|a nadie|never|always|no one)\b/i)
  assert.doesNotMatch(porQue, /Sin servidores intermedios|No middle servers/)
  assert.doesNotMatch(html, /Tus automatizaciones|Your automations/)
})

const COLORES_LINEA = /var\(--(claude|codex|copilot|gemini|antigravity)\)/
test('el color de cada agente solo marca a su agente: figuras de Inicio, diagrama «Con Tramtai» y tarjetas de Agentes (52)', () => {
  assert.doesNotMatch(html, /class="linea"/)
  // En Agentes, el router y las tarjetas: sus puntos los vigila «ya no hay línea de metro».
  const sinAgentes = html.replace(/<section id="fig"[\s\S]*?<\/section>/, '').replace(seccion('router'), '').replace(seccion('agentes'), '').replace(/<figure class="pagas-diagrama con">[\s\S]*?<\/figure>/, '')
  assert.doesNotMatch(sinAgentes, COLORES_LINEA)
  assert.match(seccion('agentes'), /Cinco agentes; <em>cuatro desde tus apps\.<\/em>/)
})

test('las secciones no abren todas igual: sin etiqueta sobre el h2 y no todas centradas (35, 55)', () => {
  assert.doesNotMatch(html, /<div class="cabeza[^"]*">\s*<span class="etiqueta"/)
  const aperturas = [...html.matchAll(/<div class="cabeza( centrada)?">/g)]
  assert.ok(aperturas.filter((a) => !a[1]).length >= 3, 'al menos tres a la izquierda')
  assert.ok(aperturas.filter((a) => a[1]).length >= 1)
})

test('junto a Descargar se avisa de que hace falta un agente, que Tramtai instala, y del aviso de Windows (58)', () => {
  const descargar = seccion('descargar')
  assert.ok(descargar.includes(`<span lang="es">${AVISO_DESCARGA.es.split('. ')[0]}.</span>`))
  assert.ok(descargar.includes(`<span lang="en">${AVISO_DESCARGA.en.split('. ')[0]}.</span>`))
  assert.doesNotMatch(html, /Tramtai no los instala|doesn’t install them|Necesitas tener instalado el CLI|CLI must be installed/)
  assert.match(descargar, /Windows protegió su PC/)
  assert.match(descargar, /Windows protected your PC/)
})

test('los comentarios del código salen en el idioma de la página y Copiar copia solo ese (59)', () => {
  for (const pre of html.match(/<pre class="(codigo|bloque-codigo)"[\s\S]*?<\/pre>/g) ?? [])
    for (const c of pre.match(/<span class="c"[^>]*>/g) ?? []) assert.match(c, /lang="(es|en)"/, pre.slice(0, 60))
  assert.match(js, /\[lang\]:not\(\[lang="' \+ document\.documentElement\.lang \+ '"\]\)/)
})

test('la tarjeta del aislamiento no contradice a Codex y la nota de Agentes fecha el canario', () => {
  assert.doesNotMatch(html, /Agentes sin herramientas|Agents without tools/)
  const nota = html.match(/<p class="nota-agentes[\s\S]*?<\/p>/)?.[0] ?? ''
  assert.match(nota, /<span lang="es">[^<]*el 2026-10-18[^<]*<\/span>/)
  assert.match(nota, /<span lang="en">[^<]*on 2026-10-18[^<]*<\/span>/)
})

test('los cinco riesgos siguen en Privacidad y no se anuncia Docker', () => {
  assert.match(seccion('privacidad'), /<div class="cabeza riesgos-cabeza" id="riesgos">[\s\S]*<!-- comun:riesgos -->/)
  const aviso = seccion('privacidad').match(/<ol class="riesgos">[\s\S]*?<\/ol>/)?.[0] ?? ''
  assert.equal((aviso.match(/<li>/g) ?? []).length, 5, 'los cinco riesgos')
  // Desde Inicio, el servidor lleva a ellos antes de instalar.
  assert.ok(formas.includes('Es otro entorno: tus APIs y claves no pasan de uno a otro.'))
  assert.doesNotMatch(html, /docker/i)
})

test('Homebrew sin la app: el tap lleva la URL del repositorio y después «brew install tramtai» (el tap corto no existe)', () => {
  const tap = 'brew tap mscottb/tramtai https://github.com/MscottB/Tramtai'
  const linux = html.match(/<article class="tarjeta"[^>]* data-sistema="linux">[\s\S]*?<\/article>/)?.[0] ?? ''
  assert.ok(linux.includes(tap + '\nbrew install tramtai'), 'tarjeta de Linux')
  assert.ok(inicio.includes(tap + '\nbrew install --cask tramtai'), 'macOS en Inicio')
  for (const textos of [es, en]) {
    const r = textos['ayuda.empezar.rTerminal']
    assert.ok(r.includes(tap) && r.includes('brew install tramtai'), r)
  }
  assert.ok(![html, es['ayuda.empezar.rTerminal'], en['ayuda.empezar.rTerminal']].some((x) => x.includes('mscottb/tramtai/tramtai')))
})

// ---------------------------------------------------------------------------------------------------------------------
// Capturas y código

// v0.59: cada captura de la app y del servidor existe en español e inglés. Las del servidor van a 1440 y 960 px.
test('cada captura de la app y del servidor tiene su versión es y en en disco', () => {
  const archivos = readdirSync(new URL('../../site/capturas/', import.meta.url))
  const pedidas = [
    ...PORTADA.map((p) => ({ nombre: p.nombre, anchos: [1440, 960, 640] })),
    ...['servidor-entrar', 'servidor-inicio'].map((nombre) => ({ nombre, anchos: [1440, 960] })),
  ]
  for (const { nombre, anchos } of pedidas)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const ancho of anchos) assert.ok(archivos.includes(`${nombre}-${tema}-${idioma}-${ancho}.webp`), `falta ${nombre}-${tema}-${idioma}-${ancho}.webp`)
})

test('el sitio no habla de la «API principal»', () => {
  for (const pagina of ['index.html', 'producto.html', 'desarrolladores.html'])
    assert.doesNotMatch(leer(pagina), /API principal|main API/i, pagina)
})

// v0.56: cada captura del recorrido guiado es la zona que señala el paso, ampliada y sin el globo del recorrido (las
// saca capturas.mjs; las usa la Tarea 9).
const capturasJs = readFileSync(new URL('./capturas.mjs', import.meta.url), 'utf8')
test('las capturas del recorrido recortan la zona del paso y ocultan el globo', () => {
  assert.ok(/function zonaDelPaso\(/.test(capturasJs), 'falta zonaDelPaso()')
  assert.ok(capturasJs.includes("paso.ancla ? '.driver-popover'") && capturasJs.includes("visibility = 'hidden'"), 'el globo del recorrido no sale en la captura')
  assert.ok(capturasJs.includes('clip: { ...zona, scale: 1440 / zona.width }'), 'cada captura recorta la zona y la amplía al ancho')
  assert.match(capturasJs, /Math\.min\(maximo, Math\.max\(720, /)
  assert.match(capturasJs, /async function zonaDelPaso\(maximo = 820\)/)
  assert.match(capturasJs, /ENTERAS = new Set\(\['agentes\.tarjeta', 'espacios\.lista', 'inicio\.mapa'\]\)/)
  assert.ok(capturasJs.includes('paso.ancla ? await zonaDelPaso(ENTERAS.has(paso.ancla) ? 1440 : 820) : { x: 0, y: 0, width: 1440, height: 900 }'), 'el último paso recorta una zona')
})

test('las capturas del recorrido existen y pesan poco: 150 KB cada una de 1440 y 6 MB en total', () => {
  const carpeta = new URL('../../site/capturas/', import.meta.url)
  const archivos = readdirSync(carpeta).filter((a) => a.startsWith('bienvenida-'))
  let total = 0
  for (let n = 1; n <= pasos.length; n++)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const ancho of [960, 1440]) {
          const nombre = `bienvenida-${n}-${tema}-${idioma}-${ancho}.webp`
          assert.ok(archivos.includes(nombre), nombre)
          const peso = statSync(new URL(nombre, carpeta)).size
          total += peso
          if (ancho === 1440) assert.ok(peso <= 150 * 1024, `${nombre} pesa ${peso} bytes`)
        }
  assert.equal(archivos.length, pasos.length * 8, 'sobran capturas del recorrido')
  assert.ok(total <= 6 * 1024 * 1024, `pesan ${total} bytes`)
})

// Las órdenes y los ejemplos de código siguen el idioma elegido: lo que se ve (y se copia) en inglés no trae texto en
// español conocido. Los campos de la API propia (mensajes, rol, contenido…) son su contrato y se quedan.
test('los bloques de código en inglés no traen texto en español', () => {
  const servidor = leer('servidor.html')
  const espanol = /\b(?:cliente|respuesta|flujo|trozo|tu-clave|Resume|Traduce|Explica|Saca las|acta\.pdf|instalar|administrador|iniciar)\b|contraseña/
  const bloques = [...html.matchAll(/<pre class="(?:codigo|bloque-codigo|terminal-viva)"[^>]*>[\s\S]*?<\/pre>/g), ...servidor.matchAll(/<pre class="codigo"[^>]*>[\s\S]*?<\/pre>/g)].map((m) => m[0])
  assert.ok(bloques.length >= 12, `${bloques.length} bloques`)
  for (const b of bloques) {
    const ingles = b.replace(/<span[^>]*\blang="es"[^>]*>[^<]*<\/span>/g, '').replace(/<[^>]+>/g, '')
    assert.doesNotMatch(ingles, espanol, ingles.slice(0, 120))
  }
})

// La guía del desarrollador en inglés enseña (y copia) los ejemplos de docs/ejemplos/*/en: cada panel lleva un
// <code lang="es"> y uno lang="en" con su archivo, los dos existen, y ni el archivo inglés ni lo demás que se ve en
// inglés (la terminal, los curl de una línea) trae español. Los campos y las rutas de la API propia (mensajes, rol,
// contenido, argumentos, archivo…) son su contrato y se quedan.
test('desarrolladores.html en inglés enseña los ejemplos ingleses', () => {
  const dev = leer('desarrolladores.html')
  const espanol = /\b(?:cliente|respuesta|flujo|trozo|CLAVE|MODELO|TRAMTAI_CLAVE|TRAMTAI_MODELO|clima|ciudad|Hola|despejado|agente pide|siguió|muestra|tu-usuario|tu-equipo|servicio|iniciar|ruta|datos|esquema|cabeceras|llamada|ronda|terminado|resumen|entorno|predeterminado|pregunta)\b|¿|quién/
  const bloques = [...dev.matchAll(/<pre class="codigo"[^>]*>[\s\S]*?<\/pre>/g)].map((m) => m[0])
  let ejemplos = 0
  for (const b of bloques) {
    const rutas = (lang: string) => [...b.matchAll(new RegExp(`<code lang="${lang}" data-ejemplo="([^"]+)"`, 'g'))].map((m) => m[1])
    const [rEs, rEn] = [rutas('es'), rutas('en')]
    if (b.includes('data-ejemplo')) {
      assert.equal(rEs.length + rEn.length, 2, `cada panel, un ejemplo por idioma: ${b.slice(0, 160)}`)
      assert.ok(rEn.length === 1 && rEn[0].split('/')[1] === 'en' && !rEs[0].includes('/en/'), `${rEs} / ${rEn}`)
      for (const r of [rEs[0], rEn[0]]) statSync(new URL(`../../docs/ejemplos/${r}`, import.meta.url))
      const codigo = readFileSync(new URL(`../../docs/ejemplos/${rEn[0]}`, import.meta.url), 'utf8')
      assert.doesNotMatch(codigo, espanol, rEn[0])
      ejemplos++
    }
    const ingles = b.replace(/<(code|span)[^>]*\blang="es"[^>]*>[\s\S]*?<\/\1>/g, '').replace(/<[^>]+>/g, '')
    assert.doesNotMatch(ingles, espanol, ingles.slice(0, 120))
  }
  assert.equal(ejemplos, 21)
})
