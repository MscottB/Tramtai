import { temas } from '../src/ayuda/indice.ts'
import { capturaDeAncla } from '../src/ayuda/vista.ts'
import { pasosBienvenida } from '../src/recorrido.ts'

// CAPTURAS son las zonas de la app que muestra la Ayuda: cada una es el elemento [data-captura=<id>] de esa página.
// PORTADA son las pantallas completas de la página del producto (site/capturas/<nombre>-<claro|oscuro>-<640|960|1440>.webp).
// guardado son valores de localStorage que la página necesita (la pestaña de Ajustes donde está la zona). servidor dice
// que sale del Tramtai en modo servidor (en contenedores, detrás de Caddy); cabecera, que la zona es la cabecera fija;
// ancho, el de la ventana si no es el de siempre (1280).
export const CAPTURAS: { id: string; pagina: string; guardado?: Record<string, unknown>; servidor?: true; cabecera?: true; ancho?: number }[] = [
  { id: 'inicio', pagina: 'inicio' },
  { id: 'agente', pagina: 'agentes' },
  { id: 'receta', pagina: 'ejecutar' },
  { id: 'mis-apis', pagina: 'conectar' },
  { id: 'presupuesto', pagina: 'conectar' },
  { id: 'historial', pagina: 'historial' },
  { id: 'conversor', pagina: 'configuracion', guardado: { 'tramtai.pestanaAjustes': 'tramtai' } },
  { id: 'actualizaciones', pagina: 'configuracion', guardado: { 'tramtai.pestanaAjustes': 'tramtai' } },
  { id: 'espacios', pagina: 'espacios' },
  { id: 'arranque', pagina: 'configuracion', guardado: { 'tramtai.pestanaAjustes': 'tramtai' } },
  { id: 'entrar', pagina: 'inicio', servidor: true },
  // La cabecera, en una ventana estrecha: así cabe entera, con la marca, en la columna de la Ayuda sin reducirse.
  { id: 'marca', pagina: 'inicio', servidor: true, cabecera: true, ancho: 720 },
  { id: 'servidor', pagina: 'configuracion', guardado: { 'tramtai.pestanaAjustes': 'servidor' }, servidor: true },
  { id: 'seguridad', pagina: 'configuracion', guardado: { 'tramtai.pestanaAjustes': 'seguridad' }, servidor: true },
]

// TERMINAL son las páginas de la terminal de la portada (site/capturas/terminal-<nombre>-<claro|oscuro>-<es|en>-
// <960|1440>.webp), con las teclas que llevan a ella desde Inicio.
export const TERMINAL: { nombre: string; teclas: string[] }[] = [
  { nombre: 'inicio', teclas: [] },
  { nombre: 'apis', teclas: ['4'] },
]

// ANSI16 son los 16 colores básicos (los de la paleta de Windows Terminal); 256 sigue la tabla de xterm.
const ANSI16 = ['#0c0c0c', '#c50f1f', '#13a10e', '#c19c00', '#0037da', '#881798', '#3a96dd', '#cccccc',
  '#767676', '#e74856', '#16c60c', '#f9f1a5', '#3b78ff', '#b4009e', '#61d6d6', '#f2f2f2']
const hex = (r: number, g: number, b: number) => '#' + [r, g, b].map((v) => v.toString(16).padStart(2, '0')).join('')
function color256(n: number): string {
  if (n < 16) return ANSI16[n]
  if (n >= 232) { const v = 8 + (n - 232) * 10; return hex(v, v, v) }
  const c = n - 16, nivel = (x: number) => (x ? 55 + x * 40 : 0)
  return hex(nivel(Math.floor(c / 36)), nivel(Math.floor(c / 6) % 6), nivel(c % 6))
}

// ansiAHtml convierte lo que pinta la terminal (SGR: negrita, tenue, cursiva, subrayado, inverso y color de 16, 256 y
// 24 bits) en HTML con estilos en línea, para sacarle una captura en el navegador. Lo demás (mover el cursor, borrar,
// enlaces OSC 8) se quita. tema da los colores por defecto, que el inverso intercambia.
export function ansiAHtml(ansi: string, tema: { fondo: string; texto: string }): string {
  const limpio = ansi.replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, '').replace(/\x1b\[[0-9;?]*[A-Za-ln-z]/g, '')
  let e = { fg: '', bg: '', negrita: false, tenue: false, cursiva: false, subrayado: false, inverso: false }
  const inicial = { ...e }
  let html = ''
  for (const trozo of limpio.split(/(\x1b\[[0-9;]*m)/)) {
    const sgr = trozo.match(/^\x1b\[([0-9;]*)m$/)
    if (!sgr) {
      if (!trozo) continue
      const texto = trozo.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      let fg = e.fg, bg = e.bg
      if (e.inverso) [fg, bg] = [bg || tema.fondo, fg || tema.texto]
      const estilo = [fg && `color:${fg}`, bg && `background:${bg}`, e.negrita && 'font-weight:bold', e.tenue && 'opacity:.65',
        e.cursiva && 'font-style:italic', e.subrayado && 'text-decoration:underline'].filter(Boolean).join(';')
      html += estilo ? `<span style="${estilo}">${texto}</span>` : texto
      continue
    }
    const p = sgr[1] === '' ? [0] : sgr[1].split(';').map(Number)
    for (let i = 0; i < p.length; i++) {
      const n = p[i]
      if (n === 0) e = { ...inicial }
      else if (n === 1) e.negrita = true
      else if (n === 2) e.tenue = true
      else if (n === 22) e.negrita = e.tenue = false
      else if (n === 3 || n === 23) e.cursiva = n === 3
      else if (n === 4 || n === 24) e.subrayado = n === 4
      else if (n === 7 || n === 27) e.inverso = n === 7
      else if (n === 39) e.fg = ''
      else if (n === 49) e.bg = ''
      else if ((n === 38 || n === 48) && p[i + 1] === 5) { e[n === 38 ? 'fg' : 'bg'] = color256(p[i + 2]); i += 2 }
      else if ((n === 38 || n === 48) && p[i + 1] === 2) { e[n === 38 ? 'fg' : 'bg'] = hex(p[i + 2], p[i + 3], p[i + 4]); i += 4 }
      else if (n >= 30 && n <= 37) e.fg = ANSI16[n - 30]
      else if (n >= 90 && n <= 97) e.fg = ANSI16[n - 82]
      else if (n >= 40 && n <= 47) e.bg = ANSI16[n - 40]
      else if (n >= 100 && n <= 107) e.bg = ANSI16[n - 92]
    }
  }
  return html
}

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

// BIENVENIDA son los pasos de la bienvenida, en su orden: la demo de la portada tiene una captura por paso, la zona que
// señala sin la ventana del recorrido (site/capturas/bienvenida-<n>-<claro|oscuro>-<es|en>-<960|1440>.webp, n desde 1).
export const BIENVENIDA = pasosBienvenida()

export const PORTADA: { nombre: string; pagina: string }[] = [
  { nombre: 'inicio', pagina: 'inicio' },
  { nombre: 'agentes', pagina: 'agentes' },
  { nombre: 'mis-apis', pagina: 'conectar' },
  { nombre: 'probar', pagina: 'ejecutar' },
  { nombre: 'espacios', pagina: 'espacios' },
]
