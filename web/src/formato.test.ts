import { test } from 'node:test'
import assert from 'node:assert/strict'
import { idioma, t } from './i18n.ts'
import { costo, decimal, duracion, enRango, estadoInfo, fecha, haceCuanto, registroCubre } from './formato.ts'

idioma.value = 'es'
const hace = (segundos: number) => new Date(Date.now() - segundos * 1000).toISOString()

test('fecha usa el idioma de la Web UI y sin fecha da «Sin dato»', () => {
  const texto = '2026-01-02T03:04:05Z'
  assert.equal(fecha(texto), new Date(texto).toLocaleString('es'))
  idioma.value = 'en'
  assert.equal(fecha(texto), new Date(texto).toLocaleString('en'))
  idioma.value = 'es'
  assert.equal(fecha(), 'Sin dato')
})

test('haceCuanto elige la unidad y pasada una semana da la fecha corta', () => {
  assert.equal(haceCuanto(), 'Sin dato')
  assert.equal(haceCuanto(hace(30)), 'hace 30 segundos')
  assert.equal(haceCuanto(hace(5 * 60)), 'hace 5 minutos')
  assert.equal(haceCuanto(hace(2 * 3600)), 'hace 2 horas')
  assert.equal(haceCuanto(hace(86400)), 'ayer')
  const antigua = hace(10 * 86400)
  assert.equal(haceCuanto(antigua), new Date(antigua).toLocaleDateString('es'))
})

test('enRango acepta números dentro de los límites (enteros si se piden) y si no da el mensaje con los límites', () => {
  const horas = enRango(0, 720, true)
  assert.equal(horas(0), true)
  assert.equal(horas(720), true)
  assert.equal(horas(721), 'Escribe un número entre 0 y 720.')
  assert.equal(horas(1.5), 'Escribe un número entre 0 y 720.')
  assert.equal(horas(Number.NaN), 'Escribe un número entre 0 y 720.')
  assert.equal(horas(''), 'Escribe un número entre 0 y 720.')
  assert.equal(enRango(0, 90, false)(1.25), true)
  assert.equal(enRango(100, 100000, true)(50), `Escribe un número entre 100 y ${(100000).toLocaleString('es')}.`)
})

// Revisión final web (M1, hallazgo 44): la regla «nunca menos que las respuestas» del servicio (ValidarRetencion), en el
// campo de días del registro. Los días se guardan redondeados a horas, igual que al guardar.
test('registroCubre pide que el registro dure al menos lo que las respuestas', () => {
  const regla = registroCubre(() => 48)
  assert.equal(regla(2), true)
  assert.equal(regla(3), true)
  assert.equal(regla(1.99), true, '47,76 h se guardan como 48 h')
  assert.equal(regla(1), 'El registro no puede durar menos que las respuestas (48 h).')
  assert.equal(registroCubre(() => 0)(0), true)
})

test('duracion en segundos con un decimal y, desde un minuto, en minutos y segundos', () => {
  assert.equal(duracion('2026-01-01T00:00:00Z'), '')
  assert.equal(duracion(undefined, '2026-01-01T00:00:00Z'), '')
  assert.equal(duracion('2026-01-01T00:00:00Z', '2026-01-01T00:00:12.340Z'), '12,3 s')
  assert.equal(duracion('2026-01-01T00:00:00Z', '2026-01-01T00:02:05Z'), '2 min 5 s')
  idioma.value = 'en'
  assert.equal(duracion('2026-01-01T00:00:00Z', '2026-01-01T00:00:12.340Z'), '12.3 s')
  idioma.value = 'es'
})

// v0.56, Tarea 16: el coste lleva la coma decimal en español, como la duración y el registro.
test('costo y decimal escriben los decimales del idioma', () => {
  assert.equal(costo(0.0069), 'US$ 0,0069')
  assert.equal(decimal(2, 2), '2,00')
  idioma.value = 'en'
  assert.equal(costo(0.0069), 'US$ 0.0069')
  assert.equal(decimal(12345.5, 1), '12,345.5')
  idioma.value = 'es'
})

test('estadoInfo da nombre y clase (punto y texto); un estado desconocido se ve tal cual', () => {
  assert.deepEqual(estadoInfo('completado'), { etiqueta: t('estado.completado'), clase: 'etiqueta etiqueta--ok' })
  assert.equal(estadoInfo('fallido').clase, 'etiqueta etiqueta--error')
  assert.equal(estadoInfo('en_cola').clase, 'etiqueta etiqueta--neutro')
  assert.deepEqual(estadoInfo('raro'), { etiqueta: 'raro', clase: 'etiqueta etiqueta--neutro' })
})
