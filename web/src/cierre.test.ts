import { test } from 'node:test'
import assert from 'node:assert/strict'
import { Dialog } from 'quasar'
import { escaparConAviso } from './cierre.ts'

// El Esc global de Quasar: escucha keydown y keyup en window y cierra el diálogo de más arriba. Se carga el módulo de
// verdad sobre un window falso para que la prueba recorra el mismo camino que el navegador.
const ventana = new EventTarget()
Object.assign(globalThis, { window: ventana })
const { addEscapeKey, removeEscapeKey } = await import('quasar/src/utils/private.keyboard/escape-key.js')

const tecla = (tipo: 'keydown' | 'keyup') => Object.assign(new Event(tipo, { cancelable: true }), { key: 'Escape', keyCode: 27 })

// pulsarEsc: el keydown sube hasta window; el keyup nace en la tarjeta del diálogo y sube a window salvo que alguien
// pare la propagación.
function pulsarEsc(tarjeta: EventTarget) {
  ventana.dispatchEvent(tecla('keydown'))
  const soltar = tecla('keyup')
  tarjeta.dispatchEvent(soltar)
  if (!soltar.cancelBubble) ventana.dispatchEvent(tecla('keyup'))
}

// montar: un diálogo persistent (Quasar lo sacude con Esc) con la tarjeta que llama a escaparConAviso, y el aviso
// que abriría Dialog.create: un diálogo normal, que Quasar cierra con Esc.
function montar(perderia: boolean) {
  const estado = { cerrado: 0, sacudidas: 0, avisos: 0, avisoAbierto: false }
  const sacudir = () => estado.sacudidas++
  addEscapeKey(sacudir)
  Object.assign(Dialog, {
    create: () => {
      estado.avisos++
      estado.avisoAbierto = true
      const cancelar = () => {
        estado.avisoAbierto = false
        removeEscapeKey(cancelar)
      }
      addEscapeKey(cancelar)
      return { onOk: () => undefined }
    },
  })
  const tarjeta = new EventTarget()
  tarjeta.addEventListener('keyup', (e) => escaparConAviso(e, perderia, () => estado.cerrado++))
  return { estado, tarjeta, desmontar: () => removeEscapeKey(sacudir) }
}

test('Esc cierra el diálogo sin preguntar si no se pierde nada', () => {
  const { estado, tarjeta, desmontar } = montar(false)
  pulsarEsc(tarjeta)
  assert.deepEqual(estado, { cerrado: 1, sacudidas: 0, avisos: 0, avisoAbierto: false })
  desmontar()
})

test('Esc con algo que se perdería abre el aviso y el mismo keyup no lo cierra', () => {
  const { estado, tarjeta, desmontar } = montar(true)
  pulsarEsc(tarjeta)
  assert.deepEqual(estado, { cerrado: 0, sacudidas: 0, avisos: 1, avisoAbierto: true })
  // un segundo Esc, ya dentro del aviso, lo cancela y el diálogo sigue abierto
  ventana.dispatchEvent(tecla('keydown'))
  ventana.dispatchEvent(tecla('keyup'))
  assert.deepEqual(estado, { cerrado: 0, sacudidas: 0, avisos: 1, avisoAbierto: false })
  desmontar()
})
