import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, statSync } from 'node:fs'
import { temas } from '../src/ayuda/indice.ts'
import { capturaDeAncla } from '../src/ayuda/vista.ts'
import { readFileSync } from 'node:fs'
import { ANCHO_AYUDA, ANCLAS, CAPTURAS, POR_SACAR, ansiAHtml, medidas, PORTADA, revisarAnclas, rutaPrivada, SIN_CAPTURA, TERMINAL, TOKENS_TERMINAL } from './capturas-lista.ts'

const carpeta = new URL('../src/ayuda/capturas/', import.meta.url)
const citadas = new Set(temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'captura' ? [b.id] : []))))
// Una captura por ancla de recorrido, aunque la ancla salga en varias páginas.
const anclas = [...new Set(temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'recorrido' ? b.pasos.map((p) => p.ancla) : []))))]
const archivos = readdirSync(carpeta).filter((a) => a.endsWith('.webp'))
const variantes = (id: string) => ['claro', 'oscuro'].flatMap((tema) => ['es', 'en'].map((idioma) => `${id}.${tema}.${idioma}.webp`))

test('el script saca justo las capturas que citan los temas', () => {
  assert.deepEqual(CAPTURAS.map((c) => c.id).sort(), [...citadas].sort())
  assert.deepEqual(PORTADA.map((p) => p.nombre).sort(), ['agentes', 'espacios', 'inicio', 'mis-apis', 'probar'])
})

test('el script saca una captura por cada ancla de los recorridos, en la página de su tema', () => {
  assert.deepEqual(ANCLAS.map((a) => a.ancla).sort(), [...anclas].sort())
  for (const a of ANCLAS) {
    assert.equal(a.id, capturaDeAncla(a.ancla))
    const tema = temas.find((t) => t.bloques.some((b) => b.tipo === 'recorrido' && b.pasos.some((p) => p.ancla === a.ancla)))
    assert.equal(a.pagina, tema?.paginaApp, a.ancla)
  }
})

test('cada captura citada existe en claro y oscuro y en los dos idiomas', () => {
  for (const id of citadas) for (const v of variantes(id)) if (!(id in POR_SACAR)) assert.ok(archivos.includes(v), v)
})

test('POR_SACAR solo lista capturas citadas que aún no existen, y dice por qué', () => {
  for (const [id, motivo] of Object.entries(POR_SACAR)) {
    assert.ok(citadas.has(id), `${id} no la cita ningún tema`)
    assert.ok(motivo.length >= 20, `${id}: falta el motivo`)
    for (const v of variantes(id)) assert.ok(!archivos.includes(v), `sobra ${v}: ${id} ya está sacada, quítala de POR_SACAR`)
  }
})

test('cada ancla de cada recorrido tiene sus 4 capturas, salvo las excepciones explicadas', () => {
  assert.deepEqual(revisarAnclas(anclas, SIN_CAPTURA, archivos), [])
})

test('revisarAnclas: una excepción de SIN_CAPTURA no necesita capturas y no puede tenerlas', () => {
  const todas = (id: string) => variantes(id)
  const sin = { 'historial.detalle': 'la demo no tiene peticiones que abrir' }
  // la ancla normal sin capturas falla; la excepción, sin capturas, no
  assert.deepEqual(revisarAnclas(['inicio.mapa', 'historial.detalle'], sin, []), todas('recorrido-inicio-mapa').map((v) => `falta ${v}`))
  assert.deepEqual(revisarAnclas(['inicio.mapa', 'historial.detalle'], sin, todas('recorrido-inicio-mapa')), [])
  // una excepción que ya tiene capturas sobra
  assert.deepEqual(revisarAnclas(['historial.detalle'], sin, todas('recorrido-historial-detalle')).length, 4)
})

