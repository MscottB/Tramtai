// Tipos de lo que envía y recibe el servicio. Los del contrato se generan desde Go (tipos.gen.ts, con go generate
// ./internal/contrato); aquí quedan la sesión, que el servicio arma sin un tipo de contrato, y lo que solo envía la
// Web UI.
import type { AjustesServicio, Presupuesto } from './tipos.gen'

export type * from './tipos.gen'

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
  // entorno: servidor, portable o escritorio (la marca de la cabecera); sin él, un servicio anterior: este equipo.
  entorno?: string
  instancias?: InstanciaResumida[]
  // autoarranque: si Tramtai arranca al iniciar sesión y modo_arranque, en qué modo (la app en la bandeja o solo el
  // servicio); solo en sesiones locales de sistemas que lo admiten.
  autoarranque?: boolean
  modo_arranque?: AjustesServicio['modo_arranque']
  // servicio: el puerto en uso, el configurado y si se puede reiniciar desde aquí (solo sesiones locales).
  servicio?: Omit<AjustesServicio, 'autoarranque' | 'modo_arranque'>
  // almacen: dónde guarda este sistema las claves de proveedor, cifradas y fuera de Tramtai (vacío si no hay).
  almacen?: { disponible: boolean; nombre: string }
}
// Novedad es una versión publicada con sus notas, para el diálogo que sale tras actualizar.
export interface Novedad {
  version: string
  url?: string
  notas?: string
}

// AjustesPunto son los cambios de una API (PATCH): solo cambia lo que se envía. modelo es un id del catálogo y ''
// vuelve al del espacio; esquema null lo quita.
export interface AjustesPunto {
  instrucciones?: string
  esfuerzo?: string
  modelo?: string
  forzar_modelo?: boolean
  modelos_permitidos?: string[]
  esquema?: Record<string, unknown> | null
  respaldo?: string[]
  reintentos_salida?: number
  presupuesto?: Presupuesto | null
}
