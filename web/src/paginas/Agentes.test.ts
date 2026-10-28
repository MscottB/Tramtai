import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { formaSuscripcion, ordenClave, ordenSuscripcion, puedeElegirSuscripcion, puedeVolverAClave } from '../servidor.ts'
import type { EstadoServidor, Sesion } from '../api/tipos.ts'

const sesion = (entorno: string): Sesion => ({ instancia: 'global', csrf: 'tk', version: 'v0', local: true, entorno })
const servidor = (expuesto: boolean): EstadoServidor => ({ tls: 'ninguno', dominios: [], confiables: [], administrador: true, totp: true, expuesto, suscripciones: [] })

test('en el escritorio se elige la suscripción aquí; en el servidor, con su orden', () => {
  assert.equal(formaSuscripcion(sesion('escritorio'), null), 'elegir')
  assert.equal(formaSuscripcion(sesion('portable'), null), 'elegir')
  assert.equal(formaSuscripcion(sesion('servidor'), servidor(false)), 'entrar')
  assert.equal(formaSuscripcion(sesion('servidor'), servidor(true)), 'personal')
  assert.equal(formaSuscripcion(sesion('servidor'), null), 'entrar', 'sin estado del servidor no se ofrece elegir aquí')
  assert.equal(ordenSuscripcion('claude', 'entrar'), 'tramtai-servidor agentes entrar claude')
  assert.equal(ordenSuscripcion('codex', 'personal'), 'tramtai-servidor agentes suscripcion codex --personal')
})

test('la clave que falta se guarda con tramtai-servidor en el servidor', () => {
  assert.equal(ordenClave(sesion('escritorio'), 'ANTHROPIC_API_KEY'), 'tramtai claves guardar ANTHROPIC_API_KEY')
  assert.equal(ordenClave(sesion('servidor'), 'ANTHROPIC_API_KEY'), 'tramtai-servidor claves guardar ANTHROPIC_API_KEY')
})

test('«Usar mi suscripción» solo sale en una sesión de este equipo fuera del servidor, con un agente que la admite', () => {
  assert.equal(puedeElegirSuscripcion(true, sesion('escritorio'), null), true)
  assert.equal(puedeElegirSuscripcion(false, sesion('escritorio'), null), false)
  assert.equal(puedeElegirSuscripcion(true, { ...sesion('escritorio'), local: false }, null), false)
  assert.equal(puedeElegirSuscripcion(true, sesion('servidor'), servidor(false)), false)
  assert.equal(puedeElegirSuscripcion(true, sesion('servidor'), servidor(true)), false)
  // Agentes.vue decide los botones de la suscripción con ella, y en el servidor enseña la orden.
  const pagina = readFileSync(new URL('./Agentes.vue', import.meta.url), 'utf8')
  assert.ok(pagina.includes('puedeElegirSuscripcion(') && pagina.includes('ordenSuscripcion('))
})

test('volver a la clave de API sí se puede en el servidor, también desde la sesión remota, como en la terminal', () => {
  assert.equal(puedeVolverAClave(true, sesion('escritorio')), true)
  assert.equal(puedeVolverAClave(true, { ...sesion('escritorio'), local: false }), false, 'en el escritorio, solo desde este equipo')
  assert.equal(puedeVolverAClave(true, { ...sesion('servidor'), local: false }), true)
  assert.equal(puedeVolverAClave(false, sesion('servidor')), false)
  const pagina = readFileSync(new URL('./Agentes.vue', import.meta.url), 'utf8')
  assert.ok(pagina.includes("puedeVolverAClave(") && pagina.includes("cambiar(a, 'clave_api')"))
})