test('rutaPrivada: la carpeta personal, la temporal y la del repositorio, con / o \\; la ruta de ejemplo no', () => {
  const privada = rutaPrivada(['C:\\Users\\Ana', 'C:\\Users\\ANA~1\\AppData\\Local\\Temp', 'D:\\repos\\Tramtai', '/tmp'])
  for (const texto of ['C:\\Users\\Ana\\Documents', 'c:/users/ana/app', 'C:\\Users\\ANA~1\\AppData\\Local\\Temp\\x', 'D:\\repos\\Tramtai\\bin',
    'bin\\capturas-a1B2c3', '"C:\\\\Users\\\\Ana\\\\Documents"'])
    assert.ok(privada.test(texto), texto)
  for (const texto of ['C:\\Proyectos\\mi-app', '~/proyectos/mi-app', 'C:\\Users', 'guardado en /tmp', 'needs no Ana rights'])
    assert.ok(!privada.test(texto), texto)
})

test('las capturas de la Ayuda no pasan del ancho de la columna del sitio (sin bytes de más al descargarlas)', () => {
  for (const a of archivos) {
    const { ancho } = medidas(readFileSync(new URL(a, carpeta)))
    assert.ok(ancho <= ANCHO_AYUDA, `${a} mide ${ancho} px`)
  }
})

test('las excepciones son pocas, son anclas reales y dicen por qué', () => {
  assert.ok(Object.keys(SIN_CAPTURA).length <= 3, 'una lista corta: primero se intenta que la demo muestre el ancla')
  for (const [ancla, motivo] of Object.entries(SIN_CAPTURA)) {
    assert.ok(anclas.includes(ancla), `${ancla} no es ancla de ningún recorrido`)
    assert.ok(motivo.length >= 20, `${ancla}: falta el motivo`)
  }
})

test('no hay capturas que nadie cite y entre todas no pasan de 5 MB', () => {
  const todas = new Set([...citadas, ...anclas.map(capturaDeAncla)])
  for (const a of archivos) assert.ok(todas.has(a.split('.')[0]), `${a} no la cita ningún tema`)
  const total = archivos.reduce((s, a) => s + statSync(new URL(a, carpeta)).size, 0)
  assert.ok(total <= 5 * 1024 * 1024, `las capturas pesan ${total} bytes`)
})

// v0.58: las capturas del modo servidor salen de un Tramtai en modo servidor detrás de Caddy, en contenedores.
test('las capturas del servidor son las de Ajustes → Servidor y Seguridad, la entrada y la marca', () => {
  assert.deepEqual(CAPTURAS.filter((c) => c.servidor).map((c) => c.id).sort(), ['entrar', 'marca', 'seguridad', 'servidor'])
  assert.ok(CAPTURAS.some((c) => c.id === 'arranque' && !c.servidor), 'Ajustes → Arranque sale del Tramtai de escritorio')
})

// v0.59: la terminal de la portada es texto real. ansiAHtml no escribe colores: escribe la clase del token de DESIGN.md
// (t-<token> para la tinta, tf-<token> para el fondo), y la hoja del sitio le da el valor del tema.
test('ansiAHtml: el mismo token en el tema claro y en el oscuro, negrita, tenue, fondo y restablecer', () => {
  assert.equal(ansiAHtml('\x1b[38;2;45;212;191mX'), '<span class="t-acento">X</span>')
  assert.equal(ansiAHtml('\x1b[38;2;11;110;100mX'), '<span class="t-acento">X</span>')
  assert.equal(ansiAHtml('a\x1b[1;38;2;11;110;100mb\x1b[0mc'), 'a<span class="t-acento t-b">b</span>c')
  assert.equal(ansiAHtml('\x1b[2;38;2;147;165;172mx\x1b[22m y'), '<span class="t-suave t-d">x</span><span class="t-suave"> y</span>')
  assert.equal(ansiAHtml('\x1b[48;2;17;56;56m\x1b[38;2;229;237;240mx\x1b[49m\x1b[39my'), '<span class="t-texto tf-velo">x</span>y')
  assert.equal(ansiAHtml('\x1b[1mn\x1b[m'), '<span class="t-b">n</span>')
})

