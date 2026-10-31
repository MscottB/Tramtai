// La pestaña Herramientas de Mis APIs: lo que decide (estado, interruptor, acciones, modelo inicial y llamadas al servicio)
// vive en herramientas.ts y se prueba aquí con fetch simulado, como cliente.test.ts; la plantilla se comprueba por su texto,
// porque estas pruebas no montan componentes.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { sesion } from '../../estado.ts'
import { idioma } from '../../i18n.ts'
import { fichas, opcionModelo, type FichaAgente } from '../../agentes.ts'
import { accionesDe, claseEstado, crearHerramientas, estaPuesta, modeloInicial, opcionesPermitidas, puedeActivarTodas, puedeCambiar, TEMA_AYUDA, textoEstado, textoMotivo, textoRestaurar } from '../../herramientas.ts'
import { temaPorId } from '../../ayuda/indice.ts'
import type { EstadoHerramienta, Modelo, Sesion } from '../../api/tipos.ts'

idioma.value = 'es' // el idioma inicial sale del sistema, y el runner del CI está en inglés
const h = (id: string, estado: string, resto: Partial<EstadoHerramienta> = {}): EstadoHerramienta =>
  ({ id, nombre: id, estado, clave_en_archivo: false, gestionado: false, hay_copia: false, ...resto })
const respuesta = (estado: number, cuerpo?: unknown) => new Response(cuerpo === undefined ? null : JSON.stringify(cuerpo), { status: estado })

// simular sustituye fetch: lista es lo que devuelve GET /api/v1/herramientas y f responde las demás rutas.
function simular(lista: EstadoHerramienta[], f?: (metodo: string, ruta: string) => Response | undefined) {
  sesion.value = { instancia: 'g', csrf: 'tk', version: 'v0', local: true } as Sesion
  const llamadas: { metodo: string; ruta: string; cuerpo?: unknown }[] = []
  globalThis.fetch = async (ruta, opciones) => {
    const metodo = opciones?.method ?? 'GET'
    llamadas.push({ metodo, ruta: String(ruta), cuerpo: opciones?.body ? JSON.parse(String(opciones.body)) : undefined })
    return f?.(metodo, String(ruta)) ?? respuesta(200, { herramientas: lista })
  }
  return llamadas
}

test('cada estado se dice con una palabra, fija en inglés, y no solo con color', () => {
  const palabras: Record<string, [string, string]> = {
    no_detectada: ['No detectada', 'Not detected'], detectada: ['Detectada', 'Detected'], activa: ['Activa', 'Active'],
    desactivada: ['Desactivada', 'Turned off'], desactualizada: ['Desactualizada', 'Out of date'], solo_global: ['Solo global', 'Global only'],
  }
  try {
    for (const [estado, [es, en]] of Object.entries(palabras)) {
      idioma.value = 'es'
      assert.equal(textoEstado(estado), es)
      idioma.value = 'en'
      assert.equal(textoEstado(estado), en)
    }
  } finally {
    idioma.value = 'es'
  }
  assert.deepEqual(['activa', 'desactualizada', 'detectada', 'desactivada', 'no_detectada', 'solo_global'].map(claseEstado),
    ['ok', 'aviso', 'info', 'neutro', 'neutro', 'neutro'])
})

test('la lista pinta una fila por herramienta con su estado en texto, el modelo y la ruta', () => {
  const fuente = readFileSync(new URL('./Herramientas.vue', import.meta.url), 'utf8')
  for (const parte of ['v-for="herramienta in lista"', 'class="etiqueta"', 'textoEstado(herramienta.estado)', 'herramienta.modelo', 'herramienta.ruta', '<q-toggle']) {
    assert.ok(fuente.includes(parte), parte)
  }
})

