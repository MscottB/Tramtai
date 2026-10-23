// pasos pone sus textos a los Primeros pasos que calcula el servicio (GET /api/v1/instancia/pasos). Sin Vue, para poder
// probarlo: Inicio solo pinta lo que sale de aquí.
import { t } from './i18n'
import type { Paso } from './api/tipos'

export interface PasoConTexto {
  titulo: string
  detalle: string
  hecho: boolean
  pagina: string
}

function textos(p: Paso, sistema: string) {
  switch (p.pagina) {
    case 'agentes':
      return { titulo: t('inicio.paso1'), detalle: t('inicio.paso1Detalle', { listos: p.listos, total: p.total }) }
    case 'ejecutar':
      return { titulo: t('inicio.paso2'), detalle: t('inicio.paso2Detalle') }
    case 'conectar':
      return { titulo: t('inicio.paso3'), detalle: t('inicio.paso3Detalle') }
    case 'configuracion':
      return { titulo: t('inicio.paso4', { sistema }), detalle: t('inicio.paso4Detalle') }
  }
}

// conTexto son los pasos que llegan, en su orden y con su estado; uno de una página que esta Web UI no conoce no sale.
export function conTexto(pasos: Paso[], sistema: string): PasoConTexto[] {
  return pasos.flatMap((p) => {
    const x = textos(p, sistema)
    return x ? [{ ...x, hecho: p.hecho, pagina: p.pagina }] : []
  })
}

// guiaVisible dice si Inicio enseña la guía: solo se esconde si la ocultaste o si llegaron pasos y están todos hechos.
// Sin lista (cargando o con error) sigue a la vista, para que un fallo no parezca una guía completa.
export const guiaVisible = (oculta: boolean, pasos?: Paso[]) => !oculta && !(pasos?.length && pasos.every((p) => p.hecho))
