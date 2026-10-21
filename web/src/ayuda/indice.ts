// indice ordena los temas de la Ayuda y dice cuál abre el botón ? de cada página de la app.
import type { Clave } from '../idiomas/es'
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
import { espacios } from './temas/espacios'
import { claves } from './temas/claves'
import { problemas } from './temas/problemas'
import { conceptos } from './temas/conceptos'
import { preguntas } from './temas/preguntas'

export const temas: Tema[] = [
  empezar, agentes, probar, conectar, archivos, json, recetas, herramientas, presupuestos, historial, espacios, claves, ajustes,
  problemas, conceptos, preguntas,
]

export function temaDe(pagina: Pagina): Tema | undefined {
  return temas.find((t) => t.paginaApp === pagina)
}

export function temaPorId(id: string): Tema | undefined {
  return temas.find((t) => t.id === id)
}

// CLAVES_UI son los textos de la Ayuda que no son de un tema: el inicio, la vista de tema y el panel.
export const CLAVES_UI: Clave[] = [
  'ayuda.subtitulo', 'ayuda.buscar', 'ayuda.sinResultados', 'ayuda.mostrarGuia', 'ayuda.desarrolladores', 'ayuda.desarrolladoresTexto',
  'ayuda.documentacion', 'ayuda.documentacionTexto', 'ayuda.ir', 'ayuda.ui.relacionados', 'ayuda.ui.abrirWeb', 'ayuda.abrirPanel',
  'ayuda.ui.abrirEnAyuda', 'ayuda.ui.queHago', 'ayuda.ui.saberMas', 'ayuda.ui.resultados', 'ayuda.ui.resultadosUno',
]
