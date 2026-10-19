// useSesion abre la sesión al cargar la página y cambia de espacio; al abrirse una sesión, busca versión nueva y pide
// las fichas de los agentes.
import { onMounted, ref, watch } from 'vue'
import { api, avisarError } from '../api/cliente'
import { sesion } from '../estado'
import { cargarFichas } from '../agentes'
import { t } from '../i18n'

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

  onMounted(async () => {
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
