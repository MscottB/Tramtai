import { test } from 'node:test'
import assert from 'node:assert/strict'
import { sugerirApiPropia } from './claves.ts'

test('se sugiere una API propia con más de 20 peticiones de apps con la clave del espacio en 24 h', () => {
  assert.equal(sugerirApiPropia(0), false)
  assert.equal(sugerirApiPropia(20), false)
  assert.equal(sugerirApiPropia(21), true)
  assert.equal(sugerirApiPropia(undefined), false)
})
