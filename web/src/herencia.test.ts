import { test } from 'node:test'
import assert from 'node:assert/strict'
import { heredable, procedencia } from './herencia.ts'
import type { EntradaConfiguracion } from './api/tipos.ts'

const entradas: EntradaConfiguracion[] = [
  { clave: 'predeterminados.agente', valor: 'codex', origen: 'local' },
  { clave: 'predeterminados.perfil_modelo', valor: 'equilibrado', origen: 'proyecto' },
  { clave: 'retencion.horas_resultado', valor: 24, origen: 'global' },
  { clave: 'retencion.max_trabajos', valor: 1000, origen: 'interno' },
  { clave: 'presupuesto.diario.usd', valor: 5, origen: 'entorno' },
]

test('en un proyecto, cada valor dice si es propio o de Global', () => {
  assert.equal(procedencia(entradas, 'predeterminados.agente', 'proyecto'), 'propio')
  assert.equal(procedencia(entradas, 'predeterminados.perfil_modelo', 'proyecto'), 'propio')
  assert.equal(procedencia(entradas, 'retencion.horas_resultado', 'proyecto'), 'global')
  assert.equal(procedencia(entradas, 'retencion.max_trabajos', 'proyecto'), 'global')
  assert.equal(procedencia(entradas, 'presupuesto.diario.usd', 'proyecto'), undefined)
  assert.equal(procedencia(entradas, 'no.existe', 'proyecto'), undefined)
})

test('en la global no se marca nada', () => {
  assert.equal(procedencia(entradas, 'predeterminados.agente', 'global'), undefined)
})

test('solo vuelven al de Global los valores que la app deja tener propios', () => {
  assert.equal(heredable('predeterminados.agente'), true)
  assert.equal(heredable('retencion.horas_resultado'), true)
  assert.equal(heredable('presupuesto.diario.usd'), true)
  assert.equal(heredable('comandos_agentes.claude'), false)
})