test('cargar trae la lista del servicio', async () => {
  const llamadas = simular([h('claude-code', 'activa'), h('codex', 'detectada')])
  const x = crearHerramientas()
  await x.cargar()
  assert.deepEqual(x.lista.value.map((e) => e.id), ['claude-code', 'codex'])
  assert.deepEqual(llamadas.map((l) => `${l.metodo} ${l.ruta}`), ['GET /api/v1/herramientas'])
  assert.equal(x.cargando.value, false)
})

test('el interruptor activa esa herramienta con el modelo elegido y vuelve a pedir la lista', async () => {
  const llamadas = simular([h('codex', 'activa', { modelo: 'claude/sonnet' })])
  const x = crearHerramientas()
  assert.equal(await x.activar(['codex'], 'claude/sonnet'), true)
  assert.deepEqual(llamadas[0], { metodo: 'POST', ruta: '/api/v1/herramientas/activar', cuerpo: { ids: ['codex'], todas: false, modelo: 'claude/sonnet' } })
  assert.equal(llamadas[1].metodo, 'GET')
  assert.equal(x.lista.value[0].estado, 'activa')
  assert.equal(x.ocupado.value, '')
})

test('«Activar todas» manda todas: true, y «Desactivar todas» también', async () => {
  const llamadas = simular([])
  const x = crearHerramientas()
  await x.activarTodas('claude/haiku')
  await x.desactivar([], true)
  assert.deepEqual(llamadas[0].cuerpo, { ids: [], todas: true, modelo: 'claude/haiku' })
  assert.deepEqual(llamadas.find((l) => l.ruta.endsWith('/desactivar'))?.cuerpo, { ids: [], todas: true })
})

test('apagar el interruptor desactiva solo esa herramienta', async () => {
  const llamadas = simular([h('codex', 'desactivada')])
  await crearHerramientas().desactivar(['codex'], false)
  assert.deepEqual(llamadas[0], { metodo: 'POST', ruta: '/api/v1/herramientas/desactivar', cuerpo: { ids: ['codex'], todas: false } })
})

test('olvidar y restaurar usan la ruta de la herramienta; restaurar da la copia', async () => {
  const llamadas = simular([], (metodo, ruta) => (ruta.endsWith('/restaurar') ? respuesta(200, { copia: 'C:\\copias\\codex-1.toml' }) : ruta.endsWith('/olvidar') ? respuesta(204) : undefined))
  const x = crearHerramientas()
  assert.equal(await x.olvidar('claude-code'), true)
  assert.equal(await x.restaurar('codex'), 'C:\\copias\\codex-1.toml')
  assert.deepEqual(llamadas.filter((l) => l.metodo === 'POST').map((l) => l.ruta), ['/api/v1/herramientas/claude-code/olvidar', '/api/v1/herramientas/codex/restaurar'])
})

test('una fila solo_global no se puede cambiar, ni una no detectada; las demás sí', () => {
  assert.equal(puedeCambiar(h('codex', 'solo_global')), false)
  assert.equal(puedeCambiar(h('codex', 'no_detectada')), false)
  for (const e of ['detectada', 'activa', 'desactivada', 'desactualizada']) assert.equal(puedeCambiar(h('codex', e)), true, e)
  assert.deepEqual(['activa', 'desactualizada', 'detectada', 'desactivada'].map((e) => estaPuesta(h('x', e))), [true, true, false, false])
  const fuente = readFileSync(new URL('./Herramientas.vue', import.meta.url), 'utf8')
  assert.ok(fuente.includes(':disable="!puedeCambiar(herramienta)') && fuente.includes("t('herramientas.soloGlobal')"))
})

test('el menú ofrece Cambiar modelo solo si está puesta, Olvidar si Tramtai sabe de ella y Restaurar al final solo con copia', () => {
  assert.deepEqual(accionesDe(h('a', 'activa', { hay_copia: true })), ['modelo', 'olvidar', 'restaurar'])
  assert.deepEqual(accionesDe(h('a', 'desactualizada')), ['modelo', 'olvidar'])
  assert.deepEqual(accionesDe(h('a', 'desactivada')), ['olvidar'])
  for (const e of ['detectada', 'no_detectada', 'solo_global']) assert.deepEqual(accionesDe(h('a', e)), [], e)
})

