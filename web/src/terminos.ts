// La aceptación de los términos de uso al abrir la Web UI («Antes de seguir»): se consulta una vez con la sesión abierta y,
// mientras esté pendiente, App.vue no monta las páginas. Aquí va la lógica sin Vue ni Quasar para poder probarla.
import { ref } from 'vue'
import { api } from './api/cliente'
import type { EstadoTerminos } from './api/tipos'
import { idioma, idiomas, t } from './i18n'

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
    estadoTerminos.value = { version: '', pendiente: false, riesgos: [], servidor: false }
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

// riesgosTexto son los cinco riesgos en el idioma de la Web UI. Sin el modo servidor, los que tienen variante
// (riesgos.<n>.sinServidor, el 3 y el 4: lo dice terminos.Riesgos en Go) usan esa redacción.
export function riesgosTexto(servidor = false): string[] {
  return [1, 2, 3, 4, 5].map((n) => {
    const variante = `riesgos.${n}.sinServidor`
    return !servidor && variante in idiomas[idioma.value].textos ? t(variante as 'riesgos.3.sinServidor') : t(`riesgos.${n}` as 'riesgos.1')
  })
}