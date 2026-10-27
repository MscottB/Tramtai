// La página de entrada (Entrar.vue): según GET /ui/entrada, el formulario de la contraseña del administrador del
// servidor, por qué aquí no se puede usar, o la entrada con el código de un solo uso de siempre. Aquí va la lógica sin
// Vue ni Quasar para poder probarla.
import { ref } from 'vue'
import { api } from './api/cliente'
import type { EntradaRemota } from './api/tipos'

// estadoEntrada es lo último que dijo el servicio; null, mientras no se consultó.
export const estadoEntrada = ref<EntradaRemota | null>(null)

export type VistaEntrada = 'consultando' | 'formulario' | 'sinHttps' | 'codigo'

// vistaEntrada: con administrador y por HTTPS, el formulario; con administrador sin HTTPS, el porqué (la contraseña
// viajaría en claro); sin administrador (el escritorio o un servidor sin contraseña), el código de un solo uso.
export function vistaEntrada(e: EntradaRemota | null): VistaEntrada {
  if (!e) return 'consultando'
  if (!e.contrasena) return 'codigo'
  return e.https ? 'formulario' : 'sinHttps'
}

// consultarEntrada pregunta al servicio; si no responde, queda la entrada con el código.
export async function consultarEntrada() {
  try {
    estadoEntrada.value = await api.entrada()
  } catch {
    estadoEntrada.value = { contrasena: false, totp: false, https: false }
  }
}

// acceder entra con la contraseña y, si el servidor lo pide, el código; la sesión queda abierta. Si falla, el error sube
// (el servicio da el mismo mensaje para la contraseña y el código, y 429 tras varios fallos).
export async function acceder(contrasena: string, codigo: string) {
  await api.acceder(contrasena, codigo.trim())
}
