import { temas } from '../src/ayuda/indice.ts'
import { capturaDeAncla } from '../src/ayuda/vista.ts'
import { pasosBienvenida } from '../src/recorrido.ts'

// CAPTURAS son las zonas de la app que muestra la Ayuda: cada una es el elemento [data-captura=<id>] de esa página.
// PORTADA son las pantallas completas de la página del producto (site/capturas/<nombre>-<claro|oscuro>-<960|1440>.webp).
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

// BIENVENIDA son los pasos de la bienvenida, en su orden: la demo de la portada tiene una pantalla completa por paso
// (site/capturas/bienvenida-<n>-<claro|oscuro>-<960|1440>.webp, n desde 1), con la ventana del recorrido encima.
export const BIENVENIDA = pasosBienvenida()

export const PORTADA: { nombre: string; pagina: string }[] = [
  { nombre: 'inicio', pagina: 'inicio' },
  { nombre: 'agentes', pagina: 'agentes' },
  { nombre: 'mis-apis', pagina: 'conectar' },
  { nombre: 'probar', pagina: 'ejecutar' },
  { nombre: 'espacios', pagina: 'espacios' },
]
