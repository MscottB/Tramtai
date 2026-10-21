import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { ANCLAS_VIEJAS, PAGINAS_FIJAS, URL_SITIO, destinoAncla, escapar, generar, rellenar, type Fuente } from './sitio.ts'

const fuente: Fuente = {
  temas, es, en, capturas: new Map([['inicio.claro.es.webp', { ancho: 1280, alto: 720 }]]),
  ejemplos: (_b, idioma) => [{ id: 'python', titulo: 'Python', codigo: idioma === 'es' ? 'print("<hola> & adiós")' : 'print("hi")' }],
}
const paginas = generar(fuente)

test('escapar deja el texto sin HTML', () => {
  assert.equal(escapar(`<a & "b" 'c'>`), '&lt;a &amp; &quot;b&quot; &#39;c&#39;&gt;')
})

test('genera el inicio, una página por tema, el índice de búsqueda y la página de paso', () => {
  assert.ok(paginas.has('ayuda/index.html'))
  for (const t of temas) assert.ok(paginas.has(`ayuda/${t.id}.html`), t.id)
  assert.ok(paginas.has('ayuda/indice.json'))
  assert.ok(paginas.has('ayuda.html'))
})

test('el código de los ejemplos sale escapado', () => {
  const html = paginas.get('ayuda/conectar.html')!
  assert.ok(html.includes('print(&quot;&lt;hola&gt; &amp; adiós&quot;)'))
  assert.ok(!html.includes('<hola>'))
})

test('un texto con <script> no queda sin escapar', () => {
  const malo: Fuente = { ...fuente, es: { ...es, 'ayuda.empezar.nota': '<script>alert(1)</script>' } }
  assert.ok(!generar(malo).get('ayuda/empezar.html')!.includes('<script>alert'))
})

