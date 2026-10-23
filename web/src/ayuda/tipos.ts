// Forma de la Ayuda: un tema es una lista de bloques de tipos fijos. Los mismos datos los dibujan la app
// (componentes/ayuda) y el generador del sitio (web/scripts), así que solo importan tipos, nunca i18n ni Vue.
import type { Clave } from '../idiomas/es'

// PAGINAS son las páginas principales de la app (App.vue las usa con este tipo); cada una tiene un tema que abre su
// botón ?.
export const PAGINAS = ['inicio', 'agentes', 'ejecutar', 'conectar', 'historial', 'espacios', 'configuracion'] as const
export type Pagina = (typeof PAGINAS)[number]

export interface Parada {
  icono: string
  titulo: Clave
  nota?: Clave
}

// PasoRecorrido es un paso del recorrido guiado de una página: ancla es el data-recorrido del elemento que señala y
// bienvenida lo incluye también en el recorrido de bienvenida.
export interface PasoRecorrido {
  ancla: string
  titulo: Clave
  texto: Clave
  bienvenida?: boolean
}

export type Bloque =
  // ramas une dos paradas (índices desde 0) fuera de la línea principal, como el paso al respaldo.
  | { tipo: 'diagrama'; paradas: Parada[]; ramas?: { desde: number; hacia: number; etiqueta: Clave }[] }
  | { tipo: 'pasos'; pasos: Clave[] }
  | { tipo: 'captura'; id: string; alt: Clave }
  // ejemplo es el código de un mensaje (con esos archivos) por cada contrato, como lo arma ejemplos().
  | { tipo: 'ejemplo'; contratos: string[]; archivos: string[] }
  | { tipo: 'tabla'; columnas: Clave[]; filas: { titulo: Clave; celdas: Clave[] }[] }
  // id es el ancla de cada pregunta (la búsqueda y los enlaces llevan a ella); único en toda la Ayuda. tema enlaza la
  // respuesta con el tema que la explica (los conceptos). soloWeb deja la pregunta fuera de la Ayuda de la terminal,
  // donde no se puede hacer lo que dice (el recorrido guiado, el mapa de Inicio). Un texto con otra forma en la terminal
  // lleva su variante <clave>.terminal en el catálogo.
  | { tipo: 'preguntas'; preguntas: { id: string; pregunta: Clave; respuesta: Clave; tema?: string; soloWeb?: true }[] }
  | { tipo: 'errores'; codigos: string[] }
  | { tipo: 'nota'; texto: Clave }
  | { tipo: 'ir'; pagina: Pagina }
  | { tipo: 'descargas' }
  | { tipo: 'recorrido'; pasos: PasoRecorrido[] }

export interface Tema {
  id: string
  icono: string
  titulo: Clave
  resumen: Clave
  // terminal dice cómo se hace en la terminal, con sus teclas («En la terminal: …»). Solo lo enseña la Ayuda de la
  // terminal, donde no valen el botón ? ni el panel de la Web UI.
  terminal: Clave
  paginaApp?: Pagina
  conDiagrama: boolean
  relacionados: string[]
  bloques: Bloque[]
}
