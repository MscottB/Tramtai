import { test } from 'node:test'
import assert from 'node:assert/strict'
import { idioma, t } from './i18n.ts'
import { configDeReceta, esquemaDeReceta, recetaDePunto, recetaDeSolicitud, recetaVacia } from './recetas.ts'
import type { EstadoPunto } from './api/tipos.ts'

idioma.value = 'es'
const punto = (p: Partial<EstadoPunto>): EstadoPunto => ({ nombre: 'facturas', instancia: 'i', contratos: ['openai'], activo: true, origen: 'ui', ...p })
const esquema = { type: 'object', properties: { total: { type: 'number' } } }

test('receta de una API: modelo como id del catálogo, esquema como texto y copia del respaldo', () => {
  const respaldo = ['codex+alto']
  const r = recetaDePunto(punto({ agente: 'claude', modelo: 'sonnet', forzar_modelo: true, esfuerzo: 'medio', respaldo, esquema, reintentos_salida: 2,
    instrucciones: 'Extrae el total' }))
  assert.deepEqual(r, { instrucciones: 'Extrae el total', modelo: 'claude/sonnet', forzar: true, esfuerzo: 'medio', respaldo: ['codex+alto'],
    esquema: JSON.stringify(esquema, null, 2), reintentos: 2 })
  assert.notEqual(r.respaldo, respaldo)
  assert.equal(recetaDePunto(punto({ perfil_modelo: 'rapido', agente: 'claude' })).modelo, 'perfil/rapido')
  assert.deepEqual(recetaDePunto(punto({})), recetaVacia())
})

test('receta de una petición enviada: empieza por el agente de la solicitud y nunca fuerza el modelo', () => {
  assert.deepEqual(recetaDeSolicitud({ agente: 'codex', modelo: 'gpt-5', esfuerzo: 'alto', instrucciones: 'Resume', respaldo: ['claude'],
    salida: { esquema, reintentos: 1 } }),
  { instrucciones: 'Resume', forzar: false, esfuerzo: 'alto', respaldo: ['claude'], modelo: 'codex/gpt-5', esquema: JSON.stringify(esquema, null, 2), reintentos: 1 })
  assert.equal(recetaDeSolicitud({ agente: 'codex' }).modelo, 'codex')
  assert.equal(recetaDeSolicitud({ perfil_modelo: 'profundo' }).modelo, 'perfil/profundo')
  assert.deepEqual(recetaDeSolicitud({}), recetaVacia())
})

test('esquema de una receta: vacío no hay; texto que no es un objeto JSON da un error legible', () => {
  assert.equal(esquemaDeReceta({ ...recetaVacia(), esquema: '  ' }), undefined)
  assert.deepEqual(esquemaDeReceta({ ...recetaVacia(), esquema: JSON.stringify(esquema) }), esquema)
  for (const malo of ['{', '[]', 'null', '3']) {
    assert.throws(() => esquemaDeReceta({ ...recetaVacia(), esquema: malo }), { message: t('receta.esquemaInvalido') }, malo)
  }
})

test('config de una receta: solo lo que tiene valor, forzar con modelo y reintentos con esquema', () => {
  assert.deepEqual(configDeReceta({ instrucciones: '  Resume  ', modelo: 'claude/sonnet', forzar: true, esfuerzo: 'bajo', respaldo: ['codex'],
    esquema: JSON.stringify(esquema), reintentos: 2 }),
  { instrucciones: 'Resume', agente: 'claude', modelo: 'sonnet', perfil_modelo: undefined, forzar_modelo: true, esfuerzo: 'bajo', respaldo: ['codex'],
    esquema, reintentos_salida: 2 })
  const sinNada = configDeReceta({ ...recetaVacia(), forzar: true, reintentos: 3 })
  assert.deepEqual(sinNada, { instrucciones: undefined, agente: undefined, modelo: undefined, perfil_modelo: undefined, forzar_modelo: undefined,
    esfuerzo: undefined, respaldo: undefined, esquema: undefined, reintentos_salida: undefined })
  assert.equal(configDeReceta({ ...recetaVacia(), modelo: 'perfil/rapido' }).perfil_modelo, 'rapido')
})
