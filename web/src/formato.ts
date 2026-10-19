// Textos para mostrar: estados, fechas, duraciones, números y esfuerzos en el idioma de la Web UI.
import { idioma, t } from './i18n'
import type { Clave } from './idiomas/es'
import type { EventoRegistro, Trabajo } from './api/tipos'

export const terminales = ['completado', 'fallido', 'cancelado', 'interrumpido']

// estados da a cada estado un color y un icono; el nombre sale de las traducciones (estado.<estado>).
const estados: Record<string, { color: string; icono: string }> = {
  en_cola: { color: 'grey-7', icono: 'schedule' },
  preparando_entrada: { color: 'info', icono: 'description' },
  esperando_agente: { color: 'info', icono: 'hourglass_top' },
  ejecutando: { color: 'primary', icono: 'autorenew' },
  validando_salida: { color: 'primary', icono: 'rule' },
  cancelacion_solicitada: { color: 'warning', icono: 'pending' },
  completado: { color: 'positive', icono: 'check_circle' },
  fallido: { color: 'negative', icono: 'error' },
  cancelado: { color: 'grey-7', icono: 'block' },
  interrumpido: { color: 'warning', icono: 'report' },
}

const clases: Record<string, string> = { positive: 'ok', negative: 'error', warning: 'aviso', info: 'info', primary: 'info' }

export function estadoInfo(estado: string) {
  const e = estados[estado] ?? { color: 'grey-7', icono: 'help' }
  const etiqueta = estados[estado] ? t(`estado.${estado}` as Clave) : estado
  return { ...e, etiqueta, clase: 'etiqueta etiqueta--' + (clases[e.color] ?? 'neutro') }
}

export function fecha(texto?: string): string {
  return texto ? new Date(texto).toLocaleString(idioma.value) : '—'
}

// haceCuanto da la fecha relativa ("hace 3 minutos"); más de una semana, la fecha corta.
export function haceCuanto(texto?: string): string {
  if (!texto) return '—'
  const segundos = (new Date(texto).getTime() - Date.now()) / 1000
  const unidades: [Intl.RelativeTimeFormatUnit, number][] = [['second', 60], ['minute', 3600], ['hour', 86400], ['day', 604800]]
  let anterior = 1
  for (const [unidad, limite] of unidades) {
    if (Math.abs(segundos) < limite)
      return new Intl.RelativeTimeFormat(idioma.value, { numeric: 'auto' }).format(Math.round(segundos / anterior), unidad)
    anterior = limite
  }
  return new Date(texto).toLocaleDateString(idioma.value)
}

export function duracion(desde?: string, hasta?: string): string {
  if (!desde || !hasta) return ''
  const s = (new Date(hasta).getTime() - new Date(desde).getTime()) / 1000
  return s < 60 ? `${s.toFixed(1)} s` : `${Math.floor(s / 60)} min ${Math.round(s % 60)} s`
}

// numero da un número con los separadores del idioma («200.000»).
export const numero = (n: number) => n.toLocaleString(idioma.value)

export function nombreEsfuerzo(nivel?: string): string {
  return nivel ? t(`esfuerzo.${nivel}` as Clave) : t('esfuerzo.agente')
}

// esfuerzoUsado es el esfuerzo con que respondió la petición, según su último intento: el pedido o, si no se pidió,
// el del agente con «(del agente)»; sin intentos registrados, el de la solicitud. '' si no hubo esfuerzo.
export function esfuerzoUsado(trabajo: Trabajo, eventos: EventoRegistro[]): string {
  const intento = eventos.filter((e) => e.tipo === 'intento').at(-1)
  if (!intento) return trabajo.solicitud.esfuerzo ? nombreEsfuerzo(trabajo.solicitud.esfuerzo) : ''
  try {
    const i = JSON.parse(intento.datos) as { esfuerzo?: string; del_agente?: boolean }
    if (!i.esfuerzo) return ''
    return i.del_agente ? t('esfuerzo.delAgente', { nivel: nombreEsfuerzo(i.esfuerzo) }) : nombreEsfuerzo(i.esfuerzo)
  } catch {
    return ''
  }
}

export function textoDe(trabajo: Trabajo): string {
  return trabajo.solicitud.mensajes?.at(-1)?.contenido ?? (trabajo.entradas_eliminadas ? t('trabajo.entradaEliminada') : '')
}

// esJSON distingue una respuesta JSON (se muestra en monoespaciada) del texto normal.
export function esJSON(texto?: string): boolean {
  return !!texto && /^\s*[[{]/.test(texto)
}
