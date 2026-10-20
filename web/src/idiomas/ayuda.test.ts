import { test } from 'node:test'
import assert from 'node:assert/strict'
import ayudaEs from './ayuda.es.ts'
import ayudaEn from './ayuda.en.ts'
import es from './es.ts'

test('los textos de la Ayuda viven en su archivo y en los dos idiomas', () => {
  assert.deepEqual(Object.keys(ayudaEn).sort(), Object.keys(ayudaEs).sort())
  assert.ok(Object.keys(ayudaEs).every((k) => k.startsWith('ayuda.')))
  assert.equal(Object.keys(es).filter((k) => k.startsWith('ayuda.')).length, Object.keys(ayudaEs).length)
})
