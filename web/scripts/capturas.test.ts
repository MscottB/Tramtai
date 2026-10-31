import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, statSync } from 'node:fs'
import { temas } from '../src/ayuda/indice.ts'
import { capturaDeAncla } from '../src/ayuda/vista.ts'
import { readFileSync } from 'node:fs'
import { ANCHO_AYUDA, ANCLAS, CAPTURAS, ansiAHtml, medidas, PORTADA, revisarAnclas, rutaPrivada, SIN_CAPTURA, TERMINAL } from './capturas-lista.ts'

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
  for (const id of citadas) for (const v of variantes(id)) assert.ok(archivos.includes(v), v)
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

// Tarea 14 de la v0.56, hecha en la v0.58: la terminal se pinta en ANSI y se convierte a HTML para la captura.
test('ansiAHtml: un color, negrita, restablecer, 256 colores, inverso y lo que no es color', () => {
  const tema = { fondo: '#ffffff', texto: '#0f1b1e' }
  assert.equal(ansiAHtml('a\x1b[1;38;2;11;110;100mb<\x1b[0mc', tema), 'a<span style="color:#0b6e64;font-weight:bold">b&lt;</span>c')
  assert.equal(ansiAHtml('\x1b[38;5;196mx\x1b[m', tema), '<span style="color:#ff0000">x</span>')
  assert.equal(ansiAHtml('\x1b[7my\x1b[27mz', tema), '<span style="color:#ffffff;background:#0f1b1e">y</span>z')
  assert.equal(ansiAHtml('\x1b[2Ku\x1b]8;;https://x\x1b\\v\x1b]8;;\x1b\\', tema), 'uv', 'sin borrados de línea ni enlaces OSC 8')
})

test('las 16 capturas de la terminal existen y pesan poco', () => {
  const portada = new URL('../../site/capturas/', import.meta.url)
  let total = 0
  for (const p of TERMINAL)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const ancho of [960, 1440]) {
          const archivo = `terminal-${p.nombre}-${tema}-${idioma}-${ancho}.webp`
          const tamano = statSync(new URL(archivo, portada), { throwIfNoEntry: false })?.size
          assert.ok(tamano, `falta ${archivo}`)
          assert.ok(tamano <= 150 * 1024, `${archivo} pesa ${tamano} bytes`)
          total += tamano
        }
  assert.deepEqual(TERMINAL.map((p) => p.nombre), ['inicio', 'apis'])
  assert.ok(total <= 2 * 1024 * 1024, `las de la terminal pesan ${total} bytes`)
})

test('cada pantalla de la portada existe en WebP, en claro y oscuro, a 640, 960 y 1440 px', () => {
  const portada = readdirSync(new URL('../../site/capturas/', import.meta.url))
  assert.ok(!portada.some((a) => a.endsWith('.png')), 'quedan capturas en PNG')
  for (const p of PORTADA)
    for (const tema of ['claro', 'oscuro'])
      for (const ancho of [640, 960, 1440]) assert.ok(portada.includes(`${p.nombre}-${tema}-${ancho}.webp`), `${p.nombre}-${tema}-${ancho}.webp`)
})
