// La bienvenida se ofrece una vez: con la sesión abierta, ya en Inicio y sin haberla hecho o descartado antes.
import type { Pagina } from './ayuda/tipos'

export type RespuestaBienvenida = 'hecha' | 'descartada'

// leerBienvenida es lo guardado en tramtai.bienvenida: cualquier otro valor cuenta como no respondida.
export const leerBienvenida = (v: string | null): RespuestaBienvenida | null => (v === 'hecha' || v === 'descartada' ? v : null)

// alCerrarBienvenida: «Ahora no», Esc o clic fuera la descartan (no se insiste); «Empezar» ya la marcó hecha.
export const alCerrarBienvenida = (empezada: boolean): RespuestaBienvenida | undefined => (empezada ? undefined : 'descartada')

// entradaPorEnlace: se abrió Tramtai en una página concreta (un enlace profundo), no por la puerta de siempre.
export const entradaPorEnlace = (hash: string): boolean => !['', '#', '#inicio'].includes(hash)

export function ofrecerBienvenida(o: {
  sesion: boolean
  pagina: Pagina
  entradaPorEnlace: boolean
  estado: RespuestaBienvenida | null
  yaOfrecida: boolean
  otroDialogo: boolean // Novedades, Actualizar, Abrir proyecto o Acerca de: se espera a que se cierre, sin dar la bienvenida por ofrecida
  recorridoActivo: boolean // un recorrido de página que cruza Inicio no deja la bienvenida encima: espera a que acabe
}): boolean {
  return o.sesion && o.pagina === 'inicio' && !o.entradaPorEnlace && o.estado === null && !o.yaOfrecida && !o.otroDialogo && !o.recorridoActivo
}
