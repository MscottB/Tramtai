// Forma de la Ayuda: un tema es una lista de bloques de tipos fijos. Los mismos datos los dibujan la app
// (componentes/ayuda) y el generador del sitio (web/scripts), así que solo importan tipos, nunca i18n ni Vue.
import type { Clave } from '../idiomas/es'

// Pagina son las páginas principales de la app; cada una tiene un tema que abre su botón ?.
export type Pagina = 'inicio' | 'agentes' | 'ejecutar' | 'conectar' | 'historial' | 'configuracion'

export interface Parada {
  icono: string
  titulo: Clave
  nota?: Clave
}

export type Bloque =
  // ramas une dos paradas (índices desde 0) fuera de la línea principal, como el paso al respaldo.
  | { tipo: 'diagrama'; paradas: Parada[]; ramas?: { desde: number; hacia: number; etiqueta: Clave }[] }
  | { tipo: 'pasos'; pasos: Clave[] }
  | { tipo: 'captura'; id: string; alt: Clave }
  // ejemplo es el código de un mensaje (con esos archivos) por cada contrato, como lo arma ejemplos().
  | { tipo: 'ejemplo'; contratos: string[]; archivos: string[] }
  | { tipo: 'tabla'; columnas: Clave[]; filas: { titulo: Clave; celdas: Clave[] }[] }
  // id es el ancla de cada pregunta (la búsqueda y los enlaces llevan a ella); único en toda la Ayuda.
  | { tipo: 'preguntas'; preguntas: { id: string; pregunta: Clave; respuesta: Clave }[] }
  | { tipo: 'errores'; codigos: string[] }
  | { tipo: 'nota'; texto: Clave }
  | { tipo: 'ir'; pagina: Pagina }
  | { tipo: 'descargas' }

export interface Tema {
  id: string
  icono: string
  titulo: Clave
  resumen: Clave
  paginaApp?: Pagina
  conDiagrama: boolean
  relacionados: string[]
  bloques: Bloque[]
}
