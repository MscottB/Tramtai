import { test } from 'node:test'
import assert from 'node:assert/strict'
import { hashDe, leerHash } from './ruta.ts'
import { temas } from './indice.ts'

test('un tema de la Ayuda tiene su dirección', () => {
  assert.deepEqual(leerHash('#ayuda/archivos'), { pagina: 'ayuda', tema: 'archivos' })
})

test('un tema que no existe, o vacío, abre el inicio de la Ayuda', () => {
  assert.deepEqual(leerHash('#ayuda/noexiste'), { pagina: 'ayuda' })
  assert.deepEqual(leerHash('#ayuda/'), { pagina: 'ayuda' })
  assert.deepEqual(leerHash('#ayuda'), { pagina: 'ayuda' })
})

test('las direcciones viejas y las desconocidas', () => {
  assert.deepEqual(leerHash('#panel'), { pagina: 'inicio' })
  assert.deepEqual(leerHash('#trabajos'), { pagina: 'historial' })
  assert.deepEqual(leerHash('#otra'), { pagina: 'inicio' })
  assert.deepEqual(leerHash(''), { pagina: 'inicio' })
  assert.deepEqual(leerHash('#agentes/archivos'), { pagina: 'agentes' })
})

test('ida y vuelta para los 14 temas', () => {
  for (const t of temas) assert.deepEqual(leerHash(hashDe('ayuda', t.id)), { pagina: 'ayuda', tema: t.id })
  assert.equal(hashDe('conectar'), '#conectar')
})
