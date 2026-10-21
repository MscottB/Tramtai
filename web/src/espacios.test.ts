import { test } from 'node:test'
import assert from 'node:assert/strict'
import { accionesDe, nombreConfirmado, ordenar } from './espacios.ts'
import type { EspacioUI } from './api/tipos.ts'

const e = (o: Partial<EspacioUI>): EspacioUI => ({ id: 'x', ambito: 'proyecto', nombre: 'X', activo: true, estado: 'activo', confiado: true,
  apis: 0, peticiones: 0, ...o })

test('acciones según el estado del espacio', () => {
  assert.deepEqual(accionesDe(e({ id: 'global', ambito: 'global', nombre: 'Global' }), 'global'), ['detener'])
  assert.deepEqual(accionesDe(e({ id: 'a' }), 'global'), ['abrir', 'detener', 'dejarDeConfiar'])
  assert.deepEqual(accionesDe(e({ id: 'a', activo: false, estado: 'detenido', confiado: false }), 'global'), ['activar', 'confiar', 'olvidar'])
  assert.deepEqual(accionesDe(e({ id: 'a', activo: false, estado: 'no_encontrado', confiado: false }), 'global'), ['buscar', 'olvidar'])
  assert.ok(!accionesDe(e({ id: 'global', ambito: 'global', activo: false, estado: 'detenido' }), 'otro').includes('olvidar'), 'la global no se olvida')
})

test('orden: la global, los activos por último uso y el resto por nombre', () => {
  const lista = ordenar([
    e({ id: 'z', nombre: 'Zeta', activo: false, estado: 'detenido' }),
    e({ id: 'v', nombre: 'Viejo', ultimo_uso: '2026-10-20T10:00:00Z' }),
    e({ id: 'global', ambito: 'global', nombre: 'Global' }),
    e({ id: 'n', nombre: 'Nuevo', ultimo_uso: '2026-10-20T12:00:00Z' }),
    e({ id: 'a', nombre: 'Alfa', activo: false, estado: 'no_encontrado' }),
  ])
  assert.deepEqual(lista.map((x) => x.id), ['global', 'n', 'v', 'a', 'z'])
})

test('confirmar el nombre al olvidar', () => {
  assert.equal(nombreConfirmado(e({ nombre: 'Demo App' }), ' demo app '), true)
  assert.equal(nombreConfirmado(e({ nombre: 'Demo App' }), 'Demo'), false)
})
