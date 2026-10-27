import { test } from 'node:test'
import assert from 'node:assert/strict'
import { api } from './api/cliente.ts'
import { idioma } from './i18n.ts'
import es from './idiomas/es.ts'
import en from './idiomas/en.ts'
import { aceptarTerminos, consultarTerminos, estadoTerminos, pantallaTerminos, riesgosTexto, salioDeTerminos, salirDeTerminos } from './terminos.ts'
import type { EstadoTerminos } from './api/tipos.ts'

const respuesta = (estado: number, cuerpo?: unknown) => new Response(cuerpo === undefined ? null : JSON.stringify(cuerpo), { status: estado })
const pendiente: EstadoTerminos = { version: '2.0', pendiente: true, riesgos: ['a', 'b', 'c', 'd', 'e'], servidor: false }

// simular sustituye fetch y apunta método, ruta y token CSRF de cada llamada.
function simular(f: (metodo: string, ruta: string) => Response) {
  const llamadas: { metodo: string; ruta: string; csrf?: string }[] = []
  globalThis.fetch = async (ruta, opciones) => {
    const metodo = opciones?.method ?? 'GET'
    llamadas.push({ metodo, ruta: String(ruta), csrf: (opciones?.headers as Record<string, string>)['X-CSRF-Token'] })
    return f(metodo, String(ruta))
  }
  return llamadas
}

// abrir deja una sesión abierta (con su CSRF) y los términos sin consultar, como al cargar la página.
async function abrir() {
  idioma.value = 'es'
  estadoTerminos.value = null
  salioDeTerminos.value = false
  const llamadas = simular(() => respuesta(200, { csrf: 'tk', version: 'v0', local: true, instancia: 'global' }))
  await api.sesion()
  llamadas.length = 0
  return llamadas
}

test('la pantalla depende de si se consultó, si hay algo pendiente y si se salió', () => {
  assert.equal(pantallaTerminos(null, false), 'consultando')
  assert.equal(pantallaTerminos(pendiente, false), 'pedir')
  assert.equal(pantallaTerminos({ ...pendiente, pendiente: false }, false), 'app')
  assert.equal(pantallaTerminos(pendiente, true), 'salio')
})

test('con pendiente:true se pide la aceptación con los cinco riesgos, y «Acepto» llama al POST con el CSRF', async () => {
  await abrir()
  simular(() => respuesta(200, pendiente))
  await consultarTerminos()
  assert.equal(pantallaTerminos(estadoTerminos.value, salioDeTerminos.value), 'pedir')
  assert.equal(riesgosTexto().length, 5)

  const hechas = simular(() => respuesta(204))
  await aceptarTerminos()
  assert.deepEqual(hechas, [{ metodo: 'POST', ruta: '/ui/terminos', csrf: 'tk' }])
  assert.equal(pantallaTerminos(estadoTerminos.value, salioDeTerminos.value), 'app')
})

test('«Salir» deja la app en la pantalla «Has salido» y no se vuelve a pedir nada', async () => {
  await abrir()
  const llamadas = simular(() => respuesta(200, pendiente))
  await consultarTerminos()
  salirDeTerminos()
  assert.equal(pantallaTerminos(estadoTerminos.value, salioDeTerminos.value), 'salio')
  assert.deepEqual(llamadas.map((l) => `${l.metodo} ${l.ruta}`), ['GET /ui/terminos'], 'salir no acepta ni llama a nada')
})

test('si no se puede consultar, no se bloquea la app', async () => {
  await abrir()
  simular(() => respuesta(500, { error: { codigo: 'error' } }))
  await consultarTerminos()
  assert.equal(pantallaTerminos(estadoTerminos.value, salioDeTerminos.value), 'app')
})

test('los riesgos son los cinco de los términos en el idioma de la Web UI; sin el modo servidor, el 3 y el 4 lo dicen', () => {
  for (const [lengua, textos] of [['es', es], ['en', en]] as const) {
    idioma.value = lengua
    const servidor = riesgosTexto(true)
    assert.deepEqual(servidor, [1, 2, 3, 4, 5].map((n) => textos[`riesgos.${n}` as 'riesgos.1']))
    const escritorio = riesgosTexto(false)
    assert.deepEqual([0, 1, 4].map((i) => escritorio[i]), [0, 1, 4].map((i) => servidor[i]))
    assert.equal(escritorio[2], textos['riesgos.3.sinServidor'])
    assert.equal(escritorio[3], textos['riesgos.4.sinServidor'])
    for (const i of [2, 3]) assert.ok(escritorio[i].startsWith(lengua === 'es' ? 'Si usas el modo servidor' : 'If you use server mode'), `${lengua} riesgo ${i + 1}: ${escritorio[i]}`)
  }
  idioma.value = 'es'
})
