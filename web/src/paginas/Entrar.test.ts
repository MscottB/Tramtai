import { test } from 'node:test'
import assert from 'node:assert/strict'
import { idioma, t } from '../i18n.ts'
import es from '../idiomas/es.ts'
import en from '../idiomas/en.ts'
import { sesion } from '../estado.ts'
import { acceder, consultarEntrada, estadoEntrada, vistaEntrada } from '../entrada.ts'

const respuesta = (estado: number, cuerpo?: unknown) => new Response(cuerpo === undefined ? null : JSON.stringify(cuerpo), { status: estado })

// simular sustituye fetch y apunta método, ruta y cuerpo de cada llamada.
function simular(f: (metodo: string, ruta: string) => Response) {
  const llamadas: { metodo: string; ruta: string; cuerpo?: string }[] = []
  globalThis.fetch = async (ruta, opciones) => {
    const metodo = opciones?.method ?? 'GET'
    llamadas.push({ metodo, ruta: String(ruta), cuerpo: opciones?.body as string | undefined })
    return f(metodo, String(ruta))
  }
  return llamadas
}

test('con contrasena:true y https:true se muestra el formulario; con https:false, el porqué', () => {
  assert.equal(vistaEntrada(null), 'consultando')
  assert.equal(vistaEntrada({ contrasena: true, totp: true, https: true }), 'formulario')
  assert.equal(vistaEntrada({ contrasena: true, totp: false, https: false }), 'sinHttps')
  assert.equal(vistaEntrada({ contrasena: false, totp: false, https: true }), 'codigo')
  for (const [lengua, textos] of [['es', es], ['en', en]] as const) {
    idioma.value = lengua
    assert.ok(t('entrar.sinHttps').includes('HTTPS'), `${lengua}: el porqué nombra HTTPS`)
    assert.equal(t('entrar.sinHttps'), textos['entrar.sinHttps'])
  }
  idioma.value = 'es'
})

test('consulta GET /ui/entrada; si falla, queda la entrada con el código', async () => {
  simular(() => respuesta(200, { contrasena: true, totp: false, https: true }))
  await consultarEntrada()
  assert.equal(vistaEntrada(estadoEntrada.value), 'formulario')
  simular(() => respuesta(500, { error: { codigo: 'error' } }))
  await consultarEntrada()
  assert.equal(vistaEntrada(estadoEntrada.value), 'codigo')
})

test('acceder envía la contraseña y el código a POST /ui/acceder y abre la sesión', async () => {
  sesion.value = null
  const llamadas = simular(() => respuesta(200, { csrf: 'tk', version: 'v0', local: false, instancia: 'global' }))
  await acceder('una contraseña larga', ' 123456 ')
  assert.deepEqual(llamadas.map((l) => [l.metodo, l.ruta, JSON.parse(l.cuerpo ?? '{}')]), [['POST', '/ui/acceder', { contrasena: 'una contraseña larga', codigo: '123456' }]])
  assert.equal(sesion.value?.instancia, 'global')
  assert.equal(sesion.value?.local, false)
})

test('si la contraseña o el código no valen, el error sube con el mensaje del servicio y no hay sesión', async () => {
  sesion.value = null
  simular(() => respuesta(401, { error: { codigo: 'no_autorizado', mensaje: 'contraseña o código incorrectos' } }))
  await assert.rejects(acceder('mala', ''), /contraseña o código incorrectos/)
  assert.equal(sesion.value, null)
})
