import { test } from 'node:test'
import assert from 'node:assert/strict'
import { sesion } from '../estado.ts'
<<<<<<< HEAD
import { api, duracionAviso, ErrorApi, pasoError, textosAviso } from './cliente.ts'
=======
import { api, duracionAviso, ErrorApi, pasoError } from './cliente.ts'
>>>>>>> v0.56/5-textos
import { idioma } from '../i18n.ts'
import es from '../idiomas/es.ts'
import en from '../idiomas/en.ts'
import type { Sesion } from './tipos.ts'

test('un aviso dura al menos 4 s y más cuanto más largo es su texto, hasta 10 s', () => {
  assert.equal(duracionAviso('Copiado'), 4000)
  assert.equal(duracionAviso('Clave regenerada: actualízala en tus aplicaciones'), 4000)
  assert.ok(duracionAviso('x'.repeat(100)) > 4000)
  assert.equal(duracionAviso('x'.repeat(1000)), 10000)
})

test('el paso siguiente de un error es la solución de la Ayuda, y nada si no la tiene', () => {
  idioma.value = 'es' // el idioma inicial sale del sistema, y el runner del CI está en inglés
  assert.equal(pasoError('agente_desconectado'), es['ayuda.problemas.agente_desconectado'])
  assert.equal(pasoError('codigo_que_no_existe'), '')
  assert.equal(pasoError(undefined), '')
})

// Revisión final web (I5, hallazgo 79): un código sin solución en la Ayuda trae el qué hacer en su texto error.*. El
// aviso lo enseña en los dos idiomas: en español el mensaje es el del servicio, así que va debajo, y en inglés ya es
// el mensaje y no se repite.
test('el aviso de un error sin Ayuda enseña el qué hacer de error.* en español y en inglés, sin repetirlo', () => {
  const e = new ErrorApi('no_encontrado', 'petición no encontrada: x', 404)
  try {
    idioma.value = 'es'
    assert.deepEqual(textosAviso(e), { message: 'petición no encontrada: x', caption: es['error.no_encontrado'] })
    assert.deepEqual(textosAviso(new Error('sin red')), { message: 'sin red', caption: undefined })
    idioma.value = 'en'
    assert.deepEqual(textosAviso(e), { message: en['error.no_encontrado'], caption: 'petición no encontrada: x' })
  } finally {
    idioma.value = 'es'
  }
})

const abierta = (csrf: string): Sesion => ({ instancia: 'i', csrf, version: 'v0', local: true })
const respuesta = (estado: number, cuerpo?: unknown) => new Response(cuerpo === undefined ? null : JSON.stringify(cuerpo), { status: estado })

// simular sustituye fetch: responde cada llamada con f y apunta método, ruta y token CSRF.
function simular(f: (metodo: string, ruta: string, n: number) => Response | Promise<Response>) {
  const llamadas: { metodo: string; ruta: string; csrf?: string }[] = []
  globalThis.fetch = async (ruta, opciones) => {
    const metodo = opciones?.method ?? 'GET'
    llamadas.push({ metodo, ruta: String(ruta), csrf: (opciones?.headers as Record<string, string>)['X-CSRF-Token'] })
    return f(metodo, String(ruta), llamadas.filter((l) => l.ruta === String(ruta)).length)
  }
  return llamadas
}

test('un 401 con sesión abre otra y repite la petición una vez', async () => {
  sesion.value = abierta('viejo')
  const llamadas = simular((_, ruta, n) =>
    ruta === '/ui/sesion' ? respuesta(200, abierta('nuevo')) : n === 1 ? respuesta(401, { error: { codigo: 'no_autorizado' } }) : respuesta(200, { id: 'i' }))
  assert.deepEqual(await api.instancia(), { id: 'i' })
  assert.deepEqual(llamadas.map((l) => l.ruta), ['/api/v1/instancia', '/ui/sesion', '/api/v1/instancia'])
  assert.equal(sesion.value?.csrf, 'nuevo')
})

test('dos 401 a la vez renuevan la sesión una sola vez y repiten con el CSRF nuevo', async () => {
  sesion.value = abierta('viejo')
  const llamadas = simular(async (_, ruta, n) => {
    if (ruta === '/ui/sesion') {
      await new Promise((listo) => setTimeout(listo, 10))
      return respuesta(200, abierta('csrf-2'))
    }
    return n === 1 ? respuesta(401) : respuesta(204)
  })
  await Promise.all([api.eliminar('a'), api.eliminar('b')])
  assert.equal(llamadas.filter((l) => l.ruta === '/ui/sesion').length, 1)
  const repetidas = llamadas.filter((l) => l.metodo === 'DELETE').slice(2)
  assert.deepEqual(repetidas.map((l) => [l.ruta, l.csrf]), [['/api/v1/trabajos/a', 'csrf-2'], ['/api/v1/trabajos/b', 'csrf-2']])
})

test('si la sesión no se puede renovar, se cierra y la petición falla con sesion_caducada', async () => {
  sesion.value = abierta('viejo')
  const llamadas = simular(() => respuesta(401))
  await assert.rejects(api.instancia(), (e) => e instanceof ErrorApi && e.codigo === 'sesion_caducada' && e.estado === 401)
  assert.equal(sesion.value, null)
  assert.deepEqual(llamadas.map((l) => l.ruta), ['/api/v1/instancia', '/ui/sesion'])
})

test('sin sesión un 401 no se repite: llega el error del servicio', async () => {
  sesion.value = null
  const llamadas = simular(() => respuesta(401, { error: { codigo: 'no_autorizado', mensaje: 'Falta la sesión' } }))
  await assert.rejects(api.instancia(), { codigo: 'no_autorizado', message: 'Falta la sesión', estado: 401 })
  assert.equal(llamadas.length, 1)
})

test('un 403 de Origin no renueva la sesión ni dice que cambió el espacio', async () => {
  sesion.value = abierta('viejo')
  const llamadas = simular(() => respuesta(403, { error: { codigo: 'origen_no_permitido', mensaje: 'Origin' } }))
  await assert.rejects(api.eliminar('a'), (e) => e instanceof ErrorApi && e.codigo === 'origen_no_permitido')
  assert.equal(llamadas.filter((l) => l.ruta === '/ui/sesion').length, 0)
})

test('un 403 de CSRF renueva una sola vez', async () => {
  sesion.value = abierta('viejo')
  const llamadas = simular((_, ruta) =>
    ruta === '/ui/sesion' ? respuesta(200, abierta('nuevo')) : respuesta(403, { error: { codigo: 'csrf_invalido', mensaje: 'CSRF' } }))
  await assert.rejects(api.eliminar('a'), (e) => e instanceof ErrorApi && e.codigo === 'espacio_cambiado' && e.estado === 403)
  assert.equal(llamadas.filter((l) => l.ruta === '/ui/sesion').length, 1, 'renueva la sesión una vez')
  assert.equal(llamadas.filter((l) => l.metodo === 'DELETE').length, 1, 'no repite la acción: el espacio de la sesión cambió')
  assert.equal(sesion.value?.csrf, 'nuevo')
})