test('cada enlace interno de la Ayuda lleva a una página que existe', () => {
  for (const [ruta, html] of paginas) {
    if (!ruta.endsWith('.html')) continue
    for (const [, href] of html.matchAll(/href="([^"#]*)(?:#[^"]*)?"/g)) {
      if (!href || /^(https?:|mailto:)/.test(href)) continue
      const base = ruta.includes('/') ? 'ayuda/' : ''
      const destino = new URL(href, 'http://x/' + base).pathname.slice(1)
      const archivo = destino.endsWith('/') ? destino + 'index.html' : destino
      assert.ok(paginas.has(archivo) || existsSync(new URL('../../site/' + archivo, import.meta.url)), `${ruta}: ${href}`)
    }
  }
})

test('cada ancla vieja de ayuda.html lleva a un tema que existe', () => {
  for (const destino of Object.values(ANCLAS_VIEJAS)) assert.ok(paginas.has('ayuda/' + destino.split('#')[0]), destino)
  assert.equal(destinoAncla('#terminos'), 'ayuda/agentes.html#terminos')
  assert.equal(destinoAncla('#noexiste'), 'ayuda/')
  assert.equal(destinoAncla(''), 'ayuda/')
  assert.equal(destinoAncla('#constructor'), 'ayuda/')
  assert.equal(destinoAncla('#__proto__'), 'ayuda/')
})

test('el script de la página de paso funciona tal cual sale en ayuda.html', () => {
  const script = paginas.get('ayuda.html')!.match(/<script>([\s\S]*?)<\/script>/)![1]
  for (const [hash, destino] of [['#terminos', 'ayuda/agentes.html#terminos'], ['#nada', 'ayuda/'], ['', 'ayuda/']]) {
    let a = ''
    new Function('location', script)({ hash, replace: (url: string) => { a = url } })
    assert.equal(a, destino, hash)
  }
})

test('el índice de búsqueda tiene los 16 temas en los dos idiomas', () => {
  const indice = JSON.parse(paginas.get('ayuda/indice.json')!)
  for (const idioma of ['es', 'en']) assert.equal(new Set(indice[idioma].map((e: { t: string }) => e.t)).size, 16)
})

const leer = (archivo: string) => readFileSync(new URL('../../site/' + archivo, import.meta.url), 'utf8')
const fijas = new Map(PAGINAS_FIJAS.map((p) => [p.archivo, leer(p.archivo)]))
const todas = new Map([...fijas, ...[...paginas].filter(([r]) => r.endsWith('.html') && r !== 'ayuda.html')])

test('la portada y la guía tienen al día la cabeza, la barra y el pie de sitio.ts', () => {
  for (const p of PAGINAS_FIJAS) assert.equal(rellenar(fijas.get(p.archivo)!, p), fijas.get(p.archivo), `${p.archivo}: cd web; npm run sitio`)
})

test('la barra y el pie llevan a lo mismo en todas las páginas', () => {
  const destinos = (html: string) => {
    const zona = (html.match(/<header class="barra">[\s\S]*?<\/header>/)?.[0] ?? '') + (html.match(/<footer>[\s\S]*?<\/footer>/)?.[0] ?? '')
    return [...zona.matchAll(/<a [^>]*href="([^"]+)"/g)].map((m) => m[1].replace(URL_SITIO, '').replace(/^(\.\.\/|\.\/)+/, ''))
  }
  const esperados = ['#descargar', 'ayuda/empezar.html', '#agentes', 'ayuda/', 'desarrolladores.html', 'https://github.com/MscottB/Tramtai',
    'https://github.com/MscottB/Tramtai/releases', 'https://github.com/MscottB/Tramtai/blob/main/LICENSE.md', 'https://github.com/MscottB/Tramtai/blob/main/TERMINOS.md']
  const casos: [string, string][] = [...fijas, ['ayuda/agentes.html', paginas.get('ayuda/agentes.html')!], ['404.html', paginas.get('404.html')!]]
  for (const [ruta, html] of casos) {
    // Desde la Ayuda, «ayuda/» es «./» o «index.html».
    const d = destinos(html).map((x) => (ruta.startsWith('ayuda/') && !x.startsWith('http') && !x.includes('#') && !x.includes('.html') && !x.startsWith('ayuda/') ? 'ayuda/' + x : x))
      .map((x) => (ruta.startsWith('ayuda/') && /^(index\.html|empezar\.html)$/.test(x) ? 'ayuda/' + x.replace('index.html', '') : x))
    for (const e of esperados) assert.ok(d.includes(e), `${ruta}: falta ${e} (hay ${d.join(', ')})`)
  }
})

// Lighthouse (SEO, «link-text») marca como genérico un enlace que solo dice «Empezar»: el pie dice a dónde lleva.
test('el enlace del pie a empezar.html dice «Primeros pasos» / «Getting started» (como la tarjeta de la app), no un «Empezar» suelto', () => {
  for (const [ruta, html] of todas) {
    const pie = html.match(/<footer>[\s\S]*?<\/footer>/)?.[0] ?? ''
    const enlace = pie.match(/<a href="[^"]*ayuda\/empezar\.html">([\s\S]*?)<\/a>/)?.[1] ?? ''
    assert.equal(enlace, `<span lang="es">${es['inicio.primerosPasos']}</span><span lang="en">${en['inicio.primerosPasos']}</span>`, ruta)
  }
})

test('cada página tiene «Saltar al contenido», canonical y Open Graph absolutos', () => {
  for (const [ruta, html] of todas) {
    if (ruta === '404.html') continue
    assert.match(html, /<a class="saltar" href="#contenido">/, ruta)
    assert.match(html, /<main id="contenido"/, ruta)
    const url = URL_SITIO + (ruta === 'index.html' ? '' : ruta.replace(/index\.html$/, ''))
    assert.ok(html.includes(`<link rel="canonical" href="${url}">`), `${ruta}: canonical ${url}`)
    assert.ok(html.includes(`<meta property="og:url" content="${url}">`), `${ruta}: og:url`)
    assert.ok(html.includes(`<meta property="og:image" content="${URL_SITIO}og.png">`), `${ruta}: og:image`)
    for (const m of ['og:type', 'og:site_name', 'og:locale', 'og:title', 'og:description'])
      assert.match(html, new RegExp(`property="${m}" content="[^"]+"`), `${ruta}: ${m}`)
    assert.match(html, /<meta name="twitter:card" content="summary_large_image">/, ruta)
  }
})

test('el idioma guardado se aplica en el <head>, antes de pintar', () => {
  for (const [ruta, html] of todas) {
    const cabeza = html.slice(0, html.indexOf('</head>'))
    assert.match(cabeza, /localStorage\.getItem\('tramtai\.idioma'\)/, ruta)
    assert.match(cabeza, /<script src="[^"]*sitio\.js" defer><\/script>/, ruta)
    assert.doesNotMatch(html.slice(html.indexOf('</head>')), /<script src="[^"]*sitio\.js"/, `${ruta}: sitio.js va en el <head> con defer`)
  }
})

test('las etiquetas para lectores de pantalla van en el idioma de la página y los títulos son reales', () => {
  for (const [ruta, html] of todas) {
    assert.doesNotMatch(html, /(aria-label|placeholder|title)="[^"]* \/ [^"]*"/, `${ruta}: un atributo con los dos idiomas a la vez`)
    assert.doesNotMatch(html, /role="heading"/, ruta)
    assert.doesNotMatch(html, /[◐☀☾]/, `${ruta}: los iconos de tema van en SVG`)
  }
})

test('sitemap.xml lista la portada, la guía, la Ayuda y sus 16 temas', () => {
  const urls = [...paginas.get('sitemap.xml')!.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1])
  for (const u of [URL_SITIO, URL_SITIO + 'desarrolladores.html', URL_SITIO + 'ayuda/', ...temas.map((t) => `${URL_SITIO}ayuda/${t.id}.html`)])
    assert.ok(urls.includes(u), u)
  assert.equal(urls.length, 3 + 16)
})

