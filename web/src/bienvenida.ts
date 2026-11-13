// La bienvenida se ofrece una vez: con la sesión abierta, ya en Inicio y sin haberla hecho o descartado antes.
import type { Pagina } from './ayuda/tipos'
import type { Adaptador } from './api/tipos'

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
  agentesConocidos: boolean // se espera a saber cómo están los agentes, para que la oferta no aparezca con el diálogo abierto
  plazoAgotado: boolean // si la comprobación no llega a tiempo, se abre sin la oferta
}): boolean {
  return (o.agentesConocidos || o.plazoAgotado) && o.sesion && o.pagina === 'inicio' && !o.entradaPorEnlace && o.estado === null && !o.yaOfrecida && !o.otroDialogo && !o.recorridoActivo
}

// OFRECIDOS: los agentes que la bienvenida deja preparar, en orden. Claude Code y Codex vienen marcados; Copilot y
// Antigravity (que solo se instala y luego pide su clave en Agentes) se pueden marcar. Gemini no tiene CLI que instalar.
const OFRECIDOS = [{ agente: 'claude', marcado: true }, { agente: 'codex', marcado: true }, { agente: 'copilot', marcado: false }, { agente: 'antigravity', marcado: false }]

// opcionesAgentes es lo que ofrece «Prepara tus agentes»: vacío si aún no se sabe cómo están los agentes o si alguno ya
// está listo (entonces no hace falta).
export const opcionesAgentes = (adaptadores: Adaptador[] | null): Array<{ agente: string; marcado: boolean }> =>
  adaptadores?.length && !adaptadores.some((a) => a.estado === 'listo') ? OFRECIDOS.filter((o) => adaptadores.some((a) => a.agente === o.agente)) : []
