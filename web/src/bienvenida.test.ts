// Cuándo se ofrece la bienvenida: solo con sesión, en Inicio, sin enlace profundo, sin respuesta guardada y una vez por carga.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { entradaPorEnlace, ofrecerBienvenida } from './bienvenida.ts'

const limpio = { sesion: true, pagina: 'inicio', entradaPorEnlace: false, estado: null, yaOfrecida: false, otroDialogo: false, recorridoActivo: false } as const

test('ofrecerBienvenida: el caso limpio la ofrece', () => {
  assert.equal(ofrecerBienvenida(limpio), true)
})

test('ofrecerBienvenida: cada condición que falta la impide', () => {
  assert.equal(ofrecerBienvenida({ ...limpio, sesion: false }), false, 'sin sesión')
  assert.equal(ofrecerBienvenida({ ...limpio, pagina: 'agentes' }), false, 'otra página')
  assert.equal(ofrecerBienvenida({ ...limpio, entradaPorEnlace: true }), false, 'enlace profundo')
  assert.equal(ofrecerBienvenida({ ...limpio, estado: 'hecha' }), false, 'ya hecha')
  assert.equal(ofrecerBienvenida({ ...limpio, estado: 'descartada' }), false, 'descartada')
  assert.equal(ofrecerBienvenida({ ...limpio, yaOfrecida: true }), false, 'ya ofrecida en esta carga')
  assert.equal(ofrecerBienvenida({ ...limpio, otroDialogo: true }), false, 'otro diálogo abierto (Novedades, Actualizar…)')
})

test('ofrecerBienvenida: con un recorrido en marcha no se ofrece, y al acabar sí (un recorrido de página que cruza Inicio)', () => {
  assert.equal(ofrecerBienvenida({ ...limpio, recorridoActivo: true }), false)
  assert.equal(ofrecerBienvenida({ ...limpio, recorridoActivo: false }), true)
})

test('ofrecerBienvenida: al cerrarse el otro diálogo se ofrece', () => {
  assert.equal(ofrecerBienvenida({ ...limpio, otroDialogo: true }), false)
  assert.equal(ofrecerBienvenida({ ...limpio, otroDialogo: false }), true)
})

test('entradaPorEnlace: solo un hash que no sea vacío ni #inicio es un enlace profundo', () => {
  for (const hash of ['', '#', '#inicio']) assert.equal(entradaPorEnlace(hash), false, hash)
  for (const hash of ['#agentes', '#ayuda/archivos', '#conectar']) assert.equal(entradaPorEnlace(hash), true, hash)
})
