import { test } from 'node:test'
import assert from 'node:assert/strict'
import { idioma, t } from './i18n.ts'
import { agenteListo, agruparOpciones, esfuerzoPredeterminadoDe, estadoAgente, ofrecerDejarSuscripcion, soloDesconectados, esfuerzosDe, fichas, opcionEscrita, opcionModelo, ordenarAgentes, type FichaAgente } from './agentes.ts'
import { numero } from './formato.ts'
import type { Adaptador, Modelo } from './api/tipos.ts'

idioma.value = 'es'
const ficha = (agente: string, nombre: string): FichaAgente => ({ agente, nombre, programa: agente, codigo: nombre.slice(0, 2).toUpperCase(),
  color: { claro: '#000000', oscuro: '#ffffff' }, orden: 0, streaming: 'no', salida_estructurada: true, modelo_efectivo_informado: true, esfuerzos: [],
  autenticaciones: [], variables_clave: [], permitido_apps: true, url_instalacion: '' })
fichas.value = [ficha('claude', 'Claude'), ficha('codex', 'Codex')]

test('ordenarAgentes sigue el orden de las fichas y deja los desconocidos al final por nombre', () => {
  const lista = [{ agente: 'zeta' }, { agente: 'codex' }, { agente: 'alfa' }, { agente: 'claude' }]
  assert.deepEqual(ordenarAgentes(lista).map((a) => a.agente), ['claude', 'codex', 'alfa', 'zeta'])
  assert.equal(lista[0].agente, 'zeta', 'no cambia la lista original')
})

test('opcionModelo: modelo configurado, versión vista y perfil', () => {
  assert.deepEqual(opcionModelo({ id: 'claude/sonnet', agente: 'claude', modelo: 'sonnet', version: 'claude-sonnet-5', contexto_tokens: 200000,
    esfuerzos: ['bajo', 'alto'], predeterminado: true }),
  { label: 'Claude · sonnet', corto: 'sonnet', caption: 'claude-sonnet-5 · ' + t('modelo.contexto', { n: numero(200000) }), version: 'claude-sonnet-5',
    grupo: 'claude', value: 'claude/sonnet', predeterminado: true, esfuerzos: ['bajo', 'alto'], exacta: false })
  const vista = opcionModelo({ id: 'claude/claude-sonnet-5-5', agente: 'claude', modelo: 'claude-sonnet-5-5', origen: 'visto' })
  assert.equal(vista.exacta, true)
  assert.equal(vista.version, 'claude-sonnet-5-5')
  assert.equal(vista.caption, t('modelo.versionConcreta'))
  assert.equal(opcionModelo({ id: 'codex', agente: 'codex' }).corto, t('modelo.delCli'))
  const perfil = opcionModelo({ id: 'perfil/rapido', perfil: 'rapido', resuelto: 'claude/haiku', version: 'claude-haiku-5' })
  assert.equal(perfil.grupo, 'perfil')
  assert.equal(perfil.label, t('modelo.perfil', { nombre: t('perfil.rapido') }))
  assert.equal(perfil.corto, perfil.label)
  assert.equal(perfil.caption, 'Claude · haiku · claude-haiku-5')
})

test('opcionEscrita es un agente/modelo que no está en el catálogo', () => {
  assert.deepEqual(opcionEscrita('codex/gpt-x'), { label: 'Codex · gpt-x', corto: 'gpt-x', caption: t('modelo.escrito'), version: '', grupo: 'codex',
    value: 'codex/gpt-x', predeterminado: false, esfuerzos: [] })
  assert.equal(opcionEscrita('codex').corto, t('modelo.delCli'))
})

test('agruparOpciones: perfiles, agentes en su orden y los no disponibles al final; dentro, CLI, configurados y versiones vistas', () => {
  const lista = [
    opcionEscrita('codex/a'),
    opcionModelo({ id: 'claude/b', agente: 'claude', modelo: 'b', origen: 'visto' }),
    opcionModelo({ id: 'claude/a', agente: 'claude', modelo: 'a' }),
    opcionModelo({ id: 'claude', agente: 'claude' }),
    opcionModelo({ id: 'perfil/rapido', perfil: 'rapido' }),
  ]
  const valores = (l: typeof lista) => l.map((o) => o.value)
  assert.deepEqual(valores(agruparOpciones(lista)), ['#perfil', 'perfil/rapido', '#claude', 'claude', 'claude/a', 'claude/b', '#codex', 'codex/a'])
  const apagado = agruparOpciones(lista.map((o) => (o.grupo === 'claude' ? { ...o, disable: true } : o)))
  assert.deepEqual(valores(apagado), ['#perfil', 'perfil/rapido', '#codex', 'codex/a', '#claude', 'claude', 'claude/a', 'claude/b'])
  const cabecera = apagado.find((o) => o.value === '#claude')!
  assert.equal(cabecera.label, 'Claude · ' + t('modelo.noDisponible'))
  assert.equal(cabecera.cabecera, true)
  assert.equal(cabecera.disable, true)
})

