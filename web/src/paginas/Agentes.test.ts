import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { formaSuscripcion, ordenClave, ordenesSuscripcion, ordenUsarSuscripcion, puedeElegirSuscripcion, puedeVolverAClave } from '../servidor.ts'
import type { EstadoServidor, Sesion } from '../api/tipos.ts'

const sesion = (entorno: string): Sesion => ({ instancia: 'global', csrf: 'tk', version: 'v0', local: true, entorno })
const servidor = (expuesto: boolean): EstadoServidor => ({ tls: 'ninguno', dominios: [], confiables: [], administrador: true, totp: true, expuesto, suscripciones: [] })

test('en el escritorio se elige la suscripción aquí; en el servidor, con su orden', () => {
  assert.equal(formaSuscripcion(sesion('escritorio'), null), 'elegir')
  assert.equal(formaSuscripcion(sesion('portable'), null), 'elegir')
  assert.equal(formaSuscripcion(sesion('servidor'), servidor(false)), 'entrar')
  assert.equal(formaSuscripcion(sesion('servidor'), servidor(true)), 'personal')
  assert.equal(formaSuscripcion(sesion('servidor'), null), 'entrar', 'sin estado del servidor no se ofrece elegir aquí')
  // En local, dos pasos: iniciar sesión en el CLI y después usar la suscripción (Ruling 34), cada uno su orden.
  assert.deepEqual(ordenesSuscripcion('claude', 'entrar'), ['tramtai-servidor agentes entrar claude', 'tramtai-servidor agentes suscripcion claude'])
  assert.deepEqual(ordenesSuscripcion('codex', 'personal'), ['tramtai-servidor agentes suscripcion codex --personal'])
})

test('dos órdenes seguidas van en dos bloques, cada uno con su copiar (Ruling 35); sin sesión, solo entrar', () => {
  // Juntas, en cmd.exe o sh la segunda línea quedaría en el búfer y la leería la primera como respuesta.
  const bloques = readFileSync(new URL('../componentes/OrdenesCopiables.vue', import.meta.url), 'utf8')
  assert.match(bloques, /v-for="\(orden, i\) in ordenes"/)
  assert.match(bloques, /\{\{ orden \}\}<q-btn[^>]*@click="copiar\(orden\)"/)
  assert.match(bloques, /v-if="i"[^>]*>\{\{ t\('comun\.despues'\) \}\}/)
  const pagina = readFileSync(new URL('./Agentes.vue', import.meta.url), 'utf8')
  assert.ok(pagina.includes('<OrdenesCopiables :ordenes="ordenesSuscripcion(a.agente, forma)"'), 'sin clave: las dos')
  assert.ok(pagina.includes(':ordenes="ordenesSuscripcion(a.agente, forma).slice(0, 1)"'), 'sin sesión (ya en suscripción): solo entrar')
  assert.deepEqual(ordenesSuscripcion('claude', 'entrar').slice(0, 1), ['tramtai-servidor agentes entrar claude'])
  assert.ok(!pagina.includes('copiar(ordenesSuscripcion'), 'nunca se copian juntas')
  const ajustes = readFileSync(new URL('./Configuracion.vue', import.meta.url), 'utf8')
  assert.ok(ajustes.includes('<OrdenesCopiables :ordenes="f.ordenes"') && !ajustes.includes('pre-line'), 'Ajustes → Servidor, igual')
})

test('tras iniciar sesión, la suscripción que falta se activa con su orden donde no hay botón (Ruling 34)', () => {
  assert.equal(ordenUsarSuscripcion('claude', { ...sesion('escritorio'), local: false }, null), 'tramtai agentes suscripcion claude')
  assert.equal(ordenUsarSuscripcion('claude', sesion('servidor'), servidor(false)), 'tramtai-servidor agentes suscripcion claude')
  assert.equal(ordenUsarSuscripcion('codex', sesion('servidor'), servidor(true)), 'tramtai-servidor agentes suscripcion codex --personal')
  // El panel de preparación y la bienvenida ofrecen el mismo aviso de riesgo que Agentes (suscripcion.ts), o esa orden.
  for (const archivo of ['../componentes/PanelPreparacion.vue', '../componentes/DialogoBienvenida.vue', './Agentes.vue']) {
    const fuente = readFileSync(new URL(archivo, import.meta.url), 'utf8')
    assert.ok(fuente.includes("from '../suscripcion'"), archivo)
  }
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
  assert.ok(pagina.includes('puedeElegirSuscripcion(') && pagina.includes('ordenesSuscripcion('))
})

test('volver a la clave de API sí se puede en el servidor, también desde la sesión remota, como en la terminal', () => {
  assert.equal(puedeVolverAClave(true, sesion('escritorio')), true)
  assert.equal(puedeVolverAClave(true, { ...sesion('escritorio'), local: false }), false, 'en el escritorio, solo desde este equipo')
  assert.equal(puedeVolverAClave(true, { ...sesion('servidor'), local: false }), true)
  assert.equal(puedeVolverAClave(false, sesion('servidor')), false)
  const pagina = readFileSync(new URL('./Agentes.vue', import.meta.url), 'utf8')
  assert.ok(pagina.includes("puedeVolverAClave(") && pagina.includes("cambiar(a, 'clave_api')"))
})
