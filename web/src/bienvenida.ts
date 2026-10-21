// La bienvenida se ofrece una vez: con la sesión abierta, ya en Inicio y sin haberla hecho o descartado antes.
import type { Pagina } from './ayuda/tipos'

// entradaPorEnlace: se abrió Tramtai en una página concreta (un enlace profundo), no por la puerta de siempre.
export const entradaPorEnlace = (hash: string): boolean => !['', '#', '#inicio'].includes(hash)

export function ofrecerBienvenida(o: {
  sesion: boolean
  pagina: Pagina
  entradaPorEnlace: boolean
  estado: 'hecha' | 'descartada' | null
  yaOfrecida: boolean
}): boolean {
  return o.sesion && o.pagina === 'inicio' && !o.entradaPorEnlace && o.estado === null && !o.yaOfrecida
}
