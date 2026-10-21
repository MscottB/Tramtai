import { temas } from '../src/ayuda/indice.ts'
import { capturaDeAncla } from '../src/ayuda/vista.ts'
import { pasosBienvenida } from '../src/recorrido.ts'

// CAPTURAS son las zonas de la app que muestra la Ayuda: cada una es el elemento [data-captura=<id>] de esa página.
// PORTADA son las pantallas completas de la página del producto (site/capturas/<nombre>-<claro|oscuro>-<640|960|1440>.webp).
// guardado son valores de localStorage que la página necesita (la pestaña de Ajustes donde está la zona).
export const CAPTURAS: { id: string; pagina: string; guardado?: Record<string, unknown> }[] = [
  { id: 'inicio', pagina: 'inicio' },
  { id: 'agente', pagina: 'agentes' },
  { id: 'receta', pagina: 'ejecutar' },
  { id: 'mis-apis', pagina: 'conectar' },
  { id: 'presupuesto', pagina: 'conectar' },
  { id: 'historial', pagina: 'historial' },
  { id: 'conversor', pagina: 'configuracion', guardado: { 'tramtai.pestanaAjustes': 'tramtai' } },
  { id: 'actualizaciones', pagina: 'configuracion', guardado: { 'tramtai.pestanaAjustes': 'tramtai' } },
  { id: 'espacios', pagina: 'espacios' },
]

// ANCLAS son los elementos que señalan los recorridos guiados (data-recorrido): una captura por ancla aunque salga en
// varias páginas, la primera que la cita. Salen de los temas, así que un paso nuevo trae su captura sin tocar esta lista.
export interface AnclaCaptura { ancla: string; id: string; pagina: string; preparar?: string }
// PREPARAR deja visible un ancla que la página no muestra al abrirla: es JavaScript que corre en la página y da true
// cuando ya hizo lo suyo (se repite hasta entonces, por si los datos aún no llegaron).
const PREPARAR: Record<string, string> = {
  // el detalle de un trabajo es el panel que se abre al pulsar una fila del Historial
  'historial.detalle': `(() => { const fila = document.querySelector('[data-recorrido="historial.lista"] tbody tr'); fila?.click(); return !!fila })()`,
}
export const ANCLAS: AnclaCaptura[] = []
for (const tema of temas)
  for (const bloque of tema.bloques)
    if (bloque.tipo === 'recorrido')
      for (const { ancla } of bloque.pasos)
        if (tema.paginaApp && !ANCLAS.some((a) => a.ancla === ancla))
          ANCLAS.push({ ancla, id: capturaDeAncla(ancla), pagina: tema.paginaApp, preparar: PREPARAR[ancla] })

// SIN_CAPTURA son las anclas que la demo no puede mostrar, con el motivo: su paso sale en el sitio sin imagen.
// Primero se intenta que los datos de demostración la hagan visible; esto es el último recurso.
export const SIN_CAPTURA: Record<string, string> = {}

// revisarAnclas dice qué falta o sobra: cada ancla tiene sus 4 capturas (claro/oscuro × es/en), salvo las de SIN_CAPTURA,
// que no deben tener ninguna (si la demo ya la muestra, la excepción sobra).
export function revisarAnclas(anclas: string[], sinCaptura: Record<string, string>, archivos: string[]): string[] {
  return anclas.flatMap((ancla) => ['claro', 'oscuro'].flatMap((tema) => ['es', 'en'].flatMap((idioma) => {
    const v = `${capturaDeAncla(ancla)}.${tema}.${idioma}.webp`
    if (!(ancla in sinCaptura)) return archivos.includes(v) ? [] : [`falta ${v}`]
    return archivos.includes(v) ? [`sobra ${v}: ${ancla} ya no está en SIN_CAPTURA`] : []
  })))
}

// ANCHO_AYUDA es el ancho de la columna de la Ayuda del sitio en escritorio (52em): una captura más ancha se saca
// reducida a este ancho, así no se descargan píxeles que no se ven (Lighthouse, image-delivery). En la app se ven igual
// de nítidas: nunca se amplían.
export const ANCHO_AYUDA = 736

// rutaPrivada reconoce una ruta del equipo de quien saca las capturas: sus carpetas (la personal, la temporal y la del
// repositorio, con / o \ y sin distinguir mayúsculas) y la carpeta de trabajo del script (capturas-XXXXXX). La ruta de
// ejemplo que sale a propósito (C:\Proyectos\mi-app) no cuenta. Ni el nombre de usuario suelto (Administrator saldría
// en «administrator rights») ni una carpeta de un solo nivel (/tmp) se buscan: saldrían en textos de la app.
export function rutaPrivada(rutas: string[]): RegExp {
  const escapar = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const niveles = (r: string) => r.split(/[\\/]+/).filter(Boolean)
  const partes = rutas.filter((r) => niveles(r).length >= 2).map((r) => niveles(r).map(escapar).join('[\\\\/]+'))
  return new RegExp([...partes, 'capturas-\\w{6}'].join('|'), 'i')
}

// medidas lee el ancho y el alto de la cabecera de un WebP (VP8X, VP8 con pérdida o VP8L sin pérdida).
export function medidas(b: Buffer): { ancho: number; alto: number } {
  const tipo = b.toString('ascii', 12, 16)
  if (tipo === 'VP8X') return { ancho: 1 + b.readUIntLE(24, 3), alto: 1 + b.readUIntLE(27, 3) }
  if (tipo === 'VP8 ') return { ancho: b.readUInt16LE(26) & 0x3fff, alto: b.readUInt16LE(28) & 0x3fff }
  if (tipo === 'VP8L') {
    const v = b.readUInt32LE(21)
    return { ancho: 1 + (v & 0x3fff), alto: 1 + ((v >> 14) & 0x3fff) }
  }
  throw new Error('WebP desconocido: ' + tipo)
}

// BIENVENIDA son los pasos de la bienvenida, en su orden: la demo de la portada tiene una pantalla completa por paso
// (site/capturas/bienvenida-<n>-<claro|oscuro>-<es|en>-<960|1440>.webp, n desde 1), con la ventana del recorrido encima.
export const BIENVENIDA = pasosBienvenida()

export const PORTADA: { nombre: string; pagina: string }[] = [
  { nombre: 'inicio', pagina: 'inicio' },
  { nombre: 'agentes', pagina: 'agentes' },
  { nombre: 'mis-apis', pagina: 'conectar' },
  { nombre: 'probar', pagina: 'ejecutar' },
  { nombre: 'espacios', pagina: 'espacios' },
]
