import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { accionPrincipal, confirmarDesinstalar, conReintentos, listoParaUsar, masNueva, ordenSeguir, pideSeguir, prepararAgentes, seguirPreparacion,
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
  const motivo = 'Claude Code 2.2.0 aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, puedes hacerlo en una consola con claude auth login'
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
