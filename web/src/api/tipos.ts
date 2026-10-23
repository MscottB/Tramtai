// Tipos de lo que envía y recibe el servicio. Los del contrato se generan desde Go (tipos.gen.ts, con go generate
// ./internal/contrato); aquí quedan los de las rutas /ui de la sesión, que el servicio arma sin un tipo de contrato.
import type { Presupuesto } from './tipos.gen'

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
  instancias?: InstanciaResumida[]
  // autoarranque: si Tramtai arranca al iniciar sesión; solo en sesiones locales de sistemas que lo admiten.
  autoarranque?: boolean
  // servicio: el puerto en uso, el configurado y si se puede reiniciar desde aquí (solo sesiones locales).
  servicio?: { puerto: number; puerto_configurado: number; puerto_automatico: boolean; reiniciable: boolean }
  // almacen: dónde guarda este sistema las claves de proveedor, cifradas y fuera de Tramtai (vacío si no hay).
  almacen?: { disponible: boolean; nombre: string }
}
export interface EstadoActualizacion {
  actual: string
  buscar: boolean
  disponible: boolean
  version: string
  url: string
  notas?: string
  instalable: boolean
  comprobado?: string // cuándo consultó el servicio a GitHub por última vez
  fallo?: boolean // si esa consulta falló
}

// Acerca son los datos de «Acerca de Tramtai»; rutas solo llega a una sesión local.
export interface Acerca {
  version: string
  commit?: string
  modificado?: boolean
  go: string
  sistema: string
  arquitectura: string
  almacen: string
  servicio: { direccion: string; pid: number; iniciado_en: string }
  rutas?: { datos: string; configuracion: string; registro: string }
  portable?: boolean // la copia guarda sus datos junto al .exe (portable.txt)
}
// Novedad es una versión publicada con sus notas, para el diálogo que sale tras actualizar.
export interface Novedad {
  version: string
  url?: string
  notas?: string
}

// ProgresoActualizacion es el avance de «Actualizar ahora»: fase vacía si no hay ninguna en curso.
export interface ProgresoActualizacion {
  fase: '' | 'descargando' | 'verificando' | 'listo' | 'instalando' | 'error'
  version: string
  recibidos: number
  total: number
  instala_en?: string // en la fase listo: cuándo instalará si nadie decide antes
  error?: string
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
