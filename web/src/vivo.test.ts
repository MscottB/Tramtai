import { test } from 'node:test'
import assert from 'node:assert/strict'
import { aplicar, claveEtapa, esperaReconexion, eventoDe, limpiar, podar, posicion, reconciliar, visibles, type EventoTrabajo, type Tranvias } from './vivo.ts'
import type { Trabajo } from './api/tipos.ts'

const ev = (o: Partial<EventoTrabajo>): EventoTrabajo => ({ id: 'a', estado: 'en_cola', etapa: 'cola', agente: 'claude', modelo: 'claude/sonnet',
  api: '', origen: 'probar', intento: 1, codigo_error: '', creado: '', actualizado: '', ...o })

test('alta y cambio de etapa', () => {
  const t: Tranvias = new Map()
  const borrados = new Set<string>()
  aplicar(t, ev({}), 1000, borrados)
  assert.deepEqual(posicion(t.get('a')!), { tramo: 'entrada', avance: 0.5 })
  aplicar(t, ev({ estado: 'preparando_entrada', etapa: 'preparacion' }), 1100, borrados)
  assert.deepEqual(posicion(t.get('a')!), { tramo: 'centro', avance: 0 })
  aplicar(t, ev({ estado: 'ejecutando', etapa: 'ejecucion' }), 1200, borrados)
  assert.deepEqual(posicion(t.get('a')!), { tramo: 'linea', avance: 0.45 })
  aplicar(t, ev({ estado: 'validando_salida', etapa: 'validacion' }), 1300, borrados)
  assert.deepEqual(posicion(t.get('a')!), { tramo: 'linea', avance: 0.85 })
  aplicar(t, ev({ estado: 'completado', etapa: 'llegada' }), 1400, borrados)
  assert.deepEqual(posicion(t.get('a')!), { tramo: 'linea', avance: 1 })
  assert.equal(t.get('a')!.desde, 1000)
  assert.equal(t.get('a')!.terminadoEn, 1400)
})

test('el respaldo cambia de línea', () => {
  const t: Tranvias = new Map()
  aplicar(t, ev({ estado: 'ejecutando', etapa: 'ejecucion' }), 0, new Set())
  aplicar(t, ev({ estado: 'esperando_agente', etapa: 'preparacion', agente: 'codex', intento: 2 }), 10, new Set())
  assert.equal(t.get('a')!.agente, 'codex')
  assert.equal(t.get('a')!.intento, 2)
  assert.deepEqual(posicion(t.get('a')!), { tramo: 'centro', avance: 0 })
})

test('un fallo queda donde iba', () => {
  const t: Tranvias = new Map()
  aplicar(t, ev({ estado: 'ejecutando', etapa: 'ejecucion' }), 0, new Set())
  aplicar(t, ev({ estado: 'fallido', etapa: 'llegada', codigo_error: 'error_agente' }), 10, new Set())
  assert.deepEqual(posicion(t.get('a')!), { tramo: 'linea', avance: 0.45 })
})

test('limpiar borra llegadas a los 2 s y fallos a los 4 s', () => {
  const t: Tranvias = new Map()
  const borrados = new Set<string>()
  aplicar(t, ev({ id: 'ok', estado: 'completado', etapa: 'llegada' }), 0, borrados)
  aplicar(t, ev({ id: 'mal', estado: 'fallido', etapa: 'llegada' }), 0, borrados)
  aplicar(t, ev({ id: 'vivo', estado: 'ejecutando', etapa: 'ejecucion' }), 0, borrados)
  limpiar(t, 1999, borrados)
  assert.equal(t.size, 3)
  limpiar(t, 2000, borrados)
  assert.deepEqual([...t.keys()].sort(), ['mal', 'vivo'])
  limpiar(t, 3999, borrados)
  assert.equal(t.has('mal'), true)
  limpiar(t, 4000, borrados)
  assert.deepEqual([...t.keys()], ['vivo'])
  assert.deepEqual([...borrados].sort(), ['mal', 'ok'])
})

test('un evento tardío de uno ya borrado no vuelve', () => {
  const t: Tranvias = new Map()
  const borrados = new Set(['ok'])
  aplicar(t, ev({ id: 'ok', estado: 'completado', etapa: 'llegada' }), 5000, borrados)
  assert.equal(t.size, 0)
})

