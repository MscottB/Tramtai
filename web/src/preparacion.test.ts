import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { accionPrincipal, confirmarDesinstalar, conReintentos, faltaSuscripcion, listoParaUsar, masNueva, opcionesParada, ordenSeguir, pideSeguir, prepararAgentes, resumenFinal, seguirPreparacion, siguienteTrasSaltar,
  versionNueva, vistaEspera, type Corrida } from './preparacion.ts'
import { fichas, type FichaAgente } from './agentes.ts'
import { t } from './i18n.ts'
import type { Adaptador, EstadoPreparacion, PedidoPreparacion, VersionesAgente } from './api/tipos.ts'

const ficha = (agente: string, nombre: string, autenticaciones: string[]) => ({ agente, nombre, autenticaciones } as unknown as FichaAgente)
fichas.value = [ficha('claude', 'Claude Code', ['suscripcion', 'clave_api']), ficha('antigravity', 'Antigravity', ['clave_api'])]

const agente = (estado: Adaptador['estado'], id = 'claude'): Adaptador =>
  ({ agente: id, estado, autenticacion: 'suscripcion', capacidades: {} } as unknown as Adaptador)
const versiones = (v: Partial<VersionesAgente>): VersionesAgente => ({ agente: 'claude', comprobadas: {}, acciones: [], ...v })
const instalado = versiones({ instalada: '2.1.288', ultima: '2.1.288', comprobadas: { actualizar: true, iniciar_sesion: true, desinstalar: true },
  acciones: ['actualizar', 'iniciar_sesion', 'desinstalar'] })

