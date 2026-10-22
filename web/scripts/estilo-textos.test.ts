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

// sinFallos enseña la lista entera: assert.deepEqual la recorta cuando es larga.
const sinFallos = (lista: string[]) => assert.ok(lista.length === 0, [`${lista.length} fallos:`, ...lista].join('\n'))

// Las palabras huecas de antislop R-16, en los dos idiomas.
const huecas: Regla[] = [
  { nombre: 'palabra hueca (R-16)', re: /\b(ai[- ]powered|next[- ]generation|revolutionary|seamless(ly)?|cutting[- ]edge|intelligent|ultimate|powerful|effortless(ly)?)\b/i },
  { nombre: 'palabra hueca (R-16)', re: /(impulsad[oa]s? por (la )?ia|de nueva generación|revolucionari[oa]s?|sin fisuras|de vanguardia|inteligentes?\b|\bpotentes?\b|sin esfuerzo)/i },
]
const raya: Regla = { nombre: 'raya larga', re: /\u2014/ }

// tecnico quita lo que no se traduce: código entre acentos graves, órdenes de la CLI (tramtai …, en minúsculas) y
// nombres con _, -, / o punto (claves de configuración, cabeceras, rutas, archivos).
const tecnico = (t: string) =>
  t.replace(/`[^`]*`/g, ' ').replace(/\btramtai(?: [a-z][a-z-]*)+/g, ' ').replace(/[\w.]*[_/][\w./-]*|\b\w+(?:-\w+)+/g, ' ')
const sinTecnico = (textos: Texto[]) => textos.map((x) => ({ ...x, texto: tecnico(x.texto) }))

// Los términos que el glosario descarta para el usuario.
const descartadosEs: Regla[] = [
  { nombre: 'instancia → espacio', re: /\binstancias?\b/i },
  { nombre: 'punto de acceso → API', re: /\bpuntos? de acceso\b/i },
  { nombre: 'token → clave', re: /\btokens? (del|de la|de servicio|de la api|nuevo)\b|\b(el|un|su|tu) token\b/i },
  { nombre: 'trabajo → petición', re: /\btrabajos?\b(?<!espacios? de trabajos?)/i },
  { nombre: 'solicitud → petición', re: /\bsolicitud(es)?\b/i },
  { nombre: 'coste → costo', re: /\bcostes?\b/i },
  { nombre: 'Intro → Enter', re: /\bIntro\b/ },
  { nombre: 'pasarela o puente → API local', re: /\b(pasarela|puente)\b/i },
  { nombre: 'la API de Gemini → la Gemini API', re: /\bAPI de Gemini\b/ },
  { nombre: 'asistente o programa de IA → agente', re: /\b(asistentes?|programas?) de IA\b/i },
  { nombre: 'en proceso → en curso', re: /\ben proceso\b/i },
  { nombre: 'bienvenida → recorrido guiado', re: /\brecorrido de bienvenida\b|\bLa bienvenida de Tramtai\b/i },
]
const descartadosEn: Regla[] = [
  { nombre: 'instance → workspace', re: /\binstances?\b/i },
  { nombre: 'access point → API', re: /\baccess points?\b/i },
  { nombre: 'token → key', re: /\b(the|a|its|your|new|service|workspace) token\b/i },
  { nombre: 'job → request', re: /\bjobs?\b/i },
  { nombre: 'gateway o bridge → local API', re: /\b(gateway|bridge)\b/i },
  { nombre: 'welcome tour → guided tour', re: /\bwelcome tour\b/i },
]

test('los textos usan espacio, API, clave y petición', () => {
  sinFallos([...fallos(sinTecnico(textosEs), descartadosEs), ...fallos(sinTecnico(textosEn), descartadosEn)])
})

// El inglés: un solo estilo de comillas (curvas), la variante de Estados Unidos, «Playground» como nombre de la página
// Probar («Try» solo como verbo) y «log» para el registro. Las {llaves} de JSON y el código no cuentan.
const ingles: Regla[] = [
  { nombre: 'comillas rectas → “ ”', re: /"[^"]*"/ },
  { nombre: 'comillas angulares → “ ”', re: /[«»]/ },
  { nombre: 'apóstrofo recto → ’', re: /[A-Za-z]'[A-Za-z]/ },
  { nombre: 'variante UK → US', re: /\bcancell(ed|ing)\b/i },
  // «Try» con mayúscula es la página salvo en los usos de verbo: «Try again», «Try it», «Try a/an», «Try another»,
  // «Try the…» y «Try <code>…» (una orden). Así atrapa también «(Try offers» y «Try shows» al principio de frase.
  { nombre: 'Try como página → Playground', re: /\bTry\b(?! (again|it|an?|another|the)\b| <code>)/ },
  { nombre: 'record → log', re: /\brecords?\b/i },
]
const sinJson = (textos: Texto[]) => textos.map((x) => ({ ...x, texto: x.texto.replace(/`[^`]*`/g, ' ').replace(/\{[^{}]*\}/g, ' ') }))

test('el inglés usa comillas curvas, la variante de EE. UU. y los nombres del glosario', () => {
  sinFallos(fallos(sinJson(textosEn), ingles))
})

test('ningún texto de producto lleva la raya larga ni palabras huecas', () => {
  assert.ok(go.es.length > 500 && web.es.length >= 20 && web.en.length > 100, 'no se leyeron los catálogos o el sitio')
  sinFallos(fallos([...textosEs, ...textosEn], [raya, ...huecas]))
})
