// useRecorrido enseña un recorrido guiado con driver.js: si el paso es de otra página va a ella, espera a que se vea su
// ancla y, si no aparece, muestra el paso centrado sin resaltar. Solo enseña: nunca pulsa nada ni cambia datos.
// App.vue crea uno y lo pasa a quien lo necesite (la bienvenida, el «Muéstrame en pantalla» de la Ayuda).
import { nextTick, ref, type Ref } from 'vue'
import { driver, type Driver } from 'driver.js'
import 'driver.js/dist/driver.css'
import type { Pagina } from '../ayuda/tipos'
import { t } from '../i18n'
import type { PasoMostrado } from '../recorrido'
import { aDriver, botonConFoco, capturarEsc, cerrarAlNavegar, elegirElemento, esperarAncla, EVENTO_PASO, focoAlCerrar, OPCIONES_DRIVER } from '../recorridoDom'

export function useRecorrido(ir: (pagina: Pagina) => void, paginaActual: Readonly<Ref<string>>) {
  const activo = ref(false)
  let d: Driver | undefined
  let pasos: PasoMostrado[] = []
  let alFin: (() => void) | undefined
  let volverA: Element | null = null
  // turno es el último paso pedido: una espera vieja no muestra nada si mientras tanto se pulsó otra cosa.
  let turno = 0
  let soltarEsc: (() => void) | undefined
  let soltarNavegar: (() => void) | undefined
  // Para el foco al pintar: qué lo tenía antes y si se repinta el mismo paso (su ancla apareció tarde).
  let focoAntes: Element | null = null
  let repinta = false

  function terminar() {
    if (!activo.value) return
    activo.value = false
    soltarEsc?.()
    soltarNavegar?.()
    soltarEsc = soltarNavegar = undefined
    turno++
    d = undefined
    const fin = alFin
    alFin = undefined
    const antes = volverA
    // después de driver.js, que al cerrar devuelve el foco a lo que lo tenía al empezar el último paso (su botón)
    // (si entretanto empezó otro recorrido, el foco es suyo)
    setTimeout(() => { if (!activo.value) (focoAlCerrar(antes, document) as HTMLElement | undefined)?.focus?.() })
    fin?.()
  }

  function cerrar() {
    d?.destroy()
    terminar()
  }

  function pintar(i: number) {
    if (!d) return
    // driver.js muestra centrado el paso sin elemento: así sale el paso sin ancla o sin nada visible.
    const steps = pasos.map((p, j) => {
      const elemento = elegirElemento(p.ancla, document)
      return elemento ? { ...aDriver(p, j, pasos.length, t), element: elemento } : aDriver(p, j, pasos.length, t)
    })
    d.setConfig({ ...d.getConfig(), steps })
    repinta = d.isActive() && d.getActiveIndex() === i
    focoAntes = document.activeElement
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
      window.dispatchEvent(new CustomEvent(EVENTO_PASO, { detail: { ancla: paso.ancla } }))
      await nextTick()
      if (!vigente()) return
    }
    pintar(i)
    const ancla = paso.ancla
    if (!ancla || elegirElemento(ancla, document)) return
    await esperarAncla(() => !!elegirElemento(ancla, document), vigente)
    if (vigente() && elegirElemento(ancla, document)) pintar(i)
  }

  function empezar(lista: PasoMostrado[], alTerminar?: () => void) {
    cerrar()
    pasos = lista
    alFin = alTerminar
    volverA = document.activeElement
    activo.value = true
    const quieto = matchMedia('(prefers-reduced-motion: reduce)').matches
    d = driver({
      ...OPCIONES_DRIVER,
      animate: !quieto,
      smoothScroll: !quieto,
      onNextClick: () => void mostrar((d?.getActiveIndex() ?? -1) + 1),
      onPrevClick: () => void mostrar((d?.getActiveIndex() ?? 1) - 1),
      onPopoverRender: (ventana) => {
        const boton = botonConFoco(ventana, focoAntes, repinta)
        queueMicrotask(() => boton.focus())
      },
      // Esc, la ✕, el fondo y «Hecho» pasan por cerrar(): onDestroyed no llega si se cierra durante la primera animación
      onDestroyStarted: cerrar,
    })
    // Después de driver(): si fallara, no queda nadie escuchando. Esc es del recorrido, no del panel «?» ni del diálogo
    // que haya debajo (Quasar los cierra con la misma tecla).
    soltarEsc = capturarEsc(window, cerrar)
    soltarNavegar = cerrarAlNavegar(window, cerrar)
    void mostrar(0)
  }

  return { empezar, cerrar, activo }
}
