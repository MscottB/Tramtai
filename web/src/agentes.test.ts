import { test } from 'node:test'
import assert from 'node:assert/strict'
import { idioma, t } from './i18n.ts'
import { agenteListo, agruparOpciones, chapaDe, contraste, esfuerzoPredeterminadoDe, fijarDisponibilidad, modeloDe, problemasAgentes, ofrecerDejarSuscripcion, soloDesconectados, esfuerzosDe, fichas, opcionEscrita, opcionModelo, ordenarAgentes, type FichaAgente } from './agentes.ts'
import { numero } from './formato.ts'
import type { Adaptador, Modelo, Trabajo } from './api/tipos.ts'

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

const adaptador = (o: Partial<Adaptador>): Adaptador => ({ agente: 'claude', estado: 'listo', desconectado: false, capacidades: { texto: true, streaming: 'tokens',
  salida_estructurada: true, uso_tokens: 'informado_por_cli', modelo_efectivo_informado: true, tool_calling_compatible: false, imagenes: false,
  sesiones_persistentes: false }, ...o })

// El estado lo calcula el servicio (api.estadoAgente): la Web UI pinta el que llega, sin volver a deducirlo.
test('fijarDisponibilidad dice por qué no se puede usar cada agente según el estado que manda el servicio', () => {
  fijarDisponibilidad([adaptador({ agente: 'a' }), adaptador({ agente: 'b', estado: 'no_instalado', desconectado: true, problema: 'x', problema_codigo: 'no_instalado' }),
    adaptador({ agente: 'c', estado: 'desconectado', desconectado: true }), adaptador({ agente: 'd', estado: 'sin_sesion', problema: 'x' }),
    adaptador({ agente: 'e', estado: 'revisar', problema: 'roto' })])
  assert.deepEqual(problemasAgentes.value, { b: t('modelo.noInstalado'), c: t('modelo.desconectado'), d: t('modelo.faltaClave'), e: t('modelo.conProblema', { problema: 'roto' }) })
})

test('«Dejar de usar la suscripción» en cualquier estado si la ficha admite las dos', () => {
  for (const problema_codigo of [undefined, 'falta_clave', 'usa_clave_api', 'sesion_no_responde', '']) {
    const a = adaptador({ autenticacion: 'suscripcion', problema: problema_codigo === undefined ? undefined : 'algo', problema_codigo })
    assert.equal(ofrecerDejarSuscripcion(a, true), true, String(problema_codigo))
  }
  assert.equal(ofrecerDejarSuscripcion(adaptador({ autenticacion: 'clave_api' }), true), false)
  assert.equal(ofrecerDejarSuscripcion(adaptador({ autenticacion: 'suscripcion' }), false), false)
})

test('agenteListo pinta el estado del servicio', () => {
  assert.equal(agenteListo(adaptador({ estado: 'desconectado', desconectado: true })), false)
  assert.equal(agenteListo(adaptador({})), true)
  assert.equal(agenteListo(adaptador({ problema: 'un aviso', problema_codigo: 'aviso' })), true, 'no lo vuelve a deducir del problema')
})

test('una API que solo puede usar agentes desconectados', () => {
  fichas.value = [{ ...ficha('claude', 'Claude'), desconectado: true }, ficha('codex', 'Codex')]
  assert.equal(soloDesconectados({ nombre: 'a', agente: 'claude' }), true)
  assert.equal(soloDesconectados({ nombre: 'a', agente: 'claude', respaldo: ['codex/m1'] }), false)
  assert.equal(soloDesconectados({ nombre: 'a', modelos_permitidos: ['claude/haiku', 'claude/sonnet'] }), true)
  assert.equal(soloDesconectados({ nombre: 'a' }), false, 'sin agente propio usa el predeterminado del espacio')
  // Un perfil no es un agente llamado «perfil»: se resuelve en el servicio y no cuenta.
  assert.equal(soloDesconectados({ nombre: 'a', agente: 'claude', modelos_permitidos: ['perfil/rapido'] }), true)
  fichas.value = [ficha('claude', 'Claude'), ficha('codex', 'Codex')]
})

test('chapaDe da a la chapa de cada línea un texto que llega a 4,5:1, aunque el color de la ficha no lo permita', () => {
  for (const color of ['#c4561d', '#2563eb', '#7c3aed', '#db2777', '#1f883d', '#64748b', '#f28a4f', '#5b8def', '#a78bfa', '#f472b6', '#3fb950', '#ffff00', '#808080']) {
    const { fondo, texto } = chapaDe(color)
    assert.ok(contraste(fondo, texto) >= 4.5, `${color}: ${fondo} con ${texto} da ${contraste(fondo, texto).toFixed(2)}`)
  }
  assert.deepEqual(chapaDe('#2563eb'), { fondo: '#2563eb', texto: '#ffffff' }, 'si ya llega, el color de la ficha no cambia')
  assert.equal(chapaDe('#f28a4f').texto, '#0a1116', 'sobre un color claro, texto oscuro')
  assert.notEqual(chapaDe('#c4561d').fondo, '#c4561d', 'el naranja de Claude se oscurece lo justo para el texto blanco')
})

test('modeloDe sin agente ni modelo da «Sin dato»; con uno de los dos, solo ese', () => {
  const trabajo = (agente?: string, modelo?: string) => ({ solicitud: { agente }, modelo_enviado: modelo }) as unknown as Trabajo
  assert.equal(modeloDe(trabajo()), t('comun.sinDato'))
  assert.equal(modeloDe(trabajo('claude')), 'Claude')
  assert.equal(modeloDe(trabajo(undefined, 'sonnet')), 'sonnet')
  assert.equal(modeloDe(trabajo('claude', 'sonnet')), 'Claude · sonnet')
})
