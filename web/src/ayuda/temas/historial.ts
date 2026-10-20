import type { Tema } from '../tipos'

// historial explica qué queda de cada petición y cómo revisarla.
export const historial: Tema = {
  id: 'historial',
  icono: 'o_history',
  titulo: 'ayuda.historial.titulo',
  resumen: 'ayuda.historial.resumen',
  paginaApp: 'historial',
  conDiagrama: false,
  relacionados: ['problemas', 'probar', 'ajustes'],
  bloques: [
    { tipo: 'pasos', pasos: ['ayuda.historial.paso1', 'ayuda.historial.paso2', 'ayuda.historial.paso3'] },
    { tipo: 'captura', id: 'historial', alt: 'ayuda.historial.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'estados', pregunta: 'ayuda.historial.pEstados', respuesta: 'ayuda.historial.rEstados' },
        { id: 'tiempos', pregunta: 'ayuda.historial.pTiempos', respuesta: 'ayuda.historial.rTiempos' },
        { id: 'en-vivo', pregunta: 'ayuda.historial.pVivo', respuesta: 'ayuda.historial.rVivo' },
        { id: 'borrar-datos', pregunta: 'ayuda.historial.pBorrar', respuesta: 'ayuda.historial.rBorrar' },
      ],
    },
    { tipo: 'ir', pagina: 'historial' },
  ],
}
