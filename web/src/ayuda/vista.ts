// vista tiene lo que deciden los componentes de la Ayuda y el generador del sitio, sin Vue para poder probarlo.
import type { Bloque, Pagina, Tema } from './tipos'

export type Contexto = 'app' | 'panel' | 'sitio'

// varianteCaptura es el archivo de una captura para el tema de colores y el idioma que se ven.
export function varianteCaptura(id: string, oscuro: boolean, idioma: 'es' | 'en'): string {
  return `${id}.${oscuro ? 'oscuro' : 'claro'}.${idioma}.webp`
}

export function altDiagrama(paradas: string[]): string {
  return paradas.map((p, i) => `${i + 1}. ${p}`).join(' → ')
}

// bloquesVisibles: las descargas solo en el sitio; el botón a una página de la app, fuera del sitio y, en el panel,
// fuera si lleva a la página donde ya estás.
export function bloquesVisibles(tema: Tema, contexto: Contexto, paginaActual?: Pagina): Bloque[] {
  return tema.bloques.filter((b) => {
    if (b.tipo === 'descargas') return contexto === 'sitio'
    if (b.tipo === 'ir') return contexto !== 'sitio' && !(contexto === 'panel' && b.pagina === paginaActual)
    return true
  })
}
