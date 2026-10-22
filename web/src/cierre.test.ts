import { test } from 'node:test'
import assert from 'node:assert/strict'
import { escaparConAviso } from './cierre.ts'

test('Esc cierra el diálogo sin preguntar si no se pierde nada', () => {
  let cerrado = 0
  escaparConAviso(false, () => cerrado++)
  assert.equal(cerrado, 1)
})
