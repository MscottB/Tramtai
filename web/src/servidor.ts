// Los dos entornos y el modo servidor en la Web UI: la marca de la cabecera («Servidor», «Este equipo» o «Portable»),
// Ajustes → Servidor (solo lectura, cada dato con la orden que lo cambia), lo que Agentes ofrece en lugar de «Usar mi
// suscripción» y el aviso fijo de la suscripción personal. Aquí va la lógica sin Vue ni Quasar para poder probarla.
import { ref } from 'vue'
import { api } from './api/cliente'
import type { EstadoServidor, Sesion } from './api/tipos'
import type { Clave } from './idiomas/es'
import type { PestanaAjustes } from './pestanasAjustes'
import { t } from './i18n'

// marcaEntorno es la clave del texto de la marca; sin entorno (un servicio anterior), «Este equipo».
export function marcaEntorno(entorno?: string): Clave {
  if (entorno === 'servidor') return 'entorno.servidor'
  return entorno === 'portable' ? 'entorno.portable' : 'entorno.escritorio'
}

export const esServidor = (s: Sesion | null | undefined) => s?.entorno === 'servidor'

// estadoServidor es lo último que dijo GET /ui/servidor; null fuera del servidor o si no respondió.
export const estadoServidor = ref<EstadoServidor | null>(null)

export async function cargarServidor(s: Sesion | null | undefined) {
  if (!esServidor(s)) {
    estadoServidor.value = null
    return
  }
  try {
    estadoServidor.value = await api.servidor()
  } catch {
    estadoServidor.value = null
  }
}

// suscripcionesPersonales son los agentes que usan la suscripción personal del administrador: cada uno lleva su aviso fijo.
export const suscripcionesPersonales = (e: EstadoServidor | null) => (e?.suscripciones ?? []).filter((s) => s.personal).map((s) => s.agente)

// pestanasAjustes: Servidor y Seguridad solo en el servidor.
export const pestanasAjustes = (servidor: boolean): PestanaAjustes[] => (servidor ? ['espacio', 'tramtai', 'servidor', 'seguridad'] : ['espacio', 'tramtai'])

// conArranque: el arranque con el sistema es del Tramtai del usuario; el servicio del sistema arranca con el equipo.
export const conArranque = (s: Sesion | null | undefined) => !esServidor(s) && s?.autoarranque !== undefined

// modosArranque son las opciones del arranque con el sistema: no arrancar, la app en la bandeja o solo el servicio. El
// valor es el modo_arranque de la sesión y el que se envía al elegirlo.
export const modosArranque = [
  { valor: '', titulo: 'arranque.no', texto: 'arranque.noTexto' },
  { valor: 'app', titulo: 'arranque.app', texto: 'arranque.appTexto' },
  { valor: 'servicio', titulo: 'arranque.servicio', texto: 'arranque.servicioTexto' },
] as const satisfies readonly { valor: Sesion['modo_arranque']; titulo: Clave; texto: Clave }[]

// ofrecerInstalar: «Instalar ahora» no existe en el servidor (se actualiza con tramtai-servidor actualizar).
export const ofrecerInstalar = (instalable: boolean | undefined, s: Sesion | null | undefined) => !!instalable && !esServidor(s)

// formaSuscripcion es lo que Agentes ofrece para la suscripción: en el escritorio, elegirla aquí; en el servidor, la
// orden que inicia sesión con el CLI (en local) o la que activa la suscripción personal (abierto a la red).
export type FormaSuscripcion = 'elegir' | 'entrar' | 'personal'
export function formaSuscripcion(s: Sesion | null | undefined, e: EstadoServidor | null): FormaSuscripcion {
  if (!esServidor(s)) return 'elegir'
  return e?.expuesto ? 'personal' : 'entrar'
}

// puedeElegirSuscripcion: los botones de la suscripción, solo con un agente que la admite, desde este equipo y fuera del
// servidor (ahí va la orden).
export const puedeElegirSuscripcion = (admite: boolean, s: Sesion | null | undefined, e: EstadoServidor | null) =>
  admite && !!s?.local && formaSuscripcion(s, e) === 'elegir'

// puedeVolverAClave: dejar la suscripción por la clave de API vale desde este equipo y, en el servidor, también desde la
// sesión remota del administrador (como en la terminal): no abre nada nuevo.
export const puedeVolverAClave = (admite: boolean, s: Sesion | null | undefined) => admite && (!!s?.local || esServidor(s))

export function ordenSuscripcion(agente: string, forma: FormaSuscripcion) {
  return forma === 'personal' ? `tramtai-servidor agentes suscripcion ${agente} --personal` : `tramtai-servidor agentes entrar ${agente}`
}

// ordenClave guarda la clave que falta: la del servidor la pide tramtai-servidor en el propio servidor.
export const ordenClave = (s: Sesion | null | undefined, variable: string) => `${esServidor(s) ? 'tramtai-servidor' : 'tramtai'} claves guardar ${variable}`

// FilaServidor es un dato de Ajustes → Servidor: su valor y lo que lo cambia (una orden o una clave de config.yaml) o, si
// no hay orden, una nota.
export interface FilaServidor {
  etiqueta: Clave
  valor: string
  orden?: string
  clave?: string // una clave de config.yaml (servicio.*), que no es una orden
  nota?: string
}

const modosTLS: Record<string, Clave> = { ninguno: 'servidor.tlsNinguno', archivos: 'servidor.tlsArchivos', letsencrypt: 'servidor.tlsLetsencrypt' }

export function filasServidor(e: EstadoServidor, fecha: (iso: string) => string, nombre: (agente: string) => string): FilaServidor[] {
  const lista = (l: string[]) => (l.length ? l.join(', ') : t('servidor.ninguno'))
  const filas: FilaServidor[] = [
    { etiqueta: 'servidor.https', valor: modosTLS[e.tls] ? t(modosTLS[e.tls]) : e.tls, clave: 'servicio.tls.modo' },
    { etiqueta: 'servidor.dominios', valor: lista(e.dominios), clave: 'servicio.dominios' },
    { etiqueta: 'servidor.confiables', valor: lista(e.confiables), clave: 'servicio.proxy.confiables' },
  ]
  if (e.vence_certificado) {
    filas.push({ etiqueta: 'servidor.vence', valor: fecha(e.vence_certificado), nota: t(e.tls === 'letsencrypt' ? 'servidor.renuevaSolo' : 'servidor.renovarArchivos') })
  }
  filas.push({
    etiqueta: 'servidor.administrador',
    valor: t(!e.administrador ? 'servidor.sinAdministrador' : e.totp ? 'servidor.conTotp' : 'servidor.sinTotp'),
    orden: e.administrador && !e.totp ? 'tramtai-servidor administrador totp activar' : 'tramtai-servidor administrador contraseña',
  })
  filas.push({
    etiqueta: 'servidor.suscripciones',
    valor: e.suscripciones.length
      ? e.suscripciones.map((s) => t(s.personal ? 'servidor.suscripcionPersonal' : 'servidor.suscripcionLocal', { agente: nombre(s.agente), n: s.simultaneas })).join(' · ')
      : t('servidor.sinSuscripciones'),
    // Como en Agentes: en local, el inicio de sesión del CLI; abierto a la red, la suscripción personal.
    orden: ordenSuscripcion('<agente>', e.expuesto ? 'personal' : 'entrar'),
  })
  return filas
}