test('esfuerzos del modelo elegido: uno escrito a mano tiene los de su agente, pero no su predeterminado', () => {
  const catalogo: Modelo[] = [
    { id: 'claude', agente: 'claude', esfuerzos: ['bajo', 'alto'], esfuerzo_predeterminado: 'alto' },
    { id: 'claude/sonnet', agente: 'claude', modelo: 'sonnet', esfuerzos: ['bajo', 'medio'], esfuerzo_predeterminado: 'medio', predeterminado: true },
  ]
  assert.deepEqual(esfuerzosDe(catalogo, 'claude/sonnet'), ['bajo', 'medio'])
  assert.deepEqual(esfuerzosDe(catalogo, 'claude/escrito'), ['bajo', 'alto'])
  assert.deepEqual(esfuerzosDe(catalogo, null), ['bajo', 'medio'])
  assert.deepEqual(esfuerzosDe(catalogo, 'otro/x'), [])
  assert.equal(esfuerzoPredeterminadoDe(catalogo, 'claude'), 'alto')
  assert.equal(esfuerzoPredeterminadoDe(catalogo, 'claude/escrito'), '')
  assert.equal(esfuerzoPredeterminadoDe(catalogo, null), 'medio')
})

const adaptador = (o: Partial<Adaptador>): Adaptador => ({ agente: 'claude', capacidades: { streaming: 'tokens', salida_estructurada: true, modelo_efectivo_informado: true }, ...o })

test('estadoAgente según el problema, la autenticación y si está desconectado', () => {
  assert.equal(estadoAgente(adaptador({})), 'listo')
  assert.equal(estadoAgente(adaptador({ problema: 'x', problema_codigo: 'aviso' })), 'listo')
  assert.equal(estadoAgente(adaptador({ problema: 'x', problema_codigo: 'no_instalado' })), 'noInstalado')
  assert.equal(estadoAgente(adaptador({ problema: 'x', problema_codigo: 'falta_clave' })), 'faltaClave')
  assert.equal(estadoAgente(adaptador({ problema: 'x', problema_codigo: 'falta_clave', autenticacion: 'suscripcion' })), 'sinSesion')
  assert.equal(estadoAgente(adaptador({ problema: 'x', problema_codigo: 'usa_clave_api', autenticacion: 'suscripcion' })), 'usaClave')
  assert.equal(estadoAgente(adaptador({ problema: 'x', problema_codigo: 'sesion_no_responde' })), 'sesionNoResponde')
  assert.equal(estadoAgente(adaptador({ problema: 'x' })), 'revisar')
  assert.equal(estadoAgente(adaptador({ desconectado: true, problema: 'x', problema_codigo: 'no_instalado' })), 'desconectado')
})

test('«Dejar de usar la suscripción» en cualquier estado si la ficha admite las dos', () => {
  for (const problema_codigo of [undefined, 'falta_clave', 'usa_clave_api', 'sesion_no_responde', '']) {
    const a = adaptador({ autenticacion: 'suscripcion', problema: problema_codigo === undefined ? undefined : 'algo', problema_codigo })
    assert.equal(ofrecerDejarSuscripcion(a, true), true, String(problema_codigo))
  }
  assert.equal(ofrecerDejarSuscripcion(adaptador({ autenticacion: 'clave_api' }), true), false)
  assert.equal(ofrecerDejarSuscripcion(adaptador({ autenticacion: 'suscripcion' }), false), false)
})

test('un agente desconectado no está listo', () => {
  assert.equal(agenteListo(adaptador({ desconectado: true })), false)
  assert.equal(agenteListo(adaptador({})), true)
})

test('una API que solo puede usar agentes desconectados', () => {
  fichas.value = [{ ...ficha('claude', 'Claude'), desconectado: true }, ficha('codex', 'Codex')]
  assert.equal(soloDesconectados({ nombre: 'a', agente: 'claude' }), true)
  assert.equal(soloDesconectados({ nombre: 'a', agente: 'claude', respaldo: ['codex/m1'] }), false)
  assert.equal(soloDesconectados({ nombre: 'a', modelos_permitidos: ['claude/haiku', 'claude/sonnet'] }), true)
  assert.equal(soloDesconectados({ nombre: 'a' }), false, 'sin agente propio usa el predeterminado del espacio')
  fichas.value = [ficha('claude', 'Claude'), ficha('codex', 'Codex')]
})
