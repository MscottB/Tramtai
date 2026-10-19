import { test } from 'node:test'
import assert from 'node:assert/strict'
import { compartida } from './compartida.ts'

test('cinco llamadas a la vez ejecutan f una sola vez y reciben lo mismo', async () => {
  let llamadas = 0
  const sesion = compartida(async () => {
    llamadas++
    await new Promise((listo) => setTimeout(listo, 10))
    return 'csrf-' + llamadas
  })
  const resultados = await Promise.all([1, 2, 3, 4, 5].map(() => sesion()))
  assert.equal(llamadas, 1)
  assert.deepEqual(new Set(resultados), new Set(['csrf-1']))
  assert.equal(await sesion(), 'csrf-2', 'cuando termina, la siguiente vuelve a llamar')
})

test('un fallo no deja la promesa atascada', async () => {
  let llamadas = 0
  const f = compartida(async () => {
    llamadas++
    if (llamadas === 1) throw new Error('caída')
    return 'bien'
  })
  await assert.rejects(f())
  assert.equal(await f(), 'bien')
})