test('con el servicio en modo servidor (409) la pestaña dice que no está disponible', async () => {
  simular([], () => respuesta(409, { error: { codigo: 'no_disponible_en_servidor', mensaje: 'las herramientas no están disponibles en el modo servidor' } }))
  const x = crearHerramientas()
  await x.cargar()
  assert.equal(x.noDisponible.value, true)
  assert.equal(x.error.value, '')
  assert.deepEqual(x.lista.value, [])
  const fuente = readFileSync(new URL('./Herramientas.vue', import.meta.url), 'utf8')
  assert.ok(fuente.includes("t('herramientas.servidor')"))
})

test('otro fallo al cargar sin lista deja el error para Reintentar, sin decir que es el servidor', async () => {
  simular([], () => respuesta(500, { error: { codigo: 'error_interno', mensaje: 'se rompió' } }))
  const x = crearHerramientas()
  await x.cargar()
  assert.equal(x.noDisponible.value, false)
  assert.equal(x.error.value, 'se rompió')
})

const ficha = (agente: string, extra: Partial<FichaAgente> = {}): FichaAgente => ({ agente, nombre: agente, programa: agente, codigo: agente.slice(0, 2), color: { claro: '#000', oscuro: '#fff' },
  orden: 0, streaming: '', salida_estructurada: false, modelo_efectivo_informado: false, esfuerzos: [], autenticaciones: [], variables_clave: [], permitido_apps: true,
  url_instalacion: '', ...extra })
const modelo = (id: string, extra: Partial<Modelo> = {}): Modelo => ({ id, agente: id.split('/')[0], ...extra }) as Modelo

test('el modelo inicial es el primer agente conectado y permitido en APIs, en el orden de las fichas', () => {
  fichas.value = [ficha('claude', { desconectado: true }), ficha('codex'), ficha('copilot')]
  const opciones = ['claude', 'claude/sonnet', 'codex', 'codex/gpt-5', 'copilot'].map((id) => opcionModelo(modelo(id)))
  assert.equal(modeloInicial(opciones), 'codex')
  fichas.value = [ficha('claude', { permitido_apps: false }), ficha('codex', { permitido_apps: false })]
  assert.equal(modeloInicial(opciones.filter((o) => o.grupo !== 'codex' && o.grupo !== 'claude')), 'copilot')
  assert.equal(modeloInicial([]), '')
  fichas.value = []
})

test('el selector no ofrece agentes que no se permiten en APIs', () => {
  fichas.value = [ficha('claude'), ficha('antigravity', { permitido_apps: false })]
  const todas = ['claude/sonnet', 'antigravity/pro', 'perfil/rapido'].map((id) => opcionModelo(modelo(id, id.startsWith('perfil') ? { agente: undefined, perfil: 'rapido' } : {})))
  assert.deepEqual(opcionesPermitidas(todas).map((o) => o.value), ['claude/sonnet', 'perfil/rapido'])
  fichas.value = []
})

test('la plantilla usa el selector de Probar, un diálogo persistente para restaurar y el estado vacío con la Ayuda', () => {
  const fuente = readFileSync(new URL('./Herramientas.vue', import.meta.url), 'utf8')
  for (const parte of ['SelectorModelo', 'persistent', 'escaparConAviso(', 'EstadoVacio', 'BotonAyuda', 'hay_copia', 'text-negative', "t('herramientas.claveEnArchivo')"]) {
    assert.ok(fuente.includes(parte), parte)
  }
  const pagina = readFileSync(new URL('../../paginas/Accesos.vue', import.meta.url), 'utf8')
  assert.ok(pagina.includes('name="herramientas"') && pagina.includes("'apis' | 'principal' | 'herramientas'") && pagina.includes('<Herramientas'))
})

