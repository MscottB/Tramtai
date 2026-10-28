import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { idioma, t } from './i18n.ts'
import { cargarServidor, estadoServidor, marcaEntorno, suscripcionesPersonales } from './servidor.ts'
import type { Sesion } from './api/tipos.ts'

const sesion = (entorno?: string): Sesion => ({ instancia: 'global', csrf: 'tk', version: 'v0', local: false, entorno })

test('la marca de la cabecera dice el entorno: «Servidor», «Este equipo» o «Portable», en los dos idiomas', () => {
  const quiere = { es: ['Servidor', 'Este equipo', 'Portable'], en: ['Server', 'This device', 'Portable'] }
  for (const lengua of ['es', 'en'] as const) {
    idioma.value = lengua
    assert.deepEqual(['servidor', 'escritorio', 'portable'].map((e) => t(marcaEntorno(e))), quiere[lengua])
    assert.equal(t(marcaEntorno(undefined)), quiere[lengua][1], 'un servicio anterior, sin entorno, es este equipo')
  }
  idioma.value = 'es'
})

test('la marca se ve siempre: también en pantallas estrechas (sin gt-xs) y en la cabecera', () => {
  const app = readFileSync(new URL('./App.vue', import.meta.url), 'utf8')
  const linea = app.split('\n').find((l) => l.includes('data-marca-entorno'))
  assert.ok(linea, 'App.vue lleva la marca del entorno')
  assert.ok(!/gt-(xs|sm|md)/.test(linea), `la marca no se esconde en el móvil: ${linea}`)
  assert.ok(app.indexOf('data-marca-entorno') < app.indexOf('</q-header>'), 'va en la cabecera')
})

test('el aviso fijo es el de cada agente con suscripción personal (y solo esos)', () => {
  assert.deepEqual(suscripcionesPersonales(null), [])
  assert.deepEqual(suscripcionesPersonales({ tls: 'archivos', dominios: [], confiables: [], administrador: true, totp: true, expuesto: true,
    suscripciones: [{ agente: 'claude', personal: true, simultaneas: 1 }, { agente: 'codex', personal: false, simultaneas: 1 }] }), ['claude'])
  assert.equal(t('servidor.avisoSuscripcion', { agente: 'Claude' }), 'Este servidor usa tu suscripción de Claude: solo para ti')
})

test('el estado del servidor solo se pide en el servidor', async () => {
  const llamadas: string[] = []
  globalThis.fetch = async (ruta) => {
    llamadas.push(String(ruta))
    return new Response(JSON.stringify({ tls: 'ninguno', dominios: [], confiables: [], administrador: false, totp: false, expuesto: false, suscripciones: [] }), { status: 200 })
  }
  await cargarServidor(sesion('escritorio'))
  assert.deepEqual(llamadas, [])
  assert.equal(estadoServidor.value, null)
  await cargarServidor(sesion('servidor'))
  assert.deepEqual(llamadas, ['/ui/servidor'])
  assert.equal(estadoServidor.value?.tls, 'ninguno')
})
