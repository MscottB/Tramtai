// Cliente de la API de Tramtai para la Web UI: usa la cookie de sesión (mismo origen) y envía el token CSRF en
// las peticiones que modifican. El token de la instancia solo se escribe al entrar; nunca se guarda en el navegador.
import { ref } from 'vue'

export interface Sesion {
  instancia: string
  ambito: string
  raiz?: string
  csrf: string
  version: string
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
  mensajes: Mensaje[]
  cargas?: string[]
  salida?: { formato?: string; esquema?: unknown }
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
  puerto?: number
  contratos?: string[]
  agente?: string
  modelo?: string
  perfil_modelo?: string
  modelos_permitidos?: string[]
  forzar_modelo?: boolean
  alias_modelos?: Record<string, string>
}
export interface EntradaConfiguracion {
  clave: string
  valor: unknown
  origen: string
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

function recordar(s: Sesion): Sesion {
  csrf = s.csrf
  return s
}

export const api = {
  sesion: () => pedir<Sesion>('GET', '/ui/sesion').then(recordar),
  entrar: (token: string) => pedir<Sesion>('POST', '/ui/sesion', { token }).then(recordar),
  salir: () => pedir<void>('DELETE', '/ui/sesion'),
  instancia: () => pedir<EstadoInstancia>('GET', '/api/v1/instancia'),
  configuracion: () =>
    pedir<{ configuracion: EntradaConfiguracion[] }>('GET', '/api/v1/instancia/configuracion').then((r) => r.configuracion),
  modelos: () => pedir<{ modelos: { id: string }[] }>('GET', '/api/v1/modelos').then((r) => r.modelos.map((m) => m.id)),
  capacidades: () => pedir<Record<string, any>>('GET', '/api/v1/capacidades'),
  trabajos: (limite = 50) => pedir<{ trabajos: Trabajo[] }>('GET', `/api/v1/trabajos?limite=${limite}`).then((r) => r.trabajos),
  trabajo: (id: string) => pedir<Trabajo>('GET', `/api/v1/trabajos/${encodeURIComponent(id)}`),
  crear: (s: Solicitud) => pedir<Trabajo>('POST', '/api/v1/trabajos', s),
  accion: (id: string, accion: 'cancelar' | 'reintentar') =>
    pedir<Trabajo>('POST', `/api/v1/trabajos/${encodeURIComponent(id)}/${accion}`),
  eliminar: (id: string) => pedir<void>('DELETE', `/api/v1/trabajos/${encodeURIComponent(id)}`),
  subir: (archivo: File) => {
    const datos = new FormData()
    datos.append('archivo', archivo)
    return pedir<{ identificador_carga: string }>('POST', '/api/v1/cargas', datos)
  },
  puntos: () => pedir<{ puntos_acceso: EstadoPunto[] }>('GET', '/api/v1/puntos').then((r) => r.puntos_acceso),
  crearPunto: (p: ConfigPunto) => pedir<{ punto_acceso?: EstadoPunto; token: string }>('POST', '/api/v1/puntos', p),
  activarPunto: (nombre: string, activo: boolean) => pedir<unknown>('PATCH', `/api/v1/puntos/${nombre}`, { activo }),
  eliminarPunto: (nombre: string) => pedir<void>('DELETE', `/api/v1/puntos/${nombre}`),
  tokenPunto: (nombre: string) => pedir<{ token: string }>('GET', `/api/v1/puntos/${nombre}/token`).then((r) => r.token),
}

// trabajoSeguido es el trabajo que la página Ejecutar debe seguir en vivo al abrirse desde Trabajos.
export const trabajoSeguido = ref('')

export const terminales = ['completado', 'fallido', 'cancelado', 'interrumpido']

export function colorEstado(estado: string): string {
  return { completado: 'positive', fallido: 'negative', interrumpido: 'negative', cancelado: 'grey' }[estado] ?? 'warning'
}

export function fecha(texto?: string): string {
  return texto ? new Date(texto).toLocaleString() : '—'
}

// aplicarModelo traduce la selección del catálogo (agente, agente/modelo o perfil/nombre) a la solicitud.
export function aplicarModelo(s: Solicitud, seleccion: string): Solicitud {
  if (!seleccion) return s
  if (seleccion.startsWith('perfil/')) return { ...s, perfil_modelo: seleccion.slice(7) }
  const [agente, ...resto] = seleccion.split('/')
  return { ...s, agente, modelo: resto.join('/') || undefined }
}

export function modeloDe(t: Trabajo): string {
  return [t.solicitud.agente, t.modelo_efectivo || t.modelo_enviado].filter(Boolean).join('/') || '—'
}

export function mensajeError(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
