import { test } from 'node:test'
import assert from 'node:assert/strict'
import { ref } from 'vue'
import { recordarBorrador } from './borrador.ts'

const almacen = () => {
  const m = new Map<string, string>()
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v), removeItem: (k: string) => void m.delete(k), m }
}

test('lo escrito sobrevive a una recarga y se borra al vaciarlo', () => {
  const a = almacen()
  const antes = ref('')
  recordarBorrador('b', antes, a)
  antes.value = 'resume esto'
  const despues = ref('')
  recordarBorrador('b', despues, a)
  assert.equal(despues.value, 'resume esto', 'tras recargar vuelve lo escrito')
  despues.value = ''
  assert.equal(a.m.has('b'), false)
})

test('sin almacén (o si falla) el campo funciona igual', () => {
  const v = ref('hola')
  recordarBorrador('b', v, undefined)
  v.value = 'otra'
  assert.equal(v.value, 'otra')
  const roto = { getItem: () => { throw new Error('x') }, setItem: () => { throw new Error('x') }, removeItem: () => {} }
  recordarBorrador('b', v, roto)
  v.value = 'más'
  assert.equal(v.value, 'más')
})
