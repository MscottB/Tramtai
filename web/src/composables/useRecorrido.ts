// useRecorrido enseña un recorrido guiado con driver.js: si el paso es de otra página va a ella, espera a que se vea su
// ancla y, si no aparece, muestra el paso centrado sin resaltar. Solo enseña: nunca pulsa nada ni cambia datos.
// App.vue crea uno y lo pasa a quien lo necesite (la bienvenida, el «Muéstrame en pantalla» de la Ayuda).
import { nextTick, ref, type Ref } from 'vue'
import { driver, type Driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import type { Pagina } from '../ayuda/tipos'
import { t } from '../i18n'
import type { PasoMostrado } from '../recorrido'
import { aDriver, capturarEsc, elegirElemento } from '../recorridoDom'

const ESPERA_MS = 5000
const CADA_MS = 100

// esperarAncla vuelve en cuanto el ancla se ve (las que salen con v-if al cargar datos), a los 5 s o si deja de hacer
// falta (vigente da falso: se pidió otro paso o se cerró el recorrido).
async function esperarAncla(ancla: string, vigente: () => boolean) {
  for (let espera = 0; espera < ESPERA_MS && vigente() && !elegirElemento(ancla, document); espera += CADA_MS)
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
  let soltarEsc: (() => void) | undefined

  function terminar() {
    if (!activo.value) return
    activo.value = false
    soltarEsc?.()
    soltarEsc = undefined
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

  function pintar(i: number) {
    if (!d) return
    // driver.js muestra centrado el paso cuyo elemento no existe: así sale el paso sin ancla o sin nada visible.
    const elemento = (p: PasoMostrado) => () => elegirElemento(p.ancla, document) as Element
    d.setConfig({ ...d.getConfig(), steps: pasos.map((p, j) => ({ ...aDriver(p, j, pasos.length, t), element: elemento(p) })) })
    if (d.isActive()) d.moveTo(i)
    else d.drive(i)
  }

  async function mostrar(i: number) {
    const mio = ++turno
    const vigente = () => mio === turno
    const paso = pasos[i]
    if (!paso) return cerrar()
    if (paso.pagina !== paginaActual.value) {
      ir(paso.pagina)
      await nextTick()
      if (!vigente()) return
    }
    // El paso se pinta ya, centrado si su ancla aún no se ve: así no queda la ventana del paso anterior sobre el hueco
    // de un elemento desmontado, y Siguiente avanza en vez de volver a empezar la espera. Si el ancla aparece en los 5 s
    // siguientes, el paso pasa a señalarla.
    // Antes de buscar el ancla, la página puede abrir la pestaña donde está (Ajustes); cambiar de pestaña no cambia datos.
    if (paso.ancla) {
      window.dispatchEvent(new CustomEvent('tramtai:recorrido-paso', { detail: { ancla: paso.ancla } }))
      await nextTick()
      if (!vigente()) return
    }
    pintar(i)
    if (!paso.ancla || elegirElemento(paso.ancla, document)) return
    await esperarAncla(paso.ancla, vigente)
    if (vigente() && elegirElemento(paso.ancla, document)) pintar(i)
  }

  function empezar(lista: PasoMostrado[], alTerminar?: () => void) {
    cerrar()
    pasos = lista
    alFin = alTerminar
    volverA = document.activeElement
    activo.value = true
    // Esc es del recorrido, no del panel «?» ni del diálogo que haya debajo (Quasar los cierra con la misma tecla)
    soltarEsc = capturarEsc(window, cerrar)
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
      // Esc, la ✕, el fondo y «Hecho» pasan por cerrar(): onDestroyed no llega si se cierra durante la primera animación
      onDestroyStarted: cerrar,
    })
    void mostrar(0)
  }

  return { empezar, activo }
}
