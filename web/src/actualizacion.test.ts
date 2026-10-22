import { test } from 'node:test'
import assert from 'node:assert/strict'
import { abandonarSinFase } from './actualizacion.ts'

test('sin fase después de haber visto una, la actualización terminó o falló en otra pestaña: se deja de seguir', () => {
  assert.equal(abandonarSinFase(true, 1), true)
})

test('sin fase al principio, el servicio aún consulta la versión: se sigue esperando un rato', () => {
  assert.equal(abandonarSinFase(false, 1), false)
  assert.equal(abandonarSinFase(false, 74), false)
})

test('sin fase durante 30 s seguidos (75 consultas de 400 ms), la consulta falló: se deja de seguir', () => {
  assert.equal(abandonarSinFase(false, 75), true)
})
