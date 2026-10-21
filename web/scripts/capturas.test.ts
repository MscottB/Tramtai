import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, statSync } from 'node:fs'
import { temas } from '../src/ayuda/indice.ts'
import { CAPTURAS, PORTADA } from './capturas-lista.ts'

const carpeta = new URL('../src/ayuda/capturas/', import.meta.url)
const citadas = new Set(temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'captura' ? [b.id] : []))))
const archivos = readdirSync(carpeta).filter((a) => a.endsWith('.webp'))

test('el script saca justo las capturas que citan los temas', () => {
  assert.deepEqual(CAPTURAS.map((c) => c.id).sort(), [...citadas].sort())
  assert.deepEqual(PORTADA.map((p) => p.nombre).sort(), ['agentes', 'espacios', 'inicio', 'mis-apis', 'probar'])
})

test('cada captura citada existe en claro y oscuro y en los dos idiomas', () => {
  for (const id of citadas)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en']) assert.ok(archivos.includes(`${id}.${tema}.${idioma}.webp`), `${id}.${tema}.${idioma}.webp`)
})

test('no hay capturas que nadie cite y entre todas no pasan de 3 MB', () => {
  for (const a of archivos) assert.ok(citadas.has(a.split('.')[0]), `${a} no la cita ningún tema`)
  const total = archivos.reduce((s, a) => s + statSync(new URL(a, carpeta)).size, 0)
  assert.ok(total <= 3 * 1024 * 1024, `las capturas pesan ${total} bytes`)
})

test('cada pantalla de la portada existe en WebP, en claro y oscuro, a 960 y 1440 px', () => {
  const portada = readdirSync(new URL('../../site/capturas/', import.meta.url))
  assert.ok(!portada.some((a) => a.endsWith('.png')), 'quedan capturas en PNG')
  for (const p of PORTADA)
    for (const tema of ['claro', 'oscuro'])
      for (const ancho of [960, 1440]) assert.ok(portada.includes(`${p.nombre}-${tema}-${ancho}.webp`), `${p.nombre}-${tema}-${ancho}.webp`)
})
