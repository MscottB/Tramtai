import { test } from 'node:test'
import assert from 'node:assert/strict'
import { idioma, t } from './i18n.ts'
import { duracion, estadoInfo, fecha, haceCuanto } from './formato.ts'

idioma.value = 'es'
const hace = (segundos: number) => new Date(Date.now() - segundos * 1000).toISOString()

test('fecha usa el idioma de la Web UI y sin fecha da una raya', () => {
  const texto = '2026-01-02T03:04:05Z'
  assert.equal(fecha(texto), new Date(texto).toLocaleString('es'))
  idioma.value = 'en'
  assert.equal(fecha(texto), new Date(texto).toLocaleString('en'))
  idioma.value = 'es'
  assert.equal(fecha(), '—')
})

test('haceCuanto elige la unidad y pasada una semana da la fecha corta', () => {
  assert.equal(haceCuanto(), '—')
  assert.equal(haceCuanto(hace(30)), 'hace 30 segundos')
  assert.equal(haceCuanto(hace(5 * 60)), 'hace 5 minutos')
  assert.equal(haceCuanto(hace(2 * 3600)), 'hace 2 horas')
  assert.equal(haceCuanto(hace(86400)), 'ayer')
  const antigua = hace(10 * 86400)
  assert.equal(haceCuanto(antigua), new Date(antigua).toLocaleDateString('es'))
})

test('duracion en segundos con un decimal y, desde un minuto, en minutos y segundos', () => {
  assert.equal(duracion('2026-01-01T00:00:00Z'), '')
  assert.equal(duracion(undefined, '2026-01-01T00:00:00Z'), '')
  assert.equal(duracion('2026-01-01T00:00:00Z', '2026-01-01T00:00:12.340Z'), '12.3 s')
  assert.equal(duracion('2026-01-01T00:00:00Z', '2026-01-01T00:02:05Z'), '2 min 5 s')
})

test('estadoInfo da color, icono, nombre y clase; un estado desconocido se ve tal cual', () => {
  assert.deepEqual(estadoInfo('completado'),
    { color: 'positive', icono: 'check_circle', etiqueta: t('estado.completado'), clase: 'etiqueta etiqueta--ok' })
  assert.equal(estadoInfo('fallido').clase, 'etiqueta etiqueta--error')
  assert.equal(estadoInfo('en_cola').clase, 'etiqueta etiqueta--neutro')
  assert.deepEqual(estadoInfo('raro'), { color: 'grey-7', icono: 'help', etiqueta: 'raro', clase: 'etiqueta etiqueta--neutro' })
})
