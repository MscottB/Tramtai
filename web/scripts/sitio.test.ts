import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { ANCLAS_VIEJAS, destinoAncla, escapar, generar, type Fuente } from './sitio.ts'

const fuente: Fuente = {
  temas, es, en, capturas: new Set(),
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
