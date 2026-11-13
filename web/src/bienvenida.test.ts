// Cuándo se ofrece la bienvenida: solo con sesión, en Inicio, sin enlace profundo, sin respuesta guardada y una vez por carga.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { alCerrarBienvenida, entradaPorEnlace, leerBienvenida, ofrecerBienvenida, opcionesAgentes } from './bienvenida.ts'

const limpio = { sesion: true, pagina: 'inicio', entradaPorEnlace: false, estado: null, yaOfrecida: false, otroDialogo: false, recorridoActivo: false, agentesConocidos: true, plazoAgotado: false } as const

test('ofrecerBienvenida: el caso limpio la ofrece', () => {
  assert.equal(ofrecerBienvenida(limpio), true)
})

test('ofrecerBienvenida: cada condición que falta la impide', () => {
  assert.equal(ofrecerBienvenida({ ...limpio, sesion: false }), false, 'sin sesión')
  assert.equal(ofrecerBienvenida({ ...limpio, pagina: 'agentes' }), false, 'otra página')
  assert.equal(ofrecerBienvenida({ ...limpio, entradaPorEnlace: true }), false, 'enlace profundo')
  assert.equal(ofrecerBienvenida({ ...limpio, estado: 'hecha' }), false, 'ya hecha')
  assert.equal(ofrecerBienvenida({ ...limpio, estado: 'descartada' }), false, 'descartada')
  assert.equal(ofrecerBienvenida({ ...limpio, yaOfrecida: true }), false, 'ya ofrecida en esta carga')
  assert.equal(ofrecerBienvenida({ ...limpio, otroDialogo: true }), false, 'otro diálogo abierto (Novedades, Actualizar…)')
})

test('ofrecerBienvenida: con un recorrido en marcha no se ofrece, y al acabar sí (un recorrido de página que cruza Inicio)', () => {
  assert.equal(ofrecerBienvenida({ ...limpio, recorridoActivo: true }), false)
  assert.equal(ofrecerBienvenida({ ...limpio, recorridoActivo: false }), true)
})

test('ofrecerBienvenida: al cerrarse el otro diálogo se ofrece', () => {
  assert.equal(ofrecerBienvenida({ ...limpio, otroDialogo: true }), false)
  assert.equal(ofrecerBienvenida({ ...limpio, otroDialogo: false }), true)
})

test('leerBienvenida: solo «hecha» y «descartada» cuentan; cualquier otra cosa (o nada) la vuelve a ofrecer', () => {
  assert.equal(leerBienvenida('hecha'), 'hecha')
  assert.equal(leerBienvenida('descartada'), 'descartada')
  for (const v of [null, '', 'HECHA', '1']) assert.equal(leerBienvenida(v), null, String(v))
})

test('alCerrarBienvenida: «Ahora no», Esc o clic fuera la descartan; tras «Empezar» ya quedó hecha', () => {
  assert.equal(alCerrarBienvenida(false), 'descartada')
  assert.equal(alCerrarBienvenida(true), undefined)
})

test('entradaPorEnlace: solo un hash que no sea vacío ni #inicio es un enlace profundo', () => {
  for (const hash of ['', '#', '#inicio']) assert.equal(entradaPorEnlace(hash), false, hash)
  for (const hash of ['#agentes', '#ayuda/archivos', '#conectar']) assert.equal(entradaPorEnlace(hash), true, hash)
})

const ad = (agente: string, estado: string) => ({ agente, estado }) as never

test('opcionesAgentes: sin ningún agente listo se ofrece prepararlos, con Claude Code y Codex marcados; Copilot y Antigravity, sin marcar', () => {
  const todos = ['claude', 'codex', 'copilot', 'antigravity', 'gemini'].map((a) => ad(a, 'no_instalado'))
  assert.deepEqual(opcionesAgentes(todos), [
    { agente: 'claude', marcado: true }, { agente: 'codex', marcado: true }, { agente: 'copilot', marcado: false }, { agente: 'antigravity', marcado: false },
  ])
})

test('opcionesAgentes: con uno listo, o sin saber aún cómo están, no se ofrece nada', () => {
  assert.deepEqual(opcionesAgentes([ad('claude', 'listo'), ad('codex', 'no_instalado')]), [])
  assert.deepEqual(opcionesAgentes(null), [])
  assert.deepEqual(opcionesAgentes([]), [])
})

test('opcionesAgentes: solo los agentes que el servicio conoce', () => {
  assert.deepEqual(opcionesAgentes([ad('claude', 'sin_sesion')]).map((o) => o.agente), ['claude'])
})

test('ofrecerBienvenida: espera a conocer los agentes, y pasado el plazo se abre sin la oferta', () => {
  assert.equal(ofrecerBienvenida({ ...limpio, agentesConocidos: false }), false, 'aún no se sabe cómo están los agentes')
  assert.equal(ofrecerBienvenida({ ...limpio, agentesConocidos: false, plazoAgotado: true }), true, 'la comprobación no llegó a tiempo')
})
