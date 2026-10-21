// Los recorridos guiados salen de los bloques `recorrido` de la Ayuda: aquí solo se arman las listas de pasos.
import { temas as temasAyuda } from './ayuda/indice'
import type { Pagina, PasoRecorrido, Tema } from './ayuda/tipos'
import type { Clave } from './idiomas/es'

// PasoMostrado es un paso ya colocado en su página; sin ancla es un paso centrado, sin señalar nada.
export interface PasoMostrado {
  pagina: Pagina
  ancla?: string
  titulo: Clave
  texto: Clave
}

// ORDEN_BIENVENIDA es el orden de la bienvenida; Historial y Ajustes tienen su propio recorrido desde su ?.
// Sigue el orden del menú (menu.ts: menu.test.ts comprueba que es una subsecuencia de ordenMenu()).
export const ORDEN_BIENVENIDA: Pagina[] = ['inicio', 'agentes', 'ejecutar', 'conectar', 'espacios']

function pasosDelTema(pagina: Pagina, temas: Tema[]): PasoRecorrido[] {
  return temas.filter((t) => t.paginaApp === pagina).flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'recorrido' ? b.pasos : [])))
}

const mostrar = (pagina: Pagina) => ({ ancla, titulo, texto }: PasoRecorrido): PasoMostrado => ({ pagina, ancla, titulo, texto })

export function pasosDePagina(pagina: Pagina, temas: Tema[] = temasAyuda): PasoMostrado[] {
  return pasosDelTema(pagina, temas).map(mostrar(pagina))
}

export function pasosBienvenida(temas: Tema[] = temasAyuda): PasoMostrado[] {
  const pasos = ORDEN_BIENVENIDA.flatMap((pagina) => pasosDelTema(pagina, temas).filter((p) => p.bienvenida).map(mostrar(pagina)))
  return [...pasos, { pagina: 'espacios', titulo: 'recorrido.finTitulo', texto: 'recorrido.finTexto' }]
}
