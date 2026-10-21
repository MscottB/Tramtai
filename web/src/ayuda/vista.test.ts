import { test } from 'node:test'
import assert from 'node:assert/strict'
import { altDiagrama, bloquesVisibles, capturaDeAncla, varianteCaptura } from './vista.ts'
import type { Tema } from './tipos.ts'

test('la captura sigue el tema y el idioma', () => {
  assert.equal(varianteCaptura('inicio', true, 'en'), 'inicio.oscuro.en.webp')
  assert.equal(varianteCaptura('inicio', false, 'es'), 'inicio.claro.es.webp')
})

test('la captura de un paso se llama como su ancla, con guiones', () => {
  assert.equal(capturaDeAncla('inicio.mapa'), 'recorrido-inicio-mapa')
  assert.equal(capturaDeAncla('conectar.clave-espacio'), 'recorrido-conectar-clave-espacio')
})

test('el texto alternativo del diagrama nombra las paradas en orden', () => {
  assert.equal(altDiagrama(['A', 'B', 'C']), '1. A → 2. B → 3. C')
})

const tema: Tema = {
  id: 'x', icono: '', titulo: 'ayuda.empezar.titulo', resumen: 'ayuda.empezar.resumen', conDiagrama: false, relacionados: [],
  bloques: [{ tipo: 'descargas' }, { tipo: 'nota', texto: 'ayuda.empezar.nota' }, { tipo: 'ir', pagina: 'agentes' }],
}

test('qué bloques se ven en la app, en el panel y en el sitio', () => {
  assert.deepEqual(bloquesVisibles(tema, 'app').map((b) => b.tipo), ['nota', 'ir'])
  assert.deepEqual(bloquesVisibles(tema, 'panel', 'agentes').map((b) => b.tipo), ['nota'])
  assert.deepEqual(bloquesVisibles(tema, 'panel', 'inicio').map((b) => b.tipo), ['nota', 'ir'])
  assert.deepEqual(bloquesVisibles(tema, 'sitio').map((b) => b.tipo), ['descargas', 'nota'])
})

test('el recorrido se ve en la app, en el panel y en el sitio', () => {
  const conRecorrido: Tema = { ...tema, bloques: [{ tipo: 'recorrido', pasos: [{ ancla: 'a', titulo: 'ayuda.empezar.titulo', texto: 'ayuda.empezar.resumen' }] }] }
  for (const contexto of ['app', 'panel', 'sitio'] as const) assert.deepEqual(bloquesVisibles(conRecorrido, contexto, 'agentes').map((b) => b.tipo), ['recorrido'], contexto)
})
