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

export function cerrarAyuda(): void {
  panel.abierto = false
  previo?.focus?.()
  previo = null
}
