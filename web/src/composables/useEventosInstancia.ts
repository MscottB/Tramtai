// useEventosInstancia sigue en vivo las peticiones de la instancia (GET /api/v1/eventos) mientras la página que lo
// usa está montada y a la vista: con la pestaña oculta cierra el flujo (no ocupa una plaza ni una conexión) y al volver
// lo abre de nuevo, y la foto pone el mapa al día. Si el flujo se corta reconecta con espera creciente; tras 3 fallos
// seguidos, o sin EventSource, consulta los trabajos cada 5 s hasta que el flujo vuelva.
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api/cliente'
import { aplicar, esperaReconexion, eventoDe, limpiar, podar, reconciliar, type Tranvias } from '../vivo'

export function useEventosInstancia() {
  const tranvias = ref<Tranvias>(new Map())
  const enVivo = ref(false)
  const borrados = new Set<string>()
  let fuente: EventSource | undefined
  let fallos = 0
  let activo = false
  let reconexion: number | undefined
  let consulta: number | undefined
  let limpieza: number | undefined

  async function consultar() {
    if (document.hidden) return
    try {
      reconciliar(tranvias.value, (await api.trabajos(20)).map(eventoDe), Date.now(), borrados)
    } catch {
      // sin servicio: el mapa se queda como está hasta la próxima
    }
  }
  function consultarCada5s() {
    if (consulta !== undefined) return
    consultar()
    consulta = window.setInterval(consultar, 5000)
  }
  function abrir() {
    if (!activo || fuente || document.hidden) return
    if (typeof EventSource === 'undefined') return consultarCada5s()
    let enFoto: Set<string> | undefined = new Set() // los ids de la foto, hasta el evento foto que la cierra
    fuente = new EventSource('/api/v1/eventos')
    fuente.addEventListener('trabajo', (m) => {
      const e = JSON.parse((m as MessageEvent).data)
      enFoto?.add(e.id)
      aplicar(tranvias.value, e, Date.now(), borrados)
    })
    // Solo con la foto entera el flujo cuenta como recuperado: uno que abre y se corta enseguida sigue esperando más.
    fuente.addEventListener('foto', () => {
      podar(tranvias.value, enFoto ?? new Set())
      enFoto = undefined
      fallos = 0
      enVivo.value = true
      window.clearInterval(consulta)
      consulta = undefined
    })
    fuente.onerror = () => {
      fuente?.close()
      fuente = undefined
      enVivo.value = false
      if (++fallos >= 3) consultarCada5s()
      reconexion = window.setTimeout(abrir, esperaReconexion(fallos - 1))
    }
  }

  function cerrar() {
    fuente?.close()
    fuente = undefined
    window.clearTimeout(reconexion)
  }
  function alCambiarVisibilidad() {
    if (document.hidden) return cerrar()
    fallos = 0
    abrir()
  }

  onMounted(() => {
    activo = true
    abrir()
    document.addEventListener('visibilitychange', alCambiarVisibilidad)
    limpieza = window.setInterval(() => limpiar(tranvias.value, Date.now(), borrados), 500)
  })
  onUnmounted(() => {
    activo = false
    cerrar()
    document.removeEventListener('visibilitychange', alCambiarVisibilidad)
    window.clearInterval(consulta)
    window.clearInterval(limpieza)
  })
  return { tranvias, enVivo }
}
