// rutaConfig es la ruta del archivo de configuración global: la real que da el servicio a una sesión local (también la
// de una portable, en datos-tramtai) o, si no, la habitual de cada sistema. La enseñan Ajustes y Agentes.
import { computed, ref } from 'vue'
import { api } from './api/cliente'
import { sistema } from './estado'

const habitual = { Windows: '%APPDATA%\\tramtai\\config.yaml', macOS: '~/Library/Application Support/tramtai/config.yaml', Linux: '~/.config/tramtai/config.yaml' }[sistema]
const real = ref('')
export const rutaConfig = computed(() => real.value || habitual)

export function cargarRutaConfig() {
  api.acerca().then((a) => (real.value = a.rutas?.configuracion ?? '')).catch(() => {})
}
