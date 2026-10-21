// useRecorrido enseña un recorrido guiado con driver.js: si el paso es de otra página va a ella, espera a que se vea su
// ancla y, si no aparece, muestra el paso centrado sin resaltar. Solo enseña: nunca pulsa nada ni cambia datos.
// App.vue crea uno y lo pasa a quien lo necesite (la bienvenida, el «Muéstrame en pantalla» de la Ayuda).
import { nextTick, ref, type Ref } from 'vue'
import { driver, type Driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import type { Pagina } from '../ayuda/tipos'
import { t } from '../i18n'
import type { PasoMostrado } from '../recorrido'
import { aDriver, elegirElemento } from '../recorridoDom'

const ESPERA_MS = 5000
const CADA_MS = 100

// esperarAncla vuelve en cuanto el ancla se ve (las que salen con v-if al cargar datos) o a los 5 s.
async function esperarAncla(ancla: string | undefined) {
  for (let espera = 0; ancla && espera < ESPERA_MS && !elegirElemento(ancla, document); espera += CADA_MS)
    await new Promise((listo) => setTimeout(listo, CADA_MS))
}

export function useRecorrido(ir: (pagina: Pagina) => void, paginaActual: Readonly<Ref<string>>) {
  const activo = ref(false)
  let d: Driver | undefined
  let pasos: PasoMostrado[] = []
  let alFin: (() => void) | undefined
  let volverA: Element | null = null
  // turno es el último paso pedido: una espera vieja no muestra nada si mientras tanto se pulsó otra cosa.
  let turno = 0

  function terminar() {
    if (!activo.value) return
    activo.value = false
    turno++
    d = undefined
    const fin = alFin
    alFin = undefined
    const foco = volverA as HTMLElement | null
    // después de driver.js, que al cerrar devuelve el foco a lo que lo tenía al empezar el último paso (su botón)
    setTimeout(() => foco?.focus?.())
    fin?.()
  }

  function cerrar() {
    d?.destroy()
    terminar()
  }

  async function mostrar(i: number) {
    const mio = ++turno
    const paso = pasos[i]
    if (!paso) return cerrar()
    if (paso.pagina !== paginaActual.value) {
      ir(paso.pagina)
      await nextTick()
    }
    await esperarAncla(paso.ancla)
    if (mio !== turno || !d) return
    // driver.js muestra centrado el paso cuyo elemento no existe: así sale el paso sin ancla o sin nada visible.
    const elemento = (p: PasoMostrado) => () => elegirElemento(p.ancla, document) as Element
    d.setConfig({ ...d.getConfig(), steps: pasos.map((p, j) => ({ ...aDriver(p, j, pasos.length, t), element: elemento(p) })) })
    if (d.isActive()) d.moveTo(i)
    else d.drive(i)
  }

  function empezar(lista: PasoMostrado[], alTerminar?: () => void) {
    cerrar()
    pasos = lista
    alFin = alTerminar
    volverA = document.activeElement
    activo.value = true
    const quieto = matchMedia('(prefers-reduced-motion: reduce)').matches
    d = driver({
      animate: !quieto,
      smoothScroll: !quieto,
      showProgress: true,
      allowKeyboardControl: true,
      popoverClass: 'recorrido',
      overlayOpacity: 0.6,
      stagePadding: 6,
      stageRadius: 10,
      onNextClick: () => void mostrar((d?.getActiveIndex() ?? -1) + 1),
      onPrevClick: () => void mostrar((d?.getActiveIndex() ?? 1) - 1),
      // driver.js enfoca la ✕ (el primer botón); Intro debe avanzar, no cerrar
      onPopoverRender: (ventana) => queueMicrotask(() => ventana.nextButton.focus()),
      onDestroyed: terminar,
    })
    void mostrar(0)
  }

  return { empezar, activo }
}
