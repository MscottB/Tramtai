import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { bloquesVisibles } from '../src/ayuda/vista.ts'
import { ANCLAS_VIEJAS, GRUPOS_AYUDA, PAGINAS_FIJAS, URL_SITIO, destinoAncla, escapar, generar, rellenar, rellenarMarkdown, riesgos, type Fuente } from './sitio.ts'

const SITIO = (archivo: string) => readFileSync(new URL('../../site/' + archivo, import.meta.url), 'utf8')
const fuente: Fuente = {
  paginas: { 'servidor.html': SITIO('servidor.html'), 'desarrolladores.html': SITIO('desarrolladores.html') },
  temas, es, en, capturas: new Map([['inicio.claro.es.webp', { ancho: 1280, alto: 720 }]]),
  ejemplos: (_b, idioma) => [{ id: 'python', titulo: 'Python', codigo: idioma === 'es' ? 'print("<hola> & adiós")' : 'print("hi")' }],
}
const paginas = generar(fuente)

test('escapar deja el texto sin HTML', () => {
  assert.equal(escapar(`<a & "b" 'c'>`), '&lt;a &amp; &quot;b&quot; &#39;c&#39;&gt;')
})

test('genera el inicio, una página por tema, el buscador y la página de paso', () => {
  assert.ok(paginas.has('ayuda/index.html'))
  for (const t of temas) assert.ok(paginas.has(`ayuda/${t.id}.html`), t.id)
  assert.ok(paginas.has('buscar.json'))
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

test('buscar.json tiene una entrada por tema y apartado', () => {
  const indice = JSON.parse(paginas.get('buscar.json')!)
  for (const idioma of ['es', 'en']) {
    const us: string[] = indice[idioma].map((e: { u: string }) => e.u)
    for (const t of temas) assert.ok(us.includes('ayuda/' + t.id + '.html'), t.id)
    assert.ok(us.some((u) => u.startsWith('ayuda/agentes.html#')), 'anclas de un tema')
    for (const p of ['servidor.html#', 'desarrolladores.html#']) assert.ok(us.some((u) => u.startsWith(p)), p)
    for (const e of indice[idioma]) assert.deepEqual(Object.keys(e), ['u', 'ti', 'd', 'x'])
  }
  const [s] = indice.es.filter((e: { u: string }) => e.u === 'servidor.html#red')
  assert.equal(s.ti, 'Abrir a la red')
  assert.match(s.x, /servidor solo atiende/, 'lleva el párrafo, no el título')
  assert.equal(indice.en.find((e: { u: string }) => e.u === 'servidor.html#red').ti, 'Open to the network')
})

test('buscar.json no pasa de 100 KB', () => {
  assert.ok(Buffer.byteLength(paginas.get('buscar.json')!) <= 100 * 1024)
})

test('cada u de buscar.json lleva a un ancla que existe', () => {
  const indice = JSON.parse(paginas.get('buscar.json')!)
  for (const idioma of ['es', 'en'])
    for (const { u } of indice[idioma] as { u: string }[]) {
      const [archivo, ancla] = u.split('#')
      const html = paginas.get(archivo) ?? SITIO(archivo)
      if (ancla) assert.match(html, new RegExp('\\sid="' + ancla + '"'), u)
    }
})

const leer = (archivo: string) => readFileSync(new URL('../../site/' + archivo, import.meta.url), 'utf8')
const fijas = new Map(PAGINAS_FIJAS.map((p) => [p.archivo, leer(p.archivo)]))
const todas = new Map([...fijas, ...[...paginas].filter(([r]) => r.endsWith('.html') && r !== 'ayuda.html')])

// Un #ancla interno lleva a un id que existe en su página: así no se rompe sin avisar el de la portada a
// ayuda/empezar.html#s-soloterminal, que sale de la clave de la sección (sitio.ts).
test('cada #ancla interno de la portada, la guía y la Ayuda lleva a un id que existe', () => {
  for (const [ruta, html] of todas)
    for (const [, href, ancla] of html.matchAll(/href="([^"#]*)#([^"]+)"/g)) {
      if (/^(https?:|mailto:)/.test(href)) continue
      const base = ruta.includes('/') ? 'ayuda/' : ''
      const destino = href ? new URL(href, 'http://x/' + base).pathname.slice(1) : ruta
      const pagina = todas.get(!destino || destino.endsWith('/') ? destino + 'index.html' : destino)
      assert.ok(pagina, `${ruta}: ${href}#${ancla} lleva a una página que no se comprueba`)
      assert.ok(pagina.includes(`id="${ancla}"`), `${ruta}: falta id="${ancla}" en ${destino || ruta}`)
    }
  assert.match(todas.get('ayuda/empezar.html')!, /<section id="s-soloterminal"><h2><span lang="es">Solo con la terminal<\/span><span lang="en">Terminal only<\/span><\/h2>/)
})

test('la portada y las guías tienen al día la cabeza, la barra, el pie y el aviso de sitio.ts', () => {
  assert.deepEqual(PAGINAS_FIJAS.map((p) => p.archivo), ['index.html', 'producto.html', 'agentes.html', 'servidor.html', 'desarrolladores.html', 'descargar.html'])
  const terminal = JSON.parse(leer('capturas/terminal.json'))
  for (const p of PAGINAS_FIJAS) assert.equal(rellenar(fijas.get(p.archivo)!, p, { es, en, terminal }), fijas.get(p.archivo), `${p.archivo}: cd web; npm run sitio`)
})

// Los cinco riesgos (spec §10) son un solo texto, riesgos.* de los catálogos: el sitio, el README y la guía del
// servidor los toman de ahí al generar, nunca copiados a mano. Sin el modo servidor, el 3 y el 4 llevan su variante.
test('riesgos da los cinco riesgos de los catálogos, con la variante del escritorio en el 3 y el 4', () => {
  assert.deepEqual(riesgos(es, true), [1, 2, 3, 4, 5].map((n) => es[`riesgos.${n}` as keyof typeof es]))
  assert.deepEqual(riesgos(en, false), [en['riesgos.1'], en['riesgos.2'], en['riesgos.3.sinServidor'], en['riesgos.4.sinServidor'], en['riesgos.5']])
})

test('el aviso de la portada y de la guía del servidor salen de riesgos.*, en los dos idiomas', () => {
  for (const [archivo, servidor] of [['producto.html', false], ['servidor.html', true]] as const) {
    const html = fijas.get(archivo)!
    const zona = html.match(new RegExp(`<!-- comun:${servidor ? 'riesgos-servidor' : 'riesgos'} -->([\\s\\S]*?)<!-- /comun:`))?.[1] ?? ''
    for (const [i, r] of riesgos(es, servidor).entries()) assert.ok(zona.includes(`<span lang="es">${escapar(r)}</span>`), `${archivo}: riesgo ${i + 1} en español`)
    for (const [i, r] of riesgos(en, servidor).entries()) assert.ok(zona.includes(`<span lang="en">${escapar(r)}</span>`), `${archivo}: riesgo ${i + 1} en inglés`)
    assert.match(html, /href="https:\/\/github\.com\/MscottB\/Tramtai\/blob\/main\/TERMINOS\.md"/, `${archivo}: el aviso lleva a los términos`)
  }
})

// v0.59 (spec §6): servidor.html abre presentando el modo servidor (titular, entrada, el mapa, «¿Es para ti?», tres pasos
// por sistema y dos capturas reales), y debajo sigue la documentación; desde la v0.61, todo en la columna del contenido.
const servidor = fijas.get('servidor.html')!
const cabeceraServidor = servidor.slice(servidor.indexOf('<article id="contenido"'), servidor.indexOf('<section id="que-es">'))
test('servidor.html: la cabecera nueva tiene titular, entrada, mapa, «¿Es para ti?» y las capturas en es/en', () => {
  assert.match(cabeceraServidor, /^<article id="contenido"[^>]*>\s*<header class="doc-cabeza">[\s\S]*?<h1 class="titular">/)
  const entrada = cabeceraServidor.match(/<p class="entrada"><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/p>/)
  assert.ok(entrada, 'falta la entrada')
  for (const t of entrada.slice(1)) assert.ok(Math.abs(t.split(/\s+/).length - 30) <= 8, `unas 30 palabras: ${t}`)
  const mapa = cabeceraServidor.match(/<figure class="tarjeta fig-mapa vivo">[\s\S]*?<\/figure>/)?.[0] ?? ''
  assert.equal((mapa.match(/<svg [^>]*role="img" aria-label="[^"]+" data-aria-en="[^"]+"/g) ?? []).length, 2)
  assert.match(cabeceraServidor, /<section id="para-ti">[\s\S]*<article class="tarjeta">[\s\S]*El escritorio, si…[\s\S]*<article class="tarjeta">[\s\S]*El servidor, si…/)
  // Dos pantallas reales, en claro y oscuro y en los dos idiomas, cada una con su alt en su idioma.
  const imgs = cabeceraServidor.match(/<img [^>]*>/g) ?? []
  assert.equal(imgs.length, 8)
  for (const img of imgs) {
    const m = img.match(/^<img class="solo-(claro|oscuro)" lang="(es|en)" src="capturas\/(servidor-(?:entrar|inicio))-\1-\2-1440\.webp"/)
    assert.ok(m, img.slice(0, 120))
    const base = `capturas/${m[3]}-${m[1]}-${m[2]}`
    assert.ok(img.includes(`srcset="${base}-960.webp 960w, ${base}-1440.webp 1440w"`), base)
    assert.match(img, /width="1440" height="900" loading="lazy" decoding="async" alt="[^"]+">$/)
    assert.doesNotMatch(img, /data-alt-en/)
  }
  for (const [nombre, alt] of [['entrar', 'contraseña'], ['inicio', 'Servidor']]) assert.match(cabeceraServidor, new RegExp(`servidor-${nombre}-claro-es-1440\\.webp"[^>]*alt="[^"]*${alt}`), nombre)
})

test('servidor.html: tres pasos por sistema (instalar, contraseña, HTTPS o proxy) con las órdenes en su idioma', () => {
  const pasos = cabeceraServidor.match(/<section id="pasos">[\s\S]*?<\/section>/)?.[0] ?? ''
  const lista = pasos.match(/<div class="pestanas-lista" role="tablist"[^>]*>[\s\S]*?<\/div>/)?.[0] ?? ''
  assert.match(lista, / hidden>/)
  assert.deepEqual((lista.match(/<button type="button" role="tab"[^>]*>[^<]*<\/button>/g) ?? []).map((p) => p.replace(/<[^>]+>/g, '')), ['Linux', 'Windows Server'])
  assert.match(pasos, /data-pestanas="sistema"/)
  const paneles = pasos.match(/<div class="pasos-panel" id="[^"]+" aria-labelledby="[^"]+">[\s\S]*?<\/ol>\s*<\/div>/g) ?? []
  assert.equal(paneles.length, 2)
  for (const [i, panel] of paneles.entries()) {
    assert.equal((panel.match(/<li>/g) ?? []).length, 3, `panel ${i}`)
    const instalar = i === 0 ? ['sudo ./tramtai servidor instalar', 'sudo ./tramtai server install'] : ['.\\tramtai.exe servidor instalar', '.\\tramtai.exe server install']
    assert.ok(panel.includes(`<span lang="es">${instalar[0]}</span><span lang="en">${instalar[1]}</span>`), instalar[1])
    assert.match(panel, /tramtai-servidor administrador contraseña<\/span><span lang="en">(sudo )?tramtai-servidor admin password<\/span>/)
    assert.match(panel, /href="#red"/)
  }
  // Los pasos son instrucciones de instalación: antes, el enlace al aviso de los cinco riesgos.
  assert.ok(pasos.indexOf('href="#aviso"') > 0 && pasos.indexOf('href="#aviso"') < pasos.indexOf('<ol'), 'el aviso antes de los pasos')
})

test('servidor.html: el aviso de los cinco riesgos va justo antes de Instalar, también en «En esta página»', () => {
  const ids = [...servidor.matchAll(/<section id="([^"]+)"/g)].map((m) => m[1])
  assert.equal(ids[ids.indexOf('instalar') - 1], 'aviso')
  const indice = [...(servidor.match(/<aside class="doc-pagina">[\s\S]*?<\/aside>/)?.[0] ?? '').matchAll(/href="#([^"]+)"/g)].map((m) => m[1])
  assert.deepEqual(indice, ids.filter((id) => indice.includes(id)), 'el índice sigue el orden de la página')
  assert.equal(indice[indice.indexOf('instalar') - 1], 'aviso')
  const aviso = servidor.match(/<section id="aviso">[\s\S]*?<\/section>/)?.[0] ?? ''
  assert.match(aviso, /<!-- comun:riesgos-servidor -->[\s\S]*<!-- \/comun:riesgos-servidor -->[\s\S]*href="https:\/\/github\.com\/MscottB\/Tramtai\/blob\/main\/TERMINOS\.md"/)
})

test('servidor.html: ni Docker en la cabecera ni órdenes en español en la parte en inglés', () => {
  assert.doesNotMatch(cabeceraServidor, /docker/i)
  // Lo que lee quien eligió inglés: los bloques lang="en" y las líneas en inglés de los <pre>.
  const ingles = [...servidor.split('<div lang="en">').slice(1).map((s) => s.split(/<div lang="es">|<\/section>/)[0]),
    ...[...servidor.matchAll(/<span lang="en">([\s\S]*?)<\/span>/g)].map((m) => m[1])].join('\n')
  const codigos = [...ingles.matchAll(/<code>([^<]*)<\/code>/g)].map((m) => m[1])
  assert.ok(codigos.length > 10)
  for (const c of codigos) assert.doesNotMatch(c, /\b(instalar|administrador|contraseña|claves|guardar|actualizar|desinstalar|estado|registro|borrar-datos|activar)\b/, c)
  // Ningún <pre> enseña a la vez las dos formas: cada orden de Tramtai va dentro de su lang.
  for (const pre of servidor.match(/<pre[\s\S]*?<\/pre>/g) ?? [])
    for (const linea of pre.split('\n')) if (/(^|>)(sudo )?(\.\/|\.\\)?tramtai/.test(linea.replace(/<code>/, ''))) assert.match(linea, /lang="es"[\s\S]*lang="en"/, linea)
})

test('el README y la guía del servidor tienen al día el aviso de riesgos.* (npm run sitio)', () => {
  for (const [ruta, zonas] of [['../../README.md', ['riesgos:es', 'riesgos:en']], ['../../docs/servidor.md', ['riesgos-servidor:es', 'riesgos-servidor:en']]] as const) {
    const md = readFileSync(new URL(ruta, import.meta.url), 'utf8')
    for (const z of zonas) assert.ok(md.includes(`<!-- ${z} -->`), `${ruta}: falta la zona ${z}`)
    assert.equal(rellenarMarkdown(md, es, en), md, `${ruta}: cd web; npm run sitio`)
  }
  const ejemplo = rellenarMarkdown('a\n<!-- riesgos-servidor:en -->\nviejo\n<!-- /riesgos-servidor:en -->\nb', es, en)
  assert.equal(ejemplo, `a\n<!-- riesgos-servidor:en -->\n${riesgos(en, true).map((r, i) => `${i + 1}. ${r}`).join('\n')}\n<!-- /riesgos-servidor:en -->\nb`)
})

test('la barra y el pie llevan a lo mismo en todas las páginas', () => {
  const destinos = (zona: string) => {
    return [...zona.matchAll(/<a [^>]*href="([^"]+)"/g)].map((m) => m[1].replace(URL_SITIO, '').replace(/^(\.\.\/|\.\/)+/, ''))
  }
  const esperados = ['', 'producto.html', 'agentes.html', 'servidor.html', 'desarrolladores.html', 'ayuda/', 'descargar.html', 'ayuda/empezar.html', 'https://github.com/MscottB/Tramtai',
    'https://github.com/MscottB/Tramtai/releases', 'https://github.com/MscottB/Tramtai/blob/main/LICENSE.md', 'https://github.com/MscottB/Tramtai/blob/main/TERMINOS.md']
  const casos: [string, string][] = [...fijas, ['ayuda/agentes.html', paginas.get('ayuda/agentes.html')!], ['404.html', paginas.get('404.html')!]]
  for (const [ruta, html] of casos) {
    // Cada zona por separado: el logo de la barra ya lleva a Inicio, así que el pie debe llevar con su propio enlace.
    const zonas = { barra: html.match(/<header class="barra">[\s\S]*?<\/header>/)?.[0] ?? '', pie: html.match(/<footer>[\s\S]*?<\/footer>/)?.[0] ?? '' }
    for (const e of esperados.slice(0, 7)) {
      assert.ok(destinos(zonas.barra).includes(e), `${ruta}: la barra no lleva a «${e}»`)
      assert.ok(destinos(zonas.pie).includes(e), `${ruta}: el pie no lleva a «${e}»`)
    }
    for (const e of esperados.slice(7)) assert.ok(destinos(zonas.pie).includes(e), `${ruta}: el pie no lleva a «${e}»`)
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
    assert.match(html, /<(main|article) id="contenido"/, ruta)
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

// Auditoría 001 (25, 26, 27): ninguna raya larga y ninguna promesa absoluta que el producto no cumple. Cada frase de la
// lista se cambió por su versión con condición («por defecto», «si le das una lista de respaldo»…).
const PROHIBIDAS = [/Sin cortes/, /No outages/, /Todo se queda en tu equipo/i, /Everything stays on your computer/i, /sin que tu app lo note/,
  /without your app noticing/, /hostil/i, /hostile/i, /caducan a las 24 horas/, /expire after 24 hours/, /Escucha solo en/,
  /Listens only on/, /(?<!by default, tramtai )only listens on/i, /en \d+ minutos/, /in \d+ minutes/, /unos minutos/, /minutes away/,
  /jamás/, /no vuelve a pasar/, /won’t happen again/]

test('el sitio no tiene rayas largas ni promesas absolutas', () => {
  const textos = new Map([...todas, ['sitio.js', leer('sitio.js')], ['buscar.json', paginas.get('buscar.json')!]])
  for (const [ruta, html] of textos) {
    assert.ok(!html.includes('—'), `${ruta}: raya larga`)
    for (const p of PROHIBIDAS) assert.ok(!p.test(html), `${ruta}: ${p}`)
  }
})

test('Empezar avisa, junto al botón de descargar, de que hace falta un agente (que Tramtai instala) y del aviso de Windows', () => {
  const html = paginas.get('ayuda/empezar.html')!
  const tras = html.slice(html.indexOf('Descargar Tramtai'))
  assert.match(tras.slice(0, 1500), /Necesitas al menos un agente: Tramtai instala el CLI[\s\S]*Windows protegió su PC[\s\S]*href="#windows"/)
  assert.doesNotMatch(html, /Tramtai no los instala|doesn’t install them/)
})

test('sitemap.xml lista las siete páginas y los 20 temas de la Ayuda', () => {
  const urls = [...paginas.get('sitemap.xml')!.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1])
  for (const u of [URL_SITIO, ...['producto', 'agentes', 'servidor', 'desarrolladores', 'descargar'].map((p) => `${URL_SITIO}${p}.html`), URL_SITIO + 'ayuda/', ...temas.map((t) => `${URL_SITIO}ayuda/${t.id}.html`)])
    assert.ok(urls.includes(u), u)
  assert.equal(urls.length, 7 + 20)
})

test('404.html lleva a la portada, la Ayuda y Descargar con direcciones absolutas', () => {
  const html = paginas.get('404.html')!
  assert.match(html, /<meta name="robots" content="noindex">/)
  for (const u of [URL_SITIO, URL_SITIO + 'ayuda/', URL_SITIO + 'descargar.html']) assert.ok(html.includes(`href="${u}"`), u)
  assert.doesNotMatch(html, /(href|src)="(?!https?:|#)/, 'en una ruta cualquiera, lo relativo no carga')
})

// v0.61 (spec §1): Inter e Instrument Serif alojadas, con su licencia; Cascadia (la terminal) no cuenta para el tope.
const estilo = leer('estilo.css')
test('las letras de Inicio no pasan de 150 KB', () => {
  const letras = [...estilo.matchAll(/@font-face \{[^}]*url\('([^']+)'\)/g)].map((m) => m[1]).filter((f) => f !== 'cascadia.woff2')
  assert.ok(letras.length >= 2, letras.join(', '))
  const total = letras.reduce((s, f) => s + statSync(new URL('../../site/' + f, import.meta.url)).size, 0)
  assert.ok(total <= 150 * 1024, `${Math.round(total / 1024)} KB`)
})

test('no queda Overpass', () => {
  assert.doesNotMatch(estilo, /overpass/i)
  for (const [ruta, html] of todas) assert.doesNotMatch(html, /overpass/i, ruta)
})

test('cada letra lleva su licencia', () => {
  for (const f of ['INTER-OFL.txt', 'INSTRUMENT-OFL.txt', 'CASCADIA-OFL.txt']) assert.ok(existsSync(new URL('../../site/' + f, import.meta.url)), f)
})

const html_ = readdirSync(new URL('../../site/', import.meta.url), { recursive: true }).filter((f) => String(f).endsWith('.html')).map((f) => [String(f), leer(String(f))] as const)
test('ninguna página usa var(--titulo)', () => {
  for (const [ruta, html] of html_) assert.doesNotMatch(html, /var\(--titulo\)/, ruta)
})

// Inter e Instrument Serif llevan las letras de letras-caracteres.txt (receta en docs/equipo/desarrollo.md). El código, la
// terminal (<pre>, <code>) y los scripts los pinta Tramtai Mono: van aparte.
test('cada carácter del sitio está en el subconjunto de las letras', () => {
  const lista = new Set(readFileSync(new URL('letras-caracteres.txt', import.meta.url), 'utf8'))
  const entidades: Record<string, string> = { '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'" }
  for (const [ruta, html] of html_) {
    const texto = html.replace(/<(pre|code|script|style)\b[\s\S]*?<\/\1>/g, ' ').replace(/<[^>]*>/g, ' ').replace(/&[#\w]+;/g, (e) => entidades[e] ?? e)
    const faltan = [...new Set(texto)].filter((c) => !lista.has(c) && !/\s/.test(c))
    assert.deepEqual(faltan, [], `${ruta}: rehaz las letras con estos caracteres`)
  }
})

test('reducir movimiento para lo vivo', () => {
  const bloques = estilo.split('@media (prefers-reduced-motion: reduce)').slice(1)
  assert.ok(bloques.some((b) => /^[^@]*\.vivo/.test(b)))
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
  assert.match(html, /<aside class="doc-pagina">[\s\S]*?href="#s-recorrido"[\s\S]*?<\/aside>/)
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

test('cada tema dice al final, en una nota, cómo se hace en la terminal (antes de los temas relacionados)', () => {
  for (const t of temas) {
    const html = paginas.get(`ayuda/${t.id}.html`)!
    const nota = `<p class="aviso"><span lang="es">${escapar(es[t.terminal])}</span><span lang="en">${escapar(en[t.terminal])}</span></p>`
    const i = html.indexOf(nota)
    assert.ok(i > 0, t.id)
    const relacionados = html.indexOf('id="s-relacionados"')
    assert.ok(relacionados < 0 || i < relacionados, `${t.id}: la nota va antes de los temas relacionados`)
  }
})

test('en el sitio no hay botón «Muéstrame en pantalla»: no hay app', () => {
  for (const [ruta, html] of paginas) assert.doesNotMatch(html, /Muéstrame en pantalla|Show me on screen/, ruta)
})

test('los temas largos tienen «En esta página» con un h2 por sección y el buscador a mano', () => {
  const html = paginas.get('ayuda/agentes.html')!
  const nav = html.match(/<aside class="doc-pagina">([\s\S]*?)<\/aside>/)?.[1] ?? ''
  const anclas = [...nav.matchAll(/href="#([^"]+)"/g)].map((m) => m[1])
  assert.ok(anclas.length >= 3, 'faltan secciones en el índice')
  for (const a of anclas) assert.match(html, new RegExp(`<section id="${a}"><h2>`), a)
  assert.match(html, /<nav class="doc-indice"[\s\S]*?<button type="button" class="buscar-campo" data-buscar hidden>[\s\S]*?<\/nav>/)
})

// v0.61 (spec §6): la Ayuda, Servidor y Desarrolladores comparten la plantilla de tres columnas: el índice (con un botón
// para el móvil), el contenido y «En esta página», que sale de los <section id> con su <h2>.
const tresColumnas = /<div class="doc">\s*<nav class="doc-indice" aria-label="Índice" data-aria-en="Contents">\s*(?:<!-- comun:indice-boton -->\s*)?<button type="button" class="doc-indice-boton" aria-expanded="false" aria-controls="doc-indice-lista" hidden>[\s\S]*?<div class="doc-indice-lista" id="doc-indice-lista">[\s\S]*?<\/nav>\s*<article id="contenido" tabindex="-1" class="prosa">[\s\S]*<\/article>\s*(?:<!-- comun:en-esta-pagina -->\s*)?<aside class="doc-pagina">/
const enLaPagina = (html: string) => {
  const articulo = html.slice(html.indexOf('<article id="contenido"'), html.indexOf('<aside class="doc-pagina">'))
  const secciones = [...articulo.matchAll(/<section id="([^"]+)">\s*<h2\b|<h2 id="([^"]+)"/g)].map((m) => m[1] ?? m[2])
  const aside = html.match(/<aside class="doc-pagina">[\s\S]*?<\/aside>/)?.[0] ?? ''
  return { secciones, enlaces: [...aside.matchAll(/href="#([^"]+)"/g)].map((m) => m[1]) }
}

test('cada tema de la Ayuda va en tres columnas', () => {
  assert.deepEqual(GRUPOS_AYUDA.flatMap((g) => g.temas), temas.map((t) => t.id), 'cada tema en un grupo, en el orden de temas')
  for (const [ruta, actual] of [['ayuda/index.html', 'index.html'], ...temas.map((t) => [`ayuda/${t.id}.html`, `${t.id}.html`])]) {
    const html = paginas.get(ruta)!
    assert.match(html, tresColumnas, ruta)
    const indice = html.match(/<nav class="doc-indice"[\s\S]*?<\/nav>/)![0]
    for (const t of temas) assert.ok(indice.includes(`<a href="${t.id}.html"`), `${ruta}: el índice no lleva a ${t.id}`)
    for (const g of GRUPOS_AYUDA) assert.ok(indice.includes(`<p class="fig doc-grupo"><span lang="es">${g.es}</span><span lang="en">${g.en}</span></p>`), `${ruta}: ${g.es}`)
    assert.deepEqual([...indice.matchAll(/<a href="([^"]+)" aria-current="page"/g)].map((m) => m[1]), [actual], ruta)
    const { secciones, enlaces } = enLaPagina(html)
    assert.deepEqual(enlaces, secciones, `${ruta}: «En esta página» no sigue sus apartados`)
  }
  assert.ok(enLaPagina(paginas.get('ayuda/agentes.html')!).enlaces.length >= 3)
})

test('Servidor y Desarrolladores, en tres columnas', () => {
  for (const archivo of ['servidor.html', 'desarrolladores.html']) {
    const html = fijas.get(archivo)!
    assert.match(html, tresColumnas, archivo)
    assert.match(html, /<main class="envoltura doc-envoltura">/, archivo)
    // El titular en dos partes, con la cursiva en <em>.
    assert.match(html, /<h1 class="titular"><span lang="es">[^<]+<em>[^<]+<\/em><\/span><span lang="en">[^<]+<em>[^<]+<\/em><\/span><\/h1>/, archivo)
    const { secciones, enlaces } = enLaPagina(html)
    assert.ok(secciones.length >= 8, `${archivo}: ${secciones}`)
    assert.deepEqual(enlaces, secciones, `${archivo}: cada <h2 id> del contenido, en «En esta página»`)
    // El índice de la izquierda lleva a cada apartado de la página.
    const indice = html.match(/<nav class="doc-indice"[\s\S]*?<\/nav>/)![0]
    for (const id of secciones) assert.ok(indice.includes(`href="#${id}"`), `${archivo}: el índice no lleva a #${id}`)
  }
  // Los diagramas: el mapa del servidor y, en Desarrolladores, tu app → Tramtai → agente; con su FIG y vivos.
  assert.match(servidor, /<figure class="tarjeta fig-mapa vivo">\s*<figcaption class="fig">/)
  assert.match(fijas.get('desarrolladores.html')!, /<figure class="tarjeta fig-camino vivo">\s*<figcaption class="fig">/)
})

// La Ayuda del sitio y la de la app salen de los mismos textos (spec, Riesgos): cada tema generado lleva los suyos.
test('los textos de la Ayuda no cambian', () => {
  const claves = (x: unknown): string[] => typeof x === 'string' ? (Object.hasOwn(es, x) ? [x] : [])
    : Array.isArray(x) ? x.flatMap(claves)
    // alt solo sale con su captura y ancla es un data-recorrido de la app, no un texto.
    : x && typeof x === 'object' ? Object.entries(x).flatMap(([k, v]) => (k === 'alt' || k === 'ancla' ? [] : claves(v))) : []
  const plano = (s: string) => s.replace(/<[^>]+>/g, '').replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&').replace(/\s+/g, ' ')
  for (const t of temas) {
    const html = plano(paginas.get(`ayuda/${t.id}.html`)!)
    const lista = [t.titulo, t.resumen, t.terminal, ...claves(bloquesVisibles(t, 'sitio'))]
    assert.ok(lista.length > 5, t.id)
    for (const k of lista)
      for (const textos of [es, en]) assert.ok(html.includes(textos[k as keyof typeof es].replace(/\s+/g, ' ')), `${t.id}: ${k}`)
  }
})

test('iconos, imagen para compartir, manifest y datos estructurados', () => {
  for (const a of ['og.png', 'favicon.svg', 'favicon-32.png', 'apple-touch-icon.png'])
    assert.ok(existsSync(new URL('../../site/' + a, import.meta.url)), a)
  assert.equal(JSON.parse(leer('manifest.webmanifest')).name, 'Tramtai')
  const ld = JSON.parse(fijas.get('index.html')!.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/)![1])
  assert.equal(ld['@type'], 'SoftwareApplication')
  assert.match(leer('sitio.js'), /'tramtai\.lenguaje-ayuda'/, 'la Ayuda recuerda su lenguaje aparte de la guía')
})

// funcionDe saca una función de arriba del todo de sitio.js (sin sangría) para probarla sin navegador, como el script de
// la página de paso.
const funcionDe = (nombre: string) =>
  new Function(`${leer('sitio.js').match(new RegExp(`\\nfunction ${nombre}\\([\\s\\S]*?\\n\\}`))![0]}\nreturn ${nombre}`)()

// Pendiente de la Tarea 1: con el margen de abajo recortado un 20 %, un .vivo corto al final de la página no llegaba.
test('lo vivo arranca con un cuarto a la vista, también al final de la página o si es más alto que ella', () => {
  const visto = funcionDe('bastanteVisto')
  const e = (ratio: number, alto: number) => ({ intersectionRatio: ratio, intersectionRect: { height: alto } })
  assert.equal(visto(e(1, 80), 900), true, 'corto y entero a la vista, al final de la página')
  assert.equal(visto(e(0.1, 60), 900), false, 'solo asoma')
  assert.equal(visto(e(0.12, 300), 900), true, 'más alto que la pantalla: un cuarto de ella')
  assert.equal(visto(e(0.2, 200), 900), false)
  assert.match(leer('sitio.js'), /\{ threshold: \[0, 0\.25\] \}/)
  assert.doesNotMatch(leer('sitio.js'), /rootMargin: '0px 0px -20% 0px'/)
})

// Pendientes de la Tarea 3.
test('en el buscador, Ctrl+Enter o ⌘+Enter abre el resultado en otra pestaña', () => {
  assert.match(leer('sitio.js'), /e\.key === 'Enter' && activo >= 0\) \{[\s\S]{0,80}opciones\[activo\]\.dispatchEvent\(new MouseEvent\('click', \{[^}]*ctrlKey: e\.ctrlKey, metaKey: e\.metaKey/)
})

test('cada tema de buscar.json tiene texto donde buscar', () => {
  const indice = JSON.parse(paginas.get('buscar.json')!)
  for (const idioma of ['es', 'en'])
    for (const t of temas) {
      const e = indice[idioma].find((x: { u: string }) => x.u === `ayuda/${t.id}.html`)
      assert.ok(e?.x.trim(), `${idioma}: ${t.id}`)
    }
})
