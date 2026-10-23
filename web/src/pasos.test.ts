import { test } from 'node:test'
import assert from 'node:assert/strict'
import { t } from './i18n.ts'
import { conTexto, guiaVisible } from './pasos.ts'

// Los Primeros pasos los calcula el servicio: la Web UI pinta los que llegan, con su estado y en su orden.
test('conTexto pinta los pasos que manda el servicio', () => {
  const pasos = conTexto([
    { pagina: 'agentes', hecho: true, listos: 2, total: 3 },
    { pagina: 'ejecutar', hecho: false, listos: 0, total: 0 },
    { pagina: 'otra', hecho: false, listos: 0, total: 0 },
    { pagina: 'configuracion', hecho: true, listos: 0, total: 0 },
  ], 'Windows')
  assert.deepEqual(pasos.map((p) => [p.pagina, p.hecho]), [['agentes', true], ['ejecutar', false], ['configuracion', true]])
  assert.equal(pasos[0].detalle, t('inicio.paso1Detalle', { listos: 2, total: 3 }))
  assert.equal(pasos[2].titulo, t('inicio.paso4', { sistema: 'Windows' }))
})

// Revisión final web (I2): si /instancia/pasos falla no llega ninguna lista, y la guía sigue a la vista (con su error y
// Reintentar) en lugar de esconderse como si estuviera completa.
test('la guía solo se esconde si la ocultaste o si llegaron pasos y están todos hechos', () => {
  const hecho = { pagina: 'agentes', hecho: true, listos: 1, total: 1 }
  assert.equal(guiaVisible(false, undefined), true, 'sin respuesta (cargando o con error)')
  assert.equal(guiaVisible(false, []), true, 'una lista vacía no es una guía completa')
  assert.equal(guiaVisible(false, [hecho, { ...hecho, pagina: 'ejecutar', hecho: false }]), true)
  assert.equal(guiaVisible(false, [hecho]), false)
  assert.equal(guiaVisible(true, undefined), false)
})
