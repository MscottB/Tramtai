// panel es el estado del panel lateral de la Ayuda (el botón ? de cada página, «¿Qué hago?» y «Saber más»). Al
// cerrarlo, el foco vuelve a lo que lo abrió.
import { reactive } from 'vue'
import { temaPorId } from './indice'

export const panel = reactive({ abierto: false, tema: '', ancla: '' })
let previo: HTMLElement | null = null

export function abrirAyuda(tema: string, ancla = ''): void {
  if (!temaPorId(tema)) return
  if (!panel.abierto && typeof document !== 'undefined') previo = document.activeElement as HTMLElement | null
  Object.assign(panel, { abierto: true, tema, ancla })
}

// escDelPanel: Esc cierra el panel abierto, salvo con un recorrido en marcha (la tecla es suya: el panel la escucha antes,
// en la misma fase de captura, y si la detuviera el recorrido no se cerraría) o con un menú desplegado encima.
export function escDelPanel(e: KeyboardEvent, abierto: boolean, recorridoActivo: boolean, menuAbierto: boolean): boolean {
  return e.key === 'Escape' && abierto && !recorridoActivo && !menuAbierto
}

export function cerrarAyuda(): void {
  panel.abierto = false
  previo?.focus?.()
  previo = null
}
