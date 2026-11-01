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
  { id: 'herramientas', pagina: 'conectar', guardado: { 'tramtai.pestanaApis': 'herramientas' } },
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

// POR_SACAR son las capturas de la Ayuda que un tema ya cita y todavía no se han sacado, con el motivo: mientras tanto el tema sale
// sin imagen (la Web UI y el sitio la omiten). Al sacarla (npm run capturas) la entrada sobra y la prueba lo avisa.
export const POR_SACAR: Record<string, string> = {
  herramientas: 'v0.59: la saca la Tarea 13 del plan, con la copia de prueba y sus dos peticiones reales a claude/haiku, que se avisan antes',
}

// TERMINAL son las páginas de la terminal de la portada (site/capturas/terminal.json), con las teclas que llevan a ella
// desde Inicio. Agentes no está: enseñaría los CLI reales del equipo que saca la captura.
export const TERMINAL: { nombre: 'inicio' | 'apis' | 'historial'; teclas: string[] }[] = [
  { nombre: 'inicio', teclas: [] },
  { nombre: 'apis', teclas: ['4'] },
  { nombre: 'historial', teclas: ['5'] },
]

// TOKENS_TERMINAL traduce cada color que pinta la TUI (internal/tui/tema.go, la paleta clara y la oscura de
// docs/diseno/DESIGN.md) al token de DESIGN.md: por eso un solo HTML sirve en los dos temas del sitio. Cada token lleva su
// color de día y el de noche; velo es el fondo de la fila elegida.
export const TOKENS_TERMINAL: Record<string, string> = {
  '#0b6e64': 'acento', '#2dd4bf': 'acento', '#0f1b1e': 'texto', '#e5edf0': 'texto', '#55666a': 'suave', '#93a5ac': 'suave',
  '#166534': 'ok', '#4ade80': 'ok', '#b42525': 'error', '#f87171': 'error', '#854d0e': 'aviso', '#fbbf24': 'aviso',
  '#c4561d': 'claude', '#f28a4f': 'claude', '#2563eb': 'codex', '#5b8def': 'codex', '#1f883d': 'copilot', '#3fb950': 'copilot',
  '#7c3aed': 'gemini', '#a78bfa': 'gemini', '#db2777': 'antigravity', '#f472b6': 'antigravity', '#d3e5e3': 'velo', '#113838': 'velo',
}

// ansiAHtml convierte lo que pinta la terminal en HTML para la portada: cada trozo con el mismo estilo es un
// <span class="…"> con t-<token> (tinta), tf-<token> (fondo), t-b (negrita) y t-d (tenue); la hoja del sitio da a cada
// clase el valor de su tema. Solo entiende lo que usa la TUI: color de 24 bits que esté en TOKENS_TERMINAL, negrita y
// tenue. Un color sin token, o cualquier otro estilo (16 y 256 colores, inverso, subrayado…), lanza un error: así no se
// cuela un color suelto ni un estilo que se pierde sin avisar. Lo demás (mover el cursor, borrar, enlaces OSC 8) se quita.
export function ansiAHtml(ansi: string): string {
  const limpio = ansi.replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, '').replace(/\x1b\[[0-9;?]*[A-Za-ln-z]/g, '')
  const vacio = { fg: '', bg: '', negrita: false, tenue: false }
  let e = { ...vacio }
  let html = '', clase = '', texto = ''
  const volcar = () => {
    if (texto) html += clase ? `<span class="${clase}">${texto}</span>` : texto
    texto = ''
  }
  const token = (hex: string) => {
    const t = TOKENS_TERMINAL[hex]
    if (!t) throw new Error(`ansiAHtml: el color ${hex} no tiene token en TOKENS_TERMINAL`)
    return t
  }
  for (const trozo of limpio.split(/(\x1b\[[0-9;]*m)/)) {
    const sgr = trozo.match(/^\x1b\[([0-9;]*)m$/)
    if (!sgr) {
      if (!trozo) continue
      const nueva = [e.fg && 't-' + e.fg, e.bg && 'tf-' + e.bg, e.negrita && 't-b', e.tenue && 't-d'].filter(Boolean).join(' ')
      if (nueva !== clase) { volcar(); clase = nueva }
      texto += trozo.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      continue
    }
    const p = sgr[1] === '' ? [0] : sgr[1].split(';').map(Number)
    for (let i = 0; i < p.length; i++) {
      const n = p[i]
      if (n === 0) e = { ...vacio }
      else if (n === 1) e.negrita = true
      else if (n === 2) e.tenue = true
      else if (n === 22) e.negrita = e.tenue = false
      else if (n === 39) e.fg = ''
      else if (n === 49) e.bg = ''
      else if ((n === 38 || n === 48) && p[i + 1] === 2) {
        const [r, g, b] = p.slice(i + 2, i + 5)
        e[n === 38 ? 'fg' : 'bg'] = token('#' + [r, g, b].map((v) => v.toString(16).padStart(2, '0')).join(''))
        i += 4
      } else throw new Error(`ansiAHtml: el estilo SGR ${n} no se sabe pintar`)
    }
  }
  volcar()
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