test('el motivo se dice en el idioma de la interfaz según su código, con sus datos, y si no hay texto para el código se queda el del servicio', () => {
  const puerto = h('codex', 'desactualizada', { motivo: 'Tramtai usa ahora el puerto 2 y la herramienta apunta al 1: vuelve a activarla', motivo_codigo: 'puerto', motivo_datos: { actual: '2', anotado: '1' } })
  const gestionada = h('claude-code', 'activa', { gestionado: true, motivo: 'una política de la organización manda…', motivo_codigo: 'gestionado' })
  try {
    idioma.value = 'es'
    assert.equal(textoMotivo(puerto), 'Tramtai usa ahora el puerto 2 y la herramienta apunta al 1: vuelve a activarla.')
    idioma.value = 'en'
    assert.equal(textoMotivo(puerto), 'Tramtai now uses port 2 and the tool points to 1: turn it on again.')
    assert.ok(textoMotivo(gestionada).startsWith('An organization policy'))
    // un código que la Web UI no conoce, o sin código (un servicio anterior): el texto del servicio
    assert.equal(textoMotivo(h('a', 'detectada', { motivo: 'texto del servicio', motivo_codigo: 'previa' })), 'texto del servicio')
    assert.equal(textoMotivo(h('a', 'detectada', { motivo: 'texto del servicio' })), 'texto del servicio')
    assert.equal(textoMotivo(h('a', 'detectada')), '')
  } finally {
    idioma.value = 'es'
  }
})

test('cada código de motivo que pone el servicio tiene su texto en español y en inglés', () => {
  const codigos = ['gestionado', 'sin_https_propio', 'api_no_existe', 'api_detenida', 'puerto', 'orden_no_existe', 'clave_no_almacen', 'editada_a_mano', 'otro_archivo', 'anotacion_ilegible', 'continue_json']
  const servicio = readFileSync(new URL('../../../../internal/servicio/herramientas.go', import.meta.url), 'utf8')
  try {
    for (const c of codigos) {
      assert.ok(servicio.includes(`"${c}"`), `el servicio no pone ${c}`)
      for (const l of ['es', 'en'] as const) {
        idioma.value = l
        const datos = { api: 'A', ruta: 'R', actual: '1', anotado: '2', detalle: 'D' }
        assert.notEqual(textoMotivo(h('a', 'activa', { motivo: 'x', motivo_codigo: c, motivo_datos: datos })), 'x', `${c} en ${l}`)
      }
    }
  } finally {
    idioma.value = 'es'
  }
})

test('restaurar dice de qué fecha es la copia y, sin fecha, no la inventa', () => {
  const con = h('codex', 'activa', { hay_copia: true, fecha_copia: '2026-11-05T14:05:00Z' })
  const sin = h('codex', 'activa', { hay_copia: true })
  try {
    for (const l of ['es', 'en'] as const) {
      idioma.value = l
      const texto = textoRestaurar(con)
      assert.ok(texto.includes(new Date('2026-11-05T14:05:00Z').toLocaleString(l)) && !texto.includes('{fecha}'), texto)
      assert.ok(!textoRestaurar(sin).includes('{fecha}'))
      assert.notEqual(textoRestaurar(sin), texto)
    }
  } finally {
    idioma.value = 'es'
  }
})

test('«Activar todas» solo se ofrece si alguna fila se puede activar o reactivar', () => {
  assert.equal(puedeActivarTodas([h('a', 'activa'), h('b', 'no_detectada'), h('c', 'solo_global')]), false, 'todas activas o sin archivo')
  for (const e of ['detectada', 'desactivada', 'desactualizada']) assert.equal(puedeActivarTodas([h('a', 'activa'), h(e, e)]), true, e)
  assert.equal(puedeActivarTodas([]), false)
})

