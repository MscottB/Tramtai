import { test } from 'node:test'
import assert from 'node:assert/strict'
import { sesion } from '../estado.ts'
import { api, ErrorApi } from './cliente.ts'
import type { Sesion } from './tipos.ts'

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

test('un 403 de CSRF renueva una sola vez', async () => {
  sesion.value = abierta('viejo')
  const llamadas = simular((_, ruta) =>
    ruta === '/ui/sesion' ? respuesta(200, abierta('nuevo')) : respuesta(403, { error: { codigo: 'origen_no_permitido', mensaje: 'CSRF' } }))
  await assert.rejects(api.eliminar('a'), (e) => e instanceof ErrorApi && e.codigo === 'espacio_cambiado' && e.estado === 403)
  assert.equal(llamadas.filter((l) => l.ruta === '/ui/sesion').length, 1, 'renueva la sesión una vez')
  assert.equal(llamadas.filter((l) => l.metodo === 'DELETE').length, 1, 'no repite la acción: el espacio de la sesión cambió')
  assert.equal(sesion.value?.csrf, 'nuevo')
})