test('la acción principal de cada tarjeta sale de su estado y de sus versiones', () => {
  assert.deepEqual(accionPrincipal(agente('no_instalado'), versiones({ comprobadas: { instalar: true }, acciones: ['instalar'] })),
    { accion: 'instalar', etiqueta: 'preparar.instalar', aviso: undefined })
  assert.equal(accionPrincipal(agente('sin_sesion'), instalado).etiqueta, 'preparar.iniciarSesion')
  for (const estado of ['sesion_no_responde', 'usa_clave'] as const) {
    assert.deepEqual(accionPrincipal(agente(estado), instalado), { accion: 'iniciar_sesion', etiqueta: 'preparar.volverAIniciar', aviso: undefined }, estado)
  }
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

test('una versión no comprobada enseña el aviso; para iniciar sesión, sin el botón', () => {
  const motivo = 'Claude Code 2.2.0 aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, inicia sesión en una consola con claude auth login'
  const v = versiones({ instalada: '2.2.0', comprobadas: { actualizar: true, iniciar_sesion: false, desinstalar: true }, acciones: ['actualizar', 'desinstalar'],
    motivo_codigo: 'version_no_comprobada', motivo })
  assert.deepEqual(accionPrincipal(agente('sin_sesion'), v), { aviso: motivo })
  // Instalar o actualizar una no comprobada sí se puede, con el aviso de su acción delante (y «Seguir» en el panel); el
  // motivo del servicio (que puede ser el de iniciar sesión, o «no se pudo consultar…») no se usa.
  const instalar = versiones({ comprobadas: { instalar: false }, acciones: ['instalar'], motivo_codigo: 'sin_red', motivo: 'no se pudo consultar la última versión de Claude Code' })
  assert.deepEqual(accionPrincipal(agente('no_instalado'), instalar), { accion: 'instalar', etiqueta: 'preparar.instalar',
    aviso: t('preparar.noComprobadaTitulo', { agente: 'Claude Code' }) + '. ' + t('preparar.noComprobada.instalar') })
  // Antigravity no inicia sesión: el aviso no habla del inicio de sesión.
  assert.equal(accionPrincipal(agente('no_instalado', 'antigravity'), { ...instalar, agente: 'antigravity' }).aviso,
    t('preparar.noComprobadaTitulo', { agente: 'Antigravity' }) + '.')
  const actualizar = { ...instalado, ultima: '2.1.300', comprobadas: { ...instalado.comprobadas, actualizar: false }, motivo_codigo: 'version_no_comprobada' as const, motivo }
  assert.deepEqual(accionPrincipal(agente('listo'), actualizar), { accion: 'actualizar', etiqueta: 'preparar.actualizar', aviso: t('preparar.noComprobada.actualizar') })
  for (const p of [accionPrincipal(agente('listo'), actualizar), accionPrincipal(agente('no_instalado'), instalar)]) {
    assert.match(p.aviso ?? '', /^[A-ZÁÉÍÓÚ]/, 'sin minúscula inicial suelta')
  }
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
  assert.deepEqual(ordenSeguir(error('actualizar'), true), { accion: 'actualizar', borrar_datos: true, confirmar: true })
})

test('un listo que no inició sesión (versión sin comprobar) no cuenta como listo para usar', () => {
  assert.equal(listoParaUsar({ agente: 'claude', accion: 'instalar', estado: 'listo' }), true)
  assert.equal(listoParaUsar({ agente: 'claude', accion: 'instalar', estado: 'listo', motivo_codigo: 'version_no_comprobada', mensaje: '…' }), false)
  assert.equal(listoParaUsar({ agente: 'claude', accion: 'instalar', estado: 'error' }), false)
})

// conClave: el agente en modo clave de API tal como lo cuenta el servicio (falta_clave: sin clave guardada).
const conClave = (estado: Adaptador['estado'], id = 'claude'): Adaptador => ({ ...agente(estado, id), autenticacion: 'clave_api' })

test('con la sesión iniciada pero aún con clave de API y sin clave, falta usar la suscripción: no está listo (Ruling 34)', () => {
  const sesion: EstadoPreparacion = { agente: 'claude', accion: 'iniciar_sesion', estado: 'listo' }
  const instalado: EstadoPreparacion = { agente: 'claude', accion: 'instalar', estado: 'listo' }
  for (const e of [sesion, instalado]) {
    assert.equal(faltaSuscripcion(e, conClave('falta_clave')), true, e.accion)
    assert.equal(listoParaUsar(e, conClave('falta_clave')), false, e.accion)
  }
  assert.equal(listoParaUsar(sesion, conClave('listo')), true, 'con su clave guardada sí está listo')
  assert.equal(listoParaUsar(sesion, agente('listo')), true, 'ya en suscripción')
  assert.equal(listoParaUsar(sesion), true, 'sin saber cómo está el agente, como antes')
  assert.equal(faltaSuscripcion({ ...sesion, estado: 'error' }, conClave('falta_clave')), false)
  assert.equal(faltaSuscripcion({ ...sesion, accion: 'actualizar' }, conClave('falta_clave')), false)
  assert.equal(faltaSuscripcion({ ...instalado, agente: 'antigravity' }, conClave('falta_clave', 'antigravity')), false, 'Antigravity va con clave')
})

test('con esperando_usuario el panel enseña el enlace, el código y el campo para pegar; el enlace se abre aparte con «Abrir»', () => {
  const e: EstadoPreparacion = { agente: 'codex', accion: 'iniciar_sesion', estado: 'esperando_usuario', enlace: 'https://auth.openai.com/codex/device', codigo: 'ABCD-12345' }
  assert.deepEqual(vistaEspera(e), { enlace: 'https://auth.openai.com/codex/device', codigo: 'ABCD-12345', pegar: false })
  assert.equal(vistaEspera({ ...e, pide_pegar: true, codigo: undefined })?.pegar, true)
  assert.equal(vistaEspera({ ...e, enlace: 'javascript:alert(1)' })?.enlace, undefined, 'solo enlaces https')
  assert.equal(vistaEspera({ ...e, estado: 'comprobando' }), undefined)
  const panel = readFileSync(new URL('./componentes/PanelPreparacion.vue', import.meta.url), 'utf8')
  const abrir = panel.match(/<q-btn[^>]*:label="t\('preparar\.abrir'\)"[^>]*\/>/)?.[0] ?? ''
  assert.match(abrir, /:href="[^"]*enlace"/, 'el botón «Abrir» lleva el enlace')
  assert.match(abrir, /target="_blank" rel="noopener noreferrer"/)
  // El campo para pegar: un código de un solo uso (los gestores no lo ofrecen guardar), sin corrector.
  const campo = panel.match(/<q-input[^>]*v-model="pegado"[^>]*\/>/)?.[0] ?? ''
  assert.match(campo, /type="text"/)
  assert.match(campo, /autocomplete="one-time-code"/)
  assert.match(campo, /spellcheck="false"/)
  assert.match(panel, /role="status"/)
  assert.match(panel, /role="log"/)
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

// falsa es una API de preparación con un guion de estados por agente: el POST da el primero y cada GET el siguiente (el
// último se repite).
function falsa(guiones: Record<string, Array<Partial<EstadoPreparacion>>>, opciones: { conflicto?: string; fallosGet?: number } = {}) {
  const pedidos: Array<{ agente: string } & PedidoPreparacion> = []
  const cola: Record<string, EstadoPreparacion[]> = {}
  let fallos = opciones.fallosGet ?? 0
  const llenar = (agente: string, accion: EstadoPreparacion['accion']) => {
    cola[agente] = guiones[agente].map((g) => ({ agente, accion, estado: 'instalando', ...g } as EstadoPreparacion))
  }
  const siguiente = (agente: string) => (cola[agente].length > 1 ? cola[agente].shift()! : cola[agente][0])
  const api: Corrida['api'] = {
    async preparar(agente, pedido) {
      pedidos.push({ agente, ...pedido })
      llenar(agente, pedido.accion)
      if (opciones.conflicto === agente) throw Object.assign(new Error('ya'), { codigo: 'ya_en_curso', datos: { preparacion: siguiente(agente) } })
      return siguiente(agente)
    },
    async estado(agente) {
      if (fallos > 0) {
        fallos--
        throw new Error('sin conexión')
      }
      return cola[agente] ? siguiente(agente) : null
    },
  }
  return { api, pedidos }
}

function corrida(api: Corrida['api'], vivo = () => true) {
  const vistos: string[] = []
  const fallos: string[] = []
  const lanzadas: string[] = []
  const fines: string[] = []
  const c: Corrida = { api, esperar: async () => {}, vivo, cambio: (a, e) => vistos.push(`${a}:${e?.estado}`), fallo: (a, e) => fallos.push(`${a}:${(e as Error).message}`),
    lanzada: (a) => lanzadas.push(a), fin: (a, e) => fines.push(`${a}:${e?.estado ?? null}`) }
  return { c, vistos, fallos, lanzadas, fines }
}

test('varios agentes seguidos: se para en el primero que no acaba en listo y «Seguir» repite su acción con confirmar', async () => {
  const guiones: Record<string, Array<Partial<EstadoPreparacion>>> = {
    claude: [{ estado: 'instalando' }, { estado: 'listo' }],
    codex: [{ estado: 'instalando' }, { estado: 'error', motivo_codigo: 'version_no_comprobada' }],
    copilot: [{ estado: 'listo' }],
  }
  const { api, pedidos } = falsa(guiones)
  const { c, fines, lanzadas } = corrida(api)
  const agentes = ['claude', 'codex', 'copilot']
  assert.equal(await prepararAgentes(c, agentes, 0, { accion: 'instalar' }, { accion: 'instalar' }), 'parado')
  assert.deepEqual(fines, ['claude:listo', 'codex:error'])
  assert.deepEqual(lanzadas, ['claude', 'codex'], 'se avisa en cuanto el servicio acepta cada una')
  // «Seguir» desde codex: su acción (la del estado) con confirmar, y después el resto con su orden.
  guiones.codex = [{ estado: 'instalando' }, { estado: 'listo' }]
  assert.equal(await prepararAgentes(c, agentes, 1, ordenSeguir({ agente: 'codex', accion: 'instalar', estado: 'error' }), { accion: 'instalar' }), 'listo')
  assert.deepEqual(pedidos.map((p) => `${p.agente}:${p.accion}:${p.confirmar}`), ['claude:instalar:false', 'codex:instalar:false', 'codex:instalar:true', 'copilot:instalar:false'])
})

test('con otra en marcha (409 ya_en_curso) se sigue la del servicio', async () => {
  const { api, pedidos } = falsa({ claude: [{ estado: 'esperando_usuario', codigo: 'ABCD' }, { estado: 'listo' }] }, { conflicto: 'claude' })
  const { c, vistos, lanzadas, fallos } = corrida(api)
  assert.equal(await prepararAgentes(c, ['claude'], 0, { accion: 'iniciar_sesion' }, {}), 'listo')
  assert.equal(pedidos.length, 1)
  assert.deepEqual(vistos, ['claude:esperando_usuario', 'claude:listo'])
  assert.deepEqual(lanzadas, ['claude'])
  assert.deepEqual(fallos, [])
})

test('sin acción solo se sigue el estado, sin lanzar nada', async () => {
  const { api, pedidos } = falsa({ claude: [{ estado: 'listo' }] })
  await api.preparar('claude', { accion: 'actualizar', borrar_datos: false, confirmar: false })
  const { c, lanzadas } = corrida(api)
  assert.equal(await prepararAgentes(c, ['claude'], 0, {}, {}), 'listo')
  assert.equal(pedidos.length, 1)
  assert.deepEqual(lanzadas, [])
})

test('se corta cuando la pantalla ya no está, sin avisar de nada', async () => {
  const { api } = falsa({ claude: [{ estado: 'instalando' }, { estado: 'instalando' }, { estado: 'listo' }], codex: [{ estado: 'listo' }] })
  let vivo = true
  const { c, fines, fallos } = corrida(api, () => vivo)
  c.esperar = async () => { vivo = false }
  assert.equal(await prepararAgentes(c, ['claude', 'codex'], 0, { accion: 'instalar' }, { accion: 'instalar' }), 'cortado')
  assert.deepEqual(fines, [])
  assert.deepEqual(fallos, [])
})

test('un GET que falla se reintenta; si sigue fallando, se para con el error', async () => {
  let r = falsa({ claude: [{ estado: 'instalando' }, { estado: 'listo' }] }, { fallosGet: 2 })
  let k = corrida(r.api)
  assert.equal(await prepararAgentes(k.c, ['claude'], 0, { accion: 'instalar' }, {}), 'listo', 'dos fallos seguidos se superan')
  assert.deepEqual(k.fallos, [])
  r = falsa({ claude: [{ estado: 'instalando' }, { estado: 'listo' }] }, { fallosGet: 3 })
  k = corrida(r.api)
  assert.equal(await prepararAgentes(k.c, ['claude'], 0, { accion: 'instalar' }, {}), 'parado')
  assert.deepEqual(k.fallos, ['claude:sin conexión'])
  assert.deepEqual(k.fines, ['claude:null'])
  let intentos = 0
  await assert.rejects(conReintentos(async () => { intentos++; throw new Error('x') }, async () => {}), /x/)
  assert.equal(intentos, 3)
})

test('masNueva compara versiones por números', () => {
  assert.equal(masNueva('2.1.300', '2.1.288'), true)
  assert.equal(masNueva('2.10.0', '2.9.9'), true)
  assert.equal(masNueva('2.1.288', '2.1.288'), false)
  assert.equal(masNueva('0.167.2', '0.170.0'), false)
})

test('opcionesParada: con varios agentes se ofrece Reintentar y Saltar y se enseñan los pendientes; con uno, nada de eso', () => {
  assert.deepEqual(opcionesParada(['claude', 'codex', 'copilot'], 1), { reintentar: true, saltar: true, pendientes: ['copilot'] })
  assert.deepEqual(opcionesParada(['claude', 'codex'], 1), { reintentar: true, saltar: true, pendientes: [] })
  assert.deepEqual(opcionesParada(['claude'], 0), { reintentar: false, saltar: false, pendientes: [] })
  // desde la bienvenida (tanda), un solo agente también se reintenta, pero no se salta
  assert.deepEqual(opcionesParada(['claude'], 0, true), { reintentar: true, saltar: false, pendientes: [] })
})

test('siguienteTrasSaltar: el índice del siguiente, o fin si era el último', () => {
  assert.equal(siguienteTrasSaltar(0, 3), 1)
  assert.equal(siguienteTrasSaltar(1, 3), 2)
  assert.equal(siguienteTrasSaltar(2, 3), 'fin')
  assert.equal(siguienteTrasSaltar(0, 1), 'fin')
})

test('Reintentar vuelve a pedir el mismo agente y Saltar sigue con el siguiente', async () => {
  const agentes = ['claude', 'codex', 'copilot']
  const guiones: Record<string, Array<Partial<EstadoPreparacion>>> = {
    claude: [{ estado: 'listo' }],
    codex: [{ estado: 'error', motivo_codigo: 'sin_red' }],
    copilot: [{ estado: 'listo' }],
  }
  const { api, pedidos } = falsa(guiones)
  const { c, fines } = corrida(api)
  const orden = { accion: 'instalar', borrar_datos: false, confirmar: false } as const
  assert.equal(await prepararAgentes(c, agentes, 0, orden, orden), 'parado')
  // Reintentar: codex otra vez, ahora bien, y sigue con copilot.
  guiones.codex = [{ estado: 'listo' }]
  assert.equal(await prepararAgentes(c, agentes, 1, orden, orden), 'listo')
  assert.deepEqual(pedidos.map((p) => `${p.agente}:${p.confirmar}`), ['claude:false', 'codex:false', 'codex:false', 'copilot:false'])
  // Saltar: si codex falla de nuevo, se sigue desde copilot sin volver a pedir codex.
  guiones.codex = [{ estado: 'error' }]
  pedidos.length = 0
  fines.length = 0
  assert.equal(await prepararAgentes(c, agentes, 1, orden, orden), 'parado')
  assert.equal(await prepararAgentes(c, agentes, 2, orden, orden), 'listo')
  assert.deepEqual(pedidos.map((p) => p.agente), ['codex', 'copilot'])
  assert.deepEqual(fines, ['codex:error', 'copilot:listo'])
})

test('resumenFinal: separa los que quedaron listos de verdad de los que no, con el porqué', () => {
  const e = (agente: string, extra: Partial<EstadoPreparacion>) => ({ agente, accion: 'instalar', estado: 'listo', ...extra } as EstadoPreparacion)
  const r = resumenFinal(['claude', 'codex', 'copilot', 'antigravity'], {
    claude: e('claude', {}),
    codex: e('codex', { estado: 'error', motivo_codigo: 'sin_red' }),
    copilot: e('copilot', { motivo_codigo: 'version_no_comprobada', mensaje: 'Instalado, sin sesión.' }),
  })
  assert.deepEqual(r.listos, ['claude'])
  assert.deepEqual(r.pendientes.map((p) => p.agente), ['codex', 'copilot', 'antigravity'])
  assert.equal(r.pendientes[0].razon, t('preparar.error.sin_red'))
  assert.equal(r.pendientes[1].razon, 'Instalado, sin sesión.')
  assert.equal(r.pendientes[2].razon, t('bienvenida.saltado'))
})

test('resumenFinal: un agente que va con clave (Antigravity) instalado no está listo para usar: falta su clave', () => {
  const r = resumenFinal(['claude', 'antigravity'], {
    claude: { agente: 'claude', accion: 'instalar', estado: 'listo' } as EstadoPreparacion,
    antigravity: { agente: 'antigravity', accion: 'instalar', estado: 'listo' } as EstadoPreparacion,
  })
  assert.deepEqual(r.listos, ['claude'])
  assert.deepEqual(r.pendientes, [{ agente: 'antigravity', razon: t('bienvenida.faltaClave') }])
})

test('resumenFinal: con la sesión iniciada pero sin usar la suscripción falta un paso; con clave guardada, listo', () => {
  const e = (agente: string) => ({ agente, accion: 'instalar', estado: 'listo' } as EstadoPreparacion)
  const r = resumenFinal(['claude', 'codex'], { claude: e('claude'), codex: e('codex') }, {}, [conClave('falta_clave'), conClave('listo', 'codex')])
  assert.deepEqual(r.listos, ['codex'])
  assert.deepEqual(r.pendientes, [{ agente: 'claude', razon: t('bienvenida.faltaSuscripcion'), suscripcion: true }])
})

test('resumenFinal: no se lanzó (con su motivo) no es lo mismo que saltado', () => {
  const r = resumenFinal(['claude', 'codex'], {}, { claude: 'sin conexión' })
  assert.equal(r.pendientes[0].razon, t('bienvenida.noSeLanzo', { motivo: 'sin conexión' }))
  assert.equal(r.pendientes[1].razon, t('bienvenida.saltado'))
})

test('resumenFinal: sin enlace reconocido o con el inicio de sesión fallido, la razón es el mensaje con cómo hacerlo a mano', () => {
  const mensaje = 'no se reconoció lo que escribió Codex en 30 s: inicia sesión en una consola con codex login --device-auth'
  const r = resumenFinal(['codex', 'copilot', 'claude'], {
    codex: { agente: 'codex', accion: 'instalar', estado: 'error', motivo_codigo: 'salida_no_reconocida', mensaje } as EstadoPreparacion,
    copilot: { agente: 'copilot', accion: 'instalar', estado: 'error', motivo_codigo: 'inicio_sesion_fallo', mensaje: 'falló: usa su clave de API' } as EstadoPreparacion,
    claude: { agente: 'claude', accion: 'instalar', estado: 'error', motivo_codigo: 'inicio_sesion_fallo' } as EstadoPreparacion,
  })
  assert.deepEqual(r.pendientes.map((p) => p.razon), [mensaje, 'falló: usa su clave de API', t('preparar.error.inicio_sesion_fallo')])
})
