// Textos para mostrar: estados, fechas, duraciones, números y esfuerzos en el idioma de la Web UI.
import { idioma, t } from './i18n'
import type { Clave } from './idiomas/es'
import type { EventoRegistro, Trabajo } from './api/tipos'

export const terminales = ['completado', 'fallido', 'cancelado', 'interrumpido']

// estados da a cada estado el color de su etiqueta (punto y texto, sin icono); el nombre sale de las traducciones.
const estados: Record<string, 'neutro' | 'info' | 'aviso' | 'ok' | 'error'> = {
  en_cola: 'neutro',
  preparando_entrada: 'info',
  esperando_agente: 'info',
  ejecutando: 'info',
  validando_salida: 'info',
  cancelacion_solicitada: 'aviso',
  completado: 'ok',
  fallido: 'error',
  cancelado: 'neutro',
  interrumpido: 'aviso',
}

export function estadoInfo(estado: string) {
  const etiqueta = estados[estado] ? t(`estado.${estado}` as Clave) : estado
  return { etiqueta, clase: 'etiqueta etiqueta--' + (estados[estado] ?? 'neutro') }
}

// sinDato es el valor de un dato que falta, en el idioma de la Web UI: el único de esta capa.
export const sinDato = () => t('comun.sinDato')

export function fecha(texto?: string): string {
  return texto ? new Date(texto).toLocaleString(idioma.value) : sinDato()
}

// haceCuanto da la fecha relativa ("hace 3 minutos"); más de una semana, la fecha corta.
export function haceCuanto(texto?: string): string {
  if (!texto) return sinDato()
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

// enRango es la regla de un campo numérico (rules de Quasar): un número entre min y max, entero si se pide; si no, el
// mensaje con los límites.
export const enRango = (min: number, max: number, entero: boolean) => (v: unknown) =>
  (typeof v === 'number' && v >= min && v <= max && (!entero || Number.isInteger(v))) || t('historialAjustes.rango', { min: numero(min), max: numero(max) })

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
