// El menú lateral va por grupos, en el orden en que se usa la app.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { GRUPOS_MENU, ordenMenu } from './menu.ts'
import { ORDEN_BIENVENIDA } from './recorrido.ts'
import { PAGINAS } from './ayuda/tipos.ts'

test('cada página sale exactamente una vez en el menú', () => {
  const orden = ordenMenu()
  assert.equal(orden.length, PAGINAS.length)
  assert.deepEqual([...orden].sort(), [...PAGINAS].sort())
})

test('el menú tiene tres grupos, en el orden de uso', () => {
  assert.deepEqual(GRUPOS_MENU.map((g) => g.grupo), ['empezar', 'conectar', 'organizar'])
  assert.deepEqual(ordenMenu(), ['inicio', 'agentes', 'ejecutar', 'conectar', 'historial', 'espacios', 'configuracion'])
})

test('el orden de la bienvenida es una subsecuencia del menú', () => {
  const orden = ordenMenu()
  const posiciones = ORDEN_BIENVENIDA.map((p) => orden.indexOf(p))
  assert.ok(posiciones.every((i) => i >= 0))
  assert.deepEqual(posiciones, [...posiciones].sort((a, b) => a - b))
})
