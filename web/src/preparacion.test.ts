import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { accionPrincipal, confirmarDesinstalar, masNueva, pideSeguir, seguirPreparacion, versionNueva, vistaEspera } from './preparacion.ts'
import type { Adaptador, EstadoPreparacion, VersionesAgente } from './api/tipos.ts'

const agente = (estado: Adaptador['estado']): Adaptador =>
  ({ agente: 'claude', estado, autenticacion: 'suscripcion', capacidades: {} } as unknown as Adaptador)
const versiones = (v: Partial<VersionesAgente>): VersionesAgente => ({ agente: 'claude', comprobadas: {}, acciones: [], ...v })
const instalado = versiones({ instalada: '2.1.288', ultima: '2.1.288', comprobadas: { actualizar: true, iniciar_sesion: true, desinstalar: true },
  acciones: ['actualizar', 'iniciar_sesion', 'desinstalar'] })

test('la acción principal de cada tarjeta sale de su estado y de sus versiones', () => {
  assert.deepEqual(accionPrincipal(agente('no_instalado'), versiones({ comprobadas: { instalar: true }, acciones: ['instalar'] })),
    { accion: 'instalar', etiqueta: 'preparar.instalar', aviso: undefined })
  assert.equal(accionPrincipal(agente('sin_sesion'), instalado).etiqueta, 'preparar.iniciarSesion')
  assert.equal(accionPrincipal(agente('sesion_no_responde'), instalado).etiqueta, 'preparar.volverAIniciar')
  assert.equal(accionPrincipal(agente('sesion_no_responde'), instalado).accion, 'iniciar_sesion')
  // Listo: solo «Actualizar» si hay una versión más nueva que la instalada.
  assert.equal(accionPrincipal(agente('listo'), instalado).accion, undefined)
  const nueva = { ...instalado, ultima: '2.1.300' }
  assert.equal(accionPrincipal(agente('listo'), nueva).accion, 'actualizar')
  assert.equal(versionNueva(nueva), '2.1.300')
  assert.equal(versionNueva(instalado), '')
  // Sin receta (o sin versiones aún) no hay acción de preparar: la tarjeta sigue como antes.
  assert.deepEqual(accionPrincipal(agente('no_instalado'), undefined), {})
  // Sin inicio de sesión en su receta (Antigravity: solo clave), la tarjeta no ofrece iniciarla.
  assert.deepEqual(accionPrincipal(agente('sin_sesion'), versiones({ instalada: '1.0.0', comprobadas: { actualizar: true }, acciones: ['actualizar'] })), {})
})

test('una versión no comprobada para iniciar sesión enseña el aviso y no el botón', () => {
  const motivo = 'Claude Code 2.2.0 aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, puedes hacerlo en una consola con claude auth login'
  const v = versiones({ instalada: '2.2.0', comprobadas: { actualizar: true, iniciar_sesion: false, desinstalar: true }, acciones: ['actualizar', 'desinstalar'],
    motivo_codigo: 'version_no_comprobada', motivo })
  assert.deepEqual(accionPrincipal(agente('sin_sesion'), v), { aviso: motivo })
  // Instalar una no comprobada sí se puede, con el aviso delante (y «Seguir» en el panel).
  const aviso = 'Claude Code 2.2.0 aún no está comprobado con Tramtai: si sigues, el inicio de sesión automático quedará desactivado hasta que lo esté'
  const instalar = versiones({ comprobadas: { instalar: false }, acciones: ['instalar'], motivo_codigo: 'version_no_comprobada', motivo: aviso })
  assert.deepEqual(accionPrincipal(agente('no_instalado'), instalar), { accion: 'instalar', etiqueta: 'preparar.instalar', aviso })
})

test('tras un version_no_comprobada se ofrece «Seguir», salvo para iniciar sesión, y desinstalar confirma si no está comprobada', () => {
  const error = (accion: EstadoPreparacion['accion']): EstadoPreparacion => ({ agente: 'claude', accion, estado: 'error', motivo_codigo: 'version_no_comprobada' })
  assert.equal(pideSeguir(error('instalar')), true)
  assert.equal(pideSeguir(error('actualizar')), true)
  assert.equal(pideSeguir(error('desinstalar')), true)
  assert.equal(pideSeguir(error('iniciar_sesion')), false, 'el inicio de sesión no se salta nunca')
  assert.equal(pideSeguir({ ...error('instalar'), motivo_codigo: 'plazo_agotado' }), false)
  assert.equal(confirmarDesinstalar({ ...instalado, comprobadas: { desinstalar: false } }), true)
  assert.equal(confirmarDesinstalar(instalado), false)
})

test('con esperando_usuario el panel enseña el enlace, el código y el campo para pegar; el enlace se abre aparte', () => {
  const e: EstadoPreparacion = { agente: 'codex', accion: 'iniciar_sesion', estado: 'esperando_usuario', enlace: 'https://auth.openai.com/codex/device', codigo: 'ABCD-12345' }
  assert.deepEqual(vistaEspera(e), { enlace: 'https://auth.openai.com/codex/device', codigo: 'ABCD-12345', pegar: false })
  assert.equal(vistaEspera({ ...e, pide_pegar: true, codigo: undefined })?.pegar, true)
  assert.equal(vistaEspera({ ...e, enlace: 'javascript:alert(1)' })?.enlace, undefined, 'solo enlaces https')
  assert.equal(vistaEspera({ ...e, estado: 'comprobando' }), undefined)
  const panel = readFileSync(new URL('./componentes/PanelPreparacion.vue', import.meta.url), 'utf8')
  assert.match(panel, /target="_blank" rel="noopener noreferrer"/)
  assert.match(panel, /type="password"/)
  assert.match(panel, /autocomplete="off"/)
  assert.match(panel, /role="status"/)
})

test('el sondeo sigue mientras está en marcha y para con listo, error o cancelado', async () => {
  for (const final of ['listo', 'error', 'cancelado'] as const) {
    const estados: EstadoPreparacion[] = ['instalando', 'comprobando', 'esperando_usuario', final]
      .map((estado) => ({ agente: 'claude', accion: 'instalar', estado } as EstadoPreparacion))
    const vistos: string[] = []
    let esperas = 0
    const fin = await seguirPreparacion(async () => estados.shift()!, (e) => vistos.push(e?.estado ?? ''), async () => { esperas++ })
    assert.equal(fin?.estado, final)
    assert.deepEqual(vistos, ['instalando', 'comprobando', 'esperando_usuario', final])
    assert.equal(esperas, 3)
  }
  // Sin ninguna preparación (null) no se espera nada.
  let consultas = 0
  assert.equal(await seguirPreparacion(async () => (consultas++, null), () => {}, async () => {}), null)
  assert.equal(consultas, 1)
})

test('masNueva compara versiones por números', () => {
  assert.equal(masNueva('2.1.300', '2.1.288'), true)
  assert.equal(masNueva('2.10.0', '2.9.9'), true)
  assert.equal(masNueva('2.1.288', '2.1.288'), false)
  assert.equal(masNueva('0.167.2', '0.170.0'), false)
})