test('activar todas de principio a fin: se elige el modelo una vez y se manda todas: true', async () => {
  const llamadas = simular([h('codex', 'activa'), h('continue', 'activa')])
  fichas.value = [ficha('claude'), ficha('codex')]
  const opciones = ['claude', 'claude/sonnet', 'codex'].map((id) => opcionModelo(modelo(id)))
  const x = crearHerramientas()
  x.pedirModelo(opciones)
  assert.equal(x.dialogoModelo.value, true)
  assert.deepEqual(x.objetivo.value, { ids: [], todas: true, cambiando: false, nombre: '' })
  assert.equal(x.modelo.value, 'claude')
  x.modelo.value = 'claude/sonnet'
  assert.equal(await x.confirmarModelo(), true)
  assert.equal(x.dialogoModelo.value, false)
  assert.deepEqual(llamadas[0], { metodo: 'POST', ruta: '/api/v1/herramientas/activar', cuerpo: { ids: [], todas: true, modelo: 'claude/sonnet' } })
  fichas.value = []
})

test('el interruptor de una activa pide cambiar de modelo con el suyo; el de una detectada, activar con el primero conectado', async () => {
  const llamadas = simular([h('codex', 'activa', { modelo: 'claude/haiku' }), h('continue', 'detectada')])
  fichas.value = [ficha('codex')]
  const opciones = ['codex', 'claude/haiku'].map((id) => opcionModelo(modelo(id)))
  const x = crearHerramientas()
  await x.cargar()
  x.pedirModelo(opciones, ['codex'], 'claude/haiku')
  assert.deepEqual(x.objetivo.value, { ids: ['codex'], todas: false, cambiando: true, nombre: 'codex' })
  assert.equal(x.modelo.value, 'claude/haiku')
  x.pedirModelo(opciones, ['continue'])
  assert.deepEqual(x.objetivo.value, { ids: ['continue'], todas: false, cambiando: false, nombre: 'continue' })
  assert.equal(x.modelo.value, 'codex')
  await x.confirmarModelo()
  assert.deepEqual(llamadas.find((l) => l.metodo === 'POST')?.cuerpo, { ids: ['continue'], todas: false, modelo: 'codex' })
  fichas.value = []
})

test('lo que cambia en una herramienta avisa a Mis APIs; un fallo no', async () => {
  let avisos = 0
  simular([h('codex', 'activa')])
  const x = crearHerramientas(() => avisos++)
  await x.activar(['codex'], 'claude/haiku')
  await x.desactivar(['codex'], false)
  await x.olvidar('codex')
  assert.equal(avisos, 3)
  simular([h('codex', 'activa')], (metodo) => (metodo === 'POST' ? respuesta(400, { error: { codigo: 'solicitud_invalida', mensaje: 'no' } }) : undefined))
  // sin lista aún, el fallo queda en error (con lista sería un aviso de Quasar, que estas pruebas no montan)
  const y = crearHerramientas(() => avisos++)
  assert.equal(await y.activar(['codex'], 'x'), false)
  assert.equal(avisos, 3)
})

test('el tema de Ayuda de la pestaña existe', () => {
  assert.ok(temaPorId(TEMA_AYUDA), TEMA_AYUDA)
})

test('la plantilla avisa del cambio, olvida en color de error y usa lo que decide herramientas.ts', () => {
  const fuente = readFileSync(new URL('./Herramientas.vue', import.meta.url), 'utf8')
  for (const parte of ["emit('cambio')", "color: 'negative', label: t('herramientas.olvidar')", 'puedeActivarTodas(lista)', 'textoMotivo(herramienta)', 'textoRestaurar(aRestaurar']) {
    assert.ok(fuente.includes(parte), parte)
  }
  assert.ok(readFileSync(new URL('../../paginas/Accesos.vue', import.meta.url), 'utf8').includes('@cambio="cargarPuntos"'))
})
