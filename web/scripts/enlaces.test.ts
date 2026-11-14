// La prueba de enlaces de la v0.61: lee site/ tal cual queda tras `npm run sitio` y vigila que ningún enlace, ancla o
// dirección de una app instalada (spec §7) se rompa en el resto de la versión.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { ANCLAS_PORTADA, URL_SITIO, destinoPortada } from './sitio.ts'
import { temas } from '../src/ayuda/indice.ts'

const SITIO = new URL('../../site/', import.meta.url)
const PAGINAS = ['index.html', 'producto.html', 'agentes.html', 'servidor.html', 'desarrolladores.html', 'ayuda/index.html', 'descargar.html']
const todas = [...readdirSync(SITIO).filter((a) => a.endsWith('.html')), ...readdirSync(new URL('ayuda/', SITIO)).filter((a) => a.endsWith('.html')).map((a) => 'ayuda/' + a)]
const leer = (ruta: string) => readFileSync(new URL(ruta, SITIO), 'utf8')

// destino lleva un href o src de una página a su archivo (y su ancla), o null si es externo, un mailto o un data:.
function destino(desde: string, valor: string): { archivo: string; ancla: string } | null {
  if (valor.startsWith(URL_SITIO)) [desde, valor] = ['', valor.slice(URL_SITIO.length) || './']
  else if (/^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(valor)) return null
  const url = new URL(valor, 'http://x/' + desde)
  let archivo = decodeURIComponent(url.pathname.slice(1))
  if (!archivo || archivo.endsWith('/')) archivo += 'index.html'
  return { archivo, ancla: url.hash.slice(1) }
}

// enlaces saca cada href y src, con comillas dobles o simples, y cada URL de un srcset.
const enlaces = (html: string) => [...html.matchAll(/\b(href|src|srcset)=(?:"([^"]*)"|'([^']*)')/g)]
  .flatMap((m) => (m[1] === 'srcset' ? (m[2] ?? m[3]).split(',').map((c) => c.trim().split(/\s+/)[0]) : [m[2] ?? m[3]]))
  .map((v) => v.replace(/&amp;/g, '&')).filter((v) => v && v !== '#')

// existe compara el nombre exacto de cada tramo con su carpeta: en Windows existsSync no distingue mayúsculas y GitHub Pages sí.
const existe = (archivo: string) => {
  let dir = new URL('./', SITIO)
  const tramos = archivo.split('/')
  return tramos.every((tramo, i) => {
    if (!readdirSync(dir).includes(tramo)) return false
    dir = new URL(tramo + (i < tramos.length - 1 ? '/' : ''), dir)
    return true
  })
}
// tieneId busca id="x" exacto (no data-id="x"), con el ancla escapada.
const tieneId = (archivo: string, ancla: string) => new RegExp(`\\sid="${ancla.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}"`).test(leer(archivo))

test('la prueba ve las comillas simples, los srcset, las mayúsculas y los data-id', () => {
  assert.deepEqual(enlaces(`<a href='zzz.html'><img srcset="a.png 1x, b.png 2x" src="c.png">`), ['zzz.html', 'a.png', 'b.png', 'c.png'])
  assert.ok(existe('ayuda/index.html') && !existe('Ayuda/index.html') && !existe('INDEX.html') && !existe('zzz.html'))
  assert.ok(tieneId('index.html', 'descargar') && !new RegExp('\\sid="x"').test('<b data-id="x">'))
})

test('cada enlace interno lleva a un archivo que existe', () => {
  for (const pagina of todas)
    for (const valor of enlaces(leer(pagina))) {
      const d = destino(pagina, valor)
      if (d) assert.ok(existe(d.archivo), `${pagina}: ${valor} lleva a ${d.archivo}, que no existe`)
    }
})

test('cada ancla existe en su página', () => {
  for (const pagina of todas)
    for (const valor of enlaces(leer(pagina))) {
      const d = destino(pagina, valor)
      if (!d?.ancla || !d.archivo.endsWith('.html')) continue
      // Los anclajes viejos de la portada saltan a su sitio nuevo con destinoPortada, aunque ya no tengan id en index.html.
      if (d.archivo === 'index.html' && Object.hasOwn(ANCLAS_PORTADA, d.ancla)) continue
      assert.ok(tieneId(d.archivo, d.ancla), `${pagina}: ${valor} no encuentra id="${d.ancla}" en ${d.archivo}`)
    }
})

test('las direcciones de las apps instaladas siguen', () => {
  for (const archivo of ['servidor.html', 'desarrolladores.html', 'ayuda/index.html', ...temas.map((t) => `ayuda/${t.id}.html`)])
    assert.ok(existe(archivo), archivo)
  assert.equal(destinoPortada('#descargar'), 'descargar.html')
  assert.equal(destinoPortada('#formas'), null, 'un ancla de la portada nueva no salta')
  for (const [ancla, destinoNuevo] of Object.entries(ANCLAS_PORTADA)) {
    const d = destino('index.html', destinoNuevo)!
    assert.ok(existe(d.archivo), `#${ancla} → ${destinoNuevo}`)
    if (d.ancla && d.archivo !== 'index.html') assert.ok(tieneId(d.archivo, d.ancla), `#${ancla} → ${destinoNuevo}`)
  }
})

test('cada página tiene la barra, el pie y los dos idiomas', () => {
  for (const pagina of [...PAGINAS, '404.html']) {
    const html = leer(pagina)
    assert.match(html, /<header class="barra">[\s\S]*<\/header>/, `${pagina}: falta la barra`)
    assert.match(html, /<footer>[\s\S]*<\/footer>/, `${pagina}: falta el pie`)
    assert.ok(html.includes('lang="es"') && html.includes('lang="en"'), `${pagina}: faltan los dos idiomas`)
    if (pagina === '404.html' || pagina === 'ayuda/index.html') continue
    for (const zona of ['cabecera', 'pie'])
      assert.match(html, new RegExp(`<!-- comun:${zona} -->\\s*<(?:a|header|footer)\\b[\\s\\S]*<!-- /comun:${zona} -->`), `${pagina}: comun:${zona} vacío`)
  }
})

test('la barra marca la página actual y lleva a las páginas nuevas', () => {
  for (const [pagina, esperado] of [['producto.html', 'producto.html'], ['agentes.html', 'agentes.html'], ['servidor.html', 'servidor.html'],
    ['desarrolladores.html', 'desarrolladores.html'], ['ayuda/index.html', '../ayuda/'], ['descargar.html', 'descargar.html']]) {
    const nav = leer(pagina).match(/<nav aria-label="Secciones"[\s\S]*?<\/nav>/)?.[0] ?? ''
    const marcadas = [...nav.matchAll(/<a href="([^"]+)"[^>]*aria-current="page"/g)].map((m) => m[1])
    assert.equal(marcadas.length, 1, `${pagina}: ${marcadas}`)
    assert.ok(marcadas[0].endsWith(esperado.replace('../', '')), `${pagina}: ${marcadas[0]}`)
  }
})

test('el sitemap tiene las siete páginas y los temas', () => {
  const urls = [...leer('sitemap.xml').matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1])
  for (const u of ['', 'producto.html', 'agentes.html', 'servidor.html', 'desarrolladores.html', 'ayuda/', 'descargar.html', ...temas.map((t) => `ayuda/${t.id}.html`)])
    assert.ok(urls.includes(URL_SITIO + u), u)
  assert.equal(urls.length, 7 + temas.length)
})
