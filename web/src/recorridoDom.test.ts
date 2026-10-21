// La parte del motor de los recorridos que no necesita navegador: qué elemento se señala y qué dice cada paso.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { aDriver, botonConFoco, capturarEsc, cerrarAlNavegar, elegirElemento, esperarAncla, EVENTO_PASO, focoAlCerrar, OPCIONES_DRIVER } from './recorridoDom.ts'
import { escDelPanel } from './ayuda/panel.ts'
import type { PasoMostrado } from './recorrido.ts'

// elemento falso con el tamaño que devolvería getBoundingClientRect().
const elemento = (width: number, height: number) => ({ getBoundingClientRect: () => ({ width, height }) }) as unknown as Element
// raiz falsa: cada ancla devuelve sus elementos, como querySelectorAll('[data-recorrido="…"]').
const raiz = (anclas: Record<string, Element[]>) => ({
  querySelectorAll: (selector: string) => anclas[/data-recorrido="([^"]+)"/.exec(selector)?.[1] ?? ''] ?? [],
}) as unknown as Pick<Document, 'querySelectorAll'>

test('elegirElemento devuelve el primer elemento visible del ancla', () => {
  const visible = elemento(40, 20)
  assert.equal(elegirElemento('app.ayuda', raiz({ 'app.ayuda': [visible] })), visible)
  // una cabecera oculta (0×0) no gana a la visible que viene detrás
  assert.equal(elegirElemento('app.ayuda', raiz({ 'app.ayuda': [elemento(0, 0), visible] })), visible)
})

test('elegirElemento sin ancla, sin elemento o sin tamaño da undefined: el paso sale centrado', () => {
  assert.equal(elegirElemento(undefined, raiz({})), undefined)
  assert.equal(elegirElemento('inicio.mapa', raiz({})), undefined)
  assert.equal(elegirElemento('inicio.mapa', raiz({ 'inicio.mapa': [elemento(0, 0)] })), undefined)
  assert.equal(elegirElemento('inicio.mapa', raiz({ 'inicio.mapa': [elemento(120, 0)] })), undefined)
})

// tecla falsa: un Event con la tecla, que pasa por el mismo EventTarget que el window del navegador.
const tecla = (tipo: 'keydown' | 'keyup', key: string) => Object.assign(new Event(tipo, { cancelable: true }), { key })

test('capturarEsc: Esc es solo del recorrido; nadie de debajo (panel, diálogo) la recibe, y se cierra al soltarla', () => {
  const ventana = new EventTarget()
  const vistas: string[] = []
  let cierres = 0
  const quitar = capturarEsc(ventana, () => cierres++)
  // lo que Quasar escucha en window para cerrar su panel o su diálogo, registrado después
  ventana.addEventListener('keydown', (e) => vistas.push(e.type))
  ventana.addEventListener('keyup', (e) => vistas.push(e.type))
  ventana.dispatchEvent(tecla('keydown', 'Escape'))
  assert.equal(cierres, 0) // al pulsar aún no se cierra
  ventana.dispatchEvent(tecla('keyup', 'Escape'))
  assert.equal(cierres, 1)
  assert.deepEqual(vistas, [])
  // las demás teclas pasan (las flechas son de driver.js)
  ventana.dispatchEvent(tecla('keyup', 'ArrowRight'))
  assert.deepEqual(vistas, ['keyup'])
  assert.equal(cierres, 1)
  // al terminar el recorrido se deja de capturar: Esc vuelve a ser del panel
  quitar()
  ventana.dispatchEvent(tecla('keyup', 'Escape'))
  assert.equal(cierres, 1)
  assert.deepEqual(vistas, ['keyup', 'keyup'])
})

test('capturarEsc con el panel «?» abierto: su listener de captura va antes, pero con el recorrido en marcha deja pasar Esc', () => {
  const ventana = new EventTarget()
  const panel = { abierto: true, cierres: 0 }
  let recorridoActivo = true
  // como PanelAyuda: registrado al montar la app, antes que el del recorrido, en la misma fase de captura
  const delPanel = (e: Event) => {
    if (!escDelPanel(e as KeyboardEvent, panel.abierto, recorridoActivo, false)) return
    e.stopImmediatePropagation()
    if (e.type === 'keyup') panel.cierres++
  }
  ventana.addEventListener('keydown', delPanel, true)
  ventana.addEventListener('keyup', delPanel, true)
  let cierres = 0
  const quitar = capturarEsc(ventana, () => cierres++)
  ventana.dispatchEvent(tecla('keydown', 'Escape'))
  ventana.dispatchEvent(tecla('keyup', 'Escape'))
  assert.deepEqual([cierres, panel.cierres], [1, 0]) // cierra el recorrido; el panel sigue abierto
  quitar()
  recorridoActivo = false
  ventana.dispatchEvent(tecla('keyup', 'Escape'))
  assert.deepEqual([cierres, panel.cierres], [1, 1]) // sin recorrido, Esc vuelve a ser del panel
})

