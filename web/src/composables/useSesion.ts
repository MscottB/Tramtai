// useSesion abre la sesión al cargar la página y cambia de espacio; al abrirse una sesión, busca versión nueva y pide
// las fichas de los agentes.
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { api, avisar, avisarError } from '../api/cliente'
import { sesion } from '../estado'
import { cargarFichas } from '../agentes'
import { t } from '../i18n'

// recargarConAviso deja el aviso para después de recargar (sobrevive en sessionStorage) y recarga la página.
function recargarConAviso(texto: string) {
  try {
    sessionStorage.setItem('tramtai.aviso', texto)
  } catch {
    // sin sessionStorage se recarga igual, sin aviso
  }
  location.reload()
}

export function useSesion(comprobarActualizacion: () => unknown) {
  const cargando = ref(true)
  const activando = ref(false)
  // Al abrir la sesión (al cargar o tras Entrar) se busca versión nueva sin esperar al siguiente intervalo.
  watch(() => !!sesion.value, (abierta) => {
    if (!abierta) return
    comprobarActualizacion()
    cargarFichas()
  })
  const avisoEntrada = ref(new URLSearchParams(location.search).get('error') === 'codigo' ? t('app.codigoCaducado') : '')

  // Varias pestañas: al cambiar de espacio en una, las demás se recargan en el nuevo (su cookie ya es la nueva).
  const canal = typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('tramtai-sesion') : undefined
  watch(() => sesion.value?.instancia, (nueva, antes) => {
    if (antes && nueva && nueva !== antes) canal?.postMessage({ instancia: nueva, nombre: sesion.value?.nombre ?? nueva })
  })
  if (canal) {
    canal.onmessage = (m: MessageEvent<{ instancia: string; nombre: string }>) => {
      if (m.data?.instancia && m.data.instancia !== sesion.value?.instancia) recargarConAviso(t('app.cambiasteEnOtra', { nombre: m.data.nombre }))
    }
  }
  // Una acción que falló por CSRF viejo (el cliente ya renovó la sesión): se recarga en el espacio actual.
  const alCambiarEspacio = () => recargarConAviso(t('app.espacioCambiado'))
  window.addEventListener('tramtai:espacio-cambiado', alCambiarEspacio)
  onUnmounted(() => {
    canal?.close()
    window.removeEventListener('tramtai:espacio-cambiado', alCambiarEspacio)
  })

  onMounted(async () => {
    try {
      const aviso = sessionStorage.getItem('tramtai.aviso')
      if (aviso) {
        sessionStorage.removeItem('tramtai.aviso')
        avisar(aviso)
      }
    } catch {
      // sin sessionStorage no hay aviso pendiente
    }
    try {
      await api.sesion()
    } catch {
      sesion.value = null
    } finally {
      cargando.value = false
    }
  })

  async function cambiar(instancia: string) {
    if (instancia === sesion.value?.instancia) return
    try {
      await api.cambiarInstancia(instancia)
    } catch (e) {
      avisarError(e)
    }
  }

  async function activarGlobal() {
    activando.value = true
    try {
      await api.activarGlobal()
    } catch (e) {
      avisarError(e)
    } finally {
      activando.value = false
    }
  }

  return { cargando, activando, avisoEntrada, cambiar, activarGlobal }
}
