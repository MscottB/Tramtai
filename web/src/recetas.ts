// La receta de una API (lo que se edita en Probar y en Mis APIs) y su paso a la solicitud y a la configuración.
import { t } from './i18n'
import type { AjustesPunto, ConfigPunto, EstadoPunto, Presupuesto, Solicitud, Trabajo } from './api/tipos'

// Receta es lo que define una API y se prueba en Probar: instrucciones, modelo, esfuerzo, respaldo, esquema (como
// texto del editor; '' = sin respuesta con forma) y cuántas veces se corrige una respuesta que no lo cumple (0 a 3).
export interface Receta {
  instrucciones: string
  modelo: string | null
  forzar: boolean
  esfuerzo: string
  respaldo: string[]
  esquema: string
  reintentos: number
}

export const recetaVacia = (): Receta => ({ instrucciones: '', modelo: null, forzar: false, esfuerzo: '', respaldo: [], esquema: '', reintentos: 0 })

// idModeloDe es el modelo de una API como id del catálogo (agente, agente/modelo o perfil/nombre); null si no tiene.
export const idModeloDe = (p: EstadoPunto) => (p.perfil_modelo ? 'perfil/' + p.perfil_modelo : [p.agente, p.modelo].filter(Boolean).join('/') || null)

export function recetaDePunto(p: EstadoPunto): Receta {
  return { instrucciones: p.instrucciones ?? '', modelo: idModeloDe(p), forzar: !!p.forzar_modelo, esfuerzo: p.esfuerzo ?? '',
    respaldo: [...(p.respaldo ?? [])], esquema: p.esquema ? JSON.stringify(p.esquema, null, 2) : '', reintentos: p.reintentos_salida ?? 0 }
}

// recetaDeSolicitud rehace la receta de una petición ya enviada (Usar de nuevo en Probar). Si respondió un agente de
// respaldo, la solicitud guardada ya apunta a él: la receta empieza por el agente que respondió.
export function recetaDeSolicitud(s: Solicitud): Receta {
  return { instrucciones: s.instrucciones ?? '', forzar: false, esfuerzo: s.esfuerzo ?? '', respaldo: [...(s.respaldo ?? [])],
    modelo: s.perfil_modelo ? 'perfil/' + s.perfil_modelo : [s.agente, s.modelo].filter(Boolean).join('/') || null,
    esquema: s.salida?.esquema ? JSON.stringify(s.salida.esquema, null, 2) : '', reintentos: s.salida?.reintentos ?? 0 }
}

// mensajeDe es el último mensaje del usuario de una petición; '' si sus entradas ya se borraron. Una corrección añade
// la respuesta y el pedido de corrección (el texto fijo que escribe el servicio): se muestra el mensaje de antes.
export function mensajeDe(x: Trabajo): string {
  const m = x.solicitud.mensajes ?? []
  let fin = m.length
  while (fin >= 3 && m[fin - 2].rol === 'asistente' && m[fin - 1].rol === 'usuario' && m[fin - 1].contenido.startsWith(prefijoCorreccion)) fin -= 2
  return m.slice(0, fin).filter((x) => x.rol === 'usuario').at(-1)?.contenido ?? ''
}
const prefijoCorreccion = 'Tu respuesta no cumple el formato pedido:'

// esquemaDeReceta convierte el texto del esquema en un objeto; lanza un error legible si no es un objeto JSON.
export function esquemaDeReceta(r: Receta): Record<string, unknown> | undefined {
  if (!r.esquema.trim()) return undefined
  let valor: unknown
  try {
    valor = JSON.parse(r.esquema)
  } catch {
    throw new Error(t('receta.esquemaInvalido'))
  }
  if (!valor || typeof valor !== 'object' || Array.isArray(valor)) throw new Error(t('receta.esquemaInvalido'))
  return valor as Record<string, unknown>
}

// configDeReceta son los campos de la receta para crear una API; los reintentos solo valen con esquema.
export function configDeReceta(r: Receta): Partial<ConfigPunto> {
  const m = aplicarModelo({ mensajes: [] }, r.modelo ?? '')
  const esquema = esquemaDeReceta(r)
  return { instrucciones: r.instrucciones.trim() || undefined, agente: m.agente, modelo: m.modelo, perfil_modelo: m.perfil_modelo,
    forzar_modelo: (r.forzar && !!r.modelo) || undefined, esfuerzo: r.esfuerzo || undefined,
    respaldo: r.respaldo.length ? r.respaldo : undefined, esquema, reintentos_salida: (esquema && r.reintentos) || undefined }
}

// ajustesDeReceta son los cambios para actualizar una API con la receta.
export function ajustesDeReceta(r: Receta): AjustesPunto {
  const esquema = esquemaDeReceta(r)
  return { instrucciones: r.instrucciones, esfuerzo: r.esfuerzo, modelo: r.modelo ?? '', forzar_modelo: r.forzar && !!r.modelo,
    respaldo: r.respaldo, esquema: esquema ?? null, reintentos_salida: esquema ? r.reintentos : 0 }
}

// aplicarModelo traduce la selección del catálogo a la solicitud.
export function aplicarModelo(s: Solicitud, seleccion: string): Solicitud {
  if (!seleccion) return s
  if (seleccion.startsWith('perfil/')) return { ...s, perfil_modelo: seleccion.slice(7) }
  const [agente, ...resto] = seleccion.split('/')
  return { ...s, agente, modelo: resto.join('/') || undefined }
}

// dividirRespaldo separa una entrada de respaldo (agente[/modelo][+esfuerzo]) en su base y su esfuerzo propio.
export function dividirRespaldo(e: string): { base: string; esfuerzo: string } {
  const [base, esfuerzo = ''] = e.split('+')
  return { base, esfuerzo }
}
export const unirRespaldo = (base: string, esfuerzo: string) => (esfuerzo ? `${base}+${esfuerzo}` : base)

// presupuestoLimpio deja solo los límites mayores que 0; sin ninguno devuelve undefined (sin presupuesto).
export function presupuestoLimpio(p: Presupuesto): Presupuesto | undefined {
  const limpio: Presupuesto = {}
  for (const periodo of ['diario', 'mensual'] as const) {
    const l = Object.fromEntries(Object.entries(p[periodo] ?? {}).filter(([, v]) => Number(v) > 0).map(([k, v]) => [k, Number(v)]))
    if (Object.keys(l).length) limpio[periodo] = l
  }
  return Object.keys(limpio).length ? limpio : undefined
}
