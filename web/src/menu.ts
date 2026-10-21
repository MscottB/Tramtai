// El menú lateral, por grupos y en el orden en que se usa la app: empezar, conectar y organizar.
import type { Pagina } from './ayuda/tipos'

export type GrupoMenu = 'empezar' | 'conectar' | 'organizar'

export const GRUPOS_MENU: { grupo: GrupoMenu; paginas: Pagina[] }[] = [
  { grupo: 'empezar', paginas: ['inicio', 'agentes', 'ejecutar'] },
  { grupo: 'conectar', paginas: ['conectar', 'historial'] },
  { grupo: 'organizar', paginas: ['espacios', 'configuracion'] },
]

// ordenMenu devuelve las páginas del menú en orden, de arriba abajo.
export const ordenMenu = (): Pagina[] => GRUPOS_MENU.flatMap((g) => g.paginas)
