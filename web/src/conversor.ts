// conversor lleva el estado del conversor de archivos (markitdown) que comparten Probar, Ajustes e Inicio, y sigue
// una preparación hasta que termina.
import { ref } from 'vue'
import { api, avisarError } from './api/cliente'
import type { EstadoConversor } from './api/tipos'
import { extensionesConversor } from './api/tipos.gen'
import { t } from './i18n'
import type { Clave } from './idiomas/es'

// Formatos son las extensiones que admite el servicio (/api/v1/limites).
type Formatos = { extensiones_texto?: string[]; extensiones_conversor?: string[] }
const extensiones = (f?: Formatos) => [...(f?.extensiones_conversor ?? []), ...(f?.extensiones_texto ?? [])]

// aceptar es el filtro del selector de archivos; sin la lista (un servicio anterior) no filtra y decide el servicio.
export const aceptar = (f?: Formatos) => extensiones(f).join(',') || undefined

// listaFormatos es la lista para leer: «DOCX, PDF, MD, TXT».
export const listaFormatos = (f?: Formatos) => extensiones(f).map((e) => e.slice(1).toUpperCase()).join(', ')

export function necesitaConversor(nombre: string): boolean {
  const punto = nombre.lastIndexOf('.')
  return punto >= 0 && extensionesConversor.includes(nombre.slice(punto).toLowerCase())
}

// conversor es null mientras no se sabe, o si el servicio no tiene conversor (entonces no se avisa de nada).
export const conversor = ref<EstadoConversor | null>(null)

// preparando dice si hay una preparación en curso: «preparando», o «listo» con etapa (sirve el anterior mientras tanto).
export const preparando = (e: EstadoConversor | null) => !!e && (e.estado === 'preparando' || !!e.etapa)

// seguirPreparacion consulta el estado hasta que deja de preparar; esperar separa las consultas.
export async function seguirPreparacion(obtener: () => Promise<EstadoConversor>, alCambiar: (e: EstadoConversor) => void,
  esperar: () => Promise<void> = () => new Promise((listo) => setTimeout(listo, 2000))): Promise<EstadoConversor> {
  let e = await obtener()
  alCambiar(e)
  while (preparando(e)) {
    await esperar()
    e = await obtener()
    alCambiar(e)
  }
  return e
}

let siguiendo = false
async function seguir() {
  if (siguiendo) return
  siguiendo = true
  try {
    await seguirPreparacion(api.conversor, (e) => (conversor.value = e))
  } catch (e) {
    avisarError(e)
  } finally {
    siguiendo = false
  }
}

export async function cargarConversor() {
  try {
    conversor.value = await api.conversor()
  } catch {
    conversor.value = null // sin conversor en este servicio: no hay nada que avisar
  }
  if (preparando(conversor.value)) void seguir()
}

export async function prepararConversor() {
  try {
    conversor.value = await api.prepararConversor()
    await seguir()
  } catch (e) {
    avisarError(e)
  }
}

// textoConversor describe el estado en una frase.
export function textoConversor(e: EstadoConversor): string {
  switch (e.estado) {
    case 'listo': {
      const origen = t(`conversor.origen.${e.origen ?? 'sistema'}` as Clave)
      const listo = e.version ? t('conversor.listo', { version: e.version, origen }) : t('conversor.listoSinVersion', { origen })
      if (e.etapa) return `${listo} ${t('conversor.preparandoNuevo', { etapa: t(`conversor.etapa.${e.etapa}` as Clave), progreso: e.progreso ?? 0 })}`
      return e.mensaje ? `${listo} ${t('conversor.ultimoError', { mensaje: e.mensaje })}` : listo
    }
    case 'preparando':
      return t('conversor.preparando', { etapa: t(`conversor.etapa.${e.etapa ?? 'python'}` as Clave), progreso: e.progreso ?? 0 })
    case 'error':
      return t('conversor.error', { mensaje: e.mensaje ?? '' })
    default:
      return t('conversor.falta')
  }
}