test('como mucho 12 visibles, los más recientes, y el resto', () => {
  const t: Tranvias = new Map()
  for (let i = 0; i < 15; i++) aplicar(t, ev({ id: 't' + i }), i, new Set())
  const { lista, resto } = visibles(t)
  assert.equal(lista.length, 12)
  assert.equal(resto, 3)
  assert.equal(lista[0].id, 't14')
  assert.ok(!lista.some((x) => x.id === 't0'))
})

test('esperas de reconexión', () => {
  assert.deepEqual([0, 1, 2, 3, 4, 5, 6, 7].map(esperaReconexion), [1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000])
})

test('desde es cuándo se aceptó la petición, no cuándo apareció en el mapa', () => {
  const t: Tranvias = new Map()
  aplicar(t, ev({ estado: 'ejecutando', etapa: 'ejecucion', creado: '2026-10-20T12:00:00Z' }), Date.parse('2026-10-20T12:02:00Z'), new Set())
  assert.equal(t.get('a')!.desde, Date.parse('2026-10-20T12:00:00Z'))
})

test('podar quita los que la foto no trajo y no habían llegado', () => {
  const t: Tranvias = new Map()
  const borrados = new Set<string>()
  aplicar(t, ev({ id: 'sigue', estado: 'ejecutando', etapa: 'ejecucion' }), 0, borrados)
  aplicar(t, ev({ id: 'perdido', estado: 'ejecutando', etapa: 'ejecucion' }), 0, borrados)
  aplicar(t, ev({ id: 'llegando', estado: 'completado', etapa: 'llegada' }), 0, borrados)
  podar(t, new Set(['sigue']))
  assert.deepEqual([...t.keys()].sort(), ['llegando', 'sigue'])
})

test('reconciliar con la consulta: el que termina llega, el que falta se va y los terminados nuevos no aparecen', () => {
  const t: Tranvias = new Map()
  const borrados = new Set<string>()
  aplicar(t, ev({ id: 'termina', estado: 'ejecutando', etapa: 'ejecucion' }), 0, borrados)
  aplicar(t, ev({ id: 'falta', estado: 'ejecutando', etapa: 'ejecucion' }), 0, borrados)
  reconciliar(t, [ev({ id: 'termina', estado: 'completado', etapa: 'llegada' }), ev({ id: 'viejo', estado: 'completado', etapa: 'llegada' }),
    ev({ id: 'nuevo', estado: 'en_cola', etapa: 'cola' })], 100, borrados)
  assert.deepEqual([...t.keys()].sort(), ['nuevo', 'termina'])
  assert.equal(t.get('termina')!.terminadoEn, 100)
  limpiar(t, 2100, borrados)
  assert.deepEqual([...t.keys()], ['nuevo'])
})

test('eventoDe convierte un trabajo de la consulta, con la etapa que manda el servicio', () => {
  const trabajo = { id: 'x', estado: 'fallido', etapa: 'llegada', solicitud: { agente: 'codex' }, modelo_efectivo: 'codex/gpt', punto_acceso: 'otra', origen: 'api',
    intento: 2, codigo_error: 'error_agente', creado_en: '2026-10-20T12:00:00Z' } as unknown as Trabajo
  assert.deepEqual(eventoDe(trabajo), { id: 'x', estado: 'fallido', etapa: 'llegada', agente: 'codex', modelo: 'codex/gpt', api: 'otra', origen: 'api',
    intento: 2, codigo_error: 'error_agente', creado: '2026-10-20T12:00:00Z', actualizado: '' })
})

test('la llegada se nombra según cómo terminó', () => {
  const t: Tranvias = new Map()
  for (const estado of ['completado', 'fallido', 'interrumpido', 'cancelado']) aplicar(t, ev({ id: estado, estado, etapa: 'llegada' }), 0, new Set())
  aplicar(t, ev({ id: 'va', estado: 'ejecutando', etapa: 'ejecucion' }), 0, new Set())
  assert.deepEqual(['completado', 'fallido', 'interrumpido', 'cancelado', 'va'].map((id) => claveEtapa(t.get(id)!)),
    ['vivo.etapaLlegada', 'vivo.etapaFallida', 'vivo.etapaFallida', 'vivo.etapaCancelada', 'vivo.etapaEjecucion'])
})
