// indice ordena los temas de la Ayuda y dice cuál abre el botón ? de cada página de la app.
import type { Pagina, Tema } from './tipos'
import { empezar } from './temas/empezar'
import { agentes } from './temas/agentes'
import { probar } from './temas/probar'
import { conectar } from './temas/conectar'
import { historial } from './temas/historial'
import { ajustes } from './temas/ajustes'

export const temas: Tema[] = [empezar, agentes, probar, conectar, historial, ajustes]

export function temaDe(pagina: Pagina): Tema | undefined {
  return temas.find((t) => t.paginaApp === pagina)
}

export function temaPorId(id: string): Tema | undefined {
  return temas.find((t) => t.id === id)
}
