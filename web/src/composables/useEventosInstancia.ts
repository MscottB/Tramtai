// useEventosInstancia sigue en vivo las peticiones de la instancia (GET /api/v1/eventos) mientras la página que lo
// usa está montada. Si el flujo se corta reconecta con espera creciente; tras 3 fallos seguidos, o sin EventSource,
// consulta los trabajos cada 5 s hasta que el flujo vuelva.
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api/cliente'
import type { Trabajo } from '../api/tipos'
import { terminales } from '../formato'
import { aplicar, esperaReconexion, etapaDe, limpiar, type EventoTrabajo, type Tranvias } from '../vivo'

// eventoDe convierte un trabajo de la consulta en el evento que habría mandado el flujo.
function eventoDe(t: Trabajo): EventoTrabajo {
  return { id: t.id, estado: t.estado, etapa: etapaDe(t.estado), agente: t.solicitud?.agente ?? '', modelo: t.modelo_efectivo || t.modelo_enviado || '',
    api: t.punto_acceso ?? '', origen: t.origen ?? '', intento: 1, codigo_error: t.codigo_error ?? '', actualizado: '' }
}

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
      for (const t of await api.trabajos(20)) if (!terminales.includes(t.estado)) aplicar(tranvias.value, eventoDe(t), Date.now(), borrados)
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
    if (!activo) return
    if (typeof EventSource === 'undefined') return consultarCada5s()
    fuente = new EventSource('/api/v1/eventos')
    fuente.addEventListener('trabajo', (m) => aplicar(tranvias.value, JSON.parse((m as MessageEvent).data), Date.now(), borrados))
    fuente.onopen = () => {
      fallos = 0
      enVivo.value = true
      window.clearInterval(consulta)
      consulta = undefined
    }
    fuente.onerror = () => {
      fuente?.close()
      fuente = undefined
      enVivo.value = false
      if (++fallos >= 3) consultarCada5s()
      reconexion = window.setTimeout(abrir, esperaReconexion(fallos - 1))
    }
  }

  onMounted(() => {
    activo = true
    abrir()
    limpieza = window.setInterval(() => limpiar(tranvias.value, Date.now(), borrados), 500)
  })
  onUnmounted(() => {
    activo = false
    fuente?.close()
    window.clearTimeout(reconexion)
    window.clearInterval(consulta)
    window.clearInterval(limpieza)
  })
  return { tranvias, enVivo }
}
