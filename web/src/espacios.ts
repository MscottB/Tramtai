// espacios ordena los espacios de trabajo y dice qué se puede hacer con cada uno en la página «Espacios». Sin Vue,
// para poder probarlo.
import type { EspacioUI } from './api/tipos'

export type Accion = 'abrir' | 'detener' | 'activar' | 'buscar' | 'olvidar' | 'confiar' | 'dejarDeConfiar'

// accionesDe son los botones de la tarjeta de un espacio; actual es el espacio en el que está la sesión. Solo se olvida
// un proyecto que no está activo, y la global nunca.
export function accionesDe(e: EspacioUI, actual: string): Accion[] {
  const acciones: Accion[] = []
  if (e.estado === 'no_encontrado') acciones.push('buscar')
  else if (e.estado === 'activo') {
    if (e.id !== actual) acciones.push('abrir')
    acciones.push('detener')
  } else acciones.push('activar')
  if (e.ambito === 'proyecto' && e.estado !== 'no_encontrado') acciones.push(e.confiado ? 'dejarDeConfiar' : 'confiar')
  if (e.ambito === 'proyecto' && e.estado !== 'activo') acciones.push('olvidar')
  return acciones
}

// ordenar pone la global primero, después los activos del más al menos usado y al final el resto por nombre.
export function ordenar(lista: EspacioUI[]): EspacioUI[] {
  const uso = (e: EspacioUI) => (e.ultimo_uso ? Date.parse(e.ultimo_uso) : 0)
  return [...lista].sort((a, b) => {
    if ((a.ambito === 'global') !== (b.ambito === 'global')) return a.ambito === 'global' ? -1 : 1
    const activoA = a.estado === 'activo', activoB = b.estado === 'activo'
    if (activoA !== activoB) return activoA ? -1 : 1
    if (activoA && uso(a) !== uso(b)) return uso(b) - uso(a)
    return a.nombre.localeCompare(b.nombre)
  })
}

// nombreConfirmado: para olvidar un espacio hay que escribir su nombre (sin importar mayúsculas ni espacios al borde).
export function nombreConfirmado(e: EspacioUI, escrito: string): boolean {
  return escrito.trim().toLocaleLowerCase() === e.nombre.trim().toLocaleLowerCase()
}