test('ansiAHtml junta lo contiguo con las mismas clases y quita lo que no es estilo', () => {
  assert.equal(ansiAHtml('\x1b[38;2;74;222;128ma\x1b[38;2;74;222;128mb'), '<span class="t-ok">ab</span>')
  assert.equal(ansiAHtml('\x1b[2Ku\x1b]8;;https://x\x1b\\v\x1b]8;;\x1b\\'), 'uv', 'sin borrados de línea ni enlaces OSC 8')
})

test('ansiAHtml envuelve cada glifo que Cascadia no trae (✗, flechas, formas) en su span g y deja │ y ░', () => {
  assert.equal(ansiAHtml('\x1b[38;2;248;113;113m✗ x'), '<span class="t-error g">✗</span><span class="t-error"> x</span>')
  assert.equal(ansiAHtml('a ✓ b'), 'a <span class="g">✓</span> b')
  assert.equal(ansiAHtml('↑↓'), '<span class="g">↑</span><span class="g">↓</span>', 'cada glifo en el suyo: ocupa una celda')
  assert.equal(ansiAHtml('\x1b[1;38;2;11;110;100m▸ 1\x1b[m'), '<span class="t-acento t-b g">▸</span><span class="t-acento t-b"> 1</span>')
  assert.equal(ansiAHtml('│░·'), '│░·')
  for (const g of ['←', '⇿', '■', '▸', '▾', '○', '●', '✓', '✗', '➿']) assert.equal(ansiAHtml(g), `<span class="g">${g}</span>`, g)
})

test('ansiAHtml escapa <, & y >', () => {
  assert.equal(ansiAHtml('\x1b[38;2;248;113;113m<a & b>'), '<span class="t-error">&lt;a &amp; b&gt;</span>')
  assert.equal(ansiAHtml('<b>&amp;'), '&lt;b&gt;&amp;amp;')
})

