// Estado compartido entre páginas: la sesión y el aviso de versión. Lo que una página le pasa a otra va en la
// intención de «ir» (navegacion.ts).
import { computed, ref } from 'vue'
import { t } from './i18n'
import type { EstadoActualizacion, Sesion } from './api/tipos'
import { leerBienvenida, type RespuestaBienvenida } from './bienvenida'

// sesion es la sesión actual, compartida por todas las páginas.
export const sesion = ref<Sesion | null>(null)

// sistema es el nombre del sistema operativo para los textos de arranque automático («Arranque con Windows»).
export const sistema = /Windows/i.test(navigator.userAgent) ? 'Windows' : /Mac/i.test(navigator.userAgent) ? 'macOS' : 'Linux'

export const actualizacion = ref<EstadoActualizacion | null>(null)

// dialogoActualizar muestra las novedades y el botón de instalar; lo abren la cabecera, Ajustes y «Ver novedades» de la bandeja.
export const dialogoActualizar = ref(false)

// diasHistorial son los días que se guarda el registro (para los textos «en N días»); cargarDiasHistorial lo lee.
export const diasHistorial = ref(7)
// periodoHistorial es «los últimos N días» o «el último día», para los textos que dicen cuánto se cuenta.
export const periodoHistorial = computed(() =>
  diasHistorial.value === 1 ? t('comun.ultimoDia') : t('comun.ultimosDias', { dias: diasHistorial.value }))

// guiaOculta: la guía de inicio es opcional; ocultarla se recuerda en este navegador y se deshace desde Ayuda.
const claveGuia = 'tramtai.guiaOculta'
export const guiaOculta = ref(((): boolean => {
  try {
    return localStorage.getItem(claveGuia) === '1'
  } catch {
    return false
  }
})())
export function ocultarGuia(oculta: boolean) {
  guiaOculta.value = oculta
  try {
    if (oculta) localStorage.setItem(claveGuia, '1')
    else localStorage.removeItem(claveGuia)
  } catch {
    // no se recuerda, pero se aplica
  }
}

// bienvenida: la bienvenida se ofrece una vez; empezarla o cerrarla a medias es 'hecha', «Ahora no» es 'descartada'.
// Sin almacenamiento solo queda en memoria: se ofrece como mucho una vez por carga.
const claveBienvenida = 'tramtai.bienvenida'
export const bienvenida = ref<RespuestaBienvenida | null>(((): RespuestaBienvenida | null => {
  try {
    return leerBienvenida(localStorage.getItem(claveBienvenida))
  } catch {
    return null
  }
})())
export function marcarBienvenida(valor: RespuestaBienvenida) {
  bienvenida.value = valor
  try {
    localStorage.setItem(claveBienvenida, valor)
  } catch {
    // no se recuerda, pero se aplica
  }
}
