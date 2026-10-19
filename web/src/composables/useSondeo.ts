// useSondeo sigue si el servicio está en marcha: lo consulta cada 10 s mientras la ventana se ve.
import { onMounted, onUnmounted, ref } from 'vue'

export function useSondeo() {
  const conectado = ref(true)
  let sondeo: number
  let comprobando = false
  // comprobar consulta /healthz (sin clave): si el servicio se detiene, la cabecera lo muestra; cuando vuelve, la
  // página se recarga para abrir una sesión nueva (las sesiones no sobreviven a un reinicio del servicio).
  async function comprobar() {
    if (comprobando) return
    comprobando = true
    const antes = conectado.value
    const signal = typeof AbortSignal.timeout === 'function' ? AbortSignal.timeout(5000) : undefined
    conectado.value = await fetch('/healthz', { cache: 'no-store', signal }).then((r) => r.ok).catch(() => false)
    comprobando = false
    if (!antes && conectado.value) location.reload()
  }

  onMounted(() => {
    sondeo = window.setInterval(() => !document.hidden && comprobar(), 10000)
  })
  onUnmounted(() => clearInterval(sondeo))
  return { conectado, comprobar }
}
