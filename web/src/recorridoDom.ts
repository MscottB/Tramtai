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
  return [...raiz.querySelectorAll(`[data-recorrido="${ancla}"]`)].find((e) => {
    const { width, height } = e.getBoundingClientRect()
    return width > 0 && height > 0
  })
}

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