test('404.html lleva a la portada, la Ayuda y Descargar con direcciones absolutas', () => {
  const html = paginas.get('404.html')!
  assert.match(html, /<meta name="robots" content="noindex">/)
  for (const u of [URL_SITIO, URL_SITIO + 'ayuda/', URL_SITIO + '#descargar']) assert.ok(html.includes(`href="${u}"`), u)
  assert.doesNotMatch(html, /(href|src)="(?!https?:|#)/, 'en una ruta cualquiera, lo relativo no carga')
})

test('las tablas no pierden su semántica y las capturas de la Ayuda declaran su tamaño', () => {
  for (const [, selector, cuerpo] of leer('estilo.css').matchAll(/([^{}]+)\{([^}]*)\}/g))
    if (/\btable\b/.test(selector)) assert.doesNotMatch(cuerpo, /display:\s*block/, selector.trim())
  for (const [ruta, html] of todas) {
    assert.equal((html.match(/<table/g) ?? []).length, (html.match(/<div class="tabla"><table/g) ?? []).length, `${ruta}: tabla sin su contenedor`)
    for (const img of html.match(/<img [^>]*>/g) ?? []) assert.match(img, /width="\d+" height="\d+"/, `${ruta}: ${img.slice(0, 80)}`)
  }
  assert.ok([...paginas.values()].some((h) => h.includes('src="capturas/inicio.claro.es.webp" width="1280" height="720"')))
})

const medida = { ancho: 640, alto: 300 }
const conRecorrido: Fuente = { ...fuente, capturas: new Map([['recorrido-inicio-mapa.claro.es.webp', medida], ['recorrido-inicio-mapa.oscuro.en.webp', medida]]) }

test('cada tema de página enseña «Qué verás en pantalla»: un paso por cada paso del recorrido y «En esta página» lo cita', () => {
  const html = generar(conRecorrido).get('ayuda/empezar.html')!
  const seccion = html.match(/<section id="s-recorrido">([\s\S]*?)<\/section>/)?.[1] ?? ''
  assert.match(seccion, /<h2><span lang="es">Qué verás en pantalla<\/span><span lang="en">What you will see on screen<\/span><\/h2>/)
  const pasos = temas.find((t) => t.id === 'empezar')!.bloques.flatMap((b) => (b.tipo === 'recorrido' ? b.pasos : []))
  const lista = seccion.match(/<ol class="pasos recorrido-pasos">([\s\S]*?)<\/ol>/)?.[1] ?? ''
  assert.equal((lista.match(/<li>/g) ?? []).length, pasos.length)
  for (const p of pasos) assert.ok(lista.includes(`<strong><span lang="es">${escapar(es[p.titulo])}</span>`), p.titulo)
  assert.match(html, /<nav class="indice"[\s\S]*?href="#s-recorrido"[\s\S]*?<\/nav>/)
  for (const t of temas.filter((x) => x.paginaApp)) assert.match(paginas.get(`ayuda/${t.id}.html`)!, /<section id="s-recorrido">/, t.id)
})

test('la captura de un paso lleva su tamaño, carga diferida y un texto alternativo que no repite el título a secas; sin captura, sin <img>', () => {
  const html = generar(conRecorrido).get('ayuda/empezar.html')!
  const imgs = html.match(/<img [^>]*recorrido-[^>]*>/g) ?? []
  assert.deepEqual(imgs, [
    '<img class="solo-claro" lang="es" src="capturas/recorrido-inicio-mapa.claro.es.webp" width="640" height="300" loading="lazy" alt="Captura de pantalla: El mapa en vivo" style="max-width: 640px">',
    '<img class="solo-oscuro" lang="en" src="capturas/recorrido-inicio-mapa.oscuro.en.webp" width="640" height="300" loading="lazy" alt="Screenshot: The live map" style="max-width: 640px">',
  ])
  assert.ok(!paginas.get('ayuda/empezar.html')!.includes('recorrido-inicio-mapa'), 'sin capturas, el paso sale sin <img>')
})

test('en el sitio no hay botón «Muéstrame en pantalla»: no hay app', () => {
  for (const [ruta, html] of paginas) assert.doesNotMatch(html, /Muéstrame en pantalla|Show me on screen/, ruta)
})

test('los temas largos tienen «En esta página» con un h2 por sección y el buscador a mano', () => {
  const html = paginas.get('ayuda/agentes.html')!
  const nav = html.match(/<nav class="indice"[^>]*>([\s\S]*?)<\/nav>/)?.[1] ?? ''
  const anclas = [...nav.matchAll(/href="#([^"]+)"/g)].map((m) => m[1])
  assert.ok(anclas.length >= 3, 'faltan secciones en el índice')
  for (const a of anclas) assert.match(html, new RegExp(`<section id="${a}"><h2>`), a)
  assert.match(html, /<a href="index\.html#buscar-ayuda">/)
})

test('iconos, imagen para compartir, manifest y datos estructurados', () => {
  for (const a of ['og.png', 'favicon.svg', 'favicon-32.png', 'apple-touch-icon.png'])
    assert.ok(existsSync(new URL('../../site/' + a, import.meta.url)), a)
  assert.equal(JSON.parse(leer('manifest.webmanifest')).name, 'Tramtai')
  const ld = JSON.parse(fijas.get('index.html')!.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/)![1])
  assert.equal(ld['@type'], 'SoftwareApplication')
  assert.match(leer('sitio.js'), /'tramtai\.lenguaje-ayuda'/, 'la Ayuda recuerda su lenguaje aparte de la guía')
})
