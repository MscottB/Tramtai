// vivo convierte los eventos de GET /api/v1/eventos en los tranvías del mapa de Inicio: dónde va cada petición en
// curso y cuándo deja de verse. Sin Vue, para poder probarlo; MapaRed.vue solo dibuja lo que sale de aquí.

import type { Clave } from './idiomas/es'

export type Etapa = 'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'
export type Tramo = 'entrada' | 'centro' | 'linea'

export interface EventoTrabajo {
  id: string
  estado: string
  etapa: Etapa
  agente: string
  modelo: string
  api: string
  origen: string
  intento: number
  codigo_error: string
  actualizado: string
}

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
  // desde es cuándo apareció en el mapa; terminadoEn, cuándo llegó (o falló), para borrarlo después.
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
    codigoError: e.codigo_error, desde: previo?.desde ?? ahora, terminadoEn: e.etapa === 'llegada' ? (previo?.terminadoEn ?? ahora) : undefined,
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

export function posicion(tr: Tranvia): { tramo: Tramo; avance: number } {
  return { tramo: tr.tramo, avance: tr.avance }
}

// visibles son los que caben en el mapa (los más recientes) y cuántos quedan fuera.
export function visibles(t: Tranvias, max = 12): { lista: Tranvia[]; resto: number } {
  const todos = [...t.values()].sort((a, b) => b.desde - a.desde)
  return { lista: todos.slice(0, max), resto: Math.max(0, todos.length - max) }
}

// etapaDe agrupa un estado en una de las 5 paradas, igual que EtapaDe del servicio (internal/api/vivo.go); la usa el
// mapa cuando no hay flujo y consulta los trabajos cada 5 s.
export function etapaDe(estado: string): Etapa {
  if (estado === 'en_cola') return 'cola'
  if (estado === 'preparando_entrada' || estado === 'esperando_agente') return 'preparacion'
  if (estado === 'ejecutando' || estado === 'cancelacion_solicitada') return 'ejecucion'
  if (estado === 'validando_salida') return 'validacion'
  return 'llegada'
}

// esperaReconexion es la espera antes del intento n de reconectar el flujo: 1, 2, 4, 8, 16 s y después 30 s.
export function esperaReconexion(intento: number): number {
  return Math.min(1000 * 2 ** intento, 30000)
}
