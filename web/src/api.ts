// Cliente de la API de Tramtai para la Web UI: usa la cookie de sesión (mismo origen) y envía el token CSRF en
// las peticiones que modifican. Las claves nunca se guardan en el navegador.
import { ref } from 'vue'
import { Notify } from 'quasar'
import { idioma, t } from './i18n'
import type { Clave } from './idiomas/es'

export interface InstanciaResumida {
  id: string
  ambito: string
  raiz?: string
  nombre: string
}
export interface Sesion {
  instancia: string
  ambito?: string
  raiz?: string
  nombre?: string
  csrf: string
  version: string
  // local: entró sin clave por ser el mismo usuario del equipo; puede cambiar de instancia.
  local: boolean
  instancias?: InstanciaResumida[]
  // autoarranque: si Tramtai arranca al iniciar sesión; solo en sesiones locales de sistemas que lo admiten.
  autoarranque?: boolean
}
export interface Uso {
  tokens_entrada: number
  tokens_salida: number
  costo_usd?: number
}
export interface Mensaje {
  rol: string
  contenido: string
}
export interface Solicitud {
  agente?: string
  modelo?: string
  perfil_modelo?: string
  perfil?: string
  mensajes: Mensaje[]
  cargas?: string[]
  salida?: { formato?: string; esquema?: unknown }
  ejecucion?: { tiempo_limite_segundos?: number }
}
export interface Trabajo {
  id: string
  instancia: string
  estado: string
  codigo_error?: string
  mensaje_error?: string
  solicitud: Solicitud
  salida?: string
  disponibilidad?: string
  entradas_eliminadas?: boolean
  intento_de?: string
  punto_acceso?: string
  modelo_enviado?: string
  modelo_efectivo?: string
  uso: Uso
  creado_en: string
  terminado_en?: string
}
export interface EstadoInstancia {
  id: string
  ambito: string
  raiz?: string
  archivo_token: string
  agente_predeterminado: string
  perfil_predeterminado?: string
  trabajos: Record<string, number>
  base: string
}
export interface EstadoPunto {
  nombre: string
  grupo?: string
  instrucciones?: string
  instancia: string
  url?: string
  contratos: string[]
  agente?: string
  modelo?: string
  activo: boolean
  error?: string
  origen: string
}
export interface ConfigPunto {
  nombre: string
  grupo?: string
  instrucciones?: string
  puerto?: number
  contratos?: string[]
  agente?: string
  modelo?: string
  perfil_modelo?: string
  modelos_permitidos?: string[]
  forzar_modelo?: boolean
  alias_modelos?: Record<string, string>
}
// Modelo es una entrada del catálogo: agente, agente/modelo o perfil/nombre, con la versión real que informó el
// agente la última vez y si es el predeterminado del espacio.
export interface Modelo {
  id: string
  agente?: string
  modelo?: string
  perfil?: string
  resuelto?: string
  version?: string
  predeterminado?: boolean
}
export interface EntradaConfiguracion {
  clave: string
  valor: unknown
  origen: string
}
export interface Adaptador {
  agente: string
  version_detectada?: string
  autenticacion?: string
  terminos?: string
  problema?: string
  capacidades: { streaming: string; salida_estructurada: boolean; modelo_efectivo_informado: boolean }
}

export class ErrorApi extends Error {
  constructor(
    public codigo: string,
    mensaje: string,
    public estado: number,
  ) {
    super(mensaje)
  }
}

let csrf = ''

async function pedir<T>(metodo: string, ruta: string, cuerpo?: unknown): Promise<T> {
  const cabeceras: Record<string, string> = {}
  let body: BodyInit | undefined
  if (cuerpo instanceof FormData) {
    body = cuerpo
  } else if (cuerpo !== undefined) {
    body = JSON.stringify(cuerpo)
    cabeceras['Content-Type'] = 'application/json'
  }
  if (metodo !== 'GET') cabeceras['X-CSRF-Token'] = csrf
  const respuesta = await fetch(ruta, { method: metodo, headers: cabeceras, body, credentials: 'same-origin' })
  if (respuesta.status === 204) return undefined as T
  const datos = await respuesta.json().catch(() => ({}))
  if (!respuesta.ok) {
    throw new ErrorApi(datos?.error?.codigo ?? 'error', datos?.error?.mensaje ?? respuesta.statusText, respuesta.status)
  }
  return datos as T
}

// sesion es la sesión actual, compartida por todas las páginas.
export const sesion = ref<Sesion | null>(null)

function recordar(s: Sesion): Sesion {
  csrf = s.csrf
  sesion.value = s
  return s
}

