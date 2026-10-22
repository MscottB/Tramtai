// useActualizacion lleva el aviso de versión nueva, «Actualizar ahora» con su avance y las novedades tras actualizar.
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, avisarError } from '../api/cliente'
import type { Novedad, ProgresoActualizacion } from '../api/tipos'
import { actualizacion, dialogoActualizar, sesion } from '../estado'
import { idioma, t } from '../i18n'
import { abandonarSinFase } from '../actualizacion'

export function useActualizacion() {
  let buscarActualizacion: number
  // Aviso de versión nueva: el servicio consulta GitHub como mucho una vez por hora y aquí se le pregunta al cargar,
  // cada 15 minutos y cada vez que la ventana vuelve a verse. En la app de escritorio, «Actualizar a…» de la bandeja abre este mismo diálogo.
  const actualizando = ref(false)
  async function comprobarActualizacion() {
    if (!sesion.value) return // sin sesión el servicio no responde; se comprueba al entrar
    try {
      actualizacion.value = await api.actualizacion()
    } catch {
      // sin servicio o sin sesión: se vuelve a intentar en la próxima comprobación
    }
  }
  // «Actualizar ahora»: el servicio descarga e instala en segundo plano y aquí se sigue su avance hasta que el instalador
  // lo detiene (entonces las consultas fallan y se deja el último estado: «instalando»).
  const progreso = ref<ProgresoActualizacion | null>(null)
  let seguirProgreso: number | undefined
  const megas = (bytes: number) => (bytes / 1048576).toLocaleString(idioma.value, { minimumFractionDigits: 1, maximumFractionDigits: 1 })
  const fraccion = computed(() => {
    const p = progreso.value
    if (!p || p.fase === 'error') return p?.total ? p.recibidos / p.total : 0
    return p.fase === 'descargando' && p.total ? p.recibidos / p.total : 1
  })
  // La cuenta atrás antes de instalar: ahora avanza con el sondeo del avance (cada 400 ms).
  const ahora = ref(Date.now())
  const segundosParaInstalar = computed(() => {
    const en = progreso.value?.instala_en
    return en ? Math.max(0, Math.ceil((Date.parse(en) - ahora.value) / 1000)) : 0
  })
  async function reiniciarAhora() {
    // Un error aquí es que ya no había nada que decidir (ya instala o se canceló): el sondeo muestra el estado real.
    await api.instalarYa().catch(() => {})
  }
  async function cancelarInstalacion() {
    try {
      await api.cancelarActualizacion()
    } catch (e) {
      avisarError(e) // ya estaba instalando: el diálogo sigue y pasa a «Instalando…»
      return
    }
    clearInterval(seguirProgreso)
    progreso.value = null
    dialogoActualizar.value = false
  }
  const indeterminado = computed(() => !!progreso.value && ((progreso.value.fase === 'descargando' && !progreso.value.total) || progreso.value.fase === 'instalando'))
  const textoProgreso = computed(() => {
    const p = progreso.value
    if (!p) return ''
    if (p.fase === 'error') return t('actualizacion.fallo', { error: p.error ?? '' })
    if (p.fase === 'verificando') return t('actualizacion.verificando')
    if (p.fase === 'instalando') return t('actualizacion.instalando')
    if (p.fase === 'listo') return t('actualizacion.reiniciaraEn', { n: segundosParaInstalar.value })
    if (!p.total) return t('actualizacion.descargandoSinTotal', { recibido: megas(p.recibidos) })
    return t('actualizacion.descargando', { recibido: megas(p.recibidos), total: megas(p.total), porcentaje: Math.floor((p.recibidos / p.total) * 100) })
  })
  async function instalarActualizacion() {
    actualizando.value = true
    // El avance se muestra al instante, antes de que responda el servicio: nunca un diálogo sin nada que ver.
    progreso.value = { fase: 'descargando', version: actualizacion.value?.version ?? '', recibidos: 0, total: 0 }
    try {
      const { version } = await api.actualizar()
      progreso.value = { fase: 'descargando', version, recibidos: 0, total: 0 }
      clearInterval(seguirProgreso)
      let enVuelo = false // si el servicio va lento, no se acumulan consultas
      let vistaFase = false, sinFase = 0
      seguirProgreso = window.setInterval(async () => {
        if (enVuelo) return
        enVuelo = true
        try {
          ahora.value = Date.now()
          const p = await api.progresoActualizacion()
          if (p.fase) { progreso.value = p; vistaFase = true; sinFase = 0 }
          else if (abandonarSinFase(vistaFase, ++sinFase)) {
            // Otra pestaña la canceló, o la consulta a la que esta se unió falló: no queda nada que esperar.
            clearInterval(seguirProgreso)
            progreso.value = null
            dialogoActualizar.value = false
          }
          if (p.fase === 'error') clearInterval(seguirProgreso)
        } catch {
          // el instalador ya detuvo el servicio: queda «instalando» hasta que la app se cierre
        } finally {
          enVuelo = false
        }
      }, 400)
    } catch (e) {
      progreso.value = null
      avisarError(e)
    } finally {
      actualizando.value = false
    }
  }

  const alVolver = () => !document.hidden && comprobarActualizacion()
  onMounted(() => {
    buscarActualizacion = window.setInterval(comprobarActualizacion, 15 * 60 * 1000)
    document.addEventListener('visibilitychange', alVolver)
  })
  onUnmounted(() => {
    clearInterval(buscarActualizacion)
    clearInterval(seguirProgreso)
    document.removeEventListener('visibilitychange', alVolver)
  })
  // Tras actualizar, muestra una vez las novedades de las versiones desde la última vista en este navegador (una
  // compilación dev no tiene versión con la que comparar).
  const dialogoNovedades = ref(false)
  const novedades = ref<Novedad[]>([])
  const cargandoNovedades = ref(false)
  watch(() => sesion.value?.version, async (version) => {
    if (!version) return
    try {
      const vista = localStorage.getItem('tramtai.version')
      localStorage.setItem('tramtai.version', version)
      // Sin la fecha (+DDMMAAAA): la misma versión compilada otro día no es una versión nueva.
      if (vista && vista.split('+')[0] !== version.split('+')[0] && /^v\d/.test(version)) {
        // El diálogo se abre al momento con un indicador; las notas llegan después (o no, sin red).
        novedades.value = []
        cargandoNovedades.value = true
        dialogoNovedades.value = true
        novedades.value = await api.novedades(vista).catch(() => [])
        cargandoNovedades.value = false
      }
    } catch {
      // sin almacenamiento local no hay aviso
    }
  })
  // novedadesNueva es lo que trae cada versión desde la instalada hasta la nueva, para el aviso de antes de actualizar;
  // se pide una vez por versión nueva y, si GitHub no responde, el aviso muestra solo las notas de la nueva.
  const novedadesNueva = ref<Novedad[]>([])
  let novedadesPedidas = ''
  const notasNueva = computed(() => actualizacion.value?.notas ?? '')
  const paginaNueva = computed(() => actualizacion.value?.url)
  watch(() => (dialogoActualizar.value && actualizacion.value?.disponible ? actualizacion.value.version : ''), async (nueva) => {
    if (!nueva || nueva === novedadesPedidas || !actualizacion.value) return
    novedadesPedidas = nueva
    novedadesNueva.value = []
    try {
      novedadesNueva.value = await api.novedades(actualizacion.value.actual, nueva)
    } catch {
      novedadesPedidas = '' // se vuelve a intentar al abrir el aviso otra vez
    }
  })

  return { actualizando, comprobarActualizacion, progreso, fraccion, indeterminado, textoProgreso, reiniciarAhora, cancelarInstalacion,
    instalarActualizacion, dialogoNovedades, novedades, cargandoNovedades, novedadesNueva, notasNueva, paginaNueva }
}
