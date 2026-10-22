import { test } from 'node:test'
import assert from 'node:assert/strict'
import { destinoDe, intencionPara } from './navegacion'

test('destinoDe: una dirección va sin intención y una intención lleva a su página', () => {
  assert.deepEqual(destinoDe('ayuda/archivos'), { id: 'ayuda/archivos', intencion: null })
  const ver = { pagina: 'historial', abrir: 'trabajo_1' } as const
  assert.deepEqual(destinoDe(ver), { id: 'historial', intencion: ver })
})

test('intencionPara: la página de destino recibe su intención y las demás no reciben nada', () => {
  const probar = { pagina: 'ejecutar', modelo: 'claude' } as const
  assert.deepEqual(intencionPara('ejecutar', probar), { intencion: probar })
  assert.deepEqual(intencionPara('historial', probar), {})
  assert.deepEqual(intencionPara('ejecutar', null), {})
})