test('escDelPanel: el panel se queda Esc solo abierto, sin recorrido y sin un menú desplegado encima', () => {
  const esc = tecla('keyup', 'Escape') as unknown as KeyboardEvent
  assert.equal(escDelPanel(esc, true, false, false), true)
  assert.equal(escDelPanel(esc, false, false, false), false)
  assert.equal(escDelPanel(esc, true, true, false), false)
  assert.equal(escDelPanel(esc, true, false, true), false)
  assert.equal(escDelPanel(tecla('keyup', 'Enter') as unknown as KeyboardEvent, true, false, false), false)
})

test('esperarAncla vuelve en cuanto aparece el ancla, a los 5 s o en cuanto deja de hacer falta', async () => {
  let esperas = 0
  const dormir = async () => void esperas++
  // aparece a la tercera mirada
  let miradas = 0
  await esperarAncla(() => ++miradas >= 3, () => true, dormir)
  assert.equal(esperas, 2)
  // nunca aparece: 5 s en pasos de 100 ms
  esperas = 0
  await esperarAncla(() => false, () => true, dormir)
  assert.equal(esperas, 50)
  // se pidió otro paso o se cerró el recorrido: no quedan esperas vivas
  esperas = 0
  await esperarAncla(() => false, () => esperas < 2, dormir)
  assert.equal(esperas, 2)
})

// ventana falsa de driver.js: tres botones con su clase.
const boton = (clase: string) => ({ clase, classList: { contains: (c: string) => c === clase } }) as unknown as HTMLElement
const ventana = { nextButton: boton('driver-popover-next-btn'), previousButton: boton('driver-popover-prev-btn'), closeButton: boton('driver-popover-close-btn') }

test('botonConFoco: un paso nuevo enfoca Siguiente; el mismo paso repintado deja el foco en el botón que lo tenía', () => {
  assert.equal(botonConFoco(ventana, boton('driver-popover-prev-btn'), false), ventana.nextButton)
  assert.equal(botonConFoco(ventana, boton('driver-popover-prev-btn'), true), ventana.previousButton)
  assert.equal(botonConFoco(ventana, boton('driver-popover-close-btn'), true), ventana.closeButton)
  assert.equal(botonConFoco(ventana, boton('driver-popover-next-btn'), true), ventana.nextButton)
  assert.equal(botonConFoco(ventana, null, true), ventana.nextButton)
})

test('EVENTO_PASO es el aviso que escucha Ajustes para abrir la pestaña del paso', () => {
  assert.equal(EVENTO_PASO, 'tramtai:recorrido-paso')
})

test('focoAlCerrar: vuelve a lo que abrió el recorrido si sigue en la página; si no, al «?» de la página', () => {
  const ayuda = elemento(32, 32)
  const boton = { isConnected: true } as unknown as Element
  assert.equal(focoAlCerrar(boton, raiz({ 'app.ayuda': [ayuda] })), boton)
  // el botón de la bienvenida, de «Haz el recorrido» o de la página de Ayuda ya no está: no se deja el foco en body
  const quitado = { isConnected: false } as unknown as Element
  assert.equal(focoAlCerrar(quitado, raiz({ 'app.ayuda': [ayuda] })), ayuda)
  assert.equal(focoAlCerrar(null, raiz({ 'app.ayuda': [ayuda] })), ayuda)
})

test('OPCIONES_DRIVER: solo enseña, el elemento resaltado no se puede pulsar ni enfocar con Tab', () => {
  assert.equal(OPCIONES_DRIVER.disableActiveInteraction, true)
  assert.equal(OPCIONES_DRIVER.allowKeyboardControl, true)
})

test('cerrarAlNavegar: Atrás o Alt+← (popstate) cierra el recorrido; al soltarlo deja de escuchar', () => {
  const ventana = new EventTarget()
  let cierres = 0
  const quitar = cerrarAlNavegar(ventana, () => cierres++)
  ventana.dispatchEvent(new Event('popstate'))
  assert.equal(cierres, 1)
  quitar()
  ventana.dispatchEvent(new Event('popstate'))
  assert.equal(cierres, 1)
})

const paso: PasoMostrado = { pagina: 'inicio', ancla: 'inicio.mapa', titulo: 'comun.aceptar', texto: 'comun.cancelar' }

test('aDriver traduce con la t que recibe: el idioma se resuelve al mostrar el paso', () => {
  const es = aDriver(paso, 0, 3, (k) => `es:${k}`).popover
  const en = aDriver(paso, 0, 3, (k) => `en:${k}`).popover
  assert.equal(es?.title, 'es:comun.aceptar')
  assert.equal(es?.description, 'es:comun.cancelar')
  assert.equal(en?.title, 'en:comun.aceptar')
  assert.equal(es?.prevBtnText, 'es:recorrido.anterior')
  assert.equal(es?.closeBtnLabel, 'es:recorrido.cerrar')
})

test('aDriver pone el contador «3 de 14» y «Hecho» solo en el último paso', () => {
  const t = (k: string, p?: Record<string, string | number>) => (k === 'recorrido.progreso' ? `${p?.actual} de ${p?.total}` : k)
  assert.equal(aDriver(paso, 2, 14, t).popover?.progressText, '3 de 14')
  assert.equal(aDriver(paso, 2, 14, t).popover?.nextBtnText, 'recorrido.siguiente')
  assert.equal(aDriver(paso, 13, 14, t).popover?.nextBtnText, 'recorrido.hecho')
})
