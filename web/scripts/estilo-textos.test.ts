// Las reglas de escritura de docs/GLOSARIO.md hechas prueba, sobre todo texto de producto: la Web UI (es.ts y en.ts,
// que incluyen la Ayuda), los catálogos de la CLI, el servicio, la terminal y la app de escritorio (en_*.go) y el sitio
// publicado (site/, también la Ayuda generada). Los comentarios de código y docs/equipo quedan fuera.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'

type Texto = { donde: string; texto: string }
type Regla = { nombre: string; re: RegExp }

const raiz = new URL('../../', import.meta.url)
const leer = (ruta: string) => readFileSync(new URL(ruta, raiz), 'utf8')

// catalogos lee los pares «español: inglés» de internal/idioma/en_*.go; un texto partido con + se une.
function catalogos() {
  const r = { es: [] as Texto[], en: [] as Texto[] }
  const dir = 'internal/idioma/'
  for (const archivo of readdirSync(new URL(dir, raiz)).filter((a) => /^en_.*\.go$/.test(a))) {
    let actual = ''
    let clave: string | undefined
    for (const [token] of leer(dir + archivo).matchAll(/"(?:[^"\\]|\\.)*"|\/\/[^\n]*|[:,]/g)) {
      if (token.startsWith('//')) continue
      if (token.startsWith('"')) actual += JSON.parse(token) as string
      else if (token === ':') [clave, actual] = [actual, '']
      else if (clave !== undefined) {
        r.es.push({ donde: `${archivo} (clave)`, texto: clave })
        r.en.push({ donde: archivo, texto: actual })
        ;[clave, actual] = [undefined, '']
      } else actual = ''
    }
  }
  return r
}

const quitarEtiquetas = (html: string) =>
  html
    .replace(/<(script|style|pre|code)\b[\s\S]*?<\/\1>/g, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')

// sitio separa el texto de cada página publicada por idioma: lo marcado con lang="en" o data-en es inglés; lo demás,
// español.
function sitio() {
  const r = { es: [] as Texto[], en: [] as Texto[] }
  const paginas = ['site/index.html', 'site/desarrolladores.html', 'site/404.html',
    ...readdirSync(new URL('site/ayuda/', raiz)).filter((a) => a.endsWith('.html')).map((a) => 'site/ayuda/' + a)]
  for (const ruta of paginas) {
    const html = leer(ruta).replace(/<(script|style|pre|code)\b[\s\S]*?<\/\1>/g, ' ')
    const ingles = /<(\w+)\b[^>]*\blang="en"[^>]*>([\s\S]*?)<\/\1>/g
    for (const m of html.matchAll(ingles)) r.en.push({ donde: ruta, texto: quitarEtiquetas(m[2]) })
    for (const m of html.matchAll(/\bdata-en="([^"]*)"/g)) r.en.push({ donde: ruta, texto: m[1] })
    const espanol = html.replace(ingles, ' ').replace(/\bdata-en="[^"]*"/g, ' ').replace(/<head>[\s\S]*?<\/head>/, ' ')
    r.es.push({ donde: ruta, texto: quitarEtiquetas(espanol) })
  }
  return r
}

const go = catalogos()
const web = sitio()
const textosEs: Texto[] = [...Object.entries(es).map(([k, v]) => ({ donde: `es.ts ${k}`, texto: v })), ...go.es, ...web.es]
const textosEn: Texto[] = [...Object.entries(en).map(([k, v]) => ({ donde: `en.ts ${k}`, texto: v })), ...go.en, ...web.en]

// fallos devuelve cada texto que rompe una regla, con dónde está y el trozo que la rompe.
function fallos(textos: Texto[], reglas: Regla[]) {
  const r: string[] = []
  for (const { donde, texto } of textos)
    for (const { nombre, re } of reglas) {
      const m = texto.match(re)
      if (m) r.push(`${donde}: ${nombre}: ${m[0]}`)
    }
  return r
}

// Las palabras huecas de antislop R-16, en los dos idiomas.
const huecas: Regla[] = [
  { nombre: 'palabra hueca (R-16)', re: /\b(ai[- ]powered|next[- ]generation|revolutionary|seamless(ly)?|cutting[- ]edge|intelligent|ultimate|powerful|effortless(ly)?)\b/i },
  { nombre: 'palabra hueca (R-16)', re: /(impulsad[oa]s? por (la )?ia|de nueva generación|revolucionari[oa]s?|sin fisuras|de vanguardia|inteligentes?\b|\bpotentes?\b|sin esfuerzo)/i },
]
const raya: Regla = { nombre: 'raya larga', re: /—/ }

test('ningún texto de producto lleva la raya larga ni palabras huecas', () => {
  assert.ok(go.es.length > 500 && web.es.length >= 20 && web.en.length > 100, 'no se leyeron los catálogos o el sitio')
  assert.deepEqual(fallos([...textosEs, ...textosEn], [raya, ...huecas]), [])
})
