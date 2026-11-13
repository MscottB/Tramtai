import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { ANCLAS_VIEJAS, PAGINAS_FIJAS, URL_SITIO, destinoAncla, escapar, generar, rellenar, rellenarMarkdown, riesgos, type Fuente } from './sitio.ts'

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

test('el índice de búsqueda tiene los 20 temas en los dos idiomas', () => {
  const indice = JSON.parse(paginas.get('ayuda/indice.json')!)
  for (const idioma of ['es', 'en']) assert.equal(new Set(indice[idioma].map((e: { t: string }) => e.t)).size, 20)
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
  assert.deepEqual(PAGINAS_FIJAS.map((p) => p.archivo), ['index.html', 'desarrolladores.html', 'servidor.html'])
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
  for (const [archivo, servidor] of [['index.html', false], ['servidor.html', true]] as const) {
    const html = fijas.get(archivo)!
    const zona = html.match(new RegExp(`<!-- comun:${servidor ? 'riesgos-servidor' : 'riesgos'} -->([\\s\\S]*?)<!-- /comun:`))?.[1] ?? ''
    for (const [i, r] of riesgos(es, servidor).entries()) assert.ok(zona.includes(`<span lang="es">${escapar(r)}</span>`), `${archivo}: riesgo ${i + 1} en español`)
    for (const [i, r] of riesgos(en, servidor).entries()) assert.ok(zona.includes(`<span lang="en">${escapar(r)}</span>`), `${archivo}: riesgo ${i + 1} en inglés`)
    assert.match(html, /href="https:\/\/github\.com\/MscottB\/Tramtai\/blob\/main\/TERMINOS\.md"/, `${archivo}: el aviso lleva a los términos`)
  }
})

// v0.59 (spec §6): servidor.html abre con una cabecera que presenta el modo servidor (titular, entrada, el mapa de la
// portada en grande, «¿Es para ti?», tres pasos por sistema y dos capturas reales), y debajo sigue la documentación.
const servidor = fijas.get('servidor.html')!
const cabeceraServidor = servidor.slice(servidor.indexOf('<main'), servidor.indexOf('<nav class="indice"'))
test('servidor.html: la cabecera nueva tiene titular, entrada, mapa, «¿Es para ti?» y las capturas en es/en', () => {
  assert.match(cabeceraServidor, /<div class="servidor-cabecera">\s*<header>\s*<h1>/)
  const entrada = cabeceraServidor.match(/<p class="entrada"><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/p>/)
  assert.ok(entrada, 'falta la entrada')
  for (const t of entrada.slice(1)) assert.ok(Math.abs(t.split(/\s+/).length - 30) <= 8, `unas 30 palabras: ${t}`)
  const mapa = cabeceraServidor.match(/<figure class="mapa-servidor">[\s\S]*?<\/figure>/)?.[0] ?? ''
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
  const indice = [...(servidor.match(/<nav class="indice"[\s\S]*?<\/nav>/)?.[0] ?? '').matchAll(/href="#([^"]+)"/g)].map((m) => m[1])
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
  const destinos = (html: string) => {
    const zona = (html.match(/<header class="barra">[\s\S]*?<\/header>/)?.[0] ?? '') + (html.match(/<footer>[\s\S]*?<\/footer>/)?.[0] ?? '')
    return [...zona.matchAll(/<a [^>]*href="([^"]+)"/g)].map((m) => m[1].replace(URL_SITIO, '').replace(/^(\.\.\/|\.\/)+/, ''))
  }
  const esperados = ['#descargar', 'ayuda/empezar.html', '#agentes', 'ayuda/', 'desarrolladores.html', 'servidor.html', 'https://github.com/MscottB/Tramtai',
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

// Auditoría 001 (25, 26, 27): ninguna raya larga y ninguna promesa absoluta que el producto no cumple. Cada frase de la
// lista se cambió por su versión con condición («por defecto», «si le das una lista de respaldo»…).
const PROHIBIDAS = [/Sin cortes/, /No outages/, /Todo se queda en tu equipo/i, /Everything stays on your computer/i, /sin que tu app lo note/,
  /without your app noticing/, /hostil/i, /hostile/i, /caducan a las 24 horas/, /expire after 24 hours/, /Escucha solo en/,
  /Listens only on/, /(?<!by default, tramtai )only listens on/i, /en \d+ minutos/, /in \d+ minutes/, /unos minutos/, /minutes away/,
  /jamás/, /no vuelve a pasar/, /won’t happen again/]

test('el sitio no tiene rayas largas ni promesas absolutas', () => {
  const textos = new Map([...todas, ['sitio.js', leer('sitio.js')], ['ayuda/indice.json', paginas.get('ayuda/indice.json')!]])
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

test('sitemap.xml lista la portada, las dos guías, la Ayuda y sus 20 temas', () => {
  const urls = [...paginas.get('sitemap.xml')!.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1])
  for (const u of [URL_SITIO, URL_SITIO + 'desarrolladores.html', URL_SITIO + 'servidor.html', URL_SITIO + 'ayuda/', ...temas.map((t) => `${URL_SITIO}ayuda/${t.id}.html`)])
    assert.ok(urls.includes(u), u)
  assert.equal(urls.length, 4 + 20)
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
