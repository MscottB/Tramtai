// indice ordena los temas de la Ayuda y dice cuál abre el botón ? de cada página de la app.
import type { Pagina, Tema } from './tipos'
import { empezar } from './temas/empezar'
import { agentes } from './temas/agentes'
import { probar } from './temas/probar'
import { conectar } from './temas/conectar'
import { archivos } from './temas/archivos'
import { json } from './temas/json'
import { recetas } from './temas/recetas'
import { herramientas } from './temas/herramientas'
import { presupuestos } from './temas/presupuestos'
import { historial } from './temas/historial'
import { ajustes } from './temas/ajustes'
import { problemas } from './temas/problemas'
import { conceptos } from './temas/conceptos'
import { preguntas } from './temas/preguntas'

export const temas: Tema[] = [
  empezar, agentes, probar, conectar, archivos, json, recetas, herramientas, presupuestos, historial, ajustes,
  problemas, conceptos, preguntas,
]

export function temaDe(pagina: Pagina): Tema | undefined {
  return temas.find((t) => t.paginaApp === pagina)
}

export function temaPorId(id: string): Tema | undefined {
  return temas.find((t) => t.id === id)
}
