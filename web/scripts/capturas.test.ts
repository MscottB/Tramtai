import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, statSync } from 'node:fs'
import { temas } from '../src/ayuda/indice.ts'
import { capturaDeAncla } from '../src/ayuda/vista.ts'
import { ANCLAS, CAPTURAS, PORTADA, SIN_CAPTURA } from './capturas-lista.ts'

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
  for (const ancla of anclas)
    for (const v of variantes(capturaDeAncla(ancla)))
      if (!(ancla in SIN_CAPTURA)) assert.ok(archivos.includes(v), `${v} (si la demo no la muestra, explícalo en SIN_CAPTURA)`)
      else assert.ok(!archivos.includes(v), `${v} sobra: ya no está en SIN_CAPTURA`)
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

test('cada pantalla de la portada existe en WebP, en claro y oscuro, a 960 y 1440 px', () => {
  const portada = readdirSync(new URL('../../site/capturas/', import.meta.url))
  assert.ok(!portada.some((a) => a.endsWith('.png')), 'quedan capturas en PNG')
  for (const p of PORTADA)
    for (const tema of ['claro', 'oscuro'])
      for (const ancho of [960, 1440]) assert.ok(portada.includes(`${p.nombre}-${tema}-${ancho}.webp`), `${p.nombre}-${tema}-${ancho}.webp`)
})
