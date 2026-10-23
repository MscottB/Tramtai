// vivo convierte los eventos de GET /api/v1/eventos en los tranvías del mapa de Inicio: dónde va cada petición en
// curso y cuándo deja de verse. Sin Vue, para poder probarlo; MapaRed.vue solo dibuja lo que sale de aquí.

import type { EventoTrabajo, Trabajo } from './api/tipos'
import type { Clave } from './idiomas/es'

// Etapa es la parada de la ruta en que va una petición; el evento «trabajo» (tipo generado desde Go) la trae calculada.
export type Etapa = EventoTrabajo['etapa']
export type Tramo = 'entrada' | 'centro' | 'linea'

export type { EventoTrabajo }

export interface Tranvia {
  id: string
  agente: string
  modelo: string
  api: string
  origen: string
  etapa: Etapa
  estado: string
  intento: number
  codigoError: string
  // desde es cuándo se aceptó la petición (o, si no se sabe, cuándo apareció en el mapa); terminadoEn, cuándo llegó
  // (o falló), para borrarlo después.
  desde: number
  terminadoEn?: number
  tramo: Tramo
  avance: number
}

export type Tranvias = Map<string, Tranvia>

// CLAVES_ETAPA son los textos de cada etapa («en ejecución»…) para los nombres de los tranvías.
export const CLAVES_ETAPA: Record<Etapa, Clave> = { cola: 'vivo.etapaCola', preparacion: 'vivo.etapaPreparacion', ejecucion: 'vivo.etapaEjecucion',
  validacion: 'vivo.etapaValidacion', llegada: 'vivo.etapaLlegada' }

// PARADAS es dónde está cada etapa: el tramo del mapa y cuánto avanzó por él (de 0 a 1).
const PARADAS: Record<Etapa, { tramo: Tramo; avance: number }> = {
  cola: { tramo: 'entrada', avance: 0.5 },
  preparacion: { tramo: 'centro', avance: 0 },
  ejecucion: { tramo: 'linea', avance: 0.45 },
  validacion: { tramo: 'linea', avance: 0.85 },
  llegada: { tramo: 'linea', avance: 1 },
}

// VISIBLE es cuánto sigue en el mapa una petición terminada, por estado (ms).
const VISIBLE: Record<string, number> = { completado: 2000, cancelado: 2000, fallido: 4000, interrumpido: 4000 }

export function aplicar(t: Tranvias, e: EventoTrabajo, ahora: number, borrados: Set<string>): void {
  if (borrados.has(e.id)) return // ya llegó y se borró: un evento tardío no lo hace volver
  const previo = t.get(e.id)
  // Un fallo o una cancelación se quedan donde iba el tranvía; una llegada correcta, al final de su línea.
  const parada = e.etapa === 'llegada' && e.estado !== 'completado' ? (previo ? { tramo: previo.tramo, avance: previo.avance } : PARADAS.preparacion) : PARADAS[e.etapa]
  t.set(e.id, {
    id: e.id, agente: e.agente, modelo: e.modelo, api: e.api, origen: e.origen, etapa: e.etapa, estado: e.estado, intento: e.intento,
    codigoError: e.codigo_error, desde: previo?.desde ?? (Date.parse(e.creado) || ahora), terminadoEn: e.etapa === 'llegada' ? (previo?.terminadoEn ?? ahora) : undefined,
    tramo: parada.tramo, avance: parada.avance,
  })
}

export function limpiar(t: Tranvias, ahora: number, borrados: Set<string>): void {
  for (const [id, tr] of t)
    if (tr.terminadoEn !== undefined && ahora - tr.terminadoEn >= (VISIBLE[tr.estado] ?? 2000)) {
      t.delete(id)
      borrados.add(id)
    }
}

// podar quita los tranvías que no llegaron y no están entre presentes: lo que la foto de un flujo nuevo no trajo ya
// terminó hace rato (un corte largo del flujo no deja tranvías congelados).
export function podar(t: Tranvias, presentes: Set<string>): void {
  for (const [id, tr] of t) if (tr.terminadoEn === undefined && !presentes.has(id)) t.delete(id)
}

// reconciliar pone el mapa al día con una consulta de los trabajos recientes cuando no hay flujo: los del mapa que
// terminaron llegan, los que ya no salen se van y los terminados que no estaban no aparecen.
export function reconciliar(t: Tranvias, eventos: EventoTrabajo[], ahora: number, borrados: Set<string>): void {
  for (const e of eventos) if (e.etapa !== 'llegada' || t.has(e.id)) aplicar(t, e, ahora, borrados)
  podar(t, new Set(eventos.map((e) => e.id)))
}

// eventoDe convierte un trabajo de la consulta en el evento que habría mandado el flujo.
export function eventoDe(t: Trabajo): EventoTrabajo {
  return { id: t.id, estado: t.estado, etapa: t.etapa, agente: t.solicitud?.agente ?? '', modelo: t.modelo_efectivo || t.modelo_enviado || '',
    api: t.punto_acceso ?? '', origen: t.origen ?? '', intento: 1, codigo_error: t.codigo_error ?? '', creado: t.creado_en, actualizado: '' }
}

// claveEtapa es el texto de dónde va un tranvía; la llegada se nombra según cómo terminó.
export function claveEtapa(tr: Tranvia): Clave {
  if (tr.etapa !== 'llegada' || tr.estado === 'completado') return CLAVES_ETAPA[tr.etapa]
  return tr.estado === 'cancelado' ? 'vivo.etapaCancelada' : 'vivo.etapaFallida'
}

export function posicion(tr: Tranvia): { tramo: Tramo; avance: number } {
  return { tramo: tr.tramo, avance: tr.avance }
}

// visibles son los que caben en el mapa (los más recientes) y cuántos quedan fuera.
export function visibles(t: Tranvias, max = 12): { lista: Tranvia[]; resto: number } {
  const todos = [...t.values()].sort((a, b) => b.desde - a.desde)
  return { lista: todos.slice(0, max), resto: Math.max(0, todos.length - max) }
}

// esperaReconexion es la espera antes del intento n de reconectar el flujo: 1, 2, 4, 8, 16 s y después 30 s.
export function esperaReconexion(intento: number): number {
  return Math.min(1000 * 2 ** intento, 30000)
}
