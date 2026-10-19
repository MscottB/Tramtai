import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buscarTexto } from './textos.ts'

test('singular, plural y clave que no existe', () => {
  const textos = { 'a.peticiones': '{n} peticiones', 'a.peticionesUno': '1 petición' }
  assert.equal(buscarTexto(textos, 'a.peticiones', 1), '1 petición')
  assert.equal(buscarTexto(textos, 'a.peticiones', 3), '{n} peticiones')
  assert.equal(buscarTexto(textos, 'a.noExiste'), 'a.noExiste')
})