export const api = {
  sesion: () => pedir<Sesion>('GET', '/ui/sesion').then(recordar),
  entrar: (token: string) => pedir<Sesion>('POST', '/ui/sesion', { token }).then(recordar),
  cambiarInstancia: (instancia: string) => pedir<Sesion>('POST', '/ui/sesion', { instancia }).then(recordar),
  activarGlobal: () => pedir<Sesion>('POST', '/ui/instancias', { ambito: 'global' }).then(recordar),
  autoarranque: (activo: boolean) => pedir<Sesion>('POST', '/ui/autoarranque', { activo }).then(recordar),
  cambiarAutenticacion: (agente: string, modo: 'suscripcion' | 'clave_api') => pedir<void>('POST', '/ui/autenticacion', { agente, modo }),
  salir: () => pedir<void>('DELETE', '/ui/sesion').then(() => (sesion.value = null)),
  instancia: () => pedir<EstadoInstancia>('GET', '/api/v1/instancia'),
  claveInstancia: () => pedir<{ token: string; archivo: string }>('GET', '/api/v1/instancia/token'),
  configuracion: () =>
    pedir<{ configuracion: EntradaConfiguracion[] }>('GET', '/api/v1/instancia/configuracion').then((r) => r.configuracion),
  catalogo: () => pedir<{ modelos: Modelo[] }>('GET', '/api/v1/modelos').then((r) => r.modelos),
  fijarPredeterminado: (modelo: string) => pedir<void>('POST', '/api/v1/instancia/predeterminado', { modelo }),
  capacidades: () => pedir<{ adaptadores: Adaptador[]; entradas?: string[] }>('GET', '/api/v1/capacidades'),
  trabajos: (limite = 50) => pedir<{ trabajos: Trabajo[] }>('GET', `/api/v1/trabajos?limite=${limite}`).then((r) => r.trabajos),
  trabajo: (id: string) => pedir<Trabajo>('GET', `/api/v1/trabajos/${encodeURIComponent(id)}`),
  crear: (s: Solicitud) => pedir<Trabajo>('POST', '/api/v1/trabajos', s),
  accion: (id: string, accion: 'cancelar' | 'reintentar') =>
    pedir<Trabajo>('POST', `/api/v1/trabajos/${encodeURIComponent(id)}/${accion}`),
  eliminar: (id: string) => pedir<void>('DELETE', `/api/v1/trabajos/${encodeURIComponent(id)}`),
  vaciarHistorial: () => pedir<{ eliminados: number }>('DELETE', '/api/v1/trabajos').then((r) => r.eliminados),
  subir: (archivo: File) => {
    const datos = new FormData()
    datos.append('archivo', archivo)
    return pedir<{ identificador_carga: string }>('POST', '/api/v1/cargas', datos)
  },
  puntos: () => pedir<{ puntos_acceso: EstadoPunto[] }>('GET', '/api/v1/puntos').then((r) => r.puntos_acceso),
  crearPunto: (p: ConfigPunto) => pedir<{ punto_acceso?: EstadoPunto; token: string }>('POST', '/api/v1/puntos', p),
  activarPunto: (nombre: string, activo: boolean) => pedir<unknown>('PATCH', `/api/v1/puntos/${nombre}`, { activo }),
  instruccionesPunto: (nombre: string, instrucciones: string) => pedir<unknown>('PATCH', `/api/v1/puntos/${nombre}`, { instrucciones }),
  eliminarPunto: (nombre: string) => pedir<void>('DELETE', `/api/v1/puntos/${nombre}`),
  tokenPunto: (nombre: string) => pedir<{ token: string }>('GET', `/api/v1/puntos/${nombre}/token`).then((r) => r.token),
}

// trabajoSeguido es el trabajo que la página Ejecutar debe seguir en vivo al abrirse desde el historial.
export const trabajoSeguido = ref('')

export const terminales = ['completado', 'fallido', 'cancelado', 'interrumpido']

// estados da a cada estado un color y un icono; el nombre sale de las traducciones (estado.<estado>).
const estados: Record<string, { color: string; icono: string }> = {
  en_cola: { color: 'grey-7', icono: 'schedule' },
  preparando_entrada: { color: 'info', icono: 'description' },
  esperando_agente: { color: 'info', icono: 'hourglass_top' },
  ejecutando: { color: 'primary', icono: 'autorenew' },
  validando_salida: { color: 'primary', icono: 'rule' },
  cancelacion_solicitada: { color: 'warning', icono: 'pending' },
  completado: { color: 'positive', icono: 'check_circle' },
  fallido: { color: 'negative', icono: 'error' },
  cancelado: { color: 'grey-7', icono: 'block' },
  interrumpido: { color: 'warning', icono: 'report' },
}

export function estadoInfo(estado: string) {
  const e = estados[estado]
  return e ? { ...e, etiqueta: t(`estado.${estado}` as Clave) } : { etiqueta: estado, color: 'grey-7', icono: 'help' }
}

export function fecha(texto?: string): string {
  return texto ? new Date(texto).toLocaleString(idioma.value) : '—'
}

