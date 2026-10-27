// La aceptación de los términos de uso al abrir la Web UI («Antes de seguir»): se consulta una vez con la sesión abierta y,
// mientras esté pendiente, App.vue no monta las páginas. Aquí va la lógica sin Vue ni Quasar para poder probarla.
import { ref } from 'vue'
import { api } from './api/cliente'
import type { EstadoTerminos } from './api/tipos'
import { t } from './i18n'

// estadoTerminos es lo último que dijo el servicio; null, mientras no se consultó.
export const estadoTerminos = ref<EstadoTerminos | null>(null)
// salioDeTerminos: se pulsó «Salir» sin aceptar.
export const salioDeTerminos = ref(false)

export const URL_TERMINOS = 'https://github.com/MscottB/Tramtai/blob/main/TERMINOS.md'

export type PantallaTerminos = 'consultando' | 'pedir' | 'salio' | 'app'

// pantallaTerminos es lo que se ve: la consulta, la pregunta, «Has salido» o la app.
export function pantallaTerminos(estado: EstadoTerminos | null, salio: boolean): PantallaTerminos {
  if (salio) return 'salio'
  if (!estado) return 'consultando'
  return estado.pendiente ? 'pedir' : 'app'
}

// consultarTerminos pregunta al servicio. Si no responde no se bloquea la app: el servicio sigue siendo quien manda y la
// terminal y la CLI vuelven a avisar.
export async function consultarTerminos() {
  try {
    estadoTerminos.value = await api.terminos()
  } catch {
    estadoTerminos.value = { version: '', pendiente: false, riesgos: [] }
  }
}

// aceptarTerminos guarda la aceptación; si falla, el error sube y se sigue en la pregunta.
export async function aceptarTerminos() {
  await api.aceptarTerminos()
  if (estadoTerminos.value) estadoTerminos.value = { ...estadoTerminos.value, pendiente: false }
}

export function salirDeTerminos() {
  salioDeTerminos.value = true
}

// riesgosTexto son los cinco riesgos en el idioma de la Web UI. Sin el modo servidor, el 3 y el 4 llevan delante
// «Si usas el modo servidor, » (como terminos.Riesgos en Go).
export function riesgosTexto(servidor = false): string[] {
  return [1, 2, 3, 4, 5].map((n) => {
    const riesgo = t(`riesgos.${n}` as 'riesgos.1')
    return servidor || (n !== 3 && n !== 4) ? riesgo : t('riesgos.siServidor') + riesgo.charAt(0).toLowerCase() + riesgo.slice(1)
  })
}
