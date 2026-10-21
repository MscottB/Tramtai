import { test } from 'node:test'
import assert from 'node:assert/strict'
import { temas } from './ayuda/indice'
import { pestanaDeAncla } from './pestanasAjustes'

const anclas = temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'recorrido' ? b.pasos.flatMap((p) => (p.ancla ? [p.ancla] : [])) : [])))

test('las anclas de lo de este espacio piden la pestaña «espacio»', () => {
  assert.equal(pestanaDeAncla('configuracion.modelo'), 'espacio')
  assert.equal(pestanaDeAncla('configuracion.datos'), 'espacio')
})

test('las pestañas mismas y las anclas de otras páginas no piden ninguna pestaña', () => {
  assert.equal(pestanaDeAncla('configuracion.pestanas'), undefined)
  assert.equal(pestanaDeAncla('inicio.mapa'), undefined)
  assert.equal(pestanaDeAncla(undefined), undefined)
})

test('todas las anclas de Ajustes salvo las pestañas tienen su pestaña, así un paso nuevo no queda oculto', () => {
  const deAjustes = anclas.filter((a) => a.startsWith('configuracion.') && a !== 'configuracion.pestanas')
  assert.ok(deAjustes.length >= 2)
  for (const a of deAjustes) assert.ok(pestanaDeAncla(a), `${a} no dice en qué pestaña está`)
})