// haceCuanto da la fecha relativa ("hace 3 minutos"); más de una semana, la fecha corta.
export function haceCuanto(texto?: string): string {
  if (!texto) return '—'
  const segundos = (new Date(texto).getTime() - Date.now()) / 1000
  const unidades: [Intl.RelativeTimeFormatUnit, number][] = [['second', 60], ['minute', 3600], ['hour', 86400], ['day', 604800]]
  let anterior = 1
  for (const [unidad, limite] of unidades) {
    if (Math.abs(segundos) < limite)
      return new Intl.RelativeTimeFormat(idioma.value, { numeric: 'auto' }).format(Math.round(segundos / anterior), unidad)
    anterior = limite
  }
  return new Date(texto).toLocaleDateString(idioma.value)
}

export function duracion(desde?: string, hasta?: string): string {
  if (!desde || !hasta) return ''
  const s = (new Date(hasta).getTime() - new Date(desde).getTime()) / 1000
  return s < 60 ? `${s.toFixed(1)} s` : `${Math.floor(s / 60)} min ${Math.round(s % 60)} s`
}

const agentes: Record<string, string> = { claude: 'Claude', codex: 'Codex', gemini: 'Gemini', antigravity: 'Antigravity' }
const perfiles = ['rapido', 'equilibrado', 'profundo']

export function nombreAgente(agente?: string): string {
  return agente ? (agentes[agente] ?? agente) : '—'
}

// etiquetaModelo da el nombre legible de agente o agente/modelo: "Claude · sonnet".
function etiquetaModelo(id: string): string {
  const [agente, ...resto] = id.split('/')
  return resto.length ? `${nombreAgente(agente)} · ${resto.join('/')}` : nombreAgente(agente)
}

export interface OpcionModelo {
  label: string
  caption: string
  value: string
  predeterminado: boolean
}

// opcionModelo convierte una entrada del catálogo en una opción legible, con la versión real si se conoce.
export function opcionModelo(m: Modelo): OpcionModelo {
  let label = etiquetaModelo(m.id)
  let caption = m.version || t('modelo.sinVersion')
  if (m.perfil) {
    label = t('modelo.perfil', { nombre: perfiles.includes(m.perfil) ? t(`perfil.${m.perfil}` as Clave) : m.perfil })
    caption = [m.resuelto ? etiquetaModelo(m.resuelto) : t('modelo.perfilDetalle'), m.version].filter(Boolean).join(' · ')
  } else if (!m.modelo) {
    caption = [m.version, t('modelo.predeterminadoCli')].filter(Boolean).join(' · ')
  }
  return { label, caption, value: m.id, predeterminado: !!m.predeterminado }
}

// descripcionPredeterminado resume el modelo predeterminado del catálogo: "Claude · sonnet · claude-sonnet-5-5".
export function descripcionPredeterminado(catalogo: Modelo[]): string {
  const m = catalogo.find((x) => x.predeterminado)
  if (!m) return ''
  const o = opcionModelo(m)
  return `${o.label} · ${o.caption}`
}

// aplicarModelo traduce la selección del catálogo a la solicitud.
export function aplicarModelo(s: Solicitud, seleccion: string): Solicitud {
  if (!seleccion) return s
  if (seleccion.startsWith('perfil/')) return { ...s, perfil_modelo: seleccion.slice(7) }
  const [agente, ...resto] = seleccion.split('/')
  return { ...s, agente, modelo: resto.join('/') || undefined }
}

export function modeloDe(trabajo: Trabajo): string {
  const modelo = trabajo.modelo_efectivo || trabajo.modelo_enviado
  return [nombreAgente(trabajo.solicitud.agente), modelo].filter((x) => x && x !== '—').join(' · ') || '—'
}

export function textoDe(trabajo: Trabajo): string {
  return trabajo.solicitud.mensajes?.at(-1)?.contenido ?? (trabajo.entradas_eliminadas ? t('trabajo.entradaEliminada') : '')
}

// esJSON distingue una respuesta JSON (se muestra en monoespaciada) del texto normal.
export function esJSON(texto?: string): boolean {
  return !!texto && /^\s*[[{]/.test(texto)
}

export function mensajeError(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export function avisarError(e: unknown) {
  Notify.create({ type: 'negative', message: mensajeError(e), position: 'top' })
}

export function avisar(mensaje: string) {
  Notify.create({ type: 'positive', message: mensaje, position: 'top', timeout: 2000 })
}

export async function copiar(texto: string, que = t('comun.copiado')) {
  try {
    await navigator.clipboard.writeText(texto)
    Notify.create({ message: que, icon: 'content_copy', timeout: 1200, position: 'top' })
  } catch {
    Notify.create({ type: 'warning', message: t('comun.noCopiado'), position: 'top' })
  }
}