test('ansiAHtml falla con un color sin token y con un estilo que no sabe pintar', () => {
  assert.throws(() => ansiAHtml('\x1b[38;2;18;52;86mX'), /#123456/)
  assert.throws(() => ansiAHtml('\x1b[48;2;18;52;86mX'), /#123456/)
  assert.throws(() => ansiAHtml('\x1b[31mX'), /31/, 'los 16 colores no tienen token')
  assert.throws(() => ansiAHtml('\x1b[38;5;196mX'), /38/, 'ni los 256')
  assert.throws(() => ansiAHtml('\x1b[7mX'), /7/, 'ni el inverso')
})

test('TOKENS_TERMINAL: los 24 colores de la TUI, dos por token (día y noche), todos en minúsculas', () => {
  const colores = Object.keys(TOKENS_TERMINAL)
  assert.equal(colores.length, 24)
  assert.ok(colores.every((c) => /^#[0-9a-f]{6}$/.test(c)), 'hex en minúsculas')
  const porToken = Object.groupBy(colores, (c) => TOKENS_TERMINAL[c])
  assert.deepEqual(Object.keys(porToken).sort(), ['acento', 'aviso', 'antigravity', 'claude', 'codex', 'copilot', 'error', 'gemini', 'ok', 'suave', 'texto', 'velo'].sort())
  for (const [token, hexes] of Object.entries(porToken)) assert.equal(hexes?.length, 2, `${token}: un color de día y uno de noche`)
})

test('TERMINAL: Inicio, Mis APIs e Historial, con las teclas de la TUI', () => {
  assert.deepEqual(TERMINAL, [{ nombre: 'inicio', teclas: [] }, { nombre: 'apis', teclas: ['4'] }, { nombre: 'historial', teclas: ['5'] }])
})

test('terminal.json trae las seis pantallas completas, con clases de token, sin rutas del equipo y la fuente pesa poco', () => {
  const t = JSON.parse(readFileSync(new URL('../../site/capturas/terminal.json', import.meta.url), 'utf8')) as Record<string, Record<string, string>>
  assert.deepEqual(Object.keys(t), ['es', 'en'])
  const clases = new Set([...Object.values(TOKENS_TERMINAL).flatMap((x) => [`t-${x}`, `tf-${x}`]), 't-b', 't-d', 'g'])
  const privada = rutaPrivada(['C:\\Users\\Administrator', 'C:\\Users\\Administrator\\AppData', '/home/runner', '/tmp/x'])
  for (const idioma of ['es', 'en']) {
    assert.deepEqual(Object.keys(t[idioma]), TERMINAL.map((p) => p.nombre))
    for (const [pagina, html] of Object.entries(t[idioma])) {
      const donde = `${idioma}/${pagina}`
      assert.equal(html.replace(/<[^>]+>/g, '').split('\n').length, 28, `${donde}: 28 filas`)
      assert.ok(html.length < 12 * 1024, `${donde} pesa ${html.length} bytes`)
      assert.ok(!/style=|color:/.test(html), `${donde}: ni estilos en línea ni colores fijos`)
      assert.ok(!/Comprobando…|Checking…/.test(html), `${donde}: quedó comprobando`)
      assert.ok(!/\d+ peticiones en curso|1 petición en curso|requests? in progress/.test(html), `${donde}: con peticiones en curso`)
      assert.ok(!/[←-⇿■-➿]/u.test(html.replace(/<span class="[^"]*\bg\b[^"]*">.<\/span>/gu, '')), `${donde}: un glifo fuera de su span g`)
      assert.ok(!privada.test(html), `${donde}: una ruta del equipo`)
      for (const [, lista] of html.matchAll(/class="([^"]+)"/g)) for (const c of lista.split(' ')) assert.ok(clases.has(c), `${donde}: clase ${c}`)
    }
    assert.doesNotMatch(t[idioma].historial, /Aún no hay peticiones|No requests yet/, `${idioma}: el Historial tiene peticiones`)
  }
  assert.ok(statSync(new URL('../../site/cascadia.woff2', import.meta.url)).size <= 60 * 1024, 'cascadia.woff2 pasa de 60 KB')
  assert.match(readFileSync(new URL('../../site/CASCADIA-OFL.txt', import.meta.url), 'utf8'), /SIL OPEN FONT LICENSE Version 1\.1/)
})

// Hasta la Tarea 15, que borra estas imágenes, la portada todavía las usa: Inicio y Mis APIs.
test('las 16 capturas de la terminal de la portada actual existen y pesan poco', () => {
  const portada = new URL('../../site/capturas/', import.meta.url)
  let total = 0
  for (const p of TERMINAL.filter((p) => p.nombre !== 'historial'))
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const ancho of [960, 1440]) {
          const archivo = `terminal-${p.nombre}-${tema}-${idioma}-${ancho}.webp`
          const tamano = statSync(new URL(archivo, portada), { throwIfNoEntry: false })?.size
          assert.ok(tamano, `falta ${archivo}`)
          assert.ok(tamano <= 150 * 1024, `${archivo} pesa ${tamano} bytes`)
          total += tamano
        }
  assert.ok(total <= 2 * 1024 * 1024, `las de la terminal pesan ${total} bytes`)
})

test('cada pantalla de la portada existe en WebP, en claro y oscuro, a 640, 960 y 1440 px', () => {
  const portada = readdirSync(new URL('../../site/capturas/', import.meta.url))
  assert.ok(!portada.some((a) => a.endsWith('.png')), 'quedan capturas en PNG')
  for (const p of PORTADA)
    for (const tema of ['claro', 'oscuro'])
      for (const ancho of [640, 960, 1440]) assert.ok(portada.includes(`${p.nombre}-${tema}-${ancho}.webp`), `${p.nombre}-${tema}-${ancho}.webp`)
})
