import { test } from 'node:test'
import assert from 'node:assert/strict'
import { idioma, t } from '../i18n.ts'
import { conArranque, filasServidor, ofrecerInstalar, pestanasAjustes } from '../servidor.ts'
import type { EstadoServidor, Sesion } from '../api/tipos.ts'

const sesion = (entorno: string): Sesion => ({ instancia: 'global', csrf: 'tk', version: 'v0', local: true, entorno, autoarranque: false, modo_arranque: '' })

test('Servidor y Seguridad solo en el servidor', () => {
  assert.deepEqual(pestanasAjustes(false), ['espacio', 'tramtai'])
  assert.deepEqual(pestanasAjustes(true), ['espacio', 'tramtai', 'servidor', 'seguridad'])
})

test('en el servidor no hay arranque con el sistema ni «Instalar ahora»', () => {
  assert.equal(conArranque(sesion('escritorio')), true)
  assert.equal(conArranque(sesion('portable')), true)
  assert.equal(conArranque(sesion('servidor')), false)
  assert.equal(conArranque({ ...sesion('escritorio'), autoarranque: undefined }), false, 'un sistema sin arranque no lo enseña')
  assert.equal(ofrecerInstalar(true, sesion('escritorio')), true)
  assert.equal(ofrecerInstalar(true, sesion('servidor')), false)
  assert.equal(ofrecerInstalar(false, sesion('escritorio')), false)
})

test('Ajustes → Servidor: cada dato con la orden (o la clave de config.yaml) que lo cambia', () => {
  const e: EstadoServidor = { tls: 'letsencrypt', dominios: ['tramtai.ejemplo.org'], confiables: [], vence_certificado: '2027-01-02T03:04:05Z',
    administrador: true, totp: false, expuesto: true, suscripciones: [{ agente: 'claude', personal: true, simultaneas: 2 }] }
  const filas = filasServidor(e, (iso) => iso.slice(0, 10), (a) => a.toUpperCase())
  const de = (clave: string) => filas.find((f) => f.etiqueta === clave)
  assert.deepEqual(de('servidor.https'), { etiqueta: 'servidor.https', valor: t('servidor.tlsLetsencrypt'), orden: 'servicio.tls.modo' })
  assert.deepEqual(de('servidor.dominios'), { etiqueta: 'servidor.dominios', valor: 'tramtai.ejemplo.org', orden: 'servicio.dominios' })
  assert.equal(de('servidor.confiables')?.valor, t('servidor.ninguno'))
  assert.deepEqual(de('servidor.vence'), { etiqueta: 'servidor.vence', valor: '2027-01-02', nota: t('servidor.renuevaSolo') })
  assert.deepEqual(de('servidor.administrador'), { etiqueta: 'servidor.administrador', valor: t('servidor.sinTotp'), orden: 'tramtai-servidor administrador totp activar' })
  assert.equal(de('servidor.suscripciones')?.valor, t('servidor.suscripcionPersonal', { agente: 'CLAUDE', n: 2 }))
  for (const f of filas) assert.ok(f.orden || f.nota, `${f.etiqueta} dice cómo se cambia`)

  const sinNada = filasServidor({ ...e, tls: 'ninguno', dominios: [], vence_certificado: undefined, administrador: false, suscripciones: [] }, String, String)
  assert.equal(sinNada.find((f) => f.etiqueta === 'servidor.vence'), undefined, 'sin certificado propio no hay vencimiento')
  assert.equal(sinNada.find((f) => f.etiqueta === 'servidor.administrador')?.orden, 'tramtai-servidor administrador contraseña')
  assert.equal(sinNada.find((f) => f.etiqueta === 'servidor.suscripciones')?.valor, t('servidor.sinSuscripciones'))
})

test('los textos de Servidor existen en los dos idiomas', () => {
  for (const lengua of ['es', 'en'] as const) {
    idioma.value = lengua
    for (const clave of ['config.pestanaServidor', 'servidor.texto', 'servidor.cambiaEn', 'servidor.tlsNinguno', 'servidor.renovarArchivos'] as const) {
      assert.notEqual(t(clave), clave, `${lengua}: ${clave}`)
    }
  }
  idioma.value = 'es'
})
