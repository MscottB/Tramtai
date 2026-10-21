// La lógica de los recorridos guiados: qué pasos salen de los temas de la Ayuda, con temas de prueba (no los reales).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { ORDEN_BIENVENIDA, pasosBienvenida, pasosDePagina } from './recorrido.ts'
import type { Pagina, PasoRecorrido, Tema } from './ayuda/tipos.ts'

function tema(pagina: Pagina, pasos: PasoRecorrido[]): Tema {
  return {
    id: pagina, icono: 'x', titulo: 'comun.aceptar', resumen: 'comun.aceptar', paginaApp: pagina, conDiagrama: false,
    relacionados: [], bloques: pasos.length ? [{ tipo: 'recorrido', pasos }] : [],
  }
}
const paso = (ancla: string, bienvenida?: boolean): PasoRecorrido => ({ ancla, titulo: 'comun.aceptar', texto: 'comun.cancelar', bienvenida })

// Desordenados a propósito: el orden de la bienvenida lo manda ORDEN_BIENVENIDA, no el de los temas.
const temas: Tema[] = [
  tema('espacios', [paso('e1', true)]),
  tema('historial', [paso('h1', true)]),
  tema('agentes', [paso('a1', true), paso('a2'), paso('a3', true)]),
  tema('inicio', [paso('i1', true)]),
  tema('ejecutar', []),
  tema('conectar', [paso('c1'), paso('c2', true)]),
]

test('pasosDePagina devuelve los pasos de la página en orden', () => {
  assert.deepEqual(pasosDePagina('agentes', temas).map((p) => [p.pagina, p.ancla]), [['agentes', 'a1'], ['agentes', 'a2'], ['agentes', 'a3']])
  assert.equal(pasosDePagina('agentes', temas)[0].titulo, 'comun.aceptar')
  assert.equal(pasosDePagina('agentes', temas)[0].texto, 'comun.cancelar')
})

test('pasosDePagina devuelve [] si la página no tiene recorrido', () => {
  assert.deepEqual(pasosDePagina('ejecutar', temas), [])
  assert.deepEqual(pasosDePagina('configuracion', temas), [])
})

test('pasosBienvenida sigue ORDEN_BIENVENIDA, solo toma los marcados y cierra con el paso sin ancla', () => {
  const pasos = pasosBienvenida(temas)
  assert.deepEqual(pasos.map((p) => `${p.pagina}:${p.ancla}`), ['inicio:i1', 'agentes:a1', 'agentes:a3', 'conectar:c2', 'espacios:e1', 'espacios:undefined'])
  assert.deepEqual(pasos.at(-1), { pagina: 'espacios', titulo: 'recorrido.finTitulo', texto: 'recorrido.finTexto' })
  assert.ok(!pasos.some((p) => p.pagina === 'historial'), 'historial no está en la bienvenida')
})

test('ORDEN_BIENVENIDA no incluye historial ni ajustes', () => {
  assert.deepEqual(ORDEN_BIENVENIDA, ['inicio', 'agentes', 'ejecutar', 'conectar', 'espacios'])
})
