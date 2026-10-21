// La parte del motor de los recorridos que se prueba sin navegador: qué elemento señala un paso y cómo se le pasa a
// driver.js. useRecorrido (composables) la usa para mostrar los pasos.
import type { DriveStep } from 'driver.js'
import type { Clave } from './idiomas/es'
import type { PasoMostrado } from './recorrido'

type Traductor = (clave: Clave, parametros?: Record<string, string | number>) => string

// elegirElemento es el primer elemento visible con ese data-recorrido: app.ayuda está en la cabecera de cada página y
// una oculta no debe ganar. Sin ancla o sin nada visible da undefined, y el paso sale centrado sin resaltar ningún hueco.
export function elegirElemento(ancla: string | undefined, raiz: Pick<Document, 'querySelectorAll'>): Element | undefined {
  if (!ancla) return undefined
  return [...raiz.querySelectorAll(`[data-recorrido="${ancla}"]`)].find(seVe)
}

const seVe = (e: Element) => {
  const { width, height } = e.getBoundingClientRect()
  return width > 0 && height > 0
}

// capturarEsc deja Esc solo al recorrido mientras dura: en la fase de captura la detiene antes de que llegue al panel «?» o
// a un diálogo de debajo (Quasar los cierra con Esc desde window) y al soltarla llama a alEsc. Devuelve cómo dejar de capturar.
export function capturarEsc(destino: Pick<EventTarget, 'addEventListener' | 'removeEventListener'>, alEsc: () => void): () => void {
  const alTeclear = (e: Event) => {
    if ((e as KeyboardEvent).key !== 'Escape') return
    e.stopImmediatePropagation()
    if (e.type === 'keyup') alEsc()
  }
  destino.addEventListener('keydown', alTeclear, true)
  destino.addEventListener('keyup', alTeclear, true)
  return () => {
    destino.removeEventListener('keydown', alTeclear, true)
    destino.removeEventListener('keyup', alTeclear, true)
  }
}

// EVENTO_PASO avisa en window del ancla de cada paso antes de buscarla: Ajustes abre la pestaña donde está.
export const EVENTO_PASO = 'tramtai:recorrido-paso'

const ESPERA_MS = 5000
const CADA_MS = 100
const dormirMs = (ms: number) => new Promise<void>((listo) => setTimeout(listo, ms))

// esperarAncla vuelve en cuanto el ancla se ve (las que salen con v-if al cargar datos), a los 5 s o en cuanto deja de
// hacer falta (vigente da falso: se pidió otro paso o se cerró el recorrido), así no quedan esperas vivas.
// No se usan waitForElement ni skipMissingElement de driver.js: el primero esconde la ventana mientras espera (queda el
// hueco del paso anterior sobre una página que ya no está) y el segundo salta el paso en vez de mostrarlo centrado.
export async function esperarAncla(hay: () => boolean, vigente: () => boolean, dormir = dormirMs): Promise<void> {
  for (let espera = 0; espera < ESPERA_MS && vigente() && !hay(); espera += CADA_MS) await dormir(CADA_MS)
}

type Botones = { nextButton: HTMLElement; previousButton: HTMLElement; closeButton: HTMLElement }

// botonConFoco es el botón que se enfoca al pintar la ventana (driver.js enfocaría la ✕, e Intro debe avanzar). driver.js
// rehace la ventana en cada pintado: si es el mismo paso (su ancla apareció tarde), el foco vuelve al botón equivalente al
// que lo tenía en vez de saltar a Siguiente.
export function botonConFoco(ventana: Botones, antes: Element | null, mismoPaso: boolean): HTMLElement {
  if (mismoPaso && antes?.classList.contains('driver-popover-prev-btn')) return ventana.previousButton
  if (mismoPaso && antes?.classList.contains('driver-popover-close-btn')) return ventana.closeButton
  return ventana.nextButton
}

// focoAlCerrar es adónde vuelve el foco al acabar: a lo que abrió el recorrido si sigue en la página y se ve; si no (el
// botón de la bienvenida, «Haz el recorrido» en Inicio, «Muéstrame» de la página de Ayuda o del panel «?», que ya se
// cerró), al «?» de la página en la que acaba.
export function focoAlCerrar(volverA: Element | null, raiz: Pick<Document, 'querySelectorAll'>): Element | undefined {
  return volverA?.isConnected && seVe(volverA) ? volverA : elegirElemento('app.ayuda', raiz)
}

// cerrarAlNavegar cierra el recorrido si se navega por fuera de él (Atrás, Alt+←): el hueco señalaría una página que ya
// no está. La navegación del recorrido usa pushState (ir de App.vue), que no lanza popstate. Devuelve cómo dejar de escuchar.
export function cerrarAlNavegar(destino: Pick<EventTarget, 'addEventListener' | 'removeEventListener'>, cerrar: () => void): () => void {
  destino.addEventListener('popstate', cerrar)
  return () => destino.removeEventListener('popstate', cerrar)
}

// OPCIONES_DRIVER son las opciones fijas de driver.js. Solo enseña: disableActiveInteraction quita los clics al elemento
// resaltado (Nueva API, Abrir un proyecto…) y, como driver.js deja fuera de su Tab los que no reciben clics, tampoco se
// llega con el teclado al mensaje de Probar: escribir no cambia de paso ni Ctrl+Intro envía nada.
export const OPCIONES_DRIVER = {
  showProgress: true,
  allowKeyboardControl: true,
  disableActiveInteraction: true,
  popoverClass: 'recorrido',
  overlayOpacity: 0.6,
  stagePadding: 6,
  stageRadius: 10,
} as const

// aDriver es el paso tal como lo pinta driver.js; se llama al mostrar cada paso, así que sale en el idioma de ese momento.
export function aDriver(paso: PasoMostrado, i: number, total: number, t: Traductor): DriveStep {
  return {
    popover: {
      title: t(paso.titulo),
      description: t(paso.texto),
      progressText: t('recorrido.progreso', { actual: i + 1, total }),
      nextBtnText: t(i === total - 1 ? 'recorrido.hecho' : 'recorrido.siguiente'),
      prevBtnText: t('recorrido.anterior'),
      closeBtnLabel: t('recorrido.cerrar'),
    },
  }
}
